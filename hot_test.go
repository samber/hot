package hot

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samber/go-singleflightx"
	"github.com/samber/hot/pkg/safe"
	"github.com/stretchr/testify/assert"
)

func TestNewHotCache(t *testing.T) {
	is := assert.New(t)

	lru := composeInternalCache[int, int](false, LRU, 42, 0, -1, nil, nil, nil)
	safeLru := composeInternalCache[int, int](true, LRU, 42, 0, -1, nil, nil, nil)

	// locking
	cache := newHotCache(lru, false, nil, 0, 0, 0, 0, nil, nil, DropOnError, nil, nil, nil)
	_, ok := cache.cache.(*safe.SafeInMemoryCache[int, *item[int]])
	is.False(ok)
	cache = newHotCache(safeLru, false, safeLru, 0, 0, 0, 0, nil, nil, DropOnError, nil, nil, nil)
	_, ok = cache.cache.(*safe.SafeInMemoryCache[int, *item[int]])
	is.True(ok)
	_, ok = cache.missingCache.(*safe.SafeInMemoryCache[int, *item[int]])
	is.True(ok)

	// ttl, stale, jitter
	cache = newHotCache(safeLru, false, nil, 42_000, 21_000, 2, time.Second, nil, nil, DropOnError, nil, nil, nil)
	is.EqualValues(&HotCache[int, int]{sync.RWMutex{}, nil, nil, nil, nil, safeLru, false, nil, 42, 21, 2, time.Second, nil, nil, DropOnError, nil, nil, nil, singleflightx.Group[int, int]{}}, cache)

	// @TODO: test locks
	// @TODO: more tests
}

