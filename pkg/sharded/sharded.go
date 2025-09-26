package sharded

import (
	"github.com/samber/hot/internal"
	"github.com/samber/hot/pkg/base"
)

// NewShardedInMemoryCache creates a sharded cache that distributes keys across multiple
// underlying cache instances for better concurrency performance.
// The cache uses a hash function to determine which shard each key belongs to.
// Each shard is an independent cache instance that can be accessed concurrently.
func NewShardedInMemoryCache[K comparable, V any](shards uint64, newCache func(shardIndex int) base.InMemoryCache[K, V], fn Hasher[K]) base.InMemoryCache[K, V] {
	// Create the specified number of cache shards
	caches := make([]base.InMemoryCache[K, V], shards)
	for i := uint64(0); i < shards; i++ {
		caches[i] = newCache(int(i))
	}

	return &ShardedInMemoryCache[K, V]{
		shards: shards,
		fn:     fn,
		caches: caches,
	}
}

// ShardedInMemoryCache is a cache that distributes data across multiple shards
// to improve concurrency performance by reducing lock contention.
// Each shard is an independent cache instance that can be accessed concurrently
// without interfering with other shards.
type ShardedInMemoryCache[K comparable, V any] struct {
	noCopy internal.NoCopy // Prevents accidental copying of the cache

	shards uint64                     // Number of shards (must be > 1)
	fn     Hasher[K]                  // Hash function to determine shard for each key
	caches []base.InMemoryCache[K, V] // Array of cache shards
}

// Ensure ShardedInMemoryCache implements InMemoryCache interface.
var _ base.InMemoryCache[string, int] = (*ShardedInMemoryCache[string, int])(nil)

// Set stores a key-value pair in the appropriate shard based on the key's hash.
// The key is hashed to determine which shard to use, providing O(1) average case performance.
func (c *ShardedInMemoryCache[K, V]) Set(key K, value V) {
	c.caches[c.fn.computeHash(key, c.shards)].Set(key, value)
}

// Has checks if a key exists in the appropriate shard based on the key's hash.
// Returns true if the key exists in the cache, false otherwise.
func (c *ShardedInMemoryCache[K, V]) Has(key K) bool {
	return c.caches[c.fn.computeHash(key, c.shards)].Has(key)
}

// Get retrieves a value from the appropriate shard based on the key's hash.
// Returns the value and a boolean indicating if the key was found.
func (c *ShardedInMemoryCache[K, V]) Get(key K) (value V, ok bool) {
	return c.caches[c.fn.computeHash(key, c.shards)].Get(key)
}

// Peek retrieves a value from the appropriate shard without updating access order.
// Returns the value and a boolean indicating if the key was found.
func (c *ShardedInMemoryCache[K, V]) Peek(key K) (value V, ok bool) {
	return c.caches[c.fn.computeHash(key, c.shards)].Peek(key)
}

// Keys returns all keys from all shards combined into a single slice.
// The order of keys in the returned slice is not guaranteed.
// Time complexity: O(n) where n is the total number of keys across all shards.
func (c *ShardedInMemoryCache[K, V]) Keys() []K {
	keys := []K{}
	for i := range c.caches {
		keys = append(keys, c.caches[i].Keys()...)
	}
	return keys
}

// Values returns all values from all shards combined into a single slice.
// The order of values in the returned slice is not guaranteed.
// Time complexity: O(n) where n is the total number of values across all shards.
func (c *ShardedInMemoryCache[K, V]) Values() []V {
	values := []V{}
	for i := range c.caches {
		values = append(values, c.caches[i].Values()...)
	}
	return values
}

// All returns all key-value pairs from all shards.
// The order of key-value pairs in the returned map is not guaranteed.
func (c *ShardedInMemoryCache[K, V]) All() map[K]V {
	all := make(map[K]V)
	for i := range c.caches {
		for k, v := range c.caches[i].All() {
			all[k] = v
		}
	}
	return all
}

// Range iterates over all key-value pairs from all shards.
// The iteration stops if the function returns false.
// The iteration order is not guaranteed and may not be consistent across calls.
func (c *ShardedInMemoryCache[K, V]) Range(f func(K, V) bool) {
	ok := true
	for i := range c.caches {
		c.caches[i].Range(func(k K, v V) bool {
			ok = f(k, v)
			return ok
		})
		if !ok {
			return
		}
	}
}

// Delete removes a key from the appropriate shard based on the key's hash.
// Returns true if the key was found and removed, false otherwise.
func (c *ShardedInMemoryCache[K, V]) Delete(key K) bool {
	return c.caches[c.fn.computeHash(key, c.shards)].Delete(key)
}

// Purge removes all keys and values from all shards.
// This operation clears all cache shards simultaneously.
func (c *ShardedInMemoryCache[K, V]) Purge() {
	for i := range c.caches {
		c.caches[i].Purge()
	}
}

// SetMany stores multiple key-value pairs by grouping them by shard for efficiency.
// Keys are hashed and grouped by their target shard, then each shard is updated
// with its respective key-value pairs in a single batch operation.
// If the input map is empty, the operation is skipped entirely.
func (c *ShardedInMemoryCache[K, V]) SetMany(items map[K]V) {
	if len(items) == 0 {
		return
	}

	// Group items by their target shard
	batch := map[uint64]map[K]V{}
	for k, v := range items {
		hash := c.fn.computeHash(k, c.shards)
		if batch[hash] == nil {
			batch[hash] = map[K]V{}
		}
		batch[hash][k] = v
	}

	// Update each shard with its grouped items
	for i := range batch {
		c.caches[i].SetMany(batch[i])
	}
}

