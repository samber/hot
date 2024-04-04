package lru

import (
	"container/list"

	"github.com/samber/hot/internal"
	"github.com/samber/hot/pkg/base"
)

type entry[K comparable, V any] struct {
	key   K
	value V
}

func NewLRUCache[K comparable, V any](capacity int) *LRUCache[K, V] {
	return NewLRUCacheWithEvictionCallback[K, V](capacity, nil)
}

func NewLRUCacheWithEvictionCallback[K comparable, V any](capacity int, onEviction base.EvictionCallback[K, V]) *LRUCache[K, V] {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}

	return &LRUCache[K, V]{
		capacity: capacity,
		ll:       list.New(),
		cache:    make(map[K]*list.Element),

		onEviction: onEviction,
	}
}

// Cache is an LRU cache. It is not safe for concurrent access.
type LRUCache[K comparable, V any] struct {
	noCopy internal.NoCopy

	capacity int
	ll       *list.List // @TODO: build a custom list.List implementation
	cache    map[K]*list.Element

	onEviction base.EvictionCallback[K, V]
}

var _ base.InMemoryCache[string, int] = (*LRUCache[string, int])(nil)

// implements base.InMemoryCache
func (c *LRUCache[K, V]) Set(key K, value V) {
	if e, ok := c.cache[key]; ok {
		c.ll.MoveToFront(e)
		e.Value.(*entry[K, V]).value = value
		return
	}

	e := c.ll.PushFront(&entry[K, V]{key, value})
	c.cache[key] = e
	if c.capacity != 0 && c.ll.Len() > c.capacity {
		k, v, ok := c.DeleteOldest()
		if ok && c.onEviction != nil {
			c.onEviction(k, v)
		}
	}
}

// implements base.InMemoryCache
func (c *LRUCache[K, V]) Has(key K) bool {
	_, hit := c.cache[key]
	return hit
}

// implements base.InMemoryCache
func (c *LRUCache[K, V]) Get(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		c.ll.MoveToFront(e)
		return e.Value.(*entry[K, V]).value, true
	}
	return value, false
}

// implements base.InMemoryCache
func (c *LRUCache[K, V]) Peek(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		return e.Value.(*entry[K, V]).value, true
	}
	return value, false
}

// implements base.InMemoryCache
func (c *LRUCache[K, V]) Keys() []K {
	all := make([]K, 0, c.ll.Len())
	for k := range c.cache {
		all = append(all, k)
	}
	return all
}

// implements base.InMemoryCache
func (c *LRUCache[K, V]) Values() []V {
	all := make([]V, 0, c.ll.Len())
	for _, v := range c.cache {
		all = append(all, v.Value.(*entry[K, V]).value)
	}
	return all
}

// implements base.InMemoryCache
func (c *LRUCache[K, V]) Range(f func(K, V) bool) {
	for k, v := range c.cache {
		if !f(k, v.Value.(*entry[K, V]).value) {
			break
		}
	}
}

// implements base.InMemoryCache
func (c *LRUCache[K, V]) Delete(key K) bool {
	if e, hit := c.cache[key]; hit {
		c.deleteElement(e)
		return true
	}
	return false
}

// implements base.InMemoryCache
func (c *LRUCache[K, V]) SetMany(items map[K]V) {
	for k, v := range items {
		c.Set(k, v)
	}
}

// implements base.InMemoryCache
func (c *LRUCache[K, V]) HasMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Has(k)
	}
	return m
}

// implements base.InMemoryCache
func (c *LRUCache[K, V]) GetMany(keys []K) (map[K]V, []K) {
	m := make(map[K]V, len(keys))
	var missing []K
	for _, k := range keys {
		if v, ok := c.Get(k); ok {
			m[k] = v
		} else {
			missing = append(missing, k)
		}
	}
	return m, missing
}

// implements base.InMemoryCache
func (c *LRUCache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
	m := make(map[K]V, len(keys))
	var missing []K
	for _, k := range keys {
		if v, ok := c.Peek(k); ok {
			m[k] = v
		} else {
			missing = append(missing, k)
		}
	}
	return m, missing
}

// implements base.InMemoryCache
func (c *LRUCache[K, V]) DeleteMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Delete(k)
	}
	return m
}

// implements base.InMemoryCache
func (c *LRUCache[K, V]) Purge() {
	c.ll = list.New()
	c.cache = make(map[K]*list.Element)
}

// implements base.InMemoryCache
func (c *LRUCache[K, V]) Capacity() int {
	return c.capacity
}

// implements base.InMemoryCache
func (c *LRUCache[K, V]) Algorithm() string {
	return "lru"
}

// implements base.InMemoryCache
func (c *LRUCache[K, V]) Len() int {
	return c.ll.Len()
}

func (c *LRUCache[K, V]) DeleteOldest() (k K, v V, ok bool) {
	e := c.ll.Back()
	if e != nil {
		c.deleteElement(e)
		return e.Value.(*entry[K, V]).key, e.Value.(*entry[K, V]).value, true
	}

	return k, v, false
}

func (c *LRUCache[K, V]) deleteElement(e *list.Element) {
	c.ll.Remove(e)
	kv := e.Value.(*entry[K, V])
	delete(c.cache, kv.key)
}
