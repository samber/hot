package twoqueue

import (
	"container/list"

	"github.com/DmitriyVTitov/size"
	"github.com/samber/hot/internal"
	"github.com/samber/hot/pkg/base"
	"github.com/samber/hot/pkg/lru"
)

const (
	// Default2QRecentRatio is the ratio of the 2Q cache dedicated
	// to recently added entries that have only been accessed once.
	// This is typically set to 25% of the total cache capacity.
	Default2QRecentRatio = 0.25

	// Default2QGhostEntries is the default ratio of ghost
	// entries kept to track entries recently evicted.
	// This is typically set to 50% of the total cache capacity.
	Default2QGhostEntries = 0.50
)

// New2QCache creates a new 2Q cache with the specified capacity.
// Uses default ratios for recent entries (25%) and ghost entries (50%).
func New2QCache[K comparable, V any](capacity int) *TwoQueueCache[K, V] {
	return New2QCacheWithRatioAndEvictionCallback[K, V](capacity, Default2QRecentRatio, Default2QGhostEntries, nil)
}

// New2QCacheWithEvictionCallback creates a new 2Q cache with the specified capacity and eviction callback.
// Uses default ratios for recent entries (25%) and ghost entries (50%).
// The callback will be called whenever an item is evicted from the cache.
func New2QCacheWithEvictionCallback[K comparable, V any](capacity int, onEviction base.EvictionCallback[K, V]) *TwoQueueCache[K, V] {
	return New2QCacheWithRatioAndEvictionCallback(capacity, Default2QRecentRatio, Default2QGhostEntries, onEviction)
}

// New2QCacheWithRatio creates a new 2Q cache with the specified capacity and custom ratios.
// The recentRatio determines what portion of the cache is dedicated to recently accessed items.
// The ghostRatio determines what portion of the cache is used for tracking evicted items.
func New2QCacheWithRatio[K comparable, V any](capacity int, recentRatio, ghostRatio float64) *TwoQueueCache[K, V] {
	return New2QCacheWithRatioAndEvictionCallback[K, V](capacity, recentRatio, ghostRatio, nil)
}

// New2QCacheWithRatioAndEvictionCallback creates a new 2Q cache with the specified capacity,
// ratios, and eviction callback. This is the main constructor for 2Q caches.
// The 2Q algorithm separates items into three categories: recent, frequent, and ghost entries.
func New2QCacheWithRatioAndEvictionCallback[K comparable, V any](capacity int, recentRatio, ghostRatio float64, onEviction base.EvictionCallback[K, V]) *TwoQueueCache[K, V] {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}
	if recentRatio < 0.0 || recentRatio > 1.0 {
		panic("recentRatio must be between 0 and 1")
	}
	if ghostRatio < 0.0 || ghostRatio > 1.0 {
		panic("ghostRatio must be between 0 and 1")
	}

	// Determine the sub-capacities based on the provided ratios
	recentCapacity := int(float64(capacity) * recentRatio)
	ghostCapacity := int(float64(capacity) * ghostRatio)
	frequentCapacity := capacity - recentCapacity

	// Ensure minimum capacities
	if recentCapacity < 1 {
		recentCapacity = 1
	}
	if frequentCapacity < 1 {
		frequentCapacity = 1
	}

	return &TwoQueueCache[K, V]{
		capacity:         capacity,
		recentCapacity:   recentCapacity,
		ghostCapacity:    ghostCapacity,
		frequentCapacity: frequentCapacity,
		recentRatio:      recentRatio,
		ghostRatio:       ghostRatio,

		recent:   newFIFOCache[K, V](recentCapacity),
		frequent: lru.NewLRUCache[K, V](frequentCapacity),
		ghost:    newFIFOCache[K, struct{}](ghostCapacity),

		onEviction: onEviction,
	}
}

