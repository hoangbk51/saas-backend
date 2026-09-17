package controllers

import (
	"database/sql"
	"fmt"
	"go-saas/models"
	"go-saas/utils"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// Helper: helper function to update or insert stock in manage_stocks
func adjustStockHelper(db *sqlx.DB, warehouseID uint64, productID uint64, variantID *uint64, diffQuantity doubleOrFloat) error {
	var currentStock float64
	var stockID uint64

	var err error
	if variantID != nil && *variantID > 0 {
		err = db.QueryRow("SELECT id, quantity FROM manage_stocks WHERE warehouse_id = ? AND product_id = ? AND variant_id = ? LIMIT 1",
			warehouseID, productID, *variantID).Scan(&stockID, &currentStock)
	} else {
		err = db.QueryRow("SELECT id, quantity FROM manage_stocks WHERE warehouse_id = ? AND product_id = ? AND (variant_id IS NULL OR variant_id = 0) LIMIT 1",
			warehouseID, productID).Scan(&stockID, &currentStock)
	}

	if err == sql.ErrNoRows {
		// Insert new stock
		newQty := math.Max(0, float64(diffQuantity))
		now := time.Now()
		_, insErr := db.Exec("INSERT INTO manage_stocks (warehouse_id, product_id, variant_id, quantity, created_at, updated_at, alert) VALUES (?, ?, ?, ?, ?, ?, 0)",
			warehouseID, productID, variantID, newQty, now, now)
		return insErr
	} else if err != nil {
		return err
	}

	// Update existing stock
	newQty := math.Max(0, currentStock+float64(diffQuantity))
	now := time.Now()
	_, updErr := db.Exec("UPDATE manage_stocks SET quantity = ?, updated_at = ? WHERE id = ?", newQty, now, stockID)
	return updErr
}

type doubleOrFloat float64

// =========================================================================
// 1. WAREHOUSES
// =========================================================================

// GetWarehouses handles GET /api/v2/admin/warehouses and /api/v2/admin/warehouse
func GetWarehouses(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	search := strings.TrimSpace(c.Query("search"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "1000"))
	if perPage < 1 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	whereClause := "WHERE deleted_at IS NULL"
	args := []interface{}{}

	if search != "" {
		whereClause += " AND (name LIKE ? OR email LIKE ?)"
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm)
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM warehouses %s", whereClause)
	_ = db.QueryRow(countQuery, args...).Scan(&total)

	query := fmt.Sprintf(`
		SELECT id, name, email, incharge, description, active, deleted_at, created_at, updated_at, state_id 
		FROM warehouses 
		%s 
		ORDER BY id DESC 
		LIMIT ? OFFSET ?`, whereClause)
	queryArgs := append(args, perPage, offset)

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	defer rows.Close()

	warehouses := []models.Warehouse{}
	for rows.Next() {
		var w models.Warehouse
		if err := rows.Scan(&w.ID, &w.Name, &w.Email, &w.Incharge, &w.Description, &w.Active, &w.DeletedAt, &w.CreatedAt, &w.UpdatedAt, &w.StateID); err == nil {
			warehouses = append(warehouses, w)
		}
	}

	lastPage := int(math.Ceil(float64(total) / float64(perPage)))
	if lastPage < 1 {
		lastPage = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      warehouses,
		"total":     total,
		"page":      page,
		"per_page":  perPage,
		"last_page": lastPage,
	})
}

// GetWarehouse handles GET /api/v2/admin/warehouses/:id
func GetWarehouse(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	id := c.Param("id")
	var w models.Warehouse
	err = db.QueryRow(`
		SELECT id, name, email, incharge, description, active, deleted_at, created_at, updated_at, state_id 
		FROM warehouses 
		WHERE id = ? AND deleted_at IS NULL LIMIT 1`, id).
		Scan(&w.ID, &w.Name, &w.Email, &w.Incharge, &w.Description, &w.Active, &w.DeletedAt, &w.CreatedAt, &w.UpdatedAt, &w.StateID)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Warehouse not found"})
		return
	}

	// Get stock stats for warehouse
	var totalStock float64
	var totalProducts int
	_ = db.QueryRow("SELECT COALESCE(SUM(quantity), 0), COUNT(DISTINCT product_id) FROM manage_stocks WHERE warehouse_id = ?", w.ID).
		Scan(&totalStock, &totalProducts)
	w.TotalStock = &totalStock
	w.TotalProducts = &totalProducts

	c.JSON(http.StatusOK, gin.H{"success": true, "data": w})
}

// CreateWarehouse handles POST /api/v2/admin/warehouses and /api/v2/admin/warehouse
func CreateWarehouse(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	var req struct {
		Name        string  `json:"name" binding:"required"`
		Email       *string `json:"email"`
		Incharge    *uint64 `json:"incharge"`
		Description *string `json:"description"`
		Active      *bool   `json:"active"`
		StateID     *uint64 `json:"state_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	activeVal := 1 // Giá trị mặc định
	if req.Active != nil {
		if *req.Active {
			activeVal = 1
		} else {
			activeVal = 0
		}
	}

	stateVal := uint64(1)
	if req.StateID != nil {
		stateVal = *req.StateID
	}

	now := time.Now()
	res, err := db.Exec(`
		INSERT INTO warehouses (name, email, incharge, description, active, state_id, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Name, req.Email, req.Incharge, req.Description, activeVal, stateVal, now, now)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	newID, _ := res.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Warehouse created successfully",
		"data": gin.H{
			"id":          newID,
			"name":        req.Name,
			"email":       req.Email,
			"incharge":    req.Incharge,
			"description": req.Description,
			"active":      activeVal,
			"state_id":    stateVal,
			"created_at":  now,
			"updated_at":  now,
		},
	})
}

