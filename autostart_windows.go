//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	// runKeyPath 是当前用户的开机启动项注册表键。
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	// autostartValueName 是本应用在 Run 键下的值名。
	autostartValueName = "ClipSync"
)

// autostartEnabled 以 Run 键下是否存在本应用条目为准（OS 即唯一事实来源）。
func autostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue(autostartValueName)
	return err == nil && v != ""
}

// setAutostart 写入/删除 HKCU Run 键。路径加引号以兼容含空格的安装目录。
func setAutostart(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !enable {
		if err := k.DeleteValue(autostartValueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := k.SetStringValue(autostartValueName, fmt.Sprintf(`"%s"`, exe)); err != nil {
		return err
	}
	return nil
}
