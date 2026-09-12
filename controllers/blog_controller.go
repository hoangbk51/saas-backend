package controllers

import (
	"go-saas/models"
	"go-saas/services" // Thay bằng path thực tế của bạn
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func AllBlogs(c *gin.Context) {

	blogs, err := services.GetAllBlogs(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"data": blogs})
}

func GetBlogsByIds(c *gin.Context) {

	blogs, err := services.GetBlogByIds(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"data": blogs})
}

func StoreBlog(c *gin.Context) {
	var input models.Blog
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu JSON không hợp lệ: " + err.Error()})
		return
	}

	// Lấy UserID từ Auth Middleware (giả sử đã lưu vào context)
	// input.UserID = c.MustGet("user_id").(uint64)

	id, err := services.CreateBlog(c, &input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể tạo bài viết"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "message": "Blog đã được lưu thành công"})
}

func ListBlogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	blogs, err := services.GetBlogs(c, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi truy vấn danh sách"})
		return
	}
	c.JSON(http.StatusOK, blogs)
}

func UpdateBlog(c *gin.Context) {
	id := c.Param("id")
	var input models.Blog
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := services.UpdateBlog(c, id, &input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật thất bại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Cập nhật thành công"})
}
func DeleteBlog(c *gin.Context) {
	id := c.Param("id")
	if err := services.DeleteBlog(c, id); err != nil {
		c.JSON(500, gin.H{"error": "Xóa thất bại"})
		return
	}
	c.JSON(200, gin.H{"message": "Đã xóa bản ghi"})
}

func GetBlogDetailById(c *gin.Context) {
	id := c.Param("id")

	blog, err := services.GetBlogDetailById(c, id)
	if err != nil {
		c.JSON(404, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status": "success",
		"data":   blog,
	})
}
