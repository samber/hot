# HOT Cache Examples

This directory contains practical examples demonstrating how to use the HOT cache library in various scenarios.

## Quick Start

Choose an example based on your needs:

### Basic Examples
- **[Simple LRU](basic/simple-lru/)** - Basic cache operations and LRU eviction
- **[TTL Cache](basic/ttl/)** - Time-based expiration with jitter and background cleanup
- **[Batch Operations](basic/batch/)** - Performance optimization with batch operations
- **[Concurrency](basic/concurrency/)** - Thread safety and concurrent access patterns

### Advanced Examples
- **[Observability](observability/)** - Prometheus metrics and monitoring
- **[Database Integration](database/)** - PostgreSQL integration with cache-aside pattern
- **[Web Application](web/)** - HTTP server with RESTful API and caching

## Getting Started

1. **Clone the repository**:
   ```bash
   git clone https://github.com/samber/hot.git
   cd hot/examples
   ```

2. **Run a basic example**:
   ```bash
   cd basic/simple-lru
   go run simple.go
   ```

3. **Explore advanced features**:
   ```bash
   cd ../web
   go run web-server.go
   ```

## Contributing

Feel free to add new examples or improve existing ones. Each example should:
- Be concise and readable in a few seconds
- Include a comprehensive README
- Demonstrate practical use cases
- Follow Go best practices
