# HOT - Blazing Fast In-Memory Caching for Go

[![tag](https://img.shields.io/github/tag/samber/hot.svg)](https://github.com/samber/hot/releases)
![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.22-%23007d9c)
[![GoDoc](https://godoc.org/github.com/samber/hot?status.svg)](https://pkg.go.dev/github.com/samber/hot)
![Build Status](https://github.com/samber/hot/actions/workflows/test.yml/badge.svg)
[![Go report](https://goreportcard.com/badge/github.com/samber/hot)](https://goreportcard.com/report/github.com/samber/hot)
[![Coverage](https://img.shields.io/codecov/c/github/samber/hot)](https://codecov.io/gh/samber/hot)
[![Contributors](https://img.shields.io/github/contributors/samber/hot)](https://github.com/samber/hot/graphs/contributors)
[![License](https://img.shields.io/github/license/samber/hot)](./LICENSE)

**HOT** stands for **H**ot **O**bject **T**racker - a feature-complete, blazing-fast caching library for Go applications.

![image](https://github.com/user-attachments/assets/acae303a-73b9-4cea-98a8-c7d77285feba)

## 🚀 Features

- ⚡ **High Performance**: Optimized for speed
- 🔄 **Multiple Eviction Policies**: LRU, LFU, TinyLFU, W-TinyLFU, S3FIFO, ARC, 2Q, and FIFO algorithms
- ⏰ **TTL with Jitter**: Prevent cache stampedes with exponential distribution
- 🔄 **Stale-While-Revalidate**: Serve stale data while refreshing in background
- ❌ **Missing Key Caching**: Cache negative results to avoid repeated lookups
- 🍕 **Sharded Cache**: Scale horizontally with multiple cache shards
- 🔒 **Thread Safety**: Optional locking with zero-cost when disabled
- 🔗 **Loader Chains**: Chain multiple data sources with in-flight deduplication
- 🌶️ **Cache Warmup**: Preload frequently accessed data
- 📦 **Batch Operations**: Efficient bulk operations for better performance
- 🧩 **Composable Design**: Mix and match caching strategies
- 📝 **Copy-on-Read/Write**: Optional value copying for thread safety
- 📊 **Metrics Collection**: Built-in statistics and monitoring
- 💫 **Go Generics**: Type-safe caching with compile-time guarantees
- 🏡 **Bring your own cache**: Pluggable APIs and highly customizable

## 📋 Table of Contents

- [🏎️ Performance](#️-performance)
- [📦 Installation](#-installation)
- [🤠 Getting Started](#-getting-started)
- [🍱 API Reference](#-api-reference)
- [🏛️ Architecture](#️-architecture)
- [🪄 Examples](#-examples)
- [👀 Observability](#-observability)
- [🤝 Contributing](#-contributing)

## 🏎️ Performance

HOT is optimized for high-performance scenarios:

- **Zero-allocation operations** where possible
- **Lock-free operations** when thread safety is disabled
- **Batch operations** for better throughput
- **Sharded architecture** for high concurrency
- **Monotonic clock lookup** (2.5x faster)

## 📦 Installation

```bash
go get github.com/samber/hot
```

This library is v0 and follows SemVer strictly.

Some breaking changes might be made to exported APIs before v1.0.0.

## 🤠 Getting started

Let's start with a simple LRU cache and 10 minutes TTL:

```go
import "github.com/samber/hot"

cache := hot.NewHotCache[string, int](hot.LRU, 1_000_000).
    WithTTL(10*time.Minute).
    Build()

cache.Set("hello", 42)

values, missing := cache.GetMany([]string{"bar", "baz", "hello"})
// values: {"hello": 42}
// missing: ["baz", "bar"]
```

## 🍱 API Reference

[GoDoc: https://godoc.org/github.com/samber/hot](https://godoc.org/github.com/samber/hot)

### Configuration Options

TTL and expiration settings:

```go
// Set default time-to-live for all cache entries
WithTTL(ttl time.Duration)
// Add random jitter to TTL to prevent cache stampedes
WithJitter(lambda float64, upperBound time.Duration)
// Enable background cleanup of expired items
WithJanitor()
```

Background cache revalidation (stale-while-revalidate pattern):

```go
// Keep serving stale data while refreshing in background
WithRevalidation(stale time.Duration, loaders ...hot.Loader[K, V])
// Control behavior when revalidation fails (KeepOnError/DropOnError)
WithRevalidationErrorPolicy(policy hot.RevalidationErrorPolicy)
```

Missing key caching - prevents repeated lookups for non-existent keys:

```go
// Use separate cache for missing keys (prevents main cache pollution)
WithMissingCache(algorithm hot.EvictionAlgorithm, capacity int)
// Share missing key cache with main cache (good for low missing rate)
WithMissingSharedCache()
```

Data source integration:

```go
// Set chain of loaders for cache misses (primary, fallback, etc.)
WithLoaders(loaders ...hot.Loader[K, V])
```

Thread safety configuration:

```go
// Disable mutex for single-threaded applications (performance boost)
WithoutLocking()
// Copy values when reading (prevents external modification)
WithCopyOnRead(copier func(V) V)
// Copy values when writing (ensures cache owns the data)
WithCopyOnWrite(copier func(V) V)
```

Sharding for high concurrency scenarios:

```go
// Split cache into multiple shards to reduce lock contention
WithSharding(shards uint64, hasher sharded.Hasher[K])
```

Event callbacks and hooks:

```go
// Called when items are evicted (LRU/LFU/TinyLFU/W-TinyLFU/S3FIFO/expiration)
WithEvictionCallback(callback func(key K, value V))
// Preload cache on startup with data from loader
WithWarmUp(loader func() (map[K]V, []K, error))
// Preload with timeout protection for slow data sources
WithWarmUpWithTimeout(timeout time.Duration, loader func() (map[K]V, []K, error))
```

Monitoring and metrics:

```go
// Enable Prometheus metrics collection with the specified cache name
WithPrometheusMetrics(cacheName string)
```

Eviction algorithms:

```go
hot.LRU
hot.LFU
hot.TinyLFU
hot.WTinyLFU
hot.S3FIFO
hot.TwoQueue
hot.ARC
hot.FIFO
```

Revalidation policies:

```go
hot.KeepOnError
hot.DropOnError
```

### Core Methods

Basic operations:

```go
// Store a key-value pair in the cache
cache.Set(key K, value V)
// Store with custom time-to-live (overrides default TTL)
cache.SetWithTTL(key K, value V, ttl time.Duration)
// Retrieve value by key, returns found status and any error
cache.Get(key K) -> (value V, found bool, error error)
// Check if key exists in cache (no side effects)
cache.Has(key K) -> bool
// Remove key from cache, returns true if key existed
cache.Delete(key K) -> bool
```

Batch operations (more efficient for multiple items):

```go
// Store multiple key-value pairs atomically
cache.SetMany(items map[K]V)
// Retrieve multiple values, returns found items and missing keys
cache.GetMany(keys []K) -> (found map[K]V, missing []K)
// Check existence of multiple keys, returns map of key->exists
cache.HasMany(keys []K) -> map[K]bool
// Remove multiple keys, returns map of key->was_deleted
cache.DeleteMany(keys []K) -> map[K]bool
```

Inspection methods (no side effects on cache state):

```go
// Get value without updating access time or LRU position
cache.Peek(key K) -> (value V, found bool)
// Get multiple values without side effects
cache.PeekMany(keys []K) -> (found map[K]V, missing []K)
// Get all keys in cache (order not guaranteed)
cache.Keys() -> []K
// Get all values in cache (order not guaranteed)
cache.Values() -> []V
// Get all keys and values in cache (order not guaranteed)
cache.All() -> map[K]V
// Iterate over all key-value pairs, return false to stop
cache.Range(fn func(key K, value V) bool)
// Get current number of items in cache
cache.Len() -> int
// Get (current_size, max_capacity) of cache
cache.Capacity() -> (current int, max int)
// Get (algorithm_name, algorithm_version) info
cache.Algorithm() -> (name string, version string)
```

Cache management and lifecycle:

```go
// Remove all items from cache immediately
cache.Purge()
// Preload cache with data from loader function
cache.WarmUp(loader hot.Loader[K, V]) -> error
// Start background cleanup of expired items
cache.Janitor()
// Stop background janitor process
cache.StopJanitor()
```

### Loader Interface

```go
// Loader function signature for fetching data from external sources
// Called when cache misses occur, with automatic deduplication of concurrent requests
type Loader[K comparable, V any] func(keys []K) (found map[K]V, err error)

// Example:
func userLoader(keys []string) (found map[string]*User, err error) {
    // Fetch users from database
    // Return map of found users (key -> user object)
    // Return empty map if no users found (not an error)
    // Return error if database query fails
    return users, nil
}
```

### Shard partitioner

```go
// Hasher is responsible for generating unsigned, 16 bit hash of provided key.
// Hasher should minimize collisions. For great performance, a fast function is preferable.
type Hasher[K any] func(key K) uint64

// Example:
func hash(key string) uint64 {
    hasher := fnv.New64a()
    hasher.Write([]byte(s))
    return hasher.Sum64()
}
```

## 🏛️ Architecture

This project has been split into multiple layers to respect the separation of concern.

Each cache layer implements the `pkg/base.InMemoryCache[K, V]` interface. Combining multiple encapsulation has a small cost (~1ns per call), but offers great customization.

We highly recommend using `hot.HotCache[K, V]` instead of lower layers.

Example:

```
┌─────────────────────────────────────────────────────────────┐
│                    hot.HotCache[K, V]                       │
│              (High-level, feature-complete)                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              pkg/sharded.ShardedInMemoryCache               │
│                    (Sharding layer)                         │
└─────────────────────────────────────────────────────────────┘
                    │    │    │    │    │
                    ▼    ▼    ▼    ▼    ▼
┌─────────────────────────────────────────────────────────────┐
│              pkg/metrics.InstrumentedCache[K, V]            │
│                   (Metric collection layer)                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              pkg/safe.SafeInMemoryCache[K, V]               │
│                   (Thread safety layer)                     │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              pkg/lru.LRUCache[K, V]                         │
│              pkg/lfu.LFUCache[K, V]                         │
│              pkg/lfu.TinyLFUCache[K, V]                     │
│              pkg/lfu.WTinyLFUCache[K, V]                    │
│              pkg/lfu.S3FIFOCache[K, V]                      │
│              pkg/arc.ARCCache[K, V]                         │
│              pkg/fifo.FIFOCache[K, V]                       │
│              pkg/twoqueue.TwoQueueCache[K, V]               │
│                   (Eviction policies)                       │
└─────────────────────────────────────────────────────────────┘
```

### Eviction policies

This project provides multiple eviction policies. Each implements the `pkg/base.InMemoryCache[K, V]` interface.

They are not protected against concurrent access. If safety is required, encapsulate it into `pkg/safe.SafeInMemoryCache[K comparable, V any]`.

Packages:
- `pkg/lru`
- `pkg/lfu`
- `pkg/tinylfu`
- `pkg/wtinylfu`
- `pkg/s3fifo`
- `pkg/twoqueue`
- `pkg/arc`
- `pkg/fifo`

Example:

```go
cache := lru.NewLRUCache[string, *User](100_000)
```

### Concurrent access

The `hot.HotCache[K, V]` offers protection against concurrent access by default. But in some cases, unnecessary locking might just slow down a program.

Low-level cache layers are not protected by default. Use the following encapsulation to bring safety:

```go
import (
	"github.com/samber/hot/pkg/lfu"
	"github.com/samber/hot/pkg/safe"
)

cache := safe.NewSafeInMemoryCache(
    lru.NewLRUCache[string, *User](100_000),
)
```

### Sharded cache

A sharded cache might be useful in two scenarios:

* highly concurrent application slowed down by cache locking -> 1 lock per shard instead of 1 global lock
* highly parallel application with no concurrency -> no lock

The sharding key must not be too costly to compute and must offer a nice balance between shards. The hashing function must have `func(k K) uint64` signature.

A sharded cache can be created via `hot.HotCache[K, V]` or using a low-level layer:

```go
import (
    "hash/fnv"
    "github.com/samber/hot/pkg/lfu"
    "github.com/samber/hot/pkg/safe"
    "github.com/samber/hot/pkg/sharded"
)

cache := sharded.NewShardedInMemoryCache(
    100, // Number of shards
    func() base.InMemoryCache[K, *item[V]] {
        // Cache builder for each shard
        return safe.NewSafeInMemoryCache(
            lru.NewLRUCache[string, *User](100_000),
        )
    },
    func(key string) uint64 {
        // Hash function
        h := fnv.New64a()
        h.Write([]byte(key))
        return h.Sum64()
    },
)
```

### Missing key caching

Instead of calling the loader chain every time an invalid key is requested, a "missing cache" can be enabled. Note that it won't protect your app against a DDoS attack with high cardinality keys.

If the missing keys are infrequent, sharing the missing cache with the main cache might be reasonable:

```go
import "github.com/samber/hot"

cache := hot.NewHotCache[string, int](hot.LRU, 100_000).
    WithMissingSharedCache().
    Build()
```

If the missing keys are frequent, use a dedicated cache to prevent pollution of the main cache:

```go
import "github.com/samber/hot"

cache := hot.NewHotCache[string, int](hot.LRU, 100_000).
    WithMissingCache(hot.LFU, 50_000).
    Build()
```

## 🪄 Examples

### Simple LRU cache

```go
import "github.com/samber/hot"

// Available eviction policies: hot.LRU, hot.LFU, hot.TinyLFU, hot.WTinyLFU, hot.S3FIFO, hot.TwoQueue, hot.ARC, hot.FIFO
// Capacity: 100k keys/values
cache := hot.NewHotCache[string, int](hot.LRU, 100_000).
    Build()

cache.Set("hello", 42)
cache.SetMany(map[string]int{"foo": 1, "bar": 2})

values, missing := cache.GetMany([]string{"bar", "baz", "hello"})
// values: {"bar": 2, "hello": 42}
// missing: ["baz"]

value, found, _ := cache.Get("foo")
// value: 1
// found: true
```

### Error Handling Patterns

```go
// Handle cache operations with proper error checking
value, found, err := cache.Get("key")
if err != nil {
    // Handle loader errors (database connection, network issues, etc.)
    log.Printf("Cache get error: %v", err)
    return
}
if !found {
    // Key doesn't exist in cache and wasn't found by loaders
    log.Printf("Key not found: %s", "key")
    return
}
// Use value safely
fmt.Printf("Value: %v", value)

// Batch operations with error handling
values, missing := cache.GetMany([]string{"key1", "key2", "key3"})
if len(missing) > 0 {
    log.Printf("Missing keys: %v", missing)
}
// Process found values
for key, value := range values {
    fmt.Printf("%s: %v", key, value)
}
```

### Cache with remote data source

If a value is not available in the in-memory cache, it will be fetched from a database or any data source.

Concurrent calls to loaders are deduplicated by key.

```go
import "github.com/samber/hot"

cache := hot.NewHotCache[string, *User](hot.LRU, 100_000).
    WithLoaders(func(keys []string) (found map[string]*User, err error) {
        rows, err := db.Query("SELECT * FROM users WHERE id IN (?)", keys)
        // ...
        return users, err
    }).
    Build()

user, found, err := cache.Get("user-123")
// might fail if "user-123" is not in cache and loader returns error

// get or create
user, found, err := cache.GetWithLoaders(
    "user-123",
    func(keys []string) (found map[string]*User, err error) {
        rows, err := db.Query("SELECT * FROM users WHERE id IN (?)", keys)
        // ...
        return users, err
    },
    func(keys []string) (found map[string]*User, err error) {
        rows, err := db.Query("INSERT INTO users (id, email) VALUES (?, ?)", id, email)
        // ...
        return users, err
    },
)
// either `err` is not nil, or `found` is true

// missing value vs nil value
user, found, err := cache.GetWithLoaders(
    "user-123",
    func(keys []string) (found map[string]*User, err error) {
        // value could not be found
        return map[string]*User{}, nil

       // or

        // value exists but is nil
        return map[string]*User{"user-123": nil}, nil
    },
)
```

### Cache with expiration

```go
import "github.com/samber/hot"

cache := hot.NewHotCache[string, int](hot.LRU, 100_000).
    WithTTL(1 * time.Minute).      // items will expire after 1 minute
    WithJitter(2, 30*time.Second). // optional: randomizes the TTL with an exponential distribution in the range [0, +30s)
    WithJanitor(1 * time.Minute).  // optional: background job will purge expired keys every minutes
    Build()

cache.SetWithTTL("foo", 42, 10*time.Second) // shorter TTL for "foo" key
```

With cache revalidation:

```go
loader := func(keys []string) (found map[string]*User, err error) {
    rows, err := db.Query("SELECT * FROM users WHERE id IN (?)", keys)
    // ...
    return users, err
}

cache := hot.NewHotCache[string, *User](hot.LRU, 100_000).
    WithTTL(1 * time.Minute).
    // Keep delivering cache 5 more second, but refresh value in background.
    // Keys that are not fetched during the interval will be dropped anyway.
    // A timeout or error in loader will drop keys.
    WithRevalidation(5 * time.Second, loader).
    // On revalidation error, the cache entries are either kept or dropped.
    // Optional (default: drop)
    WithRevalidationErrorPolicy(hot.KeepOnError).
    Build()
```

If WithRevalidation is used without loaders, the one provided in `WithRevalidation()` or `GetWithLoaders()` is used.

## 👀 Observability

HOT provides comprehensive Prometheus metrics for monitoring cache performance and behavior. Enable metrics by calling `WithPrometheusMetrics()` with a cache name:

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/samber/hot"
    "net/http"
)

// Create cache with Prometheus metrics
cache := hot.NewHotCache[string, string](hot.LRU, 1000).
    WithTTL(5*time.Minute).
    WithJitter(0.5, 10*time.Second).
    WithRevalidation(10*time.Second).
    WithRevalidationErrorPolicy(hot.KeepOnError).
    WithPrometheusMetrics("users-by-id").
    WithMissingCache(hot.ARC, 1000).
    Build()

// Register the cache metrics with Prometheus
err := prometheus.Register(cache)
if err != nil {
    log.Fatalf("Failed to register metrics: %v", err)
}
defer prometheus.Unregister(cache)

// Set up HTTP server to expose metrics
http.Handle("/metrics", promhttp.Handler())
http.ListenAndServe(":8080", nil)
```

### Available Metrics

**Counters:**
- `hot_insertion_total` - Total number of items inserted into the cache
- `hot_eviction_total{reason}` - Total number of items evicted from the cache (by reason)
- `hot_hit_total` - Total number of cache hits
- `hot_miss_total` - Total number of cache misses

**Gauges:**
- `hot_size_bytes` - Current size of the cache in bytes (including keys and values)
- `hot_length` - Current number of items in the cache

**Configuration Gauges:**
- `hot_settings_capacity` - Maximum number of items the cache can hold
- `hot_settings_algorithm` - Eviction algorithm type (0=lru, 1=lfu, 2=arc, 3=2q, 4=fifo, 5=tinylfu, 6=wtinylfu, 7=s3fifo)
- `hot_settings_ttl_seconds` - Time-to-live duration in seconds (if set)
- `hot_settings_jitter_lambda` - Jitter lambda parameter for TTL randomization (if set)
- `hot_settings_jitter_upper_bound_seconds` - Jitter upper bound duration in seconds (if set)
- `hot_settings_stale_seconds` - Stale duration in seconds (if set)
- `hot_settings_missing_capacity` - Maximum number of missing keys the cache can hold (if set)

#### Example Prometheus Queries

```promql
# Cache hit ratio
rate(hot_hit_total[5m]) / (rate(hot_hit_total[5m]) + rate(hot_miss_total[5m]))

# Eviction rate by reason
rate(hot_eviction_total[5m])

# Cache size in MB
hot_size_bytes / 1024 / 1024

# Cache utilization percentage
hot_length / hot_settings_capacity * 100

# Insertion rate
rate(hot_insertion_total[5m])
```

## 🏎️ Benchmark

TODO

## 🤝 Contributing

- Ping me on Twitter [@samuelberthe](https://twitter.com/samuelberthe) (DMs, mentions, whatever :))
- Fork the [project](https://github.com/samber/hot)
- Fix [open issues](https://github.com/samber/hot/issues) or request new features

Don't hesitate ;)

```bash
# Install some dev dependencies
make tools

# Run tests
make test
# or
make watch-test
```

## 👤 Contributors

![Contributors](https://contrib.rocks/image?repo=samber/hot)

## 💫 Show your support

Give a ⭐️ if this project helped you!

[![GitHub Sponsors](https://img.shields.io/github/sponsors/samber?style=for-the-badge)](https://github.com/sponsors/samber)

## 📝 License

Copyright © 2024 [Samuel Berthe](https://github.com/samber).

This project is [MIT](./LICENSE) licensed.
