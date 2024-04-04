package bench

import (
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

//
// This file aims to compare the performance of different implementations of internal stuff.
// For example: time.Now() vs syscall.Gettimeofday(), or std linked list vs custom.
//

// go test -benchmem -benchtime=100000000x -bench=Time
func BenchmarkDevelTime(b *testing.B) {
	b.Run("TimeGo", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			_ = time.Now()
		}
	})

	// syscal.Gettimeofday is faster than time.Now()
	b.Run("TimeSyscall", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var tv syscall.Timeval
			_ = syscall.Gettimeofday(&tv)
		}
	})
}

type counter struct {
	value uint64
}

func (m *counter) Inc() {
	m.value++
}

type mock struct{}

func (m *mock) Inc() {
	// do nothing
}

// go test -benchmem -benchtime=100000000x -bench=Counter
func BenchmarkDevelCounter(b *testing.B) {
	b.Run("Inc", func(b *testing.B) {
		counter := uint64(0)
		for n := 0; n < b.N; n++ {
			counter++
		}
	})

	b.Run("AtomicInc", func(b *testing.B) {
		counter := uint64(0)
		for n := 0; n < b.N; n++ {
			atomic.AddUint64(&counter, 1)
		}
	})

	b.Run("EncapsulatedCounter", func(b *testing.B) {
		c := counter{0}
		for n := 0; n < b.N; n++ {
			c.Inc()
		}
	})

	b.Run("EncapsulatedMockCounter", func(b *testing.B) {
		c := mock{}
		for n := 0; n < b.N; n++ {
			c.Inc()
		}
	})
}