func TestHotCache_Set(t *testing.T) {
	is := assert.New(t)

	// simple set
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	cache.Set("a", 1)
	is.Equal(1, cache.cache.Len())
	v, ok := cache.cache.Get("a")
	is.True(ok)
	is.EqualValues(&item[int]{true, 1, 8, 0, 0}, v)

	// simple set with copy on write
	cache = NewHotCache[string, int](LRU, 10).
		WithCopyOnWrite(func(v int) int {
			return v * 2
		}).
		Build()
	cache.Set("a", 1)
	is.Equal(1, cache.cache.Len())
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.EqualValues(&item[int]{true, 2, 8, 0, 0}, v)

	// simple set with default ttl + stale + jitter
	cache = NewHotCache[string, int](LRU, 10).
		WithTTL(1*time.Second).
		WithRevalidation(1*time.Second).
		WithJitter(2, 100*time.Millisecond).
		Build()
	cache.Set("a", 1)
	is.Equal(1, cache.cache.Len())
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+1_000_000, v.expiryMicro, 1_100_000)
	is.InEpsilon(time.Now().UnixNano()+1_000_000+1_000_000, v.staleExpiryMicro, 1_100_000)

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_SetMissing(t *testing.T) {
	is := assert.New(t)

	// no missing cache
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	is.Panics(func() {
		cache.SetMissing("a")
	})

	// dedicated cache
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LRU, 42).
		WithTTL(1*time.Second).
		WithRevalidation(100*time.Millisecond).
		WithJitter(2, 100*time.Millisecond).
		Build()
	cache.SetMissing("a")
	is.Equal(0, cache.cache.Len())
	is.Equal(1, cache.missingCache.Len())
	v, ok := cache.missingCache.Get("a")
	is.True(ok)
	is.False(v.hasValue)
	is.Equal(0, v.value)
	is.Equal(uint(0), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+1_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+1_000_000+100_000, v.staleExpiryMicro, 110_000)

	// shared cache
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		WithTTL(1*time.Second).
		WithRevalidation(100*time.Millisecond).
		WithJitter(2, 100*time.Millisecond).
		Build()
	cache.SetMissing("a")
	is.Equal(1, cache.cache.Len())
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.False(v.hasValue)
	is.Equal(0, v.value)
	is.Equal(uint(0), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+1_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+1_000_000+100_000, v.staleExpiryMicro, 110_000)

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_SetWithTTL(t *testing.T) {
	is := assert.New(t)

	// simple set
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	cache.SetWithTTL("a", 1, 10*time.Second)
	is.Equal(1, cache.cache.Len())
	v, ok := cache.cache.Get("a")
	is.True(ok)
	is.Equal(1, v.value)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+10_000_000, v.expiryMicro, 10_000)
	is.InEpsilon(time.Now().UnixNano()+10_000_000+100_000, v.staleExpiryMicro, 10_000)

	// simple set with copy on write
	cache = NewHotCache[string, int](LRU, 10).
		WithCopyOnWrite(func(v int) int {
			return v * 2
		}).
		Build()
	cache.SetWithTTL("a", 1, 10*time.Second)
	is.Equal(1, cache.cache.Len())
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.Equal(2, v.value)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+10_000_000, v.expiryMicro, 10_000)
	is.InEpsilon(time.Now().UnixNano()+10_000_000+100_000, v.staleExpiryMicro, 10_000)

	// simple set with default ttl + stale + jitter
	cache = NewHotCache[string, int](LRU, 10).
		WithTTL(1*time.Second).
		WithRevalidation(100*time.Millisecond).
		WithJitter(2, 100*time.Millisecond).
		Build()
	cache.SetWithTTL("a", 1, 10*time.Second)
	is.Equal(1, cache.cache.Len())
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+10_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+10_000_000+100_000, v.staleExpiryMicro, 110_000)

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_SetMissingWithTTL(t *testing.T) {
	is := assert.New(t)

	// no missing cache
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	is.Panics(func() {
		cache.SetMissingWithTTL("a", 10*time.Second)
	})

	// dedicated cache
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LRU, 42).
		WithTTL(1*time.Second).
		WithRevalidation(100*time.Millisecond).
		WithJitter(2, 100*time.Millisecond).
		Build()
	cache.SetMissingWithTTL("a", 10*time.Second)
	is.Equal(0, cache.cache.Len())
	is.Equal(1, cache.missingCache.Len())
	v, ok := cache.missingCache.Get("a")
	is.True(ok)
	is.False(v.hasValue)
	is.Equal(0, v.value)
	is.Equal(uint(0), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+10_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+10_000_000+100_000, v.staleExpiryMicro, 110_000)

	// shared cache
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		WithTTL(1*time.Second).
		WithRevalidation(100*time.Millisecond).
		WithJitter(2, 100*time.Millisecond).
		Build()
	cache.SetMissingWithTTL("a", 10*time.Second)
	is.Equal(1, cache.cache.Len())
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.False(v.hasValue)
	is.Equal(0, v.value)
	is.Equal(uint(0), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+10_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+10_000_000+100_000, v.staleExpiryMicro, 110_000)

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_SetMany(t *testing.T) {
	is := assert.New(t)

	// simple set
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	cache.SetMany(map[string]int{"a": 1, "b": 2})
	is.Equal(2, cache.cache.Len())
	v, ok := cache.cache.Get("a")
	is.True(ok)
	is.EqualValues(&item[int]{true, 1, 8, 0, 0}, v)
	v, ok = cache.cache.Get("b")
	is.True(ok)
	is.EqualValues(&item[int]{true, 2, 8, 0, 0}, v)

	// simple set with copy on write
	cache = NewHotCache[string, int](LRU, 10).
		WithCopyOnWrite(func(v int) int {
			return v * 2
		}).
		Build()
	cache.SetMany(map[string]int{"a": 1, "b": 2})
	is.Equal(2, cache.cache.Len())
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.EqualValues(&item[int]{true, 2, 8, 0, 0}, v)
	v, ok = cache.cache.Get("b")
	is.True(ok)
	is.EqualValues(&item[int]{true, 4, 8, 0, 0}, v)

	// simple set with default ttl + stale + jitter
	cache = NewHotCache[string, int](LRU, 10).
		WithTTL(1*time.Second).
		WithRevalidation(100*time.Millisecond).
		WithJitter(2, 100*time.Millisecond).
		Build()
	cache.SetMany(map[string]int{"a": 1, "b": 2})
	is.Equal(2, cache.cache.Len())
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+1_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+1_000_000+100_000, v.staleExpiryMicro, 110_000)
	v, ok = cache.cache.Get("b")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(2, v.value)
	is.Equal(uint(8), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+1_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+1_000_000+100_000, v.staleExpiryMicro, 110_000)

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_SetMissingMany(t *testing.T) {
	is := assert.New(t)

	// no missing cache
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	is.Panics(func() {
		cache.SetMissingMany([]string{"a", "b"})
	})

	// dedicated cache
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LRU, 42).
		WithTTL(1*time.Second).
		WithRevalidation(100*time.Millisecond).
		WithJitter(2, 100*time.Millisecond).
		Build()
	cache.SetMissingMany([]string{"a", "b"})
	is.Equal(0, cache.cache.Len())
	is.Equal(2, cache.missingCache.Len())
	v, ok := cache.missingCache.Get("a")
	is.True(ok)
	is.False(v.hasValue)
	is.Equal(0, v.value)
	is.Equal(uint(0), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+1_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+1_000_000+100_000, v.staleExpiryMicro, 110_000)
	v, ok = cache.missingCache.Get("b")
	is.True(ok)
	is.False(v.hasValue)
	is.Equal(0, v.value)
	is.Equal(uint(0), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+1_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+1_000_000+100_000, v.staleExpiryMicro, 110_000)

	// shared cache
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		WithTTL(1*time.Second).
		WithRevalidation(100*time.Millisecond).
		WithJitter(2, 100*time.Millisecond).
		Build()
	cache.SetMissingMany([]string{"a", "b"})
	is.Equal(2, cache.cache.Len())
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.False(v.hasValue)
	is.Equal(0, v.value)
	is.Equal(uint(0), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+1_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+1_000_000+100_000, v.staleExpiryMicro, 110_000)
	v, ok = cache.cache.Get("b")
	is.True(ok)
	is.False(v.hasValue)
	is.Equal(0, v.value)
	is.Equal(uint(0), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+1_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+1_000_000+100_000, v.staleExpiryMicro, 110_000)

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_SetManyWithTTL(t *testing.T) {
	is := assert.New(t)

	// simple set
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	cache.SetManyWithTTL(map[string]int{"a": 1, "b": 2}, 10*time.Second)
	is.Equal(2, cache.cache.Len())
	v, ok := cache.cache.Get("a")
	is.True(ok)
	is.Equal(1, v.value)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+10_000_000, v.expiryMicro, 10_000)
	is.InEpsilon(time.Now().UnixNano()+10_000_000+100_000, v.staleExpiryMicro, 10_000)
	v, ok = cache.cache.Get("b")
	is.True(ok)
	is.Equal(2, v.value)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+10_000_000, v.expiryMicro, 10_000)
	is.InEpsilon(time.Now().UnixNano()+10_000_000+100_000, v.staleExpiryMicro, 10_000)

	// simple set with copy on write
	cache = NewHotCache[string, int](LRU, 10).
		WithCopyOnWrite(func(v int) int {
			return v * 2
		}).
		Build()
	cache.SetManyWithTTL(map[string]int{"a": 1, "b": 2}, 10*time.Second)
	is.Equal(2, cache.cache.Len())
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.Equal(2, v.value)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+10_000_000, v.expiryMicro, 10_000)
	is.InEpsilon(time.Now().UnixNano()+10_000_000+100_000, v.staleExpiryMicro, 10_000)
	v, ok = cache.cache.Get("b")
	is.True(ok)
	is.Equal(4, v.value)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+10_000_000, v.expiryMicro, 10_000)
	is.InEpsilon(time.Now().UnixNano()+10_000_000+100_000, v.staleExpiryMicro, 10_000)

	// simple set with default ttl + stale + jitter
	cache = NewHotCache[string, int](LRU, 10).
		WithTTL(1*time.Second).
		WithRevalidation(100*time.Millisecond).
		WithJitter(2, 100*time.Millisecond).
		Build()
	cache.SetManyWithTTL(map[string]int{"a": 1, "b": 2}, 10*time.Second)
	is.Equal(2, cache.cache.Len())
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+10_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+10_000_000+100_000, v.staleExpiryMicro, 110_000)
	v, ok = cache.cache.Get("b")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(2, v.value)
	is.Equal(uint(8), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+10_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+10_000_000+100_000, v.staleExpiryMicro, 110_000)

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_SetMissingManyWithTTL(t *testing.T) {
	is := assert.New(t)

	// no missing cache
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	is.Panics(func() {
		cache.SetMissingManyWithTTL([]string{"a", "b"}, 10*time.Second)
	})

	// dedicated cache
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LRU, 42).
		WithTTL(1*time.Second).
		WithRevalidation(100*time.Millisecond).
		WithJitter(2, 100*time.Millisecond).
		Build()
	cache.SetMissingManyWithTTL([]string{"a", "b"}, 10*time.Second)
	is.Equal(0, cache.cache.Len())
	is.Equal(2, cache.missingCache.Len())
	v, ok := cache.missingCache.Get("a")
	is.True(ok)
	is.False(v.hasValue)
	is.Equal(0, v.value)
	is.Equal(uint(0), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+10_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+10_000_000+100_000, v.staleExpiryMicro, 110_000)
	v, ok = cache.missingCache.Get("b")
	is.True(ok)
	is.False(v.hasValue)
	is.Equal(0, v.value)
	is.Equal(uint(0), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+10_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+10_000_000+100_000, v.staleExpiryMicro, 110_000)

	// shared cache
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		WithTTL(1*time.Second).
		WithRevalidation(100*time.Millisecond).
		WithJitter(2, 100*time.Millisecond).
		Build()
	cache.SetMissingManyWithTTL([]string{"a", "b"}, 10*time.Second)
	is.Equal(2, cache.cache.Len())
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.False(v.hasValue)
	is.Equal(0, v.value)
	is.Equal(uint(0), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+10_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+10_000_000+100_000, v.staleExpiryMicro, 110_000)
	v, ok = cache.cache.Get("b")
	is.True(ok)
	is.False(v.hasValue)
	is.Equal(0, v.value)
	is.Equal(uint(0), v.bytes)
	is.NotEqual(v.expiryMicro, v.staleExpiryMicro)
	is.InEpsilon(time.Now().UnixNano()+10_000_000, v.expiryMicro, 110_000)
	is.InEpsilon(time.Now().UnixNano()+10_000_000+100_000, v.staleExpiryMicro, 110_000)

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_Has(t *testing.T) {
	is := assert.New(t)

	// simple
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	is.False(cache.Has("a"))
	cache.Set("a", 1)
	is.True(cache.Has("a"))

	// with shared missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		Build()
	is.False(cache.Has("a"))
	cache.Set("a", 1)
	is.True(cache.Has("a"))
	cache.SetMissing("a")
	is.False(cache.Has("a"))

	// with dedicated missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LRU, 10).
		Build()
	is.False(cache.Has("a"))
	cache.Set("a", 1)
	is.True(cache.Has("a"))
	cache.SetMissing("a")
	is.False(cache.Has("a"))
}

