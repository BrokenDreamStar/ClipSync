//go:build linux

package updater

import (
	"bytes"
	"fmt"
	"os"
)

// Linux 产物是单文件 AppImage（或裸二进制）：与 Windows 同款换名替换。
// 运行中的 AppImage 持有的是旧 inode 的 squashfs 挂载，替换磁盘文件互不影响；
// 新版本以同一路径启动，XDG autostart 里记录的 $APPIMAGE 路径继续有效。

// platformSupported 报告当前运行方式能否自更新。
func platformSupported() error {
	if _, err := executablePath(); err != nil {
		return errUnsupportedInstall
	}
	return nil
}

// platformInstall 把暂存的 AppImage 换名就位，返回以原路径重启的命令。
func platformInstall(tmpPath string, asset *Asset, m *Manifest) (relaunchSpec, error) {
	target, err := executablePath()
	if err != nil {
		return relaunchSpec{}, err
	}
	if err := swapInPlace(tmpPath, target); err != nil {
		return relaunchSpec{}, err
	}
	return relaunchSpec{cmd: target, args: os.Args[1:]}, nil
}

// verifyAssetHeader 核对暂存文件的 ELF 魔数。
// sha256 已经校验过内容，这里再挡一道「下载到的是错误平台的文件」。
func verifyAssetHeader(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var head [4]byte
	if _, err := f.Read(head[:]); err != nil {
		return fmt.Errorf("读取安装包头失败：%w", err)
	}
	if !bytes.Equal(head[:], []byte("\x7fELF")) {
		return fmt.Errorf("下载的文件不是 Linux 可执行格式（可能被代理或镜像替换），已放弃安装")
	}
	return nil
}

// platformCleanup 清理上次更新留下的 .old 回滚点。
func platformCleanup() {
	target, err := executablePath()
	if err != nil {
		return
	}
	// 删不掉就算了（旧进程可能还没退干净），下次启动再清。
	removeQuietly(target + ".old")
}
