package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func NewMetrics(ttl time.Duration, jitterLambda float64, jitterUpperBound time.Duration, stale time.Duration) *Metrics {
	metrics := &Metrics{
		Insertion: prometheus.NewCounter(prometheus.CounterOpts{Name: "cache_insertion_total"}),
		Eviction:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "cache_eviction_total"}, []string{"reason"}),

		Hit:  prometheus.NewCounter(prometheus.CounterOpts{Name: "cache_hit_total"}),
		Miss: prometheus.NewCounter(prometheus.CounterOpts{Name: "cache_miss_total"}),

		Length: prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_length"}),
		Weight: prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_memory_bytes"}),

		// SettingsCapacity:        prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_settings_capacity"}),
		SettingsTTL:              prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_settings_ttl_seconds"}),
		SettingsJitterLambda:     prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_settings_jitter_lambda"}),
		SettingsJitterUpperBound: prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_settings_jitter_upper_bound_seconds"}),
		SettingsStale:            prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_settings_stale_seconds"}),
		// SettingsMissingCapacity: prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_settings_missing_capacity"}),
		SettingsMissingTTL:   prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_settings_missing_ttl_seconds"}),
		SettingsMissingStale: prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_settings_missing_stale_seconds"}),
	}

	// metrics.SettingsCapacity.Set(float64(capacity))
	metrics.SettingsTTL.Set(ttl.Seconds())
	metrics.SettingsJitterLambda.Set(float64(jitterLambda))
	metrics.SettingsJitterUpperBound.Set(float64(jitterUpperBound.Seconds()))
	metrics.SettingsStale.Set(stale.Seconds())
	// metrics.SettingsMissingCapacity.Set(float64(missingCapacity))
	metrics.SettingsMissingTTL.Set(ttl.Seconds())
	metrics.SettingsMissingStale.Set(stale.Seconds())

	return metrics
}

// @TODO: Should be simple atomic counters and gauges.
// @TODO: If metrics are disabled, no need to collect them (use a no-op implementation).
// @TODO: If prometheus metrics are enabled, collect them in a separate goroutine.
// @TODO: collect revalidation count+delay
// @TODO: add a label for the cache name
// @TODO: add comment to metric declaration
// @TODO: weight should be diplicated in order to include *item[V] weight
type Metrics struct {
	Insertion prometheus.Counter
	Eviction  *prometheus.CounterVec

	Hit  prometheus.Counter
	Miss prometheus.Counter

	// locks ?
	// del ?
	// range ?
	// purge ?

	Length prometheus.Gauge
	Weight prometheus.Gauge

	// settings
	// SettingsCapacity        prometheus.Gauge
	SettingsTTL              prometheus.Gauge
	SettingsJitterLambda     prometheus.Gauge
	SettingsJitterUpperBound prometheus.Gauge
	SettingsStale            prometheus.Gauge
	// SettingsMissingCapacity prometheus.Gauge
	SettingsMissingTTL   prometheus.Gauge
	SettingsMissingStale prometheus.Gauge
}
