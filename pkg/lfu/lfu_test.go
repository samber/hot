package lfu

import (
	"testing"

	"github.com/samber/hot/pkg/base"
	"github.com/stretchr/testify/assert"
)

func TestNewLFUCache(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

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
	is.NotNil(cache.cache)
	is.NotNil(cache.freqMap)

	cache = NewLFUCacheWithEvictionSize[string, int](42, 10)
	is.Equal(42, cache.capacity)
	is.Equal(10, cache.evictionSize)
	is.NotNil(cache.cache)
	is.NotNil(cache.freqMap)
}

func TestSet(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	evicted := 0
	cache := NewLFUCacheWithEvictionCallback(2, func(reason base.EvictionReason, k string, v int) {
		is.Equal(base.EvictionReasonCapacity, reason)
		evicted += v
	})

	cache.Set("a", 1)
	is.Equal(1, cache.Len())
	is.Len(cache.cache, 1)
	is.Equal("a", cache.cache["a"].Value.key)
	is.Equal(1, cache.cache["a"].Value.value)
	is.Equal(0, evicted)

	cache.Set("b", 2)
	is.Equal(2, cache.Len())
	is.Len(cache.cache, 2)
	is.Equal(1, cache.cache["a"].Value.value)
	is.Equal(2, cache.cache["b"].Value.value)
	is.Equal(0, evicted)

	// Set existing key "b" - increments frequency
	cache.Set("b", 2)
	is.Equal(2, cache.Len())
	is.Len(cache.cache, 2)
	is.Equal(1, cache.cache["b"].Value.freq) // freq incremented
	is.Equal(0, evicted)

	// Set new key "c" - evicts "a" (freq=0, LRU within freq 0)
	cache.Set("c", 3)
	is.Equal(2, cache.Len())
	is.Len(cache.cache, 2)
	is.NotNil(cache.cache["b"])
	is.NotNil(cache.cache["c"])
	is.Nil(cache.cache["a"])
	is.Equal(1, evicted) // a(1) evicted

	// Test with evictionSize=2
	cache = NewLFUCacheWithEvictionSizeAndCallback(3, 2, func(reason base.EvictionReason, k string, v int) {
		is.Equal(base.EvictionReasonCapacity, reason)
		evicted += v
	})
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)
	// All at freq 0. Adding "d" evicts 2 items: "a" (oldest LRU), then "b"
	cache.Set("d", 4)
	is.Equal(2, cache.Len())
	is.Len(cache.cache, 2)
	is.NotNil(cache.cache["c"])
	is.NotNil(cache.cache["d"])
	is.Equal(4, evicted) // prior(1) + a(1) + b(2) = 4
}

func TestHas(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.True(cache.Has("a"))
	is.True(cache.Has("b"))
	is.False(cache.Has("c"))

	// Set "c" evicts "a" (freq=0, oldest/LRU within freq 0)
	cache.Set("c", 3)
	is.False(cache.Has("a"))
	is.True(cache.Has("b"))
	is.True(cache.Has("c"))
}

func TestGet(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](2)
	cache.Set("a", 1) // a:freq=0
	cache.Set("b", 2) // b:freq=0

	val, ok := cache.Get("b") // b:freq=0→1
	is.True(ok)
	is.Equal(2, val)

	val, ok = cache.Get("a") // a:freq=0→1
	is.True(ok)
	is.Equal(1, val)

	val, ok = cache.Get("c") // miss
	is.False(ok)
	is.Zero(val)

	// Set "c" - both a and b at freq=1. LRU within freq 1 is "b" (accessed earlier).
	cache.Set("c", 3) // evicts "b"
	is.False(cache.Has("b"))
	is.True(cache.Has("a"))
	is.True(cache.Has("c"))

	val, ok = cache.Get("a") // a:freq=1→2
	is.True(ok)
	is.Equal(1, val)

	val, ok = cache.Get("b") // miss
	is.False(ok)
	is.Zero(val)

	val, ok = cache.Get("c") // c:freq=0→1
	is.True(ok)
	is.Equal(3, val)
}

func TestPeak(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	val, ok := cache.Peek("a")
	is.True(ok)
	is.Equal(1, val)
	is.Equal(0, cache.cache["a"].Value.freq) // freq unchanged

	val, ok = cache.Peek("b")
	is.True(ok)
	is.Equal(2, val)
	is.Equal(0, cache.cache["b"].Value.freq) // freq unchanged

	val, ok = cache.Peek("c")
	is.False(ok)
	is.Zero(val)
}

func TestKey(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.ElementsMatch([]string{"b", "a"}, cache.Keys())
}

func TestValues(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.ElementsMatch([]int{1, 2}, cache.Values())
}

