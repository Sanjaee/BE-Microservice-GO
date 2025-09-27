# Product Service

A high-performance product service built with Go, featuring worker pools, Redis caching, and efficient pagination.

## Features

- **Worker Pool Architecture**: Handles concurrent requests efficiently with configurable worker count
- **Redis Caching**: Implements intelligent caching for improved performance
- **Keyset Pagination**: Efficient pagination using cursor-based approach
- **Context Timeout**: Automatic request cancellation for better resource management
- **GORM Integration**: Database operations with PostgreSQL
- **RESTful API**: Clean API endpoints for product operations

## Architecture

### Worker Pool

- Configurable number of workers (default: 100)
- Request queuing with channels
- Graceful shutdown handling
- Request timeout management

### Caching Strategy

- Product list caching (5 minutes TTL)
- Individual product caching (10 minutes TTL)
- Cache invalidation on updates
- Pattern-based cache clearing

### Pagination

- Keyset pagination for better performance
- Cursor-based navigation
- Configurable page sizes (max 100)
- Support for filtering and searching

## API Endpoints

### Products

- `GET /api/v1/products` - Get all products with pagination
- `GET /api/v1/products/:id` - Get product by ID
- `GET /health` - Health check

### Query Parameters

- `page` - Page number (default: 1)
- `limit` - Items per page (default: 20, max: 100)
- `cursor` - Cursor for keyset pagination
- `search` - Search in name and description
- `min_price` - Minimum price filter
- `max_price` - Maximum price filter
- `is_active` - Filter by active status

## Environment Variables

```bash
# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=microservice_db

# Redis Configuration
REDIS_HOST=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# Server Configuration
PORT=8082
WORKER_COUNT=100

# Environment
GIN_MODE=debug
```

## Running the Service

1. **Start dependencies**:

   ```bash
   # Start PostgreSQL and Redis
   docker-compose up -d
   ```

2. **Seed the database**:

   ```bash
   go run cmd/seed.go
   ```

3. **Run the service**:
   ```bash
   go run cmd/main.go
   ```

## Database Schema

### Products Table

```sql
CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    price DECIMAL NOT NULL,
    stock INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### Product Images Table

```sql
CREATE TABLE product_images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL,
    image_url VARCHAR(500) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
```

## Performance Features

### Worker Pool Benefits

- **Concurrent Processing**: Multiple requests handled simultaneously
- **Resource Management**: Controlled goroutine usage prevents resource exhaustion
- **Request Queuing**: Smooth handling of traffic spikes
- **Timeout Handling**: Automatic request cancellation

### Caching Benefits

- **Reduced Database Load**: Frequently accessed data cached in Redis
- **Faster Response Times**: Sub-millisecond cache retrieval
- **Intelligent Invalidation**: Cache cleared only when data changes
- **Memory Efficiency**: TTL-based cache expiration

### Pagination Benefits

- **Consistent Performance**: O(log n) complexity regardless of offset
- **Real-time Data**: No duplicate or missing records during pagination
- **Memory Efficient**: Only loads required data
- **Scalable**: Works with millions of records

## Monitoring

The service provides health check endpoints and worker pool metrics:

```bash
# Health check
curl http://localhost:8082/health

# Response includes worker pool status
{
  "status": "ok",
  "service": "product-service",
  "timestamp": 1234567890,
  "worker_pool": {
    "active_jobs": 5
  }
}
```

## Integration with API Gateway

The service is integrated with the API Gateway at `http://localhost:8080`:

- `GET /api/v1/products` - Proxied to product service
- `GET /api/v1/products/:id` - Proxied to product service

## Frontend Integration

The frontend uses infinite scroll with the following features:

- **Automatic Loading**: Products load as user scrolls
- **Error Handling**: Graceful error display and retry
- **Loading States**: Visual feedback during data fetching
- **Responsive Design**: Works on all device sizes
- **Performance Optimized**: Only loads visible products

## Development

### Project Structure

```
product-service/
├── cmd/
│   ├── main.go          # Service entry point
│   └── seed.go          # Database seeding
├── internal/
│   ├── cache/
│   │   └── redis.go     # Redis client
│   ├── handlers/
│   │   ├── product_handler.go  # HTTP handlers
│   │   └── worker_pool.go      # Worker pool implementation
│   ├── models/
│   │   └── product.go   # Data models
│   └── repository/
│       └── product_repository.go  # Database operations
├── go.mod
├── go.sum
└── README.md
```

### Adding New Features

1. **New Endpoints**: Add handlers in `product_handler.go`
2. **Database Operations**: Extend `product_repository.go`
3. **Worker Tasks**: Add new request types in `worker_pool.go`
4. **Caching**: Implement cache strategies in repository layer

## Testing

```bash
# Run tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test
go test -run TestGetProducts ./internal/handlers
```

## Deployment

The service is designed for containerized deployment:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o product-service cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/product-service .
CMD ["./product-service"]
```
