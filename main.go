package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"go-saas/controllers"
	"go-saas/middleware"
	"go-saas/models"
	"go-saas/routes"
	"go-saas/services"
	"go-saas/tasks"
	"go-saas/utils"
	"log"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time" // Import package vừa tạo

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
)

type Domain struct {
	TenantID string `json:"tenant_id"`
}

type TenantPool struct {
	DB         *sql.DB
	LastActive time.Time
}

var (
	domainTenantIDCache = make(map[string]string)
	tenantRegistry      = make(map[string]*TenantPool)
	mu                  sync.RWMutex
	centralDB           *sql.DB
	allowedTables       = map[string]bool{
		"categories":          true,
		"blogs":               true,
		"users":               true,
		"pages":               true,
		"customers":           true,
		"settings":            true,
		"products":            true,
		"attributes":          true,
		"options":             true,
		"coupons":             true,
		"orders":              true,
		"languages":           true,
		"currencies":          true,
		"countries":           true,
		"states":              true,
		"districts":           true,
		"tax_classes":         true,
		"tax_rates":           true,
		"zones":               true,
		"blog_categories":     true,
		"filters":             true,
		"inventories":         true,
		"inventory_adjusts":   true,
		"inventory_histories": true,
		"newsletters":         true,
		"payment_methods":     true,
		"plugins":             true,
		"purchases":           true,
		"purchase_returns":    true,
		"reviews":             true,
		"roles":               true,
		"transactions":        true,
		"warehouses":          true,
		"suppliers":           true,
		"transfers":           true,
		"adjustments":         true,
		"expenses":            true,
	}
)

var (
	// Định nghĩa các field được phép cho Create/Update theo từng table
	tableSchemas = map[string][]string{
		"activity_log":          {"log_name", "description", "subject_id", "subject_type", "causer_id", "causer_type", "properties"},
		"addons":                {"name", "status"},
		"address_types":         {"type"},
		"addresses":             {"type", "address_title", "address_line_1", "address_line_2", "city", "state_id", "zip_code", "country_id", "phone", "latitude", "longitude", "addressable_id", "addressable_type", "is_default"},
		"adjustment_items":      {"adjustment_id", "product_id", "quantity", "method_type"},
		"adjustments":           {"date", "reference_code", "warehouse_id", "total_products"},
		"announcements":         {"user_id", "body", "action_text", "action_url"},
		"attachments":           {"path", "name", "extension", "size", "attachable_id", "attachable_type"},
		"attribute_category":    {"category_id", "attribute_id"},
		"attribute_product":     {"attribute_id", "product_id", "attribute_value_id"},
		"attribute_types":       {"type"},
		"attribute_values":      {"name", "color", "attribute_id", "sort_order"},
		"attributes":            {"name", "attribute_type_id", "order", "filterable"},
		"banner_groups":         {"name"},
		"banners":               {"title", "description", "link", "link_label", "bg_color", "group_id", "columns", "order"},
		"block_templates":       {"name", "setting", "image", "category_id"},
		"blocks":                {"name", "block_template_id", "data", "template_name", "theme_id", "setting"},
		"blog_blog_category":    {"blog_category_id", "blog_id"},
		"blog_categories":       {"parent_id", "name", "slug", "description", "active", "featured", "left", "right"},
		"blog_comments":         {"content", "blog_id", "user_id", "parent", "approved", "likes", "dislikes"},
		"blogs":                 {"title", "slug", "excerpt", "content", "user_id", "status", "approved", "published_at", "likes", "dislikes"},
		"brands":                {"title", "description", "link", "order"},
		"builders":              {"data", "code"},
		"carriers":              {"tax_id", "name", "email", "phone", "tracking_url", "active", "setting", "description", "code"},
		"cart_items":            {"cart_id", "product_id", "item_description", "quantity", "unit_price", "option"},
		"carts":                 {"customer_id", "ip_address", "shipping_country_id", "shipping_zone_id", "shipping_rate_id", "packaging_id", "item_count", "quantity", "total", "discount", "shipping", "packaging", "handling", "taxes", "grand_total", "taxrate", "shipping_weight", "billing_address", "shipping_address", "coupon_id", "payment_status", "payment_method_id", "message_to_customer", "admin_note", "shipping_method_code", "shipping_state_id"},
		"categories":            {"parent_id", "name", "slug", "description", "active", "featured", "left", "right"},
		"category_filter":       {"category_id", "filter_id"},
		"category_product":      {"category_id", "product_id"},
		"cities":                {"name", "type"},
		"compares":              {"product_id", "customer_id"},
		"contact_us":            {"name", "phone", "email", "subject", "message", "read"},
		"countries":             {"capital", "citizenship", "country_code", "currency", "currency_code", "currency_sub_unit", "currency_symbol", "full_name", "iso_3166_2", "iso_3166_3", "name", "region_code", "sub_region_code", "eea", "calling_code", "flag", "active"},
		"coupon_customer":       {"coupon_id", "customer_id"},
		"coupons":               {"name", "code", "description", "value", "min_order_amount", "type", "quantity", "quantity_per_customer", "starting_time", "ending_time", "active"},
		"currencies":            {"priority", "iso_code", "name", "symbol", "symbol_first", "decimal_mark", "thousands_separator", "active", "exchange_rate"},
		"customer_histories":    {"customer_id", "comment"},
		"customer_ips":          {"customer_id", "ip"},
		"customer_transactions": {"customer_id", "order_id", "description", "amount"},
		"customers":             {"name", "nice_name", "email", "password", "dob", "sex", "description", "last_visited_at", "last_visited_from", "stripe_id", "card_holder_name", "card_brand", "card_last_four", "active", "accepts_marketing", "verification_token", "remember_token", "phone"},
		"dashboard_configs":     {"upgrade_plan_notice"},
		"dispute_types":         {"detail"},
		"disputes":              {"dispute_type_id", "customer_id", "order_id", "product_id", "description", "order_received", "return_goods", "refund_amount", "status"},
		"products":              {"slug", "title", "description", "sale_price", "category_id", "id", "brand_id", "title", "model_number", "mpn", "gtin", "gtin_type", "description", "origin_country", "has_variant", "requires_shipping", "downloadable", "warehouse_id", "supplier_id", "sku", "condition", "condition_note", "key_features", "stock_quantity", "damaged_quantity", "user_id", "purchase_price", "sale_price", "offer_price", "offer_start", "offer_end", "shipping_weight", "free_shipping", "available_from", "min_order_quantity", "linked_items", "stuff_pick", "slug", "meta_title", "meta_description", "sale_count", "active", "rating", "video", "short_description", "is_new", "is_hot", "link_video", "tax_class_id", "view", "reward_point", "try_on_status", "try_on_provider_id"},
		"purchase_details":      {"product_id", "quantity", "purchase_price", "sku", "purchase_id"},
		"purchase_items":        {"purchase_id", "product_id", "product_cost", "net_unit_cost", "tax_type", "tax_value", "tax_amount", "discount_type", "discount_value", "discount_amount", "purchase_unit", "quantity", "sub_total"},
		"purchase_returns":      {"date", "supplier_id", "warehouse_id", "tax_rate", "tax_amount", "discount", "shipping", "grand_total", "received_amount", "paid_amount", "payment_type", "status", "payment_status", "notes", "reference_code"},
		"purchases":             {"purchases_number", "warehouse_id", "supplier_id", "payment_status", "stock_status", "note", "total", "debt", "total_paid", "status", "date", "tax_rate", "tax_amount", "shipping", "discount"},
		"refunds":               {"order_id", "order_fulfilled", "amount", "description", "status"},
		"replies":               {"reply", "user_id", "customer_id", "read", "repliable_id", "repliable_type"},
		"reviews":               {"customer_id", "rating", "comment", "product_id", "approved", "spam"},
		"reward_points":         {"customer_id", "order_id", "total", "status"},
		"roles":                 {"name", "guard_name", "level", "description"},
		"suppliers":             {"name", "email", "contact_person", "url", "description"},
		"transfers":             {"date", "id", "date", "from_warehouse_id", "to_warehouse_id", "tax_rate", "tax_amount", "discount", "shipping", "grand_total", "status", "note", "reference_code"},
		"expenses":              {"title", "amount", "category", "description", "expense_date"},
	}
)

const (
	redisTenantIDKey = "tenant_id:"
)

func filterAllowedFields(tableName string, rawData map[string]interface{}) map[string]interface{} {
	allowed, exists := tableSchemas[tableName]
	if !exists {
		return nil // Hoặc trả về rỗng nếu bảng chưa được định nghĩa schema
	}

	filtered := make(map[string]interface{})
	for _, field := range allowed {
		if val, ok := rawData[field]; ok {
			filtered[field] = val
		}
	}
	return filtered
}

/*
	func init() {
		fmt.Println("--> Đang khởi tạo Redis...")

		utils.InitRedis()

		// 2. Load ENV & Khởi tạo Central MySQL DB
		err := godotenv.Load()
		if err != nil {
			log.Println("Lưu ý: Không tìm thấy file .env, dùng biến môi trường hệ thống")
		}

		dbUser := os.Getenv("DB_USER")
		dbPass := os.Getenv("DB_PASS")
		dbHost := os.Getenv("DB_HOST")
		dbPort := os.Getenv("DB_PORT")
		dbName := os.Getenv("DB_NAME")

		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			dbUser, dbPass, dbHost, dbPort, dbName,
		)

		centralDB, err = sql.Open("mysql", dsn)
		if err != nil {
			log.Fatalf("Error opening central database: %v", err)
		}

		err = centralDB.Ping()
		if err != nil {
			log.Fatalf("Error pinging central database: %v (DSN: %s)", err, dsn)
		}

		go cleanupIdlePools()
	}
*/
func init() {
	fmt.Println("--> Đang khởi tạo Redis...")
	utils.InitRedis()

	err := godotenv.Load()
	if err != nil {
		log.Println("Lưu ý: Không tìm thấy file .env, dùng biến môi trường hệ thống")
	}

	// Khởi tạo Central DB từ package utils
	utils.InitCentralDB()
}

func AuthAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var rawToken string

		// 1. Ưu tiên lấy Token từ Header Authorization
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			rawToken = strings.TrimPrefix(authHeader, "Bearer ")
		}

		// 2. Nếu Header rỗng, lấy từ Query Param "?token=" (Dành cho SSE EventSource)
		if rawToken == "" {
			rawToken = c.Query("token")
		}

		if rawToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		// 3. Decode URL (Chuyển %7C thành dấu | nếu token nằm trên Query String)
		decodedToken, err := url.QueryUnescape(rawToken)
		if err == nil {
			rawToken = decodedToken
		}

		// 4. Parse token dạng "ID|PlainToken" (Laravel Sanctum format)
		parts := strings.SplitN(rawToken, "|", 2)
		if len(parts) < 2 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token format"})
			c.Abort()
			return
		}

		tokenID := parts[0]
		plainToken := parts[1]
		hashedToken := utils.HashToken(plainToken)

		tenantDB, err := utils.GetDBFromContext(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection error"})
			c.Abort()
			return
		}

		var adminID uint64
		var tokenableType string

		// Chỉ cho phép lọc các tokenable_type hợp lệ của Admin/Super Admin
		query := `
			SELECT tokenable_type, tokenable_id 
			FROM personal_access_tokens 
			WHERE id = ? AND token = ? 
			  AND tokenable_type IN (
				'App\\Models\\Tenant\\User', 
				'App\\Models\\Central\\Client'
			  )
			LIMIT 1`

		//args := []interface{}{tokenID, hashedToken}
		//utils.LogSQL(query, args...)

		err = tenantDB.QueryRowContext(c, query, tokenID, hashedToken).Scan(&tokenableType, &adminID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired Admin Token"})
			c.Abort()
			return
		}

		// Xác định Super Admin dựa trên type đã lấy từ DB
		isSuperAdmin := (tokenableType == "App\\Models\\Central\\Client")

		// Lưu thông tin xác thực vào Context
		c.Set("adminID", adminID)
		c.Set("isSuperAdmin", isSuperAdmin)
		c.Set("notifiableType", tokenableType)

		c.Next()
	}
}

func AuthCustomerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing token"})
			c.Abort()
			return
		}

		// 1. Lấy token thô: "22|KU1SWj..."
		rawToken := strings.TrimPrefix(authHeader, "Bearer ")

		// 2. Tách ID và Plain Token
		parts := strings.SplitN(rawToken, "|", 2)
		if len(parts) < 2 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token format"})
			c.Abort()
			return
		}

		tokenID := parts[0]    // "22"
		plainToken := parts[1] // "KU1SWj..."

		// 3. Hash plainToken để so sánh với DB
		hashedToken := utils.HashToken(plainToken)

		// 4. Truy vấn Database
		tenantDB, _ := getDBFromContext(c)
		var adminID int

		// Query kiểm tra ID và Hash Token
		query := `
            SELECT tokenable_id 
            FROM personal_access_tokens 
            WHERE id = ? AND token = ? AND tokenable_type = 'App\\Models\\Tenant\\Customer' 
            LIMIT 1`
		args := []interface{}{tokenID, hashedToken}
		utils.LogSQL(query, args...)
		err := tenantDB.QueryRow(query, args...).Scan(&adminID)
		if err != nil {
			errorMessage := fmt.Sprintf("!!! [SQL ERROR]: %v | QUERY: %s", err, query)
			logSQL(errorMessage)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token expired or invalid"})
			c.Abort()
			return
		}
		utils.LogSQL(fmt.Sprintf("DEBUG: adminID nhận được là %d", adminID))

		// 5. Lưu customerID vào context
		c.Set("customerID", adminID)
		c.Next()
	}
}

func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Mặc định luôn set customerID = 0 (Guest)
		c.Set("customerID", 0)

		authHeader := c.GetHeader("Authorization")

		// 1. Kiểm tra Header
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.Next()
			return // Bắt buộc return ngay
		}

		// 2. Tách token
		rawToken := strings.TrimPrefix(authHeader, "Bearer ")
		parts := strings.SplitN(rawToken, "|", 2)

		if len(parts) < 2 {
			c.Next()
			return // Bắt buộc return ngay
		}

		tokenID := parts[0]
		plainToken := parts[1]
		hashedToken := utils.HashToken(plainToken)

		// 3. Lấy DB kết nối Tenant
		tenantDB, err := utils.GetDBFromContext(c)
		if err != nil {
			utils.LogToFile("OptionalAuthMiddleware: Không lấy được tenantDB: %v", err)
			c.Next()
			return // Bắt buộc return ngay
		}

		// 4. Query tìm Customer
		var customerID int
		query := `
			SELECT tokenable_id 
			FROM personal_access_tokens 
			WHERE id = ? AND token = ? AND tokenable_type = 'App\\Models\\Tenant\\Customer' 
			LIMIT 1`

		err = tenantDB.QueryRow(query, tokenID, hashedToken).Scan(&customerID)
		if err != nil {
			// Nếu token hỏng/hết hạn -> Ghi log nhẹ để debug nếu cần, vẫn cho qua dạng Guest
			utils.LogToFile("OptionalAuthMiddleware: Token không hợp lệ hoặc hết hạn (ID: %s): %v", tokenID, err)
		} else {
			// Gán ID thật vào Context
			c.Set("customerID", customerID)
		}
		//	utils.LogToFile("finish OptionalAuthMiddleware")
		// CHỈ GỌI c.Next() ĐÚNG 1 LẦN Ở CUỐI CHO NHÁNH CHẠY TỚI ĐÂY
		c.Next()
	}
}

