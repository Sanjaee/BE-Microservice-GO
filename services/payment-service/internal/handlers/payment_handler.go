package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"payment-service/internal/cache"
	"payment-service/internal/consumers"
	"payment-service/internal/events"
	"payment-service/internal/models"
	"payment-service/internal/repository"
	"payment-service/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PaymentHandler handles payment-related HTTP requests
type PaymentHandler struct {
	paymentRepo   *repository.PaymentRepository
	midtransSvc   *services.MidtransService
	eventSvc      *events.EventService
	cacheSvc      *cache.CacheService
	userServiceURL string
	productServiceURL string
	validationConsumer *consumers.ValidationConsumer
}

// NewPaymentHandler creates a new payment handler
func NewPaymentHandler(
	paymentRepo *repository.PaymentRepository,
	midtransSvc *services.MidtransService,
	eventSvc *events.EventService,
	cacheSvc *cache.CacheService,
	userServiceURL, productServiceURL string,
	validationConsumer *consumers.ValidationConsumer,
) *PaymentHandler {
	return &PaymentHandler{
		paymentRepo:       paymentRepo,
		midtransSvc:       midtransSvc,
		eventSvc:          eventSvc,
		cacheSvc:          cacheSvc,
		userServiceURL:    userServiceURL,
		productServiceURL: productServiceURL,
		validationConsumer: validationConsumer,
	}
}

