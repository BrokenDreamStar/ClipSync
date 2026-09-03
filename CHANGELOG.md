# 更新日志

本文件记录 ClipSync 的版本变更。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [0.0.3] - 2026-09-04

### 变更

- **macOS 托盘**：将应用激活策略切换为 `accessory`，应用不再出现在 Dock 与 Cmd+Tab 切换器里，仅保留菜单栏托盘入口。
- **关于页面**：移除「特性」板块。
- **设置页面**：移除「配对方式」说明板块。
- **版本号**：由 0.0.1 提升至 0.0.3（`wails.json` / 关于页 `const version` 同步更新）。
- **功能合入**：将 `main` 分支 v0.0.2 的「主题切换」与「开机自启」合并进本分支，当前版本包含该两项功能。

### 修复

- **Windows 交叉编译**：为 Windows / 其他平台补充 `SetAccessoryActivationPolicy` 空实现，解决 `main.go` 在 `windows` 目标下因找不到该符号而导致的编译失败（该函数此前仅在 `darwin` 文件中定义）。
- **Windows 资源版本**：将 `build/windows/winres.json` 中陈旧的 `0.2.0.0` 修正为 `0.0.3.0`，并移除含有过期版本号的 `rsrc_windows_amd64.syso`（该已编译产物不参与当前构建，Wails 会在构建时自行生成资源）。

## [0.0.2] - 2026-09-03

### 新增

- **界面主题切换**：设置页新增「外观」，支持浅色 / 深色 / 跟随系统三种模式，切换即时生效并持久化；「跟随系统」随系统深浅色自动切换。
- **开机自启**：设置页新增「通用 → 开机自启」开关。macOS 通过 LaunchAgent、Windows 通过注册表 Run 键实现，开关状态即系统真实状态。
- 全套浅色主题配色：界面颜色全面变量化，浅色模式下的侧栏、卡片、输入框、弹窗、Toast 等均已适配。
- 启动时按主题偏好设置原生窗口外观与背景色（macOS NSAppearance / Windows 标题栏主题 / WebView 背景色），避免启动与拉伸时闪深色。

### 变更

- 配置文件新增 `theme` 字段（`dark` / `light` / `system`），旧配置文件自动兼容并补默认值 `dark`。

## [0.0.1] - 初始版本

- 文本、图片（PNG）双向自动同步
- 局域网自动发现与配对（UDP 广播），DHCP/Wi-Fi 切换 IP 后自动恢复连接
- 剪贴板历史（含图片缩略图），一键写回本机剪贴板并同步
- 设备身份 + 每对独享密钥认证
- 系统托盘（macOS NSStatusItem / Windows systray），关闭窗口后驻留后台
- 手动配对 IP/端口兜底入口

[Unreleased]: https://github.com/BrokenDreamStar/ClipSync/compare/v0.0.3...HEAD
[0.0.3]: https://github.com/BrokenDreamStar/ClipSync/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/BrokenDreamStar/ClipSync/compare/v0.0.1...v0.0.2