func main() {

	logFile, err := os.OpenFile("C:/promtail/logs/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Không thể mở file log: %v", err)
	}
	defer logFile.Close()

	// 2. Cấu hình slog ghi định dạng JSON ra file này
	logger := slog.New(slog.NewJSONHandler(logFile, nil))
	slog.SetDefault(logger)

	defer utils.CentralDB.Close()

	redisOpt := asynq.RedisClientOpt{
		Addr: "127.0.0.1:6379",
	}

	utils.AsynqClient = asynq.NewClient(redisOpt)
	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: 10,
			LogLevel:    asynq.ErrorLevel,
			Queues: map[string]int{
				"emails":  6,
				"default": 3,
			},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeSendDynamicEmail, tasks.HandleDynamicEmailTask)

	// 💡 Đăng ký Handler mới cho Abandoned Cart
	mux.HandleFunc(tasks.TypeAbandonedCartRecovery, tasks.HandleAbandonedCartTask)

	go func() {
		if err := srv.Run(mux); err != nil {
			log.Printf("[Asynq Worker CRASHED] Không thể chạy Worker Server: %v", err)
		}
	}()

	gin.SetMode(gin.DebugMode)
	r := gin.Default()

	// Sử dụng middleware từ package utils
	r.Use(utils.TenantMiddleware())
	r.Use(middleware.JSONRecoveryMiddleware())

	apiV2 := r.Group("/api/v2")
	{
		authGroup := apiV2.Group("/auth")
		authGroup.POST("/login", controllers.AdminLoginHandler)
		// --- CUSTOMER AREA ---
		customerGroup := apiV2.Group("/customer")
		customerGroup.POST("/login", controllers.CustomerLoginHandler)
		customerGroup.POST("/register", controllers.CustomerRegisterHandler)

		protectedCustomerGroup := customerGroup.Group("/")
		protectedCustomerGroup.Use(AuthCustomerMiddleware()) // App\Models\Customer
		{

			protectedCustomerGroup.POST("/logout", controllers.LogoutCustomer)
			protectedCustomerGroup.GET("/profile", customerProfile)
			protectedCustomerGroup.GET("/orders", customerOrder)
			protectedCustomerGroup.GET("/order/:id", controllers.CustomerOrderDetail)
			protectedCustomerGroup.GET("/cart/customer/load", cartByCustomer)
			protectedCustomerGroup.GET("/addresses", controllers.ListCustomerAddresses)
			protectedCustomerGroup.POST("/address/create", controllers.StoreAddress)
			protectedCustomerGroup.POST("/address/:id/update", controllers.UpdateAddress)
			protectedCustomerGroup.DELETE("/address/:id/delete", controllers.DeleteAddress)
			protectedCustomerGroup.GET("/try-on-history", controllers.ListCustomerTryOnImages)
			returnService := services.NewReturnService()
			returnCtrl := controllers.NewReturnController(returnService)
			returns := protectedCustomerGroup.Group("/refund")
			{
				returns.POST("", returnCtrl.CreateCustomerReturn)       // Tạo mới
				returns.GET("", returnCtrl.GetCustomerReturns)          // Danh sách
				returns.GET("/:id", returnCtrl.GetCustomerReturnDetail) // Chi tiết
			}

		}
		wishlistGroup := apiV2.Group("/wishlist")
		wishlistGroup.Use(AuthCustomerMiddleware()) // App\Models\Customer
		{

		}

		compareGroup := apiV2.Group("/compare")
		compareGroup.Use(AuthCustomerMiddleware()) // App\Models\Customer
		{
			compareGroup.GET("/", controllers.ListCompares)
			//compareGroup.POST("/add", controllers.AddCompare)
			compareGroup.DELETE("/:id/delete", controllers.DeleteCompare)
		}

		// --- ADMIN AREA (App\User) ---
		// Tạo một group chung dùng AuthUserMiddleware
		adminAuth := apiV2.Group("/admin")
		//adminAuth.Use(AuthAdminMiddleware())
		{ // Chỉ dành cho App\Models\User  //AuthUserMiddleware()
			{
				routes.RegisterAdminRoutes(adminAuth)
				// Products (CUD - Create, Update, Delete)
				productsGroup := adminAuth.Group("/product")
				{
					productsGroup.GET("/", controllers.ProductSearch)
					productsGroup.GET("/:id", controllers.ProductDetail)
					productsGroup.POST("/", controllers.CreateProduct)
					productsGroup.POST("/:id", controllers.UpdateProduct)
					productsGroup.PUT("/:id", controllers.UpdateProduct)
					productsGroup.DELETE("/:id", controllers.DeleteProduct)
				}

				// Categories (CUD)
				categoriesGroup := adminAuth.Group("/category")
				{
					categoriesGroup.GET("/", getCategoriesHandler)
					categoriesGroup.GET("/:id", controllers.CategoryDetail)

					categoriesGroup.POST("/", controllers.StoreCategory)
					categoriesGroup.PUT("/:id", controllers.DeleteCategory)
					categoriesGroup.DELETE("/:id", controllers.DeleteCategory)
				}

				// Blogs API
				blogsGroup := adminAuth.Group("/blog")
				{
					blogsGroup.GET("/:id", controllers.GetBlogDetailById)

					blogsGroup.GET("/", controllers.ListBlogs)
					blogsGroup.POST("/", controllers.StoreBlog)
					blogsGroup.PUT("/:id", controllers.UpdateBlog)
					blogsGroup.DELETE("/:id", controllers.DeleteBlog)
				}

				// Users API
				usersGroup := adminAuth.Group("/user")
				{
					usersGroup.GET("", controllers.GetUsers)
					usersGroup.GET("/:id", controllers.GetUserByID)
					usersGroup.POST("", controllers.CreateUser)
					usersGroup.PUT("/:id", controllers.UpdateUser)
					usersGroup.DELETE("/:id", controllers.DeleteUser)
				}

				// Pages API
				pagesGroup := adminAuth.Group("/page")
				{
					pagesGroup.GET("/:id", controllers.GetPageById)
					pagesGroup.GET("/", controllers.ListPages)
					pagesGroup.POST("/", controllers.StorePage)
					pagesGroup.PUT("/:id", controllers.UpdatePage)
					pagesGroup.DELETE("/:id", controllers.DestroyPage)
				}

				// Customers API
				customersGroup := adminAuth.Group("/customer")
				{

					customersGroup.GET("/transaction", controllers.GetCustomerTransactions)
					customersGroup.GET("/address", controllers.ListAdminCustomerAddresses)
					customersGroup.POST("/address", controllers.StoreAdminAddress)
					customersGroup.PUT("/address/:id", controllers.UpdateAdminAddress)
					customersGroup.DELETE("/address/:id", controllers.DeleteAddress)
					customersGroup.GET("/", getCustomersHandler)
					customersGroup.POST("/", createCustomerHandler)
					customersGroup.PUT("/:id", updateCustomerHandler)
					customersGroup.DELETE("/:id", deleteCustomerHandler)

				}

				// Settings API
				settingsGroup := adminAuth.Group("/setting")
				{

					settingsGroup.PUT("/", controllers.UpdateSettings)
					settingsGroup.GET("/", getSettingsHandler)
					settingsGroup.POST("/", createSettingHandler)
					//settingsGroup.PUT("/:id", updateSettingHandler)
					settingsGroup.DELETE("/:id", deleteSettingHandler)
				}

				// Attribute API
				attributeGroup := adminAuth.Group("/attribute")
				{
					attributeGroup.GET("", controllers.GetAttributes)
					attributeGroup.GET("/:id", controllers.GetAttributeByID)
					attributeGroup.POST("", controllers.CreateAttribute)
					attributeGroup.PUT("/:id", controllers.UpdateAttribute)
					attributeGroup.DELETE("/:id", controllers.DeleteAttribute)
				}

				// Option API
				optionGroup := adminAuth.Group("/option")
				{
					optionGroup.GET("", controllers.GetOptions)        // GET /admin/options?page=1&limit=20
					optionGroup.GET("/:id", controllers.GetOptionByID) // GET /admin/options/15
					optionGroup.POST("", controllers.CreateOption)     // POST /admin/options
					optionGroup.PUT("/:id", controllers.UpdateOption)  // PUT /admin/options/15
					optionGroup.DELETE("/:id", controllers.DeleteOption)
				}

				// Coupon API
				couponGroup := adminAuth.Group("/coupon")
				{
					couponGroup.GET("", controllers.GetCoupons)        // GET /admin/options?page=1&limit=20
					couponGroup.GET("/:id", controllers.GetCouponByID) // GET /admin/options/15
					couponGroup.POST("", controllers.CreateCoupon)     // POST /admin/options
					couponGroup.PUT("/:id", controllers.UpdateCoupon)  // PUT /admin/options/15
					couponGroup.DELETE("/:id", controllers.DeleteCoupon)
				}

				// Order API
				orderGroup := adminAuth.Group("/order")
				{
					orderGroup.POST("/:id/history", controllers.AddOrderHistory)
					orderGroup.GET("/", controllers.SearchOrders)
					orderGroup.GET("/:id", controllers.GetOrderDetail)
					orderGroup.POST("/", func(c *gin.Context) { createHandler(c, "orders") })
					orderGroup.PUT("/:id", controllers.UpdateOrder)
					orderGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "orders") })
					orderGroup.POST("/:id/create-shipment", controllers.SendToCarrierHandler)
					orderGroup.POST("/:id/cancel-shipment", controllers.CancelShipment)
				}

				// Language API
				languageGroup := adminAuth.Group("/language")
				{
					languageGroup.GET("/", func(c *gin.Context) { getHandler(c, "languages") })
					languageGroup.POST("/", func(c *gin.Context) { createHandler(c, "languages") })
					languageGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "languages") })
					languageGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "languages") })
				}

				// Currency API
				currencyGroup := adminAuth.Group("/currency")
				{
					currencyGroup.GET("/", func(c *gin.Context) { getHandler(c, "currencies") })
					currencyGroup.POST("/", func(c *gin.Context) { createHandler(c, "currencies") })
					currencyGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "currencies") })
					currencyGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "currencies") })
				}

				// Country API
				countryGroup := adminAuth.Group("/country")
				{
					countryGroup.GET("/", func(c *gin.Context) { getHandler(c, "countries") })
					countryGroup.POST("/", func(c *gin.Context) { createHandler(c, "countries") })
					countryGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "countries") })
					countryGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "countries") })
				}

				// State API
				stateGroup := adminAuth.Group("/state")
				{
					stateGroup.GET("/", func(c *gin.Context) { getHandler(c, "states") })
					stateGroup.POST("/", func(c *gin.Context) { createHandler(c, "states") })
					stateGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "states") })
					stateGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "states") })
				}

				// Districts API
				districtsGroup := adminAuth.Group("/districts")
				{
					districtsGroup.GET("/", func(c *gin.Context) { getHandler(c, "districts") })
					districtsGroup.POST("/", func(c *gin.Context) { createHandler(c, "districts") })
					districtsGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "districts") })
					districtsGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "districts") })
				}

				// Tax Class API
				taxClassGroup := adminAuth.Group("/tax_class")
				{
					taxClassGroup.GET("/", func(c *gin.Context) { getHandler(c, "tax_classes") })
					taxClassGroup.POST("/", func(c *gin.Context) { createHandler(c, "tax_classes") })
					taxClassGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "tax_classes") })
					taxClassGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "tax_classes") })
				}

				// Tax Rate API
				taxRateGroup := adminAuth.Group("/tax_rate")
				{
					taxRateGroup.GET("/", func(c *gin.Context) { getHandler(c, "tax_rates") })
					taxRateGroup.POST("/", func(c *gin.Context) { createHandler(c, "tax_rates") })
					taxRateGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "tax_rates") })
					taxRateGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "tax_rates") })
				}

				// Blog Category API
				blogCatGroup := adminAuth.Group("/blog_category")
				{
					blogCatGroup.GET("/", func(c *gin.Context) { getHandler(c, "blog_categories") })
					blogCatGroup.POST("/", func(c *gin.Context) { createHandler(c, "blog_categories") })
					blogCatGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "blog_categories") })
					blogCatGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "blog_categories") })
				}

				// Filters API
				filtersGroup := adminAuth.Group("/filter")
				{
					filtersGroup.GET("/", func(c *gin.Context) { getHandler(c, "filters") })
					filtersGroup.POST("/", func(c *gin.Context) { createHandler(c, "filters") })
					filtersGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "filters") })
					filtersGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "filters") })
				}
				/*
					// Inventories API
					inventoriesGroup := adminAuth.Group("/inventory")
					{
						inventoriesGroup.GET("/", func(c *gin.Context) { getHandler(c, "inventories") })
						inventoriesGroup.POST("/", func(c *gin.Context) { createHandler(c, "inventories") })
						inventoriesGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "inventories") })
						inventoriesGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "inventories") })
					}

					// Inventory Adjust API
					invAdjustGroup := adminAuth.Group("/inventory_adjust")
					{
						invAdjustGroup.GET("/", func(c *gin.Context) { getHandler(c, "inventory_adjusts") })
						invAdjustGroup.POST("/", func(c *gin.Context) { createHandler(c, "inventory_adjusts") })
						invAdjustGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "inventory_adjusts") })
						invAdjustGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "inventory_adjusts") })
					}

					// Inventory History API
					invHistoryGroup := adminAuth.Group("/inventory_history")
					{
						invHistoryGroup.GET("/", func(c *gin.Context) { getHandler(c, "inventory_histories") })
						invHistoryGroup.POST("/", func(c *gin.Context) { createHandler(c, "inventory_histories") })
						invHistoryGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "inventory_histories") })
						invHistoryGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "inventory_histories") })
					}
				*/
				// Newsletter API
				newsletterGroup := adminAuth.Group("/newsletter")
				{
					newsletterGroup.GET("/", func(c *gin.Context) { getHandler(c, "newsletters") })
					newsletterGroup.POST("/", func(c *gin.Context) { createHandler(c, "newsletters") })
					newsletterGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "newsletters") })
					newsletterGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "newsletters") })
				}

				// Payment Method API
				paymentMethodGroup := adminAuth.Group("/payment_method")
				{
					paymentMethodGroup.GET("/", func(c *gin.Context) { getHandler(c, "payment_methods") })
					paymentMethodGroup.POST("/", func(c *gin.Context) { createHandler(c, "payment_methods") })
					paymentMethodGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "payment_methods") })
					paymentMethodGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "payment_methods") })
				}

				// Plugin API
				pluginGroup := adminAuth.Group("/plugin")
				{
					pluginGroup.GET("/form/:id", controllers.GetModuleFormSchemaHandler)

					pluginGroup.GET("/settings/:central_id", controllers.GetPluginSettings)

					// API lưu/thay đổi cấu hình (Dùng POST hoặc PUT đều được)
					pluginGroup.POST("/settings/:central_id", controllers.SavePluginSettings)
					pluginGroup.PUT("/settings/:central_id", controllers.SavePluginSettings)

					pluginGroup.GET("/", controllers.GetPlugins)
					pluginGroup.GET("/installed", controllers.GetInstalledPlugins)
					pluginGroup.POST("/", func(c *gin.Context) { createHandler(c, "plugins") })
					pluginGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "plugins") })
					pluginGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "plugins") })
				}

				// Review API
				reviewGroup := adminAuth.Group("/review")
				{
					reviewGroup.GET("/", func(c *gin.Context) { getHandler(c, "reviews") })
					reviewGroup.POST("/", func(c *gin.Context) { createHandler(c, "reviews") })
					reviewGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "reviews") })
					reviewGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "reviews") })
				}

				// Role API
				roleGroup := adminAuth.Group("/role")
				{
					roleGroup.GET("", controllers.GetRoles)
					roleGroup.GET("/:id", controllers.GetRoleByID)
					roleGroup.POST("", controllers.CreateRole)
					roleGroup.PUT("/:id", controllers.UpdateRole)
					roleGroup.DELETE("/:id", controllers.DeleteRole)
				}

				// Transaction API
				transactionGroup := adminAuth.Group("/transaction")
				{
					transactionGroup.GET("/", func(c *gin.Context) { getHandler(c, "transactions") })
					transactionGroup.POST("/", func(c *gin.Context) { createHandler(c, "transactions") })
					transactionGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "transactions") })
					transactionGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "transactions") })
				}
				/*
					// Purchase API
					purchaseGroup := adminAuth.Group("/purchase")
					{
						purchaseGroup.GET("/", func(c *gin.Context) { getHandler(c, "purchases") })
						purchaseGroup.POST("/", func(c *gin.Context) { createHandler(c, "purchases") })
						purchaseGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "purchases") })
						purchaseGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "purchases") })
					}

					// Purchase Return API
					purchaseReturnGroup := adminAuth.Group("/purchase_return")
					{
						purchaseReturnGroup.GET("/", func(c *gin.Context) { getHandler(c, "purchase_returns") })
						purchaseReturnGroup.POST("/", func(c *gin.Context) { createHandler(c, "purchase_returns") })
						purchaseReturnGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "purchase_returns") })
						purchaseReturnGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "purchase_returns") })
					}
				*/
				/*
					// Warehouse API
					warehouseGroup := adminAuth.Group("/warehouse")
					{
						warehouseGroup.GET("/", func(c *gin.Context) { getHandler(c, "warehouses") })
						warehouseGroup.POST("/", func(c *gin.Context) { createHandler(c, "warehouses") })
						warehouseGroup.PUT("/:id", func(c *gin.Context) { updateHandler(c, "warehouses") })
						warehouseGroup.DELETE("/:id", func(c *gin.Context) { deleteHandler(c, "warehouses") })
					}

					suppliersGroup := adminAuth.Group("/supplier")
					{
						suppliersGroup.GET("/", getSuppliersHandler)
						suppliersGroup.POST("/", createSupplierHandler)
						suppliersGroup.PUT("/:id", updateSupplierHandler)
						suppliersGroup.DELETE("/:id", deleteSupplierHandler)
					}

					// Transfers API (Điều chuyển kho)
					transfersGroup := adminAuth.Group("/transfer")
					{
						transfersGroup.GET("/", getTransfersHandler)
						transfersGroup.POST("/", createTransferHandler)
						transfersGroup.PUT("/:id", updateTransferHandler)
						transfersGroup.DELETE("/:id", deleteTransferHandler)
					}

					// Adjustments API (Kiểm kê/Điều chỉnh kho)
					adjustmentsGroup := adminAuth.Group("/adjustment")
					{
						adjustmentsGroup.GET("/", getAdjustmentsHandler)
						adjustmentsGroup.POST("/", createAdjustmentHandler)
						adjustmentsGroup.PUT("/:id", updateAdjustmentHandler)
						adjustmentsGroup.DELETE("/:id", deleteAdjustmentHandler)
					}

					// Expenses API (Quản lý chi phí)
					expensesGroup := adminAuth.Group("/expense")
					{
						expensesGroup.GET("/", getExpensesHandler)
						expensesGroup.POST("/", createExpenseHandler)
						expensesGroup.PUT("/:id", updateExpenseHandler)
						expensesGroup.DELETE("/:id", deleteExpenseHandler)
					}
				*/
				pageThemeGroup := adminAuth.Group("/page-themes")
				{
					pageThemeGroup.GET("/", controllers.GetPageThemesHandler)
					pageThemeGroup.GET("/other", controllers.GetOtherPageThemesHandler)

					pageThemeGroup.POST("/", controllers.CreatePageThemeHandler)
					pageThemeGroup.PUT("/:id", controllers.UpdatePageThemeHandler)
					pageThemeGroup.DELETE("/:id", controllers.DeletePageThemeHandler)
				}

				productItems := adminAuth.Group("/product-item-template")
				{
					productItems.GET("/", controllers.GetProductItemTemplateData)
					productItems.POST("/", controllers.StoreProductItemTemplate)
					productItems.GET("/:id", controllers.ShowProductItemTemplate)
					productItems.PUT("/:id", controllers.UpdateProductItemTemplate)
					productItems.DELETE("/:id", controllers.DestroyProductItemTemplate)

				}
				permissions := adminAuth.Group("/permissions")
				{
					permissions.GET("/", controllers.GetGroupedPermissions)
				}

				ctl := controllers.NewEmailTemplateController()

				templatesGroup := adminAuth.Group("/email-templates")
				{
					templatesGroup.GET("/", ctl.GetAll)
					templatesGroup.GET("/:id", ctl.GetByID)
					templatesGroup.POST("", ctl.Create)
					templatesGroup.PUT("/:id", ctl.Update)
					templatesGroup.DELETE("/:id", ctl.Delete)
				}

				report := adminAuth.Group("/report")
				{
					report.GET("/quantity/variant", controllers.GetStockVariant)
					report.GET("/quantity/product", controllers.GetStockProduct)
					report.GET("/customer-orders", controllers.GetCustomerOrderReportHandler)
					report.GET("/products-sell", controllers.GetProductsSellReportHandler)
					report.GET("/products-view", controllers.GetProductsViewReportHandler)
					report.GET("/sales", controllers.GetSalesReportHandler)
					report.GET("/coupons", controllers.GetCouponsReportHandler)
				}

				returnService := services.NewReturnService()
				returnCtrl := controllers.NewReturnController(returnService)
				returns := adminAuth.Group("/return-request")
				{
					returns.GET("", returnCtrl.GetAllAdminRefund)
					returns.GET("/:id", returnCtrl.GetAdminRefundByID)
					returns.POST("", returnCtrl.CreateAdminRefund)
					returns.PUT("/:id", returnCtrl.UpdateAdminRefund)
					returns.DELETE("/:id", returnCtrl.DeleteAdminRefund)
					returns.POST("/:id/accept", returnCtrl.AcceptReturn)
				}

				carrierService := services.NewCarrierService()
				carrierController := controllers.NewCarrierController(carrierService)

				carrier := adminAuth.Group("/carrier")
				{
					carrier.GET("/", carrierController.GetCarriers)
					carrier.GET("/:id", carrierController.GetCarierByID)
					carrier.POST("", carrierController.CreateCarier)
					carrier.PUT("/:id", carrierController.UpdateCarier)
					carrier.DELETE("/:id", carrierController.DeleteCarier)
					carrier.GET("/:id/rate-table", controllers.GetCarrierRateTable)
					carrier.POST("/:id/rate-table", controllers.SaveCarrierRateTable)
				}

				zoneGroup := adminAuth.Group("/zone")
				{
					zoneGroup.GET("", controllers.GetZones)
					zoneGroup.GET("/:id", controllers.GetZoneByID)
					zoneGroup.POST("", controllers.CreateZone)
					zoneGroup.POST("/:id", controllers.UpdateZone)
					zoneGroup.DELETE("/:id", controllers.DeleteZone)
				}

				aIUsage := adminAuth.Group("/ai/token")
				{
					aIUsage.GET("/", controllers.LogAIUsage)

				}

				adminAuth.GET("/billing-history", controllers.GetBillingHistoryHandler)
				adminAuth.GET("/tenant/information", controllers.GetTenantInformationHandler)

				adminAuth.POST("/domains", controllers.AddDomainHandler)
				adminAuth.GET("/dashboard/summary", controllers.GetDashboardSummaryHandler)

				notifications := adminAuth.Group("/notification")
				{
					notifications.GET("/unread", controllers.GetUnreadNotificationsHandler)
					notifications.POST("/read", controllers.MarkAsReadHandler)
				}
				adminAuth.GET("/notifications/stream", controllers.StreamAdminNotifications)
				adminAuth.POST("/cache/clear", controllers.ClearTenantCache)
				adminAuth.GET("/cache/stats", controllers.GetTenantCacheStatsHandler)

				shopee := adminAuth.Group("/shopee")
				{
					shopee.GET("/connect-url", controllers.GetShopeeConnectURL)

					// Order Endpoints
					shopee.GET("/order/list", controllers.FetchShopeeOrderList)
					shopee.GET("/order/detail", controllers.FetchShopeeOrderDetail)
					shopee.POST("/order/ship", controllers.ShipShopeeOrder)
					shopee.GET("/order/shipments", controllers.FetchShopeeShipmentList)
					shopee.GET("/order/shipping-parameter", controllers.FetchShopeeShippingParameter)
					shopee.GET("/order/tracking-number", controllers.FetchShopeeTrackingNumber)
					shopee.POST("/order/cancel", controllers.CancelShopeeOrder)

					// Product Endpoints
					shopee.GET("/product/list", controllers.FetchShopeeItemList)
					shopee.GET("/product/detail", controllers.FetchShopeeItemBaseInfo)
					shopee.POST("/product/stock", controllers.UpdateShopeeStock)
					shopee.POST("/product/price", controllers.UpdateShopeePrice)

					shopee.POST("/sync-products", controllers.SyncProductsFromChannel)
					// 2. API Lấy danh sách sản phẩm chờ ghép tay (status = UNMAPPED, product_id IS NULL)
					shopee.GET("/unmapped-products", controllers.GetUnmappedProducts)
					// 3. API Thực hiện ghép nối thủ công từ giao diện
					shopee.POST("/manual-map", controllers.ManualMapProduct)
					shopee.POST("/approve-create", controllers.ApproveAndCreateProduct)
					shopee.POST("/syn-orders", controllers.SyncOrders)

				}

				// ==================== TIKTOK ROUTES ====================
				tiktok := adminAuth.Group("/tiktok")
				{
					tiktok.GET("/connect-url", controllers.GetTikTokConnectURL)

					// Order Endpoints
					//tiktok.POST("/order/search", controllers.SearchTikTokOrders)
					tiktok.GET("/order/detail", controllers.FetchTikTokOrderDetail)
					tiktok.POST("/order/ship", controllers.ShipTikTokOrder)

					// Product Endpoints
					tiktok.POST("/product/search", controllers.SearchTikTokProducts)
					tiktok.POST("/product/stock", controllers.UpdateTikTokStock)
					tiktok.POST("/product/price", controllers.UpdateTikTokPrice)
					tiktok.POST("/syn-orders", controllers.SyncTikTokOrdersHandler)
					tiktok.POST("/order/search", controllers.SearchTikTokOrdersHandler)

					// Đồng bộ Đơn hàng TikTok về Database Web

					// Cập nhật trạng thái giao hàng / vận đơn trên TikTok
					//	tiktok.POST("/order/ship", controllers.ShipTikTokOrderHandler)

					// --- PRODUCT MANAGEMENT ---
					// Xem danh sách sản phẩm TikTok (Proxy)
					//tiktok.POST("/product/search", controllers.SearchTikTokProductsHandler)

					// Đồng bộ Sản phẩm & SKU biến thể TikTok về Database Web
					tiktok.POST("/sync-products", controllers.SyncTikTokProductsHandler)
					tiktok.GET("/unmapped-products", controllers.GetUnmappedProductsHandler)
					tiktok.POST("/manual-map", controllers.ManualMapSKUHandler)
					// Cập nhật tồn kho SKU sang TikTok
					//	tiktok.POST("/product/stock", controllers.UpdateTikTokStockHandler)

					// Cập nhật giá bán SKU sang TikTok
					//	tiktok.POST("/product/price", controllers.UpdateTikTokPriceHandler)
					//shopee.POST("/sync-products", controllers.SyncProductsFromChannel)
				}

			}
		}

		pageHistories := apiV2.Group("/page-builder-history")
		{
			pageHistories.GET("/", controllers.GetPageHistories)
			pageHistories.POST("/", controllers.StorePageHistory)
			pageHistories.GET("/:id", controllers.ShowPageHistory)
			pageHistories.PUT("/:id", controllers.UpdatePageHistory)
			pageHistories.DELETE("/:id", controllers.DestroyPageHistory)

		}
		apiV2.GET("/page-builder/:id", controllers.GetLatestHistory)
		apiV2.GET("/category/tree", controllers.CategoryTree)

		apiV2.POST("/upload", uploadHandler)
		apiV2.GET("/upload", upload2Handler)
		apiV2.GET("/countries", controllers.GetCountries)
		apiV2.GET("/states/:id", controllers.StatesByCountry)
		apiV2.GET("/district/:id", controllers.DistrictsByState)
		apiV2.GET("/ward/:id", controllers.WardsByDistrict)

		apiV2.GET("/payment_method", controllers.AllPaymentMethods)
		apiV2.GET("/shipping_method", controllers.AllShippingMethodMethods)
		apiV2.GET("/language/all", controllers.AllLanguages)
		apiV2.GET("/language/frontend", controllers.FrontendLanguages)

		apiV2.GET("/currency/all", controllers.AllCurrencies)
		apiV2.GET("/coupon/all", controllers.AllCoupons)
		apiV2.GET("/coupon/by/ids", controllers.GetCouponByIds)
		apiV2.GET("/blog/categories", controllers.AllBlogCategories)

		apiV2.GET("/blog/all", controllers.AllBlogs)
		apiV2.GET("/blog/by/ids", controllers.GetBlogsByIds)

		apiV2.POST("/wishlist/add", controllers.AddWishlist)
		apiV2.GET("/wishlist", controllers.ListWishlists)
		apiV2.DELETE("/:id/delete", controllers.DeleteWishlist)

		apiV2.GET("/product/by_ids", fetchProductsByIds)
		apiV2.POST("/cart/add", OptionalAuthMiddleware(), AddToCart)
		apiV2.POST("/cart/coupon/apply", OptionalAuthMiddleware(), controllers.ApplyCouponHandler)
		apiV2.GET("/cart/guest/load", OptionalAuthMiddleware(), controllers.LoadCart)
		//apiV2.GET("/cart/address/update", OptionalAuthMiddleware(), controllers.UpdateCartAddress)

		apiV2.POST("/cart/update/quantity", controllers.CartUpdateQuantity)
		apiV2.POST("/cart/remove-item", controllers.RemoveCartItem)
		apiV2.POST("/checkout/cart/update", OptionalAuthMiddleware(), controllers.CartUpdate)

		apiV2.GET("/product/search", controllers.ProductSearch)
		apiV2.GET("/product/", controllers.ProductSearch)
		apiV2.GET("/product/by_slug/:slug", controllers.ProductBySlug)
		apiV2.GET("/product/:id", controllers.ProductDetail)

		apiV2.POST("/newsletter", controllers.SaveNewsletter)
		apiV2.POST("/contact_us", controllers.SaveContactHandler)
		apiV2.POST("/review", OptionalAuthMiddleware(), controllers.CreateReviewHandler)
		apiV2.POST("/checkout/ship/method/change", controllers.ShippingMethodChange)
		apiV2.POST("/checkout/ship/address/change", OptionalAuthMiddleware(), controllers.AddressChange)
		apiV2.POST("/order/:id", OptionalAuthMiddleware(), controllers.CheckoutHandler)
		apiV2.GET("/verify", controllers.CustomerVerifyHandler)
		apiV2.GET("/paypal/callback", controllers.PayPalCallbackHandler)
		apiV2.GET("/flutterwave/callback", controllers.FlutterwaveCallbackHandler)
		apiV2.GET("/stripe/success", controllers.StripeHandleSuccess)
		apiV2.GET("/paystack/callback", controllers.PaystackCallback)
		apiV2.GET("/mollie/callback", controllers.MollieCallback)
		apiV2.POST("/sslcommerz/callback", controllers.SSLCommerzCallback)
		apiV2.POST("/razorpay/callback/:code", controllers.RazorpayCallback)
		apiV2.POST("/authorizenet/submit/:code", controllers.AuthorizeNetPaymentSubmit)
		apiV2.GET("/settings", controllers.GetSettings)
		apiV2.POST("/sso/exchange-token", controllers.ExchangeSSOTokenHandler)
		apiV2.GET("/tenant/init", controllers.TenantInit)

		apiV2.GET("/chat/conversations", middleware.ChatAuthMiddleware(), controllers.GetConversations)
		apiV2.GET("/chat/conversations/:conversationId", middleware.ChatAuthMiddleware(), controllers.GetConversation)
		apiV2.POST("/chat/conversations", middleware.ChatAuthMiddleware(), controllers.CreateConversation)
		apiV2.GET("/chat/conversations/:conversationId/messages", middleware.ChatAuthMiddleware(), controllers.GetConversationMessages)
		apiV2.POST("/chat/conversations/:conversationId/messages", middleware.ChatAuthMiddleware(), controllers.SendMessage)
		apiV2.GET("/chat/ws/:conversationId", middleware.ChatAuthMiddleware(), controllers.ChatWebSocket)
		apiV2.GET("/forgot-password", controllers.RenderForgotPasswordPage)
		apiV2.GET("/reset-password", controllers.RenderResetPasswordPage)
		apiV2.GET("/auth/forgot-password", controllers.ForgotPasswordHandler)
		apiV2.GET("/auth/reset-password", controllers.ResetPasswordHandler)
		apiV2.GET("/test", controllers.Test)

		apiV2.POST("/goship/event", controllers.HandleGoshipEvent)
		apiV2.GET("/goship/event", controllers.HandleGoshipEvent)
		apiV2.POST("webhooks/telegram", controllers.HandleTelegramWebhook)
		apiV2.GET("webhooks/telegram", controllers.HandleTelegramWebhookTest)
		webhookGroup := apiV2.Group("/webhooks")
		{
			// Webhook từ Shopee
			webhookGroup.POST("/shopee", controllers.HandleShopeeWebhook)
			webhookGroup.POST("/tiktok", controllers.TikTokWebhookHandler)

		}
		apiV2.POST("telegram/internal/messages/process", controllers.HandleProcessMessage)

	}
	r.Run(":9002")
}

