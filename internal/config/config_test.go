package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unimap/project/internal/utils"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager("test.yaml")
	assert.NotNil(t, mgr)
	assert.Equal(t, "test.yaml", mgr.path)
}

func TestResolveEnv(t *testing.T) {
	mgr := NewManager("test.yaml")

	// Test $VAR format
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")

	result := mgr.ResolveEnv("$TEST_VAR")
	assert.Equal(t, "test_value", result)

	// Test ${VAR} format
	result = mgr.ResolveEnv("${TEST_VAR}")
	assert.Equal(t, "test_value", result)

	// Test non-existent env var — now returns empty string (2026-07-12)
	result = mgr.ResolveEnv("$NON_EXISTENT")
	assert.Equal(t, "", result)

	// Test empty string
	result = mgr.ResolveEnv("")
	assert.Equal(t, "", result)

	// Test regular string
	result = mgr.ResolveEnv("regular_string")
	assert.Equal(t, "regular_string", result)
}

func TestApplyDefaults(t *testing.T) {
	mgr := NewManager("test.yaml")
	var cfg Config

	mgr.applyDefaults(&cfg)

	// Test default engine configurations
	assert.Equal(t, "https://quake.360.net/api", cfg.Engines.Quake.BaseURL)
	assert.Equal(t, 5, cfg.Engines.Quake.QPS)
	assert.Equal(t, 30, cfg.Engines.Quake.Timeout)

	assert.Equal(t, "https://api.zoomeye.org", cfg.Engines.Zoomeye.BaseURL)
	assert.Equal(t, 3, cfg.Engines.Zoomeye.QPS)
	assert.Equal(t, 30, cfg.Engines.Zoomeye.Timeout)

	assert.Equal(t, "https://hunter.qianxin.com", cfg.Engines.Hunter.BaseURL)
	assert.Equal(t, 5, cfg.Engines.Hunter.QPS)
	assert.Equal(t, 30, cfg.Engines.Hunter.Timeout)

	assert.Equal(t, "https://fofa.info", cfg.Engines.Fofa.APIBaseURL)
	assert.Equal(t, "https://fofa.info", cfg.Engines.Fofa.WebBaseURL)
	assert.Equal(t, 3, cfg.Engines.Fofa.QPS)
	assert.Equal(t, 30, cfg.Engines.Fofa.Timeout)

	assert.Equal(t, "https://api.shodan.io", cfg.Engines.Shodan.BaseURL)
	assert.Equal(t, 1, cfg.Engines.Shodan.QPS)
	assert.Equal(t, 30, cfg.Engines.Shodan.Timeout)

	// Test default system configurations
	assert.Equal(t, 10, cfg.System.MaxConcurrent)
	assert.Equal(t, 3600, cfg.System.CacheTTL)
	assert.Equal(t, 1000, cfg.System.CacheMaxSize)
	assert.Equal(t, 300, cfg.System.CacheCleanupInterval)
	assert.Equal(t, 3, cfg.System.RetryAttempts)
	assert.Equal(t, "unimap/1.0", cfg.System.UserAgent)

	// Test default log configurations
	assert.Equal(t, "info", cfg.Log.Level)
	assert.Equal(t, "console", cfg.Log.Encoding)

	// Test default screenshot configurations
	assert.Equal(t, utils.ScreenshotsDir(), cfg.Screenshot.BaseDir)
	assert.Equal(t, 30, cfg.Screenshot.Timeout)
	assert.Equal(t, 1365, cfg.Screenshot.WindowWidth)
	assert.Equal(t, 768, cfg.Screenshot.WindowHeight)
	assert.Equal(t, 500, cfg.Screenshot.WaitTime)

	// Test default web configurations
	assert.Equal(t, []string{"http://localhost:8448", "http://127.0.0.1:8448"}, cfg.Web.CORS.AllowedOrigins)
	assert.Equal(t, []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}, cfg.Web.CORS.AllowedMethods)
	assert.Equal(t, []string{"Content-Type", "Authorization", "X-Admin-Token", "X-Requested-With", "X-WebSocket-Token"}, cfg.Web.CORS.AllowedHeaders)
	assert.Equal(t, 600, cfg.Web.CORS.MaxAge)

	assert.Equal(t, 60, cfg.Web.RateLimit.RequestsPerWindow)
	assert.Equal(t, 60, cfg.Web.RateLimit.WindowSeconds)

	assert.Equal(t, int64(10*1024*1024), cfg.Web.RequestLimits.MaxBodyBytes)
	assert.Equal(t, int64(10*1024*1024), cfg.Web.RequestLimits.MaxMultipartMemory)

	// Test default cache configurations
	assert.Equal(t, "memory", cfg.Cache.Backend)
	assert.Equal(t, "127.0.0.1:6379", cfg.Cache.Redis.Addr)
	assert.Equal(t, "unimap:", cfg.Cache.Redis.Prefix)
}

