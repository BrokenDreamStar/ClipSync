//go:build !darwin && !linux

package main

// systemPrefersDark 其他平台暂不检测系统外观，按浅色处理。
// （Linux 侧见 appearance_linux.go；Windows 侧启动时的原生窗口主题见 main.go 的 windowsTheme。）
func systemPrefersDark() bool { return false }

// applyAppearance 无运行时原生外观切换；窗口主题在下次启动时生效。
func applyAppearance(theme string) { _ = theme }
