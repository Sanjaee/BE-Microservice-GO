package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"payment-service/internal/cache"
	"payment-service/internal/events"
	"payment-service/internal/handlers"
	"payment-service/internal/models"
	"payment-service/internal/repository"
	"payment-service/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	DB *gorm.DB
)

func initDB() {
	// Load .env for main application configuration
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env file not found in main, using system env")
	}

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
		dbUser = "postgres"
	}

	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		dbPass = "password"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "microservice_db"
	}

	// Connection string
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		dbHost, dbUser, dbPass, dbName, dbPort,
	)

	// Connect to database
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}

	// Get underlying sql.DB to configure connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("❌ Failed to get underlying sql.DB: %v", err)
	}

	// Configure connection pool
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Println("✅ Connected to database successfully")

	// Auto migrate the schema (only Payment table, no foreign key constraints)
	if err := DB.AutoMigrate(&models.Payment{}); err != nil {
		log.Fatalf("❌ Failed to migrate database: %v", err)
	}

	log.Println("✅ Database migration completed")
}

func main() {
	// Initialize database
	initDB()

	// Initialize Redis cache
	cacheSvc, err := cache.NewCacheService()
	if err != nil {
		log.Fatalf("❌ Failed to initialize cache service: %v", err)
	}
	defer cacheSvc.Close()

	// Initialize RabbitMQ events
	eventSvc, err := events.NewEventService()
	if err != nil {
		log.Fatalf("❌ Failed to initialize event service: %v", err)
	}
	defer eventSvc.Close()

	// Initialize services
	midtransSvc := services.NewMidtransService()
	paymentRepo := repository.NewPaymentRepository(DB)

	// Get service URLs from environment
	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		userServiceURL = "http://localhost:8081"
	}

	productServiceURL := os.Getenv("PRODUCT_SERVICE_URL")
	if productServiceURL == "" {
		productServiceURL = "http://localhost:8082"
	}

	// Initialize handlers
	paymentHandler := handlers.NewPaymentHandler(
		paymentRepo,
		midtransSvc,
		eventSvc,
		cacheSvc,
		userServiceURL,
		productServiceURL,
	)

	// Initialize Gin router
	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		// Check database connection
		sqlDB, err := DB.DB()
		if err != nil {
			c.JSON(500, gin.H{
				"status":  "error",
				"service": "payment-service",
				"error":   "Database connection failed",
			})
			return
		}

		if err := sqlDB.Ping(); err != nil {
			c.JSON(500, gin.H{
				"status":  "error",
				"service": "payment-service",
				"error":   "Database ping failed",
			})
			return
		}

		// Check Redis connection
		if err := cacheSvc.HealthCheck(); err != nil {
			c.JSON(500, gin.H{
				"status":  "error",
				"service": "payment-service",
				"error":   "Redis connection failed",
			})
			return
		}

		// Check RabbitMQ connection
		if err := eventSvc.HealthCheck(); err != nil {
			c.JSON(500, gin.H{
				"status":  "error",
				"service": "payment-service",
				"error":   "RabbitMQ connection failed",
			})
			return
		}

		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "payment-service",
			"version": "1.0.0",
		})
	})

	// API routes
	api := r.Group("/api/v1")
	{
		// Payment routes
		payments := api.Group("/payments")
		{
			// Public routes
			payments.GET("/config", paymentHandler.GetMidtransConfig)
			payments.POST("/midtrans/callback", paymentHandler.MidtransCallback)

			// Protected routes (require authentication)
			protected := payments.Group("")
			// protected.Use(authMiddleware()) // Add auth middleware here
			{
				protected.POST("", paymentHandler.CreatePayment)
				protected.GET("/:id", paymentHandler.GetPayment)
				protected.GET("/order/:order_id", paymentHandler.GetPaymentByOrderID)
				protected.GET("/user", paymentHandler.GetUserPayments)
			}
		}
	}

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}

	log.Printf("🚀 Payment Service running on http://localhost:%s", port)
	log.Printf("📚 Available endpoints:")
	log.Printf("  POST /api/v1/payments              - Create payment")
	log.Printf("  GET  /api/v1/payments/:id          - Get payment by ID")
	log.Printf("  GET  /api/v1/payments/order/:id    - Get payment by order ID")
	log.Printf("  GET  /api/v1/payments/user         - Get user payments")
	log.Printf("  GET  /api/v1/payments/config       - Get Midtrans config")
	log.Printf("  POST /api/v1/payments/midtrans/callback - Midtrans webhook")
	log.Printf("  GET  /health                       - Health check")

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}
