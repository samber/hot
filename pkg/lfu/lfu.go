package lfu

import (
	"container/list"

	"github.com/DmitriyVTitov/size"
	"github.com/samber/hot/internal"
	"github.com/samber/hot/pkg/base"
)

const (
	// DefaultEvictionSize is the number of elements to evict when the cache is full.
	// This provides a balance between memory efficiency and performance.
	DefaultEvictionSize = 1
)

// entry represents a key-value pair stored in the LFU cache.
// Each entry is stored as a list element to maintain frequency order.
type entry[K comparable, V any] struct {
	key   K // The cache key
	value V // The cached value
}

// NewLFUCache creates a new LFU cache with the specified capacity.
// Uses the default eviction size of 1 element when the cache is full.
func NewLFUCache[K comparable, V any](capacity int) *LFUCache[K, V] {
	return NewLFUCacheWithEvictionSizeAndCallback[K, V](capacity, DefaultEvictionSize, nil)
}

// NewLFUCacheWithEvictionCallback creates a new LFU cache with the specified capacity and eviction callback.
// The callback will be called whenever an item is evicted from the cache.
// Uses the default eviction size of 1 element when the cache is full.
func NewLFUCacheWithEvictionCallback[K comparable, V any](capacity int, onEviction base.EvictionCallback[K, V]) *LFUCache[K, V] {
	return NewLFUCacheWithEvictionSizeAndCallback(capacity, DefaultEvictionSize, onEviction)
}

// NewLFUCacheWithEvictionSize creates a new LFU cache with the specified capacity and eviction size.
// The eviction size determines how many elements are removed when the cache reaches capacity.
func NewLFUCacheWithEvictionSize[K comparable, V any](capacity int, evictionSize int) *LFUCache[K, V] {
	return NewLFUCacheWithEvictionSizeAndCallback[K, V](capacity, evictionSize, nil)
}

// NewLFUCacheWithEvictionSizeAndCallback creates a new LFU cache with the specified capacity,
// eviction size, and eviction callback. This is the main constructor for LFU caches.
// The cache will evict the least frequently used items when it reaches capacity.
func NewLFUCacheWithEvictionSizeAndCallback[K comparable, V any](capacity int, evictionSize int, onEviction base.EvictionCallback[K, V]) *LFUCache[K, V] {
	if capacity <= 1 {
		panic("capacity must be greater than 1")
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

// LFUCache is a Least Frequently Used cache implementation.
// It is not safe for concurrent access and should be wrapped with a thread-safe layer if needed.
// Items are ordered by their access frequency, with least frequently used items at the front.
type LFUCache[K comparable, V any] struct {
	noCopy internal.NoCopy // Prevents accidental copying of the cache

	capacity     int // Maximum number of items the cache can hold
	evictionSize int // Number of items to evict when cache is full
	// @TODO: build a custom list.List implementation
	ll    *list.List          // Doubly-linked list maintaining frequency order (least frequent at front)
	cache map[K]*list.Element // Map for O(1) key lookups to list elements

	onEviction base.EvictionCallback[K, V] // Optional callback called when items are evicted
}

// Ensure LFUCache implements InMemoryCache interface
var _ base.InMemoryCache[string, int] = (*LFUCache[string, int])(nil)

// Set stores a key-value pair in the cache.
// If the key already exists, its value is updated and its frequency is incremented.
// If the cache is at capacity, the least frequently used items are evicted.
// Time complexity: O(1) average case, O(evictionSize) worst case when eviction occurs.
func (c *LFUCache[K, V]) Set(key K, value V) {
	if e, ok := c.cache[key]; ok {
		// Key exists: increment frequency by moving after next element
		if e.Next() != nil {
			c.ll.MoveAfter(e, e.Next())
		}
		e.Value.(*entry[K, V]).value = value
		return
	}

	// Evict least frequently used items if cache is full
	if c.ll.Len() >= c.capacity {
		for i := 0; i < c.evictionSize; i++ {
			k, v, ok := c.DeleteLeastFrequent()
			if ok && c.onEviction != nil {
				c.onEviction(base.EvictionReasonCapacity, k, v)
			}
		}
	}

	// Add new entry at front (least frequent position)
	e := c.ll.PushFront(&entry[K, V]{key, value})
	c.cache[key] = e
}

// Has checks if a key exists in the cache.
// This operation does not affect the frequency count of the key.
func (c *LFUCache[K, V]) Has(key K) bool {
	_, hit := c.cache[key]
	return hit
}

// Get retrieves a value from the cache and increments its frequency count.
// Returns the value and a boolean indicating if the key was found.
// Time complexity: O(1) average case.
func (c *LFUCache[K, V]) Get(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		// Increment frequency by moving after next element
		if e.Next() != nil {
			c.ll.MoveAfter(e, e.Next())
		}
		return e.Value.(*entry[K, V]).value, true
	}
	return value, false
}

// Peek retrieves a value from the cache without updating the frequency count.
// Returns the value and a boolean indicating if the key was found.
// This operation does not affect the eviction order.
func (c *LFUCache[K, V]) Peek(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		return e.Value.(*entry[K, V]).value, true
	}
	return value, false
}

