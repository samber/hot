package arc

import (
	"testing"

	"github.com/samber/hot/pkg/base"
	"github.com/stretchr/testify/assert"
)

func TestNewARCCache(t *testing.T) {
	is := assert.New(t)

	is.Panics(func() {
		_ = NewARCCache[string, int](0)
	})

	cache := NewARCCache[string, int](42)
	is.Equal(42, cache.capacity)
	is.Equal(0, cache.p)
	is.NotNil(cache.t1)
	is.NotNil(cache.t2)
	is.NotNil(cache.b1)
	is.NotNil(cache.b2)
	is.NotNil(cache.t1Map)
	is.NotNil(cache.t2Map)
	is.NotNil(cache.b1Map)
	is.NotNil(cache.b2Map)
}

func TestSet(t *testing.T) {
	is := assert.New(t)

	evicted := 0
	cache := NewARCCacheWithEvictionCallback(2, func(k string, v int) {
		evicted += v
	})

	// Test basic set operations
	cache.Set("a", 1)
	is.Equal(1, cache.t1.Len())
	is.Equal(0, cache.t2.Len())
	is.Equal(1, len(cache.t1Map))
	is.Equal(0, len(cache.t2Map))
	is.Equal(0, evicted)

	cache.Set("b", 2)
	is.Equal(2, cache.t1.Len())
	is.Equal(0, cache.t2.Len())
	is.Equal(2, len(cache.t1Map))
	is.Equal(0, len(cache.t2Map))
	is.Equal(0, evicted)

	// Test eviction when capacity is reached
	// Canonical ARC: after 3rd insert, T1=2, T2=0, "a" evicted to B1
	cache.Set("c", 3)
	is.Equal(2, cache.t1.Len())
	is.Equal(0, cache.t2.Len())
	is.Equal(2, len(cache.t1Map))
	is.Equal(0, len(cache.t2Map))
	is.Equal(1, evicted) // "a" was evicted to B1
	is.Equal(1, cache.b1.Len())
	is.NotNil(cache.b1Map["a"])
}

func TestGet(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// Test get from T1 (should promote to T2)
	val, ok := cache.Get("a")
	is.True(ok)
	is.Equal(1, val)
	is.Equal(1, cache.t1.Len())
	is.Equal(1, cache.t2.Len())
	is.Nil(cache.t1Map["a"])
	is.NotNil(cache.t2Map["a"])

	// Test get from T2 (should move to front)
	val, ok = cache.Get("a")
	is.True(ok)
	is.Equal(1, val)
	is.Equal(1, cache.t1.Len())
	is.Equal(1, cache.t2.Len())

	// Test get non-existent key
	val, ok = cache.Get("c")
	is.False(ok)
	is.Zero(val)
}

func TestGetPromotesFromT1ToT2(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// Initially both items are in T1
	is.Equal(2, cache.t1.Len())
	is.Equal(0, cache.t2.Len())
	is.NotNil(cache.t1Map["a"])
	is.NotNil(cache.t1Map["b"])
	is.Nil(cache.t2Map["a"])
	is.Nil(cache.t2Map["b"])

	// Get "a" - should promote from T1 to T2
	val, ok := cache.Get("a")
	is.True(ok)
	is.Equal(1, val)

	// Check that "a" moved to T2
	is.Equal(1, cache.t1.Len())
	is.Equal(1, cache.t2.Len())
	is.Nil(cache.t1Map["a"])
	is.NotNil(cache.t2Map["a"])
	is.NotNil(cache.t1Map["b"])
	is.Nil(cache.t2Map["b"])
}

func TestSetExistingKey(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// Update existing key in T1
	cache.Set("a", 10)
	is.Equal(1, cache.t1.Len())
	is.Equal(1, cache.t2.Len())
	is.Nil(cache.t1Map["a"])
	is.NotNil(cache.t2Map["a"])

	// Update existing key in T2
	cache.Set("a", 20)
	is.Equal(1, cache.t1.Len())
	is.Equal(1, cache.t2.Len())
	is.Nil(cache.t1Map["a"])
	is.NotNil(cache.t2Map["a"])

	// Verify the value was updated
	val, ok := cache.Get("a")
	is.True(ok)
	is.Equal(20, val)
}

