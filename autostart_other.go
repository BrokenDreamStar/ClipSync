//go:build !darwin && !windows

package main

import "fmt"

func autostartEnabled() bool { return false }

func setAutostart(enable bool) error {
	_ = enable
	return fmt.Errorf("当前平台暂不支持开机自启")
}
