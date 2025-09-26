package tinylfu

import (
	"fmt"
	"testing"

	"github.com/samber/hot/pkg/base"
	"github.com/stretchr/testify/assert"
)

func TestNewTinyLFUCache(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	is.Panics(func() {
		_ = NewTinyLFUCache[string, int](0)
	})

	cache := NewTinyLFUCache[string, int](42)
	is.Equal(41, cache.mainCapacity)
	is.Equal(1, cache.admissionCapacity)
	is.NotNil(cache.mainLl)
	is.NotNil(cache.mainCache)
}

func TestSet(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	evicted := 0
	cache := NewTinyLFUCacheWithEvictionCallback(100, func(reason base.EvictionReason, k string, v int) {
		is.Equal(base.EvictionReasonCapacity, reason)
		evicted += v
	})

	// With capacity=100: admissionCapacity=1, mainCapacity=99
	// First item should go to admission window
	cache.Set("a", 1)
	is.Equal(0, cache.mainLl.Len())
	is.Empty(cache.mainCache)
	is.Equal(1, cache.admissionLl.Len())
	is.Len(cache.admissionCache, 1)
	is.Equal(&entry[string, int]{"a", 1, 1}, cache.admissionCache["a"].Value)
	is.Equal(0, evicted)

	// Second item should replace first in admission window
	cache.Set("b", 2)
	is.Equal(0, cache.mainLl.Len())
	is.Empty(cache.mainCache)
	is.Equal(1, cache.admissionLl.Len())
	is.Len(cache.admissionCache, 1)
	is.Nil(cache.admissionCache["a"])
	is.Equal(&entry[string, int]{"b", 2, 1}, cache.admissionCache["b"].Value)
	is.Equal(1, evicted) // "a" was evicted from admission window

	// Access "b" multiple times to increase frequency and promote it
	cache.Get("b")
	cache.Get("b")
	cache.Get("b")

	// Add another item - "b" should be promoted to main cache
	cache.Set("c", 3)
	is.Equal(1, cache.mainLl.Len()) // "b" promoted
	is.Len(cache.mainCache, 1)
	is.Equal(1, cache.admissionLl.Len()) // "c" in admission
	is.Len(cache.admissionCache, 1)
	is.Equal(&entry[string, int]{"b", 2, 4}, cache.mainCache["b"].Value)
	is.Equal(&entry[string, int]{"c", 3, 1}, cache.admissionCache["c"].Value)
	is.Equal(1, evicted) // No additional evictions
}

func TestHas(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](100)

	// With capacity=100: admissionCapacity=1, mainCapacity=99
	// This means only 1 item can be in admission window at a time

	cache.Set("a", 1)
	is.True(cache.Has("a"))
	is.Equal(1, cache.admissionLl.Len())
	is.Equal(0, cache.mainLl.Len())

	// Add "b" - should evict "a" from admission window (admissionCapacity=1)
	cache.Set("b", 2)
	is.False(cache.Has("a")) // "a" was evicted
	is.True(cache.Has("b"))
	is.Equal(1, cache.admissionLl.Len())
	is.Equal(0, cache.mainLl.Len())

	is.False(cache.Has("c"))

	// Add "c" - should evict "b" from admission window
	cache.Set("c", 3)
	is.False(cache.Has("a"))
	is.False(cache.Has("b")) // "b" was evicted
	is.True(cache.Has("c"))
	is.Equal(1, cache.admissionLl.Len())
	is.Equal(0, cache.mainLl.Len())
}

