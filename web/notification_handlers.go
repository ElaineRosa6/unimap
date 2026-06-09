package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/notify"
	"github.com/unimap/project/internal/service"
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
		if ch.Type == "feishu_app" {
			infos[i]["app_id"] = ch.AppID
			infos[i]["chat_id"] = ch.ChatID
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
			AppID:          cc.AppID,
			AppSecret:      cc.AppSecret,
			ChatID:         cc.ChatID,
			Headers:        cc.Headers,
			AllowPrivateIP: cc.AllowPrivateIP,
		})
	}

	if s.notifyRegistry != nil {
		s.notifyRegistry.Reload(chanCfgs)
	}
}

// reloadEngineAdapters re-registers all engine adapters from the current config.
// This allows quota and API queries to work immediately after saving API keys
// without restarting the server.
func (s *Server) reloadEngineAdapters() {
	if s.orchestrator == nil || s.config == nil {
		return
	}
	for _, name := range []string{"fofa", "hunter", "zoomeye", "quake", "shodan"} {
		s.orchestrator.UnregisterAdapter(name)
	}
	s.registerCoreEngineAdapters()
	if provider := s.browserQueryProvider(); provider != nil {
		s.orchestrator.SetWebOnlyBrowserBackend(&browserBackendAdapter{provider: provider})
	}
	s.reloadBrowserFallbackConfig()
}

// registerCoreEngineAdapters 注册 5 个核心引擎适配器
func (s *Server) registerCoreEngineAdapters() {
	cfg := s.config
	type engineReg struct {
		enabled bool
		apiKey  string
		regAPI  func()
		regWeb  func()
		name    string
	}
	engines := []engineReg{
		{cfg.Engines.Fofa.Enabled, cfg.Engines.Fofa.APIKey,
			func() { s.orchestrator.RegisterAdapter(adapter.NewFofaAdapter(cfg.Engines.Fofa.APIBaseURL, cfg.Engines.Fofa.APIKey, cfg.Engines.Fofa.Email, cfg.Engines.Fofa.QPS, time.Duration(cfg.Engines.Fofa.Timeout)*time.Second)) },
			func() { s.orchestrator.RegisterAdapter(adapter.NewFofaAdapterWebOnly()) }, "FOFA"},
		{cfg.Engines.Hunter.Enabled, cfg.Engines.Hunter.APIKey,
			func() { s.orchestrator.RegisterAdapter(adapter.NewHunterAdapter(cfg.Engines.Hunter.BaseURL, cfg.Engines.Hunter.APIKey, cfg.Engines.Hunter.QPS, time.Duration(cfg.Engines.Hunter.Timeout)*time.Second)) },
			func() { s.orchestrator.RegisterAdapter(adapter.NewHunterAdapterWebOnly()) }, "Hunter"},
		{cfg.Engines.Zoomeye.Enabled, cfg.Engines.Zoomeye.APIKey,
			func() { s.orchestrator.RegisterAdapter(adapter.NewZoomEyeAdapter(cfg.Engines.Zoomeye.BaseURL, cfg.Engines.Zoomeye.APIKey, cfg.Engines.Zoomeye.QPS, time.Duration(cfg.Engines.Zoomeye.Timeout)*time.Second)) },
			func() { s.orchestrator.RegisterAdapter(adapter.NewZoomEyeAdapterWebOnly()) }, "ZoomEye"},
		{cfg.Engines.Quake.Enabled, cfg.Engines.Quake.APIKey,
			func() { s.orchestrator.RegisterAdapter(adapter.NewQuakeAdapter(cfg.Engines.Quake.BaseURL, cfg.Engines.Quake.APIKey, cfg.Engines.Quake.QPS, time.Duration(cfg.Engines.Quake.Timeout)*time.Second)) },
			func() { s.orchestrator.RegisterAdapter(adapter.NewQuakeAdapterWebOnly()) }, "Quake"},
		{cfg.Engines.Shodan.Enabled, cfg.Engines.Shodan.APIKey,
			func() { s.orchestrator.RegisterAdapter(adapter.NewShodanAdapter(cfg.Engines.Shodan.BaseURL, cfg.Engines.Shodan.APIKey, cfg.Engines.Shodan.QPS, time.Duration(cfg.Engines.Shodan.Timeout)*time.Second)) },
			func() { s.orchestrator.RegisterAdapter(adapter.NewShodanAdapterWebOnly()) }, "Shodan"},
	}
	for _, e := range engines {
		if !e.enabled {
			continue
		}
		if e.apiKey != "" {
			e.regAPI()
			logger.Infof("%s engine re-registered (API mode)", e.name)
		} else {
			e.regWeb()
			logger.Infof("%s engine re-registered (Web-only mode)", e.name)
		}
	}
}

