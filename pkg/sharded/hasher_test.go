package sharded

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasher(t *testing.T) {
	is := assert.New(t)

	hasher := Hasher[int](func(i int) uint64 {
		return uint64(i * 2)
	})
	is.Equal(uint64(0), hasher.computeHash(0, 42))
	is.Equal(uint64(40), hasher.computeHash(20, 42))
	is.Equal(uint64(0), hasher.computeHash(21, 42))
	is.Equal(uint64(2), hasher.computeHash(22, 42))
}