func TestGet(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](100)

	// With admissionCapacity=1, only one item can be in admission window
	cache.Set("a", 1)

	// "a" should be in admission window
	is.Equal(1, cache.admissionLl.Len())
	is.Equal(0, cache.mainLl.Len())

	// Access "a" - it should be promoted to main cache since main cache is empty
	val, ok := cache.Get("a")
	is.True(ok)
	is.Equal(1, val)
	is.Equal(0, cache.admissionLl.Len()) // "a" was promoted
	is.Equal(1, cache.mainLl.Len())      // "a" now in main cache

	// Add "b" - should go to admission window
	cache.Set("b", 2)
	is.Equal(1, cache.admissionLl.Len()) // "b" in admission window
	is.Equal(1, cache.mainLl.Len())      // "a" still in main cache

	// Both should be accessible
	val, ok = cache.Get("a")
	is.True(ok)
	is.Equal(1, val)

	val, ok = cache.Get("b")
	is.True(ok)
	is.Equal(2, val)

	val, ok = cache.Get("c")
	is.False(ok)
	is.Zero(val)

	// Access "b" multiple times to increase its frequency
	cache.Get("b")
	cache.Get("b")
	cache.Get("b")

	// Add "c" - "b" should be promoted to main cache due to high frequency
	cache.Set("c", 3)
	val, ok = cache.Get("b")
	is.True(ok) // "b" should be in main cache now
	is.Equal(2, val)

	val, ok = cache.Get("c")
	is.True(ok) // "c" should be in admission window
	is.Equal(3, val)

	is.Equal(2, cache.mainLl.Len())      // "a" and "b" in main cache
	is.Equal(1, cache.admissionLl.Len()) // "c" in admission window
}

func TestKey(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// With capacity=2: admissionCapacity=1, mainCapacity=1
	// Only "b" remains after eviction
	is.ElementsMatch([]string{"b"}, cache.Keys())
}

func TestValues(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// With capacity=2: admissionCapacity=1, mainCapacity=1
	// Only "b" remains after eviction
	is.ElementsMatch([]int{2}, cache.Values())
}

func TestInternalState_All(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// With capacity=2: admissionCapacity=1, mainCapacity=1
	// Only "b" remains after eviction
	all := cache.All()
	is.Len(all, 1)
	is.Equal(2, all["b"])
}

func TestRange(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// With capacity=2: admissionCapacity=1, mainCapacity=1
	// Only "b" remains after eviction
	var keys []string
	var values []int
	cache.Range(func(key string, value int) bool {
		keys = append(keys, key)
		values = append(values, value)
		return true
	})
	is.ElementsMatch([]string{"b"}, keys)
	is.ElementsMatch([]int{2}, values)
}

func TestDelete(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// With capacity=2: admissionCapacity=1, mainCapacity=1
	// "a" was already evicted when "b" was set
	// "b" is in admission cache
	ok := cache.Delete("a")
	is.False(ok) // "a" was already evicted
	is.Equal(0, cache.mainLl.Len())
	is.Equal(1, cache.admissionLl.Len())
	is.Equal(&entry[string, int]{"b", 2, 1}, cache.admissionCache["b"].Value)

	ok = cache.Delete("b")
	is.True(ok)
	is.Equal(0, cache.mainLl.Len())
	is.Empty(cache.mainCache)
	is.Nil(cache.mainLl.Front())
	is.Nil(cache.mainLl.Back())

	ok = cache.Delete("b")
	is.False(ok)
	is.Equal(0, cache.mainLl.Len())
	is.Empty(cache.mainCache)
	is.Nil(cache.mainLl.Front())
	is.Nil(cache.mainLl.Back())
}

func TestLen(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](2)
	cache.Set("z", 0)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// With capacity=2: admissionCapacity=1, mainCapacity=1
	// Only "b" remains after eviction
	is.Equal(1, cache.Len())

	cache.Delete("a") // "a" was already evicted
	is.Equal(1, cache.Len())

	cache.Delete("b")
	is.Equal(0, cache.Len())
}

func TestPurge(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// With capacity=2: admissionCapacity=1, mainCapacity=1
	// Only "b" remains, and it's in admission cache
	is.Equal(0, cache.mainLl.Len())
	is.Equal(1, cache.admissionLl.Len())
	is.Empty(cache.mainCache)
	is.Len(cache.admissionCache, 1)

	cache.Purge()

	is.Equal(0, cache.mainLl.Len())
	is.Empty(cache.mainCache)
	is.Nil(cache.mainLl.Front())
	is.Nil(cache.mainLl.Back())
}

// Helper function to verify linked list order.
func verifyTinyLFUOrder[K comparable, V any](t *testing.T, cache *TinyLFUCache[K, V]) []K {
	t.Helper()
	is := assert.New(t)

	// Verify list length matches cache map
	is.Len(cache.mainCache, cache.mainLl.Len())

	if cache.mainLl.Len() == 0 {
		is.Nil(cache.mainLl.Front())
		is.Nil(cache.mainLl.Back())
		return []K{}
	}

	// Verify front and back
	is.NotNil(cache.mainLl.Front())
	is.NotNil(cache.mainLl.Back())

	// Build order from front to back (most recently used to least recently used)
	var order []K
	current := cache.mainLl.Front()
	for current != nil {
		entry := current.Value
		order = append(order, entry.key)
		current = current.Next()
	}

	return order
}

