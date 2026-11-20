# ClipSync

局域网内 macOS、Windows 与 Linux 之间的剪贴板同步工具，支持文本和图片。

UI 基于 Wails v3 + Vue 3 + Vite + Tailwind，后端为纯 Go。

## 功能

- 文本、图片（PNG）双向自动同步
- 局域网自动发现与配对（UDP 广播）：DHCP/Wi-Fi 切换 IP 后仍能继续同步
- 原生窗口界面：设备状态、已发现/已配对设备、剪贴板历史（含图片缩略图）
- 界面主题：浅色 / 深色 / 跟随系统，切换即时生效
- 开机自启：登录系统后自动在后台启动（macOS LaunchAgent / Windows 注册表 / Linux XDG autostart）
- 系统托盘：关闭窗口后继续在托盘同步，可随时打开主窗口或退出
- 设备身份 + 每对独享密钥认证，防止局域网内未授权设备连接
- 手动配对：在扫描列表点击「配对」，对方确认后建立连接
- 断线自动重连（每 5 秒重试），地址变化自动刷新
- 心跳保活（60 秒），内容指纹去重避免回环
- 单个二进制文件，无运行时依赖（macOS / Windows）；Linux 提供 AppImage（依赖系统 WebKitGTK 4.1）

## 使用

### 1. 启动

直接运行，首次会在 `~/.clipsync/config.json` 生成默认配置（含随机令牌），并弹出主窗口：

```bash
# macOS
open build/bin/ClipSync.app
# 或者直接执行二进制
./build/bin/ClipSync.app/Contents/MacOS/ClipSync

# Windows
build\bin\ClipSync.exe

# Linux (AppImage；按 CPU 架构选 amd64 / arm64，也可直接运行裸二进制)
chmod +x ClipSync-*-linux-*.AppImage
./ClipSync-*-linux-*.AppImage
```

### 2. 配对（自动发现 + 手动确认）

无需在两台设备上配置任何相同的东西。两台设备启动后，窗口「设备 → 发现的设备」会
自动列出局域网里的其他 ClipSync 设备（每台有一个持久化的 `device_id` 身份）。
点右侧「配对」发起请求；对方窗口会弹出「配对请求」提示，点「同意」即完成配对并开始
同步，点「拒绝」则取消。

- 「🔄 立即扫描」按钮：手动触发一次广播，立刻刷新扫描结果。
- 「高级 → 手动配对 IP/端口」：兜底入口，给跨子网等特殊场景使用。
- 配对时双端自动生成一对独享密钥；之后换 IP（DHCP/Wi-Fi）会自动刷新地址并恢复连接，
  无需重新确认。

也可以用命令行直接向某个地址发起配对（对方需在界面点「同意」）：

```bash
./build/bin/ClipSync.app/Contents/MacOS/ClipSync -add-peer 192.168.1.20:9250
```

### 3. 同步

两台设备都启动后，在一台机器上 `Cmd+C`（或 `Ctrl+C`），另一台直接粘贴即可。
在「剪贴板历史」中点击「复制」可把任意历史条目写回本机剪贴板并同步到对端。

## 命令行参数

| 参数 | 说明 |
|---|---|
| `-config <路径>` | 指定配置文件路径（默认 `~/.clipsync/config.json`） |
| `-add-peer <host:port>` | 向指定地址发起配对请求并退出（对方确认后写入配置） |
| `-peers` | 打印当前对端列表并退出 |
| `-headless` | 无界面模式：不启动窗口和托盘，用于服务器/调试 |

配置文件示例：

```json
{
  "name":      "my-mac",
  "port":      9250,
  "device_id": "自动生成的设备唯一身份",
  "peers":     [
    { "id": "对手 device_id", "name": "office-pc", "addr": "192.168.1.21:9250", "secret": "配对生成的密钥" }
  ]
}
```

## 注意事项

- Windows 剪贴板图片格式为位图，同步时会转为 PNG 传输；接收端以 PNG 写入
- 自动发现走 UDP 广播（端口 9251），要求两端在同一子网/广播域；跨子网请用手动配对
- 传输未加密（每对设备的密钥仅用于认证）。家庭局域网可放心使用；不可信网络建议自行套 VPN/WireGuard
- 系统字体：macOS 用 PingFang SC，Windows 用 Microsoft YaHei UI，Linux 用 WebKitGTK 默认字体，均带中文回退
- Linux 剪贴板优先走 X11（Wayland 会话下经 XWayland 亦可用）；纯 Wayland 会话（未启用 XWayland）自动切换到 wl-clipboard（`wl-copy` / `wl-paste`，AppImage 已内置，裸二进制需自行安装 `wl-clipboard` 包）；原生 GNOME 桌面如需托盘图标，需启用 AppIndicator 扩展

