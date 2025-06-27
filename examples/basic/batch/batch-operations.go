package main

import (
	"fmt"
	"time"

	"github.com/samber/hot"
)

func main() {
	fmt.Println("📦 Batch Operations Example")
	fmt.Println("===========================")

	// Example 1: Individual vs Batch Set operations
	fmt.Println("\n📝 Example 1: Individual vs Batch Set")
	fmt.Println("--------------------------------------")

	cache := hot.NewHotCache[string, int](hot.LRU, 1000).
		WithTTL(5 * time.Minute).
		Build()

	// Test individual operations
	fmt.Println("🔄 Testing individual Set operations...")
	start := time.Now()
	for i := 0; i < 100; i++ {
		cache.Set(fmt.Sprintf("indiv:%d", i), i)
	}
	individualDuration := time.Since(start)
	fmt.Printf("⏱️ Individual Set: 100 operations in %v\n", individualDuration)

	// Test batch operations
	fmt.Println("🔄 Testing batch SetMany operations...")
	start = time.Now()
	batch := make(map[string]int)
	for i := 0; i < 100; i++ {
		batch[fmt.Sprintf("batch:%d", i)] = i
	}
	cache.SetMany(batch)
	batchDuration := time.Since(start)
	fmt.Printf("⏱️ Batch SetMany: 100 operations in %v\n", batchDuration)

	improvement := float64(individualDuration) / float64(batchDuration)
	fmt.Printf("🚀 Performance improvement: %.1fx faster\n", improvement)

	// Example 2: Individual vs Batch Get operations
	fmt.Println("\n📝 Example 2: Individual vs Batch Get")
	fmt.Println("--------------------------------------")

	// Test individual Get operations
	fmt.Println("🔄 Testing individual Get operations...")
	start = time.Now()
	for i := 0; i < 100; i++ {
		cache.Get(fmt.Sprintf("batch:%d", i))
	}
	individualGetDuration := time.Since(start)
	fmt.Printf("⏱️ Individual Get: 100 operations in %v\n", individualGetDuration)

	// Test batch Get operations
	fmt.Println("🔄 Testing batch GetMany operations...")
	start = time.Now()
	keys := make([]string, 100)
	for i := 0; i < 100; i++ {
		keys[i] = fmt.Sprintf("batch:%d", i)
	}
	found, missing, _ := cache.GetMany(keys)
	batchGetDuration := time.Since(start)
	fmt.Printf("⏱️ Batch GetMany: 100 operations in %v\n", batchGetDuration)
	fmt.Printf("📊 Found: %d items, Missing: %d items\n", len(found), len(missing))

	getImprovement := float64(individualGetDuration) / float64(batchGetDuration)
	fmt.Printf("🚀 Performance improvement: %.1fx faster\n", getImprovement)

	// Example 3: Batch operations with different eviction policies
	fmt.Println("\n🔄 Example 3: Batch Operations with Different Policies")
	fmt.Println("------------------------------------------------------")

	policies := []struct {
		name   string
		policy hot.EvictionAlgorithm
	}{
		{"LRU", hot.LRU},
		{"LFU", hot.LFU},
		{"ARC", hot.ARC},
		{"2Q", hot.TwoQueue},
	}

	for _, p := range policies {
		cache := hot.NewHotCache[string, int](p.policy, 100).
			WithTTL(1 * time.Minute).
			Build()

		batch := make(map[string]int)
		for i := 0; i < 150; i++ {
			batch[fmt.Sprintf("test:%d", i)] = i
		}

		start := time.Now()
		cache.SetMany(batch)
		duration := time.Since(start)

		current, max := cache.Capacity()
		fmt.Printf("✅ %s: %d/%d items in %v\n",
			p.name, current, max, duration)
	}

	// Example 4: Batch operations with different data types
	fmt.Println("\n🛒 Example 4: Mixed Data Types")
	fmt.Println("-------------------------------")

	stringCache := hot.NewHotCache[string, string](hot.LRU, 100).Build()
	intCache := hot.NewHotCache[string, int](hot.LRU, 100).Build()

	// Batch operations with strings
	stringBatch := map[string]string{
		"name": "John Doe",
		"city": "New York",
		"job":  "Developer",
	}
	start = time.Now()
	stringCache.SetMany(stringBatch)
	stringDuration := time.Since(start)
	fmt.Printf("⏱️ String batch: %d items in %v\n", len(stringBatch), stringDuration)

	// Batch operations with integers
	intBatch := make(map[string]int)
	for i := 0; i < 50; i++ {
		intBatch[fmt.Sprintf("num:%d", i)] = i * 10
	}
	start = time.Now()
	intCache.SetMany(intBatch)
	intDuration := time.Since(start)
	fmt.Printf("⏱️ Integer batch: %d items in %v\n", len(intBatch), intDuration)

	fmt.Println("\n🎉 Example completed!")
}
