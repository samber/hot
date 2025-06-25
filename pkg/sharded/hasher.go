package sharded

// Hasher is responsible for generating an unsigned 64-bit hash of the provided key.
// The hash function should minimize collisions to ensure even distribution across shards.
// For optimal performance, a fast hash function is preferable since it's called on every cache operation.
// The hash function should be deterministic - the same key should always produce the same hash.
type Hasher[K any] func(key K) uint64

// computeHash computes the target shard index for a given key.
// It applies the hash function to the key and then uses modulo operation
// to map the hash to a valid shard index in the range [0, shards-1].
// This ensures that keys are distributed evenly across all available shards.
func (fn Hasher[K]) computeHash(key K, shards uint64) uint64 {
	return fn(key) % shards
}
