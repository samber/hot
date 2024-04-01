package hot

import (
	"math/rand/v2"

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
	var expiry int64
	var staleExpiry int64
	if ttlMicro != 0 {
		expiry = int64(internal.NowMicro()) + ttlMicro
		staleExpiry = expiry + staleMicro
	}

	return &item[V]{
		hasValue:         true,
		value:            v,
		bytes:            uint(size.Of(v)),
		expiryMicro:      expiry,
		staleExpiryMicro: staleExpiry,
	}
}

func newItemNoValue[V any](ttlMicro int64, staleMicro int64) *item[V] {
	var expiry int64
	var staleExpiry int64
	if ttlMicro != 0 {
		expiry = int64(internal.NowMicro()) + ttlMicro
		staleExpiry = expiry + staleMicro
	}

	return &item[V]{
		hasValue:         false,
		expiryMicro:      expiry,
		staleExpiryMicro: staleExpiry,
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

func itemSlicesToValues[V any](copyOnRead func(V) V, slices ...[]*item[V]) []V {
	values := []V{}

	for _, s := range slices {
		for _, v := range s {
			if v.hasValue {
				if copyOnRead != nil {
					values = append(values, copyOnRead(v.value))
				} else {
					values = append(values, v.value)
				}
			}
		}
	}

	return values
}

func applyJitter(ttlMicro int64, jitter float64) int64 {
	if jitter == 0 {
		return ttlMicro
	}

	variation := (rand.Float64() * (jitter * 2)) - jitter
	return int64(float64(ttlMicro) * (1 + variation))
}
