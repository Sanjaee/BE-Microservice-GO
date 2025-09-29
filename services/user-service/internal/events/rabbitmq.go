package events

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

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

// UserRegisteredEvent represents user registration event
type UserRegisteredEvent struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// UserVerifiedEvent represents user verification event
type UserVerifiedEvent struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// UserLoginEvent represents user login event
type UserLoginEvent struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// PasswordResetEvent represents password reset event
type PasswordResetEvent struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// PasswordResetSuccessEvent represents password reset success event
type PasswordResetSuccessEvent struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
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
	if err := ch.ExchangeDeclare(
		"user.events", // name
		"topic",       // type
		true,          // durable
		false,         // auto-deleted
		false,         // internal
		false,         // no-wait
		nil,           // arguments
	); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	return &EventService{
		conn:    conn,
		channel: ch,
	}, nil
}

// PublishUserRegistered publishes user registration event
func (es *EventService) PublishUserRegistered(userID, username, email string) error {
	event := Event{
		Type: "user.registered",
		Data: UserRegisteredEvent{
			UserID:   userID,
			Username: username,
			Email:    email,
		},
	}

	return es.publishEvent("user.registered", event)
}

// PublishUserVerified publishes user verification event
func (es *EventService) PublishUserVerified(userID, username, email string) error {
	event := Event{
		Type: "user.verified",
		Data: UserVerifiedEvent{
			UserID:   userID,
			Username: username,
			Email:    email,
		},
	}

	return es.publishEvent("user.verified", event)
}

// PublishUserLogin publishes user login event
func (es *EventService) PublishUserLogin(userID, username, email string) error {
	event := Event{
		Type: "user.login",
		Data: UserLoginEvent{
			UserID:   userID,
			Username: username,
			Email:    email,
		},
	}

	return es.publishEvent("user.login", event)
}

// PublishPasswordReset publishes password reset event
func (es *EventService) PublishPasswordReset(userID, username, email string) error {
	event := Event{
		Type: "password.reset",
		Data: PasswordResetEvent{
			UserID:   userID,
			Username: username,
			Email:    email,
		},
	}

	return es.publishEvent("password.reset", event)
}

// PublishPasswordResetSuccess publishes password reset success event
func (es *EventService) PublishPasswordResetSuccess(userID, username, email string) error {
	event := Event{
		Type: "password.reset.success",
		Data: PasswordResetSuccessEvent{
			UserID:   userID,
			Username: username,
			Email:    email,
		},
	}

	return es.publishEvent("password.reset.success", event)
}

// UserValidationResponse represents user validation response
type UserValidationResponse struct {
	PaymentID string `json:"payment_id"`
	OrderID   string `json:"order_id"`
	UserID    string `json:"user_id"`
	Status    string `json:"status"` // "USER_OK" or "USER_INVALID"
	Message   string `json:"message,omitempty"`
}

// PublishUserValidationResponse publishes user validation response
func (es *EventService) PublishUserValidationResponse(response UserValidationResponse) error {
	event := Event{
		Type:   "user.validation.response",
		UserID: response.UserID,
		Data:   response,
	}

	return es.publishEvent("user.validation.response", event)
}

// publishEvent publishes a generic event
func (es *EventService) publishEvent(routingKey string, event Event) error {
	// Marshal event to JSON
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish message
	err = es.channel.Publish(
		"user.events", // exchange
		routingKey,    // routing key
		false,         // mandatory
		false,         // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

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
