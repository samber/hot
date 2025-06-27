package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/samber/hot"
)

// User represents a user from the database
type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Age       int       `json:"age"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Product represents a product from the database
type Product struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	Category  string    `json:"category"`
	Stock     int       `json:"stock"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DatabaseManager handles database operations with caching
type DatabaseManager struct {
	db           *sql.DB
	userCache    *hot.HotCache[string, *User]
	productCache *hot.HotCache[string, *Product]
}

// NewDatabaseManager creates a new database manager with caching
func NewDatabaseManager(connStr string) (*DatabaseManager, error) {
	// Connect to PostgreSQL
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	// Create user cache with LRU eviction and 10-minute TTL
	userCache := hot.NewHotCache[string, *User](hot.LRU, 1000).
		WithTTL(10 * time.Minute).
		WithPrometheusMetrics("postgres-user-cache").
		WithLoaders(func(keys []string) (found map[string]*User, err error) {
			return loadUsersFromDB(db, keys)
		}).
		Build()

	// Create product cache with LFU eviction and 15-minute TTL
	productCache := hot.NewHotCache[string, *Product](hot.LFU, 2000).
		WithTTL(15 * time.Minute).
		WithPrometheusMetrics("postgres-product-cache").
		WithLoaders(func(keys []string) (found map[string]*Product, err error) {
			return loadProductsFromDB(db, keys)
		}).
		Build()

	return &DatabaseManager{
		db:           db,
		userCache:    userCache,
		productCache: productCache,
	}, nil
}

// Close closes the database connection
func (dm *DatabaseManager) Close() error {
	return dm.db.Close()
}

// GetUser retrieves a user by ID with caching
func (dm *DatabaseManager) GetUser(id string) (*User, error) {
	user, found, err := dm.userCache.Get(id)
	if err != nil {
		return nil, fmt.Errorf("cache error: %v", err)
	}
	if !found {
		return nil, fmt.Errorf("user not found: %s", id)
	}
	return user, nil
}

// GetUsers retrieves multiple users by IDs with caching
func (dm *DatabaseManager) GetUsers(ids []string) (map[string]*User, error) {
	found, missing, err := dm.userCache.GetMany(ids)
	if err != nil {
		return nil, fmt.Errorf("cache error: %v", err)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("users not found: %v", missing)
	}
	return found, nil
}

