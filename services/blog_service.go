package services

import (
	"database/sql"
	"fmt"
	"go-saas/models"
	"go-saas/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

type Blog struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func GetAllBlogs(c *gin.Context) ([]Blog, error) {
	// Service tự đi lấy DB từ context
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	query := "SELECT id, name FROM blogs ORDER BY id ASC"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blogs []Blog
	for rows.Next() {
		var c Blog
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			continue
		}
		blogs = append(blogs, c)
	}
	return blogs, nil
}

func GetBlogByIds(c *gin.Context) ([]Blog, error) {
	ids := c.QueryArray("ids[]")

	// Chuyển []string sang []interface{} để truyền vào hàm Query SQL
	blogIds := make([]interface{}, len(ids))
	for i, v := range ids {
		blogIds[i] = v
	}

	// Tạo placeholders: ?,?,?
	placeholders := make([]string, len(blogIds))
	for i := range blogIds {
		placeholders[i] = "?"
	}

	// Service tự đi lấy DB từ context
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
       SELECT id, title as name FROM blogs as b
        WHERE b.id IN (%s)  ORDER BY name ASC`, strings.Join(placeholders, ","))
	utils.LogSQL(query, blogIds...)
	rows, err := db.Query(query, blogIds...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blogs []Blog
	for rows.Next() {
		var c Blog
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			continue
		}
		blogs = append(blogs, c)
	}
	return blogs, nil
}

// Lấy danh sách blog (với phân trang)
func GetBlogs(c *gin.Context, limit, offset int) ([]models.Blog, error) {
	db, err := utils.GetDBFromContext(c)
	domainApi := c.Request.Host // Hoặc lấy từ biến môi trường
	tenantId, _ := c.Get("tenantId")
	query := `SELECT b.id,  JSON_UNQUOTE(JSON_EXTRACT(title, '$.en')) as title, slug,
	  JSON_UNQUOTE(JSON_EXTRACT(excerpt, '$.en')) as excerpt
	 ,  JSON_UNQUOTE(JSON_EXTRACT(content, '$.en')) as content , user_id, status, approved, published_at, likes, dislikes, b.created_at , m.file_name, 
			m.id as model_id
              FROM blogs as b
			    LEFT JOIN media m ON m.id = (
            SELECT MAX(id) 
            FROM media 
            WHERE model_id = b.id 
            AND model_type = 'App\\Models\\Tenant\\Blog' 
            AND collection_name = 'main_images'
        )
			  WHERE b.deleted_at IS NULL ORDER BY b.created_at DESC LIMIT ? OFFSET ?`

	rows, err := db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blogs []models.Blog
	for rows.Next() {
		var b models.Blog
		// Scan cẩn thận các trường timestamp (cho phép null)
		err := rows.Scan(&b.ID, &b.Title, &b.Slug, &b.Excerpt, &b.Content, &b.UserID, &b.Status, &b.Approved, &b.PublishedAt, &b.Likes, &b.Dislikes, &b.CreatedAt, &b.FileName, // Cột 13
			&b.ModelID)
		if err != nil {
			return nil, err
		}

		// --- LOGIC XỬ LÝ ẢNH NHƯ BẠN YÊU CẦU ---
		if b.ModelID.Valid && b.FileName.Valid && b.FileName.String != "" {
			// Format theo cấu trúc thư mục của Laravel Tenancy
			b.Image = fmt.Sprintf("http://%s/storage/tenancy/%s/app/public/%v/%v",
				domainApi,
				tenantId,
				b.ModelID.Int64,
				b.FileName.String,
			)
		} else {
			// Ảnh mặc định nếu không có media
			b.Image = "https://tutaoweb.com/images/clothe.png"
		}

		b.Link = fmt.Sprintf("http://%s/blog/%v", domainApi, b.Slug)

		blogs = append(blogs, b)

	}
	return blogs, nil
}

// Tạo blog mới
func CreateBlog(c *gin.Context, b *models.Blog) (int64, error) {
	db, err := utils.GetDBFromContext(c)

	query := `INSERT INTO blogs (title, slug, excerpt, content, user_id, status, approved, published_at, created_at, updated_at) 
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`
	args := []interface{}{b.Title, b.Slug, b.Excerpt, b.Content, b.UserID, b.Status, b.Approved, b.PublishedAt}

	utils.LogSQL(query, args...)

	res, err := db.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	blogID, err := res.LastInsertId()
	if err != nil {
		// Xử lý lỗi nếu không lấy được ID
		return 0, err
	}
	_, err = HandleFileUpload(c, UploadParam{
		ModelType:      "App\\Models\\Tenant\\Blog",
		ModelID:        blogID,
		CollectionName: "main_images",
		FieldName:      "image",
	})

	if err != nil {
		// Log lỗi nhưng có thể vẫn trả về success cho product nếu ảnh không bắt buộc
	}

	return blogID, err
}

// Cập nhật nội dung blog
func UpdateBlog(c *gin.Context, id string, b *models.Blog) error {
	db, err := utils.GetDBFromContext(c)

	query := `UPDATE blogs SET title = ?, slug = ?, excerpt = ?, content = ?, status = ?, approved = ?, updated_at = NOW() 
              WHERE id = ? AND deleted_at IS NULL`

	_, err = db.Exec(query, b.Title, b.Slug, b.Excerpt, b.Content, b.Status, b.Approved, id)
	return err
}

// Soft Delete (Đồng bộ logic Laravel)
func DeleteBlog(c *gin.Context, id string) error {
	db, err := utils.GetDBFromContext(c)

	query := `UPDATE blogs SET deleted_at = NOW() WHERE id = ?`
	_, err = db.Exec(query, id)
	return err
}

func GetBlogDetailById(c *gin.Context, identifier string) (*models.Blog, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	b := &models.Blog{}
	var publishedAt sql.NullTime

	// Sử dụng COALESCE để làm sạch dữ liệu ngay từ SQL
	query := `
        SELECT 
            id, 
			JSON_UNQUOTE(JSON_EXTRACT(title, '$.en')) , 
            COALESCE(slug, ''), 
			JSON_UNQUOTE(JSON_EXTRACT(title, '$.en')) , 
			JSON_UNQUOTE(JSON_EXTRACT(content, '$.en')) , 
            user_id, 
            status, 
            approved, 
            published_at, 
            likes, 
            dislikes, 
            created_at, 
            updated_at
        FROM blogs
        WHERE (id = ? OR slug = ?) AND deleted_at IS NULL
        LIMIT 1
    `

	err = db.QueryRow(query, identifier, identifier).Scan(
		&b.ID,
		&b.Title,
		&b.Slug,
		&b.Excerpt,
		&b.Content,
		&b.UserID,
		&b.Status,
		&b.Approved,
		&publishedAt,
		&b.Likes,
		&b.Dislikes,
		&b.CreatedAt,
		&b.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("bài viết không tồn tại")
		}
		return nil, err
	}

	// Convert sql.NullTime sang con trỏ *time.Time cho JSON đẹp
	if publishedAt.Valid {
		b.PublishedAt = &publishedAt.Time
	}

	return b, nil
}
