package hot

import (
	"github.com/samber/hot/pkg/arc"
	"github.com/samber/hot/pkg/base"
	"github.com/samber/hot/pkg/fifo"
	"github.com/samber/hot/pkg/lfu"
	"github.com/samber/hot/pkg/lru"
	"github.com/samber/hot/pkg/metrics"
	"github.com/samber/hot/pkg/safe"
	"github.com/samber/hot/pkg/sharded"
	"github.com/samber/hot/pkg/tinylfu"
	"github.com/samber/hot/pkg/twoqueue"
	"github.com/samber/hot/pkg/wtinylfu"
)

//
// This diagram shows an example of a composition of the HotCache[K, V] and its layers.
// Combining multiple encapsulation has a small cost (~1ns per call), but offers great customization.
//
// ┌─────────────────────────────────────────────────────────────┐
// │                    hot.HotCache[K, V]                       │
// │              (High-level, feature-complete)                 │
// └─────────────────────────────────────────────────────────────┘
//                               │
//                               ▼
// ┌─────────────────────────────────────────────────────────────┐
// │              pkg/sharded.ShardedInMemoryCache               │
// │                    (Sharding layer)                         │
// └─────────────────────────────────────────────────────────────┘
//                     │    │    │    │    │
//                     ▼    ▼    ▼    ▼    ▼
// ┌─────────────────────────────────────────────────────────────┐
// │              pkg/metrics.InstrumentedCache[K, V]            │
// │                   (Metric collection layer)                 │
// └─────────────────────────────────────────────────────────────┘
//                               │
//                               ▼
// ┌─────────────────────────────────────────────────────────────┐
// │              pkg/safe.SafeInMemoryCache[K, V]               │
// │                   (Thread safety layer)                     │
// └─────────────────────────────────────────────────────────────┘
//                               │
//                               ▼
// ┌─────────────────────────────────────────────────────────────┐
// │              pkg/lru.LRUCache[K, V]                         │
// │              pkg/lfu.LFUCache[K, V]                         │
// │              pkg/arc.ARCCache[K, V]                         │
// │              pkg/fifo.FIFOCache[K, V]                       │
// │              pkg/twoqueue.TwoQueueCache[K, V]               │
// │                   (Eviction policies)                       │
// └─────────────────────────────────────────────────────────────┘
//

// composeInternalCache creates an internal cache instance based on the provided configuration.
// It handles sharding, locking, and different eviction algorithms.
func composeInternalCache[K comparable, V any](
	locking bool,
	algorithm EvictionAlgorithm,
	capacity int,
	shards uint64,
	shardIndex int,
	shardingFn sharded.Hasher[K],
	onEviction base.EvictionCallback[K, V],
	collectorBuilder func(shard int) metrics.Collector,
) base.InMemoryCache[K, *item[V]] {
	assertValue(capacity >= 0, "capacity must be a positive value")
	assertValue((shards > 1 && shardingFn != nil) || shards <= 1, "sharded cache requires sharding function")

	if shards > 1 {
		return sharded.NewShardedInMemoryCache(
			shards,
			func(shardIndex int) base.InMemoryCache[K, *item[V]] {
				return composeInternalCache(false, algorithm, capacity, 0, shardIndex, nil, onEviction, collectorBuilder)
			},
			shardingFn,
		)
	}

	var cache base.InMemoryCache[K, *item[V]]

	var onItemEviction base.EvictionCallback[K, *item[V]]
	if onEviction != nil {
		onItemEviction = func(reason base.EvictionReason, key K, value *item[V]) {
			onEviction(reason, key, value.value)
		}
	}

	switch algorithm {
	case LRU:
		cache = lru.NewLRUCacheWithEvictionCallback(capacity, onItemEviction)
	case LFU:
		cache = lfu.NewLFUCacheWithEvictionCallback(capacity, onItemEviction)
	case TinyLFU:
		cache = tinylfu.NewTinyLFUCacheWithEvictionCallback(capacity, onItemEviction)
	case WTinyLFU:
		cache = wtinylfu.NewWTinyLFUCacheWithEvictionCallback(capacity, onItemEviction)
	case TwoQueue:
		cache = twoqueue.New2QCacheWithEvictionCallback(capacity, onItemEviction)
	case ARC:
		cache = arc.NewARCCacheWithEvictionCallback(capacity, onItemEviction)
	case FIFO:
		cache = fifo.NewFIFOCacheWithEvictionCallback(capacity, onItemEviction)
	default:
		panic("unknown cache algorithm")
	}

	// Using mutexMock costs ~3ns per operation, which is more than the cost of calling base.SafeInMemoryCache abstraction (1ns).
	// Using mutexMock is more performant for this library when locking is enabled most of the time.

	if locking {
		cache = safe.NewSafeInMemoryCache(cache)
	}

	if collectorBuilder != nil {
		cache = metrics.NewInstrumentedCache(cache, collectorBuilder(shardIndex))
	}

	return cache
}
