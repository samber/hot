package lru

import (
	"testing"

	"github.com/samber/hot/pkg/base"
	"github.com/stretchr/testify/assert"
)

func TestNewLRUCache(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	is.Panics(func() {
		_ = NewLRUCache[string, int](0)
	})

	cache := NewLRUCache[string, int](42)
	is.Equal(42, cache.capacity)
	is.NotNil(cache.ll)
	is.NotNil(cache.cache)
}

func TestSet(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	evicted := 0
	cache := NewLRUCacheWithEvictionCallback(2, func(reason base.EvictionReason, k string, v int) {
		is.Equal(base.EvictionReasonCapacity, reason)
		evicted += v
	})

	cache.Set("a", 1)
	is.Equal(1, cache.ll.Len())
	is.Len(cache.cache, 1)
	is.Equal(&entry[string, int]{"a", 1}, cache.cache["a"].Value)
	is.Equal(&entry[string, int]{"a", 1}, cache.ll.Front().Value)
	is.Equal(&entry[string, int]{"a", 1}, cache.ll.Back().Value)
	is.Equal(0, evicted)

	cache.Set("b", 2)
	is.Equal(2, cache.ll.Len())
	is.Len(cache.cache, 2)
	is.Equal(&entry[string, int]{"a", 1}, cache.cache["a"].Value)
	is.Equal(&entry[string, int]{"b", 2}, cache.cache["b"].Value)
	is.Equal(&entry[string, int]{"b", 2}, cache.ll.Front().Value)
	is.Equal(&entry[string, int]{"a", 1}, cache.ll.Back().Value)
	is.Equal(0, evicted)

	cache.Set("c", 3)
	is.Equal(2, cache.ll.Len())
	is.Len(cache.cache, 2)
	is.Equal(&entry[string, int]{"b", 2}, cache.cache["b"].Value)
	is.Equal(&entry[string, int]{"c", 3}, cache.cache["c"].Value)
	is.Equal(&entry[string, int]{"c", 3}, cache.ll.Front().Value)
	is.Equal(&entry[string, int]{"b", 2}, cache.ll.Back().Value)
	is.Equal(1, evicted)
}

func TestHas(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.True(cache.Has("a"))
	is.True(cache.Has("b"))
	is.False(cache.Has("c"))

	cache.Set("c", 3)
	is.False(cache.Has("a"))
	is.True(cache.Has("b"))
	is.True(cache.Has("c"))
}

func TestGet(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](2)
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

	cache.Set("c", 3)
	val, ok = cache.Get("a")
	is.False(ok)
	is.Zero(val)

	val, ok = cache.Get("b")
	is.True(ok)
	is.Equal(2, val)

	val, ok = cache.Get("c")
	is.True(ok)
	is.Equal(3, val)
}

func TestKey(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.ElementsMatch([]string{"b", "a"}, cache.Keys())
}

func TestValues(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.ElementsMatch([]int{1, 2}, cache.Values())
}

func TestInternalState_All(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](2)
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

	cache := NewLRUCache[string, int](2)
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

func TestDelete(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	ok := cache.Delete("a")
	is.True(ok)
	is.Equal(1, cache.ll.Len())
	is.Len(cache.cache, 1)
	is.Equal(&entry[string, int]{"b", 2}, cache.cache["b"].Value)
	is.Equal(&entry[string, int]{"b", 2}, cache.ll.Front().Value)
	is.Equal(&entry[string, int]{"b", 2}, cache.ll.Back().Value)

	ok = cache.Delete("b")
	is.True(ok)
	is.Equal(0, cache.ll.Len())
	is.Empty(cache.cache)
	is.Nil(cache.ll.Front())
	is.Nil(cache.ll.Back())

	ok = cache.Delete("b")
	is.False(ok)
	is.Equal(0, cache.ll.Len())
	is.Empty(cache.cache)
	is.Nil(cache.ll.Front())
	is.Nil(cache.ll.Back())
}

