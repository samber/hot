package internal

import (
	"time"
)

// NowMicro returns the current time in microseconds, with microsecond precision.
// It is twice faster than time.Now().
// func NowMicro() int64 {
// 	var tv syscall.Timeval
// 	// @TODO: check error ?
// 	syscall.Gettimeofday(&tv) //nolint:errcheck
// 	return int64(tv.Sec)*1e3 + int64(tv.Usec)
// }

func NowMicro() int64 {
	return time.Now().UnixNano() / 1e3
}
