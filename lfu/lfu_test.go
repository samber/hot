package lfu

import (
	"testing"

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

	cache := NewLFUCache[string, int](2)

	cache.Set("a", 1)
	is.Equal(1, cache.ll.Len())
	is.Equal(1, len(cache.cache))
	is.EqualValues(&entry[string, int]{"a", 1}, cache.cache["a"].Value)
	is.EqualValues(&entry[string, int]{"a", 1}, cache.ll.Front().Value)
	is.EqualValues(&entry[string, int]{"a", 1}, cache.ll.Back().Value)

	cache.Set("b", 2)
	is.Equal(2, cache.ll.Len())
	is.Equal(2, len(cache.cache))
	is.EqualValues(&entry[string, int]{"a", 1}, cache.cache["a"].Value)
	is.EqualValues(&entry[string, int]{"b", 2}, cache.cache["b"].Value)
	is.EqualValues(&entry[string, int]{"b", 2}, cache.ll.Front().Value)
	is.EqualValues(&entry[string, int]{"a", 1}, cache.ll.Back().Value)

	cache.Set("b", 2)
	is.Equal(2, cache.ll.Len())
	is.Equal(2, len(cache.cache))
	is.EqualValues(&entry[string, int]{"a", 1}, cache.cache["a"].Value)
	is.EqualValues(&entry[string, int]{"b", 2}, cache.cache["b"].Value)
	is.EqualValues(&entry[string, int]{"a", 1}, cache.ll.Front().Value)
	is.EqualValues(&entry[string, int]{"b", 2}, cache.ll.Back().Value)

	cache.Set("c", 3)
	is.Equal(2, cache.ll.Len())
	is.Equal(2, len(cache.cache))
	is.EqualValues(&entry[string, int]{"b", 2}, cache.cache["b"].Value)
	is.EqualValues(&entry[string, int]{"c", 3}, cache.cache["c"].Value)
	is.EqualValues(&entry[string, int]{"c", 3}, cache.ll.Front().Value)
	is.EqualValues(&entry[string, int]{"b", 2}, cache.ll.Back().Value)

	cache = NewLFUCacheWithEvictionSize[string, int](3, 2)
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

	cache.Purge()
	is.Equal(0, cache.Len())
	is.Nil(cache.ll.Front())
	is.Nil(cache.ll.Back())
	is.Equal(0, len(cache.cache))
}
