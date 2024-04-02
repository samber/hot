package bench

import (
	"testing"
	"time"

	"github.com/samber/hot"
)

func BenchmarkSetGetWithTTL(b *testing.B) {
	cache := hot.NewHotCache[int, int](hot.LRU, b.N+100).
		WithTTL(10 * time.Millisecond).
		Build()
	for n := 0; n < b.N; n++ {
		cache.Set(n, n)
		_, _, _ = cache.Get(n)
	}
}

func BenchmarkSetGetWithTTLWithoutCheck(b *testing.B) {
	cache := hot.NewHotCache[int, int](hot.LRU, b.N+100).
		WithTTL(10 * time.Millisecond).
		Build()
	for n := 0; n < b.N; n++ {
		cache.Set(n, n)
		_, _ = cache.Peek(n) // No check of TTL
	}
}

func BenchmarkSetGetWithTTLAndStale(b *testing.B) {
	cache := hot.NewHotCache[int, int](hot.LRU, b.N+100).
		WithTTL(10 * time.Millisecond).
		WithRevalidation(10 * time.Millisecond).
		Build()
	for n := 0; n < b.N; n++ {
		cache.Set(n, n)
		_, _, _ = cache.Get(n)
	}
}

func BenchmarkSetGetWithTTLAndJanitor(b *testing.B) {
	cache := hot.NewHotCache[int, int](hot.LRU, b.N+100).
		WithTTL(10 * time.Millisecond).
		WithJanitor().
		Build()
	for n := 0; n < b.N; n++ {
		cache.Set(n, n)
		_, _, _ = cache.Get(n)
	}
}
