package services

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

type CarrierService struct{}

func NewCarrierService() *CarrierService {
	return &CarrierService{}
}

// 1. Get All Carriers (Phân trang + Lọc theo active/search)
func GetCarriers(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	var totalCount int
	countQuery := `SELECT COUNT(*) FROM carriers WHERE deleted_at IS NULL`
	err = tenantDB.QueryRowContext(c.Request.Context(), countQuery).Scan(&totalCount)
	if err != nil {
		return gin.H{"error": "Lỗi đếm số lượng carriers: " + err.Error()}, http.StatusInternalServerError, err
	}

	carriers := make([]map[string]interface{}, 0)
	if totalCount == 0 {
		return utils.BuildLaravelPagination(c, carriers, totalCount, page, limit), http.StatusOK, nil
	}

	query := `
		SELECT 
			id, tax_id, name, code, email, phone, 
			tracking_url, active, setting, description, 
			created_at, updated_at
		FROM carriers
		WHERE deleted_at IS NULL
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`

	rows, err := tenantDB.QueryContext(c.Request.Context(), query, limit, offset)
	if err != nil {
		return gin.H{"error": "Lỗi truy vấn carriers: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	for rows.Next() {
		item, err := utils.ScanRowToMap(rows)
		if err == nil {
			carriers = append(carriers, item)
		}
	}

	return utils.BuildLaravelPagination(c, carriers, totalCount, page, limit), http.StatusOK, nil
}

// 2. Get By ID
func (s *CarrierService) GetCarierByID(c *gin.Context, id uint32) (*models.Carrier, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("lỗi kết nối DB: %w", err)
	}

	var carrier models.Carrier
	query := `
		SELECT id, tax_id, name, code, email, phone, tracking_url, active, setting, description, created_at, updated_at 
		FROM carriers 
		WHERE id = ? AND deleted_at IS NULL 
		LIMIT 1
	`
	if err := tenantDB.Get(&carrier, query, id); err != nil {
		return nil, fmt.Errorf("không tìm thấy đơn vị vận chuyển ID %d: %w", id, err)
	}

	return &carrier, nil
}

// 3. Create Carrier
func (s *CarrierService) CreateCarier(c *gin.Context, req models.CreateCarrierReq) (int64, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return 0, fmt.Errorf("lỗi kết nối DB: %w", err)
	}

	activeVal := true
	if req.Active != nil {
		activeVal = *req.Active
	}

	now := time.Now()
	query := `
		INSERT INTO carriers (tax_id, name, code, email, phone, tracking_url, active, setting, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	res, err := tenantDB.Exec(query,
		req.TaxID, req.Name, req.Code, req.Email, req.Phone,
		req.TrackingURL, activeVal, req.Setting, req.Description,
		now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("lỗi thêm đơn vị vận chuyển: %w", err)
	}

	return res.LastInsertId()
}

// 4. Update Carrier
func (s *CarrierService) UpdateCarier(c *gin.Context, id uint32, req models.UpdateCarrierReq) error {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return fmt.Errorf("lỗi kết nối DB: %w", err)
	}

	now := time.Now()
	query := `
		UPDATE carriers 
		SET tax_id = ?, name = ?, code = ?, email = ?, phone = ?, tracking_url = ?, active = ?, setting = ?, description = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	res, err := tenantDB.Exec(query,
		req.TaxID, req.Name, req.Code, req.Email, req.Phone,
		req.TrackingURL, req.Active, req.Setting, req.Description, now,
		id,
	)
	if err != nil {
		return fmt.Errorf("lỗi cập nhật đơn vị vận chuyển: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("không tìm thấy bản ghi để cập nhật")
	}

	return nil
}

// 5. Delete Carrier (Soft Delete)
func (s *CarrierService) DeleteCarier(c *gin.Context, id uint32) error {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return fmt.Errorf("lỗi kết nối DB: %w", err)
	}

	now := time.Now()
	query := `UPDATE carriers SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`

	res, err := tenantDB.Exec(query, now, id)
	if err != nil {
		return fmt.Errorf("lỗi xóa đơn vị vận chuyển: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("bản ghi không tồn tại hoặc đã bị xóa")
	}

	return nil
}

