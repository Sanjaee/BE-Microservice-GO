package services

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"payment-service/internal/models"
)

// MidtransService handles Midtrans payment operations
type MidtransService struct {
	serverKey    string
	clientKey    string
	baseURL      string
	httpClient   *http.Client
	environment  string
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

	return &MidtransService{
		serverKey:   serverKey,
		clientKey:   clientKey,
		baseURL:     baseURL,
		environment: environment,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreatePayment creates a payment using Midtrans
func (ms *MidtransService) CreatePayment(payment *models.Payment, user *models.User, product *models.Product) (*MidtransChargeResponse, error) {
	// Prepare charge request
	chargeReq := MidtransChargeRequest{
		PaymentType: string(payment.PaymentMethod),
		TransactionDetails: TransactionDetails{
			OrderID:     payment.OrderID,
			GrossAmount: payment.TotalAmount,
		},
		CustomerDetails: CustomerDetails{
			FirstName: user.Username,
			Email:     user.Email,
		},
		ItemDetails: []ItemDetails{
			{
				ID:       product.ID.String(),
				Price:    payment.Amount,
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
			Price:    payment.AdminFee,
			Quantity: 1,
			Name:     "Admin Fee",
			Category: "fee",
		})
	}

	// Add payment method specific details
	switch payment.PaymentMethod {
	case models.PaymentMethodBankTransfer:
		bankType := "bca"
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
	}

	// Make request to Midtrans
	response, err := ms.charge(chargeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment: %w", err)
	}

	return response, nil
}

// GetPaymentStatus gets payment status from Midtrans
func (ms *MidtransService) GetPaymentStatus(orderID string) (*MidtransStatusResponse, error) {
	url := fmt.Sprintf("%s/%s/status", ms.baseURL, orderID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	auth := base64.StdEncoding.EncodeToString([]byte(ms.serverKey + ":"))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ms.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Midtrans API error: %s", string(body))
	}

	var statusResp MidtransStatusResponse
	if err := json.Unmarshal(body, &statusResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &statusResp, nil
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

// charge makes a charge request to Midtrans
func (ms *MidtransService) charge(chargeReq MidtransChargeRequest) (*MidtransChargeResponse, error) {
	url := ms.baseURL + "/charge"

	jsonData, err := json.Marshal(chargeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	auth := base64.StdEncoding.EncodeToString([]byte(ms.serverKey + ":"))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ms.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Midtrans API error: %s", string(body))
	}

	var chargeResp MidtransChargeResponse
	if err := json.Unmarshal(body, &chargeResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &chargeResp, nil
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
