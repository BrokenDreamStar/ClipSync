package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"syscall"
	"time"

	"clipsync/internal/clipboard"
	"clipsync/internal/engine"
	"clipsync/internal/updater"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

// buildConfig 内嵌构建配置，运行时解析 info.version 作为应用版本号
// （更新器比较版本用）。与 Info.plist / Windows syso 同源，且 go build /
// go test 产物始终拿得到，无需 ldflags 注入；AboutView 的硬编码版本由此消掉。
//
//go:embed build/config.yml
var buildConfig []byte

// versionRe 匹配 config.yml 里 info 块的两空格缩进版本行（如 "0.1.0"）。
var versionRe = regexp.MustCompile(`(?m)^[ \t]+version:[ \t]*"([0-9]+\.[0-9]+\.[0-9]+)"`)

// appVersionFromConfig 从内嵌 config.yml 解析版本号；解析失败返回空串
// （更新检查会以「版本号缺失」失败，而不是误判）。
func appVersionFromConfig(data []byte) string {
	m := versionRe.FindSubmatch(data)
	if m == nil {
		return ""
	}
	return string(m[1])
}

func main() {
	configFile := flag.String("config", engine.ConfigPath(), "配置文件路径")
	addPeer := flag.String("add-peer", "", "向指定地址发起配对（host:port）并退出")
	listPeers := flag.Bool("peers", false, "打印当前对端列表并退出")
	headless := flag.Bool("headless", false, "无界面模式：不启动窗口与托盘")
	flag.Parse()

	// Windows GUI subsystem 下没有 stdout/stderr；把日志重定向到文件。
	if runtime.GOOS == "windows" && !isConsole() {
		if f, err := openWindowsLogFile(); err == nil {
			log.SetOutput(f)
		}
	}

	cfg, err := engine.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("加载配置: %v", err)
	}

	// 版本号注入：更新器检查更新时与 GitHub 最新 Release 比较。
	updater.SetCurrentVersion(appVersionFromConfig(buildConfig))
	if updater.AppVersion == "" {
		log.Println("警告: 未能从 build/config.yml 解析版本号，更新检查将不可用")
	}

	// 快速路径：发起一次配对后落盘并退出。
	if *addPeer != "" {
		peer, err := engine.PairAddr(cfg, engine.NormalizePeer(*addPeer), 120*time.Second)
		if err != nil {
			log.Fatalf("配对失败: %v", err)
		}
		cfg.UpsertPeer(*peer)
		if err := cfg.Save(*configFile); err != nil {
			log.Fatalf("保存配置: %v", err)
		}
		fmt.Printf("已配对对端: %s (%s)\n", peer.Name, peer.Addr)
		return
	}
	if *listPeers {
		for _, p := range cfg.Peers {
			fmt.Printf("%s\t%s\t%s\n", p.Name, p.ID, p.Addr)
		}
		return
	}

	// 绑定由 wails3 generate bindings 静态分析生成（不再运行二进制提取），
	// 因此无需 v2 时代的 -tags bindings 旁路模式，引擎可以无条件启动。

	// 先建引擎与绑定服务、再注入 Wails app 引用、最后 Start 引擎：
	// 保证 topic 订阅在第一条引擎事件发出前就位。
	eng := engine.NewEngine(cfg, *configFile)
	svc := NewApp(eng)

	// 更新完成后的退出走「托盘退出」同一路径：置 quitting 标志 → wails.Quit，
	// 引擎停止与 wl-paste 清理统一在 OnShutdown 完成；detached helper 等本进程
	// 退出后拉起新版本（见 internal/updater 的注释）。
	updater.SetQuitFunc(func() { svc.Quit() })

	if *headless {
		if err := clipboard.Init(); err != nil {
			log.Fatalf("初始化剪贴板失败: %v", err)
		}
		eng.Start()
		updater.CleanupStaleFiles()
		fmt.Printf("ClipSync 已启动，本机名: %s，端口: %d\n", cfg.Name, cfg.Port)
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		eng.Stop()
		clipboard.Shutdown()
		return
	}

	wailsApp := application.New(application.Options{
		Name:        "ClipSync",
		Description: "局域网 macOS / Windows / Linux 剪贴板同步工具",
		Icon:        iconBytes,
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			// accessory：应用不出现在 Dock、不被 Cmd+Tab 列出，
			// 只通过菜单栏托盘交互（替代 v2 的 SetAccessoryActivationPolicy cgo hack）。
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
	})
	svc.wails = wailsApp

	bgR, bgG, bgB := backgroundColorFor(cfg.Theme)
	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   "main",
		Title:  "ClipSync",
		Width:  880,
		Height: 560,
		// Windows / Linux 去掉系统原生标题栏，与 macOS 的 TitleBarHidden 观感一致；
		// 窗口拖动走前端侧栏的拖拽区（--wails-draggable，v3 沿用同一约定）。
		// macOS 不能用 Frameless——它需要保留原生红绿灯，仍走 Mac.TitleBar。
		Frameless: runtime.GOOS == "windows" || runtime.GOOS == "linux",
		// 「启动时最小化到任务栏」：窗口以隐藏方式创建，只保留托盘图标。
		Hidden: cfg.StartMinimized,
		// 原生窗口背景色与前端 --c-surface 一致，避免浅色主题下露出深色底。
		BackgroundColour: application.NewRGB(bgR, bgG, bgB),
		Mac: application.MacWindow{
			// 隐藏标题栏但保留红绿灯（对应 v2 mac.TitleBarHidden）。
			TitleBar:   application.MacTitleBarHidden,
			Appearance: macAppearance(cfg.Theme),
		},
		Windows: application.WindowsWindow{
			// "system" 跟随系统深浅色；原生标题栏颜色随之切换。
			Theme: windowsTheme(cfg.Theme),
		},
	})
	svc.window = window

	// 关闭请求 → 隐藏窗口（替代 v2 HideWindowOnClose）：
	// 红钮 / Cmd+W / 窗口关闭按钮都只隐藏，不结束应用；
	// 真正退出走托盘菜单的"退出"（svc.Quit 置 quitting 标志后放行）。
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if svc.quitting {
			return
		}
		event.Cancel()
		window.Hide()
	})

	// 托盘：Wails v3 内置 SystemTray。Linux 后端是纯 Go DBus
	// StatusNotifierItem（与 v2 用的 fyne.io/systray 同源，由 Wails 主循环统一
	// 调度，无第二 gtk_main 冲突）；macOS / Windows 各自原生实现。
	// 必须等 ApplicationStarted 再创建：此时主循环已就绪。
	wailsApp.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		menu := wailsApp.NewMenu()
		menu.Add("打开主窗口").OnClick(func(*application.Context) { svc.ShowMainWindow() })
		menu.AddSeparator()
		menu.Add("退出 ClipSync").OnClick(func(*application.Context) { svc.Quit() })

		tray := wailsApp.SystemTray.New()
		tray.SetTooltip("ClipSync — 局域网剪贴板同步")
		if runtime.GOOS == "darwin" {
			// 模板图标：菜单栏单色剪贴板形状，自动跟随系统明暗反色。
			// 用专门的 tray-darwin.png（24pt 画布、18pt 内容居中）而不是彩色
			// tray.png：v3 会把整图缩放到菜单栏 thickness（≈24pt），用彩色图
			// 会满幅渲染，观感比 v2 的 18pt 图标大一圈、形状也不一致
			// （v2 是 SF Symbol doc.on.clipboard 线稿）。
			tray.SetTemplateIcon(trayDarwinBytes)
		} else {
			tray.SetIcon(trayBytes)
		}
		tray.SetMenu(menu)
		// 与 v2 一致：单击（macOS 菜单栏惯例）与右键都弹出菜单。
		tray.OnClick(func() { tray.ShowMenu() })
		tray.OnRightClick(func() { tray.ShowMenu() })
		tray.Run()
	})

	// 引擎在订阅就位、Wails app 注入完成后启动，此后的事件都能安全 emit。
	if err := clipboard.Init(); err != nil {
		log.Fatalf("初始化剪贴板失败: %v", err)
	}
	eng.Start()
	// 上次更新留下的 .old 回滚点与 .part 半成品：新版本能起来说明它可用，清掉。
	updater.CleanupStaleFiles()
	fmt.Printf("ClipSync 已启动，本机名: %s，端口: %d\n", cfg.Name, cfg.Port)

	// 退出清理：停引擎、终止 Wayland 的 wl-paste --watch 子进程。
	wailsApp.OnShutdown(func() {
		eng.Stop()
		clipboard.Shutdown()
	})

	if err := wailsApp.Run(); err != nil {
		eng.Stop()
		log.Fatalf("wailsApp.Run: %v", err)
	}
	// 兜底：强制结束进程，防 clipboard 等子线程残留造成"退出后进程短暂挂起"。
	os.Exit(0)
}

