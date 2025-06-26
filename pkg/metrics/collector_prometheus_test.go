package metrics

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samber/hot/pkg/base"
	"github.com/stretchr/testify/assert"
)

func TestNewPrometheusCollector(t *testing.T) {
	is := assert.New(t)

	// Test basic constructor with minimal parameters
	collector := NewPrometheusCollector("test-cache", -1, base.CacheModeMain, 100, "lru", nil, nil, nil, nil, nil)
	is.NotNil(collector)
	is.Equal("test-cache", collector.name)
	is.NotNil(collector.settingsCapacity)
	is.NotNil(collector.settingsAlgorithm)

	// Test with all parameters
	ttl := 30 * time.Second
	jitterLambda := 0.1
	jitterUpperBound := 5 * time.Second
	stale := 60 * time.Second
	missingCapacity := 50

	collector = NewPrometheusCollector(
		"full-cache",
		-1,
		base.CacheModeMain,
		200,
		"lfu",
		&ttl,
		&jitterLambda,
		&jitterUpperBound,
		&stale,
		&missingCapacity,
	)

	is.NotNil(collector)
	is.Equal("full-cache", collector.name)
	is.NotNil(collector.settingsCapacity)
	is.NotNil(collector.settingsAlgorithm)
	is.NotNil(collector.settingsTTL)
	is.NotNil(collector.settingsJitterLambda)
	is.NotNil(collector.settingsJitterUpperBound)
	is.NotNil(collector.settingsStale)
	is.NotNil(collector.settingsMissingCapacity)
}

func TestNewPrometheusCollector_AlgorithmValues(t *testing.T) {
	is := assert.New(t)

	// Test all algorithm values - just verify the constructor doesn't panic
	testCases := []struct {
		algorithm string
	}{
		{"lru"},
		{"lfu"},
		{"arc"},
		{"2q"},
		{"unknown"},
	}

	for _, tc := range testCases {
		collector := NewPrometheusCollector("test", -1, base.CacheModeMain, 100, tc.algorithm, nil, nil, nil, nil, nil)
		is.NotNil(collector, "Constructor should not return nil for algorithm %s", tc.algorithm)
		is.NotNil(collector.settingsAlgorithm, "Algorithm gauge should not be nil for %s", tc.algorithm)
	}
}

func TestPrometheusCollector_InsertionCounters(t *testing.T) {
	is := assert.New(t)

	collector := NewPrometheusCollector("test", -1, base.CacheModeMain, 100, "lru", nil, nil, nil, nil, nil)

	// Test IncInsertion
	collector.IncInsertion()
	is.Equal(int64(1), atomic.LoadInt64(&collector.insertionCount))

	collector.IncInsertion()
	collector.IncInsertion()
	is.Equal(int64(3), atomic.LoadInt64(&collector.insertionCount))

	// Test AddInsertions
	collector.AddInsertions(5)
	is.Equal(int64(8), atomic.LoadInt64(&collector.insertionCount))

	collector.AddInsertions(0)
	is.Equal(int64(8), atomic.LoadInt64(&collector.insertionCount))

	collector.AddInsertions(-2)
	is.Equal(int64(6), atomic.LoadInt64(&collector.insertionCount))
}

func TestPrometheusCollector_EvictionCounters(t *testing.T) {
	is := assert.New(t)

	collector := NewPrometheusCollector("test", -1, base.CacheModeMain, 100, "lru", nil, nil, nil, nil, nil)

	// Test IncEviction with known reasons
	collector.IncEviction(base.EvictionReasonCapacity)
	collector.IncEviction(base.EvictionReasonTTL)
	collector.IncEviction(base.EvictionReasonManual)
	collector.IncEviction(base.EvictionReasonStale)

	// Verify all known reasons are tracked
	is.Equal(int64(1), atomic.LoadInt64(collector.evictionCount[string(base.EvictionReasonCapacity)]))
	is.Equal(int64(1), atomic.LoadInt64(collector.evictionCount[string(base.EvictionReasonTTL)]))
	is.Equal(int64(1), atomic.LoadInt64(collector.evictionCount[string(base.EvictionReasonManual)]))
	is.Equal(int64(1), atomic.LoadInt64(collector.evictionCount[string(base.EvictionReasonStale)]))

	// Test IncEviction with unknown reason
	unknownReason := base.EvictionReason("unknown_reason")
	collector.IncEviction(unknownReason)
	is.Equal(int64(1), atomic.LoadInt64(collector.evictionCount[string(unknownReason)]))

	// Test AddEvictions
	collector.AddEvictions(base.EvictionReasonCapacity, 3)
	is.Equal(int64(4), atomic.LoadInt64(collector.evictionCount[string(base.EvictionReasonCapacity)]))

	collector.AddEvictions(unknownReason, 2)
	is.Equal(int64(3), atomic.LoadInt64(collector.evictionCount[string(unknownReason)]))

	// Test AddEvictions with zero and negative values
	collector.AddEvictions(base.EvictionReasonCapacity, 0)
	is.Equal(int64(4), atomic.LoadInt64(collector.evictionCount[string(base.EvictionReasonCapacity)]))

	collector.AddEvictions(base.EvictionReasonCapacity, -1)
	is.Equal(int64(3), atomic.LoadInt64(collector.evictionCount[string(base.EvictionReasonCapacity)]))
}

