package fifo

import (
	"testing"

	"github.com/samber/hot/pkg/base"
	"github.com/stretchr/testify/assert"
)

func TestNewFIFOCache(t *testing.T) {
	cache := NewFIFOCache[string, int](10)
	assert.NotNil(t, cache)
	assert.Equal(t, 10, cache.Capacity())
	assert.Equal(t, "fifo", cache.Algorithm())
	assert.Equal(t, 0, cache.Len())
}

func TestNewFIFOCacheWithEvictionCallback(t *testing.T) {
	var evictedKey string
	var evictedValue int
	var evictionReason base.EvictionReason

	onEviction := func(reason base.EvictionReason, key string, value int) {
		evictedKey = key
		evictedValue = value
		evictionReason = reason
	}

	cache := NewFIFOCacheWithEvictionCallback[string, int](2, onEviction)
	assert.NotNil(t, cache)
	assert.Equal(t, 2, cache.Capacity())

	// Test eviction callback
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3) // This should evict "a"

	assert.Equal(t, "a", evictedKey)
	assert.Equal(t, 1, evictedValue)
	assert.Equal(t, base.EvictionReasonCapacity, evictionReason)
}

func TestFIFOCache_Set(t *testing.T) {
	cache := NewFIFOCache[string, int](3)

	// Test basic set
	cache.Set("a", 1)
	assert.Equal(t, 1, cache.Len())
	assert.True(t, cache.Has("a"))

	// Test update existing key
	cache.Set("a", 10)
	assert.Equal(t, 1, cache.Len())
	value, ok := cache.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 10, value)

	// Test eviction (FIFO order)
	cache.Set("b", 2)
	cache.Set("c", 3)
	cache.Set("d", 4) // This should evict "a"

	assert.Equal(t, 3, cache.Len())
	assert.False(t, cache.Has("a"))
	assert.True(t, cache.Has("b"))
	assert.True(t, cache.Has("c"))
	assert.True(t, cache.Has("d"))
}

func TestFIFOCache_Get(t *testing.T) {
	cache := NewFIFOCache[string, int](3)

	// Test get non-existent key
	value, ok := cache.Get("a")
	assert.False(t, ok)
	assert.Equal(t, 0, value)

	// Test get existing key
	cache.Set("a", 1)
	value, ok = cache.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 1, value)

	// Test that Get doesn't change FIFO order
	cache.Set("b", 2)
	cache.Set("c", 3)
	cache.Get("a")    // Access "a" but it should still be evicted first
	cache.Set("d", 4) // This should evict "a", not "b"

	assert.False(t, cache.Has("a"))
	assert.True(t, cache.Has("b"))
	assert.True(t, cache.Has("c"))
	assert.True(t, cache.Has("d"))
}

func TestFIFOCache_Peek(t *testing.T) {
	cache := NewFIFOCache[string, int](3)

	// Test peek non-existent key
	value, ok := cache.Peek("a")
	assert.False(t, ok)
	assert.Equal(t, 0, value)

	// Test peek existing key
	cache.Set("a", 1)
	value, ok = cache.Peek("a")
	assert.True(t, ok)
	assert.Equal(t, 1, value)

	// Test that Peek doesn't change FIFO order (same as Get)
	cache.Set("b", 2)
	cache.Set("c", 3)
	cache.Peek("a")   // Access "a" but it should still be evicted first
	cache.Set("d", 4) // This should evict "a", not "b"

	assert.False(t, cache.Has("a"))
	assert.True(t, cache.Has("b"))
	assert.True(t, cache.Has("c"))
	assert.True(t, cache.Has("d"))
}

func TestFIFOCache_Has(t *testing.T) {
	cache := NewFIFOCache[string, int](3)

	assert.False(t, cache.Has("a"))

	cache.Set("a", 1)
	assert.True(t, cache.Has("a"))

	cache.Delete("a")
	assert.False(t, cache.Has("a"))
}

func TestFIFOCache_Delete(t *testing.T) {
	cache := NewFIFOCache[string, int](3)

	// Test delete non-existent key
	assert.False(t, cache.Delete("a"))

	// Test delete existing key
	cache.Set("a", 1)
	assert.True(t, cache.Delete("a"))
	assert.False(t, cache.Has("a"))
	assert.Equal(t, 0, cache.Len())

	// Test delete from middle of list
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)
	assert.True(t, cache.Delete("b"))
	assert.False(t, cache.Has("b"))
	assert.True(t, cache.Has("a"))
	assert.True(t, cache.Has("c"))
	assert.Equal(t, 2, cache.Len())
}

func TestFIFOCache_Purge(t *testing.T) {
	cache := NewFIFOCache[string, int](3)

	cache.Set("a", 1)
	cache.Set("b", 2)
	assert.Equal(t, 2, cache.Len())

	cache.Purge()
	assert.Equal(t, 0, cache.Len())
	assert.False(t, cache.Has("a"))
	assert.False(t, cache.Has("b"))
}

func TestFIFOCache_Keys(t *testing.T) {
	cache := NewFIFOCache[string, int](3)

	assert.Empty(t, cache.Keys())

	cache.Set("a", 1)
	cache.Set("b", 2)

	keys := cache.Keys()
	assert.Len(t, keys, 2)
	assert.Contains(t, keys, "a")
	assert.Contains(t, keys, "b")
}

