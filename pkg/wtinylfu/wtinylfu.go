package wtinylfu

import (
	"github.com/DmitriyVTitov/size"
	"github.com/samber/hot/internal"
	"github.com/samber/hot/internal/container/list"
	"github.com/samber/hot/internal/sketch"
	"github.com/samber/hot/pkg/base"
)

// entry represents a key-value pair stored in the W-TinyLFU cache.
type entry[K comparable, V any] struct {
	key   K   // The cache key
	value V   // The cached value
	freq  int // The frequency of the key
}

// NewWTinyLFUCache creates a new Windowed TinyLFU cache with the specified capacity.
// The cache uses a window cache (1%) and SLRU main cache (99%) with frequency-based admission.
func NewWTinyLFUCache[K comparable, V any](capacity int) *WTinyLFUCache[K, V] {
	return NewWTinyLFUCacheWithEvictionCallback[K, V](capacity, nil)
}

// NewWTinyLFUCacheWithEvictionCallback creates a new Windowed TinyLFU cache with the specified capacity and eviction callback.
func NewWTinyLFUCacheWithEvictionCallback[K comparable, V any](capacity int, onEviction base.EvictionCallback[K, V]) *WTinyLFUCache[K, V] {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}

	// W-TinyLFU uses 1% of capacity for the window cache
	windowCapacity := max(capacity/100, 1)
	mainCapacity := capacity - windowCapacity

	// SLRU: 20% of main cache for probationary, 80% for protected
	probationaryCapacity := max(mainCapacity/5, 1)
	protectedCapacity := mainCapacity - probationaryCapacity

	cmsDepth := 4
	if capacity < 10_000 {
		cmsDepth = 3
	}

	return &WTinyLFUCache[K, V]{
		sketch: sketch.NewDoorkeeperCountMinSketch[K](capacity, cmsDepth),

		// Window cache (admission window) - 1% of total capacity
		windowCapacity: windowCapacity,
		windowLl:       list.New[*entry[K, V]](),
		windowCache:    make(map[K]*list.Element[*entry[K, V]]),

		// Main cache with SLRU structure - 99% of total capacity
		mainCapacity:         mainCapacity,
		probationaryCapacity: probationaryCapacity,
		protectedCapacity:    protectedCapacity,

		probationaryLl:  list.New[*entry[K, V]](),
		probationaryMap: make(map[K]*list.Element[*entry[K, V]]),
		protectedLl:     list.New[*entry[K, V]](),
		protectedMap:    make(map[K]*list.Element[*entry[K, V]]),

		onEviction: onEviction,
	}
}

// WTinyLFUCache is a Windowed TinyLFU cache implementation.
// It uses a window cache (1%) and SLRU main cache (99%) with frequency-based admission policy.
type WTinyLFUCache[K comparable, V any] struct {
	noCopy internal.NoCopy // Prevents accidental copying of the cache

	sketch *sketch.DoorkeeperCountMinSketch[K]

	// Window cache (admission window) - 1% of total capacity
	windowCapacity int
	windowLl       *list.List[*entry[K, V]]
	windowCache    map[K]*list.Element[*entry[K, V]]

	// Main cache with SLRU structure - 99% of total capacity
	mainCapacity         int
	probationaryCapacity int
	protectedCapacity    int

	probationaryLl  *list.List[*entry[K, V]]
	probationaryMap map[K]*list.Element[*entry[K, V]]
	protectedLl     *list.List[*entry[K, V]]
	protectedMap    map[K]*list.Element[*entry[K, V]]

	onEviction base.EvictionCallback[K, V]
}

// Ensure WTinyLFUCache implements InMemoryCache interface.
var _ base.InMemoryCache[string, int] = (*WTinyLFUCache[string, int])(nil)

// Set stores a key-value pair in the cache.
func (c *WTinyLFUCache[K, V]) Set(key K, value V) {
	// Check if key exists in protected segment
	if e, ok := c.protectedMap[key]; ok {
		c.protectedLl.MoveToFront(e)
		e.Value.freq++
		e.Value.value = value
		return
	}

	// Check if key exists in probationary segment
	if e, ok := c.probationaryMap[key]; ok {
		c.probationaryLl.MoveToFront(e)
		e.Value.freq++
		e.Value.value = value

		// Promote to protected segment if accessed enough times
		if e.Value.freq >= 2 {
			c.promoteToProtected(e)
		}
		return
	}

	// Check if key exists in window cache
	if e, ok := c.windowCache[key]; ok {
		c.windowLl.MoveToFront(e)
		e.Value.freq++
		e.Value.value = value
		return
	}

	// Key doesn't exist: add to window cache
	newEntry := c.windowLl.PushFront(&entry[K, V]{key, value, 1})
	c.windowCache[key] = newEntry

	// Check if window cache is full
	if c.windowLl.Len() > c.windowCapacity {
		c.evictFromWindow()
	}
}

// Has checks if a key exists in the cache.
func (c *WTinyLFUCache[K, V]) Has(key K) bool {
	if _, hit := c.protectedMap[key]; hit {
		return true
	}
	if _, hit := c.probationaryMap[key]; hit {
		return true
	}
	_, hit := c.windowCache[key]
	return hit
}

