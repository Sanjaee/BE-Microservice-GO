package main

import (
	"fmt"
	"log"
	"os"

	"product-service/internal/models"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env file not found, using system env")
	}

	// Get database configuration from environment
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}

	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		dbPass = "123"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "productdb"
	}

	// Connection string
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		dbHost, dbUser, dbPass, dbName, dbPort,
	)

	// Connect to database using GORM
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("❌ Failed to get generic DB: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("❌ Database not responding: %v", err)
	}

	// Auto-migrate the database
	if err := db.AutoMigrate(&models.Product{}, &models.ProductImage{}, &models.User{}); err != nil {
		log.Fatalf("❌ Failed to migrate database: %v", err)
	}

	log.Println("✅ Database connected and migrated successfully!")

	// Create sample users if they don't exist
	var userCount int64
	db.Model(&models.User{}).Count(&userCount)
	
	if userCount == 0 {
		log.Println("👥 Creating sample users...")
		
		// Create more sample users for realistic data
		users := []models.User{
			{
				ID:       uuid.New(),
				Username: "john_doe",
				Email:    "john@example.com",
			},
			{
				ID:       uuid.New(),
				Username: "jane_smith",
				Email:    "jane@example.com",
			},
			{
				ID:       uuid.New(),
				Username: "mike_wilson",
				Email:    "mike@example.com",
			},
			{
				ID:       uuid.New(),
				Username: "sarah_jones",
				Email:    "sarah@example.com",
			},
			{
				ID:       uuid.New(),
				Username: "david_brown",
				Email:    "david@example.com",
			},
			{
				ID:       uuid.New(),
				Username: "lisa_garcia",
				Email:    "lisa@example.com",
			},
			{
				ID:       uuid.New(),
				Username: "alex_miller",
				Email:    "alex@example.com",
			},
			{
				ID:       uuid.New(),
				Username: "emma_davis",
				Email:    "emma@example.com",
			},
			{
				ID:       uuid.New(),
				Username: "ryan_taylor",
				Email:    "ryan@example.com",
			},
			{
				ID:       uuid.New(),
				Username: "olivia_anderson",
				Email:    "olivia@example.com",
			},
		}

		for _, user := range users {
			if err := db.Create(&user).Error; err != nil {
				log.Printf("Failed to create user %s: %v", user.Username, err)
			} else {
				log.Printf("Created user: %s", user.Username)
			}
		}
		
		log.Printf("✅ Successfully created %d users!", len(users))
	}

	// Get users for product creation
	var users []models.User
	if err := db.Find(&users).Error; err != nil {
		log.Fatal("Failed to get users:", err)
	}

	if len(users) == 0 {
		log.Fatal("No users found. Please create users first.")
	}

	// Create sample products
	var productCount int64
	db.Model(&models.Product{}).Count(&productCount)

	if productCount == 0 {
		log.Println("🌱 Creating 1000 dummy products...")
		
		// Product categories and their data
		categories := []struct {
			name        string
			description string
			priceRange  [2]float64
			stockRange  [2]int
			images      []string
		}{
			{
				name:        "Nike Basketball Shoes",
				description: "High-performance basketball shoes with advanced cushioning technology. Perfect for professional and amateur players.",
				priceRange:  [2]float64{800000, 2500000},
				stockRange:  [2]int{5, 50},
				images: []string{
					"https://static.nike.com/a/images/c_limit,w_592,f_auto/t_product_v1/9cc5599c-1dc9-4bb9-af93-94b5ddc6ae2d/LEBRON+XXIII+PVD+EP.png",
					"https://static.nike.com/a/images/c_limit,w_592,f_auto/t_product_v1/4f37fca8-6bce-43c7-925c-0e2aacd3de3a/air-jordan-1-retro-high-og-shoes-Pz6fT9.png",
					"https://static.nike.com/a/images/c_limit,w_592,f_auto/t_product_v1/8b0b3b3b-3b3b-3b3b-3b3b-3b3b3b3b3b3b/kyrie-7-ep-shoes-2Xqg6h.png",
				},
			},
			{
				name:        "Adidas Running Shoes",
				description: "Lightweight running shoes with responsive Boost technology. Ideal for long-distance running and daily training.",
				priceRange:  [2]float64{600000, 1800000},
				stockRange:  [2]int{10, 60},
				images: []string{
					"https://assets.adidas.com/images/h_840,f_auto,q_auto,fl_lossy,c_fill,g_auto/fbaf991a78bc4896a3e9ad7800abcec6_9366/Ultraboost_22_Shoes_Black_GZ0127_01_standard.jpg",
					"https://assets.adidas.com/images/h_840,f_auto,q_auto,fl_lossy,c_fill,g_auto/2c5b8b8b8b8b8b8b8b8b8b8b8b8b8b8b_9366/Ultraboost_22_Shoes_White_GZ0127_02_standard.jpg",
					"https://assets.adidas.com/images/h_840,f_auto,q_auto,fl_lossy,c_fill,g_auto/3d6c9c9c9c9c9c9c9c9c9c9c9c9c9c9c_9366/Ultraboost_22_Shoes_Blue_GZ0127_03_standard.jpg",
				},
			},
			{
				name:        "Cotton T-Shirt",
				description: "Comfortable cotton t-shirt made from 100% organic cotton. Perfect for everyday wear and casual occasions.",
				priceRange:  [2]float64{50000, 200000},
				stockRange:  [2]int{20, 100},
				images: []string{
					"https://images.unsplash.com/photo-1521572163474-6864f9cf17ab?w=500",
					"https://images.unsplash.com/photo-1503341504253-dff4815485f1?w=500",
					"https://images.unsplash.com/photo-1576566588028-4147f3842f27?w=500",
				},
			},
			{
				name:        "Denim Jeans",
				description: "Classic blue denim jeans with a comfortable fit. Made from premium denim fabric with modern styling.",
				priceRange:  [2]float64{200000, 500000},
				stockRange:  [2]int{15, 80},
				images: []string{
					"https://images.unsplash.com/photo-1542272604-787c3835535d?w=500",
					"https://images.unsplash.com/photo-1594633312681-425c7b97ccd1?w=500",
					"https://images.unsplash.com/photo-1541099649105-f69ad21f3246?w=500",
				},
			},
			{
				name:        "Leather Jacket",
				description: "Premium leather jacket with a modern design. Made from genuine leather with excellent craftsmanship.",
				priceRange:  [2]float64{800000, 2000000},
				stockRange:  [2]int{5, 25},
				images: []string{
					"https://images.unsplash.com/photo-1551028719-00167b16eac5?w=500",
					"https://images.unsplash.com/photo-1551698618-1dfe5d97d256?w=500",
					"https://images.unsplash.com/photo-1544022613-e87ca75a784a?w=500",
				},
			},
			{
				name:        "Summer Dress",
				description: "Light and breezy summer dress perfect for warm weather. Made from high-quality fabric with elegant design.",
				priceRange:  [2]float64{300000, 800000},
				stockRange:  [2]int{10, 50},
				images: []string{
					"https://images.unsplash.com/photo-1595777457583-95e059d581b8?w=500",
					"https://images.unsplash.com/photo-1515372039744-b8f02a3ae446?w=500",
					"https://images.unsplash.com/photo-1566479179817-c0d9ed0b5b10?w=500",
				},
			},
			{
				name:        "Winter Coat",
				description: "Warm winter coat with premium insulation. Perfect for cold weather protection with stylish design.",
				priceRange:  [2]float64{600000, 1500000},
				stockRange:  [2]int{8, 30},
				images: []string{
					"https://images.unsplash.com/photo-1578662996442-48f60103fc96?w=500",
					"https://images.unsplash.com/photo-1544022613-e87ca75a784a?w=500",
					"https://images.unsplash.com/photo-1551698618-1dfe5d97d256?w=500",
				},
			},
			{
				name:        "Baseball Cap",
				description: "Classic baseball cap with adjustable strap. Great for outdoor activities and casual wear.",
				priceRange:  [2]float64{80000, 200000},
				stockRange:  [2]int{25, 100},
				images: []string{
					"https://images.unsplash.com/photo-1588850561407-ed78c282e89b?w=500",
					"https://images.unsplash.com/photo-1521369909029-2afed882baee?w=500",
					"https://images.unsplash.com/photo-1583394838336-acd977736f90?w=500",
				},
			},
			{
				name:        "Handbag",
				description: "Elegant handbag made from genuine leather. Perfect for daily use with multiple compartments.",
				priceRange:  [2]float64{400000, 1200000},
				stockRange:  [2]int{5, 40},
				images: []string{
					"https://images.unsplash.com/photo-1553062407-98eeb64c6a62?w=500",
					"https://images.unsplash.com/photo-1584917865442-de89df76afd3?w=500",
					"https://images.unsplash.com/photo-1553062407-98eeb64c6a62?w=500",
				},
			},
			{
				name:        "Sunglasses",
				description: "Stylish sunglasses with UV protection. Perfect for sunny days with modern frame design.",
				priceRange:  [2]float64{150000, 500000},
				stockRange:  [2]int{20, 80},
				images: []string{
					"https://images.unsplash.com/photo-1511499767150-a48a237f0083?w=500",
					"https://images.unsplash.com/photo-1572635196237-14b3f281503f?w=500",
					"https://images.unsplash.com/photo-1574258495973-f010dfbb5371?w=500",
				},
			},
			{
				name:        "Wristwatch",
				description: "Classic wristwatch with leather strap. Elegant design for any occasion with precise movement.",
				priceRange:  [2]float64{500000, 2000000},
				stockRange:  [2]int{3, 25},
				images: []string{
					"https://images.unsplash.com/photo-1523275335684-37898b6baf30?w=500",
					"https://images.unsplash.com/photo-1524592094714-0f0654e20314?w=500",
					"https://images.unsplash.com/photo-1523170335258-f5c6c6b6b6b6?w=500",
				},
			},
		}

		// Colors and sizes for variation
		colors := []string{"Black", "White", "Blue", "Red", "Green", "Yellow", "Purple", "Orange", "Pink", "Gray"}
		sizes := []string{"XS", "S", "M", "L", "XL", "XXL", "28", "30", "32", "34", "36", "38", "40", "42"}

		// Create 1000 products
		for i := 0; i < 1000; i++ {
			category := categories[i%len(categories)]
			color := colors[i%len(colors)]
			size := sizes[i%len(sizes)]
			
			// Generate random price within range
			priceRange := category.priceRange[1] - category.priceRange[0]
			price := category.priceRange[0] + float64(i%int(priceRange))
			
			// Generate random stock within range
			stockRange := category.stockRange[1] - category.stockRange[0]
			stock := category.stockRange[0] + (i % stockRange)
			
			// Create product name with variation
			productName := fmt.Sprintf("%s %s %s", color, category.name, size)
			
			// Create product description with variation
			productDescription := fmt.Sprintf("%s Available in %s color and %s size. %s", 
				category.description, color, size, 
				"Premium quality materials with excellent craftsmanship and modern design.")
			
			// Select random user
			user := users[i%len(users)]
			
			// Create product with images
			product := models.Product{
				ID:          uuid.New(),
				UserID:      user.ID,
				Name:        productName,
				Description: productDescription,
				Price:       price,
				Stock:       stock,
				IsActive:    true,
				Images:      []models.ProductImage{},
			}
			
			// Add multiple images for each product
			for j, imageUrl := range category.images {
				product.Images = append(product.Images, models.ProductImage{
					ID:       uuid.New(),
					ImageUrl: imageUrl,
				})
				
				// Add some variation to image URLs for more diversity
				if j > 0 {
					// Add query parameters to make images unique
					product.Images[j].ImageUrl = fmt.Sprintf("%s?v=%d&color=%s", imageUrl, i, color)
				}
			}
			
			// Create product in database
			if err := db.Create(&product).Error; err != nil {
				log.Printf("Failed to create product %s: %v", product.Name, err)
			} else {
				if (i+1)%100 == 0 {
					log.Printf("Created %d products...", i+1)
				}
			}
		}
		
		log.Printf("✅ Successfully created 1000 products!")
	}

	log.Println("Database seeding completed successfully!")
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
