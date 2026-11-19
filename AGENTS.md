# ClipSync — Agent 工作指南

局域网内 macOS、Windows 与 Linux 之间的剪贴板同步工具（文本 + PNG 图片）。Wails v3 桌面应用：Go 后端 + Vue 3 / Vite / Tailwind 前端，UI 与文档均为中文。

## 常用命令

```bash
wails3 dev                 # 开发模式（Vite 热重载 + Go 重编译，读 build/config.yml）
wails3 task build          # 构建当前平台，产物在 build/bin/
wails3 task darwin:package:universal   # macOS 通用 .app（arm64+amd64 lipo 合并）
wails3 task windows:build:all          # Windows amd64+arm64 双架构（纯 syscall，无需 mingw）
wails3 task windows:build ARCH=amd64  # Windows 单架构（ARCH 可选 amd64/arm64）
wails3 task linux:build ARCH=amd64 EXTRA_TAGS=gtk3   # Linux（ARCH 可选 amd64/arm64，见下）
go test ./...             # Go 测试（集中在 internal/engine）
go test ./internal/engine -run TestPairing   # 聚焦单个测试
cd frontend && npm run build      # 含 vue-tsc --noEmit 类型检查，是前端的 lint+typecheck
cd frontend && npx vue-tsc --noEmit  # 仅类型检查
```

- CLI 安装：`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0`（与 CI / Docker 镜像里的版本保持一致）。`wails3 task` 内置任务运行器，不需要单独安装 go-task。
- 前端无 ESLint；类型检查靠 `vue-tsc --noEmit`（strict 模式）。
- `main.go` 通过 `//go:embed all:frontend/dist` 内嵌前端产物：`go build` / `go test` 根包前需保证 `frontend/dist` 已存在（`wails3 task build` 或 `npm run build` 生成）。
- 纯浏览器预览 UI（无需 Wails）：`cd frontend && npm run dev` 后打开 `http://localhost:5173/preview.html`（mock 入口 `main.preview.ts`，通过 `@wailsio/runtime` 的官方 `setTransport` 钩子按方法名路由 mock 数据）。
- Windows 构建是纯 syscall 交叉编译（`CGO_ENABLED=0`），**不再需要 mingw-w64**；图标/版本资源由 `build/windows/Taskfile.yml` 的 `generate:syso` 从 `build/windows/icon.ico` + `info.json` 生成。
- Linux 构建需 GTK3 / WebKitGTK 4.1 开发库（`sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev`），且必须带 `EXTRA_TAGS=gtk3`（v3 以该标签选 GTK3/WebKitGTK-4.1 后端，默认后端是 GTK4/WebKitGTK-6.0；v2 时代的 `webkit2_41` 标签已废弃）。AppImage：`wails3 task linux:package`。Linux 只能在 Linux 环境（或 CI）上构建，macOS 无法交叉编译。
- **macOS 上构建 Linux 版走 Docker**：`sh build/linux/build_in_docker.sh [amd64|arm64]`（ubuntu:24.04 镜像，与 CI 同构，默认 amd64）。amd64 容器在 Apple Silicon 上依赖 Docker Desktop/OrbStack 的 Rosetta 转译；**arm64 容器原生运行**，Apple Silicon 上本地即可产出 aarch64 AppImage（Intel Mac 上 arm64 走 QEMU 全模拟，慢但可用）。脚本串起 4 步：`wails3 task linux:build` → `go vet` → `go test` → `build_appimage.sh`。AppImage 阶段在 **amd64 转译容器**里会自动跳过（Rosetta 不翻译 `appimagetool` 这类 type-1 非 PIE ELF），由 CI/Linux runner 接管；arm64 原生容器无此问题。`node_modules` / `go-build` 按架构分 named volume 缓存（esbuild / Go 编译缓存都架构相关）、`gomod` 跨架构共用，不会污染宿主机依赖。

## 架构与层次边界

