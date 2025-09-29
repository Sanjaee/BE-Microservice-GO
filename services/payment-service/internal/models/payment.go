package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PaymentStatus represents the status of a payment
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "PENDING"
	PaymentStatusSuccess   PaymentStatus = "SUCCESS"
	PaymentStatusFailed    PaymentStatus = "FAILED"
	PaymentStatusCancelled PaymentStatus = "CANCELLED"
	PaymentStatusExpired   PaymentStatus = "EXPIRED"
)

// PaymentMethod represents the payment method
type PaymentMethod string

const (
	PaymentMethodCreditCard   PaymentMethod = "credit_card"
	PaymentMethodBankTransfer PaymentMethod = "bank_transfer"
	PaymentMethodGoPay        PaymentMethod = "gopay"
	PaymentMethodQRIS         PaymentMethod = "qris"
	PaymentMethodShopeepay    PaymentMethod = "shopeepay"
	PaymentMethodEchannel     PaymentMethod = "echannel"
	PaymentMethodPermata      PaymentMethod = "permata"
	PaymentMethodCstore       PaymentMethod = "cstore"
)

// Payment represents the payment model in the database
type Payment struct {
	ID                    uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrderID               string         `json:"order_id" gorm:"uniqueIndex;not null"`
	UserID                uuid.UUID      `json:"user_id" gorm:"type:uuid;not null"`
	ProductID             *uuid.UUID     `json:"product_id" gorm:"type:uuid"`
	Amount                int64          `json:"amount" gorm:"not null"` // Amount in rupiah
	AdminFee              int64          `json:"admin_fee" gorm:"default:0"` // Admin fee in rupiah
	TotalAmount           int64          `json:"total_amount" gorm:"not null"` // Total amount in rupiah
	PaymentMethod         PaymentMethod  `json:"payment_method" gorm:"not null"`
	PaymentType           string         `json:"payment_type"` // qris, bank_transfer, credit_card, etc
	Status                PaymentStatus  `json:"status" gorm:"default:'PENDING'"`
	Notes                 *string        `json:"notes"` // User notes/comments for the order
	SnapRedirectURL       *string        `json:"snap_redirect_url"`
	MidtransTransactionID *string        `json:"midtrans_transaction_id"`
	TransactionStatus     *string        `json:"transaction_status"`
	FraudStatus           *string        `json:"fraud_status"`
	PaymentCode           *string        `json:"payment_code"` // untuk bank transfer
	VANumber              *string        `json:"va_number"`    // untuk virtual account
	BankType              *string        `json:"bank_type"`    // mandiri, bca, bni, etc
	StoreType             *string        `json:"store_type"`   // alfamart, indomaret, etc
	ExpiryTime            *time.Time     `json:"expiry_time"`
	PaidAt                *time.Time     `json:"paid_at"`
	MidtransResponse      *string        `json:"midtrans_response"` // JSON response from Midtrans
	MidtransAction        *string        `json:"midtrans_action"`   // JSON.stringify(result.actions)
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`

	// Relations (no foreign key constraints - just references)
	User    *User     `json:"user,omitempty" gorm:"-"`
	Product *Product  `json:"product,omitempty" gorm:"-"`
}

// User represents a simplified user model for foreign key relationship
type User struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
}

// Product represents a simplified product model for foreign key relationship
type Product struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Stock       int       `json:"stock"`
	IsActive    bool      `json:"is_active"`
}

// CreatePaymentRequest represents the request payload for creating a payment
type CreatePaymentRequest struct {
	ProductID     *uuid.UUID    `json:"product_id" validate:"required"`
	Amount        int64         `json:"amount" validate:"required,min=1"`
	AdminFee      int64         `json:"admin_fee" validate:"min=0"`
	PaymentMethod PaymentMethod `json:"payment_method" validate:"required,oneof=credit_card bank_transfer gopay qris shopeepay echannel permata cstore"`
	BankType      *string       `json:"bank_type,omitempty"` // For bank transfer
	StoreType     *string       `json:"store_type,omitempty"` // For cstore (alfamart, indomaret)
	Notes         *string       `json:"notes,omitempty"`
}

// PaymentResponse represents the response payload for payment data
type PaymentResponse struct {
	ID                    uuid.UUID      `json:"id"`
	OrderID               string         `json:"order_id"`
	UserID                uuid.UUID      `json:"user_id"`
	ProductID             *uuid.UUID     `json:"product_id"`
	Amount                int64          `json:"amount"`
	AdminFee              int64          `json:"admin_fee"`
	TotalAmount           int64          `json:"total_amount"`
	PaymentMethod         PaymentMethod  `json:"payment_method"`
	PaymentType           string         `json:"payment_type"`
	Status                PaymentStatus  `json:"status"`
	Notes                 *string        `json:"notes"`
	SnapRedirectURL       *string        `json:"snap_redirect_url"`
	MidtransTransactionID *string        `json:"midtrans_transaction_id"`
	TransactionStatus     *string        `json:"transaction_status"`
	FraudStatus           *string        `json:"fraud_status"`
	PaymentCode           *string        `json:"payment_code"`
	VANumber              *string        `json:"va_number"`
	BankType              *string        `json:"bank_type"`
	StoreType             *string        `json:"store_type"`
	ExpiryTime            *time.Time     `json:"expiry_time"`
	PaidAt                *time.Time     `json:"paid_at"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	User                  *User          `json:"user,omitempty"`
	Product               *Product       `json:"product,omitempty"`
	Actions               []MidtransAction `json:"actions,omitempty"`
}

