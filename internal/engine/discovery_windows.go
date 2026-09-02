//go:build windows

package engine

import "syscall"

// setSockoptReuseAddrImpl 在 Windows 上把 SO_REUSEADDR 设为 1。
func setSockoptReuseAddrImpl(fd uintptr) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}