func TestInternalState_All(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](2)
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
	t.Parallel()

	cache := NewLFUCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	ok := cache.Delete("a")
	is.True(ok)
	is.Equal(1, cache.Len())
	is.Len(cache.cache, 1)
	is.NotNil(cache.cache["b"])
	is.Nil(cache.cache["a"])

	ok = cache.Delete("b")
	is.True(ok)
	is.Equal(0, cache.Len())
	is.Empty(cache.cache)

	ok = cache.Delete("b")
	is.False(ok)
	is.Equal(0, cache.Len())
	is.Empty(cache.cache)
}

func TestDeleteLeastFrequent(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](2)
	cache.Set("a", 1) // a:freq=0
	cache.Set("b", 2) // b:freq=0
	cache.Get("a")    // a:freq=1

	// Least frequent is "b" (freq=0)
	key, value, ok := cache.DeleteLeastFrequent()
	is.True(ok)
	is.Equal("b", key)
	is.Equal(2, value)
	is.Equal(1, cache.Len())
	is.Len(cache.cache, 1)
	is.NotNil(cache.cache["a"])

	// Next least frequent is "a" (freq=1)
	key, value, ok = cache.DeleteLeastFrequent()
	is.True(ok)
	is.Equal("a", key)
	is.Equal(1, value)
	is.Equal(0, cache.Len())
	is.Empty(cache.cache)

	// Empty cache
	key, value, ok = cache.DeleteLeastFrequent()
	is.False(ok)
	is.Empty(key)
	is.Zero(value)
}

func TestLen(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](2)
	cache.Set("z", 0)
	cache.Set("a", 1)
	// z:freq=0, a:freq=0. Cache full.
	cache.Set("b", 2) // evicts z (LRU at freq 0)
	cache.Get("b")    // b:freq=1
	cache.Get("c")    // miss

	is.Equal(2, cache.Len())

	cache.Delete("a")
	is.Equal(1, cache.Len())

	cache.Delete("b")
	is.Equal(0, cache.Len())
}

func TestPurge(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	evicted := 0
	cache := NewLFUCacheWithEvictionCallback(2, func(reason base.EvictionReason, k string, v int) {
		is.Equal(base.EvictionReasonCapacity, reason)
		evicted++
	})

	cache.SetMany(map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
	})

	is.Equal(2, cache.Len())
	is.Equal(1, evicted) // 1 item should be evicted
}

func TestHasMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

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
	t.Parallel()

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)

	results := cache.HasMany([]string{})
	is.Empty(results)
}

func TestGetMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

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
	t.Parallel()

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)

	found, missing := cache.GetMany([]string{})
	is.Empty(found)
	is.Empty(missing)
}

func TestPeekMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

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
	t.Parallel()

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)

	found, missing := cache.PeekMany([]string{})
	is.Empty(found)
	is.Empty(missing)
}

func TestDeleteMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

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
	t.Parallel()

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)

	results := cache.DeleteMany([]string{})
	is.Empty(results)
}

func TestCapacity(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](42)
	is.Equal(42, cache.Capacity())
}

func TestAlgorithm(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](42)
	is.Equal("lfu", cache.Algorithm())
}

func TestInterfaceCompliance(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

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

func TestRange_AllItems(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	evicted := make(map[string]int)
	cache := NewLFUCacheWithEvictionCallback(2, func(reason base.EvictionReason, k string, v int) {
		is.Equal(base.EvictionReasonCapacity, reason)
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
	is.NotEmpty(evicted)
	is.Equal(2, cache.Len())
}

func TestEvictionCallback_WithDeleteMany(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	evicted := make(map[string]int)
	cache := NewLFUCacheWithEvictionCallback(5, func(reason base.EvictionReason, k string, v int) {
		is.Equal(base.EvictionReasonCapacity, reason)
		evicted[k] = v
	})

	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// DeleteMany should not trigger eviction callback
	cache.DeleteMany([]string{"a", "b"})

	is.Empty(evicted) // No eviction callback should be called
	is.Equal(1, cache.Len())
}

func TestInternalState_InitialState(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](5)

	is.Equal(0, cache.Len())
	is.Empty(cache.cache)
	is.Empty(cache.freqMap)
}

func TestInternalState_SingleElement(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)

	is.Equal(1, cache.Len())
	is.Len(cache.cache, 1)
	is.Equal("a", cache.cache["a"].Value.key)
	is.Equal(1, cache.cache["a"].Value.value)
	is.Equal(0, cache.cache["a"].Value.freq)
}

func TestInternalState_MultipleElements(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	is.Equal(3, cache.Len())
	is.Len(cache.cache, 3)

	// All at freq 0
	is.Equal(0, cache.cache["a"].Value.freq)
	is.Equal(0, cache.cache["b"].Value.freq)
	is.Equal(0, cache.cache["c"].Value.freq)

	is.Equal(1, cache.cache["a"].Value.value)
	is.Equal(2, cache.cache["b"].Value.value)
	is.Equal(3, cache.cache["c"].Value.value)
}

func TestInternalState_GetUpdatesFrequency(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1) // a:freq=0
	cache.Set("b", 2) // b:freq=0
	cache.Set("c", 3) // c:freq=0

	// Get "c" - frequency incremented
	val, ok := cache.Get("c")
	is.True(ok)
	is.Equal(3, val)
	is.Equal(1, cache.cache["c"].Value.freq)
	is.Equal(0, cache.cache["a"].Value.freq)
	is.Equal(0, cache.cache["b"].Value.freq)

	// Get "b" - frequency incremented
	val, ok = cache.Get("b")
	is.True(ok)
	is.Equal(2, val)
	is.Equal(1, cache.cache["b"].Value.freq)

	// Get "a" - frequency incremented
	val, ok = cache.Get("a")
	is.True(ok)
	is.Equal(1, val)
	is.Equal(1, cache.cache["a"].Value.freq)
}

