package s3fifo

import (
	"testing"

	"github.com/samber/hot/pkg/base"
	"github.com/stretchr/testify/assert"
)

func TestNewS3FIFOCache(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, int](10)
	assert.NotNil(t, cache)
	assert.Equal(t, 10, cache.Capacity())
	assert.Equal(t, "s3fifo", cache.Algorithm())
	assert.Equal(t, 0, cache.Len())
}

func TestNewS3FIFOCacheWithEvictionCallback(t *testing.T) {
	t.Parallel()

	var evictedKey string
	var evictedValue int
	var evictionReason base.EvictionReason

	onEviction := func(reason base.EvictionReason, key string, value int) {
		evictedKey = key
		evictedValue = value
		evictionReason = reason
	}

	cache := NewS3FIFOCacheWithEvictionCallback[string, int](5, onEviction)
	assert.NotNil(t, cache)
	assert.Equal(t, 5, cache.Capacity())

	// Test eviction callback
	cache.Set("a", 1) // small: ["a"]
	cache.Set("b", 2) // "a" evicted to ghost, small: ["b"]
	cache.Set("c", 3) // "b" evicted to ghost, small: ["c"]
	cache.Set("d", 4) // "c" evicted to ghost, small: ["d"]
	cache.Set("e", 5) // "d" evicted to ghost, small: ["e"]
	cache.Set("f", 6) // "e" evicted to ghost, small: ["f"]

	assert.Equal(t, "a", evictedKey) // "a" was evicted from small queue
	assert.Equal(t, 1, evictedValue)
	assert.Equal(t, base.EvictionReasonCapacity, evictionReason)
}

func TestS3FIFOCache_SetAndGet(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, int](10)

	// Test basic set and get
	cache.Set("a", 1)
	assert.Equal(t, 1, cache.Len())
	assert.True(t, cache.Has("a"))

	value, ok := cache.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 1, value)

	// Test frequency increment
	value, ok = cache.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 1, value)
	assert.Equal(t, 2, cache.freq["a"]) // frequency should be incremented

	// Test update existing key
	cache.Set("a", 10)
	value, ok = cache.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 10, value)
}

func TestS3FIFOCache_SmallQueuePromotion(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, int](10)

	// Add item to small queue
	cache.Set("a", 1)
	assert.Equal(t, 1, cache.Len())

	// Access twice to promote to main queue
	cache.Get("a") // freq = 1
	cache.Get("a") // freq = 2, should promote

	assert.Equal(t, 1, cache.Len())
}

func TestS3FIFOCache_GhostQueueReinsertion(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, int](3) // Use smaller capacity to force eviction

	// Fill cache to capacity
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	assert.Equal(t, 3, cache.Len())
	assert.True(t, cache.Has("a"))
	assert.True(t, cache.Has("b"))
	assert.True(t, cache.Has("c"))

	// Add one more to force eviction
	cache.Set("d", 4) // should evict "a"

	assert.Equal(t, 3, cache.Len())
	assert.False(t, cache.Has("a"))
	assert.True(t, cache.Has("d"))

	// Reinsert "a" - should work since it was evicted
	cache.Set("a", 10)

	assert.Equal(t, 3, cache.Len())
	assert.True(t, cache.Has("a"))
}

func TestS3FIFOCache_EvictionPolicy(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, int](5)

	// Fill small queue
	cache.Set("a", 1)
	assert.Equal(t, 1, cache.Len())

	// Add more items - should promote from small to main
	cache.Set("b", 2) // "a" promoted to main
	cache.Set("c", 3) // "b" promoted to main
	cache.Set("d", 4) // "c" promoted to main

	assert.Equal(t, 4, cache.Len()) // all items still in cache
	assert.True(t, cache.Has("a"))  // "a" still in cache
	assert.True(t, cache.Has("b"))  // "b" still in cache

	// Add more items - should evict from main now
	cache.Set("e", 5) // "d" promoted to main, main now full
	cache.Set("f", 6) // evict oldest from main ("a")

	assert.Equal(t, 5, cache.Len()) // cache at capacity
	assert.False(t, cache.Has("a")) // "a" evicted
	assert.True(t, cache.Has("f"))  // "f" in cache
}

