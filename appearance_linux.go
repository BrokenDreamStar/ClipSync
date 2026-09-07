//go:build linux

package main

import (
	"os/exec"
	"strings"
)

// systemPrefersDark 在 Linux 上尽力探测系统深浅色：
// GNOME 及多数桌面（KDE 经 gtkconfig 同步）遵循 gsettings 的
// org.gnome.desktop.interface color-scheme；老桌面回退看 gtk-theme 名。
// 探测失败（无 gsettings/非桌面会话）按深色处理，与前端 matchMedia
// 不可用时的深色兜底保持一致，避免窗口背景色和界面主题不一致。
func systemPrefersDark() bool {
	if out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "color-scheme").Output(); err == nil {
		return strings.Contains(string(out), "dark")
	}
	if out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "gtk-theme").Output(); err == nil {
		return strings.Contains(strings.ToLower(string(out)), "-dark")
	}
	return true
}

// applyAppearance Linux 无运行时原生外观切换；主题在下次启动时生效。
func applyAppearance(theme string) { _ = theme }
