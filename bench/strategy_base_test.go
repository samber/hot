package bench

import (
	"testing"

	"github.com/samber/hot/pkg/lfu"
	"github.com/samber/hot/pkg/lru"
	"github.com/samber/hot/pkg/twoqueue"
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
