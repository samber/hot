package metrics

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCollector_Basic(t *testing.T) {
	is := assert.New(t)

	// Test basic collector creation
	collector := NewCollector("test-cache", -1, 100, "lru", nil, nil, nil, nil, nil)
	is.NotNil(collector)

	// Verify it's a PrometheusCollector
	_, ok := collector.(*PrometheusCollector)
	is.True(ok, "NewCollector should return a PrometheusCollector")
}

func TestNewCollector_WithShard(t *testing.T) {
	is := assert.New(t)

	// Test collector creation with shard
	collector := NewCollector("test-cache", 5, 100, "lru", nil, nil, nil, nil, nil)
	is.NotNil(collector)

	// Verify it's a PrometheusCollector
	prometheusCollector, ok := collector.(*PrometheusCollector)
	is.True(ok, "NewCollector should return a PrometheusCollector")

	// Verify shard label is set
	is.Equal("5", prometheusCollector.labels["shard"])
}

func TestNewCollector_WithAllSettings(t *testing.T) {
	is := assert.New(t)

	// Test collector creation with all optional settings
	ttl := 30 * time.Second
	jitterLambda := 0.1
	jitterUpperBound := 5 * time.Second
	stale := 60 * time.Second
	missingCapacity := 50

	collector := NewCollector("test-cache", -1, 100, "lfu", &ttl, &jitterLambda, &jitterUpperBound, &stale, &missingCapacity)
	is.NotNil(collector)

	// Verify it's a PrometheusCollector
	prometheusCollector, ok := collector.(*PrometheusCollector)
	is.True(ok, "NewCollector should return a PrometheusCollector")

	// Verify all gauges are created
	is.NotNil(prometheusCollector.settingsTTL)
	is.NotNil(prometheusCollector.settingsJitterLambda)
	is.NotNil(prometheusCollector.settingsJitterUpperBound)
	is.NotNil(prometheusCollector.settingsStale)
	is.NotNil(prometheusCollector.settingsMissingCapacity)
}

func TestNewCollector_AllAlgorithms(t *testing.T) {
	is := assert.New(t)

	algorithms := []string{"lru", "lfu", "arc", "2q", "unknown"}

	for _, algo := range algorithms {
		t.Run(algo, func(t *testing.T) {
			collector := NewCollector("test-cache", -1, 100, algo, nil, nil, nil, nil, nil)
			is.NotNil(collector, "NewCollector should not return nil for algorithm %s", algo)

			// Verify it's a PrometheusCollector
			prometheusCollector, ok := collector.(*PrometheusCollector)
			is.True(ok, "NewCollector should return a PrometheusCollector for algorithm %s", algo)

			// Verify algorithm gauge is created
			is.NotNil(prometheusCollector.settingsAlgorithm, "Algorithm gauge should not be nil for %s", algo)
		})
	}
}

func TestNewCollector_ShardLabels(t *testing.T) {
	is := assert.New(t)

	testCases := []struct {
		shard         int
		expectedShard string
	}{
		{-1, ""},     // No shard label
		{0, "0"},     // Shard 0
		{5, "5"},     // Shard 5
		{999, "999"}, // Large shard number
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("shard_%d", tc.shard), func(t *testing.T) {
			collector := NewCollector("test-cache", tc.shard, 100, "lru", nil, nil, nil, nil, nil)
			is.NotNil(collector)

			prometheusCollector, ok := collector.(*PrometheusCollector)
			is.True(ok)

			if tc.expectedShard == "" {
				// No shard label should be present
				_, exists := prometheusCollector.labels["shard"]
				is.False(exists, "Shard label should not exist for shard %d", tc.shard)
			} else {
				// Shard label should be present
				shardValue, exists := prometheusCollector.labels["shard"]
				is.True(exists, "Shard label should exist for shard %d", tc.shard)
				is.Equal(tc.expectedShard, shardValue, "Shard label should be %s for shard %d", tc.expectedShard, tc.shard)
			}
		})
	}
}

func TestNewCollector_EdgeCases(t *testing.T) {
	is := assert.New(t)

	// Test with empty cache name
	collector := NewCollector("", -1, 100, "lru", nil, nil, nil, nil, nil)
	is.NotNil(collector)

	prometheusCollector, ok := collector.(*PrometheusCollector)
	is.True(ok)
	is.Equal("", prometheusCollector.name)

	// Test with zero capacity
	collector = NewCollector("test-cache", -1, 0, "lru", nil, nil, nil, nil, nil)
	is.NotNil(collector)

	prometheusCollector, ok = collector.(*PrometheusCollector)
	is.True(ok)
	is.NotNil(prometheusCollector.settingsCapacity)

	// Test with negative capacity
	collector = NewCollector("test-cache", -1, -100, "lru", nil, nil, nil, nil, nil)
	is.NotNil(collector)

	prometheusCollector, ok = collector.(*PrometheusCollector)
	is.True(ok)
	is.NotNil(prometheusCollector.settingsCapacity)

	// Test with very large capacity
	collector = NewCollector("test-cache", -1, 999999999, "lru", nil, nil, nil, nil, nil)
	is.NotNil(collector)

	prometheusCollector, ok = collector.(*PrometheusCollector)
	is.True(ok)
	is.NotNil(prometheusCollector.settingsCapacity)
}

