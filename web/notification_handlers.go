package web

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/notify"
	"github.com/unimap/project/internal/service"
	"github.com/unimap/project/internal/utils/urlguard"
)

func (s *Server) handleNotificationChannels(w http.ResponseWriter, r *http.Request) {
	if s.currentConfig() == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "config not loaded", nil)
		return
	}

	cfg := s.currentConfig()
	if cfg == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "config not loaded", nil)
		return
	}
	channels := cfg.Notifications.Channels

	infos := make([]model.NotificationChannelInfo, len(channels))
	for i, ch := range channels {
		infos[i] = model.NotificationChannelInfo{
			ID:             ch.ID,
			Type:           ch.Type,
			Enabled:        ch.Enabled,
			AppID:          ch.AppID,
			ChatID:         ch.ChatID,
			AllowPrivateIP: ch.AllowPrivateIP,
		}
	}

	writeJSON(w, http.StatusOK, model.APIResponse{
		Success: true,
		Data:    map[string]any{"channels": infos},
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

	writeJSON(w, http.StatusOK, model.APIResponse{
		Success: true,
		Message: "channels reloaded",
	})
}

func (s *Server) reloadNotifyChannels() {
	cfg := s.currentConfig()
	if cfg == nil {
		return
	}
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
		s.notifyRegistry.Remove("feishu_app")
		s.notifyRegistry.Reload(chanCfgs)
		registerFeishuAppChannel(s.notifyRegistry, cfg)
	}
}

// reloadEngineAdapters re-registers all engine adapters from the current config.
// This allows quota and API queries to work immediately after saving API keys
// without restarting the server.
func (s *Server) reloadEngineAdapters() {
	cfg := s.currentConfig()
	if s.orchestrator == nil || cfg == nil {
		return
	}
	for _, name := range []string{"fofa", "hunter", "zoomeye", "quake", "shodan", "censys", "daydaymap"} {
		s.orchestrator.UnregisterAdapter(name)
	}
	s.registerCoreEngineAdapters(cfg)
	if provider := s.browserQueryProvider(); provider != nil {
		s.orchestrator.SetWebOnlyBrowserBackend(&browserBackendAdapter{provider: provider})
	}
	s.reloadBrowserFallbackConfig(cfg)
}

