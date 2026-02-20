package sieve

import (
	"testing"

	"github.com/samber/hot/pkg/base"
	"github.com/stretchr/testify/assert"
)

func TestNewSIEVECache(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	is.Panics(func() {
		_ = NewSIEVECache[string, int](0)
	})

	cache := NewSIEVECache[string, int](42)
	is.Equal(42, cache.capacity)
	is.NotNil(cache.ll)
	is.NotNil(cache.cache)
	is.Nil(cache.hand)
}

func TestSet(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	evicted := 0
	cache := NewSIEVECacheWithEvictionCallback(2, func(reason base.EvictionReason, k string, v int) {
		is.Equal(base.EvictionReasonCapacity, reason)
		evicted += v
	})

	cache.Set("a", 1)
	is.Equal(1, cache.ll.Len())
	is.Len(cache.cache, 1)
	is.Equal(0, evicted)

	cache.Set("b", 2)
	is.Equal(2, cache.ll.Len())
	is.Len(cache.cache, 2)
	is.Equal(0, evicted)

	// Adding third item should evictAndCallback one (SIEVE picks the one with visited=false)
	// Both "a" and "b" have visited=false, so it should evictAndCallback from the back (oldest = "a")
	cache.Set("c", 3)
	is.Equal(2, cache.ll.Len())
	is.Len(cache.cache, 2)
	is.Equal(1, evicted) // "a" was evicted (value=1)
}

func TestHas(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.True(cache.Has("a"))
	is.True(cache.Has("b"))
	is.False(cache.Has("c"))

	cache.Set("c", 3)
	is.False(cache.Has("a")) // "a" should be evicted
	is.True(cache.Has("b"))
	is.True(cache.Has("c"))
}

func TestGet(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	val, ok := cache.Get("a")
	is.True(ok)
	is.Equal(1, val)

	val, ok = cache.Get("b")
	is.True(ok)
	is.Equal(2, val)

	val, ok = cache.Get("c")
	is.False(ok)
	is.Zero(val)
}

func TestGetProtectsFromEviction(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// Access "a" to set its visited bit
	cache.Get("a")

	// Add "c" - should evictAndCallback "b" (not "a" because "a" is visited)
	cache.Set("c", 3)
	is.True(cache.Has("a"))  // "a" should survive because it was visited
	is.False(cache.Has("b")) // "b" should be evicted
	is.True(cache.Has("c"))
}

func TestPeek(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// Test peeking existing key
	val, ok := cache.Peek("a")
	is.True(ok)
	is.Equal(1, val)

	val, ok = cache.Peek("b")
	is.True(ok)
	is.Equal(2, val)

	// Test peeking non-existing key
	val, ok = cache.Peek("c")
	is.False(ok)
	is.Zero(val)
}

func TestPeekDoesNotProtect(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// Peek "a" - should NOT set visited bit
	cache.Peek("a")

	// Add "c" - should evictAndCallback "a" (oldest with visited=false)
	cache.Set("c", 3)
	is.False(cache.Has("a")) // "a" should be evicted because Peek doesn't set visited
	is.True(cache.Has("b"))
	is.True(cache.Has("c"))
}

func TestKeys(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.ElementsMatch([]string{"b", "a"}, cache.Keys())
}

func TestValues(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.ElementsMatch([]int{1, 2}, cache.Values())
}

func TestAll(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	all := cache.All()
	is.Len(all, 2)
	is.Equal(1, all["a"])
	is.Equal(2, all["b"])
}

func TestRange(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	var keys []string
	var values []int
	cache.Range(func(key string, value int) bool {
		keys = append(keys, key)
		values = append(values, value)
		return true
	})
	is.ElementsMatch([]string{"b", "a"}, keys)
	is.ElementsMatch([]int{2, 1}, values)
}

func TestRangeEarlyTermination(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Test that Range stops when callback returns false
	count := 0
	cache.Range(func(key string, value int) bool {
		count++
		return false // Stop after first iteration
	})
	is.Equal(1, count)
}

func TestDelete(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	ok := cache.Delete("a")
	is.True(ok)
	is.Equal(1, cache.ll.Len())
	is.Len(cache.cache, 1)
	is.False(cache.Has("a"))
	is.True(cache.Has("b"))

	ok = cache.Delete("b")
	is.True(ok)
	is.Equal(0, cache.ll.Len())
	is.Empty(cache.cache)

	ok = cache.Delete("b")
	is.False(ok)
}

