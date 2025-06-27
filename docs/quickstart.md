# Quick Start Guide

This guide will help you get started with HOT in under 5 minutes.

## Installation

```bash
go get github.com/samber/hot
```

## Basic Usage

### Simple LRU Cache

```go
package main

import (
    "fmt"
    "time"
    "github.com/samber/hot"
)

func main() {
    // Create a simple LRU cache with 1000 items capacity
    cache := hot.NewHotCache[string, int](hot.LRU, 1000).
        WithTTL(5*time.Minute).
        Build()

    // Store a value
    cache.Set("user:123", 42)

    // Retrieve a value
    value, found, err := cache.Get("user:123")
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    if found {
        fmt.Printf("Found: %d\n", value)
    } else {
        fmt.Println("Not found")
    }
}
```

### Cache with Database Integration

```go
package main

import (
    "database/sql"
    "fmt"
    "time"
    "github.com/samber/hot"
)

type User struct {
    ID   string
    Name string
    Email string
}

func main() {
    db, _ := sql.Open("postgres", "connection_string")

    // Create cache with database loader
    cache := hot.NewHotCache[string, *User](hot.LRU, 1000).
        WithTTL(10*time.Minute).
        WithLoaders(func(keys []string) (found map[string]*User, err error) {
            // Query database for missing keys
            found = make(map[string]*User)
            for _, key := range keys {
                user := &User{}
                err := db.QueryRow("SELECT id, name, email FROM users WHERE id = $1", key).
                    Scan(&user.ID, &user.Name, &user.Email)
                if err == nil {
                    found[key] = user
                }
            }
            return found, nil
        }).
        Build()

    // Get user - will automatically load from database if not in cache
    user, found, err := cache.Get("user:123")
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    if found {
        fmt.Printf("User: %s (%s)\n", user.Name, user.Email)
    }
}
```

### High-Performance Cache with Sharding

```go
package main

import (
    "hash/fnv"
    "time"
    "github.com/samber/hot"
)

func main() {
    // Create sharded cache for high concurrency
    cache := hot.NewHotCache[string, []byte](hot.LRU, 10000).
        WithTTL(1*time.Hour).
        WithSharding(16, func(key string) uint64 {
            h := fnv.New64a()
            h.Write([]byte(key))
            return h.Sum64()
        }).
        WithPrometheusMetrics("api-cache").
        Build()

    // Use batch operations for better performance
    cache.SetMany(map[string][]byte{
        "key1": []byte("value1"),
        "key2": []byte("value2"),
        "key3": []byte("value3"),
    })

    values, missing := cache.GetMany([]string{"key1", "key2", "key4"})
    fmt.Printf("Found: %d items, Missing: %d items\n", len(values), len(missing))
}
```

## Common Patterns

### Cache Warmup

```go
// Preload frequently accessed data
cache := hot.NewHotCache[string, *User](hot.LRU, 1000).
    WithWarmUp(func() (map[string]*User, []string, error) {
        // Load popular users from database
        users := map[string]*User{
            "user:admin": {ID: "admin", Name: "Administrator"},
            "user:guest": {ID: "guest", Name: "Guest User"},
        }
        return users, nil, nil
    }).
    Build()
```

### Stale-While-Revalidate

```go
// Keep serving stale data while refreshing in background
cache := hot.NewHotCache[string, *User](hot.LRU, 1000).
    WithTTL(5*time.Minute).
    WithRevalidation(30*time.Second, func(keys []string) (found map[string]*User, err error) {
        // Refresh data from database
        return loadUsersFromDB(keys)
    }).
    Build()
```

### Missing Key Caching

```go
// Prevent repeated lookups for non-existent keys
cache := hot.NewHotCache[string, *User](hot.LRU, 1000).
    WithMissingCache(hot.LFU, 500).  // Separate cache for missing keys
    WithLoaders(userLoader).
    Build()
```

## Next Steps

- Read the [API Reference](./api-reference.md) for detailed method documentation
- Check out [Best Practices](./best-practices.md) for production recommendations
- Explore [Performance Guide](./performance.md) for optimization tips
- See [Examples](../example/) for more use cases 