package engine

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// peerConn 封装一条对端连接，并带写锁以串行化同一连接上的帧写入。
// 心跳（每 60s ping）与广播可能并发写同一连接，若不加锁会交错帧导致协议损坏。
type peerConn struct {
	conn     net.Conn
	name     string
	deviceID string
	wmu      sync.Mutex
}

// Hub 管理所有已连接的「已配对」对端，并负责广播剪贴板内容。
// 以对端 deviceID 作为连接的唯一键，同名设备也不会互相覆盖。
type Hub struct {
	mu    sync.Mutex
	conns map[string]*peerConn
	// OnConnect / OnDisconnect 在加/减连接时回调，供 Engine 维护对端↔地址映射。
	OnConnect    func(name, deviceID, host string)
	OnDisconnect func(name, deviceID, host string)
}

func NewHub() *Hub {
	return &Hub{conns: make(map[string]*peerConn)}
}

func (h *Hub) add(deviceID, name string, conn net.Conn) *peerConn {
	pc := &peerConn{conn: conn, name: name, deviceID: deviceID}
	h.mu.Lock()
	defer h.mu.Unlock()
	if old, ok := h.conns[deviceID]; ok {
		old.conn.Close()
	}
	h.conns[deviceID] = pc
	if cb := h.OnConnect; cb != nil {
		if host, _, err := net.SplitHostPort(conn.RemoteAddr().String()); err == nil {
			cb(name, deviceID, host)
		}
	}
	log.Printf("对端已连接: %s (%s)", name, conn.RemoteAddr())
	return pc
}

func (h *Hub) remove(deviceID string, pc *peerConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// 可能已被更替（重连后新连接覆盖同名），只在仍指向同一 pc 时才移除。
	if cur, ok := h.conns[deviceID]; ok && cur == pc {
		delete(h.conns, deviceID)
		if cb := h.OnDisconnect; cb != nil {
			if host, _, err := net.SplitHostPort(pc.conn.RemoteAddr().String()); err == nil {
				cb(pc.name, deviceID, host)
			}
		}
		log.Printf("对端已断开: %s", pc.name)
	}
}

// names 返回当前在线对端名列表。
func (h *Hub) names() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, 0, len(h.conns))
	for _, pc := range h.conns {
		out = append(out, pc.name)
	}
	return out
}

// isHostOnline 判断是否存在远程 IP 匹配的已连接对端。
func (h *Hub) isHostOnline(host string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, pc := range h.conns {
		if remote, _, err := net.SplitHostPort(pc.conn.RemoteAddr().String()); err == nil && remote == host {
			return true
		}
	}
	return false
}

// closeByAddr 断开远程地址匹配的对端连接（用于删除对端）。
func (h *Hub) closeByAddr(addr string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	for deviceID, pc := range h.conns {
		if remote, _, err := net.SplitHostPort(pc.conn.RemoteAddr().String()); err == nil && remote == host {
			pc.conn.Close()
			delete(h.conns, deviceID)
		}
	}
}

// Broadcast 把消息发给所有在线对端。
// 锁内只做 conns 快照，把消息序列化一次，随后在锁外对每个连接并行写入
// （每条连接用其自身的写锁串行化）。这样慢对端只会阻塞它自己的写，
// 不会拖住整个 Hub 的加/减连接、状态查询，也不会阻塞剪贴板监听。
func (h *Hub) Broadcast(m *Message) {
	payload, err := json.Marshal(m)
	if err != nil {
		log.Printf("广播序列化失败: %v", err)
		return
	}
	h.mu.Lock()
	conns := make(map[string]*peerConn, len(h.conns))
	for deviceID, pc := range h.conns {
		conns[deviceID] = pc
	}
	h.mu.Unlock()

	var wg sync.WaitGroup
	for deviceID, pc := range conns {
		wg.Add(1)
		go func(deviceID string, pc *peerConn) {
			defer wg.Done()
			pc.wmu.Lock()
			defer pc.wmu.Unlock()
			pc.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := writeRawFrame(pc.conn, payload); err != nil {
				log.Printf("发送到 %s 失败: %v", pc.name, err)
				pc.conn.Close()
				h.remove(deviceID, pc)
			}
		}(deviceID, pc)
	}
	wg.Wait()
}

// StartServer 监听端口，接收对端连接。每条连接在独立 goroutine 中处理。
func StartServer(e *Engine) {
	addr := fmt.Sprintf(":%d", e.cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("监听 %s 失败: %v", addr, err)
	}
	log.Printf("服务端已监听 %s", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}
		go handleConn(conn, e, false, nil)
	}
}