func TestApplyDefaultsDoesNotEnableUnconfiguredEngines(t *testing.T) {
	mgr := NewManager("test.yaml")
	var cfg Config

	mgr.applyDefaults(&cfg)

	assert.False(t, cfg.Engines.Fofa.Enabled)
	assert.False(t, cfg.Engines.Hunter.Enabled)
	assert.False(t, cfg.Engines.Zoomeye.Enabled)
	assert.False(t, cfg.Engines.Quake.Enabled)
	assert.False(t, cfg.Engines.Shodan.Enabled)
	assert.False(t, cfg.Engines.Censys.Enabled)
	assert.False(t, cfg.Engines.Daydaymap.Enabled)
}

func TestValidate(t *testing.T) {
	mgr := NewManager("test.yaml")

	// Test valid configuration
	var validCfg Config
	mgr.applyDefaults(&validCfg)
	err := mgr.validate(&validCfg)
	assert.NoError(t, err)

	// Test invalid configuration - system max_concurrent <= 0
	invalidCfg := validCfg
	invalidCfg.System.MaxConcurrent = 0
	err = mgr.validate(&invalidCfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "system max_concurrent must be greater than 0")

	// Test invalid configuration - cache backend
	invalidCfg = validCfg
	invalidCfg.Cache.Backend = "invalid_backend"
	err = mgr.validate(&invalidCfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache backend must be one of: memory, redis")

	// Test invalid configuration - quake QPS <= 0 (must be enabled first)
	invalidCfg = validCfg
	invalidCfg.Engines.Quake.Enabled = true
	invalidCfg.Engines.Quake.QPS = 0
	err = mgr.validate(&invalidCfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "quake engine qps must be greater than 0")
}

func TestGetEngineConfig(t *testing.T) {
	mgr := NewManager("test.yaml")
	var cfg Config
	mgr.applyDefaults(&cfg)
	mgr.config = &cfg

	// Test getting quake config
	quakeCfg, err := mgr.GetEngineConfig("quake")
	assert.NoError(t, err)
	assert.NotNil(t, quakeCfg)

	// Test getting zoomeye config
	zoomeyeCfg, err := mgr.GetEngineConfig("zoomeye")
	assert.NoError(t, err)
	assert.NotNil(t, zoomeyeCfg)

	// Test getting hunter config
	hunterCfg, err := mgr.GetEngineConfig("hunter")
	assert.NoError(t, err)
	assert.NotNil(t, hunterCfg)

	// Test getting fofa config
	fofaCfg, err := mgr.GetEngineConfig("fofa")
	assert.NoError(t, err)
	assert.NotNil(t, fofaCfg)

	// Test getting unknown engine
	unknownCfg, err := mgr.GetEngineConfig("unknown")
	assert.Error(t, err)
	assert.Nil(t, unknownCfg)
	assert.Contains(t, err.Error(), "unknown engine")
}

func TestIsValid(t *testing.T) {
	mgr := NewManager("test.yaml")

	// Test invalid config
	assert.False(t, mgr.IsValid())

	// Test valid config
	var cfg Config
	mgr.config = &cfg
	assert.True(t, mgr.IsValid())
}

func TestLoadWithNonExistentFile(t *testing.T) {
	mgr := NewManager("non_existent_config.yaml")
	err := mgr.Load()

	// Should return error but still have default config
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
	assert.NotNil(t, mgr.config)
}

func TestSave(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "config_test_*.yaml")
	assert.NoError(t, err)
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	mgr := NewManager(tmpPath)
	var cfg Config
	mgr.applyDefaults(&cfg)
	mgr.config = &cfg

	// Test save
	err = mgr.Save()
	assert.NoError(t, err)

	// Verify file exists and can be loaded
	data, err := os.ReadFile(tmpPath)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test save with nil config
	mgr.config = nil
	err = mgr.Save()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config is nil")
}

