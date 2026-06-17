package model

// PluginConfig holds typed configuration passed to Plugin.Initialize.
// Plugins access known fields directly; unknown fields are available via Extra.
type PluginConfig struct {
	Enabled bool           `json:"enabled"`
	APIKey  string         `json:"api_key,omitempty"`
	BaseURL string         `json:"base_url,omitempty"`
	QPS     int            `json:"qps,omitempty"`
	Timeout int            `json:"timeout,omitempty"`
	Extra   map[string]any `json:"extra,omitempty"`
}

// HookData is the typed payload passed to HookFunc.
type HookData struct {
	PluginName string         `json:"plugin_name"`
	Query      string         `json:"query,omitempty"`
	Engines    []string       `json:"engines,omitempty"`
	Result     any            `json:"result,omitempty"`
	Error      string         `json:"error,omitempty"`
	Extra      map[string]any `json:"extra,omitempty"`
}

// HealthDetails replaces the untyped Details field in plugin.HealthStatus.
type HealthDetails struct {
	Uptime  int64          `json:"uptime,omitempty"`
	Memory  int64          `json:"memory,omitempty"`
	Plugins int            `json:"plugins,omitempty"`
	Extra   map[string]any `json:"extra,omitempty"`
}

// NotificationMetadata replaces the untyped Metadata field in plugin.NotificationMessage.
type NotificationMetadata struct {
	TaskID   string         `json:"task_id,omitempty"`
	TaskType string         `json:"task_type,omitempty"`
	Status   string         `json:"status,omitempty"`
	Duration int64          `json:"duration_ms,omitempty"`
	Extra    map[string]any `json:"extra,omitempty"`
}
