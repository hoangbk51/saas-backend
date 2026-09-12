package utils

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetUserIDFromContext(c *gin.Context) (int64, error) {
	value, exists := c.Get("userId")
	if !exists {
		return 0, fmt.Errorf("userId not found in context")
	}

	switch v := value.(type) {
	case int:
		return int64(v), nil

	case int64:
		return v, nil

	case int32:
		return int64(v), nil

	case uint:
		return int64(v), nil

	case uint64:
		return int64(v), nil

	case string:
		return strconv.ParseInt(v, 10, 64)

	default:
		return 0, fmt.Errorf("invalid userId type")
	}
}

func GetRoleFromContext(c *gin.Context) string {
	return c.GetString("role")
}
