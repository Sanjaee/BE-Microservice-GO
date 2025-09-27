package handlers

import (
	"log"
	"net/http"
	"time"

	"user-service/internal/events"
	"user-service/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

// UserHandler handles user-related HTTP requests
type UserHandler struct {
	db             *gorm.DB
	passwordService *models.PasswordService
	otpService     *models.OTPService
	JWTService     *JWTService
	validator      *validator.Validate
	eventService   *events.EventService
}

// NewUserHandler creates a new user handler
func NewUserHandler(db *gorm.DB) *UserHandler {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env file not found in user handlers package, using system env")
	}

	// Initialize event service
	eventService, err := events.NewEventService()
	if err != nil {
		log.Printf("⚠️ Failed to initialize event service: %v", err)
		// Continue without event service for now
	}

	return &UserHandler{
		db:              db,
		passwordService: models.NewPasswordService(),
		otpService:      models.NewOTPService(),
		JWTService:      NewJWTService(),
		validator:       validator.New(),
		eventService:    eventService,
	}
}

// Register handles user registration
func (uh *UserHandler) Register(c *gin.Context) {
	var req models.UserRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate request
	if err := uh.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists
	var existingUser models.User
	if err := uh.db.Where("email = ? OR username = ?", req.Email, req.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User with this email or username already exists"})
		return
	}

	// Hash password
	hashedPassword, err := uh.passwordService.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Generate OTP
	otp, err := uh.otpService.GenerateOTP()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate OTP"})
		return
	}

	// Create user
	user := models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hashedPassword,
		OTPCode:      &otp,
		Type:         "credential",
		IsVerified:   false,
	}

	// Save user to database
	if err := uh.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Publish user registered event to message broker
	if uh.eventService != nil {
		if err := uh.eventService.PublishUserRegistered(user.ID.String(), user.Username, user.Email); err != nil {
			log.Printf("⚠️ Failed to publish user registered event: %v", err)
			// Don't fail the registration if event publishing fails
		} else {
			log.Printf("✅ User registered event published for: %s", user.Email)
		}
	} else {
		log.Printf("⚠️ Event service not available, skipping event publishing")
	}

	// Return success response (OTP will be sent via email through message broker)
	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully. Please check your email for verification code.",
		"user":    user.ToResponse(),
	})
}

// Login handles user login
func (uh *UserHandler) Login(c *gin.Context) {
	var req models.UserLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate request
	if err := uh.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user by email
	var user models.User
	if err := uh.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User not found",
				"message": "Email tidak terdaftar. Silakan periksa kembali email Anda atau daftar akun baru.",
				"code": "USER_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Check if user type is credential (not Google OAuth user)
	if user.Type != "credential" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Account type mismatch",
			"message": "Akun ini dibuat dengan Google. Silakan gunakan tombol 'Masuk dengan Google' untuk login.",
			"code": "ACCOUNT_TYPE_MISMATCH",
		})
		return
	}

	// Verify password
	if err := uh.passwordService.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid password",
			"message": "Password yang Anda masukkan salah. Silakan coba lagi.",
			"code": "INVALID_PASSWORD",
		})
		return
	}

	// Generate tokens
	authResponse, err := uh.JWTService.GenerateTokens(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, authResponse)
}

// VerifyOTP handles OTP verification
func (uh *UserHandler) VerifyOTP(c *gin.Context) {
	var req models.OTPVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate request
	if err := uh.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate OTP format
	if !uh.otpService.ValidateOTP(req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OTP format"})
		return
	}

	// Find user by email
	var user models.User
	if err := uh.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Check if user is already verified
	if user.IsVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User is already verified"})
		return
	}

	// Verify OTP
	if user.OTPCode == nil || *user.OTPCode != req.OTPCode {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OTP"})
		return
	}

	// Update user as verified and clear OTP
	user.IsVerified = true
	user.OTPCode = nil
	user.UpdatedAt = time.Now()

	if err := uh.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify user"})
		return
	}

	// Generate tokens after successful verification
	authResponse, err := uh.JWTService.GenerateTokens(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	// Publish user verified event to message broker
	if uh.eventService != nil {
		if err := uh.eventService.PublishUserVerified(user.ID.String(), user.Username, user.Email); err != nil {
			log.Printf("⚠️ Failed to publish user verified event: %v", err)
			// Don't fail the verification if event publishing fails
		} else {
			log.Printf("✅ User verified event published for: %s", user.Email)
		}
	}

	c.JSON(http.StatusOK, authResponse)
}