// CreatePayment creates a new payment using event-driven architecture
func (ph *PaymentHandler) CreatePayment(c *gin.Context) {
	var req models.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Get user ID from header (set by API Gateway)
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	// Calculate total amount (amounts are in rupiah)
	totalAmount := req.Amount + req.AdminFee

	// Generate order ID and payment ID
	orderID := fmt.Sprintf("Order_%d", time.Now().UnixNano())
	paymentID := uuid.New().String()
	
	// Log payment details for debugging
	fmt.Printf("🔍 Event-Driven Payment Details - Amount: %d, AdminFee: %d, TotalAmount: %d, PaymentMethod: %s\n", 
		req.Amount, req.AdminFee, totalAmount, req.PaymentMethod)

	// Get user data from user service (for Midtrans)
	fmt.Printf("🔍 Getting user data for userID: %s from service: %s\n", userID.String(), ph.userServiceURL)
	user, err := ph.getUserFromService(userID)
	if err != nil {
		fmt.Printf("❌ Failed to get user data: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get user data",
			"details": err.Error(),
		})
		return
	}
	fmt.Printf("✅ Successfully got user data: %+v\n", user)

	// Get product data from product service (for Midtrans)
	product, err := ph.getProductFromService(*req.ProductID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Product not found",
		})
		return
	}

	// Check if product is active and has stock
	if !product.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Product is not active",
		})
		return
	}

	if product.Stock <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Product is out of stock",
		})
		return
	}

	// Create payment record (without Midtrans data yet)
	payment := &models.Payment{
		ID:            uuid.MustParse(paymentID),
		OrderID:       orderID,
		UserID:        userID,
		ProductID:     req.ProductID,
		Amount:        req.Amount,
		AdminFee:      req.AdminFee,
		TotalAmount:   totalAmount,
		PaymentMethod: req.PaymentMethod,
		PaymentType:   "midtrans",
		Status:        models.PaymentStatusPending,
		Notes:         req.Notes,
		BankType:      req.BankType,  // Store bank type for bank transfer payments
		StoreType:     req.StoreType, // Store store type for cstore payments
	}

	// Create payment with Midtrans first (before saving to database)
	midtransResp, err := ph.midtransSvc.CreatePayment(payment, user, product)
	if err != nil {
		// Check if it's a 505 or 500 error from Midtrans (VA number creation failed or system issues)
		if strings.Contains(err.Error(), "505") || 
		   strings.Contains(err.Error(), "500") ||
		   strings.Contains(err.Error(), "Unable to create va_number") ||
		   strings.Contains(err.Error(), "system is recovering") ||
		   strings.Contains(err.Error(), "service unavailable") {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"error":   "Payment method temporarily unavailable",
				"message": "Metode pembayaran sedang maintenance, silakan pilih metode lain (BNI, BCA, BRI, Mandiri, GoPay, QRIS, atau Credit Card)",
				"details": err.Error(),
			})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Failed to create payment with Midtrans",
				"details": err.Error(),
			})
		}
		return
	}

	// Save payment to database only after successful Midtrans response
	if err := ph.paymentRepo.Create(payment); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create payment",
		})
		return
	}

	// Update payment with Midtrans response
	midtransData := map[string]interface{}{
		"transaction_id":     midtransResp.TransactionID,
		"transaction_status": midtransResp.TransactionStatus,
		"fraud_status":       midtransResp.FraudStatus,
		"midtrans_response":  ph.marshalToJSON(midtransResp),
		"midtrans_action":    ph.marshalToJSON(midtransResp.Actions),
	}

	// Add payment method specific data
	if len(midtransResp.VANumbers) > 0 {
		midtransData["va_number"] = midtransResp.VANumbers[0].VANumber
		midtransData["bank_type"] = midtransResp.VANumbers[0].Bank
		fmt.Printf("🔍 Storing VA Number: %s, Bank: %s\n", midtransResp.VANumbers[0].VANumber, midtransResp.VANumbers[0].Bank)
	} else {
		fmt.Printf("⚠️ No VA Numbers found in Midtrans response\n")
	}

	if midtransResp.PaymentCode != "" {
		midtransData["payment_code"] = midtransResp.PaymentCode
		fmt.Printf("🔍 Storing Payment Code: %s\n", midtransResp.PaymentCode)
		// For cstore payments, also store payment_code as va_number for easier copying
		if payment.PaymentMethod == models.PaymentMethodCstore {
			midtransData["va_number"] = midtransResp.PaymentCode
			fmt.Printf("🔍 Storing Payment Code as VA Number for cstore: %s\n", midtransResp.PaymentCode)
		}
	} else {
		fmt.Printf("⚠️ No Payment Code found in Midtrans response\n")
	}

	if midtransResp.PermataVANumber != "" {
		midtransData["va_number"] = midtransResp.PermataVANumber
		midtransData["bank_type"] = "permata"
	}

	if midtransResp.ExpiryTime != "" {
		// Try different time formats from Midtrans
		timeFormats := []string{
			time.RFC3339,                    // "2006-01-02T15:04:05Z07:00"
			"2006-01-02 15:04:05",          // "2025-09-29 20:47:00"
			"2006-01-02T15:04:05",          // "2025-09-29T20:47:00"
		}
		
		var expiryTime time.Time
		var err error
		for _, format := range timeFormats {
			expiryTime, err = time.Parse(format, midtransResp.ExpiryTime)
			if err == nil {
				midtransData["expiry_time"] = expiryTime
				break
			}
		}
	}

	if midtransResp.PaidAt != "" {
		// Try different time formats from Midtrans
		timeFormats := []string{
			time.RFC3339,                    // "2006-01-02T15:04:05Z07:00"
			"2006-01-02 15:04:05",          // "2025-09-29 20:47:00"
			"2006-01-02T15:04:05",          // "2025-09-29T20:47:00"
		}
		
		var paidAt time.Time
		var err error
		for _, format := range timeFormats {
			paidAt, err = time.Parse(format, midtransResp.PaidAt)
			if err == nil {
				midtransData["paid_at"] = paidAt
				break
			}
		}
	}

	// Find QR code or redirect URL in actions
	for _, action := range midtransResp.Actions {
		if action.Name == "generate-qr-code" || action.Name == "get-status" {
			midtransData["snap_redirect_url"] = action.URL
			break
		}
	}

	// Log the data being saved
	fmt.Printf("🔍 Updating payment with Midtrans data: %+v\n", midtransData)
	
	if err := ph.paymentRepo.UpdateMidtransData(payment.ID, midtransData); err != nil {
		fmt.Printf("❌ Failed to update payment with Midtrans data: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update payment with Midtrans data",
		})
		return
	}
	
	fmt.Printf("✅ Successfully updated payment with Midtrans data\n")

	// Wait for VA number to be saved in database with retry mechanism
	updatedPayment, err := ph.waitForPaymentData(payment.ID, 5, 1*time.Second)
	if err != nil {
		fmt.Printf("⚠️ Failed to get updated payment data after retries: %v\n", err)
		// Fallback to original payment data
		updatedPayment = payment
	}

	// Cache payment data
	paymentResponse := updatedPayment.ToResponse()
	paymentResponse.Actions = ph.convertMidtransActions(midtransResp.Actions)
	
	ph.cacheSvc.SetPayment(payment.ID.String(), paymentResponse, 1*time.Hour)
	ph.cacheSvc.SetPaymentByOrderID(payment.OrderID, paymentResponse, 1*time.Hour)

	// Publish payment created event (optional for other services)
	ph.eventSvc.PublishPaymentCreated(
		payment.ID.String(),
		payment.OrderID,
		payment.UserID.String(),
		payment.ProductID,
		payment.Amount,
		payment.TotalAmount,
		string(payment.PaymentMethod),
		string(payment.Status),
	)

	// Invalidate user payments cache
	ph.cacheSvc.DeleteUserPayments(payment.UserID.String())

	// Use updated payment data for response
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"payment_id":     updatedPayment.ID,
			"order_id":       updatedPayment.OrderID,
			"amount":         updatedPayment.TotalAmount,
			"payment_method": updatedPayment.PaymentMethod,
			"status":         updatedPayment.Status,
			"actions":        midtransResp.Actions,
			"va_number":      updatedPayment.VANumber,
			"bank_type":      updatedPayment.BankType,
			"payment_code":   updatedPayment.PaymentCode,
			"expiry_time":    updatedPayment.ExpiryTime,
			"redirect_url":   updatedPayment.SnapRedirectURL,
		},
	})
}

