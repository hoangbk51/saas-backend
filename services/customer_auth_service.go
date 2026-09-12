package services

import (
	"fmt"
	"go-saas/tasks"
	"go-saas/utils"
	"log"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// Cấu trúc nhận dữ liệu từ Client
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Hàm tạo random string cho token (giống Sanctum)

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone" binding:"required,numeric,min=10,max=11"`
	Password string `json:"password" binding:"required,min=6"`
}

func CustomerRegisterHandler(c *gin.Context) error {
	var req RegisterRequest
	// 1. Validate (Tương đương $this->validator->validate())
	if err := c.ShouldBindJSON(&req); err != nil {
		//	c.JSON(http.StatusOK, gin.H{"status": "error", "message": err.Error()})
		return fmt.Errorf("Lỗi input")

	}

	tenantDB, _ := utils.GetDBFromContext(c)

	// 2. Hash mật khẩu (bcrypt)
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	verificationToken := utils.GenerateRandomString(20) // 20 bytes hex = 40 chars

	// 3. Insert vào Database
	query := `INSERT INTO customers (name, email, password,phone, verification_token, created_at, updated_at) 
              VALUES (?, ?, ?,?, ?, NOW(), NOW())`

	utils.LogSQL(query, []interface{}{req.Name, req.Email, "******", verificationToken})

	_, err := tenantDB.Exec(query, req.Name, req.Email, string(hashedPassword), req.Phone, verificationToken)
	if err != nil {
		return fmt.Errorf("Email đã tồn tại hoặc lỗi DB")
	}

	// Lấy ID vừa tạo để gửi kèm vào email nếu cần
	//customerID, _ := res.LastInsertId()

	// 4. XỬ LÝ EVENT GỬI MAIL (Bất đồng bộ - Asynchronous)
	// Sử dụng từ khóa 'go' để chạy hàm này ở background, không làm chậm request

	// 1. Đổi sang map[string]any
	replacements := map[string]any{
		"Name":  req.Name,
		"Email": req.Email,
		"Link":  fmt.Sprintf("http://hoangk53.central.test/api/v2/verify?token=%s", verificationToken),
	}

	err = tasks.EnqueueDynamicEmail(
		utils.AsynqClient,
		c.GetString("tenantId"),
		req.Email,
		"verify_registration",
		replacements,
	)

	if err != nil {
		log.Printf("[EMAIL ENQUEUE ERROR] %v", err)
		return err
	}

	//_ = tasks.EnqueueDynamicEmail(utils.AsynqClient, req.Email, "verify_registration", replacements)
	/*
		// 2. Chạy goroutine (Sử dụng c.Copy() vì dùng Context trong Goroutine là bắt buộc để tránh crash)
		ctxCopy := c.Copy()
		go func(email string, reps map[string]any) {
			mailService := utils.NewMailConfig()

			// Truyền reps (kiểu map[string]any) vào hàm
			err := mailService.SendDynamicEmail(ctxCopy, email, "verify_registration", reps)

			if err != nil {
				log.Printf("Lỗi gửi mail động cho %s: %v", email, err)
			}
		}(req.Email, replacements)
	*/
	return nil
}

type CustomerLoginResponse struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	Password     string   `json:"-"` // Không trả về password trong JSON
	OrderCount   int      `json:"orders_count"`
	Avatar       string   `json:"avatar"`
	FinalToken   string   `json:"-"`
	IsSuperAdmin bool     `json:"is_super_admin"`
	Permissions  []string `json:"permissions"` // Danh sách plugin codes cho client phân quyền
}

