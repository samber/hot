package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/samber/hot"
)

func main() {
	// Create a cache with Prometheus metrics enabled
	cache := hot.NewHotCache[string, string](hot.LRU, 1000).
		WithTTL(5*time.Minute).
		WithJitter(0.5, 10*time.Second).
		WithRevalidation(10*time.Second).
		WithRevalidationErrorPolicy(hot.KeepOnError).
		WithPrometheusMetrics("my-cache").
		WithMissingCache(hot.ARC, 1000).
		Build()

	// Register the cache metrics with Prometheus
	err := prometheus.Register(cache)
	if err != nil {
		log.Fatalf("Failed to register metrics: %v", err)
	}
	defer prometheus.Unregister(cache)

	// Use the cache
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	value, found, err := cache.Get("key1")
	if err != nil {
		log.Printf("Error getting key1: %v", err)
	} else if found {
		fmt.Printf("Found: %s\n", value)
	}

	// Set up HTTP server to expose metrics
	http.Handle("/metrics", promhttp.Handler())

	fmt.Println("Starting server on :8080")
	fmt.Println("Metrics available at http://localhost:8080/metrics")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