// GetPayment retrieves a payment by ID
func (ph *PaymentHandler) GetPayment(c *gin.Context) {
	paymentIDStr := c.Param("id")
	paymentID, err := uuid.Parse(paymentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid payment ID",
		})
		return
	}

	// Try to get from cache first
	var paymentResponse models.PaymentResponse
	if err := ph.cacheSvc.GetPayment(paymentID.String(), &paymentResponse); err == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    paymentResponse,
		})
		return
	}

	// Get from database
	payment, err := ph.paymentRepo.GetByID(paymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Payment not found",
		})
		return
	}

	paymentResponse = payment.ToResponse()
	
	// Parse Midtrans actions if available
	if payment.MidtransAction != nil {
		var actions []models.MidtransAction
		if err := json.Unmarshal([]byte(*payment.MidtransAction), &actions); err == nil {
			paymentResponse.Actions = actions
		}
	}

	// Cache the response
	ph.cacheSvc.SetPayment(payment.ID.String(), paymentResponse, 1*time.Hour)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    paymentResponse,
	})
}

// GetPaymentByOrderID retrieves a payment by order ID
func (ph *PaymentHandler) GetPaymentByOrderID(c *gin.Context) {
	orderID := c.Param("order_id")

	// Try to get from cache first
	var paymentResponse models.PaymentResponse
	if err := ph.cacheSvc.GetPaymentByOrderID(orderID, &paymentResponse); err == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    paymentResponse,
		})
		return
	}

	// Get from database
	payment, err := ph.paymentRepo.GetByOrderID(orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Payment not found",
		})
		return
	}

	paymentResponse = payment.ToResponse()
	
	// Parse Midtrans actions if available
	if payment.MidtransAction != nil {
		var actions []models.MidtransAction
		if err := json.Unmarshal([]byte(*payment.MidtransAction), &actions); err == nil {
			paymentResponse.Actions = actions
		}
	}

	// Cache the response
	ph.cacheSvc.SetPaymentByOrderID(payment.OrderID, paymentResponse, 1*time.Hour)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    paymentResponse,
	})
}

