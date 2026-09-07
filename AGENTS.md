# ClipSync — Agent 工作指南

局域网内 macOS、Windows 与 Linux 之间的剪贴板同步工具（文本 + PNG 图片）。Wails v2 桌面应用：Go 后端 + Vue 3 / Vite / Tailwind 前端，UI 与文档均为中文。

## 常用命令

```bash
wails dev                 # 开发模式（Vite 热重载 + Go 重编译，并重新生成 wailsjs 绑定）
wails build               # 构建当前平台，产物在 build/bin/
go test ./...             # Go 测试（集中在 internal/engine）
go test ./internal/engine -run TestPairing   # 聚焦单个测试
cd frontend && npm run build      # 含 vue-tsc --noEmit 类型检查，是前端的 lint+typecheck
cd frontend && npx vue-tsc --noEmit  # 仅类型检查
```

- 前端无 ESLint；类型检查靠 `vue-tsc --noEmit`（strict 模式）。
- `main.go` 通过 `//go:embed all:frontend/dist` 内嵌前端产物：`go build` / `go test` 根包前需保证 `frontend/dist` 已存在（`wails build` 或 `npm run build` 生成）。
- 纯浏览器预览 UI（无需 Wails）：`cd frontend && npm run dev` 后打开 `http://localhost:5173/preview.html`（mock 入口 `main.preview.ts`）或 `mock.html`。
- Windows 交叉编译：`CC=x86_64-w64-mingw32-gcc wails build -platform windows/amd64`。不要写 `-compiler gcc`（那是 Go 编译器参数，会导致 `go mod tidy` 报错）。
- Linux 构建需 GTK3 / WebKitGTK 4.1 开发库（`sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev`），且必须带 `-tags webkit2_41`（Wails CLI 不自动探测）：`wails build -platform linux/amd64 -tags webkit2_41`；随后 `sh build/linux/build_appimage.sh` 打 AppImage。Linux 只能在 Linux 环境（或 CI）上构建，macOS 无法交叉编译。
- **macOS 上构建 Linux 版走 Docker**：`sh build/linux/build_in_docker.sh`（amd64 ubuntu:24.04 镜像，与 CI 同构；Apple Silicon 依赖 Docker Desktop/OrbStack 的 Rosetta 转译）。脚本串起 5 步：`go mod tidy` → `wails build -tags webkit2_41` → `go vet` → `go test` → `build_appimage.sh`。AppImage 阶段在 Docker Desktop on Apple Silicon 上**会自动跳过**（其 x86_64 emulation 不翻译 `appimagetool` 这类 type-1 非 PIE ELF），由 CI/Linux runner 接管。真正要在 macOS 本机出 AppImage：用 OrbStack（QEMU 全模拟更稳）、Linux VM、WSL2，或直接 push 触发 CI。`frontend/node_modules` / `go-build` 用 named volume 缓存、不会污染宿主机 arm64 依赖。

## 架构与层次边界

