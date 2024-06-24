package hot

import (
	"math"
	"math/rand"
	"time"

	"github.com/DmitriyVTitov/size"
	"github.com/samber/hot/internal"
)

func newItem[V any](v V, hasValue bool, ttlMicro int64, staleMicro int64) *item[V] {
	if hasValue {
		return newItemWithValue(v, ttlMicro, staleMicro)
	}
	return newItemNoValue[V](ttlMicro, staleMicro)
}

func newItemWithValue[V any](v V, ttlMicro int64, staleMicro int64) *item[V] {
	var expiryMicro int64
	var staleExpiryMicro int64
	if ttlMicro != 0 {
		// @TODO: current time should be passed as an argument to make it faster in batch operations
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

func newItemNoValue[V any](ttlMicro int64, staleMicro int64) *item[V] {
	var expiryMicro int64
	var staleExpiryMicro int64
	if ttlMicro != 0 {
		// @TODO: current time should be passed as an argument to make it faster in batch operations
		expiryMicro = int64(internal.NowMicro()) + ttlMicro
		staleExpiryMicro = expiryMicro + staleMicro
	}

	return &item[V]{
		hasValue:         false,
		expiryMicro:      expiryMicro,
		staleExpiryMicro: staleExpiryMicro,
	}
}

type item[V any] struct {
	hasValue bool
	value    V
	bytes    uint
	// Better store int64 microseconds instead of time.Time (benchmark resulted in 10x speedup).
	expiryMicro      int64
	staleExpiryMicro int64
}

func (i *item[V]) isExpired(nowMicro int64) bool {
	return i.expiryMicro > 0 && nowMicro > i.staleExpiryMicro
}

func (i *item[V]) shouldRevalidate(nowMicro int64) bool {
	return i.expiryMicro > 0 && nowMicro > i.expiryMicro && nowMicro < i.staleExpiryMicro
}

func zero[V any]() V {
	var v V
	return v
}

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
				// do not append to missing slice if already present in `found`
				missing = append(missing, k)
			}
		}
	}

	return
}

func applyJitter(ttlMicro int64, jitterLambda float64, jitterUpperBound time.Duration) int64 {
	if jitterLambda == 0 || jitterUpperBound == 0 {
		return ttlMicro
	}

	u := float64(jitterUpperBound.Microseconds()) * rand.Float64()
	variation := 1 - math.Exp(-jitterLambda*u)
	return int64(float64(ttlMicro) * variation)
}
