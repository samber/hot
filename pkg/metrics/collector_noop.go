package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var _ Collector = (*NoOpCollector)(nil)

// NoOpCollector is a no-op implementation of Collector that does nothing.
// This provides better performance than conditional checks when metrics are disabled.
type NoOpCollector struct{}

func (n *NoOpCollector) IncInsertion()                                   {}
func (n *NoOpCollector) AddInsertions(count int64)                       {}
func (n *NoOpCollector) IncEviction(reason EvictionReason)               {}
func (n *NoOpCollector) AddEvictions(reason EvictionReason, count int64) {}
func (n *NoOpCollector) IncHit()                                         {}
func (n *NoOpCollector) AddHits(count int64)                             {}
func (n *NoOpCollector) IncMiss()                                        {}
func (n *NoOpCollector) AddMisses(count int64)                           {}
func (n *NoOpCollector) SetSizeBytes(bytes int64)                        {}
func (n *NoOpCollector) Collect(ch chan<- prometheus.Metric)             {}
