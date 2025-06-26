package metrics

import (
	"fmt"
	"time"

	"github.com/samber/hot/pkg/base"
)

// NewCollector creates a new metric collector based on whether metrics are enabled.
func NewCollector(
	name string,
	shard int,
	capacity int,
	algorithm string,
	ttl *time.Duration,
	jitterLambda *float64,
	jitterUpperBound *time.Duration,
	stale *time.Duration,
	missingCapacity *int,
) Collector {
	labels := map[string]string{
		"name": name,
	}
	if shard >= 0 {
		labels["shard"] = fmt.Sprintf("%d", shard)
	}

	return NewPrometheusCollector(name, labels, capacity, algorithm, ttl, jitterLambda, jitterUpperBound, stale, missingCapacity)
}

// Collector defines the interface for metric collection operations.
// This allows for both real Prometheus metrics and no-op implementations.
type Collector interface {
	IncInsertion()
	AddInsertions(count int64)
	IncEviction(reason base.EvictionReason)
	AddEvictions(reason base.EvictionReason, count int64)
	IncHit()
	AddHits(count int64)
	IncMiss()
	AddMisses(count int64)
	SetSizeBytes(bytes int64)
}
