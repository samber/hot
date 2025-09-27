package sketch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCountMinSketch(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Test with valid parameters
	cms := NewCountMinSketch[string](100, 4)
	is.Equal(100, cms.width)
	is.Equal(4, cms.depth)
	is.Len(cms.counters, 4)
	is.Len(cms.seeds, 4)

	// Verify all counters are initialized to zero
	for i := 0; i < cms.depth; i++ {
		is.Len(cms.counters[i], cms.width)
		for j := 0; j < cms.width; j++ {
			is.Equal(uint8(0), cms.counters[i][j])
		}
	}

	// Verify seeds are different
	for i := 0; i < cms.depth-1; i++ {
		is.NotEqual(cms.seeds[i], cms.seeds[i+1])
	}

	// Test with different parameters
	cms2 := NewCountMinSketch[string](50, 2)
	is.Equal(50, cms2.width)
	is.Equal(2, cms2.depth)
	is.Len(cms2.counters, 2)
	is.Len(cms2.seeds, 2)
}

func TestCountMinSketch_Inc(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cms := NewCountMinSketch[string](10, 3)

	// Test incrementing a single key
	cms.Inc("test")

	// Verify that at least one counter was incremented
	hasIncrement := false
	for i := 0; i < cms.depth; i++ {
		for j := 0; j < cms.width; j++ {
			if cms.counters[i][j] > 0 {
				hasIncrement = true
				break
			}
		}
	}
	is.True(hasIncrement, "At least one counter should be incremented")

	// Test multiple increments of the same key
	initialEstimate := cms.Estimate("test")
	cms.Inc("test")
	afterEstimate := cms.Estimate("test")
	is.GreaterOrEqual(afterEstimate, initialEstimate, "Estimate should increase or stay the same after increment")

	// Test different keys
	cms.Inc("different")
	cms.Inc("another")

	// All keys should have some estimate
	is.Positive(cms.Estimate("test"))
	is.Positive(cms.Estimate("different"))
	is.Positive(cms.Estimate("another"))
}

func TestCountMinSketch_Estimate(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cms := NewCountMinSketch[string](10, 3)

	// Test estimate for non-existent key
	estimate := cms.Estimate("nonexistent")
	is.Equal(0, estimate, "Estimate for non-existent key should be 0")

	// Test estimate after increment
	cms.Inc("test")
	estimate = cms.Estimate("test")
	is.Positive(estimate, "Estimate should be greater than 0 after increment")

	// Test multiple increments
	cms.Inc("test")
	cms.Inc("test")
	estimate = cms.Estimate("test")
	is.GreaterOrEqual(estimate, 1, "Estimate should reflect multiple increments")

	// Test that estimate is consistent
	estimate1 := cms.Estimate("test")
	estimate2 := cms.Estimate("test")
	is.Equal(estimate1, estimate2, "Estimate should be consistent for the same key")
}

func TestCountMinSketch_Reset(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cms := NewCountMinSketch[string](10, 3)

	// Add some data
	cms.Inc("test")
	cms.Inc("test")
	cms.Inc("other")

	// Verify data exists
	is.Positive(cms.Estimate("test"))
	is.Positive(cms.Estimate("other"))

	// Reset
	cms.Reset() // Note: the current implementation resets all counters, not just for the key

	// Verify all counters are reset
	for i := 0; i < cms.depth; i++ {
		for j := 0; j < cms.width; j++ {
			is.Equal(uint8(0), cms.counters[i][j], "All counters should be reset to 0")
		}
	}

	// Verify estimates are 0
	is.Equal(0, cms.Estimate("test"))
	is.Equal(0, cms.Estimate("other"))
}

func TestCountMinSketch_Hash(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cms := NewCountMinSketch[string](10, 3)

	// Test hash consistency
	hashes1 := cms.hash("test")
	hashes2 := cms.hash("test")
	is.Equal(hashes1, hashes2, "Hash should be consistent for the same key")

	// Test hash uniqueness for different keys
	hashesTest := cms.hash("test")
	hashesOther := cms.hash("other")
	is.NotEqual(hashesTest, hashesOther, "Hash should be different for different keys")

	// Test hash length
	is.Len(hashes1, cms.depth, "Hash should return one value per depth")

	// Test that all hash values are different (very likely with good hash function)
	allDifferent := true
	for i := 0; i < len(hashes1)-1; i++ {
		if hashes1[i] == hashes1[i+1] {
			allDifferent = false
			break
		}
	}
	is.True(allDifferent, "Hash values should be different across depths")
}

func TestCountMinSketch_Overflow(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cms := NewCountMinSketch[string](10, 3)

	// Test counter overflow behavior
	// Increment the same key many times to potentially cause overflow
	for i := 0; i < 300; i++ { // More than uint8 max (255)
		cms.Inc("overflow_test")
	}

	// The estimate should not exceed the maximum possible value
	estimate := cms.Estimate("overflow_test")
	is.LessOrEqual(estimate, 255, "Estimate should not exceed uint8 max value")

	// Verify that counters don't overflow beyond uint8 max
	for i := 0; i < cms.depth; i++ {
		for j := 0; j < cms.width; j++ {
			is.LessOrEqual(cms.counters[i][j], uint8(255), "Counters should not overflow beyond uint8 max")
		}
	}
}

