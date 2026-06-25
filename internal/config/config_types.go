package config

// Config 系统配置结构
type Config struct {
	Engines struct {
		Quake struct {
			Enabled bool     `yaml:"enabled"`
			APIKey  string   `yaml:"api_key"`
			BaseURL string   `yaml:"base_url"`
			QPS     int      `yaml:"qps"`
			Timeout int      `yaml:"timeout"`
			Cookies []Cookie `yaml:"cookies"`
		} `yaml:"quake"`
		Zoomeye struct {
			Enabled bool     `yaml:"enabled"`
			APIKey  string   `yaml:"api_key"`
			BaseURL string   `yaml:"base_url"`
			QPS     int      `yaml:"qps"`
			Timeout int      `yaml:"timeout"`
			Cookies []Cookie `yaml:"cookies"`
		} `yaml:"zoomeye"`
		Hunter struct {
			Enabled bool     `yaml:"enabled"`
			APIKey  string   `yaml:"api_key"`
			BaseURL string   `yaml:"base_url"`
			QPS     int      `yaml:"qps"`
			Timeout int      `yaml:"timeout"`
			Cookies []Cookie `yaml:"cookies"`
		} `yaml:"hunter"`
		Fofa struct {
			Enabled         bool     `yaml:"enabled"`
			APIKey          string   `yaml:"api_key"`
			Email           string   `yaml:"email"`
			BaseURL         string   `yaml:"base_url,omitempty"` // 废弃，保留兼容
			APIBaseURL      string   `yaml:"api_base_url"`       // API 模式使用
			WebBaseURL      string   `yaml:"web_base_url"`       // Web 模式使用（本期锁死官方域名）
			QPS             int      `yaml:"qps"`
			Timeout         int      `yaml:"timeout"`
			UseWebAPI       bool     `yaml:"use_web_api"`
			Cookies         []Cookie `yaml:"cookies"`
			AllowPrivateAPI bool     `yaml:"allow_private_api"` // 高级配置，仅限内网自建镜像
		} `yaml:"fofa"`
		Shodan struct {
			Enabled bool   `yaml:"enabled"`
			APIKey  string `yaml:"api_key"`
			BaseURL string `yaml:"base_url"`
			QPS     int    `yaml:"qps"`
			Timeout int    `yaml:"timeout"`
		} `yaml:"shodan"`
		Censys struct {
			Enabled   bool   `yaml:"enabled"`
			APIID     string `yaml:"api_id"`
			APISecret string `yaml:"api_secret"`
			BaseURL   string `yaml:"base_url"`
			QPS       int    `yaml:"qps"`
			Timeout   int    `yaml:"timeout"`
		} `yaml:"censys"`
		Daydaymap struct {
			Enabled bool   `yaml:"enabled"`
			APIKey  string `yaml:"api_key"`
			BaseURL string `yaml:"base_url"`
			QPS     int    `yaml:"qps"`
			Timeout int    `yaml:"timeout"`
		} `yaml:"daydaymap"`
	} `yaml:"engines"`

	// 系统配置
	System struct {
		MaxConcurrent        int    `yaml:"max_concurrent"`
		CacheTTL             int    `yaml:"cache_ttl"`
		CacheMaxSize         int    `yaml:"cache_max_size"`
		CacheCleanupInterval int    `yaml:"cache_cleanup_interval"`
		RetryAttempts        int    `yaml:"retry_attempts"`
		UserAgent            string `yaml:"user_agent"`
	} `yaml:"system"`

	// 日志配置
	Log struct {
		Level    string `yaml:"level"`    // debug, info, warn, error, fatal
		Encoding string `yaml:"encoding"` // console, json
		File     string `yaml:"file"`     // 可选的日志文件路径
	} `yaml:"log"`

	// 截图配置
	Screenshot struct {
		Enabled                  bool   `yaml:"enabled"`
		Engine                   string `yaml:"engine"`   // legacy: "cdp" or "extension"
		Mode                     string `yaml:"mode"`     // new: "auto"|"cdp"|"extension"
		Priority                 string `yaml:"priority"` // new: "cdp"|"extension" (for auto mode)
		Fallback                 *bool  `yaml:"fallback"` // new: explicit fallback toggle
		BaseDir                  string `yaml:"base_dir"`
		ChromePath               string `yaml:"chrome_path"`
		ProxyServer              string `yaml:"proxy_server"`
		ChromeUserDataDir        string `yaml:"chrome_user_data_dir"`
		ChromeProfileDir         string `yaml:"chrome_profile_dir"`
		ChromeRemoteDebugURL     string `yaml:"chrome_remote_debug_url"`
		ChromeRemoteDebugAddress string `yaml:"chrome_remote_debug_address"`
		Extension                struct {
			Enabled                      bool   `yaml:"enabled"`
			ListenAddr                   string `yaml:"listen_addr"`
			PairingRequired              bool   `yaml:"pairing_required"`
			PairCode                     string `yaml:"pair_code"` // optional, if set pairing must provide matching pair_code
			TokenTTLSeconds              int    `yaml:"token_ttl_seconds"`
			TaskTimeoutSeconds           int    `yaml:"task_timeout_seconds"`
			MaxConcurrency               int    `yaml:"max_concurrency"`
			CallbackSignatureRequired    bool   `yaml:"callback_signature_required"` // default: true for signature verification
			CallbackSignatureSkewSeconds int    `yaml:"callback_signature_skew_seconds"`
			CallbackNonceTTLSeconds      int    `yaml:"callback_nonce_ttl_seconds"`
			FallbackToCDP                bool   `yaml:"fallback_to_cdp"`
		} `yaml:"extension"`
		Headless     *bool `yaml:"headless"`
		Timeout      int   `yaml:"timeout"`
		WindowWidth  int   `yaml:"window_width"`
		WindowHeight int   `yaml:"window_height"`
		WaitTime     int   `yaml:"wait_time"`
		// 自动截图配置
		AutoCapture struct {
			Enabled              bool `yaml:"enabled"`
			CaptureSearchResults bool `yaml:"capture_search_results"`
			CaptureTargets       bool `yaml:"capture_targets"`
		} `yaml:"auto_capture"`
	} `yaml:"screenshot"`

	// Web 配置
	Web struct {
		Port        int    `yaml:"port"`         // 监听端口
		BindAddress string `yaml:"bind_address"` // 监听地址
		CORS        struct {
			AllowedOrigins      []string `yaml:"allowed_origins"`
			AllowedMethods      []string `yaml:"allowed_methods"`
			AllowedHeaders      []string `yaml:"allowed_headers"`
			ExposedHeaders      []string `yaml:"exposed_headers"`
			AllowCredentials    bool     `yaml:"allow_credentials"`
			MaxAge              int      `yaml:"max_age"`
			AllowedExtensionIDs []string `yaml:"allowed_extension_ids"` // chrome-extension:// 允许的扩展 ID，空表示全部允许（向后兼容）
		} `yaml:"cors"`
		RateLimit struct {
			Enabled           bool `yaml:"enabled"`
			RequestsPerWindow int  `yaml:"requests_per_window"`
			WindowSeconds     int  `yaml:"window_seconds"`
		} `yaml:"rate_limit"`
		RequestLimits struct {
			MaxBodyBytes       int64 `yaml:"max_body_bytes"`
			MaxMultipartMemory int64 `yaml:"max_multipart_memory_bytes"`
		} `yaml:"request_limits"`
		Auth struct {
			Enabled      bool   `yaml:"enabled"`       // 是否启用 Web 鉴权
			AdminToken   string `yaml:"admin_token"`   // 管理端点 token
			Username     string `yaml:"username"`      // 登录用户名
			PasswordHash string `yaml:"password_hash"` // 登录密码 bcrypt 哈希
			APIKeyStore  string `yaml:"api_key_store"` // API Key 文件路径
		} `yaml:"auth"`
	} `yaml:"web"`

	// Network 配置
	Network struct {
		ProxyPool struct {
			Enabled             bool     `yaml:"enabled"`
			Strategy            string   `yaml:"strategy"`
			Proxies             []string `yaml:"proxies"`
			FailureThreshold    int      `yaml:"failure_threshold"`
			CooldownSeconds     int      `yaml:"cooldown_seconds"`
			AllowDirectFallback bool     `yaml:"allow_direct_fallback"`
		} `yaml:"proxy_pool"`
	} `yaml:"network"`

	// Distributed 配置
	Distributed struct {
		Enabled                 bool              `yaml:"enabled"`
		HeartbeatTimeoutSeconds int               `yaml:"heartbeat_timeout_seconds"`
		MaxReassignAttempts     int               `yaml:"max_reassign_attempts"`
		AdminToken              string            `yaml:"admin_token"`
		NodeAuthTokens          map[string]string `yaml:"node_auth_tokens"`
		Scheduler               struct {
			Strategy string `yaml:"strategy"`
		} `yaml:"scheduler"`
	} `yaml:"distributed"`

	// Alerting 告警配置
	Alerting struct {
		Webhook struct {
			Enabled   bool   `yaml:"enabled"`
			URL       string `yaml:"url"`
			AuthToken string `yaml:"auth_token"`
		} `yaml:"webhook"`
		ErrorAlerting struct {
			Enabled       bool `yaml:"enabled"`
			Threshold     int  `yaml:"threshold"`      // 窗口内 ERROR 数量阈值
			WindowSeconds int  `yaml:"window_seconds"` // 滑动窗口大小（秒）
		} `yaml:"error_alerting"`
	} `yaml:"alerting"`

	// ICP 备案查询配置
	ICP struct {
		Enabled      bool   `yaml:"enabled"`       // 是否启用 ICP 查询
		BaseURL      string `yaml:"base_url"`      // sidecar 服务地址，默认 http://localhost:16181
		APIKey       string `yaml:"api_key"`       // 可选 API Key，支持 ${ENV_VAR}
		Timeout      int    `yaml:"timeout"`       // 请求超时（秒）
		DefaultType  string `yaml:"default_type"`  // web/app/mapp/kapp/bweb/bapp/bmapp/bkapp
		DatabasePath string `yaml:"database_path"` // SQLite 持久化路径，默认 ./data/icp_results.db
	} `yaml:"icp"`

	// Backup 数据备份配置
	Backup struct {
		Enabled    bool     `yaml:"enabled"`
		OutputDir  string   `yaml:"output_dir"`  // 备份输出目录
		Prefix     string   `yaml:"prefix"`      // 备份文件名前缀
		MaxBackups int      `yaml:"max_backups"` // 最大保留备份数，0=不限制
		Sources    []string `yaml:"sources"`     // 要备份的目录/文件列表
	} `yaml:"backup"`

	// History 操作历史配置
	History struct {
		Enabled      bool   `yaml:"enabled"`
		DatabasePath string `yaml:"database_path"` // SQLite 路径，默认 ./data/history.db
		MaxResults   int    `yaml:"max_results"`   // 每次查询最大保存结果数，默认 1000
	} `yaml:"history"`

	// Scheduler 定时任务配置
	Scheduler struct {
		Enabled    bool `yaml:"enabled"`
		MaxHistory int  `yaml:"max_history"` // 执行历史保留条数
	} `yaml:"scheduler"`

	// Notifications 通知配置
	Notifications struct {
		Enabled   bool `yaml:"enabled"`
		FeishuApp *struct {
			AppID     string `yaml:"app_id"`
			AppSecret string `yaml:"app_secret"`
			ChatID    string `yaml:"chat_id"`
		} `yaml:"feishu_app,omitempty"`
		Channels       []NotificationChannelCfg `yaml:"channels"`
		SendTimeoutSec int                      `yaml:"send_timeout_sec"`
		MaxRetries     int                      `yaml:"max_retries"`
	} `yaml:"notifications"`

	// 缓存配置
	Cache struct {
		Backend string `yaml:"backend"`
		Redis   struct {
			Addr     string `yaml:"addr"`
			Password string `yaml:"password"`
			DB       int    `yaml:"db"`
			Prefix   string `yaml:"prefix"`
			// 连接池配置
			PoolSize        int `yaml:"pool_size"`          // 连接池大小
			MinIdleConns    int `yaml:"min_idle_conns"`     // 最小空闲连接数
			MaxIdleConns    int `yaml:"max_idle_conns"`     // 最大空闲连接数
			MaxRetries      int `yaml:"max_retries"`        // 最大重试次数
			DialTimeout     int `yaml:"dial_timeout"`       // 连接超时（毫秒）
			ReadTimeout     int `yaml:"read_timeout"`       // 读超时（毫秒）
			WriteTimeout    int `yaml:"write_timeout"`      // 写超时（毫秒）
			PoolTimeout     int `yaml:"pool_timeout"`       // 连接池超时（毫秒）
			ConnMaxLifetime int `yaml:"conn_max_lifetime"`  // 连接最大存活时间（毫秒）
			ConnMaxIdleTime int `yaml:"conn_max_idle_time"` // 连接最大空闲时间（毫秒）
		} `yaml:"redis"`
		// 按引擎的缓存配置
		Engines map[string]EngineCacheConfig `yaml:"engines"`
	} `yaml:"cache"`

	// Query 查询配置
	Query struct {
		BrowserFallback struct {
			Enabled       bool     `yaml:"enabled"`         // 是否允许 API 失败后自动尝试浏览器采集
			OnAPIError    bool     `yaml:"on_api_error"`    // API 返回错误时是否 fallback
			OnEmptyResult bool     `yaml:"on_empty_result"` // API 返回空结果时是否 fallback
			Engines       []string `yaml:"engines"`         // 允许自动 fallback 的引擎白名单
		} `yaml:"browser_fallback"`
		} `yaml:"query"`

		// Tamper 篡改检测配置
		Tamper struct {
			PortScanEnabled    bool  `yaml:"port_scan_enabled"`     // 巡检时附带端口扫描
			PortScanTimeoutMs  int   `yaml:"port_scan_timeout_ms"`  // 单端口超时（毫秒），默认 800
			InsecureSkipVerify bool  `yaml:"insecure_skip_verify"`  // 跳过 SSL 证书验证（内网/自签证书）
		} `yaml:"tamper"`
	}

