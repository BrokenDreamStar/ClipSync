package main

import (
	"encoding/base64"
	"sort"
	"time"

	"clipsync/internal/engine"
	"clipsync/internal/updater"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// App 是暴露给前端的绑定服务。约定：绑定方法不返回 error，而是返回
// "(T, string)" 元组：第二项空字符串代表成功（见 errStr）。该约定成形于
// Wails v2（v2 绑定方法不允许返回 error），前端已按"第二项是否为空"判断
// 成败，迁移到 v3 后维持不变。
type App struct {
	wails  *application.App
	window *application.WebviewWindow
	engine *engine.Engine

	// quitting 在真正退出（托盘"退出"）前置位；WindowClosing 钩子据此
	// 放行关闭请求，而"点红钮"这类普通关闭仍被拦截成隐藏。
	quitting bool
}

// NewApp 由 main.go 在 application.New 之前调用。此处即完成引擎 topic
// 订阅（引擎尚未 Start，不会丢事件）；wails / window 引用随后由 main 注入，
// 注入完成后引擎才启动，emit 时二者必然非空。
func NewApp(e *engine.Engine) *App {
	a := &App{engine: e}
	a.subscribe()
	return a
}

func (a *App) subscribe() {
	emit := func(name string) func() {
		return func() { a.wails.Event.Emit(name) }
	}
	a.engine.Subscribe(engine.TopicHistory, emit("engine:history"))
	a.engine.Subscribe(engine.TopicPeers, emit("engine:peers"))
	a.engine.Subscribe(engine.TopicDiscovered, emit("engine:discovered"))
	a.engine.Subscribe(engine.TopicConfig, emit("engine:config"))
	a.engine.Subscribe(engine.TopicPairRequest, emit("engine:pair_request"))
	// 更新进度是带数据的快照（下载字节数等），与引擎的无参事件不同。
	updater.Subscribe(func(s updater.State) {
		a.wails.Event.Emit("updater:state", s)
	})
}

// errStr 把 error 转成空串/字符串，前端根据是否为空判断成功。
func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// ---- 配置 ----

// ConfigView 返回当前配置（不含 WebPort，避免暴露未使用的字段）。
type ConfigView struct {
	Name           string     `json:"name"`
	Port           int        `json:"port"`
	DeviceID       string     `json:"device_id"`
	Theme          string     `json:"theme"`
	StartMinimized bool       `json:"start_minimized"`
	Peers          []PeerView `json:"peers"`
}

// PeerView 是前端展示用的对端信息。
type PeerView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Addr string `json:"addr"`
}

func (a *App) GetConfig() (ConfigView, string) {
	cfg := a.engine.Cfg()
	if cfg == nil {
		return ConfigView{}, "engine 未启动"
	}
	return ConfigView{
		Name:           cfg.Name,
		Port:           cfg.Port,
		DeviceID:       cfg.DeviceID,
		Theme:          cfg.Theme,
		StartMinimized: cfg.StartMinimized,
		Peers:          peerViews(cfg),
	}, ""
}

func (a *App) SaveConfig(name string, port int) string {
	return errStr(a.engine.SaveConfig(name, port))
}

// ---- 外观 / 自启 ----

// SetTheme 保存界面主题偏好（dark / light / system），并同步原生窗口外观与背景色。
func (a *App) SetTheme(theme string) string {
	if err := a.engine.SetTheme(theme); err != nil {
		return err.Error()
	}
	applyAppearance(theme)
	r, g, b := backgroundColorFor(theme)
	a.window.SetBackgroundColour(application.NewRGB(r, g, b))
	return ""
}

// GetAutostart 返回当前是否已开启开机自启（以系统设置为准）。
func (a *App) GetAutostart() bool {
	return autostartEnabled()
}

// SetAutostart 开启/关闭开机自启：macOS 写 LaunchAgent，Windows 写注册表 Run 键，
// Linux 写 XDG autostart .desktop（AppImage 环境记录 $APPIMAGE 路径）。
func (a *App) SetAutostart(enable bool) string {
	return errStr(setAutostart(enable))
}

// SetStartMinimized 开关「启动时最小化到任务栏」：写入配置，下次启动生效。
func (a *App) SetStartMinimized(enable bool) string {
	return errStr(a.engine.SetStartMinimized(enable))
}

// ---- 对端 ----

func peerViews(cfg *engine.Config) []PeerView {
	out := make([]PeerView, 0, len(cfg.Peers))
	for _, p := range cfg.Peers {
		out = append(out, PeerView{ID: p.ID, Name: p.Name, Addr: p.Addr})
	}
	return out
}

