//go:build linux

package clipboard

// selectBackend 在 Linux 上按优先级选择剪贴板后端：
//
//  1. X11（golang.design/x/clipboard）——Wayland 会话下经 XWayland 亦可用，
//     是既有主路径（变更监听为事件驱动，无需外部依赖）；
//  2. wl-clipboard（wl-copy / wl-paste）——纯 Wayland 会话（未启用 XWayland）
//     下的原生路径，需系统或 AppImage 提供 wl-clipboard。
func selectBackend() (backend, error) {
	if b, err := newGoDesignBackend(); err == nil {
		return b, nil
	}
	if b, err := newWaylandBackend(); err == nil {
		return b, nil
	}
	return nil, errNoBackend
}
