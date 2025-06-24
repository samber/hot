package hot

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/DmitriyVTitov/size"
	"github.com/samber/hot/internal"
)

// newItem creates a new cache item with the specified value, TTL, and stale duration.
// If hasValue is false, it creates a missing key item.
func newItem[V any](v V, hasValue bool, ttlMicro int64, staleMicro int64) *item[V] {
	if hasValue {
		return newItemWithValue(v, ttlMicro, staleMicro)
	}
	return newItemNoValue[V](ttlMicro, staleMicro)
}

// newItemWithValue creates a new cache item with a value, TTL, and stale duration.
// The expiry times are calculated based on the current time plus the provided durations.
func newItemWithValue[V any](v V, ttlMicro int64, staleMicro int64) *item[V] {
	var expiryMicro int64
	var staleExpiryMicro int64
	if ttlMicro != 0 {
		// @TODO: Current time should be passed as an argument to make it faster in batch operations
		expiryMicro = int64(internal.NowMicro()) + ttlMicro
		staleExpiryMicro = expiryMicro + staleMicro
	}

	return &item[V]{
		hasValue:         true,
		value:            v,
		bytes:            uint(size.Of(v)),
		expiryMicro:      expiryMicro,
		staleExpiryMicro: staleExpiryMicro,
	}
}

// newItemNoValue creates a new cache item for a missing key with TTL and stale duration.
// The expiry times are calculated based on the current time plus the provided durations.
func newItemNoValue[V any](ttlMicro int64, staleMicro int64) *item[V] {
	var expiryMicro int64
	var staleExpiryMicro int64
	if ttlMicro != 0 {
		// @TODO: Current time should be passed as an argument to make it faster in batch operations
		expiryMicro = int64(internal.NowMicro()) + ttlMicro
		staleExpiryMicro = expiryMicro + staleMicro
	}

	return &item[V]{
		hasValue:         false,
		expiryMicro:      expiryMicro,
		staleExpiryMicro: staleExpiryMicro,
	}
}

// item represents a cache entry that can hold either a value or represent a missing key.
// It stores expiry times in microseconds for better performance.
type item[V any] struct {
	hasValue bool
	value    V
	bytes    uint
	// Store int64 microseconds instead of time.Time for better performance
	// (benchmark resulted in 10x speedup)
	expiryMicro      int64
	staleExpiryMicro int64
}

// isExpired checks if the item has expired based on the current time.
// An item is expired if it has a TTL and the current time is past the stale expiry time.
func (i *item[V]) isExpired(nowMicro int64) bool {
	return i.expiryMicro > 0 && nowMicro > i.staleExpiryMicro
}

// shouldRevalidate checks if the item should be revalidated based on the current time.
// An item should be revalidated if it has a TTL, the current time is past the expiry time,
// but not yet past the stale expiry time.
func (i *item[V]) shouldRevalidate(nowMicro int64) bool {
	return i.expiryMicro > 0 && nowMicro > i.expiryMicro && nowMicro < i.staleExpiryMicro
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

	return
}

// applyJitter applies exponential jitter to the TTL duration.
// If jitterLambda is 0 or jitterUpperBound is 0, the original TTL is returned unchanged.
// The jitter follows an exponential distribution in the range [0, upperBoundDuration).
func applyJitter(ttlMicro int64, jitterLambda float64, jitterUpperBound time.Duration) int64 {
	if jitterLambda == 0 || jitterUpperBound == 0 {
		return ttlMicro
	}

	u := float64(jitterUpperBound.Microseconds()) * rand.Float64()
	variation := 1 - math.Exp(-jitterLambda*u)
	return int64(float64(ttlMicro) * variation)
}
