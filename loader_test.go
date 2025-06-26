package hot

import (
	"errors"
	"sort"
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

func TestLoaders_runEmptyKeys(t *testing.T) {
	is := assert.New(t)

	loaders := LoaderChain[int, int]{
		func(keys []int) (map[int]int, error) {
			return map[int]int{}, nil
		},
	}

	results, missing, err := loaders.run([]int{})
	is.EqualValues(map[int]int{}, results)
	is.EqualValues([]int{}, missing)
	is.Nil(err)
}

func TestLoaders_runNilKeys(t *testing.T) {
	is := assert.New(t)

	loaders := LoaderChain[int, int]{
		func(keys []int) (map[int]int, error) {
			return map[int]int{}, nil
		},
	}

	results, missing, err := loaders.run(nil)
	is.EqualValues(map[int]int{}, results)
	is.EqualValues([]int{}, missing)
	is.Nil(err)
}

func TestLoaders_runEmptyChain(t *testing.T) {
	is := assert.New(t)

	loaders := LoaderChain[int, int]{}

	results, missing, err := loaders.run([]int{1, 2, 3})
	is.EqualValues(map[int]int{}, results)
	sort.Ints(missing)
	is.EqualValues([]int{1, 2, 3}, missing)
	is.Nil(err)
}

func TestLoaders_runSingleLoader(t *testing.T) {
	is := assert.New(t)

	loaders := LoaderChain[int, int]{
		func(keys []int) (map[int]int, error) {
			return map[int]int{1: 10, 2: 20}, nil
		},
	}

	results, missing, err := loaders.run([]int{1, 2, 3})
	is.EqualValues(map[int]int{1: 10, 2: 20}, results)
	is.EqualValues([]int{3}, missing)
	is.Nil(err)
}

func TestLoaders_runAllKeysFound(t *testing.T) {
	is := assert.New(t)

	loaders := LoaderChain[int, int]{
		func(keys []int) (map[int]int, error) {
			return map[int]int{1: 10, 2: 20, 3: 30}, nil
		},
	}

	results, missing, err := loaders.run([]int{1, 2, 3})
	is.EqualValues(map[int]int{1: 10, 2: 20, 3: 30}, results)
	is.EqualValues([]int{}, missing)
	is.Nil(err)
}

func TestLoaders_runErrorOnFirstLoader(t *testing.T) {
	is := assert.New(t)

	loaders := LoaderChain[int, int]{
		func(keys []int) (map[int]int, error) {
			return map[int]int{}, errors.New("first loader error")
		},
		func(keys []int) (map[int]int, error) {
			return map[int]int{1: 10}, nil
		},
	}

	results, missing, err := loaders.run([]int{1, 2, 3})
	is.EqualValues(map[int]int{}, results)
	is.EqualValues([]int{}, missing)
	is.Error(err)
	is.Contains(err.Error(), "first loader error")
}

func TestLoaders_runErrorOnSecondLoader(t *testing.T) {
	is := assert.New(t)

	loaders := LoaderChain[int, int]{
		func(keys []int) (map[int]int, error) {
			return map[int]int{1: 10}, nil
		},
		func(keys []int) (map[int]int, error) {
			return map[int]int{}, errors.New("second loader error")
		},
	}

	results, missing, err := loaders.run([]int{1, 2, 3})
	is.EqualValues(map[int]int{}, results)
	is.EqualValues([]int{}, missing)
	is.Error(err)
	is.Contains(err.Error(), "second loader error")
}

func TestLoaders_runOverwriteValues(t *testing.T) {
	is := assert.New(t)

	loaders := LoaderChain[int, int]{
		func(keys []int) (map[int]int, error) {
			return map[int]int{1: 10, 2: 20}, nil
		},
		func(keys []int) (map[int]int, error) {
			return map[int]int{1: 100, 3: 30}, nil // Overwrites key 1
		},
	}

	results, missing, err := loaders.run([]int{1, 2, 3})
	is.EqualValues(map[int]int{1: 100, 2: 20, 3: 30}, results) // Key 1 is overwritten
	is.EqualValues([]int{}, missing)
	is.Nil(err)
}

func TestLoaders_runPartialResults(t *testing.T) {
	is := assert.New(t)

	loaders := LoaderChain[int, int]{
		func(keys []int) (map[int]int, error) {
			return map[int]int{1: 10}, nil
		},
		func(keys []int) (map[int]int, error) {
			return map[int]int{2: 20}, nil
		},
		func(keys []int) (map[int]int, error) {
			return map[int]int{3: 30}, nil
		},
	}

	results, missing, err := loaders.run([]int{1, 2, 3, 4})
	is.EqualValues(map[int]int{1: 10, 2: 20, 3: 30}, results)
	is.EqualValues([]int{4}, missing)
	is.Nil(err)
}

func TestLoaders_runWithStringKeys(t *testing.T) {
	is := assert.New(t)

	loaders := LoaderChain[string, int]{
		func(keys []string) (map[string]int, error) {
			return map[string]int{"a": 1, "b": 2}, nil
		},
		func(keys []string) (map[string]int, error) {
			return map[string]int{"c": 3}, nil
		},
	}

	results, missing, err := loaders.run([]string{"a", "b", "c", "d"})
	is.EqualValues(map[string]int{"a": 1, "b": 2, "c": 3}, results)
	is.EqualValues([]string{"d"}, missing)
	is.Nil(err)
}
