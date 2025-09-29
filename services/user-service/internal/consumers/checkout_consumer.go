package consumers

import (
	"encoding/json"
	"fmt"
	"log"

	"user-service/internal/events"
	"user-service/internal/repository"

	"github.com/google/uuid"
	"github.com/streadway/amqp"
)

// CheckoutConsumer handles checkout-related events from RabbitMQ
type CheckoutConsumer struct {
	eventSvc *events.EventService
	userRepo *repository.UserRepository
}


// NewCheckoutConsumer creates a new checkout consumer
func NewCheckoutConsumer(eventSvc *events.EventService, userRepo *repository.UserRepository) *CheckoutConsumer {
	return &CheckoutConsumer{
		eventSvc: eventSvc,
		userRepo: userRepo,
	}
}

// Start starts consuming checkout events
func (cc *CheckoutConsumer) Start() error {
	channel := cc.eventSvc.GetChannel()
	
	// Declare queue for checkout events
	queueName := "user.checkout.queue"
	_, err := channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to payment.events exchange with checkout.init routing key
	err = channel.QueueBind(
		queueName,           // queue name
		"checkout.init",     // routing key
		"payment.events",    // exchange
		false,               // no-wait
		nil,                 // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	// Set QoS to process one message at a time
	err = channel.Qos(1, 0, false)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Start consuming messages
	msgs, err := channel.Consume(
		queueName, // queue
		"",        // consumer
		false,     // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	log.Println("🚀 User-Service checkout consumer started")

	// Process messages in a goroutine
	go func() {
		for msg := range msgs {
			cc.processMessage(msg)
		}
	}()

	return nil
}

// processMessage processes a single message
func (cc *CheckoutConsumer) processMessage(msg amqp.Delivery) {
	log.Printf("📨 Received checkout event: %s", msg.RoutingKey)

	// Parse the event
	var event events.Event
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		log.Printf("❌ Failed to unmarshal event: %v", err)
		msg.Nack(false, false) // Reject message without requeue
		return
	}

	// Handle different event types
	switch event.Type {
	case "checkout.init":
		cc.handleCheckoutInit(event)
	default:
		log.Printf("⚠️ Unknown event type: %s", event.Type)
	}

	// Acknowledge message
	msg.Ack(false)
}

// handleCheckoutInit handles checkout initialization event
func (cc *CheckoutConsumer) handleCheckoutInit(event events.Event) {
	log.Printf("🛒 Processing checkout init event for user validation")

	// Parse checkout data
	checkoutData, ok := event.Data.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid checkout data format")
		cc.sendValidationResponse("", "", "", "USER_INVALID", "Invalid checkout data format")
		return
	}

	// Extract required fields
	paymentID, _ := checkoutData["payment_id"].(string)
	orderID, _ := checkoutData["order_id"].(string)
	userIDStr, _ := checkoutData["user_id"].(string)

	if paymentID == "" || orderID == "" || userIDStr == "" {
		log.Printf("❌ Missing required fields in checkout data")
		cc.sendValidationResponse(paymentID, orderID, userIDStr, "USER_INVALID", "Missing required fields")
		return
	}

	// Parse user ID
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		log.Printf("❌ Invalid user ID: %v", err)
		cc.sendValidationResponse(paymentID, orderID, userIDStr, "USER_INVALID", "Invalid user ID")
		return
	}

	// Get user from database
	user, err := cc.userRepo.GetByID(userID)
	if err != nil {
		log.Printf("❌ Failed to get user: %v", err)
		cc.sendValidationResponse(paymentID, orderID, userIDStr, "USER_INVALID", "User not found")
		return
	}

	// Check if user is active/verified
	// Note: You might want to add an IsActive or IsVerified field to your User model
	// For now, we'll assume all users in the database are valid
	if user.ID == uuid.Nil {
		log.Printf("❌ User is not valid: %s", userIDStr)
		cc.sendValidationResponse(paymentID, orderID, userIDStr, "USER_INVALID", "User is not valid")
		return
	}

	// User validation successful
	log.Printf("✅ User validation successful: %s", userIDStr)
	cc.sendValidationResponse(paymentID, orderID, userIDStr, "USER_OK", "User validation successful")
}

// sendValidationResponse sends validation response back to payment service
func (cc *CheckoutConsumer) sendValidationResponse(paymentID, orderID, userID, status, message string) {
	response := events.UserValidationResponse{
		PaymentID: paymentID,
		OrderID:   orderID,
		UserID:    userID,
		Status:    status,
		Message:   message,
	}

	if err := cc.eventSvc.PublishUserValidationResponse(response); err != nil {
		log.Printf("❌ Failed to publish validation response: %v", err)
	} else {
		log.Printf("📤 Published validation response: %s for user %s", status, userID)
	}
}
