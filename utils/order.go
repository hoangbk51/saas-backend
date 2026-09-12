package utils

import "go-saas/models"

func MapOrderDetailToOrder(detail *models.OrderDetailResponse) *models.Order {
	if detail == nil {
		return nil
	}

	return &models.Order{
		ID:                 detail.ID,
		OrderNumber:        detail.OrderNumber,
		GrandTotal:         detail.GrandTotal,
		CustomerID:         detail.CustomerID,
		PaymentMethodCode:  detail.PaymentMethodCode,
		ShippingMethodCode: detail.ShippingMethodCode,
		OrderStatusID:      detail.OrderStatusId,
		CouponID:           detail.CouponID,
		Email:              detail.Email,
		CustomerPhone:      detail.CustomerPhone,
		BillingFirstName:   detail.BillingFirstName,
		BillingLastName:    detail.BillingLastName,
		BillingAddress1:    detail.BillingAddress1,
		BillingAddress2:    detail.BillingAddress2,
		BillingCity:        detail.BillingCity,
		BillingState:       detail.BillingState,
		BillingCountry:     detail.BillingCountry,
		BillingZip:         detail.BillingZip,
		ShippingFirstName:  detail.ShippingFirstName,
		ShippingLastName:   detail.ShippingLastName,
		ShippingAddress1:   detail.ShippingAddress1,
		ShippingAddress2:   detail.ShippingAddress2,
		ShippingState:      detail.ShippingState,
		ShippingCountry:    detail.ShippingCountry,
		ShippingCity:       detail.ShippingCity,
		ShippingZip:        detail.ShippingZip,
		ShippingWard:       detail.ShippingWard,

		TotalWeight: detail.TotalWeight,
	}
}