func TestHotCache_HasMany(t *testing.T) {
	is := assert.New(t)

	// simple
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	is.Equal(map[string]bool{"a": false, "b": false}, cache.HasMany([]string{"a", "b"}))
	cache.Set("a", 1)
	is.Equal(map[string]bool{"a": true, "b": false}, cache.HasMany([]string{"a", "b"}))
	cache.Set("b", 2)
	is.Equal(map[string]bool{"a": true, "b": true}, cache.HasMany([]string{"a", "b"}))

	// with shared missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		Build()
	is.Equal(map[string]bool{"a": false, "b": false}, cache.HasMany([]string{"a", "b"}))
	cache.Set("a", 1)
	is.Equal(map[string]bool{"a": true, "b": false}, cache.HasMany([]string{"a", "b"}))
	cache.SetMissing("b")
	is.Equal(map[string]bool{"a": true, "b": false}, cache.HasMany([]string{"a", "b"}))

	// with dedicated missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LRU, 10).
		Build()
	is.Equal(map[string]bool{"a": false, "b": false}, cache.HasMany([]string{"a", "b"}))
	cache.Set("a", 1)
	is.Equal(map[string]bool{"a": true, "b": false}, cache.HasMany([]string{"a", "b"}))
	cache.SetMissing("b")
	is.Equal(map[string]bool{"a": true, "b": false}, cache.HasMany([]string{"a", "b"}))
}

func TestHotCache_Get(t *testing.T) {
	is := assert.New(t)

	// simple
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	v, ok, err := cache.Get("a")
	is.False(ok)
	is.Nil(err)
	is.Equal(0, v)
	cache.Set("a", 42)
	v, ok, err = cache.Get("a")
	is.True(ok)
	is.Nil(err)
	is.Equal(42, v)

	// with shared missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		Build()
	v, ok, err = cache.Get("a")
	is.False(ok)
	is.Nil(err)
	is.Equal(0, v)
	cache.Set("a", 42)
	v, ok, err = cache.Get("a")
	is.True(ok)
	is.Nil(err)
	is.Equal(42, v)
	cache.SetMissing("a")
	v, ok, err = cache.Get("a")
	is.False(ok)
	is.Nil(err)
	is.Equal(0, v)

	// with dedicated missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LRU, 10).
		Build()
	v, ok, err = cache.Get("a")
	is.False(ok)
	is.Nil(err)
	is.Equal(0, v)
	cache.Set("a", 42)
	v, ok, err = cache.Get("a")
	is.True(ok)
	is.Nil(err)
	is.Equal(42, v)
	cache.SetMissing("a")
	v, ok, err = cache.Get("a")
	is.False(ok)
	is.Nil(err)
	is.Equal(0, v)

	// with copy on read
	cache = NewHotCache[string, int](LRU, 10).
		WithCopyOnRead(func(v int) int {
			return v * 2
		}).
		Build()
	v, ok, err = cache.Get("a")
	is.False(ok)
	is.Nil(err)
	is.Equal(0, v)
	cache.Set("a", 42)
	v, ok, err = cache.Get("a")
	is.True(ok)
	is.Nil(err)
	is.Equal(84, v)

	// with loader
	cache = NewHotCache[string, int](LRU, 10).
		WithLoaders(func(keys []string) (map[string]int, error) {
			is.Equal([]string{"a"}, keys)
			return map[string]int{"a": 42}, nil
		}).
		Build()
	v, ok, err = cache.Get("a")
	is.True(ok)
	is.Nil(err)
	is.Equal(42, v)

	// with failed loader
	cache = NewHotCache[string, int](LRU, 10).
		WithLoaders(func(keys []string) (map[string]int, error) {
			is.Equal([]string{"a"}, keys)
			return map[string]int{"a": 42}, assert.AnError
		}).
		Build()
	v, ok, err = cache.Get("a")
	is.False(ok)
	is.Error(assert.AnError, err)
	is.Equal(0, v)

	// with loader not found
	cache = NewHotCache[string, int](LRU, 10).
		WithLoaders(func(keys []string) (map[string]int, error) {
			is.Equal([]string{"a"}, keys)
			return map[string]int{}, nil
		}).
		Build()
	v, ok, err = cache.Get("a")
	is.False(ok)
	is.Nil(err)
	is.Equal(0, v)

	// with loader chain
	loaded := 0
	cache = NewHotCache[string, int](LRU, 10).
		WithLoaders(
			func(keys []string) (map[string]int, error) {
				loaded++
				is.Equal([]string{"a"}, keys)
				return map[string]int{}, nil
			},
			func(keys []string) (map[string]int, error) {
				loaded++
				is.Equal([]string{"a"}, keys)
				return nil, nil
			},
			func(keys []string) (map[string]int, error) {
				loaded++
				is.Equal([]string{"a"}, keys)
				return map[string]int{"a": 42}, nil
			},
		).
		Build()
	v, ok, err = cache.Get("a")
	is.True(ok)
	is.Nil(err)
	is.Equal(42, v)
	is.Equal(3, loaded)
}

// func TestHotCache_GetWithLoaders(t *testing.T) {
// }

// func TestHotCache_GetMany(t *testing.T) {
// }

// func TestHotCache_GetManyWithLoaders(t *testing.T) {
// }

func TestHotCache_Peek(t *testing.T) {
	is := assert.New(t)

	counter := int32(0)

	// simple
	cache := NewHotCache[string, int](LRU, 10).
		WithLoaders(func(keys []string) (map[string]int, error) {
			atomic.AddInt32(&counter, 1)
			return map[string]int{"a": 42}, nil
		}).
		WithCopyOnRead(func(nb int) int {
			return nb * 2
		}).
		Build()
	v, ok := cache.Peek("a")
	is.False(ok)
	is.Equal(0, v)
	cache.Set("a", 1)
	v, ok = cache.Peek("a")
	is.True(ok)
	is.Equal(2, v)
	is.Equal(int32(0), atomic.LoadInt32(&counter))

	// shared missing
	cache = NewHotCache[string, int](LRU, 10).
		WithLoaders(func(keys []string) (map[string]int, error) {
			atomic.AddInt32(&counter, 1)
			return map[string]int{"a": 42}, nil
		}).
		WithCopyOnRead(func(nb int) int {
			return nb * 2
		}).
		WithMissingSharedCache().
		Build()
	v, ok = cache.Peek("a")
	is.False(ok)
	is.Equal(0, v)
	cache.Set("a", 1)
	v, ok = cache.Peek("a")
	is.True(ok)
	is.Equal(2, v)
	is.Equal(int32(0), atomic.LoadInt32(&counter))

	// dedicated missing
	cache = NewHotCache[string, int](LRU, 10).
		WithLoaders(func(keys []string) (map[string]int, error) {
			atomic.AddInt32(&counter, 1)
			return map[string]int{"a": 42}, nil
		}).
		WithCopyOnRead(func(nb int) int {
			return nb * 2
		}).
		WithMissingCache(LRU, 10).
		Build()
	v, ok = cache.Peek("a")
	is.False(ok)
	is.Equal(0, v)
	cache.Set("a", 1)
	v, ok = cache.Peek("a")
	is.True(ok)
	is.Equal(2, v)
	is.Equal(int32(0), atomic.LoadInt32(&counter))
}

