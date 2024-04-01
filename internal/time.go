package internal

import "syscall"

// NowMicro returns the current time in microseconds, with microsecond precision.
// It is twice faster than time.Now().
func NowMicro() int64 {
	var tv syscall.Timeval
	syscall.Gettimeofday(&tv)
	return int64(tv.Sec)*1e3 + int64(tv.Usec)
}
