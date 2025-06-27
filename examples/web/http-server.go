package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/samber/hot"
)

// User represents a user entity
type User struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Age      int       `json:"age"`
	Created  time.Time `json:"created"`
	LastSeen time.Time `json:"last_seen"`
}

// Product represents a product entity
type Product struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Category    string  `json:"category"`
	Stock       int     `json:"stock"`
	Description string  `json:"description"`
}

// Server represents our HTTP server with caching
type Server struct {
	userCache    *hot.HotCache[string, *User]
	productCache *hot.HotCache[string, *Product]
}

// NewServer creates a new server instance
func NewServer() *Server {
	// Create user cache with automatic loading
	userCache := hot.NewHotCache[string, *User](hot.LRU, 1000).
		WithTTL(5 * time.Minute).
		WithPrometheusMetrics("user-cache").
		WithLoaders(func(keys []string) (found map[string]*User, err error) {
			found = make(map[string]*User)
			for _, key := range keys {
				user, err := loadUserFromDB(key)
				if err != nil {
					log.Printf("Error loading user %s: %v", key, err)
					continue
				}
				if user != nil {
					found[key] = user
				}
			}
			return found, nil
		}).
		Build()

	// Create product cache with automatic loading
	productCache := hot.NewHotCache[string, *Product](hot.LFU, 2000).
		WithTTL(10 * time.Minute).
		WithPrometheusMetrics("product-cache").
		WithLoaders(func(keys []string) (found map[string]*Product, err error) {
			found = make(map[string]*Product)
			for _, key := range keys {
				product, err := loadProductFromDB(key)
				if err != nil {
					log.Printf("Error loading product %s: %v", key, err)
					continue
				}
				if product != nil {
					found[key] = product
				}
			}
			return found, nil
		}).
		Build()

	return &Server{
		userCache:    userCache,
		productCache: productCache,
	}
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Health check
	http.HandleFunc("/health", s.healthHandler)

	// User routes
	http.HandleFunc("/users/", s.userHandler)
	http.HandleFunc("/users", s.listUsersHandler)

	// Product routes
	http.HandleFunc("/products/", s.productHandler)
	http.HandleFunc("/products", s.listProductsHandler)

	// Cache management routes
	http.HandleFunc("/cache/stats", s.cacheStatsHandler)
	http.HandleFunc("/cache/clear", s.clearCacheHandler)
	http.HandleFunc("/cache/warmup", s.warmupCacheHandler)

	// Prometheus metrics
	http.HandleFunc("/metrics", s.metricsHandler)
}

