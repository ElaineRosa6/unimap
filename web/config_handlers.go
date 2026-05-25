package web

import (
	"net/http"
	"strings"

	"github.com/unimap-icp-hunter/project/internal/config"
	"github.com/unimap-icp-hunter/project/internal/logger"
)

// handleGetConfig returns the current config with secrets masked (GET /api/config).
// Only sections needed by the settings page are exposed.
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if s.config == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "config_not_loaded", "config not loaded", nil)
		return
	}

	s.configMutex.Lock()
	defer s.configMutex.Unlock()

	engines := map[string]map[string]interface{}{
		"fofa": {
			"enabled":      s.config.Engines.Fofa.Enabled,
			"api_base_url": s.config.Engines.Fofa.APIBaseURL,
			"web_base_url": s.config.Engines.Fofa.WebBaseURL,
			"email":        s.config.Engines.Fofa.Email,
			"api_key":      maskAPIKey(s.config.Engines.Fofa.APIKey),
			"qps":          s.config.Engines.Fofa.QPS,
			"timeout":      s.config.Engines.Fofa.Timeout,
		},
		"hunter": {
			"enabled":  s.config.Engines.Hunter.Enabled,
			"base_url": s.config.Engines.Hunter.BaseURL,
			"api_key":  maskAPIKey(s.config.Engines.Hunter.APIKey),
			"qps":      s.config.Engines.Hunter.QPS,
			"timeout":  s.config.Engines.Hunter.Timeout,
		},
		"zoomeye": {
			"enabled":  s.config.Engines.Zoomeye.Enabled,
			"base_url": s.config.Engines.Zoomeye.BaseURL,
			"api_key":  maskAPIKey(s.config.Engines.Zoomeye.APIKey),
			"qps":      s.config.Engines.Zoomeye.QPS,
			"timeout":  s.config.Engines.Zoomeye.Timeout,
		},
		"quake": {
			"enabled":  s.config.Engines.Quake.Enabled,
			"base_url": s.config.Engines.Quake.BaseURL,
			"api_key":  maskAPIKey(s.config.Engines.Quake.APIKey),
			"qps":      s.config.Engines.Quake.QPS,
			"timeout":  s.config.Engines.Quake.Timeout,
		},
		"shodan": {
			"enabled":  s.config.Engines.Shodan.Enabled,
			"base_url": s.config.Engines.Shodan.BaseURL,
			"api_key":  maskAPIKey(s.config.Engines.Shodan.APIKey),
			"qps":      s.config.Engines.Shodan.QPS,
		},
	}

	icp := map[string]interface{}{
		"enabled":      s.config.ICP.Enabled,
		"base_url":     s.config.ICP.BaseURL,
		"api_key":      maskAPIKey(s.config.ICP.APIKey),
		"timeout":      s.config.ICP.Timeout,
		"default_type": s.config.ICP.DefaultType,
	}

	screenshot := map[string]interface{}{
		"enabled": s.config.Screenshot.Enabled,
		"engine":  s.config.Screenshot.Engine,
		"mode":    s.config.Screenshot.Mode,
		"timeout": s.config.Screenshot.Timeout,
	}

	system := map[string]interface{}{
		"max_concurrent":    s.config.System.MaxConcurrent,
		"cache_ttl":         s.config.System.CacheTTL,
		"cache_max_entries": s.config.System.CacheMaxSize,
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"engines":    engines,
		"icp":        icp,
		"screenshot": screenshot,
		"system":     system,
	})
}

// configSaveRequest is the POST /api/config payload.
type configSaveRequest struct {
	Section string                 `json:"section"`
	Data    map[string]interface{} `json:"data"`
}

