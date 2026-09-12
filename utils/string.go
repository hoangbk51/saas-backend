package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go-saas/models"
	"regexp"
	"strings"
)

func GenerateRandomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func HashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

// CleanDomain loại bỏ http://, https:// và trailing slash từ domain
func CleanDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")
	return domain
}
func GenerateSlug(title string) string {
	slug := strings.ToLower(title)
	// Loại bỏ dấu tiếng Việt nếu có
	// (Bạn có thể dùng thư viện go-slug hoặc viết regex đơn giản ở đây)
	reg, _ := regexp.Compile("[^a-z0-9]+")
	slug = reg.ReplaceAllString(slug, "-")
	return strings.Trim(slug, "-")
}

func GetPrimaryName(nameMap map[string]string, namePrimary string) string {
	if namePrimary != "" {
		return namePrimary
	}
	if val, ok := nameMap["vi"]; ok && val != "" {
		return val
	}
	if val, ok := nameMap["en"]; ok && val != "" {
		return val
	}
	for _, val := range nameMap {
		if val != "" {
			return val
		}
	}
	return ""
}

func ParseFlexibleField(input string) interface{} {
	if input == "" {
		return ""
	}

	var parsed map[string]interface{}
	// Kiểm tra xem có phải chuỗi JSON object (vd: {"en":"Shirt","vi":"Áo"}) không
	if err := json.Unmarshal([]byte(input), &parsed); err == nil {
		return parsed
	}

	// Nếu không phải JSON Object, giữ nguyên chuỗi thuần
	return input
}

// ParseOptions nhận chuỗi JSON option từ DB và chuyển thành slice các OptionItem đã parse
func ParseOptions(optionStr string) []models.OptionItem {
	if optionStr == "" {
		return []models.OptionItem{}
	}

	// Unmarshal chuỗi JSON mảng từ DB
	var rawItems []map[string]string
	if err := json.Unmarshal([]byte(optionStr), &rawItems); err != nil {
		return []models.OptionItem{}
	}

	result := make([]models.OptionItem, 0, len(rawItems))
	for _, item := range rawItems {
		result = append(result, models.OptionItem{
			OptionName:  ParseFlexibleField(item["option_name"]),
			OptionValue: ParseFlexibleField(item["option_value"]),
		})
	}

	return result
}

func StringPtr(s string) *string {
	return &s
}