func TestLen(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](2)
	cache.Set("z", 0)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.Equal(2, cache.Len())

	cache.Delete("a")
	is.Equal(1, cache.Len())

	cache.Delete("b")
	is.Equal(0, cache.Len())
}

func TestPurge(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.Equal(2, cache.ll.Len())
	is.Len(cache.cache, 2)

	cache.Purge()

	is.Equal(0, cache.ll.Len())
	is.Empty(cache.cache)
	is.Nil(cache.hand)
}

func TestCapacity(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](42)
	is.Equal(42, cache.Capacity())
}

func TestAlgorithm(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](2)
	is.Equal("sieve", cache.Algorithm())
}

func TestSetMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](3)

	// Test setting multiple items
	items := map[string]int{"a": 1, "b": 2, "c": 3}
	cache.SetMany(items)

	is.Equal(3, cache.Len())
	is.True(cache.Has("a"))
	is.True(cache.Has("b"))
	is.True(cache.Has("c"))

	val, ok := cache.Get("a")
	is.True(ok)
	is.Equal(1, val)

	// Test setting empty map
	cache.SetMany(map[string]int{})
	is.Equal(3, cache.Len()) // Should not change anything
}

func TestHasMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Test checking multiple existing keys
	keys := []string{"a", "b", "c"}
	results := cache.HasMany(keys)

	is.Len(results, 3)
	is.True(results["a"])
	is.True(results["b"])
	is.True(results["c"])

	// Test checking mixed existing and non-existing keys
	keys2 := []string{"a", "d", "b", "e"}
	results2 := cache.HasMany(keys2)

	is.Len(results2, 4)
	is.True(results2["a"])
	is.False(results2["d"])
	is.True(results2["b"])
	is.False(results2["e"])

	// Test checking empty slice
	results3 := cache.HasMany([]string{})
	is.Empty(results3)
}

func TestGetMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Test getting multiple existing keys
	keys := []string{"a", "b", "c"}
	values, missing := cache.GetMany(keys)

	is.Len(values, 3)
	is.Empty(missing)
	is.Equal(1, values["a"])
	is.Equal(2, values["b"])
	is.Equal(3, values["c"])

	// Test getting mixed existing and non-existing keys
	keys2 := []string{"a", "d", "b", "e"}
	values2, missing2 := cache.GetMany(keys2)

	is.Len(values2, 2)
	is.Len(missing2, 2)
	is.Equal(1, values2["a"])
	is.Equal(2, values2["b"])
	is.Contains(missing2, "d")
	is.Contains(missing2, "e")

	// Test getting empty slice
	values3, missing3 := cache.GetMany([]string{})
	is.Empty(values3)
	is.Empty(missing3)
}

func TestPeekMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Test peeking multiple existing keys
	keys := []string{"a", "b", "c"}
	values, missing := cache.PeekMany(keys)

	is.Len(values, 3)
	is.Empty(missing)
	is.Equal(1, values["a"])
	is.Equal(2, values["b"])
	is.Equal(3, values["c"])

	// Test peeking mixed existing and non-existing keys
	keys2 := []string{"a", "d", "b", "e"}
	values2, missing2 := cache.PeekMany(keys2)

	is.Len(values2, 2)
	is.Len(missing2, 2)
	is.Equal(1, values2["a"])
	is.Equal(2, values2["b"])
	is.Contains(missing2, "d")
	is.Contains(missing2, "e")
}

func TestDeleteMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Test deleting multiple existing keys
	keys := []string{"a", "b"}
	results := cache.DeleteMany(keys)

	is.Len(results, 2)
	is.True(results["a"])
	is.True(results["b"])
	is.False(cache.Has("a"))
	is.False(cache.Has("b"))
	is.True(cache.Has("c"))
	is.Equal(1, cache.Len())

	// Test deleting mixed existing and non-existing keys
	cache.Set("d", 4)
	keys2 := []string{"c", "e", "d", "f"}
	results2 := cache.DeleteMany(keys2)

	is.Len(results2, 4)
	is.True(results2["c"])
	is.False(results2["e"])
	is.True(results2["d"])
	is.False(results2["f"])
	is.Equal(0, cache.Len())
}

func TestSetExistingKey(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Update existing key - should set visited=true
	cache.Set("a", 10)

	// Verify value was updated
	val, ok := cache.Get("a")
	is.True(ok)
	is.Equal(10, val)

	// "a" should be protected now
	// Add two more items to trigger evictions
	cache.Set("d", 4) // evicts "b"
	is.True(cache.Has("a"))
	is.False(cache.Has("b"))
	is.True(cache.Has("c"))
	is.True(cache.Has("d"))
}

