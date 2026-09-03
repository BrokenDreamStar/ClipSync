//go:build windows

package main

import (
	_ "embed"
	"runtime"

	"github.com/getlantern/systray"
)

//go:embed build/windows/icon.ico
var winTrayIcon []byte

// SetAccessoryActivationPolicy 在 Windows 上是 no-op：该策略仅 macOS 需要，
// 用于把 NSApp 切成 accessory、隐藏 Dock 图标。
func SetAccessoryActivationPolicy() {}

// StartTray 在 Windows 上安装系统托盘图标与菜单。
//
// Wails v2 的 Windows 前端把 OnStartup 放在独立 goroutine 里调用，
// 主线程则被 winc.RunMainLoop 占用。systray 需要在固定线程上创建
// 隐藏的托盘窗口并持续排空其消息，因此这里为托盘单独开一条线程
// 跑 systray.Run，与 Wails 主窗口互不干扰。
//
// onOpen / onQuit 由菜单点击回调触发（独立 goroutine）。
func StartTray(onOpen, onQuit func()) {
	// systray 要求在固定的 OS 线程上注册托盘窗口和跑消息循环：
	// Register 会创建隐藏的 SystrayClass 窗口，后续 SetIcon / 菜单更新
	// 都在同一条线程处理。锁住这条线程避免 Go 调度器把两段操作拆分到不同线程。
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		systray.Run(func() {
			systray.SetIcon(winTrayIcon)
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