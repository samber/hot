package base

import (
	"sync"
)

func NewSafeInMemoryCache[K comparable, V any](cache InMemoryCache[K, V]) InMemoryCache[K, V] {
	return &SafeInMemoryCache[K, V]{
		InMemoryCache: cache,
		RWMutex:       sync.RWMutex{},
	}
}

// SafeInMemoryCache is a cache with safe concurrent access.
type SafeInMemoryCache[K comparable, V any] struct {
	InMemoryCache[K, V]
	sync.RWMutex
}

var _ InMemoryCache[string, int] = (*SafeInMemoryCache[string, int])(nil)

// implements base.InMemoryCache
func (c *SafeInMemoryCache[K, V]) Set(key K, value V) {
	c.Lock()
	c.InMemoryCache.Set(key, value)
	c.Unlock()
}

// implements base.InMemoryCache
func (c *SafeInMemoryCache[K, V]) Has(key K) bool {
	c.RLock()
	defer c.RUnlock()
	return c.InMemoryCache.Has(key)
}

// implements base.InMemoryCache
func (c *SafeInMemoryCache[K, V]) Get(key K) (value V, ok bool) {
	// not read-only lock, because underlying cache may change the item
	c.Lock()
	defer c.Unlock()
	return c.InMemoryCache.Get(key)
}

// implements base.InMemoryCache
func (c *SafeInMemoryCache[K, V]) Peek(key K) (value V, ok bool) {
	c.RLock()
	defer c.RUnlock()
	return c.InMemoryCache.Peek(key)
}

// implements base.InMemoryCache
func (c *SafeInMemoryCache[K, V]) Keys() []K {
	c.RLock()
	defer c.RUnlock()
	return c.InMemoryCache.Keys()
}

// implements base.InMemoryCache
func (c *SafeInMemoryCache[K, V]) Values() []V {
	c.RLock()
	defer c.RUnlock()
	return c.InMemoryCache.Values()
}

// implements base.InMemoryCache
func (c *SafeInMemoryCache[K, V]) Range(f func(K, V) bool) {
	c.RLock()
	c.InMemoryCache.Range(f)
	c.RUnlock()
}

// implements base.InMemoryCache
func (c *SafeInMemoryCache[K, V]) Delete(key K) bool {
	c.Lock()
	defer c.Unlock()
	return c.InMemoryCache.Delete(key)
}

// implements base.InMemoryCache
func (c *SafeInMemoryCache[K, V]) Purge() {
	c.Lock()
	c.InMemoryCache.Purge()
	c.Unlock()
}

// implements base.InMemoryCache
func (c *SafeInMemoryCache[K, V]) SetMany(items map[K]V) {
	if len(items) == 0 {
		return
	}

	c.Lock()
	c.InMemoryCache.SetMany(items)
	c.Unlock()
}

// implements base.InMemoryCache
func (c *SafeInMemoryCache[K, V]) HasMany(keys []K) map[K]bool {
	if len(keys) == 0 {
		return map[K]bool{}
	}

	c.RLock()
	defer c.RUnlock()
	return c.InMemoryCache.HasMany(keys)
}

// implements base.InMemoryCache
func (c *SafeInMemoryCache[K, V]) GetMany(keys []K) (map[K]V, []K) {
	if len(keys) == 0 {
		return map[K]V{}, []K{}
	}

	c.Lock()
	defer c.Unlock()
	return c.InMemoryCache.GetMany(keys)
}

// implements base.InMemoryCache
func (c *SafeInMemoryCache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
	if len(keys) == 0 {
		return map[K]V{}, []K{}
	}

	c.RLock()
	defer c.RUnlock()
	return c.InMemoryCache.PeekMany(keys)
}

// implements base.InMemoryCache
func (c *SafeInMemoryCache[K, V]) DeleteMany(keys []K) map[K]bool {
	if len(keys) == 0 {
		return map[K]bool{}
	}

	c.Lock()
	defer c.Unlock()
	return c.InMemoryCache.DeleteMany(keys)
}

// // implements base.InMemoryCache
// func (c *SafeInMemoryCache[K, V]) Capacity() int {
// 	return c.Cache.Capacity()
// }

// // implements base.InMemoryCache
// func (c *SafeInMemoryCache[K, V]) Algorithm() string {
// 	return c.Cache.Algorithm()
// }

// implements base.InMemoryCache
func (c *SafeInMemoryCache[K, V]) Len() int {
	c.RLock()
	defer c.RUnlock()
	return c.InMemoryCache.Len()
}
