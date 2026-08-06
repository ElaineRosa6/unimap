package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/unimap/project/internal/appversion"
	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/logger"
)

// handleHealthReady 就绪检查：依赖连接正常
func (s *Server) handleHealthReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cfg := s.healthConfigSnapshot()
	checks := make(map[string]string)
	required := map[string]bool{
		"engines":     configuredEngineRequired(cfg),
		"scheduler":   cfg != nil && cfg.Scheduler.Enabled,
		"distributed": cfg != nil && cfg.Distributed.Enabled,
		"icp_db":      cfg != nil && cfg.ICP.Enabled,
		"screenshot":  cfg != nil && cfg.Screenshot.Enabled,
		"history_db":  cfg != nil && cfg.History.Enabled,
		"user_db":     cfg != nil && cfg.Web.Auth.Enabled,
	}
	ready := true

	// 检查 orchestrator
	if s.orchestrator != nil {
		adapters := s.orchestrator.ListAdapters()
		missing := missingConfiguredEngines(cfg, adapters)
		if len(missing) > 0 {
			checks["engines"] = "missing enabled adapters: " + strings.Join(missing, ",")
			ready = false
		} else if len(adapters) > 0 {
			checks["engines"] = fmt.Sprintf("ok (%d adapters)", len(adapters))
		} else {
			checks["engines"] = "no adapters configured"
			if required["engines"] {
				ready = false
			}
		}
	} else {
		checks["engines"] = "not initialized"
		if required["engines"] {
			ready = false
		}
	}

	// 检查调度器
	if s.scheduler != nil {
		checks["scheduler"] = "ok"
	} else {
		checks["scheduler"] = "not initialized"
		if required["scheduler"] {
			ready = false
		}
	}

	// 检查分布式组件
	if s.distributed != nil && s.distributed.NodeRegistry != nil {
		checks["distributed"] = "ok"
	} else {
		checks["distributed"] = "not initialized"
		if required["distributed"] {
			ready = false
		}
	}

	// 检查 ICP 数据库
	if s.icpDB != nil {
		if err := s.icpDB.DB().PingContext(r.Context()); err != nil {
			logger.Warnf("readiness ICP database check failed: %v", err)
			checks["icp_db"] = "unavailable"
			ready = false
		} else {
			checks["icp_db"] = "ok"
		}
	} else {
		checks["icp_db"] = "not configured"
		if required["icp_db"] {
			ready = false
		}
	}

	// 检查截图路由
	if s.screenshotRouter != nil {
		cdpHealthy, extHealthy := s.screenshotRouter.HealthStatus()
		if !s.screenshotRouter.Ready() {
			checks["screenshot"] = fmt.Sprintf("degraded (mode=%s, cdp=%v, ext=%v)", s.screenshotRouter.ConfiguredMode(), cdpHealthy, extHealthy)
			if required["screenshot"] {
				ready = false
			}
		} else {
			checks["screenshot"] = fmt.Sprintf("ok (configured=%s, active=%s, cdp=%v, ext=%v)", s.screenshotRouter.ConfiguredMode(), s.screenshotRouter.ActiveMode(), cdpHealthy, extHealthy)
		}
	} else {
		checks["screenshot"] = "not configured"
		if required["screenshot"] {
			ready = false
		}
	}

	// 检查代理池
	if s.proxyPool != nil {
		if s.proxyPool.Enabled() {
			checks["proxy_pool"] = "ok"
		} else {
			checks["proxy_pool"] = "configured but disabled"
		}
	} else {
		checks["proxy_pool"] = "not configured"
	}

	if s.historyDB != nil {
		if err := s.historyDB.DB().PingContext(r.Context()); err != nil {
			logger.Warnf("readiness history database check failed: %v", err)
			checks["history_db"] = "unavailable"
			if required["history_db"] {
				ready = false
			}
		} else {
			checks["history_db"] = "ok"
		}
	} else {
		checks["history_db"] = "not configured"
		if required["history_db"] {
			ready = false
		}
	}
	if s.userDB != nil {
		if err := s.userDB.DB().PingContext(r.Context()); err != nil {
			logger.Warnf("readiness user database check failed: %v", err)
			checks["user_db"] = "unavailable"
			if required["user_db"] {
				ready = false
			}
		} else {
			checks["user_db"] = "ok"
		}
	} else {
		checks["user_db"] = "not configured"
		if required["user_db"] {
			ready = false
		}
	}

	status := "ok"
	if !ready {
		status = "degraded"
	}

	resp := struct {
		Status   string          `json:"status"`
		Version  string          `json:"version"`
		Time     string          `json:"time"`
		Checks   interface{}     `json:"checks,omitempty"`
		Required map[string]bool `json:"required"`
	}{
		Status:   status,
		Version:  appversion.Full(),
		Time:     time.Now().UTC().Format(time.RFC3339),
		Checks:   checks,
		Required: required,
	}

	if status != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) healthConfigSnapshot() *config.Config {
	return s.currentConfig()
}

func configuredEngineRequired(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return cfg.Engines.Fofa.Enabled || cfg.Engines.Hunter.Enabled || cfg.Engines.Zoomeye.Enabled ||
		cfg.Engines.Quake.Enabled || cfg.Engines.Shodan.Enabled || cfg.Engines.Censys.Enabled || cfg.Engines.Daydaymap.Enabled
}

func missingConfiguredEngines(cfg *config.Config, adapters []string) []string {
	if cfg == nil {
		return nil
	}
	available := make(map[string]struct{}, len(adapters))
	for _, name := range adapters {
		available[strings.ToLower(name)] = struct{}{}
	}
	enabled := []struct {
		name    string
		enabled bool
	}{
		{"fofa", cfg.Engines.Fofa.Enabled},
		{"hunter", cfg.Engines.Hunter.Enabled},
		{"zoomeye", cfg.Engines.Zoomeye.Enabled},
		{"quake", cfg.Engines.Quake.Enabled},
		{"shodan", cfg.Engines.Shodan.Enabled},
		{"censys", cfg.Engines.Censys.Enabled},
		{"daydaymap", cfg.Engines.Daydaymap.Enabled},
	}
	missing := make([]string, 0)
	for _, engine := range enabled {
		if !engine.enabled {
			continue
		}
		if _, ok := available[engine.name]; !ok {
			missing = append(missing, engine.name)
		}
	}
	return missing
}

// handleHealthLive 存活检查：进程是否存活
func (s *Server) handleHealthLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Status  string `json:"status"`
		Version string `json:"version"`
		Time    string `json:"time"`
	}{
		Status:  "ok",
		Version: appversion.Full(),
		Time:    time.Now().UTC().Format(time.RFC3339),
	})
}

// livenessCheck 返回 context 是否已被取消（用于关闭检测）
func livenessCheck(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	default:
		return true
	}
}
