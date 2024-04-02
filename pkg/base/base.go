package base

// InMemoryCache represents a cache abstraction.
// The setters and getters does not return error, because the cache is in-memory. Remote cache
// such as Redis, Memcached, etc. should be implemented in a chain of loaders.
type InMemoryCache[K comparable, V any] interface {
	Set(key K, value V)
	Has(key K) bool
	Get(key K) (V, bool)
	Peek(key K) (V, bool)
	Keys() []K
	Values() []V
	Range(f func(K, V) bool)
	Delete(key K) bool
	Purge()

	// batching
	SetMany(items map[K]V)
	HasMany(keys []K) map[K]bool
	GetMany(keys []K) (map[K]V, []K)
	PeekMany(keys []K) (map[K]V, []K)
	DeleteMany(keys []K) map[K]bool

	// stats
	Capacity() int
	Algorithm() string
	Len() int
}
