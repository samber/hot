package metrics

import (
	"github.com/samber/hot/pkg/base"
)

var _ Collector = (*NoOpCollector)(nil)

// NoOpCollector is a no-op implementation of Collector that does nothing.
// This provides better performance than conditional checks when metrics are disabled.
type NoOpCollector struct{}

// IncInsertion does nothing.
func (n *NoOpCollector) IncInsertion() {}

// AddInsertions does nothing.
func (n *NoOpCollector) AddInsertions(count int64) {}

// IncEviction does nothing.
func (n *NoOpCollector) IncEviction(reason base.EvictionReason) {}

// AddEvictions does nothing.
func (n *NoOpCollector) AddEvictions(reason base.EvictionReason, count int64) {}

// IncHit does nothing.
func (n *NoOpCollector) IncHit() {}

// AddHits does nothing.
func (n *NoOpCollector) AddHits(count int64) {}

// IncMiss does nothing.
func (n *NoOpCollector) IncMiss() {}

// AddMisses does nothing.
func (n *NoOpCollector) AddMisses(count int64) {}

// UpdateSizeBytes does nothing.
func (n *NoOpCollector) UpdateSizeBytes(sizeBytes int64) {}

// UpdateLength does nothing.
func (n *NoOpCollector) UpdateLength(length int64) {}
