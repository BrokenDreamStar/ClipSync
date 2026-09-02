//go:build !windows

package engine

import "syscall"

// setSockoptReuseAddrImpl 在 Unix 平台（macOS/Linux）把 SO_REUSEADDR 设为 1。
func setSockoptReuseAddrImpl(fd uintptr) error {
	return syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}