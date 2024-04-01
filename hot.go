package hot

import (
	"sync"
	"time"

	"github.com/samber/go-singleflightx"
	"github.com/samber/hot/base"
	"github.com/samber/hot/internal"
)

// Revalidation is done in batch,.
var DebounceRevalidationFactor = 0.2

func newHotCache[K comparable, V any](
	locking bool,

	cache base.InMemoryCache[K, *item[V]],
	missingSharedCache bool,
	missingCache base.InMemoryCache[K, *item[V]],

	ttl time.Duration,
	stale time.Duration,
	jitter float64,

	loaderFns LoaderChain[K, V],
	revalidationLoaderFns LoaderChain[K, V],
	copyOnRead func(V) V,
	copyOnWrite func(V) V,
) *HotCache[K, V] {
	// Using a mutexMock cost ~3ns per operation. Which is more than the cost of calling base.SafeCache abstraction (1ns).
	// Using mutexMock is more performant for this lib when locking is enabled most of time.
	var mu rwMutex = &mutexMock{}
	if locking {
		mu = &sync.RWMutex{}
	}

	return &HotCache[K, V]{
		mu: mu, // @TODO: separate for cache and cacheMissing ?

		cache:              cache,
		missingSharedCache: missingSharedCache,
		missingCache:       missingCache,

		// Better store int64 microseconds instead of time.Time (benchmark resulted in 10x speedup).
		ttlMicro:   ttl.Microseconds(),
		staleMicro: stale.Microseconds(),
		jitter:     jitter,

		loaderFns:             loaderFns,
		revalidationLoaderFns: revalidationLoaderFns,
		copyOnRead:            copyOnRead,
		copyOnWrite:           copyOnWrite,

		group:   singleflightx.Group[K, V]{},
		metrics: NewMetrics(ttl, jitter, stale),
	}
}

type HotCache[K comparable, V any] struct {
	mu     rwMutex
	ticker *time.Ticker

	cache              base.InMemoryCache[K, *item[V]]
	missingSharedCache bool
	missingCache       base.InMemoryCache[K, *item[V]]

	// Better store int64 microseconds instead of time.Time (benchmark resulted in 10x speedup).
	ttlMicro   int64
	staleMicro int64
	jitter     float64

	loaderFns             LoaderChain[K, V]
	revalidationLoaderFns LoaderChain[K, V]
	copyOnRead            func(V) V
	copyOnWrite           func(V) V

	group   singleflightx.Group[K, V]
	metrics *Metrics
}

// Set adds a value to the cache. If the key already exists, its value is updated. It uses the default ttl or none.
func (c *HotCache[K, V]) Set(key K, v V) {
	if c.copyOnWrite != nil {
		v = c.copyOnWrite(v)
	}

	c.mu.Lock()
	c.setUnsafe(key, true, v, c.ttlMicro)
	c.mu.Unlock()
}

// SetMissing adds a key to the `missing` cache. If the key already exists, its value is dropped. It uses the default ttl or none.
func (c *HotCache[K, V]) SetMissing(key K) {
	if c.missingCache == nil && !c.missingSharedCache {
		panic("missing cache is not enabled")
	}

	c.mu.Lock()
	c.setUnsafe(key, false, zero[V](), c.ttlMicro)
	c.mu.Unlock()
}

// SetWithTTL adds a value to the cache. If the key already exists, its value is updated. It uses the given ttl.
func (c *HotCache[K, V]) SetWithTTL(key K, v V, ttl time.Duration) {
	if c.copyOnWrite != nil {
		v = c.copyOnWrite(v)
	}

	c.mu.Lock()
	c.setUnsafe(key, true, v, ttl.Microseconds())
	c.mu.Unlock()
}

// SetMissingWithTTL adds a key to the `missing` cache. If the key already exists, its value is dropped. It uses the given ttl.
func (c *HotCache[K, V]) SetMissingWithTTL(key K, ttl time.Duration) {
	if c.missingCache == nil && !c.missingSharedCache {
		panic("missing cache is not enabled")
	}

	c.mu.Lock()
	c.setUnsafe(key, false, zero[V](), ttl.Microseconds())
	c.mu.Unlock()
}

// SetMany adds many values to the cache. If the keys already exist, values are updated. It uses the default ttl or none.
func (c *HotCache[K, V]) SetMany(items map[K]V) {
	if c.copyOnWrite != nil {
		for k, v := range items {
			items[k] = c.copyOnWrite(v)
		}
	}

	c.mu.Lock()
	c.setManyUnsafe(items, []K{}, c.ttlMicro)
	c.mu.Unlock()
}

