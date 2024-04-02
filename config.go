package hot

import (
	"time"

	twoqueue "github.com/samber/hot/2q"
	"github.com/samber/hot/base"
	"github.com/samber/hot/lfu"
	"github.com/samber/hot/lru"
	"github.com/samber/hot/sharded"
)

type CacheAlgorithm int

const (
	LRU CacheAlgorithm = iota
	LFU
	TwoQueue
	ARC
)

func composeInternalCache[K comparable, V any](locking bool, algorithm CacheAlgorithm, capacity int, shards uint16, shardingFn sharded.Hasher[K]) base.InMemoryCache[K, *item[V]] {
	assertValue(capacity >= 0, "capacity must be a positive value")
	assertValue((shards > 1 && shardingFn != nil) || shards == 0, "sharded cache requires sharding function")

	if shards > 1 {
		return sharded.NewShardedInMemoryCache(
			shards,
			func() base.InMemoryCache[K, *item[V]] {
				return composeInternalCache[K, V](false, algorithm, capacity, 0, nil)
			},
			shardingFn,
		)
	}

	var cache base.InMemoryCache[K, *item[V]]

	switch algorithm {
	case LRU:
		cache = lru.NewLRUCache[K, *item[V]](capacity)
	case LFU:
		cache = lfu.NewLFUCache[K, *item[V]](capacity)
	case TwoQueue:
		cache = twoqueue.New2QCache[K, *item[V]](capacity)
	case ARC:
		panic("ARC is not implemented yet")
		// return arc.NewARC(capacity)
	default:
		panic("unknown cache algorithm")
	}

	if locking {
		return base.NewSafeInMemoryCache(cache)
	}

	return cache
}

func assertValue(ok bool, msg string) {
	if !ok {
		panic(msg)
	}
}

func NewHotCache[K comparable, V any](algorithm CacheAlgorithm, capacity int) HotCacheConfig[K, V] {
	return HotCacheConfig[K, V]{
		cacheAlgo:     algorithm,
		cacheCapacity: capacity,
	}
}

type HotCacheConfig[K comparable, V any] struct {
	cacheAlgo            CacheAlgorithm
	cacheCapacity        int
	missingSharedCache   bool
	missingCacheAlgo     CacheAlgorithm
	missingCacheCapacity int

	ttl    time.Duration
	stale  time.Duration
	jitter float64

	shards     uint16
	shardingFn sharded.Hasher[K]

	lockingDisabled bool
	janitorEnabled  bool

	warmUpFn              func() (map[K]V, []K, error)
	loaderFns             LoaderChain[K, V]
	revalidationLoaderFns LoaderChain[K, V]
	copyOnRead            func(V) V
	copyOnWrite           func(V) V
}

// WithMissingSharedCache enables cache of missing keys. The missing cache is shared with the main cache.
func (cfg HotCacheConfig[K, V]) WithMissingSharedCache() HotCacheConfig[K, V] {
	cfg.missingSharedCache = true
	return cfg
}

// WithMissingCache enables cache of missing keys. The missing keys are stored in a separate cache.
func (cfg HotCacheConfig[K, V]) WithMissingCache(algorithm CacheAlgorithm, capacity int) HotCacheConfig[K, V] {
	cfg.missingCacheAlgo = algorithm
	cfg.missingCacheCapacity = capacity
	return cfg
}

func (cfg HotCacheConfig[K, V]) WithTTL(ttl time.Duration) HotCacheConfig[K, V] {
	assertValue(ttl >= 0, "ttl must be a positive value")

	cfg.ttl = ttl
	return cfg
}

func (cfg HotCacheConfig[K, V]) WithRevalidation(stale time.Duration, loaders ...Loader[K, V]) HotCacheConfig[K, V] {
	assertValue(stale >= 0, "stale must be a positive value")

	cfg.stale = stale
	cfg.revalidationLoaderFns = loaders
	return cfg
}

func (cfg HotCacheConfig[K, V]) WithJitter(jitter float64) HotCacheConfig[K, V] {
	assertValue(jitter >= 0 && jitter < 1, "jitter must be between 0 and 1")

	cfg.jitter = jitter
	return cfg
}

func (cfg HotCacheConfig[K, V]) WithSharding(nbr uint16, fn sharded.Hasher[K]) HotCacheConfig[K, V] {
	assertValue(nbr > 1, "jitter must be greater than 1")

	cfg.shards = nbr
	cfg.shardingFn = fn
	return cfg
}

func (cfg HotCacheConfig[K, V]) WithWarmUp(fn func() (map[K]V, []K, error)) HotCacheConfig[K, V] {
	cfg.warmUpFn = fn
	return cfg
}

func (cfg HotCacheConfig[K, V]) WithoutLocking() HotCacheConfig[K, V] {
	cfg.lockingDisabled = true
	return cfg
}

func (cfg HotCacheConfig[K, V]) WithJanitor() HotCacheConfig[K, V] {
	cfg.janitorEnabled = true
	return cfg
}

func (cfg HotCacheConfig[K, V]) WithLoaders(loaders ...Loader[K, V]) HotCacheConfig[K, V] {
	cfg.loaderFns = loaders
	return cfg
}

func (cfg HotCacheConfig[K, V]) WithCopyOnRead(copyOnRead func(V) V) HotCacheConfig[K, V] {
	cfg.copyOnRead = copyOnRead
	return cfg
}

func (cfg HotCacheConfig[K, V]) WithCopyOnWrite(copyOnWrite func(V) V) HotCacheConfig[K, V] {
	cfg.copyOnWrite = copyOnWrite
	return cfg
}

func (cfg HotCacheConfig[K, V]) Build() *HotCache[K, V] {
	assertValue(!cfg.janitorEnabled || !cfg.lockingDisabled, "lockingDisabled and janitorEnabled cannot be used together")

	// Using mutexMock cost ~3ns per operation. Which is more than the cost of calling base.SafeInMemoryCache abstraction (1ns).
	// Using mutexMock is more performant for this lib when locking is enabled most of time.

	hot := newHotCache(
		composeInternalCache[K, V](!cfg.lockingDisabled, cfg.cacheAlgo, cfg.cacheCapacity, cfg.shards, cfg.shardingFn),
		cfg.missingSharedCache,
		composeInternalCache[K, V](!cfg.lockingDisabled, cfg.missingCacheAlgo, cfg.missingCacheCapacity, cfg.shards, cfg.shardingFn),

		cfg.ttl,
		cfg.stale,
		cfg.jitter,

		cfg.loaderFns,
		cfg.revalidationLoaderFns,
		cfg.copyOnRead,
		cfg.copyOnWrite,
	)

	if cfg.warmUpFn != nil {
		hot.WarmUp(cfg.warmUpFn)
	}

	if cfg.janitorEnabled {
		hot.Janitor()
	}

	return hot
}