func (a *App) GetPeers() []PeerView {
	out := peerViews(a.engine.Cfg())
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// PairWith 向指定地址发起配对（发起方），对方同意后返回对端信息。
func (a *App) PairWith(addr string) (PeerView, string) {
	p, err := a.engine.PairWith(addr)
	if err != nil {
		return PeerView{}, err.Error()
	}
	return PeerView{ID: p.ID, Name: p.Name, Addr: p.Addr}, ""
}

// PairDiscovered 向已发现设备（按 deviceID）发起配对。
func (a *App) PairDiscovered(deviceID string) (PeerView, string) {
	p, err := a.engine.PairDiscovered(deviceID)
	if err != nil {
		return PeerView{}, err.Error()
	}
	return PeerView{ID: p.ID, Name: p.Name, Addr: p.Addr}, ""
}

func (a *App) RemovePeer(addr string) string {
	return errStr(a.engine.RemovePeer(addr))
}

func (a *App) RemovePeerByName(name string) string {
	return errStr(a.engine.RemovePeerByName(name))
}

func (a *App) IsAddrOnline(addr string) bool {
	return a.engine.IsAddrOnline(addr)
}

func (a *App) IsPairedByName(name string) bool {
	return a.engine.IsPaired(name)
}

// ---- 发现 ----

func (a *App) GetDiscovered() []engine.DiscoveredPeer {
	return a.engine.Discovered()
}

func (a *App) ScanNow() {
	a.engine.ScanNow()
}

// ---- 入站配对请求 ----

func (a *App) GetPairRequests() []engine.PairRequest {
	return a.engine.GetPairRequests()
}

// RespondPairRequest 同意（accept=true）或拒绝（accept=false）一条入站配对请求。
func (a *App) RespondPairRequest(deviceID string, accept bool) string {
	return errStr(a.engine.RespondPairRequest(deviceID, accept))
}

// ---- 连接状态 ----

func (a *App) GetConnectedPeers() []string {
	out := a.engine.ConnectedPeers()
	sort.Strings(out)
	return out
}

// ---- 历史 ----

// HistoryView 是前端展示用的精简版本（不含大字节 Data）。
type HistoryView struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	From    string `json:"from"`
	Size    int    `json:"size"`
	Time    string `json:"time"`
	Preview string `json:"preview"`
}

func (a *App) GetHistory() ([]HistoryView, string) {
	items := a.engine.History()
	out := make([]HistoryView, 0, len(items))
	for _, it := range items {
		out = append(out, HistoryView{
			ID:      it.ID,
			Kind:    string(it.Kind),
			From:    it.From,
			Size:    it.Size,
			Time:    it.Time.Format(time.RFC3339),
			Preview: it.Preview,
		})
	}
	return out, ""
}

// GetHistoryData 返回指定 ID 的原始字节；前端用 base64 字符串渲染。
func (a *App) GetHistoryData(id string) (string, string) {
	data, ok := a.engine.HistoryData(id)
	if !ok {
		return "", "记录不存在"
	}
	return base64.StdEncoding.EncodeToString(data), ""
}

func (a *App) CopyLocal(id string) string {
	return errStr(a.engine.CopyLocal(id))
}

// ---- 软件更新 ----

// GetAppVersion 返回当前应用版本号（AboutView 展示用，
// 来源 //go:embed build/config.yml，与 Info.plist / Windows syso 同源）。
func (a *App) GetAppVersion() (string, string) {
	return updater.AppVersion, ""
}

// CheckUpdate 检查 GitHub 最新 Release（结果缓存 60 秒）。
// 检查失败（网络 / 清单问题）不以错误返回：原因放在 CheckResult.Error 里展示。
func (a *App) CheckUpdate() (updater.CheckResult, string) {
	return updater.Check(false), ""
}

// GetUpdateState 返回进行中更新的进度快照（前端恢复界面 / 轮询兜底用）。
func (a *App) GetUpdateState() (updater.State, string) {
	return updater.GetState(), ""
}

// ApplyUpdate 开始更新到指定 tag：校验后立即返回，下载与替换在后台进行，
// 进度通过 updater:state 事件推送。完成后应用自动退出并由 helper 拉起新版本。
func (a *App) ApplyUpdate(tag string) (updater.State, string) {
	if err := updater.Apply(tag); err != nil {
		return updater.GetState(), err.Error()
	}
	return updater.GetState(), ""
}

// ---- 生命周期 / 托盘菜单回调 ----

// ShowMainWindow 由托盘菜单"打开主窗口"调用。
func (a *App) ShowMainWindow() {
	a.window.Show()
	a.window.Restore()
}

// Quit 由托盘菜单"退出"调用：置退出标志 → 让 Wails 退出主循环。
// WindowClosing 钩子看到退出标志后放行；引擎停止与 wl-paste 清理统一在
// OnShutdown 回调里完成（见 main.go）。
func (a *App) Quit() {
	a.quitting = true
	a.wails.Quit()
}
