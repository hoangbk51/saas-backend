package services

import (
	"fmt"
	"go-saas/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

func executeInsert(c *gin.Context, tableName string, data map[string]interface{}, lang string) (int64, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	keys := make([]string, 0)
	values := make([]interface{}, 0)
	placeholders := make([]string, 0)

	for k, v := range data {
		if (k == "title" && (tableName == "blogs" || tableName == "products")) || (k == "content" && tableName == "blogs") {
			jsonVal := fmt.Sprintf("{\"%s\": \"%v\"}", lang, v)
			keys = append(keys, fmt.Sprintf("`%s`", k))
			values = append(values, jsonVal)
		} else {
			keys = append(keys, fmt.Sprintf("`%s`", k))
			values = append(values, v)
		}
		placeholders = append(placeholders, "?")
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableName, strings.Join(keys, ","), strings.Join(placeholders, ","))
	result, err := tenantDB.Exec(query, values...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}
