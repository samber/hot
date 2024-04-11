package bench

import (
	randCrypto "crypto/rand"
	"math/big"
	randv1 "math/rand"
	randv2 "math/rand/v2"
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

// go test -benchmem -benchtime=100000000x -bench=Rand
func BenchmarkDevelRand(b *testing.B) {
	b.Run("MathRandV1Float", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			randv1.Float64()
		}
	})

	b.Run("MathRandV1Int", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			randv1.Int()
		}
	})

	b.Run("MathRandV1Int31n", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			randv1.Int31n(100)
		}
	})

	b.Run("MathRandV2Float", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			randv2.Float64()
		}
	})

	b.Run("MathRandV2Int", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			randv2.Int()
		}
	})

	b.Run("MathRandV2Int32N", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			randv2.Int32N(100)
		}
	})

	b.Run("CryptoRand", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			cryptoRand()
		}
	})
}

// GOMAXPROCS=1 go test -benchmem -benchtime=100000000x -bench=Rand
// GOMAXPROCS=4 go test -benchmem -benchtime=100000000x -bench=Rand
func BenchmarkDevelRandParallel(b *testing.B) {
	b.Run("MathRandV1Float", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				randv1.Float64()
				randv1.Float64()
				randv1.Float64()
				randv1.Float64()
			}
		})
	})

	b.Run("MathRandV1Int", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				randv1.Int()
				randv1.Int()
				randv1.Int()
				randv1.Int()
			}
		})
	})

	b.Run("MathRandV1Int31n", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				randv1.Int()
				randv1.Int()
				randv1.Int()
				randv1.Int()
			}
		})
	})

	b.Run("MathRandV2Float", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				randv1.Int()
				randv1.Int()
				randv1.Int()
				randv1.Int()
			}
		})
	})

	b.Run("MathRandV2Int", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				randv1.Int()
				randv1.Int()
				randv1.Int()
				randv1.Int()
			}
		})
	})

	b.Run("MathRandV2Int32N", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				randv2.Int32N(100)
				randv2.Int32N(100)
				randv2.Int32N(100)
				randv2.Int32N(100)
			}
		})
	})

	b.Run("CryptoRand", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				cryptoRand()
			}
		})
	})
}

func cryptoRand() {
	// https://brandur.org/fragments/crypto-rand-float64
	nBig, err := randCrypto.Int(randCrypto.Reader, big.NewInt(1<<53))
	if err != nil {
		panic(err)
	}

	_ = float64(nBig.Int64()) / (1 << 53)
}
