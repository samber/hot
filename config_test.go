package hot

import (
	"testing"
	"time"

	"github.com/samber/hot/pkg/safe"
	"github.com/stretchr/testify/assert"
)

func TestComposeInternalCache(t *testing.T) {
	is := assert.New(t)

	cache := composeInternalCache[string, int](true, LRU, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("lru", cache.Algorithm())
	_, ok := cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.True(ok)

	cache = composeInternalCache[string, int](true, LFU, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("lfu", cache.Algorithm())
	_, ok = cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.True(ok)

	cache = composeInternalCache[string, int](true, TwoQueue, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("2q", cache.Algorithm())
	_, ok = cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.True(ok)

	cache = composeInternalCache[string, int](true, ARC, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("arc", cache.Algorithm())
	_, ok = cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.True(ok)

	is.Panics(func() {
		_ = composeInternalCache[string, int](true, ARC, 0, 0, -1, nil, nil, nil)
	})

	cache = composeInternalCache[string, int](false, LRU, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("lru", cache.Algorithm())
	_, ok = cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.False(ok)

	cache = composeInternalCache[string, int](false, LFU, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("lfu", cache.Algorithm())
	_, ok = cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.False(ok)

	cache = composeInternalCache[string, int](false, TwoQueue, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("2q", cache.Algorithm())
	_, ok = cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.False(ok)

	cache = composeInternalCache[string, int](false, ARC, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("arc", cache.Algorithm())
	_, ok = cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.False(ok)

	is.Panics(func() {
		_ = composeInternalCache[string, int](false, ARC, 0, 0, -1, nil, nil, nil)
	})
}

func TestAssertValue(t *testing.T) {
	is := assert.New(t)

	is.NotPanics(func() {
		assertValue(true, "error")
	})
	is.PanicsWithValue("error", func() {
		assertValue(false, "error")
	})
}

func TestHotCacheConfig(t *testing.T) {
	is := assert.New(t)

	// loader1 := func(keys []string) (map[string]int, []string, error) { return map[string]int{}, []string{}, nil }
	// loader2 := func(keys []string) (map[string]int, []string, error) { return map[string]int{}, []string{}, nil }
	// loaders := []Loader[string, int]{loader1, loader2}
	// warmUp := func(f func(map[string]int)) error { return nil }
	// twice := func(v int) { return v*2 }

	opts := NewHotCache[string, int](LRU, 42)
	is.EqualValues(HotCacheConfig[string, int]{
		cacheAlgo: LRU, cacheCapacity: 42, missingSharedCache: false, missingCacheAlgo: "", missingCacheCapacity: 0,
		ttl: 0, stale: 0, jitterLambda: 0, jitterUpperBound: 0, shards: 0, shardingFn: nil,
		lockingDisabled: false, janitorEnabled: false, prometheusMetricsEnabled: false, cacheName: "",
		warmUpFn: nil, loaderFns: nil, revalidationLoaderFns: nil, revalidationErrorPolicy: DropOnError,
		onEviction: nil, copyOnRead: nil, copyOnWrite: nil,
	}, opts)

	opts = opts.WithMissingSharedCache()
	is.EqualValues(HotCacheConfig[string, int]{
		cacheAlgo: LRU, cacheCapacity: 42, missingSharedCache: true, missingCacheAlgo: "", missingCacheCapacity: 0,
		ttl: 0, stale: 0, jitterLambda: 0, jitterUpperBound: 0, shards: 0, shardingFn: nil,
		lockingDisabled: false, janitorEnabled: false, prometheusMetricsEnabled: false, cacheName: "",
		warmUpFn: nil, loaderFns: nil, revalidationLoaderFns: nil, revalidationErrorPolicy: DropOnError,
		onEviction: nil, copyOnRead: nil, copyOnWrite: nil,
	}, opts)

	opts = NewHotCache[string, int](LRU, 42).WithMissingCache(LFU, 21)
	is.EqualValues(HotCacheConfig[string, int]{
		cacheAlgo: LRU, cacheCapacity: 42, missingSharedCache: false, missingCacheAlgo: LFU, missingCacheCapacity: 21,
		ttl: 0, stale: 0, jitterLambda: 0, jitterUpperBound: 0, shards: 0, shardingFn: nil,
		lockingDisabled: false, janitorEnabled: false, prometheusMetricsEnabled: false, cacheName: "",
		warmUpFn: nil, loaderFns: nil, revalidationLoaderFns: nil, revalidationErrorPolicy: DropOnError,
		onEviction: nil, copyOnRead: nil, copyOnWrite: nil,
	}, opts)

	is.Panics(func() {
		opts = opts.WithTTL(-42 * time.Second)
	})
	opts = opts.WithTTL(42 * time.Second)
	is.EqualValues(HotCacheConfig[string, int]{
		cacheAlgo: LRU, cacheCapacity: 42, missingSharedCache: false, missingCacheAlgo: LFU, missingCacheCapacity: 21,
		ttl: 42 * time.Second, stale: 0, jitterLambda: 0, jitterUpperBound: 0, shards: 0, shardingFn: nil,
		lockingDisabled: false, janitorEnabled: false, prometheusMetricsEnabled: false, cacheName: "",
		warmUpFn: nil, loaderFns: nil, revalidationLoaderFns: nil, revalidationErrorPolicy: DropOnError,
		onEviction: nil, copyOnRead: nil, copyOnWrite: nil,
	}, opts)

	is.Panics(func() {
		opts = opts.WithRevalidation(-21 * time.Second)
	})
	// opts = opts.WithRevalidation(21*time.Second, loader1, loader2)
	// is.EqualValues(HotCacheConfig[string, int]{LRU, 42, false, LFU, 21, 42 * time.Second, 21 * time.Second, 0, 0, nil, false, false, nil, nil, loaders,DropOnError, nil,nil, nil}, opts)

	is.Panics(func() {
		opts = opts.WithJitter(-0.1, time.Second)
	})

	opts = opts.WithJitter(2, time.Second)
	is.EqualValues(HotCacheConfig[string, int]{
		cacheAlgo: LRU, cacheCapacity: 42, missingSharedCache: false, missingCacheAlgo: LFU, missingCacheCapacity: 21,
		ttl: 42 * time.Second, stale: 0, jitterLambda: 2, jitterUpperBound: time.Second, shards: 0, shardingFn: nil,
		lockingDisabled: false, janitorEnabled: false, prometheusMetricsEnabled: false, cacheName: "",
		warmUpFn: nil, loaderFns: nil, revalidationLoaderFns: nil, revalidationErrorPolicy: DropOnError,
		onEviction: nil, copyOnRead: nil, copyOnWrite: nil,
	}, opts)

	// opts = opts.WithWarmUp(warmUp)
	// is.EqualValues(HotCacheConfig[string, int]{LRU, 42, false, LFU, 21, 42 * time.Second, 0, 2, time.Second, 0, nil,false, false, warmUp, nil, nil,DropOnError,nil, nil, nil}, opts)

	opts = opts.WithoutLocking()
	is.EqualValues(HotCacheConfig[string, int]{
		cacheAlgo: LRU, cacheCapacity: 42, missingSharedCache: false, missingCacheAlgo: LFU, missingCacheCapacity: 21,
		ttl: 42 * time.Second, stale: 0, jitterLambda: 2, jitterUpperBound: time.Second, shards: 0, shardingFn: nil,
		lockingDisabled: true, janitorEnabled: false, prometheusMetricsEnabled: false, cacheName: "",
		warmUpFn: nil, loaderFns: nil, revalidationLoaderFns: nil, revalidationErrorPolicy: DropOnError,
		onEviction: nil, copyOnRead: nil, copyOnWrite: nil,
	}, opts)

	opts = opts.WithJanitor()
	is.EqualValues(HotCacheConfig[string, int]{
		cacheAlgo: LRU, cacheCapacity: 42, missingSharedCache: false, missingCacheAlgo: LFU, missingCacheCapacity: 21,
		ttl: 42 * time.Second, stale: 0, jitterLambda: 2, jitterUpperBound: time.Second, shards: 0, shardingFn: nil,
		lockingDisabled: true, janitorEnabled: true, prometheusMetricsEnabled: false, cacheName: "",
		warmUpFn: nil, loaderFns: nil, revalidationLoaderFns: nil, revalidationErrorPolicy: DropOnError,
		onEviction: nil, copyOnRead: nil, copyOnWrite: nil,
	}, opts)

	// opts = opts.WithCopyOnRead(twice)
	// is.EqualValues(HotCacheConfig[string, int]{LRU, 42, false, LFU, 21, 42 * time.Second, 0, 2, time.Second, 0, nil, true, true, nil, nil,nil,DropOnError, nil, twice, nil}, opts)

	// opts = opts.WithCopyOnRead(twice)
	// is.EqualValues(HotCacheConfig[string, int]{LRU, 42, false, LFU, 21, 42 * time.Second, 0, 2, time.Second, 0, nil,true, true, nil, nil,nil,DropOnError, nil, twice, twice}, opts)

	is.Panics(func() {
		opts.Build()
	})

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}
