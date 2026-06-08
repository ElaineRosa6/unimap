package config

import (
	"fmt"
	"strings"

	"github.com/unimap/project/internal/logger"
)

// applyEngineDefaults 应用搜索引擎默认配置
func (m *Manager) applyEngineDefaults(config *Config) {
	config.Engines.Quake.Enabled = true
	config.Engines.Zoomeye.Enabled = true
	config.Engines.Hunter.Enabled = true
	config.Engines.Fofa.Enabled = true
	config.Engines.Shodan.Enabled = true
	config.Engines.Censys.Enabled = true
	config.Engines.Daydaymap.Enabled = true
	config.Engines.Binaryedge.Enabled = true
	config.Engines.Onyphe.Enabled = true
	config.Engines.Greynoise.Enabled = true
	config.Engines.Fofa.UseWebAPI = true

	// Quake
	if config.Engines.Quake.BaseURL == "" {
		config.Engines.Quake.BaseURL = "https://quake.360.net/api"
	}
	if config.Engines.Quake.QPS == 0 {
		config.Engines.Quake.QPS = 5
	}
	if config.Engines.Quake.Timeout == 0 {
		config.Engines.Quake.Timeout = 30
	}

	// ZoomEye
	if config.Engines.Zoomeye.BaseURL == "" {
		config.Engines.Zoomeye.BaseURL = "https://api.zoomeye.org"
	}
	if config.Engines.Zoomeye.QPS == 0 {
		config.Engines.Zoomeye.QPS = 3
	}
	if config.Engines.Zoomeye.Timeout == 0 {
		config.Engines.Zoomeye.Timeout = 30
	}

	// Hunter
	if config.Engines.Hunter.BaseURL == "" {
		config.Engines.Hunter.BaseURL = "https://hunter.qianxin.com"
	}
	if config.Engines.Hunter.QPS == 0 {
		config.Engines.Hunter.QPS = 5
	}
	if config.Engines.Hunter.Timeout == 0 {
		config.Engines.Hunter.Timeout = 30
	}

	// FOFA
	if config.Engines.Fofa.APIBaseURL == "" && config.Engines.Fofa.BaseURL != "" {
		config.Engines.Fofa.APIBaseURL = config.Engines.Fofa.BaseURL
		logger.Warnf("fofa.base_url 已迁移到 fofa.api_base_url，请更新 config.yaml")
	}
	if config.Engines.Fofa.APIBaseURL == "" {
		config.Engines.Fofa.APIBaseURL = "https://fofa.info"
	}
	if config.Engines.Fofa.WebBaseURL != "" && config.Engines.Fofa.WebBaseURL != "https://fofa.info" {
		logger.Warnf("fofa.web_base_url 已强制重置为官方域名 https://fofa.info，Web/截图/扩展模式请勿修改")
	}
	config.Engines.Fofa.WebBaseURL = "https://fofa.info"
	if config.Engines.Fofa.QPS == 0 {
		config.Engines.Fofa.QPS = 3
	}
	if config.Engines.Fofa.Timeout == 0 {
		config.Engines.Fofa.Timeout = 30
	}

	// Shodan
	if config.Engines.Shodan.BaseURL == "" {
		config.Engines.Shodan.BaseURL = "https://api.shodan.io"
	}
	if config.Engines.Shodan.QPS == 0 {
		config.Engines.Shodan.QPS = 1
	}
	if config.Engines.Shodan.Timeout == 0 {
		config.Engines.Shodan.Timeout = 30
	}

	// Censys
	if config.Engines.Censys.BaseURL == "" {
		config.Engines.Censys.BaseURL = "https://search.censys.io"
	}
	if config.Engines.Censys.QPS == 0 {
		config.Engines.Censys.QPS = 2
	}
	if config.Engines.Censys.Timeout == 0 {
		config.Engines.Censys.Timeout = 30
	}

	// DayDayMap
	if config.Engines.Daydaymap.BaseURL == "" {
		config.Engines.Daydaymap.BaseURL = "https://www.daydaymap.com"
	}
	if config.Engines.Daydaymap.QPS == 0 {
		config.Engines.Daydaymap.QPS = 3
	}
	if config.Engines.Daydaymap.Timeout == 0 {
		config.Engines.Daydaymap.Timeout = 30
	}

	// BinaryEdge
	if config.Engines.Binaryedge.BaseURL == "" {
		config.Engines.Binaryedge.BaseURL = "https://api.binaryedge.io"
	}
	if config.Engines.Binaryedge.QPS == 0 {
		config.Engines.Binaryedge.QPS = 2
	}
	if config.Engines.Binaryedge.Timeout == 0 {
		config.Engines.Binaryedge.Timeout = 30
	}

	// Onyphe
	if config.Engines.Onyphe.BaseURL == "" {
		config.Engines.Onyphe.BaseURL = "https://www.onyphe.io"
	}
	if config.Engines.Onyphe.QPS == 0 {
		config.Engines.Onyphe.QPS = 1
	}
	if config.Engines.Onyphe.Timeout == 0 {
		config.Engines.Onyphe.Timeout = 30
	}

	// GreyNoise
	if config.Engines.Greynoise.BaseURL == "" {
		config.Engines.Greynoise.BaseURL = "https://api.greynoise.io"
	}
	if config.Engines.Greynoise.QPS == 0 {
		config.Engines.Greynoise.QPS = 1
	}
	if config.Engines.Greynoise.Timeout == 0 {
		config.Engines.Greynoise.Timeout = 30
	}
}

