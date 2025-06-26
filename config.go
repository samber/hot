package hot

import (
	"errors"
	"time"

	"github.com/samber/hot/pkg/base"
	"github.com/samber/hot/pkg/metrics"
	"github.com/samber/hot/pkg/sharded"
)

// EvictionAlgorithm represents the cache eviction policy to use.
type EvictionAlgorithm string

const (
	LRU      EvictionAlgorithm = "lru"
	LFU      EvictionAlgorithm = "lfu"
	TwoQueue EvictionAlgorithm = "2q"
	ARC      EvictionAlgorithm = "arc"
)

// revalidationErrorPolicy defines how to handle errors during revalidation.
type revalidationErrorPolicy int

const (
	DropOnError revalidationErrorPolicy = iota
	KeepOnError
)

// NewHotCache creates a new HotCache configuration with the specified eviction algorithm and capacity.
// This is the starting point for building a cache with the builder pattern.
func NewHotCache[K comparable, V any](algorithm EvictionAlgorithm, capacity int) HotCacheConfig[K, V] {
	return HotCacheConfig[K, V]{
		cacheAlgo:     algorithm,
		cacheCapacity: capacity,
	}
}

// HotCacheConfig holds the configuration for a HotCache instance.
// It uses the builder pattern to allow fluent configuration.
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

	// Metrics configuration
	prometheusMetricsEnabled bool
	cacheName                string
	collectors               []metrics.Collector

	warmUpFn                func() (map[K]V, []K, error)
	loaderFns               LoaderChain[K, V]
	revalidationLoaderFns   LoaderChain[K, V]
	revalidationErrorPolicy revalidationErrorPolicy
	onEviction              base.EvictionCallback[K, V]
	copyOnRead              func(V) V
	copyOnWrite             func(V) V
}

// WithMissingSharedCache enables caching of missing keys in the main cache.
// Missing keys are stored alongside regular values in the same cache instance.
func (cfg HotCacheConfig[K, V]) WithMissingSharedCache() HotCacheConfig[K, V] {
	cfg.missingSharedCache = true
	return cfg
}

// WithMissingCache enables caching of missing keys in a separate cache instance.
// The missing keys are stored in a dedicated cache with its own eviction algorithm and capacity.
func (cfg HotCacheConfig[K, V]) WithMissingCache(algorithm EvictionAlgorithm, capacity int) HotCacheConfig[K, V] {
	cfg.missingCacheAlgo = algorithm
	cfg.missingCacheCapacity = capacity
	return cfg
}

// WithTTL sets the time-to-live for cache entries.
// After this duration, entries will be considered expired and will be removed.
func (cfg HotCacheConfig[K, V]) WithTTL(ttl time.Duration) HotCacheConfig[K, V] {
	assertValue(ttl > 0, "ttl must be a positive value")

	cfg.ttl = ttl
	return cfg
}

// WithRevalidation sets the stale duration and optional revalidation loaders.
// After the TTL expires, entries become stale and can still be served while being revalidated in the background.
// Keys that are not fetched during the stale period will be dropped.
// If no revalidation loaders are provided, the default loaders or those used in GetWithLoaders() are used.
func (cfg HotCacheConfig[K, V]) WithRevalidation(stale time.Duration, loaders ...Loader[K, V]) HotCacheConfig[K, V] {
	assertValue(stale >= 0, "stale must be a positive value")

	cfg.stale = stale
	cfg.revalidationLoaderFns = loaders
	return cfg
}

// WithRevalidationErrorPolicy sets the policy to apply when a revalidation loader returns an error.
// By default, keys are dropped from the cache on revalidation errors.
func (cfg HotCacheConfig[K, V]) WithRevalidationErrorPolicy(policy revalidationErrorPolicy) HotCacheConfig[K, V] {
	cfg.revalidationErrorPolicy = policy
	return cfg
}

// WithJitter randomizes the TTL with an exponential distribution in the range [0, upperBoundDuration).
// This helps prevent cache stampedes by spreading out when entries expire.
func (cfg HotCacheConfig[K, V]) WithJitter(lambda float64, upperBoundDuration time.Duration) HotCacheConfig[K, V] {
	assertValue(lambda >= 0, "jitter lambda must be greater than or equal to 0")
	assertValue(upperBoundDuration >= 0, "jitter upper bound must be greater than or equal to 0s")

	cfg.jitterLambda = lambda
	cfg.jitterUpperBound = upperBoundDuration
	return cfg
}

// WithSharding enables cache sharding for better concurrency performance.
// The cache is split into multiple shards based on the provided hash function.
func (cfg HotCacheConfig[K, V]) WithSharding(nbr uint64, fn sharded.Hasher[K]) HotCacheConfig[K, V] {
	assertValue(nbr > 1, "shards must be greater than 1")

	cfg.shards = nbr
	cfg.shardingFn = fn
	return cfg
}

// WithWarmUp preloads the cache with data from the provided function.
// This is useful for initializing the cache with frequently accessed data.
func (cfg HotCacheConfig[K, V]) WithWarmUp(fn func() (map[K]V, []K, error)) HotCacheConfig[K, V] {
	cfg.warmUpFn = fn
	return cfg
}

