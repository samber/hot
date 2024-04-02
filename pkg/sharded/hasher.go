package sharded

// Hasher is responsible for generating unsigned, 16 bit hash of provided key.
// Hasher should minimize collisions. For great performance, a fast function is preferable.
type Hasher[K any] func(K) uint64

func (fn Hasher[K]) computeHash(key K, shards uint64) uint64 {
	return fn(key) % shards
}
