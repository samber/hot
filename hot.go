package hot

import (
	"sync"
	"time"

	"github.com/samber/go-singleflightx"
	"github.com/samber/hot/internal"
	"github.com/samber/hot/pkg/base"
)

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
) *HotCache[K, V] {
	return &HotCache[K, V]{
		cache:              cache,
		missingSharedCache: missingSharedCache,
		missingCache:       missingCache,

		// Store int64 microseconds instead of time.Time for better performance
		// (benchmark resulted in 10x speedup)
		ttlMicro:         ttl.Microseconds(),
		staleMicro:       stale.Microseconds(),
		jitterLambda:     jitterLambda,
		jitterUpperBound: jitterUpperBound,

		loaderFns:               loaderFns,
		revalidationLoaderFns:   revalidationLoaderFns,
		revalidationErrorPolicy: revalidationErrorPolicy,
		onEviction:              onEviction,
		copyOnRead:              copyOnRead,
		copyOnWrite:             copyOnWrite,

		group: singleflightx.Group[K, V]{},
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

	// Store int64 microseconds instead of time.Time for better performance
	// (benchmark resulted in 10x speedup)
	ttlMicro         int64
	staleMicro       int64
	jitterLambda     float64
	jitterUpperBound time.Duration

	loaderFns               LoaderChain[K, V]
	revalidationLoaderFns   LoaderChain[K, V]
	revalidationErrorPolicy revalidationErrorPolicy
	onEviction              base.EvictionCallback[K, V]
	copyOnRead              func(V) V
	copyOnWrite             func(V) V

	group singleflightx.Group[K, V]
}

// Set adds a value to the cache. If the key already exists, its value is updated.
// Uses the default TTL configured for the cache.
func (c *HotCache[K, V]) Set(key K, v V) {
	if c.copyOnWrite != nil {
		v = c.copyOnWrite(v)
	}

	c.setUnsafe(key, true, v, c.ttlMicro)
}

// SetMissing adds a key to the missing cache to prevent repeated lookups for non-existent keys.
// If the key already exists, its value is dropped. Uses the default TTL configured for the cache.
// Panics if missing cache is not enabled.
func (c *HotCache[K, V]) SetMissing(key K) {
	if c.missingCache == nil && !c.missingSharedCache {
		panic("missing cache is not enabled")
	}

	c.setUnsafe(key, false, zero[V](), c.ttlMicro)
}

// SetWithTTL adds a value to the cache with a specific TTL duration.
// If the key already exists, its value is updated.
func (c *HotCache[K, V]) SetWithTTL(key K, v V, ttl time.Duration) {
	if c.copyOnWrite != nil {
		v = c.copyOnWrite(v)
	}

	c.setUnsafe(key, true, v, ttl.Microseconds())
}

// SetMissingWithTTL adds a key to the missing cache with a specific TTL duration.
// If the key already exists, its value is dropped.
// Panics if missing cache is not enabled.
func (c *HotCache[K, V]) SetMissingWithTTL(key K, ttl time.Duration) {
	if c.missingCache == nil && !c.missingSharedCache {
		panic("missing cache is not enabled")
	}

	c.setUnsafe(key, false, zero[V](), ttl.Microseconds())
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

	c.setManyUnsafe(items, []K{}, c.ttlMicro)
}

// SetMissingMany adds multiple keys to the missing cache in a single operation.
// If keys already exist, their values are dropped. Uses the default TTL configured for the cache.
// Panics if missing cache is not enabled.
func (c *HotCache[K, V]) SetMissingMany(missingKeys []K) {
	if c.missingCache == nil && !c.missingSharedCache {
		panic("missing cache is not enabled")
	}

	c.setManyUnsafe(map[K]V{}, missingKeys, c.ttlMicro)
}

// SetManyWithTTL adds multiple values to the cache with a specific TTL duration.
// If keys already exist, their values are updated.
func (c *HotCache[K, V]) SetManyWithTTL(items map[K]V, ttl time.Duration) {
	if c.copyOnWrite != nil {
		for k, v := range items {
			items[k] = c.copyOnWrite(v)
		}
	}

	c.setManyUnsafe(items, []K{}, ttl.Microseconds())
}

// SetMissingManyWithTTL adds multiple keys to the missing cache with a specific TTL duration.
// If keys already exist, their values are dropped.
// Panics if missing cache is not enabled.
func (c *HotCache[K, V]) SetMissingManyWithTTL(missingKeys []K, ttl time.Duration) {
	if c.missingCache == nil && !c.missingSharedCache {
		panic("missing cache is not enabled")
	}

	c.setManyUnsafe(map[K]V{}, missingKeys, ttl.Microseconds())
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

// Range iterates over all key-value pairs in the cache and calls the provided function for each pair.
// The iteration stops if the function returns false. Missing values are not included.
// @TODO: loop over missingCache? Use a different callback?
func (c *HotCache[K, V]) Range(f func(K, V) bool) {
	c.cache.Range(func(k K, v *item[V]) bool {
		if !v.hasValue { // equalivant to testing `missingSharedCache`
			return true
		}
		if c.copyOnRead != nil {
			return f(k, c.copyOnRead(v.value))
		}
		return f(k, v.value)
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

	c.setManyUnsafe(items, missing, c.ttlMicro)

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
	c.ticker = time.NewTicker(time.Duration(c.ttlMicro) * time.Microsecond)
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
				nowMicro := internal.NowMicro()

				// Clean expired items from main cache
				{
					toDelete := []K{}
					toDeleteKV := map[K]V{}
					c.cache.Range(func(k K, v *item[V]) bool {
						if v.isExpired(nowMicro) {
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
								c.onEviction(k, toDeleteKV[k])
							}
						}
					}
				}

				// Clean expired items from missing cache (if separate cache is used)
				if c.missingCache != nil {
					toDelete := []K{}
					toDeleteKV := map[K]V{}
					c.missingCache.Range(func(k K, v *item[V]) bool {
						if v.isExpired(nowMicro) {
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
								c.onEviction(k, toDeleteKV[k])
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
func (c *HotCache[K, V]) setUnsafe(key K, hasValue bool, value V, ttlMicro int64) {
	if !hasValue && c.missingCache == nil && !c.missingSharedCache {
		return
	}

	ttlMicro = applyJitter(ttlMicro, c.jitterLambda, c.jitterUpperBound)

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
		c.cache.Set(key, newItem(value, hasValue, ttlMicro, c.staleMicro))
	} else if c.missingCache != nil {
		c.missingCache.Set(key, newItemNoValue[V](ttlMicro, c.staleMicro))
	}
}

// setManyUnsafe is an internal method that sets multiple key-value pairs in the cache without thread safety.
// It handles both regular values and missing keys, applying TTL jitter and managing separate caches.
func (c *HotCache[K, V]) setManyUnsafe(items map[K]V, missing []K, ttlMicro int64) {
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
		values[k] = newItemWithValue(v, ttlMicro, c.staleMicro)
	}

	if c.missingSharedCache {
		for _, k := range missing {
			values[k] = newItemNoValue[V](ttlMicro, c.staleMicro)
		}
	}

	c.cache.SetMany(values)

	if c.missingCache != nil {
		values = map[K]*item[V]{}
		for _, k := range missing {
			values[k] = newItemNoValue[V](ttlMicro, c.staleMicro)
		}
		c.missingCache.SetMany(values)
	}
}

// getUnsafe is an internal method that retrieves a value from the cache without thread safety.
// It returns the item, whether it needs revalidation, and whether it was found.
// Returns true if the key was found, even if it has no value (missing key).
func (c *HotCache[K, V]) getUnsafe(key K) (value *item[V], revalidate bool, found bool) {
	nowMicro := internal.NowMicro()

	// @TODO: Should be done in a single call to avoid multiple locks
	if item, ok := c.cache.Get(key); ok {
		if !item.isExpired(nowMicro) {
			return item, item.shouldRevalidate(nowMicro), true
		}

		ok := c.cache.Delete(key)
		if ok && c.onEviction != nil {
			c.onEviction(key, item.value)
		}
	}

	if c.missingCache != nil {
		// @TODO: Should be done in a single call to avoid multiple locks
		if item, ok := c.missingCache.Get(key); ok {
			if !item.isExpired(nowMicro) {
				return item, item.shouldRevalidate(nowMicro), true
			}

			ok := c.missingCache.Delete(key)
			if ok && c.onEviction != nil {
				c.onEviction(key, item.value)
			}
		}
	}

	return nil, false, false
}

// getManyUnsafe is an internal method that retrieves multiple values from the cache without thread safety.
// It returns cached items, missing keys, and items that need revalidation.
func (c *HotCache[K, V]) getManyUnsafe(keys []K) (cached map[K]*item[V], missing []K, revalidate map[K]*item[V]) {
	nowMicro := internal.NowMicro()

	cached = make(map[K]*item[V])
	revalidate = make(map[K]*item[V])

	toDeleteCache := []K{}
	toDeleteMissingCache := []K{}
	onEvictKV := map[K]V{}

	tmp, missing := c.cache.GetMany(keys)
	for k, v := range tmp {
		if !v.isExpired(nowMicro) {
			cached[k] = v
			if v.shouldRevalidate(nowMicro) {
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
					c.onEviction(k, onEvictKV[k])
				}
			}
		}

		missing = append(missing, toDeleteCache...)
	}

	if len(missing) > 0 && c.missingCache != nil {
		tmp, missing = c.missingCache.GetMany(missing)
		for k, v := range tmp {
			if !v.isExpired(nowMicro) {
				cached[k] = v
				if v.shouldRevalidate(nowMicro) {
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
						c.onEviction(k, onEvictKV[k])
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
		c.setManyUnsafe(results, stillMissing, c.ttlMicro)

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

		c.setManyUnsafe(valid, missing, c.ttlMicro)
	}
}

// // CollectMetrics collects Prometheus metrics from the cache.
// // This method calculates the current size of all cached items and keys,
// // and collects all metrics including counters and configuration settings.
// func (c *HotCache[K, V]) CollectMetrics(ch chan<- prometheus.Metric) {
// 	// Calculate total size in bytes (keys + values)
// 	var totalSize int64

// 	// Calculate size from main cache
// 	c.cache.Range(func(key K, item *item[V]) bool {
// 		// Estimate key size (assuming string representation)
// 		keySize := len(fmt.Sprintf("%v", key))

// 		// Estimate value size
// 		var valueSize int
// 		if item.hasValue {
// 			// Use reflection to estimate size of the value
// 			valueSize = estimateSize(item.value)
// 		}

// 		// Add item overhead (timestamp, flags, etc.)
// 		itemOverhead := 24 // Rough estimate for item struct overhead

// 		totalSize += int64(keySize + valueSize + itemOverhead)
// 		return true
// 	})

// 	// Calculate size from missing cache if it exists
// 	if c.missingCache != nil {
// 		c.missingCache.Range(func(key K, item *item[V]) bool {
// 			keySize := len(fmt.Sprintf("%v", key))
// 			itemOverhead := 24
// 			totalSize += int64(keySize + itemOverhead)
// 			return true
// 		})
// 	}

// 	// Update size metric
// 	c.metrics.SetSizeBytes(totalSize)

// 	// Collect all metrics
// 	c.metrics.Collect(ch)

// 	// If this is a sharded cache, collect metrics from all shards
// 	if shardedCache, ok := c.cache.(interface {
// 		CollectMetrics(ch chan<- prometheus.Metric)
// 	}); ok {
// 		shardedCache.CollectMetrics(ch)
// 	}
// }

// // estimateSize provides a rough estimate of the size of a value in bytes.
// // This is a simple implementation that can be improved for better accuracy.
// func estimateSize(v any) int {
// 	switch val := v.(type) {
// 	case string:
// 		return len(val)
// 	case []byte:
// 		return len(val)
// 	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
// 		return 8
// 	case float32, float64:
// 		return 8
// 	case bool:
// 		return 1
// 	default:
// 		// For complex types, use a rough estimate based on string representation
// 		return len(fmt.Sprintf("%v", val))
// 	}
// }

// // RegisterWithPrometheus registers the cache metrics with the default Prometheus registry.
// // This allows the metrics to be collected when Prometheus scrapes the metrics endpoint.
// // Only works when metrics are enabled for the cache.
// func (c *HotCache[K, V]) RegisterWithPrometheus() {
// 	if collector, ok := c.metrics.(prometheus.Collector); ok {
// 		prometheus.MustRegister(collector)
// 	}
// }

// // UnregisterFromPrometheus unregisters the cache metrics from the default Prometheus registry.
// // This should be called when the cache is no longer needed to prevent memory leaks.
// func (c *HotCache[K, V]) UnregisterFromPrometheus() {
// 	if collector, ok := c.metrics.(prometheus.Collector); ok {
// 		prometheus.Unregister(collector)
// 	}
// }
