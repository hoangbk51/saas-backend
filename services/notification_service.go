package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// GetUnreadNotifications Lấy danh sách thông báo chưa đọc (read_at IS NULL)
func GetUnreadNotifications(c *gin.Context, notifiableType string, notifiableID uint64, limit int) (*models.NotificationListResponse, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("không thể kết nối CSDL: %v", err)
	}

	if limit <= 0 {
		limit = 10
	}

	response := &models.NotificationListResponse{
		Items: make([]models.Notification, 0),
	}

	// 1. Đếm tổng số thông báo chưa đọc để hiển thị Badge số đỏ trên FE
	countQuery := `
		SELECT COUNT(*) 
		FROM notifications 
		WHERE notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL`
	_ = db.QueryRowContext(c, countQuery, notifiableType, notifiableID).Scan(&response.UnreadCount)

	// 2. Lấy danh sách thông báo chưa đọc giảm dần theo created_at
	query := `
		SELECT 
			id, type, notifiable_type, notifiable_id, created_by, 
			icon, action_text, action_url, data, message, read_at, created_at
		FROM notifications
		WHERE notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL
		ORDER BY created_at DESC
		LIMIT ?`
	//utils.LogToFile("[DEBUG] Checking -> Type: '%s', ID: %d", notifiableType, notifiableID)
	rows, err := db.QueryContext(c, query, notifiableType, notifiableID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item models.Notification
		var rawData string
		var readAt *time.Time

		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.NotifiableType,
			&item.NotifiableID,
			&item.CreatedBy,
			&item.Icon,
			&item.ActionText,
			&item.ActionURL,
			&rawData,
			&item.Message,
			&readAt,
			&item.CreatedAt,
		)
		if err != nil {
			continue
		}

		item.ReadAt = readAt
		// Tái sử dụng ParseFlexibleField để parse dữ liệu cột `data` nếu lưu dạng JSON string
		item.Data = utils.ParseFlexibleField(rawData)

		response.Items = append(response.Items, item)
	}

	return response, nil
}

// MarkNotificationsAsRead Đánh dấu đã đọc đơn lẻ hoặc hàng loạt
func MarkNotificationsAsRead(c *gin.Context, notifiableType string, notifiableID uint64, ids []string) (int64, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return 0, fmt.Errorf("không thể kết nối CSDL: %v", err)
	}

	now := time.Now()

	// Trường hợp 1: Nếu không truyền `ids` hoặc `ids` rỗng -> Đánh dấu ĐÃ ĐỌC TẤT CẢ
	if len(ids) == 0 {
		query := `
			UPDATE notifications 
			SET read_at = ?, updated_at = ? 
			WHERE notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL`

		res, err := db.ExecContext(c, query, now, now, notifiableType, notifiableID)
		if err != nil {
			return 0, err
		}
		return res.RowsAffected()
	}

	// Trường hợp 2: Đánh dấu theo danh sách ID truyền lên
	placeholders := make([]string, len(ids))
	args := make([]interface{}, 0, len(ids)+4)

	args = append(args, now, now, notifiableType, notifiableID)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := fmt.Sprintf(`
		UPDATE notifications 
		SET read_at = ?, updated_at = ? 
		WHERE notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL AND id IN (%s)`,
		strings.Join(placeholders, ","),
	)

	res, err := db.ExecContext(c, query, args...)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

