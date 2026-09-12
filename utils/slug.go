package utils

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func MakeSlug(s string) string {
	// 1. Chuyển về chữ thường
	s = strings.ToLower(s)

	// 2. Loại bỏ dấu tiếng Việt
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)

	// Xử lý riêng chữ đ/Đ vì transform trên không khử được
	result = strings.ReplaceAll(result, "đ", "d")

	// 3. Loại bỏ ký tự đặc biệt, chỉ giữ lại chữ cái, số và dấu gạch ngang
	reg, _ := regexp.Compile("[^a-z0-9]+")
	result = reg.ReplaceAllString(result, "-")

	// 4. Loại bỏ dấu gạch ngang dư thừa ở đầu/cuối
	result = strings.Trim(result, "-")

	return result
}
