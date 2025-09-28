package tinylfu

import (
	"github.com/samber/hot/internal/container/list"
	"github.com/samber/hot/internal/sketch"

	"github.com/DmitriyVTitov/size"
	"github.com/samber/hot/internal"
	"github.com/samber/hot/pkg/base"
)

// entry represents a key-value pair stored in the TinyLFU cache.
// Each entry is stored as a list element to maintain the access order.
type entry[K comparable, V any] struct {
	key   K   // The cache key
	value V   // The cached value
	freq  int // The frequency of the key
}

// NewTinyLFUCache creates a new TinyLFU cache with the specified capacity.
// The cache will evict the least recently used items when it reaches capacity.
func NewTinyLFUCache[K comparable, V any](capacity int) *TinyLFUCache[K, V] {
	return NewTinyLFUCacheWithEvictionCallback[K, V](capacity, nil)
}

// NewTinyLFUCacheWithEvictionCallback creates a new TinyLFU cache with the specified capacity and eviction callback.
// The callback will be called whenever an item is evicted from the cache.
func NewTinyLFUCacheWithEvictionCallback[K comparable, V any](capacity int, onEviction base.EvictionCallback[K, V]) *TinyLFUCache[K, V] {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}

	admissionCapacity := max(capacity/100, 1)
	mainCapacity := max(capacity-admissionCapacity, 1)

	cmsDepth := 4
	if capacity < 10_000 {
		cmsDepth = 3
	}

	return &TinyLFUCache[K, V]{
		sketch: sketch.NewCountMinSketch[K](capacity, cmsDepth),

		mainCapacity:      mainCapacity,
		admissionCapacity: admissionCapacity,

		admissionLl:    list.New[*entry[K, V]](),
		mainLl:         list.New[*entry[K, V]](),
		mainCache:      make(map[K]*list.Element[*entry[K, V]]),
		admissionCache: make(map[K]*list.Element[*entry[K, V]]),

		onEviction: onEviction,
	}
}

// TinyLFUCache is a Least Recently Used cache implementation.
// It is not safe for concurrent access and should be wrapped with a thread-safe layer if needed.
type TinyLFUCache[K comparable, V any] struct {
	noCopy internal.NoCopy // Prevents accidental copying of the cache

	sketch *sketch.CountMinSketch[K]

	mainCapacity      int // Maximum number of items the cache can hold
	admissionCapacity int // Maximum number of items the cache can hold for admission

	mainLl         *list.List[*entry[K, V]]          // Doubly-linked list maintaining access order (most recent at front)
	admissionLl    *list.List[*entry[K, V]]          // Doubly-linked list maintaining admission order (least recently admitted at front)
	mainCache      map[K]*list.Element[*entry[K, V]] // Map for O(1) key lookups to list elements
	admissionCache map[K]*list.Element[*entry[K, V]] // Map for O(1) key lookups to list elements

	onEviction base.EvictionCallback[K, V] // Optional callback called when items are evicted
}

// Ensure TinyLFUCache implements InMemoryCache interface.
var _ base.InMemoryCache[string, int] = (*TinyLFUCache[string, int])(nil)

// Set stores a key-value pair in the cache.
// If the key already exists, its value is updated and it becomes the most recently used item.
// If the cache is at capacity, the least recently used item is evicted.
// Time complexity: O(1) average case, O(n) worst case when eviction occurs.
func (c *TinyLFUCache[K, V]) Set(key K, value V) {
	// First, update the sketch with the key access
	c.sketch.Inc(key)

	// Check if key exists in main cache
	if e, ok := c.mainCache[key]; ok {
		// Key exists in main cache: move to front and update value
		c.mainLl.MoveToFront(e)
		e.Value.freq++
		e.Value.value = value
		return
	}

	// Check if key exists in admission window
	if e, ok := c.admissionCache[key]; ok {
		// Key exists in admission window: check if it should be promoted
		c.sketch.Inc(key) // Increment again for promotion check
		if c.shouldPromote(key) {
			// Promote to main cache
			c.promoteFromAdmission(e, value)
		} else {
			// Update in admission window
			e.Value.value = value
			e.Value.freq++
			c.admissionLl.MoveToFront(e)
		}
		return
	}

	// Key doesn't exist anywhere: add to admission window
	newEntry := c.admissionLl.PushFront(&entry[K, V]{key, value, 1})
	c.admissionCache[key] = newEntry

	// Check if admission window is full and needs eviction
	if c.admissionLl.Len() > c.admissionCapacity {
		e := c.admissionLl.Back()
		c.admissionLl.Remove(e)
		delete(c.admissionCache, e.Value.key)

		// Call eviction callback if provided
		if c.onEviction != nil {
			c.onEviction(base.EvictionReasonCapacity, e.Value.key, e.Value.value)
		}
	}
}

