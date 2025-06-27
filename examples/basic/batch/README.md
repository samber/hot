# Batch Operations Example

Demonstrates performance optimization through batch operations in the HOT cache library.

## What it does

- Compares individual vs batch operation performance
- Shows how batch operations can be significantly faster
- Demonstrates batch operations with different eviction policies
- Tests various data types with batch operations

## Key Features Demonstrated

- **Performance Comparison**: Individual vs batch Set/Get operations
- **Batch SetMany**: Store multiple items at once
- **Batch GetMany**: Retrieve multiple items efficiently
- **Different Policies**: LRU, LFU, ARC, 2Q, FIFO with batch operations
- **Data Types**: Strings, integers, floats with batch operations

## Quick Start

```bash
go run batch-operations.go
```

## Expected Output

```
📦 Batch Operations Example
===========================

📝 Example 1: Individual vs Batch Set
--------------------------------------
🔄 Testing individual Set operations...
⏱️ Individual Set: 100 operations in 2.5ms
🔄 Testing batch SetMany operations...
⏱️ Batch SetMany: 100 operations in 0.8ms
🚀 Performance improvement: 3.1x faster

📝 Example 2: Individual vs Batch Get
--------------------------------------
🔄 Testing individual Get operations...
⏱️ Individual Get: 100 operations in 1.2ms
🔄 Testing batch GetMany operations...
⏱️ Batch GetMany: 100 operations in 0.4ms
🚀 Performance improvement: 3.0x faster

🔄 Example 3: Batch Operations with Different Policies
------------------------------------------------------
✅ LRU: 100/100 items in 0.8ms
✅ LFU: 100/100 items in 0.9ms
✅ ARC: 100/100 items in 1.1ms
✅ 2Q: 100/100 items in 0.7ms
✅ FIFO: 100/100 items in 0.9ms
```

## Performance Benefits

Batch operations are typically **2-4x faster** than individual operations because they:
- Reduce function call overhead
- Minimize lock contention
- Optimize memory allocations
- Improve cache locality