// applySystemDefaults 应用系统默认配置
func (m *Manager) applySystemDefaults(config *Config) {
	if config.System.MaxConcurrent == 0 {
		config.System.MaxConcurrent = 10
	}
	if config.System.CacheTTL == 0 {
		config.System.CacheTTL = 3600
	}
	if config.System.CacheMaxSize == 0 {
		config.System.CacheMaxSize = 1000
	}
	if config.System.CacheCleanupInterval == 0 {
		config.System.CacheCleanupInterval = 300
	}
	if config.System.RetryAttempts == 0 {
		config.System.RetryAttempts = 3
	}
	if config.System.UserAgent == "" {
		config.System.UserAgent = "unimap/1.0"
	}

	if config.Log.Level == "" {
		config.Log.Level = "info"
	}
	if config.Log.Encoding == "" {
		config.Log.Encoding = "console"
	}
}

// applyScreenshotDefaults 应用截图默认配置
func (m *Manager) applyScreenshotDefaults(config *Config) {
	if config.Screenshot.Headless == nil {
		defaultHeadless := true
		config.Screenshot.Headless = &defaultHeadless
	}
	if config.Screenshot.BaseDir == "" {
		config.Screenshot.BaseDir = "./screenshots"
	}
	if strings.TrimSpace(config.Screenshot.Engine) == "" {
		config.Screenshot.Engine = "cdp"
	}

	// 解析截图模式：新字段 mode 优先，legacy engine 向后兼容
	mode := strings.ToLower(strings.TrimSpace(config.Screenshot.Mode))
	engine := strings.ToLower(strings.TrimSpace(config.Screenshot.Engine))
	if mode == "" {
		switch engine {
		case "extension":
			mode = "auto"
		default:
			mode = "cdp"
		}
	}
	config.Screenshot.Mode = mode

	// 推导 priority
	priority := strings.ToLower(strings.TrimSpace(config.Screenshot.Priority))
	if priority == "" {
		switch mode {
		case "extension":
			priority = "extension"
		default:
			priority = "cdp"
		}
	}
	config.Screenshot.Priority = priority

	// 推导 fallback
	if config.Screenshot.Fallback == nil {
		fb := true
		if mode == "cdp" || mode == "extension" {
			fb = false
		}
		config.Screenshot.Fallback = &fb
	}

	if strings.TrimSpace(config.Screenshot.Extension.ListenAddr) == "" {
		config.Screenshot.Extension.ListenAddr = "127.0.0.1:19451"
	}
	if config.Screenshot.Extension.TokenTTLSeconds == 0 {
		config.Screenshot.Extension.TokenTTLSeconds = 600
	}
	if config.Screenshot.Extension.TaskTimeoutSeconds == 0 {
		config.Screenshot.Extension.TaskTimeoutSeconds = 30
	}
	if config.Screenshot.Extension.MaxConcurrency == 0 {
		config.Screenshot.Extension.MaxConcurrency = 5
	}
	if config.Screenshot.Extension.CallbackSignatureSkewSeconds == 0 {
		config.Screenshot.Extension.CallbackSignatureSkewSeconds = 300
	}
	if config.Screenshot.Extension.CallbackNonceTTLSeconds == 0 {
		config.Screenshot.Extension.CallbackNonceTTLSeconds = 600
	}
	if !config.Screenshot.Extension.CallbackSignatureRequired {
		config.Screenshot.Extension.CallbackSignatureRequired = true
	}
	if config.Screenshot.Timeout == 0 {
		config.Screenshot.Timeout = 30
	}
	if config.Screenshot.WindowWidth == 0 {
		config.Screenshot.WindowWidth = 1365
	}
	if config.Screenshot.WindowHeight == 0 {
		config.Screenshot.WindowHeight = 768
	}
	if config.Screenshot.WaitTime == 0 {
		config.Screenshot.WaitTime = 500
	}
}

