//go:build windows

package services

func platformSetUmask(mask int) int {
	// Trên Windows không có khái niệm umask giống Linux, chỉ cần trả về 0
	return 0
}
