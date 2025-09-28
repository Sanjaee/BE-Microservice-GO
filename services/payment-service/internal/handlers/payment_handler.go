package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"payment-service/internal/cache"
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
}

// NewPaymentHandler creates a new payment handler
func NewPaymentHandler(
	paymentRepo *repository.PaymentRepository,
	midtransSvc *services.MidtransService,
	eventSvc *events.EventService,
	cacheSvc *cache.CacheService,
	userServiceURL, productServiceURL string,
) *PaymentHandler {
	return &PaymentHandler{
		paymentRepo:       paymentRepo,
		midtransSvc:       midtransSvc,
		eventSvc:          eventSvc,
		cacheSvc:          cacheSvc,
		userServiceURL:    userServiceURL,
		productServiceURL: productServiceURL,
	}
}

// CreatePayment creates a new payment
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

	// Get user data from user service
	user, err := ph.getUserFromService(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get user data",
		})
		return
	}

	// Get product data from product service
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

	// Calculate total amount (ensure amounts are in cents)
	totalAmount := req.Amount + req.AdminFee

	// Generate order ID
	orderID := fmt.Sprintf("Order_%d", time.Now().UnixNano())
	
	// Log payment details for debugging
	fmt.Printf("🔍 Payment Details - Amount: %d, AdminFee: %d, TotalAmount: %d, PaymentMethod: %s\n", 
		req.Amount, req.AdminFee, totalAmount, req.PaymentMethod)

	// Create payment record
	payment := &models.Payment{
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
	}

	// Save payment to database
	if err := ph.paymentRepo.Create(payment); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create payment",
		})
		return
	}

	// Create payment with Midtrans
	midtransResp, err := ph.midtransSvc.CreatePayment(payment, user, product)
	if err != nil {
		// Update payment status to failed
		ph.paymentRepo.UpdateStatus(payment.ID, models.PaymentStatusFailed)
		
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Failed to create payment with Midtrans",
			"details": err.Error(),
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
	}

	if midtransResp.PaymentCode != "" {
		midtransData["payment_code"] = midtransResp.PaymentCode
	}

	if midtransResp.PermataVANumber != "" {
		midtransData["va_number"] = midtransResp.PermataVANumber
		midtransData["bank_type"] = "permata"
	}

	if midtransResp.ExpiryTime != "" {
		if expiryTime, err := time.Parse(time.RFC3339, midtransResp.ExpiryTime); err == nil {
			midtransData["expiry_time"] = expiryTime
		}
	}

	if midtransResp.PaidAt != "" {
		if paidAt, err := time.Parse(time.RFC3339, midtransResp.PaidAt); err == nil {
			midtransData["paid_at"] = paidAt
		}
	}

	// Find QR code or redirect URL in actions
	for _, action := range midtransResp.Actions {
		if action.Name == "generate-qr-code" || action.Name == "get-status" {
			midtransData["snap_redirect_url"] = action.URL
			break
		}
	}

	if err := ph.paymentRepo.UpdateMidtransData(payment.ID, midtransData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update payment with Midtrans data",
		})
		return
	}

	// Cache payment data
	paymentResponse := payment.ToResponse()
	paymentResponse.Actions = ph.convertMidtransActions(midtransResp.Actions)
	
	ph.cacheSvc.SetPayment(payment.ID.String(), paymentResponse, 1*time.Hour)
	ph.cacheSvc.SetPaymentByOrderID(payment.OrderID, paymentResponse, 1*time.Hour)

	// Publish payment created event
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

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"payment_id":     payment.ID,
			"order_id":       payment.OrderID,
			"amount":         payment.TotalAmount,
			"payment_method": payment.PaymentMethod,
			"status":         payment.Status,
			"actions":        midtransResp.Actions,
			"va_number":      midtransData["va_number"],
			"bank_type":      midtransData["bank_type"],
			"payment_code":   midtransData["payment_code"],
			"expiry_time":    midtransData["expiry_time"],
			"redirect_url":   midtransData["snap_redirect_url"],
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
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid callback format",
		})
		return
	}

	// Verify signature
	if !ph.midtransSvc.VerifySignature(req.OrderID, req.StatusCode, req.GrossAmount, req.SignatureKey) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid signature",
		})
		return
	}

	// Get payment from database
	payment, err := ph.paymentRepo.GetByOrderID(req.OrderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Payment not found",
		})
		return
	}

	// Get detailed status from Midtrans
	statusResp, err := ph.midtransSvc.GetPaymentStatus(req.OrderID)
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

	// Update payment status
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
	}

	if statusResp.PermataVANumber != "" {
		midtransData["va_number"] = statusResp.PermataVANumber
		midtransData["bank_type"] = "permata"
	}

	if statusResp.ExpiryTime != "" {
		if expiryTime, err := time.Parse(time.RFC3339, statusResp.ExpiryTime); err == nil {
			midtransData["expiry_time"] = expiryTime
		}
	}

	if statusResp.PaidAt != "" {
		if paidAt, err := time.Parse(time.RFC3339, statusResp.PaidAt); err == nil {
			midtransData["paid_at"] = paidAt
		}
	}

	ph.paymentRepo.UpdateMidtransData(payment.ID, midtransData)

	// Invalidate cache
	ph.cacheSvc.InvalidatePaymentCache(payment.ID.String(), payment.OrderID, payment.UserID.String())

	// Publish events based on status change
	if newStatus != oldStatus {
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
					1, // Assuming quantity 1
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
	}

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

// Helper methods

func (ph *PaymentHandler) getUserFromService(userID uuid.UUID) (*models.User, error) {
	// This would typically make an HTTP request to user service
	// For now, return a mock user
	return &models.User{
		ID:       userID,
		Username: "testuser",
		Email:    "test@example.com",
	}, nil
}

func (ph *PaymentHandler) getProductFromService(productID uuid.UUID) (*models.Product, error) {
	// This would typically make an HTTP request to product service
	// For now, return a mock product
	return &models.Product{
		ID:          productID,
		Name:        "Test Product",
		Description: "Test Product Description",
		Price:       100000.0, // Make sure this is float64
		Stock:       10,
		IsActive:    true,
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