- `main.go` — Wails 入口：CLI 参数（`-config` / `-add-peer` / `-peers` / `-headless`）、窗口/主题/托盘初始化。
- `app.go` — 暴露给前端的绑定层。**约定：绑定方法不返回 `error`，而是返回 `(T, string)` 元组**，第二项为空串表示成功（见 `errStr`）。新增后端方法在此添加。
- `internal/engine/` — 全部同步逻辑，不依赖 Wails：`engine.go`（引擎 + topic 发布订阅）、`server.go`（TCP 服务端）、`client.go`（拨号/重连）、`protocol.go`（消息帧与握手）、`clipboard.go`、`discovery.go`（UDP 广播，端口 9251）、`config.go`。
- `internal/clipboard/` — 剪贴板抽象层，运行时选后端：非 Linux 走 `golang.design/x/clipboard`；Linux 先试 X11（同一库），失败（纯 Wayland 会话）回退 `wl-clipboard`（`wl-copy` / `wl-paste`，见 `backend_wayland_linux.go`）。engine 与 main 只 import 本包，不要直接 import `golang.design/x/clipboard`。
- 事件流：engine 通过 `Subscribe`/`publish(topic)` 通知 → `app.go` 转成 Wails 事件 `engine:history`、`engine:peers`、`engine:discovered`、`engine:config`、`engine:pair_request` → 前端 `useAppState` 监听。
- `frontend/src/` — `components/`（SettingsView / DevicesView / HistoryView / AboutView / Sidebar 等），`composables/`（useAppState / useTheme / useToast）。路径别名 `@/` → `src/`。
- `frontend/src/wailsjs/**` 为 Wails 自动生成，**不要手改**；改了 `app.go` 绑定后由 `wails dev` / `wails build` 重新生成。
- 平台相关文件用后缀区分：`tray_darwin.go` / `tray_windows.go` / `tray_linux.go` / `tray_other.go`、`autostart_*`、`appearance_*` 等。Linux 托盘用 `fyne.io/systray`（纯 Go DBus / StatusNotifierItem），**不要**换回 `getlantern/systray`——其 Linux 实现要在线程里跑第二个 `gtk_main`，与 Wails 主线程的 gtk_main 冲突。

## 已知坑

- **跨平台桩**：在某个 `*_darwin.go` / `*_windows.go` 中新增平台函数时，必须同步在其他平台的 `_other.go` / `_windows.go` 补空实现，否则交叉编译失败（v0.0.3 曾因 `SetAccessoryActivationPolicy` 缺 Windows 实现编译失败）。
- **版本号需四处同步**：`wails.json`（`version` + `info.productVersion`）、`frontend/src/components/AboutView.vue` 的 `const version`、`build/windows/winres.json`、`CHANGELOG.md`。
- **主题**：颜色全部走 `frontend/src/style.css` 的 CSS 变量（`:root, html.dark` 深色为默认，`html.light` 覆盖；RGB 三元组供 Tailwind `<alpha-value>` 用）。`html` 上的 `dark`/`light` class 由 `index.html` 内联脚本（首帧防闪烁）和 `useTheme`（运行时）写入，二者都要保持一致。写组件时用 Tailwind 主题 token（如 `text-muted`、`bg-surface`），不要硬编码颜色。
- 运行时配置在 `~/.clipsync/config.json`；Windows 日志写 `%USERPROFILE%\.clipsync\app.log`。
- **窗口无边框**：Windows / Linux 走 `Frameless`（`main.go` 里按 `runtime.GOOS` 条件开启），无原生标题栏；拖窗口靠前端 `Sidebar` 的 `.drag-region`（`--wails-draggable`）。macOS 仍用 `mac.TitleBarHidden()`（需保留红绿灯），别把 macOS 也改成 Frameless。
- **Linux 托盘/剪贴板限制**：托盘走 DBus StatusNotifierItem，原生 GNOME 需 AppIndicator 扩展；剪贴板优先走 X11（Wayland 会话下经 XWayland），纯 Wayland 会话回退 wl-clipboard——`wl-paste --watch` 是常驻子进程，`main.go` 退出路径必须调 `clipboard.Shutdown()` 清理；`wl-copy` 会 fork 常驻进程持有内容，给它接 stdout/stderr 管道会让 `cmd.Wait()` 卡到超时（见 `backend_wayland_linux.go` 的注释）。AppImage 内置这两个工具（`build_appimage.sh` + AppRun 的 PATH 注入）。

## 提交与发布约定

- 提交信息：`feat:` / `fix:` / `style:` / `ci:` / `chore:` 前缀 + 中文描述。
- 日常开发在 `dev` 分支；push 到 `main` 触发 `.github/workflows/release.yml` 构建 macOS DMG、Windows EXE 与 Linux AppImage 并发 release。
- 用户可见的变更需在 `CHANGELOG.md` 的 `[Unreleased]` 下追加中文条目（Keep a Changelog 格式）。