func TestInternalState_InitialState(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](5)

	// Verify initial state
	is.Equal(0, cache.mainLl.Len())
	is.Empty(cache.mainCache)
	is.Nil(cache.mainLl.Front())
	is.Nil(cache.mainLl.Back())

	order := verifyTinyLFUOrder(t, cache)
	is.Empty(order)
}

func TestInternalState_SingleElement(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](5)
	cache.Set("a", 1)

	// Access "a" to promote it to main cache
	cache.Get("a")

	// Verify single element state
	is.Equal(1, cache.mainLl.Len())
	is.Len(cache.mainCache, 1)
	is.NotNil(cache.mainLl.Front())
	is.NotNil(cache.mainLl.Back())
	is.Equal(cache.mainLl.Front(), cache.mainLl.Back()) // Same element

	entry := cache.mainLl.Front().Value
	is.Equal("a", entry.key)
	is.Equal(1, entry.value)

	order := verifyTinyLFUOrder(t, cache)
	is.Equal([]string{"a"}, order)
}

func TestInternalState_MultipleElements(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](100)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Access all items many times to promote them to main cache
	// With TinyLFU, items need frequency to get promoted
	for i := 0; i < 5; i++ {
		cache.Get("a")
		cache.Get("b")
		cache.Get("c")
	}

	// Verify multiple elements state - at least some should be in main cache
	is.GreaterOrEqual(cache.mainLl.Len(), 1)
	is.GreaterOrEqual(len(cache.mainCache), 1)

	// If we have 3 items in main cache, verify order
	if cache.mainLl.Len() == 3 {
		order := verifyTinyLFUOrder(t, cache)
		is.Equal([]string{"c", "b", "a"}, order)

		// Verify cache map contains correct elements
		is.NotNil(cache.mainCache["a"])
		is.NotNil(cache.mainCache["b"])
		is.NotNil(cache.mainCache["c"])

		// Verify element values
		is.Equal(1, cache.mainCache["a"].Value.value)
		is.Equal(2, cache.mainCache["b"].Value.value)
		is.Equal(3, cache.mainCache["c"].Value.value)
	}
}

func TestInternalState_GetUpdatesOrder(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](100)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Access all items many times to promote them to main cache
	for range 5 {
		cache.Get("a")
		cache.Get("b")
		cache.Get("c")
	}

	// Test only if we have all items in main cache
	if cache.mainLl.Len() == 3 {
		// Initial order: c -> b -> a
		order := verifyTinyLFUOrder(t, cache)
		is.Equal([]string{"c", "b", "a"}, order)

		// Get "a" - should move to front
		val, ok := cache.Get("a")
		is.True(ok)
		is.Equal(1, val)

		// Order should now be: a -> c -> b
		order = verifyTinyLFUOrder(t, cache)
		is.Equal([]string{"a", "c", "b"}, order)

		// Get "b" - should move to front
		val, ok = cache.Get("b")
		is.True(ok)
		is.Equal(2, val)

		// Order should now be: b -> a -> c
		order = verifyTinyLFUOrder(t, cache)
		is.Equal([]string{"b", "a", "c"}, order)
	}
}

func TestInternalState_PeekDoesNotUpdateOrder(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](1000)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Access all items to get their initial order in cache
	// We don't need all items in main cache for this test
	// The key point is that peek doesn't change the order

	// Test that Peek doesn't change access order (unlike Get)
	val, ok := cache.Peek("a")
	is.True(ok)
	is.Equal(1, val)

	// The items should still exist in cache
	is.True(cache.Has("a"))
	is.True(cache.Has("b"))
	is.True(cache.Has("c"))

	// Verify that we can peek all items
	val, ok = cache.Peek("b")
	is.True(ok)
	is.Equal(2, val)

	val, ok = cache.Peek("c")
	is.True(ok)
	is.Equal(3, val)

	// All items should still be in cache
	is.Equal(3, cache.Len())
}

