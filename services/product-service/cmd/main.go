package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"product-service/internal/cache"
	"product-service/internal/handlers"
	"product-service/internal/models"
	"product-service/internal/repository"

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

	// Connect to database using GORM
	log.Printf("🔗 Connecting to database: %s@%s:%s/%s", dbUser, dbHost, dbPort, dbName)
	
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

	log.Println("✅ Database connection established successfully!")

	// Auto migrate the models
	log.Println("🔄 Running database migrations...")
	if err := DB.AutoMigrate(&models.Product{}, &models.ProductImage{}, &models.User{}); err != nil {
		log.Fatalf("❌ Failed to migrate database: %v", err)
	}

	log.Println("✅ Database migrations completed successfully!")
}

func main() {
	// Initialize database
	initDB()

	// Get Redis configuration from environment
	redisHost := getEnv("REDIS_HOST", "localhost:6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	redisDB := getEnvAsInt("REDIS_DB", 0)
	
	// Get worker pool configuration
	workerCount := getEnvAsInt("WORKER_COUNT", 100)
	port := getEnv("PORT", "8082")

	// Connect to Redis
	log.Printf("🔗 Connecting to Redis: %s (DB: %d)", redisHost, redisDB)
	redisClient := cache.NewRedisClient(redisHost, redisPassword, redisDB)
	defer redisClient.Close()
	log.Println("✅ Redis connection established successfully!")

	// Create repository
	log.Println("🏗️ Initializing product repository...")
	productRepo := repository.NewProductRepository(DB, redisClient)
	log.Println("✅ Product repository initialized successfully!")

	// Create worker pool
	log.Printf("👥 Creating worker pool with %d workers...", workerCount)
	workerPool := handlers.NewWorkerPool(workerCount)
	workerPool.Start()
	defer workerPool.Stop()
	log.Println("✅ Worker pool started successfully!")

	// Create handlers
	log.Println("🎯 Initializing product handlers...")
	productHandler := handlers.NewProductHandler(productRepo, workerPool)
	productHandler.UpdateWorkerPoolHandlers()
	log.Println("✅ Product handlers initialized successfully!")

	// Setup Gin router
	log.Println("🌐 Setting up HTTP server...")
	r := gin.Default()

	// CORS middleware
	log.Println("🔧 Configuring CORS middleware...")
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
	log.Println("📝 Configuring request logging middleware...")
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
			"status":    "ok",
			"service":   "product-service",
			"timestamp": time.Now().Unix(),
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

		// Check Redis
		if redisClient != nil {
			health["redis"] = "ok"
		} else {
			health["redis"] = "not_configured"
		}

		// Check worker pool
		health["worker_pool"] = gin.H{
			"active_jobs": workerPool.GetActiveJobs(),
			"worker_count": workerCount,
		}

		c.JSON(200, health)
	})

	// API routes
	api := r.Group("/api/v1")
	{
		// Product routes
		products := api.Group("/products")
		{
			products.GET("", productHandler.GetProducts)
			products.GET("/:id", productHandler.GetProductByID)
		}
	}

	log.Printf("🚀 Product Service running on http://localhost:%s", port)
	log.Println("📚 API Documentation:")
	log.Println("  GET /api/v1/products        - Get all products (with pagination)")
	log.Println("  GET /api/v1/products/:id    - Get product by ID")
	log.Println("  GET /health                 - Health check")
	log.Printf("🔧 Worker pool: %d workers", workerCount)

	// Start server
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
