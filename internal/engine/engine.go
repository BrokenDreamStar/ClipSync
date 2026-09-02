package engine

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image/png"
	"log"
	"net"
	"reflect"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"golang.design/x/clipboard"
)

// HistoryItem 是一条剪贴板同步记录。
type HistoryItem struct {
	ID      string      `json:"id"`
	Kind    MessageKind `json:"kind"`
	From    string      `json:"from"`
	Size    int         `json:"size"`
	Time    time.Time   `json:"time"`
	Preview string      `json:"preview,omitempty"` // 文本预览（截断）
	Data    []byte      `json:"-"`                 // 完整内容，经 /api/history/{id}/data 提供
}

const (
	// historyMax 是历史记录条数上限。
	historyMax = 100
	// maxRetainedBytes 是历史记录在内存中的总字节上限（用于限制大量大图长期常驻）。
	maxRetainedBytes = 128 << 20 // 128 MiB
	// echoCheckWindow 是本端写入剪贴板后，watcher 读回内容时判断「是否缩回自己写入的图片」的时间窗。
	// macOS 会把写入的 PNG 重编码后再读回，字节不同，需在此窗口内用内容指纹识别回声。
	echoCheckWindow = 3 * time.Second
	// pairRequestTimeout 是配对请求等待对方确认的最长时间。
	pairRequestTimeout = 120 * time.Second
)

// Engine 是同步核心：剪贴板监听、广播、接收、历史记录、局域网发现与配对。
type Engine struct {
	cfg     *Config
	cfgPath string

	hub    *Hub
	dialer *Dialer
	disc   *Discovery
	discCh chan discoveredEvent

	mu       sync.Mutex
	lastHash string

	// 记录本端最近一次「写入剪贴板」的图片内容指纹与时间戳，
	// 用于 macOS 重编码 PNG 后识别出缩回（回声），避免重复历史与回广播。
	lastImageFP      [32]byte
	hasImageFP       bool
	lastImageWriteAt time.Time

	histMu      sync.Mutex
	history     []*HistoryItem
	totalBytes  int // history 中所有 Data 的字节总和（受 maxRetainedBytes 约束）
	nextSeq     int

	// pubsub：状态变化时按 topic 通知订阅者（Wails 前端通过 runtime.EventsEmit 收到）。
	listenersMu sync.RWMutex
	listeners   map[topic][]func()

	discCtx    context.Context
	discCancel context.CancelFunc

	discovered *DiscoveredPeerSet

	// pairMu 保护 pendingIncoming：待用户确认的「入站」配对请求。
	pairMu          sync.Mutex
	pendingIncoming map[string]*pendingPair
}

// pendingPair 记录一条等待用户确认的入站配对请求。
type pendingPair struct {
	req IncomingPairRequest
	ch  chan pairDecision
}

// pairDecision 是 Pair 确认的结果，由 Engine 产生、server 侧消费。
type pairDecision struct {
	accept bool
	secret string
	reason string
}

// discoveredEvent 是 Discovery → Engine 的内部事件。
type discoveredEvent struct {
	id   string
	name string
	addr string
}

// IncomingPairRequest 是一条待确认的入站配对请求（deviceID 即请求方身份）。
type IncomingPairRequest struct {
	DeviceID string
	Name     string
	Host     string
	Port     int
}

// PairRequest 是暴露给前端的一条待确认配对请求。
type PairRequest struct {
	DeviceID string    `json:"device_id"`
	Name     string    `json:"name"`
	Addr     string    `json:"addr"`
	Time     time.Time `json:"time"`
}

// topic 决定要广播哪类状态变化。
type topic int

const (
	TopicHistory      topic = iota // 历史新增
	TopicPeers                     // 已配对对端 / 在线状态变化
	TopicDiscovered                // 局域网发现变化
	TopicConfig                    // 配置变更
	TopicPairRequest               // 收到入站配对请求
)

