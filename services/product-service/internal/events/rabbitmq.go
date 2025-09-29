package events

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/streadway/amqp"
)

// EventService handles RabbitMQ event publishing and consuming
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

// CheckoutInitEvent represents checkout initialization event from Payment-Service
type CheckoutInitEvent struct {
	PaymentID     string `json:"payment_id"`
	OrderID       string `json:"order_id"`
	UserID        string `json:"user_id"`
	ProductID     string `json:"product_id"`
	Quantity      int    `json:"quantity"`
	Amount        int64  `json:"amount"`
	TotalAmount   int64  `json:"total_amount"`
	PaymentMethod string `json:"payment_method"`
}

// ProductValidationResponse represents product validation response
type ProductValidationResponse struct {
	PaymentID string `json:"payment_id"`
	OrderID   string `json:"order_id"`
	ProductID string `json:"product_id"`
	Status    string `json:"status"` // "PRODUCT_OK" or "OUT_OF_STOCK"
	Message   string `json:"message,omitempty"`
	Stock     int    `json:"stock,omitempty"`
}

// OrderCompletedEvent represents order completion event
type OrderCompletedEvent struct {
	PaymentID     string `json:"payment_id"`
	OrderID       string `json:"order_id"`
	UserID        string `json:"user_id"`
	ProductID     string `json:"product_id"`
	Quantity      int    `json:"quantity"`
	Amount        int64  `json:"amount"`
	TotalAmount   int64  `json:"total_amount"`
	PaymentMethod string `json:"payment_method"`
	PaidAt        string `json:"paid_at"`
}

// OrderFailedEvent represents order failure event
type OrderFailedEvent struct {
	PaymentID     string `json:"payment_id"`
	OrderID       string `json:"order_id"`
	UserID        string `json:"user_id"`
	ProductID     string `json:"product_id"`
	Quantity      int    `json:"quantity"`
	Amount        int64  `json:"amount"`
	TotalAmount   int64  `json:"total_amount"`
	PaymentMethod string `json:"payment_method"`
	FailureReason string `json:"failure_reason"`
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
		username = "admin"
	}

	password := os.Getenv("RABBITMQ_PASSWORD")
	if password == "" {
		password = "secret123"
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
	exchanges := []string{"payment.events", "product.events", "user.events"}
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

	log.Println("✅ Product-Service connected to RabbitMQ successfully")

	return &EventService{
		conn:    conn,
		channel: ch,
	}, nil
}

// PublishProductValidationResponse publishes product validation response
func (es *EventService) PublishProductValidationResponse(response ProductValidationResponse) error {
	event := Event{
		Type:   "product.validation.response",
		UserID: "", // Not needed for validation response
		Data:   response,
		Timestamp: time.Now().Unix(),
	}

	return es.publishEvent("product.events", "product.validation.response", event)
}

// PublishStockReduction publishes stock reduction event for successful orders
func (es *EventService) PublishStockReduction(productID string, quantity int, orderID, userID string) error {
	event := Event{
		Type:   "product.stock.reduced",
		UserID: userID,
		Data: map[string]interface{}{
			"product_id": productID,
			"quantity":   quantity,
			"order_id":   orderID,
			"user_id":    userID,
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

// GetChannel returns the RabbitMQ channel for consumers
func (es *EventService) GetChannel() *amqp.Channel {
	return es.channel
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
