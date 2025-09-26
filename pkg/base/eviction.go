package base

// EvictionCallback is a function type that is called when an item is evicted from the cache.
// The callback receives the key and value of the evicted item.
// This is useful for cleanup operations, logging, or monitoring when items are removed from the cache.
// Common use cases include:
// - Cleaning up resources associated with cached values
// - Logging eviction events for debugging or monitoring
// - Updating external metrics or statistics
// - Triggering background operations when items are removed
// The callback is called synchronously during the eviction process, so it should be fast
// to avoid blocking cache operations. For expensive operations, consider using a goroutine.
type EvictionCallback[K comparable, V any] func(EvictionReason, K, V)

// EvictionReason is a type that represents the reason for eviction.
type EvictionReason string

const (
	EvictionReasonCapacity EvictionReason = "capacity"
	EvictionReasonTTL      EvictionReason = "ttl"
	EvictionReasonManual   EvictionReason = "manual"
	EvictionReasonStale    EvictionReason = "stale"
)

// EvictionReasons is a list of all eviction reasons.
var EvictionReasons = []EvictionReason{
	EvictionReasonCapacity,
	EvictionReasonTTL,
	EvictionReasonManual,
	EvictionReasonStale,
}