// CreateUser creates a new user
func (dm *DatabaseManager) CreateUser(user *User) error {
	query := `
		INSERT INTO users (name, email, age, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	now := time.Now()
	err := dm.db.QueryRow(query, user.Name, user.Email, user.Age, now, now).Scan(&user.ID)
	if err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}

	// Cache the new user
	dm.userCache.Set(fmt.Sprintf("%d", user.ID), user)
	return nil
}

// UpdateUser updates an existing user
func (dm *DatabaseManager) UpdateUser(user *User) error {
	query := `
		UPDATE users 
		SET name = $1, email = $2, age = $3, updated_at = $4
		WHERE id = $5`

	now := time.Now()
	result, err := dm.db.Exec(query, user.Name, user.Email, user.Age, now, user.ID)
	if err != nil {
		return fmt.Errorf("failed to update user: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %d", user.ID)
	}

	user.UpdatedAt = now

	// Update cache
	dm.userCache.Set(fmt.Sprintf("%d", user.ID), user)
	return nil
}

// DeleteUser deletes a user
func (dm *DatabaseManager) DeleteUser(id string) error {
	query := `DELETE FROM users WHERE id = $1`
	result, err := dm.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", id)
	}

	// Remove from cache
	dm.userCache.Delete(id)
	return nil
}

// GetProduct retrieves a product by ID with caching
func (dm *DatabaseManager) GetProduct(id string) (*Product, error) {
	product, found, err := dm.productCache.Get(id)
	if err != nil {
		return nil, fmt.Errorf("cache error: %v", err)
	}
	if !found {
		return nil, fmt.Errorf("product not found: %s", id)
	}
	return product, nil
}

// GetProducts retrieves multiple products by IDs with caching
func (dm *DatabaseManager) GetProducts(ids []string) (map[string]*Product, error) {
	found, missing, err := dm.productCache.GetMany(ids)
	if err != nil {
		return nil, fmt.Errorf("cache error: %v", err)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("products not found: %v", missing)
	}
	return found, nil
}

// CreateProduct creates a new product
func (dm *DatabaseManager) CreateProduct(product *Product) error {
	query := `
		INSERT INTO products (name, price, category, stock, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`

	now := time.Now()
	err := dm.db.QueryRow(query, product.Name, product.Price, product.Category, product.Stock, now, now).Scan(&product.ID)
	if err != nil {
		return fmt.Errorf("failed to create product: %v", err)
	}

	// Cache the new product
	dm.productCache.Set(fmt.Sprintf("%d", product.ID), product)
	return nil
}

// UpdateProduct updates an existing product
func (dm *DatabaseManager) UpdateProduct(product *Product) error {
	query := `
		UPDATE products 
		SET name = $1, price = $2, category = $3, stock = $4, updated_at = $5
		WHERE id = $6`

	now := time.Now()
	result, err := dm.db.Exec(query, product.Name, product.Price, product.Category, product.Stock, now, product.ID)
	if err != nil {
		return fmt.Errorf("failed to update product: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("product not found: %d", product.ID)
	}

	product.UpdatedAt = now

	// Update cache
	dm.productCache.Set(fmt.Sprintf("%d", product.ID), product)
	return nil
}

// DeleteProduct deletes a product
func (dm *DatabaseManager) DeleteProduct(id string) error {
	query := `DELETE FROM products WHERE id = $1`
	result, err := dm.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete product: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("product not found: %s", id)
	}

	// Remove from cache
	dm.productCache.Delete(id)
	return nil
}

// GetCacheStats returns cache statistics
func (dm *DatabaseManager) GetCacheStats() map[string]interface{} {
	userCurrent, userMax := dm.userCache.Capacity()
	productCurrent, productMax := dm.productCache.Capacity()

	return map[string]interface{}{
		"user_cache": map[string]interface{}{
			"items":       userCurrent,
			"capacity":    userMax,
			"utilization": float64(userCurrent) / float64(userMax) * 100,
			"algorithm":   "LRU",
			"ttl":         "10m",
		},
		"product_cache": map[string]interface{}{
			"items":       productCurrent,
			"capacity":    productMax,
			"utilization": float64(productCurrent) / float64(productMax) * 100,
			"algorithm":   "LFU",
			"ttl":         "15m",
		},
		"timestamp": time.Now(),
	}
}

// loadUsersFromDB loads users from the database
func loadUsersFromDB(db *sql.DB, keys []string) (map[string]*User, error) {
	if len(keys) == 0 {
		return make(map[string]*User), nil
	}

	// Build query with placeholders
	query := `SELECT id, name, email, age, created_at, updated_at FROM users WHERE id = ANY($1)`

	// Convert string keys to integers for the query
	var ids []int
	for _, key := range keys {
		var id int
		if _, err := fmt.Sscanf(key, "%d", &id); err != nil {
			continue // Skip invalid keys
		}
		ids = append(ids, id)
	}

	rows, err := db.Query(query, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %v", err)
	}
	defer rows.Close()

	found := make(map[string]*User)
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Age, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %v", err)
		}
		found[fmt.Sprintf("%d", user.ID)] = &user
	}

	return found, nil
}

// loadProductsFromDB loads products from the database
func loadProductsFromDB(db *sql.DB, keys []string) (map[string]*Product, error) {
	if len(keys) == 0 {
		return make(map[string]*Product), nil
	}

	// Build query with placeholders
	query := `SELECT id, name, price, category, stock, created_at, updated_at FROM products WHERE id = ANY($1)`

	// Convert string keys to integers for the query
	var ids []int
	for _, key := range keys {
		var id int
		if _, err := fmt.Sscanf(key, "%d", &id); err != nil {
			continue // Skip invalid keys
		}
		ids = append(ids, id)
	}

	rows, err := db.Query(query, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to query products: %v", err)
	}
	defer rows.Close()

	found := make(map[string]*Product)
	for rows.Next() {
		var product Product
		err := rows.Scan(&product.ID, &product.Name, &product.Price, &product.Category, &product.Stock, &product.CreatedAt, &product.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan product: %v", err)
		}
		found[fmt.Sprintf("%d", product.ID)] = &product
	}

	return found, nil
}

// setupDatabase creates the necessary tables
func setupDatabase(db *sql.DB) error {
	// Create users table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			age INTEGER NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %v", err)
	}

	// Create products table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS products (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			price DECIMAL(10,2) NOT NULL,
			category VARCHAR(100) NOT NULL,
			stock INTEGER NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create products table: %v", err)
	}

	return nil
}

func main() {
	fmt.Println("🗄️ HOT PostgreSQL Integration Example")
	fmt.Println("=====================================")

	// Database connection string (modify as needed)
	connStr, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		log.Fatalf("DATABASE_URL environment variable is not set")
	}

	// For this example, we'll use a mock connection
	fmt.Println("⚠️  Note: This example requires a PostgreSQL database")
	fmt.Println("   Please modify the connection string in DATABASE_URL environment variable and ensure the database is running")
	fmt.Println("   Connection string:", connStr)

	dbManager, err := NewDatabaseManager(connStr)
	if err != nil {
		log.Fatalf("Failed to create database manager: %v", err)
	}
	defer dbManager.Close()

	// Setup database tables
	if err := setupDatabase(dbManager.db); err != nil {
		log.Fatalf("Failed to setup database: %v", err)
	}

	// Example: Create a user
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	if err := dbManager.CreateUser(user); err != nil {
		log.Printf("Failed to create user: %v", err)
	} else {
		fmt.Printf("✅ Created user: %s (ID: %d)\n", user.Name, user.ID)
	}

	// Example: Get user from cache
	if retrievedUser, err := dbManager.GetUser(fmt.Sprintf("%d", user.ID)); err != nil {
		log.Printf("Failed to get user: %v", err)
	} else {
		fmt.Printf("✅ Retrieved user from cache: %s\n", retrievedUser.Name)
	}

	// Example: Create a product
	product := &Product{
		Name:     "Laptop Pro",
		Price:    1299.99,
		Category: "Electronics",
		Stock:    50,
	}

	if err := dbManager.CreateProduct(product); err != nil {
		log.Printf("Failed to create product: %v", err)
	} else {
		fmt.Printf("✅ Created product: %s (ID: %d)\n", product.Name, product.ID)
	}

	// Example: Get product from cache
	if retrievedProduct, err := dbManager.GetProduct(fmt.Sprintf("%d", product.ID)); err != nil {
		log.Printf("Failed to get product: %v", err)
	} else {
		fmt.Printf("✅ Retrieved product from cache: %s\n", retrievedProduct.Name)
	}

	// Example: Batch operations
	userIDs := []string{fmt.Sprintf("%d", user.ID)}
	if users, err := dbManager.GetUsers(userIDs); err != nil {
		log.Printf("Failed to get users: %v", err)
	} else {
		fmt.Printf("✅ Retrieved %d users in batch\n", len(users))
	}

	// Example: Cache statistics
	stats := dbManager.GetCacheStats()
	statsJSON, _ := json.MarshalIndent(stats, "", "  ")
	fmt.Printf("📊 Cache Statistics:\n%s\n", string(statsJSON))

	fmt.Println("\n🎉 PostgreSQL integration example completed!")
	fmt.Println("💡 Key takeaways:")
	fmt.Println("   - Use HOT caches to reduce database load")
	fmt.Println("   - Implement cache-aside pattern for database operations")
	fmt.Println("   - Use batch operations for better performance")
	fmt.Println("   - Monitor cache statistics for optimization")
	fmt.Println("   - Handle cache invalidation on data updates")
}