// registerCoreEngineAdapters 根据当前能力边界注册引擎适配器。
func (s *Server) registerCoreEngineAdapters(cfg *config.Config) {
	type engineReg struct {
		enabled     bool
		hasCreds    bool
		supportsWeb bool
		regAPI      func()
		regWeb      func()
		name        string
	}
	engines := []engineReg{
		{cfg.Engines.Fofa.Enabled, cfg.Engines.Fofa.APIKey != "", true,
			func() {
				s.orchestrator.RegisterAdapter(adapter.NewFofaAdapter(cfg.Engines.Fofa.APIBaseURL, cfg.Engines.Fofa.APIKey, cfg.Engines.Fofa.Email, cfg.Engines.Fofa.QPS, time.Duration(cfg.Engines.Fofa.Timeout)*time.Second))
			},
			func() { s.orchestrator.RegisterAdapter(adapter.NewFofaAdapterWebOnly()) }, "FOFA"},
		{cfg.Engines.Hunter.Enabled, cfg.Engines.Hunter.APIKey != "", true,
			func() {
				s.orchestrator.RegisterAdapter(adapter.NewHunterAdapter(cfg.Engines.Hunter.BaseURL, cfg.Engines.Hunter.APIKey, cfg.Engines.Hunter.QPS, time.Duration(cfg.Engines.Hunter.Timeout)*time.Second))
			},
			func() { s.orchestrator.RegisterAdapter(adapter.NewHunterAdapterWebOnly()) }, "Hunter"},
		{cfg.Engines.Zoomeye.Enabled, cfg.Engines.Zoomeye.APIKey != "", true,
			func() {
				s.orchestrator.RegisterAdapter(adapter.NewZoomEyeAdapter(cfg.Engines.Zoomeye.BaseURL, cfg.Engines.Zoomeye.APIKey, cfg.Engines.Zoomeye.QPS, time.Duration(cfg.Engines.Zoomeye.Timeout)*time.Second))
			},
			func() { s.orchestrator.RegisterAdapter(adapter.NewZoomEyeAdapterWebOnly()) }, "ZoomEye"},
		{cfg.Engines.Quake.Enabled, cfg.Engines.Quake.APIKey != "", true,
			func() {
				s.orchestrator.RegisterAdapter(adapter.NewQuakeAdapter(cfg.Engines.Quake.BaseURL, cfg.Engines.Quake.APIKey, cfg.Engines.Quake.QPS, time.Duration(cfg.Engines.Quake.Timeout)*time.Second))
			},
			func() { s.orchestrator.RegisterAdapter(adapter.NewQuakeAdapterWebOnly()) }, "Quake"},
		{cfg.Engines.Shodan.Enabled, cfg.Engines.Shodan.APIKey != "", true,
			func() {
				s.orchestrator.RegisterAdapter(adapter.NewShodanAdapter(cfg.Engines.Shodan.BaseURL, cfg.Engines.Shodan.APIKey, cfg.Engines.Shodan.QPS, time.Duration(cfg.Engines.Shodan.Timeout)*time.Second))
			},
			func() { s.orchestrator.RegisterAdapter(adapter.NewShodanAdapterWebOnly()) }, "Shodan"},
		{cfg.Engines.Censys.Enabled, cfg.Engines.Censys.APIID != "" && cfg.Engines.Censys.APISecret != "", false,
			func() {
				s.orchestrator.RegisterAdapter(adapter.NewCensysAdapter(cfg.Engines.Censys.BaseURL, cfg.Engines.Censys.APIID, cfg.Engines.Censys.APISecret, cfg.Engines.Censys.QPS, time.Duration(cfg.Engines.Censys.Timeout)*time.Second))
			},
			nil, "Censys"},
		{cfg.Engines.Daydaymap.Enabled, cfg.Engines.Daydaymap.APIKey != "", false,
			func() {
				s.orchestrator.RegisterAdapter(adapter.NewDayDayMapAdapter(cfg.Engines.Daydaymap.BaseURL, cfg.Engines.Daydaymap.APIKey, cfg.Engines.Daydaymap.QPS, time.Duration(cfg.Engines.Daydaymap.Timeout)*time.Second))
			},
			nil, "DayDayMap"},
	}
	for _, e := range engines {
		if !e.enabled {
			continue
		}
		if e.hasCreds {
			e.regAPI()
			logger.Infof("%s engine re-registered (API mode)", e.name)
		} else if e.supportsWeb {
			e.regWeb()
			logger.Infof("%s engine re-registered (Web-only mode)", e.name)
		} else {
			logger.Warnf("%s engine is enabled but requires complete API credentials; registration skipped", e.name)
		}
	}
}

// reloadBrowserFallbackConfig 重载浏览器降级配置
func (s *Server) reloadBrowserFallbackConfig(cfg *config.Config) {
	if s.service == nil || cfg == nil {
		return
	}
	if cfg.Query.BrowserFallback.Enabled {
		bfEngines := make(map[string]bool)
		for _, e := range cfg.Query.BrowserFallback.Engines {
			bfEngines[strings.ToLower(e)] = true
		}
		s.service.SetBrowserFallbackConfig(service.BrowserFallbackConfig{
			Enabled: true, OnAPIError: cfg.Query.BrowserFallback.OnAPIError,
			OnEmptyResult: cfg.Query.BrowserFallback.OnEmptyResult, Engines: bfEngines,
		})
	} else {
		s.service.SetBrowserFallbackConfig(service.BrowserFallbackConfig{Enabled: false})
	}
}

