package services

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"time"

	"payment-service/internal/models"
)

// MidtransService handles Midtrans payment operations
type MidtransService struct {
	serverKey      string
	clientKey      string
	baseURL        string
	httpClient     *http.Client
	environment    string
	authHeader     string // Cached authorization header
}

// MidtransChargeRequest represents the charge request to Midtrans
type MidtransChargeRequest struct {
	PaymentType        string                 `json:"payment_type"`
	TransactionDetails TransactionDetails     `json:"transaction_details"`
	CustomerDetails    CustomerDetails        `json:"customer_details"`
	ItemDetails        []ItemDetails          `json:"item_details"`
	BankTransfer       *BankTransferDetails   `json:"bank_transfer,omitempty"`
	CreditCard         *CreditCardDetails     `json:"credit_card,omitempty"`
	GoPay              *GoPayDetails          `json:"gopay,omitempty"`
	QRIS               *QRISDetails           `json:"qris,omitempty"`
	ShopeePay          *ShopeePayDetails      `json:"shopeepay,omitempty"`
	Echannel           *EchannelDetails       `json:"echannel,omitempty"`
	Cstore             *CstoreDetails         `json:"cstore,omitempty"`
}

// TransactionDetails represents transaction details
type TransactionDetails struct {
	OrderID     string `json:"order_id"`
	GrossAmount int64  `json:"gross_amount"`
}

// CustomerDetails represents customer details
type CustomerDetails struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Email     string `json:"email"`
	Phone     string `json:"phone,omitempty"`
}

