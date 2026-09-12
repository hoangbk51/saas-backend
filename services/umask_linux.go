//go:build !windows

package services

import "syscall"

func platformSetUmask(mask int) int {
	return syscall.Umask(mask)
}