// GetUserPayments retrieves payments for a user
func (ph *PaymentHandler) GetUserPayments(c *gin.Context) {
	// Get user ID from header (set by API Gateway)
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	// Try to get from cache first
	cacheKey := fmt.Sprintf("%s_%d_%d", userID.String(), page, limit)
	var paymentsResponse models.PaymentListResponse
	if err := ph.cacheSvc.GetUserPayments(cacheKey, &paymentsResponse); err == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    paymentsResponse,
		})
		return
	}

	// Get from database
	payments, total, err := ph.paymentRepo.GetByUserID(userID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get payments",
		})
		return
	}

	// Convert to response format
	paymentResponses := make([]models.PaymentResponse, len(payments))
	for i, payment := range payments {
		paymentResponses[i] = payment.ToResponse()
		
		// Parse Midtrans actions if available
		if payment.MidtransAction != nil {
			var actions []models.MidtransAction
			if err := json.Unmarshal([]byte(*payment.MidtransAction), &actions); err == nil {
				paymentResponses[i].Actions = actions
			}
		}
	}

	paymentsResponse = models.PaymentListResponse{
		Payments: paymentResponses,
		Total:    total,
		Page:     page,
		Limit:    limit,
		HasMore:  int64(page*limit) < total,
	}

	// Cache the response
	ph.cacheSvc.SetUserPayments(cacheKey, paymentsResponse, 30*time.Minute)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    paymentsResponse,
	})
}

