package services

import (
	"go-saas/models"
	"go-saas/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

func AddCustomerHistoryService(c *gin.Context, customerID int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.CustomerHistoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Nội dung ghi chú không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	query := `
		INSERT INTO customer_histories (customer_id, comment, created_at, updated_at) 
		VALUES (?, ?, NOW(), NOW())`

	res, err := db.ExecContext(c.Request.Context(), query, customerID, input.Comment)
	if err != nil {
		return gin.H{"error": "Không thể thêm ghi chú: " + err.Error()}, http.StatusInternalServerError, err
	}

	historyID, _ := res.LastInsertId()
	return gin.H{"message": "Thêm ghi chú thành công", "customer_history_id": historyID}, http.StatusCreated, nil
}

func UpdateCustomerHistoryService(c *gin.Context, historyID int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.CustomerHistoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Nội dung ghi chú không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	query := `
		UPDATE customer_histories 
		SET comment = ?, updated_at = NOW() 
		WHERE customer_history_id = ?`

	res, err := db.ExecContext(c.Request.Context(), query, input.Comment, historyID)
	if err != nil {
		return gin.H{"error": "Lỗi cập nhật ghi chú: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Ghi chú không tồn tại"}, http.StatusNotFound, nil
	}

	return gin.H{"message": "Cập nhật ghi chú thành công"}, http.StatusOK, nil
}

func DeleteCustomerHistoryService(c *gin.Context, historyID int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	query := `DELETE FROM customer_histories WHERE customer_history_id = ?`
	res, err := db.ExecContext(c.Request.Context(), query, historyID)
	if err != nil {
		return gin.H{"error": "Không thể xóa ghi chú: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Ghi chú không tồn tại hoặc đã bị xóa trước đó"}, http.StatusNotFound, nil
	}

	return gin.H{"message": "Xóa ghi chú thành công"}, http.StatusOK, nil
}
