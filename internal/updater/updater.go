package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// 更新流程的阶段。前端据此渲染进度条与阶段文案；
// 替换完成后会触发退出重启，因此不会出现 done，最后一个可观察阶段是 restarting。
const (
	PhaseIdle        = "idle"
	PhaseVerifying   = "verifying"
	PhaseInstalling  = "installing"
	PhaseRestarting  = "restarting"
	PhaseDownloading = "downloading"
	PhaseError       = "error"
)

// State 是进行中更新的进度快照（前端轮询 / 订阅事件均可拿到）。
type State struct {
	Phase    string `json:"phase"`
	Tag      string `json:"tag"`
	Received int64  `json:"received"`
	Total    *int64 `json:"total"`
	Error    string `json:"error"`
}

// CurrentInfo 描述当前安装的版本。
type CurrentInfo struct {
	Version   string `json:"version"`
	Platform  string `json:"platform"`
	Supported bool   `json:"supported"`
}

// LatestInfo 是最新 Release 的基本信息（不含下载地址，外链只留在后端）。
type LatestInfo struct {
	Tag         string `json:"tag"`
	Version     string `json:"version"`
	PublishedAt string `json:"publishedAt"`
	HTMLURL     string `json:"htmlUrl"`
	AssetName   string `json:"assetName"`
	AssetSize   int64  `json:"assetSize"`
}

// CheckResult 是一次检查更新的结果。Error 非空表示检查失败
// （连不上 GitHub 等外部状态），接口本身不报错，前端原样展示原因。
type CheckResult struct {
	Current   CurrentInfo `json:"current"`
	Latest    *LatestInfo `json:"latest"`
	HasUpdate bool        `json:"hasUpdate"`
	Error     string      `json:"error"`
	CheckedAt int64       `json:"checkedAt"`
}

// 下载超时：几十 MB 的 DMG 在慢速网络上也要给足时间。
const downloadTimeout = 15 * time.Minute

// 检查结果缓存时长：避免反复点「检查更新」时重复拉清单。
const checkCacheTTL = 60 * time.Second

// 进度事件节流：下载中至少间隔这么久才 emit 一次，避免事件风暴。
const progressEmitInterval = 200 * time.Millisecond

// activePhases 是「已有更新任务在进行中」的判断依据。
var activePhases = map[string]bool{
	PhaseDownloading: true,
	PhaseVerifying:   true,
	PhaseInstalling:  true,
	PhaseRestarting:  true,
}

type cachedCheck struct {
	at       time.Time
	result   CheckResult
	manifest *Manifest
	asset    *Asset
}

var (
	mu     sync.Mutex
	cached *cachedCheck
	state  = State{Phase: PhaseIdle}

	listeners []func(State)
	quitFunc  func() // main.go 注入：置 quitting 标志并让 Wails 退出主循环
)

// SetCurrentVersion 注入当前版本号（来源 //go:embed build/config.yml）。
// 必须在第一次 Check 之前调用。
func SetCurrentVersion(v string) {
	AppVersion = strings.TrimSpace(strings.TrimPrefix(v, "v"))
}

// SetQuitFunc 注入「让应用退出」的回调（main.go：置 quitting 标志 → wails.Quit，
// 引擎停止与 wl-paste 清理统一走 OnShutdown）。更新器在文件就位后调用它，
// 让新版本由 detached helper 在旧进程退出后拉起（见 spawnRelaunch）。
func SetQuitFunc(fn func()) {
	quitFunc = fn
}

// Subscribe 注册状态变更回调（app.go 用来转成 Wails 事件）。
func Subscribe(fn func(State)) {
	mu.Lock()
	defer mu.Unlock()
	listeners = append(listeners, fn)
}

// GetState 返回当前进度快照。
func GetState() State {
	mu.Lock()
	defer mu.Unlock()
	return state
}

func setState(patch State) {
	mu.Lock()
	state = patch
	lns := listeners
	mu.Unlock()
	for _, fn := range lns {
		fn(patch)
	}
}

// SelfUpdateSupported 报告当前构建能否自更新。
// 开发模式（wails3 dev，无 production 标签）或平台安装器不可用（如
// macOS 非 .app bundle 运行）时为 false，前端据此给出说明而不是让更新失败。
func SelfUpdateSupported() bool {
	return !devBuild && platformSupported() == nil
}

