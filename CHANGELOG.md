# 更新日志

本文件记录 ClipSync 的版本变更。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

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

[Unreleased]: https://github.com/BrokenDreamStar/ClipSync/compare/v0.0.2...HEAD
[0.0.2]: https://github.com/BrokenDreamStar/ClipSync/compare/v0.0.1...v0.0.2
