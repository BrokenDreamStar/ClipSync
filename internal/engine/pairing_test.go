package engine

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// startTestServer 在回环地址上启动一个监听端口，返回其 host:port 与停止函数。
func startTestServer(t *testing.T, e *Engine) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	e.cfg.Port = port
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleConn(conn, e, false, nil)
		}
	}()
	return fmt.Sprintf("127.0.0.1:%d", port), func() { _ = ln.Close() }
}

// waitPending 轮询直到 engine 出现一条待确认的入站配对请求。
func waitPending(t *testing.T, e *Engine, timeout time.Duration) PairRequest {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if reqs := e.GetPairRequests(); len(reqs) > 0 {
			return reqs[0]
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("等待配对请求超时")
	return PairRequest{}
}

func TestPairingAcceptEndToEnd(t *testing.T) {
	cfgA := &Config{Name: "A", DeviceID: "idA"}
	cfgB := &Config{Name: "B", DeviceID: "idB"}
	eA := NewEngine(cfgA, "")
	eB := NewEngine(cfgB, "")
	addrA, stopA := startTestServer(t, eA)
	defer stopA()

	// B 作为发起方请求配对 A（远端运行在 goroutine，需等待 A 侧确认）。
	var peer *Peer
	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		peer, err = requestPair(eB, addrA, 5*time.Second)
	}()

	req := waitPending(t, eA, 5*time.Second)
	if req.DeviceID != "idB" || req.Name != "B" {
		t.Fatalf("收到意外请求: %+v", req)
	}
	if err := eA.RespondPairRequest(req.DeviceID, true); err != nil {
		t.Fatalf("RespondPairRequest: %v", err)
	}

	<-done
	if err != nil {
		t.Fatalf("B 发起配对失败: %v", err)
	}
	if peer.ID != "idA" || peer.Name != "A" {
		t.Fatalf("B 得到的对端身份不对: %+v", peer)
	}
	if peer.Secret == "" {
		t.Fatal("B 未拿到共享密钥")
	}
	// A 侧应已有 B 的记录，且共享密钥与 B 一致。
	pa := eA.cfg.PeerByID("idB")
	if pa == nil {
		t.Fatal("A 未记录对端 B")
	}
	if pa.Secret != peer.Secret {
		t.Fatalf("双端密钥不一致: A=%s B=%s", pa.Secret, peer.Secret)
	}
}

func TestPairingReject(t *testing.T) {
	cfgA := &Config{Name: "A", DeviceID: "idA"}
	cfgB := &Config{Name: "B", DeviceID: "idB"}
	eA := NewEngine(cfgA, "")
	eB := NewEngine(cfgB, "")
	addrA, stopA := startTestServer(t, eA)
	defer stopA()

	done := make(chan struct{})
	var err error
	go func() {
		defer close(done)
		_, err = requestPair(eB, addrA, 5*time.Second)
	}()

	req := waitPending(t, eA, 5*time.Second)
	if err := eA.RespondPairRequest(req.DeviceID, false); err != nil {
		t.Fatalf("RespondPairRequest: %v", err)
	}
	<-done
	if err == nil {
		t.Fatal("拒绝后 B 不应配对成功")
	}
	if eA.cfg.PeerByID("idB") != nil {
		t.Fatal("拒绝后 A 不应记录对端")
	}
}

func TestSyncAuthAfterPairing(t *testing.T) {
	cfgA := &Config{Name: "A", DeviceID: "idA"}
	cfgB := &Config{Name: "B", DeviceID: "idB"}
	eA := NewEngine(cfgA, "")
	eB := NewEngine(cfgB, "")
	addrA, stopA := startTestServer(t, eA)
	defer stopA()
	addrB, stopB := startTestServer(t, eB)
	defer stopB()

	// B 请求配对 A。
	done := make(chan struct{})
	var peer *Peer
	var err error
	go func() {
		defer close(done)
		peer, err = requestPair(eB, addrA, 5*time.Second)
	}()
	req := waitPending(t, eA, 5*time.Second)
	if eA.RespondPairRequest(req.DeviceID, true) != nil {
		t.Fatal("accept failed")
	}
	<-done
	if err != nil {
		t.Fatalf("pair failed: %v", err)
	}
	// B 落盘对端 A。
	cfgB.UpsertPeer(*peer)

	// 名字较小的一方（A）负责拨号；启动 A 的拨号循环连 B。
	aPeer := eA.cfg.PeerByID("idB")
	if aPeer == nil {
		t.Fatal("A 缺少对端 B")
	}
	aPeer.Addr = addrB
	eA.dialer.StartPeer(eA, aPeer)

	// 等待同步连接建立：A 与 B 都应看到对方在线。
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(eA.hub.names()) > 0 && len(eB.hub.names()) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(eA.hub.names()) == 0 {
		t.Fatalf("A 未建立同步连接, online=%v", eA.hub.names())
	}
	if len(eB.hub.names()) == 0 {
		t.Fatalf("B 未建立同步连接, online=%v", eB.hub.names())
	}
}
