package twoqueue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew2QCache(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	is.Panics(func() {
		_ = New2QCache[string, int](0)
	})

	cache := New2QCache[string, int](42)
	is.Equal(42, cache.capacity)
	is.Equal(10, cache.recentCapacity)
	is.Equal(21, cache.ghostCapacity)
	is.Equal(0.25, cache.recentRatio)
	is.Equal(0.50, cache.ghostRatio)
	is.NotNil(cache.recent)
	is.NotNil(cache.frequent)
	is.NotNil(cache.ghost)
}

func TestNew2QCacheWithRatio(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	is.Panics(func() {
		_ = New2QCacheWithRatio[string, int](0, 0.5, 0.25)
	})
	is.Panics(func() {
		_ = New2QCacheWithRatio[string, int](42, -0.5, 0.25)
	})
	is.Panics(func() {
		_ = New2QCacheWithRatio[string, int](42, 1.5, 0.25)
	})
	is.Panics(func() {
		_ = New2QCacheWithRatio[string, int](42, 0.5, -0.25)
	})
	is.Panics(func() {
		_ = New2QCacheWithRatio[string, int](42, 0.5, 1.25)
	})

	cache := New2QCacheWithRatio[string, int](42, 0.5, 0.25)
	is.Equal(42, cache.capacity)
	is.Equal(21, cache.recentCapacity)
	is.Equal(10, cache.ghostCapacity)
	is.Equal(0.50, cache.recentRatio)
	is.Equal(0.25, cache.ghostRatio)
	is.NotNil(cache.recent)
	is.NotNil(cache.frequent)
	is.NotNil(cache.ghost)
}

func TestSet(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCacheWithRatio[string, int](2, 0.5, 0.5)
	cache.Set("a", 1)
	is.True(cache.recent.Has("a"))
	is.False(cache.frequent.Has("a"))

	cache.Set("a", 2)
	is.False(cache.recent.Has("a"))
	is.True(cache.frequent.Has("a"))

	v, ok := cache.frequent.Get("a")
	is.True(ok)
	is.Equal(2, v)
}

func TestHas(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCacheWithRatio[string, int](2, 1, 1)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.True(cache.recent.Has("a"))
	is.True(cache.recent.Has("b"))
	is.False(cache.recent.Has("c"))
	is.False(cache.frequent.Has("a"))
	is.False(cache.frequent.Has("b"))
	is.False(cache.frequent.Has("c"))

	cache.Set("c", 3)
	is.False(cache.recent.Has("a"))
	is.True(cache.recent.Has("b"))
	is.True(cache.recent.Has("c"))
	is.False(cache.frequent.Has("a"))
	is.False(cache.frequent.Has("b"))
	is.False(cache.frequent.Has("c"))

	cache.Set("c", 3)
	is.False(cache.recent.Has("a"))
	is.True(cache.recent.Has("b"))
	is.False(cache.recent.Has("c"))
	is.False(cache.frequent.Has("a"))
	is.False(cache.frequent.Has("b"))
	is.True(cache.frequent.Has("c"))
}