func TestHotCache_PeekMany(t *testing.T) {
	is := assert.New(t)

	counter := int32(0)

	// simple
	cache := NewHotCache[string, int](LRU, 10).
		WithLoaders(func(keys []string) (map[string]int, error) {
			atomic.AddInt32(&counter, 1)
			return map[string]int{"a": 42, "b": 84}, nil
		}).
		WithCopyOnRead(func(nb int) int {
			return nb * 2
		}).
		Build()
	v, missing := cache.PeekMany([]string{"a", "b", "c"})
	is.EqualValues(map[string]int{}, v)
	is.EqualValues([]string{"a", "b", "c"}, missing)
	cache.Set("a", 1)
	cache.Set("b", 2)
	v, missing = cache.PeekMany([]string{"a", "b", "c"})
	is.EqualValues(map[string]int{"a": 2, "b": 4}, v)
	is.EqualValues([]string{"c"}, missing)
	is.Equal(int32(0), atomic.LoadInt32(&counter))

	// shared missing
	cache = NewHotCache[string, int](LRU, 10).
		WithLoaders(func(keys []string) (map[string]int, error) {
			atomic.AddInt32(&counter, 1)
			return map[string]int{"a": 42, "b": 84}, nil
		}).
		WithCopyOnRead(func(nb int) int {
			return nb * 2
		}).
		WithMissingSharedCache().
		Build()
	v, missing = cache.PeekMany([]string{"a", "b", "c"})
	is.EqualValues(map[string]int{}, v)
	is.EqualValues([]string{"a", "b", "c"}, missing)
	cache.Set("a", 1)
	cache.Set("b", 2)
	v, missing = cache.PeekMany([]string{"a", "b", "c"})
	is.EqualValues(map[string]int{"a": 2, "b": 4}, v)
	is.EqualValues([]string{"c"}, missing)
	is.Equal(int32(0), atomic.LoadInt32(&counter))

	// dedicated missing
	cache = NewHotCache[string, int](LRU, 10).
		WithLoaders(func(keys []string) (map[string]int, error) {
			atomic.AddInt32(&counter, 1)
			return map[string]int{"a": 42, "b": 84}, nil
		}).
		WithCopyOnRead(func(nb int) int {
			return nb * 2
		}).
		WithMissingCache(LRU, 10).
		Build()
	v, missing = cache.PeekMany([]string{"a", "b", "c"})
	is.EqualValues(map[string]int{}, v)
	is.EqualValues([]string{"a", "b", "c"}, missing)
	cache.Set("a", 1)
	cache.Set("b", 2)
	v, missing = cache.PeekMany([]string{"a", "b", "c"})
	is.EqualValues(map[string]int{"a": 2, "b": 4}, v)
	is.EqualValues([]string{"c"}, missing)
	is.Equal(int32(0), atomic.LoadInt32(&counter))
}

func TestHotCache_Keys(t *testing.T) {
	is := assert.New(t)

	// simple
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	is.Equal([]string{}, cache.Keys())
	cache.Set("a", 1)
	is.Equal([]string{"a"}, cache.Keys())
	cache.Set("b", 2)
	is.ElementsMatch([]string{"a", "b"}, cache.Keys())

	// with shared missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		Build()
	is.Equal([]string{}, cache.Keys())
	cache.Set("a", 1)
	is.Equal([]string{"a"}, cache.Keys())
	cache.Set("b", 2)
	is.ElementsMatch([]string{"a", "b"}, cache.Keys())
	cache.SetMissing("c")
	is.ElementsMatch([]string{"a", "b"}, cache.Keys())

	// with dedicated missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LRU, 10).
		Build()
	is.Equal([]string{}, cache.Keys())
	cache.Set("a", 1)
	is.Equal([]string{"a"}, cache.Keys())
	cache.Set("b", 2)
	is.ElementsMatch([]string{"a", "b"}, cache.Keys())
	cache.SetMissing("c")
	is.ElementsMatch([]string{"a", "b"}, cache.Keys())
}

func TestHotCache_Values(t *testing.T) {
	is := assert.New(t)

	// simple
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	is.ElementsMatch([]int{}, cache.Values())
	cache.Set("a", 1)
	is.ElementsMatch([]int{1}, cache.Values())
	cache.Set("b", 2)
	is.ElementsMatch([]int{1, 2}, cache.Values())

	// with shared missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		Build()
	is.ElementsMatch([]int{}, cache.Values())
	cache.Set("a", 1)
	is.ElementsMatch([]int{1}, cache.Values())
	cache.Set("b", 2)
	is.ElementsMatch([]int{1, 2}, cache.Values())
	cache.SetMissing("c")
	is.ElementsMatch([]int{1, 2}, cache.Values())

	// with dedicated missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LRU, 10).
		Build()
	is.ElementsMatch([]int{}, cache.Values())
	cache.Set("a", 1)
	is.ElementsMatch([]int{1}, cache.Values())
	cache.Set("b", 2)
	is.ElementsMatch([]int{1, 2}, cache.Values())
	cache.SetMissing("c")
	is.ElementsMatch([]int{1, 2}, cache.Values())
}

func TestHotCache_Range(t *testing.T) {
	is := assert.New(t)

	// normal
	cache := NewHotCache[string, int](LRU, 10).
		Build()

	counter1 := int32(0)
	cache.Range(func(string, int) bool {
		atomic.AddInt32(&counter1, 1)
		return true
	})
	is.Equal(int32(0), atomic.LoadInt32(&counter1))
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Range(func(string, int) bool {
		atomic.AddInt32(&counter1, 1)
		return true
	})
	is.Equal(int32(2), atomic.LoadInt32(&counter1))
	cache.Range(func(string, int) bool {
		atomic.AddInt32(&counter1, 1)
		return false
	})
	is.Equal(int32(3), atomic.LoadInt32(&counter1))

	// shared missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		Build()

	counter2 := int32(0)
	cache.Range(func(string, int) bool {
		atomic.AddInt32(&counter2, 1)
		return true
	})
	is.Equal(int32(0), atomic.LoadInt32(&counter2))
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.SetMissing("c")
	cache.Range(func(string, int) bool {
		atomic.AddInt32(&counter2, 1)
		return true
	})
	is.Equal(int32(2), atomic.LoadInt32(&counter2))
	cache.Range(func(string, int) bool {
		atomic.AddInt32(&counter2, 1)
		return false
	})
	is.Equal(int32(3), atomic.LoadInt32(&counter2))

	// dedicated missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		Build()

	counter3 := int32(0)
	cache.Range(func(string, int) bool {
		atomic.AddInt32(&counter3, 1)
		return true
	})
	is.Equal(int32(0), atomic.LoadInt32(&counter3))
	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.SetMissing("c")
	cache.Range(func(string, int) bool {
		atomic.AddInt32(&counter3, 1)
		return true
	})
	is.Equal(int32(2), atomic.LoadInt32(&counter3))
	cache.Range(func(string, int) bool {
		atomic.AddInt32(&counter3, 1)
		return false
	})
	is.Equal(int32(3), atomic.LoadInt32(&counter3))

}