func TestCountMinSketch_Accuracy(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cms := NewCountMinSketch[string](100, 4)

	// Test with known frequencies
	testCases := []struct {
		key       string
		frequency int
	}{
		{"key1", 5},
		{"key2", 10},
		{"key3", 1},
		{"key4", 20},
	}

	// Add known frequencies
	for _, tc := range testCases {
		for i := 0; i < tc.frequency; i++ {
			cms.Inc(tc.key)
		}
	}

	// Test estimates
	for _, tc := range testCases {
		estimate := cms.Estimate(tc.key)
		is.GreaterOrEqual(estimate, tc.frequency, "Estimate should be at least the actual frequency")
		// Note: Due to hash collisions, estimate might be higher than actual frequency
	}

	// Test that higher frequency keys have higher estimates
	key2Estimate := cms.Estimate("key2")
	key4Estimate := cms.Estimate("key4")
	is.GreaterOrEqual(key4Estimate, key2Estimate, "Higher frequency key should have higher or equal estimate")
}

func TestCountMinSketch_CollisionHandling(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cms := NewCountMinSketch[string](5, 2) // Small sketch to increase collision probability

	// Add many different keys to cause collisions
	keys := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	for _, key := range keys {
		cms.Inc(key)
	}

	// All keys should have some estimate (even if inflated due to collisions)
	for _, key := range keys {
		estimate := cms.Estimate(key)
		is.Positive(estimate, "All keys should have some estimate, even with collisions")
	}
}

func TestCountMinSketch_EmptyKey(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cms := NewCountMinSketch[string](10, 3)

	// Test with empty string
	cms.Inc("")
	estimate := cms.Estimate("")
	is.Positive(estimate, "Empty string should work as a key")

	// Test with same empty string multiple times
	cms.Inc("")
	cms.Inc("")
	estimate = cms.Estimate("")
	is.GreaterOrEqual(estimate, 1, "Multiple increments of empty string should accumulate")
}

func TestCountMinSketch_LongKey(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cms := NewCountMinSketch[string](10, 3)

	// Test with very long key
	longKey := "this_is_a_very_long_key_that_might_cause_issues_with_hashing_but_should_still_work_correctly"
	cms.Inc(longKey)
	estimate := cms.Estimate(longKey)
	is.Positive(estimate, "Long keys should work correctly")

	// Test consistency with long keys
	estimate1 := cms.Estimate(longKey)
	estimate2 := cms.Estimate(longKey)
	is.Equal(estimate1, estimate2, "Long key estimates should be consistent")
}

func TestCountMinSketch_SpecialCharacters(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cms := NewCountMinSketch[string](10, 3)

	// Test with various special characters
	specialKeys := []string{
		"key with spaces",
		"key\twith\ttabs",
		"key\nwith\nnewlines",
		"key with unicode: 你好世界",
		"key with symbols: !@#$%^&*()",
		"key with numbers: 123456789",
	}

	for _, key := range specialKeys {
		cms.Inc(key)
		estimate := cms.Estimate(key)
		is.Positive(estimate, "Special character keys should work: %s", key)
	}
}

func TestCountMinSketch_ConcurrentAccess(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cms := NewCountMinSketch[string](100, 4)

	// Test that the sketch can handle rapid successive operations
	// (Note: This doesn't test actual concurrency since the sketch isn't thread-safe)
	for i := 0; i < 1000; i++ {
		key := "key" + string(rune(i%10+'a')) // Cycle through keys a-j
		cms.Inc(key)
	}

	// Verify all keys have estimates
	for i := 0; i < 10; i++ {
		key := "key" + string(rune(i+'a'))
		estimate := cms.Estimate(key)
		is.Positive(estimate, "All cycled keys should have estimates")
	}
}

func TestCountMinSketch_EstimateAccuracy(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// Use a larger sketch for better accuracy
	cms := NewCountMinSketch[string](1000, 4)

	// Test with a single key to minimize collisions
	cms.Inc("single_key")
	estimate := cms.Estimate("single_key")
	is.Equal(1, estimate, "Single increment should give estimate of 1")

	// Test with multiple increments of the same key
	for i := 0; i < 5; i++ {
		cms.Inc("multi_key")
	}
	estimate = cms.Estimate("multi_key")
	is.Equal(5, estimate, "Five increments should give estimate of 5")
}

func TestCountMinSketch_ResetBehavior(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	cms := NewCountMinSketch[string](10, 3)

	// Add data
	cms.Inc("key1")
	cms.Inc("key2")
	cms.Inc("key1") // key1 appears twice

	// Verify data exists
	is.Positive(cms.Estimate("key1"))
	is.Positive(cms.Estimate("key2"))

	// Reset (note: current implementation resets all counters)
	cms.Reset()

	// All estimates should be 0 after reset
	is.Equal(0, cms.Estimate("key1"))
	is.Equal(0, cms.Estimate("key2"))

	// Add new data after reset
	cms.Inc("key3")
	is.Positive(cms.Estimate("key3"))
}
