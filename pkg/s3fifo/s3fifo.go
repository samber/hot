package s3fifo

//
// https://s3fifo.com/
//

import (
	"github.com/DmitriyVTitov/size"
	"github.com/samber/hot/internal"
	"github.com/samber/hot/internal/container/list"
	"github.com/samber/hot/pkg/base"
)

// entry represents a key-value pair with frequency tracking in the S3 FIFO cache.
type entry[K comparable, V any] struct {
	key   K   // The cache key
	value V   // The cached value
	freq  int // Frequency counter (capped at 3)
	queue int // Which queue this entry belongs to (0=small, 1=main)
}

// S3FIFOCache is a Simple, Scalable cache with 3 FIFO queues.
// It uses three queues: Small (10%), Main (90%), and Ghost (metadata only).
type S3FIFOCache[K comparable, V any] struct {
	noCopy internal.NoCopy // Prevents accidental copying of the cache

	capacity int                               // Maximum number of items the cache can hold
	small    *list.List[*entry[K, V]]          // Small queue (10% of capacity) for new items
	main     *list.List[*entry[K, V]]          // Main queue (90% of capacity) for popular items
	cache    map[K]*list.Element[*entry[K, V]] // Map for O(1) key lookups to list elements
	ghost    *list.List[K]                     // Ghost queue for evicted small item keys (metadata only)
	ghostMap map[K]*list.Element[K]            // Map for O(1) ghost queue lookups
	freq     map[K]int                         // Frequency counters for keys (even if evicted to ghost)

	smallLimit int // Capacity of small queue (10% of total)
	mainLimit  int // Capacity of main queue (90% of total)
	ghostLimit int // Capacity of ghost queue (matches main queue)

	onEviction base.EvictionCallback[K, V] // Optional callback called when items are evicted
}

const (
	maxFrequency = 3 // Maximum frequency cap as specified in S3-FIFO paper
)

// Ensure S3FIFOCache implements InMemoryCache interface.
var _ base.InMemoryCache[string, int] = (*S3FIFOCache[string, int])(nil)

// NewS3FIFOCache creates a new S3 FIFO cache with the specified capacity.
func NewS3FIFOCache[K comparable, V any](capacity int) *S3FIFOCache[K, V] {
	return NewS3FIFOCacheWithEvictionCallback[K, V](capacity, nil)
}

// NewS3FIFOCacheWithEvictionCallback creates a new S3 FIFO cache with the specified capacity and eviction callback.
func NewS3FIFOCacheWithEvictionCallback[K comparable, V any](capacity int, onEviction base.EvictionCallback[K, V]) *S3FIFOCache[K, V] {
	if capacity <= 0 {
		panic("capacity must be greater than 0")
	}

	smallLimit := max(capacity/10, 1)
	var mainLimit int

	// Ensure main queue has at least 1 item for capacity >= 2
	if capacity >= 2 {
		mainLimit = capacity - smallLimit
		if mainLimit <= 0 {
			mainLimit = 1
			smallLimit = capacity - 1
		}
	} else {
		// For capacity 1, use small queue only
		mainLimit = 0
	}

	// S3-FIFO paper: ghost queue should be same size as main queue
	ghostLimit := mainLimit

	return &S3FIFOCache[K, V]{
		capacity:   capacity,
		small:      list.New[*entry[K, V]](),
		main:       list.New[*entry[K, V]](),
		cache:      make(map[K]*list.Element[*entry[K, V]]),
		ghost:      list.New[K](),
		ghostMap:   make(map[K]*list.Element[K]),
		freq:       make(map[K]int),
		smallLimit: smallLimit,
		mainLimit:  mainLimit,
		ghostLimit: ghostLimit,
		onEviction: onEviction,
	}
}

// Set stores a key-value pair in the cache.
// If the key already exists, its value is updated without changing frequency.
// If the cache is at capacity, items are evicted according to S3 FIFO policy.
func (c *S3FIFOCache[K, V]) Set(key K, value V) {
	if e, ok := c.cache[key]; ok {
		// Key exists: update value only (frequency unchanged per S3-FIFO paper)
		entry := e.Value
		entry.value = value
		return
	}

	// Key doesn't exist: evict if needed, then insert
	for c.Len() >= c.capacity {
		if !c.evict() {
			break
		}
	}

	c.insert(key, value)
}

// Get retrieves a value from the cache and increments its frequency.
// Returns the value and a boolean indicating if the key was found.
func (c *S3FIFOCache[K, V]) Get(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		entry := e.Value
		entry.freq = min(entry.freq+1, maxFrequency)
		c.freq[key] = entry.freq

		// If item is in small queue and has been accessed multiple times, promote to main queue
		// S3-FIFO paper: promote after multiple accesses (freq >= 2)
		if entry.queue == 0 && entry.freq >= 2 {
			c.promoteToMain(e)
		}

		return entry.value, true
	}

	// Check if key is in ghost queue (indicates it was recently evicted)
	c.checkGhost(key)

	var zero V
	return zero, false
}

// Has checks if a key exists in the cache.
func (c *S3FIFOCache[K, V]) Has(key K) bool {
	_, hit := c.cache[key]
	return hit
}