// NotificationChannelCfg 全局通知渠道配置
type NotificationChannelCfg struct {
	ID             string            `yaml:"id"`
	Type           string            `yaml:"type"`
	Enabled        bool              `yaml:"enabled"`
	WebhookURL     string            `yaml:"webhook_url"`
	Secret         string            `yaml:"secret"`
	AppID          string            `yaml:"app_id,omitempty"`
	AppSecret      string            `yaml:"app_secret,omitempty"`
	ChatID         string            `yaml:"chat_id,omitempty"`
	Headers        map[string]string `yaml:"headers"`
	AllowPrivateIP bool              `yaml:"allow_private_ip"`
}

// EngineCacheConfig 引擎级别的缓存配置
type EngineCacheConfig struct {
	Enabled bool `yaml:"enabled"`  // 是否启用缓存
	TTL     int  `yaml:"ttl"`      // 缓存时间（秒），0 表示使用全局默认
	MaxSize int  `yaml:"max_size"` // 最大缓存条目数，0 表示使用全局默认
}

// Cookie Cookie配置
type Cookie struct {
	Name     string `yaml:"name"`
	Value    string `yaml:"value"`
	Domain   string `yaml:"domain"`
	Path     string `yaml:"path"`
	HTTPOnly bool   `yaml:"http_only"`
	Secure   bool   `yaml:"secure"`
}

