package lfu

import (
	"github.com/samber/hot/internal/container/list"

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
// Each entry tracks its own access frequency for O(1) LFU eviction.
type entry[K comparable, V any] struct {
	key   K   // The cache key
	value V   // The cached value
	freq  int // Access frequency counter
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
		minFreq:      0,
		cache:        make(map[K]*list.Element[*entry[K, V]]),
		freqMap:      make(map[int]*list.List[*entry[K, V]]),

		onEviction: onEviction,
	}
}

// LFUCache is a Least Frequently Used cache implementation using O(1) frequency tracking.
// Items are evicted by lowest access frequency, with LRU tiebreaking within the same frequency.
// It is not safe for concurrent access and should be wrapped with a thread-safe layer if needed.
type LFUCache[K comparable, V any] struct { //nolint:revive
	noCopy internal.NoCopy // Prevents accidental copying of the cache

	capacity     int // Maximum number of items the cache can hold
	evictionSize int // Number of items to evict when cache is full
	minFreq      int // Current minimum frequency across all entries

	cache   map[K]*list.Element[*entry[K, V]] // Map for O(1) key lookups to list elements
	freqMap map[int]*list.List[*entry[K, V]]  // Map from frequency to doubly-linked list of entries (front=MRU, back=LRU)

	onEviction base.EvictionCallback[K, V] // Optional callback called when items are evicted
}

// Ensure LFUCache implements InMemoryCache interface.
var _ base.InMemoryCache[string, int] = (*LFUCache[string, int])(nil)

// Set stores a key-value pair in the cache.
// If the key already exists, its value is updated and its frequency is incremented.
// If the cache is at capacity, the least frequently used items are evicted.
// Time complexity: O(1) average case, O(evictionSize) worst case when eviction occurs.
func (c *LFUCache[K, V]) Set(key K, value V) {
	if e, ok := c.cache[key]; ok {
		// Key exists: update value and increment frequency
		e.Value.value = value
		c.incrementFreq(e)
		return
	}

	// Evict least frequently used items if cache is full
	if len(c.cache) >= c.capacity {
		for i := 0; i < c.evictionSize; i++ {
			k, v, ok := c.DeleteLeastFrequent()
			if ok && c.onEviction != nil {
				c.onEviction(base.EvictionReasonCapacity, k, v)
			}
		}
	}

	// Add new entry with frequency 0
	ent := &entry[K, V]{key: key, value: value, freq: 0}
	freqList := c.getOrCreateFreqList(0)
	e := freqList.PushFront(ent)
	c.cache[key] = e
	c.minFreq = 0
}

// Has checks if a key exists in the cache.
// This operation does not affect the frequency count of the key.
func (c *LFUCache[K, V]) Has(key K) bool {
	_, hit := c.cache[key]
	return hit
}

// Get retrieves a value from the cache and increments its frequency count.
// Returns the value and a boolean indicating if the key was found.
// Time complexity: O(1).
func (c *LFUCache[K, V]) Get(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		c.incrementFreq(e)
		return e.Value.value, true
	}
	return value, false
}

// Peek retrieves a value from the cache without updating the frequency count.
// Returns the value and a boolean indicating if the key was found.
// This operation does not affect the eviction order.
func (c *LFUCache[K, V]) Peek(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		return e.Value.value, true
	}
	return value, false
}

// Keys returns all keys currently in the cache.
func (c *LFUCache[K, V]) Keys() []K {
	all := make([]K, 0, len(c.cache))
	for k := range c.cache {
		all = append(all, k)
	}
	return all
}

// Values returns all values currently in the cache.
func (c *LFUCache[K, V]) Values() []V {
	all := make([]V, 0, len(c.cache))
	for _, v := range c.cache {
		all = append(all, v.Value.value)
	}
	return all
}

// All returns all key-value pairs in the cache.
func (c *LFUCache[K, V]) All() map[K]V {
	all := make(map[K]V)
	for k, v := range c.cache {
		all[k] = v.Value.value
	}
	return all
}

