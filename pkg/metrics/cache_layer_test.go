package metrics

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/samber/hot/pkg/base"
	"github.com/samber/hot/pkg/lru"
	"github.com/samber/hot/pkg/safe"
	"github.com/stretchr/testify/assert"
)

// MockCollector is a test implementation of Collector that tracks method calls.
type MockCollector struct {
	mu sync.Mutex

	// Method call counters
	insertionCount int64
	evictionCount  map[string]int64
	hitCount       int64
	missCount      int64
	sizeBytes      int64
	length         int64

	// Optional callback functions for testing
	updateLengthFn func(int64)
	updateSizeFn   func(int64)
}

func (m *MockCollector) IncInsertion() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.insertionCount++
}

func (m *MockCollector) AddInsertions(count int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.insertionCount += count
}

func (m *MockCollector) IncEviction(reason base.EvictionReason) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.evictionCount == nil {
		m.evictionCount = make(map[string]int64)
	}
	m.evictionCount[string(reason)]++
}

func (m *MockCollector) AddEvictions(reason base.EvictionReason, count int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.evictionCount == nil {
		m.evictionCount = make(map[string]int64)
	}
	m.evictionCount[string(reason)] += count
}

func (m *MockCollector) IncHit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hitCount++
}

func (m *MockCollector) AddHits(count int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hitCount += count
}

func (m *MockCollector) IncMiss() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.missCount++
}

func (m *MockCollector) AddMisses(count int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.missCount += count
}

func (m *MockCollector) UpdateSizeBytes(sizeBytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sizeBytes = sizeBytes
	if m.updateSizeFn != nil {
		m.updateSizeFn(sizeBytes)
	}
}

func (m *MockCollector) UpdateLength(length int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.length = length
	if m.updateLengthFn != nil {
		m.updateLengthFn(length)
	}
}

func TestInstrumentedCache_BasicOperations(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Create underlying cache
	underlyingCache := lru.NewLRUCache[string, int](10)

	// Create metrics collector
	collector := NewPrometheusCollector("test-cache", -1, base.CacheModeMain, 10, "lru", nil, nil, nil, nil, nil)

	// Create metrics wrapper
	metricsCache := NewInstrumentedCache(underlyingCache, collector)

	// Test Set and Get
	metricsCache.Set("key1", 100)
	value, found := metricsCache.Get("key1")
	is.True(found)
	is.Equal(100, value)

	// Test Get with non-existent key
	_, found = metricsCache.Get("key2")
	is.False(found)

	// Test SetMany and GetMany
	items := map[string]int{
		"key3": 300,
		"key4": 400,
		"key5": 500,
	}
	metricsCache.SetMany(items)

	values, missing := metricsCache.GetMany([]string{"key3", "key4", "key6"})
	is.Len(values, 2)
	is.Len(missing, 1)
	is.Equal(300, values["key3"])
	is.Equal(400, values["key4"])
	is.Equal("key6", missing[0])

	// Test Has
	is.True(metricsCache.Has("key1"))
	is.False(metricsCache.Has("key6"))

	// Test HasMany
	hasResults := metricsCache.HasMany([]string{"key1", "key3", "key6", "key7"})
	is.True(hasResults["key1"])
	is.True(hasResults["key3"])
	is.False(hasResults["key6"])
	is.False(hasResults["key7"])

	// Test Peek (should not update access order)
	value, found = metricsCache.Peek("key1")
	is.True(found)
	is.Equal(100, value)

	// Test PeekMany
	peekValues, peekMissing := metricsCache.PeekMany([]string{"key1", "key3", "key6"})
	is.Len(peekValues, 2)
	is.Len(peekMissing, 1)
	is.Equal(100, peekValues["key1"])
	is.Equal(300, peekValues["key3"])

	// Test Keys and Values
	keys := metricsCache.Keys()
	is.Len(keys, 4) // key1, key3, key4, key5 (key1 might be evicted due to capacity)

	valuesList := metricsCache.Values()
	is.Len(valuesList, 4)

	// Test Delete
	deleted := metricsCache.Delete("key1")
	is.True(deleted)
	is.False(metricsCache.Has("key1"))

	// Test DeleteMany
	deletedMap := metricsCache.DeleteMany([]string{"key3", "key4", "key7"})
	is.True(deletedMap["key3"])
	is.True(deletedMap["key4"])
	is.False(deletedMap["key7"])

	// Test Len and Capacity
	is.Equal(1, metricsCache.Len()) // Only key5 should remain
	is.Equal(10, metricsCache.Capacity())

	// Test Algorithm
	is.Equal("lru", metricsCache.Algorithm())

	// Test Purge
	metricsCache.Purge()
	is.Equal(0, metricsCache.Len())
}