// ItemDetails represents item details
type ItemDetails struct {
	ID       string `json:"id"`
	Price    int64  `json:"price"`
	Quantity int    `json:"quantity"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

// BankTransferDetails represents bank transfer details
type BankTransferDetails struct {
	Bank string `json:"bank"`
}

// CreditCardDetails represents credit card details
type CreditCardDetails struct {
	Secure         bool `json:"secure"`
	Authentication bool `json:"authentication"`
}

// GoPayDetails represents GoPay details
type GoPayDetails struct {
	EnableCallback bool   `json:"enable_callback"`
	CallbackURL    string `json:"callback_url,omitempty"`
}

// QRISDetails represents QRIS details
type QRISDetails struct {
	Acquirer string `json:"acquirer,omitempty"`
}

// ShopeePayDetails represents ShopeePay details
type ShopeePayDetails struct {
	CallbackURL string `json:"callback_url,omitempty"`
}

// EchannelDetails represents Echannel details
type EchannelDetails struct {
	BillInfo1 string `json:"bill_info1,omitempty"`
	BillInfo2 string `json:"bill_info2,omitempty"`
}

// CstoreDetails represents Cstore details
type CstoreDetails struct {
	Store                 string `json:"store"`
	Message               string `json:"message,omitempty"`
	AlfamartFreeText1     string `json:"alfamart_free_text_1,omitempty"`
	AlfamartFreeText2     string `json:"alfamart_free_text_2,omitempty"`
	AlfamartFreeText3     string `json:"alfamart_free_text_3,omitempty"`
}

// MidtransChargeResponse represents the response from Midtrans charge API
type MidtransChargeResponse struct {
	StatusCode        string                 `json:"status_code"`
	StatusMessage     string                 `json:"status_message"`
	TransactionID     string                 `json:"transaction_id"`
	OrderID           string                 `json:"order_id"`
	GrossAmount       string                 `json:"gross_amount"`
	PaymentType       string                 `json:"payment_type"`
	TransactionTime   string                 `json:"transaction_time"`
	TransactionStatus string                 `json:"transaction_status"`
	FraudStatus       string                 `json:"fraud_status"`
	Actions           []MidtransAction       `json:"actions"`
	VANumbers         []VANumber             `json:"va_numbers,omitempty"`
	PaymentCode       string                 `json:"payment_code,omitempty"`
	PermataVANumber   string                 `json:"permata_va_number,omitempty"`
	ExpiryTime        string                 `json:"expiry_time,omitempty"`
	PaidAt            string                 `json:"paid_at,omitempty"`
	QRCode            string                 `json:"qr_code,omitempty"`
	RedirectURL       string                 `json:"redirect_url,omitempty"`
}

// MidtransAction represents Midtrans action
type MidtransAction struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	URL    string `json:"url"`
}

// VANumber represents virtual account number
type VANumber struct {
	Bank     string `json:"bank"`
	VANumber string `json:"va_number"`
}

// MidtransStatusResponse represents the response from Midtrans status API
type MidtransStatusResponse struct {
	StatusCode        string                 `json:"status_code"`
	StatusMessage     string                 `json:"status_message"`
	TransactionID     string                 `json:"transaction_id"`
	OrderID           string                 `json:"order_id"`
	GrossAmount       string                 `json:"gross_amount"`
	PaymentType       string                 `json:"payment_type"`
	TransactionTime   string                 `json:"transaction_time"`
	TransactionStatus string                 `json:"transaction_status"`
	FraudStatus       string                 `json:"fraud_status"`
	Actions           []MidtransAction       `json:"actions"`
	VANumbers         []VANumber             `json:"va_numbers,omitempty"`
	PaymentCode       string                 `json:"payment_code,omitempty"`
	PermataVANumber   string                 `json:"permata_va_number,omitempty"`
	ExpiryTime        string                 `json:"expiry_time,omitempty"`
	PaidAt            string                 `json:"paid_at,omitempty"`
}

// NewMidtransService creates a new Midtrans service
func NewMidtransService() *MidtransService {
	environment := os.Getenv("MIDTRANS_ENVIRONMENT")
	if environment == "" {
		environment = "sandbox"
	}

	var baseURL string
	var serverKey string
	var clientKey string

	if environment == "production" {
		baseURL = "https://api.midtrans.com/v2"
		serverKey = os.Getenv("MIDTRANS_SERVER_KEY_PROD")
		clientKey = os.Getenv("MIDTRANS_CLIENT_KEY_PROD")
	} else {
		baseURL = "https://api.sandbox.midtrans.com/v2"
		serverKey = os.Getenv("MIDTRANS_SERVER_KEY")
		clientKey = os.Getenv("MIDTRANS_CLIENT_KEY")
	}

	// Default sandbox keys if not provided
	if serverKey == "" {
		serverKey = "SB-Mid-server-4zIt7djwCeRdMpgF4gXDjciC"
	}
	if clientKey == "" {
		clientKey = "SB-Mid-client-4zIt7djwCeRdMpgF4gXDjciC"
	}

	// Log configuration for debugging
	fmt.Printf("🔧 Midtrans Config - Environment: %s, BaseURL: %s\n", environment, baseURL)
	fmt.Printf("🔧 Server Key: %s...\n", serverKey[:20])

	// Create optimized HTTP client with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
		DisableCompression:  false,
	}

	// Pre-compute authorization header for better performance
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(serverKey+":"))

	return &MidtransService{
		serverKey:   serverKey,
		clientKey:   clientKey,
		baseURL:     baseURL,
		environment: environment,
		authHeader:  authHeader,
		httpClient: &http.Client{
			Timeout:   60 * time.Second, // Increased timeout
			Transport: transport,
		},
	}
}

// CreatePayment creates a payment using Midtrans
func (ms *MidtransService) CreatePayment(payment *models.Payment, user *models.User, product *models.Product) (*MidtransChargeResponse, error) {
	// Map payment method to Midtrans payment type
	paymentType := string(payment.PaymentMethod)
	
	// GoPay uses "gopay" payment type directly (not qris)
	// This matches the curl example: "payment_type": "gopay"

	// Prepare charge request
	chargeReq := MidtransChargeRequest{
		PaymentType: paymentType,
		TransactionDetails: TransactionDetails{
			OrderID:     payment.OrderID,
			GrossAmount: payment.TotalAmount, // Midtrans expects amount in rupiah (not cents)
		},
		CustomerDetails: CustomerDetails{
			FirstName: user.Username,
			Email:     user.Email,
		},
		ItemDetails: []ItemDetails{
			{
				ID:       product.ID.String(),
				Price:    payment.Amount, // Amount in rupiah (Midtrans expects rupiah, not cents)
				Quantity: 1,
				Name:     product.Name,
				Category: "product",
			},
		},
	}

	// Add admin fee if exists
	if payment.AdminFee > 0 {
		chargeReq.ItemDetails = append(chargeReq.ItemDetails, ItemDetails{
			ID:       "admin_fee",
			Price:    payment.AdminFee, // Admin fee in rupiah (Midtrans expects rupiah, not cents)
			Quantity: 1,
			Name:     "Admin Fee",
			Category: "fee",
		})
	}

	// Add payment method specific details
	switch payment.PaymentMethod {
	case models.PaymentMethodBankTransfer:
		bankType := "bni" // Default to BNI instead of BCA
		if payment.BankType != nil {
			bankType = *payment.BankType
		}
		chargeReq.BankTransfer = &BankTransferDetails{
			Bank: bankType,
		}

	case models.PaymentMethodCreditCard:
		chargeReq.CreditCard = &CreditCardDetails{
			Secure:         true,
			Authentication: true,
		}

	case models.PaymentMethodGoPay:
		// GoPay implementation matches curl example
		// No additional details needed for basic GoPay payment
		chargeReq.GoPay = &GoPayDetails{
			EnableCallback: true,
			CallbackURL:    ms.getCallbackURL(),
		}

	case models.PaymentMethodQRIS:
		chargeReq.QRIS = &QRISDetails{}

	case models.PaymentMethodShopeepay:
		chargeReq.ShopeePay = &ShopeePayDetails{
			CallbackURL: ms.getCallbackURL(),
		}

	case models.PaymentMethodEchannel:
		chargeReq.Echannel = &EchannelDetails{
			BillInfo1: "Payment:",
			BillInfo2: "Online purchase",
		}

	case models.PaymentMethodPermata:
		// Permata doesn't need additional details
		// Payment type is already set to "permata"

	case models.PaymentMethodCstore:
		storeType := "alfamart" // Default to alfamart
		if payment.StoreType != nil {
			storeType = *payment.StoreType
		}
		
		if storeType == "alfamart" {
			chargeReq.Cstore = &CstoreDetails{
				Store:             "alfamart",
				Message:           "Payment for online purchase",
				AlfamartFreeText1: "1st row of receipt,",
				AlfamartFreeText2: "This is the 2nd row,",
				AlfamartFreeText3: "3rd row. The end.",
			}
		} else if storeType == "indomaret" {
			chargeReq.Cstore = &CstoreDetails{
				Store:   "indomaret",
				Message: "Message to display",
			}
		}
	}

	// Make request to Midtrans
	response, err := ms.charge(chargeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment: %w", err)
	}

	return response, nil
}

// GetPaymentStatus gets payment status from Midtrans with retry mechanism
func (ms *MidtransService) GetPaymentStatus(orderID string) (*MidtransStatusResponse, error) {
	url := fmt.Sprintf("%s/%s/status", ms.baseURL, orderID)

	// Retry mechanism with exponential backoff
	maxRetries := 3
	baseDelay := 1 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Add authorization header (pre-computed for better performance)
		req.Header.Set("Authorization", ms.authHeader)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Payment-Service/1.0")

		resp, err := ms.httpClient.Do(req)
		if err != nil {
			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to make request after %d attempts: %w", maxRetries+1, err)
			}
			
			// Exponential backoff
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
			fmt.Printf("⚠️ Status request failed (attempt %d/%d), retrying in %v: %v\n", attempt+1, maxRetries+1, delay, err)
			time.Sleep(delay)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		
		if err != nil {
			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to read response: %w", err)
			}
			
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
			fmt.Printf("⚠️ Failed to read status response (attempt %d/%d), retrying in %v: %v\n", attempt+1, maxRetries+1, delay, err)
			time.Sleep(delay)
			continue
		}

		// Handle different status codes
		if resp.StatusCode == http.StatusOK {
			var statusResp MidtransStatusResponse
			if err := json.Unmarshal(body, &statusResp); err != nil {
				return nil, fmt.Errorf("failed to unmarshal response: %w", err)
			}
			return &statusResp, nil
		}

		// Handle retryable errors (5xx and some 4xx)
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			if attempt == maxRetries {
				return nil, fmt.Errorf("Midtrans API error (Status %d): %s", resp.StatusCode, string(body))
			}
			
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
			fmt.Printf("⚠️ Status API error %d (attempt %d/%d), retrying in %v: %s\n", resp.StatusCode, attempt+1, maxRetries+1, delay, string(body))
			time.Sleep(delay)
			continue
		}

		// Non-retryable errors
		return nil, fmt.Errorf("Midtrans API error (Status %d): %s", resp.StatusCode, string(body))
	}

	return nil, fmt.Errorf("unexpected error: max retries exceeded")
}

// VerifySignature verifies Midtrans callback signature
func (ms *MidtransService) VerifySignature(orderID, statusCode, grossAmount, signatureKey string) bool {
	// Create signature string
	signatureString := orderID + statusCode + grossAmount + ms.serverKey

	// Hash with SHA512
	hash := sha512.Sum512([]byte(signatureString))
	expectedSignature := fmt.Sprintf("%x", hash)

	return signatureKey == expectedSignature
}

// MapMidtransStatusToPaymentStatus maps Midtrans status to our payment status
func (ms *MidtransService) MapMidtransStatusToPaymentStatus(midtransStatus string) models.PaymentStatus {
	switch midtransStatus {
	case "pending":
		return models.PaymentStatusPending
	case "settlement", "capture":
		return models.PaymentStatusSuccess
	case "deny":
		return models.PaymentStatusFailed
	case "cancel":
		return models.PaymentStatusCancelled
	case "expire":
		return models.PaymentStatusExpired
	default:
		return models.PaymentStatusPending
	}
}

// charge makes a charge request to Midtrans with retry mechanism
func (ms *MidtransService) charge(chargeReq MidtransChargeRequest) (*MidtransChargeResponse, error) {
	url := ms.baseURL + "/charge"

	jsonData, err := json.Marshal(chargeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Log the request for debugging
	fmt.Printf("🔍 Midtrans Request: %s\n", string(jsonData))

	// Retry mechanism with exponential backoff
	maxRetries := 3
	baseDelay := 1 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Add authorization header (pre-computed for better performance)
		req.Header.Set("Authorization", ms.authHeader)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Payment-Service/1.0")

		resp, err := ms.httpClient.Do(req)
		if err != nil {
			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to make request after %d attempts: %w", maxRetries+1, err)
			}
			
			// Exponential backoff
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
			fmt.Printf("⚠️ Request failed (attempt %d/%d), retrying in %v: %v\n", attempt+1, maxRetries+1, delay, err)
			time.Sleep(delay)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		
		if err != nil {
			if attempt == maxRetries {
				return nil, fmt.Errorf("failed to read response: %w", err)
			}
			
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
			fmt.Printf("⚠️ Failed to read response (attempt %d/%d), retrying in %v: %v\n", attempt+1, maxRetries+1, delay, err)
			time.Sleep(delay)
			continue
		}

		// Log the response for debugging
		fmt.Printf("🔍 Midtrans Response (Status %d): %s\n", resp.StatusCode, string(body))

		// Handle different status codes
		if resp.StatusCode == http.StatusOK {
			var chargeResp MidtransChargeResponse
			if err := json.Unmarshal(body, &chargeResp); err != nil {
				return nil, fmt.Errorf("failed to unmarshal response: %w", err)
			}
			
			// Log parsed response data for debugging
			fmt.Printf("🔍 Parsed Midtrans Response - PaymentCode: '%s', VANumbers: %+v, PaymentType: '%s'\n", 
				chargeResp.PaymentCode, chargeResp.VANumbers, chargeResp.PaymentType)
			
			// Check if Midtrans returned an error in the response body
			if chargeResp.StatusCode == "505" || chargeResp.StatusCode == "500" || chargeResp.StatusCode == "400" || chargeResp.StatusCode == "401" {
				return nil, fmt.Errorf("Midtrans API error (Status %s): %s", chargeResp.StatusCode, chargeResp.StatusMessage)
			}
			
			return &chargeResp, nil
		}

		// Handle retryable errors (5xx and some 4xx)
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			if attempt == maxRetries {
				return nil, fmt.Errorf("Midtrans API error (Status %d): %s", resp.StatusCode, string(body))
			}
			
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
			fmt.Printf("⚠️ API error %d (attempt %d/%d), retrying in %v: %s\n", resp.StatusCode, attempt+1, maxRetries+1, delay, string(body))
			time.Sleep(delay)
			continue
		}

		// Non-retryable errors
		return nil, fmt.Errorf("Midtrans API error (Status %d): %s", resp.StatusCode, string(body))
	}

	return nil, fmt.Errorf("unexpected error: max retries exceeded")
}

// getCallbackURL returns the callback URL for webhooks
func (ms *MidtransService) getCallbackURL() string {
	baseURL := os.Getenv("PAYMENT_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8083"
	}
	return baseURL + "/api/v1/payments/midtrans/callback"
}

// GetClientKey returns the client key for frontend
func (ms *MidtransService) GetClientKey() string {
	return ms.clientKey
}

// GetEnvironment returns the current environment
func (ms *MidtransService) GetEnvironment() string {
	return ms.environment
}