func TestDeleteOldest(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	key, value, ok := cache.DeleteOldest()
	is.True(ok)
	is.Equal("a", key)
	is.Equal(1, value)
	is.Equal(1, cache.ll.Len())
	is.Len(cache.cache, 1)
	is.Equal(&entry[string, int]{"b", 2}, cache.cache["b"].Value)
	is.Equal(&entry[string, int]{"b", 2}, cache.ll.Front().Value)
	is.Equal(&entry[string, int]{"b", 2}, cache.ll.Back().Value)

	key, value, ok = cache.DeleteOldest()
	is.True(ok)
	is.Equal("b", key)
	is.Equal(2, value)
	is.Equal(0, cache.ll.Len())
	is.Empty(cache.cache)
	is.Nil(cache.ll.Front())
	is.Nil(cache.ll.Back())

	key, value, ok = cache.DeleteOldest()
	is.False(ok)
	is.Empty(key)
	is.Zero(value)
	is.False(ok)
	is.Empty(key)
	is.Zero(value)
	is.Equal(0, cache.ll.Len())
	is.Empty(cache.cache)
	is.Nil(cache.ll.Front())
	is.Nil(cache.ll.Back())
}

func TestLen(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](2)
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

	cache := NewLRUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.Equal(2, cache.ll.Len())
	is.Len(cache.cache, 2)

	cache.Purge()

	is.Equal(0, cache.ll.Len())
	is.Empty(cache.cache)
	is.Nil(cache.ll.Front())
	is.Nil(cache.ll.Back())
}

// Helper function to verify linked list order.
func verifyLRUOrder[K comparable, V any](t *testing.T, cache *LRUCache[K, V]) []K {
	t.Helper()
	is := assert.New(t)

	// Verify list length matches cache map
	is.Len(cache.cache, cache.ll.Len())

	if cache.ll.Len() == 0 {
		is.Nil(cache.ll.Front())
		is.Nil(cache.ll.Back())
		return []K{}
	}

	// Verify front and back
	is.NotNil(cache.ll.Front())
	is.NotNil(cache.ll.Back())

	// Build order from front to back (most recently used to least recently used)
	var order []K
	current := cache.ll.Front()
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

	cache := NewLRUCache[string, int](5)

	// Verify initial state
	is.Equal(0, cache.ll.Len())
	is.Empty(cache.cache)
	is.Nil(cache.ll.Front())
	is.Nil(cache.ll.Back())

	order := verifyLRUOrder(t, cache)
	is.Empty(order)
}

func TestInternalState_SingleElement(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](5)
	cache.Set("a", 1)

	// Verify single element state
	is.Equal(1, cache.ll.Len())
	is.Len(cache.cache, 1)
	is.NotNil(cache.ll.Front())
	is.NotNil(cache.ll.Back())
	is.Equal(cache.ll.Front(), cache.ll.Back()) // Same element

	entry := cache.ll.Front().Value
	is.Equal("a", entry.key)
	is.Equal(1, entry.value)

	order := verifyLRUOrder(t, cache)
	is.Equal([]string{"a"}, order)
}

func TestInternalState_MultipleElements(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Verify multiple elements state
	is.Equal(3, cache.ll.Len())
	is.Len(cache.cache, 3)

	// Order should be: c (most recent) -> b -> a (least recent)
	order := verifyLRUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Verify cache map contains correct elements
	is.NotNil(cache.cache["a"])
	is.NotNil(cache.cache["b"])
	is.NotNil(cache.cache["c"])

	// Verify element values
	is.Equal(1, cache.cache["a"].Value.value)
	is.Equal(2, cache.cache["b"].Value.value)
	is.Equal(3, cache.cache["c"].Value.value)
}

func TestInternalState_GetUpdatesOrder(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Initial order: c -> b -> a
	order := verifyLRUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Get "a" - should move to front
	val, ok := cache.Get("a")
	is.True(ok)
	is.Equal(1, val)

	// Order should now be: a -> c -> b
	order = verifyLRUOrder(t, cache)
	is.Equal([]string{"a", "c", "b"}, order)

	// Get "b" - should move to front
	val, ok = cache.Get("b")
	is.True(ok)
	is.Equal(2, val)

	// Order should now be: b -> a -> c
	order = verifyLRUOrder(t, cache)
	is.Equal([]string{"b", "a", "c"}, order)
}

