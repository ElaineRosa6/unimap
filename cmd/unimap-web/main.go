package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/appversion"
	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/service"
	"github.com/unimap/project/internal/utils"
	"github.com/unimap/project/web"
)

const configPath = "configs/config.yaml"

func main() {
	defer logger.Sync()

	// 初始化日志系统
	logger.Init(logger.Config{
		Level:    logger.LevelInfo,
		Encoding: "console",
		File:     "",
	})

	// 加载配置
	cfgManager := config.NewManager(configPath)
	if err := cfgManager.Load(); err != nil {
		logger.Warnf("Failed to load config from %s: %v", configPath, err)
	}
	cfg := cfgManager.GetConfig()

	// 创建统一服务
	svc := service.NewUnifiedServiceWithConfig(cfg)

	// 注册引擎适配器
	if cfg != nil {
		registerEngines(svc, cfg)
	}

	// 从服务中获取编排器
	orchestrator := svc.GetOrchestrator()

	// 从配置读取端口和绑定地址
	port := 8448
	bindAddr := "0.0.0.0"
	if cfg != nil {
		if cfg.Web.Port != 0 {
			port = cfg.Web.Port
		}
		if cfg.Web.BindAddress != "" {
			bindAddr = cfg.Web.BindAddress
		}
	}

	// 创建Web服务器
	server, err := web.NewServer(port, svc, orchestrator, cfg, cfgManager)
	if err != nil {
		logger.Errorf("Failed to initialize Web server: %v", err)
		os.Exit(1)
	}

	// 创建优雅关闭管理器
	shutdownManager := utils.NewShutdownManager(30 * time.Second)

	// 注册关闭处理函数
	shutdownManager.RegisterHandler(func(ctx context.Context) error {
		logger.Info("Shutting down Web server...")
		return server.Shutdown(ctx)
	})

	shutdownManager.RegisterHandler(func(ctx context.Context) error {
		logger.Info("Shutting down service...")
		return svc.Shutdown()
	})

	// 启动优雅关闭监听
	shutdownManager.Start()

	// 启动Web服务器（在goroutine中运行，不阻塞）
	go func() {
		fmt.Printf("Starting Web server %s on %s:%d...\n", appversion.Short(), bindAddr, port)
		if err := server.Start(); err != nil {
			logger.Errorf("Web server error: %v", err)
			shutdownManager.Shutdown()
		}
	}()

	// 等待关闭信号
	shutdownManager.Wait()
	logger.Info("Application stopped gracefully")
}

// registerEngines 注册引擎适配器
func registerEngines(svc *service.UnifiedService, cfg *config.Config) {
	type engineEntry struct {
		enabled  bool
		hasCreds bool
		regAPI   func()
		regWeb   func()
		name     string
	}
	engines := []engineEntry{
		{cfg.Engines.Fofa.Enabled, cfg.Engines.Fofa.APIKey != "",
			func() {
				svc.RegisterAdapter(adapter.NewFofaAdapter(cfg.Engines.Fofa.APIBaseURL, cfg.Engines.Fofa.APIKey, cfg.Engines.Fofa.Email, cfg.Engines.Fofa.QPS, time.Duration(cfg.Engines.Fofa.Timeout)*time.Second))
			},
			func() { svc.RegisterAdapter(adapter.NewFofaAdapterWebOnly()) }, "FOFA"},
		{cfg.Engines.Hunter.Enabled, cfg.Engines.Hunter.APIKey != "",
			func() {
				svc.RegisterAdapter(adapter.NewHunterAdapter(cfg.Engines.Hunter.BaseURL, cfg.Engines.Hunter.APIKey, cfg.Engines.Hunter.QPS, time.Duration(cfg.Engines.Hunter.Timeout)*time.Second))
			},
			func() { svc.RegisterAdapter(adapter.NewHunterAdapterWebOnly()) }, "Hunter"},
		{cfg.Engines.Zoomeye.Enabled, cfg.Engines.Zoomeye.APIKey != "",
			func() {
				svc.RegisterAdapter(adapter.NewZoomEyeAdapter(cfg.Engines.Zoomeye.BaseURL, cfg.Engines.Zoomeye.APIKey, cfg.Engines.Zoomeye.QPS, time.Duration(cfg.Engines.Zoomeye.Timeout)*time.Second))
			},
			func() { svc.RegisterAdapter(adapter.NewZoomEyeAdapterWebOnly()) }, "ZoomEye"},
		{cfg.Engines.Quake.Enabled, cfg.Engines.Quake.APIKey != "",
			func() {
				svc.RegisterAdapter(adapter.NewQuakeAdapter(cfg.Engines.Quake.BaseURL, cfg.Engines.Quake.APIKey, cfg.Engines.Quake.QPS, time.Duration(cfg.Engines.Quake.Timeout)*time.Second))
			},
			func() { svc.RegisterAdapter(adapter.NewQuakeAdapterWebOnly()) }, "Quake"},
		{cfg.Engines.Shodan.Enabled, cfg.Engines.Shodan.APIKey != "",
			func() {
				svc.RegisterAdapter(adapter.NewShodanAdapter(cfg.Engines.Shodan.BaseURL, cfg.Engines.Shodan.APIKey, cfg.Engines.Shodan.QPS, time.Duration(cfg.Engines.Shodan.Timeout)*time.Second))
			},
			func() { svc.RegisterAdapter(adapter.NewShodanAdapterWebOnly()) }, "Shodan"},
		{cfg.Engines.Censys.Enabled, cfg.Engines.Censys.APIID != "" && cfg.Engines.Censys.APISecret != "",
			func() {
				svc.RegisterAdapter(adapter.NewCensysAdapter(cfg.Engines.Censys.BaseURL, cfg.Engines.Censys.APIID, cfg.Engines.Censys.APISecret, cfg.Engines.Censys.QPS, time.Duration(cfg.Engines.Censys.Timeout)*time.Second))
			},
			func() { svc.RegisterAdapter(adapter.NewCensysAdapterWebOnly()) }, "Censys"},
		{cfg.Engines.Daydaymap.Enabled, cfg.Engines.Daydaymap.APIKey != "",
			func() {
				svc.RegisterAdapter(adapter.NewDayDayMapAdapter(cfg.Engines.Daydaymap.BaseURL, cfg.Engines.Daydaymap.APIKey, cfg.Engines.Daydaymap.QPS, time.Duration(cfg.Engines.Daydaymap.Timeout)*time.Second))
			},
			func() { svc.RegisterAdapter(adapter.NewDayDayMapAdapterWebOnly()) }, "DayDayMap"},
	}
	for _, e := range engines {
		if !e.enabled {
			continue
		}
		if e.hasCreds {
			e.regAPI()
			logger.Infof("%s engine registered (API mode)", e.name)
		} else {
			e.regWeb()
			logger.Infof("%s engine registered (Web-only mode)", e.name)
		}
	}
}