// HasMany checks if multiple keys exist by grouping them by shard for efficiency.
// Keys are hashed and grouped by their target shard, then each shard is queried
// for its respective keys in a single batch operation.
// Returns a map where keys are the input keys and values indicate existence.
// If the input slice is empty, returns an empty map immediately.
func (c *ShardedInMemoryCache[K, V]) HasMany(keys []K) map[K]bool {
	if len(keys) == 0 {
		return map[K]bool{}
	}

	// Group keys by their target shard
	batch := map[uint64][]K{}
	for _, k := range keys {
		hash := c.fn.computeHash(k, c.shards)
		if batch[hash] == nil {
			batch[hash] = []K{}
		}
		batch[hash] = append(batch[hash], k)
	}

	// Query each shard and combine results
	output := map[K]bool{}
	for i := range batch {
		local := c.caches[i].HasMany(batch[i])
		for k, v := range local {
			output[k] = v
		}
	}

	return output
}

// GetMany retrieves multiple values by grouping keys by shard for efficiency.
// Keys are hashed and grouped by their target shard, then each shard is queried
// for its respective keys in a single batch operation.
// Returns a map of found key-value pairs and a slice of missing keys.
// If the input slice is empty, returns empty results immediately.
func (c *ShardedInMemoryCache[K, V]) GetMany(keys []K) (map[K]V, []K) {
	if len(keys) == 0 {
		return map[K]V{}, []K{}
	}

	// Group keys by their target shard
	batch := map[uint64][]K{}
	for _, k := range keys {
		hash := c.fn.computeHash(k, c.shards)
		if batch[hash] == nil {
			batch[hash] = []K{}
		}
		batch[hash] = append(batch[hash], k)
	}

	// Query each shard and combine results
	output := map[K]V{}
	missing := []K{}

	for i := range batch {
		localFound, localMissing := c.caches[i].GetMany(batch[i])
		for k, v := range localFound {
			output[k] = v
		}
		missing = append(missing, localMissing...)
	}

	return output, missing
}

// PeekMany retrieves multiple values without updating access order by grouping keys by shard.
// Keys are hashed and grouped by their target shard, then each shard is queried
// for its respective keys in a single batch operation.
// Returns a map of found key-value pairs and a slice of missing keys.
// If the input slice is empty, returns empty results immediately.
func (c *ShardedInMemoryCache[K, V]) PeekMany(keys []K) (map[K]V, []K) {
	if len(keys) == 0 {
		return map[K]V{}, []K{}
	}

	// Group keys by their target shard
	batch := map[uint64][]K{}
	for _, k := range keys {
		hash := c.fn.computeHash(k, c.shards)
		if batch[hash] == nil {
			batch[hash] = []K{}
		}
		batch[hash] = append(batch[hash], k)
	}

	// Query each shard and combine results
	output := map[K]V{}
	missing := []K{}

	for i := range batch {
		localFound, localMissing := c.caches[i].PeekMany(batch[i])
		for k, v := range localFound {
			output[k] = v
		}
		missing = append(missing, localMissing...)
	}

	return output, missing
}

// DeleteMany removes multiple keys by grouping them by shard for efficiency.
// Keys are hashed and grouped by their target shard, then each shard is updated
// with its respective keys in a single batch operation.
// Returns a map where keys are the input keys and values indicate if the key was found and removed.
// If the input slice is empty, returns an empty map immediately.
func (c *ShardedInMemoryCache[K, V]) DeleteMany(keys []K) map[K]bool {
	if len(keys) == 0 {
		return map[K]bool{}
	}

	// Group keys by their target shard
	batch := map[uint64][]K{}
	for _, k := range keys {
		hash := c.fn.computeHash(k, c.shards)
		if batch[hash] == nil {
			batch[hash] = []K{}
		}
		batch[hash] = append(batch[hash], k)
	}

	// Update each shard and combine results
	output := map[K]bool{}
	for i := range batch {
		local := c.caches[i].DeleteMany(batch[i])
		for k, v := range local {
			output[k] = v
		}
	}

	return output
}

// Capacity returns the total capacity across all shards.
// This is the sum of the capacities of all individual cache shards.
func (c *ShardedInMemoryCache[K, V]) Capacity() int {
	total := 0
	for i := range c.caches {
		total += c.caches[i].Capacity()
	}
	return total
}

// Algorithm returns the name of the eviction algorithm used by the cache.
// This returns "sharded" to indicate the sharding mechanism.
func (c *ShardedInMemoryCache[K, V]) Algorithm() string {
	return c.caches[0].Algorithm()
}

// Len returns the total number of items across all shards.
// Time complexity: O(n) where n is the number of shards.
func (c *ShardedInMemoryCache[K, V]) Len() int {
	total := 0
	for i := range c.caches {
		total += c.caches[i].Len()
	}
	return total
}

// SizeBytes returns the total size of all cache entries in bytes across all shards.
// Time complexity: O(n) where n is the number of shards.
func (c *ShardedInMemoryCache[K, V]) SizeBytes() int64 {
	total := int64(0)
	for i := range c.caches {
		total += c.caches[i].SizeBytes()
	}
	return total
}
