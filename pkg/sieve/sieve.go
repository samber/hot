package sieve

import (
	"github.com/DmitriyVTitov/size"
	"github.com/samber/hot/internal"
	"github.com/samber/hot/internal/container/list"
	"github.com/samber/hot/pkg/base"
)

// SIEVECache implements the SIEVE eviction algorithm
// as described in https://cachemon.github.io/SIEVE-website/
//
// SIEVE maintains a list of entries with a "hand" pointer that scans for eviction victims.
// Each entry has a visited bit: on access it's set to true, and during eviction scanning,
// entries with visited=true are given a second chance (visited is cleared) while entries
// with visited=false are evicted.
//
// References:
//   - [SIEVE is Simpler than LRU: an Efficient Turn-Key Eviction Algorithm for Web Caches](https://junchengyang.com/publication/nsdi24-SIEVE.pdf)
//     (Zhang et al., NSDI 2024)
//   - [Why Aren't We SIEVE-ing?](https://brooker.co.za/blog/2023/12/15/sieve.html) (Mark Brooker, 2023)
//
// It is not safe for concurrent access and should be wrapped with a thread-safe layer if needed.
//
// The zero value is not usable.
type SIEVECache[K comparable, V any] struct { //nolint:revive
	noCopy internal.NoCopy // Prevents accidental copying of the cache

	capacity int // Maximum number of items the cache can hold

	// We use entry[K, V] (value type) instead of *entry[K, V] (pointer) to embed the entry
	// directly in the list element. This reduces one pointer indirection and one heap
	// allocation per entry, improving cache locality and reducing GC pressure.
	ll    *list.List[entry[K, V]]          // Doubly-linked list of entries (newest at front)
	cache map[K]*list.Element[entry[K, V]] // Map for O(1) key lookups to list elements
	hand  *list.Element[entry[K, V]]       // The "hand" pointer for SIEVE eviction scanning

	onEviction base.EvictionCallback[K, V] // Optional callback called when items are evicted
}

// entry represents a key-value pair stored in the SIEVE cache.
// Each entry has a visited bit that is set on access and cleared during eviction scanning.
type entry[K comparable, V any] struct {
	key     K    // The cache key
	value   V    // The cached value
	visited bool // Whether this entry has been accessed since last eviction scan
}

// NewSIEVECache creates a new SIEVE cache with the specified capacity.
func NewSIEVECache[K comparable, V any](capacity int) *SIEVECache[K, V] {
	return NewSIEVECacheWithEvictionCallback[K, V](capacity, nil)
}

// NewSIEVECacheWithEvictionCallback creates a new SIEVE cache
// with the specified capacity and eviction callback.
// The callback will be called whenever an item is evicted from the cache.
func NewSIEVECacheWithEvictionCallback[K comparable, V any](capacity int, onEviction base.EvictionCallback[K, V]) *SIEVECache[K, V] {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}

	return &SIEVECache[K, V]{
		capacity: capacity,
		ll:       list.New[entry[K, V]](),
		cache:    make(map[K]*list.Element[entry[K, V]]),
		hand:     nil,

		onEviction: onEviction,
	}
}

// Ensure SIEVECache implements InMemoryCache interface.
var _ base.InMemoryCache[string, int] = (*SIEVECache[string, int])(nil)

// Set stores a key-value pair in the cache.
// If the key already exists, its value is updated and visited bit is set.
// If the cache is at capacity, SIEVE eviction is performed to make room.
// Time complexity: O(1) average case, O(n) worst case when eviction scans entire cache.
func (c *SIEVECache[K, V]) Set(key K, value V) {
	if e, ok := c.cache[key]; ok {
		// Key exists: update value and mark as visited
		e.Value.value = value
		e.Value.visited = true
		return
	}

	// Check if we need to evictAndCallback BEFORE inserting (per SIEVE algorithm)
	if c.capacity != 0 && c.ll.Len() >= c.capacity {
		c.evictAndCallback()
	}

	// Key doesn't exist: create new entry at head of list
	// New entries start with visited=false
	ele := c.ll.PushFront(entry[K, V]{key: key, value: value, visited: false})
	c.cache[key] = ele
}

// Has checks if a key exists in the cache.
func (c *SIEVECache[K, V]) Has(key K) bool {
	_, hit := c.cache[key]
	return hit
}

// Get retrieves a value from the cache and marks it as visited.
// Returns the value and a boolean indicating if the key was found.
func (c *SIEVECache[K, V]) Get(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		e.Value.visited = true
		return e.Value.value, true
	}
	return value, false
}

// Peek retrieves a value from the cache without marking it as visited.
// Returns the value and a boolean indicating if the key was found.
func (c *SIEVECache[K, V]) Peek(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		return e.Value.value, true
	}
	return value, false
}

