package sharded

import (
	"testing"

	"github.com/samber/hot/pkg/base"
	"github.com/samber/hot/pkg/lru"
	"github.com/stretchr/testify/assert"
)

func TestNewShardedInMemoryCache(t *testing.T) {
	is := assert.New(t)

	cache := NewShardedInMemoryCache(
		42,
		func(shardIndex int) base.InMemoryCache[int, int] {
			return lru.NewLRUCache[int, int](42)
		},
		func(i int) uint64 {
			return uint64(i * 2)
		},
	)
	c, ok := cache.(*ShardedInMemoryCache[int, int])
	is.True(ok)
	is.Len(c.caches, 42)
	is.Equal(uint64(42), c.shards)
	is.NotNil(c.fn)
	is.Equal(uint64(0), c.fn(0))
	is.Equal(uint64(42), c.fn(21))
	is.Equal(uint64(44), c.fn(22))
	is.Equal(uint64(0), c.fn.computeHash(0, 42))
	is.Equal(uint64(0), c.fn.computeHash(21, 42))
	is.Equal(uint64(2), c.fn.computeHash(22, 42))
}

func TestShardedInMemoryCache_BasicOperations(t *testing.T) {
	is := assert.New(t)

	// Create a simple hasher that distributes keys evenly
	hasher := func(s string) uint64 {
		return uint64(len(s))
	}

	cache := NewShardedInMemoryCache(
		2,
		func(shardIndex int) base.InMemoryCache[string, int] {
			return lru.NewLRUCache[string, int](10)
		},
		hasher,
	)

	// Test Set and Get
	cache.Set("a", 100)   // hash = 1 % 2 = 1
	cache.Set("aa", 200)  // hash = 2 % 2 = 0
	cache.Set("aaa", 300) // hash = 3 % 2 = 1

	value, ok := cache.Get("a")
	is.True(ok)
	is.Equal(100, value)

	value, ok = cache.Get("aa")
	is.True(ok)
	is.Equal(200, value)

	value, ok = cache.Get("aaa")
	is.True(ok)
	is.Equal(300, value)

	// Test Has
	is.True(cache.Has("a"))
	is.True(cache.Has("aa"))
	is.True(cache.Has("aaa"))
	is.False(cache.Has("b"))

	// Test Peek
	value, ok = cache.Peek("a")
	is.True(ok)
	is.Equal(100, value)

	// Test Delete
	is.True(cache.Delete("a"))
	is.False(cache.Delete("a")) // already deleted
	is.False(cache.Has("a"))
}

func TestShardedInMemoryCache_BatchOperations(t *testing.T) {
	is := assert.New(t)

	hasher := func(s string) uint64 {
		return uint64(len(s))
	}

	cache := NewShardedInMemoryCache(
		2,
		func(shardIndex int) base.InMemoryCache[string, int] {
			return lru.NewLRUCache[string, int](10)
		},
		hasher,
	)

	// Test SetMany
	items := map[string]int{
		"a":   100, // shard 1
		"aa":  200, // shard 0
		"aaa": 300, // shard 1
		"b":   400, // shard 1
	}
	cache.SetMany(items)

	// Test HasMany
	keys := []string{"a", "aa", "aaa", "b", "missing"}
	hasResults := cache.HasMany(keys)
	is.True(hasResults["a"])
	is.True(hasResults["aa"])
	is.True(hasResults["aaa"])
	is.True(hasResults["b"])
	is.False(hasResults["missing"])

	// Test GetMany
	found, missing := cache.GetMany(keys)
	is.Len(found, 4)
	is.Len(missing, 1)
	is.Equal(100, found["a"])
	is.Equal(200, found["aa"])
	is.Equal(300, found["aaa"])
	is.Equal(400, found["b"])
	is.Equal("missing", missing[0])

	// Test PeekMany
	found, missing = cache.PeekMany(keys)
	is.Len(found, 4)
	is.Len(missing, 1)

	// Test DeleteMany
	deleteKeys := []string{"a", "aa", "missing"}
	deleteResults := cache.DeleteMany(deleteKeys)
	is.True(deleteResults["a"])
	is.True(deleteResults["aa"])
	is.False(deleteResults["missing"])
}

