package hot

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/go-singleflightx"
	"github.com/samber/hot/internal"
	"github.com/samber/hot/pkg/base"
	"github.com/samber/hot/pkg/metrics"
)

var _ prometheus.Collector = (*HotCache[any, any])(nil)

// newHotCache creates a new HotCache instance with the specified configuration.
// This is an internal constructor used by the builder pattern.
func newHotCache[K comparable, V any](
	cache base.InMemoryCache[K, *item[V]],
	missingSharedCache bool,
	missingCache base.InMemoryCache[K, *item[V]],

	ttl time.Duration,
	stale time.Duration,
	jitterLambda float64,
	jitterUpperBound time.Duration,

	loaderFns LoaderChain[K, V],
	revalidationLoaderFns LoaderChain[K, V],
	revalidationErrorPolicy revalidationErrorPolicy,
	onEviction base.EvictionCallback[K, V],
	copyOnRead func(V) V,
	copyOnWrite func(V) V,

	prometheusCollectors []metrics.Collector,
) *HotCache[K, V] {
	return &HotCache[K, V]{
		cache:              cache,
		missingSharedCache: missingSharedCache,
		missingCache:       missingCache,

		// Store int64 nanoseconds instead of time.Time for better performance
		// (benchmark resulted in 10x speedup)
		ttlNano:          ttl.Nanoseconds(),
		staleNano:        stale.Nanoseconds(),
		jitterLambda:     jitterLambda,
		jitterUpperBound: jitterUpperBound,

		loaderFns:               loaderFns,
		revalidationLoaderFns:   revalidationLoaderFns,
		revalidationErrorPolicy: revalidationErrorPolicy,
		onEviction:              onEviction,
		copyOnRead:              copyOnRead,
		copyOnWrite:             copyOnWrite,

		group: singleflightx.Group[K, V]{},

		prometheusCollectors: prometheusCollectors,
	}
}

// HotCache is the main cache implementation that provides all caching functionality.
// It supports various eviction policies, TTL, revalidation, and missing key caching.
type HotCache[K comparable, V any] struct {
	// janitorMutex protects the janitor state (ticker, stopJanitor, janitorDone)
	// This prevents race conditions when multiple goroutines call Janitor() or StopJanitor()
	janitorMutex sync.RWMutex
	ticker       *time.Ticker
	stopOnce     *sync.Once
	stopJanitor  chan struct{}
	janitorDone  chan struct{}

	cache              base.InMemoryCache[K, *item[V]]
	missingSharedCache bool
	missingCache       base.InMemoryCache[K, *item[V]]

	// Store int64 nanoseconds instead of time.Time for better performance
	// (benchmark resulted in 10x speedup)
	ttlNano          int64
	staleNano        int64
	jitterLambda     float64
	jitterUpperBound time.Duration

	loaderFns               LoaderChain[K, V]
	revalidationLoaderFns   LoaderChain[K, V]
	revalidationErrorPolicy revalidationErrorPolicy
	onEviction              base.EvictionCallback[K, V]
	copyOnRead              func(V) V
	copyOnWrite             func(V) V

	group singleflightx.Group[K, V]

	// Prometheus collector for metrics registration
	prometheusCollectors []metrics.Collector
}

// Set adds a value to the cache. If the key already exists, its value is updated.
// Uses the default TTL configured for the cache.
func (c *HotCache[K, V]) Set(key K, v V) {
	if c.copyOnWrite != nil {
		v = c.copyOnWrite(v)
	}

	c.setUnsafe(key, true, v, c.ttlNano)
}

// SetMissing adds a key to the missing cache to prevent repeated lookups for non-existent keys.
// If the key already exists, its value is dropped. Uses the default TTL configured for the cache.
// Panics if missing cache is not enabled.
func (c *HotCache[K, V]) SetMissing(key K) {
	if c.missingCache == nil && !c.missingSharedCache {
		panic("missing cache is not enabled")
	}

	c.setUnsafe(key, false, zero[V](), c.ttlNano)
}

