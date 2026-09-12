package utils

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
)

// GetPluginSetting Lấy giá trị của một key cụ thể trong cột settings (JSON) của bảng plugins
func GetPluginSetting(c *gin.Context, pluginCode string, key string) (string, error) {
	tenantDB, err := GetDBFromContext(c)
	if err != nil {
		return "", fmt.Errorf("lỗi lấy DB context: %w", err)
	}

	if tenantDB == nil {
		return "", fmt.Errorf("database connection is nil")
	}

	// 1. Query cột settings (JSON string) theo code hoặc name plugin
	var settingsJSON string
	query := `
		SELECT settings 
		FROM plugins 
		WHERE (code = ? OR name = ?) 
		LIMIT 1
	`

	err = tenantDB.Get(&settingsJSON, query, pluginCode, pluginCode)
	if err != nil {
		return "", fmt.Errorf("không tìm thấy plugin '%s' hoặc plugin chưa được kích hoạt: %w", pluginCode, err)
	}

	if settingsJSON == "" {
		return "", nil
	}

	// 2. Unmarshal JSON string thành map[string]string
	var settingsMap map[string]string
	if err := json.Unmarshal([]byte(settingsJSON), &settingsMap); err != nil {
		return "", fmt.Errorf("lỗi parse JSON settings cho plugin '%s': %w", pluginCode, err)
	}
	//LogToFile("settingsMap")

	//LogToFile(settingsMap)
	// 3. Trả về giá trị theo key
	return settingsMap[key], nil
}

func SetPluginSetting(c *gin.Context, pluginCode string, key string, value string) error {
	tenantDB, err := GetDBFromContext(c)
	if err != nil {
		return fmt.Errorf("lỗi lấy DB context: %w", err)
	}

	if tenantDB == nil {
		return fmt.Errorf("database connection is nil")
	}

	// 1. Query cột settings (JSON string) hiện tại của plugin
	var settingsJSON string
	querySelect := `
		SELECT settings 
		FROM plugins 
		WHERE (code = ? OR name = ?) 
		LIMIT 1
	`

	err = tenantDB.Get(&settingsJSON, querySelect, pluginCode, pluginCode)
	if err != nil {
		return fmt.Errorf("không tìm thấy plugin '%s' để cập nhật setting: %w", pluginCode, err)
	}

	// 2. Parse JSON thành map, nếu rỗng thì khởi tạo map mới
	settingsMap := make(map[string]string)
	if settingsJSON != "" {
		if err := json.Unmarshal([]byte(settingsJSON), &settingsMap); err != nil {
			return fmt.Errorf("lỗi parse JSON settings hiện tại cho plugin '%s': %w", pluginCode, err)
		}
	}

	// 3. Cập nhật giá trị theo key
	settingsMap[key] = value

	// 4. Marshal map ngược lại thành JSON string
	updatedJSONBytes, err := json.Marshal(settingsMap)
	if err != nil {
		return fmt.Errorf("lỗi marshal JSON settings cho plugin '%s': %w", pluginCode, err)
	}

	// 5. UPDATE chuỗi JSON mới vào CSDL
	queryUpdate := `
		UPDATE plugins 
		SET settings = ?, updated_at = NOW() 
		WHERE (code = ? OR name = ?)
	`

	res, err := tenantDB.Exec(queryUpdate, string(updatedJSONBytes), pluginCode, pluginCode)
	if err != nil {
		return fmt.Errorf("lỗi cập nhật settings cho plugin '%s': %w", pluginCode, err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("không có dòng nào được cập nhật cho plugin '%s'", pluginCode)
	}

	return nil
}