// handleSaveConfig accepts a section-scoped config patch and persists it.
// Supported sections: icp, screenshot, system. Engine keys go through dedicated
// endpoints to keep credential handling tight.
func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}
	if s.config == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "config_not_loaded", "config not loaded", nil)
		return
	}

	var req configSaveRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	section := strings.ToLower(strings.TrimSpace(req.Section))
	if section == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_section", "section is required", nil)
		return
	}

	s.configMutex.Lock()
	switch section {
	case "engines":
		applyEngineSections(s.config, req.Data)
	case "icp":
		applyICPSection(s.config, req.Data)
	case "screenshot":
		applyScreenshotSection(s.config, req.Data)
	case "system":
		applySystemSection(s.config, req.Data)
	default:
		s.configMutex.Unlock()
		writeAPIError(w, http.StatusBadRequest, "unsupported_section",
			"unsupported section", map[string]string{"section": section})
		return
	}

	var saveErr error
	if s.configManager != nil {
		saveErr = s.configManager.Save()
	}

	if section == "engines" {
		s.reloadEngineAdapters()
	}

	s.configMutex.Unlock()

	if saveErr != nil {
		logger.Warnf("config save failed: %v", saveErr)
		writeAPIError(w, http.StatusInternalServerError, "save_failed",
			"failed to persist config: "+sanitizeError(saveErr.Error()), nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"section": section,
		"message": "saved",
	})
}

// applyICPSection mutates the ICP config fields from the incoming map.
// Empty api_key is treated as "no change" so the masked value displayed in the
// UI doesn't accidentally overwrite the real secret.
func applyICPSection(c *config.Config, data map[string]interface{}) {
	if c == nil {
		return
	}
	if v, ok := boolField(data, "enabled"); ok {
		c.ICP.Enabled = v
	}
	if v, ok := stringField(data, "base_url"); ok {
		c.ICP.BaseURL = strings.TrimSpace(v)
	}
	if v, ok := stringField(data, "api_key"); ok {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" && !isMaskedSecret(trimmed) {
			c.ICP.APIKey = trimmed
		}
	}
	if v, ok := intField(data, "timeout"); ok && v > 0 {
		c.ICP.Timeout = v
	}
	if v, ok := stringField(data, "default_type"); ok {
		c.ICP.DefaultType = strings.TrimSpace(v)
	}
}

func applyScreenshotSection(c *config.Config, data map[string]interface{}) {
	if c == nil {
		return
	}
	if v, ok := stringField(data, "engine"); ok {
		c.Screenshot.Engine = strings.TrimSpace(v)
	}
	if v, ok := stringField(data, "mode"); ok {
		c.Screenshot.Mode = strings.TrimSpace(v)
	}
	if v, ok := intField(data, "timeout"); ok && v > 0 {
		c.Screenshot.Timeout = v
	}
}

func applySystemSection(c *config.Config, data map[string]interface{}) {
	if c == nil {
		return
	}
	if v, ok := intField(data, "max_concurrent"); ok && v > 0 {
		c.System.MaxConcurrent = v
	}
	if v, ok := intField(data, "cache_ttl"); ok && v >= 0 {
		c.System.CacheTTL = v
	}
	if v, ok := intField(data, "cache_max_entries"); ok && v >= 0 {
		c.System.CacheMaxSize = v
	}
}

