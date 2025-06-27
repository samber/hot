# HOT Performance Benchmarks

This directory contains comprehensive benchmarks for HOT cache performance across different scenarios and configurations.

## Quick Performance Overview

HOT is designed for high-performance scenarios with the following characteristics:

- **Microsecond-precision timestamps** (2.3x faster than `time.Now()`)
- **Zero-allocation operations** where possible
- **Lock-free operations** when thread safety is disabled
- **Batch operations** for better throughput
- **Sharded architecture** for high concurrency

## Benchmark Categories

### 1. Basic Operations
- **Set/Get Performance**: Single key-value operations
- **Batch Operations**: Multiple key-value operations
- **Memory Usage**: Memory footprint analysis
- **Concurrent Access**: Multi-threaded performance

### 2. Eviction Policies
- **LRU Performance**: Least Recently Used algorithm
- **LFU Performance**: Least Frequently Used algorithm
- **ARC Performance**: Adaptive Replacement Cache
- **2Q Performance**: Two Queue algorithm
- **FIFO Performance**: First In, First Out algorithm

### 3. Advanced Features
- **TTL Performance**: Time-to-live overhead
- **Sharding Performance**: Multi-shard scalability
- **Metrics Overhead**: Prometheus metrics impact
- **Loader Performance**: Database integration overhead

### 4. Comparison Benchmarks
- **vs Standard Maps**: Go built-in map performance
- **vs Redis**: Redis performance comparison
- **vs Other Go Caches**: Popular Go caching libraries

## Running Benchmarks

### Prerequisites

```bash
# Install benchmark dependencies
go get golang.org/x/perf/cmd/benchstat
go get github.com/cespare/prettybench
```

### Basic Benchmark

```bash
# Run all benchmarks
make bench
```

### Performance Analysis

```bash
# Compare benchmark results
benchstat old.txt new.txt

# Pretty print benchmark results
prettybench -benchmem ./...
```

## Expected Performance

### Single Operations
- **Set**: ? per operation
- **Get**: ? per operation
- **Delete**: ? per operation

### Batch Operations
- **SetMany**: ? per item
- **GetMany**: ? per item
- **DeleteMany**: ? per item

### Memory Overhead
- **Per Item**: ? bytes overhead
- **Cache Structure**: ? base overhead
- **Sharding**: ? per shard

### Concurrent Performance
- **Single-threaded**: ? ops/sec
- **Multi-threaded (4 cores)**: ? ops/sec
- **Sharded (16 shards)**: ? ops/sec

## Performance Tips

### 1. Choose the Right Algorithm
```go
// For general use
cache := hot.NewHotCache[string, int](hot.LRU, 1000)

// For frequently accessed data
cache := hot.NewHotCache[string, int](hot.LFU, 1000)

// For mixed access patterns
cache := hot.NewHotCache[string, int](hot.ARC, 1000)

// For simple, predictable eviction
cache := hot.NewHotCache[string, int](hot.FIFO, 1000)
```

### 2. Use Batch Operations
```go
// ❌ Slow: Individual operations
for _, key := range keys {
    cache.Set(key, value)
}

// ✅ Fast: Batch operations
cache.SetMany(items)
```

### 3. Disable Unnecessary Features
```go
// For maximum performance
cache := hot.NewHotCache[string, int](hot.LRU, 1_000).
    WithoutLocking().  // Single-threaded only
    Build()
```

### 4. Use Appropriate Sharding
```go
// For high concurrency
cache := hot.NewHotCache[string, int](hot.LRU, 1_000).
    WithSharding(16, hasher).
    Build()
```

## Benchmark Results

### Latest Results (Go 1.23, macOS ARM64)

```
// TODO
```

### Performance Comparison

```
// TODO
```

## Contributing to Benchmarks

When adding new features, please include appropriate benchmarks:

1. **Add benchmark functions** in the relevant package
2. **Update this README** with new performance data
3. **Run benchmarks** before and after changes
4. **Document performance impact** in pull requests

### Benchmark Template

```go
func BenchmarkNewFeature(b *testing.B) {
    cache := hot.NewHotCache[string, int](hot.LRU, 1000).
        WithNewFeature().
        Build()
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            key := fmt.Sprintf("key:%d", i%1000)
            cache.Set(key, i)
            cache.Get(key)
            i++
        }
    })
}
```

## Performance Profiling

### CPU Profiling

```bash
# Generate CPU profile
go test -bench=BenchmarkLRU -cpuprofile=cpu.prof ./pkg/lru/

# Analyze with pprof
go tool pprof cpu.prof
```

### Memory Profiling

```bash
# Generate memory profile
go test -bench=BenchmarkLRU -memprofile=mem.prof ./pkg/lru/

# Analyze with pprof
go tool pprof mem.prof
```

### Flame Graph

```bash
# Generate flame graph
go test -bench=BenchmarkLRU -cpuprofile=cpu.prof ./pkg/lru/
go tool pprof -http=:8080 cpu.prof
```

## Performance Tuning

### Environment Variables

```bash
# Disable CPU frequency scaling
export GOMAXPROCS=1
export GOGC=off

# Run benchmarks
make bench
```

### System Tuning

```bash
# Linux: Disable CPU frequency scaling
echo performance | sudo tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor

# macOS: Disable App Nap
defaults write NSGlobalDomain NSAppSleepDisabled -bool true
```

## Contact

For performance-related questions or issues:

- 📖 Documentation: https://github.com/samber/hot
- 📧 Twitter: https://twitter.com/samuelberthe
- 🐛 Issues: https://github.com/samber/hot/issues