// Peek retrieves a value from the cache without updating frequency or position.
// Returns the value and a boolean indicating if the key was found.
func (c *S3FIFOCache[K, V]) Peek(key K) (value V, ok bool) {
	if e, hit := c.cache[key]; hit {
		return e.Value.value, true
	}
	var zero V
	return zero, false
}

// Keys returns all keys currently in the cache.
func (c *S3FIFOCache[K, V]) Keys() []K {
	all := make([]K, 0, len(c.cache))
	for k := range c.cache {
		all = append(all, k)
	}
	return all
}

// Values returns all values currently in the cache.
func (c *S3FIFOCache[K, V]) Values() []V {
	all := make([]V, 0, len(c.cache))
	for _, v := range c.cache {
		all = append(all, v.Value.value)
	}
	return all
}

// All returns all key-value pairs in the cache.
func (c *S3FIFOCache[K, V]) All() map[K]V {
	all := make(map[K]V, len(c.cache))
	for k, v := range c.cache {
		all[k] = v.Value.value
	}
	return all
}

// Range iterates over all key-value pairs in the cache.
// The iteration stops if the function returns false.
func (c *S3FIFOCache[K, V]) Range(f func(K, V) bool) {
	// Iterate directly over cache map to avoid creating a full copy
	for k, v := range c.cache {
		if !f(k, v.Value.value) {
			break
		}
	}
}

// Delete removes a key from the cache.
// Returns true if the key was found and removed, false otherwise.
func (c *S3FIFOCache[K, V]) Delete(key K) bool {
	if e, hit := c.cache[key]; hit {
		c.deleteElement(e)
		// Remove from ghost queue as well to prevent orphaned entries
		c.removeFromGhost(key)
		// Clean up frequency map to prevent memory leak
		delete(c.freq, key)
		return true
	}
	return false
}

// SetMany stores multiple key-value pairs in the cache.
func (c *S3FIFOCache[K, V]) SetMany(items map[K]V) {
	// Process each item individually to maintain S3-FIFO invariants
	// This ensures proper eviction logic and queue management
	for k, v := range items {
		c.Set(k, v)
	}
}

// HasMany checks if multiple keys exist in the cache.
func (c *S3FIFOCache[K, V]) HasMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Has(k)
	}
	return m
}

// GetMany retrieves multiple values from the cache.
func (c *S3FIFOCache[K, V]) GetMany(keys []K) (map[K]V, []K) {
	m := make(map[K]V, len(keys))
	var missing []K

	// First pass: get all hits without modifying frequency
	for _, k := range keys {
		if v, ok := c.Peek(k); ok {
			m[k] = v
		} else {
			missing = append(missing, k)
		}
	}

	// Second pass: increment frequency for hits (batch ghost tracking)
	for k := range m {
		if e, ok := c.cache[k]; ok {
			entry := e.Value
			entry.freq = min(entry.freq+1, maxFrequency)
			c.freq[k] = entry.freq

			// If item is in small queue and has been accessed multiple times, promote to main queue
			// S3-FIFO paper: promote after multiple accesses (freq >= 2)
			if entry.queue == 0 && entry.freq >= 2 {
				c.promoteToMain(e)
			}
		}
	}

	// Track ghost misses for missing keys
	for _, k := range missing {
		c.checkGhost(k)
	}

	return m, missing
}

