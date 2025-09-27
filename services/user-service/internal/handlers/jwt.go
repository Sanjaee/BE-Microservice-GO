package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"user-service/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

// JWTService handles JWT token operations
type JWTService struct {
	secretKey          string
	accessTokenExpiry  time.Duration
	refreshTokenExpiry time.Duration
}

// NewJWTService creates a new JWT service
func NewJWTService() *JWTService {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env file not found in handlers package, using system env")
	}

	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		secretKey = "your-secret-key" // Default for development
	}

	accessExpiry := 15 * time.Minute
	if exp := os.Getenv("JWT_ACCESS_EXPIRY"); exp != "" {
		if parsed, err := time.ParseDuration(exp); err == nil {
			accessExpiry = parsed
		}
	}

	refreshExpiry := 7 * 24 * time.Hour
	if exp := os.Getenv("JWT_REFRESH_EXPIRY"); exp != "" {
		if parsed, err := time.ParseDuration(exp); err == nil {
			refreshExpiry = parsed
		}
	}

	return &JWTService{
		secretKey:          secretKey,
		accessTokenExpiry:  accessExpiry,
		refreshTokenExpiry: refreshExpiry,
	}
}

// GenerateTokens generates both access and refresh tokens
func (js *JWTService) GenerateTokens(user *models.User) (*models.AuthResponse, error) {
	now := time.Now()
	
	// Access token claims
	accessClaims := &models.JWTClaims{
		UserID:     user.ID.String(),
		Username:   user.Username,
		Email:      user.Email,
		IsVerified: user.IsVerified,
		ExpiresAt:  now.Add(js.accessTokenExpiry).Unix(),
		IssuedAt:   now.Unix(),
	}

	// Refresh token claims
	refreshClaims := &models.JWTClaims{
		UserID:     user.ID.String(),
		Username:   user.Username,
		Email:      user.Email,
		IsVerified: user.IsVerified,
		ExpiresAt:  now.Add(js.refreshTokenExpiry).Unix(),
		IssuedAt:   now.Unix(),
	}

	// Create access token
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(js.secretKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create access token: %w", err)
	}

	// Create refresh token
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(js.secretKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh token: %w", err)
	}

	return &models.AuthResponse{
		User:         user.ToResponse(),
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresIn:    int64(js.accessTokenExpiry.Seconds()),
	}, nil
}

// ValidateToken validates a JWT token and returns the claims
func (js *JWTService) ValidateToken(tokenString string) (*models.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &models.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(js.secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*models.JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// AuthMiddleware validates JWT token and sets user context
func (js *JWTService) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		tokenString := authHeader
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		}

		claims, err := js.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("is_verified", claims.IsVerified)
		c.Next()
	}
}

// OptionalAuthMiddleware validates JWT token if present but doesn't require it
func (js *JWTService) OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		tokenString := authHeader
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		}

		claims, err := js.ValidateToken(tokenString)
		if err == nil {
			c.Set("user_id", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("email", claims.Email)
			c.Set("is_verified", claims.IsVerified)
		}

		c.Next()
	}
}

// GetUserFromContext extracts user information from gin context
func GetUserFromContext(c *gin.Context) (userID string, username string, email string, isVerified bool, ok bool) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		return "", "", "", false, false
	}
	userID, ok = userIDVal.(string)
	if !ok {
		return "", "", "", false, false
	}

	usernameVal, exists := c.Get("username")
	if !exists {
		return userID, "", "", false, false
	}
	username, ok = usernameVal.(string)
	if !ok {
		return userID, "", "", false, false
	}

	emailVal, exists := c.Get("email")
	if !exists {
		return userID, username, "", false, false
	}
	email, ok = emailVal.(string)
	if !ok {
		return userID, username, "", false, false
	}

	isVerifiedVal, exists := c.Get("is_verified")
	if !exists {
		return userID, username, email, false, false
	}
	isVerified, ok = isVerifiedVal.(bool)
	if !ok {
		return userID, username, email, false, false
	}

	return userID, username, email, isVerified, true
}
