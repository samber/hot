# Simple LRU Cache Example

A basic example demonstrating the core features of the HOT cache library.

## What it does

- Creates a simple LRU cache with 1000 items capacity
- Shows basic Set/Get operations
- Demonstrates batch operations for better performance
- Displays cache statistics and iteration

## Key Features Demonstrated

- **LRU Eviction**: Least Recently Used items are evicted when cache is full
- **TTL Support**: Items expire after 5 minutes
- **Batch Operations**: More efficient than individual operations
- **Cache Statistics**: Monitor cache utilization and performance

## Quick Start

```bash
go run simple.go
```

## Expected Output

```
🚀 HOT Simple Cache Example
============================
✅ Cache created successfully
📊 Cache capacity: 1000 items

📝 Example 1: Basic Set and Get
--------------------------------
✅ Stored user: Alice Johnson (user:1)
✅ Stored user: Bob Smith (user:2)
✅ Stored user: Carol Davis (user:3)
✅ Retrieved user: Alice Johnson (user:1)
✅ Retrieved user: Bob Smith (user:2)
✅ Retrieved user: Carol Davis (user:3)

📦 Example 2: Batch Operations
-------------------------------
✅ Stored 3 users in batch
✅ Retrieved 3 users
❌ Missing 1 users: [user:7]
   - user:4: David Wilson (david@example.com)
   - user:5: Eve Brown (eve@example.com)
   - user:6: Frank Miller (frank@example.com)
```
