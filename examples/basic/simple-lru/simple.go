package main

import (
	"fmt"
	"time"

	"github.com/samber/hot"
)

// User represents a simple user entity.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	fmt.Println("🚀 Simple LRU Cache Example")
	fmt.Println("============================")

	// Create cache with 1000 items capacity and 5-minute TTL
	cache := hot.NewHotCache[string, *User](hot.LRU, 1000).
		WithTTL(5*time.Minute).
		WithJitter(0.1, 2*time.Minute).
		WithRevalidation(30 * time.Second).
		WithRevalidationErrorPolicy(hot.KeepOnError).
		WithJanitor().
		Build()
	defer cache.StopJanitor()

	// Store users
	users := []*User{
		{ID: "user:1", Name: "Alice", Email: "alice@example.com"},
		{ID: "user:2", Name: "Bob", Email: "bob@example.com"},
		{ID: "user:3", Name: "Carol", Email: "carol@example.com"},
	}

	for _, user := range users {
		cache.Set(user.ID, user)
		fmt.Printf("✅ Stored: %s\n", user.Name)
	}

	// Retrieve users
	for _, userID := range []string{"user:1", "user:2", "user:3"} {
		if user, found, _ := cache.Get(userID); found {
			fmt.Printf("✅ Retrieved: %s\n", user.Name)
		}
	}

	// Batch operations
	batch := map[string]*User{
		"user:4": {ID: "user:4", Name: "David", Email: "david@example.com"},
		"user:5": {ID: "user:5", Name: "Eve", Email: "eve@example.com"},
	}
	cache.SetMany(batch)
	fmt.Printf("✅ Stored %d users in batch\n", len(batch))

	// Cache statistics
	current, mAx := cache.Capacity()
	fmt.Printf("📊 Cache: %d/%d items (%.1f%%)\n",
		current, mAx, float64(current)/float64(mAx)*100)

	fmt.Println("\n🎉 Example completed!")
}
