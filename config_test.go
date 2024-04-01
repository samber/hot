package hot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewInternalCache(t *testing.T) {
	is := assert.New(t)

	cache := NewInternalCache[string, int](LRU, 42)
	is.Equal(42, cache.Capacity())
	is.Equal("lru", cache.Algorithm())

	cache = NewInternalCache[string, int](LFU, 42)
	is.Equal(42, cache.Capacity())
	is.Equal("lfu", cache.Algorithm())

	cache = NewInternalCache[string, int](TwoQueue, 42)
	is.Equal(52, cache.Capacity())
	is.Equal("2q", cache.Algorithm())

	is.Panics(func() {
		_ = NewInternalCache[string, int](ARC, 0)
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

	cache1 := NewInternalCache[string, int](LRU, 42)
	cache2 := NewInternalCache[string, int](LFU, 21)
	// loader1 := func(keys []string) (map[string]int, []string, error) { return map[string]int{}, []string{}, nil }
	// loader2 := func(keys []string) (map[string]int, []string, error) { return map[string]int{}, []string{}, nil }
	// loaders := []Loader[string, int]{loader1, loader2}
	// warmUp := func(f func(map[string]int)) error { return nil }
	// twice := func(v int) { return v*2 }

	opts := NewHotCache[string, int](cache1)
	is.EqualValues(HotCacheConfig[string, int]{cache1, false, nil, 0, 0, 0, false, false, nil, nil, nil, nil, nil}, opts)

	opts = opts.WithMissingSharedCache()
	is.EqualValues(HotCacheConfig[string, int]{cache1, true, nil, 0, 0, 0, false, false, nil, nil, nil, nil, nil}, opts)

	opts = NewHotCache[string, int](cache1).WithMissingCache(cache2)
	is.EqualValues(HotCacheConfig[string, int]{cache1, false, cache2, 0, 0, 0, false, false, nil, nil, nil, nil, nil}, opts)

	is.Panics(func() {
		opts = opts.WithTTL(-42 * time.Second)
	})
	opts = opts.WithTTL(42 * time.Second)
	is.EqualValues(HotCacheConfig[string, int]{cache1, false, cache2, 42 * time.Second, 0, 0, false, false, nil, nil, nil, nil, nil}, opts)

	is.Panics(func() {
		opts = opts.WithRevalidation(-21 * time.Second)
	})
	// opts = opts.WithRevalidation(21*time.Second, loader1, loader2)
	// is.EqualValues(HotCacheConfig[string, int]{cache1, false, cache2, 42 * time.Second, 21 * time.Second, 0, false, false, nil, nil, loaders, nil, nil}, opts)

	is.Panics(func() {
		opts = opts.WithJitter(-0.1)
	})
	is.Panics(func() {
		opts = opts.WithJitter(1.1)
	})
	opts = opts.WithJitter(0.1)
	is.EqualValues(HotCacheConfig[string, int]{cache1, false, cache2, 42 * time.Second, 0, 0.1, false, false, nil, nil, nil, nil, nil}, opts)

	// opts = opts.WithWarmUp(warmUp)
	// is.EqualValues(HotCacheConfig[string, int]{cache1, false, cache2, 42 * time.Second, 0, 0.1, false, false, warmUp, nil, nil, nil, nil}, opts)

	opts = opts.WithoutLocking()
	is.EqualValues(HotCacheConfig[string, int]{cache1, false, cache2, 42 * time.Second, 0, 0.1, true, false, nil, nil, nil, nil, nil}, opts)

	opts = opts.WithJanitor()
	is.EqualValues(HotCacheConfig[string, int]{cache1, false, cache2, 42 * time.Second, 0, 0.1, true, true, nil, nil, nil, nil, nil}, opts)

	// opts = opts.WithCopyOnRead(twice)
	// is.EqualValues(HotCacheConfig[string, int]{cache1, false, cache2, 42 * time.Second, 0, 0.1, true, true, nil, nil, nil, twice, nil}, opts)

	// opts = opts.WithCopyOnRead(twice)
	// is.EqualValues(HotCacheConfig[string, int]{cache1, false, cache2, 42 * time.Second, 0, 0.1, true, true, nil, nil, nil, twice, twice}, opts)

	is.Panics(func() {
		opts.Build()
	})

	time.Sleep(10 * time.Millisecond) // purge revalidation goroutine
}
