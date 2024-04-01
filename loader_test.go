package hot

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoaders_run(t *testing.T) {
	is := assert.New(t)

	counter := int32(0)

	// without error
	loaders := LoaderChain[int, int]{
		func(keys []int) (map[int]int, error) {
			atomic.AddInt32(&counter, 1)
			return map[int]int{1: 1, 2: 2}, nil
		},
		func(keys []int) (map[int]int, error) {
			atomic.AddInt32(&counter, 1)
			return map[int]int{2: 42, 3: 3}, nil
		},
	}

	results, missing, err := loaders.run([]int{1, 2, 3, 4})

	is.EqualValues(map[int]int{1: 1, 2: 42, 3: 3}, results)
	is.EqualValues([]int{4}, missing)
	is.Nil(err)
	is.EqualValues(2, atomic.LoadInt32(&counter))

	// with error
	loaders = LoaderChain[int, int]{
		func(keys []int) (map[int]int, error) {
			atomic.AddInt32(&counter, 1)
			return map[int]int{1: 1, 2: 2}, nil
		},
		func(keys []int) (map[int]int, error) {
			atomic.AddInt32(&counter, 1)
			return map[int]int{2: 42, 3: 3}, nil
		},
		func(keys []int) (map[int]int, error) {
			atomic.AddInt32(&counter, 1)
			return map[int]int{4: 4}, assert.AnError
		},
	}

	results, missing, err = loaders.run([]int{1, 2, 3, 4})

	is.EqualValues(map[int]int{}, results)
	is.EqualValues([]int{}, missing)
	is.ErrorIs(assert.AnError, err)
	is.EqualValues(5, atomic.LoadInt32(&counter))
}
