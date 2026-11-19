// internal/updater — 从 GitHub Releases 自更新。
//
// 设计参考 StarM-Panel 的更新器，适配 ClipSync 的桌面应用产物形态：
//   - 更新源是 Release 附带的 latest.json 清单（CI 在 release.yml 里生成），
//     一次请求拿到 tag / 各平台产物名 / 大小 / sha256，不占 GitHub API 配额、
//     不需要 token、也不依赖网页 HTML 结构；
//   - 下载对象是三平台原生产物（DMG / EXE / AppImage）而非再包一层归档
//     —— DMG 本身 UDZO 压缩、AppImage 自带 squashfs 压缩，没有包归档的意义；
//   - 安装策略按平台分化（install_<goos>.go）：Windows / Linux 是单文件，
//     走「旧文件改名 .old、新文件就位」的换名替换；macOS 是 .app bundle，
//     挂载 DMG 后整包覆盖；
//   - 重启走「detached helper 等旧进程退出再拉起新版」：引擎监听 TCP/UDP 端口，
//     新旧进程并存会抢端口（见 StarM-Panel restart.ts 的同类教训），必须等
//     旧进程退出（OnShutdown 停引擎释放端口）之后再启动新版本。
package updater

import (
	"fmt"
	"regexp"
	"strconv"
)

// RepoSlash 是 GitHub 仓库的 owner/name，与 AboutView 展示的仓库地址一致。
const RepoSlash = "BrokenDreamStar/ClipSync"

// AppVersion 是当前应用版本号，由 main.go 启动时 SetCurrentVersion 注入
// （来源 //go:embed build/config.yml，与 Info.plist / Windows syso 同源）。
// 空串表示注入缺失，检查更新会以「无法识别当前版本」失败而不是误判。
var AppVersion string

// releaseTagRe 只接受本项目 release.yml 产出的 tag 形态：vX.Y.Z。
// 不符合格式返回空 —— 调用方据此拒绝「更新」，而不是猜语义比大小。
var releaseTagRe = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

// versionNums 把 "x.y.z" 解析成三段数字；解析失败返回 nil。
func versionNums(version string) ([3]int, error) {
	var n [3]int
	parts := splitVersion(version)
	if len(parts) != 3 {
		return n, fmt.Errorf("版本号应形如 x.y.z：%q", version)
	}
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 {
			return n, fmt.Errorf("版本号段 %q 不是非负整数", p)
		}
		n[i] = v
	}
	return n, nil
}

func splitVersion(version string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(version); i++ {
		if i == len(version) || version[i] == '.' {
			parts = append(parts, version[start:i])
			start = i + 1
		}
	}
	return parts
}

// CompareVersions 比较两个三段式版本号，返回 -1 / 0 / 1。
func CompareVersions(a, b string) int {
	na, _ := versionNums(a)
	nb, _ := versionNums(b)
	for i := 0; i < 3; i++ {
		if na[i] != nb[i] {
			if na[i] > nb[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

// tagVersion 校验发布 tag（vX.Y.Z）并返回其版本段 "X.Y.Z"；非法返回空串。
func tagVersion(tag string) string {
	m := releaseTagRe.FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	return fmt.Sprintf("%s.%s.%s", m[1], m[2], m[3])
}

// IsNewerRelease 报告 latest 是否比 current 新。
// current 为空（版本注入缺失）时保守返回 false，不触发更新。
func IsNewerRelease(latest, current string) bool {
	if latest == "" || current == "" {
		return false
	}
	return CompareVersions(latest, current) > 0
}
