package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"product-service/internal/cache"
	"product-service/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProductRepository struct {
	db    *gorm.DB
	cache *cache.RedisClient
}

func NewProductRepository(db *gorm.DB, cache *cache.RedisClient) *ProductRepository {
	return &ProductRepository{
		db:    db,
		cache: cache,
	}
}

// GetProducts retrieves products with pagination and caching
func (r *ProductRepository) GetProducts(ctx context.Context, query models.ProductQuery) (*models.ProductListResponse, error) {
	// Create cache key
	cacheKey := r.generateCacheKey("products", query)
	
	// Try to get from cache first
	var cachedResponse models.ProductListResponse
	if exists, _ := r.cache.Exists(ctx, cacheKey); exists {
		if err := r.cache.Get(ctx, cacheKey, &cachedResponse); err == nil {
			return &cachedResponse, nil
		}
	}
	
	// Set default values
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}
	
	// Build query
	dbQuery := r.db.WithContext(ctx).Model(&models.Product{}).Preload("User").Preload("Images")
	
	// Apply filters
	if query.Search != "" {
		dbQuery = dbQuery.Where("name ILIKE ? OR description ILIKE ?", "%"+query.Search+"%", "%"+query.Search+"%")
	}
	
	if query.MinPrice != nil {
		dbQuery = dbQuery.Where("price >= ?", *query.MinPrice)
	}
	
	if query.MaxPrice != nil {
		dbQuery = dbQuery.Where("price <= ?", *query.MaxPrice)
	}
	
	if query.IsActive != nil {
		dbQuery = dbQuery.Where("is_active = ?", *query.IsActive)
	}
	
	// Get total count
	var total int64
	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count products: %w", err)
	}
	
	// Apply pagination using keyset pagination for better performance
	var products []models.Product
	var hasMore bool
	var nextCursor string
	
	if query.Cursor != "" {
		// Keyset pagination: WHERE id > cursor
		cursorID, err := uuid.Parse(query.Cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", err)
		}
		dbQuery = dbQuery.Where("id > ?", cursorID)
	}
	
	// Order by ID for consistent pagination
	dbQuery = dbQuery.Order("id ASC")
	
	// Get one extra record to check if there are more
	limit := query.Limit + 1
	if err := dbQuery.Limit(limit).Find(&products).Error; err != nil {
		return nil, fmt.Errorf("failed to get products: %w", err)
	}
	
	// Check if there are more records
	if len(products) > query.Limit {
		hasMore = true
		products = products[:query.Limit] // Remove the extra record
		nextCursor = products[len(products)-1].ID.String()
	}
	
	// Convert to response format
	productResponses := make([]models.ProductResponse, len(products))
	for i, product := range products {
		productResponses[i] = product.ToResponse()
	}
	
	response := &models.ProductListResponse{
		Products:   productResponses,
		Total:      total,
		Page:       query.Page,
		Limit:      query.Limit,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}
	
	// Cache the response for 5 minutes
	if err := r.cache.Set(ctx, cacheKey, response, 5*time.Minute); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to cache products: %v\n", err)
	}
	
	return response, nil
}

// GetProductByID retrieves a single product by ID with caching
func (r *ProductRepository) GetProductByID(ctx context.Context, id uuid.UUID) (*models.ProductResponse, error) {
	// Create cache key
	cacheKey := fmt.Sprintf("product:%s", id.String())
	
	// Try to get from cache first
	var cachedProduct models.ProductResponse
	if exists, _ := r.cache.Exists(ctx, cacheKey); exists {
		if err := r.cache.Get(ctx, cacheKey, &cachedProduct); err == nil {
			return &cachedProduct, nil
		}
	}
	
	// Get from database
	var product models.Product
	if err := r.db.WithContext(ctx).Preload("User").Preload("Images").First(&product, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("product not found")
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}
	
	response := product.ToResponse()
	
	// Cache the response for 10 minutes
	if err := r.cache.Set(ctx, cacheKey, response, 10*time.Minute); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to cache product: %v\n", err)
	}
	
	return &response, nil
}

// InvalidateProductCache invalidates cache for a specific product
func (r *ProductRepository) InvalidateProductCache(ctx context.Context, productID uuid.UUID) error {
	cacheKey := fmt.Sprintf("product:%s", productID.String())
	return r.cache.Delete(ctx, cacheKey)
}

// InvalidateProductsCache invalidates the products list cache
func (r *ProductRepository) InvalidateProductsCache(ctx context.Context) error {
	return r.cache.DeletePattern(ctx, "products:*")
}

// generateCacheKey generates a cache key for products list
func (r *ProductRepository) generateCacheKey(prefix string, query models.ProductQuery) string {
	key := prefix
	
	if query.Page > 0 {
		key += fmt.Sprintf(":page:%d", query.Page)
	}
	
	if query.Limit > 0 {
		key += fmt.Sprintf(":limit:%d", query.Limit)
	}
	
	if query.Cursor != "" {
		key += fmt.Sprintf(":cursor:%s", query.Cursor)
	}
	
	if query.Search != "" {
		key += fmt.Sprintf(":search:%s", query.Search)
	}
	
	if query.MinPrice != nil {
		key += fmt.Sprintf(":min_price:%s", strconv.FormatFloat(*query.MinPrice, 'f', 2, 64))
	}
	
	if query.MaxPrice != nil {
		key += fmt.Sprintf(":max_price:%s", strconv.FormatFloat(*query.MaxPrice, 'f', 2, 64))
	}
	
	if query.IsActive != nil {
		key += fmt.Sprintf(":is_active:%t", *query.IsActive)
	}
	
	return key
}

// CreateProduct creates a new product (for future use)
func (r *ProductRepository) CreateProduct(ctx context.Context, product *models.Product) error {
	if err := r.db.WithContext(ctx).Create(product).Error; err != nil {
		return fmt.Errorf("failed to create product: %w", err)
	}
	
	// Invalidate products cache
	r.InvalidateProductsCache(ctx)
	
	return nil
}

// UpdateProduct updates an existing product (for future use)
func (r *ProductRepository) UpdateProduct(ctx context.Context, product *models.Product) error {
	if err := r.db.WithContext(ctx).Save(product).Error; err != nil {
		return fmt.Errorf("failed to update product: %w", err)
	}
	
	// Invalidate caches
	r.InvalidateProductCache(ctx, product.ID)
	r.InvalidateProductsCache(ctx)
	
	return nil
}

// DeleteProduct deletes a product (for future use)
func (r *ProductRepository) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Delete(&models.Product{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}
	
	// Invalidate caches
	r.InvalidateProductCache(ctx, id)
	r.InvalidateProductsCache(ctx)
	
	return nil
}
