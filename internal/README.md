# Internal Package

This package contains internal utilities used by the hot cache library.

## Benchmark

### NowMicro() Performance

The `NowMicro()` function provides microsecond-precision timestamps with better performance than `time.Now()`.

```txt
ok      github.com/samber/hot   0.182s
goos: darwin
goarch: arm64
pkg: github.com/samber/hot/bench
BenchmarkDevelTime/TimeGo-10            32602948                37.16 ns/op            0 B/op          0 allocs/op
BenchmarkDevelTime/TimeGo-10            32435061                37.34 ns/op            0 B/op          0 allocs/op
BenchmarkDevelTime/TimeGo-10            32238781                36.80 ns/op            0 B/op          0 allocs/op
BenchmarkDevelTime/TimeSyscall-10       74941058                15.93 ns/op            0 B/op          0 allocs/op
BenchmarkDevelTime/TimeSyscall-10       76003882                16.11 ns/op            0 B/op          0 allocs/op
BenchmarkDevelTime/TimeSyscall-10       74914741                15.89 ns/op            0 B/op          0 allocs/op
```

The syscall-based implementation (`TimeSyscall`) is approximately 2.3x faster than the standard `time.Now()` approach.