// applyWebDefaults 应用 Web 服务默认配置
func (m *Manager) applyWebDefaults(config *Config) {
	if config.Web.Port == 0 {
		config.Web.Port = 8448
	}
	if config.Web.BindAddress == "" {
		config.Web.BindAddress = "127.0.0.1"
	}
	if len(config.Web.CORS.AllowedOrigins) == 0 {
		config.Web.CORS.AllowedOrigins = []string{"http://localhost:8448", "http://127.0.0.1:8448"}
	}
	if len(config.Web.CORS.AllowedMethods) == 0 {
		config.Web.CORS.AllowedMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	}
	if len(config.Web.CORS.AllowedHeaders) == 0 {
		config.Web.CORS.AllowedHeaders = []string{"Content-Type", "Authorization", "X-Admin-Token", "X-Requested-With", "X-WebSocket-Token"}
	}
	if config.Web.CORS.MaxAge == 0 {
		config.Web.CORS.MaxAge = 600
	}
	if config.Web.RateLimit.RequestsPerWindow == 0 {
		config.Web.RateLimit.RequestsPerWindow = 60
	}
	if config.Web.RateLimit.WindowSeconds == 0 {
		config.Web.RateLimit.WindowSeconds = 60
	}
	if config.Web.RequestLimits.MaxBodyBytes == 0 {
		config.Web.RequestLimits.MaxBodyBytes = 10 * 1024 * 1024
	}
	if config.Web.RequestLimits.MaxMultipartMemory == 0 {
		config.Web.RequestLimits.MaxMultipartMemory = 10 * 1024 * 1024
	}
}

