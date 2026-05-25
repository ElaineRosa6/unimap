package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/unimap-icp-hunter/project/internal/config"
	"github.com/unimap-icp-hunter/project/internal/logger"
	"github.com/unimap-icp-hunter/project/internal/notify"
)

func (s *Server) handleNotificationChannels(w http.ResponseWriter, r *http.Request) {
	if s.config == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "config not loaded", nil)
		return
	}

	s.configMutex.Lock()
	channels := s.config.Notifications.Channels
	s.configMutex.Unlock()

	infos := make([]map[string]interface{}, len(channels))
	for i, ch := range channels {
		infos[i] = map[string]interface{}{
			"id":      ch.ID,
			"type":    ch.Type,
			"enabled": ch.Enabled,
		}
	}

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

	s.reloadNotifyChannels()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}

func (s *Server) reloadNotifyChannels() {
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
}

// handleNotifyChannelSave handles POST /api/notifications/channels — create or update a channel.
func (s *Server) handleNotifyChannelSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}
	if s.config == nil || s.configManager == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "config not available", nil)
		return
	}

	var req struct {
		ID             string            `json:"id"`
		Type           string            `json:"type"`
		Enabled        bool              `json:"enabled"`
		WebhookURL     string            `json:"webhook_url"`
		Secret         string            `json:"secret"`
		Headers        map[string]string `json:"headers"`
		AllowPrivateIP bool              `json:"allow_private_ip"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}

	req.ID = strings.TrimSpace(req.ID)
	req.Type = strings.TrimSpace(req.Type)
	req.WebhookURL = strings.TrimSpace(req.WebhookURL)

	if req.ID == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_id", "channel id is required", nil)
		return
	}
	if req.Type == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_type", "channel type is required", nil)
		return
	}
	validTypes := map[string]bool{"dingtalk": true, "feishu": true, "wecom": true, "webhook": true, "log": true}
	if !validTypes[req.Type] {
		writeAPIError(w, http.StatusBadRequest, "invalid_type",
			"unsupported channel type", map[string]string{"type": req.Type})
		return
	}
	if req.Type != "log" && req.WebhookURL == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_webhook_url", "webhook_url is required for this channel type", nil)
		return
	}

	s.configMutex.Lock()

	// Find existing channel and update, or append new one
	found := false
	for i := range s.config.Notifications.Channels {
		if s.config.Notifications.Channels[i].ID == req.ID {
			secret := req.Secret
			if secret == "" {
				secret = s.config.Notifications.Channels[i].Secret // preserve existing
			}
			s.config.Notifications.Channels[i] = config.NotificationChannelCfg{
				ID: req.ID, Type: req.Type, Enabled: req.Enabled,
				WebhookURL: req.WebhookURL, Secret: secret,
				Headers: req.Headers, AllowPrivateIP: req.AllowPrivateIP,
			}
			found = true
			break
		}
	}
	if !found {
		s.config.Notifications.Channels = append(s.config.Notifications.Channels,
			config.NotificationChannelCfg{
				ID: req.ID, Type: req.Type, Enabled: req.Enabled,
				WebhookURL: req.WebhookURL, Secret: req.Secret,
				Headers: req.Headers, AllowPrivateIP: req.AllowPrivateIP,
			})
	}

	saveErr := s.configManager.Save()
	s.configMutex.Unlock()

	if saveErr != nil {
		logger.Warnf("notify channel save failed: %v", saveErr)
		writeAPIError(w, http.StatusInternalServerError, "save_failed", "failed to persist config: "+sanitizeError(saveErr.Error()), nil)
		return
	}

	s.reloadNotifyChannels()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      req.ID,
		"message": "channel saved",
	})
}

// handleNotifyChannelDelete handles DELETE /api/notifications/channels — delete a channel.
func (s *Server) handleNotifyChannelDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}
	if s.config == nil || s.configManager == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "config not available", nil)
		return
	}

	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_id", "channel id is required", nil)
		return
	}

	s.configMutex.Lock()

	removed := false
	newChannels := make([]config.NotificationChannelCfg, 0, len(s.config.Notifications.Channels))
	for _, ch := range s.config.Notifications.Channels {
		if ch.ID == id {
			removed = true
			continue
		}
		newChannels = append(newChannels, ch)
	}
	if !removed {
		s.configMutex.Unlock()
		writeAPIError(w, http.StatusNotFound, "not_found", "channel not found", map[string]string{"id": id})
		return
	}
	s.config.Notifications.Channels = newChannels

	saveErr := s.configManager.Save()
	s.configMutex.Unlock()

	if saveErr != nil {
		logger.Warnf("notify channel delete failed: %v", saveErr)
		writeAPIError(w, http.StatusInternalServerError, "save_failed", "failed to persist config: "+sanitizeError(saveErr.Error()), nil)
		return
	}

	s.reloadNotifyChannels()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
		"message": "channel deleted",
	})
}

// handleNotifyChannelTest handles POST /api/notifications/channels/test — send a test message.
func (s *Server) handleNotifyChannelTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}

	var req struct {
		ID             string            `json:"id"`
		Type           string            `json:"type"`
		WebhookURL     string            `json:"webhook_url"`
		Secret         string            `json:"secret"`
		Headers        map[string]string `json:"headers"`
		AllowPrivateIP bool              `json:"allow_private_ip"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}

	// If webhook_url is missing, look up the channel in saved config
	if req.WebhookURL == "" {
		s.configMutex.Lock()
		for _, ch := range s.config.Notifications.Channels {
			if ch.ID == req.ID {
				req.Type = ch.Type
				req.WebhookURL = ch.WebhookURL
				req.Secret = ch.Secret
				req.AllowPrivateIP = ch.AllowPrivateIP
				req.Headers = ch.Headers
				break
			}
		}
		s.configMutex.Unlock()
	}

	if req.Type == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_type", "channel type is required", nil)
		return
	}
	if req.WebhookURL == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_webhook_url", "webhook_url is required — either provide it in the request or save the channel first", nil)
		return
	}

	// Build a temporary channel to test
	chCfg := notify.ChannelConfig{
		ID:             req.ID,
		Type:           req.Type,
		Enabled:        true,
		WebhookURL:     req.WebhookURL,
		Secret:         req.Secret,
		Headers:        req.Headers,
		AllowPrivateIP: req.AllowPrivateIP,
	}

	ch, err := notify.NewChannelFromConfig(chCfg)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "create_channel_failed", err.Error(), nil)
		return
	}
	defer ch.Close()

	testMsg := notify.TaskNotification{
		TaskID:   "test-" + req.ID,
		TaskName: "测试消息",
		TaskType: "system",
		Status:   "success",
		Result:   "通知渠道测试成功",
	}

	if err := ch.Send(r.Context(), testMsg); err != nil {
		writeAPIError(w, http.StatusBadGateway, "send_failed", "test message failed: "+sanitizeError(err.Error()), nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "test message sent successfully",
	})
}
