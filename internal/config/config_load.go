package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load 加载配置文件
func (m *Manager) Load() error {
	// 读取配置文件
	data, err := os.ReadFile(m.path)
	if err != nil {
		var cfg Config
		m.applyDefaults(&cfg)
		m.resolveEnv(&cfg)
		m.SetConfig(&cfg)
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析配置
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		var cfg Config
		m.applyDefaults(&cfg)
		m.resolveEnv(&cfg)
		m.SetConfig(&cfg)
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 应用默认值
	m.applyDefaults(&config)

	// 解析环境变量
	m.resolveEnv(&config)

	// 解密通知渠道密钥
	DecryptNotifySecrets(&config)

	// 验证配置
	if err := m.validate(&config); err != nil {
		var cfg Config
		m.applyDefaults(&cfg)
		m.resolveEnv(&cfg)
		m.SetConfig(&cfg)
		return fmt.Errorf("invalid config: %w", err)
	}

	m.SetConfig(&config)
	return nil
}

// resolveEnv 解析配置中的环境变量
func (m *Manager) resolveEnv(config *Config) {
	// 解析Quake配置
	config.Engines.Quake.APIKey = m.ResolveEnv(config.Engines.Quake.APIKey)
	config.Engines.Quake.BaseURL = m.ResolveEnv(config.Engines.Quake.BaseURL)

	// 解析ZoomEye配置
	config.Engines.Zoomeye.APIKey = m.ResolveEnv(config.Engines.Zoomeye.APIKey)
	config.Engines.Zoomeye.BaseURL = m.ResolveEnv(config.Engines.Zoomeye.BaseURL)

	// 解析Hunter配置
	config.Engines.Hunter.APIKey = m.ResolveEnv(config.Engines.Hunter.APIKey)
	config.Engines.Hunter.BaseURL = m.ResolveEnv(config.Engines.Hunter.BaseURL)

	// 解析FOFA配置
	config.Engines.Fofa.APIKey = m.ResolveEnv(config.Engines.Fofa.APIKey)
	config.Engines.Fofa.Email = m.ResolveEnv(config.Engines.Fofa.Email)
	config.Engines.Fofa.BaseURL = m.ResolveEnv(config.Engines.Fofa.BaseURL)
	config.Engines.Fofa.APIBaseURL = m.ResolveEnv(config.Engines.Fofa.APIBaseURL)
	config.Engines.Fofa.WebBaseURL = m.ResolveEnv(config.Engines.Fofa.WebBaseURL)

	// 解析Censys配置
	config.Engines.Censys.APIID = m.ResolveEnv(config.Engines.Censys.APIID)
	config.Engines.Censys.APISecret = m.ResolveEnv(config.Engines.Censys.APISecret)
	config.Engines.Censys.BaseURL = m.ResolveEnv(config.Engines.Censys.BaseURL)

	// 解析DayDayMap配置
	config.Engines.Daydaymap.APIKey = m.ResolveEnv(config.Engines.Daydaymap.APIKey)
	config.Engines.Daydaymap.BaseURL = m.ResolveEnv(config.Engines.Daydaymap.BaseURL)

	// 解析BinaryEdge配置
	config.Engines.Binaryedge.APIKey = m.ResolveEnv(config.Engines.Binaryedge.APIKey)
	config.Engines.Binaryedge.BaseURL = m.ResolveEnv(config.Engines.Binaryedge.BaseURL)

	// 解析Onyphe配置
	config.Engines.Onyphe.APIKey = m.ResolveEnv(config.Engines.Onyphe.APIKey)
	config.Engines.Onyphe.BaseURL = m.ResolveEnv(config.Engines.Onyphe.BaseURL)

	// 解析GreyNoise配置
	config.Engines.Greynoise.APIKey = m.ResolveEnv(config.Engines.Greynoise.APIKey)
	config.Engines.Greynoise.BaseURL = m.ResolveEnv(config.Engines.Greynoise.BaseURL)

	// 解析系统配置
	config.System.UserAgent = m.ResolveEnv(config.System.UserAgent)

	// 解析截图配置
	config.Screenshot.ChromePath = m.ResolveEnv(config.Screenshot.ChromePath)
	config.Screenshot.ProxyServer = m.ResolveEnv(config.Screenshot.ProxyServer)
	config.Screenshot.ChromeUserDataDir = m.ResolveEnv(config.Screenshot.ChromeUserDataDir)
	config.Screenshot.ChromeProfileDir = m.ResolveEnv(config.Screenshot.ChromeProfileDir)
	config.Screenshot.ChromeRemoteDebugURL = m.ResolveEnv(config.Screenshot.ChromeRemoteDebugURL)
	config.Screenshot.ChromeRemoteDebugAddress = m.ResolveEnv(config.Screenshot.ChromeRemoteDebugAddress)
	config.Screenshot.Engine = m.ResolveEnv(config.Screenshot.Engine)
	config.Screenshot.Mode = m.ResolveEnv(config.Screenshot.Mode)
	config.Screenshot.Priority = m.ResolveEnv(config.Screenshot.Priority)
	config.Screenshot.Extension.ListenAddr = m.ResolveEnv(config.Screenshot.Extension.ListenAddr)
	for i := range config.Network.ProxyPool.Proxies {
		config.Network.ProxyPool.Proxies[i] = m.ResolveEnv(config.Network.ProxyPool.Proxies[i])
	}
	config.Distributed.AdminToken = m.ResolveEnv(config.Distributed.AdminToken)
	config.Web.Auth.AdminToken = m.ResolveEnv(config.Web.Auth.AdminToken)

	// 解析 ICP 配置
	config.ICP.BaseURL = m.ResolveEnv(config.ICP.BaseURL)
	config.ICP.APIKey = m.ResolveEnv(config.ICP.APIKey)

	// 解析缓存配置
	config.Cache.Backend = m.ResolveEnv(config.Cache.Backend)
	config.Cache.Redis.Addr = m.ResolveEnv(config.Cache.Redis.Addr)
	config.Cache.Redis.Password = m.ResolveEnv(config.Cache.Redis.Password)
	config.Cache.Redis.Prefix = m.ResolveEnv(config.Cache.Redis.Prefix)

	// 解析通知渠道环境变量
	for i := range config.Notifications.Channels {
		config.Notifications.Channels[i].WebhookURL = m.ResolveEnv(config.Notifications.Channels[i].WebhookURL)
		config.Notifications.Channels[i].Secret = m.ResolveEnv(config.Notifications.Channels[i].Secret)
	}
}

// ResolveEnv 解析环境变量
func (m *Manager) ResolveEnv(value string) string {
	// 检查是否包含环境变量
	if strings.HasPrefix(value, "$") {
		envName := strings.TrimPrefix(value, "$")
		if envValue := os.Getenv(envName); envValue != "" {
			return envValue
		}
	}
	// 检查是否包含${}格式的环境变量
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		envName := strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}")
		if envValue := os.Getenv(envName); envValue != "" {
			return envValue
		}
	}
	return value
}

