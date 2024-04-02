package twoqueue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew2QCache(t *testing.T) {
	is := assert.New(t)

	is.Panics(func() {
		_ = New2QCache[string, int](0)
	})

	cache := New2QCache[string, int](42)
	is.Equal(42, cache.capacity)
	is.Equal(10, cache.recentCapacity)
	is.Equal(21, cache.recentEvictCapacity)
	is.Equal(0.25, cache.recentRatio)
	is.Equal(0.50, cache.ghostRatio)
	is.NotNil(cache.recent)
	is.NotNil(cache.frequent)
	is.NotNil(cache.recentEvict)
}

func TestNew2QCacheWithRatio(t *testing.T) {
	is := assert.New(t)

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
	is.Equal(10, cache.recentEvictCapacity)
	is.Equal(0.50, cache.recentRatio)
	is.Equal(0.25, cache.ghostRatio)
	is.NotNil(cache.recent)
	is.NotNil(cache.frequent)
	is.NotNil(cache.recentEvict)
}

func TestSet(t *testing.T) {
	// is := assert.New(t)

	// cache := NewLRUCache[string, int](2)

	// cache.Set("a", 1)
	// is.Equal(1, cache.ll.Len())
	// is.Equal(1, len(cache.cache))
	// is.EqualValues(&entry[string, int]{"a", 1}, cache.cache["a"].Value)
	// is.EqualValues(&entry[string, int]{"a", 1}, cache.ll.Front().Value)
	// is.EqualValues(&entry[string, int]{"a", 1}, cache.ll.Back().Value)

	// cache.Set("b", 2)
	// is.Equal(2, cache.ll.Len())
	// is.Equal(2, len(cache.cache))
	// is.EqualValues(&entry[string, int]{"a", 1}, cache.cache["a"].Value)
	// is.EqualValues(&entry[string, int]{"b", 2}, cache.cache["b"].Value)
	// is.EqualValues(&entry[string, int]{"b", 2}, cache.ll.Front().Value)
	// is.EqualValues(&entry[string, int]{"a", 1}, cache.ll.Back().Value)

	// cache.Set("c", 3)
	// is.Equal(2, cache.ll.Len())
	// is.Equal(2, len(cache.cache))
	// is.EqualValues(&entry[string, int]{"b", 2}, cache.cache["b"].Value)
	// is.EqualValues(&entry[string, int]{"c", 3}, cache.cache["c"].Value)
	// is.EqualValues(&entry[string, int]{"c", 3}, cache.ll.Front().Value)
	// is.EqualValues(&entry[string, int]{"b", 2}, cache.ll.Back().Value)
}

func TestHas(t *testing.T) {
	is := assert.New(t)

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

func TestKey(t *testing.T) {
	is := assert.New(t)

	cache := New2QCacheWithRatio[string, int](2, 0.5, 0.5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)
	cache.Get("c")
	cache.Set("d", 4)
	cache.Set("e", 5)

	is.ElementsMatch([]string{"c", "e"}, cache.Keys())

	cache = New2QCacheWithRatio[string, int](2, 0.5, 0.5)
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)
	cache.Set("d", 4)
	cache.Set("e", 5)
	cache.Get("c")

	is.ElementsMatch([]string{"d", "e"}, cache.Keys())
}

func TestValues(t *testing.T) {
	is := assert.New(t)

	cache := New2QCacheWithRatio[string, int](2, 0.5, 0.5)
	cache.Set("a", 1)
	cache.Get("a")
	cache.Set("b", 2)
	cache.Set("c", 3)

	is.ElementsMatch([]int{1, 3}, cache.Values())
}

func TestRange(t *testing.T) {
	is := assert.New(t)

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

	cache := New2QCacheWithRatio[string, int](2, 0.5, 0.5)
	cache.Set("a", 1)
	cache.Get("a")
	cache.Set("b", 2)

	cache.Purge()
	is.Equal(0, cache.Len())
}