// reloadBrowserFallbackConfig 重载浏览器降级配置
func (s *Server) reloadBrowserFallbackConfig() {
	if s.service == nil || s.config == nil {
		return
	}
	if s.config.Query.BrowserFallback.Enabled {
		bfEngines := make(map[string]bool)
		for _, e := range s.config.Query.BrowserFallback.Engines {
			bfEngines[strings.ToLower(e)] = true
		}
		s.service.SetBrowserFallbackConfig(service.BrowserFallbackConfig{
			Enabled: true, OnAPIError: s.config.Query.BrowserFallback.OnAPIError,
			OnEmptyResult: s.config.Query.BrowserFallback.OnEmptyResult, Engines: bfEngines,
		})
	} else {
		s.service.SetBrowserFallbackConfig(service.BrowserFallbackConfig{Enabled: false})
	}
}

// notifyChannelSaveRequest is the JSON body for handleNotifyChannelSave.
type notifyChannelSaveRequest struct {
	ID             string            `json:"id"`
	Type           string            `json:"type"`
	Enabled        bool              `json:"enabled"`
	WebhookURL     string            `json:"webhook_url"`
	Secret         string            `json:"secret"`
	AppID          string            `json:"app_id"`
	AppSecret      string            `json:"app_secret"`
	ChatID         string            `json:"chat_id"`
	Headers        map[string]string `json:"headers"`
	AllowPrivateIP bool              `json:"allow_private_ip"`
}

// parseNotifyChannelSaveRequest decodes, trims, and validates the channel save request.
func parseNotifyChannelSaveRequest(w http.ResponseWriter, r *http.Request) (notifyChannelSaveRequest, bool) {
	var req notifyChannelSaveRequest
	if !decodeJSONBody(w, r, &req) {
		return req, false
	}

	req.ID = strings.TrimSpace(req.ID)
	req.Type = strings.TrimSpace(req.Type)
	req.WebhookURL = strings.TrimSpace(req.WebhookURL)
	req.AppID = strings.TrimSpace(req.AppID)
	req.AppSecret = strings.TrimSpace(req.AppSecret)
	req.ChatID = strings.TrimSpace(req.ChatID)

	if req.ID == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_id", "channel id is required", nil)
		return req, false
	}
	if req.Type == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_type", "channel type is required", nil)
		return req, false
	}
	validTypes := map[string]bool{"dingtalk": true, "feishu": true, "feishu_app": true, "wecom": true, "webhook": true, "log": true}
	if !validTypes[req.Type] {
		writeAPIError(w, http.StatusBadRequest, "invalid_type",
			"unsupported channel type", map[string]string{"type": req.Type})
		return req, false
	}
	if req.Type == "feishu_app" {
		if req.AppID == "" || req.AppSecret == "" || req.ChatID == "" {
			writeAPIError(w, http.StatusBadRequest, "missing_feishu_app_params",
				"feishu_app requires app_id, app_secret, and chat_id", nil)
			return req, false
		}
	} else if req.Type != "log" && req.WebhookURL == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_webhook_url", "webhook_url is required for this channel type", nil)
		return req, false
	}
	return req, true
}

// upsertNotifyChannel inserts or updates a channel in the config. Must be called with configMutex held.
func (s *Server) upsertNotifyChannel(req notifyChannelSaveRequest) {
	for i := range s.config.Notifications.Channels {
		if s.config.Notifications.Channels[i].ID == req.ID {
			secret := req.Secret
			if secret == "" {
				secret = s.config.Notifications.Channels[i].Secret
			}
			appSecret := req.AppSecret
			if appSecret == "" {
				appSecret = s.config.Notifications.Channels[i].AppSecret
			}
			s.config.Notifications.Channels[i] = config.NotificationChannelCfg{
				ID: req.ID, Type: req.Type, Enabled: req.Enabled,
				WebhookURL: req.WebhookURL, Secret: secret,
				AppID: req.AppID, AppSecret: appSecret, ChatID: req.ChatID,
				Headers: req.Headers, AllowPrivateIP: req.AllowPrivateIP,
			}
			return
		}
	}
	s.config.Notifications.Channels = append(s.config.Notifications.Channels,
		config.NotificationChannelCfg{
			ID: req.ID, Type: req.Type, Enabled: req.Enabled,
			WebhookURL: req.WebhookURL, Secret: req.Secret,
			AppID: req.AppID, AppSecret: req.AppSecret, ChatID: req.ChatID,
			Headers: req.Headers, AllowPrivateIP: req.AllowPrivateIP,
		})
}