func CustomerLoginHandler(c *gin.Context) (*CustomerLoginResponse, error) {
	var req LoginRequest
	// 1. Validate dữ liệu đầu vào
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, fmt.Errorf("Email và Password là bắt buộc")

	}

	tenantDB, _ := utils.GetDBFromContext(c)
	customer := &CustomerLoginResponse{}

	// 2. Tìm Customer theo email
	/*
		var customer struct {
			ID         int    `json:"id"`
			Name       string `json:"name"`
			Email      string `json:"email"`
			Password   string `json:"-"` // Không trả về password trong JSON
			OrderCount int    `json:"orders_count"`
			Avatar     string `json:"avatar"`
		}
	*/
	query := `SELECT id, name, email, password, 
              (SELECT COUNT(*) FROM orders WHERE customer_id = customers.id) as orders_count 
              FROM customers WHERE email = ? LIMIT 1`

	// --- LOG SQL CHO BƯỚC TÌM CUSTOMER ---
	queryArgs := []interface{}{req.Email}
	utils.LogSQL(query, queryArgs...)

	err := tenantDB.QueryRow(query, req.Email).Scan(
		&customer.ID, &customer.Name, &customer.Email, &customer.Password, &customer.OrderCount,
	)

	if err != nil {
		// Tương đương Auth::attempt thất bại
		c.JSON(200, gin.H{"success": false, "status_code": 200, "message": "Unauthorized"})
		return nil, fmt.Errorf("Unauthorized")

	}

	// 3. Kiểm tra mật khẩu (BCrypt)
	// Laravel lưu mật khẩu bằng BCrypt, Go dùng thư viện tương đương để so khớp
	err = bcrypt.CompareHashAndPassword([]byte(customer.Password), []byte(req.Password))
	if err != nil {
		c.JSON(200, gin.H{"success": false, "status_code": 200, "message": "Unauthorized"})
		return nil, fmt.Errorf("Unauthorized")

	}

	// 4. Tạo Token (Giống logic Sanctum createToken)
	plainToken := utils.GenerateRandomString(20) // Phần chữ ngẫu nhiên
	hashedToken := utils.HashToken(plainToken)   // Hàm SHA256 bạn đã viết trước đó

	insertQuery := `
        INSERT INTO personal_access_tokens 
        (tokenable_type, tokenable_id, name, token, abilities, created_at, updated_at) 
        VALUES (?, ?, ?, ?, ?, NOW(), NOW())`

	// --- LOG SQL CHO BƯỚC INSERT TOKEN ---
	insertArgs := []interface{}{
		"App\\Models\\Tenant\\Customer",
		customer.ID,
		"authToken",
		hashedToken,
		"[\"*\"]",
	}
	utils.LogSQL(insertQuery, insertArgs...)

	// Lưu vào bảng personal_access_tokens
	res, err := tenantDB.Exec(insertQuery, insertArgs...)

	if err != nil {
		c.JSON(200, gin.H{"success": false, "message": "Error creating token"})
		return nil, fmt.Errorf("Error creating token")
	}

	lastID, _ := res.LastInsertId()
	// Token cuối cùng trả về dạng ID|PlainToken (Laravel style)
	customer.FinalToken = fmt.Sprintf("%d|%s", lastID, plainToken)

	// 5. Xóa giỏ hàng trùng IP (Hàm phụ bên dưới)
	deleteCartSameIp(c, customer.ID, c.ClientIP())

	return customer, nil

}

func deleteCartSameIp(c *gin.Context, customerID int, ipAddress string) {
	db, _ := utils.GetDBFromContext(c)

	// Tìm giỏ hàng hiện tại của customer để lấy IP
	var currentCartIP string
	err := db.QueryRow("SELECT ip_address FROM carts WHERE customer_id = ? LIMIT 1", customerID).Scan(&currentCartIP)

	if err == nil && currentCartIP != "" {
		// Tìm các giỏ hàng vãng lai (customer_id IS NULL) trùng IP
		// Sau đó xóa Item và xóa Cart (Laravel logic)

		// 1. Xóa items trước (Tránh lỗi khóa ngoại nếu có)
		db.Exec(`DELETE FROM cart_items WHERE cart_id IN 
                 (SELECT id FROM carts WHERE ip_address = ? AND customer_id IS NULL)`, currentCartIP)

		// 2. Xóa giỏ hàng vãng lai
		db.Exec("DELETE FROM carts WHERE ip_address = ? AND customer_id IS NULL", currentCartIP)
		utils.LogSQL("Cleaned guest carts for IP: ?", currentCartIP)

	}
}