func TestInternalState_PeekDoesNotUpdateFrequency(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Peek should not change frequency
	val, ok := cache.Peek("a")
	is.True(ok)
	is.Equal(1, val)
	is.Equal(0, cache.cache["a"].Value.freq)

	val, ok = cache.Peek("b")
	is.True(ok)
	is.Equal(2, val)
	is.Equal(0, cache.cache["b"].Value.freq)
}

func TestInternalState_SetExistingKey(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// All at freq 0
	is.Equal(0, cache.cache["a"].Value.freq)

	// Set existing key "a" with new value - increments frequency
	cache.Set("a", 10)
	is.Equal(10, cache.cache["a"].Value.value)
	is.Equal(1, cache.cache["a"].Value.freq) // freq incremented
}

func TestInternalState_Eviction(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	evicted := make(map[string]int)
	cache := NewLFUCacheWithEvictionCallback(3, func(reason base.EvictionReason, k string, v int) {
		is.Equal(base.EvictionReasonCapacity, reason)
		evicted[k] = v
	})

	cache.Set("a", 1) // a:freq=0
	cache.Set("b", 2) // b:freq=0
	cache.Set("c", 3) // c:freq=0

	is.Equal(3, cache.Len())

	// Add "d" - evicts LRU at min freq (freq=0, oldest = "a")
	cache.Set("d", 4)
	is.Equal(3, cache.Len())
	is.Equal(1, evicted["a"])
	is.Nil(cache.cache["a"])
	is.NotNil(cache.cache["b"])
	is.NotNil(cache.cache["c"])
	is.NotNil(cache.cache["d"])
}

func TestInternalState_EvictionWithFrequency(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	evicted := make(map[string]int)
	cache := NewLFUCacheWithEvictionCallback(3, func(reason base.EvictionReason, k string, v int) {
		is.Equal(base.EvictionReasonCapacity, reason)
		evicted[k] = v
	})

	cache.Set("a", 1) // a:freq=0
	cache.Set("b", 2) // b:freq=0
	cache.Set("c", 3) // c:freq=0

	// Access "c" to increase its frequency
	cache.Get("c") // c:freq=1

	is.Equal(1, cache.cache["c"].Value.freq)
	is.Equal(0, cache.cache["a"].Value.freq)
	is.Equal(0, cache.cache["b"].Value.freq)

	// Add "d" - evicts from freq=0, LRU = "a" (oldest)
	cache.Set("d", 4)
	is.Equal(3, cache.Len())
	is.Equal(1, evicted["a"])
	is.Nil(cache.cache["a"])
	is.NotNil(cache.cache["b"])
	is.NotNil(cache.cache["c"])
	is.NotNil(cache.cache["d"])
}

func TestInternalState_Delete(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	is.Equal(3, cache.Len())

	ok := cache.Delete("b")
	is.True(ok)
	is.Equal(2, cache.Len())
	is.Nil(cache.cache["b"])

	ok = cache.Delete("c")
	is.True(ok)
	is.Equal(1, cache.Len())

	ok = cache.Delete("a")
	is.True(ok)
	is.Equal(0, cache.Len())
	is.Empty(cache.cache)
}

