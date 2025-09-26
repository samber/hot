package lru

import (
	"github.com/samber/hot/internal/container/list"

	"github.com/DmitriyVTitov/size"
	"github.com/samber/hot/internal"
	"github.com/samber/hot/pkg/base"
)

// entry represents a key-value pair stored in the LRU cache.
// Each entry is stored as a list element to maintain the access order.
type entry[K comparable, V any] struct {
	key   K // The cache key
	value V // The cached value
}

// NewLRUCache creates a new LRU cache with the specified capacity.
// The cache will evict the least recently used items when it reaches capacity.
func NewLRUCache[K comparable, V any](capacity int) *LRUCache[K, V] {
	return NewLRUCacheWithEvictionCallback[K, V](capacity, nil)
}

// NewLRUCacheWithEvictionCallback creates a new LRU cache with the specified capacity and eviction callback.
// The callback will be called whenever an item is evicted from the cache.
func NewLRUCacheWithEvictionCallback[K comparable, V any](capacity int, onEviction base.EvictionCallback[K, V]) *LRUCache[K, V] {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}

	return &LRUCache[K, V]{
		capacity: capacity,
		ll:       list.New[*entry[K, V]](),
		cache:    make(map[K]*list.Element[*entry[K, V]]),

		onEviction: onEviction,
	}
}

// LRUCache is a Least Recently Used cache implementation.
// It is not safe for concurrent access and should be wrapped with a thread-safe layer if needed.
type LRUCache[K comparable, V any] struct {
	noCopy internal.NoCopy // Prevents accidental copying of the cache

	capacity int                               // Maximum number of items the cache can hold (0 = unlimited)
	ll       *list.List[*entry[K, V]]          // Doubly-linked list maintaining access order (most recent at front)
	cache    map[K]*list.Element[*entry[K, V]] // Map for O(1) key lookups to list elements

	onEviction base.EvictionCallback[K, V] // Optional callback called when items are evicted
}

// Ensure LRUCache implements InMemoryCache interface
var _ base.InMemoryCache[string, int] = (*LRUCache[string, int])(nil)

// Set stores a key-value pair in the cache.
// If the key already exists, its value is updated and it becomes the most recently used item.
// If the cache is at capacity, the least recently used item is evicted.
// Time complexity: O(1) average case, O(n) worst case when eviction occurs.
func (c *LRUCache[K, V]) Set(key K, value V) {
	if e, ok := c.cache[key]; ok {
		// Key exists: move to front (most recently used) and update value
		c.ll.MoveToFront(e)
		e.Value.value = value
		return
	}

	// Key doesn't exist: create new entry at front of list
	e := c.ll.PushFront(&entry[K, V]{key, value})
	c.cache[key] = e

	// Check if we need to evict the least recently used item
	if c.capacity != 0 && c.ll.Len() > c.capacity {
		k, v, ok := c.DeleteOldest()
		if ok && c.onEviction != nil {
			c.onEviction(base.EvictionReasonCapacity, k, v)
		}
	}
}

// Has checks if a key exists in the cache.
func (c *LRUCache[K, V]) Has(key K) bool {
	_, hit := c.cache[key]
	return hit
}

// Get retrieves a value from the cache and makes it the most recently used item.
// Returns the value and a boolean indicating if the key was found.
func (c *LRUCache[K, V]) Get(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		c.ll.MoveToFront(e)
		return e.Value.value, true
	}
	return value, false
}

// Peek retrieves a value from the cache without updating the access order.
// Returns the value and a boolean indicating if the key was found.
func (c *LRUCache[K, V]) Peek(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		return e.Value.value, true
	}
	return value, false
}

// Keys returns all keys currently in the cache.
func (c *LRUCache[K, V]) Keys() []K {
	all := make([]K, 0, c.ll.Len())
	for k := range c.cache {
		all = append(all, k)
	}
	return all
}

