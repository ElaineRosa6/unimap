package config

import (
	"fmt"
	"strings"

	"github.com/unimap/project/internal/utils/urlguard"
)

// validate 验证配置有效性
func (m *Manager) validate(config *Config) error {
	if err := validateEngines(config); err != nil {
		return err
	}
	if err := validateSystemConfig(config); err != nil {
		return err
	}
	if err := validateWebConfig(config); err != nil {
		return err
	}
	if err := validateScreenshotConfig(config); err != nil {
		return err
	}
	if err := validateNetworkConfig(config); err != nil {
		return err
	}
	if err := validateDistributedConfig(config); err != nil {
		return err
	}
	if err := validateCacheConfig(config); err != nil {
		return err
	}
	if err := validateBrowserFallbackConfig(config); err != nil {
		return err
	}
	return nil
}

// validateEngines 验证所有引擎配置
func validateEngines(config *Config) error {
	type engineCheck struct {
		enabled bool
		baseURL string
		qps     int
		timeout int
		name    string
		extra   func() error
	}
	checks := []engineCheck{
		{config.Engines.Quake.Enabled, config.Engines.Quake.BaseURL, config.Engines.Quake.QPS, config.Engines.Quake.Timeout, "quake", nil},
		{config.Engines.Zoomeye.Enabled, config.Engines.Zoomeye.BaseURL, config.Engines.Zoomeye.QPS, config.Engines.Zoomeye.Timeout, "zoomeye", nil},
		{config.Engines.Hunter.Enabled, config.Engines.Hunter.BaseURL, config.Engines.Hunter.QPS, config.Engines.Hunter.Timeout, "hunter", nil},
		{config.Engines.Daydaymap.Enabled, config.Engines.Daydaymap.BaseURL, config.Engines.Daydaymap.QPS, config.Engines.Daydaymap.Timeout, "daydaymap", nil},
		{config.Engines.Shodan.Enabled, config.Engines.Shodan.BaseURL, config.Engines.Shodan.QPS, config.Engines.Shodan.Timeout, "shodan", nil},
		{config.Engines.Censys.Enabled, config.Engines.Censys.BaseURL, config.Engines.Censys.QPS, config.Engines.Censys.Timeout, "censys", nil},
	}
	for _, c := range checks {
		if !c.enabled {
			continue
		}
		if err := validateEngineBasics(c.name, c.baseURL, c.qps, c.timeout); err != nil {
			return err
		}
		if c.extra != nil {
			if err := c.extra(); err != nil {
				return err
			}
		}
	}
	// FOFA 特殊处理（有 APIKey/Email 和 URL 校验）
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
		if err := validateEngineBasics("fofa", config.Engines.Fofa.APIBaseURL, config.Engines.Fofa.QPS, config.Engines.Fofa.Timeout); err != nil {
			return err
		}
	}
	return nil
}

// validateEngineBasics 验证引擎基础配置（baseURL、QPS、Timeout）
func validateEngineBasics(name, baseURL string, qps, timeout int) error {
	if baseURL == "" {
		return fmt.Errorf("%s engine enabled but base_url not set", name)
	}
	if qps <= 0 {
		return fmt.Errorf("%s engine qps must be greater than 0", name)
	}
	if timeout <= 0 {
		return fmt.Errorf("%s engine timeout must be greater than 0", name)
	}
	return nil
}

// validateSystemConfig 验证系统配置
func validateSystemConfig(config *Config) error {
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
	return nil
}

// validateWebConfig 验证 Web 配置
func validateWebConfig(config *Config) error {
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
	return nil
}

// validateScreenshotConfig 验证截图配置
func validateScreenshotConfig(config *Config) error {
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
	ext := config.Screenshot.Extension
	if ext.TokenTTLSeconds <= 0 {
		return fmt.Errorf("screenshot extension token_ttl_seconds must be greater than 0")
	}
	if ext.TaskTimeoutSeconds <= 0 {
		return fmt.Errorf("screenshot extension task_timeout_seconds must be greater than 0")
	}
	if ext.MaxConcurrency <= 0 {
		return fmt.Errorf("screenshot extension max_concurrency must be greater than 0")
	}
	if ext.CallbackSignatureSkewSeconds <= 0 {
		return fmt.Errorf("screenshot extension callback_signature_skew_seconds must be greater than 0")
	}
	if ext.CallbackNonceTTLSeconds <= 0 {
		return fmt.Errorf("screenshot extension callback_nonce_ttl_seconds must be greater than 0")
	}
	if ext.CallbackNonceTTLSeconds < ext.CallbackSignatureSkewSeconds {
		return fmt.Errorf("screenshot extension callback_nonce_ttl_seconds must be greater than or equal to callback_signature_skew_seconds")
	}
	return nil
}

// validateNetworkConfig 验证网络配置
func validateNetworkConfig(config *Config) error {
	if !config.Network.ProxyPool.Enabled {
		return nil
	}
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
	return nil
}

// validateDistributedConfig 验证分布式配置
func validateDistributedConfig(config *Config) error {
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
	return nil
}

// validateCacheConfig 验证缓存配置
func validateCacheConfig(config *Config) error {
	backend := strings.ToLower(strings.TrimSpace(config.Cache.Backend))
	if backend != "memory" && backend != "redis" {
		return fmt.Errorf("cache backend must be one of: memory, redis")
	}
	if backend == "redis" && strings.TrimSpace(config.Cache.Redis.Addr) == "" {
		return fmt.Errorf("cache redis addr must be set when backend is redis")
	}
	return nil
}

// validateBrowserFallbackConfig 验证浏览器降级配置
func validateBrowserFallbackConfig(config *Config) error {
	validEngines := map[string]bool{
		"fofa": true, "zoomeye": true, "shodan": true, "hunter": true,
		"quake": true, "censys": true, "daydaymap": true,
	}
	for _, e := range config.Query.BrowserFallback.Engines {
		if !validEngines[strings.ToLower(e)] {
			return fmt.Errorf("query.browser_fallback.engines: unknown engine %q, must be one of: fofa, zoomeye, shodan, hunter, quake", e)
		}
	}
	return nil
}
