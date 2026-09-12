package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

func GetCategories(c *gin.Context) ([]models.Category, error) {
	db, _ := utils.GetDBFromContext(c)
	var categories []models.Category

	query := `SELECT id, COALESCE(parent_id, 0), COALESCE(name, ''), slug, 
              COALESCE(description, ''), active, COALESCE(featured, 0), created_at 
              FROM categories WHERE deleted_at IS NULL ORDER BY id DESC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cat models.Category
		err := rows.Scan(&cat.ID, &cat.ParentID, &cat.Name, &cat.Slug,
			&cat.Description, &cat.Active, &cat.Featured, &cat.CreatedAt)
		if err != nil {
			continue
		}
		categories = append(categories, cat)
	}
	return categories, nil
}

func CreateCategory(c *gin.Context, input models.CategoryPayload) error {
	db, _ := utils.GetDBFromContext(c)
	categorySlug := utils.MakeSlug(utils.GetPrimaryName(input.Name, "vi"))

	// 1. Chuyển map Name sang chuỗi JSON
	nameJSON, err := json.Marshal(input.Name)
	if err != nil {
		return fmt.Errorf("Lỗi format dữ liệu Name")
	}

	// 2. Chuyển map Description sang chuỗi JSON (nếu Description cũng là map)
	descriptionJSON, err := json.Marshal(input.Description)
	if err != nil {
		return fmt.Errorf("Lỗi format dữ liệu Description")
	}

	query := `INSERT INTO categories (parent_id, name, slug, description, active, featured, created_at, updated_at) 
              VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())`

	_, err = db.ExecContext(c.Request.Context(), query,
		input.ParentID,
		string(nameJSON),
		categorySlug,
		string(descriptionJSON),
		input.Active,
		input.Featured,
	)
	return err
}

func GetCategoryDetail(c *gin.Context, id int64) (*models.Category, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	cat := &models.Category{}

	// Query lấy chi tiết và Join lấy tên Parent
	query := `
        SELECT 
            c1.id, 
            COALESCE(c1.parent_id, 0), 
			JSON_UNQUOTE(JSON_EXTRACT(c1.name, '$.en')) , 
            COALESCE(c1.slug, ''), 
            COALESCE(c1.description, ''), 
            c1.active, 
            COALESCE(c1.featured, 0), 
            c1.created_at, 
            c1.updated_at
        FROM categories c1
        LEFT JOIN categories c2 ON c1.parent_id = c2.id
        WHERE c1.id = ? AND c1.deleted_at IS NULL
        LIMIT 1
    `

	err = db.QueryRow(query, id).Scan(
		&cat.ID,
		&cat.ParentID,
		&cat.Name,
		&cat.Slug,
		&cat.Description,
		&cat.Active,
		&cat.Featured,
		&cat.CreatedAt,
		&cat.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("danh mục không tồn tại")
		}
		return nil, err
	}

	return cat, nil
}

func GetCategoryTree(c *gin.Context) ([]*models.Category, error) {
	db, err := utils.GetDBFromContext(c)

	var flatCategories []models.Category
	query := `SELECT id, parent_id,JSON_UNQUOTE(JSON_EXTRACT(name, '$.en')) as name, slug, description, active, featured, created_at, updated_at 
              FROM categories WHERE deleted_at IS NULL`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 2. Scan từng dòng vào struct
	for rows.Next() {
		var c models.Category
		// Thứ tự biến trong Scan PHẢI đúng với thứ tự cột trong câu SELECT
		err := rows.Scan(
			&c.ID,
			&c.ParentID,
			&c.Name,
			&c.Slug,
			&c.Description,
			&c.Active,
			&c.Featured,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		flatCategories = append(flatCategories, c)
	}

	// Chuyển đổi sang dạng cây
	tree := BuildCategoryTree(flatCategories)

	return tree, nil

}

func BuildCategoryTree(categories []models.Category) []*models.Category {
	// 1. Chuyển toàn bộ slice data sang map chứa con trỏ để thao tác an toàn
	nodes := make(map[uint32]*models.Category)
	var tree []*models.Category

	for i := range categories {
		// Đảm bảo khởi tạo slice rỗng chống trả về null trong JSON
		categories[i].Children = []*models.Category{}

		// Lưu địa chỉ thực tế của từng phần tử vào map
		nodes[categories[i].ID] = &categories[i]
	}

	// 2. Xây dựng cấu trúc cây lồng nhau
	for i := range categories {
		cat := &categories[i]

		// 💡 SỬA TẠI ĐÂY: Check xem nếu ParentID là nil HOẶC trỏ đến giá trị bằng 0
		if cat.ParentID == nil || *cat.ParentID == 0 {
			tree = append(tree, cat)
		} else {
			parentIDVal := *cat.ParentID
			// Tìm node cha trong map để đẩy node hiện tại vào mảng con (Children)
			if parent, ok := nodes[parentIDVal]; ok {
				parent.Children = append(parent.Children, cat)
			} else {
				// Nếu có parent_id khác 0 nhưng không tìm thấy cha trong DB (data lỗi)
				// thì tạm thời đưa nó ra làm nút gốc để không bị mất dữ liệu
				tree = append(tree, cat)
			}
		}
	}

	// Nếu kết quả cây vẫn trống, trả về mảng rỗng [] thay vì để null
	if tree == nil {
		return []*models.Category{}
	}

	return tree
}