// Values returns all values currently in the cache.
func (c *LRUCache[K, V]) Values() []V {
	all := make([]V, 0, c.ll.Len())
	for _, v := range c.cache {
		all = append(all, v.Value.value)
	}
	return all
}

// All returns all key-value pairs in the cache.
func (c *LRUCache[K, V]) All() map[K]V {
	all := make(map[K]V)
	for k, v := range c.cache {
		all[k] = v.Value.value
	}
	return all
}

// Range iterates over all key-value pairs in the cache.
// The iteration stops if the function returns false.
func (c *LRUCache[K, V]) Range(f func(K, V) bool) {
	all := c.All()
	for k, v := range all {
		if !f(k, v) {
			break
		}
	}
}

// Delete removes a key from the cache.
// Returns true if the key was found and removed, false otherwise.
// Time complexity: O(1) average case.
func (c *LRUCache[K, V]) Delete(key K) bool {
	if e, hit := c.cache[key]; hit {
		c.deleteElement(e)
		return true
	}
	return false
}

// SetMany stores multiple key-value pairs in the cache.
// This is more efficient than calling Set multiple times.
func (c *LRUCache[K, V]) SetMany(items map[K]V) {
	for k, v := range items {
		c.Set(k, v)
	}
}

// HasMany checks if multiple keys exist in the cache.
// Returns a map where keys are the input keys and values indicate existence.
func (c *LRUCache[K, V]) HasMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Has(k)
	}
	return m
}

// GetMany retrieves multiple values from the cache.
// Returns a map of found key-value pairs and a slice of missing keys.
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

// PeekMany retrieves multiple values from the cache without updating access order.
// Returns a map of found key-value pairs and a slice of missing keys.
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

// DeleteMany removes multiple keys from the cache.
// Returns a map where keys are the input keys and values indicate if the key was found and removed.
func (c *LRUCache[K, V]) DeleteMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Delete(k)
	}
	return m
}

// Purge removes all keys and values from the cache.
// This operation resets the cache to its initial state.
// Time complexity: O(1) - just reallocates the data structures.
func (c *LRUCache[K, V]) Purge() {
	c.ll = list.New[*entry[K, V]]()
	c.cache = make(map[K]*list.Element[*entry[K, V]])
}

// Capacity returns the maximum number of items the cache can hold.
// Returns 0 if the cache has unlimited capacity.
func (c *LRUCache[K, V]) Capacity() int {
	return c.capacity
}

// Algorithm returns the name of the eviction algorithm used by the cache.
// This is used for debugging and monitoring purposes.
func (c *LRUCache[K, V]) Algorithm() string {
	return "lru"
}

// Len returns the current number of items in the cache.
// Time complexity: O(1) - the list maintains its length.
func (c *LRUCache[K, V]) Len() int {
	return c.ll.Len()
}

// SizeBytes returns the total size of all cache entries in bytes.
// For generic caches, this returns 0 as the size cannot be determined without type information.
// Specialized implementations should override this method.
func (c *LRUCache[K, V]) SizeBytes() int64 {
	return int64(size.Of(c.cache))
}

// DeleteOldest removes and returns the least recently used item from the cache.
// Returns the key, value, and a boolean indicating if an item was removed.
// This method is used internally for eviction when the cache reaches capacity.
// Time complexity: O(1) - removes from the back of the list.
func (c *LRUCache[K, V]) DeleteOldest() (k K, v V, ok bool) {
	e := c.ll.Back()
	if e != nil {
		c.deleteElement(e)
		kv := e.Value
		return kv.key, kv.value, true
	}

	return k, v, false
}

// deleteElement removes an element from both the list and the map.
// This is an internal helper method that ensures consistency between
// the list and map data structures.
// Time complexity: O(1) average case.
func (c *LRUCache[K, V]) deleteElement(e *list.Element[*entry[K, V]]) {
	c.ll.Remove(e)
	kv := e.Value
	delete(c.cache, kv.key)
}
