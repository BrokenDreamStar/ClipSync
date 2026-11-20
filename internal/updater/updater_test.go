package updater

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.0.4", "0.0.5", -1},
		{"0.0.5", "0.0.4", 1},
		{"0.0.5", "0.0.5", 0},
		{"0.1.0", "0.0.99", 1},
		{"1.0.0", "0.99.99", 1},
		{"0.10.0", "0.9.0", 1},
		{"10.0.0", "9.99.99", 1},
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestIsNewerRelease(t *testing.T) {
	if !IsNewerRelease("0.0.5", "0.0.4") {
		t.Error("0.0.5 应比 0.0.4 新")
	}
	if IsNewerRelease("0.0.4", "0.0.4") {
		t.Error("同版本不应触发更新")
	}
	if IsNewerRelease("0.0.3", "0.0.4") {
		t.Error("旧版本不应触发更新")
	}
	// 版本缺失（注入异常）时必须保守，不能误判有更新。
	if IsNewerRelease("0.0.5", "") || IsNewerRelease("", "0.0.4") {
		t.Error("版本缺失时应返回 false")
	}
}

func TestTagVersion(t *testing.T) {
	if v := tagVersion("v0.0.5"); v != "0.0.5" {
		t.Errorf("tagVersion(v0.0.5) = %q", v)
	}
	for _, bad := range []string{"0.0.5", "v0.0", "v0.0.5-2", "latest", ""} {
		if v := tagVersion(bad); v != "" {
			t.Errorf("tagVersion(%q) = %q，应拒绝", bad, v)
		}
	}
}

// manifestFixture 构造一份合法清单（schema 1，覆盖发布矩阵全部平台）。
func manifestFixture() Manifest {
	sha := hexSum([]byte("fake"))
	return Manifest{
		Schema:  1,
		Tag:     "v0.0.5",
		Version: "0.0.5",
		Assets: []Asset{
			{Platform: "macos-universal", Name: "ClipSync-0.0.5-macos-universal.dmg", Size: 100, SHA256: sha},
			{Platform: "windows-amd64", Name: "ClipSync-0.0.5-windows-amd64.exe", Size: 100, SHA256: sha},
			{Platform: "windows-arm64", Name: "ClipSync-0.0.5-windows-arm64.exe", Size: 100, SHA256: sha},
			{Platform: "linux-amd64", Name: "ClipSync-0.0.5-linux-amd64.AppImage", Size: 100, SHA256: sha},
			{Platform: "linux-arm64", Name: "ClipSync-0.0.5-linux-arm64.AppImage", Size: 100, SHA256: sha},
		},
	}
}

func TestParseManifest(t *testing.T) {
	m := manifestFixture()
	raw := mustJSON(t, m)
	parsed, err := ParseManifest(raw)
	if err != nil {
		t.Fatalf("合法清单解析失败：%v", err)
	}
	if parsed.Tag != "v0.0.5" || len(parsed.Assets) != 5 {
		t.Fatalf("解析结果不对：%+v", parsed)
	}
	if a := parsed.AssetFor("windows-amd64"); a == nil || a.Name != "ClipSync-0.0.5-windows-amd64.exe" {
		t.Fatalf("AssetFor(windows-amd64) 结果不对")
	}
	if a := parsed.AssetFor("windows-arm64"); a == nil || a.Name != "ClipSync-0.0.5-windows-arm64.exe" {
		t.Fatalf("AssetFor(windows-arm64) 结果不对")
	}
	if a := parsed.AssetFor("linux-arm64"); a == nil || a.Name != "ClipSync-0.0.5-linux-arm64.AppImage" {
		t.Fatalf("AssetFor(linux-arm64) 结果不对")
	}
	if a := parsed.AssetFor("windows-386"); a != nil {
		t.Fatalf("清单里没有 windows-386，应返回 nil")
	}
}

