package sketch

import (
	"fmt"
	"hash/fnv"
	"math"
)

// DoorkeeperCountMinSketch extends CountMinSketch with Doorkeeper optimization.
// The Doorkeeper is a Bloom filter that tracks singletons to avoid updating
// the Count-Min Sketch for items seen only once, improving efficiency.
type DoorkeeperCountMinSketch[K comparable] struct {
	width         int       // Width of each hash table
	depth         int       // Number of hash functions
	counters      [][]uint8 // 2D array of counters
	seeds         []uint64  // Seeds for hash functions

	// Doorkeeper Bloom filter for singleton tracking
	doorkeeper    []uint64  // Bloom filter bit array
	doorkeeperSeeds []uint64 // Seeds for doorkeeper hash functions
	doorkeeperSize int      // Size of doorkeeper bloom filter
}

// NewDoorkeeperCountMinSketch creates a new Count-Min Sketch with Doorkeeper optimization.
// According to TinyLFU paper, this combination provides better efficiency for singleton tracking.
func NewDoorkeeperCountMinSketch[K comparable](width, depth int) *DoorkeeperCountMinSketch[K] {
	cms := &DoorkeeperCountMinSketch[K]{
		width:         width,
		depth:         depth,
		counters:      make([][]byte, depth),
		seeds:         make([]uint64, depth),
		doorkeeperSeeds: make([]uint64, 4), // 4 hash functions for doorkeeper
	}

	// Initialize Count-Min Sketch counters
	for i := range cms.counters {
		cms.counters[i] = make([]byte, cms.width)
	}

	// Initialize Count-Min Sketch seeds
	for i := 0; i < depth; i++ {
		cms.seeds[i] = uint64(i * 1000)
	}

	// Initialize Doorkeeper Bloom filter
	// Size doorkeeper to be approximately the same as CMS for efficiency
	cms.doorkeeperSize = width * depth / 8 // Optimal size from TinyLFU paper
	if cms.doorkeeperSize < 64 {
		cms.doorkeeperSize = 64
	}
	cms.doorkeeper = make([]uint64, (cms.doorkeeperSize+63)/64) // Round up to 64-bit words

	for i := 0; i < 4; i++ {
		cms.doorkeeperSeeds[i] = uint64(i * 2000 + 1000)
	}

	return cms
}

// Inc increments the count for the given key using Doorkeeper optimization.
// If the key is a singleton (first time seen), it only tracks it in the doorkeeper.
// Only on second access does it increment the Count-Min Sketch counters.
func (cms *DoorkeeperCountMinSketch[K]) Inc(key K) {
	if cms.isInDoorkeeper(key) {
		// Key already seen at least once, increment CMS counters
		hashes := cms.hash(key)
		for i := 0; i < cms.depth; i++ {
			slot := hashes[i] % uint64(cms.width)
			if cms.counters[i][slot] < math.MaxUint8 {
				cms.counters[i][slot]++
			}
		}
	} else {
		// First time seeing this key, add to doorkeeper
		cms.addToDoorkeeper(key)
	}
}

// Estimate returns the estimated count for the given key.
// For items in doorkeeper only, returns 1. For others, uses CMS estimate.
func (cms *DoorkeeperCountMinSketch[K]) Estimate(key K) int {
	if !cms.isInDoorkeeper(key) {
		return 0
	}

	// Check if it's only in doorkeeper (count = 1) or also in CMS (count >= 2)
	hashes := cms.hash(key)
	minCount := math.MaxUint8

	for i := 0; i < cms.depth; i++ {
		slot := hashes[i] % uint64(cms.width)
		if int(cms.counters[i][slot]) < minCount {
			minCount = int(cms.counters[i][slot])
		}
	}

	// If CMS shows 0, it's only in doorkeeper (count = 1)
	// If CMS shows > 0, actual count is CMS + 1
	if minCount == 0 {
		return 1
	}
	return minCount + 1
}

// Reset resets both the counters and doorkeeper to 0.
func (cms *DoorkeeperCountMinSketch[K]) Reset() {
	// Reset Count-Min Sketch counters
	for i := 0; i < cms.depth; i++ {
		for j := 0; j < cms.width; j++ {
			cms.counters[i][j] = 0
		}
	}

	// Reset Doorkeeper Bloom filter
	for i := range cms.doorkeeper {
		cms.doorkeeper[i] = 0
	}
}

// isInDoorkeeper checks if the key exists in the doorkeeper bloom filter.
func (cms *DoorkeeperCountMinSketch[K]) isInDoorkeeper(key K) bool {
	hashes := cms.doorkeeperHash(key)

	for _, hash := range hashes {
		wordIdx := hash % uint64(cms.doorkeeperSize)
		bitIdx := wordIdx % 64
		word := cms.doorkeeper[wordIdx/64]

		if (word & (1 << bitIdx)) == 0 {
			return false
		}
	}

	return true
}

// addToDoorkeeper adds the key to the doorkeeper bloom filter.
func (cms *DoorkeeperCountMinSketch[K]) addToDoorkeeper(key K) {
	hashes := cms.doorkeeperHash(key)

	for _, hash := range hashes {
		wordIdx := hash % uint64(cms.doorkeeperSize)
		bitIdx := wordIdx % 64
		wordIdx /= 64

		cms.doorkeeper[wordIdx] |= 1 << bitIdx
	}
}

// hash generates hash values for Count-Min Sketch using FNV-1a.
func (cms *DoorkeeperCountMinSketch[K]) hash(key K) []uint64 {
	keyString := fmt.Sprintf("%v", key)

	hashes := make([]uint64, cms.depth)
	for i := 0; i < cms.depth; i++ {
		h := fnv.New64a()
		_, _ = fmt.Fprintf(h, "%v%d", keyString, cms.seeds[i])
		hashes[i] = h.Sum64()
	}

	return hashes
}

// doorkeeperHash generates hash values for Doorkeeper Bloom filter using FNV-1a.
func (cms *DoorkeeperCountMinSketch[K]) doorkeeperHash(key K) []uint64 {
	keyString := fmt.Sprintf("%v", key)

	hashes := make([]uint64, 4)
	for i := 0; i < 4; i++ {
		h := fnv.New64a()
		_, _ = fmt.Fprintf(h, "%v%d", keyString, cms.doorkeeperSeeds[i])
		hashes[i] = h.Sum64()
	}

	return hashes
}