// Subscribe 订阅 topic 上的状态变化；返回的 cancel 用于取消订阅。
func (e *Engine) Subscribe(t topic, fn func()) (cancel func()) {
	e.listenersMu.Lock()
	if e.listeners == nil {
		e.listeners = make(map[topic][]func())
	}
	e.listeners[t] = append(e.listeners[t], fn)
	e.listenersMu.Unlock()
	cancel = func() {
		e.listenersMu.Lock()
		defer e.listenersMu.Unlock()
		listeners := e.listeners[t]
		for i, l := range listeners {
			// 通过 reflect.ValueOf 比较函数值
			if reflect.ValueOf(l).Pointer() == reflect.ValueOf(fn).Pointer() {
				e.listeners[t] = append(listeners[:i], listeners[i+1:]...)
				return
			}
		}
	}
	return cancel
}

// publish 在调用方持锁时不要调用；触发订阅者异步执行。
func (e *Engine) publish(t topic) {
	e.listenersMu.RLock()
	listeners := append([]func(){}, e.listeners[t]...)
	e.listenersMu.RUnlock()
	for _, fn := range listeners {
		go fn()
	}
}

func NewEngine(cfg *Config, cfgPath string) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		cfg:             cfg,
		cfgPath:         cfgPath,
		hub:             NewHub(),
		dialer:          NewDialer(),
		discovered:      NewDiscoveredPeerSet(),
		pendingIncoming: make(map[string]*pendingPair),
		discCh:          make(chan discoveredEvent, 32),
		discCtx:         ctx,
		discCancel:      cancel,
		listeners:       make(map[topic][]func()),
	}
}

func hashOf(data []byte) string {
	sum := sha256.Sum256(data)
	return string(sum[:])
}

// Start 启动服务端、对端连接、本机剪贴板监听和局域网发现。
func (e *Engine) Start() {
	for i := range e.cfg.Peers {
		e.dialer.StartPeer(e, &e.cfg.Peers[i])
	}
	e.hub.OnConnect = e.onPeerConnected
	e.hub.OnDisconnect = e.onPeerDisconnected
	go StartServer(e)
	go e.watchClipboard()
	e.disc = NewDiscovery(e.cfg, DiscoveryPort)
	go func() {
		err := e.disc.Run(e.discCtx, func(remote *net.UDPAddr, a Announce) {
			host := remote.IP.String()
			addr := net.JoinHostPort(host, strconv.Itoa(a.Port))
			select {
			case e.discCh <- discoveredEvent{id: a.DeviceID, name: a.Name, addr: addr}:
			default:
				// 队列满就丢弃老事件，发现端周期性重发不会丢信息。
			}
		})
		if err != nil {
			log.Printf("discovery: %v", err)
		}
	}()
	go e.discoveryLoop()
}

// Stop 关闭发现、拨号循环；用于程序退出。
func (e *Engine) Stop() {
	if e.discCancel != nil {
		e.discCancel()
	}
	e.dialer.StopAll(e.hub)
}

// discoveryLoop 处理发现事件：刷新地址、决定是否配对。
func (e *Engine) discoveryLoop() {
	evictTicker := time.NewTicker(30 * time.Second)
	defer evictTicker.Stop()
	for {
		select {
		case <-e.discCtx.Done():
			return
		case ev := <-e.discCh:
			e.handleDiscovery(ev.id, ev.name, ev.addr)
		case <-evictTicker.C:
			// 30 天没出现的设备从发现列表里淡出，但不删除已配对项。
			e.discovered.EvictStale(30 * 24 * time.Hour)
			e.publish(TopicDiscovered)
		}
	}
}

func (e *Engine) handleDiscovery(id, name, addr string) {
	newPeer, newAddr := e.discovered.Upsert(id, name, addr)
	if newPeer {
		log.Printf("发现新设备: %s (%s) @ %s", name, id, addr)
	}
	// 周期性心跳但无新增/无地址变化时直接返回，避免每 5 秒刷新一次前端。
	if !newPeer && !newAddr {
		return
	}
	e.publish(TopicDiscovered)
	// 若该设备已配对，地址变化时刷新拨号目标（自动重连换 IP 后的对端）。
	if p := e.cfg.PeerByID(id); p != nil && p.Addr != addr {
		old := p.Addr
		p.Addr = addr
		_ = e.cfg.Save(e.cfgPath)
		e.dialer.UpdatePeer(old, addr, e, p)
		log.Printf("已刷新对端 %s 的地址: %s -> %s", name, old, addr)
		e.publish(TopicPeers)
	}
}

