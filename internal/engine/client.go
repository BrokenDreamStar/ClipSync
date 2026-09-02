package engine

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// Dialer 维护到各已配对对端的常驻连接，断线自动重连，支持动态增删对端。
type Dialer struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc // 地址 -> 重连循环的取消函数
}

func NewDialer() *Dialer {
	return &Dialer{cancels: make(map[string]context.CancelFunc)}
}

// StartPeer 为一个对端启动重连循环（已存在则忽略）。拨号时携带该对端的专属 secret。
func (d *Dialer) StartPeer(e *Engine, peer *Peer) {
	if peer == nil || peer.Addr == "" {
		return
	}
	addr := peer.Addr
	d.mu.Lock()
	if _, ok := d.cancels[addr]; ok {
		d.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.cancels[addr] = cancel
	d.mu.Unlock()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
			if err == nil {
				handleConn(conn, e, true, peer)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}()
}

// StopPeer 停止某个对端的重连循环并断开其连接。
func (d *Dialer) StopPeer(addr string, hub *Hub) {
	d.mu.Lock()
	cancel, ok := d.cancels[addr]
	if ok {
		delete(d.cancels, addr)
	}
	d.mu.Unlock()
	if ok {
		cancel()
		hub.closeByAddr(addr)
	}
}

// UpdatePeer 用新地址替换旧地址的重连循环；如果旧地址不存在则按新地址启动。
// 用于局域网内对端 IP 变化（DHCP/Wi-Fi 切换）时刷新拨号目标。
func (d *Dialer) UpdatePeer(oldAddr, newAddr string, e *Engine, peer *Peer) {
	if oldAddr == newAddr {
		return
	}
	if oldAddr != "" {
		d.StopPeer(oldAddr, e.hub)
	}
	if newAddr != "" {
		d.StartPeer(e, peer)
	}
}

// StopAll 停止所有重连循环（用于程序退出）。
func (d *Dialer) StopAll(hub *Hub) {
	d.mu.Lock()
	addrs := make([]string, 0, len(d.cancels))
	for addr := range d.cancels {
		addrs = append(addrs, addr)
	}
	d.mu.Unlock()
	for _, addr := range addrs {
		d.StopPeer(addr, hub)
	}
}

// requestPair 作为「发起方」向 addr 发起一次配对请求并等待对方确认。
// 成功返回对端 Peer（含共享 secret），失败返回错误（拒绝/超时/非 ClipSync 设备）。
func requestPair(e *Engine, addr string, timeout time.Duration) (*Peer, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("连接 %s 失败: %w", addr, err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))

	// 统一用一个 reader 读取握手与确认，避免第二个 reader 读不到前一个缓冲的字节。
	reader := bufio.NewReader(conn)

	// 发送本方配对握手（携带身份与本机同步端口）。
	if err := writeHandshake(conn, &handshake{
		Type:     HandshakePair,
		Name:     e.cfg.Name,
		DeviceID: e.cfg.DeviceID,
		Port:     e.cfg.Port,
	}); err != nil {
		return nil, fmt.Errorf("发送配对请求失败: %w", err)
	}

	// 读取对方回复的配对握手，确认是 ClipSync 设备。
	hs, err := readHandshake(reader)
	if err != nil {
		return nil, fmt.Errorf("读取对端响应失败: %w", err)
	}
	if hs.Type != HandshakePair {
		return nil, fmt.Errorf("对端 %s 未启用配对协议", addr)
	}

	// 读取对方确认（等待用户在远端点击同意/拒绝）。
	resp, err := readPairResponse(reader)
	if err != nil {
		return nil, fmt.Errorf("读取配对结果失败: %w", err)
	}
	if !resp.Accept {
		reason := resp.Reason
		if reason == "" {
			reason = "对方拒绝了配对请求"
		}
		return nil, fmt.Errorf("%s", reason)
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	port := resp.Port
	if port <= 0 || port >= 65536 {
		port = e.cfg.Port
	}
	return &Peer{
		ID:     resp.DeviceID,
		Name:   resp.Name,
		Addr:   net.JoinHostPort(host, fmt.Sprintf("%d", port)),
		Secret: resp.Secret,
	}, nil
}
