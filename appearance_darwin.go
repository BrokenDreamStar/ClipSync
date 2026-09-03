//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -I${SRCDIR}
#cgo LDFLAGS: -framework Cocoa
#include "tray_darwin.h"
*/
import "C"

// systemPrefersDark 返回 macOS 系统当前是否为深色外观（"system" 主题解析用）。
func systemPrefersDark() bool {
	return C.csSystemPrefersDark() != 0
}

// applyAppearance 运行时切换 NSApp 外观；"system" 清除强制外观、重新跟随系统。
// 需在 NSApp 初始化之后调用（Wails OnStartup 之后的前端绑定调用均满足）。
func applyAppearance(theme string) {
	switch theme {
	case "light":
		C.csApplyAppearance(0)
	case "dark":
		C.csApplyAppearance(1)
	default:
		C.csApplyAppearance(-1)
	}
}
