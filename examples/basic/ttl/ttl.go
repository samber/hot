package main

import (
	"fmt"
	"time"

	"github.com/samber/hot"
)

// Session represents a user session with expiration.
type Session struct {
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Config represents application configuration.
type Config struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
	Type  string      `json:"type"`
}

func main() {
	fmt.Println("⏰ TTL Cache Example")
	fmt.Println("====================")

	// Example 1: Basic TTL
	fmt.Println("\n📝 Example 1: Basic TTL")
	fmt.Println("------------------------")

	ttlCache := hot.NewHotCache[string, *Session](hot.LRU, 100).
		WithTTL(5 * time.Minute).
		Build()

	sessions := []*Session{
		{UserID: "user:1", Token: "token:abc123"},
		{UserID: "user:2", Token: "token:def456"},
	}

	for _, session := range sessions {
		ttlCache.Set(session.UserID, session)
		fmt.Printf("✅ Stored session for %s\n", session.UserID)
	}

	// Example 2: TTL with Jitter
	fmt.Println("\n🎲 Example 2: TTL with Jitter")
	fmt.Println("-------------------------------")

	jitterCache := hot.NewHotCache[string, *Config](hot.LRU, 100).
		WithTTL(10*time.Minute).
		WithJitter(0.1, 2*time.Minute).
		Build()

	configs := map[string]*Config{
		"app:theme":    {Key: "theme", Value: "dark"},
		"app:language": {Key: "language", Value: "en"},
		"app:timezone": {Key: "timezone", Value: "UTC"},
	}

	for key, config := range configs {
		jitterCache.Set(key, config)
		fmt.Printf("✅ Stored config: %s = %v\n", key, config.Value)
	}

	// Example 3: Custom TTL per Item
	fmt.Println("\n⚙️ Example 3: Custom TTL per Item")
	fmt.Println("----------------------------------")

	customCache := hot.NewHotCache[string, string](hot.LRU, 100).
		WithTTL(1 * time.Hour).
		Build()

	customCache.SetWithTTL("short-lived", "expires in 30 seconds", 30*time.Second)
	customCache.SetWithTTL("medium-lived", "expires in 5 minutes", 5*time.Minute)
	customCache.SetWithTTL("long-lived", "expires in 2 hours", 2*time.Hour)

	fmt.Println("✅ Stored items with custom TTLs:")
	fmt.Println("   - short-lived: 30 seconds")
	fmt.Println("   - medium-lived: 5 minutes")
	fmt.Println("   - long-lived: 2 hours")

	// Example 4: Background cleanup
	fmt.Println("\n🧹 Example 4: Background Cleanup")
	fmt.Println("----------------------------------")

	janitorCache := hot.NewHotCache[string, int](hot.LRU, 100).
		WithTTL(10 * time.Second).
		WithJanitor().
		Build()
	defer janitorCache.StopJanitor()

	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("temp:%d", i)
		janitorCache.Set(key, i)
		fmt.Printf("✅ Stored temporary data: %s = %d\n", key, i)
	}

	current, mAx := janitorCache.Capacity()
	fmt.Printf("📊 Initial cache: %d/%d items\n", current, mAx)

	fmt.Println("⏳ Waiting 15 seconds for cleanup...")
	time.Sleep(15 * time.Second)

	current, mAx = janitorCache.Capacity()
	fmt.Printf("📊 Final cache: %d/%d items (expired items cleaned up)\n", current, mAx)

	fmt.Println("\n🎉 Example completed!")
}
