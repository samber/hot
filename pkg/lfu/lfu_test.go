package lfu

import (
	"testing"

	"github.com/samber/hot/pkg/base"
	"github.com/stretchr/testify/assert"
)

func TestNewLFUCache(t *testing.T) {
	is := assert.New(t)

	is.Panics(func() {
		_ = NewLFUCache[string, int](0)
	})
	is.Panics(func() {
		_ = NewLFUCache[string, int](1)
	})
	is.Panics(func() {
		_ = NewLFUCacheWithEvictionSize[string, int](2, 2)
	})

	cache := NewLFUCache[string, int](42)
	is.Equal(42, cache.capacity)
	is.Equal(1, cache.evictionSize)
	is.NotNil(cache.ll)
	is.NotNil(cache.cache)

	cache = NewLFUCacheWithEvictionSize[string, int](42, 10)
	is.Equal(42, cache.capacity)
	is.Equal(10, cache.evictionSize)
	is.NotNil(cache.ll)
	is.NotNil(cache.cache)
}

func TestSet(t *testing.T) {
	is := assert.New(t)

	evicted := 0
	cache := NewLFUCacheWithEvictionCallback(2, func(k string, v int) {
		evicted += v
	})

	cache.Set("a", 1)
	is.Equal(1, cache.ll.Len())
	is.Equal(1, len(cache.cache))
	is.EqualValues(&entry[string, int]{"a", 1}, cache.cache["a"].Value)
	is.EqualValues(&entry[string, int]{"a", 1}, cache.ll.Front().Value)
	is.EqualValues(&entry[string, int]{"a", 1}, cache.ll.Back().Value)
	is.Equal(0, evicted)

	cache.Set("b", 2)
	is.Equal(2, cache.ll.Len())
	is.Equal(2, len(cache.cache))
	is.EqualValues(&entry[string, int]{"a", 1}, cache.cache["a"].Value)
	is.EqualValues(&entry[string, int]{"b", 2}, cache.cache["b"].Value)
	is.EqualValues(&entry[string, int]{"b", 2}, cache.ll.Front().Value)
	is.EqualValues(&entry[string, int]{"a", 1}, cache.ll.Back().Value)
	is.Equal(0, evicted)

	cache.Set("b", 2)
	is.Equal(2, cache.ll.Len())
	is.Equal(2, len(cache.cache))
	is.EqualValues(&entry[string, int]{"a", 1}, cache.cache["a"].Value)
	is.EqualValues(&entry[string, int]{"b", 2}, cache.cache["b"].Value)
	is.EqualValues(&entry[string, int]{"a", 1}, cache.ll.Front().Value)
	is.EqualValues(&entry[string, int]{"b", 2}, cache.ll.Back().Value)
	is.Equal(0, evicted)

	cache.Set("c", 3)
	is.Equal(2, cache.ll.Len())
	is.Equal(2, len(cache.cache))
	is.EqualValues(&entry[string, int]{"b", 2}, cache.cache["b"].Value)
	is.EqualValues(&entry[string, int]{"c", 3}, cache.cache["c"].Value)
	is.EqualValues(&entry[string, int]{"c", 3}, cache.ll.Front().Value)
	is.EqualValues(&entry[string, int]{"b", 2}, cache.ll.Back().Value)
	is.Equal(1, evicted)

	cache = NewLFUCacheWithEvictionSizeAndCallback(3, 2, func(k string, v int) {
		evicted += v
	})
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)
	cache.Set("d", 4)
	is.Equal(2, cache.ll.Len())
	is.Equal(2, len(cache.cache))
	is.EqualValues(&entry[string, int]{"a", 1}, cache.cache["a"].Value)
	is.EqualValues(&entry[string, int]{"d", 4}, cache.cache["d"].Value)
	is.Equal("d", cache.ll.Front().Value.(*entry[string, int]).key)
	is.Equal("a", cache.ll.Back().Value.(*entry[string, int]).key)
	is.Equal(6, evicted)
}

func TestHas(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.True(cache.Has("a"))
	is.True(cache.Has("b"))
	is.False(cache.Has("c"))

	cache.Set("c", 3)
	is.True(cache.Has("a"))
	is.False(cache.Has("b"))
	is.True(cache.Has("c"))
}