func CreateAndBroadcastToTargetUsers(ctx context.Context, tenantDB *sqlx.DB, tenantID string, notify models.Notification) error {
	var targetPerms []string
	if notify.TargetPermissions != nil {
		targetPerms = notify.TargetPermissions
	}

	if notify.CreatedAt.IsZero() {
		notify.CreatedAt = time.Now()
	}

	// 1. Tìm danh sách User ID thuộc Tenant thoả mãn Role/Permission
	userIDs, err := GetUserIDsByTarget(ctx, tenantDB, notify.TargetRole, targetPerms)
	if err != nil || len(userIDs) == 0 {
		// Fallback: Lấy ID của Owner cửa hàng nếu không tìm thấy ai
		_ = tenantDB.SelectContext(ctx, &userIDs, "SELECT id FROM users WHERE is_owner = 1 LIMIT 1")
	}

	// Tối ưu: Marshal Data & Permissions 1 lần duy nhất ngoài vòng lặp
	dataBytes, err := json.Marshal(notify.Data)
	if err != nil {
		return fmt.Errorf("lỗi marshal notification data: %v", err)
	}

	var targetPermsJSON *string
	if len(notify.TargetPermissions) > 0 {
		permsBytes, err := json.Marshal(notify.TargetPermissions)
		if err == nil {
			permStr := string(permsBytes)
			targetPermsJSON = &permStr
		}
	}

	insertQuery := `INSERT INTO notifications 
		(id, type, notifiable_type, notifiable_id, target_role, target_permissions, created_by, icon, action_text, action_url, data, message, read_at, created_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// 2. Loop tạo Notification riêng cho từng User (Fan-out)
	for _, userID := range userIDs {
		uID := userID
		userNotify := notify
		userNotify.ID = uuid.New().String()                     // ✅ Sinh UUID mới cho từng dòng
		userNotify.NotifiableType = "App\\Models\\Tenant\\User" // Set chuẩn Polymorphic Type
		userNotify.NotifiableID = &uID                          // ✅ Gán ID của User hiện tại
		userNotify.CreatedAt = time.Now()

		// Lấy giá trị số của NotifiableID để log đẹp (5, 9) thay vì địa chỉ con trỏ RAM (0x1df...)
		var logNotifiableID uint64 = 0
		if userNotify.NotifiableID != nil {
			logNotifiableID = *userNotify.NotifiableID
		}

		// Log SQL dùng đúng dữ liệu của userNotify
		utils.LogSQL(insertQuery, userNotify.ID, userNotify.Type, userNotify.NotifiableType, logNotifiableID)

		// Xử lý ReadAt: Nếu unread thì truyền nil (MySQL sẽ nhận NULL)
		var readAtParam interface{} = nil
		if userNotify.ReadAt != nil && !userNotify.ReadAt.IsZero() {
			readAtParam = userNotify.ReadAt
		}

		// Exec SQL: Dùng toàn bộ thông tin từ userNotify
		_, err = tenantDB.ExecContext(
			ctx,
			insertQuery,
			userNotify.ID, // ✅ userNotify.ID
			userNotify.Type,
			userNotify.NotifiableType,
			userNotify.NotifiableID, // ✅ userNotify.NotifiableID (&uID)
			userNotify.TargetRole,
			targetPermsJSON,
			userNotify.CreatedBy,
			userNotify.Icon,
			userNotify.ActionText,
			userNotify.ActionURL,
			string(dataBytes),
			userNotify.Message,
			readAtParam, // ✅ NULL khi tạo mới
			userNotify.CreatedAt,
		)
		if err != nil {
			utils.LogToFile("[Notification Error] UserID %d: %v", uID, err)
			continue // Tiếp tục gửi cho các User còn lại trong danh sách
		}

		// 3. Bắn Redis Pub/Sub theo Channel riêng của User đó
		channelName := fmt.Sprintf("tenant:%s:user:%d", tenantID, uID)
		payloadBytes, _ := json.Marshal(userNotify)
		if utils.RedisClient != nil {
			_ = utils.RedisClient.Publish(ctx, channelName, string(payloadBytes)).Err()
		}
	}

	return nil
}

func GetUserIDsByTarget(ctx context.Context, tenantDB *sqlx.DB, targetRole *string, targetPerms []string) ([]uint64, error) {
	var userIDs []uint64

	// Xử lý trường hợp targetPerms rỗng
	if len(targetPerms) == 0 {
		targetPerms = []string{""}
	}

	// Query lấy User có Role chỉ định HOẶC có Permission chỉ định HOẶC là OWNER
	query := `
		SELECT DISTINCT u.id 
		FROM users u
		LEFT JOIN model_has_roles mhr ON u.id = mhr.model_id
		LEFT JOIN roles r ON mhr.role_id = r.id
		LEFT JOIN role_has_permissions rhp ON r.id = rhp.role_id
		LEFT JOIN permissions p ON rhp.permission_id = p.id
		WHERE u.active = 1 AND p.name IN (?)
		`

	// Dùng sqlx.In để bind slice permissions
	query, args, err := sqlx.In(query, targetPerms)
	if err != nil {
		return nil, err
	}

	query = tenantDB.Rebind(query)
	utils.LogSQL(query, args...)

	err = tenantDB.SelectContext(ctx, &userIDs, query, args...)
	return userIDs, err
}
