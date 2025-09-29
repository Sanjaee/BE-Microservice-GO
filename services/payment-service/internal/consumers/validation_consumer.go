package consumers

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"payment-service/internal/events"
	"payment-service/internal/repository"

	"github.com/google/uuid"
	"github.com/streadway/amqp"
)

// ValidationConsumer handles validation responses from other services
type ValidationConsumer struct {
	eventSvc    *events.EventService
	paymentRepo *repository.PaymentRepository
	// Map to track pending validations
	pendingValidations map[string]*PendingValidation
	mu                sync.RWMutex
}

// PendingValidation tracks a pending validation request
type PendingValidation struct {
	PaymentID     string
	OrderID       string
	UserID        string
	ProductID     string
	Amount        int64
	TotalAmount   int64
	PaymentMethod string
	Quantity      int
	CreatedAt     time.Time
	// Validation responses
	ProductValidated bool
	UserValidated    bool
	ProductStatus    string
	UserStatus       string
	ProductMessage   string
	UserMessage      string
	ProductStock     int
}

// NewValidationConsumer creates a new validation consumer
func NewValidationConsumer(eventSvc *events.EventService, paymentRepo *repository.PaymentRepository) *ValidationConsumer {
	return &ValidationConsumer{
		eventSvc:          eventSvc,
		paymentRepo:       paymentRepo,
		pendingValidations: make(map[string]*PendingValidation),
	}
}

// Start starts consuming validation response events
func (vc *ValidationConsumer) Start() error {
	channel := vc.eventSvc.GetChannel()
	
	// Declare queue for validation responses
	queueName := "payment.validation.queue"
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

	// Bind queue to product.events exchange with validation response routing key
	err = channel.QueueBind(
		queueName,                      // queue name
		"product.validation.response",  // routing key
		"product.events",               // exchange
		false,                          // no-wait
		nil,                            // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind product validation queue: %w", err)
	}

	// Bind queue to user.events exchange with validation response routing key
	err = channel.QueueBind(
		queueName,                    // queue name
		"user.validation.response",   // routing key
		"user.events",                // exchange
		false,                        // no-wait
		nil,                          // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind user validation queue: %w", err)
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

	log.Println("🚀 Payment-Service validation consumer started")

	// Process messages in a goroutine
	go func() {
		for msg := range msgs {
			vc.processMessage(msg)
		}
	}()

	// Start cleanup routine for expired validations
	go vc.cleanupExpiredValidations()

	return nil
}

// processMessage processes a single message
func (vc *ValidationConsumer) processMessage(msg amqp.Delivery) {
	log.Printf("📨 Received validation response: %s", msg.RoutingKey)

	// Parse the event
	var event events.Event
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		log.Printf("❌ Failed to unmarshal event: %v", err)
		msg.Nack(false, false) // Reject message without requeue
		return
	}

	// Handle different event types
	switch event.Type {
	case "product.validation.response":
		vc.handleProductValidationResponse(event)
	case "user.validation.response":
		vc.handleUserValidationResponse(event)
	default:
		log.Printf("⚠️ Unknown event type: %s", event.Type)
	}

	// Acknowledge message
	msg.Ack(false)
}

// handleProductValidationResponse handles product validation response
func (vc *ValidationConsumer) handleProductValidationResponse(event events.Event) {
	log.Printf("📦 Processing product validation response")

	// Parse validation response
	responseData, ok := event.Data.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid product validation response format")
		return
	}

	paymentID, _ := responseData["payment_id"].(string)
	status, _ := responseData["status"].(string)
	message, _ := responseData["message"].(string)
	stock, _ := responseData["stock"].(float64)

	if paymentID == "" {
		log.Printf("❌ Missing payment ID in product validation response")
		return
	}

	// Update pending validation
	vc.mu.Lock()
	pending, exists := vc.pendingValidations[paymentID]
	if !exists {
		log.Printf("⚠️ No pending validation found for payment ID: %s", paymentID)
		vc.mu.Unlock()
		return
	}

	pending.ProductValidated = true
	pending.ProductStatus = status
	pending.ProductMessage = message
	pending.ProductStock = int(stock)
	vc.mu.Unlock()

	log.Printf("✅ Product validation updated for payment %s: %s", paymentID, status)

	// Check if all validations are complete
	vc.checkValidationComplete(paymentID)
}

