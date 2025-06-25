package safe

import (
	"sync"
	"testing"

	"github.com/samber/hot/pkg/base"
	"github.com/samber/hot/pkg/lru"
	"github.com/stretchr/testify/assert"
)

// mockCache implements base.InMemoryCache for testing
type mockCache[K comparable, V any] struct {
	data map[K]V
}

func newMockCache[K comparable, V any]() *mockCache[K, V] {
	return &mockCache[K, V]{
		data: make(map[K]V),
	}
}

func (m *mockCache[K, V]) Set(key K, value V) {
	m.data[key] = value
}

func (m *mockCache[K, V]) Has(key K) bool {
	_, ok := m.data[key]
	return ok
}

func (m *mockCache[K, V]) Get(key K) (V, bool) {
	value, ok := m.data[key]
	return value, ok
}

func (m *mockCache[K, V]) Peek(key K) (V, bool) {
	value, ok := m.data[key]
	return value, ok
}

func (m *mockCache[K, V]) Keys() []K {
	keys := make([]K, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys
}

func (m *mockCache[K, V]) Values() []V {
	values := make([]V, 0, len(m.data))
	for _, v := range m.data {
		values = append(values, v)
	}
	return values
}

func (m *mockCache[K, V]) Range(f func(K, V) bool) {
	for k, v := range m.data {
		if !f(k, v) {
			break
		}
	}
}

func (m *mockCache[K, V]) Delete(key K) bool {
	if _, ok := m.data[key]; ok {
		delete(m.data, key)
		return true
	}
	return false
}

func (m *mockCache[K, V]) Purge() {
	m.data = make(map[K]V)
}

func (m *mockCache[K, V]) SetMany(items map[K]V) {
	for k, v := range items {
		m.data[k] = v
	}
}

func (m *mockCache[K, V]) HasMany(keys []K) map[K]bool {
	result := make(map[K]bool)
	for _, k := range keys {
		result[k] = m.Has(k)
	}
	return result
}

func (m *mockCache[K, V]) GetMany(keys []K) (map[K]V, []K) {
	found := make(map[K]V)
	missing := make([]K, 0)
	for _, k := range keys {
		if v, ok := m.data[k]; ok {
			found[k] = v
		} else {
			missing = append(missing, k)
		}
	}
	return found, missing
}

func (m *mockCache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
	found := make(map[K]V)
	missing := make([]K, 0)
	for _, k := range keys {
		if v, ok := m.data[k]; ok {
			found[k] = v
		} else {
			missing = append(missing, k)
		}
	}
	return found, missing
}

func (m *mockCache[K, V]) DeleteMany(keys []K) map[K]bool {
	result := make(map[K]bool)
	for _, k := range keys {
		result[k] = m.Delete(k)
	}
	return result
}

func (m *mockCache[K, V]) Capacity() int {
	return 1000
}

func (m *mockCache[K, V]) Algorithm() string {
	return "mock"
}

func (m *mockCache[K, V]) Len() int {
	return len(m.data)
}

func TestNewSafeInMemoryCache(t *testing.T) {
	is := assert.New(t)

	mock := newMockCache[string, int]()
	safeCache := NewSafeInMemoryCache(mock)

	is.NotNil(safeCache)
	is.Implements((*base.InMemoryCache[string, int])(nil), safeCache)
}

func TestSafeInMemoryCache_BasicOperations(t *testing.T) {
	is := assert.New(t)

	mock := newMockCache[string, int]()
	cache := NewSafeInMemoryCache(mock)

	// Test Set and Get
	cache.Set("key1", 100)
	value, ok := cache.Get("key1")
	is.True(ok)
	is.Equal(100, value)

	// Test Has
	is.True(cache.Has("key1"))
	is.False(cache.Has("key2"))

	// Test Peek
	value, ok = cache.Peek("key1")
	is.True(ok)
	is.Equal(100, value)

	// Test Delete
	is.True(cache.Delete("key1"))
	is.False(cache.Delete("key1")) // already deleted

	// Test Len
	is.Equal(0, cache.Len())
}

func TestSafeInMemoryCache_BatchOperations(t *testing.T) {
	is := assert.New(t)

	mock := newMockCache[string, int]()
	cache := NewSafeInMemoryCache(mock)

	// Test SetMany
	items := map[string]int{
		"key1": 100,
		"key2": 200,
		"key3": 300,
	}
	cache.SetMany(items)

	// Test HasMany
	keys := []string{"key1", "key2", "key4"}
	hasResults := cache.HasMany(keys)
	is.True(hasResults["key1"])
	is.True(hasResults["key2"])
	is.False(hasResults["key4"])

	// Test GetMany
	found, missing := cache.GetMany(keys)
	is.Len(found, 2)
	is.Len(missing, 1)
	is.Equal(100, found["key1"])
	is.Equal(200, found["key2"])
	is.Equal("key4", missing[0])

	// Test PeekMany
	found, missing = cache.PeekMany(keys)
	is.Len(found, 2)
	is.Len(missing, 1)

	// Test DeleteMany
	deleteKeys := []string{"key1", "key2", "key4"}
	deleteResults := cache.DeleteMany(deleteKeys)
	is.True(deleteResults["key1"])
	is.True(deleteResults["key2"])
	is.False(deleteResults["key4"])
}

func TestSafeInMemoryCache_KeysAndValues(t *testing.T) {
	is := assert.New(t)

	mock := newMockCache[string, int]()
	cache := NewSafeInMemoryCache(mock)

	cache.Set("key1", 100)
	cache.Set("key2", 200)

	keys := cache.Keys()
	is.Len(keys, 2)
	is.Contains(keys, "key1")
	is.Contains(keys, "key2")

	values := cache.Values()
	is.Len(values, 2)
	is.Contains(values, 100)
	is.Contains(values, 200)
}

func TestSafeInMemoryCache_Range(t *testing.T) {
	is := assert.New(t)

	mock := newMockCache[string, int]()
	cache := NewSafeInMemoryCache(mock)

	cache.Set("key1", 100)
	cache.Set("key2", 200)

	visited := make(map[string]int)
	cache.Range(func(k string, v int) bool {
		visited[k] = v
		return true
	})

	is.Len(visited, 2)
	is.Equal(100, visited["key1"])
	is.Equal(200, visited["key2"])
}

func TestSafeInMemoryCache_RangeEarlyExit(t *testing.T) {
	is := assert.New(t)

	mock := newMockCache[string, int]()
	cache := NewSafeInMemoryCache(mock)

	cache.Set("key1", 100)
	cache.Set("key2", 200)
	cache.Set("key3", 300)

	visited := make(map[string]int)
	cache.Range(func(k string, v int) bool {
		visited[k] = v
		return k != "key2" // stop at key2
	})

	is.Len(visited, 2) // should only have key1 and key2
}

func TestSafeInMemoryCache_Purge(t *testing.T) {
	is := assert.New(t)

	mock := newMockCache[string, int]()
	cache := NewSafeInMemoryCache(mock)

	cache.Set("key1", 100)
	cache.Set("key2", 200)

	is.Equal(2, cache.Len())

	cache.Purge()

	is.Equal(0, cache.Len())
	is.False(cache.Has("key1"))
	is.False(cache.Has("key2"))
}

func TestSafeInMemoryCache_EmptyOperations(t *testing.T) {
	is := assert.New(t)

	mock := newMockCache[string, int]()
	cache := NewSafeInMemoryCache(mock)

	// Test empty SetMany
	cache.SetMany(map[string]int{})
	is.Equal(0, cache.Len())

	// Test empty HasMany
	hasResults := cache.HasMany([]string{})
	is.Len(hasResults, 0)

	// Test empty GetMany
	found, missing := cache.GetMany([]string{})
	is.Len(found, 0)
	is.Len(missing, 0)

	// Test empty PeekMany
	found, missing = cache.PeekMany([]string{})
	is.Len(found, 0)
	is.Len(missing, 0)

	// Test empty DeleteMany
	deleteResults := cache.DeleteMany([]string{})
	is.Len(deleteResults, 0)
}

func TestSafeInMemoryCache_ConcurrentAccess(t *testing.T) {
	is := assert.New(t)

	mock := newMockCache[int, int]()
	cache := NewSafeInMemoryCache(mock)

	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup

	// Test concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := id*numOperations + j
				cache.Set(key, key*2)
			}
		}(i)
	}

	// Test concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := id*numOperations + j
				cache.Has(key)
			}
		}(i)
	}

	wg.Wait()

	// Verify all values were written correctly
	for i := 0; i < numGoroutines*numOperations; i++ {
		value, ok := cache.Get(i)
		is.True(ok)
		is.Equal(i*2, value)
	}
}

func TestSafeInMemoryCache_WithRealLRU(t *testing.T) {
	is := assert.New(t)

	lruCache := lru.NewLRUCache[string, int](100)
	safeCache := NewSafeInMemoryCache(lruCache)

	// Test basic operations with real LRU
	safeCache.Set("key1", 100)
	value, ok := safeCache.Get("key1")
	is.True(ok)
	is.Equal(100, value)

	// Test capacity and algorithm
	is.Equal(100, safeCache.Capacity())
	is.Equal("lru", safeCache.Algorithm())
}

func TestSafeInMemoryCache_InterfaceCompliance(t *testing.T) {
	is := assert.New(t)

	mock := newMockCache[string, int]()
	safeCache := NewSafeInMemoryCache(mock)

	// Verify the safe cache implements the interface
	var _ base.InMemoryCache[string, int] = safeCache

	// Test that we can assign it to the interface type
	var cache base.InMemoryCache[string, int] = safeCache

	// Test operations through the interface
	cache.Set("test", 42)
	value, ok := cache.Get("test")
	is.True(ok)
	is.Equal(42, value)
}
