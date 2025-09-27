package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"user-service/internal/consumers"
	"user-service/internal/events"
	"user-service/internal/handlers"
	"user-service/internal/models"
)

var (
	DB             *gorm.DB
	EventService   *events.EventService
	EmailConsumer  *consumers.EmailConsumer
)

func initDB() {
	// Load .env for main application configuration
	// Note: Each internal package also loads .env independently for modularity
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

	// Connection string
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		dbHost, dbUser, dbPass, dbName, dbPort,
	)

	// Connect to database using GORM
	var errDB error
	DB, errDB = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if errDB != nil {
		log.Fatalf("❌ Failed to connect to database: %v", errDB)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("❌ Failed to get generic DB: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("❌ Database not responding: %v", err)
	}

	// Auto migrate the User model
	if err := DB.AutoMigrate(&models.User{}); err != nil {
		log.Fatalf("❌ Failed to migrate database: %v", err)
	}

	// Force update OTP field size if needed
	if err := DB.Exec("ALTER TABLE users ALTER COLUMN otp_code TYPE VARCHAR(6)").Error; err != nil {
		log.Printf("⚠️ Could not alter otp_code column (might already be correct): %v", err)
	}

	log.Println("✅ Database connected and migrated successfully!")
}


func initRabbitMQ() {
	var err error
	EventService, err = events.NewEventService()
	if err != nil {
		log.Printf("⚠️ Failed to connect to RabbitMQ: %v", err)
		log.Println("⚠️ Continuing without RabbitMQ (events will not be published)")
		EventService = nil
	} else {
		log.Println("✅ RabbitMQ connected successfully!")
	}
}

func initEmailConsumer() {
	var err error
	EmailConsumer, err = consumers.NewEmailConsumer()
	if err != nil {
		log.Printf("⚠️ Failed to initialize email consumer: %v", err)
		log.Println("⚠️ Continuing without email consumer...")
	} else {
		log.Println("✅ Email consumer initialized successfully")
		
		// Start the email consumer
		if err := EmailConsumer.Start(); err != nil {
			log.Printf("⚠️ Failed to start email consumer: %v", err)
		} else {
			log.Println("✅ Email consumer started successfully")
		}
	}
}

func setupRoutes() *gin.Engine {
	// Initialize handlers
	userHandler := handlers.NewUserHandler(DB)

	// Setup Gin with middleware
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

	// Request logging middleware
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	}))

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		health := gin.H{
			"status":  "ok",
			"service": "user-service",
			"time":    time.Now().Unix(),
		}

		// Check database
		sqlDB, err := DB.DB()
		if err != nil {
			health["database"] = "error"
		} else if err := sqlDB.Ping(); err != nil {
			health["database"] = "error"
		} else {
			health["database"] = "ok"
		}

		// Redis removed - using database-only OTP storage
		health["redis"] = "not_used"

		// Check RabbitMQ
		if EventService != nil {
			if err := EventService.HealthCheck(); err != nil {
				health["rabbitmq"] = "error"
			} else {
				health["rabbitmq"] = "ok"
			}
		} else {
			health["rabbitmq"] = "not_configured"
		}

		c.JSON(200, health)
	})

	// API routes
	api := r.Group("/api/v1")
	{
		// Public routes (no authentication required)
		public := api.Group("/auth")
		{
			public.POST("/register", userHandler.Register)
			public.POST("/login", userHandler.Login)
			public.POST("/verify-otp", userHandler.VerifyOTP)
			public.POST("/resend-otp", userHandler.ResendOTP)
			public.POST("/refresh-token", userHandler.RefreshToken)
			public.POST("/google-oauth", userHandler.GoogleOAuth)
			public.POST("/request-reset-password", userHandler.RequestResetPassword)
			public.POST("/verify-reset-password", userHandler.VerifyResetPassword)
		}

		// Protected routes (authentication required)
		protected := api.Group("/user")
		protected.Use(userHandler.JWTService.AuthMiddleware())
		{
			protected.GET("/profile", userHandler.GetProfile)
			protected.PUT("/profile", userHandler.UpdateProfile)
		}
	}

	return r
}

func main() {
	// Initialize all services
	log.Println("🚀 Starting User Service...")

	// Initialize database
	initDB()

	// Initialize RabbitMQ
	initRabbitMQ()

	// Initialize Email Consumer
	initEmailConsumer()

	// Setup routes
	r := setupRoutes()

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("🚀 User Service running on http://localhost:%s", port)
	log.Println("📚 API Documentation:")
	log.Println("  POST /api/v1/auth/register     - Register new user")
	log.Println("  POST /api/v1/auth/login        - Login user")
	log.Println("  POST /api/v1/auth/verify-otp   - Verify OTP")
	log.Println("  POST /api/v1/auth/resend-otp   - Resend OTP")
	log.Println("  POST /api/v1/auth/refresh-token - Refresh JWT token")
	log.Println("  POST /api/v1/auth/google-oauth - Google OAuth login")
	log.Println("  POST /api/v1/auth/request-reset-password - Request password reset")
	log.Println("  POST /api/v1/auth/verify-reset-password - Verify reset password")
	log.Println("  GET  /api/v1/user/profile      - Get user profile (protected)")
	log.Println("  PUT  /api/v1/user/profile      - Update user profile (protected)")
	log.Println("  GET  /health                   - Health check")

	// Start server
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}
