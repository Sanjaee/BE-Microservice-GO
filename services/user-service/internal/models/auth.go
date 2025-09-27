package models

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// PasswordService handles password hashing and verification
type PasswordService struct{}

// NewPasswordService creates a new password service
func NewPasswordService() *PasswordService {
	return &PasswordService{}
}

// HashPassword hashes a password using bcrypt
func (ps *PasswordService) HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hashedBytes), nil
}

// VerifyPassword verifies a password against its hash
func (ps *PasswordService) VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// OTPService handles OTP generation and verification
type OTPService struct{}

// NewOTPService creates a new OTP service
func NewOTPService() *OTPService {
	return &OTPService{}
}

// GenerateOTP generates a random 6-digit OTP
func (os *OTPService) GenerateOTP() (string, error) {
	// Generate random number between 100000 and 999999 (6 digits)
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", err
	}
	otp := fmt.Sprintf("%06d", 100000+n.Int64())
	return otp, nil
}

// ValidateOTP validates if the provided OTP is 6 digits
func (os *OTPService) ValidateOTP(otp string) bool {
	if len(otp) != 6 {
		return false
	}
	
	for _, char := range otp {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// JWTClaims represents the JWT claims structure
type JWTClaims struct {
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	IsVerified bool   `json:"is_verified"`
	ExpiresAt  int64  `json:"exp"`
	IssuedAt   int64  `json:"iat"`
}

// Valid implements jwt.Claims interface
func (c JWTClaims) Valid() error {
	return nil
}

// GetAudience implements jwt.Claims interface
func (c JWTClaims) GetAudience() (jwt.ClaimStrings, error) {
	return nil, nil
}

// GetExpirationTime implements jwt.Claims interface
func (c JWTClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.ExpiresAt, 0)), nil
}

// GetIssuedAt implements jwt.Claims interface
func (c JWTClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.IssuedAt, 0)), nil
}

// GetIssuer implements jwt.Claims interface
func (c JWTClaims) GetIssuer() (string, error) {
	return "", nil
}

// GetNotBefore implements jwt.Claims interface
func (c JWTClaims) GetNotBefore() (*jwt.NumericDate, error) {
	return nil, nil
}

// GetSubject implements jwt.Claims interface
func (c JWTClaims) GetSubject() (string, error) {
	return c.UserID, nil
}

// TokenConfig holds JWT configuration
type TokenConfig struct {
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
	SecretKey          string
}

// DefaultTokenConfig returns default JWT configuration
func DefaultTokenConfig() *TokenConfig {
	return &TokenConfig{
		AccessTokenExpiry:  15 * time.Minute,  // 15 minutes
		RefreshTokenExpiry: 7 * 24 * time.Hour, // 7 days
		SecretKey:          "your-secret-key", // Should be from env
	}
}
