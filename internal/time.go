package internal

import (
	_ "unsafe"
)

//go:linkname nanotime runtime.nanotime
func nanotime() int64

// NowNano returns the current time in nanoseconds.
// It is approximately 3 times faster than time.Now() for high-frequency operations.
// This function uses runtime.nanotime() for better performance.
func NowNano() int64 {
	return nanotime()
}

// Using go:linkname is against the Go rules. There is another way to mesure the
// duration with monotonic time: using time.Since(startTime) where startTime is
// the program start time.
// This method is 1ns slower than calling nanotime(), which is not a big deal, but
// the improvement in code quality is not worth it.
//
// If the go:linkname directive become an issue in the future, please uncomment
// the following code, open a pull-request and explain why you did it.
//
// Follow-up: https://github.com/samber/hot/issues/39

// var startTime = time.Now()
//
// func NowNano() int64 {
// 	return time.Since(startTime).Nanoseconds()
// }
