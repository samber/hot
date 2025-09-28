package sketch

import (
	"fmt"
	"hash/fnv"
	"math"
)

// According to the paper, Count-Min Sketch is faster than the Spectral Bloom Filters
// but less accurate.

type CountMinSketch[K comparable] struct {
	width    int       // Width of each hash table
	depth    int       // Number of hash functions
	counters [][]uint8 // 2D array of counters
	seeds    []uint64  // Seeds for hash functions
}

// More width = fewer collisions
// More depth = better estimates.
func NewCountMinSketch[K comparable](width, depth int) *CountMinSketch[K] {
	cms := &CountMinSketch[K]{
		width:    width,
		depth:    depth,
		counters: make([][]byte, depth),
		seeds:    make([]uint64, depth),
	}

	for i := range cms.counters {
		cms.counters[i] = make([]byte, cms.width)
	}

	for i := 0; i < depth; i++ {
		cms.seeds[i] = uint64(i * 1000) // Simple seed generation
	}

	return cms
}

// Inc increments the count for the given key.
func (cms *CountMinSketch[K]) Inc(key K) {
	hashes := cms.hash(key)
	for i := 0; i < cms.depth; i++ {
		slot := hashes[i] % uint64(cms.width)
		if cms.counters[i][slot] < math.MaxUint8 {
			cms.counters[i][slot]++
		}
	}
}

// Estimate returns the estimated count for the given key.
func (cms *CountMinSketch[K]) Estimate(key K) int {
	overflow := math.MaxUint8

	hashes := cms.hash(key)
	for i := 0; i < cms.depth; i++ {
		slot := hashes[i] % uint64(cms.width)
		if int(cms.counters[i][slot]) < overflow {
			overflow = int(cms.counters[i][slot])
		}
	}

	return overflow
}

// Reset resets the counters to 0.
func (cms *CountMinSketch[K]) Reset() {
	for i := 0; i < cms.depth; i++ {
		for j := 0; j < cms.width; j++ {
			cms.counters[i][j] = 0
		}
	}
}

func (cms *CountMinSketch[K]) hash(key K) []uint64 {
	keyString := fmt.Sprintf("%v", key)

	hashes := make([]uint64, cms.depth)
	for i := 0; i < cms.depth; i++ {
		h := fnv.New64a()
		_, _ = fmt.Fprintf(h, "%v%d", keyString, cms.seeds[i])
		hashes[i] = h.Sum64()
	}

	return hashes
}