// ConnectedPeers 返回当前在线对端名列表。
func (e *Engine) ConnectedPeers() []string {
	return e.hub.names()
}

// Cfg 返回当前配置的指针（只读约定，外部不应修改）。
func (e *Engine) Cfg() *Config {
	return e.cfg
}

// IsAddrOnline 判断某对端地址是否在线（按远程 IP 匹配）。
func (e *Engine) IsAddrOnline(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return e.hub.isHostOnline(host)
}

// PairWith 向指定地址发起一次配对（作为发起方），对方同意后落盘并开始互连。
func (e *Engine) PairWith(addr string) (Peer, error) {
	addr = normalizePeer(addr)
	if !validPeer(addr) {
		return Peer{}, fmt.Errorf("地址格式应为 host:port")
	}
	if p := e.cfg.PeerByAddr(addr); p != nil {
		return *p, nil
	}
	peer, err := requestPair(e, addr, pairRequestTimeout)
	if err != nil {
		return Peer{}, err
	}
	e.addPeerRecord(peer)
	return *peer, nil
}

// PairDiscovered 向已发现设备（按 deviceID）发起配对。
func (e *Engine) PairDiscovered(deviceID string) (Peer, error) {
	dp, ok := e.discovered.Get(deviceID)
	if !ok {
		return Peer{}, fmt.Errorf("未发现该设备")
	}
	return e.PairWith(dp.Addr)
}

// PairAddr 作为发起方向 addr 发起一次配对并返回对端（不调用方运行引擎）。
// 供命令行等无完整运行环境（无需监听/拨号）的场景使用。
func PairAddr(cfg *Config, addr string, timeout time.Duration) (*Peer, error) {
	return requestPair(&Engine{cfg: cfg}, addr, timeout)
}

// addPeerRecord 把一次配对成功产生的对端写入配置并启动拨号。
func (e *Engine) addPeerRecord(peer *Peer) {
	old := e.cfg.PeerByID(peer.ID)
	e.cfg.UpsertPeer(*peer)
	_ = e.cfg.Save(e.cfgPath)
	if old != nil && old.Addr != peer.Addr && old.Addr != "" {
		e.dialer.UpdatePeer(old.Addr, peer.Addr, e, peer)
	} else {
		e.dialer.StartPeer(e, peer)
	}
	log.Printf("已配对对端: %s (%s) @ %s", peer.Name, peer.ID, peer.Addr)
	e.publish(TopicPeers)
}

// RemovePeer 删除对端并断开连接。
func (e *Engine) RemovePeer(addr string) error {
	if !e.cfg.RemovePeer(addr) {
		return fmt.Errorf("对端不存在")
	}
	if err := e.cfg.Save(e.cfgPath); err != nil {
		return err
	}
	e.dialer.StopPeer(addr, e.hub)
	log.Printf("已删除对端: %s", addr)
	e.publish(TopicPeers)
	return nil
}

// RemovePeerByName 通过名字删除对端。
func (e *Engine) RemovePeerByName(name string) error {
	p := e.cfg.PeerByName(name)
	if p == nil {
		return fmt.Errorf("名为 %s 的对端未配对", name)
	}
	return e.RemovePeer(p.Addr)
}

// Discovered 返回当前发现的设备快照（按 name 排序）。
func (e *Engine) Discovered() []DiscoveredPeer {
	return e.discovered.Snapshot()
}

// IsPaired 判断 name 是否已在已配对列表中。
func (e *Engine) IsPaired(name string) bool {
	p := e.cfg.PeerByName(name)
	return p != nil && p.ID != ""
}

// PairedNames 返回所有已配对的对端名。
func (e *Engine) PairedNames() []string {
	out := make([]string, 0, len(e.cfg.Peers))
	for _, p := range e.cfg.Peers {
		out = append(out, p.Name)
	}
	return out
}

// ScanNow 立刻广播一次本机存在，方便用户在 UI 上主动触发一次扫描。
func (e *Engine) ScanNow() {
	if e.disc != nil {
		e.disc.ScanNow()
	}
}

// ---- 入站配对请求 ----

