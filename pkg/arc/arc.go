package arc

import (
	"github.com/samber/hot/internal/container/list"

	"github.com/DmitriyVTitov/size"
	"github.com/samber/hot/internal"
	"github.com/samber/hot/pkg/base"
)

// entry represents a key-value pair stored in the ARC cache.
// Each entry is stored as a list element to maintain the access order.
type entry[K comparable, V any] struct {
	key   K // The cache key
	value V // The cached value
}

// NewARCCache creates a new ARC cache with the specified capacity.
// The cache will adaptively balance between LRU and LFU eviction policies.
func NewARCCache[K comparable, V any](capacity int) *ARCCache[K, V] {
	return NewARCCacheWithEvictionCallback[K, V](capacity, nil)
}

// NewARCCacheWithEvictionCallback creates a new ARC cache with the specified capacity and eviction callback.
// The callback will be called whenever an item is evicted from the cache.
func NewARCCacheWithEvictionCallback[K comparable, V any](capacity int, onEviction base.EvictionCallback[K, V]) *ARCCache[K, V] {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}

	return &ARCCache[K, V]{
		capacity: capacity,
		p:        0, // Initial adaptive parameter (starts at 0, meaning pure LRU)

		// T1: Recently accessed items (LRU list)
		t1: list.New[*entry[K, V]](),
		// T2: Frequently accessed items (LRU list)
		t2: list.New[*entry[K, V]](),
		// B1: Ghost entries for T1 (recently evicted from T1)
		b1: list.New[*entry[K, V]](),
		// B2: Ghost entries for T2 (recently evicted from T2)
		b2: list.New[*entry[K, V]](),

		// Maps for O(1) lookups
		t1Map: make(map[K]*list.Element[*entry[K, V]]),
		t2Map: make(map[K]*list.Element[*entry[K, V]]),
		b1Map: make(map[K]*list.Element[*entry[K, V]]),
		b2Map: make(map[K]*list.Element[*entry[K, V]]),

		onEviction: onEviction,
	}
}

// ARCCache is an Adaptive Replacement Cache implementation.
// It automatically balances between LRU and LFU policies based on access patterns.
// The cache maintains four lists:
// - T1: Recently accessed items (LRU)
// - T2: Frequently accessed items (LRU)
// - B1: Ghost entries for T1 (recently evicted from T1)
// - B2: Ghost entries for T2 (recently evicted from T2)
//
// The adaptive parameter 'p' determines the balance between T1 and T2 sizes.
// When p=0, the cache behaves like pure LRU.
// When p=capacity, the cache behaves like pure LFU.
//
// It is not safe for concurrent access and should be wrapped with a thread-safe layer if needed.
type ARCCache[K comparable, V any] struct {
	noCopy internal.NoCopy // Prevents accidental copying of the cache

	capacity int // Maximum number of items the cache can hold (0 = unlimited)
	p        int // Adaptive parameter that balances T1 and T2 sizes

	// Main cache lists (T1 and T2)
	t1 *list.List[*entry[K, V]] // Recently accessed items (LRU)
	t2 *list.List[*entry[K, V]] // Frequently accessed items (LRU)

	// Ghost lists (B1 and B2) - store only keys, not values
	b1 *list.List[*entry[K, V]] // Ghost entries for T1
	b2 *list.List[*entry[K, V]] // Ghost entries for T2

	// Maps for O(1) lookups to list elements
	t1Map map[K]*list.Element[*entry[K, V]] // Maps keys to T1 list elements
	t2Map map[K]*list.Element[*entry[K, V]] // Maps keys to T2 list elements
	b1Map map[K]*list.Element[*entry[K, V]] // Maps keys to B1 list elements (ghost entries)
	b2Map map[K]*list.Element[*entry[K, V]] // Maps keys to B2 list elements (ghost entries)

	onEviction base.EvictionCallback[K, V] // Optional callback called when items are evicted
}

// Ensure ARCCache implements InMemoryCache interface
var _ base.InMemoryCache[string, int] = (*ARCCache[string, int])(nil)