func TestGet(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)
	is.Equal("b", cache.ll.Front().Value.(*entry[string, int]).key)
	is.Equal("a", cache.ll.Back().Value.(*entry[string, int]).key)

	val, ok := cache.Get("b")
	is.True(ok)
	is.Equal(2, val)
	is.Equal("a", cache.ll.Front().Value.(*entry[string, int]).key)
	is.Equal("b", cache.ll.Back().Value.(*entry[string, int]).key)

	val, ok = cache.Get("a")
	is.True(ok)
	is.Equal(1, val)
	is.Equal("b", cache.ll.Front().Value.(*entry[string, int]).key)
	is.Equal("a", cache.ll.Back().Value.(*entry[string, int]).key)

	val, ok = cache.Get("c")
	is.False(ok)
	is.Zero(val)
	is.Equal("b", cache.ll.Front().Value.(*entry[string, int]).key)
	is.Equal("a", cache.ll.Back().Value.(*entry[string, int]).key)

	cache.Set("c", 3)
	is.Equal("c", cache.ll.Front().Value.(*entry[string, int]).key)
	is.Equal("a", cache.ll.Back().Value.(*entry[string, int]).key)

	val, ok = cache.Get("a")
	is.True(ok)
	is.Equal(1, val)
	is.Equal("c", cache.ll.Front().Value.(*entry[string, int]).key)
	is.Equal("a", cache.ll.Back().Value.(*entry[string, int]).key)

	val, ok = cache.Get("b")
	is.False(ok)
	is.Zero(val)
	is.Equal("c", cache.ll.Front().Value.(*entry[string, int]).key)
	is.Equal("a", cache.ll.Back().Value.(*entry[string, int]).key)

	val, ok = cache.Get("c")
	is.True(ok)
	is.Equal(3, val)
	is.Equal("a", cache.ll.Front().Value.(*entry[string, int]).key)
	is.Equal("c", cache.ll.Back().Value.(*entry[string, int]).key)
}

func TestPeak(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	val, ok := cache.Peek("a")
	is.True(ok)
	is.Equal(1, val)
	is.Equal("b", cache.ll.Front().Value.(*entry[string, int]).key)
	is.Equal("a", cache.ll.Back().Value.(*entry[string, int]).key)

	val, ok = cache.Peek("b")
	is.True(ok)
	is.Equal(2, val)
	is.Equal("b", cache.ll.Front().Value.(*entry[string, int]).key)
	is.Equal("a", cache.ll.Back().Value.(*entry[string, int]).key)

	val, ok = cache.Peek("c")
	is.False(ok)
	is.Zero(val)
	is.Equal("b", cache.ll.Front().Value.(*entry[string, int]).key)
	is.Equal("a", cache.ll.Back().Value.(*entry[string, int]).key)
}

func TestKey(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.ElementsMatch([]string{"b", "a"}, cache.Keys())
}

func TestValues(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.ElementsMatch([]int{1, 2}, cache.Values())
}

func TestRange(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](2)
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

	cache := NewLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	ok := cache.Delete("a")
	is.True(ok)
	is.Equal(1, cache.ll.Len())
	is.Equal(1, len(cache.cache))
	is.EqualValues(&entry[string, int]{"b", 2}, cache.cache["b"].Value)
	is.EqualValues(&entry[string, int]{"b", 2}, cache.ll.Front().Value)
	is.EqualValues(&entry[string, int]{"b", 2}, cache.ll.Back().Value)

	ok = cache.Delete("b")
	is.True(ok)
	is.Equal(0, cache.ll.Len())
	is.Equal(0, len(cache.cache))
	is.Nil(cache.ll.Front())
	is.Nil(cache.ll.Back())

	ok = cache.Delete("b")
	is.False(ok)
	is.Equal(0, cache.ll.Len())
	is.Equal(0, len(cache.cache))
	is.Nil(cache.ll.Front())
	is.Nil(cache.ll.Back())
}