// healthHandler returns server health status
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	userCurrent, userMax := s.userCache.Capacity()
	productCurrent, productMax := s.productCache.Capacity()

	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"cache": map[string]interface{}{
			"users": map[string]interface{}{
				"items":       userCurrent,
				"capacity":    userMax,
				"utilization": float64(userCurrent) / float64(userMax) * 100,
			},
			"products": map[string]interface{}{
				"items":       productCurrent,
				"capacity":    productMax,
				"utilization": float64(productCurrent) / float64(productMax) * 100,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// userHandler handles user CRUD operations
func (s *Server) userHandler(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from path
	path := strings.TrimPrefix(r.URL.Path, "/users/")
	if path == "" {
		http.Error(w, "User ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "GET":
		s.getUserHandler(w, r, path)
	case "PUT":
		s.updateUserHandler(w, r, path)
	case "DELETE":
		s.deleteUserHandler(w, r, path)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getUserHandler retrieves a user by ID using loader chain
func (s *Server) getUserHandler(w http.ResponseWriter, r *http.Request, userID string) {
	// Use the loader chain - cache will automatically load from DB if not found
	user, found, err := s.userCache.Get(userID)
	if err != nil {
		log.Printf("Error retrieving user %s: %v", userID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	} else if !found {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
		return
	}

	// Check if this was a cache hit or miss by looking at the cache state
	// Note: In a real implementation, you might want to track this differently
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "HIT") // The loader chain handles the miss automatically
	json.NewEncoder(w).Encode(user)
}

// createUserHandler creates a new user
func (s *Server) createUserHandler(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	// Validate required fields
	if user.Name == "" || user.Email == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Name and email are required"})
		return
	}

	// Generate ID if not provided
	if user.ID == "" {
		user.ID = fmt.Sprintf("user:%d", time.Now().UnixNano())
	}

	user.Created = time.Now()
	user.LastSeen = time.Now()

	// Store in database first
	if err := saveUserToDB(&user); err != nil {
		log.Printf("Error saving user to database: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create user"})
		return
	}

	// Store in cache
	s.userCache.Set(user.ID, &user)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// updateUserHandler updates an existing user
func (s *Server) updateUserHandler(w http.ResponseWriter, r *http.Request, userID string) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	// Validate required fields
	if user.Name == "" || user.Email == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Name and email are required"})
		return
	}

	user.ID = userID
	user.LastSeen = time.Now()

	// Update in database first
	if err := updateUserInDB(&user); err != nil {
		log.Printf("Error updating user in database: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update user"})
		return
	}

	// Update cache
	s.userCache.Set(userID, &user)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// deleteUserHandler deletes a user
func (s *Server) deleteUserHandler(w http.ResponseWriter, r *http.Request, userID string) {
	// Delete from database first
	if err := deleteUserFromDB(userID); err != nil {
		log.Printf("Error deleting user from database: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to delete user"})
		return
	}

	// Remove from cache
	s.userCache.Delete(userID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// listUsersHandler lists users with pagination
func (s *Server) listUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		s.createUserHandler(w, r)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	users, err := listUsersFromDB(page, limit)
	if err != nil {
		log.Printf("Error listing users: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to list users"})
		return
	}

	response := map[string]interface{}{
		"users": users,
		"page":  page,
		"limit": limit,
		"total": len(users),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// productHandler handles product CRUD operations
func (s *Server) productHandler(w http.ResponseWriter, r *http.Request) {
	// Extract product ID from path
	path := strings.TrimPrefix(r.URL.Path, "/products/")
	if path == "" {
		http.Error(w, "Product ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "GET":
		s.getProductHandler(w, r, path)
	case "PUT":
		s.updateProductHandler(w, r, path)
	case "DELETE":
		s.deleteProductHandler(w, r, path)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getProductHandler retrieves a product by ID using loader chain
func (s *Server) getProductHandler(w http.ResponseWriter, r *http.Request, productID string) {
	// Use the loader chain - cache will automatically load from DB if not found
	product, found, err := s.productCache.Get(productID)
	if err != nil {
		log.Printf("Error retrieving product %s: %v", productID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}

	if !found {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Product not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "HIT") // The loader chain handles the miss automatically
	json.NewEncoder(w).Encode(product)
}

// createProductHandler creates a new product
func (s *Server) createProductHandler(w http.ResponseWriter, r *http.Request) {
	var product Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	// Validate required fields
	if product.Name == "" || product.Price <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Name and price are required"})
		return
	}

	// Generate ID if not provided
	if product.ID == "" {
		product.ID = fmt.Sprintf("product:%d", time.Now().UnixNano())
	}

	// Store in database first
	if err := saveProductToDB(&product); err != nil {
		log.Printf("Error saving product to database: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create product"})
		return
	}

	// Store in cache
	s.productCache.Set(product.ID, &product)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(product)
}

// updateProductHandler updates an existing product
func (s *Server) updateProductHandler(w http.ResponseWriter, r *http.Request, productID string) {
	var product Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	// Validate required fields
	if product.Name == "" || product.Price <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Name and price are required"})
		return
	}

	product.ID = productID

	// Update in database first
	if err := updateProductInDB(&product); err != nil {
		log.Printf("Error updating product in database: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update product"})
		return
	}

	// Update cache
	s.productCache.Set(productID, &product)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(product)
}

// deleteProductHandler deletes a product
func (s *Server) deleteProductHandler(w http.ResponseWriter, r *http.Request, productID string) {
	// Delete from database first
	if err := deleteProductFromDB(productID); err != nil {
		log.Printf("Error deleting product from database: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to delete product"})
		return
	}

	// Remove from cache
	s.productCache.Delete(productID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// listProductsHandler lists products with pagination
func (s *Server) listProductsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		s.createProductHandler(w, r)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	products, err := listProductsFromDB(page, limit)
	if err != nil {
		log.Printf("Error listing products: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to list products"})
		return
	}

	response := map[string]interface{}{
		"products": products,
		"page":     page,
		"limit":    limit,
		"total":    len(products),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// cacheStatsHandler returns cache statistics
func (s *Server) cacheStatsHandler(w http.ResponseWriter, r *http.Request) {
	userCurrent, userMax := s.userCache.Capacity()
	productCurrent, productMax := s.productCache.Capacity()

	stats := map[string]interface{}{
		"user_cache": map[string]interface{}{
			"items":       userCurrent,
			"capacity":    userMax,
			"utilization": float64(userCurrent) / float64(userMax) * 100,
			"algorithm":   "LRU",
			"ttl":         "5m",
		},
		"product_cache": map[string]interface{}{
			"items":       productCurrent,
			"capacity":    productMax,
			"utilization": float64(productCurrent) / float64(productMax) * 100,
			"algorithm":   "LFU",
			"ttl":         "10m",
		},
		"timestamp": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// clearCacheHandler clears all caches
func (s *Server) clearCacheHandler(w http.ResponseWriter, r *http.Request) {
	// Clear user cache by deleting all items
	keysToDelete := make([]string, 0)
	s.userCache.Range(func(key string, value *User) bool {
		keysToDelete = append(keysToDelete, key)
		return true
	})
	if len(keysToDelete) > 0 {
		s.userCache.DeleteMany(keysToDelete)
	}

	// Clear product cache by deleting all items
	keysToDelete = make([]string, 0)
	s.productCache.Range(func(key string, value *Product) bool {
		keysToDelete = append(keysToDelete, key)
		return true
	})
	if len(keysToDelete) > 0 {
		s.productCache.DeleteMany(keysToDelete)
	}

	response := map[string]string{
		"message": "All caches cleared successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// warmupCacheHandler warms up caches with popular data
func (s *Server) warmupCacheHandler(w http.ResponseWriter, r *http.Request) {
	// Warm up user cache with popular users
	popularUsers := getPopularUsers()
	for _, user := range popularUsers {
		s.userCache.Set(user.ID, user)
	}

	// Warm up product cache with popular products
	popularProducts := getPopularProducts()
	for _, product := range popularProducts {
		s.productCache.Set(product.ID, product)
	}

	response := map[string]interface{}{
		"message":  "Cache warmup completed",
		"users":    len(popularUsers),
		"products": len(popularProducts),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// metricsHandler returns Prometheus metrics
func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	// This would typically serve Prometheus metrics
	// For now, just return a simple message
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("# HOT Cache Metrics\n# This endpoint would serve Prometheus metrics\n"))
}

// Database simulation functions with proper error handling

func loadUserFromDB(id string) (*User, error) {
	// Simulate database load with potential errors
	if id == "error" {
		return nil, fmt.Errorf("database connection error")
	}

	// Simulate user not found
	if id == "notfound" {
		return nil, nil
	}

	// Simulate successful load
	user := &User{
		ID:       id,
		Name:     fmt.Sprintf("User %s", id),
		Email:    fmt.Sprintf("user%s@example.com", id),
		Age:      25 + len(id)%20,
		Created:  time.Now().Add(-time.Duration(len(id)) * time.Hour),
		LastSeen: time.Now(),
	}

	return user, nil
}

func loadProductFromDB(id string) (*Product, error) {
	// Simulate database load with potential errors
	if id == "error" {
		return nil, fmt.Errorf("database connection error")
	}

	// Simulate product not found
	if id == "notfound" {
		return nil, nil
	}

	// Simulate successful load
	product := &Product{
		ID:          id,
		Name:        fmt.Sprintf("Product %s", id),
		Price:       10.0 + float64(len(id)),
		Category:    "electronics",
		Stock:       100 - len(id),
		Description: fmt.Sprintf("Description for product %s", id),
	}

	return product, nil
}

func saveUserToDB(user *User) error {
	// Simulate database save
	if user.ID == "error" {
		return fmt.Errorf("database save error")
	}
	return nil
}

func saveProductToDB(product *Product) error {
	// Simulate database save
	if product.ID == "error" {
		return fmt.Errorf("database save error")
	}
	return nil
}

func updateUserInDB(user *User) error {
	// Simulate database update
	if user.ID == "error" {
		return fmt.Errorf("database update error")
	}
	return nil
}

func updateProductInDB(product *Product) error {
	// Simulate database update
	if product.ID == "error" {
		return fmt.Errorf("database update error")
	}
	return nil
}

func deleteUserFromDB(id string) error {
	// Simulate database delete
	if id == "error" {
		return fmt.Errorf("database delete error")
	}
	return nil
}

func deleteProductFromDB(id string) error {
	// Simulate database delete
	if id == "error" {
		return fmt.Errorf("database delete error")
	}
	return nil
}

func listUsersFromDB(page, limit int) ([]*User, error) {
	// Simulate database list with potential errors
	if page == 999 {
		return nil, fmt.Errorf("database list error")
	}

	users := make([]*User, 0, limit)
	for i := 0; i < limit; i++ {
		user := &User{
			ID:       fmt.Sprintf("user:%d", (page-1)*limit+i+1),
			Name:     fmt.Sprintf("User %d", (page-1)*limit+i+1),
			Email:    fmt.Sprintf("user%d@example.com", (page-1)*limit+i+1),
			Age:      25 + i%20,
			Created:  time.Now().Add(-time.Duration(i) * time.Hour),
			LastSeen: time.Now(),
		}
		users = append(users, user)
	}

	return users, nil
}

func listProductsFromDB(page, limit int) ([]*Product, error) {
	// Simulate database list with potential errors
	if page == 999 {
		return nil, fmt.Errorf("database list error")
	}

	products := make([]*Product, 0, limit)
	for i := 0; i < limit; i++ {
		product := &Product{
			ID:          fmt.Sprintf("product:%d", (page-1)*limit+i+1),
			Name:        fmt.Sprintf("Product %d", (page-1)*limit+i+1),
			Price:       10.0 + float64(i),
			Category:    "electronics",
			Stock:       100 - i,
			Description: fmt.Sprintf("Description for product %d", (page-1)*limit+i+1),
		}
		products = append(products, product)
	}

	return products, nil
}

func getPopularUsers() []*User {
	return []*User{
		{ID: "popular:1", Name: "Popular User 1", Email: "popular1@example.com", Age: 30},
		{ID: "popular:2", Name: "Popular User 2", Email: "popular2@example.com", Age: 25},
	}
}

func getPopularProducts() []*Product {
	return []*Product{
		{ID: "popular:1", Name: "Popular Product 1", Price: 99.99, Category: "electronics", Stock: 50},
		{ID: "popular:2", Name: "Popular Product 2", Price: 149.99, Category: "electronics", Stock: 30},
	}
}

func main() {
	fmt.Println("🚀 HOT HTTP Server Example")
	fmt.Println("==========================")

	server := NewServer()
	server.setupRoutes()

	fmt.Println("✅ Server created with caching enabled")
	fmt.Println("🌐 Starting server on :8080")
	fmt.Println("📊 Prometheus metrics available at /metrics")
	fmt.Println("📈 Cache statistics available at /cache/stats")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