// Has checks if a key exists in the cache.
func (c *TinyLFUCache[K, V]) Has(key K) bool {
	// Check main cache first
	if _, hit := c.mainCache[key]; hit {
		return true
	}

	// Check admission window
	_, hit := c.admissionCache[key]
	return hit
}

// Get retrieves a value from the cache and makes it the most recently used item.
// Returns the value and a boolean indicating if the key was found.
func (c *TinyLFUCache[K, V]) Get(key K) (value V, ok bool) {
	// Update the sketch with the key access
	c.sketch.Inc(key)

	// Check if key exists in main cache
	if e, hit := c.mainCache[key]; hit {
		// Key exists in main cache: move to front and update frequency
		c.mainLl.MoveToFront(e)
		e.Value.freq++
		return e.Value.value, true
	}

	// Check if key exists in admission window
	if e, hit := c.admissionCache[key]; hit {
		// Key exists in admission window: check if it should be promoted
		if c.shouldPromote(key) {
			// Promote to main cache
			c.promoteFromAdmission(e, e.Value.value)
			return e.Value.value, true
		}

		// Update frequency and move to front in admission window
		e.Value.freq++
		c.admissionLl.MoveToFront(e)
		return e.Value.value, true
	}

	return value, false
}

// Peek retrieves a value from the cache without updating the access order.
// Returns the value and a boolean indicating if the key was found.
func (c *TinyLFUCache[K, V]) Peek(key K) (value V, ok bool) {
	// Check main cache first
	if e, hit := c.mainCache[key]; hit {
		return e.Value.value, true
	}

	// Check admission window
	if e, hit := c.admissionCache[key]; hit {
		return e.Value.value, true
	}

	return value, false
}

// Keys returns all keys currently in the cache.
func (c *TinyLFUCache[K, V]) Keys() []K {
	all := make([]K, 0, c.mainLl.Len()+c.admissionLl.Len())

	// Add keys from main cache
	for k := range c.mainCache {
		all = append(all, k)
	}

	// Add keys from admission window
	for k := range c.admissionCache {
		all = append(all, k)
	}

	return all
}

// Values returns all values currently in the cache.
func (c *TinyLFUCache[K, V]) Values() []V {
	all := make([]V, 0, c.mainLl.Len()+c.admissionLl.Len())

	// Add values from main cache
	for _, v := range c.mainCache {
		all = append(all, v.Value.value)
	}

	// Add values from admission window
	for _, v := range c.admissionCache {
		all = append(all, v.Value.value)
	}

	return all
}

// All returns all key-value pairs in the cache.
func (c *TinyLFUCache[K, V]) All() map[K]V {
	all := make(map[K]V)

	// Add items from main cache
	for k, v := range c.mainCache {
		all[k] = v.Value.value
	}

	// Add items from admission window
	for k, v := range c.admissionCache {
		all[k] = v.Value.value
	}

	return all
}

// Range iterates over all key-value pairs in the cache.
// The iteration stops if the function returns false.
func (c *TinyLFUCache[K, V]) Range(f func(K, V) bool) {
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
func (c *TinyLFUCache[K, V]) Delete(key K) bool {
	// Check main cache first
	if e, hit := c.mainCache[key]; hit {
		delete(c.mainCache, key)
		c.mainLl.Remove(e)
		return true
	}

	// Check admission window
	if e, hit := c.admissionCache[key]; hit {
		delete(c.admissionCache, key)
		c.admissionLl.Remove(e)
		return true
	}

	return false
}

// SetMany stores multiple key-value pairs in the cache.
// This is more efficient than calling Set multiple times.
func (c *TinyLFUCache[K, V]) SetMany(items map[K]V) {
	for k, v := range items {
		c.Set(k, v)
	}
}

// HasMany checks if multiple keys exist in the cache.
// Returns a map where keys are the input keys and values indicate existence.
func (c *TinyLFUCache[K, V]) HasMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Has(k)
	}
	return m
}

