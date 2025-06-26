package metrics

import (
	"github.com/samber/hot/pkg/base"
)

var _ base.InMemoryCache[string, int] = (*InstrumentedCache[string, int])(nil)

// NewInstrumentedCache creates a new metrics wrapper around an existing cache.
func NewInstrumentedCache[K comparable, V any](cache base.InMemoryCache[K, V], metrics Collector) *InstrumentedCache[K, V] {
	return &InstrumentedCache[K, V]{
		cache:    cache,
		metrics:  metrics,
		capacity: cache.Capacity(),
	}
}

// InstrumentedCache wraps any InMemoryCache implementation and adds metrics collection.
// It implements the InMemoryCache interface and delegates all operations to the underlying cache
// while tracking metrics for insertions, evictions, hits, and misses.
type InstrumentedCache[K comparable, V any] struct {
	cache    base.InMemoryCache[K, V]
	metrics  Collector
	capacity int
}

// Set stores a key-value pair in the cache and tracks insertion metrics.
func (m *InstrumentedCache[K, V]) Set(key K, value V) {
	m.cache.Set(key, value)
	m.metrics.IncInsertion()
}

// SetMany stores multiple key-value pairs in the cache and tracks insertion metrics.
func (m *InstrumentedCache[K, V]) SetMany(items map[K]V) {
	m.cache.SetMany(items)
	m.metrics.AddInsertions(int64(len(items)))
}

// Get retrieves a value from the cache and tracks hit/miss metrics.
func (m *InstrumentedCache[K, V]) Get(key K) (V, bool) {
	value, found := m.cache.Get(key)
	if found {
		m.metrics.IncHit()
	} else {
		m.metrics.IncMiss()
	}
	return value, found
}

// GetMany retrieves multiple values from the cache and tracks hit/miss metrics.
func (m *InstrumentedCache[K, V]) GetMany(keys []K) (map[K]V, []K) {
	values, missing := m.cache.GetMany(keys)

	// Count hits and misses
	hits := len(values)
	misses := len(missing)

	if hits > 0 {
		m.metrics.AddHits(int64(hits))
	}
	if misses > 0 {
		m.metrics.AddMisses(int64(misses))
	}

	return values, missing
}

// Delete removes a key from the cache and tracks eviction metrics.
func (m *InstrumentedCache[K, V]) Delete(key K) bool {
	deleted := m.cache.Delete(key)
	if deleted {
		m.metrics.IncEviction(base.EvictionReasonManual)
	}
	return deleted
}

// DeleteMany removes multiple keys from the cache and tracks eviction metrics.
func (m *InstrumentedCache[K, V]) DeleteMany(keys []K) map[K]bool {
	deleted := m.cache.DeleteMany(keys)

	// Count total deletions
	totalDeleted := 0
	for _, wasDeleted := range deleted {
		if wasDeleted {
			totalDeleted++
		}
	}

	if totalDeleted > 0 {
		m.metrics.AddEvictions(base.EvictionReasonManual, int64(totalDeleted))
	}

	return deleted
}

// Has checks if a key exists in the cache and tracks hit/miss metrics.
func (m *InstrumentedCache[K, V]) Has(key K) bool {
	has := m.cache.Has(key)
	if has {
		m.metrics.IncHit()
	} else {
		m.metrics.IncMiss()
	}
	return has
}

// HasMany checks if multiple keys exist in the cache and tracks hit/miss metrics.
func (m *InstrumentedCache[K, V]) HasMany(keys []K) map[K]bool {
	results := m.cache.HasMany(keys)

	// Count hits and misses
	hits := 0
	misses := 0
	for _, exists := range results {
		if exists {
			hits++
		} else {
			misses++
		}
	}

	if hits > 0 {
		m.metrics.AddHits(int64(hits))
	}
	if misses > 0 {
		m.metrics.AddMisses(int64(misses))
	}

	return results
}

// Keys returns all keys currently in the cache.
func (m *InstrumentedCache[K, V]) Keys() []K {
	return m.cache.Keys()
}

// Values returns all values currently in the cache.
func (m *InstrumentedCache[K, V]) Values() []V {
	return m.cache.Values()
}

// Range iterates over all key-value pairs in the cache.
func (m *InstrumentedCache[K, V]) Range(f func(K, V) bool) {
	m.cache.Range(f)
}

// Len returns the number of items in the cache.
func (m *InstrumentedCache[K, V]) Len() int {
	return m.cache.Len()
}

// Capacity returns the capacity of the cache.
func (m *InstrumentedCache[K, V]) Capacity() int {
	return m.cache.Capacity()
}

// Algorithm returns the eviction algorithm name.
func (m *InstrumentedCache[K, V]) Algorithm() string {
	return m.cache.Algorithm()
}

// Purge removes all items from the cache and tracks eviction metrics.
func (m *InstrumentedCache[K, V]) Purge() {
	// Count items before purging for metrics
	itemCount := m.cache.Len()

	m.cache.Purge()

	if itemCount > 0 {
		m.metrics.AddEvictions(base.EvictionReasonManual, int64(itemCount))
	}
}

// Peek retrieves a value from the cache without updating access order and tracks hit/miss metrics.
func (m *InstrumentedCache[K, V]) Peek(key K) (V, bool) {
	value, found := m.cache.Peek(key)
	if found {
		m.metrics.IncHit()
	} else {
		m.metrics.IncMiss()
	}
	return value, found
}

// PeekMany retrieves multiple values from the cache without updating access order and tracks hit/miss metrics.
func (m *InstrumentedCache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
	values, missing := m.cache.PeekMany(keys)

	// Count hits and misses
	hits := len(values)
	misses := len(missing)

	if hits > 0 {
		m.metrics.AddHits(int64(hits))
	}
	if misses > 0 {
		m.metrics.AddMisses(int64(misses))
	}

	return values, missing
}