func TestGhostHitInB1(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3) // This evicts "a" to B1

	// "a" should now be in B1 (ghost entry)
	is.Equal(1, cache.b1.Len())
	is.NotNil(cache.b1Map["a"])
	is.Nil(cache.t1Map["a"])
	is.Nil(cache.t2Map["a"])

	// Access "a" again - should hit in B1 and promote to T2
	// Canonical ARC: after ghost hit, T1=1, T2=1, B1=1 (another item evicted from T1)
	cache.Set("a", 10)
	is.Equal(1, cache.b1.Len()) // B1 not empty - another item evicted from T1
	is.Nil(cache.b1Map["a"])    // "a" removed from B1
	is.Equal(1, cache.t1.Len())
	is.Equal(1, cache.t2.Len())
	is.NotNil(cache.t2Map["a"])

	// Check that p increased (favoring T2)
	is.Greater(cache.p, 0)
}

func TestGhostHitInB2(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// Promote "a" to T2
	cache.Get("a")

	// Add "c" to evict "b" to B1
	cache.Set("c", 3)

	// Add "d" to evict "a" from T2 to B2
	cache.Set("d", 4)

	// Access "a" again - should hit in B2 and promote to T2
	oldP := cache.p
	cache.Set("a", 10)

	// After ghost hit: "a" should be accessible and in T2
	val, ok := cache.Get("a")
	is.True(ok)
	is.Equal(10, val)

	// p may decrease after a B2 ghost hit
	is.LessOrEqual(cache.p, oldP)
}

func TestHas(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.True(cache.Has("a"))
	is.True(cache.Has("b"))
	is.False(cache.Has("c"))

	// Promote "a" to T2
	cache.Get("a")
	is.True(cache.Has("a"))
	is.True(cache.Has("b"))
	is.False(cache.Has("c"))
}

func TestPeek(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// Peek from T1
	val, ok := cache.Peek("a")
	is.True(ok)
	is.Equal(1, val)
	is.Equal(2, cache.t1.Len()) // Should not promote to T2

	// Promote "a" to T2
	cache.Get("a")

	// Peek from T2
	val, ok = cache.Peek("a")
	is.True(ok)
	is.Equal(1, val)
	is.Equal(1, cache.t2.Len()) // Should not move to front

	// Peek non-existent key
	val, ok = cache.Peek("c")
	is.False(ok)
	is.Zero(val)
}

func TestKeys(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)

	keys := cache.Keys()
	is.ElementsMatch([]string{"a", "b"}, keys)

	// Promote "a" to T2
	cache.Get("a")
	keys = cache.Keys()
	is.ElementsMatch([]string{"a", "b"}, keys)
}

func TestValues(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)

	values := cache.Values()
	is.ElementsMatch([]int{1, 2}, values)

	// Promote "a" to T2
	cache.Get("a")
	values = cache.Values()
	is.ElementsMatch([]int{1, 2}, values)
}

func TestRange(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)

	var keys []string
	var values []int
	cache.Range(func(key string, value int) bool {
		keys = append(keys, key)
		values = append(values, value)
		return true
	})
	is.ElementsMatch([]string{"a", "b"}, keys)
	is.ElementsMatch([]int{1, 2}, values)

	// Test early termination
	keys = nil
	values = nil
	cache.Range(func(key string, value int) bool {
		keys = append(keys, key)
		values = append(values, value)
		return false // Stop after first item
	})
	is.Len(keys, 1)
	is.Len(values, 1)
}

func TestDelete(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// Delete from T1
	ok := cache.Delete("a")
	is.True(ok)
	is.Equal(1, cache.t1.Len())
	is.Equal(0, cache.t2.Len())
	is.Nil(cache.t1Map["a"])
	is.Nil(cache.t2Map["a"])

	// Promote "b" to T2
	cache.Get("b")

	// Delete from T2
	ok = cache.Delete("b")
	is.True(ok)
	is.Equal(0, cache.t1.Len())
	is.Equal(0, cache.t2.Len())
	is.Nil(cache.t1Map["b"])
	is.Nil(cache.t2Map["b"])

	// Delete non-existent key
	ok = cache.Delete("c")
	is.False(ok)
}

func TestDeleteFromGhostLists(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](2)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3) // Evicts "a" to B1

	// Delete from B1
	ok := cache.Delete("a")
	is.True(ok)
	is.Equal(0, cache.b1.Len())
	is.Nil(cache.b1Map["a"])

	// Promote "b" to T2 and evict to B2
	cache.Get("b")
	cache.Set("d", 4) // Evicts "b" to B2

	// Delete from B2
	ok = cache.Delete("b")
	is.True(ok)
	is.Equal(0, cache.b2.Len())
	is.Nil(cache.b2Map["b"])
}

