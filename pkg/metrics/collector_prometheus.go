package metrics

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/hot/pkg/base"
)

var _ Collector = (*PrometheusCollector)(nil)

// PrometheusCollector implements Collector using Prometheus metrics.
type PrometheusCollector struct {
	name   string
	labels prometheus.Labels

	// Counters - use atomic operations for lock-free performance
	insertionCount int64
	evictionCount  map[string]*int64 // reason -> count
	hitCount       int64
	missCount      int64

	// Gauges
	sizeBytes int64
	length    int64

	// Static configuration gauges (one per setting)
	settingsCapacity         prometheus.Gauge
	settingsTTL              prometheus.Gauge
	settingsJitterLambda     prometheus.Gauge
	settingsJitterUpperBound prometheus.Gauge
	settingsStale            prometheus.Gauge
	settingsMissingCapacity  prometheus.Gauge
	settingsAlgorithm        prometheus.Gauge

	// Prometheus metric descriptors for counters
	insertionDesc *prometheus.Desc
	evictionDesc  *prometheus.Desc
	hitDesc       *prometheus.Desc
	missDesc      *prometheus.Desc
	sizeDesc      *prometheus.Desc
	lengthDesc    *prometheus.Desc
}

// NewPrometheusCollector creates a new Prometheus-based metric collector.
//
//nolint:funlen,gocyclo
func NewPrometheusCollector(name string, shard int, mode base.CacheMode, capacity int, algorithm string, ttl *time.Duration, jitterLambda *float64, jitterUpperBound *time.Duration, stale *time.Duration, missingCapacity *int) *PrometheusCollector {
	labels := map[string]string{
		"name": name,
		"mode": string(mode),
	}
	if shard >= 0 {
		labels["shard"] = strconv.Itoa(shard)
	}

	collector := &PrometheusCollector{
		name:          name,
		labels:        prometheus.Labels(labels),
		evictionCount: make(map[string]*int64),
	}

	// Initialize eviction counters for common reasons
	for _, reason := range base.EvictionReasons {
		var count int64
		collector.evictionCount[string(reason)] = &count
	}

	// Create metric descriptors for counters
	collector.insertionDesc = prometheus.NewDesc(
		"hot_insertion_total",
		"Total number of items inserted into the cache",
		nil, labels,
	)
	collector.evictionDesc = prometheus.NewDesc(
		"hot_eviction_total",
		"Total number of items evicted from the cache",
		[]string{"reason"}, labels,
	)
	collector.hitDesc = prometheus.NewDesc(
		"hot_hit_total",
		"Total number of cache hits",
		nil, labels,
	)
	collector.missDesc = prometheus.NewDesc(
		"hot_miss_total",
		"Total number of cache misses",
		nil, labels,
	)
	collector.sizeDesc = prometheus.NewDesc(
		"hot_size_bytes",
		"Current size of the cache in bytes (including keys and values)",
		nil, labels,
	)
	collector.lengthDesc = prometheus.NewDesc(
		"hot_length",
		"Current length of the cache",
		nil, labels,
	)

	//
	// Cache settings
	//

	// Capacity is always set (non-pointer)
	collector.settingsCapacity = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "hot_settings_capacity",
		Help:        "Maximum number of items the cache can hold",
		ConstLabels: labels,
	})
	collector.settingsCapacity.Set(float64(capacity))

	// Algorithm is always set (non-pointer)
	collector.settingsAlgorithm = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "hot_settings_algorithm",
		Help:        "Eviction algorithm type (0=lru, 1=lfu, 2=arc, 3=2q, 4=fifo, 5=tinylfu, 6=wtinylfu, 7=s3fifo, 8=sieve)",
		ConstLabels: labels,
	})
	// Convert algorithm string to numeric value for the gauge
	algorithmValue := -1.0
	switch algorithm {
	case "lru":
		algorithmValue = 0.0
	case "lfu":
		algorithmValue = 1.0
	case "arc":
		algorithmValue = 2.0
	case "2q":
		algorithmValue = 3.0
	case "fifo":
		algorithmValue = 4.0
	case "tinylfu":
		algorithmValue = 5.0
	case "wtinylfu":
		algorithmValue = 6.0
	case "s3fifo":
		algorithmValue = 7.0
	case "sieve":
		algorithmValue = 8.0
	}
	collector.settingsAlgorithm.Set(algorithmValue)

	if ttl != nil {
		collector.settingsTTL = prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "hot_settings_ttl_seconds",
			Help:        "Time-to-live duration in seconds",
			ConstLabels: labels,
		})
		collector.settingsTTL.Set(ttl.Seconds())
	}
	if jitterLambda != nil {
		collector.settingsJitterLambda = prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "hot_settings_jitter_lambda",
			Help:        "Jitter lambda parameter for TTL randomization",
			ConstLabels: labels,
		})
		collector.settingsJitterLambda.Set(*jitterLambda)
	}
	if jitterUpperBound != nil {
		collector.settingsJitterUpperBound = prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "hot_settings_jitter_upper_bound_seconds",
			Help:        "Jitter upper bound duration in seconds",
			ConstLabels: labels,
		})
		collector.settingsJitterUpperBound.Set(jitterUpperBound.Seconds())
	}
	if stale != nil {
		collector.settingsStale = prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "hot_settings_stale_seconds",
			Help:        "Stale duration in seconds",
			ConstLabels: labels,
		})
		collector.settingsStale.Set(stale.Seconds())
	}
	if missingCapacity != nil {
		collector.settingsMissingCapacity = prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "hot_settings_missing_capacity",
			Help:        "Maximum number of missing keys the cache can hold",
			ConstLabels: labels,
		})
		collector.settingsMissingCapacity.Set(float64(*missingCapacity))
	}

	return collector
}