// handleNotifyChannelSave handles POST /api/v1/notifications/channels — create or update a channel.
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

	req, ok := parseNotifyChannelSaveRequest(w, r)
	if !ok {
		return
	}

	s.configMutex.Lock()
	s.upsertNotifyChannel(req)
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

// handleNotifyChannelDelete handles DELETE /api/v1/notifications/channels — delete a channel.
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

// notifyChannelTestRequest is the JSON body for handleNotifyChannelTest.
type notifyChannelTestRequest struct {
	ID             string            `json:"id"`
	Type           string            `json:"type"`
	WebhookURL     string            `json:"webhook_url"`
	Secret         string            `json:"secret"`
	AppID          string            `json:"app_id"`
	AppSecret      string            `json:"app_secret"`
	ChatID         string            `json:"chat_id"`
	Headers        map[string]string `json:"headers"`
	AllowPrivateIP bool              `json:"allow_private_ip"`
}

// resolveNotifyChannelTestRequest decodes the test request and fills missing fields from saved config.
func (s *Server) resolveNotifyChannelTestRequest(w http.ResponseWriter, r *http.Request) (notifyChannelTestRequest, bool) {
	var req notifyChannelTestRequest
	if !decodeJSONBody(w, r, &req) {
		return req, false
	}

	needLookup := req.WebhookURL == "" || req.Secret == "" || req.AppID == "" || req.AppSecret == "" || req.ChatID == ""
	if needLookup {
		s.configMutex.Lock()
		for _, ch := range s.config.Notifications.Channels {
			if ch.ID == req.ID {
				s.fillTestRequestFromChannel(&req, ch)
				break
			}
		}
		s.configMutex.Unlock()
	}

	if req.Type == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_type", "channel type is required", nil)
		return req, false
	}
	if req.Type == "feishu_app" {
		if req.AppID == "" || req.AppSecret == "" || req.ChatID == "" {
			writeAPIError(w, http.StatusBadRequest, "missing_feishu_app_params",
				"feishu_app test requires app_id, app_secret, and chat_id — either provide them or save the channel first", nil)
			return req, false
		}
	} else if req.WebhookURL == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_webhook_url", "webhook_url is required — either provide it in the request or save the channel first", nil)
		return req, false
	}
	return req, true
}

// fillTestRequestFromChannel copies saved channel fields into a test request for empty fields.
func (s *Server) fillTestRequestFromChannel(req *notifyChannelTestRequest, ch config.NotificationChannelCfg) {
	if req.WebhookURL == "" {
		req.WebhookURL = ch.WebhookURL
	}
	if req.Secret == "" {
		req.Secret = ch.Secret
		logger.Infof("notify test: loaded decrypted secret for channel %q (len=%d)", req.ID, len(req.Secret))
	}
	if req.AppID == "" {
		req.AppID = ch.AppID
	}
	if req.AppSecret == "" {
		req.AppSecret = ch.AppSecret
	}
	if req.ChatID == "" {
		req.ChatID = ch.ChatID
	}
	if req.Type == "" {
		req.Type = ch.Type
	}
	req.AllowPrivateIP = ch.AllowPrivateIP
	req.Headers = ch.Headers
}

// sendTestNotification builds a temporary channel and sends a test message.
func sendTestNotification(r *http.Request, req notifyChannelTestRequest) error {
	logger.Infof("notify test: channel=%q type=%q", req.ID, req.Type)
	chCfg := notify.ChannelConfig{
		ID: req.ID, Type: req.Type, Enabled: true,
		WebhookURL: req.WebhookURL, Secret: req.Secret,
		AppID: req.AppID, AppSecret: req.AppSecret, ChatID: req.ChatID,
		Headers: req.Headers, AllowPrivateIP: req.AllowPrivateIP,
	}

	ch, err := notify.NewChannelFromConfig(chCfg)
	if err != nil {
		return err
	}
	defer ch.Close()

	return ch.Send(r.Context(), notify.TaskNotification{
		TaskID:   "test-" + req.ID,
		TaskName: "测试消息",
		TaskType: "system",
		Status:   "success",
		Result:   "通知渠道测试成功",
	})
}

// handleNotifyChannelTest handles POST /api/v1/notifications/channels/test — send a test message.
func (s *Server) handleNotifyChannelTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}

	req, ok := s.resolveNotifyChannelTestRequest(w, r)
	if !ok {
		return
	}

	if err := sendTestNotification(r, req); err != nil {
		writeAPIError(w, http.StatusBadGateway, "send_failed", "test message failed: "+sanitizeError(err.Error()), nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "test message sent successfully",
	})
}
