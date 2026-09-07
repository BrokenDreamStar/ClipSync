//go:build linux

package clipboard

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	wlPaste = "wl-paste"
	wlCopy  = "wl-copy"

	// sentinelPollInterval 轮询哨兵文件 mtime 的间隔。touch 由 `wl-paste --watch`
	// 在每次 selection 变化时执行（事件驱动），轮询本身只做一次 stat 系统调用。
	sentinelPollInterval = 300 * time.Millisecond

	// execTimeout 限制每次 wl-paste / wl-copy 调用，防止合成器无响应时拖垮调用方。
	execTimeout = 3 * time.Second

	// watcherRestartMax 是 wl-paste --watch 异常退出后的最大重启间隔；
	// 连续失败（如合成器一直不在）时按指数退避，避免日志刷屏与空转。
	watcherRestartMax = 30 * time.Second
	// watcherHealthyAfter 表示上次运行超过该时长视为曾经健康，重启退避归零。
	watcherHealthyAfter = time.Minute
)

// wlSub 记录一个 Watch 订阅：内容格式、投递通道与随行 ctx。
type wlSub struct {
	format Format
	ch     chan []byte
	ctx    context.Context
}

// waylandBackend 通过 wl-clipboard 操作 Wayland 剪贴板。
//
// 变更监听不轮询剪贴板内容：由常驻的 `wl-paste --watch touch <哨兵文件>` 在每次
// selection 变化时 touch 一下哨兵文件，本地只轮询 mtime（纯 stat），变化后才真正
// 读取内容。写入经 wl-copy：其读入 stdin 后 fork 常驻进程应答粘贴请求，父进程
// 立即退出，因此写入调用不会阻塞。
type waylandBackend struct {
	mu   sync.Mutex
	subs []*wlSub
	last map[Format][]byte // 上次投递/写入的内容，用于去重（--watch 对相同内容也会触发）

	started     bool
	sentinel    string
	cancel      context.CancelFunc
	seenModTime time.Time
}

func newWaylandBackend() (backend, error) {
	for _, bin := range []string{wlPaste, wlCopy} {
		if _, err := exec.LookPath(bin); err != nil {
			return nil, fmt.Errorf("未找到 %s（wl-clipboard）: %w", bin, err)
		}
	}
	return &waylandBackend{last: map[Format][]byte{}}, nil
}

func (b *waylandBackend) Watch(ctx context.Context, f Format) <-chan []byte {
	ch := make(chan []byte, 4)
	sub := &wlSub{format: f, ch: ch, ctx: ctx}

	b.mu.Lock()
	b.subs = append(b.subs, sub)
	b.mu.Unlock()

	b.start()
	// 与 golang.design 的 Watch 语义对齐：订阅后先投递一次当前内容。
	b.inspect()

	go func() {
		<-ctx.Done()
		b.removeSub(sub)
	}()
	return ch
}

// start 启动哨兵文件的监听设施（幂等）：一个 touch 哨兵文件的 wl-paste --watch
// 子进程 + 一个轮询 mtime 的 goroutine。
func (b *waylandBackend) start() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.started {
		return
	}
	b.started = true

	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel

	f, err := os.CreateTemp("", "clipsync-clipboard-")
	if err != nil {
		// 写入仍可用，仅变更监听退化为不可用；引擎侧表现为收不到本机剪贴板变化。
		log.Printf("创建剪贴板哨兵文件失败，Wayland 变更监听不可用: %v", err)
		return
	}
	path := f.Name()
	f.Close()
	b.sentinel = path

	go b.runWatcher(ctx, path)
	go b.runPoller(ctx, path)
}