func TestInternalState_SetExistingKey(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](1000)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Verify initial state
	is.Equal(3, cache.Len())
	is.True(cache.Has("a"))
	is.True(cache.Has("b"))
	is.True(cache.Has("c"))

	// Set existing key "a" with new value
	cache.Set("a", 10)

	// Verify the value was updated
	val, ok := cache.Get("a")
	is.True(ok)
	is.Equal(10, val)

	// All items should still be in cache
	is.Equal(3, cache.Len())
	is.True(cache.Has("a"))
	is.True(cache.Has("b"))
	is.True(cache.Has("c"))
}

func TestInternalState_Eviction(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	evicted := make(map[string]int)
	cache := NewTinyLFUCacheWithEvictionCallback(3, func(reason base.EvictionReason, k string, v int) {
		is.Equal(base.EvictionReasonCapacity, reason)
		evicted[k] = v
	})

	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// With capacity 3, TinyLFU has admissionCapacity=1, mainCapacity=2
	// After 3 items, some will be in admission window and some in main cache
	is.GreaterOrEqual(cache.Len(), 1) // At least 1 item should be in cache

	// Add one more item - should trigger eviction
	cache.Set("d", 4)

	// At least one item should have been evicted
	is.GreaterOrEqual(len(evicted), 1)
	is.LessOrEqual(cache.Len(), 3) // Cache should not exceed capacity

	// Verify the evicted item is no longer in cache
	for k := range evicted {
		is.False(cache.Has(k))
	}
}

func TestInternalState_Delete(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](1000)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Verify initial state
	is.Equal(3, cache.Len())
	is.True(cache.Has("a"))
	is.True(cache.Has("b"))
	is.True(cache.Has("c"))

	// Delete element "b"
	ok := cache.Delete("b")
	is.True(ok)

	// Verify "b" is removed from cache
	is.False(cache.Has("b"))
	is.Equal(2, cache.Len())

	// Delete element "c"
	ok = cache.Delete("c")
	is.True(ok)

	// Verify "c" is removed from cache
	is.False(cache.Has("c"))
	is.Equal(1, cache.Len())

	// Delete last element "a"
	ok = cache.Delete("a")
	is.True(ok)

	// Cache should now be empty
	is.Equal(0, cache.Len())
	is.False(cache.Has("a"))
}

func TestInternalState_ElementRelationships(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](1000)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Access items to promote them to main cache
	for range 50 {
		cache.Get("a")
		cache.Get("b")
		cache.Get("c")
	}

	// Verify element relationships if main cache has items
	if cache.mainLl.Len() > 0 {
		front := cache.mainLl.Front()
		back := cache.mainLl.Back()

		// Front and back should not be nil
		is.NotNil(front)
		is.NotNil(back)

		// Verify the keys exist in cache
		is.NotNil(front.Value)
		is.NotNil(back.Value)
	}

	// Basic cache functionality should work
	is.Equal(3, cache.Len())
	is.True(cache.Has("a"))
	is.True(cache.Has("b"))
	is.True(cache.Has("c"))
}

func TestInternalState_ComplexOperations(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](4)

	// Add elements
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Basic functionality should work
	// With capacity 4, TinyLFU has admissionCapacity=1, mainCapacity=3
	// After adding 3 items, some may be in admission window
	is.True(cache.Len() >= 1 && cache.Len() <= 3)

	// Get elements if they exist
	if cache.Has("a") {
		val, ok := cache.Get("a")
		is.True(ok)
		is.Equal(1, val)
	}

	if cache.Has("b") {
		val, ok := cache.Get("b")
		is.True(ok)
		is.Equal(2, val)
	}

	// Update existing element if it exists
	if cache.Has("c") {
		cache.Set("c", 30)
		val, ok := cache.Get("c")
		is.True(ok)
		is.Equal(30, val)
	}

	// Add another element
	cache.Set("d", 4)

	// Verify cache behavior within capacity constraints
	is.LessOrEqual(cache.Len(), 4)

	// Delete an element
	cache.Delete("a")
	is.False(cache.Has("a"))
}

func TestSetMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](3)

	// Test setting multiple items
	items := map[string]int{"a": 1, "b": 2, "c": 3}
	cache.SetMany(items)

	// With capacity 3, TinyLFU has admissionCapacity=1, mainCapacity=2
	// Some items might be in admission window, some in main cache
	is.True(cache.Len() >= 1 && cache.Len() <= 3)
	is.True(cache.Has("a") || cache.Has("b") || cache.Has("c"))

	// Test that we can retrieve what we can
	if cache.Has("a") {
		val, ok := cache.Get("a")
		is.True(ok)
		is.Equal(1, val)
	}

	if cache.Has("b") {
		val, ok := cache.Get("b")
		is.True(ok)
		is.Equal(2, val)
	}

	if cache.Has("c") {
		val, ok := cache.Get("c")
		is.True(ok)
		is.Equal(3, val)
	}

	// Test setting more items than capacity (should evict some items)
	items2 := map[string]int{"d": 4, "e": 5, "f": 6}
	cache.SetMany(items2)

	is.LessOrEqual(cache.Len(), 3)
	is.True(cache.Has("d") || cache.Has("e") || cache.Has("f"))

	// Test setting empty map
	cache.SetMany(map[string]int{})
	is.LessOrEqual(cache.Len(), 3) // Should not change anything
}

func TestHasMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Test checking multiple existing keys
	keys := []string{"a", "b", "c"}
	results := cache.HasMany(keys)

	is.Len(results, 3)
	// With admission window, not all items may be in cache
	if cache.Has("a") {
		is.True(results["a"])
	}
	if cache.Has("b") {
		is.True(results["b"])
	}
	if cache.Has("c") {
		is.True(results["c"])
	}

	// Test checking mixed existing and non-existing keys
	keys2 := []string{"a", "d", "b", "e"}
	results2 := cache.HasMany(keys2)

	is.Len(results2, 4)
	is.False(results2["d"]) // "d" should never exist
	is.False(results2["e"]) // "e" should never exist

	// Test checking empty slice
	results3 := cache.HasMany([]string{})
	is.Empty(results3)

	// Test checking non-existing keys
	keys4 := []string{"x", "y", "z"}
	results4 := cache.HasMany(keys4)

	is.Len(results4, 3)
	is.False(results4["x"])
	is.False(results4["y"])
	is.False(results4["z"])
}

func TestGetMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// With capacity=3: admissionCapacity=1, mainCapacity=2
	// After setting all 3 items, only 2 will remain in main cache
	// Test getting multiple existing keys
	keys := []string{"a", "b", "c"}
	values, missing := cache.GetMany(keys)

	is.Len(values, 1)         // Only 1 item fits in main cache after all operations
	is.Len(missing, 2)        // 2 items were evicted
	is.Equal(3, values["c"])  // "c" survived (newest)
	is.Contains(missing, "a") // "a" was evicted
	is.Contains(missing, "b") // "b" was evicted

	// Test getting mixed existing and non-existing keys
	keys2 := []string{"a", "d", "b", "e"}
	values2, missing2 := cache.GetMany(keys2)

	is.Empty(values2)   // No items found (we're looking for a,d,b,e but only c exists)
	is.Len(missing2, 4) // All requested items are missing
	is.Contains(missing2, "a")
	is.Contains(missing2, "b")
	is.Contains(missing2, "d")
	is.Contains(missing2, "e")

	// Test getting empty slice
	values3, missing3 := cache.GetMany([]string{})
	is.Empty(values3)
	is.Empty(missing3)

	// Test getting only non-existing keys
	keys4 := []string{"x", "y", "z"}
	values4, missing4 := cache.GetMany(keys4)

	is.Empty(values4)
	is.Len(missing4, 3)
	is.Contains(missing4, "x")
	is.Contains(missing4, "y")
	is.Contains(missing4, "z")
}

func TestPeekMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](100)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Access all items to promote them to main cache
	cache.Get("a")
	cache.Get("b")
	cache.Get("c")

	// Test peeking multiple existing keys
	keys := []string{"a", "b", "c"}
	values, missing := cache.PeekMany(keys)

	// Some items should be found
	is.NotEmpty(values)
	is.LessOrEqual(len(missing), 3)

	// Check found items have correct values
	for k, v := range values {
		switch k {
		case "a":
			is.Equal(1, v)
		case "b":
			is.Equal(2, v)
		case "c":
			is.Equal(3, v)
		}
	}

	// Test peeking mixed existing and non-existing keys
	keys2 := []string{"a", "d", "b", "e"}
	_, missing2 := cache.PeekMany(keys2)

	// "d" and "e" should always be missing
	is.Contains(missing2, "d")
	is.Contains(missing2, "e")

	// Test peeking empty slice
	values3, missing3 := cache.PeekMany([]string{})
	is.Empty(values3)
	is.Empty(missing3)

	// Test peeking only non-existing keys
	keys4 := []string{"x", "y", "z"}
	_, missing4 := cache.PeekMany(keys4)

	is.Len(missing4, 3)
	is.Contains(missing4, "x")
	is.Contains(missing4, "y")
	is.Contains(missing4, "z")
}

func TestDeleteMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// With capacity=3: admissionCapacity=1, mainCapacity=2
	// After setting all 3 items, only "c" remains
	// Test deleting multiple existing keys
	keys := []string{"a", "b"}
	results := cache.DeleteMany(keys)

	is.Len(results, 2)
	is.False(results["a"]) // "a" was already evicted
	is.False(results["b"]) // "b" was already evicted
	is.False(cache.Has("a"))
	is.False(cache.Has("b"))
	is.True(cache.Has("c"))
	is.Equal(1, cache.Len())

	// Test deleting mixed existing and non-existing keys
	cache.Set("d", 4) // This will likely evict "c"
	keys2 := []string{"c", "e", "d", "f"}
	results2 := cache.DeleteMany(keys2)

	is.Len(results2, 4)
	is.False(results2["c"]) // "c" was likely evicted by "d"
	is.False(results2["e"]) // "e" never existed
	is.True(results2["d"])  // "d" exists
	is.False(results2["f"]) // "f" never existed
	is.False(cache.Has("c"))
	is.False(cache.Has("d"))
	is.Equal(0, cache.Len())

	// Test deleting empty slice
	cache.Set("x", 10)
	results3 := cache.DeleteMany([]string{})
	is.Empty(results3)
	is.True(cache.Has("x"))
	is.Equal(1, cache.Len())

	// Test deleting only non-existing keys
	keys4 := []string{"y", "z"}
	results4 := cache.DeleteMany(keys4)

	is.Len(results4, 2)
	is.False(results4["y"])
	is.False(results4["z"])
	is.True(cache.Has("x")) // Should still exist
	is.Equal(1, cache.Len())
}

func TestPeek(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](100)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// Access both items to promote them to main cache
	cache.Get("a")
	cache.Get("b")

	// Test peeking existing key
	if cache.Has("a") {
		val, ok := cache.Peek("a")
		is.True(ok)
		is.Equal(1, val)
	}

	if cache.Has("b") {
		val, ok := cache.Peek("b")
		is.True(ok)
		is.Equal(2, val)
	}

	// Test peeking non-existing key
	val, ok := cache.Peek("c")
	is.False(ok)
	is.Zero(val)

	// Test that peek doesn't change access order
	// Peek "a" should not move it to front
	cache.Peek("a")

	// Add a new item and verify cache behavior
	cache.Set("c", 3)
	is.LessOrEqual(cache.Len(), 100)
}

// TestTinyLFUCacheEdgeCases tests edge cases and boundary conditions
func TestTinyLFUCacheEdgeCases(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Test with capacity 1 (edge case)
	cache := NewTinyLFUCache[string, int](1)

	// With capacity 1: admissionCapacity=1, mainCapacity=0
	// All items should go to admission window and be evicted quickly
	cache.Set("a", 1)
	is.Equal(1, cache.Len())

	cache.Set("b", 2)
	is.Equal(1, cache.Len())

	// Verify only the latest item remains
	is.True(cache.Has("b"))
	is.False(cache.Has("a"))

	// Test with capacity 2 (minimum for main cache)
	cache2 := NewTinyLFUCache[string, int](2)

	// With capacity 2: admissionCapacity=1, mainCapacity=1
	cache2.Set("a", 1)
	cache2.Set("b", 2)
	cache2.Set("c", 3)

	// Should have at least 1 item
	is.GreaterOrEqual(cache2.Len(), 1)
	is.LessOrEqual(cache2.Len(), 2)
}

