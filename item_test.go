package hot

import (
	"testing"
	"time"

	"github.com/samber/hot/internal"
	"github.com/stretchr/testify/assert"
)

func TestNewItem(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// no value without ttl
	got := newItem[int64](0, false, 0, 0)
	is.Equal(&item[int64]{false, 0, 0, 0}, got)
	got = newItem[int64](42, false, 0, 0)
	is.Equal(&item[int64]{false, 0, 0, 0}, got)

	// no value with ttl
	got = newItem[int64](0, false, 2_000, 1_000)
	is.False(got.hasValue)
	is.Equal(int64(0), got.value)
	is.InEpsilon(internal.NowNano()+2_000_000, got.expiryNano, 100_000)
	is.InEpsilon(internal.NowNano()+2_000_000+1_000_000, got.staleExpiryNano, 100_000)

	// has value without ttl
	is.Equal(&item[int64]{true, 42, 0, 0}, newItem[int64](42, true, 0, 0))

	// has value with ttl
	got = newItem[int64](42, true, 2_000, 1_000)
	is.True(got.hasValue)
	is.Equal(int64(42), got.value)
	is.InEpsilon(internal.NowNano()+2_000_000, got.expiryNano, 100_000)
	is.InEpsilon(internal.NowNano()+2_000_000+1_000_000, got.staleExpiryNano, 100_000)

	// size
	is.Equal(&item[map[string]int]{true, map[string]int{"a": 1, "b": 2}, 0, 0}, newItem(map[string]int{"a": 1, "b": 2}, true, 0, 0))
	is.Equal(&item[*item[int64]]{true, &item[int64]{false, 0, 0, 0}, 0, 0}, newItem(newItem[int64](42, false, 0, 0), true, 0, 0))
}

func TestNewItemWithValue(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	is.Equal(&item[int64]{true, int64(42), 0, 0}, newItemWithValue(int64(42), 0, 0))

	item := newItemWithValue(int64(42), 2_000, 1_000)
	is.True(item.hasValue)
	is.Equal(int64(42), item.value)
	is.InEpsilon(internal.NowNano()+2_000_000, item.expiryNano, 100_000)
	is.InEpsilon(internal.NowNano()+2_000_000+1_000_000, item.staleExpiryNano, 100_000)
}

func TestNewItemNoValue(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	is.Equal(&item[int64]{false, 0, 0, 0}, newItemNoValue[int64](0, 0))

	item := newItemNoValue[int](2_000_000, 1_000_000)
	is.False(item.hasValue)
	is.Equal(0, item.value)
	is.InEpsilon(internal.NowNano()+2_000_000, item.expiryNano, 100_000)
	is.InEpsilon(internal.NowNano()+2_000_000+1_000_000, item.staleExpiryNano, 100_000)
}

func TestItem_isExpired(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	got := newItemNoValue[int64](0, 0)
	is.False(got.isExpired(internal.NowNano()))

	got = newItemNoValue[int64](-1_000_000, 0)
	is.True(got.isExpired(internal.NowNano()))

	got = newItemNoValue[int64](1_000_000, 0)
	is.False(got.isExpired(internal.NowNano()))

	got = newItemNoValue[int64](-1_000_000, 800_000)
	is.True(got.isExpired(internal.NowNano()))

	got = newItemNoValue[int64](-1_000_000, 1_200_000)
	is.False(got.isExpired(internal.NowNano()))
}

func TestItem_shouldRevalidate(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	got := newItemNoValue[int64](0, 0)
	is.False(got.shouldRevalidate(internal.NowNano()))

	got = newItemNoValue[int64](-1_000_000, 0)
	is.False(got.shouldRevalidate(internal.NowNano()))

	got = newItemNoValue[int64](1_000_000, 0)
	is.False(got.shouldRevalidate(internal.NowNano()))

	got = newItemNoValue[int64](-1_000_000, 800_000)
	is.False(got.shouldRevalidate(internal.NowNano()))

	got = newItemNoValue[int64](-1_000_000, 1_200_000)
	is.True(got.shouldRevalidate(internal.NowNano()))
}

func TestItemMapsToValues(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	twice := func(i int) int { return i * 2 }
	itemNo := newItem(0, false, 0, 0)
	itemA := newItem(42, true, 0, 0)
	itemB := newItem(21, true, 0, 0)

	// no map
	gotFound, gotMissing := itemMapsToValues[string, int](nil)
	is.Equal(map[string]int{}, gotFound)
	is.Equal([]string{}, gotMissing)

	// no map
	gotFound, gotMissing = itemMapsToValues[string, int](twice)
	is.Equal(map[string]int{}, gotFound)
	is.Equal([]string{}, gotMissing)

	// has map
	gotFound, gotMissing = itemMapsToValues[string, int](nil, map[string]*item[int]{"a": itemA, "b": itemB, "c": itemNo})
	is.Equal(map[string]int{"a": 42, "b": 21}, gotFound)
	is.Equal([]string{"c"}, gotMissing)
	gotFound, gotMissing = itemMapsToValues[string, int](nil, map[string]*item[int]{"a": itemA}, map[string]*item[int]{"b": itemB, "c": itemNo, "a": itemNo})
	is.Equal(map[string]int{"a": 42, "b": 21}, gotFound)
	is.Equal([]string{"c"}, gotMissing)

	// has map
	gotFound, gotMissing = itemMapsToValues[string, int](twice, map[string]*item[int]{"a": itemA, "b": itemB, "c": itemNo})
	is.Equal(map[string]int{"a": 84, "b": 42}, gotFound)
	is.Equal([]string{"c"}, gotMissing)
	gotFound, gotMissing = itemMapsToValues[string, int](twice, map[string]*item[int]{"a": itemA}, map[string]*item[int]{"b": itemB, "c": itemNo, "a": itemNo})
	is.Equal(map[string]int{"a": 84, "b": 42}, gotFound)
	is.Equal([]string{"c"}, gotMissing)
}

func TestApplyJitter(t *testing.T) {
	is := assert.New(t)
	t.Parallel()

	// no jitter
	is.Equal(int64(1_000_000), applyJitter(1_000_000, 0, 0))
	is.Equal(int64(1_000_000), applyJitter(1_000_000, 0, time.Second))
	is.Equal(int64(1_000_000), applyJitter(1_000_000, 0.5, 0))

	// with jitter
	is.InEpsilon(1_000_000, applyJitter(1_000_000, 3, 100*time.Millisecond), 100_000)
	is.InEpsilon(1_000_000, applyJitter(1_000_000, 3, 100*time.Millisecond), 100_000)
	is.InEpsilon(1_000_000, applyJitter(1_000_000, 3, 100*time.Millisecond), 100_000)
	is.InEpsilon(1_000_000, applyJitter(1_000_000, 3, 100*time.Millisecond), 100_000)
}
