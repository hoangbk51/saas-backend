package controllers

import (
	"go-saas/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func DistrictsByState(c *gin.Context) {
	stateID := c.Param("id")

	if stateID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "State ID is required"})
		return
	}

	states, err := services.DistrictsByState(c, stateID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"data": states})
}

func WardsByDistrict(c *gin.Context) {
	districtID := c.Param("id")

	if districtID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "State ID is required"})
		return
	}

	states, err := services.WardsByDistrict(c, districtID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"data": states})
}
