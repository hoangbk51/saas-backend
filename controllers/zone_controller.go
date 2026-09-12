package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"go-saas/models"
	"go-saas/services"

	"github.com/gin-gonic/gin"
)

// Helper parse zones[x][country_id] & zones[x][state_id] từ Form-Data
func parseZoneDetailsFromForm(c *gin.Context) []models.ZoneDetailItem {
	_ = c.Request.ParseMultipartForm(32 << 20)
	details := make([]models.ZoneDetailItem, 0)

	for i := 0; ; i++ {
		countryKey := fmt.Sprintf("zones[%d][country_id]", i)
		stateKey := fmt.Sprintf("zones[%d][state_id]", i)

		countryVal := c.PostForm(countryKey)
		stateVal := c.PostForm(stateKey)

		if countryVal == "" {
			break
		}

		countryID, _ := strconv.Atoi(countryVal)
		stateID, _ := strconv.Atoi(stateVal)

		details = append(details, models.ZoneDetailItem{
			CountryID: countryID,
			StateID:   stateID,
		})
	}

	return details
}

// GET /admin/zones
func GetZones(c *gin.Context) {
	res, statusCode, _ := services.GetZonesService(c)
	c.JSON(statusCode, res)
}

// GET /admin/zones/:id
func GetZoneByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	res, statusCode, _ := services.GetZoneByIDService(c, id)
	c.JSON(statusCode, res)
}

// POST /admin/zones
func CreateZone(c *gin.Context) {
	var req models.CreateZoneReq

	if c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		req.Name = c.PostForm("name")
		req.Description = c.PostForm("description")

		if req.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Tên zone không được để trống"})
			return
		}

		req.Details = parseZoneDetailsFromForm(c)
	}

	res, statusCode, _ := services.CreateZoneService(c, req)
	c.JSON(statusCode, res)
}

// PUT /admin/zones/:id
func UpdateZone(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	var req models.UpdateZoneReq

	if c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		req.Name = c.PostForm("name")
		req.Description = c.PostForm("description")

		if req.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Tên zone không được để trống"})
			return
		}

		req.Details = parseZoneDetailsFromForm(c)
	}

	res, statusCode, _ := services.UpdateZoneService(c, id, req)
	c.JSON(statusCode, res)
}

// DELETE /admin/zones/:id
func DeleteZone(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	res, statusCode, _ := services.DeleteZoneService(c, id)
	c.JSON(statusCode, res)
}
