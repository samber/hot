package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/samber/hot"
)

func main() {
	fmt.Println("🛡️ Thread Safety Example")
	fmt.Println("=========================")

	// Example 1: Basic concurrent access
	fmt.Println("\n📝 Example 1: Basic Concurrent Access")
	fmt.Println("--------------------------------------")

	cache := hot.NewHotCache[string, int](hot.LRU, 1000).
		WithTTL(5 * time.Minute).
		Build()

	fmt.Println("✅ Thread-safe cache created")

	// Test concurrent writes
	fmt.Println("🔄 Testing concurrent writes...")
	var wg sync.WaitGroup
	numGoroutines := 10
	itemsPerGoroutine := 100

	start := time.Now()
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < itemsPerGoroutine; j++ {
				key := fmt.Sprintf("goroutine:%d:item:%d", goroutineID, j)
				cache.Set(key, goroutineID*1000+j)
			}
		}(i)
	}
	wg.Wait()
	writeDuration := time.Since(start)

	fmt.Printf("✅ Concurrent writes completed: %d goroutines, %d items each\n",
		numGoroutines, itemsPerGoroutine)
	fmt.Printf("⏱️ Total write time: %v\n", writeDuration)

	// Test concurrent reads
	fmt.Println("🔄 Testing concurrent reads...")
	start = time.Now()
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < itemsPerGoroutine; j++ {
				key := fmt.Sprintf("goroutine:%d:item:%d", goroutineID, j)
				cache.Get(key)
			}
		}(i)
	}
	wg.Wait()
	readDuration := time.Since(start)

	fmt.Printf("✅ Concurrent reads completed\n")
	fmt.Printf("⏱️ Total read time: %v\n", readDuration)

	// Example 2: Read-Write contention
	fmt.Println("\n📝 Example 2: Read-Write Contention")
	fmt.Println("------------------------------------")

	rwCache := hot.NewHotCache[string, string](hot.LRU, 100).
		WithTTL(1 * time.Minute).
		Build()

	// Initialize with some data
	for i := 0; i < 50; i++ {
		rwCache.Set(fmt.Sprintf("initial:%d", i), fmt.Sprintf("value:%d", i))
	}

	// Simulate read-write contention
	fmt.Println("🔄 Simulating read-write contention...")
	start = time.Now()

	// Start readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				key := fmt.Sprintf("initial:%d", j%50)
				rwCache.Get(key)
			}
		}(i)
	}

	// Start writers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				key := fmt.Sprintf("writer:%d:item:%d", writerID, j)
				rwCache.Set(key, fmt.Sprintf("writer:%d:value:%d", writerID, j))
			}
		}(i)
	}

	wg.Wait()
	contentionDuration := time.Since(start)

	fmt.Printf("✅ Read-write contention test completed\n")
	fmt.Printf("⏱️ Total time: %v\n", contentionDuration)

	// Example 3: Concurrent batch operations
	fmt.Println("\n📦 Example 3: Concurrent Batch Operations")
	fmt.Println("------------------------------------------")

	batchCache := hot.NewHotCache[string, int](hot.LRU, 500).
		WithTTL(2 * time.Minute).
		Build()

	fmt.Println("🔄 Testing concurrent batch operations...")
	start = time.Now()

	// Concurrent batch writes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(batchID int) {
			defer wg.Done()
			batch := make(map[string]int)
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("batch:%d:item:%d", batchID, j)
				batch[key] = batchID*100 + j
			}
			batchCache.SetMany(batch)
		}(i)
	}

	wg.Wait()
	batchDuration := time.Since(start)

	fmt.Printf("✅ Concurrent batch operations completed\n")
	fmt.Printf("⏱️ Total time: %v\n", batchDuration)

	// Example 4: Cache eviction under load
	fmt.Println("\n🗑️ Example 4: Cache Eviction Under Load")
	fmt.Println("----------------------------------------")

	smallCache := hot.NewHotCache[string, int](hot.LRU, 50).
		WithTTL(30 * time.Second).
		Build()

	fmt.Println("🔄 Testing cache eviction under concurrent load...")
	start = time.Now()

	// Fill cache beyond capacity with concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				key := fmt.Sprintf("eviction:%d:item:%d", workerID, j)
				smallCache.Set(key, workerID*1000+j)
			}
		}(i)
	}

	wg.Wait()
	evictionDuration := time.Since(start)

	current, max := smallCache.Capacity()
	fmt.Printf("✅ Eviction test completed\n")
	fmt.Printf("📊 Final cache state: %d/%d items\n", current, max)
	fmt.Printf("⏱️ Total time: %v\n", evictionDuration)

	fmt.Println("\n🎉 Example completed!")
}
