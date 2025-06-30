package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNowNano(t *testing.T) {
	is := assert.New(t)

	got1 := NowNano()
	is.InEpsilon(time.Now().UnixNano(), got1, float64(time.Nanosecond))

	time.Sleep(100 * time.Millisecond)

	got2 := NowNano()
	is.InEpsilon(100_000_000, got2-got1, 200_000) // 200us is the delta

	got3 := []int64{}
	for i := 0; i < 1000; i++ {
		got3 = append(got3, NowNano())
		time.Sleep(1 * time.Nanosecond)
	}
	is.IsIncreasing(got3)
}
