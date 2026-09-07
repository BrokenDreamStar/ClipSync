//go:build linux

package main

import (
	"runtime"

	"fyne.io/systray"
)

// SetAccessoryActivationPolicy 在 Linux 上是 no-op：该策略仅 macOS 需要，
// 用于把 NSApp 切成 accessory、隐藏 Dock 图标。
func SetAccessoryActivationPolicy() {}

// StartTray 在 Linux 上安装系统托盘图标与菜单。
//
// 用 fyne.io/systray 的纯 Go DBus 后端（StatusNotifierItem / AppIndicator），
// 不经 GTK：Wails 已在主线程跑 gtk_main，GTK 不是线程安全的，任何第二线程
// 的 GTK/第二 gtk_main 都可能与之冲突。代价是桌面环境需支持
// StatusNotifierItem（Ubuntu、KDE 默认支持，原生 GNOME 需装 AppIndicator 扩展）。
//
// onOpen / onQuit 由菜单点击回调触发（独立 goroutine）。
func StartTray(onOpen, onQuit func()) {
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		systray.Run(func() {
			systray.SetIcon(trayBytes)
			systray.SetTooltip("ClipSync — 局域网剪贴板同步")

			openItem := systray.AddMenuItem("打开主窗口", "显示主窗口")
			systray.AddSeparator()
			quitItem := systray.AddMenuItem("退出 ClipSync", "退出应用")

			go func() {
				for {
					select {
					case <-openItem.ClickedCh:
						if onOpen != nil {
							onOpen()
						}
					case <-quitItem.ClickedCh:
						// 先撤掉托盘，再走 app.Quit 让 Wails 主循环真正退出。
						systray.Quit()
						if onQuit != nil {
							onQuit()
						}
						return
					}
				}
			}()
		}, nil)
	}()
}