// Range iterates over all key-value pairs in the cache.
// The iteration stops if the function returns false.
func (c *LFUCache[K, V]) Range(f func(K, V) bool) {
	all := c.All()
	for k, v := range all {
		if !f(k, v) {
			break
		}
	}
}

// Delete removes a key from the cache.
// Returns true if the key was found and removed, false otherwise.
// Time complexity: O(1).
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
	c.cache = make(map[K]*list.Element[*entry[K, V]])
	c.freqMap = make(map[int]*list.List[*entry[K, V]])
	c.minFreq = 0
}

// SetMany stores multiple key-value pairs in the cache.
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
func (c *LFUCache[K, V]) Algorithm() string {
	return "lfu"
}

// Len returns the current number of items in the cache.
// Time complexity: O(1).
func (c *LFUCache[K, V]) Len() int {
	return len(c.cache)
}

// SizeBytes returns the total size of all cache entries in bytes.
func (c *LFUCache[K, V]) SizeBytes() int64 {
	return int64(size.Of(c.cache))
}

// DeleteLeastFrequent removes and returns the least frequently used item from the cache.
// Among items with the same frequency, the least recently used is evicted.
// Returns the key, value, and a boolean indicating if an item was removed.
// Time complexity: O(1).
func (c *LFUCache[K, V]) DeleteLeastFrequent() (k K, v V, ok bool) {
	if len(c.cache) == 0 {
		return k, v, false
	}

	freqList := c.freqMap[c.minFreq]
	e := freqList.Back() // LRU within the minimum frequency bucket
	if e == nil {
		return k, v, false
	}

	kv := e.Value
	c.deleteElement(e)
	return kv.key, kv.value, true
}

// incrementFreq moves an entry from its current frequency bucket to the next one.
// If the old bucket becomes empty and was the minimum, minFreq is updated.
// Time complexity: O(1).
func (c *LFUCache[K, V]) incrementFreq(e *list.Element[*entry[K, V]]) {
	ent := e.Value
	oldFreq := ent.freq
	newFreq := oldFreq + 1

	// Remove from old frequency bucket
	oldList := c.freqMap[oldFreq]
	oldList.Remove(e)

	// Clean up empty frequency bucket
	if oldList.Len() == 0 {
		delete(c.freqMap, oldFreq)
		if c.minFreq == oldFreq {
			c.minFreq = newFreq
		}
	}

	// Add to new frequency bucket (at front = MRU position)
	ent.freq = newFreq
	newList := c.getOrCreateFreqList(newFreq)
	newE := newList.PushFront(ent)
	c.cache[ent.key] = newE
}

// getOrCreateFreqList returns the list for the given frequency, creating it if needed.
func (c *LFUCache[K, V]) getOrCreateFreqList(freq int) *list.List[*entry[K, V]] {
	if l, ok := c.freqMap[freq]; ok {
		return l
	}
	l := list.New[*entry[K, V]]()
	c.freqMap[freq] = l
	return l
}

// deleteElement removes an element from its frequency bucket and the cache map.
// Time complexity: O(1) amortized.
func (c *LFUCache[K, V]) deleteElement(e *list.Element[*entry[K, V]]) {
	ent := e.Value
	freq := ent.freq

	// Remove from frequency bucket
	freqList := c.freqMap[freq]
	freqList.Remove(e)

	// Remove from cache map
	delete(c.cache, ent.key)

	// Clean up empty frequency bucket and update minFreq
	if freqList.Len() == 0 { //nolint:nestif
		delete(c.freqMap, freq)
		if freq == c.minFreq && len(c.cache) > 0 {
			minFreq := -1
			for f := range c.freqMap {
				if minFreq == -1 || f < minFreq {
					minFreq = f
				}
			}
			if minFreq >= 0 {
				c.minFreq = minFreq
			}
		}
	}
}
