package controllers

import (
	"go-saas/services" // Thay bằng path thực tế của bạn
	"net/http"

	"github.com/gin-gonic/gin"
)

func StatesByCountry(c *gin.Context) {
	countryID := c.Param("id")

	if countryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Country ID is required"})
		return
	}

	states, err := services.StatesByCountry(c, countryID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"data": states})
}