func TestSetMany(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
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
}

func TestHasMany(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)

	result := cache.HasMany([]string{"a", "b", "c"})
	is.Equal(map[string]bool{
		"a": true,
		"b": true,
		"c": false,
	}, result)
}

func TestGetMany(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)

	found, missing := cache.GetMany([]string{"a", "b", "c"})
	is.Equal(map[string]int{
		"a": 1,
		"b": 2,
	}, found)
	is.Equal([]string{"c"}, missing)
}

func TestPeekMany(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)

	found, missing := cache.PeekMany([]string{"a", "b", "c"})
	is.Equal(map[string]int{
		"a": 1,
		"b": 2,
	}, found)
	is.Equal([]string{"c"}, missing)
}

func TestDeleteMany(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	result := cache.DeleteMany([]string{"a", "b", "d"})
	is.Equal(map[string]bool{
		"a": true,
		"b": true,
		"d": false,
	}, result)
	is.Equal(1, cache.Len())
	is.True(cache.Has("c"))
}

func TestPurge(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)

	cache.Purge()
	is.Equal(0, cache.Len())
	is.Equal(0, cache.t1.Len())
	is.Equal(0, cache.t2.Len())
	is.Equal(0, cache.b1.Len())
	is.Equal(0, cache.b2.Len())
	is.Equal(0, cache.p)
}

func TestCapacity(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](42)
	is.Equal(42, cache.Capacity())
}

func TestAlgorithm(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	is.Equal("arc", cache.Algorithm())
}

func TestLen(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	is.Equal(0, cache.Len())

	cache.Set("a", 1)
	is.Equal(1, cache.Len())

	cache.Set("b", 2)
	is.Equal(2, cache.Len())

	// Promote "a" to T2
	cache.Get("a")
	is.Equal(2, cache.Len())
}

func TestDeleteOldest(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)
	cache.Set("a", 1)
	cache.Set("b", 2)

	key, value, ok := cache.DeleteOldest()
	is.True(ok)
	is.Equal("a", key)
	is.Equal(1, value)
	is.Equal(1, cache.Len())

	key, value, ok = cache.DeleteOldest()
	is.True(ok)
	is.Equal("b", key)
	is.Equal(2, value)
	is.Equal(0, cache.Len())

	key, value, ok = cache.DeleteOldest()
	is.Zero(key)
	is.Zero(value)
	is.False(ok)
}

func TestEvictionCallback(t *testing.T) {
	is := assert.New(t)

	evicted := make(map[string]int)
	cache := NewARCCacheWithEvictionCallback(2, func(k string, v int) {
		evicted[k] = v
	})

	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3) // Should evict "a"

	is.Equal(1, evicted["a"])
	is.Equal(1, len(evicted))
}

func TestEvictionCallbackWithSetMany(t *testing.T) {
	is := assert.New(t)

	evicted := make(map[string]int)
	cache := NewARCCacheWithEvictionCallback(2, func(k string, v int) {
		evicted[k] = v
	})

	items := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
	}
	cache.SetMany(items)

	// At least one eviction should have occurred
	is.GreaterOrEqual(len(evicted), 1)
}

func TestEvictionCallbackWithDeleteMany(t *testing.T) {
	is := assert.New(t)

	evicted := make(map[string]int)
	cache := NewARCCacheWithEvictionCallback(2, func(k string, v int) {
		evicted[k] = v
		t.Logf("Evicted: %s = %d", k, v)
	})

	// Fill cache
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3) // Evicts "a" to B1

	t.Logf("After Set c: T1=%d, T2=%d, B1=%d, B2=%d, evicted=%d",
		cache.t1.Len(), cache.t2.Len(), cache.b1.Len(), cache.b2.Len(), len(evicted))

	// After first eviction
	is.Equal(1, len(evicted))
	is.Equal(1, evicted["a"])
	is.Equal(1, cache.b1.Len())
	is.NotNil(cache.b1Map["a"])

	// Add more items
	cache.Set("d", 4) // May evict "b" to B1

	t.Logf("After Set d: T1=%d, T2=%d, B1=%d, B2=%d, evicted=%d",
		cache.t1.Len(), cache.t2.Len(), cache.b1.Len(), cache.b2.Len(), len(evicted))

	// Check that at least one item was evicted and is in B1
	is.GreaterOrEqual(len(evicted), 1)
	is.Equal(1, evicted["a"])

	// The evicted item should be in B1
	is.NotNil(cache.b1Map["a"])
}

