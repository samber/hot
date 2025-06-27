# Observability Example

Demonstrates monitoring and observability features in the HOT cache library using Prometheus metrics.

## What it does

- Creates a cache with Prometheus metrics enabled
- Starts an HTTP server to expose metrics
- Simulates cache operations to generate metrics
- Shows how to monitor cache performance and behavior

## Key Features Demonstrated

- **Prometheus Integration**: Built-in metrics collection
- **HTTP Metrics Endpoint**: Expose metrics via `/metrics`
- **Cache Statistics**: Hit rates, miss rates, and utilization
- **Real-time Monitoring**: Live metrics during cache operations

## Quick Start

```bash
go run prometheus.go
```

Then visit `http://localhost:8080/metrics` to see the metrics.

## Expected Output

```
📊 Simple Prometheus Example
============================
✅ Cache created with Prometheus metrics enabled
🌐 Prometheus metrics available at http://localhost:8080/metrics

🔄 Simulating cache operations...
📊 Cache statistics: 50/100 items (50.0% utilization)

🎉 Example completed!
💡 Check http://localhost:8080/metrics to see Prometheus metrics
   Press Ctrl+C to stop the server
```