func getDBFromContext(c *gin.Context) (*sql.DB, error) {
	v, ok := c.Get("db")
	if !ok {
		return nil, fmt.Errorf("Database connection not found in context")
	}
	db, ok := v.(*sql.DB)
	if !ok {
		return nil, fmt.Errorf("Database connection in context is not of type *sql.DB")
	}
	return db, nil
}

//upload

func upload2Handler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Upload thành công!",
	})
}

/*
	func allCountry(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Upload thành công!",
		})
	}
*/
func uploadHandler(c *gin.Context) {
	tenantID, exists := c.Get("tenantId")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tenant context missing"})
		return
	}

	//uploadPath := `D:\xampp8171\htdocs\tutaoweb\storage\app\public\tenancy\hoangk66\app\public\images`
	basePath := "/var/www/html/vue-ecom-central/storage/app/public/tenancy"
	uploadPath := filepath.Join(basePath, fmt.Sprintf("%v", tenantID), "app/public/images") // 2. Nhận file từ form-data (key là "image")

	// Tạo folder nếu chưa có (rất quan trọng cho tenant mới)
	if _, err := os.Stat(uploadPath); os.IsNotExist(err) {
		os.MkdirAll(uploadPath, os.ModePerm)
	}

	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không tìm thấy file gửi lên"})
		return
	}

	// 3. Tạo tên file duy nhất để tránh trùng lặp
	// Ví dụ: 1672531200_filename.jpg
	fileName := fmt.Sprintf("%d_%s", time.Now().Unix(), filepath.Base(file.Filename))

	// Đường dẫn đầy đủ để lưu file
	dst := filepath.Join(uploadPath, fileName)

	// 4. Lưu file vào thư mục đích
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể lưu file: " + err.Error()})
		return
	}

	// 5. Trả về JSON chứa link ảnh
	// Lưu ý: Bạn cần thay "http://localhost:8080" bằng domain thật của bạn
	fileURL := fmt.Sprintf("https://popshop.tutaoweb.com/storage/tenancy/popshop/app/public/images/%s", fileName)

	c.JSON(http.StatusOK, gin.H{
		"message": "Upload thành công!",
		"url":     fileURL,
	})
}