func TestInternalState_PeekDoesNotUpdateOrder(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Initial order: c -> b -> a
	order := verifyLRUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Peek "a" - should not change order
	val, ok := cache.Peek("a")
	is.True(ok)
	is.Equal(1, val)

	// Order should remain: c -> b -> a
	order = verifyLRUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Peek "b" - should not change order
	val, ok = cache.Peek("b")
	is.True(ok)
	is.Equal(2, val)

	// Order should remain: c -> b -> a
	order = verifyLRUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)
}

func TestInternalState_SetExistingKey(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Initial order: c -> b -> a
	order := verifyLRUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Set existing key "a" with new value
	cache.Set("a", 10)

	// Order should now be: a -> c -> b (a moved to front)
	order = verifyLRUOrder(t, cache)
	is.Equal([]string{"a", "c", "b"}, order)

	// Verify value was updated
	is.Equal(10, cache.cache["a"].Value.value)
}

func TestInternalState_Eviction(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	evicted := make(map[string]int)
	cache := NewLRUCacheWithEvictionCallback(3, func(reason base.EvictionReason, k string, v int) {
		is.Equal(base.EvictionReasonCapacity, reason)
		evicted[k] = v
	})

	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Order: c -> b -> a
	order := verifyLRUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Add one more - should evict "a" (least recently used)
	cache.Set("d", 4)

	// Order should now be: d -> c -> b
	order = verifyLRUOrder(t, cache)
	is.Equal([]string{"d", "c", "b"}, order)

	// Verify "a" was evicted
	is.Equal(1, evicted["a"])
	is.Nil(cache.cache["a"])
}

func TestInternalState_Delete(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Initial order: c -> b -> a
	order := verifyLRUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Delete middle element "b"
	ok := cache.Delete("b")
	is.True(ok)

	// Order should now be: c -> a
	order = verifyLRUOrder(t, cache)
	is.Equal([]string{"c", "a"}, order)

	// Verify "b" is removed from cache map
	is.Nil(cache.cache["b"])

	// Delete front element "c"
	ok = cache.Delete("c")
	is.True(ok)

	// Order should now be: a
	order = verifyLRUOrder(t, cache)
	is.Equal([]string{"a"}, order)

	// Delete last element "a"
	ok = cache.Delete("a")
	is.True(ok)

	// Order should now be empty
	order = verifyLRUOrder(t, cache)
	is.Empty(order)
}

func TestInternalState_DeleteOldest(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Initial order: c -> b -> a
	order := verifyLRUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Delete oldest (back of list)
	key, value, ok := cache.DeleteOldest()
	is.True(ok)
	is.Equal("a", key)
	is.Equal(1, value)

	// Order should now be: c -> b
	order = verifyLRUOrder(t, cache)
	is.Equal([]string{"c", "b"}, order)

	// Delete oldest again
	key, value, ok = cache.DeleteOldest()
	is.True(ok)
	is.Equal("b", key)
	is.Equal(2, value)

	// Order should now be: c
	order = verifyLRUOrder(t, cache)
	is.Equal([]string{"c"}, order)

	// Delete oldest again
	key, value, ok = cache.DeleteOldest()
	is.True(ok)
	is.Equal("c", key)
	is.Equal(3, value)

	// Order should now be empty
	order = verifyLRUOrder(t, cache)
	is.Empty(order)

	// Try to delete from empty cache
	key, value, ok = cache.DeleteOldest()
	is.Empty(key)
	is.Zero(value)
	is.False(ok)
}

func TestInternalState_ElementRelationships(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Verify element relationships
	front := cache.ll.Front()
	back := cache.ll.Back()

	// Front should be "c" (most recent)
	is.Equal("c", front.Value.key)

	// Back should be "a" (least recent)
	is.Equal("a", back.Value.key)

	// Verify Next/Prev relationships
	middle := front.Next()
	is.Equal("b", middle.Value.key)
	is.Equal(front, middle.Prev())
	is.Equal(back, middle.Next())

	// Verify back element
	is.Equal(middle, back.Prev())
	is.Nil(back.Next())
	is.Nil(front.Prev())
}