// Check 检查是否有更新。成功结果缓存 60 秒；force=true 绕过缓存。
// 检查失败（网络 / 清单问题）不返回 error —— 「连不上 GitHub」是外部状态，
// 原因放在 CheckResult.Error 里交给前端展示。
func Check(force bool) CheckResult {
	mu.Lock()
	if !force && cached != nil && time.Since(cached.at) < checkCacheTTL {
		r := cached.result
		mu.Unlock()
		return r
	}
	mu.Unlock()

	platform, platformErr := PlatformKey()
	supported := !devBuild && platformErr == nil && platformSupported() == nil

	now := time.Now()
	result := CheckResult{
		Current: CurrentInfo{
			Version:   AppVersion,
			Platform:  platform,
			Supported: supported,
		},
		CheckedAt: now.UnixMilli(),
	}
	if platformErr != nil {
		result.Current.Platform = fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
		result.Error = platformErr.Error()
		cache(result, nil, nil)
		return result
	}
	if devBuild {
		result.Error = "开发模式构建无法自更新，请从发布页下载新版本"
		cache(result, nil, nil)
		return result
	}
	if err := platformSupported(); err != nil {
		result.Error = err.Error()
		cache(result, nil, nil)
		return result
	}
	if AppVersion == "" {
		result.Error = "当前版本号缺失（构建注入异常），无法比较版本"
		cache(result, nil, nil)
		return result
	}

	client := &http.Client{Timeout: manifestTimeout}
	m, err := fetchManifest(client)
	if err != nil {
		result.Error = err.Error()
		return result // 失败结果不缓存：下次点击立刻重试
	}

	asset := m.AssetFor(platform)
	result.Latest = &LatestInfo{
		Tag:         m.Tag,
		Version:     m.Version,
		PublishedAt: m.PublishedAt,
		HTMLURL:     ReleaseURL(m.Tag),
		AssetName:   asset.Name,
		AssetSize:   asset.Size,
	}
	result.HasUpdate = IsNewerRelease(m.Version, AppVersion)
	if result.HasUpdate && asset == nil {
		result.Error = fmt.Sprintf("最新版本 %s 没有适用于 %s 的安装包", m.Tag, platform)
	}
	cache(result, m, asset)
	return result
}

func cache(result CheckResult, m *Manifest, asset *Asset) {
	mu.Lock()
	cached = &cachedCheck{at: time.Now(), result: result, manifest: m, asset: asset}
	mu.Unlock()
}

// Apply 开始更新到指定 tag：校验目标版本后立即返回，下载与替换在后台进行。
// 与 Check 共用同一条缓存记录，tag 对不上（检查与点击之间又发了新版）就拒绝，
// 避免「看到的是 A、下的是 B」。进度通过 Subscribe 的回调 / GetState 查询。
func Apply(tag string) error {
	mu.Lock()
	if activePhases[state.Phase] {
		mu.Unlock()
		return errors.New("已有更新任务在进行中，请等待它完成")
	}
	c := cached
	mu.Unlock()

	// 缓存缺失或 tag 不一致时强制重新检查一次。
	if c == nil || c.result.Latest == nil || c.result.Latest.Tag != tag {
		Check(true)
		mu.Lock()
		c = cached
		mu.Unlock()
	}
	if c == nil || c.result.Latest == nil || c.result.Latest.Tag != tag {
		msg := "目标版本已不是最新发布，请重新检查更新"
		if c != nil && c.result.Error != "" {
			msg = c.result.Error
		}
		return errors.New(msg)
	}
	if !c.result.HasUpdate {
		return errors.New("当前已是最新版本")
	}
	if c.manifest == nil || c.asset == nil {
		return errors.New("该版本没有适用于当前平台的安装包")
	}

	m, asset := c.manifest, c.asset
	total := asset.Size
	setState(State{Phase: PhaseDownloading, Tag: tag, Total: &total})

	go func() {
		if err := runApply(m, asset); err != nil {
			removeQuietly(tempAssetPath(asset.Name))
			setState(State{Phase: PhaseError, Tag: tag, Total: &total, Error: err.Error()})
		}
	}()
	return nil
}

// runApply 编排一次完整的更新：下载 → 校验 → 平台安装 → 等待退出重启。
func runApply(m *Manifest, asset *Asset) error {
	tmp, err := downloadAsset(asset, m.Tag)
	if err != nil {
		return err
	}

	total := asset.Size
	setState(State{Phase: PhaseVerifying, Tag: m.Tag, Total: &total})
	if err := verifyDownload(tmp, asset); err != nil {
		return err
	}

	setState(State{Phase: PhaseInstalling, Tag: m.Tag, Total: &total})
	spec, err := platformInstall(tmp, asset, m)
	if err != nil {
		return err
	}

	setState(State{Phase: PhaseRestarting, Tag: m.Tag, Total: &total})
	// 先把「等本进程退出后拉起新版」的 helper 挂起来，再请求退出：
	// helper 等到进程消失才会 exec，此时引擎已停、端口已释放。
	if err := spawnRelaunch(spec); err != nil {
		return fmt.Errorf("无法拉起新版本：%w", err)
	}
	if quitFunc != nil {
		quitFunc()
	} else {
		// 没有注入退出回调（异常装配）：文件已就位，只能提示手动重启。
		fmt.Fprintln(os.Stderr, "[updater] 已安装新版本，请手动重启 ClipSync")
	}
	return nil
}