// handleConn 处理一条连接：先区分是「配对」还是「同步」，再走各自逻辑。
// outgoing 表示这是本机主动拨号的连接；peer 在主动拨号时携带本机侧的对端信息。
func handleConn(conn net.Conn, e *Engine, outgoing bool, peer *Peer) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(conn)

	// 主动方先发握手，被动方先读后发。
	var hs *handshake
	if outgoing {
		if err := writeHandshake(conn, &handshake{
			Type:     HandshakeSync,
			Name:     e.cfg.Name,
			DeviceID: e.cfg.DeviceID,
			Token:    peerSecret(peer),
		}); err != nil {
			log.Printf("发送握手失败: %v", err)
			return
		}
		var err error
		hs, err = readHandshake(reader)
		if err != nil {
			log.Printf("读取对端握手失败: %v", err)
			return
		}
	} else {
		var err error
		hs, err = readHandshake(reader)
		if err != nil {
			log.Printf("读取对端握手失败: %v", err)
			return
		}
	}

	// 配对请求走专用通道：不要求已配对，由接收方弹窗确认。
	// 注意：这里不再回声握手，回复由 handlePairConn（写 reply 握手 + 等确认）统一完成，
	// 否则对方会读到两个握手，把 reply 握手误当成确认帧。
	if hs.Type == HandshakePair {
		e.handlePairConn(conn, reader, hs, outgoing)
		return
	}

	// 同步连接：凭「对端身份 + 共享密钥」校验。
	remoteID := hs.DeviceID
	remoteName := hs.Name
	peerRec := e.cfg.PeerByID(remoteID)
	if peerRec == nil {
		// 兼容旧数据：按名字回退。
		peerRec = e.cfg.PeerByName(remoteName)
	}
	if peerRec == nil || peerSecret(peerRec) == "" || peerSecret(peerRec) != hs.Token {
		log.Printf("对端 %s (%s) 鉴权失败，拒绝连接", remoteName, conn.RemoteAddr())
		return
	}

	// 被动方在校验通过后回声 sync 握手，携带同样的共享密钥，让拨号方也能确认对端身份。
	if !outgoing {
		if err := writeHandshake(conn, &handshake{
			Type:     HandshakeSync,
			Name:     e.cfg.Name,
			DeviceID: e.cfg.DeviceID,
			Token:    peerRec.Secret,
		}); err != nil {
			log.Printf("发送握手失败: %v", err)
			return
		}
	}

	// 双方都会互相拨号，按名字仲裁只保留一条连接，避免连接抖动：
	// 名字较小的一方负责拨出（outgoing），另一方只被动接受。
	if (e.cfg.Name < remoteName) != outgoing {
		log.Printf("丢弃与 %s 的冗余连接", remoteName)
		return
	}
	conn.SetDeadline(time.Time{}) // 清除超时，进入长连接
	pc := e.hub.add(remoteID, remoteName, conn)

	// 心跳：每 60 秒发一次 ping，配合读超时剔除死连接。
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			pc.wmu.Lock()
			pc.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := writeMessage(pc.conn, &Message{Kind: KindPing, From: e.cfg.Name})
			pc.wmu.Unlock()
			if err != nil {
				return
			}
		}
	}()

	for {
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute)) // 空闲超时防僵死
		msg, err := readMessage(reader)
		if err != nil {
			e.hub.remove(remoteID, pc)
			return
		}
		if msg.Kind == KindPing {
			continue
		}
		conn.SetReadDeadline(time.Time{})
		e.onMessage(msg)
	}
}

func peerSecret(p *Peer) string {
	if p == nil {
		return ""
	}
	return p.Secret
}

// handlePairConn 处理配对通道（接收方视角）。
// 对方已发出 pair 握手（hs 即对方的身份），这里回复本方身份并等待接收方决策，
// 然后把 accept/reject 写回。接受时由 Engine 负责落盘对端并启动拨号。
func (e *Engine) handlePairConn(conn net.Conn, reader *bufio.Reader, hs *handshake, outgoing bool) {
	// 只处理「被动接受」一侧：主动拨号的配对在 client.go 的 requestPair 里完成。
	_ = outgoing
	// 回复本方身份，让对方确认找对了设备。
	if err := writeHandshake(conn, &handshake{
		Type:     HandshakePair,
		Name:     e.cfg.Name,
		DeviceID: e.cfg.DeviceID,
		Port:     e.cfg.Port,
	}); err != nil {
		log.Printf("回复配对握手失败: %v", err)
		return
	}

	req := IncomingPairRequest{
		DeviceID: hs.DeviceID,
		Name:     hs.Name,
		Port:     hs.Port,
	}
	if host, _, err := net.SplitHostPort(conn.RemoteAddr().String()); err == nil {
		req.Host = host
	}

	decision := e.decideIncomingPair(req)

	resp := pairResponse{
		Accept:   decision.accept,
		Name:     e.cfg.Name,
		DeviceID: e.cfg.DeviceID,
		Port:     e.cfg.Port,
		Secret:   decision.secret,
		Reason:   decision.reason,
	}
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	if err := writeJSON(conn, &resp); err != nil {
		log.Printf("写配对确认失败: %v", err)
	}
}
