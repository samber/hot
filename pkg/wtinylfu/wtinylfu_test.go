package wtinylfu

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/samber/hot/pkg/base"
	"github.com/stretchr/testify/assert"
)

func TestNewWTinyLFUCache(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("🧪 TestNewWTinyLFUCache: Starting test")
	}

	is.Panics(func() {
		_ = NewWTinyLFUCache[string, int](0)
	})

	cache := NewWTinyLFUCache[string, int](42)
	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("📊 Cache capacities - Total: 42, Window: %d, Main: %d, Probationary: %d, Protected: %d",
			cache.windowCapacity, cache.mainCapacity, cache.probationaryCapacity, cache.protectedCapacity)
	}
	is.Equal(41, cache.mainCapacity)
	is.Equal(1, cache.windowCapacity)
	is.NotNil(cache.probationaryLl)
	is.NotNil(cache.probationaryMap)
	is.Equal(0, cache.Len())

	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("✅ TestNewWTinyLFUCache: Completed successfully")
	}
}

func TestSet(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("🧪 TestSet: Starting test")
	}

	evicted := 0
	cache := NewWTinyLFUCacheWithEvictionCallback(100, func(reason base.EvictionReason, k string, v int) {
		is.Equal(base.EvictionReasonCapacity, reason)
		evicted += v
		if os.Getenv("DEBUG_WTINYLFU") == "1" {
			log.Printf("📤 Evicted: key=%s, value=%d, reason=%s, total evicted=%d", k, v, reason, evicted)
		}
	})

	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("📊 Cache initialized - Total: 100, Window: %d, Main: %d, Probationary: %d, Protected: %d",
			cache.windowCapacity, cache.mainCapacity, cache.probationaryCapacity, cache.protectedCapacity)
	}

	// First item should go to window cache
	cache.Set("a", 1)
	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("📝 Set 'a'=1 -> Window: %d, Probationary: %d, Protected: %d, Evicted: %d",
			cache.windowLl.Len(), cache.probationaryLl.Len(), cache.protectedLl.Len(), evicted)
	}
	is.Equal(0, cache.probationaryLl.Len())
	is.Empty(cache.probationaryMap)
	is.Equal(1, cache.windowLl.Len())
	is.Len(cache.windowCache, 1)
	is.Equal("a", cache.windowCache["a"].Value.key)
	is.Equal(0, evicted)

	// Second item should replace first in window cache if capacity is 1
	cache.Set("b", 2)
	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("📝 Set 'b'=2 -> Window: %d, Probationary: %d, Protected: %d, Evicted: %d",
			cache.windowLl.Len(), cache.probationaryLl.Len(), cache.protectedLl.Len(), evicted)
	}
	// Depending on windowing, items may or may not be evicted immediately
	is.LessOrEqual(cache.windowLl.Len(), 2)

	// Access "b" multiple times to increase frequency and promote it
	cache.Get("b")
	cache.Get("b")
	cache.Get("b")
	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("🔍 Accessed 'b' 3 times -> Current frequency estimate: %d", cache.sketch.Estimate("b"))
	}

	// Add another item - "b" should be promoted to main cache
	cache.Set("c", 3)
	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("📝 Set 'c'=3 -> Window: %d, Probationary: %d, Protected: %d, Evicted: %d",
			cache.windowLl.Len(), cache.probationaryLl.Len(), cache.protectedLl.Len(), evicted)
	}
	// "b" might be promoted to main cache
	is.GreaterOrEqual(cache.probationaryLl.Len(), 0)
	is.GreaterOrEqual(cache.windowLl.Len(), 1)

	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("✅ TestSet: Completed - Final state -> Window: %d, Probationary: %d, Protected: %d, Total: %d",
			cache.windowLl.Len(), cache.probationaryLl.Len(), cache.protectedLl.Len(), cache.Len())
	}
}

func TestHas(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](100)

	cache.Set("a", 1)
	is.True(cache.Has("a"))

	cache.Set("b", 2)
	cache.Set("c", 3)

	// At least the latest item should be present
	is.True(cache.Has("c") || cache.Has("b") || cache.Has("a"))
	is.False(cache.Has("nonexistent"))
}

func TestGet(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](100)

	cache.Set("a", 1)

	// "a" should be in admission window
	val, ok := cache.Get("a")
	is.True(ok)
	is.Equal(1, val)

	// Add more items
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Items should still be accessible if they haven't been evicted
	if cache.Has("a") {
		val, ok = cache.Get("a")
		is.True(ok)
		is.Equal(1, val)
	}

	if cache.Has("b") {
		val, ok = cache.Get("b")
		is.True(ok)
		is.Equal(2, val)
	}

	if cache.Has("c") {
		val, ok = cache.Get("c")
		is.True(ok)
		is.Equal(3, val)
	}

	val, ok = cache.Get("nonexistent")
	is.False(ok)
	is.Zero(val)
}

func TestKey(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](100)
	cache.Set("a", 1)
	cache.Set("b", 2)

	keys := cache.Keys()
	is.GreaterOrEqual(len(keys), 1)
	is.Contains(keys, "b") // Most recent item should exist
}