// SetWithTTL adds a value to the cache with a specific TTL duration.
// If the key already exists, its value is updated.
func (c *HotCache[K, V]) SetWithTTL(key K, v V, ttl time.Duration) {
	if c.copyOnWrite != nil {
		v = c.copyOnWrite(v)
	}

	c.setUnsafe(key, true, v, ttl.Nanoseconds())
}

// SetMissingWithTTL adds a key to the missing cache with a specific TTL duration.
// If the key already exists, its value is dropped.
// Panics if missing cache is not enabled.
func (c *HotCache[K, V]) SetMissingWithTTL(key K, ttl time.Duration) {
	if c.missingCache == nil && !c.missingSharedCache {
		panic("missing cache is not enabled")
	}

	c.setUnsafe(key, false, zero[V](), ttl.Nanoseconds())
}

// SetMany adds multiple values to the cache in a single operation.
// If keys already exist, their values are updated. Uses the default TTL configured for the cache.
func (c *HotCache[K, V]) SetMany(items map[K]V) {
	if c.copyOnWrite != nil {
		cOpy := map[K]V{}
		for k, v := range items {
			cOpy[k] = c.copyOnWrite(v)
		}
		items = cOpy
	}

	c.setManyUnsafe(items, []K{}, c.ttlNano)
}

// SetMissingMany adds multiple keys to the missing cache in a single operation.
// If keys already exist, their values are dropped. Uses the default TTL configured for the cache.
// Panics if missing cache is not enabled.
func (c *HotCache[K, V]) SetMissingMany(missingKeys []K) {
	if c.missingCache == nil && !c.missingSharedCache {
		panic("missing cache is not enabled")
	}

	c.setManyUnsafe(map[K]V{}, missingKeys, c.ttlNano)
}

// SetManyWithTTL adds multiple values to the cache with a specific TTL duration.
// If keys already exist, their values are updated.
func (c *HotCache[K, V]) SetManyWithTTL(items map[K]V, ttl time.Duration) {
	if c.copyOnWrite != nil {
		for k, v := range items {
			items[k] = c.copyOnWrite(v)
		}
	}

	c.setManyUnsafe(items, []K{}, ttl.Nanoseconds())
}

// SetMissingManyWithTTL adds multiple keys to the missing cache with a specific TTL duration.
// If keys already exist, their values are dropped.
// Panics if missing cache is not enabled.
func (c *HotCache[K, V]) SetMissingManyWithTTL(missingKeys []K, ttl time.Duration) {
	if c.missingCache == nil && !c.missingSharedCache {
		panic("missing cache is not enabled")
	}

	c.setManyUnsafe(map[K]V{}, missingKeys, ttl.Nanoseconds())
}

// Has checks if a key exists in the cache and has a valid value.
// Missing values (cached as missing) are not considered valid, even if cached.
func (c *HotCache[K, V]) Has(key K) bool {
	v, ok := c.cache.Peek(key)
	return ok && v.hasValue
}

// HasMany checks if multiple keys exist in the cache and have valid values.
// Missing values (cached as missing) are not considered valid, even if cached.
// Returns a map where keys are the input keys and values indicate whether the key exists and has a value.
func (c *HotCache[K, V]) HasMany(keys []K) map[K]bool {
	values, missing := c.cache.PeekMany(keys)

	output := make(map[K]bool, len(keys))
	for k, v := range values {
		output[k] = v.hasValue
	}
	for _, k := range missing {
		output[k] = false
	}

	return output
}

// Get returns a value from the cache, a boolean indicating whether the key was found,
// and an error when loaders fail. Uses the default loaders configured for the cache.
func (c *HotCache[K, V]) Get(key K) (value V, found bool, err error) {
	return c.GetWithLoaders(key, c.loaderFns...)
}

// MustGet returns a value from the cache and a boolean indicating whether the key was found.
// Panics when loaders fail. Uses the default loaders configured for the cache.
func (c *HotCache[K, V]) MustGet(key K) (value V, found bool) {
	value, found, err := c.Get(key)
	if err != nil {
		panic(err)
	}

	return value, found
}

