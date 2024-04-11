
# HOT - In-memory caching

[![tag](https://img.shields.io/github/tag/samber/hot.svg)](https://github.com/samber/hot/releases)
![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.22-%23007d9c)
[![GoDoc](https://godoc.org/github.com/samber/hot?status.svg)](https://pkg.go.dev/github.com/samber/hot)
![Build Status](https://github.com/samber/hot/actions/workflows/test.yml/badge.svg)
[![Go report](https://goreportcard.com/badge/github.com/samber/hot)](https://goreportcard.com/report/github.com/samber/hot)
[![Coverage](https://img.shields.io/codecov/c/github/samber/hot)](https://codecov.io/gh/samber/hot)
[![Contributors](https://img.shields.io/github/contributors/samber/hot)](https://github.com/samber/hot/graphs/contributors)
[![License](https://img.shields.io/github/license/samber/hot)](./LICENSE)

**H**ot **O**bject **T**racker.

A feature-complete and [blazing-fast](#🏎️-benchmark) caching library for Go.

## 💡 Features

- 🚀 Fast, concurrent
- 💫 Generics
- 🗑️ Eviction policies: LRU, LFU, 2Q
- ⏰ TTL with jitter
- 🔄 Stale while revalidation
- ❌ Missing key caching
- 🍕 Sharded cache
- 🔒 Optional locking
- 🔗 Chain of data loaders with in-flight deduplication
- 🌶️ Cache warmup
- 📦 Batching all the way
- 🧩 Composable caching strategy
- 📝 Optional copy on read and/or write
- 📊 Stat collection

## 🚀 Install

```sh
go get github.com/samber/hot
```

This library is v0 and follows SemVer strictly.

Some breaking changes might be made to exported APIs before v1.0.0.

## 🤠 Getting started

[GoDoc: https://godoc.org/github.com/samber/hot](https://godoc.org/github.com/samber/hot)

### Simple LRU cache

```go
import "github.com/samber/hot"

// Available eviction policies: hot.LRU, hot.LFU, hot.TwoQueue, hot.ARC
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
```

### Cache with expiration

```go
import "github.com/samber/hot"

cache := hot.NewHotCache[string, int](hot.LRU, 100_000).
    WithTTL(1 * time.Minute).     // items will expire after 1 minute
    WithJitter(0.2).              // optional: a 20% variation in cache expiration duration (48s to 72s)
    WithJanitor(1 * time.Minute). // optional: background job will purge expired keys every minutes
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

## 🍱 Spec

```go
hot.NewHotCache[K, V](algorithm hot.EvictionAlgorithm, capacity int).
    // Enables cache of missing keys. The missing cache is shared with the main cache.
    WithMissingSharedCache().
    // Enables cache of missing keys. The missing keys are stored in a separate cache.
    WithMissingCache(algorithm hot.EvictionAlgorithm, capacity int).
    // Sets the time-to-live for cache entries
    WithTTL(ttl time.Duration).
    // Sets the time after which the cache entry is considered stale and needs to be revalidated
    // * keys that are not fetched during the interval will be dropped anyway
    // * a timeout or error in loader will drop keys.
    // If no revalidation loader is added, the default loaders or the one used in GetWithLoaders() are used.
    WithRevalidation(stale time.Duration, loaders ...hot.Loader[K, V]).
    // Sets the policy to apply when a revalidation loader returns an error.
    // By default, the key is dropped from the cache.
    WithRevalidationErrorPolicy(policy revalidationErrorPolicy).
    // Randomizes the TTL.
    WithJitter(jitter float64).
    // Enables cache sharding.
    WithSharding(nbr uint64, fn sharded.Hasher[K]).
    // Preloads the cache with the provided data.
    WithWarmUp(fn func() (map[K]V, []K, error)).
    // Disables mutex for the cache and improves internal performances.
    WithoutLocking().
    // Enables the cache janitor.
    WithJanitor().
    // Sets the chain of loaders to use for cache misses.
    WithLoaders(loaders ...hot.Loader[K, V]).
    // Sets the callback to be called when an entry is evicted from the cache.
    // The callback is called synchronously and might block the cache operations if it is slow.
    // This implementation choice is subject to change. Please open an issue to discuss.
    WithEvictionCallback(hook func(key K, value V)).
    // Sets the function to copy the value on read.
    WithCopyOnRead(copyOnRead func(V) V).
    // Sets the function to copy the value on write.
    WithCopyOnWrite(copyOnWrite func(V) V).
    // Returns a HotCache[K, V].
    Build()
```

Available eviction algorithm:

```go
hot.LRU
hot.LFU
hot.TwoQueue
hot.ARC
```

Loaders:

```go
func loader(keys []K) (found map[K]V, err error) {
    // ...
}
```

Shard partitioner:

```go
func hash(key K) uint64 {
    // ...
}
```

## 🏛️ Architecture

This project has been split into multiple layers to respect the separation of concern.

Each cache layer implements the `pkg/base.InMemoryCache[K, V]` interface. Combining multiple encapsulation has a small cost (~1ns per call), but offers great customization.

We highly recommend using `hot.HotCache[K, V]` instead of lower layers.

### Eviction policies

This project provides multiple eviction policies. Each implements the `pkg/base.InMemoryCache[K, V]` interface.

They are not protected against concurrent access. If safety is required, encapsulate it into `pkg/safe.SafeInMemoryCache[K comparable, V any]`.

Packages:
- `pkg/lru`
- `pkg/lfu`
- `pkg/twoqueue`
- `pkg/arc`

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
    1_000, // number of shards
    func() base.InMemoryCache[K, *item[V]] {
        return safe.NewSafeInMemoryCache(
            lru.NewLRUCache[string, *User](100_000),
        )
    },
    func(key string) uint64 {
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

## 🏎️ Benchmark

// TODO: copy here the benchmarks of bench/ directory

// - compare libraries

// - measure encapsulation cost

// - measure lock cost

// - measure ttl cost

// - measure size.Of cost

// - measure stats collection cost

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
