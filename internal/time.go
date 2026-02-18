package internal

import (
	"time"
	// _ "unsafe" // required for runtime.nanotime.
)

// Using go:linkname is against the Go rules. There is another way to measure the
// duration with monotonic time: using time.Since(startTime) where startTime is
// the program start time.
// This method is 1ns slower than calling nanotime(), which is not a big deal, but
// the developers reported issues between synctest and go:linkname annotations.
//
// Follow-up: https://github.com/samber/hot/issues/39

var startTime = time.Now()

// NowNanoMonotonic returns the current time in nanoseconds.
// It is approximately 3 times faster than time.Now() for high-frequency operations.
func NowNano() int64 {
	return time.Since(startTime).Nanoseconds()
}

// //go:linkname nanotime runtime.nanotime
// func nanotime() int64

// NowNanoMonotonic returns the current time in nanoseconds.
// It is approximately 3 times faster than time.Now() for high-frequency operations.
// This function uses runtime.nanotime() for better performance.
// func NowNano() int64 {
// 	return nanotime()
// }
