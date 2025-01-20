package hot

import (
	"errors"
	"time"

	"github.com/samber/hot/pkg/base"
	"github.com/samber/hot/pkg/lfu"
	"github.com/samber/hot/pkg/lru"
	"github.com/samber/hot/pkg/safe"
	"github.com/samber/hot/pkg/sharded"
	"github.com/samber/hot/pkg/twoqueue"
)

type EvictionAlgorithm int

const (
	LRU EvictionAlgorithm = iota
	LFU
	TwoQueue
	ARC
)

type revalidationErrorPolicy int

const (
	DropOnError revalidationErrorPolicy = iota
	KeepOnError
)

func composeInternalCache[K comparable, V any](locking bool, algorithm EvictionAlgorithm, capacity int, shards uint64, shardingFn sharded.Hasher[K], onEviction base.EvictionCallback[K, V]) base.InMemoryCache[K, *item[V]] {
	assertValue(capacity >= 0, "capacity must be a positive value")
	assertValue((shards > 1 && shardingFn != nil) || shards == 0, "sharded cache requires sharding function")

	if shards > 1 {
		return sharded.NewShardedInMemoryCache(
			shards,
			func() base.InMemoryCache[K, *item[V]] {
				return composeInternalCache[K, V](false, algorithm, capacity, 0, nil, onEviction)
			},
			shardingFn,
		)
	}

	var cache base.InMemoryCache[K, *item[V]]

	var onItemEviction base.EvictionCallback[K, *item[V]]
	if onEviction != nil {
		onItemEviction = func(key K, value *item[V]) {
			onEviction(key, value.value)
		}
	}

	switch algorithm {
	case LRU:
		cache = lru.NewLRUCacheWithEvictionCallback(capacity, onItemEviction)
	case LFU:
		cache = lfu.NewLFUCacheWithEvictionCallback(capacity, onItemEviction)
	case TwoQueue:
		cache = twoqueue.New2QCacheWithEvictionCallback(capacity, onItemEviction)
	case ARC:
		panic("ARC is not implemented yet")
		// return arc.NewARC(capacity)
	default:
		panic("unknown cache algorithm")
	}

	if locking {
		return safe.NewSafeInMemoryCache(cache)
	}

	return cache
}

func assertValue(ok bool, msg string) {
	if !ok {
		panic(msg)
	}
}

func NewHotCache[K comparable, V any](algorithm EvictionAlgorithm, capacity int) HotCacheConfig[K, V] {
	return HotCacheConfig[K, V]{
		cacheAlgo:     algorithm,
		cacheCapacity: capacity,
	}
}

type HotCacheConfig[K comparable, V any] struct {
	cacheAlgo            EvictionAlgorithm
	cacheCapacity        int
	missingSharedCache   bool
	missingCacheAlgo     EvictionAlgorithm
	missingCacheCapacity int

	ttl              time.Duration
	stale            time.Duration
	jitterLambda     float64
	jitterUpperBound time.Duration

	shards     uint64
	shardingFn sharded.Hasher[K]

	lockingDisabled bool
	janitorEnabled  bool

	warmUpFn                func() (map[K]V, []K, error)
	loaderFns               LoaderChain[K, V]
	revalidationLoaderFns   LoaderChain[K, V]
	revalidationErrorPolicy revalidationErrorPolicy
	onEviction              base.EvictionCallback[K, V]
	copyOnRead              func(V) V
	copyOnWrite             func(V) V
}

// WithMissingSharedCache enables cache of missing keys. The missing cache is shared with the main cache.
func (cfg HotCacheConfig[K, V]) WithMissingSharedCache() HotCacheConfig[K, V] {
	cfg.missingSharedCache = true
	return cfg
}

// WithMissingCache enables cache of missing keys. The missing keys are stored in a separate cache.
func (cfg HotCacheConfig[K, V]) WithMissingCache(algorithm EvictionAlgorithm, capacity int) HotCacheConfig[K, V] {
	cfg.missingCacheAlgo = algorithm
	cfg.missingCacheCapacity = capacity
	return cfg
}

// WithTTL sets the time-to-live for cache entries.
func (cfg HotCacheConfig[K, V]) WithTTL(ttl time.Duration) HotCacheConfig[K, V] {
	assertValue(ttl >= 0, "ttl must be a positive value")

	cfg.ttl = ttl
	return cfg
}

// WithRevalidation sets the time after which the cache entry is considered stale and needs to be revalidated.
// Keys that are not fetched during the interval will be dropped anyway.
// A timeout or error in loader will drop keys.
// If no revalidation loader is added, the default loaders or the one used in GetWithLoaders() are used.
func (cfg HotCacheConfig[K, V]) WithRevalidation(stale time.Duration, loaders ...Loader[K, V]) HotCacheConfig[K, V] {
	assertValue(stale >= 0, "stale must be a positive value")

	cfg.stale = stale
	cfg.revalidationLoaderFns = loaders
	return cfg
}

// WithRevalidationErrorPolicy sets the policy to apply when a revalidation loader returns an error.
// By default, the key is dropped from the cache.
func (cfg HotCacheConfig[K, V]) WithRevalidationErrorPolicy(policy revalidationErrorPolicy) HotCacheConfig[K, V] {
	cfg.revalidationErrorPolicy = policy
	return cfg
}