func TestParseManifestRejects(t *testing.T) {
	sha := hexSum([]byte("fake"))
	base := manifestFixture()

	invalid := map[string]func(*Manifest){
		"schema 不符":  func(m *Manifest) { m.Schema = 2 },
		"tag 非法":     func(m *Manifest) { m.Tag = "0.0.5" },
		"tag 与版本不一致": func(m *Manifest) { m.Version = "0.0.6" },
		"空资产":        func(m *Manifest) { m.Assets = nil },
		"产物名不符":      func(m *Manifest) { m.Assets[0].Name = "ClipSync-0.0.6-macos-universal.dmg" },
		"产物名带路径分隔":   func(m *Manifest) { m.Assets[0].Name = "../evil.dmg" },
		"平台重复":       func(m *Manifest) { m.Assets = append(m.Assets, m.Assets[0]) },
		"sha256 非法":  func(m *Manifest) { m.Assets[0].SHA256 = "zz" },
		"大小非法":       func(m *Manifest) { m.Assets[0].Size = 0 },
		"扩展名不符":      func(m *Manifest) { m.Assets[1].Name = "ClipSync-0.0.5-windows-amd64.dmg" },
		"全部未知平台":     func(m *Manifest) { m.Assets = []Asset{{Platform: "freebsd-x64", Name: "x", Size: 1, SHA256: sha}} },
	}
	for name, mutate := range invalid {
		m := base
		mutate(&m)
		if _, err := ParseManifest(mustJSON(t, m)); err == nil {
			t.Errorf("%s：应被拒绝", name)
		}
	}

	if _, err := ParseManifest([]byte("not json")); err == nil {
		t.Error("非 JSON 应被拒绝")
	}
}

func TestParseManifestIgnoresUnknownPlatform(t *testing.T) {
	m := manifestFixture()
	m.Assets = append(m.Assets, Asset{Platform: "plan9-arm", Name: "whatever", Size: 1, SHA256: "invalid"})
	parsed, err := ParseManifest(mustJSON(t, m))
	if err != nil {
		t.Fatalf("未知平台条目应被忽略而不是整份判坏：%v", err)
	}
	if len(parsed.Assets) != 6 || parsed.AssetFor("plan9-arm") == nil {
		t.Fatal("未知平台条目应原样保留（前端会按平台过滤）")
	}
}

func TestPlatformKeyMatchesAssetNaming(t *testing.T) {
	// 发布矩阵键与产物扩展名一一对应，latest.json 校验依赖这份表。
	for key, ext := range knownPlatforms {
		if name := expectedAssetName("0.0.5", key); !strings.HasSuffix(name, key+ext) {
			t.Errorf("expectedAssetName(%s) = %q，平台键/扩展名对不上", key, name)
		}
	}
}

func TestExpectedAssetName(t *testing.T) {
	cases := map[string]string{
		"macos-universal": "ClipSync-0.0.5-macos-universal.dmg",
		"windows-amd64":   "ClipSync-0.0.5-windows-amd64.exe",
		"windows-arm64":   "ClipSync-0.0.5-windows-arm64.exe",
		"linux-amd64":     "ClipSync-0.0.5-linux-amd64.AppImage",
		"linux-arm64":     "ClipSync-0.0.5-linux-arm64.AppImage",
	}
	for platform, want := range cases {
		if got := expectedAssetName("0.0.5", platform); got != want {
			t.Errorf("expectedAssetName(%s) = %q, want %q", platform, got, want)
		}
	}
}

func TestDownloadURL(t *testing.T) {
	a := Asset{Name: "ClipSync-0.0.5-windows-amd64.exe"}
	want := "https://github.com/BrokenDreamStar/ClipSync/releases/download/v0.0.5/ClipSync-0.0.5-windows-amd64.exe"
	if got := a.DownloadURL("v0.0.5"); got != want {
		t.Errorf("DownloadURL = %q", got)
	}
}