// Request structure

type AddToCartRequest struct {
	ProductID        int  `json:"productId" binding:"required"`
	ProductVariantID *int `json:"productVariantId"`
	Quantity         *int `json:"quantity"`
}

type OrderListItem struct {
	ID          int     `json:"id"`
	OrderNumber string  `json:"order_number"`
	Total       float64 `json:"total"`
	Status      int     `json:"status"` // Ví dụ: pending, completed, cancelled
	ItemCount   int     `json:"item_count"`
	CreatedAt   string  `json:"created_at"`
}

func customerOrder(c *gin.Context) {

	// 1. Lấy customerID từ Middleware
	var customerID int
	if id, exists := c.Get("customerID"); exists {
		customerID = id.(int)
	} else {
		customerID = 0 // Guest
	}

	// 2. Lấy kết nối DB
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// 3. Xử lý phân trang (Optional nhưng nên có)
	pageStr := c.DefaultQuery("page", "1")
	limit := 10 // Số bản ghi mỗi trang

	// 2. Chuyển đổi string sang int (sử dụng gói strconv)
	page, err := strconv.Atoi(pageStr)

	// Chuyển đổi page string sang int để tính offset
	offset := (page - 1) * limit

	// 4. Truy vấn danh sách đơn hàng
	// Sử dụng COALESCE hoặc IFNULL để xử lý các trường có thể null
	query := `
        SELECT id, order_number, COALESCE(total, 0) as total, order_status_id, item_count, created_at 
        FROM orders 
        WHERE customer_id = ?  ORDER BY created_at DESC limit ? offset ?  `

	// Log query để debug

	utils.LogSQL(query, customerID, limit, offset)

	rows, err := tenantDB.Query(query, customerID, limit, offset)
	if err != nil {
		utils.LogToFile("Error querying orders: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch orders"})
		return
	}
	defer rows.Close() // Quan trọng: Luôn đóng rows để tránh rò rỉ bộ nhớ

	// 5. Duyệt qua các dòng kết quả (Scan)
	var orders []OrderListItem
	for rows.Next() {
		utils.LogToFile("bbb")

		var o OrderListItem
		err := rows.Scan(
			&o.ID,
			&o.OrderNumber,
			&o.Total,
			&o.Status,
			&o.ItemCount,
			&o.CreatedAt,
		)

		if err != nil {
			utils.LogToFile("Scan error: %v", err)

			continue
		}

		orders = append(orders, o)
	}
	//utils.LogToFile(orders)

	// 6. Kiểm tra nếu không có đơn hàng nào
	if orders == nil {
		orders = []OrderListItem{} // Trả về mảng rỗng [] thay vì null
	}

	// 7. Trả về kết quả
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   orders,
		"meta": gin.H{
			"current_page": page,
			"count":        len(orders),
		},
	})
}

type CustomerProfile struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	//Avatar    sql.NullString `json:"avatar"`
	//Address   sql.NullString `json:"address"`
	CreatedAt string `json:"created_at"`
}

func customerProfile(c *gin.Context) {
	// 1. Lấy customerID từ Middleware (đã set bằng c.Set("customerID", id))
	val, exists := c.Get("customerID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}
	customerID := val.(int)
	utils.LogToFile("customerID = %d ", customerID)

	// 2. Lấy kết nối DB
	tenantDB, err := getDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// 3. Truy vấn thông tin chi tiết
	var profile CustomerProfile
	query := `SELECT id, name, email, phone,  created_at 
              FROM customers 
              WHERE id = ? LIMIT 1`

	// Log query để debug nếu cần
	utils.LogSQL(query, customerID)

	err = tenantDB.QueryRow(query, customerID).Scan(
		&profile.ID,
		&profile.Name,
		&profile.Email,
		&profile.Phone,
		//&profile.Avatar,
		//	&profile.Address,
		&profile.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "Customer profile not found"})
		} else {
			log.Printf("Error fetching profile: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		}
		return
	}

	// 4. Trả về dữ liệu thành công
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   profile,
	})
}