func TestGet(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCacheWithRatio[string, int](2, 1, 1)
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

func printKeys(keys []string) {
	for _, k := range keys {
		print(k, " ")
	}
	println()
}

func TestKey(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCacheWithRatio[string, int](2, 0.5, 0.5)
	cache.Set("a", 1)
	print("after Set a: ")
	printKeys(cache.Keys())
	cache.Set("b", 2)
	print("after Set b: ")
	printKeys(cache.Keys())
	cache.Set("c", 3)
	print("after Set c: ")
	printKeys(cache.Keys())
	cache.Get("c")
	print("after Get c: ")
	printKeys(cache.Keys())
	cache.Set("d", 4)
	print("after Set d: ")
	printKeys(cache.Keys())
	cache.Set("e", 5)
	print("after Set e: ")
	printKeys(cache.Keys())

	is.ElementsMatch([]string{"c", "e"}, cache.Keys())

	cache = New2QCacheWithRatio[string, int](2, 0.5, 0.5)
	cache.Set("a", 1)
	print("[2] after Set a: ")
	printKeys(cache.Keys())
	cache.Set("b", 2)
	print("[2] after Set b: ")
	printKeys(cache.Keys())
	cache.Set("c", 3)
	print("[2] after Set c: ")
	printKeys(cache.Keys())
	cache.Set("d", 4)
	print("[2] after Set d: ")
	printKeys(cache.Keys())
	cache.Set("e", 5)
	print("[2] after Set e: ")
	printKeys(cache.Keys())
	cache.Get("c")
	print("[2] after Get c: ")
	printKeys(cache.Keys())

	is.ElementsMatch([]string{"e"}, cache.Keys())
}

func TestValues(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCacheWithRatio[string, int](2, 0.5, 0.5)
	cache.Set("a", 1)
	cache.Get("a")
	cache.Set("b", 2)
	cache.Set("c", 3)

	is.ElementsMatch([]int{1, 3}, cache.Values())
}

func TestRange(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCacheWithRatio[string, int](2, 0.5, 0.5)
	cache.Set("a", 1)
	cache.Get("a")
	cache.Set("b", 2)
	cache.Set("c", 3)

	var keys []string
	var values []int
	cache.Range(func(key string, value int) bool {
		keys = append(keys, key)
		values = append(values, value)
		return true
	})
	is.ElementsMatch([]string{"a", "c"}, keys)
	is.ElementsMatch([]int{1, 3}, values)
}

func TestDelete(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCacheWithRatio[string, int](2, 0.5, 0.5)
	cache.Set("a", 1)
	cache.Get("a")
	cache.Set("b", 2)
	cache.Set("c", 3)

	ok := cache.Delete("a")
	is.True(ok)
	is.Equal(1, cache.Len())

	ok = cache.Delete("b")
	is.True(ok)
	is.Equal(1, cache.Len())

	ok = cache.Delete("b")
	is.False(ok)
	is.Equal(1, cache.Len())
}

func TestLen(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCacheWithRatio[string, int](2, 0.5, 0.5)
	cache.Set("a", 1)
	cache.Get("a")
	cache.Set("b", 2)

	is.Equal(2, cache.Len())

	cache.Delete("a")
	is.Equal(1, cache.Len())

	cache.Delete("a")
	is.Equal(1, cache.Len())

	cache.Delete("b")
	is.Equal(0, cache.Len())
}

func TestPurge(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCache[string, int](10)
	cache.Set("a", 1)
	cache.Set("b", 2)

	is.Equal(2, cache.Len())

	cache.Purge()

	is.Equal(0, cache.Len())
	is.False(cache.Has("a"))
	is.False(cache.Has("b"))
}

// Helper function to verify 2Q cache internal state.
func verify2QState[K comparable, V any](t *testing.T, cache *TwoQueueCache[K, V]) ([]K, []K, []K) {
	t.Helper()
	is := assert.New(t)

	// Verify total length
	totalExpected := cache.recent.Len() + cache.frequent.Len()
	is.Equal(totalExpected, cache.Len())

	// Get recent list keys
	recentKeys := cache.recent.Keys()

	// Get frequent list keys
	frequentKeys := cache.frequent.Keys()

	// Get ghost list keys
	ghostKeys := cache.ghost.Keys()

	return recentKeys, frequentKeys, ghostKeys
}

func TestInternalState_InitialState(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCache[string, int](10)

	// Verify initial state
	is.Equal(0, cache.Len())
	is.Equal(0, cache.recent.Len())
	is.Equal(0, cache.frequent.Len())
	is.Equal(0, cache.ghost.Len())

	recent, frequent, ghost := verify2QState(t, cache)
	is.Empty(recent)
	is.Empty(frequent)
	is.Empty(ghost)
}

func TestInternalState_SingleElement(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCache[string, int](10)
	cache.Set("a", 1)

	// Verify single element state - should go to recent list
	is.Equal(1, cache.Len())
	is.Equal(1, cache.recent.Len())
	is.Equal(0, cache.frequent.Len())
	is.Equal(0, cache.ghost.Len())

	recent, frequent, ghost := verify2QState(t, cache)
	is.Equal([]string{"a"}, recent)
	is.Empty(frequent)
	is.Empty(ghost)
}

// func TestInternalState_MultipleElements(t *testing.T) {
// 	is := assert.New(t)

// 	cache := New2QCache[string, int](10)
// 	cache.Set("a", 1)
// 	cache.Set("b", 2)
// 	cache.Set("c", 3)

// 	// Verify multiple elements state - all should go to recent list
// 	is.Equal(3, cache.Len())
// 	is.Equal(3, cache.recent.Len())
// 	is.Equal(0, cache.frequent.Len())
// 	is.Equal(0, cache.recentEvict.Len())

// 	recent, frequent, ghost := verify2QState(t, cache)
// 	is.Equal([]string{"a", "b", "c"}, recent)
// 	is.Empty(frequent)
// 	is.Empty(ghost)
// }

// func TestInternalState_PromotionToFrequent(t *testing.T) {
// 	is := assert.New(t)

// 	cache := New2QCache[string, int](10)
// 	cache.Set("a", 1)
// 	cache.Set("b", 2)

// 	// Initial state: both in recent
// 	recent, frequent, ghost := verify2QState(t, cache)
// 	is.Equal([]string{"a", "b"}, recent)
// 	is.Empty(frequent)
// 	is.Empty(ghost)

// 	// Access "a" - should promote to frequent
// 	val, ok := cache.Get("a")
// 	is.True(ok)
// 	is.Equal(1, val)

// 	// State: "a" in frequent, "b" in recent
// 	recent, frequent, ghost = verify2QState(t, cache)
// 	is.Equal([]string{"b"}, recent)
// 	is.Equal([]string{"a"}, frequent)
// 	is.Empty(ghost)

// 	// Access "b" - should promote to frequent
// 	val, ok = cache.Get("b")
// 	is.True(ok)
// 	is.Equal(2, val)

// 	// State: both in frequent (order may vary)
// 	recent, frequent, ghost = verify2QState(t, cache)
// 	is.Empty(recent)
// 	is.Len(frequent, 2)
// 	is.Contains(frequent, "a")
// 	is.Contains(frequent, "b")
// 	is.Empty(ghost)
// }

func TestInternalState_UpdateExistingInFrequent(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCache[string, int](10)
	cache.Set("a", 1)

	// Promote to frequent
	cache.Get("a")
	recent, frequent, ghost := verify2QState(t, cache)
	is.Empty(recent)
	is.Equal([]string{"a"}, frequent)
	is.Empty(ghost)

	// Update existing key in frequent
	cache.Set("a", 10)

	// Should remain in frequent
	recent, frequent, ghost = verify2QState(t, cache)
	is.Empty(recent)
	is.Equal([]string{"a"}, frequent)
	is.Empty(ghost)

	// Verify value was updated
	val, ok := cache.Get("a")
	is.True(ok)
	is.Equal(10, val)
}

func TestInternalState_UpdateExistingInRecent(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCache[string, int](10)
	cache.Set("a", 1)

	// Should be in recent
	recent, frequent, ghost := verify2QState(t, cache)
	is.Equal([]string{"a"}, recent)
	is.Empty(frequent)
	is.Empty(ghost)

	// Update existing key in recent
	cache.Set("a", 10)

	// Should remain in recent
	recent, frequent, ghost = verify2QState(t, cache)
	is.Empty(recent)
	is.Equal([]string{"a"}, frequent)
	is.Empty(ghost)

	// Verify value was updated
	val, ok := cache.Get("a")
	is.True(ok)
	is.Equal(10, val)
}

func TestInternalState_Delete(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCache[string, int](10)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// Promote "a" to frequent
	cache.Get("a")
	recent, frequent, ghost := verify2QState(t, cache)
	is.Equal([]string{"b"}, recent)
	is.Equal([]string{"a"}, frequent)
	is.Empty(ghost)

	// Delete from frequent
	ok := cache.Delete("a")
	is.True(ok)
	recent, frequent, ghost = verify2QState(t, cache)
	is.Equal([]string{"b"}, recent)
	is.Empty(frequent)
	is.Empty(ghost)

	// Delete from recent
	ok = cache.Delete("b")
	is.True(ok)
	recent, frequent, ghost = verify2QState(t, cache)
	is.Empty(recent)
	is.Empty(frequent)
	is.Empty(ghost)

	// Delete non-existent
	ok = cache.Delete("c")
	is.False(ok)
}

func TestInternalState_CapacityAndRatios(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCacheWithRatio[string, int](10, 0.3, 0.2)

	// Verify capacities
	is.Equal(10, cache.Capacity())
	is.Equal(10, cache.capacity)
	is.Equal(3, cache.recentCapacity) // 10 * 0.3
	is.Equal(2, cache.ghostCapacity)  // 10 * 0.2
	is.Equal(0.3, cache.recentRatio)
	is.Equal(0.2, cache.ghostRatio)
}

func TestInternalState_Algorithm(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCache[string, int](10)
	is.Equal("2q", cache.Algorithm())
}

func TestInternalState_KeysAndValues(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCache[string, int](10)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// Promote "a" to frequent
	cache.Get("a")

	// Get all keys
	keys := cache.Keys()
	is.Len(keys, 2)
	is.Contains(keys, "a")
	is.Contains(keys, "b")

	// Get all values
	values := cache.Values()
	is.Len(values, 2)
	is.Contains(values, 1)
	is.Contains(values, 2)
}

func TestInternalState_All(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCache[string, int](10)
	cache.Set("a", 1)
	cache.Set("b", 2)

	all := cache.All()
	is.Len(all, 2)
	is.Equal(1, all["a"])
	is.Equal(2, all["b"])
}

func TestInternalState_Range(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cache := New2QCache[string, int](10)
	cache.Set("a", 1)
	cache.Set("b", 2)

	// Promote "a" to frequent
	cache.Get("a")

	// Test range
	visited := make(map[string]int)
	cache.Range(func(k string, v int) bool {
		visited[k] = v
		return true
	})

	is.Len(visited, 2)
	is.Equal(1, visited["a"])
	is.Equal(2, visited["b"])
}