// updateDir 返回更新暂存目录（默认 ~/.clipsync/updates）。
// CLIPSYNC_UPDATE_DIR 可覆盖（测试隔离与自定义位置）。
func updateDir() string {
	if d := strings.TrimSpace(os.Getenv("CLIPSYNC_UPDATE_DIR")); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}
	return filepath.Join(home, ".clipsync", "updates")
}

// tempAssetPath 返回下载暂存路径（updateDir 下）。
// 失败后目录里不该留几十 MB 的半成品，用 .part 后缀便于启动时清理。
func tempAssetPath(name string) string {
	return filepath.Join(updateDir(), name+".part")
}

// downloadAsset 把产物下载到暂存文件，边写边算 sha256 与进度。
func downloadAsset(asset *Asset, tag string) (string, error) {
	url := asset.DownloadURL(tag)
	mirror := mirrorBase()
	if mirror != "" {
		// ghproxy 约定：镜像地址前缀拼接原始 URL；镜像由用户显式配置，不做出域校验。
		url = mirror + url
	}
	return downloadTo(url, asset, tag, mirror == "")
}

// downloadTo 从 url 下载产物到暂存文件。verifyOrigin 为 true 时要求最终跳转
// 地址仍落在 GitHub 域名内（直连路径；镜像由用户显式配置、不做出域校验）。
// 独立成函数是为了让单测能用 httptest 走完整下载路径（见 TestDownloadTo）。
func downloadTo(url string, asset *Asset, tag string, verifyOrigin bool) (string, error) {
	client := &http.Client{Timeout: downloadTimeout}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("ClipSync/%s", AppVersion))
	resp, err := client.Do(req)
	if err != nil {
		return "", netFailure("下载安装包", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败：HTTP %d", resp.StatusCode)
	}
	// 直连（未配置镜像）时最终跳转地址必须仍落在 GitHub 域名内。
	if verifyOrigin && !isTrustedAssetURL(resp.Request.URL.String()) {
		return "", errors.New("下载地址不在允许的域名内，已中止")
	}

	dst := tempAssetPath(asset.Name)
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return "", fmt.Errorf("创建下载目录失败：%w", err)
	}
	f, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("创建暂存文件失败：%w", err)
	}
	defer f.Close()

	total := asset.Size
	st := State{Phase: PhaseDownloading, Tag: tag, Total: &total}
	var (
		h        hash.Hash = sha256.New()
		received int64
		lastEmit time.Time
	)
	buf := make([]byte, 64*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return "", fmt.Errorf("写入暂存文件失败：%w", werr)
			}
			h.Write(buf[:n])
			received += int64(n)
			st.Received = received
			// 节流：下载中不每个 chunk 都广播，phase 变化与最后一次除外。
			if time.Since(lastEmit) >= progressEmitInterval {
				lastEmit = time.Now()
				setState(st)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			// 中途断流同样是网络问题（对端重置 / 代理掐断），只写日志、UI 用固定文案。
			return "", netFailure("下载安装包", rerr)
		}
	}
	st.Received = received
	setState(st)

	// 先 Sync 落盘再 Close：Close 之后再 Sync 会报 "file already closed"
	//（v0.1.1 之前这里写反了，导致每次下载完成都失败、自更新从未走通）。
	if err := f.Sync(); err != nil {
		return "", fmt.Errorf("写入暂存文件失败：%w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("写入暂存文件失败：%w", err)
	}
	return dst, nil
}

// verifyDownload 校验暂存文件的大小与 sha256，并核对平台文件头。
// 任一步不对都停在安装之前，目录里不会留下半成品。
func verifyDownload(path string, asset *Asset) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sum, err := hashBytes(f)
	if err != nil {
		return fmt.Errorf("读取暂存文件失败：%w", err)
	}
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if fi.Size() != asset.Size {
		return fmt.Errorf("安装包不完整：应为 %d 字节，实际 %d 字节", asset.Size, fi.Size())
	}
	if sum != strings.ToLower(asset.SHA256) {
		return errors.New("安装包校验失败（sha256 不匹配），已放弃安装")
	}
	if err := verifyAssetHeader(path); err != nil {
		return err
	}
	return nil
}

// hexSum 是测试辅助：算一段字节的 sha256 hex。
func hexSum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// removeQuietly 删除临时文件 / 备份的尽力而为：清理失败不应掩盖真正的错误。
func removeQuietly(path string) {
	_ = os.RemoveAll(path)
}

// CleanupStaleFiles 清理上次更新留下的备份与半成品，在启动成功后调用：
// 新版本能起来才说明它可用，此时删掉 .old 回滚点是安全的；
// 起不来则 .old 还在，可手工恢复（此时也就不会走到这里）。
func CleanupStaleFiles() {
	platformCleanup()

	// 下载暂存目录里可能留着失败更新的 .part 文件。
	dir := updateDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".part") {
			removeQuietly(filepath.Join(dir, e.Name()))
		}
	}
}
