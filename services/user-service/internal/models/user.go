package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents the user model in the database
type User struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Username     string    `json:"username" gorm:"uniqueIndex;not null;size:100" validate:"required,min=3,max=100"`
	Email        string    `json:"email" gorm:"uniqueIndex;not null;size:150" validate:"required,email"`
	PasswordHash string    `json:"-" gorm:"not null"` // Hidden from JSON
	OTPCode      *string   `json:"-" gorm:"size:6"`   // Hidden from JSON
	ImageUrl     *string   `json:"image_url" gorm:"size:500"` // Profile image URL from OAuth providers
	Type         string    `json:"type" gorm:"not null;default:'credential'" validate:"required,oneof=credential google"` // Login type: credential or google
	IsVerified   bool      `json:"is_verified" gorm:"default:false"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserRegisterRequest represents the request payload for user registration
type UserRegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=100"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

// UserLoginRequest represents the request payload for user login
type UserLoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// OTPVerifyRequest represents the request payload for OTP verification
type OTPVerifyRequest struct {
	Email   string `json:"email" validate:"required,email"`
	OTPCode string `json:"otp_code" validate:"required,len=6"`
}

// ResetPasswordRequest represents the request payload for password reset
type ResetPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// VerifyResetPasswordRequest represents the request payload for reset password verification
type VerifyResetPasswordRequest struct {
	Email       string `json:"email" validate:"required,email"`
	OTPCode     string `json:"otp_code" validate:"required,len=6"`
	NewPassword string `json:"new_password" validate:"required,min=6"`
}

// UserResponse represents the response payload for user data
type UserResponse struct {
	ID         uuid.UUID `json:"id"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	ImageUrl   *string   `json:"image_url"`
	Type       string    `json:"type"`
	IsVerified bool      `json:"is_verified"`
	CreatedAt  time.Time `json:"created_at"`
}

// AuthResponse represents the response payload for authentication
type AuthResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int64        `json:"expires_in"`
}

// BeforeCreate hook to set UUID if not provided
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// ToResponse converts User to UserResponse
func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:         u.ID,
		Username:   u.Username,
		Email:      u.Email,
		ImageUrl:   u.ImageUrl,
		Type:       u.Type,
		IsVerified: u.IsVerified,
		CreatedAt:  u.CreatedAt,
	}
}