func TestSaveAndLoadPreservesOperationalConfiguration(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config_persistence_*.yaml")
	if err != nil {
		t.Fatalf("create temporary config: %v", err)
	}
	path := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("close temporary config: %v", err)
	}
	defer os.Remove(path)

	writer := NewManager(path)
	config := &Config{}
	writer.applyDefaults(config)
	config.System.UserAgent = "UniMap persistence test"
	config.Screenshot.Mode = "extension"
	config.Notifications.Enabled = true
	config.Notifications.Channels = []NotificationChannelCfg{{
		ID: "audit-log", Type: "log", Enabled: true,
		Headers: map[string]string{"X-Trace": "persistence-test"},
	}}
	writer.SetConfig(config)
	if err := writer.Save(); err != nil {
		t.Fatalf("save configuration: %v", err)
	}

	reader := NewManager(path)
	if err := reader.Load(); err != nil {
		t.Fatalf("reload configuration: %v", err)
	}
	loaded := reader.GetConfig()
	if loaded == nil {
		t.Fatal("expected reloaded configuration")
	}
	if loaded.System.UserAgent != "UniMap persistence test" || loaded.Screenshot.Mode != "extension" {
		t.Fatalf("operational configuration was not preserved: %#v", loaded)
	}
	if len(loaded.Notifications.Channels) != 1 || loaded.Notifications.Channels[0].ID != "audit-log" || loaded.Notifications.Channels[0].Headers["X-Trace"] != "persistence-test" {
		t.Fatalf("notification channel was not preserved: %#v", loaded.Notifications.Channels)
	}
}

func TestResolveEnvWithComplexValues(t *testing.T) {
	mgr := NewManager("test.yaml")

	// Set up test environment variables
	os.Setenv("API_KEY", "secret123")
	os.Setenv("DB_PASSWORD", "password456")
	defer os.Unsetenv("API_KEY")
	defer os.Unsetenv("DB_PASSWORD")

	// Test individual env vars (ResolveEnv only handles standalone env vars)
	testCases := []struct {
		input    string
		expected string
	}{
		{"${API_KEY}", "secret123"},
		{"$DB_PASSWORD", "password456"},
		{"regular_text", "regular_text"},
		{"${NON_EXISTENT}", ""}, // 2026-07-12: unresolved env var returns empty
		{"$NON_EXISTENT", ""},
	}

	for _, tc := range testCases {
		result := mgr.ResolveEnv(tc.input)
		assert.Equal(t, tc.expected, result, "Failed for input: %s", tc.input)
	}
}

func TestClone_NilReceiver(t *testing.T) {
	var c *Config
	cloned := c.Clone()
	assert.Nil(t, cloned)
}

func TestClone_DeepCopySlices(t *testing.T) {
	mgr := NewManager("test.yaml")
	var cfg Config
	mgr.applyDefaults(&cfg)
	cfg.Engines.Quake.Cookies = []Cookie{{Name: "session", Value: "abc", Domain: "example.com"}}
	cfg.Web.CORS.AllowedOrigins = []string{"http://a.com", "http://b.com"}
	cfg.Network.ProxyPool.Proxies = []string{"http://proxy1", "http://proxy2"}

	cloned := cfg.Clone()

	assert.Equal(t, cfg.Engines.Quake.Cookies, cloned.Engines.Quake.Cookies)
	assert.Equal(t, cfg.Web.CORS.AllowedOrigins, cloned.Web.CORS.AllowedOrigins)
	assert.Equal(t, cfg.Network.ProxyPool.Proxies, cloned.Network.ProxyPool.Proxies)

	cloned.Engines.Quake.Cookies[0].Value = "modified"
	cloned.Web.CORS.AllowedOrigins[0] = "http://hacked.com"
	assert.NotEqual(t, cfg.Engines.Quake.Cookies[0].Value, cloned.Engines.Quake.Cookies[0].Value)
	assert.NotEqual(t, cfg.Web.CORS.AllowedOrigins[0], cloned.Web.CORS.AllowedOrigins[0])
}