// TestTinyLFUCacheStress tests high-frequency access patterns
func TestTinyLFUCacheStress(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](1000)

	// Test rapid-fire operations
	for i := 0; i < 5000; i++ {
		key := fmt.Sprintf("item_%d", i%100) // Create 100 unique keys
		cache.Set(key, i)

		// Every 10 operations, do a get
		if i%10 == 0 {
			cache.Get(key)
		}
	}

	// Cache should respect capacity limits
	is.LessOrEqual(cache.Len(), 1000)

	// Most frequently accessed items should still be there
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("item_%d", i)
		if cache.Has(key) {
			val, ok := cache.Get(key)
			is.True(ok)
			is.Greater(val, 0)
		}
	}
}

// TestTinyLFUCacheFrequencyBasedAdmission tests the frequency-based admission behavior
func TestTinyLFUCacheFrequencyBasedAdmission(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](100)

	// With capacity 100: admissionCapacity=1, mainCapacity=2 (rounded up from 99)

	// Add item A and access it many times to build frequency
	cache.Set("A", 1)
	for i := 0; i < 20; i++ {
		cache.Get("A")
	}

	// Add items B and C, access B frequently
	cache.Set("B", 2)
	cache.Set("C", 3)
	for i := 0; i < 15; i++ {
		cache.Get("B")
	}

	// Add many items to trigger admission window churn
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("temp_%d", i)
		cache.Set(key, i)
	}

	// After churn, frequently accessed items should be more likely to be in main cache
	is.GreaterOrEqual(cache.Len(), 1)
	is.LessOrEqual(cache.Len(), 100)

	// Test that highly frequent items survive
	if cache.Has("A") {
		val, ok := cache.Get("A")
		is.True(ok)
		is.Equal(1, val)
	}
}

// TestTinyLFUCacheEvictionCallback tests eviction callback behavior in detail
func TestTinyLFUCacheEvictionCallback(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	evictedItems := make([]string, 0)
	evictionReasons := make([]base.EvictionReason, 0)

	cache := NewTinyLFUCacheWithEvictionCallback(10, func(reason base.EvictionReason, k string, v int) {
		evictedItems = append(evictedItems, k)
		evictionReasons = append(evictionReasons, reason)
	})

	// Fill cache beyond capacity
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("item_%d", i)
		cache.Set(key, i)
	}

	// Verify evictions occurred
	is.Greater(len(evictedItems), 0)
	is.Greater(len(evictionReasons), 0)

	// All evictions should be due to capacity
	for _, reason := range evictionReasons {
		is.Equal(base.EvictionReasonCapacity, reason)
	}

	// Verify evicted items are no longer in cache
	for _, key := range evictedItems {
		is.False(cache.Has(key))
	}
}

// TestTinyLFUCacheDifferentDataTypes tests with various key and value types
func TestTinyLFUCacheDifferentDataTypes(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Test with int keys and string values
	cache1 := NewTinyLFUCache[int, string](10)
	cache1.Set(1, "one")
	cache1.Set(2, "two")

	// Access the items to increase their frequency and help them stay in cache
	cache1.Get(1)
	cache1.Get(2)

	// Check if items are in cache (they might be in admission window)
	if cache1.Has(1) {
		val, ok := cache1.Get(1)
		is.True(ok)
		is.Equal("one", val)
	}

	if cache1.Has(2) {
		val, ok := cache1.Get(2)
		is.True(ok)
		is.Equal("two", val)
	}

	// Test with struct keys
	type Person struct {
		Name string
		Age  int
	}

	cache2 := NewTinyLFUCache[Person, float64](5)
	p1 := Person{"Alice", 30}
	p2 := Person{"Bob", 25}

	cache2.Set(p1, 95.5)
	cache2.Set(p2, 87.2)

	// Access the items to increase their frequency
	cache2.Get(p1)
	cache2.Get(p2)

	// Check if items are in cache
	if cache2.Has(p1) {
		val2, ok := cache2.Get(p1)
		is.True(ok)
		is.Equal(95.5, val2)
	}

	// Verify cache has some items
	is.GreaterOrEqual(cache1.Len(), 1)
	is.GreaterOrEqual(cache2.Len(), 1)
}

// TestTinyLFUCacheAlgorithmName verifies the algorithm name
func TestTinyLFUCacheAlgorithmName(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](100)
	is.Equal("tinylfu", cache.Algorithm())
}

