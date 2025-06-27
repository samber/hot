# Web Application Example

Demonstrates how to integrate HOT cache with an HTTP server for web applications.

## What it does

- Creates an HTTP server with caching layers
- Implements RESTful API endpoints for users and products
- Shows cache-aside pattern with automatic loading
- Provides cache management endpoints
- Exposes Prometheus metrics

## Key Features Demonstrated

- **RESTful API**: CRUD operations for users and products
- **Cache-Aside Pattern**: Automatic loading from database
- **Cache Management**: Stats, clear, and warmup endpoints
- **Prometheus Metrics**: Built-in monitoring
- **Error Handling**: Graceful error responses
- **Health Checks**: Server health monitoring

## Quick Start

```bash
go run http-server.go
```

Then visit the endpoints:
- `http://localhost:8080/health` - Server health
- `http://localhost:8080/users` - List users
- `http://localhost:8080/users/1` - Get user by ID
- `http://localhost:8080/products` - List products
- `http://localhost:8080/products/1` - Get product by ID
- `http://localhost:8080/cache/stats` - Cache statistics
- `http://localhost:8080/metrics` - Prometheus metrics

## Expected Output

```
🌐 HTTP Server with Cache Example
=================================
✅ User cache created with 1000 items capacity
✅ Product cache created with 2000 items capacity
🚀 HTTP server started on :8080

Available endpoints:
- GET  /health - Server health
- GET  /users - List users
- GET  /users/{id} - Get user by ID
- POST /users - Create user
- PUT  /users/{id} - Update user
- DELETE /users/{id} - Delete user
- GET  /products - List products
- GET  /products/{id} - Get product by ID
- POST /products - Create product
- PUT  /products/{id} - Update product
- DELETE /products/{id} - Delete product
- GET  /cache/stats - Cache statistics
- POST /cache/clear - Clear all caches
- POST /cache/warmup - Warm up caches
- GET  /metrics - Prometheus metrics
```

## API Examples

### Get User
```bash
curl http://localhost:8080/users/1
```

### Create User
```bash
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com","age":30}'
```

### Cache Statistics
```bash
curl http://localhost:8080/cache/stats
```
