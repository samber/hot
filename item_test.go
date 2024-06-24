package hot

import (
	"testing"
	"time"

	"github.com/samber/hot/internal"
	"github.com/stretchr/testify/assert"
)

func TestNewItem(t *testing.T) {
	is := assert.New(t)

	// no value without ttl
	got := newItem[int64](0, false, 0, 0)
	is.EqualValues(&item[int64]{false, 0, 0, 0, 0}, got)
	got = newItem[int64](42, false, 0, 0)
	is.EqualValues(&item[int64]{false, 0, 0, 0, 0}, got)

	// no value with ttl
	got = newItem[int64](0, false, 2_000, 1_000)
	is.False(got.hasValue)
	is.Equal(int64(0), got.value)
	is.InEpsilon(internal.NowMicro()+2_000, got.expiryMicro, 100)
	is.InEpsilon(internal.NowMicro()+2_000+1_000, got.staleExpiryMicro, 100)

	// has value without ttl
	is.EqualValues(&item[int64]{true, 42, 8, 0, 0}, newItem[int64](42, true, 0, 0))

	// has value with ttl
	got = newItem[int64](42, true, 2_000, 1_000)
	is.True(got.hasValue)
	is.Equal(int64(42), got.value)
	is.InEpsilon(internal.NowMicro()+2_000, got.expiryMicro, 100)
	is.InEpsilon(internal.NowMicro()+2_000+1_000, got.staleExpiryMicro, 100)

	// size
	is.EqualValues(&item[map[string]int]{true, map[string]int{"a": 1, "b": 2}, 79, 0, 0}, newItem(map[string]int{"a": 1, "b": 2}, true, 0, 0))
	is.EqualValues(&item[*item[int64]]{true, &item[int64]{false, 0, 0, 0, 0}, 40, 0, 0}, newItem(newItem[int64](42, false, 0, 0), true, 0, 0))
}

func TestNewItemWithValue(t *testing.T) {
	is := assert.New(t)

	is.Equal(&item[int64]{true, int64(42), 8, 0, 0}, newItemWithValue(int64(42), 0, 0))

	item := newItemWithValue(int64(42), 2_000, 1_000)
	is.True(item.hasValue)
	is.Equal(int64(42), item.value)
	is.InEpsilon(internal.NowMicro()+2_000, item.expiryMicro, 100)
	is.InEpsilon(internal.NowMicro()+2_000+1_000, item.staleExpiryMicro, 100)
}

func TestNewItemNoValue(t *testing.T) {
	is := assert.New(t)

	is.Equal(&item[int64]{false, 0, 0, 0, 0}, newItemNoValue[int64](0, 0))

	item := newItemNoValue[int](2_000_000, 1_000_000)
	is.False(item.hasValue)
	is.Equal(0, item.value)
	is.InEpsilon(internal.NowMicro()+2_000, item.expiryMicro, 100)
	is.InEpsilon(internal.NowMicro()+2_000+1_000, item.staleExpiryMicro, 100)
}

func TestItem_isExpired(t *testing.T) {
	is := assert.New(t)

	got := newItemNoValue[int64](0, 0)
	is.False(got.isExpired(internal.NowMicro()))

	got = newItemNoValue[int64](-1_000, 0)
	is.True(got.isExpired(internal.NowMicro()))

	got = newItemNoValue[int64](1_000, 0)
	is.False(got.isExpired(internal.NowMicro()))

	got = newItemNoValue[int64](-1_000, 800)
	is.True(got.isExpired(internal.NowMicro()))

	got = newItemNoValue[int64](-1_000, 1_200)
	is.False(got.isExpired(internal.NowMicro()))
}

func TestItem_shouldRevalidate(t *testing.T) {
	is := assert.New(t)

	got := newItemNoValue[int64](0, 0)
	is.False(got.shouldRevalidate(internal.NowMicro()))

	got = newItemNoValue[int64](-1_000, 0)
	is.False(got.shouldRevalidate(internal.NowMicro()))

	got = newItemNoValue[int64](1_000, 0)
	is.False(got.shouldRevalidate(internal.NowMicro()))

	got = newItemNoValue[int64](-1_000, 800)
	is.False(got.shouldRevalidate(internal.NowMicro()))

	got = newItemNoValue[int64](-1_000, 1_200)
	is.True(got.shouldRevalidate(internal.NowMicro()))
}

func TestItemMapsToValues(t *testing.T) {
	is := assert.New(t)

	twice := func(i int) int { return i * 2 }
	itemNo := newItem(0, false, 0, 0)
	itemA := newItem(42, true, 0, 0)
	itemB := newItem(21, true, 0, 0)

	// no map
	gotFound, gotMissing := itemMapsToValues[string, int](nil)
	is.EqualValues(map[string]int{}, gotFound)
	is.EqualValues([]string{}, gotMissing)

	// no map
	gotFound, gotMissing = itemMapsToValues[string, int](twice)
	is.EqualValues(map[string]int{}, gotFound)
	is.EqualValues([]string{}, gotMissing)

	// has map
	gotFound, gotMissing = itemMapsToValues[string, int](nil, map[string]*item[int]{"a": itemA, "b": itemB, "c": itemNo})
	is.EqualValues(map[string]int{"a": 42, "b": 21}, gotFound)
	is.EqualValues([]string{"c"}, gotMissing)
	gotFound, gotMissing = itemMapsToValues[string, int](nil, map[string]*item[int]{"a": itemA}, map[string]*item[int]{"b": itemB, "c": itemNo, "a": itemNo})
	is.EqualValues(map[string]int{"a": 42, "b": 21}, gotFound)
	is.EqualValues([]string{"c"}, gotMissing)

	// has map
	gotFound, gotMissing = itemMapsToValues[string, int](twice, map[string]*item[int]{"a": itemA, "b": itemB, "c": itemNo})
	is.EqualValues(map[string]int{"a": 84, "b": 42}, gotFound)
	is.EqualValues([]string{"c"}, gotMissing)
	gotFound, gotMissing = itemMapsToValues[string, int](twice, map[string]*item[int]{"a": itemA}, map[string]*item[int]{"b": itemB, "c": itemNo, "a": itemNo})
	is.EqualValues(map[string]int{"a": 84, "b": 42}, gotFound)
	is.EqualValues([]string{"c"}, gotMissing)
}

func TestApplyJitter(t *testing.T) {
	is := assert.New(t)

	// no jitter
	is.Equal(int64(1_000), applyJitter(1_000, 0, 0))
	is.Equal(int64(1_000), applyJitter(1_000, 0, time.Second))
	is.Equal(int64(1_000), applyJitter(1_000, 0.5, 0))

	// with jitter
	is.InEpsilon(1_000, applyJitter(1_000, 3, 100*time.Millisecond), 100)
	is.InEpsilon(1_000, applyJitter(1_000, 3, 100*time.Millisecond), 100)
	is.InEpsilon(1_000, applyJitter(1_000, 3, 100*time.Millisecond), 100)
	is.InEpsilon(1_000, applyJitter(1_000, 3, 100*time.Millisecond), 100)
}