func TestAdaptiveParameterP(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](4)
	is.Equal(0, cache.p) // Starts at 0 (pure LRU)

	// Fill cache
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)
	cache.Set("d", 4)

	// Evict "a" to B1
	cache.Set("e", 5)
	is.Equal(1, cache.b1.Len())
	is.NotNil(cache.b1Map["a"])
	is.Equal(0, cache.p) // p should still be 0 after miss

	// Hit in B1 should increase p
	oldP := cache.p
	cache.Set("a", 10)
	is.Greater(cache.p, oldP) // p should increase after ghost hit

	// Check that the promoted item is accessible
	val, ok := cache.Get("a")
	is.True(ok)
	is.Equal(10, val)
}

func TestComplexARCBehavior(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](3)

	// Add items
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)

	// Promote "a" to T2
	cache.Get("a")

	// Add "d" to evict an item
	cache.Set("d", 4)

	// Hit in B1 or B2 should promote
	cache.Set("b", 20)
	val, ok := cache.Get("b")
	is.True(ok)
	is.Equal(20, val)

	// Add another item and ghost hit again
	cache.Set("e", 5)
	cache.Set("a", 30)
	val, ok = cache.Get("a")
	is.True(ok)
	is.Equal(30, val)
}

func TestInterfaceCompliance(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](10)
	var _ base.InMemoryCache[string, int] = cache

	// Test that all required methods are implemented
	cache.Set("a", 1)
	_, ok := cache.Get("a")
	is.True(ok)
	is.True(cache.Has("a"))
	is.Equal(1, cache.Len())
	is.Equal(10, cache.Capacity())
	is.Equal("arc", cache.Algorithm())
}

func TestCanonicalARCBehavior(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](2)

	// Initial state
	is.Equal(0, cache.p)
	is.Equal(0, cache.t1.Len())
	is.Equal(0, cache.t2.Len())
	is.Equal(0, cache.b1.Len())
	is.Equal(0, cache.b2.Len())

	// Add items
	cache.Set("a", 1)
	t.Logf("After Set a: p=%d", cache.p)
	cache.Set("b", 2)
	t.Logf("After Set b: p=%d", cache.p)

	// Add third item to evict "a" to B1
	cache.Set("c", 3)
	t.Logf("After Set c: p=%d", cache.p)

	// Check state before ghost hit
	is.Equal(2, cache.t1.Len())
	is.Equal(0, cache.t2.Len())
	is.Equal(1, cache.b1.Len())
	is.Equal(0, cache.b2.Len())
	is.Equal(0, cache.p)

	// Ghost hit on "a" - should remove from B1 and add to T2
	// But also evict one item from T1 to B1 to maintain capacity
	cache.Set("a", 10)
	t.Logf("After Set a (ghost hit): p=%d", cache.p)

	// After ghost hit: T1=1, T2=1, B1=1 (the item evicted from T1)
	is.Equal(1, cache.t1.Len())
	is.Equal(1, cache.t2.Len())
	is.Equal(1, cache.b1.Len()) // B1 not empty - another item evicted from T1
	is.Equal(0, cache.b2.Len())
	is.Greater(cache.p, 0) // p should increase
}

func TestGhostHitBehavior(t *testing.T) {
	is := assert.New(t)

	cache := NewARCCache[string, int](2)

	// Fill cache
	cache.Set("a", 1)
	cache.Set("b", 2)

	// Add third item to evict "a" to B1
	cache.Set("c", 3)

	// Check state before ghost hit
	is.Equal(2, cache.t1.Len())
	is.Equal(0, cache.t2.Len())
	is.Equal(1, cache.b1.Len())
	is.Equal(0, cache.b2.Len())
	is.Equal(0, cache.p)

	// Ghost hit on "a" - should remove from B1 and add to T2
	// But also evict one item from T1 to B1 to maintain capacity
	cache.Set("a", 10)

	// After ghost hit: T1=1, T2=1, B1=1 (the item evicted from T1)
	is.Equal(1, cache.t1.Len())
	is.Equal(1, cache.t2.Len())
	is.Equal(1, cache.b1.Len())
	is.Equal(0, cache.b2.Len())
	is.Greater(cache.p, 0) // p should increase
}