func TestPrometheusCollector_HitMissCounters(t *testing.T) {
	is := assert.New(t)

	collector := NewPrometheusCollector("test", -1, base.CacheModeMain, 100, "lru", nil, nil, nil, nil, nil)

	// Test hit counters
	collector.IncHit()
	is.Equal(int64(1), atomic.LoadInt64(&collector.hitCount))

	collector.IncHit()
	collector.IncHit()
	is.Equal(int64(3), atomic.LoadInt64(&collector.hitCount))

	collector.AddHits(5)
	is.Equal(int64(8), atomic.LoadInt64(&collector.hitCount))

	// Test miss counters
	collector.IncMiss()
	is.Equal(int64(1), atomic.LoadInt64(&collector.missCount))

	collector.IncMiss()
	collector.IncMiss()
	is.Equal(int64(3), atomic.LoadInt64(&collector.missCount))

	collector.AddMisses(7)
	is.Equal(int64(10), atomic.LoadInt64(&collector.missCount))

	// Test edge cases
	collector.AddHits(0)
	is.Equal(int64(8), atomic.LoadInt64(&collector.hitCount))

	collector.AddHits(-2)
	is.Equal(int64(6), atomic.LoadInt64(&collector.hitCount))

	collector.AddMisses(0)
	is.Equal(int64(10), atomic.LoadInt64(&collector.missCount))

	collector.AddMisses(-3)
	is.Equal(int64(7), atomic.LoadInt64(&collector.missCount))
}

func TestPrometheusCollector_ConcurrentAccess(t *testing.T) {
	is := assert.New(t)

	collector := NewPrometheusCollector("test", -1, base.CacheModeMain, 100, "lru", nil, nil, nil, nil, nil)

	// Test concurrent access to counters
	const numGoroutines = 100
	const operationsPerGoroutine = 1000

	var wg sync.WaitGroup

	// Concurrent insertion operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				collector.IncInsertion()
			}
		}()
	}

	// Concurrent hit operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				collector.IncHit()
			}
		}()
	}

	// Concurrent miss operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				collector.IncMiss()
			}
		}()
	}

	// Concurrent eviction operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				collector.IncEviction(base.EvictionReasonCapacity)
			}
		}()
	}

	wg.Wait()

	// Verify final counts
	expectedTotal := int64(numGoroutines * operationsPerGoroutine)
	is.Equal(expectedTotal, atomic.LoadInt64(&collector.insertionCount))
	is.Equal(expectedTotal, atomic.LoadInt64(&collector.hitCount))
	is.Equal(expectedTotal, atomic.LoadInt64(&collector.missCount))
	is.Equal(expectedTotal, atomic.LoadInt64(collector.evictionCount[string(base.EvictionReasonCapacity)]))
}

func TestPrometheusCollector_InterfaceCompliance(t *testing.T) {
	is := assert.New(t)

	// Verify PrometheusCollector implements Collector interface
	var _ Collector = (*PrometheusCollector)(nil)

	collector := NewPrometheusCollector("test", -1, base.CacheModeMain, 100, "lru", nil, nil, nil, nil, nil)
	is.NotNil(collector)

	// Test all interface methods
	collector.IncInsertion()
	collector.AddInsertions(5)
	collector.IncEviction(base.EvictionReasonCapacity)
	collector.AddEvictions(base.EvictionReasonTTL, 3)
	collector.IncHit()
	collector.AddHits(2)
	collector.IncMiss()
	collector.AddMisses(1)

	// Verify the operations worked
	is.Equal(int64(6), atomic.LoadInt64(&collector.insertionCount))
	is.Equal(int64(1), atomic.LoadInt64(collector.evictionCount[string(base.EvictionReasonCapacity)]))
	is.Equal(int64(3), atomic.LoadInt64(collector.evictionCount[string(base.EvictionReasonTTL)]))
	is.Equal(int64(3), atomic.LoadInt64(&collector.hitCount))
	is.Equal(int64(2), atomic.LoadInt64(&collector.missCount))
}

