package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"api-gateway/middleware"

	"github.com/gin-gonic/gin"
)

const (
	UserServiceURL     = "http://localhost:8081"
	ProductServiceURL  = "http://localhost:8082"
	PaymentServiceURL  = "http://localhost:8083"
)

func main() {
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
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "api-gateway",
		})
	})

	// User Service Routes
	userRoutes := r.Group("/api/v1")
	{
		// Health check for user service
		userRoutes.GET("/user/health", proxyToUserService("GET", "/health"))

		// Authentication routes
		authRoutes := userRoutes.Group("/auth")
		{
			authRoutes.POST("/register", proxyToUserService("POST", "/api/v1/auth/register"))
			authRoutes.POST("/login", proxyToUserService("POST", "/api/v1/auth/login"))
			authRoutes.POST("/verify-otp", proxyToUserService("POST", "/api/v1/auth/verify-otp"))
			authRoutes.POST("/resend-otp", proxyToUserService("POST", "/api/v1/auth/resend-otp"))
			authRoutes.POST("/refresh-token", proxyToUserService("POST", "/api/v1/auth/refresh-token"))
			authRoutes.POST("/google-oauth", proxyToUserService("POST", "/api/v1/auth/google-oauth"))
			authRoutes.POST("/request-reset-password", proxyToUserService("POST", "/api/v1/auth/request-reset-password"))
			authRoutes.POST("/verify-reset-password", proxyToUserService("POST", "/api/v1/auth/verify-reset-password"))
		}

		// Protected user routes
		userProtectedRoutes := userRoutes.Group("/user")
		{
			userProtectedRoutes.GET("/profile", proxyToUserService("GET", "/api/v1/user/profile"))
			userProtectedRoutes.PUT("/profile", proxyToUserService("PUT", "/api/v1/user/profile"))
		}
	}

	// Product Service Routes
	productRoutes := r.Group("/api/v1")
	{
		// Health check for product service
		productRoutes.GET("/product/health", proxyToProductService("GET", "/health"))

		// Product routes
		products := productRoutes.Group("/products")
		{
			products.GET("", proxyToProductService("GET", "/api/v1/products"))
			products.GET("/:id", proxyToProductService("GET", "/api/v1/products/:id"))
		}
	}

	// Payment Service Routes
	paymentRoutes := r.Group("/api/v1")
	{
		// Health check for payment service
		paymentRoutes.GET("/payment/health", proxyToPaymentService("GET", "/health"))

		// Payment routes
		payments := paymentRoutes.Group("/payments")
		{
			// Public routes
			payments.GET("/config", proxyToPaymentService("GET", "/api/v1/payments/config"))
			payments.POST("/midtrans/callback", proxyToPaymentService("POST", "/api/v1/payments/midtrans/callback"))

			// Protected routes (require authentication)
			jwtSecret := os.Getenv("JWT_SECRET")
			if jwtSecret == "" {
				jwtSecret = "your-super-secret-jwt-key-change-this-in-production" // Default for development
			}
			
			protected := payments.Group("")
			protected.Use(middleware.AuthMiddleware(jwtSecret))
			{
				protected.POST("", proxyToPaymentService("POST", "/api/v1/payments"))
				protected.GET("/:id", proxyToPaymentService("GET", "/api/v1/payments/:id"))
				protected.GET("/order/:order_id", proxyToPaymentService("GET", "/api/v1/payments/order/:order_id"))
				protected.GET("/user", proxyToPaymentService("GET", "/api/v1/payments/user"))
			}
		}
	}

	log.Println("🚀 API Gateway running on http://localhost:8080")
	log.Println("📚 Available endpoints:")
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
	log.Println("  GET  /api/v1/products          - Get all products")
	log.Println("  GET  /api/v1/products/:id      - Get product by ID")
	log.Println("  POST /api/v1/payments          - Create payment")
	log.Println("  GET  /api/v1/payments/:id      - Get payment by ID")
	log.Println("  GET  /api/v1/payments/order/:id - Get payment by order ID")
	log.Println("  GET  /api/v1/payments/user     - Get user payments")
	log.Println("  GET  /api/v1/payments/config   - Get Midtrans config")
	log.Println("  POST /api/v1/payments/midtrans/callback - Midtrans webhook")
	log.Println("  GET  /health                   - Health check")

	r.Run(":8080")
}

// proxyToUserService creates a proxy handler for user service
func proxyToUserService(method, path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read request body
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
		}

		// Replace URL parameters with actual values
		actualPath := path
		for _, param := range c.Params {
			actualPath = strings.Replace(actualPath, ":"+param.Key, param.Value, -1)
		}

		// Create new request to user service
		url := UserServiceURL + actualPath
		req, err := http.NewRequest(method, url, bytes.NewBuffer(bodyBytes))
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to create request"})
			return
		}

		// Copy headers
		for key, values := range c.Request.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}

		// Make request to user service
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(500, gin.H{"error": "User service unavailable"})
			return
		}
		defer resp.Body.Close()

		// Read response body
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to read response"})
			return
		}

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				c.Header(key, value)
			}
		}

		// Return response
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
	}
}

// proxyToProductService creates a proxy handler for product service
func proxyToProductService(method, path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read request body
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
		}

		// Replace URL parameters with actual values
		actualPath := path
		for _, param := range c.Params {
			actualPath = strings.Replace(actualPath, ":"+param.Key, param.Value, -1)
		}

		// Create new request to product service
		url := ProductServiceURL + actualPath
		req, err := http.NewRequest(method, url, bytes.NewBuffer(bodyBytes))
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to create request"})
			return
		}

		// Copy headers
		for key, values := range c.Request.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}

		// Make request to product service
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(500, gin.H{"error": "Product service unavailable"})
			return
		}
		defer resp.Body.Close()

		// Read response body
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to read response"})
			return
		}

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				c.Header(key, value)
			}
		}

		// Return response
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
	}
}

// proxyToPaymentService creates a proxy handler for payment service
func proxyToPaymentService(method, path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read request body
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
		}

		// Replace URL parameters with actual values
		actualPath := path
		for _, param := range c.Params {
			actualPath = strings.Replace(actualPath, ":"+param.Key, param.Value, -1)
		}

		// Create new request to payment service
		url := PaymentServiceURL + actualPath
		req, err := http.NewRequest(method, url, bytes.NewBuffer(bodyBytes))
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to create request"})
			return
		}

		// Copy headers
		for key, values := range c.Request.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}

		// Add user context headers for payment service
		if userID, exists := c.Get("user_id"); exists {
			req.Header.Set("X-User-ID", userID.(string))
		}
		if username, exists := c.Get("username"); exists {
			req.Header.Set("X-Username", username.(string))
		}
		if email, exists := c.Get("email"); exists {
			req.Header.Set("X-Email", email.(string))
		}

		// Make request to payment service
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(500, gin.H{"error": "Payment service unavailable"})
			return
		}
		defer resp.Body.Close()

		// Read response body
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to read response"})
			return
		}

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				c.Header(key, value)
			}
		}

		// Return response
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
	}
}