func TestSIEVESecondChance(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Test the "second chance" behavior of SIEVE
	cache := NewSIEVECache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Access all items to mark them visited
	cache.Get("a")
	cache.Get("b")
	cache.Get("c")

	// Now add "d" - all items have visited=true
	// SIEVE should clear all visited bits and then evictAndCallback "a" (oldest after clearing)
	cache.Set("d", 4)

	// One of the original items should be evicted
	// After clearing all visited bits, "a" (oldest) should be evicted
	is.Equal(3, cache.Len())
	is.False(cache.Has("a"))
	is.True(cache.Has("b"))
	is.True(cache.Has("c"))
	is.True(cache.Has("d"))
}

func TestSIEVEHandWraparound(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Test that the hand pointer wraps around correctly
	cache := NewSIEVECache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Access "a" and "b" to mark them visited
	cache.Get("a")
	cache.Get("b")

	// Add "d" - should evictAndCallback "c" (only unvisited item)
	cache.Set("d", 4)
	is.True(cache.Has("a"))
	is.True(cache.Has("b"))
	is.False(cache.Has("c"))
	is.True(cache.Has("d"))

	// Now "a" and "b" should have visited=false (cleared during eviction scan)
	// Add "e" - should evictAndCallback "a" or "b" (whichever the hand points to)
	cache.Set("e", 5)
	is.Equal(3, cache.Len())
}

func TestDeleteHandElement(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Trigger an eviction to set the hand pointer
	cache.Get("a")
	cache.Get("b")
	cache.Get("c")
	cache.Set("d", 4) // This will evictAndCallback "a" and set hand

	// Manually delete what might be the hand element
	cache.Delete("b")
	is.Equal(2, cache.Len())

	// Cache should still work correctly
	cache.Set("e", 5)
	is.Equal(3, cache.Len())
}

func TestSizeBytes(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](10)
	is.GreaterOrEqual(cache.SizeBytes(), int64(0))

	cache.Set("a", 1)
	cache.Set("b", 2)
	is.Positive(cache.SizeBytes())
}

// verifyVisitedState is a helper to check the visited state of entries.
func verifyVisitedState[K comparable, V any](t *testing.T, cache *SIEVECache[K, V], key K, expectedVisited bool) {
	t.Helper()
	is := assert.New(t)

	e, ok := cache.cache[key]
	is.True(ok, "key %v should exist", key)
	is.Equal(expectedVisited, e.Value.visited, "key %v visited state", key)
}

func TestVisitedStateTracking(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewSIEVECache[string, int](5)

	// New entries should have visited=false
	cache.Set("a", 1)
	verifyVisitedState(t, cache, "a", false)

	// Get should set visited=true
	cache.Get("a")
	verifyVisitedState(t, cache, "a", true)

	// Peek should NOT set visited
	cache.Set("b", 2)
	verifyVisitedState(t, cache, "b", false)
	cache.Peek("b")
	verifyVisitedState(t, cache, "b", false)

	// Set existing key should set visited=true
	cache.Set("c", 3)
	verifyVisitedState(t, cache, "c", false)
	cache.Set("c", 30)
	verifyVisitedState(t, cache, "c", true)

	// Verify values are correct
	val, ok := cache.Get("c")
	is.True(ok)
	is.Equal(30, val)
}

func TestEvictionCallback(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	var evictedKeys []string
	var evictedValues []int
	var evictedReasons []base.EvictionReason

	cache := NewSIEVECacheWithEvictionCallback(2, func(reason base.EvictionReason, k string, v int) {
		evictedKeys = append(evictedKeys, k)
		evictedValues = append(evictedValues, v)
		evictedReasons = append(evictedReasons, reason)
	})

	cache.Set("a", 1)
	cache.Set("b", 2)
	is.Empty(evictedKeys)

	cache.Set("c", 3)
	is.Len(evictedKeys, 1)
	is.Equal("a", evictedKeys[0])
	is.Equal(1, evictedValues[0])
	is.Equal(base.EvictionReasonCapacity, evictedReasons[0])

	cache.Set("d", 4)
	is.Len(evictedKeys, 2)
	is.Equal("b", evictedKeys[1])
	is.Equal(2, evictedValues[1])
}
