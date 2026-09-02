package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFileForTest(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

func TestLoadConfigMigratesLegacy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// 旧版配置：字符串 peers、共享 token、auto_pair，且缺 device_id。
	legacy := `{
		"name": "my-pc",
		"port": 9250,
		"web_port": 9260,
		"token": "old-token",
		"peers": ["192.168.1.20:9250"],
		"auto_pair": true
	}`
	if err := writeFileForTest(path, legacy); err != nil {
		t.Fatalf("写入旧配置失败: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Name != "my-pc" || cfg.Port != 9250 {
		t.Fatalf("迁移破坏了原有字段: %+v", cfg)
	}
	if cfg.DeviceID == "" {
		t.Fatal("缺少 device_id 的旧配置应自动补充")
	}
	// 旧字符地址因缺少身份/密钥被丢弃，避免误连。
	if len(cfg.Peers) != 0 {
		t.Fatalf("旧 peers 应被丢弃，得到 %v", cfg.Peers)
	}
}

func TestLoadConfigNewFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	_ = writeFileForTest(path, `{
		"name": "my-pc",
		"port": 9251,
		"web_port": 9261,
		"device_id": "id-123",
		"peers": [{"id": "peer-x", "name": "office", "addr": "192.168.1.21:9251", "secret": "s3cret"}]
	}`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.DeviceID != "id-123" {
		t.Fatalf("device_id 解析错误: %s", cfg.DeviceID)
	}
	if len(cfg.Peers) != 1 || cfg.Peers[0].ID != "peer-x" || cfg.Peers[0].Secret != "s3cret" {
		t.Fatalf("peers 解析错误: %+v", cfg.Peers)
	}
}

func TestUpsertPeer(t *testing.T) {
	cfg := &Config{}
	changed := cfg.UpsertPeer(Peer{ID: "a", Name: "A", Addr: "1.1.1.1:9250", Secret: "s1"})
	if !changed {
		t.Fatal("新增应返回 changed")
	}
	changed = cfg.UpsertPeer(Peer{ID: "a", Name: "A", Addr: "1.1.1.1:9250", Secret: "s1"})
	if changed {
		t.Fatal("无变化应返回 false")
	}
	if cfg.PeerByID("a") == nil {
		t.Fatal("按 ID 查找失败")
	}
	cfg.RemovePeer("1.1.1.1:9250")
	if len(cfg.Peers) != 0 {
		t.Fatalf("删除失败: %+v", cfg.Peers)
	}
}
