package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// 清单拉取的超时预算：检查阶段不联网下载大文件，10 秒拿不到就直接失败。
const manifestTimeout = 10 * time.Second

// knownPlatforms 是 release.yml 发布矩阵覆盖的平台键（资产名后缀一致）。
// 生成脚本（release.yml 的 latest.json 步骤）与这里共用同一份命名约定。
var knownPlatforms = map[string]string{
	"macos-universal": ".dmg",
	"windows-amd64":   ".exe",
	"windows-arm64":   ".exe",
	"linux-x86_64":    ".AppImage",
	"linux-aarch64":   ".AppImage",
}

// Asset 是 latest.json 里单个平台的产物条目。
type Asset struct {
	Platform string `json:"platform"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
}

// Manifest 是随 Release 发布的更新清单（schema = 1）。
type Manifest struct {
	Schema      int     `json:"schema"`
	Tag         string  `json:"tag"`
	Version     string  `json:"version"`
	PublishedAt string  `json:"publishedAt,omitempty"`
	Assets      []Asset `json:"assets"`
}

// PlatformKey 返回当前运行平台对应的产物键（与 Release 资产名后缀一致）。
// 发布矩阵未覆盖的平台返回错误，前端据此提示「该平台没有发布包」。
func PlatformKey() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		// macOS 只发布 universal（arm64 + amd64 lipo 合并）。
		return "macos-universal", nil
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return "windows-amd64", nil
		case "arm64":
			return "windows-arm64", nil
		}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "linux-x86_64", nil
		case "arm64":
			return "linux-aarch64", nil
		}
	}
	return "", fmt.Errorf("当前平台（%s/%s）没有对应的发布包", runtime.GOOS, runtime.GOARCH)
}

// AssetFor 从清单里挑当前平台的产物；该 Release 没有对应包时返回 nil。
func (m *Manifest) AssetFor(platform string) *Asset {
	for i := range m.Assets {
		if m.Assets[i].Platform == platform {
			return &m.Assets[i]
		}
	}
	return nil
}

// DownloadURL 把清单条目拼成固定到 tag 的资产地址。
// 用 tag 而不是 /latest/：检查与下载之间再发版也不会串版本。
func (a *Asset) DownloadURL(tag string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s",
		RepoSlash, url.PathEscape(tag), url.PathEscape(a.Name))
}

// ReleaseURL 返回该 Release 的网页地址（前端用来打开发布说明）。
func ReleaseURL(tag string) string {
	return fmt.Sprintf("https://github.com/%s/releases/tag/%s", RepoSlash, url.PathEscape(tag))
}

var (
	sha256Re    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	assetNameRe = regexp.MustCompile(`^ClipSync-\d+\.\d+\.\d+-[A-Za-z0-9_.-]+$`)
)

// expectedAssetName 按发布命名约定算出平台产物的文件名。
// 清单里的 name 必须与它完全一致 —— 名字对不上就整份清单判坏，
// 否则可能出现「校验的是 A、下载的是 B」。
func expectedAssetName(version, platform string) string {
	return fmt.Sprintf("ClipSync-%s-%s%s", version, platform, knownPlatforms[platform])
}

// ParseManifest 校验并解析 latest.json。这里是更新器的信任边界：
// schema、tag 与版本一致性、平台、文件名、大小、sha256 任何一项不合法
// 都返回错误；不做「尽量猜」的容错 —— 猜测只会让坏清单变成装错文件。
// 未知平台的条目直接忽略（以后扩发布矩阵时旧客户端不该整份清单判坏）。
func ParseManifest(raw []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("latest.json 不是合法 JSON：%w", err)
	}
	if m.Schema != 1 {
		return nil, fmt.Errorf("latest.json schema 不支持（%d）", m.Schema)
	}
	if v := tagVersion(m.Tag); v == "" {
		return nil, fmt.Errorf("发布 tag 非法：%q（应形如 v0.0.5）", m.Tag)
	} else if v != m.Version {
		return nil, fmt.Errorf("清单版本不一致：tag %s 对应 %s，version 字段却是 %s", m.Tag, v, m.Version)
	}
	if len(m.Assets) == 0 {
		return nil, fmt.Errorf("latest.json 没有任何产物条目")
	}

	seen := make(map[string]bool, len(m.Assets))
	for _, a := range m.Assets {
		ext, known := knownPlatforms[a.Platform]
		if !known {
			continue // 未知平台：忽略（向前兼容）
		}
		if seen[a.Platform] {
			return nil, fmt.Errorf("平台 %s 在清单里出现两次", a.Platform)
		}
		seen[a.Platform] = true
		if !assetNameRe.MatchString(a.Name) || a.Name != expectedAssetName(m.Version, a.Platform) {
			return nil, fmt.Errorf("平台 %s 的产物名不符：预期 %s，实际 %s",
				a.Platform, expectedAssetName(m.Version, a.Platform), a.Name)
		}
		if !strings.HasSuffix(a.Name, ext) {
			return nil, fmt.Errorf("平台 %s 的产物扩展名不符：%s", a.Platform, a.Name)
		}
		if a.Size <= 0 {
			return nil, fmt.Errorf("平台 %s 的产物大小非法（%d）", a.Platform, a.Size)
		}
		if !sha256Re.MatchString(strings.ToLower(a.SHA256)) {
			return nil, fmt.Errorf("平台 %s 的 sha256 非法：%q", a.Platform, a.SHA256)
		}
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("latest.json 全部条目都是未知平台")
	}
	return &m, nil
}

// mirrorBase 返回配置的下载镜像前缀（ghproxy 约定：前缀拼接原始 URL）。
// 为空表示直连 GitHub。环境变量 CLIPSYNC_UPDATE_MIRROR。
func mirrorBase() string {
	return strings.TrimSpace(os.Getenv("CLIPSYNC_UPDATE_MIRROR"))
}

// isTrustedAssetURL 报告下载地址是否落在可信域名内。
// 正常情况是 github.com，重定向后是 *.githubusercontent.com；
// 白名单之外的地址一律拒绝，避免把更新器当成任意地址的下载器。
func isTrustedAssetURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "https" || u.User != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "github.com" || strings.HasSuffix(host, ".githubusercontent.com")
}

// netFailure 统一处理「连不上 GitHub」类的报错：具体原因（拨号超时 / 代理拒绝 /
// DNS 解析失败等）只写日志，UI 固定展示「无法连接 GitHub」——细节对用户没有
// 可操作性，反而暴露网络环境。返回的错误会原样进入前端展示，别夹带底层 err。
func netFailure(stage string, err error) error {
	log.Printf("[updater] %s 连接失败：%v", stage, err)
	return errors.New("无法连接 GitHub，请检查网络后重试")
}

// fetchManifest 拉取最新版本的 latest.json。
// 地址是 GitHub 官方支持的稳定媒体资产链接，404 说明最新发布没带清单
// （比如 0.0.4 及更早的手动发布），此时如实报错而不是误判「没有新版本」。
func fetchManifest(client *http.Client) (*Manifest, error) {
	manifestURL := fmt.Sprintf("https://github.com/%s/releases/latest/download/latest.json", RepoSlash)
	req, err := http.NewRequest(http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("ClipSync/%s", AppVersion))
	if client.Timeout == 0 {
		client.Timeout = manifestTimeout
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, netFailure("拉取更新清单", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("最新发布没有附带更新清单（latest.json），请到发布页手动下载")
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("GitHub 拒绝访问（HTTP %d），请稍后再试", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("下载 latest.json 失败（HTTP %d）", resp.StatusCode)
	}
	if !isTrustedAssetURL(resp.Request.URL.String()) {
		return nil, fmt.Errorf("更新清单地址不在允许的域名内，已中止")
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 latest.json 失败：%v", err)
	}
	return ParseManifest(raw)
}

// hashBytes 流式计算 sha256（hex 小写）。产物几十 MB，不整块读进内存。
func hashBytes(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