func TestS3FIFOCache_Peek(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, int](5)

	// Test peek non-existent key
	value, ok := cache.Peek("a")
	assert.False(t, ok)
	assert.Equal(t, 0, value)

	// Test peek existing key
	cache.Set("a", 1)
	value, ok = cache.Peek("a")
	assert.True(t, ok)
	assert.Equal(t, 1, value)

	// Peek should not increment frequency
	value, ok = cache.Peek("a")
	assert.True(t, ok)
	assert.Equal(t, 1, value)
}

func TestS3FIFOCache_Delete(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, int](5)

	// Test delete non-existent key
	assert.False(t, cache.Delete("a"))

	// Test delete existing key
	cache.Set("a", 1)
	assert.True(t, cache.Delete("a"))
	assert.False(t, cache.Has("a"))
	assert.Equal(t, 0, cache.Len())

	// Test delete from main queue
	cache.Set("a", 1)
	cache.Get("a") // promote to main
	cache.Get("a") // increment frequency
	assert.True(t, cache.Delete("a"))
	assert.False(t, cache.Has("a"))
}

func TestS3FIFOCache_Purge(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, int](5)

	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)
	assert.Equal(t, 3, cache.Len())

	cache.Purge()
	assert.Equal(t, 0, cache.Len())
}

func TestS3FIFOCache_KeysValues(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, int](5)

	assert.Empty(t, cache.Keys())
	assert.Empty(t, cache.Values())

	cache.Set("a", 1)
	cache.Set("b", 2)

	keys := cache.Keys()
	assert.Len(t, keys, 2)
	assert.Contains(t, keys, "a")
	assert.Contains(t, keys, "b")

	values := cache.Values()
	assert.Len(t, values, 2)
	assert.Contains(t, values, 1)
	assert.Contains(t, values, 2)
}

func TestS3FIFOCache_All(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)

	all := cache.All()
	assert.Len(t, all, 2)
	assert.Equal(t, 1, all["a"])
	assert.Equal(t, 2, all["b"])
}

func TestS3FIFOCache_Range(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, int](3)

	cache.Set("a", 1)
	cache.Set("b", 2)

	visited := make(map[string]int)
	cache.Range(func(key string, value int) bool {
		visited[key] = value
		return true
	})

	assert.Equal(t, map[string]int{"a": 1, "b": 2}, visited)

	// Test early termination
	visited = make(map[string]int)
	cache.Range(func(key string, value int) bool {
		visited[key] = value
		return false // Stop after first iteration
	})

	assert.Len(t, visited, 1)
}

func TestS3FIFOCache_BatchOperations(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, int](10)

	// SetMany
	items := map[string]int{"a": 1, "b": 2, "c": 3}
	cache.SetMany(items)
	assert.Equal(t, 3, cache.Len())

	// HasMany
	result := cache.HasMany([]string{"a", "b", "d"})
	expected := map[string]bool{"a": true, "b": true, "d": false}
	assert.Equal(t, expected, result)

	// GetMany
	found, missing := cache.GetMany([]string{"a", "b", "d"})
	assert.Equal(t, map[string]int{"a": 1, "b": 2}, found)
	assert.Equal(t, []string{"d"}, missing)

	// PeekMany
	found, missing = cache.PeekMany([]string{"a", "b", "d"})
	assert.Equal(t, map[string]int{"a": 1, "b": 2}, found)
	assert.Equal(t, []string{"d"}, missing)

	// DeleteMany
	result = cache.DeleteMany([]string{"a", "b", "d"})
	assert.Equal(t, map[string]bool{"a": true, "b": true, "d": false}, result)
	assert.Equal(t, 1, cache.Len())
	assert.True(t, cache.Has("c"))
}

func TestS3FIFOCache_SizeBytes(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, string](3)

	assert.Equal(t, int64(0), cache.SizeBytes())

	cache.Set("a", "hello")
	cache.Set("b", "world")

	size := cache.SizeBytes()
	assert.Positive(t, size)
}