// GetPairRequests 返回所有待确认的入站配对请求。
func (e *Engine) GetPairRequests() []PairRequest {
	e.pairMu.Lock()
	defer e.pairMu.Unlock()
	out := make([]PairRequest, 0, len(e.pendingIncoming))
	for _, pp := range e.pendingIncoming {
		out = append(out, PairRequest{
			DeviceID: pp.req.DeviceID,
			Name:     pp.req.Name,
			Addr:     net.JoinHostPort(pp.req.Host, strconv.Itoa(pp.req.Port)),
			Time:     time.Now(),
		})
	}
	return out
}

// RespondPairRequest 处理一条入站配对请求。accept 为 true 表示同意并生成共享密钥。
func (e *Engine) RespondPairRequest(deviceID string, accept bool) error {
	e.pairMu.Lock()
	pp, ok := e.pendingIncoming[deviceID]
	e.pairMu.Unlock()
	if !ok {
		return fmt.Errorf("没有来自 %s 的待处理配对请求", deviceID)
	}
	if accept {
		secret := newID()
		host := pp.req.Host
		port := pp.req.Port
		if port <= 0 || port >= 65536 {
			port = e.cfg.Port
		}
		if host == "" {
			return fmt.Errorf("无法确定对方地址")
		}
		e.addPeerRecord(&Peer{
			ID:     pp.req.DeviceID,
			Name:   pp.req.Name,
			Addr:   net.JoinHostPort(host, strconv.Itoa(port)),
			Secret: secret,
		})
		pp.ch <- pairDecision{accept: true, secret: secret}
	} else {
		pp.ch <- pairDecision{accept: false, reason: "对方拒绝了配对请求"}
	}
	return nil
}

// decideIncomingPair 由 server 侧在收到配对握手后调用，返回最终决策。
// 已配对设备直接复用既有密钥（自动恢复链路并刷新地址）；否则挂起等待用户确认。
func (e *Engine) decideIncomingPair(req IncomingPairRequest) pairDecision {
	// 已配对：直接复用既有密钥（设备已存在：双向同时发请求、或一方重置后重连）。
	if p := e.cfg.PeerByID(req.DeviceID); p != nil {
		if req.Host != "" && req.Port > 0 {
			newAddr := net.JoinHostPort(req.Host, strconv.Itoa(req.Port))
			if newAddr != p.Addr {
				old := p.Addr
				p.Addr = newAddr
				_ = e.cfg.Save(e.cfgPath)
				e.dialer.UpdatePeer(old, newAddr, e, p)
				e.publish(TopicPeers)
			}
		}
		return pairDecision{accept: true, secret: p.Secret}
	}

	e.pairMu.Lock()
	pp, ok := e.pendingIncoming[req.DeviceID]
	if !ok {
		pp = &pendingPair{req: req, ch: make(chan pairDecision, 1)}
		e.pendingIncoming[req.DeviceID] = pp
	}
	e.pairMu.Unlock()
	if !ok {
		e.publish(TopicPairRequest)
	}

	select {
	case d := <-pp.ch:
		e.pairMu.Lock()
		delete(e.pendingIncoming, req.DeviceID)
		e.pairMu.Unlock()
		return d
	case <-time.After(pairRequestTimeout):
		e.pairMu.Lock()
		delete(e.pendingIncoming, req.DeviceID)
		e.pairMu.Unlock()
		return pairDecision{accept: false, reason: "等待确认超时"}
	}
}

// SaveConfig 更新并持久化基本配置（名称/端口需重启生效）。
func (e *Engine) SaveConfig(name string, port int) error {
	if name != "" {
		e.cfg.Name = name
	}
	if port > 0 && port < 65536 {
		e.cfg.Port = port
	}
	if err := e.cfg.Save(e.cfgPath); err != nil {
		return err
	}
	e.publish(TopicConfig)
	return nil
}