- `main.go` — Wails v3 入口：CLI 参数（`-config` / `-add-peer` / `-peers` / `-headless`）、`application.New(Options)` + `app.Window.NewWithOptions` + `SystemTray` + `RegisterHook`。注意初始化顺序：`NewApp`（订阅引擎 topic）→ 注入 `svc.wails` → 创建窗口 → `eng.Start()`，保证第一条引擎事件发出前订阅与 app 引用都已就位。
- `app.go` — 暴露给前端的绑定服务（`application.NewService`）。**约定：绑定方法不返回 `error`，而是返回 `(T, string)` 元组**，第二项为空串表示成功（见 `errStr`）。该约定成形于 v2（v2 绑定不允许返回 error），前端已按元组解包判断成败，维持不变；新增后端方法在此添加。
- `internal/engine/` — 全部同步逻辑，不依赖 Wails：`engine.go`（引擎 + topic 发布订阅）、`server.go`（TCP 服务端）、`client.go`（拨号/重连）、`protocol.go`（消息帧与握手）、`clipboard.go`、`discovery.go`（UDP 广播，端口 9251）、`config.go`。
- `internal/clipboard/` — 剪贴板抽象层，运行时选后端：非 Linux 走 `golang.design/x/clipboard`；Linux 先试 X11（同一库），失败（纯 Wayland 会话）回退 `wl-clipboard`（`wl-copy` / `wl-paste`，见 `backend_wayland_linux.go`）。engine 与 main 只 import 本包，不要直接 import `golang.design/x/clipboard`。
- `internal/updater/` — 从 GitHub Releases 自更新，不依赖 Wails。更新源是 Release 附带的 `latest.json` 清单（CI 在 release.yml 里生成，schema 1）：拉取走 `releases/latest/download/latest.json` 稳定地址，`manifest.go` 做严格校验（tag/版本一致性、产物名 = `ClipSync-<版本>-<平台键><扩展名>`、sha256），是更新器的信任边界，**不做容错猜测**。平台安装器按 GOOS 分文件：Windows/Linux 单文件换名替换（`install_common.go` 的 `swapInPlace`，旧文件留 `.old` 回滚点），macOS 挂 DMG + `ditto` 覆盖 `.app`（`install_darwin.go`）。重启统一走「detached helper 等旧进程退出再拉起新版」——引擎的 TCP/UDP 端口必须先释放，**不能改成直接启动新进程**。版本号由 `main.go` 内嵌 `build/config.yml` 解析注入（`SetCurrentVersion`），前端 `useUpdate` 经 `updater:state` 事件拿进度；dev 构建（无 `production` tag）与 macOS 非 `.app` bundle 运行会拒绝自更新。
- 事件流：engine 通过 `Subscribe`/`publish(topic)` 通知 → `app.go` 转成 Wails 事件 `engine:history`、`engine:peers`、`engine:discovered`、`engine:config`、`engine:pair_request`（`a.wails.Event.Emit`）→ 前端 `useAppState` 里 `Events.On`（`@wailsio/runtime`）监听；更新进度走 `updater:state` 事件（Emit 单参数时前端 `ev.data` 即对象本身）→ 前端 `useUpdate`。
- `frontend/bindings/**` 为 `wails3 generate bindings -ts -i -names` 静态生成，**不要手改**；改了 `app.go` 绑定后由 `wails3 task build`（依赖链里含 generate:bindings）重新生成。生成以方法名调用（`Call.ByName('main.App.方法')`）而非数字 ID，浏览器预览 mock 依赖这一点。
- 前端运行时是 npm 包 `@wailsio/runtime`（锁定 `3.0.0`，与 wails3 CLI / go.mod 的 v3 版本保持一致升级）。
- 托盘：`main.go` 里用 v3 内置 `app.SystemTray.New()` + `app.NewMenu()`（ApplicationStarted 回调中创建；Linux 后端是纯 Go DBus StatusNotifierItem，与主循环统一调度，无第二 gtk_main 冲突）。macOS 托盘图标用 `SetTemplateIcon`（模板语义、自动随系统明暗反色），Windows/Linux 用 `SetIcon`。
- 平台相关文件用后缀区分：`autostart_*`（自定义实现，勿换成 v3 内置 AutostartManager——Linux 版会记录 `$APPIMAGE` 路径，AppImage 重启后仍有效）、`appearance_*`（macOS 运行时切 NSApp 外观，v3 无对应 API，保留 cgo）、`console_windows.go` 等。
- 构建任务体系：根 `Taskfile.yml`（build/package/run/dev 分发到 `build/<平台>/Taskfile.yml`），公共任务在 `build/Taskfile.yml`；项目元数据与版本号在 `build/config.yml`，改后跑 `wails3 task common:update:build-assets` 渲染 Info.plist / Windows info.json 等资产（会覆盖 `build/darwin`、`build/windows` 下的模板化资产）。

## 已知坑