func TestPrometheusCollector_EdgeCases(t *testing.T) {
	is := assert.New(t)

	// Test with empty labels
	collector := NewPrometheusCollector("test", -1, base.CacheModeMain, 100, "lru", nil, nil, nil, nil, nil)
	is.NotNil(collector)
	is.Equal("test", collector.name)
	is.Len(collector.labels, 2)
	is.Equal("test", collector.labels["name"])
	is.Equal("main", collector.labels["mode"])

	// Test with sharding
	collector = NewPrometheusCollector("test", 2, base.CacheModeMain, 100, "lru", nil, nil, nil, nil, nil)
	is.NotNil(collector)
	is.NotNil(collector.labels)
	is.NotEmpty(collector.labels)
	is.Len(collector.labels, 3)
	is.Equal("test", collector.labels["name"])
	is.Equal("2", collector.labels["shard"])
	is.Equal("main", collector.labels["mode"])

	// Test with zero capacity
	collector = NewPrometheusCollector("test", -1, base.CacheModeMain, 0, "lru", nil, nil, nil, nil, nil)
	is.NotNil(collector)
	is.NotNil(collector.settingsCapacity)

	// Test with negative capacity
	collector = NewPrometheusCollector("test", -1, base.CacheModeMain, -100, "lru", nil, nil, nil, nil, nil)
	is.NotNil(collector)
	is.NotNil(collector.settingsCapacity)

	// Test with very large values
	collector = NewPrometheusCollector("test", -1, base.CacheModeMain, 999999999, "lru", nil, nil, nil, nil, nil)
	is.NotNil(collector)
	is.NotNil(collector.settingsCapacity)
}

func TestPrometheusCollector_EvictionReasonsInitialization(t *testing.T) {
	is := assert.New(t)

	collector := NewPrometheusCollector("test", -1, base.CacheModeMain, 100, "lru", nil, nil, nil, nil, nil)

	// Verify all known eviction reasons are initialized
	for _, reason := range base.EvictionReasons {
		counter, exists := collector.evictionCount[string(reason)]
		is.True(exists, "Eviction reason %s should be initialized", reason)
		is.NotNil(counter, "Counter for reason %s should not be nil", reason)
		is.Equal(int64(0), atomic.LoadInt64(counter), "Counter for reason %s should start at 0", reason)
	}

	// Verify the map has the expected size
	expectedSize := len(base.EvictionReasons)
	is.Equal(expectedSize, len(collector.evictionCount), "Should have %d eviction counters", expectedSize)
}

func TestPrometheusCollector_MetricDescriptors(t *testing.T) {
	is := assert.New(t)

	collector := NewPrometheusCollector("test-cache", -1, base.CacheModeMain, 100, "lru", nil, nil, nil, nil, nil)

	// Test insertion descriptor
	is.NotNil(collector.insertionDesc)
	is.Contains(collector.insertionDesc.String(), "hot_insertion_total")

	// Test eviction descriptor
	is.NotNil(collector.evictionDesc)
	is.Contains(collector.evictionDesc.String(), "hot_eviction_total")

	// Test hit descriptor
	is.NotNil(collector.hitDesc)
	is.Contains(collector.hitDesc.String(), "hot_hit_total")

	// Test miss descriptor
	is.NotNil(collector.missDesc)
	is.Contains(collector.missDesc.String(), "hot_miss_total")

	// Test size descriptor
	is.NotNil(collector.sizeDesc)
	is.Contains(collector.sizeDesc.String(), "hot_size_bytes")
}

func TestPrometheusCollector_SettingsGauges(t *testing.T) {
	is := assert.New(t)

	// Test with all optional settings
	ttl := 30 * time.Second
	jitterLambda := 0.1
	jitterUpperBound := 5 * time.Second
	stale := 60 * time.Second
	missingCapacity := 50

	collector := NewPrometheusCollector(
		"test",
		-1,
		base.CacheModeMain,
		100,
		"arc",
		&ttl,
		&jitterLambda,
		&jitterUpperBound,
		&stale,
		&missingCapacity,
	)

	// Verify all gauges are created
	is.NotNil(collector.settingsCapacity)
	is.NotNil(collector.settingsAlgorithm)
	is.NotNil(collector.settingsTTL)
	is.NotNil(collector.settingsJitterLambda)
	is.NotNil(collector.settingsJitterUpperBound)
	is.NotNil(collector.settingsStale)
	is.NotNil(collector.settingsMissingCapacity)

	// Test with no optional settings
	collector = NewPrometheusCollector("test", -1, base.CacheModeMain, 100, "2q", nil, nil, nil, nil, nil)

	// Verify required gauges are created
	is.NotNil(collector.settingsCapacity)
	is.NotNil(collector.settingsAlgorithm)

	// Verify optional gauges are nil when not provided
	is.Nil(collector.settingsTTL)
	is.Nil(collector.settingsJitterLambda)
	is.Nil(collector.settingsJitterUpperBound)
	is.Nil(collector.settingsStale)
	is.Nil(collector.settingsMissingCapacity)
}