func TestInternalState_DeleteLeastFrequent(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1) // a:freq=0
	cache.Set("b", 2) // b:freq=0
	cache.Set("c", 3) // c:freq=0

	// All at freq 0, LRU order: a (oldest), b, c (newest)
	// Delete least frequent = a (LRU at freq 0)
	key, value, ok := cache.DeleteLeastFrequent()
	is.True(ok)
	is.Equal("a", key)
	is.Equal(1, value)
	is.Equal(2, cache.Len())

	// Next: b (LRU at freq 0)
	key, value, ok = cache.DeleteLeastFrequent()
	is.True(ok)
	is.Equal("b", key)
	is.Equal(2, value)
	is.Equal(1, cache.Len())

	// Next: c
	key, value, ok = cache.DeleteLeastFrequent()
	is.True(ok)
	is.Equal("c", key)
	is.Equal(3, value)
	is.Equal(0, cache.Len())

	// Empty cache
	key, value, ok = cache.DeleteLeastFrequent()
	is.False(ok)
	is.Empty(key)
	is.Equal(0, value)
}

func TestInternalState_ComplexOperations(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](4)

	cache.Set("a", 1) // a:0
	cache.Set("b", 2) // b:0
	cache.Set("c", 3) // c:0

	is.Equal(3, cache.Len())

	// Get "b" to increase frequency
	cache.Get("b") // b:1

	is.Equal(1, cache.cache["b"].Value.freq)
	is.Equal(0, cache.cache["a"].Value.freq)
	is.Equal(0, cache.cache["c"].Value.freq)

	// Add "d"
	cache.Set("d", 4) // d:0, cache has a:0, b:1, c:0, d:0

	is.Equal(4, cache.Len())
	is.True(cache.Has("a"))
	is.True(cache.Has("b"))
	is.True(cache.Has("c"))
	is.True(cache.Has("d"))

	// Update existing "b" (increments freq)
	cache.Set("b", 30) // b:2

	is.Equal(30, cache.cache["b"].Value.value)
	is.Equal(2, cache.cache["b"].Value.freq)

	// Add "e" - evicts from freq=0, LRU = "a" (oldest at freq 0)
	cache.Set("e", 5)
	is.Equal(4, cache.Len())
	is.False(cache.Has("a")) // evicted
	is.True(cache.Has("b"))
	is.True(cache.Has("c"))
	is.True(cache.Has("d"))
	is.True(cache.Has("e"))

	// Delete "c"
	cache.Delete("c")
	is.Equal(3, cache.Len())
	is.False(cache.Has("c"))
}

func TestInternalState_FrequencyOrdering(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](5)

	cache.Set("a", 1) // a:0
	cache.Set("b", 2) // b:0
	cache.Set("c", 3) // c:0

	// Access "c" multiple times
	cache.Get("c") // c:1
	cache.Get("c") // c:2
	cache.Get("c") // c:3

	is.Equal(3, cache.cache["c"].Value.freq)
	is.Equal(0, cache.cache["a"].Value.freq)
	is.Equal(0, cache.cache["b"].Value.freq)

	// Access "b" once
	cache.Get("b") // b:1

	is.Equal(1, cache.cache["b"].Value.freq)

	// Eviction order should be: a (freq=0), then b (freq=1), then c (freq=3)
	key, _, ok := cache.DeleteLeastFrequent()
	is.True(ok)
	is.Equal("a", key) // freq=0

	key, _, ok = cache.DeleteLeastFrequent()
	is.True(ok)
	is.Equal("b", key) // freq=1

	key, _, ok = cache.DeleteLeastFrequent()
	is.True(ok)
	is.Equal("c", key) // freq=3
}

func TestInternalState_FrequencyTiebreaking(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](5)

	cache.Set("a", 1) // a:0
	cache.Set("b", 2) // b:0
	cache.Set("c", 3) // c:0

	// Access all once
	cache.Get("a") // a:1
	cache.Get("b") // b:1
	cache.Get("c") // c:1

	// All at freq=1. LRU tiebreaking: a was moved to MRU first, then b, then c.
	// Within freq=1 bucket: front=[c, b, a]=back. LRU = a (back).
	key, _, ok := cache.DeleteLeastFrequent()
	is.True(ok)
	is.Equal("a", key) // LRU at freq=1

	key, _, ok = cache.DeleteLeastFrequent()
	is.True(ok)
	is.Equal("b", key)

	key, _, ok = cache.DeleteLeastFrequent()
	is.True(ok)
	is.Equal("c", key)
}

func TestInternalState_ElementRelationships(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := NewLFUCache[string, int](5)
	cache.Set("a", 1) // a:0
	cache.Set("b", 2) // b:0
	cache.Set("c", 3) // c:0

	// All in freq=0 bucket
	is.Equal(0, cache.minFreq)
	freqList, exists := cache.freqMap[0]
	is.True(exists)
	is.Equal(3, freqList.Len())

	// Get "a" to move to freq=1
	cache.Get("a") // a:1

	// freq=0 has b, c; freq=1 has a
	is.Equal(0, cache.minFreq)
	is.Equal(2, cache.freqMap[0].Len())
	is.Equal(1, cache.freqMap[1].Len())
}
