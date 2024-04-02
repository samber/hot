
# HOT - In-memory caching

[![tag](https://img.shields.io/github/tag/samber/hot.svg)](https://github.com/samber/hot/releases)
![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.21-%23007d9c)
[![GoDoc](https://godoc.org/github.com/samber/hot?status.svg)](https://pkg.go.dev/github.com/samber/hot)
![Build Status](https://github.com/samber/hot/actions/workflows/test.yml/badge.svg)
[![Go report](https://goreportcard.com/badge/github.com/samber/hot)](https://goreportcard.com/report/github.com/samber/hot)
[![Coverage](https://img.shields.io/codecov/c/github/samber/hot)](https://codecov.io/gh/samber/hot)
[![Contributors](https://img.shields.io/github/contributors/samber/hot)](https://github.com/samber/hot/graphs/contributors)
[![License](https://img.shields.io/github/license/samber/hot)](./LICENSE)

**H**OT **O**bject **T**racker.

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

No breaking changes will be made to exported APIs before v1.0.0.

## 🤠 Getting started

[GoDoc: https://godoc.org/github.com/samber/hot](https://godoc.org/github.com/samber/hot)

// TODO

## 🏛️ Architecture

This project has been split into multiple layers to respect the separation of concern.

Each cache layer implements the `pkg/base.InMemoryCache[K, V]` interface. Chaining multiple encapsulation has a minimal cost (~1ns per call), but is highly customizable.

This is highly recommended to use `hot.HotCache[K, V]` instead of lower layers.

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

Copyright © 2023 [Samuel Berthe](https://github.com/samber).

This project is [MIT](./LICENSE) licensed.
