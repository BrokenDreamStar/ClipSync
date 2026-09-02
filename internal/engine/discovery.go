package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sort"
	"sync"
	"syscall"
	"time"
)

// Discovery 在局域网内周期性广播本机存在并监听其他 ClipSync 实例的宣告。
// 同一链路层域内的所有 ClipSync 实例（无论身份）都会互相可见，供用户手动配对。
type Discovery struct {
	cfg    *Config
	port   int
	period time.Duration

	outConn net.Conn       // UDP 广播套接字（无需关闭监听端口，listener 关即可）
	listen  *net.UDPConn   // UDP 监听套接字
	ownIPs  map[string]bool // 本机所有非 loopback IPv4，用于过滤自收
}

// NewDiscovery 构造发现器；端口必须与现有 TCP 同步端口区分开。
func NewDiscovery(cfg *Config, port int) *Discovery {
	return &Discovery{
		cfg:    cfg,
		port:   port,
		period: 5 * time.Second,
		ownIPs: collectOwnIPs(),
	}
}

// collectOwnIPs 列出本机所有非 loopback IPv4 地址，用于过滤自己发出的广播包。
func collectOwnIPs() map[string]bool {
	out := map[string]bool{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if v4 := ip.To4(); v4 != nil {
				out[v4.String()] = true
			}
		}
	}
	return out
}

// Announce 是 UDP 广播中携带的宣告消息。DeviceID 用于区分不同的 ClipSync 实例。
type Announce struct {
	Ver      int    `json:"ver"`
	Name     string `json:"name"`
	Port     int    `json:"port"`
	DeviceID string `json:"device_id"`
}

// ScanNow 立即触发一次广播，便于用户在 UI 上手动刷新「发现的设备」列表。
func (d *Discovery) ScanNow() {
	d.broadcast()
}

// Run 启动广播+监听循环；ctx 取消时退出。
func (d *Discovery) Run(ctx context.Context, onSeen func(remoteAddr *net.UDPAddr, a Announce)) error {
	dialer := net.Dialer{}
	out, err := dialer.DialContext(ctx, "udp", fmt.Sprintf("255.255.255.255:%d", d.port))
	if err != nil {
		return fmt.Errorf("打开广播套接字失败: %w", err)
	}
	if c, ok := out.(*net.UDPConn); ok {
		_ = c.SetWriteBuffer(4096)
	}
	d.outConn = out
	defer out.Close()

	addr := &net.UDPAddr{IP: net.IPv4zero, Port: d.port}
	lc := net.ListenConfig{Control: func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			// SO_REUSEADDR 允许同一台机器重启 ClipSync 时不需等 TIME_WAIT。
			// 注意：Windows 上 syscall.SetsockoptInt 期望 Handle 而非 int。
			_ = setSockoptReuseAddr(fd)
		})
	}}
	pc, err := lc.ListenPacket(ctx, "udp4", addr.String())
	if err != nil {
		return fmt.Errorf("监听发现端口 %d 失败: %w", d.port, err)
	}
	ln, ok := pc.(*net.UDPConn)
	if !ok {
		pc.Close()
		return fmt.Errorf("监听发现端口 %d 返回非 UDP 套接字", d.port)
	}
	d.listen = ln
	defer ln.Close()
	log.Printf("局域网发现已启用：UDP %d（每 %s 广播一次）", d.port, d.period)

	go d.broadcastLoop(ctx)

	buf := make([]byte, 1500)
	for {
		ln.SetReadDeadline(time.Now().Add(2 * time.Second))
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		n, from, err := ln.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			log.Printf("discovery read: %v", err)
			continue
		}
		var a Announce
		if err := json.Unmarshal(buf[:n], &a); err != nil {
			continue
		}
		if a.Ver != 1 {
			continue
		}
		if a.DeviceID == d.cfg.DeviceID {
			// 本机自己的宣告（可能经多网卡回环）或同一台机器的另一进程；丢弃。
			continue
		}
		if d.ownIPs[from.IP.String()] {
			continue
		}
		onSeen(from, a)
	}
}

func (d *Discovery) broadcastLoop(ctx context.Context) {
	t := time.NewTicker(d.period)
	defer t.Stop()
	// 立即广播一次，让对方更快看到我们。
	d.broadcast()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			d.broadcast()
		}
	}
}

func (d *Discovery) broadcast() {
	payload, err := json.Marshal(&Announce{
		Ver:      1,
		Name:     d.cfg.Name,
		Port:     d.cfg.Port,
		DeviceID: d.cfg.DeviceID,
	})
	if err != nil {
		return
	}
	// 主路径：逐网卡向每个接口的子网定向广播地址发送。
	// 相比只发 255.255.255.255（受限全局广播），子网定向广播会绑定到具体网卡，
	// 内核把包交给指定网卡发出，避免多网卡 / VPN(utun) 环境下全局广播被默认路由
	// 截走到错误出口，导致同网段却互扫不到。
	for _, t := range d.broadcastTargets() {
		d.sendTo(t.local, t.bcast, payload)
	}
	// 兜底：受限全局广播。某些环境（单网卡且路由正常）仍依赖它。
	if d.outConn == nil {
		return
	}
	_ = d.outConn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, _ = d.outConn.Write(payload)
}