// MidtransCallback handles Midtrans webhook callback
func (ph *PaymentHandler) MidtransCallback(c *gin.Context) {
	var req models.MidtransCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("❌ Invalid callback format: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid callback format",
		})
		return
	}

	// Log callback received
	fmt.Printf("📞 Midtrans callback received for order: %s, status: %s\n", req.OrderID, req.TransactionStatus)

	// Verify signature
	if !ph.midtransSvc.VerifySignature(req.OrderID, req.StatusCode, req.GrossAmount, req.SignatureKey) {
		fmt.Printf("❌ Invalid signature for order: %s\n", req.OrderID)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid signature",
		})
		return
	}

	// Get payment from database
	payment, err := ph.paymentRepo.GetByOrderID(req.OrderID)
	if err != nil {
		fmt.Printf("❌ Payment not found for order: %s, error: %v\n", req.OrderID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Payment not found",
		})
		return
	}

	fmt.Printf("🔍 Found payment: %s, current status: %s\n", payment.ID.String(), payment.Status)

	// Get detailed status from Midtrans with retry mechanism
	var statusResp *services.MidtransStatusResponse
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		statusResp, err = ph.midtransSvc.GetPaymentStatus(req.OrderID)
		if err == nil {
			break
		}
		fmt.Printf("⚠️ Attempt %d: Failed to get payment status from Midtrans: %v\n", attempt+1, err)
		if attempt < maxRetries-1 {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	if err != nil {
		fmt.Printf("❌ Failed to get payment status from Midtrans after %d attempts: %v\n", maxRetries, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get payment status from Midtrans",
		})
		return
	}

	// Map Midtrans status to our status
	newStatus := ph.midtransSvc.MapMidtransStatusToPaymentStatus(statusResp.TransactionStatus)
	oldStatus := payment.Status

	fmt.Printf("🔄 Status change: %s -> %s (Midtrans: %s)\n", oldStatus, newStatus, statusResp.TransactionStatus)

	// Update payment status
	if err := ph.paymentRepo.UpdateStatus(payment.ID, newStatus); err != nil {
		fmt.Printf("❌ Failed to update payment status: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update payment status",
		})
		return
	}

	// Update Midtrans data
	midtransData := map[string]interface{}{
		"transaction_id":     statusResp.TransactionID,
		"transaction_status": statusResp.TransactionStatus,
		"fraud_status":       statusResp.FraudStatus,
		"midtrans_response":  ph.marshalToJSON(statusResp),
		"midtrans_action":    ph.marshalToJSON(statusResp.Actions),
	}

	// Add payment method specific data
	if len(statusResp.VANumbers) > 0 {
		midtransData["va_number"] = statusResp.VANumbers[0].VANumber
		midtransData["bank_type"] = statusResp.VANumbers[0].Bank
		fmt.Printf("🔍 Updated VA Number: %s, Bank: %s\n", statusResp.VANumbers[0].VANumber, statusResp.VANumbers[0].Bank)
	}

	if statusResp.PaymentCode != "" {
		midtransData["payment_code"] = statusResp.PaymentCode
		fmt.Printf("🔍 Updated Payment Code: %s\n", statusResp.PaymentCode)
		// For cstore payments, also store payment_code as va_number for easier copying
		if payment.PaymentMethod == models.PaymentMethodCstore {
			midtransData["va_number"] = statusResp.PaymentCode
		}
	}

	if statusResp.PermataVANumber != "" {
		midtransData["va_number"] = statusResp.PermataVANumber
		midtransData["bank_type"] = "permata"
		fmt.Printf("🔍 Updated Permata VA Number: %s\n", statusResp.PermataVANumber)
	}

	if statusResp.ExpiryTime != "" {
		// Try different time formats from Midtrans
		timeFormats := []string{
			time.RFC3339,                    // "2006-01-02T15:04:05Z07:00"
			"2006-01-02 15:04:05",          // "2025-09-29 20:47:00"
			"2006-01-02T15:04:05",          // "2025-09-29T20:47:00"
		}
		
		var expiryTime time.Time
		var err error
		for _, format := range timeFormats {
			expiryTime, err = time.Parse(format, statusResp.ExpiryTime)
			if err == nil {
				midtransData["expiry_time"] = expiryTime
				fmt.Printf("🔍 Updated Expiry Time: %s\n", expiryTime.Format(time.RFC3339))
				break
			}
		}
	}

	if statusResp.PaidAt != "" {
		// Try different time formats from Midtrans
		timeFormats := []string{
			time.RFC3339,                    // "2006-01-02T15:04:05Z07:00"
			"2006-01-02 15:04:05",          // "2025-09-29 20:47:00"
			"2006-01-02T15:04:05",          // "2025-09-29T20:47:00"
		}
		
		var paidAt time.Time
		var err error
		for _, format := range timeFormats {
			paidAt, err = time.Parse(format, statusResp.PaidAt)
			if err == nil {
				midtransData["paid_at"] = paidAt
				fmt.Printf("🔍 Updated Paid At: %s\n", paidAt.Format(time.RFC3339))
				break
			}
		}
	} else if newStatus == models.PaymentStatusSuccess && payment.PaidAt == nil {
		// If payment is successful but no paid_at from Midtrans, set it to current time
		midtransData["paid_at"] = time.Now()
		fmt.Printf("🔍 Set Paid At to current time for successful payment\n")
	}

	// Update Midtrans data in database
	if err := ph.paymentRepo.UpdateMidtransData(payment.ID, midtransData); err != nil {
		fmt.Printf("❌ Failed to update Midtrans data: %v\n", err)
		// Don't return error here, just log it
	}

	// Invalidate cache
	ph.cacheSvc.InvalidatePaymentCache(payment.ID.String(), payment.OrderID, payment.UserID.String())
	fmt.Printf("🗑️ Invalidated cache for payment: %s\n", payment.ID.String())

	// Publish events based on status change
	if newStatus != oldStatus {
		fmt.Printf("📢 Publishing status change event: %s -> %s\n", oldStatus, newStatus)
		
		ph.eventSvc.PublishPaymentStatusUpdated(
			payment.ID.String(),
			payment.OrderID,
			payment.UserID.String(),
			payment.ProductID,
			string(oldStatus),
			string(newStatus),
			payment.Amount,
			payment.TotalAmount,
			string(payment.PaymentMethod),
			payment.PaidAt,
		)

		if newStatus == models.PaymentStatusSuccess {
			fmt.Printf("🎉 Payment successful! Publishing success event\n")
			ph.eventSvc.PublishPaymentSuccess(
				payment.ID.String(),
				payment.OrderID,
				payment.UserID.String(),
				payment.ProductID,
				payment.Amount,
				payment.TotalAmount,
				string(payment.PaymentMethod),
				time.Now(),
			)

			// Publish stock reduction event
			if payment.ProductID != nil {
				ph.eventSvc.PublishStockReduction(
					*payment.ProductID,
					1, // Assuming quantity 1
					payment.OrderID,
					payment.UserID.String(),
				)
				fmt.Printf("📦 Published stock reduction event for product: %s\n", payment.ProductID.String())
			}
		} else if newStatus == models.PaymentStatusFailed || newStatus == models.PaymentStatusCancelled || newStatus == models.PaymentStatusExpired {
			fmt.Printf("❌ Payment failed/cancelled/expired! Publishing failure event\n")
			ph.eventSvc.PublishPaymentFailed(
				payment.ID.String(),
				payment.OrderID,
				payment.UserID.String(),
				payment.ProductID,
				payment.Amount,
				payment.TotalAmount,
				string(payment.PaymentMethod),
				string(newStatus),
			)
		}
	} else {
		fmt.Printf("ℹ️ No status change detected\n")
	}

	fmt.Printf("✅ Callback processed successfully for order: %s\n", req.OrderID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Callback processed successfully",
	})
}