// Set stores a key-value pair in the cache.
// If the key already exists, its value is updated and it becomes the most recently used item.
// If the cache is at capacity, items are evicted according to the ARC algorithm.
// Time complexity: O(1) average case.
func (c *ARCCache[K, V]) Set(key K, value V) {
	// Case 1: Key exists in T1
	if e, ok := c.t1Map[key]; ok {
		// Move from T1 to T2 (promotion)
		c.t1.Remove(e)
		delete(c.t1Map, key)
		entry := e.Value
		entry.value = value
		e = c.t2.PushFront(entry)
		c.t2Map[key] = e
		return
	}

	// Case 2: Key exists in T2
	if e, ok := c.t2Map[key]; ok {
		// Move to front of T2
		c.t2.MoveToFront(e)
		e.Value.value = value
		return
	}

	// Case 3: Ghost hit in B1
	if _, ok := c.b1Map[key]; ok {
		c.handleGhostHit(key, value, true)
		return
	}

	// Case 4: Ghost hit in B2
	if _, ok := c.b2Map[key]; ok {
		c.handleGhostHit(key, value, false)
		return
	}

	// Case 5: Miss - new item
	c.handleMiss(key, value)
}

// handleGhostHit handles the case when we hit a ghost entry in B1 or B2.
// This is the core of the ARC algorithm's adaptive behavior.
func (c *ARCCache[K, V]) handleGhostHit(key K, value V, fromB1 bool) {
	// Remove from ghost list
	if fromB1 {
		if e, ok := c.b1Map[key]; ok {
			c.b1.Remove(e)
			delete(c.b1Map, key)
		}
	} else {
		if e, ok := c.b2Map[key]; ok {
			c.b2.Remove(e)
			delete(c.b2Map, key)
		}
	}

	// Update adaptive parameter p based on ghost hit
	b1Size := c.b1.Len()
	b2Size := c.b2.Len()
	if fromB1 {
		var delta int
		if b1Size == 0 {
			delta = 1
		} else {
			// Integer division is used intentionally here to adjust the adaptive parameter 'p'.
			// This affects the granularity of adjustments, ensuring that 'delta' scales appropriately.
			delta = max(1, b2Size/b1Size)
		}
		c.p = min(c.p+delta, c.capacity)
	} else {
		var delta int
		if b2Size == 0 {
			delta = 1
		} else {
			delta = max(1, b1Size/b2Size)
		}
		c.p = max(c.p-delta, 0)
	}

	// Canonical ARC: if |T1| >= p, evict from T1, else from T2
	if c.t1.Len() >= max(1, c.p) {
		c.evictFromT1()
	} else {
		c.evictFromT2()
	}

	// Insert at MRU of T2
	e := c.t2.PushFront(&entry[K, V]{key, value})
	c.t2Map[key] = e
	c.trimGhostLists()
}

// handleMiss handles the case when we have a complete miss (item not in cache or ghost lists).
func (c *ARCCache[K, V]) handleMiss(key K, value V) {
	cSize := c.capacity
	t1b1 := c.t1.Len() + c.b1.Len()

	// Canonical ARC miss handling
	if t1b1 == cSize {
		if c.t1.Len() == cSize {
			// Remove LRU from T1 and move to B1
			c.evictFromT1()
		} else {
			// Remove LRU from B1
			if c.b1.Len() > 0 {
				old := c.b1.Back()
				if old != nil {
					oldEntry := old.Value
					c.b1.Remove(old)
					delete(c.b1Map, oldEntry.key)
				}
			}
		}
	} else if t1b1 < cSize {
		total := c.t1.Len() + c.t2.Len() + c.b1.Len() + c.b2.Len()
		if total >= cSize {
			if total >= 2*cSize {
				// Remove LRU from B2
				if c.b2.Len() > 0 {
					old := c.b2.Back()
					if old != nil {
						oldEntry := old.Value
						c.b2.Remove(old)
						delete(c.b2Map, oldEntry.key)
					}
				}
			}
			// Canonical ARC: if |T1| >= p, evict from T1, else from T2
			if c.t1.Len() >= max(1, c.p) {
				c.evictFromT1()
			} else {
				c.evictFromT2()
			}
		}
	}

	// Insert at MRU of T1
	e := c.t1.PushFront(&entry[K, V]{key, value})
	c.t1Map[key] = e
	c.trimGhostLists()
}

