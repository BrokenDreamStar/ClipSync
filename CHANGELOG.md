# 更新日志

本文件记录 ClipSync 的版本变更。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [0.1.1] - 2026-11-19

### 修复

- **macOS 菜单栏图标过大**：Wails v3 会把托盘图片缩放到菜单栏厚度（≈24pt），此前直接复用 256px 的彩色 `tray.png`，图标比 v2 时期大一圈且形状变成应用图标剪影。现改用专用模板图标 `tray-darwin.png`（SF Symbol `doc.on.clipboard` 导出，24pt 画布 / 18pt 内容居中），视觉尺寸与形状均与 v2 一致，并继续自动跟随系统明暗反色。

### 变更

- **更新检查的网络报错不再暴露细节**：连不上 GitHub 时界面统一显示「无法连接 GitHub，请检查网络后重试」，拨号超时、代理拒绝等底层原因只写日志（Windows 在 `%USERPROFILE%\.clipsync\app.log`），不再把内网地址、代理配置等信息直接展示给用户。

## [0.1.0] - 2026-11-19

> 同日重发：首发 macOS DMG 因签名封印被破坏、Gatekeeper 判「已损坏」无法启动，修复后已重新发布；Windows EXE 与 Linux AppImage 首发产物不受影响。重发同时把 Release 矩阵从三平台扩展到五平台（Windows ARM64 + Linux aarch64）。

### 修复

- **macOS 下载版无法启动（首发 DMG 回归）**：CI 在 ad-hoc 签名**之后**用 `build_icns.sh` 覆盖 `.app` 内的 `icons.icns`，破坏了代码签名封印（`codesign --verify` 报 sealed resource is missing or invalid）。从浏览器下载的 DMG 带 Gatekeeper 隔离属性，无效签名直接被判「已损坏，无法打开」。修复：`build_icns.sh` 覆盖 icns 后重新 ad-hoc 签名，CI 在打包 DMG 前增加 `codesign --verify` 门禁拦截同类回归。注意：未做 Developer ID 签名与公证前，下载版首次打开仍需在「系统设置 → 隐私与安全性」中点击「仍要打开」。

### 新增

- **应用内更新（GitHub Releases）**：「关于」页新增软件更新卡片——检查 GitHub 最新版本、一键下载安装并自动重启。启动时也会静默检查一次，发现新版本时通过 toast 提示并引导到「关于」页。
  - 更新源是 Release 附带的 `latest.json` 清单（CI 发布时自动生成，含各平台产物的文件名 / 大小 / sha256），更新器走 `releases/latest/download/latest.json` 稳定地址，不占 GitHub API 配额、无需 token；下载后校验 sha256 与平台文件头再安装。
  - 三平台安装策略：Windows 便携 EXE 与 Linux AppImage 走「旧文件改名 `.old`、新文件就位」的换名替换（`.old` 保留为新版本起不来时的回滚点，新版启动成功后自动清理）；macOS 下载 DMG 后挂载、`ditto` 整包覆盖 `.app`，退出后由 helper 以 `open` 重新拉起。
  - 重启顺序保证端口不冲突：安装完成后由 detached helper 等旧进程退出（引擎停完、TCP/UDP 端口已释放）再启动新版本。
  - 版本号改为运行时从内嵌的 `build/config.yml` 读取（与 Info.plist / Windows syso 同源），「关于」页版本号不再硬编码。
  - 网络受限环境可设 `CLIPSYNC_UPDATE_MIRROR` 环境变量走 ghproxy 类镜像（前缀拼接原始下载地址）。
  - 注意：0.0.4 及更早的发布没有 `latest.json`，旧版本内的更新检查会提示到发布页手动下载；从首个带清单的版本起，后续版本均可应用内自更新。
- **Windows ARM64 与 Linux ARM64 发布**（重发扩展）：Release 矩阵从三平台扩展到五平台——Windows EXE 增加 arm64（与 amd64 同一份 CI 作业里纯 syscall 交叉编译，本地 `wails3 task windows:build:all` 一次产出双架构），Linux AppImage 增加 aarch64（GitHub 公开仓库免费的 arm64 原生 runner 构建，无需 QEMU 模拟）。应用内更新器同步支持：Windows ARM64 设备现在能识别自己的平台并下载对应更新包；`latest.json` 清单收录全部五个平台条目。
  - 本地构建支持：`build/linux/build_in_docker.sh` 接受 `amd64|arm64` 参数，Apple Silicon 上 arm64 容器原生运行，本机即可产出 aarch64 AppImage（amd64 容器的 Rosetta 转译限制依旧，AppImage 阶段自动跳过，见 AGENTS.md）。

### 变更

