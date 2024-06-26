package sharded

import (
	"github.com/samber/hot/internal"
	"github.com/samber/hot/pkg/base"
)

func NewShardedInMemoryCache[K comparable, V any](shards uint64, newCache func() base.InMemoryCache[K, V], fn Hasher[K]) base.InMemoryCache[K, V] {
	caches := map[uint64]base.InMemoryCache[K, V]{}
	for i := uint64(0); i < shards; i++ {
		caches[i] = newCache()
	}

	return &ShardedInMemoryCache[K, V]{
		shards: shards,
		fn:     fn,
		caches: caches,
	}
}

// ShardedInMemoryCache is a cache with safe concurrent access.
type ShardedInMemoryCache[K comparable, V any] struct {
	noCopy internal.NoCopy

	shards uint64
	fn     Hasher[K]
	caches map[uint64]base.InMemoryCache[K, V]
}

var _ base.InMemoryCache[string, int] = (*ShardedInMemoryCache[string, int])(nil)

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) Set(key K, value V) {
	c.caches[c.fn.computeHash(key, c.shards)].Set(key, value)
}

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) Has(key K) bool {
	return c.caches[c.fn.computeHash(key, c.shards)].Has(key)
}

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) Get(key K) (value V, ok bool) {
	return c.caches[c.fn.computeHash(key, c.shards)].Get(key)
}

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) Peek(key K) (value V, ok bool) {
	return c.caches[c.fn.computeHash(key, c.shards)].Peek(key)
}

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) Keys() []K {
	keys := []K{}
	for i := range c.caches {
		keys = append(keys, c.caches[i].Keys()...)
	}
	return keys
}

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) Values() []V {
	values := []V{}
	for i := range c.caches {
		values = append(values, c.caches[i].Values()...)
	}
	return values
}

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) Range(f func(K, V) bool) {
	ok := true
	for i := range c.caches {
		c.caches[i].Range(func(k K, v V) bool {
			ok = f(k, v)
			return ok
		})
		if !ok {
			return
		}
	}
}

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) Delete(key K) bool {
	return c.caches[c.fn.computeHash(key, c.shards)].Delete(key)
}

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) Purge() {
	for i := range c.caches {
		c.caches[i].Purge()
	}
}

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) SetMany(items map[K]V) {
	if len(items) == 0 {
		return
	}

	batch := map[uint64]map[K]V{}
	for k, v := range items {
		hash := c.fn.computeHash(k, c.shards)
		if batch[hash] == nil {
			batch[hash] = map[K]V{}
		}
		batch[hash][k] = v
	}

	for i := range batch {
		c.caches[i].SetMany(batch[i])
	}
}

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) HasMany(keys []K) map[K]bool {
	if len(keys) == 0 {
		return map[K]bool{}
	}

	batch := map[uint64][]K{}
	for _, k := range keys {
		hash := c.fn.computeHash(k, c.shards)
		if batch[hash] == nil {
			batch[hash] = []K{}
		}
		batch[hash] = append(batch[hash], k)
	}

	output := map[K]bool{}

	for i := range batch {
		local := c.caches[i].HasMany(batch[i])
		for k, v := range local {
			output[k] = v
		}
	}

	return output
}

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) GetMany(keys []K) (map[K]V, []K) {
	if len(keys) == 0 {
		return map[K]V{}, []K{}
	}

	batch := map[uint64][]K{}
	for _, k := range keys {
		hash := c.fn.computeHash(k, c.shards)
		if batch[hash] == nil {
			batch[hash] = []K{}
		}
		batch[hash] = append(batch[hash], k)
	}

	output := map[K]V{}
	missing := []K{}

	for i := range batch {
		localFound, localMissing := c.caches[i].GetMany(batch[i])
		for k, v := range localFound {
			output[k] = v
		}
		missing = append(missing, localMissing...)
	}

	return output, missing
}

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
	if len(keys) == 0 {
		return map[K]V{}, []K{}
	}

	batch := map[uint64][]K{}
	for _, k := range keys {
		hash := c.fn.computeHash(k, c.shards)
		if batch[hash] == nil {
			batch[hash] = []K{}
		}
		batch[hash] = append(batch[hash], k)
	}

	output := map[K]V{}
	missing := []K{}

	for i := range batch {
		localFound, localMissing := c.caches[i].PeekMany(batch[i])
		for k, v := range localFound {
			output[k] = v
		}
		missing = append(missing, localMissing...)
	}

	return output, missing
}

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) DeleteMany(keys []K) map[K]bool {
	if len(keys) == 0 {
		return map[K]bool{}
	}

	batch := map[uint64][]K{}
	for _, k := range keys {
		hash := c.fn.computeHash(k, c.shards)
		if batch[hash] == nil {
			batch[hash] = []K{}
		}
		batch[hash] = append(batch[hash], k)
	}

	output := map[K]bool{}

	for i := range batch {
		local := c.caches[i].DeleteMany(batch[i])
		for k, v := range local {
			output[k] = v
		}
	}

	return output
}

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) Capacity() int {
	return c.caches[0].Capacity() * int(c.shards)
}

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) Algorithm() string {
	return c.caches[0].Algorithm()
}

// implements base.InMemoryCache
func (c *ShardedInMemoryCache[K, V]) Len() int {
	sum := 0
	for i := range c.caches {
		sum += c.caches[i].Len()
	}
	return 0
}
