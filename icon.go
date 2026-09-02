package main

import (
	_ "embed"
	"runtime"
)

//go:embed icon.png
var iconBytes []byte

//go:embed tray.png
var trayBytes []byte

// trayIcon 是 systray 用的 PNG 字节；用 1024×1024 应用图标。
// systray 在 macOS 会自动按 NSStatusItem 缩放。
var trayIcon = trayBytes

var (
	buildOS   = runtime.GOOS
	buildArch = runtime.GOARCH
)