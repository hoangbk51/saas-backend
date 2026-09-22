package services

import (
	"go-saas/models"
	"go-saas/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type BlogCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func GetAllBlogCategories(c *gin.Context) ([]BlogCategory, error) {
	// Service tự đi lấy DB từ context
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	query := "SELECT id, name FROM blog_categories ORDER BY id ASC"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blogCategories []BlogCategory
	for rows.Next() {
		var c BlogCategory
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			continue
		}
		blogCategories = append(blogCategories, c)
	}
	return blogCategories, nil
}

func ListBlogCategoriesService(c *gin.Context) (utils.LaravelCollection, error) {
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

	search := c.Query("search")
	active := c.Query("active")
	featured := c.Query("featured")
	parentID := c.Query("parent_id")

	whereClause := "WHERE deleted_at IS NULL"
	args := []interface{}{}

	if active != "" {
		whereClause += " AND active = ?"
		args = append(args, active)
	}
	if featured != "" {
		whereClause += " AND featured = ?"
		args = append(args, featured)
	}
	if parentID != "" {
		whereClause += " AND parent_id = ?"
		args = append(args, parentID)
	}
	if search != "" {
		whereClause += " AND (name LIKE ? OR slug LIKE ?)"
		args = append(args, "%"+search+"%", "%"+search+"%")
	}

	// 1. Tính tổng bản ghi
	var total int
	countQuery := "SELECT COUNT(*) FROM blog_categories " + whereClause
	if err := db.GetContext(c.Request.Context(), &total, countQuery, args...); err != nil {
		return utils.LaravelCollection{}, err
	}

	// 2. Query dữ liệu
	dataQuery := `
		SELECT 
			id, parent_id, COALESCE(name, '') as name, COALESCE(slug, '') as slug, 
			COALESCE(description, '') as description, active, COALESCE(featured, 0) as featured, 
			` + "`left`" + `, ` + "`right`" + `, created_at, updated_at
		FROM blog_categories ` + whereClause + ` ORDER BY id DESC LIMIT ? OFFSET ?`

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
			sanitizeByteMap(row)
			results = append(results, row)
		}
	}

	return utils.BuildLaravelPagination(c, results, total, page, perPage), nil
}

func GetBlogCategoryDetailService(c *gin.Context, id int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	query := `
		SELECT 
			id, parent_id, COALESCE(name, '') as name, COALESCE(slug, '') as slug, 
			COALESCE(description, '') as description, active, COALESCE(featured, 0) as featured, 
			` + "`left`" + `, ` + "`right`" + `, created_at, updated_at
		FROM blog_categories 
		WHERE id = ? AND deleted_at IS NULL LIMIT 1`

	rows, err := db.QueryxContext(c.Request.Context(), query, id)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	if !rows.Next() {
		return gin.H{"error": "Danh mục bài viết không tồn tại"}, http.StatusNotFound, nil
	}

	result := make(map[string]interface{})
	rows.MapScan(result)
	sanitizeByteMap(result)

	return result, http.StatusOK, nil
}

func CreateBlogCategoryService(c *gin.Context) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.BlogCategoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	query := `
		INSERT INTO blog_categories (
			parent_id, name, slug, description, active, featured, 
			` + "`left`" + `, ` + "`right`" + `, created_at, updated_at
		) VALUES (?, ?, ?, ?, COALESCE(?, 1), COALESCE(?, 0), COALESCE(?, 0), COALESCE(?, 0), NOW(), NOW())`

	res, err := db.ExecContext(
		c.Request.Context(), query,
		input.ParentID, input.Name, input.Slug, input.Description,
		input.Active, input.Featured, input.Left, input.Right,
	)
	if err != nil {
		return gin.H{"error": "Không thể tạo danh mục bài viết: " + err.Error()}, http.StatusInternalServerError, err
	}

	id, _ := res.LastInsertId()
	return gin.H{"message": "Tạo danh mục bài viết thành công", "id": id}, http.StatusCreated, nil
}

func UpdateBlogCategoryService(c *gin.Context, id int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.BlogCategoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	query := `
		UPDATE blog_categories 
		SET parent_id = ?, name = ?, slug = ?, description = ?, 
		    active = COALESCE(?, active), 
		    featured = COALESCE(?, featured),
		    ` + "`left`" + ` = COALESCE(?, ` + "`left`" + `),
		    ` + "`right`" + ` = COALESCE(?, ` + "`right`" + `),
		    updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL`

	res, err := db.ExecContext(
		c.Request.Context(), query,
		input.ParentID, input.Name, input.Slug, input.Description,
		input.Active, input.Featured, input.Left, input.Right, id,
	)
	if err != nil {
		return gin.H{"error": "Lỗi cập nhật danh mục bài viết: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Danh mục bài viết không tồn tại"}, http.StatusNotFound, nil
	}

	return gin.H{"message": "Cập nhật danh mục bài viết thành công"}, http.StatusOK, nil
}

func DeleteBlogCategoryService(c *gin.Context, id int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	query := `UPDATE blog_categories SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL`
	res, err := db.ExecContext(c.Request.Context(), query, id)
	if err != nil {
		return gin.H{"error": "Lỗi khi xóa danh mục bài viết: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Danh mục bài viết không tồn tại hoặc đã bị xóa"}, http.StatusNotFound, nil
	}

	return gin.H{"message": "Xóa danh mục bài viết thành công"}, http.StatusOK, nil
}
