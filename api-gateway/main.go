package main

import (
	"bytes"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	UserServiceURL    = "http://localhost:8081"
	ProductServiceURL = "http://localhost:8082"
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

	log.Println("🚀 API Gateway running on http://localhost:8080")
	log.Println("📚 Available endpoints:")
	log.Println("  POST /api/v1/auth/register     - Register new user")
	log.Println("  POST /api/v1/auth/login        - Login user")
	log.Println("  POST /api/v1/auth/verify-otp   - Verify OTP")
	log.Println("  POST /api/v1/auth/resend-otp   - Resend OTP")
	log.Println("  POST /api/v1/auth/refresh-token - Refresh JWT token")
	log.Println("  POST /api/v1/auth/google-oauth - Google OAuth login")
	log.Println("  GET  /api/v1/user/profile      - Get user profile (protected)")
	log.Println("  PUT  /api/v1/user/profile      - Update user profile (protected)")
	log.Println("  GET  /api/v1/products          - Get all products")
	log.Println("  GET  /api/v1/products/:id      - Get product by ID")
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

		// Create new request to user service
		url := UserServiceURL + path
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

		// Create new request to product service
		url := ProductServiceURL + path
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
