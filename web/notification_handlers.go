package web

import (
	"encoding/json"
	"net/http"

	"github.com/unimap-icp-hunter/project/internal/notify"
)

func (s *Server) handleNotificationChannels(w http.ResponseWriter, r *http.Request) {
	if s.notifyRegistry == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "notification system not initialized", nil)
		return
	}

	infos := s.notifyRegistry.ListAllInfos()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"channels": infos,
	})
}

func (s *Server) handleNotifyReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.configManager == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "config manager not available", nil)
		return
	}

	cfg := s.configManager.GetConfig()
	var chanCfgs []notify.ChannelConfig
	for _, cc := range cfg.Notifications.Channels {
		chanCfgs = append(chanCfgs, notify.ChannelConfig{
			ID:             cc.ID,
			Type:           cc.Type,
			Enabled:        cc.Enabled,
			WebhookURL:     cc.WebhookURL,
			Secret:         cc.Secret,
			Headers:        cc.Headers,
			AllowPrivateIP: cc.AllowPrivateIP,
		})
	}

	if s.notifyRegistry != nil {
		s.notifyRegistry.Reload(chanCfgs)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"loaded": len(chanCfgs),
	})
}