func TestDeleteLeastFrequent(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Get("a")

	key, value, ok := cache.DeleteLeastFrequent()
	is.True(ok)
	is.Equal("b", key)
	is.Equal(2, value)
	is.Equal(1, cache.ll.Len())
	is.Equal(1, len(cache.cache))
	is.EqualValues(&entry[string, int]{"a", 1}, cache.cache["a"].Value)
	is.EqualValues(&entry[string, int]{"a", 1}, cache.ll.Front().Value)
	is.EqualValues(&entry[string, int]{"a", 1}, cache.ll.Back().Value)

	key, value, ok = cache.DeleteLeastFrequent()
	is.True(ok)
	is.Equal("a", key)
	is.Equal(1, value)
	is.Equal(0, cache.ll.Len())
	is.Equal(0, len(cache.cache))
	is.Nil(cache.ll.Front())
	is.Nil(cache.ll.Back())

	key, value, ok = cache.DeleteLeastFrequent()
	is.False(ok)
	is.Zero(key)
	is.Zero(value)
	is.False(ok)
	is.Zero(key)
	is.Zero(value)
	is.Equal(0, cache.ll.Len())
	is.Equal(0, len(cache.cache))
	is.Nil(cache.ll.Front())
	is.Nil(cache.ll.Back())
}

func TestLen(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](2)
	cache.Set("z", 0)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Get("b")
	cache.Get("c")

	is.Equal(2, cache.Len())

	cache.Delete("a")
	is.Equal(2, cache.Len())

	cache.Delete("z")
	is.Equal(1, cache.Len())

	cache.Delete("b")
	is.Equal(0, cache.Len())
}

func TestPurge(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.Equal(2, cache.Len())

	cache.Purge()

	is.Equal(0, cache.Len())
	is.False(cache.Has("a"))
	is.False(cache.Has("b"))
}

func TestSetMany(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	items := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
	}

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
}

func TestSetMany_WithEviction(t *testing.T) {
	is := assert.New(t)

	evicted := 0
	cache := NewLFUCacheWithEvictionCallback(2, func(k string, v int) {
		evicted += v
	})

	cache.SetMany(map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
	})

	is.Equal(2, cache.Len())
	is.Equal(1, evicted) // 1 should be evicted
}

func TestHasMany(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	keys := []string{"a", "b", "c", "d", "e"}
	results := cache.HasMany(keys)

	is.True(results["a"])
	is.True(results["b"])
	is.True(results["c"])
	is.False(results["d"])
	is.False(results["e"])
}

func TestHasMany_EmptyKeys(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)

	results := cache.HasMany([]string{})
	is.Len(results, 0)
}

func TestGetMany(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	keys := []string{"a", "b", "c", "d", "e"}
	found, missing := cache.GetMany(keys)

	is.Len(found, 3)
	is.Len(missing, 2)
	is.Equal(1, found["a"])
	is.Equal(2, found["b"])
	is.Equal(3, found["c"])
	is.Contains(missing, "d")
	is.Contains(missing, "e")
}

func TestGetMany_EmptyKeys(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)

	found, missing := cache.GetMany([]string{})
	is.Len(found, 0)
	is.Len(missing, 0)
}

func TestPeekMany(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	keys := []string{"a", "b", "c", "d", "e"}
	found, missing := cache.PeekMany(keys)

	is.Len(found, 3)
	is.Len(missing, 2)
	is.Equal(1, found["a"])
	is.Equal(2, found["b"])
	is.Equal(3, found["c"])
	is.Contains(missing, "d")
	is.Contains(missing, "e")
}

func TestPeekMany_EmptyKeys(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)

	found, missing := cache.PeekMany([]string{})
	is.Len(found, 0)
	is.Len(missing, 0)
}

func TestDeleteMany(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	keys := []string{"a", "b", "d", "e"}
	results := cache.DeleteMany(keys)

	is.True(results["a"])
	is.True(results["b"])
	is.False(results["d"])
	is.False(results["e"])

	is.Equal(1, cache.Len())
	is.True(cache.Has("c"))
	is.False(cache.Has("a"))
	is.False(cache.Has("b"))
}

func TestDeleteMany_EmptyKeys(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)

	results := cache.DeleteMany([]string{})
	is.Len(results, 0)
}

func TestCapacity(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](42)
	is.Equal(42, cache.Capacity())
}

func TestAlgorithm(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](42)
	is.Equal("lfu", cache.Algorithm())
}

