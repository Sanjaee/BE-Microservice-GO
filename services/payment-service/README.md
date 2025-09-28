# Payment Service

A comprehensive payment service built with Go, integrating Midtrans payment gateway for handling various payment methods including bank transfer, credit card, GoPay, and QRIS.

## Features

- **Multiple Payment Methods**: Support for bank transfer, credit card, GoPay, QRIS, and ShopeePay
- **Midtrans Integration**: Full integration with Midtrans payment gateway
- **Real-time Status Updates**: Webhook support for payment status updates
- **Event-driven Architecture**: RabbitMQ integration for payment events
- **Caching**: Redis caching for improved performance
- **Database**: PostgreSQL with GORM for data persistence
- **RESTful API**: Clean and well-documented API endpoints

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   API Gateway   │    │  Payment Service│    │   Midtrans      │
│                 │◄──►│                 │◄──►│   Gateway       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌─────────────────┐
                       │   RabbitMQ      │
                       │   (Events)      │
                       └─────────────────┘
                              │
                              ▼
                       ┌─────────────────┐
                       │     Redis       │
                       │   (Cache)       │
                       └─────────────────┘
                              │
                              ▼
                       ┌─────────────────┐
                       │   PostgreSQL    │
                       │   (Database)    │
                       └─────────────────┘
```

## Payment Flow

1. **Payment Creation**: User initiates payment through checkout
2. **Midtrans Integration**: Payment request sent to Midtrans
3. **Payment Processing**: User completes payment via chosen method
4. **Webhook Callback**: Midtrans sends status update via webhook
5. **Event Publishing**: Payment events published to RabbitMQ
6. **Status Update**: Payment status updated in database and cache

## API Endpoints

### Public Endpoints

- `GET /health` - Health check
- `GET /api/v1/payments/config` - Get Midtrans configuration
- `POST /api/v1/payments/midtrans/callback` - Midtrans webhook callback

### Protected Endpoints (Require Authentication)

- `POST /api/v1/payments` - Create new payment
- `GET /api/v1/payments/:id` - Get payment by ID
- `GET /api/v1/payments/order/:order_id` - Get payment by order ID
- `GET /api/v1/payments/user` - Get user payments

## Environment Variables

Create a `.env` file based on `env.example`:

```bash
# Server Configuration
PORT=8083
PAYMENT_SERVICE_URL=http://localhost:8083

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=microservice_db
DB_SSLMODE=disable

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

# Midtrans Configuration
MIDTRANS_ENVIRONMENT=sandbox
MIDTRANS_SERVER_KEY=SB-Mid-server-4zIt7djwCeRdMpgF4gXDjciC
MIDTRANS_CLIENT_KEY=SB-Mid-client-4zIt7djwCeRdMpgF4gXDjciC

# Service URLs
USER_SERVICE_URL=http://localhost:8081
PRODUCT_SERVICE_URL=http://localhost:8082

# JWT Configuration
JWT_SECRET=your-jwt-secret-key
JWT_EXPIRY=24h
```

## Database Schema

### Payment Table

```sql
CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id VARCHAR UNIQUE NOT NULL,
    user_id UUID NOT NULL,
    product_id UUID,
    amount BIGINT NOT NULL,
    admin_fee BIGINT DEFAULT 0,
    total_amount BIGINT NOT NULL,
    payment_method VARCHAR NOT NULL,
    payment_type VARCHAR,
    status VARCHAR DEFAULT 'PENDING',
    notes TEXT,
    snap_redirect_url TEXT,
    midtrans_transaction_id VARCHAR,
    transaction_status VARCHAR,
    fraud_status VARCHAR,
    payment_code VARCHAR,
    va_number VARCHAR,
    bank_type VARCHAR,
    expiry_time TIMESTAMP,
    paid_at TIMESTAMP,
    midtrans_response TEXT,
    midtrans_action TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

## Payment Methods

### Bank Transfer

- Virtual Account (VA) generation
- Support for BCA, BNI, BRI, Mandiri, Permata
- Automatic VA number and bank type detection

### Credit Card

- Secure payment processing
- 3D Secure authentication
- Support for Visa, Mastercard, JCB

### GoPay

- Mobile wallet integration
- QR code generation
- Real-time payment status

### QRIS

- QR code generation
- Multi-bank support
- Mobile banking integration

## Events

The service publishes the following events to RabbitMQ:

- `payment.created` - Payment created
- `payment.status.updated` - Payment status changed
- `payment.success` - Payment completed successfully
- `payment.failed` - Payment failed
- `product.stock.reduced` - Stock reduced after successful payment

## Running the Service

1. **Install Dependencies**:

   ```bash
   go mod tidy
   ```

2. **Set up Environment**:

   ```bash
   cp env.example .env
   # Edit .env with your configuration
   ```

3. **Run the Service**:

   ```bash
   go run cmd/main.go
   ```

4. **Using Docker**:
   ```bash
   docker build -t payment-service .
   docker run -p 8083:8083 --env-file .env payment-service
   ```

## Testing

### Health Check

```bash
curl http://localhost:8083/health
```

### Create Payment

```bash
curl -X POST http://localhost:8083/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "product_id": "product-uuid",
    "amount": 100000,
    "admin_fee": 2500,
    "payment_method": "bank_transfer",
    "bank_type": "bca",
    "notes": "Test payment"
  }'
```

### Get Payment

```bash
curl http://localhost:8083/api/v1/payments/payment-uuid \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## Frontend Integration

The service is integrated with a Next.js frontend that provides:

- **Checkout Page**: Product selection and payment method choice
- **Payment Page**: Payment instructions and status tracking
- **Real-time Updates**: Automatic payment status refresh

### Frontend Routes

- `/checkout/[id]` - Product checkout page
- `/payment/[id]` - Payment status and instructions page

## Security

- JWT token authentication
- Request validation
- Signature verification for webhooks
- Environment-based configuration
- Secure payment processing through Midtrans

## Monitoring

- Health check endpoints
- Structured logging
- Error tracking
- Performance metrics

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

This project is licensed under the MIT License.