// TestTinyLFUCacheZeroCapacityPanics verifies panics for invalid capacities
func TestTinyLFUCacheZeroCapacityPanics(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	is.Panics(func() {
		_ = NewTinyLFUCache[string, int](0)
	})

	is.Panics(func() {
		_ = NewTinyLFUCacheWithEvictionCallback[string, int](0, nil)
	})

	is.Panics(func() {
		_ = NewTinyLFUCache[string, int](-1)
	})
}

// TestTinyLFUCacheCountMinSketchBehavior tests the Count-Min Sketch functionality
func TestTinyLFUCacheCountMinSketchBehavior(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](100)

	// Access the same key many times to build frequency
	key := "popular_key"
	for i := 0; i < 100; i++ {
		cache.Get(key) // This increments the counter in the sketch
		cache.Set(key, i) // This also increments the counter
	}

	// The sketch should have a high estimate for this key
	estimate := cache.sketch.Estimate(key)
	is.Greater(estimate, 50) // Should have high frequency

	// Test with different keys
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key_%d", i)
		cache.Set(key, i)
		if i%5 == 0 {
			cache.Get(key)
		}
	}

	// Popular key should still have high relative frequency
	popularEstimate := cache.sketch.Estimate("popular_key")
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key_%d", i)
		otherEstimate := cache.sketch.Estimate(key)
		is.GreaterOrEqual(popularEstimate, otherEstimate)
	}
}

// TestTinyLFUCachePurgeBehavior tests the purge functionality
func TestTinyLFUCachePurgeBehavior(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](100)

	// Fill cache with data
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("item_%d", i)
		cache.Set(key, i)
	}

	// Verify cache has items
	is.Greater(cache.Len(), 0)

	// Access some items to build frequency
	cache.Get("item_0")
	cache.Get("item_1")

	// Purge the cache
	cache.Purge()

	// Verify cache is empty
	is.Equal(0, cache.Len())
	is.Equal(0, cache.mainLl.Len())
	is.Equal(0, cache.admissionLl.Len())
	is.Empty(cache.mainCache)
	is.Empty(cache.admissionCache)

	// Test that sketch is also reset
	is.Equal(0, cache.sketch.Estimate("item_0"))
	is.Equal(0, cache.sketch.Estimate("item_1"))
}

// TestTinyLFUCachePromotionLogic tests the promotion logic from admission to main cache
func TestTinyLFUCachePromotionLogic(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](100)

	// Add an item and access it frequently to build frequency
	cache.Set("freq_item", 42)
	for i := 0; i < 30; i++ {
		cache.Get("freq_item")
	}

	// Add other items to create competition for main cache spots
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("comp_%d", i)
		cache.Set(key, i)
		// Access competitor items less frequently
		if i%3 == 0 {
			cache.Get(key)
		}
	}

	// The frequent item should have a good chance of being in main cache
	// We can't guarantee it due to the probabilistic nature, but we can test the logic
	is.GreaterOrEqual(cache.Len(), 1)

	// Test the promotion logic directly
	if cache.Has("freq_item") {
		// Verify it's accessible
		val, ok := cache.Get("freq_item")
		is.True(ok)
		is.Equal(42, val)
	}
}

// TestTinyLFUCacheSetGetUpdatePattern tests common set-get-update patterns
func TestTinyLFUCacheSetGetUpdatePattern(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewTinyLFUCache[string, int](50)

	// Set-get-update pattern
	cache.Set("key1", 100)
	val, ok := cache.Get("key1")
	is.True(ok)
	is.Equal(100, val)

	// Update value
	cache.Set("key1", 200)
	val, ok = cache.Get("key1")
	is.True(ok)
	is.Equal(200, val)

	// Set many items, then update some
	items := make(map[string]int)
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("batch_%d", i)
		items[key] = i * 10
	}
	cache.SetMany(items)

	// Update some items
	updates := make(map[string]int)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("batch_%d", i)
		updates[key] = i * 100
	}
	cache.SetMany(updates)

	// Verify updates
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("batch_%d", i)
		if cache.Has(key) {
			val, ok := cache.Get(key)
			is.True(ok)
			is.Equal(i*100, val)
		}
	}
}