// notifyChannelSaveRequest is the JSON body for handleNotifyChannelSave.
type notifyChannelSaveRequest struct {
	ID               string            `json:"id"`
	Type             string            `json:"type"`
	Enabled          bool              `json:"enabled"`
	WebhookURL       string            `json:"webhook_url"`
	Secret           string            `json:"secret"`
	AppID            string            `json:"app_id"`
	AppSecret        string            `json:"app_secret"`
	ChatID           string            `json:"chat_id"`
	Headers          map[string]string `json:"headers"`
	AllowPrivateIP   bool              `json:"allow_private_ip"`
	PreserveExisting bool              `json:"preserve_existing"`
}

type notifyChannelInputError struct {
	status  int
	code    string
	message string
	details any
}

func (e *notifyChannelInputError) Error() string { return e.message }

func notifyChannelRequiredFieldsError(req notifyChannelSaveRequest) *notifyChannelInputError {
	if req.Type == "feishu_app" && (req.AppID == "" || req.AppSecret == "" || req.ChatID == "") {
		return &notifyChannelInputError{
			status: http.StatusBadRequest, code: "missing_feishu_app_params",
			message: "feishu_app requires app_id, app_secret, and chat_id",
		}
	}
	if req.Type != "log" && req.Type != "feishu_app" && req.WebhookURL == "" {
		return &notifyChannelInputError{
			status: http.StatusBadRequest, code: "missing_webhook_url",
			message: "webhook_url is required for this channel type",
		}
	}
	return nil
}

func validateNotifyChannelRequiredFields(w http.ResponseWriter, req notifyChannelSaveRequest) bool {
	if inputErr := notifyChannelRequiredFieldsError(req); inputErr != nil {
		writeAPIError(w, inputErr.status, inputErr.code, inputErr.message, inputErr.details)
		return false
	}
	return true
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
	if !req.PreserveExisting && !validateNotifyChannelRequiredFields(w, req) {
		return req, false
	}
	return req, true
}

func mergeExistingNotifyChannel(req *notifyChannelSaveRequest, existing config.NotificationChannelCfg) {
	if req.WebhookURL == "" {
		req.WebhookURL = existing.WebhookURL
	}
	if req.Secret == "" {
		req.Secret = existing.Secret
	}
	if req.AppID == "" {
		req.AppID = existing.AppID
	}
	if req.AppSecret == "" {
		req.AppSecret = existing.AppSecret
	}
	if req.ChatID == "" {
		req.ChatID = existing.ChatID
	}
	if req.Headers == nil {
		req.Headers = existing.Headers
	}
}