// ---- 主题解析 ----

// effectiveDark 把主题偏好解析为「当前是否深色」；"system" 依据系统实时外观。
func effectiveDark(theme string) bool {
	switch theme {
	case "light":
		return false
	case "system":
		return systemPrefersDark()
	default:
		return true
	}
}

// backgroundColorFor 返回主题对应的原生窗口背景色（与前端 --c-surface 一致），
// 避免浅色主题下窗口加载/拉伸时露出深色底。
func backgroundColorFor(theme string) (r, g, b uint8) {
	if effectiveDark(theme) {
		return 0x24, 0x23, 0x21
	}
	return 0xFA, 0xFA, 0xF8
}

// macAppearance 返回 macOS NSWindow 外观；"system" 返回默认值让窗口跟随系统外观。
func macAppearance(theme string) application.MacAppearanceType {
	switch theme {
	case "light":
		return application.NSAppearanceNameAqua
	case "dark":
		return application.NSAppearanceNameDarkAqua
	default:
		return application.DefaultAppearance
	}
}

// windowsTheme 返回 Windows 原生窗口主题（标题栏颜色）；"system" 跟随系统。
func windowsTheme(theme string) application.Theme {
	switch theme {
	case "light":
		return application.Light
	case "dark":
		return application.Dark
	default:
		return application.SystemDefault
	}
}

func isConsole() bool {
	return getConsoleWindow() != 0
}

func openWindowsLogFile() (*os.File, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".clipsync")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(dir, "app.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}