// Keys returns all keys currently in the cache.
// The order of keys in the returned slice is not guaranteed to match frequency order.
func (c *LFUCache[K, V]) Keys() []K {
	all := make([]K, 0, c.ll.Len())
	for k := range c.cache {
		all = append(all, k)
	}
	return all
}

// Values returns all values currently in the cache.
// The order of values in the returned slice is not guaranteed to match frequency order.
func (c *LFUCache[K, V]) Values() []V {
	all := make([]V, 0, c.ll.Len())
	for _, v := range c.cache {
		all = append(all, v.Value.(*entry[K, V]).value)
	}
	return all
}

// Range iterates over all key-value pairs in the cache.
// The iteration stops if the function returns false.
// The iteration order is not guaranteed to match frequency order.
func (c *LFUCache[K, V]) Range(f func(K, V) bool) {
	for k, v := range c.cache {
		if !f(k, v.Value.(*entry[K, V]).value) {
			break
		}
	}
}

// Delete removes a key from the cache.
// Returns true if the key was found and removed, false otherwise.
// Time complexity: O(1) average case.
func (c *LFUCache[K, V]) Delete(key K) bool {
	if e, hit := c.cache[key]; hit {
		c.deleteElement(e)
		return true
	}
	return false
}

// Purge removes all keys and values from the cache.
// This operation resets the cache to its initial state.
// Time complexity: O(1) - just reallocates the data structures.
func (c *LFUCache[K, V]) Purge() {
	c.ll = list.New()
	c.cache = make(map[K]*list.Element)
}

// SetMany stores multiple key-value pairs in the cache.
// This is more efficient than calling Set multiple times.
// Each key-value pair is processed individually, so frequency counts are updated correctly.
func (c *LFUCache[K, V]) SetMany(items map[K]V) {
	for k, v := range items {
		c.Set(k, v)
	}
}

// HasMany checks if multiple keys exist in the cache.
// Returns a map where keys are the input keys and values indicate existence.
// This operation does not affect the frequency count of any keys.
func (c *LFUCache[K, V]) HasMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Has(k)
	}
	return m
}

// GetMany retrieves multiple values from the cache.
// Returns a map of found key-value pairs and a slice of missing keys.
// Each found key has its frequency count incremented.
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

// PeekMany retrieves multiple values from the cache without updating frequency counts.
// Returns a map of found key-value pairs and a slice of missing keys.
// This operation does not affect the eviction order.
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

// DeleteMany removes multiple keys from the cache.
// Returns a map where keys are the input keys and values indicate if the key was found and removed.
func (c *LFUCache[K, V]) DeleteMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Delete(k)
	}
	return m
}

// Capacity returns the maximum number of items the cache can hold.
func (c *LFUCache[K, V]) Capacity() int {
	return c.capacity
}

// Algorithm returns the name of the eviction algorithm used by the cache.
// This is used for debugging and monitoring purposes.
func (c *LFUCache[K, V]) Algorithm() string {
	return "lfu"
}

// Len returns the current number of items in the cache.
// Time complexity: O(1) - the list maintains its length.
func (c *LFUCache[K, V]) Len() int {
	return c.ll.Len()
}

// SizeBytes returns the total size of all cache entries in bytes.
// For generic caches, this returns 0 as the size cannot be determined without type information.
// Specialized implementations should override this method.
func (c *LFUCache[K, V]) SizeBytes() int64 {
	return int64(size.Of(c.cache))
}

// DeleteLeastFrequent removes and returns the least frequently used item from the cache.
// Returns the key, value, and a boolean indicating if an item was removed.
// This method is used internally for eviction when the cache reaches capacity.
// Time complexity: O(1) - removes from the front of the list.
func (c *LFUCache[K, V]) DeleteLeastFrequent() (k K, v V, ok bool) {
	e := c.ll.Front()
	if e != nil {
		c.deleteElement(e)
		kv := e.Value.(*entry[K, V])
		return kv.key, kv.value, true
	}

	return k, v, false
}

// deleteElement removes an element from both the list and the map.
// This is an internal helper method that ensures consistency between
// the list and map data structures.
// Time complexity: O(1) average case.
func (c *LFUCache[K, V]) deleteElement(e *list.Element) {
	c.ll.Remove(e)
	kv := e.Value.(*entry[K, V])
	delete(c.cache, kv.key)
}
