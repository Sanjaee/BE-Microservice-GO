# User Service

A comprehensive microservice for user authentication and management with JWT tokens, OTP verification, Redis caching, and RabbitMQ event publishing.

## Features

- ✅ User Registration with email validation
- ✅ User Login with JWT authentication
- ✅ OTP (One-Time Password) generation and verification
- ✅ Password hashing with bcrypt
- ✅ JWT access and refresh tokens
- ✅ Database-only OTP storage (no Redis required)
- ✅ RabbitMQ integration for event publishing
- ✅ Comprehensive API documentation
- ✅ Health check endpoints
- ✅ CORS support
- ✅ Request logging
- ✅ Database auto-migration

## API Endpoints

### Authentication Endpoints

#### Register User

```http
POST /api/v1/auth/register
Content-Type: application/json

{
  "username": "johndoe",
  "email": "john@example.com",
  "password": "password123"
}
```

**Response:**

```json
{
  "message": "User registered successfully. Please verify your email.",
  "user": {
    "id": "uuid",
    "username": "johndoe",
    "email": "john@example.com",
    "is_verified": false,
    "created_at": "2024-01-01T00:00:00Z"
  },
  "otp": "123456"
}
```

#### Login User

```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "email": "john@example.com",
  "password": "password123"
}
```

**Response:**

```json
{
  "user": {
    "id": "uuid",
    "username": "johndoe",
    "email": "john@example.com",
    "is_verified": true,
    "created_at": "2024-01-01T00:00:00Z"
  },
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 900
}
```

#### Verify OTP

```http
POST /api/v1/auth/verify-otp
Content-Type: application/json

{
  "email": "john@example.com",
  "otp_code": "123456"
}
```

**Response:**

```json
{
  "message": "Email verified successfully",
  "user": {
    "id": "uuid",
    "username": "johndoe",
    "email": "john@example.com",
    "is_verified": true,
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

#### Resend OTP

```http
POST /api/v1/auth/resend-otp
Content-Type: application/json

{
  "email": "john@example.com"
}
```

**Response:**

```json
{
  "message": "OTP sent successfully",
  "otp": "654321"
}
```

#### Refresh Token

```http
POST /api/v1/auth/refresh-token
Content-Type: application/json