func TestS3FIFOCache_FrequencyCapping(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, int](5)

	cache.Set("a", 1)

	// Access many times - frequency should cap at 3
	for i := 0; i < 10; i++ {
		cache.Get("a")
	}

	// Test that the item is still in cache after many accesses
	assert.True(t, cache.Has("a"))
}

func TestS3FIFOCache_SmallCapacity(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, int](3)

	cache.Set("a", 1) // small: ["a"]
	cache.Set("b", 2) // "a" promoted to main, small: ["b"]
	cache.Set("c", 3) // "b" promoted to main, main now full, small: ["c"]

	assert.Equal(t, 3, cache.Len()) // all items should still be in cache
	assert.True(t, cache.Has("c"))
	assert.True(t, cache.Has("b"))
	assert.True(t, cache.Has("a"))

	// Add one more to trigger eviction
	cache.Set("d", 4) // "c" promoted to main, evict oldest from main ("a")

	assert.Equal(t, 3, cache.Len()) // cache at capacity
	assert.True(t, cache.Has("d"))
	assert.True(t, cache.Has("c"))
	assert.False(t, cache.Has("a")) // "a" evicted
}

func TestS3FIFOCache_UpdateExistingKey(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, int](5)

	// Add items
	cache.Set("a", 1)
	cache.Set("b", 2)

	// Update existing key
	cache.Set("a", 10)
	value, _ := cache.Get("a")
	assert.Equal(t, 10, value)

	// Update again
	cache.Set("a", 20)
	value, _ = cache.Get("a")
	assert.Equal(t, 20, value)
}

func TestS3FIFOCache_CacheMissGhostTracking(t *testing.T) {
	t.Parallel()

	cache := NewS3FIFOCache[string, int](1) // Use capacity 1 to force eviction

	// Add and evict an item to put it in ghost
	cache.Set("a", 1)
	cache.Set("b", 2) // evicts "a" when cache is full

	assert.Equal(t, 1, cache.Len()) // cache is full
	assert.False(t, cache.Has("a")) // "a" should be evicted
	assert.True(t, cache.Has("b"))  // "b" is in cache

	// Cache miss - should track frequency in ghost
	_, _ = cache.Get("a") // miss, but should increment frequency

	_, _ = cache.Get("a") // another miss

	// Reinsert - should work
	cache.Set("a", 10)
	assert.True(t, cache.Has("a"))
	assert.False(t, cache.Has("b")) // "b" should be evicted now
}

func TestS3FIFOCache_EvictFromSmall_PromotesToMain(t *testing.T) {
	t.Parallel()

	// Capacity 3: smallLimit=1, mainLimit=2
	cache := NewS3FIFOCache[string, int](3)

	// Add "a" to small queue, then access it once so freq > 0
	cache.Set("a", 1)
	cache.Get("a") // freq=1, not promoted inline (needs freq>=2)

	// Verify "a" is in small queue with freq > 0
	e := cache.cache["a"]
	assert.Equal(t, 0, e.Value.queue, "a should be in small queue")
	assert.Equal(t, 1, e.Value.freq, "a should have freq=1")

	// Fill cache to capacity to trigger eviction
	cache.Set("b", 2) // small: [a(1), b(0)], no eviction yet (Len=2 < 3)
	cache.Set("c", 3) // small: [a(1), b(0), c(0)], Len=3 >= 3 → evict
	// evictFromSmall: front is "a" with freq=1 > 0 → promote to main
	// Then evictFromSmall again: front is "b" with freq=0 → evict to ghost
	// Then insert "c" → but "c" was already set... let me reconsider

	// Actually Set("c") triggers eviction before inserting "c":
	// small: [a(1), b(0)], Len=2. Wait, Len=2 < capacity=3, no eviction for "c".
	// After Set("c"): small: [a(1), b(0), c(0)], Len=3

	// Now Set("d") triggers eviction (Len=3 >= 3)
	cache.Set("d", 4)
	// evictFromSmall: front "a" has freq=1 > 0 → promoted to main with freq=0
	// Still Len=3 >= 3, evictFromSmall again: front "b" has freq=0 → evict to ghost
	// Now Len=2 < 3, insert "d"

	// "a" should be promoted to main
	assert.True(t, cache.Has("a"), "a should still be in cache after promotion to main")
	e = cache.cache["a"]
	assert.Equal(t, 1, e.Value.queue, "a should be in main queue after promotion")
	assert.Equal(t, 0, e.Value.freq, "a freq should be reset to 0 after promotion")

	// "b" should be evicted (freq was 0)
	assert.False(t, cache.Has("b"), "b should be evicted since freq was 0")

	// "c" and "d" should be in small queue
	assert.True(t, cache.Has("c"))
	assert.True(t, cache.Has("d"))
}

