package controllers

import (
	"go-saas/services" // Thay bằng path thực tế của bạn
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func AllLanguages(c *gin.Context) {

	languages, err := services.GetAllLanguages(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"data": languages})
}

func FrontendLanguages(c *gin.Context) {

	languages, err := services.FrontendLanguages(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"data": languages})
}

func ListLanguages(c *gin.Context) {
	list, err := services.ListLanguagesService(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func GetLanguageDetail(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, code, err := services.GetLanguageDetailService(c, id)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

func CreateLanguage(c *gin.Context) {
	res, code, err := services.CreateLanguageService(c)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

func UpdateLanguage(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, code, err := services.UpdateLanguageService(c, id)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

func DeleteLanguage(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, code, err := services.DeleteLanguageService(c, id)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

// --- CURRENCY CONTROLLERS ---

func ListCurrencies(c *gin.Context) {
	list, err := services.ListCurrenciesService(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}
