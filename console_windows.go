//go:build windows

package main

import "golang.org/x/sys/windows"

// getConsoleWindow 调用 Win32 GetConsoleWindow，0 表示没有控制台（GUI 子系统）。
func getConsoleWindow() uintptr {
	h, _, _ := windows.NewLazySystemDLL("kernel32").NewProc("GetConsoleWindow").Call()
	return h
}