// handleUserValidationResponse handles user validation response
func (vc *ValidationConsumer) handleUserValidationResponse(event events.Event) {
	log.Printf("👤 Processing user validation response")

	// Parse validation response
	responseData, ok := event.Data.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid user validation response format")
		return
	}

	paymentID, _ := responseData["payment_id"].(string)
	status, _ := responseData["status"].(string)
	message, _ := responseData["message"].(string)

	if paymentID == "" {
		log.Printf("❌ Missing payment ID in user validation response")
		return
	}

	// Update pending validation
	vc.mu.Lock()
	pending, exists := vc.pendingValidations[paymentID]
	if !exists {
		log.Printf("⚠️ No pending validation found for payment ID: %s", paymentID)
		vc.mu.Unlock()
		return
	}

	pending.UserValidated = true
	pending.UserStatus = status
	pending.UserMessage = message
	vc.mu.Unlock()

	log.Printf("✅ User validation updated for payment %s: %s", paymentID, status)

	// Check if all validations are complete
	vc.checkValidationComplete(paymentID)
}

// checkValidationComplete checks if all validations are complete and processes accordingly
func (vc *ValidationConsumer) checkValidationComplete(paymentID string) {
	vc.mu.Lock()
	pending, exists := vc.pendingValidations[paymentID]
	if !exists {
		vc.mu.Unlock()
		return
	}

	// Check if both validations are complete
	if !pending.ProductValidated || !pending.UserValidated {
		vc.mu.Unlock()
		return
	}

	// Remove from pending validations
	delete(vc.pendingValidations, paymentID)
	vc.mu.Unlock()

	log.Printf("🔍 All validations complete for payment %s", paymentID)

	// Check if both validations are successful
	if pending.ProductStatus == "PRODUCT_OK" && pending.UserStatus == "USER_OK" {
		log.Printf("✅ All validations successful for payment %s", paymentID)
		// Here you would proceed with Midtrans payment creation
		// For now, we'll just log success
		vc.handleValidationSuccess(pending)
	} else {
		log.Printf("❌ Validation failed for payment %s - Product: %s, User: %s", 
			paymentID, pending.ProductStatus, pending.UserStatus)
		// Handle validation failure
		vc.handleValidationFailure(pending)
	}
}

// handleValidationSuccess handles successful validation
func (vc *ValidationConsumer) handleValidationSuccess(pending *PendingValidation) {
	log.Printf("🎉 Validation successful for payment %s, proceeding with payment creation", pending.PaymentID)
	
	// Here you would:
	// 1. Create payment with Midtrans
	// 2. Save payment to database
	// 3. Return success response to client
	
	// For now, we'll just publish an order completed event (this would normally happen after Midtrans success)
	vc.eventSvc.PublishOrderCompleted(
		pending.PaymentID,
		pending.OrderID,
		pending.UserID,
		func() *uuid.UUID {
			if pending.ProductID != "" {
				if id, err := uuid.Parse(pending.ProductID); err == nil {
					return &id
				}
			}
			return nil
		}(),
		pending.Quantity,
		pending.Amount,
		pending.TotalAmount,
		pending.PaymentMethod,
		time.Now(),
	)
}

// handleValidationFailure handles validation failure
func (vc *ValidationConsumer) handleValidationFailure(pending *PendingValidation) {
	log.Printf("💥 Validation failed for payment %s", pending.PaymentID)
	
	// Publish order failed event
	vc.eventSvc.PublishOrderFailed(
		pending.PaymentID,
		pending.OrderID,
		pending.UserID,
		func() *uuid.UUID {
			if pending.ProductID != "" {
				if id, err := uuid.Parse(pending.ProductID); err == nil {
					return &id
				}
			}
			return nil
		}(),
		pending.Quantity,
		pending.Amount,
		pending.TotalAmount,
		pending.PaymentMethod,
		fmt.Sprintf("Validation failed - Product: %s, User: %s", pending.ProductStatus, pending.UserStatus),
	)
}

// AddPendingValidation adds a pending validation to track
func (vc *ValidationConsumer) AddPendingValidation(paymentID, orderID, userID, productID string, quantity int, amount, totalAmount int64, paymentMethod string) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	vc.pendingValidations[paymentID] = &PendingValidation{
		PaymentID:     paymentID,
		OrderID:       orderID,
		UserID:        userID,
		ProductID:     productID,
		Amount:        amount,
		TotalAmount:   totalAmount,
		PaymentMethod: paymentMethod,
		Quantity:      quantity,
		CreatedAt:     time.Now(),
		ProductValidated: false,
		UserValidated:    false,
	}

	log.Printf("📝 Added pending validation for payment %s", paymentID)
}

// cleanupExpiredValidations cleans up expired validations
func (vc *ValidationConsumer) cleanupExpiredValidations() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		vc.mu.Lock()
		now := time.Now()
		for paymentID, pending := range vc.pendingValidations {
			if now.Sub(pending.CreatedAt) > 10*time.Minute {
				log.Printf("🧹 Cleaning up expired validation for payment %s", paymentID)
				delete(vc.pendingValidations, paymentID)
			}
		}
		vc.mu.Unlock()
	}
}

