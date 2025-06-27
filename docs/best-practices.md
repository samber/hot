# Best Practices

This guide covers best practices for using HOT in production environments.

## Cache Configuration

### Choose the Right Eviction Policy

| Use Case                 | Recommended Policy | Reason                                        |
| ------------------------ | ------------------ | --------------------------------------------- |
| General purpose          | LRU                | Good balance of performance and hit rate      |
| Frequently accessed data | LFU                | Better for data with varying access patterns  |
| Mixed access patterns    | ARC                | Automatically adapts to access patterns       |
| Large datasets           | 2Q                 | Good for datasets larger than cache capacity  |
| Simple, predictable      | FIFO               | Evicts oldest items first, easy to understand |

### Set Appropriate TTL Values

```go
// Good: Short TTL for frequently changing data
cache := hot.NewHotCache[string, *User](hot.LRU, 100_000).
    WithTTL(5*time.Minute).
    Build()

// Good: Longer TTL for stable data
cache := hot.NewHotCache[string, *Config](hot.LRU, 100_000).
    WithTTL(1*time.Hour).
    Build()

// Good: Use jitter to prevent cache stampedes
cache := hot.NewHotCache[string, *Data](hot.LRU, 100_000).
    WithTTL(10*time.Minute).
    WithJitter(0.1, 30*time.Second).  // ±30s jitter
    Build()
```

### Size Your Cache Appropriately

```go
// Rule of thumb: Cache should hold 10-20% of your dataset
// For 1M users, cache 100k-200k user records
cache := hot.NewHotCache[string, *User](hot.LRU, 150_000).
    WithTTL(15*time.Minute).
    Build()
```

## Performance Optimization

### Use Batch Operations

```go
// ❌ Bad: Multiple individual operations
for _, key := range keys {
    cache.Set(key, value)
}

// ✅ Good: Batch operations
items := make(map[string]Value)
for _, key := range keys {
    items[key] = value
}
cache.SetMany(items)
```

### Implement Proper Error Handling

```go
// ✅ Good: Handle loader errors gracefully
cache := hot.NewHotCache[string, *User](hot.LRU, 100_000).
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

### Use Sharding for High Concurrency

```go
// ✅ Good: Shard cache for high concurrency
cache := hot.NewHotCache[string, *Data](hot.LRU, 100_000).
    WithSharding(32, func(key string) uint64 {
        h := fnv.New64a()
        h.Write([]byte(key))
        return h.Sum64()
    }).
    Build()
```

## Monitoring and Observability

### Enable Prometheus Metrics

```go
// ✅ Good: Always enable metrics in production
cache := hot.NewHotCache[string, *Data](hot.LRU, 100_000).
    WithPrometheusMetrics("user-cache").
    Build()

// Register with Prometheus
prometheus.MustRegister(cache)
```

### Monitor Key Metrics

```promql
# Cache hit ratio (should be > 80% for good performance)
rate(hot_hit_total[5m]) / (rate(hot_hit_total[5m]) + rate(hot_miss_total[5m]))

# Eviction rate (high rates indicate undersized cache)
rate(hot_eviction_total[5m])