// GetMidtransConfig returns Midtrans configuration for frontend
func (ph *PaymentHandler) GetMidtransConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"client_key":  ph.midtransSvc.GetClientKey(),
			"environment": ph.midtransSvc.GetEnvironment(),
		},
	})
}

// CheckPaymentStatus manually checks payment status from Midtrans
func (ph *PaymentHandler) CheckPaymentStatus(c *gin.Context) {
	paymentIDStr := c.Param("id")
	paymentID, err := uuid.Parse(paymentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid payment ID",
		})
		return
	}

	// Get payment from database
	payment, err := ph.paymentRepo.GetByID(paymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Payment not found",
		})
		return
	}

	// Get detailed status from Midtrans
	statusResp, err := ph.midtransSvc.GetPaymentStatus(payment.OrderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get payment status from Midtrans",
		})
		return
	}

	// Map Midtrans status to our status
	newStatus := ph.midtransSvc.MapMidtransStatusToPaymentStatus(statusResp.TransactionStatus)
	oldStatus := payment.Status

	fmt.Printf("🔍 Manual status check - Order: %s, Old: %s, New: %s (Midtrans: %s)\n", 
		payment.OrderID, oldStatus, newStatus, statusResp.TransactionStatus)

	// Update payment status if changed
	if newStatus != oldStatus {
		if err := ph.paymentRepo.UpdateStatus(payment.ID, newStatus); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to update payment status",
			})
			return
		}

		// Update Midtrans data
		midtransData := map[string]interface{}{
			"transaction_id":     statusResp.TransactionID,
			"transaction_status": statusResp.TransactionStatus,
			"fraud_status":       statusResp.FraudStatus,
			"midtrans_response":  ph.marshalToJSON(statusResp),
			"midtrans_action":    ph.marshalToJSON(statusResp.Actions),
		}

		// Add payment method specific data
		if len(statusResp.VANumbers) > 0 {
			midtransData["va_number"] = statusResp.VANumbers[0].VANumber
			midtransData["bank_type"] = statusResp.VANumbers[0].Bank
		}

		if statusResp.PaymentCode != "" {
			midtransData["payment_code"] = statusResp.PaymentCode
			if payment.PaymentMethod == models.PaymentMethodCstore {
				midtransData["va_number"] = statusResp.PaymentCode
			}
		}

		if statusResp.PermataVANumber != "" {
			midtransData["va_number"] = statusResp.PermataVANumber
			midtransData["bank_type"] = "permata"
		}

		if statusResp.ExpiryTime != "" {
			timeFormats := []string{
				time.RFC3339,
				"2006-01-02 15:04:05",
				"2006-01-02T15:04:05",
			}
			
			for _, format := range timeFormats {
				if expiryTime, err := time.Parse(format, statusResp.ExpiryTime); err == nil {
					midtransData["expiry_time"] = expiryTime
					break
				}
			}
		}

		if statusResp.PaidAt != "" {
			timeFormats := []string{
				time.RFC3339,
				"2006-01-02 15:04:05",
				"2006-01-02T15:04:05",
			}
			
			for _, format := range timeFormats {
				if paidAt, err := time.Parse(format, statusResp.PaidAt); err == nil {
					midtransData["paid_at"] = paidAt
					break
				}
			}
		} else if newStatus == models.PaymentStatusSuccess && payment.PaidAt == nil {
			midtransData["paid_at"] = time.Now()
		}

		ph.paymentRepo.UpdateMidtransData(payment.ID, midtransData)

		// Invalidate cache
		ph.cacheSvc.InvalidatePaymentCache(payment.ID.String(), payment.OrderID, payment.UserID.String())

		// Publish events based on status change
		ph.eventSvc.PublishPaymentStatusUpdated(
			payment.ID.String(),
			payment.OrderID,
			payment.UserID.String(),
			payment.ProductID,
			string(oldStatus),
			string(newStatus),
			payment.Amount,
			payment.TotalAmount,
			string(payment.PaymentMethod),
			payment.PaidAt,
		)

		if newStatus == models.PaymentStatusSuccess {
			ph.eventSvc.PublishPaymentSuccess(
				payment.ID.String(),
				payment.OrderID,
				payment.UserID.String(),
				payment.ProductID,
				payment.Amount,
				payment.TotalAmount,
				string(payment.PaymentMethod),
				time.Now(),
			)

			// Publish stock reduction event
			if payment.ProductID != nil {
				ph.eventSvc.PublishStockReduction(
					*payment.ProductID,
					1,
					payment.OrderID,
					payment.UserID.String(),
				)
			}
		} else if newStatus == models.PaymentStatusFailed || newStatus == models.PaymentStatusCancelled || newStatus == models.PaymentStatusExpired {
			ph.eventSvc.PublishPaymentFailed(
				payment.ID.String(),
				payment.OrderID,
				payment.UserID.String(),
				payment.ProductID,
				payment.Amount,
				payment.TotalAmount,
				string(payment.PaymentMethod),
				string(newStatus),
			)
		}

		fmt.Printf("✅ Status updated from %s to %s\n", oldStatus, newStatus)
	}

	// Get updated payment data
	updatedPayment, err := ph.paymentRepo.GetByID(paymentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get updated payment data",
		})
		return
	}

	paymentResponse := updatedPayment.ToResponse()
	
	// Parse Midtrans actions if available
	if updatedPayment.MidtransAction != nil {
		var actions []models.MidtransAction
		if err := json.Unmarshal([]byte(*updatedPayment.MidtransAction), &actions); err == nil {
			paymentResponse.Actions = actions
		}
	}

	// Cache the response
	ph.cacheSvc.SetPayment(payment.ID.String(), paymentResponse, 1*time.Hour)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    paymentResponse,
		"status_changed": newStatus != oldStatus,
		"old_status": string(oldStatus),
		"new_status": string(newStatus),
	})
}

