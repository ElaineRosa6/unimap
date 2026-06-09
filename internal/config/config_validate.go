package config

import (
	"fmt"
	"strings"

	"github.com/unimap/project/internal/utils/urlguard"
)

// validate 验证配置有效性
func (m *Manager) validate(config *Config) error {
	// 验证引擎配置
	if config.Engines.Quake.Enabled {
		if config.Engines.Quake.BaseURL == "" {
			return fmt.Errorf("quake engine enabled but base_url not set")
		}
		if config.Engines.Quake.QPS <= 0 {
			return fmt.Errorf("quake engine qps must be greater than 0")
		}
		if config.Engines.Quake.Timeout <= 0 {
			return fmt.Errorf("quake engine timeout must be greater than 0")
		}
	}

	if config.Engines.Zoomeye.Enabled {
		if config.Engines.Zoomeye.BaseURL == "" {
			return fmt.Errorf("zoomeye engine enabled but base_url not set")
		}
		if config.Engines.Zoomeye.QPS <= 0 {
			return fmt.Errorf("zoomeye engine qps must be greater than 0")
		}
		if config.Engines.Zoomeye.Timeout <= 0 {
			return fmt.Errorf("zoomeye engine timeout must be greater than 0")
		}
	}

	if config.Engines.Hunter.Enabled {
		if config.Engines.Hunter.BaseURL == "" {
			return fmt.Errorf("hunter engine enabled but base_url not set")
		}
		if config.Engines.Hunter.QPS <= 0 {
			return fmt.Errorf("hunter engine qps must be greater than 0")
		}
		if config.Engines.Hunter.Timeout <= 0 {
			return fmt.Errorf("hunter engine timeout must be greater than 0")
		}
	}

	if config.Engines.Fofa.Enabled {
		if !config.Engines.Fofa.UseWebAPI {
			if config.Engines.Fofa.APIKey == "" || config.Engines.Fofa.Email == "" {
				return fmt.Errorf("fofa engine enabled but api_key or email not set")
			}
		}
		if config.Engines.Fofa.APIBaseURL == "" {
			return fmt.Errorf("fofa engine enabled but api_base_url not set")
		}
		if _, err := urlguard.Check(config.Engines.Fofa.APIBaseURL, urlguard.CheckOptions{
			AllowPrivate: config.Engines.Fofa.AllowPrivateAPI,
		}); err != nil {
			return fmt.Errorf("fofa api_base_url validation failed: %w", err)
		}
		if config.Engines.Fofa.QPS <= 0 {
			return fmt.Errorf("fofa engine qps must be greater than 0")
		}
		if config.Engines.Fofa.Timeout <= 0 {
			return fmt.Errorf("fofa engine timeout must be greater than 0")
		}
	}

	if config.Engines.Daydaymap.Enabled {
		if config.Engines.Daydaymap.BaseURL == "" {
			return fmt.Errorf("daydaymap engine enabled but base_url not set")
		}
		if config.Engines.Daydaymap.QPS <= 0 {
			return fmt.Errorf("daydaymap engine qps must be greater than 0")
		}
		if config.Engines.Daydaymap.Timeout <= 0 {
			return fmt.Errorf("daydaymap engine timeout must be greater than 0")
		}
	}

	if config.Engines.Binaryedge.Enabled {
		if config.Engines.Binaryedge.BaseURL == "" {
			return fmt.Errorf("binaryedge engine enabled but base_url not set")
		}
		if config.Engines.Binaryedge.QPS <= 0 {
			return fmt.Errorf("binaryedge engine qps must be greater than 0")
		}
		if config.Engines.Binaryedge.Timeout <= 0 {
			return fmt.Errorf("binaryedge engine timeout must be greater than 0")
		}
	}

	if config.Engines.Onyphe.Enabled {
		if config.Engines.Onyphe.BaseURL == "" {
			return fmt.Errorf("onyphe engine enabled but base_url not set")
		}
		if config.Engines.Onyphe.QPS <= 0 {
			return fmt.Errorf("onyphe engine qps must be greater than 0")
		}
		if config.Engines.Onyphe.Timeout <= 0 {
			return fmt.Errorf("onyphe engine timeout must be greater than 0")
		}
	}

	// 验证系统配置
	if config.System.MaxConcurrent <= 0 {
		return fmt.Errorf("system max_concurrent must be greater than 0")
	}
	if config.System.CacheTTL <= 0 {
		return fmt.Errorf("system cache_ttl must be greater than 0")
	}
	if config.System.CacheMaxSize <= 0 {
		return fmt.Errorf("system cache_max_size must be greater than 0")
	}
	if config.System.CacheCleanupInterval <= 0 {
		return fmt.Errorf("system cache_cleanup_interval must be greater than 0")
	}
	if config.System.RetryAttempts < 0 {
		return fmt.Errorf("system retry_attempts must be greater than or equal to 0")
	}
	if config.System.UserAgent == "" {
		return fmt.Errorf("system user_agent must be set")
	}

	// 验证 Web 配置
	if config.Web.CORS.MaxAge < 0 {
		return fmt.Errorf("web cors max_age must be greater than or equal to 0")
	}
	if config.Web.RateLimit.RequestsPerWindow <= 0 {
		return fmt.Errorf("web rate_limit requests_per_window must be greater than 0")
	}
	if config.Web.RateLimit.WindowSeconds <= 0 {
		return fmt.Errorf("web rate_limit window_seconds must be greater than 0")
	}
	if config.Web.RequestLimits.MaxBodyBytes <= 0 {
		return fmt.Errorf("web request_limits max_body_bytes must be greater than 0")
	}
	if config.Web.RequestLimits.MaxMultipartMemory <= 0 {
		return fmt.Errorf("web request_limits max_multipart_memory_bytes must be greater than 0")
	}

	engine := strings.ToLower(strings.TrimSpace(config.Screenshot.Engine))
	if engine != "" && engine != "cdp" && engine != "extension" {
		return fmt.Errorf("screenshot engine must be one of: cdp, extension")
	}

	mode := strings.ToLower(strings.TrimSpace(config.Screenshot.Mode))
	if mode != "auto" && mode != "cdp" && mode != "extension" {
		return fmt.Errorf("screenshot mode must be one of: auto, cdp, extension")
	}

	priority := strings.ToLower(strings.TrimSpace(config.Screenshot.Priority))
	if priority != "" && priority != "cdp" && priority != "extension" {
		return fmt.Errorf("screenshot priority must be one of: cdp, extension")
	}
	if config.Screenshot.Extension.TokenTTLSeconds <= 0 {
		return fmt.Errorf("screenshot extension token_ttl_seconds must be greater than 0")
	}
	if config.Screenshot.Extension.TaskTimeoutSeconds <= 0 {
		return fmt.Errorf("screenshot extension task_timeout_seconds must be greater than 0")
	}
	if config.Screenshot.Extension.MaxConcurrency <= 0 {
		return fmt.Errorf("screenshot extension max_concurrency must be greater than 0")
	}
	if config.Screenshot.Extension.CallbackSignatureSkewSeconds <= 0 {
		return fmt.Errorf("screenshot extension callback_signature_skew_seconds must be greater than 0")
	}
	if config.Screenshot.Extension.CallbackNonceTTLSeconds <= 0 {
		return fmt.Errorf("screenshot extension callback_nonce_ttl_seconds must be greater than 0")
	}
	if config.Screenshot.Extension.CallbackNonceTTLSeconds < config.Screenshot.Extension.CallbackSignatureSkewSeconds {
		return fmt.Errorf("screenshot extension callback_nonce_ttl_seconds must be greater than or equal to callback_signature_skew_seconds")
	}

	if config.Network.ProxyPool.Enabled {
		strategy := strings.ToLower(strings.TrimSpace(config.Network.ProxyPool.Strategy))
		if strategy != "round_robin" {
			return fmt.Errorf("network proxy_pool strategy must be: round_robin")
		}
		if len(config.Network.ProxyPool.Proxies) == 0 {
			return fmt.Errorf("network proxy_pool enabled but proxies are not set")
		}
		if config.Network.ProxyPool.FailureThreshold <= 0 {
			return fmt.Errorf("network proxy_pool failure_threshold must be greater than 0")
		}
		if config.Network.ProxyPool.CooldownSeconds <= 0 {
			return fmt.Errorf("network proxy_pool cooldown_seconds must be greater than 0")
		}
	}

	if config.Distributed.HeartbeatTimeoutSeconds <= 0 {
		return fmt.Errorf("distributed heartbeat_timeout_seconds must be greater than 0")
	}
	if config.Distributed.MaxReassignAttempts < 0 {
		return fmt.Errorf("distributed max_reassign_attempts must be greater than or equal to 0")
	}
	if config.Distributed.MaxReassignAttempts > 10 {
		return fmt.Errorf("distributed max_reassign_attempts must be less than or equal to 10")
	}
	strategy := strings.ToLower(strings.TrimSpace(config.Distributed.Scheduler.Strategy))
	if strategy != "health_load" {
		return fmt.Errorf("distributed scheduler strategy must be: health_load")
	}

	backend := strings.ToLower(strings.TrimSpace(config.Cache.Backend))
	if backend != "memory" && backend != "redis" {
		return fmt.Errorf("cache backend must be one of: memory, redis")
	}
	if backend == "redis" && strings.TrimSpace(config.Cache.Redis.Addr) == "" {
		return fmt.Errorf("cache redis addr must be set when backend is redis")
	}

	// 分布式安全校验：启用但未配置 token 时告警
	if config.Distributed.Enabled {
		if strings.TrimSpace(config.Distributed.AdminToken) == "" {
			// 不阻塞启动，但记录严重警告
			// 实际运行时 requireDistributedAdminToken 会返回 503
		}
		if len(config.Distributed.NodeAuthTokens) == 0 {
			// 同上：节点 token 为空时运行时拒绝注册
		}
	}

	// 验证浏览器降级引擎白名单
	validBFEngines := map[string]bool{"fofa": true, "zoomeye": true, "shodan": true, "hunter": true, "quake": true, "censys": true, "daydaymap": true, "binaryedge": true, "onyphe": true}
	for _, e := range config.Query.BrowserFallback.Engines {
		if !validBFEngines[strings.ToLower(e)] {
			return fmt.Errorf("query.browser_fallback.engines: unknown engine %q, must be one of: fofa, zoomeye, shodan, hunter, quake", e)
		}
	}

	return nil
}