// SetMissingMany adds many keys to the cache. If the keys already exist, values are dropped. It uses the default ttl or none.
func (c *HotCache[K, V]) SetMissingMany(missingKeys []K) {
	if c.missingCache == nil && !c.missingSharedCache {
		panic("missing cache is not enabled")
	}

	c.mu.Lock()
	c.setManyUnsafe(map[K]V{}, missingKeys, c.ttlMicro)
	c.mu.Unlock()
}

// SetManyWithTTL adds many values to the cache. If the keys already exist, values are updated. It uses the given ttl.
func (c *HotCache[K, V]) SetManyWithTTL(items map[K]V, ttl time.Duration) {
	if c.copyOnWrite != nil {
		for k, v := range items {
			items[k] = c.copyOnWrite(v)
		}
	}

	c.mu.Lock()
	c.setManyUnsafe(items, []K{}, ttl.Microseconds())
	c.mu.Unlock()
}

// SetManyWithTTL adds many keys to the cache. If the keys already exist, values are dropped. It uses the given ttl.
func (c *HotCache[K, V]) SetMissingManyWithTTL(missingKeys []K, ttl time.Duration) {
	if c.missingCache == nil && !c.missingSharedCache {
		panic("missing cache is not enabled")
	}

	c.mu.Lock()
	c.setManyUnsafe(map[K]V{}, missingKeys, ttl.Microseconds())
	c.mu.Unlock()
}

// Has checks if a key exists in the cache.
// Missing values are not valid, even if cached.
func (c *HotCache[K, V]) Has(key K) bool {
	c.mu.RLock()
	v, ok := c.cache.Peek(key)
	c.mu.RUnlock()

	return ok && v.hasValue
}

// HasMany checks if keys exist in the cache.
// Missing values are not valid, even if cached.
func (c *HotCache[K, V]) HasMany(keys []K) map[K]bool {
	c.mu.RLock()
	values, missing := c.cache.PeekMany(keys)
	c.mu.RUnlock()

	output := make(map[K]bool, len(keys))
	for k, v := range values {
		output[k] = v.hasValue
	}
	for _, k := range missing {
		output[k] = false
	}

	return output
}

// Get returns a value from the cache, a boolean indicating whether the key was found and an error when loaders fail.
func (c *HotCache[K, V]) Get(key K) (value V, ok bool, err error) {
	return c.GetWithCustomLoaders(key, c.loaderFns)
}

