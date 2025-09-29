package repository

import (
	"fmt"
	"time"

	"payment-service/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PaymentRepository handles payment database operations
type PaymentRepository struct {
	db *gorm.DB
}

// NewPaymentRepository creates a new payment repository
func NewPaymentRepository(db *gorm.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

// Create creates a new payment
func (pr *PaymentRepository) Create(payment *models.Payment) error {
	if err := pr.db.Create(payment).Error; err != nil {
		return fmt.Errorf("failed to create payment: %w", err)
	}
	return nil
}

// GetByID retrieves a payment by ID
func (pr *PaymentRepository) GetByID(id uuid.UUID) (*models.Payment, error) {
	var payment models.Payment
	if err := pr.db.Preload("User").Preload("Product").First(&payment, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("payment not found")
		}
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}
	return &payment, nil
}

// GetByIDWithoutRelations retrieves a payment by ID without loading relations
func (pr *PaymentRepository) GetByIDWithoutRelations(id uuid.UUID) (*models.Payment, error) {
	var payment models.Payment
	if err := pr.db.First(&payment, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("payment not found")
		}
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}
	return &payment, nil
}

// GetByOrderID retrieves a payment by order ID
func (pr *PaymentRepository) GetByOrderID(orderID string) (*models.Payment, error) {
	var payment models.Payment
	if err := pr.db.Preload("User").Preload("Product").First(&payment, "order_id = ?", orderID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("payment not found")
		}
		return nil, fmt.Errorf("failed to get payment by order ID: %w", err)
	}
	return &payment, nil
}

// GetByUserID retrieves payments by user ID with pagination
func (pr *PaymentRepository) GetByUserID(userID uuid.UUID, page, limit int) ([]models.Payment, int64, error) {
	var payments []models.Payment
	var total int64

	// Count total records
	if err := pr.db.Model(&models.Payment{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count payments: %w", err)
	}

	// Calculate offset
	offset := (page - 1) * limit

	// Get payments with pagination
	if err := pr.db.Preload("User").Preload("Product").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&payments).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get payments: %w", err)
	}

	return payments, total, nil
}

// GetByStatus retrieves payments by status with pagination
func (pr *PaymentRepository) GetByStatus(status models.PaymentStatus, page, limit int) ([]models.Payment, int64, error) {
	var payments []models.Payment
	var total int64

	// Count total records
	if err := pr.db.Model(&models.Payment{}).Where("status = ?", status).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count payments: %w", err)
	}

	// Calculate offset
	offset := (page - 1) * limit

	// Get payments with pagination
	if err := pr.db.Preload("User").Preload("Product").
		Where("status = ?", status).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&payments).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get payments: %w", err)
	}

	return payments, total, nil
}

// GetAll retrieves all payments with pagination and filters
func (pr *PaymentRepository) GetAll(query models.PaymentQuery) ([]models.Payment, int64, error) {
	var payments []models.Payment
	var total int64

	// Build query
	db := pr.db.Model(&models.Payment{})

	// Apply filters
	if query.UserID != nil {
		db = db.Where("user_id = ?", *query.UserID)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	if query.OrderID != nil {
		db = db.Where("order_id = ?", *query.OrderID)
	}

	// Count total records
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count payments: %w", err)
	}

	// Set default pagination values
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 {
		query.Limit = 10
	}

	// Calculate offset
	offset := (query.Page - 1) * query.Limit

	// Get payments with pagination
	if err := db.Preload("User").Preload("Product").
		Order("created_at DESC").
		Offset(offset).
		Limit(query.Limit).
		Find(&payments).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get payments: %w", err)
	}

	return payments, total, nil
}

// Update updates a payment
func (pr *PaymentRepository) Update(payment *models.Payment) error {
	if err := pr.db.Save(payment).Error; err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}
	return nil
}

// UpdateStatus updates payment status
func (pr *PaymentRepository) UpdateStatus(id uuid.UUID, status models.PaymentStatus) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	if status == models.PaymentStatusSuccess {
		updates["paid_at"] = time.Now()
	}

	if err := pr.db.Model(&models.Payment{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update payment status: %w", err)
	}
	return nil
}

