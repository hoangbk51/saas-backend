package services

import (
	"fmt"
	"time"

	"go-saas/models"
	"go-saas/tasks"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

type ReturnService struct{}

func NewReturnService() *ReturnService {
	return &ReturnService{}
}

// Tạo mới yêu cầu đổi/trả
func (s *ReturnService) CreateReturnRequest(c *gin.Context, customerID int, req *models.CreateReturnRequest) (int64, string, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return 0, "", fmt.Errorf("không thể kết nối CSDL")
	}
	ctx := c.Request.Context()

	// 1. Kiểm tra tính hợp lệ của đơn hàng
	var orderStatus string
	var deliveredAt *time.Time
	checkOrderQuery := `SELECT delivery_status, created_at FROM orders WHERE id = ? AND customer_id = ? AND deleted_at IS NULL`
	utils.LogSQL(checkOrderQuery, req.OrderID, customerID)
	err = db.QueryRowContext(ctx, checkOrderQuery, req.OrderID, customerID).Scan(&orderStatus, &deliveredAt)
	if err != nil {
		return 0, "", fmt.Errorf("đơn hàng không tồn tại hoặc không thuộc quyền sở hữu của bạn")
	}

	if orderStatus != "delivered" {
		//return 0, "", fmt.Errorf("chỉ có thể tạo yêu cầu đổi/trả đối với đơn hàng đã giao thành công")
	}

	if deliveredAt != nil && time.Since(*deliveredAt) > 7*24*time.Hour {
		return 0, "", fmt.Errorf("đã quá hạn 7 ngày cho phép đổi trả sản phẩm")
	}

	// 2. Kiểm tra yêu cầu đang chờ xử lý
	var pendingCount int
	_ = db.GetContext(ctx, &pendingCount, `SELECT COUNT(*) FROM return_requests WHERE order_id = ? AND status IN ('pending', 'approved')`, req.OrderID)
	if pendingCount > 0 {
		return 0, "", fmt.Errorf("đơn hàng này đã có yêu cầu đổi/trả đang được xử lý")
	}

	// 3. Thực thi Transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, "", fmt.Errorf("lỗi khởi tạo giao dịch DB")
	}
	defer tx.Rollback()

	returnCode := fmt.Sprintf("RET-%d-%d", req.OrderID, time.Now().Unix())

	insertReturnQuery := `INSERT INTO return_requests (code, order_id, customer_id, type, reason, note, status, created_at) 
                          VALUES (?, ?, ?, ?, ?, ?, 'pending', NOW())`
	res, err := tx.ExecContext(ctx, insertReturnQuery, returnCode, req.OrderID, customerID, req.Type, req.Reason, req.Note)
	if err != nil {
		return 0, "", fmt.Errorf("lỗi lưu yêu cầu đổi/trả: %v", err)
	}

	returnID, _ := res.LastInsertId()

	insertItemQuery := `INSERT INTO return_items (return_request_id, order_item_id, quantity, reason) VALUES (?, ?, ?, ?)`
	for _, item := range req.Items {
		_, err := tx.ExecContext(ctx, insertItemQuery, returnID, item.OrderItemID, item.Quantity, item.Reason)
		if err != nil {
			return 0, "", fmt.Errorf("lỗi lưu chi tiết sản phẩm đổi/trả: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, "", fmt.Errorf("lỗi commit transaction: %v", err)
	}

	// 4. Upload file bằng chứng (nếu có)
	_, _ = HandleFileUpload(c, UploadParam{
		ModelType:      "App\\Models\\Tenant\\ReturnRequest",
		ModelID:        returnID,
		CollectionName: "return_proofs",
		FieldName:      "images",
	})

	return returnID, returnCode, nil
}

// Lấy danh sách yêu cầu của Customer
func (s *ReturnService) GetCustomerReturns(c *gin.Context, customerID int64) ([]models.ReturnRequest, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("không thể kết nối CSDL")
	}

	var list []models.ReturnRequest
	query := `SELECT id, code, order_id, customer_id, type, reason, status, created_at, updated_at 
              FROM return_requests 
              WHERE customer_id = ? 
              ORDER BY id DESC`

	err = db.SelectContext(c.Request.Context(), &list, query, customerID)
	return list, err
}

// Lấy chi tiết yêu cầu đổi/trả
func (s *ReturnService) GetCustomerReturnDetail(c *gin.Context, customerID int64, returnID string) (*models.ReturnDetailResponse, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("không thể kết nối CSDL")
	}
	ctx := c.Request.Context()

	var detail models.ReturnRequest
	query := `SELECT * FROM return_requests WHERE id = ? AND customer_id = ?`
	if err := db.GetContext(ctx, &detail, query, returnID, customerID); err != nil {
		return nil, fmt.Errorf("không tìm thấy yêu cầu đổi/trả")
	}

	var items []models.ReturnItem
	_ = db.SelectContext(ctx, &items, `SELECT * FROM return_items WHERE return_request_id = ?`, detail.ID)

	var images []string
	_ = db.SelectContext(ctx, &images, `SELECT url FROM media WHERE model_id = ? AND model_type = 'App\\Models\\Tenant\\ReturnRequest'`, detail.ID)

	return &models.ReturnDetailResponse{
		ReturnRequest: detail,
		Items:         items,
		ProofImages:   images,
	}, nil
}