// GetWithLoaders returns a value from the cache, a boolean indicating whether the key was found,
// and an error when loaders fail. Uses the provided loaders for cache misses.
// Concurrent calls for the same key are deduplicated using singleflight.
func (c *HotCache[K, V]) GetWithLoaders(key K, loaders ...Loader[K, V]) (value V, found bool, err error) {
	// The item might be found, but without value (missing key)
	cached, revalidate, found := c.getUnsafe(key)

	if found {
		if revalidate {
			go c.revalidate(map[K]*item[V]{key: cached}, loaders)
		}

		if cached.hasValue && c.copyOnRead != nil {
			return c.copyOnRead(cached.value), true, nil
		}

		return cached.value, cached.hasValue, nil
	}

	loaded, err := c.loadAndSetMany([]K{key}, loaders)
	if err != nil {
		return zero[V](), false, err
	}

	// `loaded` is expected to contain `key`, even if values was not available

	item, ok := loaded[key]
	if !ok || !item.hasValue {
		return zero[V](), false, nil
	}

	if c.copyOnRead != nil {
		return c.copyOnRead(item.value), true, nil
	}

	return item.value, true, nil
}

// MustGetWithLoaders returns a value from the cache and a boolean indicating whether the key was found.
// Panics when loaders fail. Uses the provided loaders for cache misses.
func (c *HotCache[K, V]) MustGetWithLoaders(key K, loaders ...Loader[K, V]) (value V, found bool) {
	value, found, err := c.GetWithLoaders(key, loaders...)
	if err != nil {
		panic(err)
	}

	return value, found
}

// GetMany returns multiple values from the cache, a slice of missing keys, and an error when loaders fail.
// Uses the default loaders configured for the cache.
func (c *HotCache[K, V]) GetMany(keys []K) (values map[K]V, missing []K, err error) {
	return c.GetManyWithLoaders(keys, c.loaderFns...)
}

// MustGetMany returns multiple values from the cache and a slice of missing keys.
// Panics when loaders fail. Uses the default loaders configured for the cache.
func (c *HotCache[K, V]) MustGetMany(keys []K) (values map[K]V, missing []K) {
	values, missing, err := c.GetMany(keys)
	if err != nil {
		panic(err)
	}

	return values, missing
}

// GetManyWithLoaders returns multiple values from the cache, a slice of missing keys, and an error when loaders fail.
// Uses the provided loaders for cache misses. Concurrent calls for the same keys are deduplicated using singleflight.
func (c *HotCache[K, V]) GetManyWithLoaders(keys []K, loaders ...Loader[K, V]) (values map[K]V, missing []K, err error) {
	// Some items might be found in cache, but without value (missing keys).
	// Other items will be returned in `missing`.
	cached, missing, revalidate := c.getManyUnsafe(keys)

	loaded, err := c.loadAndSetMany(missing, loaders)
	if err != nil {
		return nil, nil, err
	}

	if len(revalidate) > 0 {
		go c.revalidate(revalidate, loaders)
	}

	found, missing := itemMapsToValues(c.copyOnRead, cached, loaded)
	return found, missing, nil
}

// MustGetManyWithLoaders returns multiple values from the cache and a slice of missing keys.
// Panics when loaders fail. Uses the provided loaders for cache misses.
func (c *HotCache[K, V]) MustGetManyWithLoaders(keys []K, loaders ...Loader[K, V]) (values map[K]V, missing []K) {
	values, missing, err := c.GetManyWithLoaders(keys, loaders...)
	if err != nil {
		panic(err)
	}

	return values, missing
}

// Peek returns a value from the cache without checking expiration or calling loaders/revalidation.
// Missing values are not returned, even if cached. This is useful for inspection without side effects.
func (c *HotCache[K, V]) Peek(key K) (value V, ok bool) {
	// No need to check missingCache, since it will be missing anyway
	item, ok := c.cache.Peek(key)

	if ok && item.hasValue {
		if c.copyOnRead != nil {
			return c.copyOnRead(item.value), true
		}

		return item.value, true
	}

	return zero[V](), false
}

