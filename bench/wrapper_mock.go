package bench

import (
	"github.com/samber/hot/pkg/base"
)

func newWrappedCache[K comparable, V any](cache base.InMemoryCache[K, V]) base.InMemoryCache[K, V] {
	return &wrappedCache[K, V]{
		InMemoryCache: cache,
	}
}

// wrappedCache is a simple wrapper to base.InMemoryCache to test cost during benchmark
type wrappedCache[K comparable, V any] struct {
	base.InMemoryCache[K, V]
}

var _ base.InMemoryCache[string, int] = (*wrappedCache[string, int])(nil)

func (c *wrappedCache[K, V]) Set(key K, value V) {
	c.InMemoryCache.Set(key, value)
}

func (c *wrappedCache[K, V]) Has(key K) bool {
	return c.InMemoryCache.Has(key)
}

func (c *wrappedCache[K, V]) Get(key K) (value V, ok bool) {
	return c.InMemoryCache.Get(key)
}

func (c *wrappedCache[K, V]) Peek(key K) (value V, ok bool) {
	return c.InMemoryCache.Peek(key)
}

func (c *wrappedCache[K, V]) Keys() []K {
	return c.InMemoryCache.Keys()
}

func (c *wrappedCache[K, V]) Values() []V {
	return c.InMemoryCache.Values()
}

func (c *wrappedCache[K, V]) Range(f func(K, V) bool) {
	c.InMemoryCache.Range(f)
}

func (c *wrappedCache[K, V]) Delete(key K) bool {
	return c.InMemoryCache.Delete(key)
}

func (c *wrappedCache[K, V]) Len() int {
	return c.InMemoryCache.Len()
}

func (c *wrappedCache[K, V]) Purge() {
	c.InMemoryCache.Len()
}