// IncInsertion atomically increments the insertion counter.
func (p *PrometheusCollector) IncInsertion() {
	atomic.AddInt64(&p.insertionCount, 1)
}

// AddInsertions atomically adds the specified count to the insertion counter.
func (p *PrometheusCollector) AddInsertions(count int64) {
	atomic.AddInt64(&p.insertionCount, count)
}

// IncEviction atomically increments the eviction counter for the given reason.
func (p *PrometheusCollector) IncEviction(reason base.EvictionReason) {
	if counter, exists := p.evictionCount[string(reason)]; exists {
		atomic.AddInt64(counter, 1)
	} else {
		// Create new counter for unknown reason
		var count int64
		p.evictionCount[string(reason)] = &count
		atomic.AddInt64(&count, 1)
	}
}

// AddEvictions atomically adds the specified count to the eviction counter for the given reason.
func (p *PrometheusCollector) AddEvictions(reason base.EvictionReason, count int64) {
	if counter, exists := p.evictionCount[string(reason)]; exists {
		atomic.AddInt64(counter, count)
	} else {
		// Create new counter for unknown reason
		var newCount int64
		p.evictionCount[string(reason)] = &newCount
		atomic.AddInt64(&newCount, count)
	}
}

// IncHit atomically increments the hit counter.
func (p *PrometheusCollector) IncHit() {
	atomic.AddInt64(&p.hitCount, 1)
}

// AddHits atomically adds the specified count to the hit counter.
func (p *PrometheusCollector) AddHits(count int64) {
	atomic.AddInt64(&p.hitCount, count)
}

// IncMiss atomically increments the miss counter.
func (p *PrometheusCollector) IncMiss() {
	atomic.AddInt64(&p.missCount, 1)
}

// AddMisses atomically adds the specified count to the miss counter.
func (p *PrometheusCollector) AddMisses(count int64) {
	atomic.AddInt64(&p.missCount, count)
}

// UpdateSizeBytes atomically updates the cache size in bytes.
func (p *PrometheusCollector) UpdateSizeBytes(sizeBytes int64) {
	atomic.StoreInt64(&p.sizeBytes, sizeBytes)
}

// UpdateLength atomically updates the cache length.
func (p *PrometheusCollector) UpdateLength(length int64) {
	atomic.StoreInt64(&p.length, length)
}

// Describe implements prometheus.Collector interface.
func (p *PrometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- p.insertionDesc
	ch <- p.evictionDesc
	ch <- p.hitDesc
	ch <- p.missDesc
	ch <- p.sizeDesc
	ch <- p.lengthDesc
	if p.settingsCapacity != nil {
		ch <- p.settingsCapacity.Desc()
	}
	if p.settingsTTL != nil {
		ch <- p.settingsTTL.Desc()
	}
	if p.settingsJitterLambda != nil {
		ch <- p.settingsJitterLambda.Desc()
	}
	if p.settingsJitterUpperBound != nil {
		ch <- p.settingsJitterUpperBound.Desc()
	}
	if p.settingsStale != nil {
		ch <- p.settingsStale.Desc()
	}
	if p.settingsMissingCapacity != nil {
		ch <- p.settingsMissingCapacity.Desc()
	}
	if p.settingsAlgorithm != nil {
		ch <- p.settingsAlgorithm.Desc()
	}
}

// Collect implements prometheus.Collector interface.
func (p *PrometheusCollector) Collect(ch chan<- prometheus.Metric) {
	// Collect counters
	ch <- prometheus.MustNewConstMetric(
		p.insertionDesc,
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&p.insertionCount)),
	)

	ch <- prometheus.MustNewConstMetric(
		p.hitDesc,
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&p.hitCount)),
	)

	ch <- prometheus.MustNewConstMetric(
		p.missDesc,
		prometheus.CounterValue,
		float64(atomic.LoadInt64(&p.missCount)),
	)

	// Collect size gauge
	ch <- prometheus.MustNewConstMetric(
		p.sizeDesc,
		prometheus.GaugeValue,
		float64(atomic.LoadInt64(&p.sizeBytes)),
	)

	// Collect length gauge
	ch <- prometheus.MustNewConstMetric(
		p.lengthDesc,
		prometheus.GaugeValue,
		float64(atomic.LoadInt64(&p.length)),
	)

	// Collect eviction counters
	for reason, counter := range p.evictionCount {
		ch <- prometheus.MustNewConstMetric(
			p.evictionDesc,
			prometheus.CounterValue,
			float64(atomic.LoadInt64(counter)),
			reason,
		)
	}

	// Collect configuration gauges
	if p.settingsCapacity != nil {
		p.settingsCapacity.Collect(ch)
	}
	if p.settingsTTL != nil {
		p.settingsTTL.Collect(ch)
	}
	if p.settingsJitterLambda != nil {
		p.settingsJitterLambda.Collect(ch)
	}
	if p.settingsJitterUpperBound != nil {
		p.settingsJitterUpperBound.Collect(ch)
	}
	if p.settingsStale != nil {
		p.settingsStale.Collect(ch)
	}
	if p.settingsMissingCapacity != nil {
		p.settingsMissingCapacity.Collect(ch)
	}
	if p.settingsAlgorithm != nil {
		p.settingsAlgorithm.Collect(ch)
	}
}