// applyAuthDefaults 应用认证默认配置（admin token + 登录凭据）
func (m *Manager) applyAuthDefaults(config *Config) {
	if strings.TrimSpace(config.Web.Auth.AdminToken) == "" {
		if config.Web.BindAddress != "127.0.0.1" && config.Web.BindAddress != "localhost" {
			config.Web.Auth.AdminToken = generateSecureToken(32)
			config.Web.Auth.Enabled = true
			if len(config.Web.Auth.AdminToken) > 4 {
				fmt.Printf("[config] Generated production admin token (bind=%s): %s***\n", config.Web.BindAddress, config.Web.Auth.AdminToken[:4])
			} else {
				fmt.Printf("[config] Generated production admin token (bind=%s): ***\n", config.Web.BindAddress)
			}
			fmt.Printf("[config] SAVE THIS TOKEN: it will not be shown again. Set 'admin_token' in your config file.\n")
		} else {
			token := generateSecureToken(32)
			config.Web.Auth.AdminToken = token
			config.Web.Auth.Enabled = true
			if len(token) > 4 {
				fmt.Printf("[config] Generated development admin token (bind=%s): %s***\n", config.Web.BindAddress, token[:4])
			} else {
				fmt.Printf("[config] Generated development admin token (bind=%s): ***\n", config.Web.BindAddress)
			}
		}
	} else if !config.Web.Auth.Enabled {
		config.Web.Auth.Enabled = true
	}

	isPublic := config.Web.BindAddress != "127.0.0.1" &&
		config.Web.BindAddress != "localhost" &&
		config.Web.BindAddress != "0.0.0.0"

	if strings.TrimSpace(config.Web.Auth.Username) == "" {
		if isPublic {
			logger.Fatalf("生产环境 (bind=%s) 禁止使用默认用户名，请在配置文件中设置 'username'", config.Web.BindAddress)
		}
		config.Web.Auth.Username = "admin"
	}
	if strings.TrimSpace(config.Web.Auth.PasswordHash) == "" {
		if isPublic {
			logger.Fatalf("生产环境 (bind=%s) 禁止使用默认密码，请在配置文件中设置 'password_hash'", config.Web.BindAddress)
		}
		hash, err := HashPassword("admin")
		if err != nil {
			fmt.Printf("[config] WARNING: failed to hash default password: %v\n", err)
		} else {
			config.Web.Auth.PasswordHash = hash
			fmt.Printf("[config] Generated default login credentials: admin/admin\n")
			fmt.Printf("[config] CHANGE THESE CREDENTIALS: set 'username' and 'password_hash' in your config file.\n")
		}
	}
}

// applyCacheDefaults 应用缓存与 Redis 默认配置
func (m *Manager) applyCacheDefaults(config *Config) {
	if strings.TrimSpace(config.Cache.Backend) == "" {
		config.Cache.Backend = "memory"
	}
	if strings.TrimSpace(config.Cache.Redis.Addr) == "" {
		config.Cache.Redis.Addr = "127.0.0.1:6379"
	}
	if strings.TrimSpace(config.Cache.Redis.Prefix) == "" {
		config.Cache.Redis.Prefix = "unimap:"
	}
	if config.Cache.Redis.PoolSize == 0 {
		config.Cache.Redis.PoolSize = 10
	}
	if config.Cache.Redis.MinIdleConns == 0 {
		config.Cache.Redis.MinIdleConns = 2
	}
	if config.Cache.Redis.MaxRetries == 0 {
		config.Cache.Redis.MaxRetries = 3
	}
	if config.Cache.Redis.DialTimeout == 0 {
		config.Cache.Redis.DialTimeout = 5000
	}
	if config.Cache.Redis.ReadTimeout == 0 {
		config.Cache.Redis.ReadTimeout = 3000
	}
	if config.Cache.Redis.WriteTimeout == 0 {
		config.Cache.Redis.WriteTimeout = 3000
	}
	if config.Cache.Redis.PoolTimeout == 0 {
		config.Cache.Redis.PoolTimeout = 4000
	}
	if config.Cache.Redis.ConnMaxLifetime == 0 {
		config.Cache.Redis.ConnMaxLifetime = 0
	}
	if config.Cache.Redis.ConnMaxIdleTime == 0 {
		config.Cache.Redis.ConnMaxIdleTime = 300000
	}

	if config.Cache.Engines == nil {
		config.Cache.Engines = make(map[string]EngineCacheConfig)
	}

	engineDefaults := map[string]EngineCacheConfig{
		"quake":      {Enabled: true, TTL: 3600, MaxSize: 500},
		"zoomeye":    {Enabled: true, TTL: 1800, MaxSize: 500},
		"hunter":     {Enabled: true, TTL: 3600, MaxSize: 500},
		"fofa":       {Enabled: true, TTL: 1800, MaxSize: 500},
		"shodan":     {Enabled: true, TTL: 7200, MaxSize: 500},
		"censys":     {Enabled: true, TTL: 7200, MaxSize: 500},
		"daydaymap":  {Enabled: true, TTL: 3600, MaxSize: 500},
		"binaryedge": {Enabled: true, TTL: 7200, MaxSize: 500},
		"onyphe":     {Enabled: true, TTL: 7200, MaxSize: 500},
		"greynoise":  {Enabled: true, TTL: 7200, MaxSize: 500},
	}

	for engine, defaultCfg := range engineDefaults {
		if _, exists := config.Cache.Engines[engine]; !exists {
			config.Cache.Engines[engine] = defaultCfg
		} else {
			cfg := config.Cache.Engines[engine]
			if cfg.TTL == 0 {
				cfg.TTL = defaultCfg.TTL
			}
			if cfg.MaxSize == 0 {
				cfg.MaxSize = defaultCfg.MaxSize
			}
			config.Cache.Engines[engine] = cfg
		}
	}
}