func TestInstrumentedCache_MetricsTracking(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Create underlying cache
	underlyingCache := lru.NewLRUCache[string, int](5)

	// Create metrics collector
	collector := NewPrometheusCollector("test-cache", -1, base.CacheModeMain, 5, "lru", nil, nil, nil, nil, nil)

	// Create metrics wrapper
	metricsCache := NewInstrumentedCache(underlyingCache, collector)

	// Test insertion metrics
	metricsCache.Set("key1", 100)
	metricsCache.Set("key2", 200)

	items := map[string]int{
		"key3": 300,
		"key4": 400,
	}
	metricsCache.SetMany(items)

	// Test hit/miss metrics
	metricsCache.Get("key1")  // hit
	metricsCache.Get("key2")  // hit
	metricsCache.Get("key5")  // miss
	metricsCache.Has("key3")  // hit
	metricsCache.Has("key6")  // miss
	metricsCache.Peek("key1") // hit
	metricsCache.Peek("key7") // miss

	// Test eviction metrics
	metricsCache.Set("key7", 700) // This should evict key1 due to capacity
	metricsCache.Delete("key2")
	metricsCache.DeleteMany([]string{"key3", "key8"}) // key8 doesn't exist

	// Verify metrics are being tracked
	// Note: In a real test, you would collect the metrics and verify the values
	// For now, we just verify the cache operations work correctly
	// After evictions and deletions, we should have key4, key5, and key7 (3 items)
	is.Equal(3, metricsCache.Len())
}

func TestInstrumentedCache_NoOpMetrics(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Create underlying cache
	underlyingCache := lru.NewLRUCache[string, int](10)

	// Create no-op metrics collector
	collector := &NoOpCollector{}

	// Create metrics wrapper
	metricsCache := NewInstrumentedCache(underlyingCache, collector)

	// Test that operations work correctly even with no-op metrics
	metricsCache.Set("key1", 100)
	value, found := metricsCache.Get("key1")
	is.True(found)
	is.Equal(100, value)

	// Test that we can access the underlying cache and metrics
	is.Equal(underlyingCache, metricsCache.cache)
	is.Equal(collector, metricsCache.metrics)
}

func TestInstrumentedCache_EdgeCases(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Create underlying cache
	underlyingCache := lru.NewLRUCache[string, int](3)

	// Create metrics collector
	collector := NewPrometheusCollector("test-cache", -1, base.CacheModeMain, 3, "lru", nil, nil, nil, nil, nil)

	// Create metrics wrapper
	metricsCache := NewInstrumentedCache(underlyingCache, collector)

	// Test empty operations
	_, found := metricsCache.Get("nonexistent")
	is.False(found)

	values, missing := metricsCache.GetMany([]string{})
	is.Empty(values)
	is.Empty(missing)

	hasResults := metricsCache.HasMany([]string{})
	is.Empty(hasResults)

	peekValues, peekMissing := metricsCache.PeekMany([]string{})
	is.Empty(peekValues)
	is.Empty(peekMissing)

	// Test operations on empty cache
	is.Equal(0, metricsCache.Len())
	is.Equal(3, metricsCache.Capacity())
	is.Equal("lru", metricsCache.Algorithm())

	// Test Range on empty cache
	count := 0
	metricsCache.Range(func(k string, v int) bool {
		count++
		return true
	})
	is.Equal(0, count)

	// Test Keys and Values on empty cache
	keys := metricsCache.Keys()
	is.Empty(keys)

	valuesList := metricsCache.Values()
	is.Empty(valuesList)

	// Test DeleteMany with empty slice
	deletedMap := metricsCache.DeleteMany([]string{})
	is.Empty(deletedMap)

	// Test SetMany with empty map
	metricsCache.SetMany(map[string]int{})
	is.Equal(0, metricsCache.Len())
}

