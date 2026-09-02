package engine

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
)

// MessageKind distinguishes clipboard content types.
type MessageKind string

const (
	KindText  MessageKind = "text"
	KindImage MessageKind = "image" // Data 为 PNG 编码
	KindPing  MessageKind = "ping"  // 心跳，不承载内容
)

// Message is the unit exchanged between peers.
type Message struct {
	Kind MessageKind `json:"kind"`
	From string      `json:"from"`
	Data []byte      `json:"data"`
}

// 握手类型：区分「已配对后的同步连接」与「未配对时的配对通道」。
const (
	HandshakeSync = "sync"
	HandshakePair = "pair"
)

// handshake 在 TCP 建连后立即交换，用于身份确认与鉴权。
type handshake struct {
	Type     string `json:"type"`      // sync | pair
	Name     string `json:"name"`      // 本机显示名
	DeviceID string `json:"device_id"` // 本机设备身份
	Port     int    `json:"port"`      // 本机同步端口（pair 握手时携带，供对方建立回闸）
	Token    string `json:"token"`     // sync 握手：与对端之间的共享密钥
}

// pairResponse 是配对通道上「接受方」回复发起方的确认帧。
type pairResponse struct {
	Accept   bool   `json:"accept"`
	Name     string `json:"name"`      // 接受方显示名
	DeviceID string `json:"device_id"` // 接受方设备身份
	Port     int    `json:"port"`      // 接受方同步端口
	Secret   string `json:"secret,omitempty"` // accept 时携带的共享密钥
	Reason   string `json:"reason,omitempty"` // reject 时的说明
}

// writeMessage frames a message with a 4-byte big-endian length prefix.
func writeMessage(w io.Writer, m *Message) error {
	return writeJSON(w, m)
}

// writeHandshake 把握手帧写入连接。
func writeHandshake(conn net.Conn, h *handshake) error {
	return writeJSON(conn, h)
}

// writeJSON 序列化 v 并以长度前缀帧写入。
func writeJSON(w io.Writer, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeRawFrame(w, payload)
}

// writeRawFrame 写入一段已序列化的负载（4 字节长度前缀 + 负载）。
// 供广播场景复用：同一消息只需序列化一次，再向每个对端写同一份字节。
func writeRawFrame(w io.Writer, payload []byte) error {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(payload)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

// readMessage reads one length-prefixed message; returns io.EOF on clean close.
func readMessage(r *bufio.Reader) (*Message, error) {
	var m Message
	if err := readJSON(r, &m, 256<<20); err != nil { // 256MB 上限，防异常包
		return nil, err
	}
	return &m, nil
}

// readHandshake 读取握手帧。
func readHandshake(r *bufio.Reader) (*handshake, error) {
	var h handshake
	if err := readJSON(r, &h, 4096); err != nil {
		return nil, err
	}
	return &h, nil
}

// readPairResponse 读取配对确认帧。
func readPairResponse(r *bufio.Reader) (*pairResponse, error) {
	var p pairResponse
	if err := readJSON(r, &p, 4096); err != nil {
		return nil, err
	}
	return &p, nil
}

// readJSON 读取一条长度前缀帧，并用 max 字节上限防止异常大包，再反序列化到 v。
func readJSON(r *bufio.Reader, v any, max uint32) error {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return err
	}
	n := binary.BigEndian.Uint32(lenBuf[:])
	if n > max {
		return fmt.Errorf("帧过大: %d 字节", n)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return err
	}
	return json.Unmarshal(payload, v)
}
