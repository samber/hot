package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNowMicro(t *testing.T) {
	is := assert.New(t)

	got1 := NowMicro()
	is.InEpsilon(time.Now().UnixNano(), got1, float64(time.Microsecond))

	time.Sleep(100 * time.Millisecond)

	got2 := NowMicro()
	is.InEpsilon(100_000, got2-got1, 200) // 200us is the delta

	got3 := []int64{}
	for i := 0; i < 1000; i++ {
		got3 = append(got3, NowMicro())
		time.Sleep(1 * time.Microsecond)
	}
	is.IsIncreasing(got3)
}
