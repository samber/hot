package metrics

type EvictionReason string

const (
	EvictionReasonCapacity EvictionReason = "capacity"
	EvictionReasonTTL      EvictionReason = "ttl"
	EvictionReasonManual   EvictionReason = "manual"
	EvictionReasonStale    EvictionReason = "stale"
)

var EvictionReasons = []EvictionReason{
	EvictionReasonCapacity,
	EvictionReasonTTL,
	EvictionReasonManual,
	EvictionReasonStale,
}
