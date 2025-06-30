package internal

import _ "unsafe"

//go:linkname nanotime runtime.nanotime
func nanotime() int64

// NowNano returns the current time in nanosecond.
// It is approximately 3 times faster than time.Now() for high-frequency operations.
// This function uses runtime.nanotime() for better performance.
func NowNano() int64 {
	return nanotime()
}