// TwoQueueCache implements the 2Q (Two-Queue) eviction algorithm, which is an enhancement
// over the standard LRU cache that tracks both frequently and recently used entries separately.
// This avoids a burst in access to new entries from evicting frequently used entries.
// The algorithm adds some additional tracking overhead but provides better cache performance
// for workloads with temporal locality patterns.
// TwoQueueCache is not safe for concurrent access.
type TwoQueueCache[K comparable, V any] struct {
	noCopy internal.NoCopy // Prevents accidental copying of the cache

	capacity         int     // Total cache capacity
	recentCapacity   int     // Capacity allocated for recent entries
	ghostCapacity    int     // Capacity allocated for ghost entries
	frequentCapacity int     // Capacity allocated for frequent entries
	recentRatio      float64 // Ratio of capacity for recent entries
	ghostRatio       float64 // Ratio of capacity for ghost entries

	recent   *FIFOCache[K, V]        // FIFO list for recently accessed items
	frequent *lru.LRUCache[K, V]     // LRU list for frequently accessed items
	ghost    *FIFOCache[K, struct{}] // FIFO list for ghost entries

	onEviction base.EvictionCallback[K, V] // Optional callback called when items are evicted
}

// Ensure TwoQueueCache implements InMemoryCache interface
var _ base.InMemoryCache[string, int] = (*TwoQueueCache[string, int])(nil)

// Set stores a key-value pair in the cache using the 2Q algorithm.
// The algorithm determines where to place the item based on its access history:
// 1. If the key is already in the frequent cache, update its value
// 2. If the key is in the recent cache, promote it to the frequent cache
// 3. If the key is in the ghost cache, add it directly to the frequent cache
// 4. Otherwise, add it to the recent cache
func (c *TwoQueueCache[K, V]) Set(key K, value V) {
	// Check if the key is already in the frequent cache
	if c.frequent.Has(key) {
		c.frequent.Set(key, value)
		return
	}

	// Check if the key is in the recent cache, promote to frequent
	if c.recent.Has(key) {
		c.recent.Delete(key)
		c.ensureFrequentSpace()
		c.frequent.Set(key, value)
		return
	}

	// Check if the key is in the ghost cache, add directly to frequent
	if c.ghost.Has(key) {
		c.ghost.Delete(key)
		c.ensureFrequentSpace()
		c.frequent.Set(key, value)
		return
	}

	// Add to the recent cache
	c.ensureRecentSpace()
	c.recent.Set(key, value)
}

// Has checks if a key exists in either the frequent or recent caches.
// This operation does not affect the cache state or promote items between caches.
func (c *TwoQueueCache[K, V]) Has(key K) bool {
	return c.frequent.Has(key) || c.recent.Has(key)
}

// Get retrieves a value from the cache and may promote items between caches.
// If the key is in the frequent cache, it's returned directly.
// If the key is in the recent cache, it's promoted to the frequent cache.
// Returns the value and a boolean indicating if the key was found.
func (c *TwoQueueCache[K, V]) Get(key K) (value V, ok bool) {
	// Check if this is a frequent value
	if val, ok := c.frequent.Get(key); ok {
		return val, ok
	}

	// If the value is contained in recent, then we promote it to frequent
	if val, ok := c.recent.Peek(key); ok {
		c.recent.Delete(key)
		c.ensureFrequentSpace()
		c.frequent.Set(key, val)
		return val, ok
	}

	// No hit
	return value, false
}

// Peek retrieves a value from the cache without affecting the cache state.
// This operation does not promote items between caches or update access order.
// Returns the value and a boolean indicating if the key was found.
func (c *TwoQueueCache[K, V]) Peek(key K) (value V, ok bool) {
	// Check if this is a frequent value
	if val, ok := c.frequent.Peek(key); ok {
		return val, ok
	}

	// Check if this is a recent value
	return c.recent.Peek(key)
}

// Keys returns all keys from both frequent and recent caches combined.
// The order of keys in the returned slice is not guaranteed.
func (c *TwoQueueCache[K, V]) Keys() []K {
	k1 := c.frequent.Keys()
	k2 := c.recent.Keys()
	return append(k1, k2...)
}

// Values returns all values from both frequent and recent caches combined.
// The order of values in the returned slice is not guaranteed.
func (c *TwoQueueCache[K, V]) Values() []V {
	v1 := c.frequent.Values()
	v2 := c.recent.Values()
	return append(v1, v2...)
}

