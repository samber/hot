package lfu

import (
	"container/list"

	"github.com/samber/hot/internal"
	"github.com/samber/hot/pkg/base"
)

const (
	// DefaultEvictionSize is the number of element to evict when the cache is full.
	DefaultEvictionSize = 1
)

type entry[K comparable, V any] struct {
	key   K
	value V
}

func NewLFUCache[K comparable, V any](capacity int) *LFUCache[K, V] {
	return NewLFUCacheWithEvictionSizeAndCallback[K, V](capacity, DefaultEvictionSize, nil)
}

func NewLFUCacheWithEvictionCallback[K comparable, V any](capacity int, onEviction base.EvictionCallback[K, V]) *LFUCache[K, V] {
	return NewLFUCacheWithEvictionSizeAndCallback[K, V](capacity, DefaultEvictionSize, onEviction)
}

func NewLFUCacheWithEvictionSize[K comparable, V any](capacity int, evictionSize int) *LFUCache[K, V] {
	return NewLFUCacheWithEvictionSizeAndCallback[K, V](capacity, evictionSize, nil)
}

func NewLFUCacheWithEvictionSizeAndCallback[K comparable, V any](capacity int, evictionSize int, onEviction base.EvictionCallback[K, V]) *LFUCache[K, V] {
	if capacity <= 1 {
		panic("capacity must be greater than 0")
	}
	if evictionSize >= capacity {
		panic("capacity must be greater than evictionSize")
	}

	return &LFUCache[K, V]{
		capacity:     capacity,
		evictionSize: evictionSize,
		ll:           list.New(), // sorted from least to most frequent
		cache:        make(map[K]*list.Element),

		onEviction: onEviction,
	}
}

// Cache is an LFU cache. It is not safe for concurrent access.
type LFUCache[K comparable, V any] struct {
	noCopy internal.NoCopy

	capacity     int
	evictionSize int
	ll           *list.List // @TODO: build a custom list.List implementation
	cache        map[K]*list.Element

	onEviction base.EvictionCallback[K, V]
}

var _ base.InMemoryCache[string, int] = (*LFUCache[string, int])(nil)

// implements base.InMemoryCache
func (c *LFUCache[K, V]) Set(key K, value V) {
	if e, ok := c.cache[key]; ok {
		if e.Next() != nil {
			c.ll.MoveAfter(e, e.Next())
		}
		e.Value.(*entry[K, V]).value = value
		return
	}

	// pop front
	if c.ll.Len() >= c.capacity {
		for i := 0; i < c.evictionSize; i++ {
			k, v, ok := c.DeleteLeastFrequent()
			if ok && c.onEviction != nil {
				c.onEviction(k, v)
			}
		}
	}

	e := c.ll.PushFront(&entry[K, V]{key, value})
	c.cache[key] = e
}

// implements base.InMemoryCache
func (c *LFUCache[K, V]) Has(key K) bool {
	_, hit := c.cache[key]
	return hit
}

// implements base.InMemoryCache
func (c *LFUCache[K, V]) Get(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		if e.Next() != nil {
			c.ll.MoveAfter(e, e.Next())
		}
		return e.Value.(*entry[K, V]).value, true
	}
	return value, false
}

// implements base.InMemoryCache
func (c *LFUCache[K, V]) Peek(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		return e.Value.(*entry[K, V]).value, true
	}
	return value, false
}

// implements base.InMemoryCache
func (c *LFUCache[K, V]) Keys() []K {
	all := make([]K, 0, c.ll.Len())
	for k := range c.cache {
		all = append(all, k)
	}
	return all
}

// implements base.InMemoryCache
func (c *LFUCache[K, V]) Values() []V {
	all := make([]V, 0, c.ll.Len())
	for _, v := range c.cache {
		all = append(all, v.Value.(*entry[K, V]).value)
	}
	return all
}

// implements base.InMemoryCache
func (c *LFUCache[K, V]) Range(f func(K, V) bool) {
	for k, v := range c.cache {
		if !f(k, v.Value.(*entry[K, V]).value) {
			break
		}
	}
}

// implements base.InMemoryCache
func (c *LFUCache[K, V]) Delete(key K) bool {
	if e, hit := c.cache[key]; hit {
		c.deleteElement(e)
		return true
	}
	return false
}

// implements base.InMemoryCache
func (c *LFUCache[K, V]) Purge() {
	c.ll = list.New()
	c.cache = make(map[K]*list.Element)
}

// implements base.InMemoryCache
func (c *LFUCache[K, V]) SetMany(items map[K]V) {
	for k, v := range items {
		c.Set(k, v)
	}
}

// implements base.InMemoryCache
func (c *LFUCache[K, V]) HasMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Has(k)
	}
	return m
}

// implements base.InMemoryCache
func (c *LFUCache[K, V]) GetMany(keys []K) (map[K]V, []K) {
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
func (c *LFUCache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
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
func (c *LFUCache[K, V]) DeleteMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Delete(k)
	}
	return m
}

// implements base.InMemoryCache
func (c *LFUCache[K, V]) Capacity() int {
	return c.capacity
}

// implements base.InMemoryCache
func (c *LFUCache[K, V]) Algorithm() string {
	return "lfu"
}

// implements base.InMemoryCache
func (c *LFUCache[K, V]) Len() int {
	return c.ll.Len()
}

func (c *LFUCache[K, V]) DeleteLeastFrequent() (k K, v V, ok bool) {
	e := c.ll.Front()
	if e != nil {
		c.deleteElement(e)
		return e.Value.(*entry[K, V]).key, e.Value.(*entry[K, V]).value, true
	}

	return k, v, false
}

func (c *LFUCache[K, V]) deleteElement(e *list.Element) {
	c.ll.Remove(e)
	kv := e.Value.(*entry[K, V])
	delete(c.cache, kv.key)
}
