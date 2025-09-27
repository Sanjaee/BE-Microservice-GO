package consumers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"user-service/internal/events"
	"user-service/internal/models"
	"user-service/internal/services"

	"github.com/joho/godotenv"
	"github.com/streadway/amqp"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// initDB initializes database connection
func initDB() (*gorm.DB, error) {
	// Get database configuration from environment
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "user_service"
	}

	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		dbPass = "userpass"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "userdb"
	}

	// Create DSN
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)

	// Connect to database
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto migrate
	if err := db.AutoMigrate(&models.User{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

// EmailConsumer handles email-related events from RabbitMQ
type EmailConsumer struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	emailService *services.EmailService
	db           *gorm.DB
}

// NewEmailConsumer creates a new email consumer
func NewEmailConsumer() (*EmailConsumer, error) {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env file not found in email consumer, using system env")
	}

	// Initialize email service
	emailService, err := services.NewEmailService()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize email service: %w", err)
	}

	// Initialize database connection
	db, err := initDB()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Connect to RabbitMQ (reuse connection logic from events)
	conn, err := amqp.Dial("amqp://admin:secret123@localhost:5672/")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Declare exchange
	if err := ch.ExchangeDeclare(
		"user.events",
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare queue for email events
	q, err := ch.QueueDeclare(
		"email_queue",
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to exchange for multiple event types
	bindings := []string{
		"user.registered",
		"user.verified", 
		"password.reset",
		"password.reset.success",
	}
	
	for _, binding := range bindings {
		if err := ch.QueueBind(
			q.Name,
			binding,
			"user.events",
			false,
			nil,
		); err != nil {
			ch.Close()
			conn.Close()
			return nil, fmt.Errorf("failed to bind queue to %s: %w", binding, err)
		}
	}

	return &EmailConsumer{
		conn:         conn,
		channel:      ch,
		emailService: emailService,
		db:           db,
	}, nil
}

// Start starts consuming email events
func (ec *EmailConsumer) Start() error {
	log.Println("🚀 Starting email consumer...")

	// Set QoS to process one message at a time
	if err := ec.channel.Qos(1, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Start consuming messages
	msgs, err := ec.channel.Consume(
		"email_queue",
		"",    // consumer
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	// Process messages
	go func() {
		for msg := range msgs {
			ec.processMessage(msg)
		}
	}()

	log.Println("✅ Email consumer started successfully")
	return nil
}

// processMessage processes a single message
func (ec *EmailConsumer) processMessage(msg amqp.Delivery) {
	log.Printf("📧 Processing email event: %s", msg.RoutingKey)

	var event events.Event
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		log.Printf("❌ Failed to unmarshal event: %v", err)
		msg.Nack(false, false) // Reject message
		return
	}

	// Process based on event type
	switch event.Type {
	case "user.registered":
		if err := ec.handleUserRegistered(event); err != nil {
			log.Printf("❌ Failed to handle user registered event: %v", err)
			msg.Nack(false, true) // Reject and requeue
			return
		}
	case "user.verified":
		if err := ec.handleUserVerified(event); err != nil {
			log.Printf("❌ Failed to handle user verified event: %v", err)
			msg.Nack(false, true) // Reject and requeue
			return
		}
	case "password.reset":
		if err := ec.handlePasswordReset(event); err != nil {
			log.Printf("❌ Failed to handle password reset event: %v", err)
			msg.Nack(false, true) // Reject and requeue
			return
		}
	case "password.reset.success":
		if err := ec.handlePasswordResetSuccess(event); err != nil {
			log.Printf("❌ Failed to handle password reset success event: %v", err)
			msg.Nack(false, true) // Reject and requeue
			return
		}
	default:
		log.Printf("⚠️ Unknown event type: %s", event.Type)
		msg.Ack(false) // Acknowledge unknown events
		return
	}

	// Acknowledge successful processing
	msg.Ack(false)
	log.Printf("✅ Successfully processed email event: %s", event.Type)
}

// handleUserRegistered handles user registration email
func (ec *EmailConsumer) handleUserRegistered(event events.Event) error {
	// Extract user data from event
	userData, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid user data format")
	}

	userID, ok := userData["user_id"].(string)
	if !ok {
		return fmt.Errorf("missing user_id")
	}

	username, ok := userData["username"].(string)
	if !ok {
		return fmt.Errorf("missing username")
	}

	email, ok := userData["email"].(string)
	if !ok {
		return fmt.Errorf("missing email")
	}

	// Get OTP from database
	var user models.User
	if err := ec.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	if user.OTPCode == nil {
		return fmt.Errorf("no OTP found for user")
	}

	otp := *user.OTPCode

	log.Printf("📧 Sending OTP email to: %s (%s)", username, email)

	// Send OTP email
	if err := ec.emailService.SendOTPEmail(email, username, otp); err != nil {
		return fmt.Errorf("failed to send OTP email: %w", err)
	}

	log.Printf("✅ OTP email sent successfully to: %s", email)
	return nil
}

// handleUserVerified handles user verification email
func (ec *EmailConsumer) handleUserVerified(event events.Event) error {
	// Extract user data from event
	userData, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid user data format")
	}

	username, ok := userData["username"].(string)
	if !ok {
		return fmt.Errorf("missing username")
	}

	email, ok := userData["email"].(string)
	if !ok {
		return fmt.Errorf("missing email")
	}

	log.Printf("📧 Sending welcome email to: %s (%s)", username, email)

	// Send welcome email
	if err := ec.emailService.SendWelcomeEmail(email, username); err != nil {
		return fmt.Errorf("failed to send welcome email: %w", err)
	}

	log.Printf("✅ Welcome email sent successfully to: %s", email)
	return nil
}

