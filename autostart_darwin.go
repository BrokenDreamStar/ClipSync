//go:build darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// autostartLabel 是 LaunchAgent 的唯一标识，同时决定 plist 文件名。
const autostartLabel = "com.clipsync.app"

func autostartPlistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "LaunchAgents", autostartLabel+".plist")
}

// autostartEnabled 以 LaunchAgent plist 是否存在为准（OS 即唯一事实来源）。
func autostartEnabled() bool {
	p := autostartPlistPath()
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

// setAutostart 写入/删除 ~/Library/LaunchAgents 下的 LaunchAgent，登录时由 launchd 拉起。
// 只落盘、不执行 launchctl load，避免立刻再拉起一个重复实例；下次登录即生效。
func setAutostart(enable bool) error {
	p := autostartPlistPath()
	if p == "" {
		return fmt.Errorf("无法确定用户主目录")
	}
	if !enable {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.Abs(exe); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + autostartLabel + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + exe + `</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`
	return os.WriteFile(p, []byte(plist), 0o644)
}