func TestInterfaceCompliance(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)

	// Verify the LFU cache implements the interface
	var _ base.InMemoryCache[string, int] = cache

	// Test that we can assign it to the interface type
	var cacheInterface base.InMemoryCache[string, int] = cache

	// Test operations through the interface
	cacheInterface.Set("test", 42)
	value, ok := cacheInterface.Get("test")
	is.True(ok)
	is.Equal(42, value)
}

func TestRange_EarlyExit(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	visited := make(map[string]int)
	cache.Range(func(k string, v int) bool {
		visited[k] = v
		return k != "b" // stop at "b"
	})

	// Should have visited some items but not all
	is.Less(len(visited), 3)
}

func TestRange_AllItems(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	visited := make(map[string]int)
	cache.Range(func(k string, v int) bool {
		visited[k] = v
		return true
	})

	is.Len(visited, 3)
	is.Equal(1, visited["a"])
	is.Equal(2, visited["b"])
	is.Equal(3, visited["c"])
}

func TestKeys(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	keys := cache.Keys()
	is.Len(keys, 3)
	is.Contains(keys, "a")
	is.Contains(keys, "b")
	is.Contains(keys, "c")
}

func TestEvictionCallback_WithSetMany(t *testing.T) {
	is := assert.New(t)

	evicted := make(map[string]int)
	cache := NewLFUCacheWithEvictionCallback(2, func(k string, v int) {
		evicted[k] = v
	})

	items := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
		"d": 4,
	}

	cache.SetMany(items)

	// Should have evicted some items
	is.Greater(len(evicted), 0)
	is.Equal(2, cache.Len())
}

func TestEvictionCallback_WithDeleteMany(t *testing.T) {
	is := assert.New(t)

	evicted := make(map[string]int)
	cache := NewLFUCacheWithEvictionCallback(5, func(k string, v int) {
		evicted[k] = v
	})

	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// DeleteMany should not trigger eviction callback
	cache.DeleteMany([]string{"a", "b"})

	is.Len(evicted, 0) // No eviction callback should be called
	is.Equal(1, cache.Len())
}

// Helper function to verify LFU linked list order
func verifyLFUOrder[K comparable, V any](t *testing.T, cache *LFUCache[K, V]) []K {
	is := assert.New(t)

	// Verify list length matches cache map
	is.Equal(cache.ll.Len(), len(cache.cache))

	if cache.ll.Len() == 0 {
		is.Nil(cache.ll.Front())
		is.Nil(cache.ll.Back())
		return []K{}
	}

	// Verify front and back
	is.NotNil(cache.ll.Front())
	is.NotNil(cache.ll.Back())

	// Build order from front to back (most frequently used to least frequently used)
	var order []K
	current := cache.ll.Front()
	for current != nil {
		entry := current.Value.(*entry[K, V])
		order = append(order, entry.key)
		current = current.Next()
	}

	return order
}

func TestInternalState_InitialState(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)

	// Verify initial state
	is.Equal(0, cache.ll.Len())
	is.Equal(0, len(cache.cache))
	is.Nil(cache.ll.Front())
	is.Nil(cache.ll.Back())

	order := verifyLFUOrder(t, cache)
	is.Empty(order)
}

func TestInternalState_SingleElement(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)

	// Verify single element state
	is.Equal(1, cache.ll.Len())
	is.Equal(1, len(cache.cache))
	is.NotNil(cache.ll.Front())
	is.NotNil(cache.ll.Back())
	is.Equal(cache.ll.Front(), cache.ll.Back()) // Same element

	entry := cache.ll.Front().Value.(*entry[string, int])
	is.Equal("a", entry.key)
	is.Equal(1, entry.value)

	order := verifyLFUOrder(t, cache)
	is.Equal([]string{"a"}, order)
}

func TestInternalState_MultipleElements(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Verify multiple elements state
	is.Equal(3, cache.ll.Len())
	is.Equal(3, len(cache.cache))

	// Order should be: c (most recent) -> b -> a (least recent)
	order := verifyLFUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Verify cache map contains correct elements
	is.NotNil(cache.cache["a"])
	is.NotNil(cache.cache["b"])
	is.NotNil(cache.cache["c"])

	// Verify element values
	is.Equal(1, cache.cache["a"].Value.(*entry[string, int]).value)
	is.Equal(2, cache.cache["b"].Value.(*entry[string, int]).value)
	is.Equal(3, cache.cache["c"].Value.(*entry[string, int]).value)
}

