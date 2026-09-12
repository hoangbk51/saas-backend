package services

import (
	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

func GetBillingHistoryByDomain(c *gin.Context, rawDomain string, filter models.BillingFilter) ([]models.BillingHistory, error) {
	db, err := utils.GetCentralDB()
	if err != nil {
		return nil, err
	}

	cleanedDomain := utils.CleanDomain(rawDomain)

	// Câu SQL cơ bản với LEFT JOIN
	query := `
		SELECT 
			co.id, co.client_id, co.tenant_id, co.total_amount, co.status, 
			co.order_email, co.order_phone, co.payment_method, pm.name as payment_name,
			co.coupon_code, co.sub_total, co.total, co.discount_amount, co.note, 
			co.created_at, co.type, co.object_id,
			IF(co.type = 1, 'Gói Web Hệ Thống', IFNULL(m.name, 'Plugin Không Tồn Tại')) as object_name
		FROM client_orders co
		LEFT JOIN modules m ON co.object_id = m.id AND co.type = 2
		LEFT JOIN payment_methods pm ON co.payment_method = pm.code
		WHERE co.tenant_id = (SELECT tenant_id FROM domains WHERE domain = ? LIMIT 1)
	`

	// Khởi tạo tham số với cleanedDomain làm tham số đầu tiên
	args := []interface{}{cleanedDomain}

	// Thêm điều kiện filter động theo Type
	if filter.Type != "" {
		query += " AND co.type = ?"
		args = append(args, filter.Type)
	}

	// Thêm điều kiện filter động theo khoảng thời gian (Start Date / End Date)
	if filter.StartDate != "" {
		query += " AND co.created_at >= ?"
		args = append(args, filter.StartDate+" 00:00:00")
	}
	if filter.EndDate != "" {
		query += " AND co.created_at <= ?"
		args = append(args, filter.EndDate+" 23:59:59")
	}

	// Sắp xếp hóa đơn mới nhất lên đầu
	query += " ORDER BY co.created_at DESC"
	utils.LogSQL(query, args...)

	rows, err := db.Query(query, args...)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []models.BillingHistory
	for rows.Next() {
		var h models.BillingHistory
		err := rows.Scan(
			&h.ID, &h.ClientID, &h.TenantID, &h.TotalAmount, &h.Status,
			&h.OrderEmail, &h.OrderPhone, &h.PaymentMethod, &h.PaymentName,
			&h.CouponCode, &h.SubTotal, &h.Total, &h.DiscountAmount, &h.Note,
			&h.CreatedAt, &h.Type, &h.ObjectID, &h.ObjectName,
		)
		if err != nil {
			return nil, err
		}
		histories = append(histories, h)
	}

	return histories, nil
}