// Range iterates over all key-value pairs from both frequent and recent caches.
// The iteration stops if the function returns false.
// The iteration order is not guaranteed.
func (c *TwoQueueCache[K, V]) Range(f func(K, V) bool) {
	c.frequent.Range(f)
	c.recent.Range(f)
}

// Delete removes a key from all caches (frequent, recent, and ghost).
// Returns true if the key was found and removed from any cache, false otherwise.
func (c *TwoQueueCache[K, V]) Delete(key K) bool {
	return c.frequent.Delete(key) || c.recent.Delete(key) || c.ghost.Delete(key)
}

// Purge removes all keys and values from all caches.
// This operation clears the frequent, recent, and ghost caches simultaneously.
func (c *TwoQueueCache[K, V]) Purge() {
	c.recent.Purge()
	c.frequent.Purge()
	c.ghost.Purge()
}

// SetMany stores multiple key-value pairs in the cache.
// This is more efficient than calling Set multiple times.
// Each key-value pair is processed individually according to the 2Q algorithm.
func (c *TwoQueueCache[K, V]) SetMany(items map[K]V) {
	for k, v := range items {
		c.Set(k, v)
	}
}

// HasMany checks if multiple keys exist in the cache.
// Returns a map where keys are the input keys and values indicate existence.
// This operation does not affect the cache state.
func (c *TwoQueueCache[K, V]) HasMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Has(k)
	}
	return m
}

// GetMany retrieves multiple values from the cache.
// Returns a map of found key-value pairs and a slice of missing keys.
// Items may be promoted between caches during this operation.
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

// PeekMany retrieves multiple values from the cache without affecting the cache state.
// Returns a map of found key-value pairs and a slice of missing keys.
// This operation does not promote items between caches.
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

// DeleteMany removes multiple keys from all caches.
// Returns a map where keys are the input keys and values indicate if the key was found and removed.
func (c *TwoQueueCache[K, V]) DeleteMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Delete(k)
	}
	return m
}

// Capacity returns the total capacity of the cache.
// This is the sum of the capacities of all cache components.
func (c *TwoQueueCache[K, V]) Capacity() int {
	return c.capacity
}

// Algorithm returns the name of the eviction algorithm used by the cache.
// This is used for debugging and monitoring purposes.
func (c *TwoQueueCache[K, V]) Algorithm() string {
	return "2q"
}

// Len returns the total number of items across all caches.
// This is the sum of the lengths of frequent and recent caches (ghost cache not included).
func (c *TwoQueueCache[K, V]) Len() int {
	return c.frequent.Len() + c.recent.Len()
}

// SizeBytes returns the total size of all cache entries in bytes.
// For generic caches, this returns 0 as the size cannot be determined without type information.
// Specialized implementations should override this method.
func (c *TwoQueueCache[K, V]) SizeBytes() int64 {
	return int64(c.frequent.SizeBytes() + int64(size.Of(c.recent.cache)) + int64(size.Of(c.ghost.cache)))
}

// ensureRecentSpace makes room in the recent cache by evicting items when necessary.
// Evicted items from recent are added to the ghost cache.
func (c *TwoQueueCache[K, V]) ensureRecentSpace() {
	if c.recent.Len() < c.recentCapacity {
		return
	}

	// Evict oldest item from recent and add to ghost
	if key, value, ok := c.recent.DeleteOldest(); ok {
		if c.onEviction != nil {
			c.onEviction(base.EvictionReasonCapacity, key, value)
		}
		c.ensureGhostSpace()
		c.ghost.Set(key, struct{}{})
	}
}

// ensureFrequentSpace makes room in the frequent cache by evicting items when necessary.
// Evicted items from frequent are discarded (not added to ghost).
func (c *TwoQueueCache[K, V]) ensureFrequentSpace() {
	if c.frequent.Len() < c.frequentCapacity {
		return
	}

	// Evict oldest item from frequent
	if key, value, ok := c.frequent.DeleteOldest(); ok {
		if c.onEviction != nil {
			c.onEviction(base.EvictionReasonCapacity, key, value)
		}
	}
}

// ensureGhostSpace makes room in the ghost cache by evicting items when necessary.
// Evicted items from ghost are discarded.
func (c *TwoQueueCache[K, V]) ensureGhostSpace() {
	if c.ghost.Len() < c.ghostCapacity {
		return
	}

	// Evict oldest item from ghost
	c.ghost.DeleteOldest()
}

