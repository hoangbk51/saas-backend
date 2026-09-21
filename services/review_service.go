package services

import (
	"go-saas/models"
	"go-saas/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func CreateReview(c *gin.Context, input models.ReviewInput) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return err
	}

	query := `INSERT INTO reviews (product_id, comment, customer_id, rating, created_at) 
			  VALUES (?, ?, ?, ?, NOW())`

	args := []interface{}{input.ProductID, input.Comment, input.CustomerID, input.Rating}

	// Ghi log SQL để debug
	utils.LogSQL(query, args...)

	_, err = db.Exec(query, args...)
	return err
}

func ListReviewsService(c *gin.Context) (utils.LaravelCollection, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return utils.LaravelCollection{}, err
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "15"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	// Filters
	productID := c.Query("product_id")
	customerID := c.Query("customer_id")
	approved := c.Query("approved")
	spam := c.Query("spam")
	search := c.Query("search")

	whereClause := "WHERE 1=1"
	args := []interface{}{}

	if productID != "" {
		whereClause += " AND r.product_id = ?"
		args = append(args, productID)
	}
	if customerID != "" {
		whereClause += " AND r.customer_id = ?"
		args = append(args, customerID)
	}
	if approved != "" {
		whereClause += " AND r.approved = ?"
		args = append(args, approved)
	}
	if spam != "" {
		whereClause += " AND r.spam = ?"
		args = append(args, spam)
	}
	if search != "" {
		whereClause += " AND r.comment LIKE ?"
		args = append(args, "%"+search+"%")
	}

	// 1. Tính tổng bản ghi
	var total int
	countQuery := "SELECT COUNT(*) FROM reviews r " + whereClause
	if err := db.GetContext(c.Request.Context(), &total, countQuery, args...); err != nil {
		return utils.LaravelCollection{}, err
	}

	// 2. Query lấy danh sách
	dataQuery := `
		SELECT 
			r.id, r.customer_id, r.product_id, 
			COALESCE(r.rating, 0) AS rating, 
			COALESCE(r.comment, '') AS comment, 
			r.approved, r.spam, r.created_at, r.updated_at,
			COALESCE(c.name, '') AS customer_name,
			COALESCE(p.title, '') AS product_title
		FROM reviews r
		LEFT JOIN customers c ON r.customer_id = c.id
		LEFT JOIN products p ON r.product_id = p.id
		` + whereClause + ` ORDER BY r.created_at DESC LIMIT ? OFFSET ?`

	queryArgs := append(args, perPage, offset)
	utils.LogSQL(dataQuery, queryArgs...)

	results := []map[string]interface{}{}
	rows, err := db.QueryxContext(c.Request.Context(), dataQuery, queryArgs...)
	if err != nil {
		return utils.LaravelCollection{}, err
	}
	defer rows.Close()

	for rows.Next() {
		row := make(map[string]interface{})
		if err := rows.MapScan(row); err == nil {
			for k, v := range row {
				if b, ok := v.([]byte); ok {
					row[k] = string(b)
				}
			}
			results = append(results, row)
		}
	}

	return utils.BuildLaravelPagination(c, results, total, page, perPage), nil
}

func GetReviewDetailService(c *gin.Context, reviewID int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	query := `
		SELECT 
			r.id, r.customer_id, r.product_id, 
			COALESCE(r.rating, 0) AS rating, 
			COALESCE(r.comment, '') AS comment, 
			r.approved, r.spam, r.created_at, r.updated_at,
			COALESCE(c.name, '') AS customer_name,
			COALESCE(p.title, '') AS product_title
		FROM reviews r
		LEFT JOIN customers c ON r.customer_id = c.id
		LEFT JOIN products p ON r.product_id = p.id
		WHERE r.id = ? LIMIT 1`

	rows, err := db.QueryxContext(c.Request.Context(), query, reviewID)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	if !rows.Next() {
		return gin.H{"error": "Đánh giá không tồn tại"}, http.StatusNotFound, nil
	}

	result := make(map[string]interface{})
	if err := rows.MapScan(result); err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	for k, v := range result {
		if b, ok := v.([]byte); ok {
			result[k] = string(b)
		}
	}

	return result, http.StatusOK, nil
}

func UpdateReviewService(c *gin.Context, reviewID int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.ReviewUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	query := `
		UPDATE reviews 
		SET rating = COALESCE(?, rating),
		    comment = COALESCE(?, comment),
		    approved = COALESCE(?, approved),
		    spam = COALESCE(?, spam),
		    updated_at = NOW()
		WHERE id = ?`

	res, err := db.ExecContext(c.Request.Context(), query, input.Rating, input.Comment, input.Approved, input.Spam, reviewID)
	if err != nil {
		return gin.H{"error": "Lỗi cập nhật đánh giá: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Đánh giá không tồn tại"}, http.StatusNotFound, nil
	}

	return gin.H{"message": "Cập nhật đánh giá thành công"}, http.StatusOK, nil
}

func DeleteReviewService(c *gin.Context, reviewID int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	query := `DELETE FROM reviews WHERE id = ?`
	res, err := db.ExecContext(c.Request.Context(), query, reviewID)
	if err != nil {
		return gin.H{"error": "Không thể xóa đánh giá: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Đánh giá không tồn tại hoặc đã bị xóa trước đó"}, http.StatusNotFound, nil
	}

	return gin.H{"message": "Xóa đánh giá thành công"}, http.StatusOK, nil
}

func CreateReviewService(c *gin.Context, customerID int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.ReviewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	query := `
		INSERT INTO reviews (customer_id, product_id, rating, comment, approved, spam, created_at, updated_at) 
		VALUES (?, ?, ?, ?, 1, 0, NOW(), NOW())`

	res, err := db.ExecContext(c.Request.Context(), query, customerID, input.ProductID, input.Rating, input.Comment)
	if err != nil {
		return gin.H{"error": "Không thể gửi đánh giá: " + err.Error()}, http.StatusInternalServerError, err
	}

	reviewID, _ := res.LastInsertId()
	return gin.H{"message": "Gửi đánh giá thành công", "id": reviewID}, http.StatusCreated, nil
}