// Keys returns all keys currently in the cache.
func (c *SIEVECache[K, V]) Keys() []K {
	all := make([]K, 0, c.ll.Len())
	for k := range c.cache {
		all = append(all, k)
	}
	return all
}

// Values returns all values currently in the cache.
func (c *SIEVECache[K, V]) Values() []V {
	all := make([]V, 0, c.ll.Len())
	for _, v := range c.cache {
		all = append(all, v.Value.value)
	}
	return all
}

// All returns all key-value pairs in the cache.
func (c *SIEVECache[K, V]) All() map[K]V {
	all := make(map[K]V)
	for k, v := range c.cache {
		all[k] = v.Value.value
	}
	return all
}

// Range iterates over all key-value pairs in the cache.
// The iteration stops if the function returns false.
func (c *SIEVECache[K, V]) Range(f func(K, V) bool) {
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
func (c *SIEVECache[K, V]) Delete(key K) bool {
	if e, hit := c.cache[key]; hit {
		c.removeElementAndUpdateHand(e)
		return true
	}
	return false
}

// SetMany stores multiple key-value pairs in the cache.
// This is more efficient than calling Set multiple times.
func (c *SIEVECache[K, V]) SetMany(items map[K]V) {
	for k, v := range items {
		c.Set(k, v)
	}
}

// HasMany checks if multiple keys exist in the cache.
// Returns a map where keys are the input keys and values indicate existence.
func (c *SIEVECache[K, V]) HasMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Has(k)
	}
	return m
}

// GetMany retrieves multiple values from the cache.
// Returns a map of found key-value pairs and a slice of missing keys.
func (c *SIEVECache[K, V]) GetMany(keys []K) (map[K]V, []K) {
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

// PeekMany retrieves multiple values from the cache without marking them as visited.
// Returns a map of found key-value pairs and a slice of missing keys.
func (c *SIEVECache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
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
func (c *SIEVECache[K, V]) DeleteMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Delete(k)
	}
	return m
}

// Purge removes all keys and values from the cache.
func (c *SIEVECache[K, V]) Purge() {
	c.ll = list.New[entry[K, V]]()
	c.cache = make(map[K]*list.Element[entry[K, V]])
	c.hand = nil
}

// Capacity returns the maximum number of items the cache can hold.
func (c *SIEVECache[K, V]) Capacity() int {
	return c.capacity
}

// Algorithm returns the name of the eviction algorithm used by the cache.
func (c *SIEVECache[K, V]) Algorithm() string {
	return "sieve"
}

// Len returns the current number of items in the cache.
func (c *SIEVECache[K, V]) Len() int {
	return c.ll.Len()
}

// SizeBytes returns the total size of all cache entries in bytes.
func (c *SIEVECache[K, V]) SizeBytes() int64 {
	return int64(size.Of(c.cache))
}

// evict removes and returns the item that would be evicted next by SIEVE.
// Returns the key, value, and a boolean indicating if an item was removed.
// Note: This follows the SIEVE eviction logic, not necessarily the oldest inserted item.
func (c *SIEVECache[K, V]) evict() (k K, v V, ok bool) {
	if c.ll.Len() == 0 {
		return k, v, false
	}

	ele := c.hand
	if ele == nil {
		ele = c.ll.Back()
	}

	// Scan for an entry with visited=false
	for ele != nil && ele.Value.visited {
		ele.Value.visited = false
		ele = ele.Prev()
	}

	// If we scanned past the front, wrap around to back
	if ele == nil {
		ele = c.ll.Back()
		// Scan again from back
		for ele != nil && ele.Value.visited {
			ele.Value.visited = false
			ele = ele.Prev()
		}
	}

	if ele == nil {
		// Should not happen if cache is non-empty, but handle gracefully
		return k, v, false
	}

	k, v = ele.Value.key, ele.Value.value
	c.hand = ele.Prev()
	c.removeElement(ele)

	return k, v, true
}

// evictAndCallback is an internal method that performs SIEVE eviction and calls the callback.
func (c *SIEVECache[K, V]) evictAndCallback() {
	k, v, ok := c.evict()
	if ok && c.onEviction != nil {
		c.onEviction(base.EvictionReasonCapacity, k, v)
	}
}

// removeElementAndUpdateHand removes an element from both the list and the map.
// Handles updating the hand pointer if the deleted element is the current hand.
func (c *SIEVECache[K, V]) removeElementAndUpdateHand(e *list.Element[entry[K, V]]) {
	// If we're deleting the hand, move it first
	if c.hand == e {
		c.hand = e.Prev()
	}
	c.removeElement(e)
}

// removeElement removes an element without adjusting the hand pointer.
// Used internally when hand has already been adjusted.
func (c *SIEVECache[K, V]) removeElement(e *list.Element[entry[K, V]]) {
	c.ll.Remove(e)
	delete(c.cache, e.Value.key)
}