// applyMiscDefaults 应用网络/分布式/ICP/调度/通知等默认配置
func (m *Manager) applyMiscDefaults(config *Config) {
	// 网络代理池
	if strings.TrimSpace(config.Network.ProxyPool.Strategy) == "" {
		config.Network.ProxyPool.Strategy = "round_robin"
	}
	if config.Network.ProxyPool.FailureThreshold == 0 {
		config.Network.ProxyPool.FailureThreshold = 2
	}
	if config.Network.ProxyPool.CooldownSeconds == 0 {
		config.Network.ProxyPool.CooldownSeconds = 60
	}
	config.Network.ProxyPool.Proxies = normalizeProxyList(config.Network.ProxyPool.Proxies)

	// 分布式
	if config.Distributed.HeartbeatTimeoutSeconds == 0 {
		config.Distributed.HeartbeatTimeoutSeconds = 30
	}
	if config.Distributed.MaxReassignAttempts == 0 {
		config.Distributed.MaxReassignAttempts = 1
	}
	if strings.TrimSpace(config.Distributed.Scheduler.Strategy) == "" {
		config.Distributed.Scheduler.Strategy = "health_load"
	}
	if config.Distributed.NodeAuthTokens == nil {
		config.Distributed.NodeAuthTokens = make(map[string]string)
	}

	// ICP
	if strings.TrimSpace(config.ICP.BaseURL) == "" {
		config.ICP.BaseURL = "http://localhost:16181"
	}
	if config.ICP.Timeout <= 0 {
		config.ICP.Timeout = 30
	}
	if strings.TrimSpace(config.ICP.DefaultType) == "" {
		config.ICP.DefaultType = "web"
	}
	if strings.TrimSpace(config.ICP.DatabasePath) == "" {
		config.ICP.DatabasePath = "./data/icp_results.db"
	}

	// 定时任务
	if !config.Scheduler.Enabled {
		config.Scheduler.Enabled = true
	}
	if config.Scheduler.MaxHistory == 0 {
		config.Scheduler.MaxHistory = 500
	}

	// 查询降级
	if config.Query.BrowserFallback.Engines == nil {
		config.Query.BrowserFallback.Engines = []string{"fofa", "zoomeye", "shodan", "censys"}
	}

	// 通知
	if config.Notifications.SendTimeoutSec == 0 {
		config.Notifications.SendTimeoutSec = 10
	}
	if config.Notifications.MaxRetries == 0 {
		config.Notifications.MaxRetries = 0
	}
}
