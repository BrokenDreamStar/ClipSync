package engine

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Peer 是一台已配对的对端。
//
// 与旧版把对端当成「一串 host:port」不同，配对后对端以稳定的 DeviceID 作为身份，
// 并携带一份「本机与该对端之间独享」的 Secret：双方拨号同步时凭它完成鉴权，
// 不再需要全局共享 token。
type Peer struct {
	ID     string `json:"id"`     // 对端 device_id（唯一身份）
	Name   string `json:"name"`   // 对端显示名
	Addr   string `json:"addr"`   // 最近一次已知 host:port，IP 变化后自动刷新
	Secret string `json:"secret"` // 与对端之间的共享密钥（配对时交换/生成）
}

// Config 是持久化的配置。
type Config struct {
	Name     string `json:"name"`      // 本机显示名
	Port     int    `json:"port"`      // 同步监听端口
	WebPort  int    `json:"web_port"`  // 管理界面端口（仅本机访问）
	DeviceID string `json:"device_id"` // 本机唯一身份，首次启动生成后不变
	Peers    []Peer `json:"peers"`     // 已配对对端列表
}

func configPath() string {
	if p := os.Getenv("CLIPSYNC_CONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".clipsync", "config.json")
}

// ConfigPath 暴露给顶层 main 使用。
func ConfigPath() string { return configPath() }

func defaultConfig() *Config {
	return &Config{
		Name:     hostname(),
		Port:     9250,
		WebPort:  9260,
		DeviceID: newID(),
	}
}

// DiscoveryPort 是局域网发现使用的 UDP 端口（与 TCP 同步端口分开）。
const DiscoveryPort = 9251

// LoadConfig 是 loadConfig 的导出版本。
func LoadConfig(path string) (*Config, error) { return loadConfig(path) }

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := defaultConfig()
		if err := cfg.Save(path); err != nil {
			return nil, err
		}
		fmt.Printf("未找到配置文件，已生成默认配置: %s\n", path)
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	cfg := defaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置 %s: %w", path, err)
	}
	// 旧配置或手写配置缺少 device_id：分配一个并落盘，保证身份稳定、不会每次启动漂移。
	if cfg.DeviceID == "" || !bytes.Contains(data, []byte(`"device_id"`)) {
		cfg.DeviceID = newID()
		if err := cfg.Save(path); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// UnmarshalJSON 兼容旧格式的 peers（字符串数组）与新格式（对象数组）。
// 旧版「共享 token + 自动配对」在该次重构后不再适用，旧 peers 因缺少身份与
// 密钥会被丢弃，需要按新流程重新配对；token / auto_pair 字段被忽略。
func (c *Config) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name     string          `json:"name"`
		Port     int             `json:"port"`
		WebPort  int             `json:"web_port"`
		DeviceID string          `json:"device_id"`
		Peers    json.RawMessage `json:"peers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	// 只在字段非空时覆盖，保留 defaultConfig 预填的本机名/端口等默认值。
	if raw.Name != "" {
		c.Name = raw.Name
	}
	if raw.Port != 0 {
		c.Port = raw.Port
	}
	if raw.WebPort != 0 {
		c.WebPort = raw.WebPort
	}
	if raw.DeviceID != "" {
		c.DeviceID = raw.DeviceID
	}

	if len(raw.Peers) == 0 {
		c.Peers = nil
		return nil
	}
	// 先按新格式解析；失败再退回旧格式的字符串数组。
	var peers []Peer
	if err := json.Unmarshal(raw.Peers, &peers); err == nil {
		c.Peers = peers
		return nil
	}
	var legacy []string
	if err := json.Unmarshal(raw.Peers, &legacy); err != nil {
		return fmt.Errorf("peers 字段格式无法识别")
	}
	// 旧字符串地址无法完成新的身份握手，直接丢弃，提示需重新配对。
	c.Peers = nil
	return nil
}

// Save writes the config back to disk.
func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}
	return h
}

// newID 生成一个 128 位随机 hex 串，用作设备身份或配对密钥。
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// ---- Peer 查询助手（供 engine / server 按身份或地址查找对端）----

// PeerByID 返回已配对列表中 ID 匹配的对端；未找到返回 nil。
func (c *Config) PeerByID(id string) *Peer {
	if id == "" {
		return nil
	}
	for i := range c.Peers {
		if c.Peers[i].ID == id {
			return &c.Peers[i]
		}
	}
	return nil
}

// PeerByName 返回名字匹配的对端；未找到返回 nil。
func (c *Config) PeerByName(name string) *Peer {
	if name == "" {
		return nil
	}
	for i := range c.Peers {
		if c.Peers[i].Name == name {
			return &c.Peers[i]
		}
	}
	return nil
}

// PeerByAddr 返回地址匹配的对端；未找到返回 nil。
func (c *Config) PeerByAddr(addr string) *Peer {
	for i := range c.Peers {
		if c.Peers[i].Addr == addr {
			return &c.Peers[i]
		}
	}
	return nil
}

// UpsertPeer 更新/新增一个对端；新增或地址变化时返回 true。
func (c *Config) UpsertPeer(p Peer) bool {
	if p.ID == "" {
		return false
	}
	for i := range c.Peers {
		if c.Peers[i].ID == p.ID {
			changed := c.Peers[i].Addr != p.Addr || c.Peers[i].Name != p.Name || c.Peers[i].Secret != p.Secret
			c.Peers[i] = p
			return changed
		}
	}
	c.Peers = append(c.Peers, p)
	return true
}

// RemovePeer 删除地址或 ID 匹配的对端，返回是否删除成功。
func (c *Config) RemovePeer(addr string) bool {
	out := c.Peers[:0]
	found := false
	for _, p := range c.Peers {
		if p.Addr == addr || p.ID == addr {
			found = true
			continue
		}
		out = append(out, p)
	}
	c.Peers = out
	return found
}

// PeerAddrs 返回所有已配对对端的地址列表（供 UI 或命令行展示）。
func (c *Config) PeerAddrs() []string {
	out := make([]string, 0, len(c.Peers))
	for _, p := range c.Peers {
		out = append(out, p.Addr)
	}
	return out
}

// normalizePeer 补全省略的端口并去除空白。
func normalizePeer(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr != "" && !strings.Contains(addr, ":") {
		addr += ":" + strconv.Itoa(defaultConfig().Port)
	}
	return addr
}

// NormalizePeer 是 normalizePeer 的导出版本。
func NormalizePeer(addr string) string { return normalizePeer(addr) }

// validPeer 校验 host:port 格式。
func validPeer(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	n, err := strconv.Atoi(port)
	if err != nil || n <= 0 || n >= 65536 {
		return false
	}
	if host == "" {
		return false
	}
	if net.ParseIP(host) == nil {
		// 允许主机名
		return !strings.ContainsAny(host, " /\\?#@")
	}
	return true
}