// PeekMany returns multiple values from the cache without checking expiration or calling loaders/revalidation.
// Missing values are not returned, even if cached. This is useful for inspection without side effects.
func (c *HotCache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
	cached := make(map[K]V)
	missing := []K{}

	// No need to check missingCache, since it will be missing anyway
	items, _ := c.cache.PeekMany(keys)

	for _, key := range keys {
		if item, ok := items[key]; ok && item.hasValue {
			if c.copyOnRead != nil {
				cached[key] = c.copyOnRead(item.value)
			} else {
				cached[key] = item.value
			}
		} else {
			missing = append(missing, key)
		}
	}

	return cached, missing
}

// Keys returns all keys in the cache that have valid values.
// Missing keys are not included in the result.
func (c *HotCache[K, V]) Keys() []K {
	output := []K{}

	c.cache.Range(func(k K, v *item[V]) bool {
		if v.hasValue { // Equivalent to testing `missingSharedCache`
			output = append(output, k)
		}
		return true
	})

	return output
}

// Values returns all values in the cache.
// Missing values are not included in the result.
func (c *HotCache[K, V]) Values() []V {
	values := c.cache.Values()

	output := []V{}

	for _, v := range values {
		if v.hasValue {
			if c.copyOnRead != nil {
				output = append(output, c.copyOnRead(v.value))
			} else {
				output = append(output, v.value)
			}
		}
	}

	return output
}

// All returns all key-value pairs in the cache.
func (c *HotCache[K, V]) All() map[K]V {
	nowNano := internal.NowNano()

	all := make(map[K]V)
	// we do not need to check missingCache, since it will be missing anyway
	c.cache.Range(func(k K, v *item[V]) bool {
		if v.hasValue && !v.isExpired(nowNano) {
			// we do not revalidate here, since it is too costly to revalidate all expired items at the same time

			if c.copyOnRead != nil {
				all[k] = c.copyOnRead(v.value)
			} else {
				all[k] = v.value
			}
		}
		return true
	})

	return all
}

// Range iterates over all key-value pairs in the cache and calls the provided function for each pair.
// The iteration stops if the function returns false. Missing values are not included.
// @TODO: loop over missingCache? Use a different callback?
func (c *HotCache[K, V]) Range(f func(K, V) bool) {
	nowNano := internal.NowNano()

	// we do not need to check missingCache, since it will be missing anyway
	c.cache.Range(func(k K, v *item[V]) bool {
		if v.hasValue && !v.isExpired(nowNano) {
			// we do not revalidate here, since it is too costly to revalidate all expired items at the same time
			if c.copyOnRead != nil {
				return f(k, c.copyOnRead(v.value))
			} else {
				return f(k, v.value)
			}
		}
		return true
	})
}

// Delete removes a key from the cache.
// Returns true if the key was found and removed, false otherwise.
func (c *HotCache[K, V]) Delete(key K) bool {
	return c.cache.Delete(key) || (c.missingCache != nil && c.missingCache.Delete(key))
}

// DeleteMany removes multiple keys from the cache in a single operation.
// Returns a map where keys are the input keys and values indicate whether the key was found and removed.
func (c *HotCache[K, V]) DeleteMany(keys []K) map[K]bool {
	// @TODO: should be done in a single call to avoid multiple locks
	a := c.cache.DeleteMany(keys)
	b := map[K]bool{}
	if c.missingCache != nil {
		b = c.missingCache.DeleteMany(keys)
	}

	output := map[K]bool{}
	for _, key := range keys {
		output[key] = a[key] || b[key]
	}

	return output
}

// Purge removes all keys and values from the cache.
// This operation clears both the main cache and the missing cache if enabled.
func (c *HotCache[K, V]) Purge() {
	c.cache.Purge()
	if c.missingCache != nil {
		// @TODO: should be done in a single call to avoid multiple locks
		c.missingCache.Purge()
	}
}