// UpdateWarehouse handles PUT/POST /api/v2/admin/warehouses/:id
func UpdateWarehouse(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	id := c.Param("id")
	var req struct {
		Name        string  `json:"name"`
		Email       *string `json:"email"`
		Incharge    *uint64 `json:"incharge"`
		Description *string `json:"description"`
		Active      *int    `json:"active"`
		StateID     *uint64 `json:"state_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	now := time.Now()
	_, err = db.Exec(`
		UPDATE warehouses 
		SET name = COALESCE(NULLIF(?, ''), name), 
		    email = ?, 
		    incharge = ?, 
		    description = ?, 
		    active = COALESCE(?, active), 
		    state_id = COALESCE(?, state_id), 
		    updated_at = ? 
		WHERE id = ? AND deleted_at IS NULL`,
		req.Name, req.Email, req.Incharge, req.Description, req.Active, req.StateID, now, id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Warehouse updated successfully",
	})
}

// DeleteWarehouse handles DELETE /api/v2/admin/warehouses/:id
func DeleteWarehouse(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	id := c.Param("id")
	now := time.Now()
	_, err = db.Exec("UPDATE warehouses SET deleted_at = ? WHERE id = ?", now, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Warehouse deleted successfully",
	})
}

// =========================================================================
// 2. SUPPLIERS
// =========================================================================

// GetSuppliers handles GET /api/v2/admin/suppliers and /api/v2/admin/supplier
func GetSuppliers(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	search := strings.TrimSpace(c.Query("search"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "1000"))
	if perPage < 1 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	whereClause := "WHERE deleted_at IS NULL"
	args := []interface{}{}

	if search != "" {
		whereClause += " AND (name LIKE ? OR email LIKE ? OR contact_person LIKE ?)"
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM suppliers %s", whereClause)
	_ = db.QueryRow(countQuery, args...).Scan(&total)

	query := fmt.Sprintf(`
		SELECT id, name, email, contact_person, url, description, active, deleted_at, created_at, updated_at 
		FROM suppliers 
		%s 
		ORDER BY id DESC 
		LIMIT ? OFFSET ?`, whereClause)
	queryArgs := append(args, perPage, offset)

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	defer rows.Close()

	suppliers := []models.Supplier{}
	for rows.Next() {
		var s models.Supplier
		if err := rows.Scan(&s.ID, &s.Name, &s.Email, &s.ContactPerson, &s.URL, &s.Description, &s.Active, &s.DeletedAt, &s.CreatedAt, &s.UpdatedAt); err == nil {
			suppliers = append(suppliers, s)
		}
	}

	lastPage := int(math.Ceil(float64(total) / float64(perPage)))
	if lastPage < 1 {
		lastPage = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      suppliers,
		"total":     total,
		"page":      page,
		"per_page":  perPage,
		"last_page": lastPage,
	})
}

// GetSupplier handles GET /api/v2/admin/suppliers/:id
func GetSupplier(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	id := c.Param("id")
	var s models.Supplier
	err = db.QueryRow(`
		SELECT id, name, email, contact_person, url, description, active, deleted_at, created_at, updated_at 
		FROM suppliers 
		WHERE id = ? AND deleted_at IS NULL LIMIT 1`, id).
		Scan(&s.ID, &s.Name, &s.Email, &s.ContactPerson, &s.URL, &s.Description, &s.Active, &s.DeletedAt, &s.CreatedAt, &s.UpdatedAt)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Supplier not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": s})
}

// CreateSupplier handles POST /api/v2/admin/suppliers
func CreateSupplier(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	var req struct {
		Name          string  `json:"name" binding:"required"`
		Email         *string `json:"email"`
		ContactPerson *string `json:"contact_person"`
		URL           *string `json:"url"`
		Description   *string `json:"description"`
		Active        *int    `json:"active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	activeVal := 1
	if req.Active != nil {
		activeVal = *req.Active
	}

	now := time.Now()
	res, err := db.Exec(`
		INSERT INTO suppliers (name, email, contact_person, url, description, active, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Name, req.Email, req.ContactPerson, req.URL, req.Description, activeVal, now, now)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	newID, _ := res.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Supplier created successfully",
		"data": gin.H{
			"id":             newID,
			"name":           req.Name,
			"email":          req.Email,
			"contact_person": req.ContactPerson,
			"url":            req.URL,
			"description":    req.Description,
			"active":         activeVal,
			"created_at":     now,
			"updated_at":     now,
		},
	})
}

// UpdateSupplier handles PUT /api/v2/admin/suppliers/:id
func UpdateSupplier(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	id := c.Param("id")
	var req struct {
		Name          string  `json:"name"`
		Email         *string `json:"email"`
		ContactPerson *string `json:"contact_person"`
		URL           *string `json:"url"`
		Description   *string `json:"description"`
		Active        *int    `json:"active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	now := time.Now()
	_, err = db.Exec(`
		UPDATE suppliers 
		SET name = COALESCE(NULLIF(?, ''), name), 
		    email = ?, 
		    contact_person = ?, 
		    url = ?, 
		    description = ?, 
		    active = COALESCE(?, active), 
		    updated_at = ? 
		WHERE id = ? AND deleted_at IS NULL`,
		req.Name, req.Email, req.ContactPerson, req.URL, req.Description, req.Active, now, id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Supplier updated successfully",
	})
}

// DeleteSupplier handles DELETE /api/v2/admin/suppliers/:id
func DeleteSupplier(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	id := c.Param("id")
	now := time.Now()
	_, err = db.Exec("UPDATE suppliers SET deleted_at = ? WHERE id = ?", now, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Supplier deleted successfully",
	})
}