// upsertNotifyChannel inserts or updates a channel in a candidate config.
func upsertNotifyChannel(cfg *config.Config, req notifyChannelSaveRequest) {
	for i := range cfg.Notifications.Channels {
		if cfg.Notifications.Channels[i].ID == req.ID {
			secret := req.Secret
			if secret == "" {
				secret = cfg.Notifications.Channels[i].Secret
			}
			appSecret := req.AppSecret
			if appSecret == "" {
				appSecret = cfg.Notifications.Channels[i].AppSecret
			}
			cfg.Notifications.Channels[i] = config.NotificationChannelCfg{
				ID: req.ID, Type: req.Type, Enabled: req.Enabled,
				WebhookURL: req.WebhookURL, Secret: secret,
				AppID: req.AppID, AppSecret: appSecret, ChatID: req.ChatID,
				Headers: req.Headers, AllowPrivateIP: req.AllowPrivateIP,
			}
			return
		}
	}
	cfg.Notifications.Channels = append(cfg.Notifications.Channels,
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
	if !requireTrustedRequest(w, r, s.allowedOrigins()) {
		return
	}
	if s.currentConfig() == nil || s.configManager == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "config not available", nil)
		return
	}

	req, ok := parseNotifyChannelSaveRequest(w, r)
	if !ok {
		return
	}
	_, saveErr := s.updateConfig(func(cfg *config.Config) error {
		resolved := req
		if resolved.PreserveExisting {
			found := false
			for _, channel := range cfg.Notifications.Channels {
				if channel.ID == resolved.ID {
					if channel.Type != resolved.Type {
						return &notifyChannelInputError{
							status: http.StatusBadRequest, code: "channel_type_change_not_supported",
							message: "changing channel type requires deleting and recreating the channel",
						}
					}
					mergeExistingNotifyChannel(&resolved, channel)
					found = true
					break
				}
			}
			if !found {
				return &notifyChannelInputError{
					status: http.StatusNotFound, code: "not_found", message: "channel to edit was not found",
					details: map[string]string{"id": resolved.ID},
				}
			}
			if inputErr := notifyChannelRequiredFieldsError(resolved); inputErr != nil {
				return inputErr
			}
		}
		// Validate the resolved URL's scheme and literal address here. The notify
		// client's guarded dialer performs the authoritative DNS/IP check at send time.
		if resolved.Type == "webhook" && resolved.WebhookURL != "" && !resolved.AllowPrivateIP {
			if _, err := urlguard.Check(resolved.WebhookURL, urlguard.CheckOptions{
				AllowPrivate: false, AllowedSchemes: []string{"http", "https"},
			}); err != nil {
				return &notifyChannelInputError{
					status: http.StatusBadRequest, code: "blocked_webhook_url",
					message: "webhook URL is not allowed: " + sanitizeError(err.Error()),
				}
			}
		}
		upsertNotifyChannel(cfg, resolved)
		return nil
	})

	if saveErr != nil {
		var inputErr *notifyChannelInputError
		if errors.As(saveErr, &inputErr) {
			writeAPIError(w, inputErr.status, inputErr.code, inputErr.message, inputErr.details)
			return
		}
		logger.Warnf("notify channel save failed: %v", saveErr)
		writeAPIError(w, http.StatusInternalServerError, "save_failed", "failed to persist config: "+sanitizeError(saveErr.Error()), nil)
		return
	}

	s.reloadNotifyChannels()

	writeJSON(w, http.StatusOK, model.APIResponse{
		Success: true,
		Message: "channel saved",
		Data:    map[string]any{"id": req.ID},
	})
}

// handleNotifyChannelDelete handles DELETE /api/v1/notifications/channels — delete a channel.
func (s *Server) handleNotifyChannelDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireTrustedRequest(w, r, s.allowedOrigins()) {
		return
	}
	if s.currentConfig() == nil || s.configManager == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "config not available", nil)
		return
	}

	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_id", "channel id is required", nil)
		return
	}
	current := s.currentConfig()
	found := false
	for _, ch := range current.Notifications.Channels {
		if ch.ID == id {
			found = true
			break
		}
	}
	if !found {
		writeAPIError(w, http.StatusNotFound, "not_found", "channel not found", map[string]string{"id": id})
		return
	}

	removed := false
	_, saveErr := s.updateConfig(func(cfg *config.Config) error {
		newChannels := make([]config.NotificationChannelCfg, 0, len(cfg.Notifications.Channels))
		for _, ch := range cfg.Notifications.Channels {
			if ch.ID == id {
				removed = true
				continue
			}
			newChannels = append(newChannels, ch)
		}
		if !removed {
			return nil
		}
		cfg.Notifications.Channels = newChannels
		return nil
	})

	if saveErr != nil {
		logger.Warnf("notify channel delete failed: %v", saveErr)
		writeAPIError(w, http.StatusInternalServerError, "save_failed", "failed to persist config: "+sanitizeError(saveErr.Error()), nil)
		return
	}

	s.reloadNotifyChannels()

	writeJSON(w, http.StatusOK, model.APIResponse{
		Success: true,
		Message: "channel deleted",
		Data:    map[string]any{"id": id},
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
		cfg := s.currentConfig()
		if cfg == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "config not loaded", nil)
			return req, false
		}
		for _, ch := range cfg.Notifications.Channels {
			if ch.ID == req.ID {
				s.fillTestRequestFromChannel(&req, ch)
				break
			}
		}
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
	if !requireTrustedRequest(w, r, s.allowedOrigins()) {
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

	writeJSON(w, http.StatusOK, model.APIResponse{
		Success: true,
		Message: "test message sent successfully",
	})
}
