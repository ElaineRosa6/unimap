package config

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

// Manager 配置管理器
type Manager struct {
	config *Config
	path   string
	mu     sync.RWMutex // protects config pointer reads/writes
}

// NewManager 创建配置管理器
func NewManager(path string) *Manager {
	return &Manager{
		path: path,
	}
}

// GetConfig 获取配置 (thread-safe)
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// SetConfig replaces the current config (thread-safe).
// Used by hot-update and rollback.
func (m *Manager) SetConfig(cfg *Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
}

// applyDefaults 应用默认值
func (m *Manager) applyDefaults(config *Config) {
	m.applyEngineDefaults(config)
	m.applySystemDefaults(config)
	m.applyScreenshotDefaults(config)
	m.applyWebDefaults(config)
	m.applyAuthDefaults(config)
	m.applyCacheDefaults(config)
	m.applyMiscDefaults(config)
}


// IsValid 检查配置是否有效
func (m *Manager) IsValid() bool {
	return m.config != nil
}

// GetEngineConfig 获取引擎配置
func (m *Manager) GetEngineConfig(name string) (interface{}, error) {
	if !m.IsValid() {
		return nil, fmt.Errorf("config not loaded")
	}

	switch strings.ToLower(name) {
	case "quake":
		return &m.config.Engines.Quake, nil
	case "zoomeye":
		return &m.config.Engines.Zoomeye, nil
	case "hunter":
		return &m.config.Engines.Hunter, nil
	case "fofa":
		return &m.config.Engines.Fofa, nil
	case "shodan":
		return &m.config.Engines.Shodan, nil
	case "censys":
		return &m.config.Engines.Censys, nil
	case "daydaymap":
		return &m.config.Engines.Daydaymap, nil
	default:
		return nil, fmt.Errorf("unknown engine: %s", name)
	}
}

// Save 保存配置文件
func (m *Manager) Save() error {
	if m.config == nil {
		return fmt.Errorf("config is nil")
	}

	// 加密通知渠道密钥后再持久化
	EncryptNotifySecrets(m.config)
	defer DecryptNotifySecrets(m.config)

	data, err := yaml.Marshal(m.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(m.path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	if err := os.WriteFile(m.path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

// GetEngineCacheConfig 获取引擎级别的缓存配置
func (m *Manager) GetEngineCacheConfig(engineName string) EngineCacheConfig {
	if !m.IsValid() {
		return EngineCacheConfig{Enabled: true, TTL: 3600, MaxSize: 500}
	}

	engineName = strings.ToLower(strings.TrimSpace(engineName))
	if cfg, exists := m.config.Cache.Engines[engineName]; exists {
		return cfg
	}

	// 返回默认配置
	return EngineCacheConfig{Enabled: true, TTL: m.config.System.CacheTTL, MaxSize: m.config.System.CacheMaxSize}
}

// GetAllEngineCacheConfigs 获取所有引擎的缓存配置
func (m *Manager) GetAllEngineCacheConfigs() map[string]EngineCacheConfig {
	if !m.IsValid() {
		return make(map[string]EngineCacheConfig)
	}
	return m.config.Cache.Engines
}

// IsCacheEnabledForEngine 检查指定引擎是否启用缓存
func (m *Manager) IsCacheEnabledForEngine(engineName string) bool {
	cfg := m.GetEngineCacheConfig(engineName)
	return cfg.Enabled
}

func normalizeProxyList(raw []string) []string {
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, item := range raw {
		parts := strings.FieldsFunc(item, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
		})
		if len(parts) == 0 {
			parts = []string{item}
		}
		for _, part := range parts {
			proxy := strings.TrimSpace(part)
			if proxy == "" {
				continue
			}
			if _, exists := seen[proxy]; exists {
				continue
			}
			seen[proxy] = struct{}{}
			out = append(out, proxy)
		}
	}
	return out
}

// GetCacheTTLForEngine 获取指定引擎的缓存 TTL（秒）
func (m *Manager) GetCacheTTLForEngine(engineName string) int {
	cfg := m.GetEngineCacheConfig(engineName)
	if cfg.TTL > 0 {
		return cfg.TTL
	}
	if m.IsValid() {
		return m.config.System.CacheTTL
	}
	return 3600 // 默认1小时
}

// GetCacheMaxSizeForEngine 获取指定引擎的最大缓存条目数
func (m *Manager) GetCacheMaxSizeForEngine(engineName string) int {
	cfg := m.GetEngineCacheConfig(engineName)
	if cfg.MaxSize > 0 {
		return cfg.MaxSize
	}
	if m.IsValid() {
		return m.config.System.CacheMaxSize
	}
	return 1000 // 默认1000条
}

// GetCacheBackend 获取缓存后端类型
func (m *Manager) GetCacheBackend() string {
	if !m.IsValid() {
		return "memory"
	}
	backend := strings.ToLower(strings.TrimSpace(m.config.Cache.Backend))
	if backend == "" {
		return "memory"
	}
	return backend
}

// GetRedisAddr 获取Redis地址
func (m *Manager) GetRedisAddr() string {
	if !m.IsValid() {
		return "127.0.0.1:6379"
	}
	return strings.TrimSpace(m.config.Cache.Redis.Addr)
}

// GetRedisPassword 获取Redis密码
func (m *Manager) GetRedisPassword() string {
	if !m.IsValid() {
		return ""
	}
	return m.config.Cache.Redis.Password
}

// GetRedisDB 获取Redis数据库
func (m *Manager) GetRedisDB() int {
	if !m.IsValid() {
		return 0
	}
	return m.config.Cache.Redis.DB
}

// GetRedisPrefix 获取Redis键前缀
func (m *Manager) GetRedisPrefix() string {
	if !m.IsValid() {
		return "unimap:"
	}
	prefix := strings.TrimSpace(m.config.Cache.Redis.Prefix)
	if prefix == "" {
		return "unimap:"
	}
	return prefix
}

// generateSecureToken generates a cryptographically secure random token
// of the specified length using URL-safe base64 encoding.
func generateSecureToken(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	token := make([]byte, length)
	for i := range token {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// Fall back to a simple hex token if crypto/rand fails (extremely unlikely)
			b := make([]byte, length/2)
			if _, err := rand.Read(b); err == nil {
				return fmt.Sprintf("%x", b)
			}
			return "fallback-token-" + fmt.Sprintf("%d", os.Getpid())
		}
		token[i] = charset[n.Int64()]
	}
	return string(token)
}

// HashPassword hashes a password using bcrypt with default cost.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword compares a password against a bcrypt hash.
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