func TestInternalState_GetUpdatesFrequency(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Initial order: c -> b -> a
	order := verifyLFUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Get "c" - should move to front (higher frequency)
	val, ok := cache.Get("c")
	is.True(ok)
	is.Equal(3, val)

	// Order should now be: b -> c -> a
	order = verifyLFUOrder(t, cache)
	is.Equal([]string{"b", "c", "a"}, order)

	// Get "b" - should move to front
	val, ok = cache.Get("b")
	is.True(ok)
	is.Equal(2, val)

	// Order should now be: c -> b -> a
	order = verifyLFUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Get "a" again - should move to front
	val, ok = cache.Get("a")
	is.True(ok)
	is.Equal(1, val)

	// Order should now be: c -> b -> a
	order = verifyLFUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)
}

func TestInternalState_PeekDoesNotUpdateFrequency(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Initial order: c -> b -> a
	order := verifyLFUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Peek "a" - should not change order
	val, ok := cache.Peek("a")
	is.True(ok)
	is.Equal(1, val)

	// Order should remain: c -> b -> a
	order = verifyLFUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Peek "b" - should not change order
	val, ok = cache.Peek("b")
	is.True(ok)
	is.Equal(2, val)

	// Order should remain: c -> b -> a
	order = verifyLFUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)
}

func TestInternalState_SetExistingKey(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Initial order: c -> b -> a
	order := verifyLFUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Set existing key "a" with new value - should not change order
	cache.Set("a", 10)

	// Order should remain: c -> b -> a (no frequency change)
	order = verifyLFUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Verify value was updated
	is.Equal(10, cache.cache["a"].Value.(*entry[string, int]).value)
}

func TestInternalState_Eviction(t *testing.T) {
	is := assert.New(t)

	evicted := make(map[string]int)
	cache := NewLFUCacheWithEvictionCallback(3, func(k string, v int) {
		evicted[k] = v
	})

	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Order: c -> b -> a
	order := verifyLFUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Add one more - should evict "c" (least frequently used)
	cache.Set("d", 4)

	// Order should now be: d -> b -> a
	order = verifyLFUOrder(t, cache)
	is.Equal([]string{"d", "b", "a"}, order)

	// Verify "a" was evicted
	is.Equal(3, evicted["c"])
	is.Nil(cache.cache["c"])
}

func TestInternalState_EvictionWithFrequency(t *testing.T) {
	is := assert.New(t)

	evicted := make(map[string]int)
	cache := NewLFUCacheWithEvictionCallback(3, func(k string, v int) {
		evicted[k] = v
	})

	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Order: c -> b -> a
	order := verifyLFUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Access "a" to increase its frequency
	cache.Get("c")

	// Order should now be: a -> c -> b
	order = verifyLFUOrder(t, cache)
	is.Equal([]string{"b", "c", "a"}, order)

	// Add one more - should evict "b" (least frequently used)
	cache.Set("d", 4)

	// Order should now be: d -> a -> c
	order = verifyLFUOrder(t, cache)
	is.Equal([]string{"d", "c", "a"}, order)

	// Verify "b" was evicted (not "a" because "a" has higher frequency)
	is.Equal(2, evicted["b"])
	is.Nil(cache.cache["b"])
}

func TestInternalState_Delete(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Initial order: c -> b -> a
	order := verifyLFUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Delete middle element "b"
	ok := cache.Delete("b")
	is.True(ok)

	// Order should now be: c -> a
	order = verifyLFUOrder(t, cache)
	is.Equal([]string{"c", "a"}, order)

	// Verify "b" is removed from cache map
	is.Nil(cache.cache["b"])

	// Delete front element "c"
	ok = cache.Delete("c")
	is.True(ok)

	// Order should now be: a
	order = verifyLFUOrder(t, cache)
	is.Equal([]string{"a"}, order)

	// Delete last element "a"
	ok = cache.Delete("a")
	is.True(ok)

	// Order should now be empty
	order = verifyLFUOrder(t, cache)
	is.Empty(order)
}

