//go:build windows

package updater

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Windows 产物是便携单文件 EXE：StarM-Panel 同款换名替换
//（Windows 不能覆盖运行中的 exe，但可以改名，所以「旧 .exe → .exe.old、
// 暂存 → 原 .exe」这条路走得通）。

// platformSupported：裸 EXE 任何位置都可替换。
func platformSupported() error {
	if _, err := executablePath(); err != nil {
		return errUnsupportedInstall
	}
	return nil
}

// platformInstall 把暂存的新 EXE 换名就位，返回以原路径重启的命令。
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

// verifyAssetHeader 核对暂存文件的 MZ 魔数，挡住 HTML 错误页 / 错误平台的包。
func verifyAssetHeader(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var head [2]byte
	if _, err := f.Read(head[:]); err != nil {
		return fmt.Errorf("读取安装包头失败：%w", err)
	}
	if !bytes.Equal(head[:], []byte("MZ")) {
		return fmt.Errorf("下载的文件不是 Windows 可执行格式（可能被代理或镜像替换），已放弃安装")
	}
	return nil
}

// platformCleanup 清理上次更新留下的 .exe.old 回滚点。
func platformCleanup() {
	target, err := executablePath()
	if err != nil {
		return
	}
	// 旧进程可能还没退干净，删不掉就留着下次启动再清。
	removeQuietly(target + ".old")
}

// CREATE_NO_WINDOW：powershell 子进程不闪控制台窗口。
const createNoWindow = 0x08000000

// spawnRelaunch 挂起一个 detached 的 PowerShell helper：
// 等本进程退出（引擎停完端口已释放）后再 Start-Process 新版本。
func spawnRelaunch(spec relaunchSpec) error {
	pid := os.Getpid()
	script := fmt.Sprintf(
		`while (Get-Process -Id %d -ErrorAction SilentlyContinue) { Start-Sleep -Milliseconds 300 }`,
		pid)
	if len(spec.args) > 0 {
		script += fmt.Sprintf(`; Start-Process -FilePath '%s' -ArgumentList %s`,
			psQuote(spec.cmd), psArgList(spec.args))
	} else {
		script += fmt.Sprintf(`; Start-Process -FilePath '%s'`, psQuote(spec.cmd))
	}
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
	if err := cmd.Start(); err != nil {
		return err
	}
	// 不 Wait：helper 的生命周期长于本进程。
	_ = cmd.Process.Release()
	return nil
}

// psQuote 把值包成 PowerShell 单引号串（内部 ' 双写转义）。
func psQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// psArgList 把参数拼成 -ArgumentList 的值：'a','b'。
func psArgList(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = "'" + psQuote(a) + "'"
	}
	return strings.Join(quoted, ",")
}