// evictFromT1 evicts the least recently used item from T1.
// The evicted item becomes a ghost entry in B1.
func (c *ARCCache[K, V]) evictFromT1() {
	if c.t1.Len() == 0 {
		return
	}

	// Remove from end of T1 (LRU)
	e := c.t1.Back()
	c.t1.Remove(e)
	entryValue := e.Value
	delete(c.t1Map, entryValue.key)

	// Add to B1 as ghost entry (key only)
	ghostEntry := &entry[K, V]{key: entryValue.key}
	e = c.b1.PushFront(ghostEntry)
	c.b1Map[entryValue.key] = e

	// Trim B1 if it's too large
	if c.b1.Len() > c.capacity {
		old := c.b1.Back()
		if old != nil {
			oldEntry := old.Value
			c.b1.Remove(old)
			delete(c.b1Map, oldEntry.key)
		}
	}

	// Call eviction callback
	if c.onEviction != nil {
		c.onEviction(base.EvictionReasonCapacity, entryValue.key, entryValue.value)
	}
}

// evictFromT2 evicts the least recently used item from T2.
// The evicted item becomes a ghost entry in B2.
func (c *ARCCache[K, V]) evictFromT2() {
	if c.t2.Len() == 0 {
		return
	}

	// Remove from end of T2 (LRU)
	e := c.t2.Back()
	c.t2.Remove(e)
	entryValue := e.Value
	delete(c.t2Map, entryValue.key)

	// Add to B2 as ghost entry (key only)
	ghostEntry := &entry[K, V]{key: entryValue.key}
	e = c.b2.PushFront(ghostEntry)
	c.b2Map[entryValue.key] = e

	// Trim B2 if it's too large
	if c.b2.Len() > c.capacity {
		old := c.b2.Back()
		if old != nil {
			oldEntry := old.Value
			c.b2.Remove(old)
			delete(c.b2Map, oldEntry.key)
		}
	}

	// Call eviction callback
	if c.onEviction != nil {
		c.onEviction(base.EvictionReasonCapacity, entryValue.key, entryValue.value)
	}
}

// Has checks if a key exists in the cache.
func (c *ARCCache[K, V]) Has(key K) bool {
	_, hit := c.t1Map[key]
	if hit {
		return true
	}
	_, hit = c.t2Map[key]
	return hit
}

// Get retrieves a value from the cache and updates the access order.
// Returns the value and a boolean indicating if the key was found.
func (c *ARCCache[K, V]) Get(key K) (value V, ok bool) {
	// Check T1
	if e, hit := c.t1Map[key]; hit {
		// Move from T1 to T2 (promotion)
		c.t1.Remove(e)
		delete(c.t1Map, key)
		entry := e.Value
		e = c.t2.PushFront(entry)
		c.t2Map[key] = e
		return entry.value, true
	}

	// Check T2
	if e, hit := c.t2Map[key]; hit {
		// Move to front of T2
		c.t2.MoveToFront(e)
		return e.Value.value, true
	}

	return value, false
}

// Peek retrieves a value from the cache without updating the access order.
// Returns the value and a boolean indicating if the key was found.
func (c *ARCCache[K, V]) Peek(key K) (value V, ok bool) {
	// Check T1
	if e, hit := c.t1Map[key]; hit {
		return e.Value.value, true
	}

	// Check T2
	if e, hit := c.t2Map[key]; hit {
		return e.Value.value, true
	}

	return value, false
}

// Keys returns all keys currently in the cache.
func (c *ARCCache[K, V]) Keys() []K {
	all := make([]K, 0, c.t1.Len()+c.t2.Len())
	for k := range c.t1Map {
		all = append(all, k)
	}
	for k := range c.t2Map {
		all = append(all, k)
	}
	return all
}

// Values returns all values currently in the cache.
func (c *ARCCache[K, V]) Values() []V {
	all := make([]V, 0, c.t1.Len()+c.t2.Len())
	for _, v := range c.t1Map {
		all = append(all, v.Value.value)
	}
	for _, v := range c.t2Map {
		all = append(all, v.Value.value)
	}
	return all
}

// All returns all key-value pairs in the cache.
func (c *ARCCache[K, V]) All() map[K]V {
	all := make(map[K]V)
	for k, v := range c.t1Map {
		all[k] = v.Value.value
	}
	for k, v := range c.t2Map {
		all[k] = v.Value.value
	}
	return all
}

// Range iterates over all key-value pairs in the cache.
// The iteration stops if the function returns false.
func (c *ARCCache[K, V]) Range(f func(K, V) bool) {
	all := make(map[K]V)
	for k, v := range c.t1Map {
		all[k] = v.Value.value
	}
	for k, v := range all {
		if !f(k, v) {
			return
		}
	}

	all = make(map[K]V)
	for k, v := range c.t2Map {
		all[k] = v.Value.value
	}
	for k, v := range all {
		if !f(k, v) {
			return
		}
	}
}