func TestHotCache_Delete(t *testing.T) {
	is := assert.New(t)

	// normal
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	cache.Set("a", 1)
	is.Equal(1, cache.Len())
	is.True(cache.Delete("a"))
	is.False(cache.Delete("a"))
	is.Equal(0, cache.Len())

	// shared missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		Build()
	cache.Set("a", 1)
	cache.SetMissing("b")
	is.Equal(2, cache.Len())
	is.True(cache.Delete("a"))
	is.False(cache.Delete("a"))
	is.True(cache.Delete("b"))
	is.False(cache.Delete("b"))
	is.Equal(0, cache.Len())

	// dedicated missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LFU, 42).
		Build()
	cache.Set("a", 1)
	cache.SetMissing("b")
	is.Equal(2, cache.Len())
	is.True(cache.Delete("a"))
	is.False(cache.Delete("a"))
	is.True(cache.Delete("b"))
	is.False(cache.Delete("b"))
	is.Equal(0, cache.Len())
}

func TestHotCache_DeleteMany(t *testing.T) {
	is := assert.New(t)

	// normal
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	cache.Set("a", 1)
	is.Equal(1, cache.Len())
	is.EqualValues(map[string]bool{"a": true, "b": false}, cache.DeleteMany([]string{"a", "b"}))
	is.EqualValues(map[string]bool{"a": false, "b": false}, cache.DeleteMany([]string{"a", "b"}))
	is.Equal(0, cache.Len())

	// shared missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		Build()
	cache.Set("a", 1)
	cache.SetMissing("b")
	is.Equal(2, cache.Len())
	is.EqualValues(map[string]bool{"a": true, "b": true, "c": false}, cache.DeleteMany([]string{"a", "b", "c"}))
	is.EqualValues(map[string]bool{"a": false, "b": false, "c": false}, cache.DeleteMany([]string{"a", "b", "c"}))
	is.Equal(0, cache.Len())

	// dedicated missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LFU, 42).
		Build()
	cache.Set("a", 1)
	cache.SetMissing("b")
	is.Equal(2, cache.Len())
	is.EqualValues(map[string]bool{"a": true, "b": true, "c": false}, cache.DeleteMany([]string{"a", "b", "c"}))
	is.EqualValues(map[string]bool{"a": false, "b": false, "c": false}, cache.DeleteMany([]string{"a", "b", "c"}))
	is.Equal(0, cache.Len())
}

func TestHotCache_Purge(t *testing.T) {
	is := assert.New(t)

	// normal
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	cache.Set("a", 1)
	is.Equal(1, cache.Len())
	cache.Purge()
	is.Equal(0, cache.Len())

	// shared missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		Build()
	cache.Set("a", 1)
	cache.SetMissing("b")
	is.Equal(2, cache.Len())
	cache.Purge()
	is.Equal(0, cache.Len())

	// dedicated missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LFU, 42).
		Build()
	cache.Set("a", 1)
	cache.SetMissing("b")
	is.Equal(2, cache.Len())
	cache.Purge()
	is.Equal(0, cache.Len())
}

func TestHotCache_Capacity(t *testing.T) {
	is := assert.New(t)

	// normal
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	a, b := cache.Capacity()
	is.Equal(10, a)
	is.Equal(0, b)

	// shared missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		Build()
	a, b = cache.Capacity()
	is.Equal(10, a)
	is.Equal(0, b)

	// dedicated missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LFU, 42).
		Build()
	a, b = cache.Capacity()
	is.Equal(10, a)
	is.Equal(42, b)
}

func TestHotCache_Algorithm(t *testing.T) {
	is := assert.New(t)

	// normal
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	a, b := cache.Algorithm()
	is.Equal("lru", a)
	is.Equal("", b)

	// shared missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		Build()
	a, b = cache.Algorithm()
	is.Equal("lru", a)
	is.Equal("", b)

	// dedicated missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LFU, 42).
		Build()
	a, b = cache.Algorithm()
	is.Equal("lru", a)
	is.Equal("lfu", b)
}

func TestHotCache_Len(t *testing.T) {
	is := assert.New(t)

	// normal
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	is.Equal(0, cache.Len())
	cache.Set("a", 1)
	is.Equal(1, cache.Len())
	cache.Set("b", 2)
	is.Equal(2, cache.Len())

	// shared missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		Build()
	is.Equal(0, cache.Len())
	cache.Set("a", 1)
	cache.SetMissing("c")
	is.Equal(2, cache.Len())
	cache.Set("b", 2)
	cache.SetMissing("d")
	is.Equal(4, cache.Len())

	// dedicated missing
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LFU, 42).
		Build()
	is.Equal(0, cache.Len())
	cache.Set("a", 1)
	cache.SetMissing("c")
	is.Equal(2, cache.Len())
	cache.Set("b", 2)
	cache.SetMissing("d")
	is.Equal(4, cache.Len())
}