// onPeerConnected 在握手成功后由 Hub 回调，更新对端↔真实远端 IP。
func (e *Engine) onPeerConnected(name, deviceID, host string) {
	if p := e.cfg.PeerByID(deviceID); p != nil {
		if h, _, err := net.SplitHostPort(p.Addr); err != nil && h != host {
			p.Addr = net.JoinHostPort(host, portOf(p.Addr))
			_ = e.cfg.Save(e.cfgPath)
		}
	} else if p := e.cfg.PeerByName(name); p != nil {
		if h, _, err := net.SplitHostPort(p.Addr); err != nil && h != host {
			p.Addr = net.JoinHostPort(host, portOf(p.Addr))
			_ = e.cfg.Save(e.cfgPath)
		}
	}
	e.publish(TopicPeers)
}

func (e *Engine) onPeerDisconnected(name, deviceID, host string) {
	// 当前仅用于日志；保留扩展点。
	_ = name
	_ = deviceID
	_ = host
	e.publish(TopicPeers)
}

func portOf(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return strconv.Itoa(defaultConfig().Port)
	}
	return port
}

func (e *Engine) onMessage(m *Message) {
	if m.Kind == KindPing {
		return
	}
	h := hashOf(m.Data)
	e.mu.Lock()
	same := h == e.lastHash
	e.mu.Unlock()
	if same {
		return
	}
	// 非回声图片：先算内容指纹，供本机 watcher 读回重编码后的图片时正确识别回声。
	var imgFP *[32]byte
	if m.Kind == KindImage {
		if fp, err := imageFP(m.Data); err == nil {
			imgFP = &fp
		}
	}
	e.mu.Lock()
	e.lastHash = h
	if imgFP != nil {
		e.lastImageFP = *imgFP
		e.hasImageFP = true
		e.lastImageWriteAt = time.Now()
	}
	e.mu.Unlock()
	// 先记录指纹再写入剪贴板，避免本机 watcher 抢先触发造成回环。
	if err := writeClipboard(m.Kind, m.Data); err != nil {
		log.Printf("写入剪贴板失败: %v", err)
		return
	}
	e.record(m)
	e.publish(TopicHistory)
	log.Printf("已接收 %s 来自 %s (%d 字节)", m.Kind, m.From, len(m.Data))
}

// CopyLocal 把历史记录内容写回本机剪贴板并广播给对端（Web UI 的"复制"按钮）。
func (e *Engine) CopyLocal(id string) error {
	item := e.findHistory(id)
	if item == nil {
		return fmt.Errorf("记录不存在")
	}
	// 图片写回时同样记录内容指纹，供本机 watcher 识别重编码后的回声。
	var imgFP *[32]byte
	if item.Kind == KindImage {
		if fp, err := imageFP(item.Data); err == nil {
			imgFP = &fp
		}
	}
	e.mu.Lock()
	e.lastHash = hashOf(item.Data)
	if imgFP != nil {
		e.lastImageFP = *imgFP
		e.hasImageFP = true
		e.lastImageWriteAt = time.Now()
	}
	e.mu.Unlock()
	if err := writeClipboard(item.Kind, item.Data); err != nil {
		return err
	}
	e.hub.Broadcast(&Message{Kind: item.Kind, From: e.cfg.Name, Data: item.Data})
	return nil
}

func (e *Engine) watchClipboard() {
	ctx := context.Background()
	chText := clipboard.Watch(ctx, clipboard.FmtText)
	chImage := clipboard.Watch(ctx, clipboard.FmtImage)
	for {
		var data []byte
		var kind MessageKind
		select {
		case data = <-chText:
			kind = KindText
		case data = <-chImage:
			kind = KindImage
		}
		if len(data) == 0 {
			continue
		}
		e.mu.Lock()
		h := hashOf(data)
		echo := h == e.lastHash
		recentWrite := kind == KindImage && e.hasImageFP && time.Since(e.lastImageWriteAt) < echoCheckWindow
		e.mu.Unlock()
		// 字节哈希不相同，可能是 macOS 已把写入的 PNG 重编码：在自写窗口内用内容指纹识别回声。
		if !echo && recentWrite {
			if fp, err := imageFP(data); err == nil {
				e.mu.Lock()
				echo = fp == e.lastImageFP
				e.mu.Unlock()
			}
		}
		e.mu.Lock()
		e.lastHash = h
		e.mu.Unlock()
		if echo {
			continue
		}
		e.hub.Broadcast(&Message{Kind: kind, From: e.cfg.Name, Data: data})
		e.record(&Message{Kind: kind, From: e.cfg.Name, Data: data})
		e.publish(TopicHistory)
		log.Printf("已发送 %s (%d 字节)", kind, len(data))
	}
}

