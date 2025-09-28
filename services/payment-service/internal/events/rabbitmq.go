package events

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/streadway/amqp"
)

// EventService handles RabbitMQ event publishing
type EventService struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// Event represents a generic event structure
type Event struct {
	Type      string      `json:"type"`
	UserID    string      `json:"user_id,omitempty"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

// PaymentCreatedEvent represents payment creation event
type PaymentCreatedEvent struct {
	PaymentID     string `json:"payment_id"`
	OrderID       string `json:"order_id"`
	UserID        string `json:"user_id"`
	ProductID     string `json:"product_id,omitempty"`
	Amount        int64  `json:"amount"`
	TotalAmount   int64  `json:"total_amount"`
	PaymentMethod string `json:"payment_method"`
	Status        string `json:"status"`
}

// PaymentStatusUpdatedEvent represents payment status update event
type PaymentStatusUpdatedEvent struct {
	PaymentID     string `json:"payment_id"`
	OrderID       string `json:"order_id"`
	UserID        string `json:"user_id"`
	ProductID     string `json:"product_id,omitempty"`
	OldStatus     string `json:"old_status"`
	NewStatus     string `json:"new_status"`
	Amount        int64  `json:"amount"`
	TotalAmount   int64  `json:"total_amount"`
	PaymentMethod string `json:"payment_method"`
	PaidAt        string `json:"paid_at,omitempty"`
}

// PaymentSuccessEvent represents successful payment event
type PaymentSuccessEvent struct {
	PaymentID     string `json:"payment_id"`
	OrderID       string `json:"order_id"`
	UserID        string `json:"user_id"`
	ProductID     string `json:"product_id,omitempty"`
	Amount        int64  `json:"amount"`
	TotalAmount   int64  `json:"total_amount"`
	PaymentMethod string `json:"payment_method"`
	PaidAt        string `json:"paid_at"`
}

// PaymentFailedEvent represents failed payment event
type PaymentFailedEvent struct {
	PaymentID     string `json:"payment_id"`
	OrderID       string `json:"order_id"`
	UserID        string `json:"user_id"`
	ProductID     string `json:"product_id,omitempty"`
	Amount        int64  `json:"amount"`
	TotalAmount   int64  `json:"total_amount"`
	PaymentMethod string `json:"payment_method"`
	FailureReason string `json:"failure_reason"`
}

// StockReductionEvent represents stock reduction event for successful payments
type StockReductionEvent struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
	OrderID   string `json:"order_id"`
	UserID    string `json:"user_id"`
}

// NewEventService creates a new event service
func NewEventService() (*EventService, error) {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env file not found in events package, using system env")
	}

	// Get RabbitMQ configuration from environment
	host := os.Getenv("RABBITMQ_HOST")
	if host == "" {
		host = "localhost"
	}

	port := os.Getenv("RABBITMQ_PORT")
	if port == "" {
		port = "5672"
	}

	username := os.Getenv("RABBITMQ_USERNAME")
	if username == "" {
		username = "guest"
	}

	password := os.Getenv("RABBITMQ_PASSWORD")
	if password == "" {
		password = "guest"
	}

	// Create connection URL
	url := fmt.Sprintf("amqp://%s:%s@%s:%s/", username, password, host, port)

	// Connect to RabbitMQ
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Create channel
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Declare exchanges
	exchanges := []string{"payment.events", "product.events", "notification.events"}
	for _, exchange := range exchanges {
		if err := ch.ExchangeDeclare(
			exchange, // name
			"topic",  // type
			true,     // durable
			false,    // auto-deleted
			false,    // internal
			false,    // no-wait
			nil,      // arguments
		); err != nil {
			ch.Close()
			conn.Close()
			return nil, fmt.Errorf("failed to declare exchange %s: %w", exchange, err)
		}
	}


	log.Println("✅ Connected to RabbitMQ successfully")

	return &EventService{
		conn:    conn,
		channel: ch,
	}, nil
}

// PublishPaymentCreated publishes payment creation event
func (es *EventService) PublishPaymentCreated(paymentID, orderID, userID string, productID *uuid.UUID, amount, totalAmount int64, paymentMethod, status string) error {
	productIDStr := ""
	if productID != nil {
		productIDStr = productID.String()
	}

	event := Event{
		Type:   "payment.created",
		UserID: userID,
		Data: PaymentCreatedEvent{
			PaymentID:     paymentID,
			OrderID:       orderID,
			UserID:        userID,
			ProductID:     productIDStr,
			Amount:        amount,
			TotalAmount:   totalAmount,
			PaymentMethod: paymentMethod,
			Status:        status,
		},
		Timestamp: time.Now().Unix(),
	}

	return es.publishEvent("payment.events", "payment.created", event)
}

// PublishPaymentStatusUpdated publishes payment status update event
func (es *EventService) PublishPaymentStatusUpdated(paymentID, orderID, userID string, productID *uuid.UUID, oldStatus, newStatus string, amount, totalAmount int64, paymentMethod string, paidAt *time.Time) error {
	productIDStr := ""
	if productID != nil {
		productIDStr = productID.String()
	}

	paidAtStr := ""
	if paidAt != nil {
		paidAtStr = paidAt.Format(time.RFC3339)
	}

	event := Event{
		Type:   "payment.status.updated",
		UserID: userID,
		Data: PaymentStatusUpdatedEvent{
			PaymentID:     paymentID,
			OrderID:       orderID,
			UserID:        userID,
			ProductID:     productIDStr,
			OldStatus:     oldStatus,
			NewStatus:     newStatus,
			Amount:        amount,
			TotalAmount:   totalAmount,
			PaymentMethod: paymentMethod,
			PaidAt:        paidAtStr,
		},
		Timestamp: time.Now().Unix(),
	}

	return es.publishEvent("payment.events", "payment.status.updated", event)
}

// PublishPaymentSuccess publishes successful payment event
func (es *EventService) PublishPaymentSuccess(paymentID, orderID, userID string, productID *uuid.UUID, amount, totalAmount int64, paymentMethod string, paidAt time.Time) error {
	productIDStr := ""
	if productID != nil {
		productIDStr = productID.String()
	}

	event := Event{
		Type:   "payment.success",
		UserID: userID,
		Data: PaymentSuccessEvent{
			PaymentID:     paymentID,
			OrderID:       orderID,
			UserID:        userID,
			ProductID:     productIDStr,
			Amount:        amount,
			TotalAmount:   totalAmount,
			PaymentMethod: paymentMethod,
			PaidAt:        paidAt.Format(time.RFC3339),
		},
		Timestamp: time.Now().Unix(),
	}

	return es.publishEvent("payment.events", "payment.success", event)
}

// PublishPaymentFailed publishes failed payment event
func (es *EventService) PublishPaymentFailed(paymentID, orderID, userID string, productID *uuid.UUID, amount, totalAmount int64, paymentMethod, failureReason string) error {
	productIDStr := ""
	if productID != nil {
		productIDStr = productID.String()
	}

	event := Event{
		Type:   "payment.failed",
		UserID: userID,
		Data: PaymentFailedEvent{
			PaymentID:     paymentID,
			OrderID:       orderID,
			UserID:        userID,
			ProductID:     productIDStr,
			Amount:        amount,
			TotalAmount:   totalAmount,
			PaymentMethod: paymentMethod,
			FailureReason: failureReason,
		},
		Timestamp: time.Now().Unix(),
	}

	return es.publishEvent("payment.events", "payment.failed", event)
}

// PublishStockReduction publishes stock reduction event
func (es *EventService) PublishStockReduction(productID uuid.UUID, quantity int, orderID, userID string) error {
	event := Event{
		Type:   "product.stock.reduced",
		UserID: userID,
		Data: StockReductionEvent{
			ProductID: productID.String(),
			Quantity:  quantity,
			OrderID:   orderID,
			UserID:    userID,
		},
		Timestamp: time.Now().Unix(),
	}

	return es.publishEvent("product.events", "product.stock.reduced", event)
}

// publishEvent publishes a generic event
func (es *EventService) publishEvent(exchange, routingKey string, event Event) error {
	// Marshal event to JSON
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish message
	err = es.channel.Publish(
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
			Timestamp:   time.Now(),
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	log.Printf("📤 Published event: %s to %s", routingKey, exchange)
	return nil
}

// Close closes the RabbitMQ connection
func (es *EventService) Close() error {
	if es.channel != nil {
		es.channel.Close()
	}
	if es.conn != nil {
		return es.conn.Close()
	}
	return nil
}

// HealthCheck checks if RabbitMQ connection is healthy
func (es *EventService) HealthCheck() error {
	if es.conn == nil || es.channel == nil {
		return fmt.Errorf("RabbitMQ connection not initialized")
	}

	// Try to declare a temporary queue to test connection
	_, err := es.channel.QueueDeclare(
		"health_check", // name
		false,          // durable
		true,           // delete when unused
		true,           // exclusive
		false,          // no-wait
		nil,            // arguments
	)

	if err != nil {
		return fmt.Errorf("RabbitMQ health check failed: %w", err)
	}

	// Clean up the temporary queue
	es.channel.QueueDelete("health_check", false, false, false)

	return nil
}
