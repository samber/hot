package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/samber/hot"
)

func main() {
	fmt.Println("📊 Simple Prometheus Example")
	fmt.Println("============================")

	// Create a cache with Prometheus metrics
	cache := hot.NewHotCache[string, string](hot.LRU, 100).
		WithTTL(5 * time.Minute).
		WithPrometheusMetrics("simple_cache").
		Build()

	fmt.Println("✅ Cache created with Prometheus metrics enabled")

	// Start HTTP server for metrics
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		fmt.Println("🌐 Prometheus metrics available at http://localhost:8080/metrics")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// Simulate cache operations
	fmt.Println("\n🔄 Simulating cache operations...")

	// Store some data
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key:%d", i)
		value := fmt.Sprintf("value:%d", i)
		cache.Set(key, value)
	}

	// Generate some hits and misses
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key:%d", i%60) // Some will hit, some will miss
		cache.Get(key)
	}

	// Show cache statistics
	current, max := cache.Capacity()
	fmt.Printf("📊 Cache statistics: %d/%d items (%.1f%% utilization)\n",
		current, max, float64(current)/float64(max)*100)

	fmt.Println("\n🎉 Example completed!")
	fmt.Println("💡 Check http://localhost:8080/metrics to see Prometheus metrics")
	fmt.Println("   Press Ctrl+C to stop the server")

	// Keep the server running
	select {}
}