// Capacity returns the capacity of the main cache and missing cache.
// If missing cache is shared or not enabled, missingCacheCapacity will be 0.
func (c *HotCache[K, V]) Capacity() (mainCacheCapacity int, missingCacheCapacity int) {
	if c.missingCache != nil {
		// @TODO: should be done in a single call to avoid multiple locks
		return c.cache.Capacity(), c.missingCache.Capacity()
	}

	return c.cache.Capacity(), 0
}

// Algorithm returns the eviction algorithm names for the main cache and missing cache.
// If missing cache is shared or not enabled, missingCacheAlgorithm will be empty.
func (c *HotCache[K, V]) Algorithm() (mainCacheAlgorithm string, missingCacheAlgorithm string) {
	if c.missingCache != nil {
		// @TODO: should be done in a single call to avoid multiple locks
		return c.cache.Algorithm(), c.missingCache.Algorithm()
	}

	return c.cache.Algorithm(), ""
}

// Len returns the number of items in the main cache.
// This includes both valid values and missing keys if using shared missing cache.
func (c *HotCache[K, V]) Len() int {
	if c.missingCache != nil {
		// @TODO: should be done in a single call to avoid multiple locks
		return c.cache.Len() + c.missingCache.Len()
	}

	return c.cache.Len()
}

// WarmUp preloads the cache with data from the provided loader function.
// This is useful for initializing the cache with frequently accessed data.
// The loader function should return a map of key-value pairs and a slice of missing keys.
func (c *HotCache[K, V]) WarmUp(loader func() (map[K]V, []K, error)) error {
	if loader == nil {
		return nil
	}
	items, missing, err := loader()
	if err != nil {
		return err
	}

	if c.copyOnWrite != nil {
		for k, v := range items {
			items[k] = c.copyOnWrite(v)
		}
	}

	if c.missingCache == nil && !c.missingSharedCache && len(missing) > 0 {
		panic("missing cache is not enabled")
	}

	c.setManyUnsafe(items, missing, c.ttlNano)

	return nil
}

// Janitor starts a background goroutine that periodically removes expired items from the cache.
// The janitor runs until StopJanitor() is called or the cache is garbage collected.
// This method is safe to call multiple times, but only the first call will start the janitor.
func (c *HotCache[K, V]) Janitor() {
	// Acquire write lock to protect janitor state initialization
	// This prevents race conditions if multiple goroutines call Janitor() simultaneously
	c.janitorMutex.Lock()
	defer c.janitorMutex.Unlock()

	// Check if janitor is already running to prevent duplicate goroutines
	if c.ticker != nil {
		return
	}

	// Initialize janitor components atomically under lock protection
	c.ticker = time.NewTicker(time.Duration(c.ttlNano) * time.Nanosecond)
	c.stopOnce = &sync.Once{}
	c.stopJanitor = make(chan struct{})
	c.janitorDone = make(chan struct{})

	// Start the janitor goroutine
	go func() {
		// Ensure cleanup happens even if the goroutine panics
		// This is the key fix for the memory leak bug #10
		defer func() {
			// Acquire lock to safely update janitor state
			c.janitorMutex.Lock()
			c.ticker = nil // Allow garbage collection of ticker
			c.janitorMutex.Unlock()

			// Signal that janitor has finished cleanup
			// This allows StopJanitor() to proceed safely
			close(c.janitorDone)
		}()

		// Main janitor loop - runs until stop signal is received
		for {
			select {
			case <-c.stopJanitor:
				// Received stop signal, exit gracefully
				return

			case <-c.ticker.C:
				// Ticker fired, time to clean expired items
				nowNano := internal.NowNano()

				// Clean expired items from main cache
				{
					toDelete := []K{}
					toDeleteKV := map[K]V{}
					c.cache.Range(func(k K, v *item[V]) bool {
						if v.isExpired(nowNano) {
							toDelete = append(toDelete, k)
							if c.onEviction != nil {
								toDeleteKV[k] = v.value
							}
						}
						return true
					})

					deleted := c.cache.DeleteMany(toDelete)
					if c.onEviction != nil {
						for k, ok := range deleted {
							if ok {
								c.onEviction(base.EvictionReasonTTL, k, toDeleteKV[k])
							}
						}
					}
				}

				// Clean expired items from missing cache (if separate cache is used)
				if c.missingCache != nil {
					toDelete := []K{}
					toDeleteKV := map[K]V{}
					c.missingCache.Range(func(k K, v *item[V]) bool {
						if v.isExpired(nowNano) {
							toDelete = append(toDelete, k)
							if c.onEviction != nil {
								toDeleteKV[k] = v.value
							}
						}
						return true
					})

					deleted := c.missingCache.DeleteMany(toDelete)
					if c.onEviction != nil {
						for k, ok := range deleted {
							if ok {
								c.onEviction(base.EvictionReasonTTL, k, toDeleteKV[k])
							}
						}
					}
				}
			}
		}
	}()
}