- **框架升级到 Wails v3.0.0**：桌面框架从 Wails v2.15.0 迁移到 v3.0.0 稳定版，用户可见行为不变。
  - 构建体系切换到 `wails3` CLI + Taskfile（`wails3 task build` / `wails3 dev`）；项目元数据由 `wails.json` / `winres.json` 迁至 `build/config.yml`，版本号一处配置（Info.plist、Windows syso、AppImage 命名共用）。
  - 系统托盘改用 Wails v3 内置 SystemTray（Linux 后端同为纯 Go DBus StatusNotifierItem），删除 macOS cgo 托盘与 `getlantern/systray`、`fyne.io/systray` 依赖；macOS 菜单栏改用模板图标（自动跟随系统明暗）。macOS 的 accessory 激活策略改由 `MacOptions.ActivationPolicy` 官方配置实现。
  - 绑定生成改为 `wails3 generate bindings` 静态分析（不再以 `-tags bindings` 运行二进制提取），v2 时代的绑定模式桩整体移除，"应用运行时构建因 9250 端口被占用而失败"的坑从根上消除。
  - 前端运行时改用 npm 包 `@wailsio/runtime`；绑定导入自 `frontend/bindings/`，`(T, string)` 元组返回值在 v3 下前端按元组解包（第二项空串为成功，约定不变）。
  - Linux 构建标签 `webkit2_41` 改为 `gtk3`（v3 以 build tag 区分 GTK3/WebKitGTK-4.1 与 GTK4/WebKitGTK-6.0 后端）；Windows 构建为纯 syscall 交叉编译（CGO_ENABLED=0），不再需要 mingw-w64。
  - 窗口"关闭即隐藏"改经 `WindowClosing` 事件钩子实现（v3 无 `HideWindowOnClose` 选项），托盘"退出"置退出标志后放行关闭。

### 新增

- **纯 Wayland 支持**：Linux 剪贴板不再强依赖 X11——X11 不可用（未启用 XWayland 的纯 Wayland 会话）时自动切换到 `wl-clipboard`（`wl-copy` / `wl-paste`）实现剪贴板监听与写入，文本与图片均可同步。AppImage 会内置这两个工具（打包机需装有 `wl-clipboard`），裸二进制部署需自行安装 `wl-clipboard` 包。

## [0.0.4] - 2026-09-07

### 新增

- **Linux 支持**：适配 Linux 平台——系统托盘（纯 Go DBus / StatusNotifierItem，无 GTK 依赖）、XDG 开机自启（AppImage 环境自动记录 AppImage 路径）、通过 gsettings 探测深浅色的「跟随系统」主题。Release 产物新增 `ClipSync-*-linux-x86_64.AppImage`（依赖系统 WebKitGTK 4.1）。
- **无边框窗口**：Windows 与 Linux 不再显示系统原生标题栏（即 macOS 顶部那条 "ClipSync" 标题条），三端观感一致；窗口拖动通过左侧栏拖拽区完成。
- **启动时最小化到任务栏**：设置页「通用」新增开关。开启后应用启动时不再弹出主窗口，仅在托盘显示图标，可通过托盘菜单「打开主窗口」唤出。配置文件新增 `start_minimized` 字段（默认关闭，旧配置自动兼容）。

### 变更

- **构建**：Wails 绑定生成阶段（`-tags bindings`）不再启动同步引擎与剪贴板监听，修复"应用正在运行时执行 `wails build` / `wails dev` 因 9250 端口被占用而失败"的问题。
- **构建**：新增 Docker 化 Linux 构建环境（`build/linux/Dockerfile` + `build/linux/build_in_docker.sh`），macOS 上一条命令即可在与 CI 同构的 amd64 Ubuntu 24.04 容器里完成 `wails build` / `go vet` / `go test`。AppImage 阶段在 Docker Desktop on Apple Silicon 上自动跳过（emulation 不翻译 `appimagetool` 这类非 PIE ELF），由 CI/Linux runner 完成。
- **CI**：Release 流水线新增 `build-linux` 作业（ubuntu-latest + `webkit2_41` 构建标签 + appimagetool），与 macOS DMG、Windows EXE 一同发布。

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

[Unreleased]: https://github.com/BrokenDreamStar/ClipSync/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/BrokenDreamStar/ClipSync/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/BrokenDreamStar/ClipSync/compare/v0.0.4...v0.1.0
[0.0.4]: https://github.com/BrokenDreamStar/ClipSync/compare/v0.0.3...v0.0.4
[0.0.3]: https://github.com/BrokenDreamStar/ClipSync/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/BrokenDreamStar/ClipSync/compare/v0.0.1...v0.0.2
