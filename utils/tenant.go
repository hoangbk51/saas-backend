package utils

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

type TenantPool struct {
	DB         *sql.DB
	LastActive time.Time
}

const RedisTenantIDKey = "tenant_id:"

var (
	TenantRegistry = make(map[string]*TenantPool)
	PoolMu         sync.RWMutex
	CentralDB      *sql.DB
)

// InitCentralDB khởi tạo kết nối đến Central MySQL Database
func InitCentralDB() {
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPass, dbHost, dbPort, dbName,
	)

	var err error
	CentralDB, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Error opening central database: %v", err)
	}

	if err = CentralDB.Ping(); err != nil {
		log.Fatalf("Error pinging central database: %v (DSN: %s)", err, dsn)
	}

	go CleanupIdlePools()
}

// CleanupIdlePools đóng các kết nối DB tenant không hoạt động sau 30 phút
func CleanupIdlePools() {
	for {
		time.Sleep(10 * time.Minute)
		PoolMu.Lock()
		for id, p := range TenantRegistry {
			if time.Since(p.LastActive) > 30*time.Minute {
				p.DB.Close()
				delete(TenantRegistry, id)
				LogToFile("Cleaned up idle database pool for tenant: %s\n", id)
			}
		}
		PoolMu.Unlock()
	}
}

// GetDomainFromHost tách tên miền từ host request
func GetDomainFromHost(host string) string {
	parts := strings.Split(host, ":")
	return parts[0]
}

// GetTenantIDFromDomain truy vấn tenant_id từ Redis cache hoặc Central DB
func GetTenantIDFromDomain(domain string) (string, error) {
	ctx := context.Background()

	tenantID, err := RedisClient.Get(ctx, RedisTenantIDKey+domain).Result()
	if err == redis.Nil {
		LogToFile("TenantID for domain %s not found in Redis, querying main DB", domain)

		var tempTenantID string
		err = CentralDB.QueryRow("SELECT tenant_id FROM domains WHERE domain = ?", domain).Scan(&tempTenantID)
		if err != nil {
			return "", fmt.Errorf("Tenant not found for domain %s: %v", domain, err)
		}
		tenantID = tempTenantID

		err = RedisClient.Set(ctx, RedisTenantIDKey+domain, tenantID, 15*time.Minute).Err()
		if err != nil {
			LogToFile("Error setting tenantID in Redis: %v", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("Error getting tenantID from Redis: %v", err)
	}

	//LogToFile("tenantID for %s = %s", domain, tenantID)
	return tenantID, nil
}

// GetTenantDBByTenantID lấy kết nối *sql.DB của tenant từ Registry hoặc khởi tạo mới
func GetTenantDBByTenantID(tenantID string) (*sql.DB, error) {
	PoolMu.RLock()
	if p, ok := TenantRegistry[tenantID]; ok {
		p.LastActive = time.Now()
		PoolMu.RUnlock()
		return p.DB, nil
	}
	PoolMu.RUnlock()

	PoolMu.Lock()
	defer PoolMu.Unlock()

	if p, ok := TenantRegistry[tenantID]; ok {
		p.LastActive = time.Now()
		return p.DB, nil
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPass, dbHost, dbPort, tenantID,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	p := &TenantPool{DB: db, LastActive: time.Now()}
	TenantRegistry[tenantID] = p
	return db, nil
}

// TenantMiddleware Middleware gán tenantID và *sql.DB vào Gin Context
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		//LogToFile("TenantMiddleware = ")

		domain := GetDomainFromHost(c.Request.Host)
		//LogToFile("domain = " + domain)
		if domain == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Domain không xác định"})
			return
		}

		tenantID, err := GetTenantIDFromDomain(domain)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy tenant cho domain này"})
			return
		}

		c.Set("tenantId", tenantID)

		db, err := GetTenantDBByTenantID(tenantID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Set("db", db)
		c.Next()
	}
}
