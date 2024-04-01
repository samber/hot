package twoqueue

import (
	"github.com/samber/hot/base"
	"github.com/samber/hot/lru"
)

const (
	// Default2QRecentRatio is the ratio of the 2Q cache dedicated
	// to recently added entries that have only been accessed once.
	Default2QRecentRatio = 0.25

	// Default2QGhostEntries is the default ratio of ghost
	// entries kept to track entries recently evicted
	Default2QGhostEntries = 0.50
)

func New2QCache[K comparable, V any](capacity int) *TwoQueueCache[K, V] {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}

	return New2QCacheWithRatio[K, V](capacity, Default2QRecentRatio, Default2QGhostEntries)
}

func New2QCacheWithRatio[K comparable, V any](capacity int, recentRatio, ghostRatio float64) *TwoQueueCache[K, V] {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}
	if recentRatio < 0.0 || recentRatio > 1.0 {
		panic("recentRatio must be between 0 and 1")
	}
	if ghostRatio < 0.0 || ghostRatio > 1.0 {
		panic("ghostRatio must be between 0 and 1")
	}

	// Determine the sub-capacities
	recentCapacity := int(float64(capacity) * recentRatio)
	recentEvictCapacity := int(float64(capacity) * ghostRatio)

	// Allocate the LRUs
	recent := lru.NewLRUCache[K, V](capacity)
	frequent := lru.NewLRUCache[K, V](capacity)
	recentEvict := lru.NewLRUCache[K, struct{}](recentEvictCapacity)

	return &TwoQueueCache[K, V]{
		capacity:            capacity,
		recentCapacity:      recentCapacity,
		recentEvictCapacity: recentEvictCapacity,
		recentRatio:         recentRatio,
		ghostRatio:          ghostRatio,

		recent:      recent,
		frequent:    frequent,
		recentEvict: recentEvict,
	}
}

// 2Q is an enhancement over the standard LRU cache
// in that it tracks both frequently and recently used
// entries separately. This avoids a burst in access to new
// entries from evicting frequently used entries. It adds some
// additional tracking overhead to the standard LRU cache, and is
// computationally about 2x the cost, and adds some metadata over
// head.
// TwoQueueCache is not safe for concurrent access.
type TwoQueueCache[K comparable, V any] struct {
	capacity            int
	recentCapacity      int
	recentEvictCapacity int
	recentRatio         float64
	ghostRatio          float64

	// @TODO: recent and recentEvict should be FIFO lists
	recent      *lru.LRUCache[K, V]        // @TODO: build a custom FIFO implementation
	frequent    *lru.LRUCache[K, V]        // @TODO: build a custom list.List implementation
	recentEvict *lru.LRUCache[K, struct{}] // @TODO: build a custom FIFO implementation
}

var _ base.InMemoryCache[string, int] = (*TwoQueueCache[string, int])(nil)

// implements base.InMemoryCache
func (c *TwoQueueCache[K, V]) Set(key K, value V) {
	// Check if the value is frequently used already,
	// and just update the value
	if c.frequent.Has(key) {
		c.frequent.Set(key, value)
		return
	}

	// Check if the value is recently used, and promote
	// the value into the frequent list
	if c.recent.Has(key) {
		c.recent.Delete(key)
		c.frequent.Set(key, value)
		return
	}

	// If the value was recently evicted, add it to the
	// frequently used list
	if c.recentEvict.Has(key) {
		c.ensureSpace(true)
		c.recentEvict.Delete(key)
		c.frequent.Set(key, value)
		return
	}

	// Add to the recently seen list
	c.ensureSpace(false)
	c.recent.Set(key, value)
}

// implements base.InMemoryCache
func (c *TwoQueueCache[K, V]) Has(key K) bool {
	return c.frequent.Has(key) || c.recent.Has(key)
}

// implements base.InMemoryCache
func (c *TwoQueueCache[K, V]) Get(key K) (value V, ok bool) {
	// Check if this is a frequent value
	if val, ok := c.frequent.Get(key); ok {
		return val, ok
	}

	// If the value is contained in recent, then we
	// promote it to frequent
	if val, ok := c.recent.Peek(key); ok {
		c.recent.Delete(key)
		c.frequent.Set(key, val)
		return val, ok
	}

	// No hit
	return value, false
}

// implements base.InMemoryCache
func (c *TwoQueueCache[K, V]) Peek(key K) (value V, ok bool) {
	// Check if this is a frequent value
	if val, ok := c.frequent.Peek(key); ok {
		return val, ok
	}

	// Check if this is a recent value
	return c.recent.Peek(key)
}

// implements base.InMemoryCache
func (c *TwoQueueCache[K, V]) Keys() []K {
	k1 := c.frequent.Keys()
	k2 := c.recent.Keys()
	return append(k1, k2...)
}

// implements base.InMemoryCache
func (c *TwoQueueCache[K, V]) Values() []V {
	v1 := c.frequent.Values()
	v2 := c.recent.Values()
	return append(v1, v2...)
}

// implements base.InMemoryCache
func (c *TwoQueueCache[K, V]) Range(f func(K, V) bool) {
	c.frequent.Range(f)
	c.recent.Range(f)
}

// implements base.InMemoryCache
func (c *TwoQueueCache[K, V]) Delete(key K) bool {
	return c.frequent.Delete(key) || c.recent.Delete(key) || c.recentEvict.Delete(key)
}

// implements base.InMemoryCache
func (c *TwoQueueCache[K, V]) Purge() {
	c.recent.Purge()
	c.frequent.Purge()
	c.recentEvict.Purge()
}

// implements base.InMemoryCache
func (c *TwoQueueCache[K, V]) SetMany(items map[K]V) {
	for k, v := range items {
		c.Set(k, v)
	}
}

// implements base.InMemoryCache
func (c *TwoQueueCache[K, V]) HasMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Has(k)
	}
	return m
}

// implements base.InMemoryCache
func (c *TwoQueueCache[K, V]) GetMany(keys []K) (map[K]V, []K) {
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
func (c *TwoQueueCache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
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
func (c *TwoQueueCache[K, V]) DeleteMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Delete(k)
	}
	return m
}

// implements base.InMemoryCache
func (c *TwoQueueCache[K, V]) Capacity() int {
	return c.capacity + c.recentCapacity
}

// implements base.InMemoryCache
func (c *TwoQueueCache[K, V]) Algorithm() string {
	return "2q"
}

// implements base.InMemoryCache
func (c *TwoQueueCache[K, V]) Len() int {
	return c.recent.Len() + c.frequent.Len()
}

// ensureSpace is used to ensure we have space in the cache
func (c *TwoQueueCache[K, V]) ensureSpace(recentEvict bool) {
	// If we have space, nothing to do
	recentLen := c.recent.Len()
	freqLen := c.frequent.Len()
	if recentLen+freqLen < c.capacity {
		return
	}

	// If the recent buffer is larger than
	// the target, evict from there
	if recentLen > 0 && (recentLen > c.recentCapacity || (recentLen == c.recentCapacity && !recentEvict)) {
		k, _, _ := c.recent.DeleteOldest()
		c.recentEvict.Set(k, struct{}{})
		return
	}

	// Remove from the frequent list otherwise
	c.frequent.DeleteOldest()
}
