package metrics

import (
	"testing"

	"github.com/samber/hot/pkg/lru"
	"github.com/stretchr/testify/assert"
)

func TestMetricsCache_BasicOperations(t *testing.T) {
	is := assert.New(t)

	// Create underlying cache
	underlyingCache := lru.NewLRUCache[string, int](10)

	// Create metrics collector
	collector := NewCollector("test-cache", -1, 10, "lru", nil, nil, nil, nil, nil)

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
	is.Equal(2, len(values))
	is.Equal(1, len(missing))
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

func TestMetricsCache_MetricsTracking(t *testing.T) {
	is := assert.New(t)

	// Create underlying cache
	underlyingCache := lru.NewLRUCache[string, int](5)

	// Create metrics collector
	collector := NewCollector("test-cache", -1, 5, "lru", nil, nil, nil, nil, nil)

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
	metricsCache.Get("key1") // hit
	metricsCache.Get("key2") // hit
	metricsCache.Get("key5") // miss
	metricsCache.Has("key3") // hit
	metricsCache.Has("key6") // miss

	// Test eviction metrics
	metricsCache.Set("key7", 700) // This should evict key1 due to capacity
	metricsCache.Delete("key2")
	metricsCache.DeleteMany([]string{"key3", "key8"}) // key8 doesn't exist

	// Test size metrics
	metricsCache.UpdateSizeBytes(func(v int) int {
		return 8 // Assume int is 8 bytes
	})

	// Verify metrics are being tracked
	// Note: In a real test, you would collect the metrics and verify the values
	// For now, we just verify the cache operations work correctly
	// After evictions and deletions, we should have key4, key5, and key7 (3 items)
	is.Equal(3, metricsCache.Len())
}
