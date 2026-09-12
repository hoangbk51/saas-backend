package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

var logMutex sync.Mutex

// LogToFile ghi log tổng quát vào file logs/app.log (thay thế trực tiếp cho log.Printf)
func LogToFile(formatOrErr any, args ...any) {
	if formatOrErr == nil {
		return
	}

	logMutex.Lock()
	defer logMutex.Unlock()

	// 1. Lấy vị trí dòng code gọi LogToFile (file:line)
	_, fileCaller, line, ok := runtime.Caller(1)
	callerInfo := ""
	if ok {
		callerInfo = fmt.Sprintf("[%s:%d] ", filepath.Base(fileCaller), line)
	}

	// 2. Format nội dung log dựa trên kiểu dữ liệu truyền vào
	var message string
	switch v := formatOrErr.(type) {
	case error:
		message = fmt.Sprintf("%sERROR: %v", callerInfo, v)
	case string:
		if len(args) > 0 {
			message = fmt.Sprintf(v, args...)
		} else {
			message = v
		}
		// Nếu là string thông thường thì gắn callerInfo vào đầu
		message = callerInfo + message
	default:
		message = fmt.Sprintf("%s%+v", callerInfo, v)
	}

	// 3. Tạo thư mục logs nếu chưa có
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return
	}

	// 4. Mở file logs/app.log
	logPath := filepath.Join(logDir, "app.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return
	}
	defer file.Close()

	// 5. Ghi dòng log kèm Timestamp
	now := time.Now().Format("2006-01-02 15:04:05")
	logLine := fmt.Sprintf("[%s] %s\n", now, message)
	_, _ = file.WriteString(logLine)
}