// PeekMany retrieves multiple values from the cache without updating access order.
func (c *S3FIFOCache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
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
func (c *S3FIFOCache[K, V]) DeleteMany(keys []K) map[K]bool {
	m := make(map[K]bool, len(keys))
	for _, k := range keys {
		m[k] = c.Delete(k)
	}
	return m
}

// Purge removes all keys and values from the cache.
func (c *S3FIFOCache[K, V]) Purge() {
	c.small.Init()
	c.main.Init()
	c.ghost.Init()
	c.cache = make(map[K]*list.Element[*entry[K, V]])
	c.ghostMap = make(map[K]*list.Element[K])
	c.freq = make(map[K]int)
}

// Capacity returns the maximum number of items the cache can hold.
func (c *S3FIFOCache[K, V]) Capacity() int {
	return c.capacity
}

// Algorithm returns the name of the eviction algorithm used by the cache.
func (c *S3FIFOCache[K, V]) Algorithm() string {
	return "s3fifo"
}

// Len returns the current number of items in the cache.
func (c *S3FIFOCache[K, V]) Len() int {
	return c.small.Len() + c.main.Len()
}

// SizeBytes returns the total size of all cache entries in bytes.
func (c *S3FIFOCache[K, V]) SizeBytes() int64 {
	var total int64
	for _, v := range c.cache {
		total += int64(size.Of(v.Value.value))
	}
	return total
}

// insert adds a new entry to the appropriate queue.
func (c *S3FIFOCache[K, V]) insert(key K, value V) {
	// Check if key is in ghost queue (indicates it was recently evicted)
	if c.isInGhost(key) {
		// Remove from ghost queue immediately upon reinsertion
		c.removeFromGhost(key)

		// Get the ghost frequency (frequency accumulated while in ghost queue)
		ghostFreq := c.freq[key]

		// S3-FIFO paper: frequency = ghost frequency + 1 (reinsertion frequency)
		initialFreq := min(ghostFreq+1, maxFrequency)

		// Add to small queue with reinsertion frequency
		e := c.small.PushBack(&entry[K, V]{key: key, value: value, freq: initialFreq, queue: 0})
		c.cache[key] = e
		c.freq[key] = initialFreq
	} else {
		// Add to small queue for new items
		e := c.small.PushBack(&entry[K, V]{key: key, value: value, freq: 0, queue: 0})
		c.cache[key] = e
		c.freq[key] = 0
	}
}

// evict removes an item from the cache according to S3 FIFO policy.
func (c *S3FIFOCache[K, V]) evict() bool {
	// Priority 1: Evict from small queue if it's over limit (S3-FIFO paper)
	if c.small.Len() > c.smallLimit {
		return c.evictFromSmallToGhost()
	}
	// Priority 2: Evict from main queue if it's over limit
	if c.main.Len() > c.mainLimit {
		return c.evictFromMain()
	}
	// Priority 3: If both queues within limits but total at capacity
	if c.Len() >= c.capacity {
		if c.main.Len() > 0 {
			return c.evictFromMain()
		} else if c.small.Len() > 0 {
			// Fallback: evict from small queue if main is empty
			return c.evictFromSmallToGhost()
		}
	}
	return false
}

// evictFromMain removes the oldest item from the main queue.
func (c *S3FIFOCache[K, V]) evictFromMain() bool {
	if c.main.Len() == 0 {
		return false
	}

	e := c.main.Front()
	entry := e.Value

	// Remove from main queue
	c.main.Remove(e)
	delete(c.cache, entry.key)
	// Clean up frequency map to prevent memory leak
	delete(c.freq, entry.key)

	// Call eviction callback
	if c.onEviction != nil {
		c.onEviction(base.EvictionReasonCapacity, entry.key, entry.value)
	}

	return true
}

// evictFromSmallToGhost removes the oldest item from small queue and moves it to ghost queue.
func (c *S3FIFOCache[K, V]) evictFromSmallToGhost() bool {
	if c.small.Len() == 0 {
		return false
	}

	e := c.small.Front()
	entry := e.Value

	// Remove from small queue and cache
	c.small.Remove(e)
	delete(c.cache, entry.key)

	// Add to ghost queue
	c.addToGhost(entry.key)

	// Call eviction callback
	if c.onEviction != nil {
		c.onEviction(base.EvictionReasonCapacity, entry.key, entry.value)
	}

	return true
}

// promoteToMain moves an item from small queue to main queue.
func (c *S3FIFOCache[K, V]) promoteToMain(e *list.Element[*entry[K, V]]) {
	entry := e.Value
	if entry.queue != 0 {
		return // Already in main queue
	}

	// Remove from small queue
	c.small.Remove(e)

	// Add to main queue
	entry.queue = 1
	newE := c.main.PushBack(entry)
	c.cache[entry.key] = newE
}

// checkGhost checks if a key is in ghost queue and handles frequency tracking.
func (c *S3FIFOCache[K, V]) checkGhost(key K) {
	// This is called on cache misses to track frequency of evicted items
	// All items in ghost queue came from small queue (per S3-FIFO paper)
	if _, exists := c.ghostMap[key]; exists {
		currentFreq, freqExists := c.freq[key]
		if !freqExists {
			currentFreq = 0
		}
		c.freq[key] = min(currentFreq+1, maxFrequency)

		// S3-FIFO paper: items stay in ghost queue to prevent thrashing
		// Only removed when ghost queue is full (FIFO eviction in addToGhost)
	}
}

// addToGhost adds a key to the ghost queue.
func (c *S3FIFOCache[K, V]) addToGhost(key K) {
	// Remove oldest ghost entry if at limit
	if c.ghost.Len() >= c.ghostLimit {
		if oldest := c.ghost.Front(); oldest != nil {
			c.ghost.Remove(oldest)
			delete(c.ghostMap, oldest.Value)
		}
	}
	e := c.ghost.PushBack(key)
	c.ghostMap[key] = e
}

// removeFromGhost removes a key from the ghost queue.
func (c *S3FIFOCache[K, V]) removeFromGhost(key K) {
	if e, exists := c.ghostMap[key]; exists {
		c.ghost.Remove(e)
		delete(c.ghostMap, key)
		// Note: Frequency is preserved in freq map for potential reinsertion
		// per S3-FIFO paper specification
	}
}

// isInGhost checks if a key exists in the ghost queue.
func (c *S3FIFOCache[K, V]) isInGhost(key K) bool {
	_, exists := c.ghostMap[key]
	return exists
}

// deleteElement removes an element from its queue and the cache map.
func (c *S3FIFOCache[K, V]) deleteElement(e *list.Element[*entry[K, V]]) {
	entry := e.Value

	if entry.queue == 0 {
		c.small.Remove(e)
	} else {
		c.main.Remove(e)
	}

	delete(c.cache, entry.key)
	// Keep frequency for potential future ghost hits
}
