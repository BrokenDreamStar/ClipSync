//go:build !windows

package main

// getConsoleWindow 非 Windows 实现：始终返回非零（视作有控制台）。
// 因为我们在 main 里只在 windows 分支里检查它，所以这里返回什么无所谓。
func getConsoleWindow() uintptr { return 1 }