// FIFOCache implements a simple FIFO (First-In-First-Out) cache using a linked list.
// This is used for the recent and ghost components of the 2Q algorithm.
type FIFOCache[K comparable, V any] struct {
	capacity int
	ll       *list.List
	cache    map[K]*list.Element
}

// newFIFOCache creates a new FIFO cache with the specified capacity.
func newFIFOCache[K comparable, V any](capacity int) *FIFOCache[K, V] {
	return &FIFOCache[K, V]{
		capacity: capacity,
		ll:       list.New(),
		cache:    make(map[K]*list.Element),
	}
}

// entry represents a key-value pair stored in the FIFO cache.
type entry[K comparable, V any] struct {
	key   K
	value V
}

// Set stores a key-value pair in the FIFO cache.
// If the key already exists, its value is updated but position is not changed.
// If the cache is at capacity, the oldest item is evicted.
func (c *FIFOCache[K, V]) Set(key K, value V) {
	if e, ok := c.cache[key]; ok {
		// Key exists: update value but don't change position
		e.Value.(*entry[K, V]).value = value
		return
	}

	// Key doesn't exist: create new entry at back of list (FIFO)
	e := c.ll.PushBack(&entry[K, V]{key, value})
	c.cache[key] = e

	// Check if we need to evict the oldest item
	if c.capacity > 0 && c.ll.Len() > c.capacity {
		c.DeleteOldest()
	}
}

// Has checks if a key exists in the FIFO cache.
func (c *FIFOCache[K, V]) Has(key K) bool {
	_, hit := c.cache[key]
	return hit
}

// Get retrieves a value from the FIFO cache.
// This operation does not change the position of the item in the FIFO order.
func (c *FIFOCache[K, V]) Get(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		return e.Value.(*entry[K, V]).value, true
	}
	return value, false
}

// Peek retrieves a value from the FIFO cache without affecting the cache state.
func (c *FIFOCache[K, V]) Peek(key K) (value V, ok bool) {
	return c.Get(key)
}

// Delete removes a key from the FIFO cache.
// Returns true if the key was found and removed, false otherwise.
func (c *FIFOCache[K, V]) Delete(key K) bool {
	if e, hit := c.cache[key]; hit {
		c.deleteElement(e)
		return true
	}
	return false
}

// DeleteOldest removes and returns the oldest item from the FIFO cache.
// Returns the key, value, and a boolean indicating if an item was removed.
func (c *FIFOCache[K, V]) DeleteOldest() (k K, v V, ok bool) {
	e := c.ll.Front()
	if e != nil {
		c.deleteElement(e)
		kv := e.Value.(*entry[K, V])
		return kv.key, kv.value, true
	}
	return k, v, false
}

// deleteElement removes an element from both the list and the map.
func (c *FIFOCache[K, V]) deleteElement(e *list.Element) {
	c.ll.Remove(e)
	kv := e.Value.(*entry[K, V])
	delete(c.cache, kv.key)
}

// Keys returns all keys currently in the FIFO cache.
func (c *FIFOCache[K, V]) Keys() []K {
	all := make([]K, 0, c.ll.Len())
	for k := range c.cache {
		all = append(all, k)
	}
	return all
}

// Values returns all values currently in the FIFO cache.
func (c *FIFOCache[K, V]) Values() []V {
	all := make([]V, 0, c.ll.Len())
	for _, v := range c.cache {
		all = append(all, v.Value.(*entry[K, V]).value)
	}
	return all
}

// Range iterates over all key-value pairs in the FIFO cache.
// The iteration stops if the function returns false.
func (c *FIFOCache[K, V]) Range(f func(K, V) bool) {
	for k, v := range c.cache {
		if !f(k, v.Value.(*entry[K, V]).value) {
			break
		}
	}
}

// Purge removes all keys and values from the FIFO cache.
func (c *FIFOCache[K, V]) Purge() {
	c.ll = list.New()
	c.cache = make(map[K]*list.Element)
}

// Len returns the current number of items in the FIFO cache.
func (c *FIFOCache[K, V]) Len() int {
	return c.ll.Len()
}