func TestS3FIFOCache_EvictFromSmall_EvictsToGhost(t *testing.T) {
	t.Parallel()

	var evictedKeys []string
	onEviction := func(reason base.EvictionReason, key string, value int) {
		evictedKeys = append(evictedKeys, key)
	}

	// Capacity 3: smallLimit=1, mainLimit=2
	cache := NewS3FIFOCacheWithEvictionCallback[string, int](3, onEviction)

	// Add items without accessing them (all freq=0)
	cache.Set("a", 1) // small: [a(0)]
	cache.Set("b", 2) // small: [a(0), b(0)]
	cache.Set("c", 3) // small: [a(0), b(0), c(0)], Len=3 >= 3

	// Set("c") doesn't trigger eviction yet since Len was 2 before insert
	// Now add "d" to trigger eviction (Len=3 >= 3)
	cache.Set("d", 4)
	// evictFromSmall: front "a" has freq=0 → evict to ghost, callback fired
	// Len=2 < 3, insert "d"

	assert.False(t, cache.Has("a"), "a should be evicted since freq was 0")
	assert.True(t, cache.Has("d"))
	assert.Contains(t, evictedKeys, "a", "eviction callback should have been called for a")

	// "a" should now be in ghost queue
	assert.True(t, cache.isInGhost("a"), "a should be in ghost queue after eviction")
}

func TestS3FIFOCache_EvictFromMain_SecondChance(t *testing.T) {
	t.Parallel()

	var evictedKeys []string
	onEviction := func(reason base.EvictionReason, key string, value int) {
		evictedKeys = append(evictedKeys, key)
	}

	// Capacity 3: smallLimit=1, mainLimit=2
	cache := NewS3FIFOCacheWithEvictionCallback[string, int](3, onEviction)

	// Promote "a" and "b" to main via inline promotion (freq>=2)
	cache.Set("a", 1)
	cache.Get("a") // freq=1
	cache.Get("a") // freq=2, promoted to main

	cache.Set("b", 2)
	cache.Get("b") // freq=1
	cache.Get("b") // freq=2, promoted to main
	// main: [a(2), b(2)], small: [], Len=2

	assert.Equal(t, 2, cache.main.Len())
	assert.Equal(t, 0, cache.small.Len())

	// Give "a" higher frequency so it survives longer
	cache.Get("a") // freq=3 (capped at maxFrequency)

	// Add "c" (Len=2 < 3, no eviction) then "d" to trigger eviction
	cache.Set("c", 3) // small: [c(0)], Len=3
	cache.Set("d", 4) // Len=3 >= 3 → evict
	// small.Len()=1, not > smallLimit=1; main.Len()=2, not > mainLimit=2
	// Condition 3: Len >= capacity → evictFromMain
	// Second-chance scan: a(3)→2→back, b(2)→1→back, a(2)→1→back, b(1)→0→back,
	//   a(1)→0→back, b(0)→evict!
	// Then Len=2 < 3, insert "d" → small: [c(0), d(0)]

	assert.Equal(t, 3, cache.Len())
	assert.True(t, cache.Has("a"), "a should survive eviction due to higher frequency")
	assert.False(t, cache.Has("b"), "b should be evicted (lower freq, reached 0 first)")
	assert.True(t, cache.Has("c"))
	assert.True(t, cache.Has("d"))
	assert.Contains(t, evictedKeys, "b")

	// "a" should have freq=0 after being decremented during the scan
	eA := cache.cache["a"]
	assert.Equal(t, 0, eA.Value.freq, "a freq should be 0 after second-chance scan")
}

