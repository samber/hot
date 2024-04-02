package bench

import (
	"testing"

	"github.com/samber/hot/pkg/lru"
	"github.com/samber/hot/pkg/safe"
)

func BenchmarkSetGetLRUUnsafe(b *testing.B) {
	cache := lru.NewLRUCache[int, int](100)
	for n := 0; n < b.N; n++ {
		cache.Set(n, n)
		cache.Get(n)
	}
}

func BenchmarkSetGetLRUWrapped(b *testing.B) {
	// cacheWrapper is a simple wrapper to base.InMemoryCache to test cost of a SafeCache abstraction, instead of embedding mutex in LRU.
	cache := newWrappedCache(lru.NewLRUCache[int, int](100))
	for n := 0; n < b.N; n++ {
		cache.Set(n, n)
		cache.Get(n)
	}
}

func BenchmarkSetGetLRUSafe(b *testing.B) {
	cache := safe.NewSafeInMemoryCache(lru.NewLRUCache[int, int](100))
	for n := 0; n < b.N; n++ {
		cache.Set(n, n)
		cache.Get(n)
	}
}
