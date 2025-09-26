package hot

import (
	"testing"
	"time"

	"github.com/samber/hot/pkg/base"
	"github.com/stretchr/testify/assert"
)

func TestAssertValue(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	is.NotPanics(func() {
		assertValue(true, "error")
	})
	is.PanicsWithValue("error", func() {
		assertValue(false, "error")
	})
}

func TestHotCacheConfig(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// loader1 := func(keys []string) (map[string]int, []string, error) { return map[string]int{}, []string{}, nil }
	// loader2 := func(keys []string) (map[string]int, []string, error) { return map[string]int{}, []string{}, nil }
	// loaders := []Loader[string, int]{loader1, loader2}
	// warmUp := func(f func(map[string]int)) error { return nil }
	// twice := func(v int) { return v*2 }

	opts := NewHotCache[string, int](LRU, 42)
	is.Equal(HotCacheConfig[string, int]{
		cacheAlgo: LRU, cacheCapacity: 42, missingSharedCache: false, missingCacheAlgo: "", missingCacheCapacity: 0,
		ttl: 0, stale: 0, jitterLambda: 0, jitterUpperBound: 0, shards: 0, shardingFn: nil,
		lockingDisabled: false, janitorEnabled: false, prometheusMetricsEnabled: false, cacheName: "",
		warmUpFn: nil, loaderFns: nil, revalidationLoaderFns: nil, revalidationErrorPolicy: DropOnError,
		onEviction: nil, copyOnRead: nil, copyOnWrite: nil,
	}, opts)

	opts = opts.WithMissingSharedCache()
	is.Equal(HotCacheConfig[string, int]{
		cacheAlgo: LRU, cacheCapacity: 42, missingSharedCache: true, missingCacheAlgo: "", missingCacheCapacity: 0,
		ttl: 0, stale: 0, jitterLambda: 0, jitterUpperBound: 0, shards: 0, shardingFn: nil,
		lockingDisabled: false, janitorEnabled: false, prometheusMetricsEnabled: false, cacheName: "",
		warmUpFn: nil, loaderFns: nil, revalidationLoaderFns: nil, revalidationErrorPolicy: DropOnError,
		onEviction: nil, copyOnRead: nil, copyOnWrite: nil,
	}, opts)

	opts = NewHotCache[string, int](LRU, 42).WithMissingCache(LFU, 21)
	is.Equal(HotCacheConfig[string, int]{
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
	is.Equal(HotCacheConfig[string, int]{
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
	is.Equal(HotCacheConfig[string, int]{
		cacheAlgo: LRU, cacheCapacity: 42, missingSharedCache: false, missingCacheAlgo: LFU, missingCacheCapacity: 21,
		ttl: 42 * time.Second, stale: 0, jitterLambda: 2, jitterUpperBound: time.Second, shards: 0, shardingFn: nil,
		lockingDisabled: false, janitorEnabled: false, prometheusMetricsEnabled: false, cacheName: "",
		warmUpFn: nil, loaderFns: nil, revalidationLoaderFns: nil, revalidationErrorPolicy: DropOnError,
		onEviction: nil, copyOnRead: nil, copyOnWrite: nil,
	}, opts)

	// opts = opts.WithWarmUp(warmUp)
	// is.EqualValues(HotCacheConfig[string, int]{LRU, 42, false, LFU, 21, 42 * time.Second, 0, 2, time.Second, 0, nil,false, false, warmUp, nil, nil,DropOnError,nil, nil, nil}, opts)

	opts = opts.WithoutLocking()
	is.Equal(HotCacheConfig[string, int]{
		cacheAlgo: LRU, cacheCapacity: 42, missingSharedCache: false, missingCacheAlgo: LFU, missingCacheCapacity: 21,
		ttl: 42 * time.Second, stale: 0, jitterLambda: 2, jitterUpperBound: time.Second, shards: 0, shardingFn: nil,
		lockingDisabled: true, janitorEnabled: false, prometheusMetricsEnabled: false, cacheName: "",
		warmUpFn: nil, loaderFns: nil, revalidationLoaderFns: nil, revalidationErrorPolicy: DropOnError,
		onEviction: nil, copyOnRead: nil, copyOnWrite: nil,
	}, opts)

	opts = opts.WithJanitor()
	is.Equal(HotCacheConfig[string, int]{
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

func TestWithRevalidationErrorPolicy(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	opts := NewHotCache[string, int](LRU, 42)

	// Test KeepOnError policy
	opts = opts.WithRevalidationErrorPolicy(KeepOnError)
	is.Equal(KeepOnError, opts.revalidationErrorPolicy)

	// Test DropOnError policy (default)
	opts = opts.WithRevalidationErrorPolicy(DropOnError)
	is.Equal(DropOnError, opts.revalidationErrorPolicy)
}

func TestWithSharding(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	opts := NewHotCache[string, int](LRU, 42)

	// Test valid sharding
	hashFn := func(key string) uint64 { return uint64(len(key)) }
	opts = opts.WithSharding(4, hashFn)
	is.Equal(uint64(4), opts.shards)
	is.NotNil(opts.shardingFn)

	// Test invalid sharding (should panic)
	is.Panics(func() {
		opts = opts.WithSharding(1, hashFn)
	})
}

func TestWithWarmUpWithTimeout(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	opts := NewHotCache[string, int](LRU, 42)

	// Test successful warmup
	warmUpFn := func() (map[string]int, []string, error) {
		return map[string]int{"key1": 1, "key2": 2}, []string{"missing1"}, nil
	}

	opts = opts.WithWarmUpWithTimeout(100*time.Millisecond, warmUpFn)
	is.NotNil(opts.warmUpFn)

	// Test timeout
	slowWarmUpFn := func() (map[string]int, []string, error) {
		time.Sleep(200 * time.Millisecond)
		return map[string]int{"key1": 1}, []string{}, nil
	}

	opts = opts.WithWarmUpWithTimeout(50*time.Millisecond, slowWarmUpFn)
	result, missing, err := opts.warmUpFn()
	is.Nil(result)
	is.Nil(missing)
	is.Error(err)
	is.Contains(err.Error(), "WarmUp timeout")
}

func TestWithEvictionCallback(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	opts := NewHotCache[string, int](LRU, 42)

	var callbackCalled bool
	var callbackKey string
	var callbackValue int
	var callbackReason base.EvictionReason

	evictionCallback := func(reason base.EvictionReason, key string, value int) {
		callbackCalled = true
		callbackReason = reason
		callbackKey = key
		callbackValue = value
	}

	opts = opts.WithEvictionCallback(evictionCallback)
	is.NotNil(opts.onEviction)

	// Test the callback
	opts.onEviction(base.EvictionReasonCapacity, "test-key", 42)
	is.True(callbackCalled)
	is.Equal(base.EvictionReasonCapacity, callbackReason)
	is.Equal("test-key", callbackKey)
	is.Equal(42, callbackValue)
}

func TestWithPrometheusMetrics(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	opts := NewHotCache[string, int](LRU, 42)

	// Test valid cache name
	opts = opts.WithPrometheusMetrics("test-cache")
	is.True(opts.prometheusMetricsEnabled)
	is.Equal("test-cache", opts.cacheName)

	// Test empty cache name (should panic)
	is.Panics(func() {
		opts = opts.WithPrometheusMetrics("")
	})
}

func TestBuildWithPrometheusMetrics(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	opts := NewHotCache[string, int](LRU, 42).WithPrometheusMetrics("test-cache")
	cache := opts.Build()
	is.NotNil(cache)

	// Test that metrics are properly configured
	is.True(opts.prometheusMetricsEnabled)
	is.Equal("test-cache", opts.cacheName)
}

func TestBuildWithSharding(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	hashFn := func(key string) uint64 { return uint64(len(key)) }
	opts := NewHotCache[string, int](LRU, 42).WithSharding(4, hashFn)
	cache := opts.Build()
	is.NotNil(cache)
}

func TestBuildWithMissingCache(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	opts := NewHotCache[string, int](LRU, 42).WithMissingCache(LFU, 21)
	cache := opts.Build()
	is.NotNil(cache)
}

func TestBuildWithJanitorAndLockingConflict(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// This should panic because janitor and without locking cannot be used together
	is.Panics(func() {
		opts := NewHotCache[string, int](LRU, 42).WithoutLocking().WithJanitor()
		opts.Build()
	})
}

func TestBuildWithWarmUp(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	warmUpFn := func() (map[string]int, []string, error) {
		return map[string]int{"key1": 1, "key2": 2}, []string{"missing1"}, nil
	}

	is.Panics(func() {
		_ = NewHotCache[string, int](LRU, 42).
			WithWarmUp(warmUpFn).
			Build()
	})

	cache := NewHotCache[string, int](LRU, 42).
		WithMissingCache(LFU, 21).
		WithWarmUp(warmUpFn).Build()
	is.NotNil(cache)
}

func TestBuildWithRevalidation(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	loader := func(keys []string) (map[string]int, error) {
		return map[string]int{"key1": 1}, nil
	}

	opts := NewHotCache[string, int](LRU, 42).WithRevalidation(100*time.Millisecond, loader)
	cache := opts.Build()
	is.NotNil(cache)
}

func TestBuildWithLoaders(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	loader := func(keys []string) (map[string]int, error) {
		return map[string]int{"key1": 1}, nil
	}

	opts := NewHotCache[string, int](LRU, 42).WithLoaders(loader)
	cache := opts.Build()
	is.NotNil(cache)
}

func TestBuildWithCopyFunctions(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	copyFn := func(v int) int { return v * 2 }

	opts := NewHotCache[string, int](LRU, 42).
		WithCopyOnRead(copyFn).
		WithCopyOnWrite(copyFn)
	cache := opts.Build()
	is.NotNil(cache)
}

func TestBuildWithEvictionCallback(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	evictionCallback := func(reason base.EvictionReason, key string, value int) {}

	opts := NewHotCache[string, int](LRU, 42).WithEvictionCallback(evictionCallback)
	cache := opts.Build()
	is.NotNil(cache)
}