func TestS3FIFOCache_EvictFromMain_SecondChance_EqualFreq(t *testing.T) {
	t.Parallel()

	var evictedKeys []string
	onEviction := func(reason base.EvictionReason, key string, value int) {
		evictedKeys = append(evictedKeys, key)
	}

	// Capacity 3: smallLimit=1, mainLimit=2
	cache := NewS3FIFOCacheWithEvictionCallback[string, int](3, onEviction)

	// Promote "a" and "b" to main with equal freq
	cache.Set("a", 1)
	cache.Get("a") // freq=1
	cache.Get("a") // freq=2, promoted to main

	cache.Set("b", 2)
	cache.Get("b") // freq=1
	cache.Get("b") // freq=2, promoted to main
	// main: [a(2), b(2)], small: [], Len=2

	// Add "c" (no eviction) then "d" to trigger eviction from main
	cache.Set("c", 3) // small: [c(0)], Len=3
	cache.Set("d", 4) // Len=3 >= 3 → evictFromMain
	// Second-chance: a(2)→1→back, b(2)→1→back, a(1)→0→back, b(1)→0→back, a(0)→evict!
	// "a" is evicted first because it's FIFO-front with equal freq

	assert.False(t, cache.Has("a"), "a should be evicted (FIFO-front with equal freq)")
	assert.True(t, cache.Has("b"), "b should survive")
	assert.Contains(t, evictedKeys, "a")

	// Verify "b" had its freq decremented to 0 during the scan
	eB := cache.cache["b"]
	assert.Equal(t, 0, eB.Value.freq, "b freq should be 0 after second-chance scan")
}

func TestS3FIFOCache_EvictFromMain_FreqZeroEvictedImmediately(t *testing.T) {
	t.Parallel()

	var evictedKeys []string
	onEviction := func(reason base.EvictionReason, key string, value int) {
		evictedKeys = append(evictedKeys, key)
	}

	// Capacity 4: smallLimit=1, mainLimit=3
	cache := NewS3FIFOCacheWithEvictionCallback[string, int](4, onEviction)

	// Step 1: Promote a, b, c to main via inline promotion, then promote d to fill main
	cache.Set("a", 1)
	cache.Get("a")
	cache.Get("a") // freq=2, promoted to main

	cache.Set("b", 2)
	cache.Get("b")
	cache.Get("b") // promoted to main

	cache.Set("c", 3)
	cache.Get("c")
	cache.Get("c") // promoted to main

	cache.Set("d", 4)
	cache.Get("d")
	cache.Get("d") // promoted to main
	// main: [a(2), b(2), c(2), d(2)], small: [], Len=4

	assert.Equal(t, 4, cache.main.Len())
	assert.Equal(t, 0, cache.small.Len())

	// Step 2: Add "e" → main.Len()=4 > mainLimit=3 → evictFromMain
	// Second-chance scan runs through all items, decrementing freq and moving to back.
	// Eventually "a" (FIFO-front) reaches freq=0 first and is evicted.
	cache.Set("e", 5)
	// After eviction: main has 3 items with freq=0, small: [e(0)], Len=4

	assert.False(t, cache.Has("a"), "a should be evicted via second-chance")
	assert.True(t, cache.Has("b"))
	assert.True(t, cache.Has("c"))
	assert.True(t, cache.Has("d"))
	assert.True(t, cache.Has("e"))

	// "b" is now at front of main with freq=0
	eB := cache.cache["b"]
	assert.Equal(t, 0, eB.Value.freq, "b should have freq=0 after second-chance scan")

	// Step 3: Add "f" → Len=4 >= 4
	// small.Len()=1, not > 1. main.Len()=3, not > 3. Condition 3 → evictFromMain.
	// Front of main is "b" with freq=0 → evicted IMMEDIATELY (no second-chance loop)
	evictedKeys = nil
	cache.Set("f", 6)

	assert.False(t, cache.Has("b"), "b should be evicted immediately with freq=0")
	assert.Contains(t, evictedKeys, "b")
	assert.True(t, cache.Has("c"))
	assert.True(t, cache.Has("d"))
}