// ResendOTP handles OTP resending
func (uh *UserHandler) ResendOTP(c *gin.Context) {
	var req struct {
		Email string `json:"email" validate:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate request
	if err := uh.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user by email
	var user models.User
	if err := uh.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Check if user is already verified
	if user.IsVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User is already verified"})
		return
	}

	// Generate new OTP
	otp, err := uh.otpService.GenerateOTP()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate OTP"})
		return
	}

	// Update user with new OTP
	user.OTPCode = &otp
	user.UpdatedAt = time.Now()

	if err := uh.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update OTP"})
		return
	}

	// Publish user registered event again to resend OTP
	if uh.eventService != nil {
		if err := uh.eventService.PublishUserRegistered(user.ID.String(), user.Username, user.Email); err != nil {
			log.Printf("⚠️ Failed to publish resend OTP event: %v", err)
			// Don't fail the resend if event publishing fails
		} else {
			log.Printf("✅ Resend OTP event published for: %s", user.Email)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "OTP sent successfully. Please check your email.",
	})
}

// GetProfile handles getting user profile
func (uh *UserHandler) GetProfile(c *gin.Context) {
	userID, _, _, _, ok := GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var user models.User
	if err := uh.db.Where("id = ?", userID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user.ToResponse()})
}

// UpdateProfile handles updating user profile
func (uh *UserHandler) UpdateProfile(c *gin.Context) {
	userID, _, _, _, ok := GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req struct {
		Username string `json:"username" validate:"omitempty,min=3,max=100"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate request
	if err := uh.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := uh.db.Where("id = ?", userID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Check if username is already taken by another user
	if req.Username != "" && req.Username != user.Username {
		var existingUser models.User
		if err := uh.db.Where("username = ? AND id != ?", req.Username, userID).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already taken"})
			return
		}
		user.Username = req.Username
	}

	user.UpdatedAt = time.Now()

	if err := uh.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profile updated successfully",
		"user":    user.ToResponse(),
	})
}

// RefreshToken handles token refresh
func (uh *UserHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate refresh token
	claims, err := uh.JWTService.ValidateToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	// Find user
	var user models.User
	if err := uh.db.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Generate new tokens
	authResponse, err := uh.JWTService.GenerateTokens(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, authResponse)
}

// RequestResetPassword handles password reset request
func (uh *UserHandler) RequestResetPassword(c *gin.Context) {
	var req models.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate request
	if err := uh.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user by email
	var user models.User
	if err := uh.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Don't reveal if email exists or not for security
			c.JSON(http.StatusOK, gin.H{
				"message": "If the email exists, a reset code has been sent.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Check if user is verified
	if !user.IsVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Account not verified. Please verify your email first."})
		return
	}

	// Generate OTP for password reset
	otp, err := uh.otpService.GenerateOTP()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate reset code"})
		return
	}

	// Update user with reset OTP
	user.OTPCode = &otp
	user.UpdatedAt = time.Now()

	if err := uh.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate reset code"})
		return
	}

	// Publish password reset event to message broker
	if uh.eventService != nil {
		if err := uh.eventService.PublishPasswordReset(user.ID.String(), user.Username, user.Email); err != nil {
			log.Printf("⚠️ Failed to publish password reset event: %v", err)
			// Don't fail the request if event publishing fails
		} else {
			log.Printf("✅ Password reset event published for: %s", user.Email)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "If the email exists, a reset code has been sent.",
	})
}

// VerifyResetPassword handles password reset verification
func (uh *UserHandler) VerifyResetPassword(c *gin.Context) {
	var req models.VerifyResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate request
	if err := uh.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate OTP format
	if !uh.otpService.ValidateOTP(req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reset code format"})
		return
	}

	// Find user by email
	var user models.User
	if err := uh.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Check if user is verified
	if !user.IsVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Account not verified. Please verify your email first."})
		return
	}

	// Verify OTP
	if user.OTPCode == nil || *user.OTPCode != req.OTPCode {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reset code"})
		return
	}

	// Hash new password
	hashedPassword, err := uh.passwordService.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process new password"})
		return
	}

	// Update user password and clear OTP
	user.PasswordHash = hashedPassword
	user.OTPCode = nil
	user.UpdatedAt = time.Now()

	if err := uh.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	// Generate new tokens after successful password reset
	authResponse, err := uh.JWTService.GenerateTokens(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	// Publish password reset success event
	if uh.eventService != nil {
		if err := uh.eventService.PublishPasswordResetSuccess(user.ID.String(), user.Username, user.Email); err != nil {
			log.Printf("⚠️ Failed to publish password reset success event: %v", err)
			// Don't fail the request if event publishing fails
		} else {
			log.Printf("✅ Password reset success event published for: %s", user.Email)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Password reset successfully",
		"user":    user.ToResponse(),
		"access_token": authResponse.AccessToken,
		"refresh_token": authResponse.RefreshToken,
		"expires_in": authResponse.ExpiresIn,
	})
}

// GoogleOAuth handles Google OAuth user creation/update
func (uh *UserHandler) GoogleOAuth(c *gin.Context) {
	var req struct {
		Email     string `json:"email" validate:"required,email"`
		Username  string `json:"username" validate:"required,min=3,max=100"`
		ImageUrl  string `json:"image_url"`
		GoogleID  string `json:"google_id" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate request
	if err := uh.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists by email
	var user models.User
	err := uh.db.Where("email = ?", req.Email).First(&user).Error
	
	if err == gorm.ErrRecordNotFound {
		// Create new user
		user = models.User{
			Username:   req.Username,
			Email:      req.Email,
			ImageUrl:   &req.ImageUrl,
			Type:       "google",
			IsVerified: true, // Google users are automatically verified
		}
		
		if err := uh.db.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	} else {
		// Check if existing user is credential type
		if user.Type == "credential" {
			c.JSON(http.StatusConflict, gin.H{"error": "This email is already registered with credentials. Please use email/password login instead."})
			return
		}
		
		// Update existing Google user with new info
		user.ImageUrl = &req.ImageUrl
		user.IsVerified = true // Ensure Google users are verified
		user.UpdatedAt = time.Now()
		
		if err := uh.db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
			return
		}
	}

	// Generate tokens
	authResponse, err := uh.JWTService.GenerateTokens(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, authResponse)
}
