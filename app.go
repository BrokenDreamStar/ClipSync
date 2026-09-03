package main

import (
	"context"
	"encoding/base64"
	"sort"
	"time"

	"clipsync/internal/engine"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 是暴露给前端的绑定结构体。Wails v2 不允许方法返回 error，
// 故绑定方法均以 "(T, string)" 元组返回错误信息：第二项空字符串代表成功。
type App struct {
	ctx    context.Context
	engine *engine.Engine
}

// NewApp 由 main.go 在 wails.OnStartup 中调用。
func NewApp(ctx context.Context, e *engine.Engine) *App {
	return &App{ctx: ctx, engine: e}
}

func (a *App) subscribe() {
	emit := func(name string) func() {
		return func() { runtime.EventsEmit(a.ctx, name) }
	}
	a.engine.Subscribe(engine.TopicHistory, emit("engine:history"))
	a.engine.Subscribe(engine.TopicPeers, emit("engine:peers"))
	a.engine.Subscribe(engine.TopicDiscovered, emit("engine:discovered"))
	a.engine.Subscribe(engine.TopicConfig, emit("engine:config"))
	a.engine.Subscribe(engine.TopicPairRequest, emit("engine:pair_request"))
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
	Name     string     `json:"name"`
	Port     int        `json:"port"`
	DeviceID string     `json:"device_id"`
	Theme    string     `json:"theme"`
	Peers    []PeerView `json:"peers"`
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
		Name:     cfg.Name,
		Port:     cfg.Port,
		DeviceID: cfg.DeviceID,
		Theme:    cfg.Theme,
		Peers:    peerViews(cfg),
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
	runtime.WindowSetBackgroundColour(a.ctx, r, g, b, 1)
	return ""
}

// GetAutostart 返回当前是否已开启开机自启（以系统设置为准）。
func (a *App) GetAutostart() bool {
	return autostartEnabled()
}

// SetAutostart 开启/关闭开机自启：macOS 写 LaunchAgent，Windows 写注册表 Run 键。
func (a *App) SetAutostart(enable bool) string {
	return errStr(setAutostart(enable))
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

// ---- 生命周期 / 托盘菜单回调 ----

// ShowMainWindow 由托盘菜单"打开主窗口"调用。
func (a *App) ShowMainWindow() {
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

// Quit 由托盘菜单"退出"调用：停 engine → 让 Wails 退出主循环。
// OnBeforeClose 会直接放行,wails.Run 真正返回,main 再补一次 os.Exit
// 兜底处理 NSStatusItem / clipboard 子线程残留。
func (a *App) Quit() {
	a.engine.Stop()
	runtime.Quit(a.ctx)
}
