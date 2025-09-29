package consumers

import (
	"encoding/json"
	"fmt"
	"log"

	"product-service/internal/events"
	"product-service/internal/models"
	"product-service/internal/repository"

	"github.com/google/uuid"
	"github.com/streadway/amqp"
	"gorm.io/gorm"
)

// CheckoutConsumer handles checkout-related events from RabbitMQ
type CheckoutConsumer struct {
	eventSvc *events.EventService
	repo     *repository.ProductRepository
}

// NewCheckoutConsumer creates a new checkout consumer
func NewCheckoutConsumer(eventSvc *events.EventService, repo *repository.ProductRepository) *CheckoutConsumer {
	return &CheckoutConsumer{
		eventSvc: eventSvc,
		repo:     repo,
	}
}

// Start starts consuming checkout events
func (cc *CheckoutConsumer) Start() error {
	channel := cc.eventSvc.GetChannel()
	
	// Declare queue for checkout events
	queueName := "product.checkout.queue"
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

	log.Println("🚀 Product-Service checkout consumer started")

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
	log.Printf("🛒 Processing checkout init event")

	// Parse checkout data
	checkoutData, ok := event.Data.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid checkout data format")
		cc.sendValidationResponse("", "", "", "OUT_OF_STOCK", "Invalid checkout data format", 0)
		return
	}

	// Extract required fields
	paymentID, _ := checkoutData["payment_id"].(string)
	orderID, _ := checkoutData["order_id"].(string)
	productIDStr, _ := checkoutData["product_id"].(string)
	quantity, _ := checkoutData["quantity"].(float64)

	if paymentID == "" || orderID == "" || productIDStr == "" {
		log.Printf("❌ Missing required fields in checkout data")
		cc.sendValidationResponse(paymentID, orderID, productIDStr, "OUT_OF_STOCK", "Missing required fields", 0)
		return
	}

	// Parse product ID
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		log.Printf("❌ Invalid product ID: %v", err)
		cc.sendValidationResponse(paymentID, orderID, productIDStr, "OUT_OF_STOCK", "Invalid product ID", 0)
		return
	}

	// Get product from database directly (bypassing cache to avoid Redis issues)
	var product models.Product
	if err := cc.repo.GetDB().Preload("User").Preload("Images").First(&product, "id = ?", productID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("❌ Product not found: %s", productIDStr)
			cc.sendValidationResponse(paymentID, orderID, productIDStr, "OUT_OF_STOCK", "Product not found", 0)
		} else {
			log.Printf("❌ Failed to get product: %v", err)
			cc.sendValidationResponse(paymentID, orderID, productIDStr, "OUT_OF_STOCK", "Database error", 0)
		}
		return
	}

	// Check if product is active
	if !product.IsActive {
		log.Printf("❌ Product is not active: %s", productIDStr)
		cc.sendValidationResponse(paymentID, orderID, productIDStr, "OUT_OF_STOCK", "Product is not active", product.Stock)
		return
	}

	// Check stock availability
	requiredQuantity := int(quantity)
	if requiredQuantity <= 0 {
		requiredQuantity = 1 // Default to 1 if not specified
	}

	if product.Stock < requiredQuantity {
		log.Printf("❌ Insufficient stock: required %d, available %d", requiredQuantity, product.Stock)
		cc.sendValidationResponse(paymentID, orderID, productIDStr, "OUT_OF_STOCK", "Insufficient stock", product.Stock)
		return
	}

	// Product validation successful
	log.Printf("✅ Product validation successful: %s (stock: %d)", productIDStr, product.Stock)
	cc.sendValidationResponse(paymentID, orderID, productIDStr, "PRODUCT_OK", "Product validation successful", product.Stock)
}

// sendValidationResponse sends validation response back to payment service
func (cc *CheckoutConsumer) sendValidationResponse(paymentID, orderID, productID, status, message string, stock int) {
	response := events.ProductValidationResponse{
		PaymentID: paymentID,
		OrderID:   orderID,
		ProductID: productID,
		Status:    status,
		Message:   message,
		Stock:     stock,
	}

	if err := cc.eventSvc.PublishProductValidationResponse(response); err != nil {
		log.Printf("❌ Failed to publish validation response: %v", err)
	} else {
		log.Printf("📤 Published validation response: %s for product %s", status, productID)
	}
}

