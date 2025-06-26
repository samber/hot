package hot

import (
	"testing"

	"github.com/samber/hot/pkg/arc"
	"github.com/samber/hot/pkg/base"
	"github.com/samber/hot/pkg/lfu"
	"github.com/samber/hot/pkg/lru"
	"github.com/samber/hot/pkg/metrics"
	"github.com/samber/hot/pkg/safe"
	"github.com/samber/hot/pkg/sharded"
	"github.com/samber/hot/pkg/twoqueue"
	"github.com/stretchr/testify/assert"
)

func mockCollectorBuilder(shard int) metrics.Collector {
	return &metrics.NoOpCollector{}
}

func TestComposeInternalCache(t *testing.T) {
	is := assert.New(t)

	// Test LRU with locking
	cache := composeInternalCache[string, int](true, LRU, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("lru", cache.Algorithm())
	_, ok := cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.True(ok)

	// Test LFU with locking
	cache = composeInternalCache[string, int](true, LFU, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("lfu", cache.Algorithm())
	_, ok = cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.True(ok)

	// Test TwoQueue with locking
	cache = composeInternalCache[string, int](true, TwoQueue, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("2q", cache.Algorithm())
	_, ok = cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.True(ok)

	// Test ARC with locking
	cache = composeInternalCache[string, int](true, ARC, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("arc", cache.Algorithm())
	_, ok = cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.True(ok)

	// Test invalid capacity (should panic)
	is.Panics(func() {
		_ = composeInternalCache[string, int](true, ARC, 0, 0, -1, nil, nil, nil)
	})

	// Test LRU without locking
	cache = composeInternalCache[string, int](false, LRU, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("lru", cache.Algorithm())
	_, ok = cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.False(ok)
	_, ok = cache.(*lru.LRUCache[string, *item[int]])
	is.True(ok)

	// Test LFU without locking
	cache = composeInternalCache[string, int](false, LFU, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("lfu", cache.Algorithm())
	_, ok = cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.False(ok)
	_, ok = cache.(*lfu.LFUCache[string, *item[int]])
	is.True(ok)

	// Test TwoQueue without locking
	cache = composeInternalCache[string, int](false, TwoQueue, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("2q", cache.Algorithm())
	_, ok = cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.False(ok)
	_, ok = cache.(*twoqueue.TwoQueueCache[string, *item[int]])
	is.True(ok)

	// Test ARC without locking
	cache = composeInternalCache[string, int](false, ARC, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("arc", cache.Algorithm())
	_, ok = cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.False(ok)
	_, ok = cache.(*arc.ARCCache[string, *item[int]])
	is.True(ok)

	// Test invalid capacity without locking (should panic)
	is.Panics(func() {
		_ = composeInternalCache[string, int](false, ARC, 0, 0, -1, nil, nil, nil)
	})
}

func TestComposeInternalCacheWithSharding(t *testing.T) {
	is := assert.New(t)

	hashFn := func(key string) uint64 { return uint64(len(key)) }
	shards := uint64(4)
	capacity := 42

	// Test sharded cache with locking
	cache := composeInternalCache[string, int](true, LRU, capacity, shards, -1, hashFn, nil, nil)
	is.Equal(capacity*int(shards), cache.Capacity())
	is.Equal("lru", cache.Algorithm())
	_, ok := cache.(*sharded.ShardedInMemoryCache[string, *item[int]])
	is.True(ok)

	// Test sharded cache without locking
	cache = composeInternalCache[string, int](false, LRU, capacity, shards, -1, hashFn, nil, nil)
	is.Equal(capacity*int(shards), cache.Capacity())
	is.Equal("lru", cache.Algorithm())
	_, ok = cache.(*sharded.ShardedInMemoryCache[string, *item[int]])
	is.True(ok)

	// Test invalid sharding configuration (should panic)
	is.Panics(func() {
		_ = composeInternalCache[string, int](true, LRU, capacity, shards, -1, nil, nil, nil)
	})
}

func TestComposeInternalCacheWithEvictionCallback(t *testing.T) {
	is := assert.New(t)

	evictionCallback := func(reason base.EvictionReason, key string, value int) {
		// Callback implementation for testing
	}

	// Test with eviction callback
	cache := composeInternalCache[string, int](false, LRU, 42, 0, -1, nil, evictionCallback, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("lru", cache.Algorithm())

	// Test that the callback is properly wrapped
	item := &item[int]{value: 123}
	cache.Set("test-key", item)
	cache.Delete("test-key") // This should trigger eviction callback

	// Note: The actual eviction callback behavior depends on the underlying cache implementation
	// We're just testing that the cache is created successfully with the callback
}

func TestComposeInternalCacheWithMetrics(t *testing.T) {
	is := assert.New(t)

	// Test with metrics collector
	cache := composeInternalCache[string, int](false, LRU, 42, 0, 0, nil, nil, mockCollectorBuilder)
	is.Equal(42, cache.Capacity())
	is.Equal("lru", cache.Algorithm())
	// Should be an InstrumentedCache
	_, isInstrumented := cache.(*metrics.InstrumentedCache[string, *item[int]])
	is.True(isInstrumented)

	// Test with metrics and locking
	cache = composeInternalCache[string, int](true, LRU, 42, 0, 0, nil, nil, mockCollectorBuilder)
	is.Equal(42, cache.Capacity())
	is.Equal("lru", cache.Algorithm())
	_, isSafe := cache.(*safe.SafeInMemoryCache[string, *item[int]])
	is.False(isSafe)
	_, isInstrumented = cache.(*metrics.InstrumentedCache[string, *item[int]])
	is.True(isInstrumented)
}

func TestComposeInternalCacheWithShardingAndMetrics(t *testing.T) {
	is := assert.New(t)

	hashFn := func(key string) uint64 { return uint64(len(key)) }
	shards := uint64(4)
	capacity := 42

	// Test sharded cache with metrics
	cache := composeInternalCache[string, int](false, LRU, capacity, shards, -1, hashFn, nil, mockCollectorBuilder)
	is.Equal(capacity*int(shards), cache.Capacity())
	is.Equal("lru", cache.Algorithm())
	_, ok := cache.(*sharded.ShardedInMemoryCache[string, *item[int]])
	is.True(ok)

	// Test sharded cache with metrics and locking
	cache = composeInternalCache[string, int](true, LRU, capacity, shards, -1, hashFn, nil, mockCollectorBuilder)
	is.Equal(capacity*int(shards), cache.Capacity())
	is.Equal("lru", cache.Algorithm())
	_, ok = cache.(*sharded.ShardedInMemoryCache[string, *item[int]])
	is.True(ok)
}

func TestComposeInternalCacheUnknownAlgorithm(t *testing.T) {
	is := assert.New(t)

	// Test unknown algorithm (should panic)
	is.Panics(func() {
		_ = composeInternalCache[string, int](false, "unknown", 42, 0, -1, nil, nil, nil)
	})
}

func TestComposeInternalCacheEdgeCases(t *testing.T) {
	is := assert.New(t)

	// Test with negative capacity (should panic)
	is.Panics(func() {
		_ = composeInternalCache[string, int](false, LRU, -1, 0, -1, nil, nil, nil)
	})

	// Test with zero shards (should work)
	cache := composeInternalCache[string, int](false, LRU, 42, 0, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("lru", cache.Algorithm())

	// Test with one shard (should work, treated as no sharding)
	cache = composeInternalCache[string, int](false, LRU, 42, 1, -1, nil, nil, nil)
	is.Equal(42, cache.Capacity())
	is.Equal("lru", cache.Algorithm())
	_, ok := cache.(*sharded.ShardedInMemoryCache[string, *item[int]])
	is.False(ok)

	// Test with shards > 1 and shardingFn provided (should work)
	hashFn := func(key string) uint64 { return uint64(len(key)) }
	cache = composeInternalCache[string, int](false, LRU, 10, 3, -1, hashFn, nil, nil)
	is.Equal(30, cache.Capacity())
	is.Equal("lru", cache.Algorithm())
}
