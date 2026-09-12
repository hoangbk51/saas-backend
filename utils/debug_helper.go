package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

func LogSQL(query string, args ...interface{}) {
	pwd, _ := os.Getwd()
	path := filepath.Join(pwd, "sql_debug.log")

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		return
	}
	defer f.Close()

	output := query

	// Xử lý trường hợp truyền vào map cho Named Query (ví dụ :id, :type)
	if len(args) == 1 && args[0] != nil && reflect.TypeOf(args[0]).Kind() == reflect.Map {
		if namedArgs, ok := args[0].(map[string]interface{}); ok {
			for key, val := range namedArgs {
				var formattedVal string
				switch v := val.(type) {
				case string:
					formattedVal = fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
				case nil:
					formattedVal = "NULL"
				default:
					formattedVal = fmt.Sprintf("%v", v)
				}
				// Thay thế :key trong query
				output = strings.ReplaceAll(output, ":"+key, formattedVal)
			}
		}
	} else {
		// Xử lý trường hợp truyền danh sách args cho dấu ? truyền thống
		for _, arg := range args {
			var formattedVal string
			switch v := arg.(type) {
			case string:
				formattedVal = fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
			case nil:
				formattedVal = "NULL"
			default:
				formattedVal = fmt.Sprintf("%v", v)
			}
			output = strings.Replace(output, "?", formattedVal, 1)
		}
	}

	entry := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), output)

	f.WriteString(entry)
	fmt.Print("SQL_LOG >> ", entry)
}
