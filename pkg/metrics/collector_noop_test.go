package metrics

import (
	"sync"
	"testing"

	"github.com/samber/hot/pkg/base"
	"github.com/stretchr/testify/assert"
)

func TestNoOpCollector_InterfaceCompliance(t *testing.T) {
	is := assert.New(t)

	// Verify NoOpCollector implements Collector interface
	var _ Collector = (*NoOpCollector)(nil)

	collector := &NoOpCollector{}
	is.NotNil(collector)
}

func TestNoOpCollector_AllMethods(t *testing.T) {
	is := assert.New(t)

	collector := &NoOpCollector{}

	// Test all methods - they should not panic and should do nothing
	// This test ensures the methods are callable and don't cause any issues

	// Test insertion methods
	is.NotPanics(func() {
		collector.IncInsertion()
	})

	is.NotPanics(func() {
		collector.AddInsertions(5)
	})

	is.NotPanics(func() {
		collector.AddInsertions(0)
	})

	is.NotPanics(func() {
		collector.AddInsertions(-10)
	})

	// Test eviction methods
	is.NotPanics(func() {
		collector.IncEviction(base.EvictionReasonCapacity)
	})

	is.NotPanics(func() {
		collector.IncEviction(base.EvictionReasonTTL)
	})

	is.NotPanics(func() {
		collector.IncEviction(base.EvictionReasonManual)
	})

	is.NotPanics(func() {
		collector.IncEviction(base.EvictionReasonStale)
	})

	is.NotPanics(func() {
		collector.IncEviction(base.EvictionReason("unknown_reason"))
	})

	is.NotPanics(func() {
		collector.AddEvictions(base.EvictionReasonCapacity, 10)
	})

	is.NotPanics(func() {
		collector.AddEvictions(base.EvictionReasonTTL, 0)
	})

	is.NotPanics(func() {
		collector.AddEvictions(base.EvictionReasonManual, -5)
	})

	// Test hit/miss methods
	is.NotPanics(func() {
		collector.IncHit()
	})

	is.NotPanics(func() {
		collector.AddHits(3)
	})

	is.NotPanics(func() {
		collector.AddHits(0)
	})

	is.NotPanics(func() {
		collector.AddHits(-2)
	})

	is.NotPanics(func() {
		collector.IncMiss()
	})

	is.NotPanics(func() {
		collector.AddMisses(7)
	})

	is.NotPanics(func() {
		collector.AddMisses(0)
	})

	is.NotPanics(func() {
		collector.AddMisses(-3)
	})
}

func TestNoOpCollector_ConcurrentAccess(t *testing.T) {
	is := assert.New(t)

	collector := &NoOpCollector{}

	// Test concurrent access to all methods
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

	// NoOpCollector should complete all operations without any issues
	is.True(true, "All concurrent operations completed successfully")
}

func TestNoOpCollector_EdgeCases(t *testing.T) {
	is := assert.New(t)

	collector := &NoOpCollector{}

	// Test with extreme values
	is.NotPanics(func() {
		collector.AddInsertions(9223372036854775807) // Max int64
	})

	is.NotPanics(func() {
		collector.AddInsertions(-9223372036854775808) // Min int64
	})

	is.NotPanics(func() {
		collector.AddHits(9223372036854775807)
	})

	is.NotPanics(func() {
		collector.AddHits(-9223372036854775808)
	})

	is.NotPanics(func() {
		collector.AddMisses(9223372036854775807)
	})

	is.NotPanics(func() {
		collector.AddMisses(-9223372036854775808)
	})

	is.NotPanics(func() {
		collector.AddEvictions(base.EvictionReasonCapacity, 9223372036854775807)
	})

	is.NotPanics(func() {
		collector.AddEvictions(base.EvictionReasonCapacity, -9223372036854775808)
	})

	// Test with empty string eviction reason
	is.NotPanics(func() {
		collector.IncEviction(base.EvictionReason(""))
	})

	// Test with very long eviction reason
	is.NotPanics(func() {
		collector.IncEviction(base.EvictionReason("very_long_eviction_reason_that_might_cause_issues_if_not_handled_properly"))
	})
}
