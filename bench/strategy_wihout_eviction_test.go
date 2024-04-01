package bench

import (
	"testing"

	twoqueue "github.com/samber/hot/2q"
	"github.com/samber/hot/lfu"
	"github.com/samber/hot/lru"
)

func BenchmarkSetGetWithoutEvictionLRU(b *testing.B) {
	cache := lru.NewLRUCache[int, int](b.N + 100)
	for n := 0; n < b.N; n++ {
		cache.Set(n, n)
		cache.Get(n)
	}
}

func BenchmarkSetGetWithoutEvictionLFU(b *testing.B) {
	cache := lfu.NewLFUCache[int, int](b.N + 100)
	for n := 0; n < b.N; n++ {
		cache.Set(n, n)
		cache.Get(n)
	}
}

func BenchmarkSetGetWithoutEvictionTwoQueue(b *testing.B) {
	cache := twoqueue.New2QCache[int, int](b.N + 100)
	for n := 0; n < b.N; n++ {
		cache.Set(n, n)
		cache.Get(n)
	}
}

// func BenchmarkSetGetWithoutEvictionARC(b *testing.B) {
// 	cache := cache.NewARCCache[int, int](b.N+100)
// 	for n := 0; n < b.N; n++ {
// 		cache.Set(n, n)
// 		cache.Get(n)
// 	}
// }
