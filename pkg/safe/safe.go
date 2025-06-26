package safe

import (
	"sync"

	"github.com/samber/hot/pkg/base"
)

// NewSafeInMemoryCache creates a thread-safe wrapper around an existing cache implementation.
// This function wraps any cache that implements the InMemoryCache interface with read-write mutex
// protection to ensure safe concurrent access from multiple goroutines.
func NewSafeInMemoryCache[K comparable, V any](cache base.InMemoryCache[K, V]) base.InMemoryCache[K, V] {
	return &SafeInMemoryCache[K, V]{
		InMemoryCache: cache,
		RWMutex:       sync.RWMutex{},
	}
}

// SafeInMemoryCache is a thread-safe wrapper around any cache implementation.
// It uses a read-write mutex to protect all cache operations, allowing multiple
// concurrent readers but only one writer at a time.
type SafeInMemoryCache[K comparable, V any] struct {
	base.InMemoryCache[K, V] // Embedded cache implementation
	sync.RWMutex             // Read-write mutex for thread safety
}

// Ensure SafeInMemoryCache implements InMemoryCache interface
var _ base.InMemoryCache[string, int] = (*SafeInMemoryCache[string, int])(nil)

// Set stores a key-value pair in the cache with exclusive write lock.
// This operation blocks other writers and readers until completion.
func (c *SafeInMemoryCache[K, V]) Set(key K, value V) {
	c.Lock()
	c.InMemoryCache.Set(key, value)
	c.Unlock()
}

// Has checks if a key exists in the cache using a shared read lock.
// Multiple readers can access this method concurrently.
func (c *SafeInMemoryCache[K, V]) Has(key K) bool {
	c.RLock()
	defer c.RUnlock()
	return c.InMemoryCache.Has(key)
}

// Get retrieves a value from the cache and makes it the most recently used item.
// Uses an exclusive write lock because the underlying cache may modify access order
// (e.g., move items in LRU cache), which requires write access.
func (c *SafeInMemoryCache[K, V]) Get(key K) (value V, ok bool) {
	// not read-only lock, because underlying cache may change the item
	c.Lock()
	defer c.Unlock()
	return c.InMemoryCache.Get(key)
}

// Peek retrieves a value from the cache without updating access order.
// Uses a shared read lock since no modifications are made to the cache structure.
func (c *SafeInMemoryCache[K, V]) Peek(key K) (value V, ok bool) {
	c.RLock()
	defer c.RUnlock()
	return c.InMemoryCache.Peek(key)
}

// Keys returns all keys currently in the cache using a shared read lock.
// The returned slice is a snapshot and may not reflect concurrent modifications.
func (c *SafeInMemoryCache[K, V]) Keys() []K {
	c.RLock()
	defer c.RUnlock()
	return c.InMemoryCache.Keys()
}

// Values returns all values currently in the cache using a shared read lock.
// The returned slice is a snapshot and may not reflect concurrent modifications.
func (c *SafeInMemoryCache[K, V]) Values() []V {
	c.RLock()
	defer c.RUnlock()
	return c.InMemoryCache.Values()
}

// Range iterates over all key-value pairs in the cache using a shared read lock.
// The iteration is performed under lock protection to ensure consistency.
// The iteration stops if the function returns false.
func (c *SafeInMemoryCache[K, V]) Range(f func(K, V) bool) {
	c.RLock()
	c.InMemoryCache.Range(f)
	c.RUnlock()
}

// Delete removes a key from the cache using an exclusive write lock.
// Returns true if the key was found and removed, false otherwise.
func (c *SafeInMemoryCache[K, V]) Delete(key K) bool {
	c.Lock()
	defer c.Unlock()
	return c.InMemoryCache.Delete(key)
}

// Purge removes all keys and values from the cache using an exclusive write lock.
// This operation completely clears the cache and blocks all other operations.
func (c *SafeInMemoryCache[K, V]) Purge() {
	c.Lock()
	c.InMemoryCache.Purge()
	c.Unlock()
}

// SetMany stores multiple key-value pairs in the cache using an exclusive write lock.
// This is more efficient than calling Set multiple times as it uses a single lock acquisition.
// If the input map is empty, the operation is skipped entirely.
func (c *SafeInMemoryCache[K, V]) SetMany(items map[K]V) {
	if len(items) == 0 {
		return
	}

	c.Lock()
	c.InMemoryCache.SetMany(items)
	c.Unlock()
}

// HasMany checks if multiple keys exist in the cache using a shared read lock.
// Returns a map where keys are the input keys and values indicate existence.
// If the input slice is empty, returns an empty map immediately.
func (c *SafeInMemoryCache[K, V]) HasMany(keys []K) map[K]bool {
	if len(keys) == 0 {
		return map[K]bool{}
	}

	c.RLock()
	defer c.RUnlock()
	return c.InMemoryCache.HasMany(keys)
}

// GetMany retrieves multiple values from the cache using an exclusive write lock.
// Returns a map of found key-value pairs and a slice of missing keys.
// Uses write lock because underlying cache may modify access order.
// If the input slice is empty, returns empty results immediately.
func (c *SafeInMemoryCache[K, V]) GetMany(keys []K) (map[K]V, []K) {
	if len(keys) == 0 {
		return map[K]V{}, []K{}
	}

	c.Lock()
	defer c.Unlock()
	return c.InMemoryCache.GetMany(keys)
}

// PeekMany retrieves multiple values from the cache without updating access order.
// Uses a shared read lock since no modifications are made to the cache structure.
// Returns a map of found key-value pairs and a slice of missing keys.
// If the input slice is empty, returns empty results immediately.
func (c *SafeInMemoryCache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
	if len(keys) == 0 {
		return map[K]V{}, []K{}
	}

	c.RLock()
	defer c.RUnlock()
	return c.InMemoryCache.PeekMany(keys)
}

// DeleteMany removes multiple keys from the cache using an exclusive write lock.
// Returns a map where keys are the input keys and values indicate if the key was found and removed.
// If the input slice is empty, returns an empty map immediately.
func (c *SafeInMemoryCache[K, V]) DeleteMany(keys []K) map[K]bool {
	if len(keys) == 0 {
		return map[K]bool{}
	}

	c.Lock()
	defer c.Unlock()
	return c.InMemoryCache.DeleteMany(keys)
}

// Capacity returns the maximum number of items the cache can hold.
// This is a read-only operation that doesn't require locking as capacity is immutable.
func (c *SafeInMemoryCache[K, V]) Capacity() int {
	return c.InMemoryCache.Capacity()
}

// Algorithm returns the name of the eviction algorithm used by the cache.
// This is a read-only operation that doesn't require locking as algorithm is immutable.
func (c *SafeInMemoryCache[K, V]) Algorithm() string {
	return c.InMemoryCache.Algorithm()
}

// Len returns the current number of items in the cache using a shared read lock.
// The count is accurate at the time of the lock acquisition.
func (c *SafeInMemoryCache[K, V]) Len() int {
	c.RLock()
	defer c.RUnlock()
	return c.InMemoryCache.Len()
}

// SizeBytes returns the total size of all cache entries in bytes using a shared read lock.
// The size is accurate at the time of the lock acquisition.
func (c *SafeInMemoryCache[K, V]) SizeBytes() int64 {
	c.RLock()
	defer c.RUnlock()
	return c.InMemoryCache.SizeBytes()
}