- **跨平台桩**：在某个 `*_darwin.go` / `*_windows.go` 中新增平台函数时，必须同步在其他平台的 `_other.go` / `_windows.go` 补空实现，否则交叉编译失败（v0.0.3 曾因 `SetAccessoryActivationPolicy` 缺 Windows 实现编译失败）。
- **版本号需两处同步**：`build/config.yml` 的 `info.version`（CI 从这里解析，`wails3 update build-assets` 再渲染进 Info.plist / Windows syso；应用运行时也从 `main.go` 内嵌的同名文件解析版本号供更新器比较——`AboutView.vue` 不再硬编码版本，改这个 yaml 就够了）与 `CHANGELOG.md`。注意 `go:embed` 后 `build/config.yml` 同时是 Go 源的一部分，改它要重新编译才生效。
- **主题**：颜色全部走 `frontend/src/style.css` 的 CSS 变量（`:root, html.dark` 深色为默认，`html.light` 覆盖；RGB 三元组供 Tailwind `<alpha-value>` 用）。`html` 上的 `dark`/`light` class 由 `index.html` 内联脚本（首帧防闪烁）和 `useTheme`（运行时）写入，二者都要保持一致。写组件时用 Tailwind 主题 token（如 `text-muted`、`bg-surface`），不要硬编码颜色。
- 运行时配置在 `~/.clipsync/config.json`；Windows 日志写 `%USERPROFILE%\.clipsync\app.log`。
- **窗口无边框**：Windows / Linux 走 `WebviewWindowOptions.Frameless`（`main.go` 里按 `runtime.GOOS` 条件开启），无原生标题栏；拖窗口靠前端 `Sidebar` 的 `.drag-region`（CSS `--wails-draggable`，v3 沿用 v2 约定）。macOS 用 `application.MacTitleBarHidden`（保留红绿灯），别把 macOS 也改成 Frameless。
- **"关闭即隐藏"**：v3 无 v2 的 `HideWindowOnClose` 选项，由 `main.go` 里 `window.RegisterHook(events.Common.WindowClosing, ...)` + `event.Cancel()` + `window.Hide()` 实现；托盘"退出"先置 `svc.quitting` 标志再 `wails.Quit()`，钩子看到标志后放行。改这两个行为时必须保持标志位语义。
- **Linux 托盘/剪贴板限制**：托盘走 DBus StatusNotifierItem，原生 GNOME 需 AppIndicator 扩展；剪贴板优先走 X11（Wayland 会话下经 XWayland），纯 Wayland 会话回退 wl-clipboard——`wl-paste --watch` 是常驻子进程，`main.go` 的 `OnShutdown` 回调里必须调 `clipboard.Shutdown()` 清理；`wl-copy` 会 fork 常驻进程持有内容，给它接 stdout/stderr 管道会让 `cmd.Wait()` 卡到超时（见 `backend_wayland_linux.go` 的注释）。AppImage 内置这两个工具（`build_appimage.sh` + AppRun 的 PATH 注入）。
- **icns 全尺寸**：`wails3 generate icons` 生成的 icns 缺 @1x 小尺寸（16/32/64），Finder 低 DPI 视图会退化成占位图标；`build/darwin/build_icns.sh` 用 `build/appicon.png` 重打 @1x+@2x 全尺寸并覆盖 `.app` 内的 `icons.icns`（CI 在 package 之后执行；本地发 .app 也要手动跑一次）。
- **签名后不得改动 .app 内容**：`create:app:bundle` 在拷贝完资源后就 ad-hoc 签名（封印所有资源的哈希）。**任何**后续对 `.app` 内文件的覆盖（如 build_icns.sh 换 icns）都会让 `codesign --verify` 报 `sealed resource is missing or invalid`，带隔离属性的下载版会被 Gatekeeper 判为「已损坏」而拒绝启动（v0.1.0 的 macOS DMG 曾因此全员无法启动）。因此 `build_icns.sh` 末尾会重新 `codesign --force --deep --sign -`，release.yml 也在 DMG 打包前用 `codesign --verify --deep` 做门禁；新增任何"签名后再改 .app"的步骤都必须跟一次重签。ad-hoc 签名的下载版仍会触发「无法验证开发者」（无 Developer ID / 公证所致），用户需在系统设置里「仍要打开」——这不是 bug，是签名链的固有摩擦。
- **WSL2 跑 AppImage**：`./ClipSync-*.AppImage` 默认要 libfuse2 挂载 squashfs，WSL2 CLI 装的 Ubuntu 22.04 可能没装（24.04 是 `libfuse2t64`）。两种解决：(1) `sudo apt install libfuse2 fuse`（或 `libfuse2t64`）后直接运行；(2) 不装 FUSE 时加 `--appimage-extract-and-run`（CI 用的就是这参数），或 `--appimage-extract` 后跑 `./squashfs-root/AppRun`。WSL2 内运行时库需自备：`libgtk-3-0` + `libwebkit2gtk-4.1-0`（22.04 是 4.0，包名 `libwebkit2gtk-4.0-37`），AppImage 不打包这些。WSLg（Win11）默认带，能弹窗；裸 WSL2 只能跑 CLI 路径。

## 提交与发布约定

- 提交信息：`feat:` / `fix:` / `style:` / `ci:` / `chore:` 前缀 + 中文描述。
- 日常开发在 `dev` 分支；push 到 `main` 触发 `.github/workflows/release.yml` 构建五平台产物并发 release：macOS DMG（universal）、Windows EXE（amd64 + arm64，同一 amd64 runner 交叉编译）、Linux AppImage（x86_64 + aarch64，arm64 走 GitHub 公开仓库免费的 `ubuntu-24.04-arm` 原生 runner）。
- release 任务会在 Release 资产里附带 `latest.json` 更新清单（应用内更新器读取它；产物名必须保持 `ClipSync-<版本>-<平台键><扩展名>` 命名，`internal/updater/manifest.go` 按此校验，两边改动需同步 `updater_test.go` 里的 TestParseCISample）。
- 用户可见的变更需在 `CHANGELOG.md` 的 `[Unreleased]` 下追加中文条目（Keep a Changelog 格式）。