// GetWithCustomLoaders returns a value from the cache, a boolean indicating whether the key was found and an error when loaders fail.
func (c *HotCache[K, V]) GetWithCustomLoaders(key K, customLoaders LoaderChain[K, V]) (value V, ok bool, err error) {
	c.mu.RLock()
	// the item might be found, but without value
	cached, revalidate, found := c.getUnsafe(key)
	c.mu.RUnlock()

	if found {
		if revalidate {
			go c.revalidate([]K{key})
		}

		if cached.hasValue && c.copyOnRead != nil {
			return c.copyOnRead(cached.value), true, nil
		}

		return cached.value, cached.hasValue, nil
	}

	loaded, err := c.loadAndSetMany([]K{key}, c.loaderFns)
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

// GetMany returns many values from the cache, a slice of missing keys and an error when loaders fail.
func (c *HotCache[K, V]) GetMany(keys []K) (map[K]V, []K, error) {
	return c.GetManyWithCustomLoaders(keys, c.loaderFns)
}

// GetManyWithCustomLoaders returns many values from the cache, a slice of missing keys and an error when loaders fail.
func (c *HotCache[K, V]) GetManyWithCustomLoaders(keys []K, customLoaders LoaderChain[K, V]) (map[K]V, []K, error) {
	c.mu.RLock()
	// Some items might be found in cached, but without value.
	// Other items will be returned in `missing`.
	cached, missing, revalidate := c.getManyUnsafe(keys)
	c.mu.RUnlock()

	loaded, err := c.loadAndSetMany(missing, c.loaderFns)
	if err != nil {
		return nil, nil, err
	}

	if len(revalidate) > 0 {
		go c.revalidate(revalidate)
	}

	found, missing := itemMapsToValues(c.copyOnRead, cached, loaded)
	return found, missing, nil
}

// Peek is similar to Get, but do not check expiration and do not call loaders/revalidation.
// Missing values are not returned, even if cached.
func (c *HotCache[K, V]) Peek(key K) (value V, ok bool) {
	c.mu.RLock()
	// no need to check missingCache, since it will be missing anyway
	item, ok := c.cache.Peek(key)
	c.mu.RUnlock()

	if ok && item.hasValue {
		if c.copyOnRead != nil {
			return c.copyOnRead(item.value), true
		}

		return item.value, true
	}

	return zero[V](), false
}

// PeekMany is similar to GetMany, but do not check expiration and do not call loaders/revalidation.
// Missing values are not returned, even if cached.
func (c *HotCache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
	cached := make(map[K]V)
	missing := []K{}

	c.mu.RLock()
	// no need to check missingCache, since it will be missing anyway
	items, _ := c.cache.PeekMany(keys)
	c.mu.RUnlock()

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

// Keys returns all keys in the cache.
// Missing keys are not included.
func (c *HotCache[K, V]) Keys() []K {
	c.mu.RLock()
	defer c.mu.RUnlock()

	output := []K{}

	c.cache.Range(func(k K, v *item[V]) bool {
		if v.hasValue { // equalivant to testing `missingSharedCache`
			output = append(output, k)
		}
		return true
	})

	return output
}

// Values returns all values in the cache.
// Missing values are not included.
func (c *HotCache[K, V]) Values() []V {
	c.mu.RLock()
	values := c.cache.Values()
	c.mu.RUnlock()

	return itemSlicesToValues(c.copyOnRead, values)
}

// Range calls a function for each key/value pair in the cache.
// The callback should be kept short because it is called while holding a read lock.
// @TODO: loop over missingCache? Use a different callback?
// Missing values are not included.
func (c *HotCache[K, V]) Range(f func(K, V) bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.cache.Range(func(k K, v *item[V]) bool {
		if !v.hasValue { // equalivant to testing `missingSharedCache`
			return true
		}
		if c.copyOnRead != nil {
			return f(k, v.value)
		}
		return f(k, v.value)
	})
}

// Delete removes a key from the cache.
func (c *HotCache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.cache.Delete(key) || (c.missingCache != nil && c.missingCache.Delete(key))
}

// DeleteMany removes many keys from the cache.
func (c *HotCache[K, V]) DeleteMany(keys []K) map[K]bool {
	c.mu.Lock()

	a := c.cache.DeleteMany(keys)
	b := map[K]bool{}
	if c.missingCache != nil {
		b = c.missingCache.DeleteMany(keys)
	}

	c.mu.Unlock()

	output := map[K]bool{}
	for _, key := range keys {
		output[key] = a[key] || b[key]
	}

	return output
}

// Purge removes all items from the cache.
func (c *HotCache[K, V]) Purge() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache.Purge()
	if c.missingCache != nil {
		c.missingCache.Purge()
	}
}

// Capacity returns the cache capacity.
func (c *HotCache[K, V]) Capacity() (int, int) {
	if c.missingCache != nil {
		return c.cache.Capacity(), c.missingCache.Capacity()
	}

	return c.cache.Capacity(), 0
}

// Algorithm returns the cache algo.
func (c *HotCache[K, V]) Algorithm() (string, string) {
	if c.missingCache != nil {
		return c.cache.Algorithm(), c.missingCache.Algorithm()
	}

	return c.cache.Algorithm(), ""
}

// Len returns the number of items in the cache.
// Missing items are included.
func (c *HotCache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.missingCache != nil {
		return c.cache.Len() + c.missingCache.Len()
	}

	return c.cache.Len()
}

// WarmUp loads all keys from the loader and sets them in the cache.
func (c *HotCache[K, V]) WarmUp(loader func() (map[K]V, []K, error)) error {
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

	c.mu.Lock()
	c.setManyUnsafe(items, missing, c.ttlMicro)
	c.mu.Unlock()

	return nil
}

// Janitor runs a background goroutine to clean up the cache.
func (c *HotCache[K, V]) Janitor() {
	c.mu.Lock()
	c.ticker = time.NewTicker(time.Duration(c.ttlMicro) * time.Microsecond)
	c.mu.Unlock()

	go func() {
		for range c.ticker.C {
			nowMicro := internal.NowMicro()

			c.mu.Lock()

			c.cache.Range(func(k K, v *item[V]) bool {
				if v.isExpired(nowMicro) {
					c.cache.Delete(k)
				}
				return true
			})

			if c.missingCache != nil {
				c.missingCache.Range(func(k K, v *item[V]) bool {
					if v.isExpired(nowMicro) {
						c.missingCache.Delete(k)
					}
					return true
				})
			}

			c.mu.Unlock()
		}
	}()
}

func (c *HotCache[K, V]) StopJanitor() {
	c.mu.Lock()
	c.ticker.Stop()
	c.mu.Unlock()
}

func (c *HotCache[K, V]) setUnsafe(key K, hasValue bool, value V, ttlMicro int64) {
	if !hasValue && c.missingCache == nil && !c.missingSharedCache {
		return
	}

	ttlMicro = applyJitter(ttlMicro, c.jitter)

	if c.missingCache != nil {
		// since we don't know where is the previous key, we need to delete all of them
		c.cache.Delete(key)
		c.missingCache.Delete(key)
	}

	if hasValue || c.missingSharedCache {
		c.cache.Set(key, newItem(value, hasValue, ttlMicro, c.staleMicro))
	} else if c.missingCache != nil {
		c.missingCache.Set(key, newItemNoValue[V](ttlMicro, c.staleMicro))
	}
}

func (c *HotCache[K, V]) setManyUnsafe(items map[K]V, missing []K, ttlMicro int64) {
	for k, v := range items {
		c.setUnsafe(k, true, v, ttlMicro)
	}

	if c.missingCache != nil || c.missingSharedCache {
		z := zero[V]()
		for _, k := range missing {
			c.setUnsafe(k, false, z, ttlMicro)
		}
	}
}

// getUnsafe returns true if the key was fonud, even if it has no value.
func (c *HotCache[K, V]) getUnsafe(key K) (value *item[V], revalidate bool, found bool) {
	nowMicro := internal.NowMicro()

	if item, ok := c.cache.Get(key); ok {
		if !item.isExpired(nowMicro) {
			return item, item.shouldRevalidate(nowMicro), true
		}

		c.cache.Delete(key)
	}

	if c.missingCache != nil {
		if item, ok := c.missingCache.Get(key); ok {
			if !item.isExpired(nowMicro) {
				return item, item.shouldRevalidate(nowMicro), true
			}

			c.missingCache.Delete(key)
		}
	}

	return nil, false, false
}

func (c *HotCache[K, V]) getManyUnsafe(keys []K) (cached map[K]*item[V], missing []K, revalidate []K) {
	nowMicro := internal.NowMicro()

	cached = make(map[K]*item[V])
	missing = []K{}
	revalidate = []K{}

	for _, key := range keys {
		if item, ok := c.cache.Get(key); ok {
			if !item.isExpired(nowMicro) {
				cached[key] = item
				if item.shouldRevalidate(nowMicro) {
					revalidate = append(revalidate, key)
				}
				continue
			}

			c.cache.Delete(key)
		}

		if c.missingCache != nil {
			if item, ok := c.missingCache.Get(key); ok {
				if !item.isExpired(nowMicro) {
					cached[key] = item
					if item.shouldRevalidate(nowMicro) {
						revalidate = append(revalidate, key)
					}
					continue
				}

				c.missingCache.Delete(key)
			}
		}

		missing = append(missing, key)
	}

	return cached, missing, revalidate
}

// loadAndSetMany loads the keys and sets them in the cache.
// It returns a map of keys to items and an error when loaders fail.
// All requested keys are returned, even if they have no value.
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
	// Instead of looping over every loaders, we should return the valid keys as soon as possible (and it will reduce errors).
	results := c.group.DoX(keys, func(missing []K) (map[K]V, error) {
		results, stillMissing, err := loaders.run(missing)
		if err != nil {
			return nil, err
		}

		// Checking if a results is not empty before call to setManyUnsafe ensure we don't call mutex for nothing.
		if len(results) > 0 {
			if c.copyOnWrite != nil {
				for k, v := range results {
					results[k] = c.copyOnWrite(v)
				}
			}

			c.mu.Lock()
			// any values in `results` that were not requested in `keys` are cached
			c.setManyUnsafe(results, []K{}, c.ttlMicro)
			c.mu.Unlock()
		}

		// We keep track of missing keys to avoid calling the loaders again.
		// Checking if a missing cache exist before call to setManyUnsafe ensure we don't call mutex for nothing.
		if len(stillMissing) > 0 && (c.missingCache != nil || c.missingSharedCache) {
			c.mu.Lock()
			c.setManyUnsafe(map[K]V{}, stillMissing, c.ttlMicro)
			c.mu.Unlock()
		}

		return results, nil
	})

	// format output
	output := map[K]*item[V]{}
	for _, key := range keys {
		if v, ok := results[key]; ok {
			if v.Err != nil {
				return map[K]*item[V]{}, v.Err
			}

			output[key] = newItem(v.Value.Value, v.Value.Valid, 0, 0)
		} else {
			// not expected, since go-singleflightx should return all keys
			output[key] = newItemNoValue[V](0, 0)
		}
	}

	return output, nil
}

func (c *HotCache[K, V]) revalidate(keys []K) {
	if len(keys) == 0 {
		return
	}

	loaders := c.loaderFns
	if len(c.revalidationLoaderFns) > 0 {
		loaders = c.revalidationLoaderFns
	}

	// @TODO: we might be fetching keys one by one, which is not efficient.
	// We should batch the keys and fetch them after a short delay.
	_, _ = c.loadAndSetMany(keys, loaders)
}