// Helper methods

func (ph *PaymentHandler) getUserFromService(userID uuid.UUID) (*models.User, error) {
	// Make HTTP request to user service
	url := fmt.Sprintf("%s/api/v1/users/%s", ph.userServiceURL, userID.String())
	fmt.Printf("🔍 Making request to user service: %s\n", url)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("❌ Failed to create request: %v\n", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Add headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	
	// Make request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Failed to make request to user service: %v\n", err)
		return nil, fmt.Errorf("failed to make request to user service: %w", err)
	}
	defer resp.Body.Close()
	
	fmt.Printf("🔍 User service response status: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		// Read response body for error details
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("❌ User service error response: %s\n", string(body))
		return nil, fmt.Errorf("user service returned status %d: %s", resp.StatusCode, string(body))
	}
	
	// Parse response
	var userResp struct {
		Success bool `json:"success"`
		Data    struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Email    string `json:"email"`
		} `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		return nil, fmt.Errorf("failed to decode user response: %w", err)
	}
	
	if !userResp.Success {
		return nil, fmt.Errorf("user service returned error")
	}
	
	// Convert to our User model
	userUUID, err := uuid.Parse(userResp.Data.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID format: %w", err)
	}
	
	return &models.User{
		ID:       userUUID,
		Username: userResp.Data.Username,
		Email:    userResp.Data.Email,
	}, nil
}

func (ph *PaymentHandler) getProductFromService(productID uuid.UUID) (*models.Product, error) {
	// Make HTTP request to product service
	url := fmt.Sprintf("%s/api/v1/products/%s", ph.productServiceURL, productID.String())
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Add headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	
	// Make request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to product service: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("product service returned status %d", resp.StatusCode)
	}
	
	// Parse response
	var productResp struct {
		Success bool `json:"success"`
		Data    struct {
			ID          string  `json:"id"`
			Name        string  `json:"name"`
			Description string  `json:"description"`
			Price       float64 `json:"price"`
			Stock       int     `json:"stock"`
			IsActive    bool    `json:"is_active"`
		} `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&productResp); err != nil {
		return nil, fmt.Errorf("failed to decode product response: %w", err)
	}
	
	if !productResp.Success {
		return nil, fmt.Errorf("product service returned error")
	}
	
	// Convert to our Product model
	productUUID, err := uuid.Parse(productResp.Data.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid product ID format: %w", err)
	}
	
	return &models.Product{
		ID:          productUUID,
		Name:        productResp.Data.Name,
		Description: productResp.Data.Description,
		Price:       productResp.Data.Price,
		Stock:       productResp.Data.Stock,
		IsActive:    productResp.Data.IsActive,
	}, nil
}