{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Response:**

```json
{
  "user": {
    "id": "uuid",
    "username": "johndoe",
    "email": "john@example.com",
    "is_verified": true,
    "created_at": "2024-01-01T00:00:00Z"
  },
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 900
}
```

### Protected Endpoints (Require JWT Token)

#### Get User Profile

```http
GET /api/v1/user/profile
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "user": {
    "id": "uuid",
    "username": "johndoe",
    "email": "john@example.com",
    "is_verified": true,
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

#### Update User Profile

```http
PUT /api/v1/user/profile
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "username": "newusername"
}
```

**Response:**

```json
{
  "message": "Profile updated successfully",
  "user": {
    "id": "uuid",
    "username": "newusername",
    "email": "john@example.com",
    "is_verified": true,
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

### Health Check

#### Service Health

```http
GET /health
```

**Response:**

```json
{
  "status": "ok",
  "service": "user-service",
  "time": 1704067200,
  "database": "ok",
  "redis": "ok",
  "rabbitmq": "ok"
}
```

## Configuration

Copy `env.example` to `.env` and configure the following variables:

```bash
# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=user_service
DB_PASSWORD=userpass
DB_NAME=userdb

# JWT Configuration
JWT_SECRET=your-super-secret-jwt-key-change-this-in-production
JWT_ACCESS_EXPIRY=15m
JWT_REFRESH_EXPIRY=168h

# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# RabbitMQ Configuration
RABBITMQ_HOST=localhost
RABBITMQ_PORT=5672
RABBITMQ_USERNAME=admin
RABBITMQ_PASSWORD=secret123

# Server Configuration
PORT=8081
GIN_MODE=debug
```

## Database Schema

The service uses the following database schema:

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    email VARCHAR(150) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    otp_code VARCHAR(6),
    image_url VARCHAR(500),
    type VARCHAR(20) NOT NULL DEFAULT 'credential' CHECK (type IN ('credential', 'google')),
    is_verified BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT now(),
    updated_at TIMESTAMP DEFAULT now()
);
```

## Running the Service

### Prerequisites

1. Go 1.21 or higher
2. PostgreSQL 15 or higher
3. RabbitMQ 3.11 or higher (optional - for event publishing)

### Using Docker Compose

The service is configured to work with the provided `docker-compose.yml`:

```bash
# Start all services
docker-compose up -d

# Run the user service
cd services/user-service
go run cmd/main.go
```

### Manual Setup

1. **Start PostgreSQL:**

   ```bash
   # Create database and user
   psql -U postgres -f ../../db/init.sql
   ```

2. **Start Redis:**

   ```bash
   redis-server
   ```

3. **Start RabbitMQ:**

   ```bash
   docker run -d --name rabbitmq -p 5672:5672 -p 15672:15672 \
     -e RABBITMQ_DEFAULT_USER=admin \
     -e RABBITMQ_DEFAULT_PASS=secret123 \
     rabbitmq:3-management
   ```

4. **Run the service:**
   ```bash
   cd services/user-service
   go mod tidy
   go run cmd/main.go
   ```

## Event Publishing

The service publishes the following events to RabbitMQ:

- `user.registered` - When a new user registers
- `user.verified` - When a user verifies their email
- `user.login` - When a user logs in

Events are published to the `user.events` exchange with topic routing.

## OTP Storage

OTP codes are stored directly in the database:

- OTP codes are stored in the `otp_code` field of the `users` table
- OTP codes are automatically cleared after successful verification
- No external caching service required

## Security Features

- Password hashing with bcrypt
- JWT token authentication
- CORS protection
- Request validation
- Rate limiting (when Redis is available)
- Secure OTP generation

## Error Handling

The service returns consistent error responses:

```json
{
  "error": "Error message description"
}
```

Common HTTP status codes:

- `200` - Success
- `201` - Created
- `400` - Bad Request
- `401` - Unauthorized
- `404` - Not Found
- `409` - Conflict
- `500` - Internal Server Error

## Development

### Project Structure

```
services/user-service/
├── cmd/
│   └── main.go              # Application entry point
├── internal/
│   ├── cache/
│   │   └── redis.go         # Redis service (with independent .env loading)
│   ├── events/
│   │   └── rabbitmq.go      # RabbitMQ event service (with independent .env loading)
│   ├── handlers/
│   │   ├── jwt.go           # JWT service and middleware (with independent .env loading)
│   │   └── user.go          # User handlers (with independent .env loading)
│   └── models/
│       ├── auth.go          # Authentication models
│       └── user.go          # User models
├── go.mod
├── go.sum
├── env.example
└── README.md
```

### Environment Configuration

Each internal package independently loads environment variables using `godotenv.Load()`, making the service more modular and allowing each component to be self-contained. This architecture provides:

- **Modularity**: Each package can be used independently
- **Flexibility**: Different packages can have different environment configurations
- **Reliability**: If one package fails to load .env, others continue to work
- **Development**: Easy to test individual components

### Adding New Features

1. Add new models in `internal/models/`
2. Create handlers in `internal/handlers/`
3. Add routes in `cmd/main.go`
4. Update this README with new endpoints

## Testing

Test the API using curl or any HTTP client:

```bash
# Register a user
curl -X POST http://localhost:8081/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","email":"test@example.com","password":"password123"}'

# Login
curl -X POST http://localhost:8081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'

# Verify OTP (use OTP from registration response)
curl -X POST http://localhost:8081/api/v1/auth/verify-otp \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","otp_code":"123456"}'
```

## License

This project is part of a microservices architecture demonstration.