func GetCarrierRateTableService(c *gin.Context, carrierID int) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	// 1. Lấy danh sách conditions theo carrier_id (sắp xếp theo sort_order)
	conditions := make([]map[string]interface{}, 0)
	queryConditions := `
		SELECT id, min, max, sort_order, carrier_id 
		FROM shipping_rate_conditions 
		WHERE carrier_id = ? 
		ORDER BY sort_order ASC, id ASC
	`
	rowsCond, err := tenantDB.QueryContext(c.Request.Context(), queryConditions, carrierID)
	if err != nil {
		return gin.H{"error": "Lỗi truy vấn shipping_rate_conditions: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer rowsCond.Close()

	for rowsCond.Next() {
		item, err := utils.ScanRowToMap(rowsCond)
		if err == nil {
			conditions = append(conditions, item)
		}
	}

	// 2. Lấy tất cả zones (chưa bị xóa)
	zones := make([]map[string]interface{}, 0)
	queryZones := `
		SELECT id, name, description, created_at, updated_at 
		FROM zones 
		WHERE deleted_at IS NULL 
		ORDER BY id ASC
	`
	rowsZone, err := tenantDB.QueryContext(c.Request.Context(), queryZones)
	if err != nil {
		return gin.H{"error": "Lỗi truy vấn zones: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer rowsZone.Close()

	for rowsZone.Next() {
		item, err := utils.ScanRowToMap(rowsZone)
		if err == nil {
			zones = append(zones, item)
		}
	}

	// 3. Lấy shipping_rate_tables theo carrier_id và build Ma trận Data [zone_id][condition_id] = rate
	rateMatrix := make(map[string]map[string]interface{})

	queryRates := `
		SELECT id, zone_id, rate, shipping_rate_condition_id, carrier_id 
		FROM shipping_rate_tables 
		WHERE carrier_id = ?
	`
	rowsRate, err := tenantDB.QueryContext(c.Request.Context(), queryRates, carrierID)
	if err != nil {
		return gin.H{"error": "Lỗi truy vấn shipping_rate_tables: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer rowsRate.Close()

	for rowsRate.Next() {
		item, err := utils.ScanRowToMap(rowsRate)
		if err == nil {
			zoneID := strconv.Itoa(utils.InterfaceToInt(item["zone_id"]))
			conditionID := strconv.Itoa(utils.InterfaceToInt(item["shipping_rate_condition_id"]))
			rate := item["rate"]

			if rateMatrix[zoneID] == nil {
				rateMatrix[zoneID] = make(map[string]interface{})
			}
			rateMatrix[zoneID][conditionID] = rate
		}
	}

	// 4. Trả về Response
	return gin.H{
		"status": "success",
		"data": gin.H{
			"rates":      rateMatrix, // Ma trận $data[zone_id][condition_id] = rate
			"conditions": conditions,
			"zones":      zones,
		},
	}, http.StatusOK, nil
}

func SaveCarrierRateTableService(c *gin.Context, carrierID int, req models.SaveCarrierRateTableReq) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	tx, err := tenantDB.BeginTxx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Lỗi khởi tạo transaction: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	// Map lưu vết: [ID gửi lên từ Payload (có thể là ID ảo)] -> ID thực tế trong CSDL
	conditionIDMap := make(map[int]int)
	activeConditionIDs := make([]int, 0)

	// 1. Xử lý UPSERT danh sách conditions
	for _, cond := range req.Conditions {
		minVal := utils.InterfaceToFloat(cond.Min)
		maxVal := utils.InterfaceToFloat(cond.Max)

		isExist := false
		if cond.ID > 0 {
			// Kiểm tra ID gửi lên có thực sự tồn tại trong CSDL hay không
			var existingID int
			checkQuery := `SELECT id FROM shipping_rate_conditions WHERE id = ? AND carrier_id = ? LIMIT 1`
			errScan := tx.QueryRowContext(c.Request.Context(), checkQuery, cond.ID, carrierID).Scan(&existingID)
			if errScan == nil {
				isExist = true
			} else if errScan != sql.ErrNoRows {
				return gin.H{"error": fmt.Sprintf("Lỗi kiểm tra condition ID %d: %v", cond.ID, errScan)}, http.StatusInternalServerError, errScan
			}
		}

		if isExist {
			// 1.1 Tồn tại -> UPDATE
			updateQuery := `
				UPDATE shipping_rate_conditions 
				SET min = ?, max = ?, sort_order = ?, carrier_id = ? 
				WHERE id = ? AND carrier_id = ?
			`
			_, err := tx.ExecContext(c.Request.Context(), updateQuery, minVal, maxVal, cond.SortOrder, carrierID, cond.ID, carrierID)
			if err != nil {
				return gin.H{"error": fmt.Sprintf("Lỗi cập nhật condition ID %d: %v", cond.ID, err)}, http.StatusInternalServerError, err
			}

			conditionIDMap[cond.ID] = cond.ID
			activeConditionIDs = append(activeConditionIDs, cond.ID)
		} else {
			// 1.2 Không tồn tại (ID ảo hoặc ID <= 0) -> INSERT tạo mới
			insertQuery := `
				INSERT INTO shipping_rate_conditions (min, max, sort_order, carrier_id) 
				VALUES (?, ?, ?, ?)
			`
			res, err := tx.ExecContext(c.Request.Context(), insertQuery, minVal, maxVal, cond.SortOrder, carrierID)
			if err != nil {
				return gin.H{"error": "Lỗi thêm condition mới: " + err.Error()}, http.StatusInternalServerError, err
			}

			newID, _ := res.LastInsertId()
			realNewID := int(newID)

			// Ánh ánh ID ảo (cond.ID) sang ID thật vừa tạo trong DB
			conditionIDMap[cond.ID] = realNewID
			activeConditionIDs = append(activeConditionIDs, realNewID)
		}
	}

	// 2. Xóa các conditions cũ không còn nằm trong danh sách activeConditionIDs
	if len(activeConditionIDs) > 0 {
		queryDeleteCond, args, err := utils.InQuery("DELETE FROM shipping_rate_conditions WHERE carrier_id = ? AND id NOT IN (?)", carrierID, activeConditionIDs)
		if err == nil {
			queryDeleteCond = tenantDB.Rebind(queryDeleteCond)
			_, _ = tx.ExecContext(c.Request.Context(), queryDeleteCond, args...)
		}
	} else {
		_, _ = tx.ExecContext(c.Request.Context(), "DELETE FROM shipping_rate_conditions WHERE carrier_id = ?", carrierID)
	}

	// 3. Xóa toàn bộ bảng giá cũ của carrier_id này
	_, err = tx.ExecContext(c.Request.Context(), "DELETE FROM shipping_rate_tables WHERE carrier_id = ?", carrierID)
	if err != nil {
		return gin.H{"error": "Lỗi làm sạch bảng giá cũ: " + err.Error()}, http.StatusInternalServerError, err
	}

	// 4. Insert Ma trận Rates mới (zone_id x condition_id)
	insertRateQuery := `
		INSERT INTO shipping_rate_tables (zone_id, rate, shipping_rate_condition_id, carrier_id) 
		VALUES (?, ?, ?, ?)
	`
	for zoneIDStr, condMap := range req.Rates {
		zoneID, _ := strconv.Atoi(zoneIDStr)
		if zoneID <= 0 {
			continue
		}

		for condIDStr, rateVal := range condMap {
			rawCondID, _ := strconv.Atoi(condIDStr)

			// Lấy ID thực tế từ map (nếu rawCondID là ID ảo thì sẽ lấy được ID mới tạo ở Bước 1)
			realCondID, exists := conditionIDMap[rawCondID]
			if !exists {
				realCondID = rawCondID
			}

			rate := utils.InterfaceToFloat(rateVal)
			_, err := tx.ExecContext(c.Request.Context(), insertRateQuery, zoneID, rate, realCondID, carrierID)
			if err != nil {
				return gin.H{"error": fmt.Sprintf("Lỗi lưu giá cho zone %d, condition %d: %v", zoneID, realCondID, err)}, http.StatusInternalServerError, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return gin.H{"error": "Lỗi commit transaction: " + err.Error()}, http.StatusInternalServerError, err
	}

	return gin.H{
		"status":  "success",
		"message": "Lưu bảng giá vận chuyển thành công",
	}, http.StatusOK, nil
}
