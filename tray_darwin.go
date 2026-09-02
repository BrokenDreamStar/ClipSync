//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -I${SRCDIR}
#cgo LDFLAGS: -framework Cocoa -framework Foundation
#include "tray_darwin.h"

// cgo 把 ObjC 实现放在独立 .m 文件中，避免在 Go 翻译单元中重复实例化 ObjC class。
*/
import "C"
import "unsafe"

// StartTray 安装 macOS 系统托盘图标与菜单。
//
// 所有 NSStatusBar / NSWindow 相关调用必须发生在 Cocoa 主线程上；C 端通过
// dispatch_async(dispatch_get_main_queue(), ...) 完成派发,Go 侧无需关心线程,
// 但调用时机必须在 NSApp 已初始化之后（典型: Wails OnStartup 内）。
//
// onOpen / onQuit 由 Cocoa 主线程同步调用。
func StartTray(onOpen, onQuit func()) {
	csGoOpen = onOpen
	csGoQuit = onQuit
	C.csSetupTray(
		unsafe.Pointer(&trayIcon[0]), C.long(len(trayIcon)),
		C.CString("ClipSync — 局域网剪贴板同步"),
	)
}

//export csGoOpenTrampoline
func csGoOpenTrampoline() {
	if csGoOpen != nil {
		csGoOpen()
	}
}

//export csGoQuitTrampoline
func csGoQuitTrampoline() {
	if csGoQuit != nil {
		csGoQuit()
	}
}

var (
	csGoOpen func()
	csGoQuit func()
)