// GetMany retrieves multiple values from the cache.
// Returns a map of found key-value pairs and a slice of missing keys.
func (c *TinyLFUCache[K, V]) GetMany(keys []K) (map[K]V, []K) {
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
func (c *TinyLFUCache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
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
func (c *TinyLFUCache[K, V]) DeleteMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Delete(k)
	}
	return m
}

// Purge removes all keys and values from the cache.
// This operation resets the cache to its initial state.
// Time complexity: O(1) - just reallocates the data structures.
func (c *TinyLFUCache[K, V]) Purge() {
	c.mainLl = list.New[*entry[K, V]]()
	c.admissionLl = list.New[*entry[K, V]]()
	c.mainCache = make(map[K]*list.Element[*entry[K, V]])
	c.admissionCache = make(map[K]*list.Element[*entry[K, V]])
	c.sketch.Reset()
}

// Capacity returns the maximum number of items the cache can hold.
// Returns 0 if the cache has unlimited capacity.
func (c *TinyLFUCache[K, V]) Capacity() int {
	return c.mainCapacity + c.admissionCapacity
}

// Algorithm returns the name of the eviction algorithm used by the cache.
// This is used for debugging and monitoring purposes.
func (c *TinyLFUCache[K, V]) Algorithm() string {
	return "tinylfu"
}

// Len returns the current number of items in the cache.
// Time complexity: O(1) - the list maintains its length.
func (c *TinyLFUCache[K, V]) Len() int {
	return c.mainLl.Len() + c.admissionLl.Len()
}

// SizeBytes returns the total size of all cache entries in bytes.
// For generic caches, this returns 0 as the size cannot be determined without type information.
// Specialized implementations should override this method.
func (c *TinyLFUCache[K, V]) SizeBytes() int64 {
	return int64(size.Of(c.mainCache)) + int64(size.Of(c.admissionCache))
}

// shouldPromote determines if a key should be promoted from admission window to main cache.
// This is based on the TinyLFU algorithm: promote if the key's frequency is higher than
// the frequency of the least frequently used item in the main cache.
func (c *TinyLFUCache[K, V]) shouldPromote(key K) bool {
	if c.mainLl.Len() == 0 {
		return true // Always promote if main cache is empty
	}

	keyFreq := c.sketch.Estimate(key)

	// Find the least frequently used item in main cache
	minFreq := int(^uint(0) >> 1) // Max int
	for e := c.mainLl.Front(); e != nil; e = e.Next() {
		itemFreq := c.sketch.Estimate(e.Value.key)
		if itemFreq < minFreq {
			minFreq = itemFreq
		}
	}

	return keyFreq > minFreq
}

// promoteFromAdmission promotes an item from admission window to main cache.
func (c *TinyLFUCache[K, V]) promoteFromAdmission(e *list.Element[*entry[K, V]], value V) {
	// Get the entry data before removing from admission window
	entry := e.Value
	entry.value = value
	entry.freq++

	// Remove from admission window
	c.admissionLl.Remove(e)
	delete(c.admissionCache, entry.key)

	// Create new element in main cache
	newElement := c.mainLl.PushFront(entry)
	c.mainCache[entry.key] = newElement

	// Check if main cache needs eviction
	if c.mainCapacity != 0 && c.mainLl.Len() > c.mainCapacity {
		e := c.mainLl.Back()
		if e != nil {
			kv := e.Value
			delete(c.mainCache, kv.key)
			c.mainLl.Remove(e)
			if c.onEviction != nil {
				c.onEviction(base.EvictionReasonCapacity, kv.key, kv.value)
			}
		}
	}
}