// StopJanitor stops the background janitor goroutine and cleans up resources.
// This method is safe to call multiple times and will wait for the janitor to fully stop.
func (c *HotCache[K, V]) StopJanitor() {
	// Use read lock to check if janitor is running
	// This allows concurrent reads without blocking other operations
	c.janitorMutex.RLock()
	if c.ticker == nil {
		// Janitor is not running, nothing to stop
		c.janitorMutex.RUnlock()
		return
	}
	c.janitorMutex.RUnlock()

	// Use sync.Once to ensure shutdown logic runs only once
	// This prevents race conditions if multiple goroutines call StopJanitor()
	c.stopOnce.Do(func() {
		// Signal the janitor goroutine to stop by closing the channel
		// This will cause the select statement in the goroutine to receive from stopJanitor
		close(c.stopJanitor)

		// Wait for the janitor goroutine to finish its cleanup
		// This prevents memory leaks by ensuring the goroutine has exited before we stop the ticker
		<-c.janitorDone

		// Now it's safe to stop the ticker because the goroutine has finished
		// Use write lock to protect ticker access
		c.janitorMutex.Lock()
		if c.ticker != nil {
			c.ticker.Stop()
		}
		c.janitorMutex.Unlock()
	})
}

// setUnsafe is an internal method that sets a key-value pair in the cache without thread safety.
// It handles both regular values and missing keys, applying TTL jitter and managing separate caches.
func (c *HotCache[K, V]) setUnsafe(key K, hasValue bool, value V, ttlNano int64) {
	if !hasValue && c.missingCache == nil && !c.missingSharedCache {
		return
	}

	ttlNano = applyJitter(ttlNano, c.jitterLambda, c.jitterUpperBound)

	// Since we don't know where the previous key is stored, we need to delete preemptively
	if c.missingCache != nil {
		// @TODO: Should be done in a single call to avoid multiple locks
		if hasValue {
			c.missingCache.Delete(key)
		} else {
			c.cache.Delete(key)
		}
	}

	// @TODO: Should be done in a single call to avoid multiple locks
	if hasValue || c.missingSharedCache {
		c.cache.Set(key, newItem(value, hasValue, ttlNano, c.staleNano))
	} else if c.missingCache != nil {
		c.missingCache.Set(key, newItemNoValue[V](ttlNano, c.staleNano))
	}
}

// setManyUnsafe is an internal method that sets multiple key-value pairs in the cache without thread safety.
// It handles both regular values and missing keys, applying TTL jitter and managing separate caches.
func (c *HotCache[K, V]) setManyUnsafe(items map[K]V, missing []K, ttlNano int64) {
	if c.missingCache == nil && !c.missingSharedCache {
		missing = []K{}
	}

	if c.missingCache != nil {
		keysHavingValues := make([]K, 0, len(items))
		for k := range items {
			keysHavingValues = append(keysHavingValues, k)
		}

		// Since we don't know where the previous keys are stored, we need to delete all of them
		// @TODO: Should be done in a single call to avoid multiple locks
		c.cache.DeleteMany(missing)
		c.missingCache.DeleteMany(keysHavingValues)
	}

	values := map[K]*item[V]{}
	for k, v := range items {
		values[k] = newItemWithValue(v, ttlNano, c.staleNano)
	}

	if c.missingSharedCache {
		for _, k := range missing {
			values[k] = newItemNoValue[V](ttlNano, c.staleNano)
		}
	}

	c.cache.SetMany(values)

	if c.missingCache != nil {
		values = map[K]*item[V]{}
		for _, k := range missing {
			values[k] = newItemNoValue[V](ttlNano, c.staleNano)
		}
		c.missingCache.SetMany(values)
	}
}