// WithWarmUpWithTimeout preloads the cache with data from the provided function with a timeout.
// This is useful when the inner callback does not have its own timeout strategy.
func (cfg HotCacheConfig[K, V]) WithWarmUpWithTimeout(timeout time.Duration, fn func() (map[K]V, []K, error)) HotCacheConfig[K, V] {
	cfg.warmUpFn = func() (map[K]V, []K, error) {
		done := make(chan struct{}, 1)

		var result map[K]V
		var missing []K
		var err error

		go func() {
			result, missing, err = fn()
			done <- struct{}{}
			close(done)
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

// WithoutLocking disables mutex for the cache and improves internal performance.
// This should only be used when the cache is not accessed concurrently.
// Cannot be used together with WithJanitor().
func (cfg HotCacheConfig[K, V]) WithoutLocking() HotCacheConfig[K, V] {
	cfg.lockingDisabled = true
	return cfg
}

// WithJanitor enables the cache janitor that periodically removes expired items.
// The janitor runs in the background and cannot be used together with WithoutLocking().
func (cfg HotCacheConfig[K, V]) WithJanitor() HotCacheConfig[K, V] {
	cfg.janitorEnabled = true
	return cfg
}

// WithLoaders sets the chain of loaders to use for cache misses.
// These loaders will be called in sequence when a key is not found in the cache.
func (cfg HotCacheConfig[K, V]) WithLoaders(loaders ...Loader[K, V]) HotCacheConfig[K, V] {
	cfg.loaderFns = loaders
	return cfg
}

// WithEvictionCallback sets the callback to be called when an entry is evicted from the cache.
// The callback is called synchronously and might block cache operations if it is slow.
// This implementation choice is subject to change. Please open an issue to discuss.
func (cfg HotCacheConfig[K, V]) WithEvictionCallback(onEviction base.EvictionCallback[K, V]) HotCacheConfig[K, V] {
	cfg.onEviction = onEviction
	return cfg
}

// WithCopyOnRead sets the function to copy the value when reading from the cache.
// This is useful for ensuring thread safety when the cached values are mutable.
func (cfg HotCacheConfig[K, V]) WithCopyOnRead(copyOnRead func(V) V) HotCacheConfig[K, V] {
	cfg.copyOnRead = copyOnRead
	return cfg
}

// WithCopyOnWrite sets the function to copy the value when writing to the cache.
// This is useful for ensuring thread safety when the cached values are mutable.
func (cfg HotCacheConfig[K, V]) WithCopyOnWrite(copyOnWrite func(V) V) HotCacheConfig[K, V] {
	cfg.copyOnWrite = copyOnWrite
	return cfg
}

// WithPrometheusMetrics enables metric collection for the cache with the specified name.
// The cache name is required when metrics are enabled and will be used as a label in Prometheus metrics.
// When the cache is sharded, metrics will be collected for each shard with the shard number as an additional label.
func (cfg HotCacheConfig[K, V]) WithPrometheusMetrics(cacheName string) HotCacheConfig[K, V] {
	assertValue(cacheName != "", "cache name is required when metrics are enabled")

	cfg.prometheusMetricsEnabled = true
	cfg.cacheName = cacheName
	return cfg
}

// Build creates and returns a new HotCache instance with the current configuration.
// This method validates the configuration and creates all necessary internal components.
// The cache is ready to use immediately after this call.
func (cfg HotCacheConfig[K, V]) Build() *HotCache[K, V] {
	assertValue(!cfg.janitorEnabled || !cfg.lockingDisabled, "lockingDisabled and janitorEnabled cannot be used together")

	var collectorBuilderMain func(shard int) metrics.Collector
	var collectorBuilderMissing func(shard int) metrics.Collector
	if cfg.prometheusMetricsEnabled {
		collectorBuilderMain = cfg.buildPrometheusCollector(base.CacheModeMain)
		collectorBuilderMissing = cfg.buildPrometheusCollector(base.CacheModeMissing)
	}

	var missingCache base.InMemoryCache[K, *item[V]]
	if cfg.missingCacheCapacity > 0 {
		missingCache = composeInternalCache(!cfg.lockingDisabled, cfg.missingCacheAlgo, cfg.missingCacheCapacity, cfg.shards, -1, cfg.shardingFn, cfg.onEviction, collectorBuilderMissing)
	}

	cacheInstance := composeInternalCache(!cfg.lockingDisabled, cfg.cacheAlgo, cfg.cacheCapacity, cfg.shards, -1, cfg.shardingFn, cfg.onEviction, collectorBuilderMain)
	hot := newHotCache(
		cacheInstance,
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

		cfg.collectors,
	)

	if cfg.warmUpFn != nil {
		// @TODO: Check error?
		hot.WarmUp(cfg.warmUpFn) //nolint:errcheck
	}

	if cfg.janitorEnabled {
		hot.Janitor()
	}

	return hot
}

func (cfg *HotCacheConfig[K, V]) buildPrometheusCollector(mode base.CacheMode) func(shard int) metrics.Collector {
	return func(shard int) metrics.Collector {
		collector := metrics.NewPrometheusCollector(
			cfg.cacheName,
			shard,
			mode,
			cfg.cacheCapacity,
			string(cfg.cacheAlgo),
			emptyableToPtr(cfg.ttl),
			emptyableToPtr(cfg.jitterLambda),
			emptyableToPtr(cfg.jitterUpperBound),
			emptyableToPtr(cfg.stale),
			emptyableToPtr(cfg.missingCacheCapacity),
		)

		cfg.collectors = append(cfg.collectors, collector)

		return collector
	}
}
