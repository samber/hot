package base

type EvictionCallback[K comparable, V any] func(K, V)
