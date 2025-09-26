package hot

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/samber/hot/internal"
)

// newItem creates a new cache item with the specified value, TTL, and stale duration.
// If hasValue is false, it creates a missing key item.
func newItem[V any](v V, hasValue bool, ttlNano int64, staleNano int64) *item[V] {
	if hasValue {
		return newItemWithValue(v, ttlNano, staleNano)
	}
	return newItemNoValue[V](ttlNano, staleNano)
}

// newItemWithValue creates a new cache item with a value, TTL, and stale duration.
// The expiry times are calculated based on the current time plus the provided durations.
func newItemWithValue[V any](v V, ttlNano int64, staleNano int64) *item[V] {
	var expiryNano int64
	var staleExpiryNano int64
	if ttlNano != 0 {
		// @TODO: Current time should be passed as an argument to make it faster in batch operations
		expiryNano = internal.NowNano() + ttlNano
		staleExpiryNano = expiryNano + staleNano
	}

	return &item[V]{
		hasValue: true,
		value:    v,
		// bytes:            uint(size.Of(v)),
		expiryNano:      expiryNano,
		staleExpiryNano: staleExpiryNano,
	}
}

// newItemNoValue creates a new cache item for a missing key with TTL and stale duration.
// The expiry times are calculated based on the current time plus the provided durations.
func newItemNoValue[V any](ttlNano int64, staleNano int64) *item[V] {
	var expiryNano int64
	var staleExpiryNano int64
	if ttlNano != 0 {
		// @TODO: Current time should be passed as an argument to make it faster in batch operations
		expiryNano = internal.NowNano() + ttlNano
		staleExpiryNano = expiryNano + staleNano
	}

	return &item[V]{
		hasValue:        false,
		expiryNano:      expiryNano,
		staleExpiryNano: staleExpiryNano,
	}
}

// item represents a cache entry that can hold either a value or represent a missing key.
// It stores expiry times in nanoseconds for better performance.
type item[V any] struct {
	hasValue bool
	value    V
	// bytes    uint
	// Store int64 nanoseconds instead of time.Time for better performance
	// (benchmark resulted in 10x speedup)
	expiryNano      int64
	staleExpiryNano int64
}

// isExpired checks if the item has expired based on the current time.
// An item is expired if it has a TTL and the current time is past the stale expiry time.
func (i *item[V]) isExpired(nowNano int64) bool {
	return i.expiryNano > 0 && nowNano > i.staleExpiryNano
}

// shouldRevalidate checks if the item should be revalidated based on the current time.
// An item should be revalidated if it has a TTL, the current time is past the expiry time,
// but not yet past the stale expiry time.
func (i *item[V]) shouldRevalidate(nowNano int64) bool {
	return i.expiryNano > 0 && nowNano > i.expiryNano && nowNano < i.staleExpiryNano
}

// zero returns the zero value for type V.
func zero[V any]() V {
	var v V
	return v
}

// itemMapsToValues converts maps of items to maps of values and missing keys.
// It applies copyOnRead function if provided and filters out missing keys.
func itemMapsToValues[K comparable, V any](copyOnRead func(V) V, maps ...map[K]*item[V]) (found map[K]V, missing []K) {
	found = map[K]V{}
	missing = []K{}

	for _, m := range maps {
		for k, v := range m {
			if v.hasValue {
				if copyOnRead != nil {
					found[k] = copyOnRead(v.value)
				} else {
					found[k] = v.value
				}
			} else if _, ok := found[k]; !ok {
				// Do not append to missing slice if already present in `found`
				missing = append(missing, k)
			}
		}
	}

	return found, missing
}

// applyJitter applies exponential jitter to the TTL duration.
// If jitterLambda is 0 or jitterUpperBound is 0, the original TTL is returned unchanged.
// The jitter follows an exponential distribution in the range [0, upperBoundDuration).
func applyJitter(ttlNano int64, jitterLambda float64, jitterUpperBound time.Duration) int64 {
	if jitterLambda == 0 || jitterUpperBound == 0 {
		return ttlNano
	}

	u := float64(jitterUpperBound.Nanoseconds()) * rand.Float64()
	variation := 1 - math.Exp(-jitterLambda*u)
	return int64(float64(ttlNano) * variation)
}
