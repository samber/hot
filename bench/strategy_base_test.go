package bench

import (
	"testing"

	twoqueue "github.com/samber/hot/2q"
	"github.com/samber/hot/lfu"
	"github.com/samber/hot/lru"
)

func BenchmarkSetGetLRU(b *testing.B) {
	cache := lru.NewLRUCache[int, int](100)
	for n := 0; n < b.N; n++ {
		cache.Set(n, n)
		cache.Get(n)
	}
}

func BenchmarkSetGetLFU(b *testing.B) {
	cache := lfu.NewLFUCache[int, int](100)
	for n := 0; n < b.N; n++ {
		cache.Set(n, n)
		cache.Get(n)
	}
}

func BenchmarkSetGetTwoQueue(b *testing.B) {
	cache := twoqueue.New2QCache[int, int](100)
	for n := 0; n < b.N; n++ {
		cache.Set(n, n)
		cache.Get(n)
	}
}

// func BenchmarkSetGetARC(b *testing.B) {
// 	cache := cache.NewARCCache[int, int](100)
// 	for n := 0; n < b.N; n++ {
// 		cache.Set(n, n)
// 		cache.Get(n)
// 	}
// }