// Delete removes a key from the cache.
// Returns true if the key was found and removed, false otherwise.
// Time complexity: O(1) average case.
func (c *ARCCache[K, V]) Delete(key K) bool {
	// Check T1
	if e, hit := c.t1Map[key]; hit {
		c.t1.Remove(e)
		delete(c.t1Map, key)
		return true
	}

	// Check T2
	if e, hit := c.t2Map[key]; hit {
		c.t2.Remove(e)
		delete(c.t2Map, key)
		return true
	}

	// Check B1 (ghost entry)
	if e, hit := c.b1Map[key]; hit {
		c.b1.Remove(e)
		delete(c.b1Map, key)
		return true
	}

	// Check B2 (ghost entry)
	if e, hit := c.b2Map[key]; hit {
		c.b2.Remove(e)
		delete(c.b2Map, key)
		return true
	}

	return false
}

// SetMany stores multiple key-value pairs in the cache.
// This is more efficient than calling Set multiple times.
func (c *ARCCache[K, V]) SetMany(items map[K]V) {
	for k, v := range items {
		c.Set(k, v)
	}
}

// HasMany checks if multiple keys exist in the cache.
// Returns a map where keys are the input keys and values indicate existence.
func (c *ARCCache[K, V]) HasMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Has(k)
	}
	return m
}

// GetMany retrieves multiple values from the cache.
// Returns a map of found key-value pairs and a slice of missing keys.
func (c *ARCCache[K, V]) GetMany(keys []K) (map[K]V, []K) {
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
func (c *ARCCache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
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
func (c *ARCCache[K, V]) DeleteMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Delete(k)
	}
	return m
}

// Purge removes all keys and values from the cache.
func (c *ARCCache[K, V]) Purge() {
	c.t1.Init()
	c.t2.Init()
	c.b1.Init()
	c.b2.Init()
	c.t1Map = make(map[K]*list.Element[*entry[K, V]])
	c.t2Map = make(map[K]*list.Element[*entry[K, V]])
	c.b1Map = make(map[K]*list.Element[*entry[K, V]])
	c.b2Map = make(map[K]*list.Element[*entry[K, V]])
	c.p = 0
}

// Capacity returns the maximum number of items the cache can hold.
func (c *ARCCache[K, V]) Capacity() int {
	return c.capacity
}

// Algorithm returns the name of the eviction algorithm used by the cache.
func (c *ARCCache[K, V]) Algorithm() string {
	return "arc"
}

// Len returns the current number of items in the cache.
func (c *ARCCache[K, V]) Len() int {
	return c.t1.Len() + c.t2.Len()
}

// SizeBytes returns the total size of all cache entries in bytes.
// For generic caches, this returns 0 as the size cannot be determined without type information.
// Specialized implementations should override this method.
func (c *ARCCache[K, V]) SizeBytes() int64 {
	return int64(size.Of(c.t1Map) + size.Of(c.t2Map) + size.Of(c.b1Map) + size.Of(c.b2Map))
}

// DeleteOldest removes and returns the least recently used item from the cache.
// For ARC, this means removing from the end of T1 or T2 based on the current state.
// Returns the key, value, and a boolean indicating if an item was found.
func (c *ARCCache[K, V]) DeleteOldest() (k K, v V, ok bool) {
	// Try to delete from T1 first (recent items)
	if c.t1.Len() > 0 {
		e := c.t1.Back()
		c.t1.Remove(e)
		entry := e.Value
		delete(c.t1Map, entry.key)
		return entry.key, entry.value, true
	}

	// If T1 is empty, try T2
	if c.t2.Len() > 0 {
		e := c.t2.Back()
		c.t2.Remove(e)
		entry := e.Value
		delete(c.t2Map, entry.key)
		return entry.key, entry.value, true
	}

	return k, v, false
}

// Helper functions for min/max operations
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// trimGhostLists trims the ghost lists if they exceed capacity.
func (c *ARCCache[K, V]) trimGhostLists() {
	for c.b1.Len() > c.capacity {
		old := c.b1.Back()
		if old != nil {
			oldEntry := old.Value
			c.b1.Remove(old)
			delete(c.b1Map, oldEntry.key)
		}
	}
	for c.b2.Len() > c.capacity {
		old := c.b2.Back()
		if old != nil {
			oldEntry := old.Value
			c.b2.Remove(old)
			delete(c.b2Map, oldEntry.key)
		}
	}
}