// getUnsafe is an internal method that retrieves a value from the cache without thread safety.
// It returns the item, whether it needs revalidation, and whether it was found.
// Returns true if the key was found, even if it has no value (missing key).
func (c *HotCache[K, V]) getUnsafe(key K) (value *item[V], revalidate bool, found bool) {
	nowNano := internal.NowNano()

	// @TODO: Should be done in a single call to avoid multiple locks
	if item, ok := c.cache.Get(key); ok {
		if !item.isExpired(nowNano) {
			return item, item.shouldRevalidate(nowNano), true
		}

		ok := c.cache.Delete(key)
		if ok && c.onEviction != nil {
			c.onEviction(base.EvictionReasonTTL, key, item.value)
		}
	}

	if c.missingCache != nil {
		// @TODO: Should be done in a single call to avoid multiple locks
		if item, ok := c.missingCache.Get(key); ok {
			if !item.isExpired(nowNano) {
				return item, item.shouldRevalidate(nowNano), true
			}

			ok := c.missingCache.Delete(key)
			if ok && c.onEviction != nil {
				c.onEviction(base.EvictionReasonTTL, key, item.value)
			}
		}
	}

	return nil, false, false
}

// getManyUnsafe is an internal method that retrieves multiple values from the cache without thread safety.
// It returns cached items, missing keys, and items that need revalidation.
func (c *HotCache[K, V]) getManyUnsafe(keys []K) (cached map[K]*item[V], missing []K, revalidate map[K]*item[V]) {
	nowNano := internal.NowNano()

	cached = make(map[K]*item[V])
	revalidate = make(map[K]*item[V])

	toDeleteCache := []K{}
	toDeleteMissingCache := []K{}
	onEvictKV := map[K]V{}

	tmp, missing := c.cache.GetMany(keys)
	for k, v := range tmp {
		if !v.isExpired(nowNano) {
			cached[k] = v
			if v.shouldRevalidate(nowNano) {
				revalidate[k] = v
			}
			continue
		}

		toDeleteCache = append(toDeleteCache, k)
		if c.onEviction != nil {
			onEvictKV[k] = v.value
		}
	}

	if len(toDeleteCache) > 0 {
		// @TODO: Should be done in a single call to avoid multiple locks
		deleted := c.cache.DeleteMany(toDeleteCache)
		if c.onEviction != nil {
			for k, ok := range deleted {
				if ok {
					c.onEviction(base.EvictionReasonTTL, k, onEvictKV[k])
				}
			}
		}

		missing = append(missing, toDeleteCache...)
	}

	if len(missing) > 0 && c.missingCache != nil {
		tmp, missing = c.missingCache.GetMany(missing)
		for k, v := range tmp {
			if !v.isExpired(nowNano) {
				cached[k] = v
				if v.shouldRevalidate(nowNano) {
					revalidate[k] = v
				}
				continue
			}

			toDeleteMissingCache = append(toDeleteMissingCache, k)
			if c.onEviction != nil {
				onEvictKV[k] = v.value
			}
		}

		if len(toDeleteMissingCache) > 0 {
			// @TODO: Should be done in a single call to avoid multiple locks
			deleted := c.missingCache.DeleteMany(toDeleteMissingCache)
			if c.onEviction != nil {
				for k, ok := range deleted {
					if ok {
						c.onEviction(base.EvictionReasonTTL, k, onEvictKV[k])
					}
				}
			}

			missing = append(missing, toDeleteMissingCache...)
		}
	}

	return cached, missing, revalidate
}