// bcastTarget 记录一块网卡的本地 IPv4 与其子网定向广播地址。
type bcastTarget struct{ local, bcast net.IP }

// broadcastTargets 枚举所有 UP、非回环、带广播能力的 IPv4 网卡，
// 返回每块网卡的 (本机地址, 子网广播地址) 对。
// 点对点网卡（如 utun/WireGuard，没有广播地址）与 /32 单播地址会被跳过。
func (d *Discovery) broadcastTargets() []bcastTarget {
	var out []bcastTarget
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagBroadcast == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			n, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := n.IP.To4()
			if ip4 == nil {
				continue
			}
			mask := n.Mask
			if len(mask) != 4 {
				continue
			}
			// 掩码全 1（/32）没有广播地址，跳过。
			if mask[0] == 0xff && mask[1] == 0xff && mask[2] == 0xff && mask[3] == 0xff {
				continue
			}
			bc := make(net.IP, 4)
			for i := 0; i < 4; i++ {
				bc[i] = ip4[i] | ^mask[i]
			}
			out = append(out, bcastTarget{local: ip4, bcast: bc})
		}
	}
	return out
}

// sendTo 以 local 为源地址，向 dst:port 发送一条 UDP 广播报文。
func (d *Discovery) sendTo(local, dst net.IP, payload []byte) {
	if local == nil || dst == nil {
		return
	}
	conn, err := net.DialUDP("udp4", &net.UDPAddr{IP: local, Port: 0}, &net.UDPAddr{IP: dst, Port: d.port})
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, _ = conn.Write(payload)
}

// ------------------------------------------------------------------
// DiscoveredPeerSet：维护一份「见过的对端」快照，供 Engine 和 UI 使用。
// ------------------------------------------------------------------

// DiscoveredPeer 是 UI 上展示的一条发现记录，key 为设备唯一身份 DeviceID。
type DiscoveredPeer struct {
	DeviceID string    `json:"device_id"`
	Name     string    `json:"name"`
	Addr     string    `json:"addr"`
	LastSeen time.Time `json:"last_seen"`
}

// DiscoveredPeerSet 线程安全地保存发现到的对端。
type DiscoveredPeerSet struct {
	mu    sync.Mutex
	peers map[string]*DiscoveredPeer // key = device_id
	seen  map[string]bool            // key = device_id+addr，抑制重复事件
}

// NewDiscoveredPeerSet 构造一个空集合。
func NewDiscoveredPeerSet() *DiscoveredPeerSet {
	return &DiscoveredPeerSet{
		peers: make(map[string]*DiscoveredPeer),
		seen:  make(map[string]bool),
	}
}

// Upsert 记录一次发现；如果设备是新增的或地址有变化则分别返回 true。
func (s *DiscoveredPeerSet) Upsert(id, name, addr string) (newPeer bool, newAddr bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.peers[id]
	dedupKey := id + "@" + addr
	if !ok {
		s.peers[id] = &DiscoveredPeer{DeviceID: id, Name: name, Addr: addr, LastSeen: time.Now()}
		s.seen[dedupKey] = true
		return true, true
	}
	p.LastSeen = time.Now()
	if name != "" {
		p.Name = name
	}
	if p.Addr != addr {
		p.Addr = addr
		s.seen[dedupKey] = true
		newAddr = true
	}
	if !s.seen[dedupKey] {
		s.seen[dedupKey] = true
	}
	return false, newAddr
}

// Get 返回指定 deviceID 的最新记录。
func (s *DiscoveredPeerSet) Get(id string) (DiscoveredPeer, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.peers[id]
	if !ok {
		return DiscoveredPeer{}, false
	}
	return *p, true
}

// Snapshot 返回所有已发现对端的快照，按 name 排序。
func (s *DiscoveredPeerSet) Snapshot() []DiscoveredPeer {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]DiscoveredPeer, 0, len(s.peers))
	for _, p := range s.peers {
		out = append(out, *p)
	}
	sortPeersByName(out)
	return out
}

// EvictStale 把超过 ttl 没见过的设备从集合里删除，返回被移除的 name 列表。
func (s *DiscoveredPeerSet) EvictStale(ttl time.Duration) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var removed []string
	now := time.Now()
	for name, p := range s.peers {
		if now.Sub(p.LastSeen) > ttl {
			delete(s.peers, name)
			removed = append(removed, name)
		}
	}
	return removed
}

func sortPeersByName(peers []DiscoveredPeer) {
	sort.Slice(peers, func(i, j int) bool { return peers[i].Name < peers[j].Name })
}

// setSockoptReuseAddr 跨平台设置 SO_REUSEADDR（具体实现见 discovery_unix.go / discovery_windows.go）。
func setSockoptReuseAddr(fd uintptr) error {
	return setSockoptReuseAddrImpl(fd)
}