// handlePasswordReset handles password reset email
func (ec *EmailConsumer) handlePasswordReset(event events.Event) error {
	// Extract user data from event
	userData, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid user data format")
	}

	userID, ok := userData["user_id"].(string)
	if !ok {
		return fmt.Errorf("missing user_id")
	}

	username, ok := userData["username"].(string)
	if !ok {
		return fmt.Errorf("missing username")
	}

	email, ok := userData["email"].(string)
	if !ok {
		return fmt.Errorf("missing email")
	}

	// Get OTP from database
	var user models.User
	if err := ec.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	if user.OTPCode == nil {
		return fmt.Errorf("no OTP found for user")
	}

	otp := *user.OTPCode

	log.Printf("📧 Sending password reset email to: %s (%s)", username, email)

	// Send password reset email
	if err := ec.emailService.SendPasswordResetEmail(email, username, otp); err != nil {
		return fmt.Errorf("failed to send password reset email: %w", err)
	}

	log.Printf("✅ Password reset email sent successfully to: %s", email)
	return nil
}

// handlePasswordResetSuccess handles password reset success email
func (ec *EmailConsumer) handlePasswordResetSuccess(event events.Event) error {
	// Extract user data from event
	userData, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid user data format")
	}

	username, ok := userData["username"].(string)
	if !ok {
		return fmt.Errorf("missing username")
	}

	email, ok := userData["email"].(string)
	if !ok {
		return fmt.Errorf("missing email")
	}

	log.Printf("📧 Sending password reset success email to: %s (%s)", username, email)

	// Send password reset success email
	if err := ec.emailService.SendPasswordResetSuccessEmail(email, username); err != nil {
		return fmt.Errorf("failed to send password reset success email: %w", err)
	}

	log.Printf("✅ Password reset success email sent successfully to: %s", email)
	return nil
}

// Stop stops the email consumer
func (ec *EmailConsumer) Stop() error {
	log.Println("🛑 Stopping email consumer...")

	if ec.channel != nil {
		ec.channel.Close()
	}
	if ec.conn != nil {
		return ec.conn.Close()
	}

	log.Println("✅ Email consumer stopped")
	return nil
}

// HealthCheck checks if the email consumer is healthy
func (ec *EmailConsumer) HealthCheck() error {
	if ec.conn == nil || ec.channel == nil {
		return fmt.Errorf("email consumer not initialized")
	}

	// Check email service health
	if err := ec.emailService.HealthCheck(); err != nil {
		return fmt.Errorf("email service health check failed: %w", err)
	}

	return nil
}