// =========================================================================
// 3. MANAGE STOCKS
// =========================================================================

// GetStocks handles GET /api/v2/admin/manage-stocks and /api/v2/admin/stocks
func GetStocks(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	warehouseID := c.Query("warehouse_id")
	productID := c.Query("product_id")
	alert := c.Query("alert")

	whereParts := []string{"1=1"}
	args := []interface{}{}

	if warehouseID != "" {
		whereParts = append(whereParts, "ms.warehouse_id = ?")
		args = append(args, warehouseID)
	}
	if productID != "" {
		whereParts = append(whereParts, "ms.product_id = ?")
		args = append(args, productID)
	}
	if alert == "1" || alert == "true" {
		whereParts = append(whereParts, "ms.alert = 1")
	}

	whereClause := strings.Join(whereParts, " AND ")

	query := fmt.Sprintf(`
		SELECT ms.id, ms.warehouse_id, ms.product_id, ms.variant_id, ms.quantity, ms.created_at, ms.updated_at, ms.alert,
		       COALESCE(w.name, '') as warehouse_name
		FROM manage_stocks ms
		LEFT JOIN warehouses w ON ms.warehouse_id = w.id
		WHERE %s
		ORDER BY ms.id DESC`, whereClause)

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	defer rows.Close()

	stocks := []models.ManageStock{}
	for rows.Next() {
		var s models.ManageStock
		if err := rows.Scan(&s.ID, &s.WarehouseID, &s.ProductID, &s.VariantID, &s.Quantity, &s.CreatedAt, &s.UpdatedAt, &s.Alert, &s.WarehouseName); err == nil {
			stocks = append(stocks, s)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stocks,
		"total":   len(stocks),
	})
}

