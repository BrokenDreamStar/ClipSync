//go:build !darwin

package main

// setAppRegular 仅 macOS 有 Dock / Cmd+Tab 概念，其余平台无操作。
func setAppRegular(regular bool) { _ = regular }
