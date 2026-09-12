package services

import (
	"context"
	"fmt"
	"go-saas/utils" // Import package chứa helper
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	// Import package chứa helper
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// UploadParam chứa các thông tin biến thiên giữa các module
type UploadParam struct {
	ModelType      string // ví dụ: "App\\Models\\Product"
	ModelID        int64
	CollectionName string // ví dụ: "images" hoặc "avatars"
	FieldName      string // key trong form-data, ví dụ: "image"
}

/*
	func HandleFileUpload(c *gin.Context, p UploadParam) error {
		db, err := utils.GetDBFromContext(c)
		if err != nil {
			return err
		}

		// 1. Lấy TenantID và Path từ Env
		tenantID, exists := c.Get("tenantId")
		if !exists {
			return fmt.Errorf("tenant context missing")
		}

		basePath := strings.TrimSpace(os.Getenv("STORE_PATH"))
		if basePath == "" {
			return fmt.Errorf("STORE_PATH is not set in .env")
		}

		// 2. Nhận file từ Request
		file, err := c.FormFile(p.FieldName)
		if err != nil {
			return err
		}

		// 3. INSERT vào bảng media trước để lấy ID tự tăng (media_id)
		query := `INSERT INTO media (
	            model_type, model_id, collection_name, name, file_name,
	            mime_type, disk, size, manipulations, custom_properties,
	            generated_conversions, responsive_images, created_at, updated_at
	          )
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`

		emptyJSON := "{}"
		result, err := db.Exec(query,
			p.ModelType,
			p.ModelID,
			p.CollectionName,
			file.Filename,
			file.Filename,
			file.Header.Get("Content-Type"),
			"public",
			file.Size,
			emptyJSON,
			emptyJSON,
			emptyJSON,
			emptyJSON,
		)
		if err != nil {
			return fmt.Errorf("db insert error: %v", err)
		}

		// Lấy ID vừa mới tạo trong bảng media
		mediaID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get last insert id: %v", err)
		}

		tID := fmt.Sprintf("%v", tenantID)
		mID := fmt.Sprintf("%d", mediaID)

		// 3. Nối từng thành phần (Quan trọng: Không dùng "/" thủ công)
		// Đường dẫn mong muốn: basePath \ hoangk66 \ app \ public \ 11
		uploadDir := filepath.Join(basePath, tID, "app", "public", mID)

		// 4. Log để kiểm tra chính xác trước khi Mkdir
		utils.LogSQL("Final Path to Create: " + uploadDir)

		// 5. Thực hiện tạo folder và kiểm tra lỗi trả về
		err = os.MkdirAll(uploadDir, 0755)
		if err != nil {
			utils.LogSQL("MKDIR_ERROR: " + err.Error())
			return fmt.Errorf("không thể tạo thư mục: %v", err)
		}

		if err != nil {
			utils.LogSQL("MKDIR_ERROR: " + err.Error())
			return fmt.Errorf("không thể tạo thư mục: %v", err)
		}

		filePath := filepath.Join(uploadDir, file.Filename)
		if err := c.SaveUploadedFile(file, filePath); err != nil {
			return fmt.Errorf("failed to save file: %v", err)
		}

		return nil
	}
*/
func HandleFileUpload(c *gin.Context, p UploadParam) ([]string, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}
	utils.LogSQL(p.CollectionName)

	tenantID, exists := c.Get("tenantId")
	if !exists {
		return nil, fmt.Errorf("tenant context missing")
	}
	tIDStr := fmt.Sprintf("%v", tenantID)

	// Lấy mảng file từ Form
	var files []*multipart.FileHeader
	form, err := c.MultipartForm()
	if err != nil {
		file, err := c.FormFile(p.FieldName)
		if err == nil {
			files = append(files, file)
		}
	} else {
		files = form.File[p.FieldName]
	}

	if len(files) == 0 {
		return []string{}, nil // Không có file thì trả về mảng rỗng
	}

	return processAndSaveFiles(db, tIDStr, p, files, c)
}

