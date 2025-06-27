# HOT Documentation

Welcome to the HOT (Hot Object Tracker) documentation. This guide will help you understand and use HOT effectively.

## Table of Contents

- [Best Practices](./best-practices.md)
- [Quick Start Guide](./quickstart.md)
- [Frequently Asked Questions](./faq.md)

## What is HOT?

HOT is a high-performance, feature-complete in-memory caching library for Go applications. It provides:

- **Multiple Eviction Policies**: LRU, LFU, ARC, 2Q, and FIFO algorithms
- **Advanced Features**: TTL with jitter, stale-while-revalidate, missing key caching
- **High Performance**: Microsecond-precision timestamps, zero-allocation operations
- **Scalability**: Sharded architecture for high concurrency
- **Observability**: Built-in Prometheus metrics
- **Type Safety**: Go generics for compile-time guarantees

## Quick Comparison

| Feature            | HOT                     | Standard Go Maps | Redis | Memcached |
| ------------------ | ----------------------- | ---------------- | ----- | --------- |
| Eviction Policies  | LRU, LFU, ARC, 2Q, FIFO | None             | LRU   | LRU       |
| TTL Support        | ✅                       | ❌                | ✅     | ✅         |
| Thread Safety      | ✅                       | ❌                | ✅     | ✅         |
| Prometheus Metrics | ✅                       | ❌                | ✅     | ✅         |
| Go Generics        | ✅                       | ✅                | ❌     | ❌         |
| In-Memory          | ✅                       | ✅                | ❌     | ❌         |
| Distributed        | ❌                       | ❌                | ✅     | ✅         |

## Getting Started

```go
import "github.com/samber/hot"

// Create a simple LRU cache
cache := hot.NewHotCache[string, int](hot.LRU, 1000).
    WithTTL(5*time.Minute).
    Build()

// Use the cache
cache.Set("key", 42)
value, found, _ := cache.Get("key")
```

## Key Concepts

### Eviction Policies
- **LRU (Least Recently Used)**: Evicts items that haven't been accessed recently
- **LFU (Least Frequently Used)**: Evicts items with the lowest access frequency
- **ARC (Adaptive Replacement Cache)**: Automatically adapts between LRU and LFU
- **2Q (Two Queue)**: Uses two queues to separate frequently and infrequently accessed items
- **FIFO (First In, First Out)**: Evicts items in the order they were added

### Cache Modes
- **Main Cache**: Stores actual data values
- **Missing Cache**: Stores negative results to prevent repeated lookups

### Performance Features
- **Sharding**: Reduces lock contention in high-concurrency scenarios
- **Batch Operations**: Efficient bulk operations for better throughput
- **Copy-on-Read/Write**: Optional value copying for thread safety

## Support

- 📖 [Documentation](https://github.com/samber/hot)
- 🐛 [Issues](https://github.com/samber/hot/issues)
- 📧 [Twitter](https://twitter.com/samuelberthe)
