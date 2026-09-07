//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// autostartDesktopName 是写入 XDG autostart 目录的 .desktop 条目文件名。
const autostartDesktopName = "clipsync-autostart.desktop"

// autostartDir 返回用户的 XDG autostart 目录（$XDG_CONFIG_HOME/autostart，
// 未设置时为 ~/.config/autostart）。
func autostartDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "autostart")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "autostart")
}

// autostartEnabled 以 autostart 目录中是否存在条目为准（OS 即唯一事实来源）。
func autostartEnabled() bool {
	dir := autostartDir()
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, autostartDesktopName))
	return err == nil
}

// setAutostart 写入/删除 XDG autostart 条目，登录桌面后由桌面环境自动拉起。
// 应用路径优先取 $APPIMAGE（AppImage 运行时由其启动器注入），普通二进制用自身路径。
func setAutostart(enable bool) error {
	dir := autostartDir()
	if dir == "" {
		return fmt.Errorf("无法确定用户配置目录")
	}
	entry := filepath.Join(dir, autostartDesktopName)
	if !enable {
		if err := os.Remove(entry); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	target, err := autostartTarget()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	desktop := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=ClipSync
Comment=局域网剪贴板同步
Exec=%s
Icon=clipsync
Terminal=false
X-GNOME-Autostart-enabled=true
`, desktopQuote(target))
	return os.WriteFile(entry, []byte(desktop), 0o644)
}

// autostartTarget 返回开机自启应执行的应用路径。
func autostartTarget() (string, error) {
	if appimage := os.Getenv("APPIMAGE"); appimage != "" {
		return appimage, nil
	}
	return os.Executable()
}

// desktopQuote 按 desktop entry 规范给 Exec 参数加双引号并转义特殊字符，
// 兼容含空格的 AppImage/二进制路径。
func desktopQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `\$`, "`", "\\`").Replace(s) + `"`
}