func (ph *PaymentHandler) marshalToJSON(data interface{}) string {
	jsonData, _ := json.Marshal(data)
	return string(jsonData)
}

func (ph *PaymentHandler) convertMidtransActions(actions []services.MidtransAction) []models.MidtransAction {
	result := make([]models.MidtransAction, len(actions))
	for i, action := range actions {
		result[i] = models.MidtransAction{
			Name:   action.Name,
			Method: action.Method,
			URL:    action.URL,
		}
	}
	return result
}

// waitForPaymentData waits for payment data to be saved in database
func (ph *PaymentHandler) waitForPaymentData(paymentID uuid.UUID, maxRetries int, delay time.Duration) (*models.Payment, error) {
	for attempt := 0; attempt < maxRetries; attempt++ {
		payment, err := ph.paymentRepo.GetByIDWithoutRelations(paymentID)
		if err != nil {
			fmt.Printf("⚠️ Attempt %d: Failed to get payment data: %v\n", attempt+1, err)
			if attempt < maxRetries-1 {
				time.Sleep(delay)
				continue
			}
			return nil, err
		}

		// Check if VA number or payment code is available based on payment method
		hasRequiredData := false
		switch payment.PaymentMethod {
		case models.PaymentMethodBankTransfer, models.PaymentMethodPermata:
			// For bank transfer, check if VA number exists
			if payment.VANumber != nil && *payment.VANumber != "" {
				hasRequiredData = true
				fmt.Printf("✅ VA Number found: %s\n", *payment.VANumber)
			}
		case models.PaymentMethodCstore:
			// For cstore, check if payment code exists
			if payment.PaymentCode != nil && *payment.PaymentCode != "" {
				hasRequiredData = true
				fmt.Printf("✅ Payment Code found: %s\n", *payment.PaymentCode)
			}
		case models.PaymentMethodGoPay, models.PaymentMethodQRIS, models.PaymentMethodCreditCard:
			// For these methods, we don't need to wait for specific data
			hasRequiredData = true
		default:
			hasRequiredData = true
		}

		if hasRequiredData {
			fmt.Printf("✅ Payment data is ready for response\n")
			return payment, nil
		}

		fmt.Printf("⏳ Attempt %d: Payment data not ready yet, retrying...\n", attempt+1)
		if attempt < maxRetries-1 {
			time.Sleep(delay)
		}
	}

	return nil, fmt.Errorf("payment data not ready after %d attempts", maxRetries)
}
