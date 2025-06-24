package base

// EvictionCallback is a function type that is called when an item is evicted from the cache.
// The callback receives the key and value of the evicted item.
// This is useful for cleanup operations or logging when items are removed from the cache.
type EvictionCallback[K comparable, V any] func(K, V)
