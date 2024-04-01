package hot

import (
	"testing"
	"time"

	"github.com/samber/hot/base"
	"github.com/stretchr/testify/assert"
)

func TestnewInternalCache(t *testing.T) {
	is := assert.New(t)

	cache := newInternalCache[string, int](true, LRU, 42)
	is.Equal(42, cache.Capacity())
	is.Equal("lru", cache.Algorithm())
	_, ok := cache.(*base.SafeInMemoryCache[string, *item[int]])
	is.True(ok)

	cache = newInternalCache[string, int](true, LFU, 42)
	is.Equal(42, cache.Capacity())
	is.Equal("lfu", cache.Algorithm())
	_, ok = cache.(*base.SafeInMemoryCache[string, *item[int]])
	is.True(ok)

	cache = newInternalCache[string, int](true, TwoQueue, 42)
	is.Equal(52, cache.Capacity())
	is.Equal("2q", cache.Algorithm())
	_, ok = cache.(*base.SafeInMemoryCache[string, *item[int]])
	is.True(ok)

	is.Panics(func() {
		_ = newInternalCache[string, int](true, ARC, 0)
	})

	cache = newInternalCache[string, int](false, LRU, 42)
	is.Equal(42, cache.Capacity())
	is.Equal("lru", cache.Algorithm())
	_, ok = cache.(*base.SafeInMemoryCache[string, *item[int]])
	is.True(ok)

	cache = newInternalCache[string, int](false, LFU, 42)
	is.Equal(42, cache.Capacity())
	is.Equal("lfu", cache.Algorithm())
	_, ok = cache.(*base.SafeInMemoryCache[string, *item[int]])
	is.True(ok)

	cache = newInternalCache[string, int](false, TwoQueue, 42)
	is.Equal(52, cache.Capacity())
	is.Equal("2q", cache.Algorithm())
	_, ok = cache.(*base.SafeInMemoryCache[string, *item[int]])
	is.True(ok)

	is.Panics(func() {
		_ = newInternalCache[string, int](false, ARC, 0)
	})

}

func TestAssertValue(t *testing.T) {
	is := assert.New(t)

	is.NotPanics(func() {
		assertValue(true, "error")
	})
	is.PanicsWithValue("error", func() {
		assertValue(false, "error")
	})
}

func TestHotCacheConfig(t *testing.T) {
	is := assert.New(t)

	// cache1 := newInternalCache[string, int](true, LRU, 42)
	// cache2 := newInternalCache[string, int](true, LFU, 21)
	// loader1 := func(keys []string) (map[string]int, []string, error) { return map[string]int{}, []string{}, nil }
	// loader2 := func(keys []string) (map[string]int, []string, error) { return map[string]int{}, []string{}, nil }
	// loaders := []Loader[string, int]{loader1, loader2}
	// warmUp := func(f func(map[string]int)) error { return nil }
	// twice := func(v int) { return v*2 }

	opts := NewHotCache[string, int](LRU, 42)
	is.EqualValues(HotCacheConfig[string, int]{LRU, 42, false, 0, 0, 0, 0, 0, false, false, nil, nil, nil, nil, nil}, opts)

	opts = opts.WithMissingSharedCache()
	is.EqualValues(HotCacheConfig[string, int]{LRU, 42, true, 0, 0, 0, 0, 0, false, false, nil, nil, nil, nil, nil}, opts)

	opts = NewHotCache[string, int](LRU, 42).WithMissingCache(LFU, 21)
	is.EqualValues(HotCacheConfig[string, int]{LRU, 42, false, LFU, 21, 0, 0, 0, false, false, nil, nil, nil, nil, nil}, opts)

	is.Panics(func() {
		opts = opts.WithTTL(-42 * time.Second)
	})
	opts = opts.WithTTL(42 * time.Second)
	is.EqualValues(HotCacheConfig[string, int]{LRU, 42, false, LFU, 21, 42 * time.Second, 0, 0, false, false, nil, nil, nil, nil, nil}, opts)

	is.Panics(func() {
		opts = opts.WithRevalidation(-21 * time.Second)
	})
	// opts = opts.WithRevalidation(21*time.Second, loader1, loader2)
	// is.EqualValues(HotCacheConfig[string, int]{LRU, 42, false, LFU, 21, 42 * time.Second, 21 * time.Second, 0, false, false, nil, nil, loaders, nil, nil}, opts)

	is.Panics(func() {
		opts = opts.WithJitter(-0.1)
	})
	is.Panics(func() {
		opts = opts.WithJitter(1.1)
	})
	opts = opts.WithJitter(0.1)
	is.EqualValues(HotCacheConfig[string, int]{LRU, 42, false, LFU, 21, 42 * time.Second, 0, 0.1, false, false, nil, nil, nil, nil, nil}, opts)

	// opts = opts.WithWarmUp(warmUp)
	// is.EqualValues(HotCacheConfig[string, int]{LRU, 42, false, LFU, 21, 42 * time.Second, 0, 0.1, false, false, warmUp, nil, nil, nil, nil}, opts)

	opts = opts.WithoutLocking()
	is.EqualValues(HotCacheConfig[string, int]{LRU, 42, false, LFU, 21, 42 * time.Second, 0, 0.1, true, false, nil, nil, nil, nil, nil}, opts)

	opts = opts.WithJanitor()
	is.EqualValues(HotCacheConfig[string, int]{LRU, 42, false, LFU, 21, 42 * time.Second, 0, 0.1, true, true, nil, nil, nil, nil, nil}, opts)

	// opts = opts.WithCopyOnRead(twice)
	// is.EqualValues(HotCacheConfig[string, int]{LRU, 42, false, LFU, 21, 42 * time.Second, 0, 0.1, true, true, nil, nil, nil, twice, nil}, opts)

	// opts = opts.WithCopyOnRead(twice)
	// is.EqualValues(HotCacheConfig[string, int]{LRU, 42, false, LFU, 21, 42 * time.Second, 0, 0.1, true, true, nil, nil, nil, twice, twice}, opts)

	is.Panics(func() {
		opts.Build()
	})

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}