func AddToCart(c *gin.Context) {
	tenantDB, err := getDBFromContext(c)
	if err != nil {
		utils.LogToFile("[ERROR AddToCart] Không lấy được DB từ context: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	var req AddToCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogToFile("[WARN AddToCart] Invalid payload request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Log Payload nhận vào
	var variantIDVal int
	if req.ProductVariantID != nil {
		variantIDVal = *req.ProductVariantID
	}
	utils.LogToFile("[INFO AddToCart] Start -> ProductID: %d, VariantID: %d", req.ProductID, variantIDVal)

	// 1. Lấy thông tin Sản phẩm Gốc
	var product struct {
		ID               int
		MinOrderQuantity int
		BasePrice        sql.NullFloat64
		Title            string
		Condition        sql.NullString
		ShippingWeight   sql.NullFloat64
	}
	//lang := c.DefaultQuery("lang", "en")
	//jsonPath := "$." + lang

	productQuery := "SELECT id, sale_price, min_order_quantity, title as name, `condition`, shipping_weight FROM products WHERE id = ?"
	err = tenantDB.QueryRow(productQuery, req.ProductID).Scan(
		&product.ID, &product.BasePrice, &product.MinOrderQuantity, &product.Title, &product.Condition, &product.ShippingWeight,
	)

	if err == sql.ErrNoRows {
		utils.LogToFile("[WARN AddToCart] ProductID = %d không tồn tại", req.ProductID)
		c.JSON(http.StatusNotFound, gin.H{"message": "Product not found", "success": false})
		return
	} else if err != nil {
		utils.LogToFile("[ERROR AddToCart] Lỗi query product ID=%d: %v", req.ProductID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching product", "success": false})
		return
	}

	// 2. Lấy Giá từ Variant hoặc Product Gốc
	var unitPrice float64
	var sku string
	var variantTitle string
	if req.ProductVariantID != nil && *req.ProductVariantID > 0 {
		var variant struct {
			ID       int64
			Price    float64
			SKU      string
			StockQty int
		}

		// Chỉ SELECT đúng các cột có trong Schema DB
		variantQuery := "SELECT id, price, sku, quantity FROM product_variants WHERE id = ? AND product_id = ?"
		err = tenantDB.QueryRow(variantQuery, *req.ProductVariantID, product.ID).Scan(
			&variant.ID, &variant.Price, &variant.SKU, &variant.StockQty,
		)

		if err == sql.ErrNoRows {
			utils.LogToFile("[WARN AddToCart] VariantID = %d không thuộc ProductID = %d (ErrNoRows)", *req.ProductVariantID, product.ID)
			c.JSON(http.StatusNotFound, gin.H{"message": "Selected product variant not found"})
			return
		} else if err != nil {
			utils.LogToFile("[ERROR AddToCart] Lỗi query variant ID=%d: %v", *req.ProductVariantID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error checking variant"})
			return
		}

		// Vì cột `price` trong DB là NOT NULL nên có thể gán trực tiếp
		unitPrice = variant.Price
		sku = variant.SKU

		utils.LogToFile("[INFO AddToCart] Loaded VariantID = %d | Price = %.2f | SKU = %s", variant.ID, unitPrice, sku)
	} else {
		// Sản phẩm đơn lẻ
		if product.BasePrice.Valid {
			unitPrice = product.BasePrice.Float64
		}
		utils.LogToFile("[INFO AddToCart] No Variant -> Base Price = %.2f", unitPrice)
	}

	// Cảnh báo nếu unitPrice bằng 0 trước khi ghi DB
	if unitPrice <= 0 {
		utils.LogToFile("[WARN AddToCart] UnitPrice đang bằng 0.00! Vui lòng kiểm tra lại cột price trong bảng product_variants hoặc sale_price trong bảng products.")
	}

	// 3. Tính toán Số lượng
	qtt := product.MinOrderQuantity
	if req.Quantity != nil && *req.Quantity > 0 {
		qtt = *req.Quantity
	}

	// 4. Kiểm tra Auth User
	var customerID *int
	if val, exists := c.Get("customerID"); exists {
		id := val.(int)
		customerID = &id
	}

	// 5. Tìm hoặc Tạo Giỏ Hàng (Cart)
	var cart struct {
		ID             int64
		CustomerID     sql.NullInt64
		IPAddress      string
		ItemCount      int
		Quantity       int
		Total          float64
		ShippingWeight sql.NullFloat64
	}

	var cartQuery string
	var cartArgs []any

	if customerID != nil {
		cartQuery = "SELECT id, customer_id, ip_address, item_count, quantity, total, shipping_weight FROM carts WHERE customer_id = ? LIMIT 1"
		cartArgs = []any{*customerID}
	} else {
		cartQuery = "SELECT id, customer_id, ip_address, item_count, quantity, total, shipping_weight FROM carts WHERE customer_id IS NULL AND ip_address = ? LIMIT 1"
		cartArgs = []any{c.ClientIP()}
	}

	err = tenantDB.QueryRow(cartQuery, cartArgs...).Scan(
		&cart.ID, &cart.CustomerID, &cart.IPAddress, &cart.ItemCount, &cart.Quantity, &cart.Total, &cart.ShippingWeight,
	)

	if err == sql.ErrNoRows {
		createCartQuery := "INSERT INTO carts (customer_id, ip_address, item_count, quantity, total, shipping_weight) VALUES (?, ?, 0, 0, 0, 0)"
		res, createErr := tenantDB.Exec(createCartQuery, customerID, c.ClientIP())
		if createErr != nil {
			utils.LogToFile("[ERROR AddToCart] Tạo cart thất bại: %v", createErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create cart"})
			return
		}
		lastID, _ := res.LastInsertId()
		cart.ID = int64(lastID)
		utils.LogToFile("[INFO AddToCart] Tạo Cart mới thành công, CartID = %d", cart.ID)
	}

	// 6. Kiểm tra Item đã có trong Cart hay chưa
	var cartItemID int
	var currentItemQty int

	var checkItemQuery string
	var checkItemArgs []any

	if req.ProductVariantID != nil && *req.ProductVariantID > 0 {
		checkItemQuery = "SELECT id, quantity FROM cart_items WHERE cart_id = ? AND product_id = ? AND product_variant_id = ? LIMIT 1"
		checkItemArgs = []any{cart.ID, product.ID, *req.ProductVariantID}
	} else {
		checkItemQuery = "SELECT id, quantity FROM cart_items WHERE cart_id = ? AND product_id = ? AND product_variant_id IS NULL LIMIT 1"
		checkItemArgs = []any{cart.ID, product.ID}
	}

	err = tenantDB.QueryRow(checkItemQuery, checkItemArgs...).Scan(&cartItemID, &currentItemQty)

	// Tên hiển thị sản phẩm trong cart
	itemDesc := product.Title
	if variantTitle != "" {
		itemDesc = fmt.Sprintf("%s (%s)", product.Title, variantTitle)
	}

	// 7. Thêm mới hoặc Cập nhật Item
	if cartItemID > 0 {
		// UPDATE
		newQty := currentItemQty + qtt
		updateQuery := "UPDATE cart_items SET quantity = ?, unit_price = ? WHERE id = ?"
		_, err = tenantDB.Exec(updateQuery, newQty, unitPrice, cartItemID)
		if err != nil {
			utils.LogToFile("[ERROR AddToCart] Update cart_items (ID=%d) thất bại: %v", cartItemID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update item quantity"})
			return
		}
		utils.LogToFile("[INFO AddToCart] Updated cart_items ID = %d | NewQty = %d | UnitPrice = %.2f", cartItemID, newQty, unitPrice)
	} else {
		// INSERT
		insertQuery := "INSERT INTO cart_items (cart_id, product_id, product_variant_id, item_description, quantity, unit_price) VALUES (?, ?, ?, ?, ?, ?)"
		_, err = tenantDB.Exec(insertQuery, cart.ID, product.ID, req.ProductVariantID, itemDesc, qtt, unitPrice)
		if err != nil {
			utils.LogToFile("[ERROR AddToCart] Insert cart_items thất bại: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not add item to cart"})
			return
		}
		utils.LogToFile("[INFO AddToCart] Inserted new cart_item -> CartID = %d | ProductID = %d | VariantID = %d | UnitPrice = %.2f", cart.ID, product.ID, variantIDVal, unitPrice)
	}

	// 8. Tính toán lại tổng giỏ hàng

	currentCart := &models.Cart{
		ID:       cart.ID,
		CouponID: nil, // tương đương với NULL / Valid = false trong DB
	}

	recalcResult, err := services.CartRecalculate(c, currentCart)
	if err != nil {
		utils.LogToFile("[ERROR AddToCart] Recalculate giỏ hàng thất bại: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart totals"})
		return
	}
	utils.LogToFile(customerID)

	utils.LogToFile("[SUCCESS AddToCart] Hoàn thành AddToCart thành công cho CartID = %d  ", cart.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Add item successfully",
		"data":    recalcResult,
	})
}

// Products handlers
func getProductsHandler(c *gin.Context) {
	getHandler(c, "products")
}

func getProductHandler(c *gin.Context) {
	tenantDB, err := getDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB connection error"})
		return
	}

	id := c.Param("id")
	lang := c.DefaultQuery("lang", "en")

	// 1. Lấy thông tin sản phẩm
	product, err := fetchProductRow(tenantDB, id, lang)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Fetch product failed"})
		}
		return
	}

	// 2. Lấy danh sách category (Lỗi ở đây có thể bỏ qua hoặc log lại tùy bạn)
	categories, _ := fetchCategoryIDs(tenantDB, id)
	product["category_ids"] = categories

	c.JSON(http.StatusOK, product)
}

func ScanRowToMap(rows *sql.Rows) (map[string]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Tạo mảng chứa giá trị và mảng chứa con trỏ đến giá trị đó
	columns := make([]interface{}, len(cols))
	columnPointers := make([]interface{}, len(cols))
	for i := range columns {
		columnPointers[i] = &columns[i]
	}

	// Scan dữ liệu vào các con trỏ
	if err := rows.Scan(columnPointers...); err != nil {
		return nil, err
	}

	// Chuyển đổi dữ liệu sang Map
	entry := make(map[string]interface{})
	for i, colName := range cols {
		val := columns[i]

		// Xử lý kiểu []byte (thường là string/json trong MySQL)
		if b, ok := val.([]byte); ok {
			entry[colName] = string(b)
		} else {
			entry[colName] = val
		}
	}

	return entry, nil
}