func TestShardedInMemoryCache_KeysAndValues(t *testing.T) {
	is := assert.New(t)

	hasher := func(s string) uint64 {
		return uint64(len(s))
	}

	cache := NewShardedInMemoryCache(
		2,
		func(shardIndex int) base.InMemoryCache[string, int] {
			return lru.NewLRUCache[string, int](10)
		},
		hasher,
	)

	cache.Set("a", 100)   // shard 1
	cache.Set("aa", 200)  // shard 0
	cache.Set("aaa", 300) // shard 1

	keys := cache.Keys()
	is.Len(keys, 3)
	is.Contains(keys, "a")
	is.Contains(keys, "aa")
	is.Contains(keys, "aaa")

	values := cache.Values()
	is.Len(values, 3)
	is.Contains(values, 100)
	is.Contains(values, 200)
	is.Contains(values, 300)
}

func TestInternalState_All(t *testing.T) {
	is := assert.New(t)

	cache := NewShardedInMemoryCache[string, int](2, func(shardIndex int) base.InMemoryCache[string, int] {
		return lru.NewLRUCache[string, int](10)
	}, func(s string) uint64 {
		return uint64(len(s))
	})
	cache.Set("a", 1)
	cache.Set("b", 2)

	all := cache.All()
	is.Len(all, 2)
	is.Equal(1, all["a"])
	is.Equal(2, all["b"])
}

func TestShardedInMemoryCache_Range(t *testing.T) {
	is := assert.New(t)

	hasher := func(s string) uint64 {
		return uint64(len(s))
	}

	cache := NewShardedInMemoryCache(
		2,
		func(shardIndex int) base.InMemoryCache[string, int] {
			return lru.NewLRUCache[string, int](10)
		},
		hasher,
	)

	cache.Set("a", 100)   // shard 1
	cache.Set("aa", 200)  // shard 0
	cache.Set("aaa", 300) // shard 1

	visited := make(map[string]int)
	cache.Range(func(k string, v int) bool {
		visited[k] = v
		return true
	})

	is.Len(visited, 3)
	is.Equal(100, visited["a"])
	is.Equal(200, visited["aa"])
	is.Equal(300, visited["aaa"])
}

func TestShardedInMemoryCache_Purge(t *testing.T) {
	is := assert.New(t)

	hasher := func(s string) uint64 {
		return uint64(len(s))
	}

	cache := NewShardedInMemoryCache(
		2,
		func(shardIndex int) base.InMemoryCache[string, int] {
			return lru.NewLRUCache[string, int](10)
		},
		hasher,
	)

	cache.Set("a", 100)
	cache.Set("aa", 200)

	is.Equal(2, cache.Len())

	cache.Purge()

	is.Equal(0, cache.Len())
	is.False(cache.Has("a"))
	is.False(cache.Has("aa"))
}

func TestShardedInMemoryCache_EmptyOperations(t *testing.T) {
	is := assert.New(t)

	hasher := func(s string) uint64 {
		return uint64(len(s))
	}

	cache := NewShardedInMemoryCache(
		2,
		func(shardIndex int) base.InMemoryCache[string, int] {
			return lru.NewLRUCache[string, int](10)
		},
		hasher,
	)

	// Test empty SetMany
	cache.SetMany(map[string]int{})
	is.Equal(0, cache.Len())

	// Test empty HasMany
	hasResults := cache.HasMany([]string{})
	is.Len(hasResults, 0)

	// Test empty GetMany
	found, missing := cache.GetMany([]string{})
	is.Len(found, 0)
	is.Len(missing, 0)

	// Test empty PeekMany
	found, missing = cache.PeekMany([]string{})
	is.Len(found, 0)
	is.Len(missing, 0)

	// Test empty DeleteMany
	deleteResults := cache.DeleteMany([]string{})
	is.Len(deleteResults, 0)
}

