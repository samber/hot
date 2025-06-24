package base

// InMemoryCache represents a cache abstraction for in-memory storage.
// The setters and getters do not return errors because the cache is in-memory.
// Remote caches such as Redis, Memcached, etc. should be implemented as a chain of loaders.
type InMemoryCache[K comparable, V any] interface {
	// Set stores a key-value pair in the cache.
	Set(key K, value V)

	// Has checks if a key exists in the cache.
	Has(key K) bool

	// Get retrieves a value from the cache.
	// Returns the value and a boolean indicating if the key was found.
	Get(key K) (V, bool)

	// Peek retrieves a value from the cache without updating access order.
	// Returns the value and a boolean indicating if the key was found.
	Peek(key K) (V, bool)

	// Keys returns all keys currently in the cache.
	Keys() []K

	// Values returns all values currently in the cache.
	Values() []V

	// Range iterates over all key-value pairs in the cache.
	// The iteration stops if the function returns false.
	Range(f func(K, V) bool)

	// Delete removes a key from the cache.
	// Returns true if the key was found and removed, false otherwise.
	Delete(key K) bool

	// Purge removes all keys and values from the cache.
	Purge()

	// Batch operations for better performance

	// SetMany stores multiple key-value pairs in the cache.
	SetMany(items map[K]V)

	// HasMany checks if multiple keys exist in the cache.
	// Returns a map where keys are the input keys and values indicate existence.
	HasMany(keys []K) map[K]bool

	// GetMany retrieves multiple values from the cache.
	// Returns a map of found key-value pairs and a slice of missing keys.
	GetMany(keys []K) (map[K]V, []K)

	// PeekMany retrieves multiple values from the cache without updating access order.
	// Returns a map of found key-value pairs and a slice of missing keys.
	PeekMany(keys []K) (map[K]V, []K)

	// DeleteMany removes multiple keys from the cache.
	// Returns a map where keys are the input keys and values indicate if the key was found and removed.
	DeleteMany(keys []K) map[K]bool

	// Statistics and metadata

	// Capacity returns the maximum number of items the cache can hold.
	Capacity() int

	// Algorithm returns the name of the eviction algorithm used by the cache.
	Algorithm() string

	// Len returns the current number of items in the cache.
	Len() int
}