func TestHotCache_WarmUp(t *testing.T) {
	is := assert.New(t)

	is.Panics(func() {
		_ = NewHotCache[string, int](LRU, 10).
			WithCopyOnWrite(func(nb int) int {
				return nb * 2
			}).
			WithWarmUp(func() (map[string]int, []string, error) {
				return map[string]int{"a": 1}, []string{"b"}, nil
			}).
			Build()
	})

	// simple
	cache := NewHotCache[string, int](LRU, 10).
		WithCopyOnWrite(func(nb int) int {
			return nb * 2
		}).
		WithWarmUp(func() (map[string]int, []string, error) {
			return map[string]int{"a": 1}, []string{}, nil
		}).
		Build()
	time.Sleep(5 * time.Millisecond)
	v, ok, err := cache.Get("a")
	is.True(ok)
	is.Nil(err)
	is.Equal(2, v)

	// with shared missing
	cache = NewHotCache[string, int](LRU, 10).
		WithCopyOnWrite(func(nb int) int {
			return nb * 2
		}).
		WithWarmUp(func() (map[string]int, []string, error) {
			return map[string]int{"a": 1}, []string{"b"}, nil
		}).
		WithMissingSharedCache().
		Build()
	time.Sleep(5 * time.Millisecond)
	v2, ok2 := cache.cache.Get("a")
	is.True(ok2)
	is.True(v2.hasValue)
	is.Equal(2, v2.value)
	v2, ok2 = cache.cache.Get("b")
	is.True(ok2)
	is.False(v2.hasValue)
	is.Equal(0, v2.value)

	// with dedicated missing
	cache = NewHotCache[string, int](LRU, 10).
		WithCopyOnWrite(func(nb int) int {
			return nb * 2
		}).
		WithWarmUp(func() (map[string]int, []string, error) {
			return map[string]int{"a": 1}, []string{"b"}, nil
		}).
		WithMissingCache(LRU, 10).
		Build()
	time.Sleep(5 * time.Millisecond)
	v2, ok2 = cache.cache.Get("a")
	is.True(ok2)
	is.True(v2.hasValue)
	is.Equal(2, v2.value)
	v2, ok2 = cache.missingCache.Get("b")
	is.True(ok2)
	is.False(v2.hasValue)
	is.Equal(0, v2.value)

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_Janitor(t *testing.T) {
	is := assert.New(t)

	// simple
	cache := NewHotCache[string, int](LRU, 10).
		WithTTL(3*time.Millisecond).
		WithRevalidation(20*time.Millisecond, func(keys []string) (found map[string]int, err error) {
			return map[string]int{"a": 2}, nil
		}).
		WithJanitor().
		Build()

	cache.Set("a", 1)
	is.Equal(1, cache.Len())
	time.Sleep(10 * time.Millisecond)
	is.Equal(1, cache.Len())
	time.Sleep(30 * time.Millisecond)
	is.Equal(0, cache.Len())

	cache.StopJanitor()

	// with dedicated missing
	cache = NewHotCache[string, int](LRU, 10).
		WithTTL(3*time.Millisecond).
		WithRevalidation(20*time.Millisecond, func(keys []string) (found map[string]int, err error) {
			return map[string]int{"a": 2}, nil
		}).
		WithMissingCache(LRU, 10).
		WithJanitor().
		Build()

	cache.Set("a", 1)
	cache.SetMissing("b")
	is.Equal(2, cache.Len())
	time.Sleep(10 * time.Millisecond)
	is.Equal(2, cache.Len())
	time.Sleep(25 * time.Millisecond)
	is.Equal(0, cache.Len())

	cache.StopJanitor()

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_setUnsafe_noMissingCache(t *testing.T) {
	is := assert.New(t)

	cache := NewHotCache[string, int](LRU, 10).
		Build()

	cache.setUnsafe("a", false, 1, 0) // no value + no ttl + no jitter
	v, ok := cache.cache.Get("a")
	is.False(ok)
	is.Nil(v)

	cache.setUnsafe("a", true, 1, 0) // value + no ttl + no jitter
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.Equal(int64(0), v.expiryMicro)
	is.Equal(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	cache.setUnsafe("a", true, 1, 100) // value + ttl + no jitter
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.NotEqual(int64(0), v.expiryMicro)
	is.NotEqual(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	cache = NewHotCache[string, int](LRU, 10).
		WithJitter(2, 500*time.Millisecond).
		Build()

	cache.setUnsafe("a", true, 1, 0) // value + no ttl + jitter
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.Equal(int64(0), v.expiryMicro)
	is.Equal(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	cache.setUnsafe("a", true, 1, 100) // value + ttl + jitter
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.NotEqual(int64(0), v.expiryMicro)
	is.NotEqual(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_setUnsafe_sharedMissingCache(t *testing.T) {
	is := assert.New(t)

	cache := NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		Build()

	cache.setUnsafe("a", false, 1, 0) // no value + no ttl + no jitter
	v, ok := cache.cache.Get("a")
	is.True(ok)
	is.False(v.hasValue)
	is.Equal(0, v.value)
	is.Equal(uint(0), v.bytes)
	is.Equal(int64(0), v.expiryMicro)
	is.Equal(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	cache.setUnsafe("a", true, 1, 0) // value + no ttl + no jitter
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.Equal(int64(0), v.expiryMicro)
	is.Equal(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	cache.setUnsafe("a", true, 1, 100) // value + ttl + no jitter
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.NotEqual(int64(0), v.expiryMicro)
	is.NotEqual(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		WithJitter(2, 500*time.Millisecond).
		Build()

	cache.setUnsafe("a", true, 1, 0) // value + no ttl + jitter
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.Equal(int64(0), v.expiryMicro)
	is.Equal(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	cache.setUnsafe("a", true, 1, 100) // value + ttl + jitter
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.NotEqual(int64(0), v.expiryMicro)
	is.NotEqual(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_setUnsafe_dedicatedMissingCache(t *testing.T) {
	is := assert.New(t)

	cache := NewHotCache[string, int](LRU, 10).
		WithMissingCache(LRU, 10).
		Build()

	cache.setUnsafe("a", false, 1, 0) // no value + no ttl + no jitter
	v, ok := cache.cache.Get("a")
	is.False(ok)
	is.Nil(v)
	v, ok = cache.missingCache.Get("a")
	is.True(ok)
	is.False(v.hasValue)
	is.Equal(0, v.value)
	is.Equal(uint(0), v.bytes)
	is.Equal(int64(0), v.expiryMicro)
	is.Equal(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	cache.setUnsafe("a", true, 1, 0) // value + no ttl + no jitter
	v, ok = cache.missingCache.Get("a")
	is.False(ok)
	is.Nil(v)
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.Equal(int64(0), v.expiryMicro)
	is.Equal(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	cache.setUnsafe("a", true, 1, 100) // value + ttl + no jitter
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.NotEqual(int64(0), v.expiryMicro)
	is.NotEqual(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LRU, 10).
		WithJitter(2, 500*time.Millisecond).
		Build()

	cache.setUnsafe("a", true, 1, 0) // value + no ttl + jitter
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.Equal(int64(0), v.expiryMicro)
	is.Equal(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	cache.setUnsafe("a", true, 1, 100) // value + ttl + jitter
	v, ok = cache.cache.Get("a")
	is.True(ok)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.NotEqual(int64(0), v.expiryMicro)
	is.NotEqual(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_setManyUnsafe(t *testing.T) {
	is := assert.New(t)

	// simple
	cache := NewHotCache[string, int](LRU, 10).
		Build()
	cache.setManyUnsafe(map[string]int{"a": 1, "b": 2}, []string{"c"}, 0)
	is.Equal(2, cache.cache.Len())

	// shared missing cache
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		Build()
	cache.setManyUnsafe(map[string]int{"a": 1, "b": 2}, []string{"c"}, 0)
	is.Equal(3, cache.cache.Len())
	cache.setManyUnsafe(map[string]int{"a": 1}, []string{"c", "b"}, 0)
	is.Equal(3, cache.cache.Len())

	// dedicated missing cache
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LRU, 10).
		Build()
	cache.setManyUnsafe(map[string]int{"a": 1, "b": 2}, []string{"c"}, 0)
	is.Equal(2, cache.cache.Len())
	is.Equal(1, cache.missingCache.Len())
	cache.setManyUnsafe(map[string]int{"a": 1}, []string{"c", "b"}, 0)
	is.Equal(1, cache.cache.Len())
	is.Equal(2, cache.missingCache.Len())

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_getUnsafe(t *testing.T) {
	t.Parallel()
	is := assert.New(t)

	// simple
	cache := NewHotCache[string, int](LRU, 10).
		WithRevalidation(10 * time.Millisecond).
		Build()
	cache.setManyUnsafe(map[string]int{"a": 1}, []string{"b"}, 0)
	cache.setUnsafe("c", true, 3, (2 * time.Millisecond).Microseconds())
	v, revalidate, found := cache.getUnsafe("a")
	is.True(found)
	is.False(revalidate)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.Equal(int64(0), v.expiryMicro)
	is.Equal(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	v, revalidate, found = cache.getUnsafe("b")
	is.False(found)
	is.False(revalidate)
	is.Nil(v)

	v, revalidate, found = cache.getUnsafe("c")
	is.True(found)
	is.False(revalidate)
	is.NotNil(v)
	is.Equal(2, cache.cache.Len())

	time.Sleep(5 * time.Millisecond)
	v, revalidate, found = cache.getUnsafe("c")
	is.True(found)
	is.True(revalidate)
	is.NotNil(v)
	is.Equal(2, cache.cache.Len())

	time.Sleep(15 * time.Millisecond)
	v, revalidate, found = cache.getUnsafe("c")
	is.False(found)
	is.False(revalidate)
	is.Nil(v)
	is.Equal(1, cache.cache.Len())

	// shared missing cache
	cache = NewHotCache[string, int](LRU, 10).
		WithRevalidation(10 * time.Millisecond).
		WithMissingSharedCache().
		Build()
	cache.setManyUnsafe(map[string]int{"a": 1}, []string{"b"}, 0)
	cache.setUnsafe("c", true, 3, (2 * time.Millisecond).Microseconds())
	v, revalidate, found = cache.getUnsafe("a")
	is.True(found)
	is.False(revalidate)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.Equal(int64(0), v.expiryMicro)
	is.Equal(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	v, revalidate, found = cache.getUnsafe("b")
	is.True(found)
	is.False(revalidate)
	is.False(v.hasValue)
	is.Equal(0, v.value)
	is.Equal(uint(0), v.bytes)
	is.Equal(int64(0), v.expiryMicro)
	is.Equal(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	v, revalidate, found = cache.getUnsafe("c")
	is.True(found)
	is.False(revalidate)
	is.NotNil(v)
	is.Equal(3, cache.cache.Len())

	time.Sleep(5 * time.Millisecond)
	v, revalidate, found = cache.getUnsafe("c")
	is.True(found)
	is.True(revalidate)
	is.NotNil(v)
	is.Equal(3, cache.cache.Len())

	time.Sleep(10 * time.Millisecond)
	v, revalidate, found = cache.getUnsafe("c")
	is.False(found)
	is.False(revalidate)
	is.Nil(v)
	is.Equal(2, cache.cache.Len())

	// dedicated missing cache
	cache = NewHotCache[string, int](LRU, 10).
		WithRevalidation(10*time.Millisecond).
		WithMissingCache(LRU, 10).
		Build()
	cache.setManyUnsafe(map[string]int{"a": 1}, []string{"b"}, 0)
	cache.setUnsafe("c", false, 0, (2 * time.Millisecond).Microseconds())
	v, revalidate, found = cache.getUnsafe("a")
	is.True(found)
	is.False(revalidate)
	is.True(v.hasValue)
	is.Equal(1, v.value)
	is.Equal(uint(8), v.bytes)
	is.Equal(int64(0), v.expiryMicro)
	is.Equal(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	v, revalidate, found = cache.getUnsafe("b")
	is.True(found)
	is.False(revalidate)
	is.False(v.hasValue)
	is.Equal(0, v.value)
	is.Equal(uint(0), v.bytes)
	is.Equal(int64(0), v.expiryMicro)
	is.Equal(int64(0), v.staleExpiryMicro)
	is.Equal(v.expiryMicro, v.staleExpiryMicro)

	v, revalidate, found = cache.getUnsafe("c")
	is.True(found)
	is.False(revalidate)
	is.NotNil(v)
	is.Equal(2, cache.missingCache.Len())

	time.Sleep(5 * time.Millisecond)
	v, revalidate, found = cache.getUnsafe("c")
	is.True(found)
	is.True(revalidate)
	is.NotNil(v)
	is.Equal(2, cache.missingCache.Len())

	time.Sleep(15 * time.Millisecond)
	v, revalidate, found = cache.getUnsafe("c")
	is.False(found)
	is.False(revalidate)
	is.Nil(v)
	is.Equal(1, cache.missingCache.Len())

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_getManyUnsafe(t *testing.T) {
	t.Parallel()
	is := assert.New(t)

	// simple
	cache := NewHotCache[string, int](LRU, 10).
		WithRevalidation(10 * time.Millisecond).
		Build()
	cache.setManyUnsafe(map[string]int{"a": 1}, []string{"b"}, 0)
	cache.setUnsafe("c", true, 3, (2 * time.Millisecond).Microseconds())
	v, missing, revalidate := cache.getManyUnsafe([]string{"a"})
	is.Len(v, 1)
	is.Len(missing, 0)
	is.Len(revalidate, 0)

	v, missing, revalidate = cache.getManyUnsafe([]string{"b"})
	is.Len(v, 0)
	is.Len(missing, 1)
	is.Len(revalidate, 0)

	v, missing, revalidate = cache.getManyUnsafe([]string{"c"})
	is.Len(v, 1)
	is.Len(missing, 0)
	is.Len(revalidate, 0)
	is.Equal(2, cache.cache.Len())

	time.Sleep(5 * time.Millisecond)
	v, missing, revalidate = cache.getManyUnsafe([]string{"c"})
	is.Len(v, 1)
	is.Len(missing, 0)
	is.Len(revalidate, 1)
	is.Equal(2, cache.cache.Len())

	time.Sleep(10 * time.Millisecond)
	v, missing, revalidate = cache.getManyUnsafe([]string{"c"})
	is.Len(v, 0)
	is.Len(missing, 1)
	is.Len(revalidate, 0)
	is.Equal(1, cache.cache.Len())

	// shared missing cache
	cache = NewHotCache[string, int](LRU, 10).
		WithRevalidation(10 * time.Millisecond).
		WithMissingSharedCache().
		Build()
	cache.setManyUnsafe(map[string]int{"a": 1}, []string{"b"}, 0)
	cache.setUnsafe("c", true, 3, (2 * time.Millisecond).Microseconds())
	v, missing, revalidate = cache.getManyUnsafe([]string{"a"})
	is.Len(v, 1)
	is.Len(missing, 0)
	is.Len(revalidate, 0)

	v, missing, revalidate = cache.getManyUnsafe([]string{"b"})
	is.Len(v, 1)
	is.Len(missing, 0)
	is.Len(revalidate, 0)

	v, missing, revalidate = cache.getManyUnsafe([]string{"c"})
	is.Len(v, 1)
	is.Len(missing, 0)
	is.Len(revalidate, 0)
	is.Equal(3, cache.cache.Len())

	time.Sleep(5 * time.Millisecond)
	v, missing, revalidate = cache.getManyUnsafe([]string{"c"})
	is.Len(v, 1)
	is.Len(missing, 0)
	is.Len(revalidate, 1)
	is.Equal(3, cache.cache.Len())

	time.Sleep(15 * time.Millisecond)
	v, missing, revalidate = cache.getManyUnsafe([]string{"c"})
	is.Len(v, 0)
	is.Len(missing, 1)
	is.Len(revalidate, 0)
	is.Equal(2, cache.cache.Len())

	// dedicated missing cache
	cache = NewHotCache[string, int](LRU, 10).
		WithRevalidation(10*time.Millisecond).
		WithMissingCache(LRU, 10).
		Build()
	cache.setManyUnsafe(map[string]int{"a": 1}, []string{"b"}, 0)
	cache.setUnsafe("c", false, 0, (2 * time.Millisecond).Microseconds())
	v, missing, revalidate = cache.getManyUnsafe([]string{"a"})
	is.Len(v, 1)
	is.Len(missing, 0)
	is.Len(revalidate, 0)

	v, missing, revalidate = cache.getManyUnsafe([]string{"b"})
	is.Len(v, 1)
	is.Len(missing, 0)
	is.Len(revalidate, 0)

	v, missing, revalidate = cache.getManyUnsafe([]string{"c"})
	is.Len(v, 1)
	is.Len(missing, 0)
	is.Len(revalidate, 0)
	is.Equal(2, cache.missingCache.Len())

	time.Sleep(5 * time.Millisecond)
	v, missing, revalidate = cache.getManyUnsafe([]string{"c"})
	is.Len(v, 1)
	is.Len(missing, 0)
	is.Len(revalidate, 1)
	is.Equal(2, cache.missingCache.Len())

	time.Sleep(15 * time.Millisecond)
	v, missing, revalidate = cache.getManyUnsafe([]string{"c"})
	is.Len(v, 0)
	is.Len(missing, 1)
	is.Len(revalidate, 0)
	is.Equal(1, cache.missingCache.Len())

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_loadAndSetMany(t *testing.T) {
	is := assert.New(t)

	counter1 := int32(0)
	counter2 := int32(0)
	counter3 := int32(0)

	// simple
	cache := NewHotCache[string, int](LRU, 10).
		WithCopyOnWrite(func(nb int) int {
			return nb * 42
		}).
		Build()

	cache.Purge()
	v, err := cache.loadAndSetMany(
		[]string{"a", "b"},
		LoaderChain[string, int]{},
	)
	is.Nil(err)
	is.NotNil(v)
	is.False(v["a"].hasValue)
	is.False(v["b"].hasValue)
	is.Len(v, 2)

	cache.Purge()
	v, err = cache.loadAndSetMany(
		[]string{},
		LoaderChain[string, int]{
			func(keys []string) (map[string]int, error) {
				atomic.AddInt32(&counter1, 1)
				is.ElementsMatch([]string{"a", "b"}, keys)
				return map[string]int{"a": 1, "c": 3}, nil
			},
			func(keys []string) (map[string]int, error) {
				atomic.AddInt32(&counter1, 1)
				is.ElementsMatch([]string{"b"}, keys)
				return map[string]int{"a": 2}, nil
			},
		},
	)
	is.Nil(err)
	is.NotNil(v)
	is.Len(v, 0)

	cache.Purge()
	v, err = cache.loadAndSetMany(
		[]string{"a"},
		LoaderChain[string, int]{
			func(keys []string) (map[string]int, error) {
				atomic.AddInt32(&counter1, 1)
				return nil, assert.AnError
			},
			func(keys []string) (map[string]int, error) {
				atomic.AddInt32(&counter1, 1)
				is.ElementsMatch([]string{"b"}, keys)
				return map[string]int{"a": 2}, nil
			},
		},
	)
	is.EqualError(err, assert.AnError.Error())
	is.NotNil(v)
	is.Len(v, 0)
	is.Equal(int32(1), atomic.LoadInt32(&counter1))

	cache.Purge()
	v, err = cache.loadAndSetMany(
		[]string{"a", "b"},
		LoaderChain[string, int]{
			func(keys []string) (map[string]int, error) {
				atomic.AddInt32(&counter1, 1)
				is.ElementsMatch([]string{"a", "b"}, keys)
				return map[string]int{"a": 1, "c": 3}, nil
			},
			func(keys []string) (map[string]int, error) {
				atomic.AddInt32(&counter1, 1)
				is.ElementsMatch([]string{"b"}, keys)
				return map[string]int{"a": 2}, nil
			},
		},
	)
	is.Nil(err)
	is.Len(v, 2)
	is.True(v["a"].hasValue)
	is.False(v["b"].hasValue)
	is.Equal(84, v["a"].value)
	is.Equal(int32(3), atomic.LoadInt32(&counter1))
	is.Equal(2, cache.cache.Len()) // "c=3" is cached

	// shared missing cache
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingSharedCache().
		WithCopyOnWrite(func(nb int) int {
			return nb * 42
		}).
		Build()
	v, err = cache.loadAndSetMany(
		[]string{"a", "b"},
		LoaderChain[string, int]{
			func(keys []string) (map[string]int, error) {
				atomic.AddInt32(&counter2, 1)
				is.ElementsMatch([]string{"a", "b"}, keys)
				return map[string]int{"a": 1, "c": 3}, nil
			},
			func(keys []string) (map[string]int, error) {
				atomic.AddInt32(&counter2, 1)
				is.ElementsMatch([]string{"b"}, keys)
				return map[string]int{"a": 2}, nil
			},
		},
	)
	is.Nil(err)
	is.Len(v, 2)
	is.True(v["a"].hasValue)
	is.False(v["b"].hasValue)
	is.Equal(84, v["a"].value)
	is.Equal(int32(2), atomic.LoadInt32(&counter2))
	is.Equal(3, cache.cache.Len())

	// dedicated missing cache
	cache = NewHotCache[string, int](LRU, 10).
		WithMissingCache(LRU, 10).
		WithCopyOnWrite(func(nb int) int {
			return nb * 42
		}).
		Build()
	v, err = cache.loadAndSetMany(
		[]string{"a", "b"},
		LoaderChain[string, int]{
			func(keys []string) (map[string]int, error) {
				atomic.AddInt32(&counter3, 1)
				is.ElementsMatch([]string{"a", "b"}, keys)
				return map[string]int{"a": 1, "c": 3}, nil
			},
			func(keys []string) (map[string]int, error) {
				atomic.AddInt32(&counter3, 1)
				is.ElementsMatch([]string{"b"}, keys)
				return map[string]int{"a": 2}, nil
			},
		},
	)
	is.Nil(err)
	is.Len(v, 2)
	is.True(v["a"].hasValue)
	is.False(v["b"].hasValue)
	is.Equal(84, v["a"].value)
	is.Equal(int32(2), atomic.LoadInt32(&counter3))
	is.Equal(2, cache.cache.Len())
	is.Equal(1, cache.missingCache.Len())

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}

func TestHotCache_revalidate(t *testing.T) {
	is := assert.New(t)

	counter1 := int32(0)
	counter2 := int32(0)

	// with revalidation
	cache := NewHotCache[string, int](LRU, 10).
		WithTTL(1*time.Millisecond).
		WithRevalidation(10*time.Millisecond, func(keys []string) (found map[string]int, err error) {
			time.Sleep(1 * time.Millisecond)
			atomic.AddInt32(&counter1, 1)
			return map[string]int{"a": 2}, nil
		}).
		Build()

	cache.Set("a", 1)

	is.True(cache.Has("a"))
	time.Sleep(5 * time.Millisecond)
	is.True(cache.Has("a"))
	_, _, _ = cache.Get("a")
	is.True(cache.Has("a"))
	time.Sleep(5 * time.Millisecond)
	is.True(cache.Has("a"))

	v, ok, err := cache.Get("a")
	is.True(ok)
	is.Nil(err)
	is.Equal(2, v)
	is.Equal(int32(1), atomic.LoadInt32(&counter1)) // revalidated async

	time.Sleep(15 * time.Millisecond)
	is.True(cache.Has("a"))

	v, ok, err = cache.Get("a")
	is.False(ok)
	is.Nil(err)
	is.Equal(0, v)
	is.Equal(int32(2), atomic.LoadInt32(&counter1))

	cache.Purge()

	// with loader
	cache = NewHotCache[string, int](LRU, 10).
		WithTTL(1 * time.Millisecond).
		WithLoaders(func(keys []string) (found map[string]int, err error) {
			time.Sleep(1 * time.Millisecond)
			atomic.AddInt32(&counter2, 1)
			return map[string]int{"a": 2}, nil
		}).
		WithRevalidation(10 * time.Millisecond).
		Build()

	cache.Set("a", 1)

	is.True(cache.Has("a"))
	time.Sleep(5 * time.Millisecond)
	is.True(cache.Has("a"))
	_, _, _ = cache.Get("a")
	is.True(cache.Has("a"))
	time.Sleep(5 * time.Millisecond)
	is.True(cache.Has("a"))

	v, ok, err = cache.Get("a")
	is.True(ok)
	is.Nil(err)
	is.Equal(2, v)
	is.Equal(int32(1), atomic.LoadInt32(&counter2)) // revalidated async

	time.Sleep(15 * time.Millisecond)
	is.True(cache.Has("a"))

	v, ok, err = cache.Get("a")
	is.True(ok)
	is.Nil(err)
	is.Equal(2, v)
	is.Equal(int32(3), atomic.LoadInt32(&counter2))

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}