// WithJitter randomizes the TTL with an exponential distribution in the range [0, +upperBoundDuration).
func (cfg HotCacheConfig[K, V]) WithJitter(lambda float64, upperBoundDuration time.Duration) HotCacheConfig[K, V] {
	assertValue(lambda >= 0, "jitter lambda must be greater than or equal to 0")
	assertValue(upperBoundDuration >= 0, "jitter upper bound must be greater than or equal to 0s")

	cfg.jitterLambda = lambda
	cfg.jitterUpperBound = upperBoundDuration
	return cfg
}

// WithSharding enables cache sharding.
func (cfg HotCacheConfig[K, V]) WithSharding(nbr uint64, fn sharded.Hasher[K]) HotCacheConfig[K, V] {
	assertValue(nbr > 1, "jitter must be greater than 1")

	cfg.shards = nbr
	cfg.shardingFn = fn
	return cfg
}

// WithWarmUp preloads the cache with the provided data.
func (cfg HotCacheConfig[K, V]) WithWarmUp(fn func() (map[K]V, []K, error)) HotCacheConfig[K, V] {
	cfg.warmUpFn = fn
	return cfg
}

// WithWarmUpWithTimeout preloads the cache with the provided data.
// It can be used when the inner callback does not have timeout strategy.
func (cfg HotCacheConfig[K, V]) WithWarmUpWithTimeout(timeout time.Duration, fn func() (map[K]V, []K, error)) HotCacheConfig[K, V] {
	cfg.warmUpFn = func() (map[K]V, []K, error) {
		done := make(chan struct{})
		defer close(done)

		var result map[K]V
		var missing []K
		var err error

		go func() {
			result, missing, err = fn()
			done <- struct{}{}
		}()

		select {
		case <-time.After(timeout):
			return nil, nil, errors.New("WarmUp timeout")
		case <-done:
			return result, missing, err
		}
	}
	return cfg
}

// WithoutLocking disables mutex for the cache and improves internal performances.
func (cfg HotCacheConfig[K, V]) WithoutLocking() HotCacheConfig[K, V] {
	cfg.lockingDisabled = true
	return cfg
}

// WithJanitor enables the cache janitor.
func (cfg HotCacheConfig[K, V]) WithJanitor() HotCacheConfig[K, V] {
	cfg.janitorEnabled = true
	return cfg
}

// WithLoaders sets the chain of loaders to use for cache misses.
func (cfg HotCacheConfig[K, V]) WithLoaders(loaders ...Loader[K, V]) HotCacheConfig[K, V] {
	cfg.loaderFns = loaders
	return cfg
}

// WithEvictionCallback sets the callback to be called when an entry is evicted from the cache.
// The callback is called synchronously and might block the cache operations if it is slow.
// This implementation choice is subject to change. Please open an issue to discuss.
func (cfg HotCacheConfig[K, V]) WithEvictionCallback(onEviction base.EvictionCallback[K, V]) HotCacheConfig[K, V] {
	cfg.onEviction = onEviction
	return cfg
}

// WithCopyOnRead sets the function to copy the value on read.
func (cfg HotCacheConfig[K, V]) WithCopyOnRead(copyOnRead func(V) V) HotCacheConfig[K, V] {
	cfg.copyOnRead = copyOnRead
	return cfg
}

// WithCopyOnWrite sets the function to copy the value on write.
func (cfg HotCacheConfig[K, V]) WithCopyOnWrite(copyOnWrite func(V) V) HotCacheConfig[K, V] {
	cfg.copyOnWrite = copyOnWrite
	return cfg
}

func (cfg HotCacheConfig[K, V]) Build() *HotCache[K, V] {
	assertValue(!cfg.janitorEnabled || !cfg.lockingDisabled, "lockingDisabled and janitorEnabled cannot be used together")

	// Using mutexMock cost ~3ns per operation. Which is more than the cost of calling base.SafeInMemoryCache abstraction (1ns).
	// Using mutexMock is more performant for this lib when locking is enabled most of time.

	var missingCache base.InMemoryCache[K, *item[V]]
	if cfg.missingCacheCapacity > 0 {
		missingCache = composeInternalCache(!cfg.lockingDisabled, cfg.missingCacheAlgo, cfg.missingCacheCapacity, cfg.shards, cfg.shardingFn, cfg.onEviction)
	}

	hot := newHotCache(
		composeInternalCache(!cfg.lockingDisabled, cfg.cacheAlgo, cfg.cacheCapacity, cfg.shards, cfg.shardingFn, cfg.onEviction),
		cfg.missingSharedCache,
		missingCache,

		cfg.ttl,
		cfg.stale,
		cfg.jitterLambda,
		cfg.jitterUpperBound,

		cfg.loaderFns,
		cfg.revalidationLoaderFns,
		cfg.revalidationErrorPolicy,
		cfg.onEviction,
		cfg.copyOnRead,
		cfg.copyOnWrite,
	)

	if cfg.warmUpFn != nil {
		// @TODO: check error ?
		hot.WarmUp(cfg.warmUpFn) //nolint:errcheck
	}

	if cfg.janitorEnabled {
		hot.Janitor()
	}

	return hot
}