func TestIsTrustedAssetURL(t *testing.T) {
	ok := []string{
		"https://github.com/BrokenDreamStar/ClipSync/releases/download/v1/x",
		"https://objects.githubusercontent.com/github-release/xxx",
	}
	for _, u := range ok {
		if !isTrustedAssetURL(u) {
			t.Errorf("%s 应可信", u)
		}
	}
	bad := []string{
		"http://github.com/x",
		"https://evil.com/github.com/x",
		"https://user:pass@github.com/x",
		"https://githubusercontent.com.evil.com/x",
	}
	for _, u := range bad {
		if isTrustedAssetURL(u) {
			t.Errorf("%s 应拒绝", u)
		}
	}
}

// TestSwapInPlace 覆盖单文件产物（Windows EXE / Linux AppImage）的换名替换：
// 旧文件 → .old、新文件就位；失败时回滚。
func TestSwapInPlace(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "ClipSync")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(dir, "ClipSync.part")
	if err := os.WriteFile(tmp, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := swapInPlace(tmp, target); err != nil {
		t.Fatalf("swapInPlace: %v", err)
	}
	if got, _ := os.ReadFile(target); string(got) != "new" {
		t.Fatalf("目标文件内容 = %q", got)
	}
	if got, _ := os.ReadFile(target + ".old"); string(got) != "old" {
		t.Fatalf("备份内容 = %q", got)
	}
	if fi, err := os.Stat(target); err != nil || fi.Mode().Perm() != 0o755 {
		t.Fatalf("目标文件权限应沿用旧文件（0755），实际 %v（err=%v）", fi.Mode().Perm(), err)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatal("暂存文件应已被移走")
	}

	// 回滚路径：目标就位成功，再次替换时旧 .old 被清掉。
	if err := os.WriteFile(tmp, []byte("newer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := swapInPlace(tmp, target); err != nil {
		t.Fatalf("第二次 swapInPlace: %v", err)
	}
	if got, _ := os.ReadFile(target); string(got) != "newer" {
		t.Fatalf("第二轮替换失败：%q", got)
	}
}

func TestCheckUnsupportedPlatformFillsError(t *testing.T) {
	// 不便在单测里模拟 GOOS；这里只验证 Check 在「无版本注入 + dev 构建」下
	// 不 panic 且给出可读错误。
	SetCurrentVersion("0.0.4")
	r := Check(true)
	if r.Current.Version != "0.0.4" {
		t.Errorf("Current.Version = %q", r.Current.Version)
	}
	// dev 构建必给「无法自更新」的说明；platformSupported 在 darwin 裸测试二进制
	// 下同样不可安装，两种错误都算合法结果。
	if r.Current.Supported {
		t.Fatal("dev 构建不应声称支持自更新")
	}
	if r.Error == "" {
		t.Fatal("应给出不可自更新的原因")
	}
}

// TestParseCISample 用 release.yml 的 latest.json 生成步骤（jq）产出的真实形状
// 喂一遍解析器：生成端与解析端的契约锁在这里，两边谁改了命名 / 结构都会先炸测试。
func TestParseCISample(t *testing.T) {
	sample := `{
  "schema": 1,
  "tag": "v0.0.5",
  "version": "0.0.5",
  "publishedAt": "2026-11-19T07:09:09Z",
  "assets": [
    {"platform":"macos-universal","name":"ClipSync-0.0.5-macos-universal.dmg","size":11088,"sha256":"66a917f2cdec9367d84093dccadfa327281573c2a8f1eeec2add79c57c754032"},
    {"platform":"windows-amd64","name":"ClipSync-0.0.5-windows-amd64.exe","size":11088,"sha256":"66a917f2cdec9367d84093dccadfa327281573c2a8f1eeec2add79c57c754032"},
    {"platform":"windows-arm64","name":"ClipSync-0.0.5-windows-arm64.exe","size":11088,"sha256":"66a917f2cdec9367d84093dccadfa327281573c2a8f1eeec2add79c57c754032"},
    {"platform":"linux-amd64","name":"ClipSync-0.0.5-linux-amd64.AppImage","size":11088,"sha256":"66a917f2cdec9367d84093dccadfa327281573c2a8f1eeec2add79c57c754032"},
    {"platform":"linux-arm64","name":"ClipSync-0.0.5-linux-arm64.AppImage","size":11088,"sha256":"66a917f2cdec9367d84093dccadfa327281573c2a8f1eeec2add79c57c754032"}
  ]
}`
	m, err := ParseManifest([]byte(sample))
	if err != nil {
		t.Fatalf("CI 生成的清单应能被解析：%v", err)
	}
	if len(m.Assets) != 5 || m.AssetFor("windows-arm64") == nil || m.AssetFor("linux-arm64") == nil {
		t.Fatalf("解析结果不对：%+v", m)
	}
}

// TestDownloadTo 回归测试：完整下载路径必须能正常结束并产出可校验的暂存文件。
// 背景：v0.1.1 之前 downloadTo 把 f.Sync() 写在 f.Close() 之后，每次下载完成
// 都会报 "file already closed"，自更新从未走通过一次（真实环境实测触发）。
// payload 以 ELF 魔数开头，让 verifyDownload 的完整校验链（大小 + sha256 +
// 平台文件头）一起被覆盖。
func TestDownloadTo(t *testing.T) {
	t.Setenv("CLIPSYNC_UPDATE_DIR", t.TempDir())
	payload := append([]byte("\x7fELF"), bytes.Repeat([]byte("clipsync"), 4096)...)
	asset := &Asset{
		Platform: "linux-amd64",
		Name:     "ClipSync-0.0.5-linux-amd64.AppImage",
		Size:     int64(len(payload)),
		SHA256:   hexSum(payload),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	path, err := downloadTo(srv.URL, asset, "v0.0.5", false)
	if err != nil {
		t.Fatalf("downloadTo: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取暂存文件: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("暂存文件内容与下载内容不一致")
	}
	if err := verifyDownload(path, asset); err != nil {
		t.Fatalf("下载后校验失败: %v", err)
	}
}

// TestDownloadToBadStatus 非 200 响应直接失败，不产生暂存文件。
func TestDownloadToBadStatus(t *testing.T) {
	t.Setenv("CLIPSYNC_UPDATE_DIR", t.TempDir())
	asset := &Asset{Name: "ClipSync-0.0.5-linux-amd64.AppImage", Size: 1, SHA256: hexSum([]byte("x"))}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := downloadTo(srv.URL, asset, "v0.0.5", false); err == nil {
		t.Fatal("HTTP 404 应返回错误")
	}
	if _, err := os.Stat(tempAssetPath(asset.Name)); !os.IsNotExist(err) {
		t.Fatal("失败时不应留下暂存文件")
	}
}

// TestNetFailure 锁定 UI 报错契约：网络层错误对用户只有固定文案，
// 底层细节（拨号地址 / 代理错误等）只允许出现在日志里，不允许进入前端展示。
func TestNetFailure(t *testing.T) {
	leaky := fmt.Errorf("dial tcp 203.0.113.7:443: i/o timeout (proxy refused)")
	err := netFailure("下载安装包", leaky)
	if err == nil || err.Error() != "无法连接 GitHub，请检查网络后重试" {
		t.Fatalf("netFailure 返回了意外文案：%v", err)
	}
	if strings.Contains(err.Error(), "203.0.113.7") || strings.Contains(err.Error(), "timeout") {
		t.Fatal("netFailure 的返回值不得夹带底层错误细节")
	}
}

func TestApplyRejectsWhenNoCache(t *testing.T) {
	mu.Lock()
	cached = nil
	mu.Unlock()
	defer func() { mu.Lock(); cached = nil; mu.Unlock() }()

	err := Apply("v9.9.9")
	if err == nil {
		t.Fatal("无缓存的 tag 应被拒绝")
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