// imageFP 计算一张图片的内容指纹：解码后按固定网格采样像素做 SHA-256。
// macOS 在「写入→读回」PNG 时可能重编码（字节不同），但像素内容不变；
// 因此用内容指纹来识别「刚由本端写入、又被 watcher 读回」的图片回声，避免重复历史与回广播。
// 网格采样把解码后的像素量限制在约 32×32=1024 个点，代价可控。
func imageFP(data []byte) ([32]byte, error) {
	var out [32]byte
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return out, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return out, fmt.Errorf("空图片")
	}
	hasher := sha256.New()
	var head [16]byte
	binary.BigEndian.PutUint32(head[0:4], uint32(w))
	binary.BigEndian.PutUint32(head[4:8], uint32(h))
	hasher.Write(head[:])
	sx := w / 32
	if sx < 1 {
		sx = 1
	}
	sy := h / 32
	if sy < 1 {
		sy = 1
	}
	var px [16]byte
	for y := b.Min.Y; y < b.Max.Y; y += sy {
		for x := b.Min.X; x < b.Max.X; x += sx {
			r, g, bl, a := img.At(x, y).RGBA()
			binary.BigEndian.PutUint32(px[0:4], r)
			binary.BigEndian.PutUint32(px[4:8], g)
			binary.BigEndian.PutUint32(px[8:12], bl)
			binary.BigEndian.PutUint32(px[12:16], a)
			hasher.Write(px[:])
		}
	}
	copy(out[:], hasher.Sum(nil))
	return out, nil
}

// ---- 剪贴板历史 ----

func (e *Engine) record(m *Message) {
	e.histMu.Lock()
	defer e.histMu.Unlock()
	e.nextSeq++
	n := len(m.Data)
	item := &HistoryItem{
		ID:   fmt.Sprintf("%d", e.nextSeq),
		Kind: m.Kind,
		From: m.From,
		Size: n,
		Time: time.Now(),
		Data: m.Data,
	}
	if m.Kind == KindText {
		item.Preview = truncateRunes(string(m.Data), 120)
	}
	// 头部插入（最新在前）：原地右移复用底层数组，避免每次分配新切片。
	if len(e.history) < historyMax {
		e.history = append(e.history, nil)
	}
	copy(e.history[1:], e.history[:len(e.history)-1])
	e.history[0] = item
	e.totalBytes += n
	e.trimHistory()
}

// trimHistory 在持有 histMu 时调用；把历史裁剪到条数与内存上限内，超限时从最旧开始丢弃。
func (e *Engine) trimHistory() {
	// 保留至少 1 条，避免单条超大内容把预算耗尽后清空整个列表。
	for len(e.history) > 1 && (len(e.history) > historyMax || e.totalBytes > maxRetainedBytes) {
		tail := e.history[len(e.history)-1]
		e.totalBytes -= tail.Size
		e.history = e.history[:len(e.history)-1]
	}
}

// truncateRunes 只解码前 n 个 rune 用于文本预览。
// 相比把整段文本转成 []rune 再截断，这样不会为超长文本分配巨大中间切片。
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size <= 1 {
			// 非法 UTF-8 字节：保留该字节，继续向后。
			i++
			continue
		}
		if count == n {
			return s[:i]
		}
		count++
		i += size
	}
	return s
}

func (e *Engine) History() []*HistoryItem {
	e.histMu.Lock()
	defer e.histMu.Unlock()
	out := make([]*HistoryItem, len(e.history))
	copy(out, e.history)
	return out
}

// HistoryData 返回指定 ID 的完整字节内容（PNG / 文本），用于前端按需加载。
func (e *Engine) HistoryData(id string) ([]byte, bool) {
	item := e.findHistory(id)
	if item == nil {
		return nil, false
	}
	return item.Data, true
}

func (e *Engine) findHistory(id string) *HistoryItem {
	e.histMu.Lock()
	defer e.histMu.Unlock()
	for _, item := range e.history {
		if item.ID == id {
			return item
		}
	}
	return nil
}
