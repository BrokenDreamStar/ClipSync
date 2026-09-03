# ClipSync

局域网内 macOS 与 Windows 之间的剪贴板同步工具，支持文本和图片。

UI 基于 Wails v2 + Vue 3 + Vite + Tailwind，后端为纯 Go。

## 功能

- 文本、图片（PNG）双向自动同步
- 局域网自动发现与配对（UDP 广播）：DHCP/Wi-Fi 切换 IP 后仍能继续同步
- 原生窗口界面：设备状态、已发现/已配对设备、剪贴板历史（含图片缩略图）
- 界面主题：浅色 / 深色 / 跟随系统，切换即时生效
- 开机自启：登录系统后自动在后台启动（macOS LaunchAgent / Windows 注册表）
- 系统托盘：关闭窗口后继续在托盘同步，可随时打开主窗口或退出
- 设备身份 + 每对独享密钥认证，防止局域网内未授权设备连接
- 手动配对：在扫描列表点击「配对」，对方确认后建立连接
- 断线自动重连（每 5 秒重试），地址变化自动刷新
- 心跳保活（60 秒），内容指纹去重避免回环
- 单个二进制文件，无运行时依赖（macOS / Windows）

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
- 系统字体：macOS 用 PingFang SC，Windows 用 Microsoft YaHei UI，WebView 自带中文回退

## 构建

```bash
# 安装 Wails CLI（如未装过）
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 安装前端依赖并构建前端
cd frontend && npm install && npm run build && cd ..

# 一次性构建当前平台
wails build

# 开发模式（Vite 热重载 + Go 热编译）
wails dev

# macOS 交叉编译 Windows（需要 mingw-w64）
# 注意：-compiler 指定的是 Go 编译器，不是 C 编译器；C 交叉编译器用 CC 环境变量。
# 不要写成 -compiler gcc，否则 wails 会用 gcc 执行 `go mod tidy` 而报错。
CC=x86_64-w64-mingw32-gcc wails build -platform windows/amd64
```

> Windows 下 `-H windowsgui` 不再需要（Wails 默认不开控制台）。所有日志会写入
> `%USERPROFILE%\.clipsync\app.log`。

## 项目结构

| 文件 / 目录 | 说明 |
|---|---|
| `main.go` | Wails 入口；命令行参数 + `wails.Run` |
| `app.go` | Wails 绑定层（暴露给前端的方法） |
| `internal/engine/` | 后端：剪贴板监听、广播、接收、历史、发现、配置 |
| `internal/engine/engine.go` | 同步引擎（hub / dialer / clipboard / discovery） |
| `internal/engine/server.go` | TCP 服务端 |
| `internal/engine/client.go` | 对端拨号与自动重连 |
| `internal/engine/protocol.go` | 消息帧、握手协议 |
| `internal/engine/clipboard.go` | 系统剪贴板读写 |
| `internal/engine/discovery.go` | UDP 局域网发现 |
| `internal/engine/config.go` | 配置加载与保存 |
| `tray_darwin.go` / `tray_darwin.m` / `tray_darwin.h` | macOS NSStatusItem 托盘（cgo） |
| `tray_other.go` | 非 macOS 平台 no-op 占位 |
| `icon.go` | 嵌入图标字节 |
| `frontend/` | Vue 3 + Vite + Tailwind 前端项目 |
| `frontend/src/App.vue` | 主布局：状态条 + 三段卡片 |
| `frontend/src/components/` | StatusBar / DeviceSettings / PeersSection / HistoryList / ToastStack |
| `frontend/src/composables/` | useAppState / useToast |
| `wails.json` | Wails 项目元数据 |
| `build/bin/` | `wails build` 产物目录 |