// 1. Get All Return Requests (Có phân trang & filter status)
func (s *ReturnService) GetAllAdminRefund(c *gin.Context, status string, limit, offset int) ([]models.ReturnRequest, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, 0, fmt.Errorf("lỗi kết nối DB: %w", err)
	}

	requests := []models.ReturnRequest{}
	var total int

	baseQuery := `FROM return_requests WHERE deleted_at IS NULL`
	var args []any

	if status != "" {
		baseQuery += ` AND status = ?`
		args = append(args, status)
	}

	// Count total
	countQuery := `SELECT COUNT(*) ` + baseQuery
	if err := tenantDB.Get(&total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	// Select data
	selectQuery := `SELECT id, order_id, customer_id, reason, note, status, admin_note, created_at, updated_at ` +
		baseQuery + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	if err := tenantDB.Select(&requests, selectQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("lỗi truy vấn return_requests: %w", err)
	}

	return requests, total, nil
}

// 2. Get By ID
func (s *ReturnService) GetAdminRefundByID(c *gin.Context, id uint32) (*models.ReturnRequest, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("lỗi kết nối DB: %w", err)
	}

	var req models.ReturnRequest
	query := `
		SELECT id, order_id, customer_id, reason, note, status, admin_note, created_at, updated_at 
		FROM return_requests 
		WHERE id = ? AND deleted_at IS NULL 
		LIMIT 1
	`
	if err := tenantDB.Get(&req, query, id); err != nil {
		return nil, fmt.Errorf("không tìm thấy yêu cầu trả hàng ID %d: %w", id, err)
	}

	return &req, nil
}

// 3. Create (Tạo mới yêu cầu trả hàng)
func (s *ReturnService) CreateAdminRefund(c *gin.Context, req models.CreateReturnRequestReq) (int64, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return 0, fmt.Errorf("lỗi kết nối DB: %w", err)
	}
	returnCode := fmt.Sprintf("RET-%d-%d", req.OrderID, time.Now().Unix())

	now := time.Now()
	query := `
		INSERT INTO return_requests (code,order_id, customer_id, reason, note, status, created_at, updated_at)
		VALUES (?,?, ?, ?, ?, 'PENDING', ?, ?)
	`

	res, err := tenantDB.Exec(query, returnCode, req.OrderID, req.CustomerID, req.Reason, req.Note, now, now)
	if err != nil {
		return 0, fmt.Errorf("lỗi tạo yêu cầu trả hàng: %w", err)
	}

	return res.LastInsertId()
}

// 4. Update Status & Admin Note (Admin duyệt/từ chối/hoàn tất)
func (s *ReturnService) UpdateAdminRefund(c *gin.Context, id uint32, req models.UpdateReturnRequestReq) error {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return fmt.Errorf("lỗi kết nối DB: %w", err)
	}

	now := time.Now()
	query := `
		UPDATE return_requests 
		SET status = ?, admin_note = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	res, err := tenantDB.Exec(query, req.Status, req.AdminNote, now, id)
	if err != nil {
		return fmt.Errorf("lỗi cập nhật yêu cầu trả hàng: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("không tìm thấy bản ghi để cập nhật")
	}

	return nil
}

// 5. Delete (Soft Delete)
func (s *ReturnService) DeleteAdminRefund(c *gin.Context, id uint32) error {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return fmt.Errorf("lỗi kết nối DB: %w", err)
	}

	now := time.Now()
	query := `UPDATE return_requests SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`

	res, err := tenantDB.Exec(query, now, id)
	if err != nil {
		return fmt.Errorf("lỗi xóa yêu cầu trả hàng: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("bản ghi không tồn tại hoặc đã bị xóa")
	}

	return nil
}

func (s *ReturnService) AcceptReturn(c *gin.Context, id uint32, adminNote string) error {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return fmt.Errorf("lỗi kết nối DB: %w", err)
	}

	// 1. Kiểm tra tồn tại và trạng thái của Return Request
	var req models.ReturnRequest
	queryGet := `SELECT id, order_id, customer_id, status FROM return_requests WHERE id = ? AND deleted_at IS NULL LIMIT 1`
	if err := tenantDB.Get(&req, queryGet, id); err != nil {
		return fmt.Errorf("không tìm thấy yêu cầu trả hàng ID %d: %w", id, err)
	}

	if req.Status != "pending" {
		return fmt.Errorf("yêu cầu trả hàng đã được xử lý trước đó (Trạng thái hiện tại: %s)", req.Status)
	}

	// 2. Cập nhật trạng thái thành APPROVED
	now := time.Now()
	queryUpdate := `
		UPDATE return_requests 
		SET status = 'approved', admin_note = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`
	res, err := tenantDB.Exec(queryUpdate, adminNote, now, id)
	if err != nil {
		return fmt.Errorf("lỗi cập nhật trạng thái: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("không thể cập nhật trạng thái yêu cầu trả hàng")
	}

	// 3. (Optional) Lấy email khách hàng và Enqueue Task gửi Mail thông báo duyệt trả hàng
	var customerEmail string
	_ = tenantDB.Get(&customerEmail, "SELECT email FROM customers WHERE id = ? LIMIT 1", req.CustomerID)

	if customerEmail != "" && utils.AsynqClient != nil {
		reps := map[string]any{
			"ReturnID":  req.ID,
			"OrderID":   req.OrderID,
			"AdminNote": adminNote,
		}
		_ = tasks.EnqueueDynamicEmail(
			utils.AsynqClient,
			c.GetString("tenantId"),
			customerEmail,
			"return_request_approved",
			reps,
		)
	}

	return nil
}