func TestInternalState_ComplexOperations(t *testing.T) {
	t.Parallel()
	cache := NewLRUCache[string, int](4)

	// Add elements
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Order: c -> b -> a
	order := verifyLRUOrder(t, cache)
	assert.Equal(t, []string{"c", "b", "a"}, order)

	// Get middle element
	cache.Get("b")

	// Order: b -> c -> a
	order = verifyLRUOrder(t, cache)
	assert.Equal(t, []string{"b", "c", "a"}, order)

	// Add another element
	cache.Set("d", 4)

	// Order: d -> b -> c -> a
	order = verifyLRUOrder(t, cache)
	assert.Equal(t, []string{"d", "b", "c", "a"}, order)

	// Update existing element
	cache.Set("c", 30)

	// Order: c -> d -> b -> a
	order = verifyLRUOrder(t, cache)
	assert.Equal(t, []string{"c", "d", "b", "a"}, order)

	// Add one more - should evict "a"
	cache.Set("e", 5)

	// Order: e -> c -> d -> b
	order = verifyLRUOrder(t, cache)
	assert.Equal(t, []string{"e", "c", "d", "b"}, order)

	// Delete middle element
	cache.Delete("d")

	// Order: e -> c -> b
	order = verifyLRUOrder(t, cache)
	assert.Equal(t, []string{"e", "c", "b"}, order)
}

func TestSetMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](3)

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

	val, ok = cache.Get("b")
	is.True(ok)
	is.Equal(2, val)

	val, ok = cache.Get("c")
	is.True(ok)
	is.Equal(3, val)

	// Test setting more items than capacity (should evict oldest)
	items2 := map[string]int{"d": 4, "e": 5, "f": 6}
	cache.SetMany(items2)

	is.Equal(3, cache.Len())
	is.False(cache.Has("a"))
	is.False(cache.Has("b"))
	is.False(cache.Has("c"))
	is.True(cache.Has("d"))
	is.True(cache.Has("e"))
	is.True(cache.Has("f"))

	// Test setting empty map
	cache.SetMany(map[string]int{})
	is.Equal(3, cache.Len()) // Should not change anything
}

func TestHasMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](3)
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

	cache := NewLRUCache[string, int](3)
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

	cache := NewLRUCache[string, int](3)
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

	// Test peeking empty slice
	values3, missing3 := cache.PeekMany([]string{})
	is.Empty(values3)
	is.Empty(missing3)

	// Test peeking only non-existing keys
	keys4 := []string{"x", "y", "z"}
	values4, missing4 := cache.PeekMany(keys4)

	is.Empty(values4)
	is.Len(missing4, 3)
	is.Contains(missing4, "x")
	is.Contains(missing4, "y")
	is.Contains(missing4, "z")

	// Verify that peeking doesn't change access order
	// Get "a" to move it to front
	cache.Get("a")

	// Peek "b" and "c" - they should still be in original order
	peekKeys := []string{"b", "c"}
	peekValues, _ := cache.PeekMany(peekKeys)
	is.Len(peekValues, 2)
	is.Equal(2, peekValues["b"])
	is.Equal(3, peekValues["c"])

	// Verify order hasn't changed by checking that "b" is still the oldest
	// and will be evicted when we add a new item
	// Order after Get("a"): a (front), c, b (back)
	cache.Set("d", 4)
	is.False(cache.Has("b")) // "b" should be evicted as it was oldest
	is.True(cache.Has("a"))  // "a" should still exist (was moved to front)
	is.True(cache.Has("c"))  // "c" should still exist
	is.True(cache.Has("d"))  // "d" should exist
}

func TestDeleteMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLRUCache[string, int](3)
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

	cache := NewLRUCache[string, int](2)
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

	// Test that peek doesn't change access order
	// Peek "a" should not move it to front
	cache.Peek("a")

	// Add a new item - "a" should be evicted because it wasn't moved to front
	cache.Set("c", 3)
	is.False(cache.Has("a"))
	is.True(cache.Has("b"))
	is.True(cache.Has("c"))
}
