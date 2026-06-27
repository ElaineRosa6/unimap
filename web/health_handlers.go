package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/unimap/project/internal/appversion"
)

// handleHealthReady 就绪检查：依赖连接正常
func (s *Server) handleHealthReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	checks := make(map[string]string)
	ready := true

	// 检查 orchestrator
	if s.orchestrator != nil {
		adapters := s.orchestrator.ListAdapters()
		if len(adapters) > 0 {
			checks["engines"] = fmt.Sprintf("ok (%d adapters)", len(adapters))
		} else {
			checks["engines"] = "no adapters configured"
		}
	} else {
		checks["engines"] = "not initialized"
	}

	// 检查调度器
	if s.scheduler != nil {
		checks["scheduler"] = "ok"
	} else {
		checks["scheduler"] = "not initialized"
	}

	// 检查分布式组件
	if s.distributed != nil && s.distributed.NodeRegistry != nil {
		checks["distributed"] = "ok"
	} else {
		checks["distributed"] = "not initialized"
	}

	// 检查 ICP 数据库
	if s.icpDB != nil {
		if err := s.icpDB.DB().PingContext(r.Context()); err != nil {
			checks["icp_db"] = fmt.Sprintf("unavailable: %v", err)
			ready = false
		} else {
			checks["icp_db"] = "ok"
		}
	} else {
		checks["icp_db"] = "not configured"
	}

	// 检查截图路由
	if s.screenshotRouter != nil {
		cdpHealthy, extHealthy := s.screenshotRouter.HealthStatus()
		if !cdpHealthy && !extHealthy {
			checks["screenshot"] = "degraded (no healthy backend)"
			ready = false
		} else {
			mode := s.screenshotRouter.ActiveMode()
			checks["screenshot"] = fmt.Sprintf("ok (mode=%s, cdp=%v, ext=%v)", mode, cdpHealthy, extHealthy)
		}
	} else {
		checks["screenshot"] = "not configured"
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

	status := "ok"
	if !ready {
		status = "degraded"
	}

	resp := struct {
		Status  string      `json:"status"`
		Version string      `json:"version"`
		Time    string      `json:"time"`
		Checks  interface{} `json:"checks,omitempty"`
	}{
		Status:  status,
		Version: appversion.Full(),
		Time:    time.Now().UTC().Format(time.RFC3339),
		Checks:  checks,
	}

	if status != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(resp)
}

// handleHealthLive 存活检查：进程是否存活
func (s *Server) handleHealthLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
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
