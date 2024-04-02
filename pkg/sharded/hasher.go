package sharded

// Hasher is responsible for generating unsigned, 16 bit hash of provided key.
// Hasher should minimize collisions. For great performance, a fast function is preferable.
type Hasher[K any] func(K) uint16

func (fn Hasher[K]) computeHash(key K, shards uint16) uint16 {
	return fn(key) % shards
}