// UpdateMidtransData updates Midtrans-related fields
func (pr *PaymentRepository) UpdateMidtransData(id uuid.UUID, midtransData map[string]interface{}) error {
	fmt.Printf("🔍 UpdateMidtransData called with ID: %s, Data: %+v\n", id.String(), midtransData)
	
	updates := map[string]interface{}{
		"updated_at": time.Now(),
	}

	// Add Midtrans fields if they exist
	if transactionID, ok := midtransData["transaction_id"].(string); ok {
		updates["midtrans_transaction_id"] = transactionID
	}
	if transactionStatus, ok := midtransData["transaction_status"].(string); ok {
		updates["transaction_status"] = transactionStatus
	}
	if fraudStatus, ok := midtransData["fraud_status"].(string); ok {
		updates["fraud_status"] = fraudStatus
	}
	if paymentCode, ok := midtransData["payment_code"].(string); ok {
		updates["payment_code"] = paymentCode
		fmt.Printf("🔍 Storing Payment Code in DB: %s\n", paymentCode)
	} else {
		fmt.Printf("⚠️ Payment Code not found or not a string: %v\n", midtransData["payment_code"])
	}
	if vaNumber, ok := midtransData["va_number"].(string); ok {
		updates["va_number"] = vaNumber
		fmt.Printf("🔍 Storing VA Number in DB: %s\n", vaNumber)
	} else {
		fmt.Printf("⚠️ VA Number not found or not a string: %v\n", midtransData["va_number"])
	}
	if bankType, ok := midtransData["bank_type"].(string); ok {
		updates["bank_type"] = bankType
	}
	if expiryTime, ok := midtransData["expiry_time"].(time.Time); ok {
		updates["expiry_time"] = expiryTime
	}
	if paidAt, ok := midtransData["paid_at"].(time.Time); ok {
		updates["paid_at"] = paidAt
	}
	if midtransResponse, ok := midtransData["midtrans_response"].(string); ok {
		updates["midtrans_response"] = midtransResponse
	}
	if midtransAction, ok := midtransData["midtrans_action"].(string); ok {
		updates["midtrans_action"] = midtransAction
	}
	if snapRedirectURL, ok := midtransData["snap_redirect_url"].(string); ok {
		updates["snap_redirect_url"] = snapRedirectURL
	}

	fmt.Printf("🔍 Final updates to save: %+v\n", updates)
	
	if err := pr.db.Model(&models.Payment{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		fmt.Printf("❌ Failed to update Midtrans data: %v\n", err)
		return fmt.Errorf("failed to update Midtrans data: %w", err)
	}
	
	fmt.Printf("✅ Successfully updated Midtrans data in database\n")
	return nil
}

// Delete deletes a payment
func (pr *PaymentRepository) Delete(id uuid.UUID) error {
	if err := pr.db.Delete(&models.Payment{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete payment: %w", err)
	}
	return nil
}

// GetPendingPayments retrieves pending payments older than specified duration
func (pr *PaymentRepository) GetPendingPayments(olderThan time.Duration) ([]models.Payment, error) {
	var payments []models.Payment
	cutoffTime := time.Now().Add(-olderThan)

	if err := pr.db.Where("status = ? AND created_at < ?", models.PaymentStatusPending, cutoffTime).
		Find(&payments).Error; err != nil {
		return nil, fmt.Errorf("failed to get pending payments: %w", err)
	}

	return payments, nil
}

// GetExpiredPayments retrieves expired payments
func (pr *PaymentRepository) GetExpiredPayments() ([]models.Payment, error) {
	var payments []models.Payment
	now := time.Now()

	if err := pr.db.Where("status = ? AND expiry_time < ?", models.PaymentStatusPending, now).
		Find(&payments).Error; err != nil {
		return nil, fmt.Errorf("failed to get expired payments: %w", err)
	}

	return payments, nil
}

// GetPaymentStats retrieves payment statistics
func (pr *PaymentRepository) GetPaymentStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count payments by status
	var statusCounts []struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}

	if err := pr.db.Model(&models.Payment{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&statusCounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get status counts: %w", err)
	}

	stats["status_counts"] = statusCounts

	// Total amount by status
	var amountByStatus []struct {
		Status string  `json:"status"`
		Amount float64 `json:"amount"`
	}

	if err := pr.db.Model(&models.Payment{}).
		Select("status, sum(total_amount) as amount").
		Group("status").
		Scan(&amountByStatus).Error; err != nil {
		return nil, fmt.Errorf("failed to get amount by status: %w", err)
	}

	stats["amount_by_status"] = amountByStatus

	// Total payments count
	var totalCount int64
	if err := pr.db.Model(&models.Payment{}).Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}

	stats["total_count"] = totalCount

	return stats, nil
}