func TestInstrumentedCache_CapacityAndEviction(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Create underlying cache with small capacity
	underlyingCache := lru.NewLRUCache[string, int](2)

	// Create metrics collector
	collector := NewPrometheusCollector("test-cache", -1, base.CacheModeMain, 2, "lru", nil, nil, nil, nil, nil)

	// Create metrics wrapper
	metricsCache := NewInstrumentedCache(underlyingCache, collector)

	// Fill the cache
	metricsCache.Set("key1", 100)
	metricsCache.Set("key2", 200)
	is.Equal(2, metricsCache.Len())

	// Add one more item - should evict key1 (LRU)
	metricsCache.Set("key3", 300)
	is.Equal(2, metricsCache.Len())

	// key1 should be evicted
	is.False(metricsCache.Has("key1"))
	is.True(metricsCache.Has("key2"))
	is.True(metricsCache.Has("key3"))

	// Test bulk operations with eviction
	metricsCache.SetMany(map[string]int{
		"key4": 400,
		"key5": 500,
	})
	is.Equal(2, metricsCache.Len()) // Only key4 and key5 should remain

	// Test Purge
	metricsCache.Purge()
	is.Equal(0, metricsCache.Len())
}

func TestInstrumentedCache_UpdateSizeBytes(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Create underlying cache
	underlyingCache := lru.NewLRUCache[string, int](10)

	// Create metrics collector
	collector := NewPrometheusCollector("test-cache", -1, base.CacheModeMain, 10, "lru", nil, nil, nil, nil, nil)

	// Create metrics wrapper
	metricsCache := NewInstrumentedCache(underlyingCache, collector)

	// Add some items
	metricsCache.Set("key1", 100)
	metricsCache.Set("key2", 200)
	metricsCache.Set("key3", 300)

	// Verify the cache still works correctly
	is.Equal(3, metricsCache.Len())
	value, found := metricsCache.Get("key1")
	is.True(found)
	is.Equal(100, value)
}

func TestInstrumentedCache_WithPrometheusCollector(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Create underlying cache
	underlyingCache := lru.NewLRUCache[string, int](5)

	// Create Prometheus collector directly
	collector := NewPrometheusCollector("test-cache", -1, base.CacheModeMain, 5, "lru", nil, nil, nil, nil, nil)

	// Create metrics wrapper
	metricsCache := NewInstrumentedCache(underlyingCache, collector)

	// Test basic operations
	metricsCache.Set("key1", 100)
	value, found := metricsCache.Get("key1")
	is.True(found)
	is.Equal(100, value)

	// Test that metrics are being tracked
	// Note: We can't easily verify the actual metric values without Prometheus registry
	// But we can verify the operations work correctly
	metricsCache.Set("key2", 200)
	metricsCache.Get("key2") // hit
	metricsCache.Get("key3") // miss

	is.Equal(2, metricsCache.Len())
}

func TestInstrumentedCache_WithNoOpCollector(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Create underlying cache
	underlyingCache := lru.NewLRUCache[string, int](5)

	// Create NoOp collector
	collector := &NoOpCollector{}

	// Create metrics wrapper
	metricsCache := NewInstrumentedCache(underlyingCache, collector)

	// Test basic operations
	metricsCache.Set("key1", 100)
	value, found := metricsCache.Get("key1")
	is.True(found)
	is.Equal(100, value)

	// Test that operations work correctly with NoOp metrics
	metricsCache.Set("key2", 200)
	metricsCache.Get("key2") // hit
	metricsCache.Get("key3") // miss

	is.Equal(2, metricsCache.Len())
}

func TestInstrumentedCache_ConcurrentAccess(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Create underlying cache with thread-safe wrapper
	underlyingCache := safe.NewSafeInMemoryCache(lru.NewLRUCache[string, int](100))

	// Create metrics collector
	collector := NewPrometheusCollector("test-cache", -1, base.CacheModeMain, 100, "lru", nil, nil, nil, nil, nil)

	// Create metrics wrapper
	metricsCache := NewInstrumentedCache(underlyingCache, collector)

	// Test concurrent access
	const numGoroutines = 10
	const operationsPerGoroutine = 100

	var wg sync.WaitGroup

	// Concurrent Set operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				metricsCache.Set(key, j)
			}
		}(i)
	}

	// Concurrent Get operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				metricsCache.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// Verify the cache still works correctly
	is.Positive(metricsCache.Len())
}

