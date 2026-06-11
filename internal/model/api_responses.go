package model

// APIResponse is the standard JSON envelope for API responses.
type APIResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// HealthResponse is the typed response for GET /api/v1/health.
type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Uptime    int64  `json:"uptime"`
	GoVersion string `json:"go_version"`
}

// UserInfo represents a user in API responses.
type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// UserListResponse is the response for GET /api/v1/users.
type UserListResponse struct {
	Users []UserInfo `json:"users"`
}

// NotificationChannelInfo represents a notification channel in API responses.
type NotificationChannelInfo struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// NodeInfo represents a distributed node in API responses.
type NodeInfo struct {
	NodeID   string `json:"node_id"`
	Status   string `json:"status"`
	LastSeen string `json:"last_seen"`
}

// BridgeDiagnosticSnapshot is the typed response for bridge status.
type BridgeDiagnosticSnapshot struct {
	BridgeConnected  bool   `json:"bridge_connected"`
	ExtensionOnline  bool   `json:"extension_online"`
	RouterExtHealthy bool   `json:"router_ext_healthy"`
	LiveClients      int    `json:"live_clients"`
	Mode             string `json:"mode"`
	LastError        string `json:"last_error,omitempty"`
}

// QueryAPIPayload is the typed response for query results.
type QueryAPIPayload struct {
	Query          string   `json:"query"`
	Engines        []string `json:"engines"`
	Status         string   `json:"status"`
	Results        any      `json:"results,omitempty"`
	Total          int      `json:"total,omitempty"`
	Page           int      `json:"page,omitempty"`
	PageSize       int      `json:"page_size,omitempty"`
	BrowserOutcome string   `json:"browser_outcome,omitempty"`
	BrowserAction  string   `json:"browser_action,omitempty"`
	Error          string   `json:"error,omitempty"`
	Errors         []string `json:"errors,omitempty"`
}

// WSMessage is the typed WebSocket message envelope.
type WSMessage struct {
	Type    string         `json:"type"`
	Query   string         `json:"query,omitempty"`
	Engines []string       `json:"engines,omitempty"`
	Error   string         `json:"error,omitempty"`
	Data    any            `json:"data,omitempty"`
	Extra   map[string]any `json:"extra,omitempty"`
}
