package fifo

import (
	"container/list"

	"github.com/DmitriyVTitov/size"
	"github.com/samber/hot/internal"
	"github.com/samber/hot/pkg/base"
)

// entry represents a key-value pair stored in the FIFO cache.
// Each entry is stored as a list element to maintain the insertion order.
type entry[K comparable, V any] struct {
	key   K // The cache key
	value V // The cached value
}

// NewFIFOCache creates a new FIFO cache with the specified capacity.
// The cache will evict the first inserted items when it reaches capacity.
func NewFIFOCache[K comparable, V any](capacity int) *FIFOCache[K, V] {
	return NewFIFOCacheWithEvictionCallback[K, V](capacity, nil)
}

// NewFIFOCacheWithEvictionCallback creates a new FIFO cache with the specified capacity and eviction callback.
// The callback will be called whenever an item is evicted from the cache.
func NewFIFOCacheWithEvictionCallback[K comparable, V any](capacity int, onEviction base.EvictionCallback[K, V]) *FIFOCache[K, V] {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}

	return &FIFOCache[K, V]{
		capacity: capacity,
		ll:       list.New(),
		cache:    make(map[K]*list.Element),

		onEviction: onEviction,
	}
}

// FIFOCache is a First In, First Out cache implementation.
// It is not safe for concurrent access and should be wrapped with a thread-safe layer if needed.
type FIFOCache[K comparable, V any] struct {
	noCopy internal.NoCopy // Prevents accidental copying of the cache

	capacity int                 // Maximum number of items the cache can hold
	ll       *list.List          // Doubly-linked list maintaining insertion order (oldest at front)
	cache    map[K]*list.Element // Map for O(1) key lookups to list elements

	onEviction base.EvictionCallback[K, V] // Optional callback called when items are evicted
}

// Ensure FIFOCache implements InMemoryCache interface
var _ base.InMemoryCache[string, int] = (*FIFOCache[string, int])(nil)

// Set stores a key-value pair in the cache.
// If the key already exists, its value is updated but its position remains unchanged.
// If the cache is at capacity, the first inserted item is evicted.
// Time complexity: O(1) average case, O(n) worst case when eviction occurs.
func (c *FIFOCache[K, V]) Set(key K, value V) {
	if e, ok := c.cache[key]; ok {
		// Key exists: update value but keep position (FIFO order is preserved)
		e.Value.(*entry[K, V]).value = value
		return
	}

	// Key doesn't exist: create new entry at back of list (newest)
	e := c.ll.PushBack(&entry[K, V]{key, value})
	c.cache[key] = e

	// Check if we need to evict the first inserted item
	if c.capacity != 0 && c.ll.Len() > c.capacity {
		k, v, ok := c.DeleteOldest()
		if ok && c.onEviction != nil {
			c.onEviction(base.EvictionReasonCapacity, k, v)
		}
	}
}

// Has checks if a key exists in the cache.
func (c *FIFOCache[K, V]) Has(key K) bool {
	_, hit := c.cache[key]
	return hit
}

// Get retrieves a value from the cache without changing its position.
// Returns the value and a boolean indicating if the key was found.
func (c *FIFOCache[K, V]) Get(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		return e.Value.(*entry[K, V]).value, true
	}
	return value, false
}

// Peek retrieves a value from the cache without updating the access order.
// Returns the value and a boolean indicating if the key was found.
func (c *FIFOCache[K, V]) Peek(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		return e.Value.(*entry[K, V]).value, true
	}
	return value, false
}

// Keys returns all keys currently in the cache.
func (c *FIFOCache[K, V]) Keys() []K {
	all := make([]K, 0, c.ll.Len())
	for k := range c.cache {
		all = append(all, k)
	}
	return all
}

// Values returns all values currently in the cache.
func (c *FIFOCache[K, V]) Values() []V {
	all := make([]V, 0, c.ll.Len())
	for _, v := range c.cache {
		all = append(all, v.Value.(*entry[K, V]).value)
	}
	return all
}

// Range iterates over all key-value pairs in the cache.
// The iteration stops if the function returns false.
func (c *FIFOCache[K, V]) Range(f func(K, V) bool) {
	for k, v := range c.cache {
		if !f(k, v.Value.(*entry[K, V]).value) {
			break
		}
	}
}

// Delete removes a key from the cache.
// Returns true if the key was found and removed, false otherwise.
// Time complexity: O(1) average case.
func (c *FIFOCache[K, V]) Delete(key K) bool {
	if e, hit := c.cache[key]; hit {
		c.deleteElement(e)
		return true
	}
	return false
}

// SetMany stores multiple key-value pairs in the cache.
// This is more efficient than calling Set multiple times.
func (c *FIFOCache[K, V]) SetMany(items map[K]V) {
	for k, v := range items {
		c.Set(k, v)
	}
}

// HasMany checks if multiple keys exist in the cache.
// Returns a map where keys are the input keys and values indicate existence.
func (c *FIFOCache[K, V]) HasMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Has(k)
	}
	return m
}

// GetMany retrieves multiple values from the cache.
// Returns a map of found key-value pairs and a slice of missing keys.
func (c *FIFOCache[K, V]) GetMany(keys []K) (map[K]V, []K) {
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
func (c *FIFOCache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
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
func (c *FIFOCache[K, V]) DeleteMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Delete(k)
	}
	return m
}

// Purge removes all keys and values from the cache.
func (c *FIFOCache[K, V]) Purge() {
	c.ll.Init()
	for k := range c.cache {
		delete(c.cache, k)
	}
}

// Capacity returns the maximum number of items the cache can hold.
func (c *FIFOCache[K, V]) Capacity() int {
	return c.capacity
}

// Algorithm returns the name of the eviction algorithm used by the cache.
func (c *FIFOCache[K, V]) Algorithm() string {
	return "fifo"
}

// Len returns the current number of items in the cache.
func (c *FIFOCache[K, V]) Len() int {
	return c.ll.Len()
}

// SizeBytes returns the total size of all cache entries in bytes.
func (c *FIFOCache[K, V]) SizeBytes() int64 {
	var total int64
	for _, v := range c.cache {
		total += int64(size.Of(v.Value.(*entry[K, V]).value))
	}
	return total
}

// DeleteOldest removes and returns the oldest item in the cache.
// Returns the key, value, and a boolean indicating if an item was found.
func (c *FIFOCache[K, V]) DeleteOldest() (k K, v V, ok bool) {
	if c.ll.Len() == 0 {
		return k, v, false
	}

	e := c.ll.Front()
	c.deleteElement(e)
	return e.Value.(*entry[K, V]).key, e.Value.(*entry[K, V]).value, true
}

// deleteElement removes an element from both the list and the map.
func (c *FIFOCache[K, V]) deleteElement(e *list.Element) {
	c.ll.Remove(e)
	delete(c.cache, e.Value.(*entry[K, V]).key)
}