func TestInternalState_DeleteLeastFrequent(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Initial order: c -> b -> a
	order := verifyLFUOrder(t, cache)
	is.Equal([]string{"c", "b", "a"}, order)

	// Delete least frequent (front of list)
	key, value, ok := cache.DeleteLeastFrequent()
	is.True(ok)
	is.Equal("c", key)
	is.Equal(3, value)

	// Order should now be: b -> a
	order = verifyLFUOrder(t, cache)
	is.Equal([]string{"b", "a"}, order)

	// Delete least frequent again
	key, value, ok = cache.DeleteLeastFrequent()
	is.True(ok)
	is.Equal("b", key)
	is.Equal(2, value)

	// Order should now be: c
	order = verifyLFUOrder(t, cache)
	is.Equal([]string{"a"}, order)

	// Delete least frequent again
	key, value, ok = cache.DeleteLeastFrequent()
	is.True(ok)
	is.Equal("a", key)
	is.Equal(1, value)

	// Order should now be empty
	order = verifyLFUOrder(t, cache)
	is.Empty(order)

	// Try to delete from empty cache
	key, value, ok = cache.DeleteLeastFrequent()
	is.False(ok)
	is.Equal("", key)
	is.Equal(0, value)
}

func TestInternalState_ElementRelationships(t *testing.T) {
	is := assert.New(t)

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Verify element relationships
	front := cache.ll.Front()
	back := cache.ll.Back()

	// Front should be "c" (most recent)
	is.Equal("c", front.Value.(*entry[string, int]).key)

	// Back should be "a" (least recent)
	is.Equal("a", back.Value.(*entry[string, int]).key)

	// Verify Next/Prev relationships
	middle := front.Next()
	is.Equal("b", middle.Value.(*entry[string, int]).key)
	is.Equal(front, middle.Prev())
	is.Equal(back, middle.Next())

	// Verify back element
	is.Equal(middle, back.Prev())
	is.Nil(back.Next())
	is.Nil(front.Prev())
}

func TestInternalState_ComplexOperations(t *testing.T) {
	cache := NewLFUCache[string, int](4)

	// Add elements
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Order: c -> b -> a
	order := verifyLFUOrder(t, cache)
	assert.Equal(t, []string{"c", "b", "a"}, order)

	// Get middle element to increase frequency
	cache.Get("b")

	// Order: c -> a -> b
	order = verifyLFUOrder(t, cache)
	assert.Equal(t, []string{"c", "a", "b"}, order)

	// Add another element
	cache.Set("d", 4)

	// Order: d -> c -> a -> b
	order = verifyLFUOrder(t, cache)
	assert.Equal(t, []string{"d", "c", "a", "b"}, order)

	// Update existing element (should not change frequency)
	cache.Set("b", 30)

	// Order should remain: d -> c -> a -> b
	order = verifyLFUOrder(t, cache)
	assert.Equal(t, []string{"d", "c", "a", "b"}, order)

	// Add one more - should evict "d"
	cache.Set("e", 5)

	// Order: e -> c -> a -> b
	order = verifyLFUOrder(t, cache)
	assert.Equal(t, []string{"e", "c", "a", "b"}, order)

	// Delete middle element
	cache.Delete("c")

	// Order: e -> a -> b
	order = verifyLFUOrder(t, cache)
	assert.Equal(t, []string{"e", "a", "b"}, order)
}

func TestInternalState_FrequencyOrdering(t *testing.T) {
	cache := NewLFUCache[string, int](5)

	// Add elements
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Initial order: c -> b -> a
	order := verifyLFUOrder(t, cache)
	assert.Equal(t, []string{"c", "b", "a"}, order)

	// Access "a" multiple times to increase its frequency
	cache.Get("c") // a becomes most frequent
	cache.Get("c") // a stays most frequent
	cache.Get("c") // a stays most frequent

	// Order should be: b -> a -> c
	order = verifyLFUOrder(t, cache)
	assert.Equal(t, []string{"b", "a", "c"}, order)

	// Access "b" to increase its frequency
	cache.Get("b") // b becomes more frequent than c

	// Order should be: a -> b -> c
	order = verifyLFUOrder(t, cache)
	assert.Equal(t, []string{"a", "b", "c"}, order)

	// Access "c" to make it most frequent
	cache.Get("c")
	cache.Get("c")
	cache.Get("c")

	// Order should be: a -> b -> c
	order = verifyLFUOrder(t, cache)
	assert.Equal(t, []string{"a", "b", "c"}, order)
}
