# Frequently Asked Questions (FAQ)

## General Questions

### What is HOT?

HOT (Hot Object Tracker) is a high-performance, feature-complete in-memory caching library for Go applications. It provides multiple eviction policies, TTL support, sharding, Prometheus metrics, and Go generics for type safety.

### Why should I use HOT instead of a simple map?

While Go maps are fast, they lack essential caching features:
- **No eviction policies** (maps grow indefinitely)
- **No TTL support** (items never expire)
- **No thread safety** (requires external synchronization)
- **No metrics** (difficult to monitor performance)
- **No batch operations** (inefficient for multiple items)

HOT provides all these features with minimal performance overhead.

### How does HOT compare to Redis?

| Feature         | HOT                    | Redis                   |
| --------------- | ---------------------- | ----------------------- |
| **Performance** | **In-memory, fastest** | Network overhead        |
| **Deployment**  | **Embedded in app**    | Separate service        |
| **Complexity**  | **Simple setup**       | Requires infrastructure |
| **Scalability** | Single instance        | **Distributed**         |
| **Memory**      | Shared with app        | Dedicated memory        |
| **Use Case**    | Application cache      | Distributed cache       |

### Is HOT production-ready?

Yes! HOT is designed for production use with:
- Comprehensive test coverage (>80%)
- Performance benchmarks
- Prometheus metrics
- Thread safety
- Memory management
- Error handling

## Performance Questions

### How fast is HOT?

HOT is optimized for high performance:
- **Single operations**: ?ns for Set, ?ns for Get
- **Batch operations**: ?ns per item for SetMany, ?ns per item for GetMany
- **Memory overhead**: ? bytes per item
- **Concurrent**: ? ops/sec with 16 shards

### Which eviction policy should I use?

- **LRU**: General purpose, good balance of performance and hit rate
- **LFU**: Better for data with varying access patterns
- **ARC**: Automatically adapts to access patterns
- **2Q**: Good for datasets larger than cache capacity
- **FIFO**: Simple and predictable, evicts oldest items first

### How do I optimize HOT performance?

1. **Use batch operations**: `SetMany()` and `GetMany()` are much faster
2. **Choose appropriate capacity**: Cache 10-20% of your dataset
3. **Use sharding for high concurrency**: Reduces lock contention
4. **Disable unnecessary features**: `WithoutLocking()` for single-threaded apps
5. **Monitor metrics**: Use Prometheus to identify bottlenecks

### Does HOT cause memory leaks?

No, HOT has built-in memory management:
- **Automatic eviction**: Items are removed when capacity is reached
- **TTL expiration**: Items expire automatically
- **Background cleanup**: Optional janitor removes expired items
- **Memory tracking**: Built-in size monitoring

## Configuration Questions

### How do I set up TTL with jitter?

```go
cache := hot.NewHotCache[string, int](hot.LRU, 1000).
    WithTTL(5*time.Minute).
    WithJitter(0.1, 30*time.Second).  // ±30s random jitter
    Build()
```

Jitter prevents cache stampedes by randomizing expiration times.

### What is stale-while-revalidate?

Stale-while-revalidate keeps serving expired data while refreshing it in the background:

```go
cache := hot.NewHotCache[string, *User](hot.LRU, 1000).
    WithTTL(5*time.Minute).
    WithRevalidation(30*time.Second, userLoader).  // Keep serving stale for 30s
    Build()
```

This improves response times and reduces database load.

### How do I cache missing keys?

Missing key caching prevents repeated lookups for non-existent keys:

```go
// Separate cache for missing keys
cache := hot.NewHotCache[string, *User](hot.LRU, 1000).
    WithMissingCache(hot.LFU, 100).
    Build()

// Share with main cache
cache := hot.NewHotCache[string, *User](hot.LRU, 1000).
    WithMissingSharedCache().
    Build()
```

### How do I set up sharding?

```go
cache := hot.NewHotCache[string, *Data](hot.LRU, 10000).
    WithSharding(16, func(key string) uint64 {
        h := fnv.New64a()
        h.Write([]byte(key))
        return h.Sum64()
    }).
    Build()
```

Use 2^n shards (4, 8, 16, 32) for best performance.

## Monitoring Questions

### How do I enable Prometheus metrics?

```go
cache := hot.NewHotCache[string, *Data](hot.LRU, 1000).
    WithPrometheusMetrics("my-cache").
    Build()

// Register with Prometheus
prometheus.MustRegister(cache)
```

### What metrics are available?

- **Counters**: `hot_insertion_total`, `hot_eviction_total`, `hot_hit_total`, `hot_miss_total`
- **Gauges**: `hot_size_bytes`, `hot_length`
- **Configuration**: `hot_settings_capacity`, `hot_settings_algorithm`, etc.

### How do I monitor cache performance?

Key metrics to watch:
- **Hit ratio**: `rate(hot_hit_total[5m]) / (rate(hot_hit_total[5m]) + rate(hot_miss_total[5m]))`
- **Eviction rate**: `rate(hot_eviction_total[5m])`
- **Cache utilization**: `hot_length / hot_settings_capacity`

## Integration Questions

### How do I integrate with a database?