// loadAndSetMany loads the keys using the provided loaders and sets them in the cache.
// It returns a map of keys to items and an error when loaders fail.
// All requested keys are returned, even if they have no value.
// Concurrent calls for the same keys are deduplicated using singleflight.
func (c *HotCache[K, V]) loadAndSetMany(keys []K, loaders LoaderChain[K, V]) (map[K]*item[V], error) {
	if len(keys) == 0 || len(loaders) == 0 {
		result := map[K]*item[V]{}
		for _, key := range keys {
			result[key] = newItemNoValue[V](0, 0)
		}
		return result, nil
	}

	// go-singleflightx is used to avoid calling the loaders multiple times for concurrent loads.
	// go-singleflightx returns all keys, so we don't need to keep track of missing keys.
	// Instead of looping over every loader, we should return the valid keys as soon as possible (and it will reduce errors).
	// @TODO: A custom implementation of go-singleflightx could be used to avoid some loops.
	results := c.group.DoX(keys, func(missing []K) (map[K]V, error) {
		results, stillMissing, err := loaders.run(missing)
		if err != nil {
			return nil, err
		}

		// Checking if results is not empty before calling setManyUnsafe ensures we don't call mutex for nothing.
		if c.copyOnWrite != nil && len(results) > 0 {
			for k, v := range results {
				results[k] = c.copyOnWrite(v)
			}
		}

		// We keep track of missing keys to avoid calling the loaders again.
		// Any values in `results` that were not requested in `keys` are cached.
		c.setManyUnsafe(results, stillMissing, c.ttlNano)

		return results, nil
	})

	// Format output
	output := map[K]*item[V]{}
	for _, key := range keys {
		if v, ok := results[key]; ok {
			if v.Err != nil {
				return map[K]*item[V]{}, v.Err
			}

			output[key] = newItem(v.Value.Value, v.Value.Valid, 0, 0)
		} else {
			// Not expected, since go-singleflightx should return all keys
			output[key] = newItemNoValue[V](0, 0)
		}
	}

	return output, nil
}

// revalidate revalidates stale items in the background using the provided fallback loaders.
// If revalidation loaders are configured, they are used instead of fallback loaders.
// If revalidation fails and the error policy is KeepOnError, the original items are preserved.
func (c *HotCache[K, V]) revalidate(items map[K]*item[V], fallbackLoaders LoaderChain[K, V]) {
	if len(items) == 0 {
		return
	}

	keys := []K{}
	for k := range items {
		keys = append(keys, k)
	}

	loaders := fallbackLoaders
	if len(c.revalidationLoaderFns) > 0 {
		loaders = c.revalidationLoaderFns
	}

	// @TODO: We might be fetching keys one by one, which is not efficient.
	// We should batch the keys and fetch them after a short delay.
	_, err := c.loadAndSetMany(keys, loaders)
	if err != nil && c.revalidationErrorPolicy == KeepOnError {
		valid := map[K]V{}
		missing := []K{}

		for k, v := range items {
			if v.hasValue {
				valid[k] = v.value
			} else {
				missing = append(missing, k)
			}
		}

		c.setManyUnsafe(valid, missing, c.ttlNano)
	}
}

// Describe implements the prometheus.Collector interface.
func (c *HotCache[K, V]) Describe(ch chan<- *prometheus.Desc) {
	for _, collector := range c.prometheusCollectors {
		if prometheusCollector, ok := collector.(prometheus.Collector); ok {
			prometheusCollector.Describe(ch)
		}
	}
}

// Collect implements the prometheus.Collector interface.
func (c *HotCache[K, V]) Collect(ch chan<- prometheus.Metric) {
	// Triggers a size calculation.
	// Warning: This is very slow.
	c.cache.SizeBytes()
	c.cache.Len()
	if c.missingCache != nil {
		c.missingCache.SizeBytes()
		c.missingCache.Len()
	}

	for _, collector := range c.prometheusCollectors {
		if prometheusCollector, ok := collector.(prometheus.Collector); ok {
			prometheusCollector.Collect(ch)
		}
	}
}
