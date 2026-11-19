package updater

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// relaunchSpec 是新版本的启动命令，由各平台 platformInstall 返回，
// spawnRelaunch 在旧进程退出后执行它。
type relaunchSpec struct {
	cmd  string
	args []string
}

// platformInstall 是各平台（install_<goos>.go）的安装入口约定：
// 把已下载并校验过的产物装到运行位置，返回重启命令。
// tmpPath 指向暂存文件；安装成功后暂存路径不再存在（已换名 / 已清理）。
//
// 头部魔数校验（verifyAssetHeader）与残留清理（platformCleanup）
// 也在各平台文件里实现 —— 都强依赖平台产物格式。
//
// 新增平台函数时必须同步补齐其他平台的实现（见 AGENTS.md「已知坑」）。

// swapInPlace 用下载好的文件替换目标文件（单文件产物：Windows EXE / Linux AppImage）。
//
// 顺序：旧文件 → `<目标>.old`，暂存文件 → 目标。第二条改名失败时回滚，
// 保证不会出现「旧文件没了、新文件也没就位」的空窗。
// 目标正在运行不影响改名（POSIX 与 Windows 都允许，Windows 不能覆盖
// 运行中的 exe 但可以改名）。.old 保留到新版本启动成功后由 platformCleanup 清理
// —— 新版本起不来时它就是回滚点。
func swapInPlace(tmpPath, targetPath string) error {
	mode := os.FileMode(0o755)
	if fi, err := os.Stat(targetPath); err == nil {
		mode = fi.Mode().Perm()
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		// Windows 上 chmod 只影响只读位，失败也无妨（见下：0o755 已默认设置）
		if fi, statErr := os.Stat(tmpPath); statErr != nil || fi.Mode().Perm()&0o100 == 0 {
			return fmt.Errorf("无法设置安装包可执行位：%w", err)
		}
	}

	backup := targetPath + ".old"
	removeQuietly(backup)
	if err := os.Rename(targetPath, backup); err != nil {
		return fmt.Errorf("无法替换程序文件（%s）：%v。请检查运行用户对该目录的写权限", targetPath, err)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		// 回滚，避免目录里既没有原程序也没有新程序
		_ = os.Rename(backup, targetPath)
		return fmt.Errorf("写入新版本失败：%v", err)
	}
	return nil
}

// executablePath 返回当前可执行文件的绝对路径。
func executablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if abs, err := filepath.Abs(exe); err == nil {
		exe = abs
	}
	return exe, nil
}

// errUnsupportedInstall 是平台安装器不可用时的统一口径。
var errUnsupportedInstall = errors.New("当前运行方式不支持自更新，请到发布页手动下载")
