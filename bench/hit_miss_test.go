package bench

import (
	"testing"

	"github.com/samber/hot"
)

func BenchmarkHit(b *testing.B) {
	b.Run("SingleCache", func(b *testing.B) {
		cache := hot.NewHotCache[int, int](hot.LRU, b.N+100).
			Build()
		for n := 0; n < b.N; n++ {
			cache.Set(n, n)
			_, _, _ = cache.Get(0)
		}
	})

	b.Run("MissingSharedCache", func(b *testing.B) {
		cache := hot.NewHotCache[int, int](hot.LRU, b.N+100).
			WithMissingSharedCache().
			Build()
		for n := 0; n < b.N; n++ {
			cache.Set(n, n)
			_, _, _ = cache.Get(0)
		}
	})

	b.Run("DedicatedMissingCache", func(b *testing.B) {
		b.Run("First", func(b *testing.B) {
			cache := hot.NewHotCache[int, int](hot.LRU, b.N+100).
				WithMissingCache(hot.LRU, b.N+100).
				Build()
			for n := 0; n < b.N; n++ {
				cache.Set(n, n)
				_, _, _ = cache.Get(0)
			}
		})
		b.Run("Second", func(b *testing.B) {
			cache := hot.NewHotCache[int, int](hot.LRU, b.N+100).
				WithMissingCache(hot.LRU, b.N+100).
				Build()
			cache.SetMissing(b.N + 1)
			for n := 0; n < b.N; n++ {
				cache.Set(n, n)
				_, _, _ = cache.Get(b.N + 1)
			}
		})
	})
}

func BenchmarkMiss(b *testing.B) {
	b.Run("SingleCache", func(b *testing.B) {
		cache := hot.NewHotCache[int, int](hot.LRU, b.N+100).
			Build()
		for n := 0; n < b.N; n++ {
			cache.Set(n, n)
			_, _, _ = cache.Get(b.N + 1)
		}
	})

	b.Run("MissingSharedCache", func(b *testing.B) {
		cache := hot.NewHotCache[int, int](hot.LRU, b.N+100).
			WithMissingSharedCache().
			Build()
		for n := 0; n < b.N; n++ {
			cache.Set(n, n)
			_, _, _ = cache.Get(b.N + 1)
		}
	})

	b.Run("DedicatedMissingCache", func(b *testing.B) {
		cache := hot.NewHotCache[int, int](hot.LRU, b.N+100).
			WithMissingCache(hot.LRU, b.N+100).
			Build()
		for n := 0; n < b.N; n++ {
			cache.Set(n, n)
			_, _, _ = cache.Get(b.N + 1)
		}
	})
}
