//go:build darwin

package updater

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// macOS 产物是 DMG（内含 .app bundle），不能走单文件换名：
// 挂载 DMG → 找到新 .app → 旧 .app 改名 .old（运行中的进程持旧 inode，不受影响）
// → ditto 整包拷贝就位 → 卸载 DMG。新 .app 由 CI ad-hoc 签名，拷贝后签名原样保留。

// platformSupported：只有以 .app bundle 形态运行才能自更新；
// wails3 dev 的裸二进制（build/bin/ClipSync）不满足，给出明确提示。
func platformSupported() error {
	_, err := appBundlePath()
	return err
}

// appBundlePath 返回当前运行的 .app bundle 路径（…/ClipSync.app）。
// 可执行文件不在 `<名>.app/Contents/MacOS/` 布局下时返回 errUnsupportedInstall。
func appBundlePath() (string, error) {
	exe, err := executablePath()
	if err != nil {
		return "", err
	}
	bundle := filepath.Dir(filepath.Dir(exe))
	if filepath.Base(bundle) == "Contents" && strings.HasSuffix(filepath.Base(filepath.Dir(bundle)), ".app") {
		bundle = filepath.Dir(bundle)
	}
	if !strings.HasSuffix(filepath.Base(bundle), ".app") {
		return "", errUnsupportedInstall
	}
	return bundle, nil
}

// verifyAssetHeader：DMG 不是可执行文件，无魔数可查；
// 内容完整性由清单 sha256 把关（有 digest 的清单路径不需要头字节兜底）。
func verifyAssetHeader(path string) error { return nil }

// platformInstall 挂载 DMG 并用新 .app 覆盖旧 bundle。
func platformInstall(tmpPath string, asset *Asset, m *Manifest) (relaunchSpec, error) {
	bundle, err := appBundlePath()
	if err != nil {
		return relaunchSpec{}, err
	}
	appName := filepath.Base(bundle) // 与 CI 打包的 .app 同名

	mount, err := attachDMG(tmpPath)
	if err != nil {
		return relaunchSpec{}, err
	}
	defer func() {
		detachDMG(mount)
		// DMG 已用完，及时清掉几十 MB 的暂存（失败路径同样清理，不留半成品）。
		removeQuietly(tmpPath)
	}()

	src := filepath.Join(mount, appName)
	if _, err := os.Stat(src); err != nil {
		return relaunchSpec{}, fmt.Errorf("DMG 里找不到 %s，无法安装", appName)
	}

	backup := bundle + ".old"
	removeQuietly(backup)
	if err := os.Rename(bundle, backup); err != nil {
		return relaunchSpec{}, fmt.Errorf("无法移开旧版本（%v）。若应用在只读目录里，请手动更新", err)
	}
	// ditto 保留权限 / 符号链接 / 签名，比 cp -R 更适合整包拷贝 .app。
	if out, err := exec.Command("ditto", src, bundle).CombinedOutput(); err != nil {
		// 回滚，避免目录里既没有旧 .app 也没有新 .app
		_ = os.Rename(backup, bundle)
		return relaunchSpec{}, fmt.Errorf("写入新版本失败：%v（%s）", err, strings.TrimSpace(string(out)))
	}

	// 以 open 启动新 bundle：走 LaunchServices 注册（托盘 / Dock 语义完整），
	// -n 强制新实例。helper 会等本进程退出后才执行。
	return relaunchSpec{cmd: "open", args: []string{"-n", bundle}}, nil
}

// attachDMG 只读挂载 DMG，返回挂载点。
func attachDMG(dmgPath string) (string, error) {
	out, err := exec.Command("hdiutil", "attach", "-nobrowse", "-readonly", "-plist", dmgPath).Output()
	if err != nil {
		return "", fmt.Errorf("挂载安装包失败：%v", err)
	}
	mount := firstMountPoint(out)
	if mount == "" {
		return "", fmt.Errorf("无法解析挂载点（hdiutil 输出异常）")
	}
	return mount, nil
}

func detachDMG(mount string) {
	// 卸载失败不影响更新结果（卷会在进程退出后由系统回收），尽力而为。
	_ = exec.Command("hdiutil", "detach", mount, "-quiet").Run()
}

// firstMountPoint 从 `hdiutil attach -plist` 的 XML 输出里解析首个 mount-point。
// 不引第三方 plist 库：只需要「key 后跟 string」的一层字典遍历。
func firstMountPoint(plistXML []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(plistXML))
	var lastKey string
	for {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "key":
				lastKey = readCharData(dec)
			case "string":
				if lastKey == "mount-point" {
					if v := readCharData(dec); v != "" {
						return v
					}
				}
			}
		}
	}
}

func readCharData(dec *xml.Decoder) string {
	var sb strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return sb.String()
		}
		switch t := tok.(type) {
		case xml.CharData:
			sb.Write(t)
		case xml.EndElement:
			return sb.String()
		}
	}
}

// platformCleanup 清理上次更新留下的 .app.old 回滚点。
func platformCleanup() {
	bundle, err := appBundlePath()
	if err != nil {
		return
	}
	removeQuietly(bundle + ".old")
}
