# Internal Package

This package contains internal utilities used by the hot cache library.

## Benchmark

### NowNano() Performance

The `NowNano()` function provides nanosecond-precision timestamps with better performance than `time.Now()`.

```txt
ok      github.com/samber/hot   0.182s
goos: darwin
goarch: arm64
pkg: github.com/samber/hot/bench
cpu: Apple M3
BenchmarkDevelTime/TimeGo-8                             10000000                35.66 ns/op            0 B/op          0 allocs/op
BenchmarkDevelTime/TimeSyscallMonotonicTime-8           10000000                37.26 ns/op            0 B/op          0 allocs/op
BenchmarkDevelTime/TimeSyscallWallTime-8                10000000                12.19 ns/op            0 B/op          0 allocs/op
BenchmarkDevelTime/TimeRuntimeMonotonicTime-8           10000000                12.38 ns/op            0 B/op          0 allocs/op
```

The syscall-based implementation (`TimeSyscall`) and `runtime.nanosecond()` are approximately 2.5x faster than the standard `time.Now()` approach.