// runWatcher 常驻 `wl-paste --watch touch <sentinel>`，selection 每变化一次就
// touch 一次哨兵文件。子进程意外退出（如合成器重启）时按退避策略自动拉起。
func (b *waylandBackend) runWatcher(ctx context.Context, path string) {
	backoff := time.Second
	for {
		start := time.Now()
		cmd := exec.CommandContext(ctx, wlPaste, "--watch", "touch", path)
		err := cmd.Run()
		// 运行足够久说明连接曾经健康，下次失败从头退避。
		if time.Since(start) > watcherHealthyAfter {
			backoff = time.Second
		}
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("wl-paste --watch 异常退出，%v 后重启: %v", backoff, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > watcherRestartMax {
			backoff = watcherRestartMax
		}
		// 合成器重启后剪贴板可能已变化，拉起后立刻同步一次。
		b.inspect()
	}
}

// runPoller 轮询哨兵文件 mtime，变化才触发一次实际的剪贴板读取。
func (b *waylandBackend) runPoller(ctx context.Context, path string) {
	tick := time.NewTicker(sentinelPollInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		fi, err := os.Stat(path)
		if err != nil {
			continue
		}
		mod := fi.ModTime()
		b.mu.Lock()
		seen := b.seenModTime
		b.seenModTime = mod
		b.mu.Unlock()
		if !mod.After(seen) {
			continue
		}
		b.inspect()
	}
}

// inspect 读取当前剪贴板的类型清单，把可用且内容有变化的部分投递给订阅者。
// 剪贴板为空（无 data offer）或合成器暂不可用时静默返回。
func (b *waylandBackend) inspect() {
	mimes, err := b.listTypes()
	if err != nil {
		return
	}
	if mime := pickMime(mimes, textReadMimes); mime != "" {
		if data, err := b.read(mime); err == nil && len(data) > 0 {
			b.deliver(FmtText, data)
		}
	}
	if mime := pickMime(mimes, imageReadMimes); mime != "" {
		if data, err := b.read(mime); err == nil && len(data) > 0 {
			b.deliver(FmtImage, data)
		}
	}
}

func (b *waylandBackend) listTypes() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, wlPaste, "--list-types").Output()
	if err != nil {
		return nil, err
	}
	var mimes []string
	for _, line := range strings.Split(string(out), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			mimes = append(mimes, s)
		}
	}
	return mimes, nil
}

func (b *waylandBackend) read(mime string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()
	return exec.CommandContext(ctx, wlPaste, "--no-newline", "-t", mime).Output()
}

func (b *waylandBackend) Write(f Format, data []byte) error {
	mime := textWriteMime
	if f == FmtImage {
		mime = imageWriteMime
	}
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, wlCopy, "-t", mime)
	cmd.Stdin = bytes.NewReader(data)
	// 不接管 stdout/stderr：wl-copy 成功后会 fork 常驻进程持有剪贴板内容，
	// 该子进程若继承管道写端，Wait/CombinedOutput 会一直等管道 EOF 直到超时。
	// 继承应用自身的日志输出即可，失败靠退出码识别。
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wl-copy 写入 %s 失败: %w", mime, err)
	}
	// 先记为已投递内容：写入会触发 --watch，读回的内容与之一致时 deliver 直接跳过。
	b.remember(f, data)
	return nil
}

// deliver 把内容投递给该格式的所有订阅者；与上次相同则跳过
// （--watch 对内容未变化的 selection 变更同样会触发）。
func (b *waylandBackend) deliver(f Format, data []byte) {
	b.mu.Lock()
	if bytes.Equal(b.last[f], data) {
		b.mu.Unlock()
		return
	}
	b.last[f] = append([]byte(nil), data...)
	var targets []chan []byte
	var ctxs []context.Context
	for _, s := range b.subs {
		if s.format == f {
			targets = append(targets, s.ch)
			ctxs = append(ctxs, s.ctx)
		}
	}
	b.mu.Unlock()

	for i, ch := range targets {
		select {
		case ch <- data:
		case <-ctxs[i].Done():
		}
	}
}

func (b *waylandBackend) remember(f Format, data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.last[f] = append([]byte(nil), data...)
}

func (b *waylandBackend) removeSub(sub *wlSub) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, s := range b.subs {
		if s == sub {
			b.subs = append(b.subs[:i], b.subs[i+1:]...)
			return
		}
	}
}

// Shutdown 终止随行的 wl-paste --watch 子进程并清理哨兵文件。
// wl-copy 写入时 fork 的常驻进程刻意保留：它是剪贴板内容的持有者，
// 应用退出后内容仍可粘贴，这是 wl-clipboard 的标准行为。
func (b *waylandBackend) Shutdown() {
	b.mu.Lock()
	cancel := b.cancel
	path := b.sentinel
	b.cancel = nil
	b.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if path != "" {
		os.Remove(path)
	}
}
