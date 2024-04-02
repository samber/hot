package bench

import (
	"syscall"
	"testing"
	"time"
)

//
// This file aims to compare the performance of different implementations of internal stuff.
// For example: time.Now() vs syscall.Gettimeofday(), or std linked list vs custom.
//

// go test -benchmem -benchtime=100000000x -bench=Time
func BenchmarkTime(b *testing.B) {
	b.Run("TimeGo", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			_ = time.Now()
		}
	})

	// syscal.Gettimeofday is faster than time.Now()
	b.Run("TimeSyscall", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var tv syscall.Timeval
			syscall.Gettimeofday(&tv)
		}
	})
}
