package internal

import "syscall"

// NowMicro returns the current time in microseconds with microsecond precision.
// It is approximately twice as fast as time.Now() for high-frequency operations.
// This function uses syscall.Gettimeofday for better performance.
func NowMicro() int64 {
	var tv syscall.Timeval
	// @TODO: Check error?
	_ = syscall.Gettimeofday(&tv) //nolint:errcheck
	return int64(tv.Sec)*1e6 + int64(tv.Usec)
}
