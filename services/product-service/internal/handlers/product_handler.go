package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"product-service/internal/models"
	"product-service/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ProductHandler struct {
	repo       *repository.ProductRepository
	workerPool *WorkerPool
}

func NewProductHandler(repo *repository.ProductRepository, workerPool *WorkerPool) *ProductHandler {
	return &ProductHandler{
		repo:       repo,
		workerPool: workerPool,
	}
}

// GetProducts handles GET /api/v1/products
func (h *ProductHandler) GetProducts(c *gin.Context) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	
	// Parse query parameters
	var query models.ProductQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters", "details": err.Error()})
		return
	}
	
	// Validate and set default values
	if query.Page < 1 {
		query.Page = 1
	}
	if query.Limit < 1 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}
	
	// Create request for worker pool
	req := Request{
		ID:        uuid.New().String(),
		Type:      "get_products",
		Data:      query,
		Context:   ctx,
		Response:  make(chan Response, 1),
		Timestamp: time.Now(),
	}
	
	// Submit request to worker pool
	if err := h.workerPool.SubmitRequest(req); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable", "details": err.Error()})
		return
	}
	
	// Wait for response with timeout
	select {
	case response := <-req.Response:
		if response.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get products", "details": response.Error.Error()})
			return
		}
		
		// Type assert the response data
		products, ok := response.Data.(*models.ProductListResponse)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response format"})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    products,
			"meta": gin.H{
				"request_id": req.ID,
				"duration":   response.Duration.String(),
			},
		})
		
	case <-ctx.Done():
		c.JSON(http.StatusRequestTimeout, gin.H{"error": "Request timeout"})
		return
	}
}

// GetProductByID handles GET /api/v1/products/:id
func (h *ProductHandler) GetProductByID(c *gin.Context) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	
	// Parse product ID
	productIDStr := c.Param("id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}
	
	// Create request for worker pool
	req := Request{
		ID:        uuid.New().String(),
		Type:      "get_product_by_id",
		Data:      productID,
		Context:   ctx,
		Response:  make(chan Response, 1),
		Timestamp: time.Now(),
	}
	
	// Submit request to worker pool
	if err := h.workerPool.SubmitRequest(req); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable", "details": err.Error()})
		return
	}
	
	// Wait for response with timeout
	select {
	case response := <-req.Response:
		if response.Error != nil {
			if response.Error.Error() == "product not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get product", "details": response.Error.Error()})
			return
		}
		
		// Type assert the response data
		product, ok := response.Data.(*models.ProductResponse)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response format"})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    product,
			"meta": gin.H{
				"request_id": req.ID,
				"duration":   response.Duration.String(),
			},
		})
		
	case <-ctx.Done():
		c.JSON(http.StatusRequestTimeout, gin.H{"error": "Request timeout"})
		return
	}
}

// Health handles GET /health
func (h *ProductHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"service":   "product-service",
		"timestamp": time.Now().Unix(),
		"worker_pool": gin.H{
			"active_jobs": h.workerPool.GetActiveJobs(),
		},
	})
}

// UpdateWorkerPoolHandlers updates the worker pool handlers to use the repository
func (h *ProductHandler) UpdateWorkerPoolHandlers() {
	// Override the worker pool handlers to use the repository
	h.workerPool.handleGetProducts = h.handleGetProducts
	h.workerPool.handleGetProductByID = h.handleGetProductByID
}

// handleGetProducts processes get products requests using the repository
func (h *ProductHandler) handleGetProducts(req Request) Response {
	start := time.Now()
	
	query, ok := req.Data.(models.ProductQuery)
	if !ok {
		return Response{
			ID:       req.ID,
			Data:     nil,
			Error:    fmt.Errorf("invalid query data"),
			Duration: time.Since(start),
		}
	}
	
	products, err := h.repo.GetProducts(req.Context, query)
	if err != nil {
		return Response{
			ID:       req.ID,
			Data:     nil,
			Error:    err,
			Duration: time.Since(start),
		}
	}
	
	return Response{
		ID:       req.ID,
		Data:     products,
		Error:    nil,
		Duration: time.Since(start),
	}
}

// handleGetProductByID processes get product by ID requests using the repository
func (h *ProductHandler) handleGetProductByID(req Request) Response {
	start := time.Now()
	
	productID, ok := req.Data.(uuid.UUID)
	if !ok {
		return Response{
			ID:       req.ID,
			Data:     nil,
			Error:    fmt.Errorf("invalid product ID data"),
			Duration: time.Since(start),
		}
	}
	
	product, err := h.repo.GetProductByID(req.Context, productID)
	if err != nil {
		return Response{
			ID:       req.ID,
			Data:     nil,
			Error:    err,
			Duration: time.Since(start),
		}
	}
	
	return Response{
		ID:       req.ID,
		Data:     product,
		Error:    nil,
		Duration: time.Since(start),
	}
}
