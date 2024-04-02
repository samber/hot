package sharded

import (
	"testing"

	"github.com/samber/hot/base"
	"github.com/samber/hot/lru"
	"github.com/stretchr/testify/assert"
)

func TestNewShardedInMemoryCache(t *testing.T) {
	is := assert.New(t)

	cache := NewShardedInMemoryCache[int, int](
		42,
		func() base.InMemoryCache[int, int] {
			return lru.NewLRUCache[int, int](42)
		},
		func(i int) uint16 {
			return uint16(i * 2)
		},
	)
	c, ok := cache.(*ShardedInMemoryCache[int, int])
	is.True(ok)
	is.Len(c.caches, 42)
	is.Equal(uint16(42), c.shards)
	is.NotNil(c.fn)
	is.Equal(uint16(0), c.fn(0))
	is.Equal(uint16(42), c.fn(21))
	is.Equal(uint16(44), c.fn(22))
}
