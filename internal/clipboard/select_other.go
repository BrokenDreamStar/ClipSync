//go:build !linux

package clipboard

// 非 Linux 平台只有一种实现：golang.design/x/clipboard。
func selectBackend() (backend, error) {
	return newGoDesignBackend()
}
