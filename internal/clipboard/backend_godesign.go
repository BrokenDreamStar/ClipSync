package clipboard

import (
	"context"

	"golang.design/x/clipboard"
)

// godesignBackend 包装 golang.design/x/clipboard：
// macOS / Windows 的原生实现，以及 Linux 上的 X11 实现。
type godesignBackend struct{}

func newGoDesignBackend() (backend, error) {
	if err := clipboard.Init(); err != nil {
		return nil, err
	}
	return godesignBackend{}, nil
}

func (godesignBackend) Watch(ctx context.Context, f Format) <-chan []byte {
	return clipboard.Watch(ctx, clipboard.Format(f))
}

func (godesignBackend) Write(f Format, data []byte) error {
	// golang.design 的 Write 不返回错误（写入失败静默），保持接口一致即可。
	clipboard.Write(clipboard.Format(f), data)
	return nil
}
