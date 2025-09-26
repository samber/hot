package base

type CacheMode string

const (
	CacheModeMain CacheMode = "main"
	// CacheModeShared  CacheMode = "shared".
	CacheModeMissing CacheMode = "missing"
)
