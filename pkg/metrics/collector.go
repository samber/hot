package metrics

import (
	"github.com/samber/hot/pkg/base"
)

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
}
