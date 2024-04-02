package sharded

import (
	"testing"

	"github.com/samber/hot/pkg/base"
	"github.com/samber/hot/pkg/lru"
	"github.com/stretchr/testify/assert"
)

func TestNewShardedInMemoryCache(t *testing.T) {
	is := assert.New(t)

	cache := NewShardedInMemoryCache[int, int](
		42,
		func() base.InMemoryCache[int, int] {
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
}
