package services

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"go-saas/utils"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetPlugins(c *gin.Context, page, perPage int, lang, search, categoryID string) ([]map[string]interface{}, int, error) {
	db, err := utils.GetCentralDB()

	offset := (page - 1) * perPage

	// 1. Xây dựng điều kiện WHERE động
	whereClause := " WHERE 1=1 "
	var searchArgs []interface{}

	if search != "" {
		likePattern := "%" + search + "%"
		// Giả định name lưu dạng JSON đa ngôn ngữ giống mẫu cũ của bạn
		whereClause += " AND JSON_EXTRACT(modules.name, ?) LIKE ? "
		searchArgs = append(searchArgs, "$."+lang, likePattern)
	}

	if categoryID != "" {
		whereClause += " AND modules.category_id = ? "
		searchArgs = append(searchArgs, categoryID)
	}

	// 2. Đếm tổng số bản ghi phù hợp điều kiện lọc
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM modules %s", whereClause)
	utils.LogSQL(countQuery, whereClause)

	_ = db.QueryRow(countQuery, searchArgs...).Scan(&total)

	if err != nil {
		log.Printf("[Service Error] Count query failed: %v", err)
		return nil, 0, err
	}

	// 3. Query lấy danh sách data kèm trạng thái check từ bảng client_orders (type = 2)
	query := fmt.Sprintf(`
		SELECT 
			modules.id,
			modules.category_id,
			JSON_UNQUOTE(JSON_EXTRACT(modules.name, ?)) as name,
			JSON_UNQUOTE(JSON_EXTRACT(modules.description, ?)) as description,
			main_image,modules.type,modules.code,rate
		FROM modules
		
		%s
		LIMIT ? OFFSET ?
	`, whereClause)

	// Chuẩn bị tham số gán vào query chính
	var queryArgs []interface{}
	queryArgs = append(queryArgs, "$."+lang, "$."+lang)
	queryArgs = append(queryArgs, searchArgs...)
	queryArgs = append(queryArgs, perPage, offset)

	rows, err := db.Query(query, queryArgs...)

	utils.LogSQL(query, queryArgs...)

	if err != nil {
		log.Printf("[Service Error] Select query failed: %v", err)
		return nil, 0, err
	}
	defer rows.Close()

	// 4. Đọc dữ liệu động (Map Columns - Đồng bộ với hàm mẫu của bạn)
	results := make([]map[string]interface{}, 0)
	cols, _ := rows.Columns()
	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, 0, err
		}

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

	return results, total, nil
}

func GetInstalledPlugins(c *gin.Context, page, perPage int, lang, search, categoryID string) ([]map[string]interface{}, int, error) {
	db, err := utils.GetCentralDB()
	if err != nil {
		log.Printf("[Service Error] Central DB connection failed: %v", err)
		return nil, 0, err
	}

	rawDomain := c.Request.Host
	cleanedDomain := utils.CleanDomain(rawDomain)
	offset := (page - 1) * perPage

	// LƯU Ý TỐI ƯU: Lấy tenant_id từ domain ra trước để dùng làm tham số độc lập,
	// tránh việc gọi subquery lặp đi lặp lại trong JOIN gây lỗi Collation/Binary và giảm hiệu năng.
	var tenantID string
	tenantQuery := "SELECT tenant_id FROM domains WHERE domain = CONVERT(? USING utf8mb4) COLLATE utf8mb4_unicode_ci LIMIT 1"
	err = db.QueryRow(tenantQuery, cleanedDomain).Scan(&tenantID)
	if err != nil {
		log.Printf("[Service Error] Get tenant_id failed for domain %s: %v", cleanedDomain, err)
		return nil, 0, err
	}

	// 1. Xây dựng điều kiện WHERE động (Lúc này lọc trên bảng modules)
	whereClause := " WHERE tp.tenant_id = ? AND tp.status != 0 "
	var searchArgs []interface{}
	searchArgs = append(searchArgs, tenantID) // Tham số cho tp.tenant_id

	if search != "" {
		likePattern := "%" + search + "%"
		whereClause += " AND JSON_EXTRACT(m.name, ?) LIKE ? "
		searchArgs = append(searchArgs, "$."+lang, likePattern)
	}

	if categoryID != "" {
		whereClause += " AND m.category_id = ? "
		searchArgs = append(searchArgs, categoryID)
	}

	// 2. Đếm tổng số plugin thực tế ĐÃ CÀI ĐẶT của tenant này
	var total int
	countQuery := fmt.Sprintf(`
        SELECT COUNT(*) 
        FROM tenant_plugins tp
        INNER JOIN modules m ON tp.plugin_id = m.id 
        %s`, whereClause)

	utils.LogSQL(countQuery, searchArgs...)
	err = db.QueryRow(countQuery, searchArgs...).Scan(&total)
	if err != nil {
		log.Printf("[Service Error] Count query failed: %v", err)
		return nil, 0, err
	}

	// 3. Query chi tiết danh sách plugin đã cài đặt
	query := fmt.Sprintf(`
        SELECT 
            m.id,
            m.category_id,
            JSON_UNQUOTE(JSON_EXTRACT(m.name, ?)) as name,
            JSON_UNQUOTE(JSON_EXTRACT(m.description, ?)) as description,
            m.main_image, 
            m.type, 
            m.code, 
            m.rate,
            1 as is_installed, -- Chắc chắn là 1 vì lấy từ danh sách đã cài
            tp.status as tenant_plugin_status
        FROM tenant_plugins tp
        INNER JOIN modules m ON tp.plugin_id = m.id
        %s
        LIMIT ? OFFSET ?
    `, whereClause)

	// Chuẩn bị mảng tham số theo đúng thứ tự
	var queryArgs []interface{}
	queryArgs = append(queryArgs, "$."+lang, "$."+lang) // Cho 2 hàm JSON ở SELECT
	queryArgs = append(queryArgs, searchArgs...)        // Cho phần WHERE (đã bao gồm cả tenantID)
	queryArgs = append(queryArgs, perPage, offset)      // Cho LIMIT, OFFSET

	utils.LogSQL(query, queryArgs...)

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		log.Printf("[Service Error] Select query failed: %v", err)
		return nil, 0, err
	}
	defer rows.Close()

	// 4. Đọc dữ liệu động
	results := make([]map[string]interface{}, 0)
	cols, _ := rows.Columns()
	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, 0, err
		}

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

	return results, total, nil
}

