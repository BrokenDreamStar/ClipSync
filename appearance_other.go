//go:build !darwin

package main

// systemPrefersDark 非 macOS 暂不检测系统外观，按深色处理。
// （Windows 侧启动时的原生窗口主题见 main.go 的 windowsTheme。）
func systemPrefersDark() bool { return false }

// applyAppearance 非 macOS 无运行时原生外观切换；窗口主题在下次启动时生效。
func applyAppearance(theme string) { _ = theme }
