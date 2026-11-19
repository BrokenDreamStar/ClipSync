package main

import (
	_ "embed"
)

//go:embed icon.png
var iconBytes []byte

//go:embed tray.png
var trayBytes []byte

// trayDarwinBytes 是 macOS 菜单栏用的模板图标（SF Symbol doc.on.clipboard 导出）：
// 24pt 画布、18pt 内容居中、黑色+透明。Wails v3 会把整图缩放到菜单栏
// thickness（≈24pt），内容因此保持 v2 的 18pt 视觉尺寸；
// 直接用彩色 tray.png 会按 24pt 满幅渲染（比 v2 大一圈）。
//
//go:embed tray-darwin.png
var trayDarwinBytes []byte