func GetModuleFormSchema(c *gin.Context, moduleID int) (string, string, []byte, error) {
	db, err := utils.GetCentralDB()
	if err != nil {
		log.Printf("[Service Error] Central DB connection failed: %v", err)
		return "", "", nil, err
	}

	query := `
		SELECT id, code, config_schema 
		FROM modules 
		WHERE id = ? 
		LIMIT 1
	`

	var id int
	var code string
	var schemaBytes []byte

	err = db.QueryRow(query, moduleID).Scan(&id, &code, &schemaBytes)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", nil, errors.New("không tìm thấy module hoặc module không hỗ trợ cấu hình")
		}
		log.Printf("[Service Error] Query module form failed: %v", err)
		return "", "", nil, err
	}

	// Xử lý nếu chuỗi JSON trống/NULL để tránh làm lỗi giao diện phía frontend
	if len(schemaBytes) == 0 || string(schemaBytes) == "null" {
		schemaBytes = []byte("[]")
	}

	return strconv.Itoa(id), code, schemaBytes, nil
}

func GetPluginSettings(c *gin.Context, centralID int) ([]byte, error) {
	db, err := utils.GetDBFromContext(c) // Hàm lấy DB của Tenant hiện tại
	if err != nil {
		log.Printf("[Service Error] Tenant DB connection failed: %v", err)
		return nil, err
	}

	query := "SELECT settings FROM plugins WHERE central_id = ? LIMIT 1"
	var settingsBytes []byte

	err = db.QueryRow(query, centralID).Scan(&settingsBytes)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("plugin chưa được cài đặt trên tenant này")
		}
		log.Printf("[Service Error] Get plugin settings failed: %v", err)
		return nil, err
	}

	// Nếu cột settings trong DB đang NULL hoặc rỗng, trả về đối tượng JSON rỗng {}
	if len(settingsBytes) == 0 || string(settingsBytes) == "null" {
		settingsBytes = []byte("{}")
	}

	return settingsBytes, nil
}

type SaveSettingsPayload struct {
	Settings json.RawMessage `json:"settings"`
}

func SavePluginSettings(c *gin.Context, centralID int, bodyBytes []byte) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		log.Printf("[Service Error] Tenant DB connection failed: %v", err)
		return err
	}

	// 1. Bóc tách lấy dữ liệu thực tế bên trong key "settings"
	var payload SaveSettingsPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return errors.New("invalid json format")
	}

	// Nếu field settings gửi lên bị rỗng hoặc không hợp lệ
	if len(payload.Settings) == 0 || string(payload.Settings) == "null" {
		return errors.New("settings field is required")
	}

	// 2. Thực hiện Upsert vào DB (Lưu chuỗi json thuần, không bị bọc key settings)
	upsertQuery := `
		INSERT INTO plugins (central_id, name, description, active, settings, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE 
			settings = VALUES(settings),
			updated_at = NOW()
	`

	defaultName := "Plugin " + strconv.Itoa(centralID)
	defaultDesc := "Module config"

	_, err = db.Exec(upsertQuery, centralID, defaultName, defaultDesc, string(payload.Settings))
	if err != nil {
		log.Printf("[Service Error] Upsert plugin settings failed: %v", err)
		return err
	}

	return nil
}

func GetTenantPluginCodes(tenantID string) ([]string, error) {
	centralDB, err := utils.GetCentralDB()

	query := `
        SELECT m.code 
        FROM tenant_plugins tp
        INNER JOIN modules m ON tp.plugin_id = m.id
        WHERE tp.tenant_id = ? 
          AND tp.status = 1 
          AND (tp.expires_at IS NULL OR tp.expires_at > NOW())
          AND m.status = 1 
          AND m.deleted_at IS NULL
    `

	// Log SQL
	utils.LogSQL(query, tenantID)

	rows, err := centralDB.Query(query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err == nil {
			codes = append(codes, code)
		}
	}

	// Đảm bảo trả về mảng rỗng [] thay vì null nếu không tìm thấy plugin
	if codes == nil {
		codes = []string{}
	}

	return codes, nil
}