```go
cache := hot.NewHotCache[string, *User](hot.LRU, 1000).
    WithLoaders(func(keys []string) (found map[string]*User, err error) {
        // Query database for missing keys
        found = make(map[string]*User)
        for _, key := range keys {
            user, err := db.GetUser(key)
            if err == nil {
                found[key] = user
            }
        }
        return found, nil
    }).
    Build()
```

### How do I handle database errors?

```go
cache := hot.NewHotCache[string, *User](hot.LRU, 1000).
    WithLoaders(func(keys []string) (found map[string]*User, err error) {
        // Implement retry logic
        for retries := 0; retries < 3; retries++ {
            found, err = loadFromDatabase(keys)
            if err == nil {
                return found, nil
            }
            time.Sleep(time.Duration(retries+1) * 100 * time.Millisecond)
        }
        return nil, fmt.Errorf("failed after 3 retries: %w", err)
    }).
    Build()
```

### How do I warm up the cache?

```go
cache := hot.NewHotCache[string, *User](hot.LRU, 1000).
    WithWarmUp(func() (map[string]*User, []string, error) {
        // Load frequently accessed data
        return loadPopularUsers(), nil, nil
    }).
    Build()
```

## Troubleshooting Questions

### Why is my cache hit ratio low?

Common causes:
1. **Cache too small**: Increase capacity
2. **TTL too short**: Extend expiration time
3. **Poor key distribution**: Check your hashing function
4. **Too many unique keys**: Consider key normalization

### Why is memory usage high?

Check these settings:
1. **Cache capacity**: Reduce if too large
2. **TTL**: Shorten if items expire too slowly
3. **Missing cache**: Disable if not needed
4. **Sharding**: Reduce number of shards

### How do I debug cache issues?

1. **Enable Prometheus metrics** to monitor performance
2. **Use eviction callbacks** to track item removal
3. **Monitor memory usage** with `hot_size_bytes`
4. **Check cache statistics** with `hot_length` and capacity

### What if my loader is slow?

1. **Use stale-while-revalidate** to serve stale data
2. **Implement timeouts** in your loader
3. **Add retry logic** with exponential backoff
4. **Consider caching at multiple levels**

## Migration Questions

### How do I migrate from a simple map?

```go
// Before
var cache = make(map[string]*User)

// After
cache := hot.NewHotCache[string, *User](hot.LRU, 1000).
    WithTTL(5*time.Minute).
    Build()

// Replace operations
cache["key"] = value  // -> cache.Set("key", value)
value := cache["key"] // -> value, found, _ := cache.Get("key")
delete(cache, "key")  // -> cache.Delete("key")
```

### How do I migrate from Redis?

1. **Replace Redis client** with HOT cache
2. **Update key patterns** if needed
3. **Configure TTL** to match Redis expiration
4. **Add Prometheus metrics** for monitoring
5. **Test performance** in your environment

### How do I migrate from another Go cache library?

Most Go cache libraries have similar APIs. Common replacements:
- `bigcache` → HOT with `WithoutLocking()`
- `freecache` → HOT with appropriate capacity
- `gcache` → HOT with similar configuration

## Advanced Questions

### Can I use HOT in a distributed system?

HOT is designed for single-instance caching. For distributed systems:
- Use HOT for local caching
- Combine with Redis/Memcached for shared state
- Implement cache invalidation between instances

### How do I implement cache invalidation?

```go
// Manual invalidation
cache.Delete("key")
cache.DeleteMany([]string{"key1", "key2"})

// Pattern-based invalidation
cache.Range(func(key string, value *User) bool {
    if strings.HasPrefix(key, "user:") {
        cache.Delete(key)
    }
    return true
})
```

### Can I use HOT with custom data types?

Yes! HOT uses Go generics and works with any comparable key type and any value type:

```go
// Custom structs
type User struct { ID string; Name string }
cache := hot.NewHotCache[string, *User](hot.LRU, 1000)

// Custom key types
type UserID struct { ID string }
cache := hot.NewHotCache[UserID, *User](hot.LRU, 1000)
```

### How do I implement cache persistence?

HOT doesn't provide built-in persistence, but you can implement it:

```go
// Save cache on shutdown
func saveCache(cache *hot.HotCache[string, *User]) {
    items := make(map[string]*User)
    cache.Range(func(key string, value *User) bool {
        items[key] = value
        return true
    })
    saveToFile(items)
}

// Load cache on startup
func loadCache(cache *hot.HotCache[string, *User]) {
    items := loadFromFile()
    cache.SetMany(items)
}
```

## Support Questions

### Where can I get help?

- 📖 **Documentation**: [README.md](../README.md)
- 🐛 **Issues**: [GitHub Issues](https://github.com/samber/hot/issues)
- 📧 **Twitter**: https://twitter.com/samuelberthe

### How do I contribute?

1. **Fork the repository**
2. **Create a feature branch**
3. **Make your changes**
4. **Add tests and benchmarks**
5. **Submit a pull request**

### Is HOT actively maintained?

Yes! HOT is actively maintained with:
- Regular updates and bug fixes
- Performance improvements
- New features and algorithms
- Community contributions welcome