// Get retrieves a value from the cache.
func (c *WTinyLFUCache[K, V]) Get(key K) (value V, ok bool) {
	// Increment frequency exactly ONCE per cache access (TinyLFU paper spec)
	c.sketch.Inc(key)

	// Check protected segment first
	if e, hit := c.protectedMap[key]; hit {
		c.protectedLl.MoveToFront(e)
		e.Value.freq++
		return e.Value.value, true
	}

	// Check probationary segment
	if e, hit := c.probationaryMap[key]; hit {
		c.probationaryLl.MoveToFront(e)
		e.Value.freq++

		// Promote to protected segment if accessed enough times
		if e.Value.freq >= 2 {
			c.promoteToProtected(e)
		}
		return e.Value.value, true
	}

	// Check window cache
	if e, hit := c.windowCache[key]; hit {
		c.windowLl.MoveToFront(e)
		e.Value.freq++
		return e.Value.value, true
	}

	return value, false
}

// Peek retrieves a value from the cache without updating access order.
func (c *WTinyLFUCache[K, V]) Peek(key K) (value V, ok bool) {
	if e, hit := c.protectedMap[key]; hit {
		return e.Value.value, true
	}
	if e, hit := c.probationaryMap[key]; hit {
		return e.Value.value, true
	}
	if e, hit := c.windowCache[key]; hit {
		return e.Value.value, true
	}
	return value, false
}

// Keys returns all keys currently in the cache.
func (c *WTinyLFUCache[K, V]) Keys() []K {
	all := make([]K, 0, c.windowLl.Len()+c.probationaryLl.Len()+c.protectedLl.Len())

	// Add keys from all segments
	for k := range c.windowCache {
		all = append(all, k)
	}
	for k := range c.probationaryMap {
		all = append(all, k)
	}
	for k := range c.protectedMap {
		all = append(all, k)
	}

	return all
}

// Values returns all values currently in the cache.
func (c *WTinyLFUCache[K, V]) Values() []V {
	all := make([]V, 0, c.windowLl.Len()+c.probationaryLl.Len()+c.protectedLl.Len())

	// Add values from all segments
	for _, v := range c.windowCache {
		all = append(all, v.Value.value)
	}
	for _, v := range c.probationaryMap {
		all = append(all, v.Value.value)
	}
	for _, v := range c.protectedMap {
		all = append(all, v.Value.value)
	}

	return all
}

// All returns all key-value pairs in the cache.
func (c *WTinyLFUCache[K, V]) All() map[K]V {
	all := make(map[K]V)

	// Add items from all segments
	for k, v := range c.windowCache {
		all[k] = v.Value.value
	}
	for k, v := range c.probationaryMap {
		all[k] = v.Value.value
	}
	for k, v := range c.protectedMap {
		all[k] = v.Value.value
	}

	return all
}

// Range iterates over all key-value pairs in the cache.
func (c *WTinyLFUCache[K, V]) Range(f func(K, V) bool) {
	all := c.All()
	for k, v := range all {
		if !f(k, v) {
			break
		}
	}
}

// Delete removes a key from the cache.
func (c *WTinyLFUCache[K, V]) Delete(key K) bool {
	// Check protected segment
	if e, hit := c.protectedMap[key]; hit {
		delete(c.protectedMap, key)
		c.protectedLl.Remove(e)
		return true
	}

	// Check probationary segment
	if e, hit := c.probationaryMap[key]; hit {
		delete(c.probationaryMap, key)
		c.probationaryLl.Remove(e)
		return true
	}

	// Check window cache
	if e, hit := c.windowCache[key]; hit {
		delete(c.windowCache, key)
		c.windowLl.Remove(e)
		return true
	}

	return false
}

// SetMany stores multiple key-value pairs in the cache.
func (c *WTinyLFUCache[K, V]) SetMany(items map[K]V) {
	for k, v := range items {
		c.Set(k, v)
	}
}

// HasMany checks if multiple keys exist in the cache.
func (c *WTinyLFUCache[K, V]) HasMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Has(k)
	}
	return m
}

// GetMany retrieves multiple values from the cache.
func (c *WTinyLFUCache[K, V]) GetMany(keys []K) (map[K]V, []K) {
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
func (c *WTinyLFUCache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
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
func (c *WTinyLFUCache[K, V]) DeleteMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Delete(k)
	}
	return m
}

// Purge removes all keys and values from the cache.
func (c *WTinyLFUCache[K, V]) Purge() {
	c.windowLl = list.New[*entry[K, V]]()
	c.probationaryLl = list.New[*entry[K, V]]()
	c.protectedLl = list.New[*entry[K, V]]()
	c.windowCache = make(map[K]*list.Element[*entry[K, V]])
	c.probationaryMap = make(map[K]*list.Element[*entry[K, V]])
	c.protectedMap = make(map[K]*list.Element[*entry[K, V]])
	c.sketch.Reset()
}

// Capacity returns the maximum number of items the cache can hold.
func (c *WTinyLFUCache[K, V]) Capacity() int {
	return c.windowCapacity + c.mainCapacity
}