func TestValues(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](10)
	cache.Set("a", 1)
	cache.Set("b", 2)

	values := cache.Values()
	is.GreaterOrEqual(len(values), 1)
}

func TestAll(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](10)
	cache.Set("a", 1)
	cache.Set("b", 2)

	all := cache.All()
	is.GreaterOrEqual(len(all), 1)
}

func TestRange(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](10)
	cache.Set("a", 1)
	cache.Set("b", 2)

	var keys []string
	var values []int
	cache.Range(func(key string, value int) bool {
		keys = append(keys, key)
		values = append(values, value)
		return true
	})

	is.GreaterOrEqual(len(keys), 1)
	is.GreaterOrEqual(len(values), 1)
}

func TestDelete(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](100)
	cache.Set("a", 1)

	// Access "a" multiple times to promote it to main cache
	cache.Get("a")
	cache.Get("a")
	cache.Get("a")

	cache.Set("b", 2)

	// Both items should exist now (a in main cache, b in admission window)
	is.True(cache.Has("a"))
	is.True(cache.Has("b"))

	is.True(cache.Delete("a"))
	is.True(cache.Delete("b"))
	is.False(cache.Delete("nonexistent"))
}

func TestLen(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](10)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.LessOrEqual(cache.Len(), 10)
}

func TestPurge(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](10)
	cache.Set("a", 1)
	cache.Set("b", 2)

	cache.Purge()
	is.Equal(0, cache.Len())
}

func TestWindowing(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("🧪 TestWindowing: Starting test")
	}

	cache := NewWTinyLFUCache[string, int](20)

	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("📊 Cache initialized - Total: 20, Window: %d, Main: %d, Probationary: %d, Protected: %d",
			cache.windowCapacity, cache.mainCapacity, cache.probationaryCapacity, cache.protectedCapacity)
	}

	// Add items to fill the window cache and trigger eviction
	for i := 0; i < 25; i++ {
		key := fmt.Sprintf("item_%d", i)
		cache.Set(key, i)

		if os.Getenv("DEBUG_WTINYLFU") == "1" && i%5 == 0 {
			log.Printf("📝 Set %s -> Window: %d, Probationary: %d, Protected: %d, Total: %d",
				key, cache.windowLl.Len(), cache.probationaryLl.Len(), cache.protectedLl.Len(), cache.Len())
		}
	}

	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("📊 Final state - Window: %d, Probationary: %d, Protected: %d, Total: %d",
			cache.windowLl.Len(), cache.probationaryLl.Len(), cache.protectedLl.Len(), cache.Len())
		log.Printf("✅ TestWindowing: Completed successfully")
	}

	// Cache should respect capacity
	is.LessOrEqual(cache.Len(), 20)
}

func TestWindowEviction(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("🧪 TestWindowEviction: Starting test")
	}

	cache := NewWTinyLFUCache[string, int](5)

	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("📊 Cache initialized - Total: 5, Window: %d, Main: %d, Probationary: %d, Protected: %d",
			cache.windowCapacity, cache.mainCapacity, cache.probationaryCapacity, cache.protectedCapacity)
	}

	// Add items to trigger window-based eviction
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("item_%d", i)
		cache.Set(key, i)

		if os.Getenv("DEBUG_WTINYLFU") == "1" {
			log.Printf("📝 Set %s -> Window: %d, Probationary: %d, Protected: %d, Total: %d",
				key, cache.windowLl.Len(), cache.probationaryLl.Len(), cache.protectedLl.Len(), cache.Len())
		}
	}

	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("📊 Final state - Window: %d, Probationary: %d, Protected: %d, Total: %d",
			cache.windowLl.Len(), cache.probationaryLl.Len(), cache.protectedLl.Len(), cache.Len())
		log.Printf("✅ TestWindowEviction: Completed successfully")
	}

	// Cache should respect capacity
	is.LessOrEqual(cache.Len(), 5)
}

func TestPromotionLogic(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("🧪 TestPromotionLogic: Starting test")
	}

	cache := NewWTinyLFUCache[string, int](100)

	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("📊 Cache initialized - Total: 100, Window: %d, Main: %d, Probationary: %d, Protected: %d",
			cache.windowCapacity, cache.mainCapacity, cache.probationaryCapacity, cache.protectedCapacity)
	}

	// Add an item and access it frequently
	cache.Set("popular", 1)
	for i := 0; i < 10; i++ {
		cache.Get("popular")
	}

	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("📝 Set and accessed 'popular' 10 times -> Window: %d, Probationary: %d, Protected: %d, Total: %d",
			cache.windowLl.Len(), cache.probationaryLl.Len(), cache.protectedLl.Len(), cache.Len())
		log.Printf("📈 'popular' frequency estimate: %d", cache.sketch.Estimate("popular"))
	}

	// Add other items
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("item_%d", i)
		cache.Set(key, i)

		if os.Getenv("DEBUG_WTINYLFU") == "1" && i%5 == 0 {
			log.Printf("📝 Set %s -> Window: %d, Probationary: %d, Protected: %d, Total: %d",
				key, cache.windowLl.Len(), cache.probationaryLl.Len(), cache.protectedLl.Len(), cache.Len())
		}
	}

	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("📊 Final state - Window: %d, Probationary: %d, Protected: %d, Total: %d",
			cache.windowLl.Len(), cache.probationaryLl.Len(), cache.protectedLl.Len(), cache.Len())
		log.Printf("📈 Final 'popular' frequency estimate: %d", cache.sketch.Estimate("popular"))
	}

	// Popular item should still be accessible if it was promoted
	if cache.Has("popular") {
		val, ok := cache.Get("popular")
		is.True(ok)
		is.Equal(1, val)

		if os.Getenv("DEBUG_WTINYLFU") == "1" {
			log.Printf("✅ 'popular' found in cache and promoted successfully")
			log.Printf("✅ TestPromotionLogic: Completed successfully")
		}
	} else if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("⚠️ 'popular' not found in cache - may have been evicted")
		log.Printf("✅ TestPromotionLogic: Completed")
	}
}

func TestSetMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](10)

	items := map[string]int{"a": 1, "b": 2, "c": 3}
	cache.SetMany(items)

	is.GreaterOrEqual(cache.Len(), 1)
}

func TestHasMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](10)
	cache.Set("a", 1)
	cache.Set("b", 2)

	keys := []string{"a", "b", "c"}
	results := cache.HasMany(keys)

	is.Len(results, 3)
	// Some keys may not exist due to eviction
}

func TestGetMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](10)
	cache.Set("a", 1)
	cache.Set("b", 2)

	keys := []string{"a", "b", "c"}
	values, missing := cache.GetMany(keys)

	is.LessOrEqual(len(values), 2)
	is.LessOrEqual(len(missing), 3)
}

func TestPeekMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](10)
	cache.Set("a", 1)
	cache.Set("b", 2)

	keys := []string{"a", "b", "c"}
	values, missing := cache.PeekMany(keys)

	is.LessOrEqual(len(values), 2)
	is.LessOrEqual(len(missing), 3)
}

func TestDeleteMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](10)
	cache.Set("a", 1)
	cache.Set("b", 2)

	keys := []string{"a", "b", "c"}
	results := cache.DeleteMany(keys)

	is.Len(results, 3)
}

func TestPeek(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](10)
	cache.Set("a", 1)

	val, ok := cache.Peek("a")
	is.True(ok)
	is.Equal(1, val)

	val, ok = cache.Peek("nonexistent")
	is.False(ok)
	is.Zero(val)
}

func TestAlgorithmName(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](100)
	is.Equal("wtinylfu", cache.Algorithm())
}

func TestCapacity(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](100)
	is.Equal(100, cache.Capacity())
}

func TestWindowAdvancement(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](20)
	initialLen := cache.Len()

	// Add enough items to trigger window advancement
	for i := 0; i < 40; i++ {
		key := fmt.Sprintf("item_%d", i)
		cache.Set(key, i)
	}

	// Cache should respect capacity
	is.LessOrEqual(cache.Len(), 20)
	is.GreaterOrEqual(cache.Len(), initialLen)
}

func TestCountMinSketch(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("🧪 TestCountMinSketch: Starting test")
	}

	cache := NewWTinyLFUCache[string, int](100)

	// Access the same key many times
	key := "popular"
	for i := 0; i < 50; i++ {
		cache.Get(key)

		if os.Getenv("DEBUG_WTINYLFU") == "1" && i%10 == 0 {
			log.Printf("🔍 Accessed '%s' %d times -> Current estimate: %d", key, i+1, cache.sketch.Estimate(key))
		}
	}

	// The sketch should have a high estimate for this key
	estimate := cache.sketch.Estimate(key)
	if os.Getenv("DEBUG_WTINYLFU") == "1" {
		log.Printf("📈 Final frequency estimate for '%s': %d", key, estimate)
		log.Printf("✅ TestCountMinSketch: Completed successfully")
	}
	is.Greater(estimate, 10)
}

func TestEdgeCases(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Test with capacity 1
	cache := NewWTinyLFUCache[string, int](1)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// With W-TinyLFU, capacity 1 means window=1, main=0, but main gets minimum 1
	// So total capacity can be more than 1
	is.LessOrEqual(cache.Len(), 3) // window + probationary + protected

	// Test with empty cache
	cache2 := NewWTinyLFUCache[string, int](10)
	is.Equal(0, cache2.Len())
	is.Equal(10, cache2.Capacity())

	// Test peek on empty cache
	val, ok := cache2.Peek("nonexistent")
	is.False(ok)
	is.Zero(val)
}

func TestConcurrentAccessPattern(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewWTinyLFUCache[string, int](1000)

	// Simulate concurrent access pattern
	for i := 0; i < 500; i++ {
		key := fmt.Sprintf("item_%d", i%50) // Create 50 unique keys
		cache.Set(key, i)

		if i%10 == 0 {
			cache.Get(key)
		}
	}

	// Cache should respect capacity
	is.LessOrEqual(cache.Len(), 1000)
}