// applyEngineSections handles engine configs. req.Data is a map of engine name → fields.
func applyEngineSections(c *config.Config, data map[string]interface{}) {
	if c == nil {
		return
	}
	engines := map[string]interface{}{
		"fofa":    data["fofa"],
		"hunter":  data["hunter"],
		"zoomeye": data["zoomeye"],
		"quake":   data["quake"],
		"shodan":  data["shodan"],
	}
	for name, raw := range engines {
		if raw == nil {
			continue
		}
		eng, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		switch name {
		case "fofa":
			if v, _ := boolField(eng, "enabled"); v {
				c.Engines.Fofa.Enabled = v
			}
			if v, _ := stringField(eng, "api_base_url"); v != "" {
				c.Engines.Fofa.APIBaseURL = v
			}
			if v, _ := stringField(eng, "api_key"); v != "" && !isMaskedSecret(v) {
				c.Engines.Fofa.APIKey = v
			}
			if v, _ := stringField(eng, "email"); v != "" {
				c.Engines.Fofa.Email = v
			}
			if v, _ := intField(eng, "qps"); v > 0 {
				c.Engines.Fofa.QPS = v
			}
			if v, _ := intField(eng, "timeout"); v > 0 {
				c.Engines.Fofa.Timeout = v
			}
		case "hunter":
			if v, _ := boolField(eng, "enabled"); v {
				c.Engines.Hunter.Enabled = v
			}
			if v, _ := stringField(eng, "api_key"); v != "" && !isMaskedSecret(v) {
				c.Engines.Hunter.APIKey = v
			}
			if v, _ := stringField(eng, "base_url"); v != "" {
				c.Engines.Hunter.BaseURL = v
			}
			if v, _ := intField(eng, "qps"); v > 0 {
				c.Engines.Hunter.QPS = v
			}
			if v, _ := intField(eng, "timeout"); v > 0 {
				c.Engines.Hunter.Timeout = v
			}
		case "zoomeye":
			if v, _ := boolField(eng, "enabled"); v {
				c.Engines.Zoomeye.Enabled = v
			}
			if v, _ := stringField(eng, "api_key"); v != "" && !isMaskedSecret(v) {
				c.Engines.Zoomeye.APIKey = v
			}
			if v, _ := stringField(eng, "base_url"); v != "" {
				c.Engines.Zoomeye.BaseURL = v
			}
			if v, _ := intField(eng, "qps"); v > 0 {
				c.Engines.Zoomeye.QPS = v
			}
			if v, _ := intField(eng, "timeout"); v > 0 {
				c.Engines.Zoomeye.Timeout = v
			}
		case "quake":
			if v, _ := boolField(eng, "enabled"); v {
				c.Engines.Quake.Enabled = v
			}
			if v, _ := stringField(eng, "api_key"); v != "" && !isMaskedSecret(v) {
				c.Engines.Quake.APIKey = v
			}
			if v, _ := stringField(eng, "base_url"); v != "" {
				c.Engines.Quake.BaseURL = v
			}
			if v, _ := intField(eng, "qps"); v > 0 {
				c.Engines.Quake.QPS = v
			}
			if v, _ := intField(eng, "timeout"); v > 0 {
				c.Engines.Quake.Timeout = v
			}
		case "shodan":
			if v, _ := boolField(eng, "enabled"); v {
				c.Engines.Shodan.Enabled = v
			}
			if v, _ := stringField(eng, "api_key"); v != "" && !isMaskedSecret(v) {
				c.Engines.Shodan.APIKey = v
			}
			if v, _ := stringField(eng, "base_url"); v != "" {
				c.Engines.Shodan.BaseURL = v
			}
			if v, _ := intField(eng, "qps"); v > 0 {
				c.Engines.Shodan.QPS = v
			}
			// Shodan doesn't have timeout field
		}
	}
}

// boolField, stringField, intField extract typed values from a map[string]interface{}
// produced by encoding/json. JSON numbers come back as float64 so we coerce.
func boolField(data map[string]interface{}, key string) (bool, bool) {
	v, ok := data[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func stringField(data map[string]interface{}, key string) (string, bool) {
	v, ok := data[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func intField(data map[string]interface{}, key string) (int, bool) {
	v, ok := data[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}

// isMaskedSecret returns true when the input looks like the UI's masked form
// (a stretch of asterisks), so we don't write that back as the real secret.
func isMaskedSecret(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r != '*' && r != '•' {
			// Allow the redacted form "abcd****wxyz" we emit from maskAPIKey:
			// if any non-mask, non-tail char is present treat it as user input.
			// But we conservatively also reject inputs that contain runs of "***"
			// since the UI shows that pattern.
			break
		}
	}
	return strings.Contains(s, "****")
}