## 构建

```bash
# 安装 Wails3 CLI（如未装过）
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0

# 一次性构建当前平台（依赖链自动完成：绑定生成 → 前端构建 → 图标 → go build）
wails3 task build

# 开发模式（Vite 热重载 + Go 热编译）
wails3 dev

# 打包 macOS 通用 .app（arm64 + amd64）
wails3 task darwin:package:universal

# 交叉编译 Windows（v3 Windows 后端为纯 syscall，无需 mingw-w64）
wails3 task windows:build:all      # 一次产出 amd64 + arm64
wails3 task windows:build ARCH=arm64   # 或单架构

# Linux（需要 GTK3 / WebKitGTK 4.1 开发库，见下；ARCH 可选 amd64 / arm64）
wails3 task linux:build ARCH=amd64 EXTRA_TAGS=gtk3
wails3 task linux:package          # 把 build/bin/ClipSync 打成 AppImage
```

> Windows 下所有日志会写入 `%USERPROFILE%\.clipsync\app.log`。
> macOS 上构建 Linux 版走 Docker：`sh build/linux/build_in_docker.sh [amd64|arm64]`。

Linux 构建依赖（Debian/Ubuntu 为例）：

```bash
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev
```

- `EXTRA_TAGS=gtk3` 选择 GTK3 / WebKitGTK 4.1 后端（v3 默认后端是 GTK4 / WebKitGTK-6.0）
- `build_appimage.sh` 依赖 PATH 中的 appimagetool（经 `--appimage-extract-and-run` 调用，无需 FUSE）
- AppImage 依赖目标系统安装 WebKitGTK 4.1（主流发行版的桌面环境均自带）；剪贴板优先走 X11（XWayland 下可用），纯 Wayland 会话自动切换到 wl-clipboard——打包机装有 `wl-clipboard` 时会内置进 AppImage，否则需用户自行安装

## 项目结构

| 文件 / 目录 | 说明 |
|---|---|
| `main.go` | Wails v3 入口；命令行参数 + `application.New` + 窗口 + 托盘 + 生命周期 |
| `app.go` | 绑定服务（暴露给前端的方法） |
| `appearance_darwin.go` / `appearance_darwin.m` | macOS 深浅色探测与运行时切换外观（cgo） |
| `internal/engine/` | 后端：剪贴板监听、广播、接收、历史、发现、配置 |
| `internal/engine/engine.go` | 同步引擎（hub / dialer / clipboard / discovery） |
| `internal/engine/server.go` | TCP 服务端 |
| `internal/engine/client.go` | 对端拨号与自动重连 |
| `internal/engine/protocol.go` | 消息帧、握手协议 |
| `internal/engine/clipboard.go` | 系统剪贴板读写 |
| `internal/engine/discovery.go` | UDP 局域网发现 |
| `internal/engine/config.go` | 配置加载与保存 |
| `autostart_*.go` | 各平台开机自启实现（保留自定义版：Linux 会记录 `$APPIMAGE` 路径） |
| `Taskfile.yml` + `build/Taskfile.yml` + `build/<平台>/Taskfile.yml` | wails3 任务体系（build / package / run / dev） |
| `build/config.yml` | 项目元数据与版本号（Info.plist / Windows syso / AppImage 共用） |
| `build/linux/` | AppImage 打包资源（desktop 条目、AppRun、打包脚本、Docker 构建） |
| `icon.go` | 嵌入图标字节 |
| `frontend/` | Vue 3 + Vite + Tailwind 前端项目 |
| `frontend/bindings/` | `wails3 generate bindings` 生成的 TS 绑定（勿手改） |
| `frontend/src/components/` | TitleBar / Sidebar / HistoryView / DevicesView / SettingsView / AboutView 等 |
| `frontend/src/composables/` | useAppState / useTheme / useToast |
| `build/bin/` | 构建产物目录 |