func fetchCategoryIDs(db *sql.DB, productID string) ([]int, error) {
	query := `SELECT category_id FROM category_product WHERE product_id = ?`
	rows, err := db.Query(query, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func fetchProductRow(db *sql.DB, id string, lang string) (map[string]interface{}, error) {
	jsonPath := "$." + lang
	query := `SELECT id, brand_id, sku, ... (các field khác) ...,
              JSON_UNQUOTE(JSON_EXTRACT(title, ?)) as name
              FROM products WHERE id = ? LIMIT 1`

	utils.LogSQL(query, id)
	rows, err := db.Query(query, jsonPath, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, sql.ErrNoRows
	}

	// Sử dụng hàm dùng chung ở đây
	return ScanRowToMap(rows)
}

type CartGuestResponse struct {
	ID             int             `json:"id"`
	CustomerID     *int            `json:"customer_id"`
	IPAddress      string          `json:"ip_address"`
	ItemCount      int             `json:"item_count"`
	Quantity       int             `json:"quantity"`
	Total          float64         `json:"total"`
	ShippingWeight sql.NullFloat64 `json:"shipping_weight"`
}

func cartByCustomer(c *gin.Context) {
	// 1. Lấy customerID từ Middleware (đã set trong AuthCustomerMiddleware)
	val, exists := c.Get("customerID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	customerID := val.(int)

	// 2. Lấy kết nối DB
	tenantDB, err := getDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// 3. Truy vấn tìm giỏ hàng theo customer_id
	// Giống: Cart::where('customer_id', auth('sanctum')->user()->id)->first()
	query := `
        SELECT id, customer_id, ip_address, item_count, quantity, total, shipping_weight 
        FROM carts 
        WHERE customer_id = ? 
        LIMIT 1`

	// Log SQL để debug
	logSQL(query, []interface{}{customerID}...)

	var cart CartGuestResponse // Dùng chung Struct đã định nghĩa ở hàm Guest
	err = tenantDB.QueryRow(query, customerID).Scan(
		&cart.ID,
		&cart.CustomerID,
		&cart.IPAddress,
		&cart.ItemCount,
		&cart.Quantity,
		&cart.Total,
		&cart.ShippingWeight,
	)

	// 4. Xử lý kết quả
	if err != nil {
		if err == sql.ErrNoRows {
			// Tương đương $cart = null trong Laravel
			c.JSON(http.StatusOK, gin.H{
				"status": "success",
				"cart":   nil,
			})
			return
		}
		log.Printf("Error fetching customer cart: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// 5. Trả về dữ liệu
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"cart":   cart,
	})
}

func cartByGuest(c *gin.Context) {
	// 1. Lấy kết nối DB từ context
	tenantDB, err := getDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// 2. Lấy IP của người dùng (tương đương request()->ip())
	clientIP := c.ClientIP()

	// 3. Truy vấn tìm giỏ hàng của khách vãng lai
	query := `
        SELECT id, customer_id, ip_address, item_count, quantity, total, shipping_weight 
        FROM carts 
        WHERE customer_id IS NULL AND ip_address = ? 
        LIMIT 1`

	// Log query để debug
	logSQL(query, []interface{}{clientIP}...)

	var cart CartGuestResponse
	err = tenantDB.QueryRow(query, clientIP).Scan(
		&cart.ID,
		&cart.CustomerID,
		&cart.IPAddress,
		&cart.ItemCount,
		&cart.Quantity,
		&cart.Total,
		&cart.ShippingWeight,
	)

	// 4. Xử lý kết quả trả về
	if err != nil {
		if err == sql.ErrNoRows {
			// Trường hợp không tìm thấy giỏ hàng (tương đương $cart = null)
			c.JSON(http.StatusOK, gin.H{
				"status": "success",
				"cart":   nil,
			})
			return
		}
		// Trường hợp lỗi database khác
		logSQL("ERROR Fetching Guest Cart: "+err.Error(), nil)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// 5. Logic recalculate (Nếu bạn cần tính lại giống cartRecalculate() trong Laravel)
	// Ở đây tôi trả về dữ liệu cart trực tiếp như trong DB
	// Bạn có thể gọi thêm một hàm helper xử lý nếu cần tính toán lại giá trị

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"cart":   cart,
	})
}

func fetchProductsByIds(c *gin.Context) {
	// 1. Lấy Database từ Context
	tenantDB, err := getDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection error"})
		return
	}

	// 2. Lấy danh sách IDs từ Request (Ví dụ từ Body JSON: { "ids": [1, 2, 3] })

	ids := c.QueryArray("ids[]")
	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No IDs provided"})
		return
	}

	// Chuyển []string sang []interface{} để truyền vào hàm Query SQL
	productIds := make([]interface{}, len(ids))
	for i, v := range ids {
		productIds[i] = v
	}

	// 3. Lấy thông tin Tenant/Domain (giả sử lấy từ Context hoặc Config)

	// 4. Gọi hàm logic
	products, err := getProductsData(c, tenantDB, productIds)
	if err != nil {
		log.Printf("Error fetching products: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}

	c.JSON(http.StatusOK, products)
}

func getProductsData(c *gin.Context, db *sql.DB, productIds []interface{}) ([]map[string]interface{}, error) {

	domainApi := c.Request.Host // Hoặc lấy từ biến môi trường

	tenantId, exists := c.Get("tenantId")

	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tenant context missing"})
		return nil, nil
	}

	if len(productIds) == 0 {
		return []map[string]interface{}{}, nil
	}

	// Tạo placeholders: ?,?,?
	placeholders := make([]string, len(productIds))
	for i := range productIds {
		placeholders[i] = "?"
	}

	query := fmt.Sprintf(`
        SELECT p.*, m.file_name, m.id as model_id 
        FROM products as p 
       LEFT JOIN media AS m ON m.id = (
    SELECT MAX(id) 
    FROM media 
    WHERE model_id = p.id 
      AND model_type = 'App\\Models\\Tenant\\Product' 
      AND collection_name = 'main_images'
)
        WHERE p.id IN (%s) `, strings.Join(placeholders, ","))

	utils.LogSQL(query, productIds...)
	rows, err := db.Query(query, productIds...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var productList []map[string]interface{}
	for rows.Next() {
		// Sử dụng hàm ScanRowToMap dùng chung đã viết ở trên
		product, err := ScanRowToMap(rows)
		if err != nil {
			return nil, err
		}

		// 1. Xử lý Image Link
		modelID := product["model_id"]
		fileName := product["file_name"]
		if modelID != nil && fileName != nil {
			product["image"] = fmt.Sprintf("http://%s/storage/tenancy/%s/app/public/%v/%v",
				domainApi, tenantId, modelID, fileName)
		} else {
			product["image"] = "https://tutaoweb.com/images/clothe.png"
		}

		// 2. Xử lý Product Link & Fields mặc định
		product["link"] = fmt.Sprintf("http://%s/product/%v", domainApi, product["slug"])
		product["hasOption"] = false
		product["sub_images"] = []interface{}{}

		// 3. Parse JSON cho title (tương đương JSON.parse)
		if titleStr, ok := product["title"].(string); ok {
			var titleObj interface{}
			if err := json.Unmarshal([]byte(titleStr), &titleObj); err == nil {
				product["title"] = titleObj
			}
		}

		// 1. Chuyển đổi giá từ string sang float64
		salePrice := utils.ParseToFloat(product["sale_price"])
		offerPrice := utils.ParseToFloat(product["offer_price"])

		// 2. Lấy thời gian hiện tại
		now := time.Now()

		// 3. Ép kiểu thời gian (Go tự động hiểu format RFC3339 từ MySQL)
		offerStart, _ := product["offer_start"].(time.Time)
		offerEnd, _ := product["offer_end"].(time.Time)

		hasOffer := false
		discountPercentage := 0.0

		// 4. Kiểm tra điều kiện giảm giá
		if offerPrice > 0 && offerPrice < salePrice {
			// Nếu offer_start rỗng (IsZero) hoặc đã đến ngày bắt đầu
			isStarted := offerStart.IsZero() || now.After(offerStart) || now.Equal(offerStart)

			// Nếu offer_end rỗng (IsZero) hoặc chưa đến ngày kết thúc
			isNotExpired := offerEnd.IsZero() || now.Before(offerEnd) || now.Equal(offerEnd)

			if isStarted && isNotExpired {
				hasOffer = true
				// Tính % giảm giá và làm tròn
				discountPercentage = math.Round(((salePrice - offerPrice) / salePrice) * 100)
			}
		}

		const timeLayout = "2006-01-02 15:04:05"

		if offerStart, ok := product["offer_start"].(time.Time); ok && !offerStart.IsZero() {
			product["offer_start"] = offerStart.Format(timeLayout)
		} else {
			product["offer_start"] = nil
		}

		if offerEnd, ok := product["offer_end"].(time.Time); ok && !offerEnd.IsZero() {
			product["offer_end"] = offerEnd.Format(timeLayout)
		} else {
			product["offer_end"] = nil
		}

		// --- Format giá tiền (Bỏ bớt số 0 thừa) ---
		// Chuyển "100.0000" (string) thành "100" hoặc "100.00"
		if priceStr, ok := product["offer_price"].(string); ok {
			if p, err := strconv.ParseFloat(priceStr, 64); err == nil {
				// %g sẽ tự động loại bỏ các số 0 vô nghĩa ở cuối
				product["offer_price"] = fmt.Sprintf("%.2f", p)
			}
		}

		// 5. Gán kết quả vào map
		product["has_offer"] = hasOffer
		product["discount_percentage"] = discountPercentage
		product["price"] = salePrice
		product["final_price"] = salePrice
		product["rate"] = utils.ParseToFloat(product["rating"])

		if hasOffer {
			product["final_price"] = offerPrice
		}

		// 6. Xử lý hiển thị thời gian cho Frontend (tránh trả về 0001-01-01)
		if offerStart.IsZero() {
			product["offer_start"] = nil
		}
		if offerEnd.IsZero() {
			product["offer_end"] = nil
		}
		productList = append(productList, product)
	}
	return productList, nil
}

func createProductHandler(c *gin.Context) {
	lang := c.DefaultQuery("lang", "en")

	// 1. Nhận thông tin text từ form-data (thay vì JSON)
	name := c.PostForm("name")
	price := c.PostForm("price")
	// ... lấy các field khác tương tự ...

	rawData := map[string]interface{}{
		"name":  name,
		"price": price,
		"title": c.PostForm("title"),
	}

	// 2. Insert Product vào DB (Tái sử dụng hàm core)
	data := filterAllowedFields("products", rawData)
	productID, err := executeInsert(c, "products", data, lang)
	if err != nil {
		c.JSON(500, gin.H{"error": "Lỗi lưu product"})
		return
	}

	_, err = services.HandleFileUpload(c, services.UploadParam{
		ModelType:      "App\\Models\\Tenant\\Product",
		ModelID:        productID,
		CollectionName: "main_images",
		FieldName:      "image",
	})

	if err != nil {
		// Log lỗi nhưng có thể vẫn trả về success cho product nếu ảnh không bắt buộc
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"id":      productID,
		"message": "Product created with image",
	})
}
func updateProductHandler(c *gin.Context) {
	tenantDB, err := getDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id := c.Param("id")
	rawData := make(map[string]interface{})

	// Kiểm tra Header xem client gửi gì lên
	contentType := c.GetHeader("Content-Type")

	if strings.Contains(contentType, "application/json") {
		// Trường hợp 1: Client gửi JSON (Không có ảnh)
		if err := c.ShouldBindJSON(&rawData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
			return
		}
	} else {
		// Trường hợp 2: Client gửi FormData (Có thể có ảnh)
		// Parse MultipartForm để lấy cả text fields và files
		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Form Data"})
			return
		}

		// Đưa các field text vào rawData để xử lý đồng nhất bên dưới
		for key, values := range form.Value {
			if len(values) > 0 {
				rawData[key] = values[0]
			}
		}

		// Xử lý file ảnh (nếu có)
		file, err := c.FormFile("image") // "image" là key từ phía JS gửi lên
		if err == nil {
			// Lưu file hoặc xử lý logic upload ảnh tại đây
			dst := "uploads/" + file.Filename
			c.SaveUploadedFile(file, dst)
			rawData["image_url"] = dst // Lưu đường dẫn vào map để update DB
		}
	}

	// Sau bước này, rawData đã có đầy đủ dữ liệu dù gửi qua JSON hay Form
	log.Printf("Dữ liệu nhận được để update ID %s: %v", id, rawData)

	// 1. Bắt đầu Transaction
	tx, err := tenantDB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not start transaction"})
		return
	}

	// Đảm bảo rollback nếu có lỗi xảy ra giữa chừng
	defer tx.Rollback()

	// 2. Cập nhật thông tin cơ bản của Product (Logic cũ của bạn)
	data := filterAllowedFields("products", rawData)

	logSQL("debug", len(data))
	if len(data) > 0 {
		setClauses := make([]string, 0)
		args := make([]interface{}, 0)
		for k, v := range data {
			if k == "title" || k == "description" {
				lang := "$." + c.DefaultQuery("lang", "en")
				setClauses = append(setClauses, fmt.Sprintf("`%s` = JSON_SET(IFNULL(`%s`, '{}'), ?, ?)", k, k))
				args = append(args, lang, v)
			} else {
				setClauses = append(setClauses, fmt.Sprintf("`%s` = ?", k))
				args = append(args, v)
			}
		}
		args = append(args, id)
		updateQuery := fmt.Sprintf("UPDATE products SET %s WHERE id = ?", strings.Join(setClauses, ","))

		if _, err := tx.Exec(updateQuery, args...); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
			return
		}
	}

	// 3. Xử lý cập nhật Category (Bảng trung gian category_product)
	if categoryRaw, ok := rawData["category"]; ok {
		// Ép kiểu category về mảng các ID (thường là []interface{} khi parse từ JSON)
		categoryIDs, ok := categoryRaw.([]interface{})
		if ok {
			// A. Xóa tất cả danh mục cũ của sản phẩm này
			deleteRelationQuery := "DELETE FROM category_product WHERE product_id = ?"
			if _, err := tx.Exec(deleteRelationQuery, id); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset categories"})
				return
			}

			// B. Chèn danh mục mới nếu có
			if len(categoryIDs) > 0 {
				insertQuery := "INSERT INTO category_product (product_id, category_id, created_at, updated_at) VALUES "
				insertValues := []interface{}{}
				placeholders := []string{}
				now := time.Now().Format("2006-01-02 15:04:05")

				for _, catID := range categoryIDs {
					placeholders = append(placeholders, "(?, ?, ?, ?)")
					insertValues = append(insertValues, id, catID, now, now)
				}
				insertQuery += strings.Join(placeholders, ",")

				if _, err := tx.Exec(insertQuery, insertValues...); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update categories"})
					return
				}
			}
		}
	}

	// 4. Commit Transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction commit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product and categories updated successfully"})
}
func deleteProductHandler(c *gin.Context) {
	deleteHandler(c, "products")
}

// Categories handlers
func getCategoriesHandler(c *gin.Context) {
	getHandler(c, "categories")
}
func createCategoryHandler(c *gin.Context) {
	createHandler(c, "categories")
}
func updateCategoryHandler(c *gin.Context) {
	updateHandler(c, "categories")
}
func deleteCategoryHandler(c *gin.Context) {
	deleteHandler(c, "categories")
}

// Blogs handlers
func getBlogsHandler(c *gin.Context) {
	getHandler(c, "blogs")
}
func createBlogHandler(c *gin.Context) {
	createHandler(c, "blogs")
}
func updateBlogHandler(c *gin.Context) {
	updateHandler(c, "blogs")
}
func deleteBlogHandler(c *gin.Context) {
	deleteHandler(c, "blogs")
}

// Users handlers
func getUsersHandler(c *gin.Context) {
	getHandler(c, "users")
}
func createUserHandler(c *gin.Context) {
	createHandler(c, "users")
}
func updateUserHandler(c *gin.Context) {
	updateHandler(c, "users")
}
func deleteUserHandler(c *gin.Context) {
	deleteHandler(c, "users")
}

// Pages handlers
func getPagesHandler(c *gin.Context) {
	getHandler(c, "pages")
}
func createPageHandler(c *gin.Context) {
	createHandler(c, "pages")
}
func updatePageHandler(c *gin.Context) {
	updateHandler(c, "pages")
}
func deletePageHandler(c *gin.Context) {
	deleteHandler(c, "pages")
}

// Customers handlers
func getCustomersHandler(c *gin.Context) {
	getHandler(c, "customers")
}
func createCustomerHandler(c *gin.Context) {
	createHandler(c, "customers")
}
func updateCustomerHandler(c *gin.Context) {
	updateHandler(c, "customers")
}
func deleteCustomerHandler(c *gin.Context) {
	deleteHandler(c, "customers")
}

