package hot

import (
	"time"

	"github.com/samber/go-singleflightx"
	"github.com/samber/hot/internal"
	"github.com/samber/hot/pkg/base"
	"github.com/samber/hot/pkg/metrics"
)

// Revalidation is done in batch,.
var DebounceRevalidationFactor = 0.2

func newHotCache[K comparable, V any](
	cache base.InMemoryCache[K, *item[V]],
	missingSharedCache bool,
	missingCache base.InMemoryCache[K, *item[V]],

	ttl time.Duration,
	stale time.Duration,
	jitter float64,

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

		// Better store int64 microseconds instead of time.Time (benchmark resulted in 10x speedup).
		ttlMicro:   ttl.Microseconds(),
		staleMicro: stale.Microseconds(),
		jitter:     jitter,

		loaderFns:               loaderFns,
		revalidationLoaderFns:   revalidationLoaderFns,
		revalidationErrorPolicy: revalidationErrorPolicy,
		onEviction:              onEviction,
		copyOnRead:              copyOnRead,
		copyOnWrite:             copyOnWrite,

		group:   singleflightx.Group[K, V]{},
		metrics: metrics.NewMetrics(ttl, jitter, stale),
	}
}

type HotCache[K comparable, V any] struct {
	ticker *time.Ticker

	cache              base.InMemoryCache[K, *item[V]]
	missingSharedCache bool
	missingCache       base.InMemoryCache[K, *item[V]]

	// Better store int64 microseconds instead of time.Time (benchmark resulted in 10x speedup).
	ttlMicro   int64
	staleMicro int64
	jitter     float64

	loaderFns               LoaderChain[K, V]
	revalidationLoaderFns   LoaderChain[K, V]
	revalidationErrorPolicy revalidationErrorPolicy
	onEviction              base.EvictionCallback[K, V]
	copyOnRead              func(V) V
	copyOnWrite             func(V) V

	group   singleflightx.Group[K, V]
	metrics *metrics.Metrics
}

// Set adds a value to the cache. If the key already exists, its value is updated. It uses the default ttl or none.
func (c *HotCache[K, V]) Set(key K, v V) {
	if c.copyOnWrite != nil {
		v = c.copyOnWrite(v)
	}

	c.setUnsafe(key, true, v, c.ttlMicro)
}

// SetMissing adds a key to the `missing` cache. If the key already exists, its value is dropped. It uses the default ttl or none.
func (c *HotCache[K, V]) SetMissing(key K) {
	if c.missingCache == nil && !c.missingSharedCache {
		panic("missing cache is not enabled")
	}

	c.setUnsafe(key, false, zero[V](), c.ttlMicro)
}

// SetWithTTL adds a value to the cache. If the key already exists, its value is updated. It uses the given ttl.
func (c *HotCache[K, V]) SetWithTTL(key K, v V, ttl time.Duration) {
	if c.copyOnWrite != nil {
		v = c.copyOnWrite(v)
	}

	c.setUnsafe(key, true, v, ttl.Microseconds())
}

// SetMissingWithTTL adds a key to the `missing` cache. If the key already exists, its value is dropped. It uses the given ttl.
func (c *HotCache[K, V]) SetMissingWithTTL(key K, ttl time.Duration) {
	if c.missingCache == nil && !c.missingSharedCache {
		panic("missing cache is not enabled")
	}

	c.setUnsafe(key, false, zero[V](), ttl.Microseconds())
}

// SetMany adds many values to the cache. If the keys already exist, values are updated. It uses the default ttl or none.
func (c *HotCache[K, V]) SetMany(items map[K]V) {
	if c.copyOnWrite != nil {
		for k, v := range items {
			items[k] = c.copyOnWrite(v)
		}
	}

	c.setManyUnsafe(items, []K{}, c.ttlMicro)
}

// SetMissingMany adds many keys to the cache. If the keys already exist, values are dropped. It uses the default ttl or none.
func (c *HotCache[K, V]) SetMissingMany(missingKeys []K) {
	if c.missingCache == nil && !c.missingSharedCache {
		panic("missing cache is not enabled")
	}

	c.setManyUnsafe(map[K]V{}, missingKeys, c.ttlMicro)
}

// SetManyWithTTL adds many values to the cache. If the keys already exist, values are updated. It uses the given ttl.
func (c *HotCache[K, V]) SetManyWithTTL(items map[K]V, ttl time.Duration) {
	if c.copyOnWrite != nil {
		for k, v := range items {
			items[k] = c.copyOnWrite(v)
		}
	}

	c.setManyUnsafe(items, []K{}, ttl.Microseconds())
}