// Algorithm returns the name of the eviction algorithm used by the cache.
func (c *WTinyLFUCache[K, V]) Algorithm() string {
	return "wtinylfu"
}

// Len returns the current number of items in the cache.
func (c *WTinyLFUCache[K, V]) Len() int {
	return c.windowLl.Len() + c.probationaryLl.Len() + c.protectedLl.Len()
}

// SizeBytes returns the total size of all cache entries in bytes.
func (c *WTinyLFUCache[K, V]) SizeBytes() int64 {
	return int64(size.Of(c.windowCache)) + int64(size.Of(c.probationaryMap)) + int64(size.Of(c.protectedMap))
}

// promoteToProtected moves an entry from probationary to protected segment.
func (c *WTinyLFUCache[K, V]) promoteToProtected(e *list.Element[*entry[K, V]]) {
	entry := e.Value

	// Remove from probationary
	c.probationaryLl.Remove(e)
	delete(c.probationaryMap, entry.key)

	// Only promote if protected segment has space or if this item has higher frequency
	if c.protectedLl.Len() < c.protectedCapacity {
		// Protected has space, promote directly
		newElement := c.protectedLl.PushFront(entry)
		c.protectedMap[entry.key] = newElement
	} else {
		// Protected is full, compete with least recently used protected item
		protectedVictim := c.protectedLl.Back()
		if protectedVictim != nil && entry.freq > protectedVictim.Value.freq {
			// Evict protected victim, promote this item
			c.evictFromProtected()
			newElement := c.protectedLl.PushFront(entry)
			c.protectedMap[entry.key] = newElement
		} else {
			// Keep in probationary
			newElement := c.probationaryLl.PushFront(entry)
			c.probationaryMap[entry.key] = newElement
		}
	}
}

// evictFromWindow evicts an item from the window cache using frequency-based admission policy.
// This is the core of W-TinyLFU: admit new item if it has higher frequency than main cache victim.
func (c *WTinyLFUCache[K, V]) evictFromWindow() {
	windowVictim := c.windowLl.Back()
	if windowVictim == nil {
		return
	}

	// Get frequency of window victim (candidate for admission)
	windowFreq := c.sketch.Estimate(windowVictim.Value.key)

	// Find victim from probationary segment (LRU)
	mainVictim := c.probationaryLl.Back()
	if mainVictim == nil {
		// No main cache victim, promote window victim
		c.promoteToProbationary(windowVictim)
		return
	}

	// Get frequency of main cache victim
	mainFreq := c.sketch.Estimate(mainVictim.Value.key)

	// ADMIT NEW ITEM if it has higher or equal frequency than existing main cache item
	if windowFreq >= mainFreq {
		// Evict main cache victim, admit window victim
		c.evictFromProbationaryItem(mainVictim)
		c.promoteToProbationary(windowVictim)
	} else {
		// Reject new item, evict from window
		c.evictFromWindowOnly(windowVictim)
	}
}

// promoteToProbationary promotes a window victim to the probationary segment.
func (c *WTinyLFUCache[K, V]) promoteToProbationary(e *list.Element[*entry[K, V]]) {
	entry := e.Value

	// Remove from window
	c.windowLl.Remove(e)
	delete(c.windowCache, entry.key)

	// Add to probationary
	newElement := c.probationaryLl.PushFront(entry)
	c.probationaryMap[entry.key] = newElement

	// Check if probationary segment is full
	if c.probationaryLl.Len() > c.probationaryCapacity {
		c.evictFromProbationary()
	}
}

// evictFromWindowOnly evicts an item from the window cache only.
func (c *WTinyLFUCache[K, V]) evictFromWindowOnly(e *list.Element[*entry[K, V]]) {
	kv := e.Value
	c.windowLl.Remove(e)
	delete(c.windowCache, kv.key)

	if c.onEviction != nil {
		c.onEviction(base.EvictionReasonCapacity, kv.key, kv.value)
	}
}

// evictFromProbationary evicts an item from the probationary segment using LRU.
func (c *WTinyLFUCache[K, V]) evictFromProbationary() {
	e := c.probationaryLl.Back()
	if e != nil {
		c.evictFromProbationaryItem(e)
	}
}

// evictFromProbationaryItem evicts a specific item from the probationary segment.
func (c *WTinyLFUCache[K, V]) evictFromProbationaryItem(e *list.Element[*entry[K, V]]) {
	kv := e.Value
	delete(c.probationaryMap, kv.key)
	c.probationaryLl.Remove(e)

	if c.onEviction != nil {
		c.onEviction(base.EvictionReasonCapacity, kv.key, kv.value)
	}
}

// evictFromProtected evicts an item from the protected segment using LRU.
func (c *WTinyLFUCache[K, V]) evictFromProtected() {
	e := c.protectedLl.Back()
	if e != nil {
		kv := e.Value
		delete(c.protectedMap, kv.key)
		c.protectedLl.Remove(e)

		if c.onEviction != nil {
			c.onEviction(base.EvictionReasonCapacity, kv.key, kv.value)
		}
	}
}
