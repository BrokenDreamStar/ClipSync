// Package clipboard 提供跨平台剪贴板读写与变更监听。
//
// Linux 上按运行环境在 Init 时自动选择后端：优先 X11（golang.design/x/clipboard，
// Wayland 会话下经 XWayland 亦可用），X11 不可用（纯 Wayland 会话）时回退到
// wl-clipboard（wl-copy / wl-paste）。macOS / Windows 直接使用 golang.design/x/clipboard。
package clipboard

import (
	"context"
	"errors"
	"log"
	"sync"
)

// Format 区分剪贴板内容类型。数值与 golang.design/x/clipboard 的
// FmtText / FmtImage 保持一致（均为 0/1），便于直接转换。
type Format int

const (
	FmtText Format = iota
	FmtImage
)

func (f Format) String() string {
	if f == FmtImage {
		return "image"
	}
	return "text"
}

// errNoBackend 汇总两路后端都初始化失败时的提示。
var errNoBackend = errors.New("X11 与 Wayland 剪贴板均不可用：纯 Wayland 会话需安装 wl-clipboard（wl-copy / wl-paste），或在启用 XWayland 的环境下运行")

// backend 是具体平台的剪贴板实现。
type backend interface {
	Watch(ctx context.Context, f Format) <-chan []byte
	Write(f Format, data []byte) error
}

var (
	mu      sync.Mutex
	chosen  backend
	initErr error
)

// Init 选择并初始化平台剪贴板后端，必须在 Watch / Write 之前调用；幂等。
func Init() error {
	mu.Lock()
	defer mu.Unlock()
	ensureBackendLocked()
	return initErr
}

// ensureBackendLocked 惰性初始化，兜底未显式调用 Init 的场景；失败时返回 nil。
func ensureBackendLocked() backend {
	if chosen == nil && initErr == nil {
		chosen, initErr = selectBackend()
	}
	return chosen
}

// Watch 返回一个通道：剪贴板中 f 类型的内容每次变化时收到最新数据
// （订阅后首条为当前内容，语义与 golang.design/x/clipboard 一致）。
// 后端不可用时返回 nil 通道——select 会永久阻塞在该 case 上，调用方自然挂起，
// 不会像关闭的通道那样造成忙循环。
func Watch(ctx context.Context, f Format) <-chan []byte {
	mu.Lock()
	defer mu.Unlock()
	b := ensureBackendLocked()
	if b == nil {
		log.Printf("剪贴板后端不可用，%s 监听未启动: %v", f, initErr)
		return nil
	}
	return b.Watch(ctx, f)
}

// Write 把 data 以 f 类型写入系统剪贴板。
func Write(f Format, data []byte) error {
	mu.Lock()
	defer mu.Unlock()
	b := ensureBackendLocked()
	if b == nil {
		return initErr
	}
	return b.Write(f, data)
}

// Shutdown 释放后端持有的系统资源（如 Wayland 下随行监听的子进程），
// 应在进程退出前调用。幂等。
func Shutdown() {
	mu.Lock()
	defer mu.Unlock()
	if s, ok := chosen.(interface{ Shutdown() }); ok {
		s.Shutdown()
	}
}
