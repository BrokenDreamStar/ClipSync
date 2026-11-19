//go:build darwin || linux

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// spawnRelaunch 挂起一个 detached 的 sh helper：等本进程退出后再拉起新版本。
//
// 为什么必须等：引擎监听 TCP / UDP 端口，新旧进程并存会抢端口
// （StarM-Panel restart.ts 的同类教训）；旧进程退出前 OnShutdown 已停引擎，
// 此时端口已释放，新进程才能顺利 bind。
//
// helper 用 kill -0 轮询 PID（0.3s 一次）；本进程退出后由它 exec 新版本，
// 期间不需要任何外部依赖 —— sh 在 macOS 与发行版上都必在。
func spawnRelaunch(spec relaunchSpec) error {
	pid := os.Getpid()
	script := fmt.Sprintf("while kill -0 %d 2>/dev/null; do sleep 0.3; done; exec %s",
		pid, shellJoin(spec.cmd, spec.args))
	cmd := exec.Command("/bin/sh", "-c", script)
	// 自成新会话：父进程（本应用）退出不会把 helper 一起带走；
	// stdio 置空（/dev/null），不继承终端与管道。
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	// 不 Wait：helper 的生命周期长于本进程。
	_ = cmd.Process.Release()
	return nil
}

// shellQuote 把一段文本包成 sh 单引号串（内部的 ' 转义成 '\”）。
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shellJoin 把命令与参数拼成可安全放进 sh 脚本的命令串。
func shellJoin(cmd string, args []string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, shellQuote(cmd))
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ")
}