func processAndSaveFiles(db *sqlx.DB, tID string, p UploadParam, files []*multipart.FileHeader, c *gin.Context) ([]string, error) {
	emptyJSON := "{}"
	var uploadedURLs []string

	// Lấy cấu hình Disk từ DB Tenant (default hoặc r2)
	storeDisk := utils.GetSetting(c, "store_disk", "default")
	//utils.LogToFile("processAndSaveFiles")
	//utils.LogToFile(storeDisk)
	for _, file := range files {
		diskName := "public"
		if storeDisk == "r2" {
			diskName = "r2"
		}

		// 1. Lưu DB để cấp ID cho Media
		query := `INSERT INTO media (
				model_type, model_id, collection_name, name, file_name, 
				mime_type, disk, size, manipulations, custom_properties, 
				generated_conversions, responsive_images, created_at, updated_at
			  ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`

		result, err := db.Exec(query, p.ModelType, p.ModelID, p.CollectionName, file.Filename, file.Filename,
			file.Header.Get("Content-Type"), diskName, file.Size, emptyJSON, emptyJSON, emptyJSON, emptyJSON)

		if err != nil {
			return nil, fmt.Errorf("db error: %v", err)
		}

		mediaID, _ := result.LastInsertId()
		var fileURL string

		// 2. Xử lý Upload theo từng loại Disk
		if storeDisk == "r2" {
			// ==========================================
			// BƯỚC 2A: UPLOAD LÊN CLOUDFLARE R2
			// ==========================================
			url, err := uploadToR2(c, tID, mediaID, file)
			if err != nil {
				return nil, fmt.Errorf("r2 upload error: %v", err)
			}
			fileURL = url

		} else {
			// ==========================================
			// BƯỚC 2B: LƯU VÀO LOCAL STORAGE (DEFAULT)
			// ==========================================
			basePath := strings.TrimSpace(os.Getenv("STORE_PATH"))
			if basePath == "" {
				return nil, fmt.Errorf("STORE_PATH is not set in .env")
			}

			uploadDir := filepath.Join(basePath, tID, "app", "public", fmt.Sprintf("%d", mediaID))
			if err := os.MkdirAll(uploadDir, 0755); err != nil {
				return nil, fmt.Errorf("mkdir error: %v", err)
			}

			filePath := filepath.Join(uploadDir, file.Filename)
			if err := c.SaveUploadedFile(file, filePath); err != nil {
				return nil, fmt.Errorf("save file error: %v", err)
			}

			// Chmod quyền hệ thống
			_ = exec.Command("chmod", "-R", "755", uploadDir).Run()
			_ = exec.Command("chmod", "644", filePath).Run()

			// Build URL Public Local
			fileURL = fmt.Sprintf("https://popshop.tutaoweb.com/storage/tenancy/%s/app/public/%d/%s", tID, mediaID, file.Filename)
		}
		updateQuery := `UPDATE media SET url = ? WHERE id = ?`
		if _, err := db.Exec(updateQuery, fileURL, mediaID); err != nil {
			return nil, fmt.Errorf("failed to update media url: %v", err)
		}
		utils.LogToFile("[UPLOAD SUCCESS] File %s saved to disk [%s] with URL: %s", file.Filename, storeDisk, fileURL)

		uploadedURLs = append(uploadedURLs, fileURL)
	}

	return uploadedURLs, nil
}

// Helper: Upload trực tiếp lên Cloudflare R2
func uploadToR2(c *gin.Context, tID string, mediaID int64, file *multipart.FileHeader) (string, error) {
	// Lấy cấu hình R2 từ `.env` hoặc DB settings
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKey := os.Getenv("R2_ACCESS_KEY_ID")
	secretKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucketName := os.Getenv("R2_BUCKET_NAME")
	publicDomain := strings.TrimSuffix(os.Getenv("R2_PUBLIC_DOMAIN"), "/") // Ví dụ: https://pub-xxx.r2.dev hoặc custom domain

	if accountID == "" || accessKey == "" || secretKey == "" || bucketName == "" {
		return "", fmt.Errorf("chưa cấu hình đầy đủ thông số R2 trong .env")
	}

	// Tạo S3 Client dành riêng cho R2
	r2Resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID),
		}, nil
	})

	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithEndpointResolverWithOptions(r2Resolver),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		awsconfig.WithRegion("auto"),
	)
	if err != nil {
		return "", err
	}

	client := s3.NewFromConfig(cfg)

	// Mở file từ memory/tmp
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	// Đặt S3 Key tương thích theo cấu trúc folder tenancy
	objectKey := fmt.Sprintf("tenancy/%s/app/public/%d/%s", tID, mediaID, file.Filename)

	// Push file lên R2 Bucket
	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(objectKey),
		Body:        src,
		ContentType: aws.String(file.Header.Get("Content-Type")),
	})
	if err != nil {
		return "", err
	}

	// Trả về Public URL ảnh từ R2 Custom Domain / Public R2 URL
	if publicDomain != "" {
		return fmt.Sprintf("%s/%s", publicDomain, objectKey), nil
	}

	return fmt.Sprintf("https://%s.r2.cloudflarestorage.com/%s", accountID, objectKey), nil
}

func setUmask(mask int) int {
	return platformSetUmask(mask)
}

func importUmask() {}