func TestNewCollector_OptionalSettings(t *testing.T) {
	is := assert.New(t)

	// Test with only TTL
	ttl := 30 * time.Second
	collector := NewCollector("test-cache", -1, 100, "lru", &ttl, nil, nil, nil, nil)
	is.NotNil(collector)

	prometheusCollector, ok := collector.(*PrometheusCollector)
	is.True(ok)
	is.NotNil(prometheusCollector.settingsTTL)
	is.Nil(prometheusCollector.settingsJitterLambda)
	is.Nil(prometheusCollector.settingsJitterUpperBound)
	is.Nil(prometheusCollector.settingsStale)
	is.Nil(prometheusCollector.settingsMissingCapacity)

	// Test with only jitterLambda
	jitterLambda := 0.1
	collector = NewCollector("test-cache", -1, 100, "lru", nil, &jitterLambda, nil, nil, nil)
	is.NotNil(collector)

	prometheusCollector, ok = collector.(*PrometheusCollector)
	is.True(ok)
	is.Nil(prometheusCollector.settingsTTL)
	is.NotNil(prometheusCollector.settingsJitterLambda)
	is.Nil(prometheusCollector.settingsJitterUpperBound)
	is.Nil(prometheusCollector.settingsStale)
	is.Nil(prometheusCollector.settingsMissingCapacity)

	// Test with only jitterUpperBound
	jitterUpperBound := 5 * time.Second
	collector = NewCollector("test-cache", -1, 100, "lru", nil, nil, &jitterUpperBound, nil, nil)
	is.NotNil(collector)

	prometheusCollector, ok = collector.(*PrometheusCollector)
	is.True(ok)
	is.Nil(prometheusCollector.settingsTTL)
	is.Nil(prometheusCollector.settingsJitterLambda)
	is.NotNil(prometheusCollector.settingsJitterUpperBound)
	is.Nil(prometheusCollector.settingsStale)
	is.Nil(prometheusCollector.settingsMissingCapacity)

	// Test with only stale
	stale := 60 * time.Second
	collector = NewCollector("test-cache", -1, 100, "lru", nil, nil, nil, &stale, nil)
	is.NotNil(collector)

	prometheusCollector, ok = collector.(*PrometheusCollector)
	is.True(ok)
	is.Nil(prometheusCollector.settingsTTL)
	is.Nil(prometheusCollector.settingsJitterLambda)
	is.Nil(prometheusCollector.settingsJitterUpperBound)
	is.NotNil(prometheusCollector.settingsStale)
	is.Nil(prometheusCollector.settingsMissingCapacity)

	// Test with only missingCapacity
	missingCapacity := 50
	collector = NewCollector("test-cache", -1, 100, "lru", nil, nil, nil, nil, &missingCapacity)
	is.NotNil(collector)

	prometheusCollector, ok = collector.(*PrometheusCollector)
	is.True(ok)
	is.Nil(prometheusCollector.settingsTTL)
	is.Nil(prometheusCollector.settingsJitterLambda)
	is.Nil(prometheusCollector.settingsJitterUpperBound)
	is.Nil(prometheusCollector.settingsStale)
	is.NotNil(prometheusCollector.settingsMissingCapacity)
}

func TestNewCollector_ExtremeValues(t *testing.T) {
	is := assert.New(t)

	// Test with extreme TTL values
	veryShortTTL := 1 * time.Nanosecond
	veryLongTTL := 24 * 365 * time.Hour // 1 year

	collector := NewCollector("test-cache", -1, 100, "lru", &veryShortTTL, nil, nil, nil, nil)
	is.NotNil(collector)

	collector = NewCollector("test-cache", -1, 100, "lru", &veryLongTTL, nil, nil, nil, nil)
	is.NotNil(collector)

	// Test with extreme jitterLambda values
	verySmallJitter := 0.000001
	veryLargeJitter := 999999.0

	collector = NewCollector("test-cache", -1, 100, "lru", nil, &verySmallJitter, nil, nil, nil)
	is.NotNil(collector)

	collector = NewCollector("test-cache", -1, 100, "lru", nil, &veryLargeJitter, nil, nil, nil)
	is.NotNil(collector)

	// Test with extreme jitterUpperBound values
	veryShortJitterBound := 1 * time.Nanosecond
	veryLongJitterBound := 24 * 365 * time.Hour // 1 year

	collector = NewCollector("test-cache", -1, 100, "lru", nil, nil, &veryShortJitterBound, nil, nil)
	is.NotNil(collector)

	collector = NewCollector("test-cache", -1, 100, "lru", nil, nil, &veryLongJitterBound, nil, nil)
	is.NotNil(collector)

	// Test with extreme stale values
	veryShortStale := 1 * time.Nanosecond
	veryLongStale := 24 * 365 * time.Hour // 1 year

	collector = NewCollector("test-cache", -1, 100, "lru", nil, nil, nil, &veryShortStale, nil)
	is.NotNil(collector)

	collector = NewCollector("test-cache", -1, 100, "lru", nil, nil, nil, &veryLongStale, nil)
	is.NotNil(collector)

	// Test with extreme missingCapacity values
	verySmallMissingCapacity := 1
	veryLargeMissingCapacity := 999999999

	collector = NewCollector("test-cache", -1, 100, "lru", nil, nil, nil, nil, &verySmallMissingCapacity)
	is.NotNil(collector)

	collector = NewCollector("test-cache", -1, 100, "lru", nil, nil, nil, nil, &veryLargeMissingCapacity)
	is.NotNil(collector)
}

func TestNewCollector_Performance(t *testing.T) {
	is := assert.New(t)

	// Benchmark collector creation
	const iterations = 10000

	start := time.Now()
	for i := 0; i < iterations; i++ {
		collector := NewCollector("test-cache", -1, 100, "lru", nil, nil, nil, nil, nil)
		is.NotNil(collector)
	}
	duration := time.Since(start)

	// Log performance metrics
	t.Logf("NewCollector performance (%d iterations): %v", iterations, duration)

	// Verify creation is reasonably fast
	maxDuration := 1 * time.Second // 100μs per collector
	is.Less(duration, maxDuration, "NewCollector should be fast")
}