// UpdateStock handles PUT /api/v2/admin/manage-stocks/:id
func UpdateStock(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	id := c.Param("id")
	var req struct {
		Quantity float64 `json:"quantity"`
		Alert    *int    `json:"alert"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	now := time.Now()
	_, err = db.Exec("UPDATE manage_stocks SET quantity = ?, alert = COALESCE(?, alert), updated_at = ? WHERE id = ?",
		req.Quantity, req.Alert, now, id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Stock updated successfully"})
}

// =========================================================================
// 4. PURCHASES (MUA HÀNG)
// =========================================================================

// GetPurchases handles GET /api/v2/admin/purchases and /api/v2/admin/purchase
func GetPurchases(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	offset := (page - 1) * perPage

	warehouseID := c.Query("warehouse_id")
	supplierID := c.Query("supplier_id")
	search := strings.TrimSpace(c.Query("search"))

	whereParts := []string{"p.deleted_at IS NULL"}
	args := []interface{}{}

	if warehouseID != "" {
		whereParts = append(whereParts, "p.warehouse_id = ?")
		args = append(args, warehouseID)
	}
	if supplierID != "" {
		whereParts = append(whereParts, "p.supplier_id = ?")
		args = append(args, supplierID)
	}
	if search != "" {
		whereParts = append(whereParts, "(p.purchases_number LIKE ? OR p.note LIKE ?)")
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm)
	}

	whereClause := strings.Join(whereParts, " AND ")

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM purchases p WHERE %s", whereClause)
	_ = db.QueryRow(countQuery, args...).Scan(&total)

	query := fmt.Sprintf(`
		SELECT p.id, p.purchases_number, p.warehouse_id, p.supplier_id, p.payment_status, p.stock_status,
		       p.note, p.total, p.debt, p.total_paid, p.status, p.date, p.tax_rate, p.tax_amount, 
		       p.shipping, p.discount, p.created_at, p.updated_at,
		       COALESCE(w.name, '') as warehouse_name,
		       COALESCE(s.name, '') as supplier_name
		FROM purchases p
		LEFT JOIN warehouses w ON p.warehouse_id = w.id
		LEFT JOIN suppliers s ON p.supplier_id = s.id
		WHERE %s
		ORDER BY p.id DESC
		LIMIT ? OFFSET ?`, whereClause)
	queryArgs := append(args, perPage, offset)

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	defer rows.Close()

	purchases := []models.Purchase{}
	for rows.Next() {
		var p models.Purchase
		if err := rows.Scan(
			&p.ID, &p.PurchasesNumber, &p.WarehouseID, &p.SupplierID, &p.PaymentStatus, &p.StockStatus,
			&p.Note, &p.Total, &p.Debt, &p.TotalPaid, &p.Status, &p.Date, &p.TaxRate, &p.TaxAmount,
			&p.Shipping, &p.Discount, &p.CreatedAt, &p.UpdatedAt,
			&p.WarehouseName, &p.SupplierName,
		); err == nil {
			p.ReferenceCode = p.PurchasesNumber
			purchases = append(purchases, p)
		}
	}

	lastPage := int(math.Ceil(float64(total) / float64(perPage)))
	if lastPage < 1 {
		lastPage = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      purchases,
		"total":     total,
		"page":      page,
		"per_page":  perPage,
		"last_page": lastPage,
	})
}

// GetPurchase handles GET /api/v2/admin/purchases/:id
func GetPurchase(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	id := c.Param("id")
	var p models.Purchase
	err = db.QueryRow(`
		SELECT p.id, p.purchases_number, p.warehouse_id, p.supplier_id, p.payment_status, p.stock_status,
		       p.note, p.total, p.debt, p.total_paid, p.status, p.date, p.tax_rate, p.tax_amount, 
		       p.shipping, p.discount, p.created_at, p.updated_at,
		       COALESCE(w.name, '') as warehouse_name,
		       COALESCE(s.name, '') as supplier_name
		FROM purchases p
		LEFT JOIN warehouses w ON p.warehouse_id = w.id
		LEFT JOIN suppliers s ON p.supplier_id = s.id
		WHERE p.id = ? AND p.deleted_at IS NULL LIMIT 1`, id).
		Scan(
			&p.ID, &p.PurchasesNumber, &p.WarehouseID, &p.SupplierID, &p.PaymentStatus, &p.StockStatus,
			&p.Note, &p.Total, &p.Debt, &p.TotalPaid, &p.Status, &p.Date, &p.TaxRate, &p.TaxAmount,
			&p.Shipping, &p.Discount, &p.CreatedAt, &p.UpdatedAt,
			&p.WarehouseName, &p.SupplierName,
		)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Purchase not found"})
		return
	}

	p.ReferenceCode = p.PurchasesNumber

	// Fetch items
	itemRows, err := db.Query(`
		SELECT id, purchase_id, product_id, product_cost, net_unit_cost, tax_type, tax_value, tax_amount,
		       discount_type, discount_value, discount_amount, purchase_unit, quantity, sub_total, created_at, updated_at
		FROM purchase_items 
		WHERE purchase_id = ?`, p.ID)
	if err == nil {
		defer itemRows.Close()
		items := []models.PurchaseItem{}
		for itemRows.Next() {
			var it models.PurchaseItem
			if err := itemRows.Scan(
				&it.ID, &it.PurchaseID, &it.ProductID, &it.ProductCost, &it.NetUnitCost, &it.TaxType, &it.TaxValue, &it.TaxAmount,
				&it.DiscountType, &it.DiscountValue, &it.DiscountAmount, &it.PurchaseUnit, &it.Quantity, &it.SubTotal, &it.CreatedAt, &it.UpdatedAt,
			); err == nil {
				items = append(items, it)
			}
		}
		p.Items = items
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": p})
}

// CreatePurchase handles POST /api/v2/admin/purchases
func CreatePurchase(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	var req struct {
		PurchasesNumber *string  `json:"purchases_number"`
		ReferenceCode   *string  `json:"reference_code"`
		WarehouseID     uint32   `json:"warehouse_id"`
		SupplierID      uint32   `json:"supplier_id"`
		Date            string   `json:"date"`
		Status          int      `json:"status"`         // 0: Pending, 1: Received, 2: Completed
		PaymentStatus   int      `json:"payment_status"` // 1: Unpaid, 2: Partial, 3: Paid
		StockStatus     uint32   `json:"stock_status"`   // 1: In stock / received
		Note            *string  `json:"note"`
		Total           *float64 `json:"total"`
		TotalPaid       float64  `json:"total_paid"`
		Debt            *string  `json:"debt"`
		TaxRate         *float64 `json:"tax_rate"`
		TaxAmount       *float64 `json:"tax_amount"`
		Shipping        *float64 `json:"shipping"`
		Discount        *float64 `json:"discount"`
		Items           []struct {
			ProductID      uint64   `json:"product_id"`
			ProductCost    *float64 `json:"product_cost"`
			NetUnitCost    *float64 `json:"net_unit_cost"`
			TaxType        int      `json:"tax_type"`
			TaxValue       *float64 `json:"tax_value"`
			TaxAmount      *float64 `json:"tax_amount"`
			DiscountType   int      `json:"discount_type"`
			DiscountValue  *float64 `json:"discount_value"`
			DiscountAmount *float64 `json:"discount_amount"`
			PurchaseUnit   int      `json:"purchase_unit"`
			Quantity       float64  `json:"quantity"`
			SubTotal       *float64 `json:"sub_total"`
		} `json:"items"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	ref := req.PurchasesNumber
	if ref == nil || *ref == "" {
		ref = req.ReferenceCode
	}
	if ref == nil || *ref == "" {
		code := fmt.Sprintf("PO-%s", time.Now().Format("20060102150405"))
		ref = &code
	}

	dateVal := req.Date
	if dateVal == "" {
		dateVal = time.Now().Format("2006-01-02")
	}

	now := time.Now()
	res, err := db.Exec(`
		INSERT INTO purchases (purchases_number, warehouse_id, supplier_id, payment_status, stock_status, note, total, debt, total_paid, status, date, tax_rate, tax_amount, shipping, discount, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ref, req.WarehouseID, req.SupplierID, req.PaymentStatus, req.StockStatus, req.Note, req.Total, req.Debt, req.TotalPaid, req.Status, dateVal, req.TaxRate, req.TaxAmount, req.Shipping, req.Discount, now, now,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	purchaseID, _ := res.LastInsertId()

	// Insert purchase items & update stock
	for _, it := range req.Items {
		if it.ProductID == 0 || it.Quantity <= 0 {
			continue
		}
		_, _ = db.Exec(`
			INSERT INTO purchase_items (purchase_id, product_id, product_cost, net_unit_cost, tax_type, tax_value, tax_amount, discount_type, discount_value, discount_amount, purchase_unit, quantity, sub_total, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			purchaseID, it.ProductID, it.ProductCost, it.NetUnitCost, it.TaxType, it.TaxValue, it.TaxAmount, it.DiscountType, it.DiscountValue, it.DiscountAmount, it.PurchaseUnit, it.Quantity, it.SubTotal, now, now,
		)

		// Also insert into purchase_details
		_, _ = db.Exec(`
			INSERT INTO purchase_details (product_id, quantity, purchase_price, purchase_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			it.ProductID, int(it.Quantity), it.ProductCost, purchaseID, now, now,
		)

		// Increment stock if received/status completed or in-stock
		if req.StockStatus == 1 || req.Status == 1 || req.Status == 2 {
			_ = adjustStockHelper(db, uint64(req.WarehouseID), it.ProductID, nil, doubleOrFloat(it.Quantity))
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Purchase order created successfully",
		"data": gin.H{
			"id":               purchaseID,
			"purchases_number": ref,
			"reference_code":   ref,
		},
	})
}

// DeletePurchase handles DELETE /api/v2/admin/purchases/:id
func DeletePurchase(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	id := c.Param("id")
	now := time.Now()
	_, err = db.Exec("UPDATE purchases SET deleted_at = ? WHERE id = ?", now, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Purchase deleted successfully"})
}

// =========================================================================
// 5. PURCHASE RETURNS (TRẢ HÀNG MUA)
// =========================================================================

// GetPurchaseReturns handles GET /api/v2/admin/purchase-returns and /api/v2/admin/purchase-return
func GetPurchaseReturns(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	offset := (page - 1) * perPage

	var total int
	_ = db.QueryRow("SELECT COUNT(*) FROM purchase_returns").Scan(&total)

	query := `
		SELECT pr.id, pr.date, pr.supplier_id, pr.warehouse_id, pr.tax_rate, pr.tax_amount, pr.discount,
		       pr.shipping, pr.grand_total, pr.received_amount, pr.paid_amount, pr.payment_type, pr.status,
		       pr.payment_status, pr.notes, pr.reference_code, pr.created_at, pr.updated_at,
		       COALESCE(w.name, '') as warehouse_name,
		       COALESCE(s.name, '') as supplier_name
		FROM purchase_returns pr
		LEFT JOIN warehouses w ON pr.warehouse_id = w.id
		LEFT JOIN suppliers s ON pr.supplier_id = s.id
		ORDER BY pr.id DESC
		LIMIT ? OFFSET ?`

	rows, err := db.Query(query, perPage, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	defer rows.Close()

	returns := []models.PurchaseReturn{}
	for rows.Next() {
		var pr models.PurchaseReturn
		if err := rows.Scan(
			&pr.ID, &pr.Date, &pr.SupplierID, &pr.WarehouseID, &pr.TaxRate, &pr.TaxAmount, &pr.Discount,
			&pr.Shipping, &pr.GrandTotal, &pr.ReceivedAmount, &pr.PaidAmount, &pr.PaymentType, &pr.Status,
			&pr.PaymentStatus, &pr.Notes, &pr.ReferenceCode, &pr.CreatedAt, &pr.UpdatedAt,
			&pr.WarehouseName, &pr.SupplierName,
		); err == nil {
			returns = append(returns, pr)
		}
	}

	lastPage := int(math.Ceil(float64(total) / float64(perPage)))
	if lastPage < 1 {
		lastPage = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      returns,
		"total":     total,
		"page":      page,
		"per_page":  perPage,
		"last_page": lastPage,
	})
}

// GetPurchaseReturn handles GET /api/v2/admin/purchase-returns/:id
func GetPurchaseReturn(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	id := c.Param("id")
	var pr models.PurchaseReturn
	err = db.QueryRow(`
		SELECT pr.id, pr.date, pr.supplier_id, pr.warehouse_id, pr.tax_rate, pr.tax_amount, pr.discount,
		       pr.shipping, pr.grand_total, pr.received_amount, pr.paid_amount, pr.payment_type, pr.status,
		       pr.payment_status, pr.notes, pr.reference_code, pr.created_at, pr.updated_at,
		       COALESCE(w.name, '') as warehouse_name,
		       COALESCE(s.name, '') as supplier_name
		FROM purchase_returns pr
		LEFT JOIN warehouses w ON pr.warehouse_id = w.id
		LEFT JOIN suppliers s ON pr.supplier_id = s.id
		WHERE pr.id = ? LIMIT 1`, id).
		Scan(
			&pr.ID, &pr.Date, &pr.SupplierID, &pr.WarehouseID, &pr.TaxRate, &pr.TaxAmount, &pr.Discount,
			&pr.Shipping, &pr.GrandTotal, &pr.ReceivedAmount, &pr.PaidAmount, &pr.PaymentType, &pr.Status,
			&pr.PaymentStatus, &pr.Notes, &pr.ReferenceCode, &pr.CreatedAt, &pr.UpdatedAt,
			&pr.WarehouseName, &pr.SupplierName,
		)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Purchase return not found"})
		return
	}

	// Items
	itemRows, err := db.Query(`
		SELECT id, purchase_return_id, product_id, product_cost, net_unit_cost, tax_type, tax_value, tax_amount,
		       discount_type, discount_value, discount_amount, purchase_unit, quantity, sub_total, created_at, updated_at
		FROM purchase_return_items
		WHERE purchase_return_id = ?`, pr.ID)
	if err == nil {
		defer itemRows.Close()
		items := []models.PurchaseReturnItem{}
		for itemRows.Next() {
			var it models.PurchaseReturnItem
			if err := itemRows.Scan(
				&it.ID, &it.PurchaseReturnID, &it.ProductID, &it.ProductCost, &it.NetUnitCost, &it.TaxType, &it.TaxValue, &it.TaxAmount,
				&it.DiscountType, &it.DiscountValue, &it.DiscountAmount, &it.PurchaseUnit, &it.Quantity, &it.SubTotal, &it.CreatedAt, &it.UpdatedAt,
			); err == nil {
				items = append(items, it)
			}
		}
		pr.Items = items
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": pr})
}

// CreatePurchaseReturn handles POST /api/v2/admin/purchase-returns
func CreatePurchaseReturn(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	var req struct {
		Date           string   `json:"date"`
		SupplierID     uint64   `json:"supplier_id"`
		WarehouseID    uint64   `json:"warehouse_id"`
		TaxRate        *float64 `json:"tax_rate"`
		TaxAmount      *float64 `json:"tax_amount"`
		Discount       *float64 `json:"discount"`
		Shipping       *float64 `json:"shipping"`
		GrandTotal     *float64 `json:"grand_total"`
		ReceivedAmount *float64 `json:"received_amount"`
		PaidAmount     *float64 `json:"paid_amount"`
		PaymentType    *int     `json:"payment_type"`
		Status         *int     `json:"status"`
		PaymentStatus  *int     `json:"payment_status"`
		Notes          *string  `json:"notes"`
		ReferenceCode  *string  `json:"reference_code"`
		Items          []struct {
			ProductID      uint64   `json:"product_id"`
			ProductCost    *float64 `json:"product_cost"`
			NetUnitCost    *float64 `json:"net_unit_cost"`
			TaxType        int      `json:"tax_type"`
			TaxValue       *float64 `json:"tax_value"`
			TaxAmount      *float64 `json:"tax_amount"`
			DiscountType   int      `json:"discount_type"`
			DiscountValue  *float64 `json:"discount_value"`
			DiscountAmount *float64 `json:"discount_amount"`
			PurchaseUnit   int      `json:"purchase_unit"`
			Quantity       float64  `json:"quantity"`
			SubTotal       *float64 `json:"sub_total"`
		} `json:"items"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	ref := req.ReferenceCode
	if ref == nil || *ref == "" {
		code := fmt.Sprintf("PR-%s", time.Now().Format("20060102150405"))
		ref = &code
	}

	dateVal := req.Date
	if dateVal == "" {
		dateVal = time.Now().Format("2006-01-02")
	}

	now := time.Now()
	res, err := db.Exec(`
		INSERT INTO purchase_returns (date, supplier_id, warehouse_id, tax_rate, tax_amount, discount, shipping, grand_total, received_amount, paid_amount, payment_type, status, payment_status, notes, reference_code, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dateVal, req.SupplierID, req.WarehouseID, req.TaxRate, req.TaxAmount, req.Discount, req.Shipping, req.GrandTotal, req.ReceivedAmount, req.PaidAmount, req.PaymentType, req.Status, req.PaymentStatus, req.Notes, ref, now, now,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	returnID, _ := res.LastInsertId()

	for _, it := range req.Items {
		if it.ProductID == 0 || it.Quantity <= 0 {
			continue
		}
		_, _ = db.Exec(`
			INSERT INTO purchase_return_items (purchase_return_id, product_id, product_cost, net_unit_cost, tax_type, tax_value, tax_amount, discount_type, discount_value, discount_amount, purchase_unit, quantity, sub_total, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			returnID, it.ProductID, it.ProductCost, it.NetUnitCost, it.TaxType, it.TaxValue, it.TaxAmount, it.DiscountType, it.DiscountValue, it.DiscountAmount, it.PurchaseUnit, it.Quantity, it.SubTotal, now, now,
		)

		// Reduce stock because goods are returned to supplier
		_ = adjustStockHelper(db, req.WarehouseID, it.ProductID, nil, doubleOrFloat(-it.Quantity))
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Purchase return created successfully",
		"data": gin.H{
			"id":             returnID,
			"reference_code": ref,
		},
	})
}

// DeletePurchaseReturn handles DELETE /api/v2/admin/purchase-returns/:id
func DeletePurchaseReturn(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	id := c.Param("id")
	_, _ = db.Exec("DELETE FROM purchase_return_items WHERE purchase_return_id = ?", id)
	_, err = db.Exec("DELETE FROM purchase_returns WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Purchase return deleted successfully"})
}

// =========================================================================
// 6. TRANSFERS (ĐỔI HÀNG GIỮA CÁC KHO)
// =========================================================================

// GetTransfers handles GET /api/v2/admin/transfers and /api/v2/admin/transfer
func GetTransfers(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	offset := (page - 1) * perPage

	var total int
	_ = db.QueryRow("SELECT COUNT(*) FROM transfers").Scan(&total)

	query := `
		SELECT t.id, t.date, t.from_warehouse_id, t.to_warehouse_id, t.tax_rate, t.tax_amount, t.discount,
		       t.shipping, t.grand_total, t.status, t.note, t.reference_code, t.created_at, t.updated_at,
		       COALESCE(w1.name, '') as from_warehouse_name,
		       COALESCE(w2.name, '') as to_warehouse_name
		FROM transfers t
		LEFT JOIN warehouses w1 ON t.from_warehouse_id = w1.id
		LEFT JOIN warehouses w2 ON t.to_warehouse_id = w2.id
		ORDER BY t.id DESC
		LIMIT ? OFFSET ?`

	rows, err := db.Query(query, perPage, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	defer rows.Close()

	transfers := []models.Transfer{}
	for rows.Next() {
		var tr models.Transfer
		if err := rows.Scan(
			&tr.ID, &tr.Date, &tr.FromWarehouseID, &tr.ToWarehouseID, &tr.TaxRate, &tr.TaxAmount, &tr.Discount,
			&tr.Shipping, &tr.GrandTotal, &tr.Status, &tr.Note, &tr.ReferenceCode, &tr.CreatedAt, &tr.UpdatedAt,
			&tr.FromWarehouseName, &tr.ToWarehouseName,
		); err == nil {
			transfers = append(transfers, tr)
		}
	}

	lastPage := int(math.Ceil(float64(total) / float64(perPage)))
	if lastPage < 1 {
		lastPage = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      transfers,
		"total":     total,
		"page":      page,
		"per_page":  perPage,
		"last_page": lastPage,
	})
}

// GetTransfer handles GET /api/v2/admin/transfers/:id
func GetTransfer(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	id := c.Param("id")
	var tr models.Transfer
	err = db.QueryRow(`
		SELECT t.id, t.date, t.from_warehouse_id, t.to_warehouse_id, t.tax_rate, t.tax_amount, t.discount,
		       t.shipping, t.grand_total, t.status, t.note, t.reference_code, t.created_at, t.updated_at,
		       COALESCE(w1.name, '') as from_warehouse_name,
		       COALESCE(w2.name, '') as to_warehouse_name
		FROM transfers t
		LEFT JOIN warehouses w1 ON t.from_warehouse_id = w1.id
		LEFT JOIN warehouses w2 ON t.to_warehouse_id = w2.id
		WHERE t.id = ? LIMIT 1`, id).
		Scan(
			&tr.ID, &tr.Date, &tr.FromWarehouseID, &tr.ToWarehouseID, &tr.TaxRate, &tr.TaxAmount, &tr.Discount,
			&tr.Shipping, &tr.GrandTotal, &tr.Status, &tr.Note, &tr.ReferenceCode, &tr.CreatedAt, &tr.UpdatedAt,
			&tr.FromWarehouseName, &tr.ToWarehouseName,
		)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Transfer not found"})
		return
	}

	// Items
	itemRows, err := db.Query(`
		SELECT id, transfer_id, product_id, product_cost, net_unit_price, tax_type, tax_value, tax_amount,
		       discount_type, discount_value, discount_amount, quantity, sub_total, created_at, updated_at
		FROM transfer_items
		WHERE transfer_id = ?`, tr.ID)
	if err == nil {
		defer itemRows.Close()
		items := []models.TransferItem{}
		for itemRows.Next() {
			var it models.TransferItem
			if err := itemRows.Scan(
				&it.ID, &it.TransferID, &it.ProductID, &it.ProductCost, &it.NetUnitPrice, &it.TaxType, &it.TaxValue, &it.TaxAmount,
				&it.DiscountType, &it.DiscountValue, &it.DiscountAmount, &it.Quantity, &it.SubTotal, &it.CreatedAt, &it.UpdatedAt,
			); err == nil {
				items = append(items, it)
			}
		}
		tr.Items = items
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": tr})
}

// CreateTransfer handles POST /api/v2/admin/transfers
func CreateTransfer(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	var req struct {
		Date            string   `json:"date"`
		FromWarehouseID uint64   `json:"from_warehouse_id"`
		ToWarehouseID   uint64   `json:"to_warehouse_id"`
		TaxRate         *float64 `json:"tax_rate"`
		TaxAmount       *float64 `json:"tax_amount"`
		Discount        *float64 `json:"discount"`
		Shipping        *float64 `json:"shipping"`
		GrandTotal      *float64 `json:"grand_total"`
		Status          *int     `json:"status"` // 1: Completed, 2: Pending, 3: Sent
		Note            *string  `json:"note"`
		ReferenceCode   *string  `json:"reference_code"`
		Items           []struct {
			ProductID      uint64   `json:"product_id"`
			ProductCost    *float64 `json:"product_cost"`
			NetUnitPrice   *float64 `json:"net_unit_price"`
			TaxType        int      `json:"tax_type"`
			TaxValue       *float64 `json:"tax_value"`
			TaxAmount      *float64 `json:"tax_amount"`
			DiscountType   int      `json:"discount_type"`
			DiscountValue  *float64 `json:"discount_value"`
			DiscountAmount *float64 `json:"discount_amount"`
			Quantity       float64  `json:"quantity"`
			SubTotal       *float64 `json:"sub_total"`
		} `json:"items"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if req.FromWarehouseID == req.ToWarehouseID {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Kho nguồn và kho đích không được trùng nhau"})
		return
	}

	ref := req.ReferenceCode
	if ref == nil || *ref == "" {
		code := fmt.Sprintf("TR-%s", time.Now().Format("20060102150405"))
		ref = &code
	}

	dateVal := req.Date
	if dateVal == "" {
		dateVal = time.Now().Format("2006-01-02")
	}

	now := time.Now()
	res, err := db.Exec(`
		INSERT INTO transfers (date, from_warehouse_id, to_warehouse_id, tax_rate, tax_amount, discount, shipping, grand_total, status, note, reference_code, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dateVal, req.FromWarehouseID, req.ToWarehouseID, req.TaxRate, req.TaxAmount, req.Discount, req.Shipping, req.GrandTotal, req.Status, req.Note, ref, now, now,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	transferID, _ := res.LastInsertId()

	for _, it := range req.Items {
		if it.ProductID == 0 || it.Quantity <= 0 {
			continue
		}
		_, _ = db.Exec(`
			INSERT INTO transfer_items (transfer_id, product_id, product_cost, net_unit_price, tax_type, tax_value, tax_amount, discount_type, discount_value, discount_amount, quantity, sub_total, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			transferID, it.ProductID, it.ProductCost, it.NetUnitPrice, it.TaxType, it.TaxValue, it.TaxAmount, it.DiscountType, it.DiscountValue, it.DiscountAmount, it.Quantity, it.SubTotal, now, now,
		)

		// Stock transfer: subtract from source warehouse, add to destination warehouse
		_ = adjustStockHelper(db, req.FromWarehouseID, it.ProductID, nil, doubleOrFloat(-it.Quantity))
		_ = adjustStockHelper(db, req.ToWarehouseID, it.ProductID, nil, doubleOrFloat(it.Quantity))
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Stock transfer created successfully",
		"data": gin.H{
			"id":             transferID,
			"reference_code": ref,
		},
	})
}

// DeleteTransfer handles DELETE /api/v2/admin/transfers/:id
func DeleteTransfer(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	id := c.Param("id")
	_, _ = db.Exec("DELETE FROM transfer_items WHERE transfer_id = ?", id)
	_, err = db.Exec("DELETE FROM transfers WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Transfer deleted successfully"})
}

// =========================================================================
// 7. ADJUSTMENTS (ĐIỀU CHỈNH SỐ LƯỢNG)
// =========================================================================

// GetAdjustments handles GET /api/v2/admin/adjustments and /api/v2/admin/adjustment
func GetAdjustments(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	offset := (page - 1) * perPage

	var total int
	_ = db.QueryRow("SELECT COUNT(*) FROM adjustments").Scan(&total)

	query := `
		SELECT a.id, a.date, a.reference_code, a.warehouse_id, a.total_products, a.created_at, a.updated_at,
		       COALESCE(w.name, '') as warehouse_name
		FROM adjustments a
		LEFT JOIN warehouses w ON a.warehouse_id = w.id
		ORDER BY a.id DESC
		LIMIT ? OFFSET ?`

	rows, err := db.Query(query, perPage, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	defer rows.Close()

	adjustments := []models.Adjustment{}
	for rows.Next() {
		var a models.Adjustment
		if err := rows.Scan(
			&a.ID, &a.Date, &a.ReferenceCode, &a.WarehouseID, &a.TotalProducts, &a.CreatedAt, &a.UpdatedAt,
			&a.WarehouseName,
		); err == nil {
			adjustments = append(adjustments, a)
		}
	}

	lastPage := int(math.Ceil(float64(total) / float64(perPage)))
	if lastPage < 1 {
		lastPage = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      adjustments,
		"total":     total,
		"page":      page,
		"per_page":  perPage,
		"last_page": lastPage,
	})
}

// GetAdjustment handles GET /api/v2/admin/adjustments/:id
func GetAdjustment(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	id := c.Param("id")
	var a models.Adjustment
	err = db.QueryRow(`
		SELECT a.id, a.date, a.reference_code, a.warehouse_id, a.total_products, a.created_at, a.updated_at,
		       COALESCE(w.name, '') as warehouse_name
		FROM adjustments a
		LEFT JOIN warehouses w ON a.warehouse_id = w.id
		WHERE a.id = ? LIMIT 1`, id).
		Scan(&a.ID, &a.Date, &a.ReferenceCode, &a.WarehouseID, &a.TotalProducts, &a.CreatedAt, &a.UpdatedAt, &a.WarehouseName)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Adjustment not found"})
		return
	}

	// Items
	itemRows, err := db.Query(`
		SELECT id, adjustment_id, product_id, quantity, method_type, created_at, updated_at
		FROM adjustment_items
		WHERE adjustment_id = ?`, a.ID)
	if err == nil {
		defer itemRows.Close()
		items := []models.AdjustmentItem{}
		for itemRows.Next() {
			var it models.AdjustmentItem
			if err := itemRows.Scan(&it.ID, &it.AdjustmentID, &it.ProductID, &it.Quantity, &it.MethodType, &it.CreatedAt, &it.UpdatedAt); err == nil {
				items = append(items, it)
			}
		}
		a.Items = items
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": a})
}

// CreateAdjustment handles POST /api/v2/admin/adjustments
func CreateAdjustment(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	var req struct {
		Date          string  `json:"date"`
		ReferenceCode *string `json:"reference_code"`
		WarehouseID   uint64  `json:"warehouse_id"`
		TotalProducts *int    `json:"total_products"`
		Items         []struct {
			ProductID  uint64  `json:"product_id"`
			Quantity   float64 `json:"quantity"`
			MethodType int     `json:"method_type"` // 1: Addition (+), 2: Subtraction (-)
		} `json:"items"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	ref := req.ReferenceCode
	if ref == nil || *ref == "" {
		code := fmt.Sprintf("ADJ-%s", time.Now().Format("20060102150405"))
		ref = &code
	}

	dateVal := req.Date
	if dateVal == "" {
		dateVal = time.Now().Format("2006-01-02")
	}

	totalProducts := len(req.Items)
	if req.TotalProducts != nil {
		totalProducts = *req.TotalProducts
	}

	now := time.Now()
	res, err := db.Exec(`
		INSERT INTO adjustments (date, reference_code, warehouse_id, total_products, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		dateVal, ref, req.WarehouseID, totalProducts, now, now,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	adjID, _ := res.LastInsertId()

	for _, it := range req.Items {
		if it.ProductID == 0 || it.Quantity <= 0 {
			continue
		}
		_, _ = db.Exec(`
			INSERT INTO adjustment_items (adjustment_id, product_id, quantity, method_type, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			adjID, it.ProductID, it.Quantity, it.MethodType, now, now,
		)

		// Adjust stock: MethodType 1 = Add, MethodType 2 = Subtract
		diff := doubleOrFloat(it.Quantity)
		if it.MethodType == 2 {
			diff = doubleOrFloat(-it.Quantity)
		}
		_ = adjustStockHelper(db, req.WarehouseID, it.ProductID, nil, diff)
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Adjustment created successfully",
		"data": gin.H{
			"id":             adjID,
			"reference_code": ref,
		},
	})
}

// DeleteAdjustment handles DELETE /api/v2/admin/adjustments/:id
func DeleteAdjustment(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection error"})
		return
	}

	id := c.Param("id")
	_, _ = db.Exec("DELETE FROM adjustment_items WHERE adjustment_id = ?", id)
	_, err = db.Exec("DELETE FROM adjustments WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Adjustment deleted successfully"})
}
