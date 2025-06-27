# Concurrency Example

Demonstrates thread safety and concurrent access patterns in the HOT cache library.

## What it does

- Tests concurrent read/write operations
- Demonstrates thread safety under load
- Shows cache eviction behavior with multiple goroutines
- Compares performance of concurrent vs sequential operations

## Key Features Demonstrated

- **Thread Safety**: Safe concurrent access from multiple goroutines
- **Read-Write Contention**: Handling simultaneous reads and writes
- **Batch Concurrency**: Concurrent batch operations
- **Eviction Under Load**: Cache behavior with high concurrency
- **Performance Scaling**: How cache performs with multiple workers

## Quick Start

```bash
go run thread-safety.go
```

## Expected Output

```
🛡️ Thread Safety Example
=========================

📝 Example 1: Basic Concurrent Access
--------------------------------------
✅ Thread-safe cache created
🔄 Testing concurrent writes...
✅ Concurrent writes completed: 10 goroutines, 100 items each
⏱️ Total write time: 15.2ms
🔄 Testing concurrent reads...
✅ Concurrent reads completed
⏱️ Total read time: 8.7ms

📝 Example 2: Read-Write Contention
------------------------------------
🔄 Simulating read-write contention...
✅ Read-write contention test completed
⏱️ Total time: 25.1ms

📦 Example 3: Concurrent Batch Operations
------------------------------------------
🔄 Testing concurrent batch operations...
✅ Concurrent batch operations completed
⏱️ Total time: 12.3ms

🗑️ Example 4: Cache Eviction Under Load
----------------------------------------
🔄 Testing cache eviction under concurrent load...
✅ Eviction test completed
📊 Final cache state: 50/50 items
⏱️ Total time: 18.9ms
```

## Thread Safety Guarantees

The HOT cache library provides:
- **Atomic Operations**: All cache operations are thread-safe
- **Lock-Free Reads**: Optimized for high read concurrency
- **Minimal Lock Contention**: Efficient locking strategies
- **Consistent State**: No data races or corruption