// SetManyWithTTL adds many keys to the cache. If the keys already exist, values are dropped. It uses the given ttl.
func (c *HotCache[K, V]) SetMissingManyWithTTL(missingKeys []K, ttl time.Duration) {
	if c.missingCache == nil && !c.missingSharedCache {
		panic("missing cache is not enabled")
	}

	c.setManyUnsafe(map[K]V{}, missingKeys, ttl.Microseconds())
}

// Has checks if a key exists in the cache.
// Missing values are not valid, even if cached.
func (c *HotCache[K, V]) Has(key K) bool {
	v, ok := c.cache.Peek(key)
	return ok && v.hasValue
}

// HasMany checks if keys exist in the cache.
// Missing values are not valid, even if cached.
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

// Get returns a value from the cache, a boolean indicating whether the key was found and an error when loaders fail.
func (c *HotCache[K, V]) Get(key K) (value V, found bool, err error) {
	return c.GetWithLoaders(key, c.loaderFns...)
}

// MustGet returns a value from the cache, a boolean indicating whether the key was found. It panics when loaders fail.
func (c *HotCache[K, V]) MustGet(key K) (value V, found bool) {
	value, found, err := c.Get(key)
	if err != nil {
		panic(err)
	}

	return value, found
}

// GetWithLoaders returns a value from the cache, a boolean indicating whether the key was found and an error when loaders fail.
func (c *HotCache[K, V]) GetWithLoaders(key K, loaders ...Loader[K, V]) (value V, found bool, err error) {
	// the item might be found, but without value
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

// MustGetWithLoaders returns a value from the cache, a boolean indicating whether the key was found. It panics when loaders fail.
func (c *HotCache[K, V]) MustGetWithLoaders(key K, loaders ...Loader[K, V]) (value V, found bool) {
	value, found, err := c.GetWithLoaders(key, loaders...)
	if err != nil {
		panic(err)
	}

	return value, found
}

// GetMany returns many values from the cache, a slice of missing keys and an error when loaders fail.
func (c *HotCache[K, V]) GetMany(keys []K) (values map[K]V, missing []K, err error) {
	return c.GetManyWithLoaders(keys, c.loaderFns...)
}

// MustGetMany returns many values from the cache, a slice of missing keys. It panics when loaders fail.
func (c *HotCache[K, V]) MustGetMany(keys []K) (values map[K]V, missing []K) {
	values, missing, err := c.GetMany(keys)
	if err != nil {
		panic(err)
	}

	return values, missing
}

// GetManyWithLoaders returns many values from the cache, a slice of missing keys and an error when loaders fail.
func (c *HotCache[K, V]) GetManyWithLoaders(keys []K, loaders ...Loader[K, V]) (values map[K]V, missing []K, err error) {
	// Some items might be found in cached, but without value.
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

// MustGetManyWithLoaders returns many values from the cache, a slice of missing keys. It panics when loaders fail.
func (c *HotCache[K, V]) MustGetManyWithLoaders(keys []K, loaders ...Loader[K, V]) (values map[K]V, missing []K) {
	values, missing, err := c.GetManyWithLoaders(keys, loaders...)
	if err != nil {
		panic(err)
	}

	return values, missing
}

// Peek is similar to Get, but do not check expiration and do not call loaders/revalidation.
// Missing values are not returned, even if cached.
func (c *HotCache[K, V]) Peek(key K) (value V, ok bool) {
	// no need to check missingCache, since it will be missing anyway
	item, ok := c.cache.Peek(key)

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

	// no need to check missingCache, since it will be missing anyway
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

// Keys returns all keys in the cache.
// Missing keys are not included.
func (c *HotCache[K, V]) Keys() []K {
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

// Range calls a function for each key/value pair in the cache.
// The callback should be kept short because it is called while holding a read lock.
// @TODO: loop over missingCache? Use a different callback?
// Missing values are not included.
func (c *HotCache[K, V]) Range(f func(K, V) bool) {
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
	return c.cache.Delete(key) || (c.missingCache != nil && c.missingCache.Delete(key))
}

// DeleteMany removes many keys from the cache.
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

// Purge removes all items from the cache.
func (c *HotCache[K, V]) Purge() {
	c.cache.Purge()
	if c.missingCache != nil {
		// @TODO: should be done in a single call to avoid multiple locks
		c.missingCache.Purge()
	}
}

// Capacity returns the cache capacity.
func (c *HotCache[K, V]) Capacity() (int, int) {
	if c.missingCache != nil {
		// @TODO: should be done in a single call to avoid multiple locks
		return c.cache.Capacity(), c.missingCache.Capacity()
	}

	return c.cache.Capacity(), 0
}

// Algorithm returns the cache algo.
func (c *HotCache[K, V]) Algorithm() (string, string) {
	if c.missingCache != nil {
		// @TODO: should be done in a single call to avoid multiple locks
		return c.cache.Algorithm(), c.missingCache.Algorithm()
	}

	return c.cache.Algorithm(), ""
}

// Len returns the number of items in the cache.
// Missing items are included.
func (c *HotCache[K, V]) Len() int {

	if c.missingCache != nil {
		// @TODO: should be done in a single call to avoid multiple locks
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

	c.setManyUnsafe(items, missing, c.ttlMicro)

	return nil
}

// Janitor runs a background goroutine to clean up the cache.
func (c *HotCache[K, V]) Janitor() {
	c.ticker = time.NewTicker(time.Duration(c.ttlMicro) * time.Microsecond)

	go func() {
		for range c.ticker.C {
			nowMicro := internal.NowMicro()

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
	}()
}

func (c *HotCache[K, V]) StopJanitor() {
	c.ticker.Stop()
}

func (c *HotCache[K, V]) setUnsafe(key K, hasValue bool, value V, ttlMicro int64) {
	if !hasValue && c.missingCache == nil && !c.missingSharedCache {
		return
	}

	ttlMicro = applyJitter(ttlMicro, c.jitter)

	// since we don't know where is the previous key, we need to delete prempetively
	if c.missingCache != nil {
		// @TODO: should be done in a single call to avoid multiple locks
		if hasValue {
			c.missingCache.Delete(key)
		} else {
			c.cache.Delete(key)
		}
	}

	// @TODO: should be done in a single call to avoid multiple locks
	if hasValue || c.missingSharedCache {
		c.cache.Set(key, newItem(value, hasValue, ttlMicro, c.staleMicro))
	} else if c.missingCache != nil {
		c.missingCache.Set(key, newItemNoValue[V](ttlMicro, c.staleMicro))
	}
}

func (c *HotCache[K, V]) setManyUnsafe(items map[K]V, missing []K, ttlMicro int64) {
	if c.missingCache == nil && !c.missingSharedCache {
		missing = []K{}
	}

	if c.missingCache != nil {
		keysHavingValues := make([]K, 0, len(items))
		for k := range items {
			keysHavingValues = append(keysHavingValues, k)
		}

		// since we don't know where is the previous key, we need to delete all of them
		// @TODO: should be done in a single call to avoid multiple locks
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

// getUnsafe returns true if the key was fonud, even if it has no value.
func (c *HotCache[K, V]) getUnsafe(key K) (value *item[V], revalidate bool, found bool) {
	nowMicro := internal.NowMicro()

	// @TODO: should be done in a single call to avoid multiple locks
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
		// @TODO: should be done in a single call to avoid multiple locks
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

func (c *HotCache[K, V]) getManyUnsafe(keys []K) (cached map[K]*item[V], missing []K, revalidate map[K]*item[V]) {
	nowMicro := internal.NowMicro()

	cached = make(map[K]*item[V])
	missing = []K{}
	revalidate = make(map[K]*item[V])

	toDeleteCache := []K{}
	toDeleteMissingCache := []K{}
	onEvictKV := map[K]V{}

	tmp, keys := c.cache.GetMany(keys)
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
		// @TODO: should be done in a single call to avoid multiple locks
		deleted := c.cache.DeleteMany(toDeleteCache)
		if c.onEviction != nil {
			for k, ok := range deleted {
				if ok {
					c.onEviction(k, onEvictKV[k])
				}
			}
		}
	}

	if c.missingCache != nil {
		tmp, missing = c.missingCache.GetMany(keys)
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
			// @TODO: should be done in a single call to avoid multiple locks
			deleted := c.missingCache.DeleteMany(toDeleteMissingCache)
			if c.onEviction != nil {
				for k, ok := range deleted {
					if ok {
						c.onEviction(k, onEvictKV[k])
					}
				}
			}
		}
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
	// @TODO: a custom implem of go-singleflightx could be used to avoid some loops.
	results := c.group.DoX(keys, func(missing []K) (map[K]V, error) {
		results, stillMissing, err := loaders.run(missing)
		if err != nil {
			return nil, err
		}

		// Checking if a results is not empty before call to setManyUnsafe ensure we don't call mutex for nothing.
		if c.copyOnWrite != nil && len(results) > 0 {
			for k, v := range results {
				results[k] = c.copyOnWrite(v)
			}
		}

		// We keep track of missing keys to avoid calling the loaders again.
		// Any values in `results` that were not requested in `keys` are cached
		c.setManyUnsafe(results, stillMissing, c.ttlMicro)

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

	// @TODO: we might be fetching keys one by one, which is not efficient.
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