# Cache utilization
hot_length / hot_settings_capacity
```

## Database Integration

### Implement Efficient Loaders

```go
// ✅ Good: Batch database queries
func userLoader(keys []string) (found map[string]*User, err error) {
    if len(keys) == 0 {
        return make(map[string]*User), nil
    }
    
    // Use IN clause for batch queries
    placeholders := make([]string, len(keys))
    args := make([]interface{}, len(keys))
    for i, key := range keys {
        placeholders[i] = fmt.Sprintf("$%d", i+1)
        args[i] = key
    }
    
    query := fmt.Sprintf("SELECT id, name, email FROM users WHERE id IN (%s)", 
        strings.Join(placeholders, ","))
    
    rows, err := db.Query(query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    found = make(map[string]*User)
    for rows.Next() {
        user := &User{}
        err := rows.Scan(&user.ID, &user.Name, &user.Email)
        if err != nil {
            return nil, err
        }
        found[user.ID] = user
    }
    
    return found, nil
}
```

### Handle Database Failures

```go
// ✅ Good: Graceful degradation
cache := hot.NewHotCache[string, *User](hot.LRU, 100_000).
    WithLoaders(
        func(keys []string) (found map[string]*User, err error) {
            // Try primary database
            return primaryDB.LoadUsers(keys)
        },
        func(keys []string) (found map[string]*User, err error) {
            // Fallback to secondary database
            log.Printf("Primary DB failed, trying secondary: %v", err)
            return secondaryDB.LoadUsers(keys)
        },
    ).
    Build()
```

## Memory Management

### Use Copy-on-Write for Mutable Data

```go
// ✅ Good: Prevent external modifications
cache := hot.NewHotCache[string, *Config](hot.LRU, 100_000).
    WithCopyOnRead(func(cfg *Config) *Config {
        // Deep copy the config
        return &Config{
            Settings: cfg.Settings.Clone(),
            Metadata: cfg.Metadata.Clone(),
        }
    }).
    Build()
```

### Monitor Memory Usage

```go
// ✅ Good: Track cache size
go func() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        current, max := cache.Capacity()
        sizeBytes := cache.SizeBytes()
        
        log.Printf("Cache: %d/%d items (%.2f MB)", 
            current, max, float64(sizeBytes)/1024/1024)
    }
}()
```

## Security Considerations

### Use Appropriate TTL for Sensitive Data

```go
// ✅ Good: Short TTL for sensitive data
cache := hot.NewHotCache[string, *Session](hot.LRU, 100_000).
    WithTTL(2*time.Minute).  // Short TTL for sessions
    Build()
```

## Testing

### Unit Test Cache Behavior

```go
func TestUserCache(t *testing.T) {
    cache := hot.NewHotCache[string, *User](hot.LRU, 100_000).
        WithTTL(1*time.Minute).
        WithLoaders(mockUserLoader).
        Build()
    
    // Test cache miss
    user, found, err := cache.Get("user:123")
    assert.NoError(t, err)
    assert.True(t, found)
    assert.Equal(t, "user:123", user.ID)
    
    // Test cache hit
    user2, found, err := cache.Get("user:123")
    assert.NoError(t, err)
    assert.True(t, found)
    assert.Equal(t, user, user2)  // Should be same instance
}
```

### Load Test Cache Performance

```go
func BenchmarkCachePerformance(b *testing.B) {
    cache := hot.NewHotCache[string, *Data](hot.LRU, 10000).
        WithTTL(10*time.Minute).
        Build()
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            key := fmt.Sprintf("key:%d", i%1000)
            cache.Set(key, &Data{Value: i})
            cache.Get(key)
            i++
        }
    })
}
```

## Common Pitfalls

### ❌ Don't Cache Everything

```go
// ❌ Bad: Caching everything
cache.Set("user:123:profile", userProfile)
cache.Set("user:123:settings", userSettings)
cache.Set("user:123:preferences", userPreferences)
cache.Set("user:123:history", userHistory)

// ✅ Good: Cache only frequently accessed data
if isFrequentlyAccessed(userID) {
    cache.Set("user:"+userID, userData)
}
```

### ❌ Don't Ignore Cache Misses

```go
// ❌ Bad: No error handling
value, found, _ := cache.Get("key")

// ✅ Good: Handle errors properly
value, found, err := cache.Get("key")
if err != nil {
    log.Printf("Cache error: %v", err)
    // Fallback to database or return error
    return nil, err
}
```

### ❌ Don't Use Too Many Shards

```go
// ❌ Bad: Too many shards (overhead)
cache.WithSharding(1000, hasher)

// ✅ Good: Reasonable number of shards
cache.WithSharding(16, hasher)  // 2^4 shards
``` 