// Settings handlers
func getSettingsHandler(c *gin.Context) {
	getHandler(c, "settings")
}
func createSettingHandler(c *gin.Context) {
	createHandler(c, "settings")
}
func updateSettingHandler(c *gin.Context) {
	updateHandler(c, "settings")
}
func deleteSettingHandler(c *gin.Context) {
	deleteHandler(c, "settings")
}

// Supplier Handlers
func getSuppliersHandler(c *gin.Context) {
	getHandler(c, "suppliers")
}
func createSupplierHandler(c *gin.Context) {
	createHandler(c, "suppliers")
}
func updateSupplierHandler(c *gin.Context) {
	updateHandler(c, "suppliers")
}
func deleteSupplierHandler(c *gin.Context) {
	deleteHandler(c, "suppliers")
}

// Transfer Handlers
func getTransfersHandler(c *gin.Context) {
	getHandler(c, "transfers")
}
func createTransferHandler(c *gin.Context) {
	createHandler(c, "transfers")
}
func updateTransferHandler(c *gin.Context) {
	updateHandler(c, "transfers")
}
func deleteTransferHandler(c *gin.Context) {
	deleteHandler(c, "transfers")
}

// Adjustment Handlers
func getAdjustmentsHandler(c *gin.Context) {
	getHandler(c, "adjustments")
}
func createAdjustmentHandler(c *gin.Context) {
	createHandler(c, "adjustments")
}
func updateAdjustmentHandler(c *gin.Context) {
	updateHandler(c, "adjustments")
}
func deleteAdjustmentHandler(c *gin.Context) {
	deleteHandler(c, "adjustments")
}

// Expense Handlers
func getExpensesHandler(c *gin.Context) {
	getHandler(c, "expenses")
}
func createExpenseHandler(c *gin.Context) {
	createHandler(c, "expenses")
}
func updateExpenseHandler(c *gin.Context) {
	updateHandler(c, "expenses")
}
func deleteExpenseHandler(c *gin.Context) {
	deleteHandler(c, "expenses")
}

func getHandler(c *gin.Context, tableName string) {
	log.Printf("getHandlert")

	if !allowedTables[tableName] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid table name"})
		return
	}

	tenantDB, err := getDBFromContext(c) // Giả định hàm này đã có của bạn
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection error"})
		return
	}

	// 1. Lấy tham số từ Query String
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "15"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage
	lang := c.DefaultQuery("lang", "en")
	searchKeyword := c.Query("search")

	// 2. Xây dựng điều kiện WHERE (Search)
	whereClause := ""
	var searchArgs []interface{}
	if searchKeyword != "" {
		likePattern := "%" + searchKeyword + "%"
		if tableName == "blogs" || tableName == "products" {
			// Search trong trường JSON Title
			whereClause = " WHERE JSON_EXTRACT(title, ?) LIKE ?"
			searchArgs = append(searchArgs, "$."+lang, likePattern)
		} else {
			// Mặc định search theo cột name cho các bảng khác
			whereClause = " WHERE name LIKE ?"
			searchArgs = append(searchArgs, likePattern)
		}
	}

	// 3. Đếm tổng số bản ghi (Phải kèm WHERE mới chính xác phân trang)
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", tableName, whereClause)
	_ = tenantDB.QueryRow(countQuery, searchArgs...).Scan(&total)

	// 4. Xây dựng Query lấy dữ liệu chính
	var query string
	var args []interface{}

	if tableName == "blogs" {
		query = fmt.Sprintf("SELECT id, slug, JSON_UNQUOTE(JSON_EXTRACT(title, ?)) as name, JSON_UNQUOTE(JSON_EXTRACT(content, ?)) as content FROM %s", tableName)
		args = append(args, "$."+lang, "$."+lang)
	} else if tableName == "products" {
		query = fmt.Sprintf("SELECT id, slug, JSON_UNQUOTE(JSON_EXTRACT(title, ?)) as name, JSON_UNQUOTE(JSON_EXTRACT(description, ?)) as description, sale_price FROM %s", tableName)
		args = append(args, "$."+lang, "$."+lang)
	} else if tableName == "categories" {
		query = fmt.Sprintf("SELECT id, slug, JSON_UNQUOTE(JSON_EXTRACT(name, ?)) as name, JSON_UNQUOTE(JSON_EXTRACT(description, ?)) as description FROM %s", tableName)
		args = append(args, "$."+lang, "$."+lang)
	} else {
		query = fmt.Sprintf("SELECT * FROM %s", tableName)
	}

	// Ghép WHERE và LIMIT/OFFSET
	if whereClause != "" {
		query += whereClause
		args = append(args, searchArgs...)
	}
	query += " LIMIT ? OFFSET ?"
	args = append(args, perPage, offset)
	logSQL(query, args...)

	rows, err := tenantDB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query execution failed"})
		return
	}
	defer rows.Close()

	// 5. Scan dữ liệu động (Map Columns)
	results := make([]map[string]interface{}, 0)
	cols, _ := rows.Columns()
	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		rows.Scan(columnPointers...)
		m := make(map[string]interface{})
		for i, colName := range cols {
			val := *(columnPointers[i].(*interface{}))
			if bytes, ok := val.([]byte); ok {
				m[colName] = string(bytes)
			} else {
				m[colName] = val
			}
		}
		results = append(results, m)
	}

	// 4. GỌI HÀM ĐÓNG GÓI
	response := utils.BuildLaravelPagination(c, results, total, page, perPage)

	c.JSON(http.StatusOK, response)
}

/*
func Paginate(c *gin.Context, db *sql.DB, baseQuery string, countQuery string, args []interface{}) (LaravelCollection, error) {
	// 1. Lấy tham số phân trang
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "15"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	// 2. Tính Total
	var total int
	err := db.QueryRowContext(c.Request.Context(), countQuery, args...).Scan(&total)
	if err != nil {
		return LaravelCollection{}, err
	}

	// 3. Lấy Data (Thêm Limit/Offset vào bản sao của args)
	dataArgs := append(args, perPage, offset)
	finalDataQuery := baseQuery + " LIMIT ? OFFSET ?"

	rows, err := db.QueryContext(c.Request.Context(), finalDataQuery, dataArgs...)
	if err != nil {
		return LaravelCollection{}, err
	}
	defer rows.Close()

	// 4. Scan động sang Map
	results := make([]map[string]interface{}, 0)
	cols, _ := rows.Columns()
	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}
		rows.Scan(columnPointers...)

		m := make(map[string]interface{})
		for i, colName := range cols {
			val := *(columnPointers[i].(*interface{}))
			if b, ok := val.([]byte); ok {
				m[colName] = string(b)
			} else {
				m[colName] = val
			}
		}
		results = append(results, m)
	}

	// 5. Build Laravel Style Response
	lastPage := int(math.Ceil(float64(total) / float64(perPage)))
	if lastPage < 1 {
		lastPage = 1
	}

	buildURL := func(p int) string {
		u, _ := url.Parse(fmt.Sprintf("https://%s%s", c.Request.Host, c.Request.URL.Path))
		q := c.Request.URL.Query()
		q.Set("page", strconv.Itoa(p))
		u.RawQuery = q.Encode()
		return u.String()
	}

	res := LaravelCollection{Data: results}
	res.Links.First, res.Links.Last = buildURL(1), buildURL(lastPage)
	if page > 1 {
		res.Links.Prev = buildURL(page - 1)
	}
	if page < lastPage {
		res.Links.Next = buildURL(page + 1)
	}

	res.Meta.CurrentPage, res.Meta.PerPage, res.Meta.Total, res.Meta.LastPage = page, perPage, total, lastPage
	res.Meta.Path = fmt.Sprintf("https://%s%s", c.Request.Host, c.Request.URL.Path)
	res.Meta.From, res.Meta.To = offset+1, offset+len(results)

	return res, nil
}
*/
// Generic CREATE handler
func createHandler(c *gin.Context, tableName string) {
	// ... validate allowedTables và get DB ...

	var rawData map[string]interface{}
	c.ShouldBindJSON(&rawData)

	data := filterAllowedFields(tableName, rawData)
	lang := c.DefaultQuery("lang", "en")

	id, err := executeInsert(c, tableName, data, lang)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id})
}

// Hàm bổ trợ thực thi Insert và trả về ID
func executeInsert(c *gin.Context, tableName string, data map[string]interface{}, lang string) (int64, error) {
	tenantDB, err := getDBFromContext(c)
	keys := make([]string, 0)
	values := make([]interface{}, 0)
	placeholders := make([]string, 0)

	for k, v := range data {
		if (k == "title" && (tableName == "blogs" || tableName == "products")) || (k == "content" && tableName == "blogs") {
			jsonVal := fmt.Sprintf("{\"%s\": \"%v\"}", lang, v)
			keys = append(keys, fmt.Sprintf("`%s`", k))
			values = append(values, jsonVal)
		} else {
			keys = append(keys, fmt.Sprintf("`%s`", k))
			values = append(values, v)
		}
		placeholders = append(placeholders, "?")
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableName, strings.Join(keys, ","), strings.Join(placeholders, ","))
	result, err := tenantDB.Exec(query, values...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// Generic UPDATE handler
func updateHandler(c *gin.Context, tableName string) {
	if !allowedTables[tableName] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid table name"})
		return
	}

	tenantDB, err := getDBFromContext(c)
	if err != nil {
		log.Printf("Error getting tenant DB from context: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID parameter is missing"})
		return
	}

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is required"})
		return
	}

	var rawData map[string]interface{}
	c.ShouldBindJSON(&rawData)

	// Lọc field
	data := filterAllowedFields(tableName, rawData)
	if len(data) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nothing to update"})
		return
	}

	setClauses := make([]string, 0)
	args := make([]interface{}, 0)

	for k, v := range data {
		// Logic JSON Update (ví dụ sử dụng JSON_SET để không mất các ngôn ngữ khác)
		if (k == "title" || k == "content") && (tableName == "blogs" || tableName == "products") {
			lang := "$." + c.DefaultQuery("lang", "en")
			setClauses = append(setClauses, fmt.Sprintf("`%s` = JSON_SET(IFNULL(`%s`, '{}'), ?, ?)", k, k))
			args = append(args, lang, v)
		} else {
			setClauses = append(setClauses, fmt.Sprintf("`%s` = ?", k))
			args = append(args, v)
		}
	}
	args = append(args, id)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = ?", tableName, strings.Join(setClauses, ","))
	log.Printf("Executing query: %s with args: %v", query, args)
	_, err = tenantDB.Exec(query, args...)
	if err != nil {
		log.Printf("Error updating %s: %v", tableName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Updated successfully"})
}

// Generic DELETE handler
func deleteHandler(c *gin.Context, tableName string) {
	if !allowedTables[tableName] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid table name"})
		return
	}

	tenantDB, err := getDBFromContext(c)
	if err != nil {
		log.Printf("Error getting tenant DB from context: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID parameter is missing"})
		return
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", tableName)
	log.Printf("Executing query: %s with args: %v", query, id)
	_, err = tenantDB.Exec(query, id)
	if err != nil {
		log.Printf("Error deleting from %s: %v", tableName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

func logSQL(query string, args ...interface{}) {
	// Lấy đường dẫn tuyệt đối của thư mục đang chạy
	pwd, _ := os.Getwd()
	path := filepath.Join(pwd, "sql_debug.log")

	// Mở file (nếu chưa có thì tạo, có rồi thì ghi tiếp)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		return
	}
	defer f.Close()

	// Format câu SQL: Thay ? bằng giá trị thực tế
	output := query
	for _, arg := range args {
		val := fmt.Sprintf("'%v'", arg)
		output = strings.Replace(output, "?", val, 1)
	}

	// Tạo nội dung log
	entry := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), output)

	// Ghi vào file và đồng thời in ra Terminal để Air bắt được
	f.WriteString(entry)
	fmt.Print("SQL_LOG >> ", entry)
}
