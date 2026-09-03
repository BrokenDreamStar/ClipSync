package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"clipsync/internal/engine"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"golang.design/x/clipboard"
)

//go:embed all:frontend/dist
var assets embed.FS

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

	if err := clipboard.Init(); err != nil {
		log.Fatalf("初始化剪贴板失败: %v", err)
	}

	eng := engine.NewEngine(cfg, *configFile)
	eng.Start()
	fmt.Printf("ClipSync 已启动，本机名: %s，端口: %d\n", cfg.Name, cfg.Port)

	if *headless {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		eng.Stop()
		return
	}

	app := NewApp(nil, eng)

	bgR, bgG, bgB := backgroundColorFor(cfg.Theme)
	err = wails.Run(&options.App{
		Title:  "ClipSync",
		Width:  880,
		Height: 560,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: bgR, G: bgG, B: bgB, A: 1},
		OnStartup: func(ctx context.Context) {
			app.ctx = ctx
			app.subscribe()
			// macOS 上安装自定义 NSStatusItem 托盘（避免与 Wails AppDelegate 冲突）。
			// 必须放在 OnStartup 里:那时 NSApp 主线程已就绪,Cocoa 主队列活跃,
			// NSStatusBar 创建窗口才不会触发 NSInternalInconsistencyException。
			// 其他平台为 no-op。
			StartTray(app.ShowMainWindow, app.Quit)
		},
		Bind: []interface{}{app},
		Mac: &mac.Options{
			TitleBar:   mac.TitleBarHidden(),
			Appearance: macAppearance(cfg.Theme),
			About: &mac.AboutInfo{
				Title:   "ClipSync",
				Message: "局域网 macOS ↔ Windows 剪贴板同步工具",
			},
		},
		Windows: &windows.Options{
			// "system" 跟随系统深浅色；native 标题栏颜色随之切换。
			Theme: windowsTheme(cfg.Theme),
		},
		HideWindowOnClose: true,
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			// "关闭"/Cmd+W 的隐藏到托盘行为由 HideWindowOnClose 在各平台原生产层处理，
			// 不会走到这里。因此 OnBeforeClose 只会收到真正的退出请求
			// （Cmd+Q / 应用菜单 Quit / 托盘"退出"），直接放行即可：
			// 若在这里拦截隐藏，Cmd+Q 会被误当成"隐藏窗口"，应用无法完全退出。
			return false
		},
	})
	if err != nil {
		eng.Stop()
		log.Fatalf("wails.Run: %v", err)
	}
	// 兜底:NSStatusItem / clipboard 等子线程残留会让进程在 Wails 退出后
	// 短暂挂起;此处强制结束,确保"退出"就是真的退出。
	eng.Stop()
	os.Exit(0)
}

// onTrayReady 已删除，改用 StartTray。

// isConsole / openWindowsLogFile 保留 Windows 帮助函数。

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

// macAppearance 返回 macOS NSAppearance；"system" 返回默认值让窗口跟随系统外观。
func macAppearance(theme string) mac.AppearanceType {
	switch theme {
	case "light":
		return mac.NSAppearanceNameAqua
	case "dark":
		return mac.NSAppearanceNameDarkAqua
	default:
		return mac.DefaultAppearance
	}
}

// windowsTheme 返回 Windows 原生窗口主题（标题栏颜色）；"system" 跟随系统。
func windowsTheme(theme string) windows.Theme {
	switch theme {
	case "light":
		return windows.Light
	case "dark":
		return windows.Dark
	default:
		return windows.SystemDefault
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