func TestFIFOCache_Values(t *testing.T) {
	cache := NewFIFOCache[string, int](3)

	assert.Empty(t, cache.Values())

	cache.Set("a", 1)
	cache.Set("b", 2)

	values := cache.Values()
	assert.Len(t, values, 2)
	assert.Contains(t, values, 1)
	assert.Contains(t, values, 2)
}

func TestFIFOCache_Range(t *testing.T) {
	cache := NewFIFOCache[string, int](3)

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

func TestFIFOCache_SetMany(t *testing.T) {
	cache := NewFIFOCache[string, int](5)

	items := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	cache.SetMany(items)

	assert.Equal(t, 3, cache.Len())
	assert.True(t, cache.Has("a"))
	assert.True(t, cache.Has("b"))
	assert.True(t, cache.Has("c"))
}

func TestFIFOCache_HasMany(t *testing.T) {
	cache := NewFIFOCache[string, int](3)

	cache.Set("a", 1)
	cache.Set("b", 2)

	result := cache.HasMany([]string{"a", "b", "c"})
	expected := map[string]bool{
		"a": true,
		"b": true,
		"c": false,
	}

	assert.Equal(t, expected, result)
}

func TestFIFOCache_GetMany(t *testing.T) {
	cache := NewFIFOCache[string, int](3)

	cache.Set("a", 1)
	cache.Set("b", 2)

	found, missing := cache.GetMany([]string{"a", "b", "c"})
	expectedFound := map[string]int{
		"a": 1,
		"b": 2,
	}
	expectedMissing := []string{"c"}

	assert.Equal(t, expectedFound, found)
	assert.Equal(t, expectedMissing, missing)
}

func TestFIFOCache_PeekMany(t *testing.T) {
	cache := NewFIFOCache[string, int](3)

	cache.Set("a", 1)
	cache.Set("b", 2)

	found, missing := cache.PeekMany([]string{"a", "b", "c"})
	expectedFound := map[string]int{
		"a": 1,
		"b": 2,
	}
	expectedMissing := []string{"c"}

	assert.Equal(t, expectedFound, found)
	assert.Equal(t, expectedMissing, missing)
}

func TestFIFOCache_DeleteMany(t *testing.T) {
	cache := NewFIFOCache[string, int](3)

	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	result := cache.DeleteMany([]string{"a", "b", "d"})
	expected := map[string]bool{
		"a": true,
		"b": true,
		"d": false,
	}

	assert.Equal(t, expected, result)
	assert.False(t, cache.Has("a"))
	assert.False(t, cache.Has("b"))
	assert.True(t, cache.Has("c"))
}

func TestFIFOCache_DeleteOldest(t *testing.T) {
	cache := NewFIFOCache[string, int](3)

	// Test empty cache
	key, value, ok := cache.DeleteOldest()
	assert.False(t, ok)
	assert.Equal(t, "", key)
	assert.Equal(t, 0, value)

	// Test with items
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	key, value, ok = cache.DeleteOldest()
	assert.True(t, ok)
	assert.Equal(t, "a", key)
	assert.Equal(t, 1, value)
	assert.Equal(t, 2, cache.Len())
	assert.False(t, cache.Has("a"))
	assert.True(t, cache.Has("b"))
	assert.True(t, cache.Has("c"))
}

func TestFIFOCache_SizeBytes(t *testing.T) {
	cache := NewFIFOCache[string, string](3)

	assert.Equal(t, int64(0), cache.SizeBytes())

	cache.Set("a", "hello")
	cache.Set("b", "world")

	size := cache.SizeBytes()
	assert.Greater(t, size, int64(0))
}

func TestFIFOCache_FIFOOrder(t *testing.T) {
	cache := NewFIFOCache[string, int](3)

	// Add items in order
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Access items in different order
	cache.Get("c")
	cache.Get("a")
	cache.Get("b")

	// Add new item - should evict "a" (first in)
	cache.Set("d", 4)

	assert.False(t, cache.Has("a"))
	assert.True(t, cache.Has("b"))
	assert.True(t, cache.Has("c"))
	assert.True(t, cache.Has("d"))

	// Add another item - should evict "b" (next in order)
	cache.Set("e", 5)

	assert.False(t, cache.Has("a"))
	assert.False(t, cache.Has("b"))
	assert.True(t, cache.Has("c"))
	assert.True(t, cache.Has("d"))
	assert.True(t, cache.Has("e"))
}

func TestFIFOCache_UpdateExistingKey(t *testing.T) {
	cache := NewFIFOCache[string, int](3)

	// Add items
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Update existing key - should not change order
	cache.Set("b", 10)

	// Add new item - should evict "b" (not "a")
	cache.Set("d", 4)

	assert.False(t, cache.Has("a"))
	assert.True(t, cache.Has("b"))
	assert.True(t, cache.Has("c"))
	assert.True(t, cache.Has("d"))

	// Verify updated value
	value, ok := cache.Get("b")
	assert.True(t, ok)
	assert.Equal(t, 10, value)
}