func TestInstrumentedCache_EvictionMetrics(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Create underlying cache with small capacity to trigger evictions
	underlyingCache := lru.NewLRUCache[string, int](2)

	// Create metrics collector
	collector := NewPrometheusCollector("test-cache", -1, base.CacheModeMain, 2, "lru", nil, nil, nil, nil, nil)

	// Create metrics wrapper
	metricsCache := NewInstrumentedCache(underlyingCache, collector)

	// Fill the cache to trigger evictions
	metricsCache.Set("key1", 100)
	metricsCache.Set("key2", 200)
	metricsCache.Set("key3", 300) // This should evict key1

	// Verify eviction occurred
	is.Equal(2, metricsCache.Len())
	is.False(metricsCache.Has("key1"))
	is.True(metricsCache.Has("key2"))
	is.True(metricsCache.Has("key3"))

	// Test manual deletion
	metricsCache.Delete("key2")
	is.Equal(1, metricsCache.Len())
	is.False(metricsCache.Has("key2"))
}

func TestInstrumentedCache_BulkOperations(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Create underlying cache
	underlyingCache := lru.NewLRUCache[string, int](10)

	// Create metrics collector
	collector := NewPrometheusCollector("test-cache", -1, base.CacheModeMain, 10, "lru", nil, nil, nil, nil, nil)

	// Create metrics wrapper
	metricsCache := NewInstrumentedCache(underlyingCache, collector)

	// Test SetMany
	items := map[string]int{
		"key1": 100,
		"key2": 200,
		"key3": 300,
		"key4": 400,
	}
	metricsCache.SetMany(items)

	// Test GetMany
	values, missing := metricsCache.GetMany([]string{"key1", "key2", "key5", "key6"})
	is.Len(values, 2)
	is.Len(missing, 2)
	is.Equal(100, values["key1"])
	is.Equal(200, values["key2"])
	is.Contains(missing, "key5")
	is.Contains(missing, "key6")

	// Test HasMany
	hasResults := metricsCache.HasMany([]string{"key1", "key2", "key5", "key6"})
	is.True(hasResults["key1"])
	is.True(hasResults["key2"])
	is.False(hasResults["key5"])
	is.False(hasResults["key6"])

	// Test PeekMany
	peekValues, peekMissing := metricsCache.PeekMany([]string{"key1", "key2", "key5"})
	is.Len(peekValues, 2)
	is.Len(peekMissing, 1)
	is.Equal(100, peekValues["key1"])
	is.Equal(200, peekValues["key2"])

	// Test DeleteMany
	deletedMap := metricsCache.DeleteMany([]string{"key1", "key2", "key7"})
	is.True(deletedMap["key1"])
	is.True(deletedMap["key2"])
	is.False(deletedMap["key7"])
}

func TestInstrumentedCache_RangeOperation(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Create underlying cache
	underlyingCache := lru.NewLRUCache[string, int](10)

	// Create metrics collector
	collector := NewPrometheusCollector("test-cache", -1, base.CacheModeMain, 10, "lru", nil, nil, nil, nil, nil)

	// Create metrics wrapper
	metricsCache := NewInstrumentedCache(underlyingCache, collector)

	// Add some items
	metricsCache.Set("key1", 100)
	metricsCache.Set("key2", 200)
	metricsCache.Set("key3", 300)

	// Test Range operation
	visited := make(map[string]int)
	metricsCache.Range(func(k string, v int) bool {
		visited[k] = v
		return true // continue iteration
	})

	is.Len(visited, 3)
	is.Equal(100, visited["key1"])
	is.Equal(200, visited["key2"])
	is.Equal(300, visited["key3"])

	// Test Range with early termination
	visited = make(map[string]int)
	metricsCache.Range(func(k string, v int) bool {
		visited[k] = v
		return false // stop iteration after first item
	})

	is.Len(visited, 1)
}

func TestInstrumentedCache_ShardLabeling(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Create underlying cache
	underlyingCache := lru.NewLRUCache[string, int](10)

	// Create metrics collector with shard label
	collector := NewPrometheusCollector("test-cache", 5, base.CacheModeMain, 10, "lru", nil, nil, nil, nil, nil)

	// Create metrics wrapper
	metricsCache := NewInstrumentedCache(underlyingCache, collector)

	// Test basic operations
	metricsCache.Set("key1", 100)
	value, found := metricsCache.Get("key1")
	is.True(found)
	is.Equal(100, value)

	// Verify the collector has the correct shard label
	// Note: We can't easily verify the actual labels without Prometheus registry
	// But we can verify the operations work correctly
	is.Equal(1, metricsCache.Len())
}

