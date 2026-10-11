package hot

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/samber/hot/pkg/lru"
	"github.com/samber/hot/pkg/metrics"
	"github.com/samber/hot/pkg/safe"
	"github.com/stretchr/testify/assert"
)

func TestShardedCachePreservesLocking(t *testing.T) {
	t.Parallel()

	for _, locking := range []bool{false, true} {
		for _, instrumented := range []bool{false, true} {
			t.Run(fmt.Sprintf("locking=%t/metrics=%t", locking, instrumented), func(t *testing.T) {
				t.Parallel()

				var collectorBuilder func(int) metrics.Collector
				if instrumented {
					collectorBuilder = mockCollectorBuilder
				}
				cache := composeInternalCache[int, int](locking, LRU, 64, 4, -1,
					func(key int) uint64 { return uint64(key) }, nil, collectorBuilder)
				want := reflect.TypeOf((*lru.LRUCache[int, *item[int]])(nil))
				if locking {
					want = reflect.TypeOf((*safe.SafeInMemoryCache[int, *item[int]])(nil))
				}

				// Inspect types only; the shard implementations are not exported.
				shards := reflect.ValueOf(cache).Elem().FieldByName("caches")
				for i := 0; i < shards.Len(); i++ {
					shard := shards.Index(i).Elem()
					if instrumented {
						shard = shard.Elem().FieldByName("cache").Elem()
					}
					assert.Equal(t, want.String(), shard.Type().String(), "shard %d", i)
				}
			})
		}
	}
}

func TestHotCacheShardedConcurrentAccess(t *testing.T) {
	t.Parallel()

	for _, algorithm := range []EvictionAlgorithm{LRU, LFU} {
		t.Run(string(algorithm), func(t *testing.T) {
			t.Parallel()

			cache := NewHotCache[int, int](algorithm, 64).
				WithSharding(4, func(key int) uint64 { return uint64(key) }).
				WithMissingCache(algorithm, 64).
				Build()
			start := make(chan struct{})
			var workers sync.WaitGroup
			for worker := 0; worker < 8; worker++ {
				workers.Add(1)
				go func() {
					defer workers.Done()
					<-start
					for i := 0; i < 64; i++ {
						key := i % 64
						cache.Set(key, key)
						value, found, err := cache.Get(key)
						if err != nil || !found || value != key {
							t.Errorf("Get(%d) = %d, %t, %v", key, value, found, err)
							return
						}
						cache.SetMissing(key + 64)
						_, found, err = cache.Get(key + 64)
						if err != nil || found {
							t.Errorf("Get missing key %d: found=%t, err=%v", key+64, found, err)
							return
						}
					}
				}()
			}
			close(start)
			workers.Wait()
		})
	}
}

func BenchmarkHotCacheShardedConcurrentAccess(b *testing.B) {
	for _, shards := range []uint64{1, 16} {
		b.Run(fmt.Sprintf("shards=%d", shards), func(b *testing.B) {
			config := NewHotCache[int, int](LRU, 64)
			if shards > 1 {
				config = config.WithSharding(shards, func(key int) uint64 { return uint64(key) })
			}
			cache := config.Build()
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				key := 0
				for pb.Next() {
					cache.Set(key, key)
					_, _, _ = cache.Get(key)
					key = (key + 1) % 64
				}
			})
		})
	}
}
