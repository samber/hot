package sharded

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasher(t *testing.T) {
	is := assert.New(t)

	hasher := Hasher[int](func(i int) uint16 {
		return uint16(i * 2)
	})
	is.Equal(uint16(0), hasher.computeHash(0, 42))
	is.Equal(uint16(40), hasher.computeHash(20, 42))
	is.Equal(uint16(0), hasher.computeHash(21, 42))
	is.Equal(uint16(2), hasher.computeHash(22, 42))
}