// Clone 克隆配置，使用手写深拷贝替代 YAML 序列化方式。
// 优点：更高效，保留 nil vs 空切片区分，不会丢失未导出字段。
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	clone := &Config{}

	// Copy engine configs with Cookie slices
	clone.Engines.Quake = c.Engines.Quake
	clone.Engines.Quake.Cookies = cloneCookies(c.Engines.Quake.Cookies)
	clone.Engines.Zoomeye = c.Engines.Zoomeye
	clone.Engines.Zoomeye.Cookies = cloneCookies(c.Engines.Zoomeye.Cookies)
	clone.Engines.Hunter = c.Engines.Hunter
	clone.Engines.Hunter.Cookies = cloneCookies(c.Engines.Hunter.Cookies)
	clone.Engines.Fofa = c.Engines.Fofa
	clone.Engines.Fofa.Cookies = cloneCookies(c.Engines.Fofa.Cookies)
	clone.Engines.Shodan = c.Engines.Shodan
	clone.Engines.Censys = c.Engines.Censys
	clone.Engines.Daydaymap = c.Engines.Daydaymap

	// System, Log are all primitives — safe to copy directly
	clone.System = c.System
	clone.Log = c.Log

	// Screenshot (has pointer fields: Fallback, Headless)
	clone.Screenshot = c.Screenshot
	if c.Screenshot.Fallback != nil {
		v := *c.Screenshot.Fallback
		clone.Screenshot.Fallback = &v
	}
	if c.Screenshot.Headless != nil {
		v := *c.Screenshot.Headless
		clone.Screenshot.Headless = &v
	}

	// Web (has slice fields: CORS)
	clone.Web = c.Web
	clone.Web.CORS.AllowedOrigins = cloneStringSlice(c.Web.CORS.AllowedOrigins)
	clone.Web.CORS.AllowedMethods = cloneStringSlice(c.Web.CORS.AllowedMethods)
	clone.Web.CORS.AllowedHeaders = cloneStringSlice(c.Web.CORS.AllowedHeaders)
	clone.Web.CORS.ExposedHeaders = cloneStringSlice(c.Web.CORS.ExposedHeaders)
	clone.Web.CORS.AllowedExtensionIDs = cloneStringSlice(c.Web.CORS.AllowedExtensionIDs)
	clone.Web.Auth = c.Web.Auth

	// Network (has slice: Proxies)
	clone.Network = c.Network
	clone.Network.ProxyPool.Proxies = cloneStringSlice(c.Network.ProxyPool.Proxies)

	// Distributed (has map: NodeAuthTokens)
	clone.Distributed = c.Distributed
	clone.Distributed.NodeAuthTokens = cloneStringMap(c.Distributed.NodeAuthTokens)

	// Scheduler
	clone.Scheduler = c.Scheduler

	// Cache (has map: Engines)
	clone.Cache = c.Cache
	clone.Cache.Engines = cloneEngineCacheMap(c.Cache.Engines)

	// Query (has slice: BrowserFallback.Engines)
	clone.Query = c.Query
	clone.Query.BrowserFallback.Engines = cloneStringSlice(c.Query.BrowserFallback.Engines)

	return clone
}

func cloneCookies(src []Cookie) []Cookie {
	if src == nil {
		return nil
	}
	out := make([]Cookie, len(src))
	copy(out, src)
	return out
}

func cloneStringSlice(src []string) []string {
	if src == nil {
		return nil
	}
	out := make([]string, len(src))
	copy(out, src)
	return out
}

func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func cloneEngineCacheMap(src map[string]EngineCacheConfig) map[string]EngineCacheConfig {
	if src == nil {
		return nil
	}
	out := make(map[string]EngineCacheConfig, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

