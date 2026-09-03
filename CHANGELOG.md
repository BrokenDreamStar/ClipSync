# Changelog

本项目的版本号遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)，变更记录遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。

## [Unreleased]

## [0.0.3] - 2026-09-04

### 变更

- **macOS 托盘**：将应用激活策略切换为 `accessory`，应用不再出现在 Dock 与 Cmd+Tab 切换器里，仅保留菜单栏托盘入口。
- **关于页面**：移除「特性」板块。
- **设置页面**：移除「配对方式」说明板块。
- **版本号**：由 0.0.1 提升至 0.0.3（`wails.json` / 关于页 `const version` 同步更新）。

### 修复

- **Windows 交叉编译**：为 Windows / 其他平台补充 `SetAccessoryActivationPolicy` 空实现，解决 `main.go` 在 `windows` 目标下因找不到该符号而导致的编译失败（该函数此前仅在 `darwin` 文件中定义）。
- **Windows 资源版本**：将 `build/windows/winres.json` 中陈旧的 `0.2.0.0` 修正为 `0.0.3.0`，并移除含有过期版本号的 `rsrc_windows_amd64.syso`（该已编译产物不参与当前构建，Wails 会在构建时自行生成资源）。

### 重建产物

- macOS 通用版（Apple Silicon + Intel）：`clipsync-0.0.3-macos-universal.dmg`
- Windows：`clipsync-0.0.3-windows-amd64.exe`

## [0.0.2] - 2026-09-03

- 主题切换与开机自启（见 `main` 分支提交 `f51eb36`，未并入 `dev`）。
