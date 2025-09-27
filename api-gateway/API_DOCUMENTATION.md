# API Gateway Documentation

API Gateway untuk microservice user authentication dengan proxy ke user-service.

## Base URL

```
http://localhost:8080
```

## Endpoints

### 1. Health Check

```http
GET /health
```

**Response:**

```json
{
  "status": "ok",
  "service": "api-gateway"
}
```

### 2. User Service Health Check

```http
GET /api/v1/user/health
```

**Response:**

```json
{
  "status": "ok",
  "service": "user-service",
  "time": 1704067200,
  "database": "ok",
  "redis": "not_used",
  "rabbitmq": "ok"
}
```

---

## Authentication Endpoints

### 3. Register User

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

### 4. Login User

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

### 5. Verify OTP

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

### 6. Resend OTP

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

### 7. Refresh Token

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

---

## Protected User Endpoints

### 8. Get User Profile

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

### 9. Update User Profile

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

---

## Error Responses

### Common Error Format

```json
{
  "error": "Error message description"
}
```

### HTTP Status Codes

- `200` - Success
- `201` - Created
- `400` - Bad Request
- `401` - Unauthorized
- `404` - Not Found
- `409` - Conflict
- `500` - Internal Server Error

---

## Testing dengan cURL

### 1. Register User

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "password123"
  }'
```

### 2. Login User

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }'
```

### 3. Verify OTP

```bash
curl -X POST http://localhost:8080/api/v1/auth/verify-otp \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "otp_code": "123456"
  }'
```

### 4. Get Profile (Protected)

```bash
curl -X GET http://localhost:8080/api/v1/user/profile \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

---

## Flow Authentication

1. **Register** → User mendaftar dengan username, email, password
2. **OTP Generated** → Sistem generate OTP dan simpan di database
3. **Verify OTP** → User verifikasi email dengan OTP
4. **Login** → User login dengan email dan password
5. **Get Tokens** → Sistem return access_token dan refresh_token
6. **Use Protected Routes** → Gunakan access_token untuk akses protected endpoints
7. **Refresh Token** → Gunakan refresh_token untuk mendapatkan access_token baru

---

## CORS Support

API Gateway mendukung CORS untuk semua origins dengan headers:

- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS`
- `Access-Control-Allow-Headers: Origin, Content-Type, Accept, Authorization`

---

## Service Dependencies

- **User Service**: `http://localhost:8081` (Required)
- **PostgreSQL**: Database untuk user data dan OTP storage
- **RabbitMQ**: Event publishing (Optional)

---

## Running the Services

1. **Start User Service:**

   ```bash
   cd services/user-service
   go run cmd/main.go
   ```

2. **Start API Gateway:**

   ```bash
   cd api-gateway
   go run main.go
   ```

3. **Test API Gateway:**
   ```bash
   curl http://localhost:8080/health
   ```