// MidtransAction represents Midtrans payment actions
type MidtransAction struct {
	Name string `json:"name"`
	Method string `json:"method"`
	URL   string `json:"url"`
}

// PaymentListResponse represents the response payload for paginated payment list
type PaymentListResponse struct {
	Payments []PaymentResponse `json:"payments"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	Limit    int               `json:"limit"`
	HasMore  bool              `json:"has_more"`
}

// PaymentQuery represents query parameters for payment listing
type PaymentQuery struct {
	Page     int            `form:"page"`
	Limit    int            `form:"limit"`
	UserID   *uuid.UUID     `form:"user_id"`
	Status   *PaymentStatus `form:"status"`
	OrderID  *string        `form:"order_id"`
}

// MidtransCallbackRequest represents the callback request from Midtrans
type MidtransCallbackRequest struct {
	OrderID       string `json:"order_id" binding:"required"`
	StatusCode    string `json:"status_code" binding:"required"`
	GrossAmount   string `json:"gross_amount" binding:"required"`
	SignatureKey  string `json:"signature_key" binding:"required"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus   string `json:"fraud_status"`
	PaymentType   string `json:"payment_type"`
	TransactionID string `json:"transaction_id"`
	PaidAt        string `json:"paid_at"`
	ExpiryTime    string `json:"expiry_time"`
}

// BeforeCreate hook to set UUID if not provided
func (p *Payment) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// ToResponse converts Payment to PaymentResponse
func (p *Payment) ToResponse() PaymentResponse {
	response := PaymentResponse{
		ID:                    p.ID,
		OrderID:               p.OrderID,
		UserID:                p.UserID,
		ProductID:             p.ProductID,
		Amount:                p.Amount,
		AdminFee:              p.AdminFee,
		TotalAmount:           p.TotalAmount,
		PaymentMethod:         p.PaymentMethod,
		PaymentType:           p.PaymentType,
		Status:                p.Status,
		Notes:                 p.Notes,
		SnapRedirectURL:       p.SnapRedirectURL,
		MidtransTransactionID: p.MidtransTransactionID,
		TransactionStatus:     p.TransactionStatus,
		FraudStatus:           p.FraudStatus,
		PaymentCode:           p.PaymentCode,
		VANumber:              p.VANumber,
		BankType:              p.BankType,
		StoreType:             p.StoreType,
		ExpiryTime:            p.ExpiryTime,
		PaidAt:                p.PaidAt,
		CreatedAt:             p.CreatedAt,
		UpdatedAt:             p.UpdatedAt,
		User:                  p.User,
		Product:               p.Product,
	}

	// Parse Midtrans actions if available
	if p.MidtransAction != nil {
		// This will be handled in the handler layer
		// where we can properly unmarshal the JSON
	}

	return response
}

// IsSuccessful checks if payment is successful
func (p *Payment) IsSuccessful() bool {
	return p.Status == PaymentStatusSuccess
}

// IsPending checks if payment is pending
func (p *Payment) IsPending() bool {
	return p.Status == PaymentStatusPending
}

// IsFailed checks if payment is failed
func (p *Payment) IsFailed() bool {
	return p.Status == PaymentStatusFailed || p.Status == PaymentStatusCancelled || p.Status == PaymentStatusExpired
}
