package utils

import (
	"database/sql"
	"fmt"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func GetDBFromContext(c *gin.Context) (*sqlx.DB, error) {
	v, ok := c.Get("db")
	if !ok {
		return nil, fmt.Errorf("database connection not found in context")
	}
	db, ok := v.(*sql.DB)
	if !ok {
		return nil, fmt.Errorf("database connection in context is not of type *sql.DB")
	}
	db1 := sqlx.NewDb(db, "mysql")
	return db1, nil
}

var (
	centralDB     *sql.DB
	centralDBOnce sync.Once
)

// GetCentralDB trả về kết nối tới database trung tâm của hệ thống SaaS
func GetCentralDB() (*sql.DB, error) {
	var err error

	// Sử dụng sync.Once để đảm bảo chỉ tạo kết nối 1 lần duy nhất trong suốt vòng đời app
	centralDBOnce.Do(func() {
		dbUser := os.Getenv("DB_USER")
		dbPass := os.Getenv("DB_PASS")
		dbHost := os.Getenv("DB_HOST")
		dbPort := os.Getenv("DB_PORT")
		dbName := os.Getenv("DB_NAME") // Tên DB Central của bạn ở đây

		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			dbUser, dbPass, dbHost, dbPort, dbName,
		)

		centralDB, err = sql.Open("mysql", dsn)
		if err != nil {
			return
		}

		// Cấu hình Pool cho Central DB vì tần suất gọi sẽ rất lớn
		centralDB.SetMaxOpenConns(50)
		centralDB.SetMaxIdleConns(10)
	})

	if err != nil {
		return nil, err
	}
	return centralDB, nil
}

func GetInt64FromContext(c *gin.Context, key string) int64 {
	value, exists := c.Get(key)

	if !exists {
		return 0
	}

	switch v := value.(type) {
	case int:
		return int64(v)

	case int8:
		return int64(v)

	case int16:
		return int64(v)

	case int32:
		return int64(v)

	case int64:
		return v

	case uint:
		return int64(v)

	case uint8:
		return int64(v)

	case uint16:
		return int64(v)

	case uint32:
		return int64(v)

	case uint64:
		return int64(v)

	default:
		return 0
	}
}