func TestShardedInMemoryCache_CapacityAndAlgorithm(t *testing.T) {
	is := assert.New(t)

	hasher := func(s string) uint64 {
		return uint64(len(s))
	}

	cache := NewShardedInMemoryCache(
		3,
		func(shardIndex int) base.InMemoryCache[string, int] {
			return lru.NewLRUCache[string, int](10)
		},
		hasher,
	)

	// Test capacity (should be capacity per shard * number of shards)
	is.Equal(30, cache.Capacity())

	// Test algorithm (should be the same as underlying cache)
	is.Equal("lru", cache.Algorithm())
}

func TestShardedInMemoryCache_Len(t *testing.T) {
	is := assert.New(t)

	hasher := func(s string) uint64 {
		return uint64(len(s))
	}

	cache := NewShardedInMemoryCache(
		2,
		func(shardIndex int) base.InMemoryCache[string, int] {
			return lru.NewLRUCache[string, int](10)
		},
		hasher,
	)

	// Initially empty
	is.Equal(0, cache.Len())

	// Add items to different shards
	cache.Set("a", 100) // shard 1
	is.Equal(1, cache.Len())

	cache.Set("aa", 200) // shard 0
	is.Equal(2, cache.Len())

	cache.Set("aaa", 300) // shard 1
	is.Equal(3, cache.Len())

	// Delete items
	cache.Delete("a")
	is.Equal(2, cache.Len())

	cache.Delete("aa")
	is.Equal(1, cache.Len())

	cache.Delete("aaa")
	is.Equal(0, cache.Len())
}

func TestShardedInMemoryCache_InterfaceCompliance(t *testing.T) {
	is := assert.New(t)

	hasher := func(s string) uint64 {
		return uint64(len(s))
	}

	cache := NewShardedInMemoryCache(
		2,
		func(shardIndex int) base.InMemoryCache[string, int] {
			return lru.NewLRUCache[string, int](10)
		},
		hasher,
	)

	// Test that we can assign it to the interface type
	var cacheInterface = cache

	// Test operations through the interface
	cacheInterface.Set("test", 42)
	value, ok := cacheInterface.Get("test")
	is.True(ok)
	is.Equal(42, value)
}

func TestShardedInMemoryCache_SingleShard(t *testing.T) {
	is := assert.New(t)

	hasher := func(s string) uint64 {
		return uint64(len(s))
	}

	cache := NewShardedInMemoryCache(
		1, // single shard
		func(shardIndex int) base.InMemoryCache[string, int] {
			return lru.NewLRUCache[string, int](10)
		},
		hasher,
	)

	// All keys should go to the same shard
	cache.Set("a", 100)
	cache.Set("aa", 200)
	cache.Set("aaa", 300)

	is.Equal(3, cache.Len())
	is.Equal(10, cache.Capacity()) // single shard capacity

	keys := cache.Keys()
	is.Len(keys, 3)
}

func TestShardedInMemoryCache_ManyShards(t *testing.T) {
	is := assert.New(t)

	hasher := func(s string) uint64 {
		return uint64(len(s))
	}

	cache := NewShardedInMemoryCache(
		10, // many shards
		func(shardIndex int) base.InMemoryCache[string, int] {
			return lru.NewLRUCache[string, int](100)
		},
		hasher,
	)

	// Add items that will be distributed across shards
	for i := 0; i < 20; i++ {
		key := string(rune('a' + i))
		cache.Set(key, i*10)
	}

	is.Equal(20, cache.Len())
	is.Equal(1000, cache.Capacity()) // 100 shards * 10 capacity each

	// Verify all items can be retrieved
	for i := 0; i < 20; i++ {
		key := string(rune('a' + i))
		value, ok := cache.Get(key)
		is.True(ok)
		is.Equal(i*10, value)
	}
}
