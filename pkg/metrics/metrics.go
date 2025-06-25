package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// NewMetrics creates a new Metrics instance with Prometheus metrics for cache monitoring.
// The metrics track cache performance, configuration settings, and operational statistics.
// All metrics are automatically registered with the default Prometheus registry.
func NewMetrics(ttl time.Duration, jitterLambda float64, jitterUpperBound time.Duration, stale time.Duration) *Metrics {
	metrics := &Metrics{
		// Cache operation counters
		Insertion: prometheus.NewCounter(prometheus.CounterOpts{Name: "cache_insertion_total"}),
		Eviction:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "cache_eviction_total"}, []string{"reason"}),

		// Cache hit/miss counters
		Hit:  prometheus.NewCounter(prometheus.CounterOpts{Name: "cache_hit_total"}),
		Miss: prometheus.NewCounter(prometheus.CounterOpts{Name: "cache_miss_total"}),

		// @TODO: Add cache size and memory usage metrics
		// Length: prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_length"}),
		// Weight: prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_memory_bytes"}),

		// Cache configuration settings
		// @TODO: Add capacity setting metric
		// SettingsCapacity:        prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_settings_capacity"}),
		SettingsTTL:              prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_settings_ttl_seconds"}),
		SettingsJitterLambda:     prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_settings_jitter_lambda"}),
		SettingsJitterUpperBound: prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_settings_jitter_upper_bound_seconds"}),
		SettingsStale:            prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_settings_stale_seconds"}),
		// @TODO: Add missing cache capacity setting
		// SettingsMissingCapacity: prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_settings_missing_capacity"}),
		SettingsMissingTTL:   prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_settings_missing_ttl_seconds"}),
		SettingsMissingStale: prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_settings_missing_stale_seconds"}),
	}

	// Set initial configuration values
	// @TODO: Add capacity setting
	// metrics.SettingsCapacity.Set(float64(capacity))
	metrics.SettingsTTL.Set(ttl.Seconds())
	metrics.SettingsJitterLambda.Set(float64(jitterLambda))
	metrics.SettingsJitterUpperBound.Set(float64(jitterUpperBound.Seconds()))
	metrics.SettingsStale.Set(stale.Seconds())
	// @TODO: Add missing cache capacity setting
	// metrics.SettingsMissingCapacity.Set(float64(missingCapacity))
	metrics.SettingsMissingTTL.Set(ttl.Seconds())
	metrics.SettingsMissingStale.Set(stale.Seconds())

	return metrics
}

// Metrics provides Prometheus metrics for monitoring cache performance and configuration.
// The metrics include counters for cache operations (insertions, evictions, hits, misses)
// and gauges for configuration settings (TTL, jitter, stale duration).
//
// @TODO:
// - Use simple atomic counters and gauges for better performance
// - Provide no-op implementation when metrics are disabled
// - Collect Prometheus metrics in a separate goroutine
// - Add revalidation count and delay metrics
// - Add cache name label for multi-cache scenarios
// - Add weight calculation including item wrapper overhead
type Metrics struct {
	// Cache operation counters
	Insertion prometheus.Counter     // Total number of items inserted into the cache
	Eviction  *prometheus.CounterVec // Total number of items evicted, labeled by reason

	// Cache performance counters
	Hit  prometheus.Counter // Total number of cache hits
	Miss prometheus.Counter // Total number of cache misses

	// @TODO: Add additional operation counters
	// locks ?    // Number of lock acquisitions
	// del ?      // Number of deletions
	// range ?    // Number of range operations
	// purge ?    // Number of purge operations

	// @TODO: Add cache state gauges
	// Length prometheus.Gauge // Current number of items in cache
	// Weight prometheus.Gauge // Current memory usage in bytes

	// Cache configuration settings (gauges)
	// @TODO: Add capacity setting gauge
	// SettingsCapacity        prometheus.Gauge // Maximum number of items the cache can hold
	SettingsTTL              prometheus.Gauge // Time-to-live duration in seconds
	SettingsJitterLambda     prometheus.Gauge // Jitter lambda parameter for TTL randomization
	SettingsJitterUpperBound prometheus.Gauge // Jitter upper bound duration in seconds
	SettingsStale            prometheus.Gauge // Stale duration in seconds
	// @TODO: Add missing cache capacity setting gauge
	// SettingsMissingCapacity prometheus.Gauge // Maximum number of missing keys the cache can hold
	SettingsMissingTTL   prometheus.Gauge // Missing key TTL duration in seconds
	SettingsMissingStale prometheus.Gauge // Missing key stale duration in seconds
}

// IncInsertion increments the insertion counter by 1.
// This should be called whenever a new item is added to the cache.
func (m *Metrics) IncInsertion() {
	m.Insertion.Inc()
}

// IncEviction increments the eviction counter by 1 for the specified reason.
// Common reasons include "capacity", "ttl", "manual", etc.
// This should be called whenever an item is removed from the cache.
func (m *Metrics) IncEviction(reason string) {
	m.Eviction.WithLabelValues(reason).Inc()
}

// IncHit increments the hit counter by 1.
// This should be called whenever a cache lookup succeeds.
func (m *Metrics) IncHit() {
	m.Hit.Inc()
}

// IncMiss increments the miss counter by 1.
// This should be called whenever a cache lookup fails.
func (m *Metrics) IncMiss() {
	m.Miss.Inc()
}
