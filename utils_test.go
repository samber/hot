package hot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEmptyableToPtr(t *testing.T) {
	is := assert.New(t)

	// Test with zero values
	is.Nil(emptyableToPtr(0))
	is.Nil(emptyableToPtr(""))
	is.Nil(emptyableToPtr(false))
	is.Nil(emptyableToPtr(0.0))
	is.Nil(emptyableToPtr(time.Duration(0)))

	// Test with non-zero values
	intPtr := emptyableToPtr(42)
	is.NotNil(intPtr)
	is.Equal(42, *intPtr)

	strPtr := emptyableToPtr("hello")
	is.NotNil(strPtr)
	is.Equal("hello", *strPtr)

	boolPtr := emptyableToPtr(true)
	is.NotNil(boolPtr)
	is.Equal(true, *boolPtr)

	floatPtr := emptyableToPtr(3.14)
	is.NotNil(floatPtr)
	is.Equal(3.14, *floatPtr)

	durationPtr := emptyableToPtr(time.Second)
	is.NotNil(durationPtr)
	is.Equal(time.Second, *durationPtr)

	slicePtr := emptyableToPtr([]int{1, 2, 3})
	is.NotNil(slicePtr)
	is.Equal([]int{1, 2, 3}, *slicePtr)

	mapPtr := emptyableToPtr(map[string]int{"a": 1, "b": 2})
	is.NotNil(mapPtr)
	is.Equal(map[string]int{"a": 1, "b": 2}, *mapPtr)

	// Test with struct types
	type testStruct struct {
		Field string
	}

	zeroStruct := testStruct{}
	is.Nil(emptyableToPtr(zeroStruct))

	nonZeroStruct := testStruct{Field: "value"}
	structPtr := emptyableToPtr(nonZeroStruct)
	is.NotNil(structPtr)
	is.Equal(nonZeroStruct, *structPtr)

	// Test with pointer types
	var nilPtr *int
	is.Nil(emptyableToPtr(nilPtr))

	nonNilPtr := &[]int{1, 2, 3}
	ptrPtr := emptyableToPtr(nonNilPtr)
	is.NotNil(ptrPtr)
	is.Equal(nonNilPtr, *ptrPtr)
}