//nolint:tparallel,paralleltest
func TestInstrumentedCache_AllAlgorithms(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	algorithms := []string{"lru", "lfu", "arc", "2q"}

	for _, algo := range algorithms {
		t.Run(algo, func(t *testing.T) {
			// Create underlying cache
			underlyingCache := lru.NewLRUCache[string, int](10)

			// Create metrics collector with different algorithm
			collector := NewPrometheusCollector("test-cache", -1, base.CacheModeMain, 10, algo, nil, nil, nil, nil, nil)

			// Create metrics wrapper
			metricsCache := NewInstrumentedCache(underlyingCache, collector)

			// Test basic operations
			metricsCache.Set("key1", 100)
			value, found := metricsCache.Get("key1")
			is.True(found)
			is.Equal(100, value)

			// Verify algorithm is reported correctly
			is.Equal("lru", metricsCache.Algorithm()) // Underlying cache is still LRU
		})
	}
}

func TestInstrumentedCache_WithTTL(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Create underlying cache
	underlyingCache := lru.NewLRUCache[string, int](10)

	// Create metrics collector with TTL
	ttl := 30 * time.Second
	collector := NewPrometheusCollector("test-cache", -1, base.CacheModeMain, 10, "lru", &ttl, nil, nil, nil, nil)

	// Create metrics wrapper
	metricsCache := NewInstrumentedCache(underlyingCache, collector)

	// Test basic operations
	metricsCache.Set("key1", 100)
	value, found := metricsCache.Get("key1")
	is.True(found)
	is.Equal(100, value)

	// Verify operations work correctly with TTL configuration
	is.Equal(1, metricsCache.Len())
}

func TestInstrumentedCache_WithAllSettings(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Create underlying cache
	underlyingCache := lru.NewLRUCache[string, int](10)

	// Create metrics collector with all optional settings
	ttl := 30 * time.Second
	jitterLambda := 0.1
	jitterUpperBound := 5 * time.Second
	stale := 60 * time.Second
	missingCapacity := 50

	collector := NewPrometheusCollector("test-cache", -1, base.CacheModeMain, 10, "lru", &ttl, &jitterLambda, &jitterUpperBound, &stale, &missingCapacity)

	// Create metrics wrapper
	metricsCache := NewInstrumentedCache(underlyingCache, collector)

	// Test basic operations
	metricsCache.Set("key1", 100)
	value, found := metricsCache.Get("key1")
	is.True(found)
	is.Equal(100, value)

	// Verify operations work correctly with all settings
	is.Equal(1, metricsCache.Len())
}

func TestInstrumentedCache_LengthMetric(t *testing.T) {
	t.Parallel()

	// Create a mock collector to track length updates
	var lastLength int64
	mockCollector := &MockCollector{
		updateLengthFn: func(length int64) {
			lastLength = length
		},
	}

	// Create a cache with capacity 3 to test evictions
	cache := lru.NewLRUCache[int, string](3)
	instrumentedCache := NewInstrumentedCache(cache, mockCollector)

	// Test initial length
	instrumentedCache.Len() // This should update the metric
	assert.Equal(t, int64(0), lastLength)

	// Test length after insertion
	instrumentedCache.Set(1, "one")
	instrumentedCache.Len() // This should update the metric
	assert.Equal(t, int64(1), lastLength)

	// Test length after multiple insertions
	instrumentedCache.Set(2, "two")
	instrumentedCache.Set(3, "three")
	instrumentedCache.Len() // This should update the metric
	assert.Equal(t, int64(3), lastLength)

	// Test length after capacity-based eviction
	instrumentedCache.Set(4, "four")      // This should evict key 1
	instrumentedCache.Len()               // This should update the metric
	assert.Equal(t, int64(3), lastLength) // Should still be 3 due to capacity limit

	// Test length after deletion
	instrumentedCache.Delete(2)
	instrumentedCache.Len() // This should update the metric
	assert.Equal(t, int64(2), lastLength)

	// Test length after purge
	instrumentedCache.Purge()
	instrumentedCache.Len() // This should update the metric
	assert.Equal(t, int64(0), lastLength)
}
