# Database Integration Example

Demonstrates how to integrate HOT cache with PostgreSQL database for improved performance.

## What it does

- Creates a database manager with caching layers
- Implements automatic cache loading from database
- Shows CRUD operations with cache invalidation
- Demonstrates cache-aside pattern with database fallback

## Key Features Demonstrated

- **Cache-Aside Pattern**: Load data from cache, fallback to database
- **Automatic Loading**: Cache automatically loads missing data from DB
- **Cache Invalidation**: Update/delete operations invalidate cache
- **Prometheus Metrics**: Monitor cache performance
- **Error Handling**: Graceful handling of database errors

## Quick Start

```bash
# Install PostgreSQL driver
go mod download

# Set up database connection
export DATABASE_URL="postgres://user:password@localhost/dbname?sslmode=disable"

# Run the example
go run postgresql-cache.go
```

## Expected Output

```
🗄️ PostgreSQL Cache Example
============================
✅ Database connected successfully
✅ User cache created with 1000 items capacity
✅ Product cache created with 2000 items capacity

📝 Example 1: User Operations
------------------------------
✅ Created user: Alice (ID: 1)
✅ Retrieved user: Alice (ID: 1) - Cache HIT
✅ Updated user: Alice Johnson (ID: 1)
✅ Retrieved user: Alice Johnson (ID: 1) - Cache HIT
✅ Deleted user: Alice Johnson (ID: 1)

📝 Example 2: Product Operations
--------------------------------
✅ Created product: Laptop (ID: 1)
✅ Retrieved product: Laptop (ID: 1) - Cache HIT
✅ Retrieved product: Laptop (ID: 1) - Cache HIT
✅ Updated product: Gaming Laptop (ID: 1)
✅ Deleted product: Gaming Laptop (ID: 1)

📊 Cache Statistics
--------------------
User Cache: 0/1000 items (0.0% utilization)
Product Cache: 0/2000 items (0.0% utilization)
```

## Database Schema

The example creates these tables:
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    age INTEGER,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10,2),
    category VARCHAR(100),
    stock INTEGER,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```
