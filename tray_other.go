//go:build !darwin && !windows

package main

// SetAccessoryActivationPolicy 在非 macOS 平台上是 no-op：该策略仅 macOS
// 需要，用于把 NSApp 切成 accessory、隐藏 Dock 图标。
func SetAccessoryActivationPolicy() {}

// StartTray 在非 macOS/Windows 平台上是 no-op：Wails v2 不内置托盘，
// 当前阶段优先保证 macOS/Windows 体验，其他平台回退到不显示托盘图标。
// 关闭按钮仍可隐藏窗口，进程仅通过 Quit 退出。
func StartTray(onOpen, onQuit func()) {
	_ = onOpen
	_ = onQuit
}