func TestClone_DeepCopyMaps(t *testing.T) {
	mgr := NewManager("test.yaml")
	var cfg Config
	mgr.applyDefaults(&cfg)
	cfg.Distributed.NodeAuthTokens = map[string]string{"node1": "token1", "node2": "token2"}
	cfg.Cache.Engines = map[string]EngineCacheConfig{"quake": {Enabled: true, TTL: 600}}

	cloned := cfg.Clone()

	assert.Equal(t, cfg.Distributed.NodeAuthTokens, cloned.Distributed.NodeAuthTokens)
	assert.Equal(t, cfg.Cache.Engines, cloned.Cache.Engines)

	cloned.Distributed.NodeAuthTokens["node1"] = "hacked"
	assert.NotEqual(t, cfg.Distributed.NodeAuthTokens["node1"], cloned.Distributed.NodeAuthTokens["node1"])
}

func TestClone_DeepCopyPointers(t *testing.T) {
	mgr := NewManager("test.yaml")
	var cfg Config
	mgr.applyDefaults(&cfg)
	fb := true
	hl := false
	cfg.Screenshot.Fallback = &fb
	cfg.Screenshot.Headless = &hl

	cloned := cfg.Clone()

	assert.Equal(t, *cfg.Screenshot.Fallback, *cloned.Screenshot.Fallback)
	assert.Equal(t, *cfg.Screenshot.Headless, *cloned.Screenshot.Headless)

	fbClone := false
	cloned.Screenshot.Fallback = &fbClone
	assert.NotEqual(t, *cfg.Screenshot.Fallback, *cloned.Screenshot.Fallback)
}

func TestClone_PreservesNotificationAndOperationalConfig(t *testing.T) {
	var cfg Config
	cfg.Alerting.Webhook.Enabled = true
	cfg.ICP.DatabasePath = "icp.db"
	cfg.Backup.Sources = []string{"data", "configs"}
	cfg.History.DatabasePath = "history.db"
	cfg.Notifications.Enabled = true
	cfg.Notifications.FeishuApp = &struct {
		AppID     string `yaml:"app_id"`
		AppSecret string `yaml:"app_secret"`
		ChatID    string `yaml:"chat_id"`
	}{AppID: "app", AppSecret: "secret", ChatID: "chat"}
	cfg.Notifications.Channels = []NotificationChannelCfg{{
		ID: "feishu-app", Type: "feishu_app", Enabled: true,
		Headers: map[string]string{"X-Mode": "live"},
	}}
	cfg.Tamper.PortScanEnabled = true

	cloned := cfg.Clone()

	assert.Equal(t, cfg.Alerting, cloned.Alerting)
	assert.Equal(t, cfg.ICP, cloned.ICP)
	assert.Equal(t, cfg.Backup, cloned.Backup)
	assert.Equal(t, cfg.History, cloned.History)
	assert.Equal(t, cfg.Notifications, cloned.Notifications)
	assert.Equal(t, cfg.Tamper, cloned.Tamper)

	cloned.Backup.Sources[0] = "changed"
	cloned.Notifications.FeishuApp.AppID = "changed"
	cloned.Notifications.Channels[0].Headers["X-Mode"] = "changed"

	assert.Equal(t, "data", cfg.Backup.Sources[0])
	assert.Equal(t, "app", cfg.Notifications.FeishuApp.AppID)
	assert.Equal(t, "live", cfg.Notifications.Channels[0].Headers["X-Mode"])
}

func TestClone_NilVsEmptySlice(t *testing.T) {
	// Use a raw Config (no defaults) to test nil slice preservation
	var cfg Config
	cfg.Network.ProxyPool.Proxies = nil

	cloned := cfg.Clone()
	assert.Nil(t, cloned.Network.ProxyPool.Proxies)

	// Empty slice should stay empty
	cfg.Network.ProxyPool.Proxies = []string{}
	cloned2 := cfg.Clone()
	assert.NotNil(t, cloned2.Network.ProxyPool.Proxies)
	assert.Empty(t, cloned2.Network.ProxyPool.Proxies)
}
