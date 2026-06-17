# TD-4/L-05 map[string]interface{} 强类型渐进迁移 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use compose:subagent (recommended) or compose:execute to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `map[string]interface{}` with typed structs across the codebase, improving type safety and IDE support while maintaining JSON contract compatibility.

**Architecture:** Gradual migration in 6 phases: define boundary structs → plugin interfaces → scheduler → distributed → bridge → web handlers. Each phase is independently testable. Keep `Extra`/`Raw` fields for extensibility.

**Tech Stack:** Go 1.26, standard library only (no new dependencies)

---

## Phase 1: Define Boundary Structs

### Task 1.1: Create `internal/model/plugin_types.go`

**Files:**
- Create: `internal/model/plugin_types.go`

- [ ] **Step 1: Create PluginConfig struct**

```go
package model

// PluginConfig holds typed configuration passed to Plugin.Initialize.
// Plugins access known fields directly; unknown fields are available via Extra.
type PluginConfig struct {
	Enabled   bool              `json:"enabled"`
	APIKey    string            `json:"api_key,omitempty"`
	BaseURL   string            `json:"base_url,omitempty"`
	QPS       int               `json:"qps,omitempty"`
	Timeout   int               `json:"timeout,omitempty"`
	Extra     map[string]any    `json:"extra,omitempty"`
}
```

- [ ] **Step 2: Create HookData struct**

```go
// HookData is the typed payload passed to HookFunc.
type HookData struct {
	PluginName string         `json:"plugin_name"`
	Query      string         `json:"query,omitempty"`
	Engines    []string       `json:"engines,omitempty"`
	Result     any            `json:"result,omitempty"`
	Error      string         `json:"error,omitempty"`
	Extra      map[string]any `json:"extra,omitempty"`
}
```

- [ ] **Step 3: Create HealthDetails struct**

```go
// HealthDetails replaces the untyped Details field in plugin.HealthStatus.
type HealthDetails struct {
	Uptime   int64          `json:"uptime,omitempty"`
	Memory   int64          `json:"memory,omitempty"`
	Plugins  int            `json:"plugins,omitempty"`
	Extra    map[string]any `json:"extra,omitempty"`
}
```

- [ ] **Step 4: Create NotificationMetadata struct**

```go
// NotificationMetadata replaces the untyped Metadata field in plugin.NotificationMessage.
type NotificationMetadata struct {
	TaskID   string         `json:"task_id,omitempty"`
	TaskType string         `json:"task_type,omitempty"`
	Status   string         `json:"status,omitempty"`
	Duration int64          `json:"duration_ms,omitempty"`
	Extra    map[string]any `json:"extra,omitempty"`
}
```

- [ ] **Step 5: Run build check**

```bash
go build ./internal/model/...
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/model/plugin_types.go
git commit -m "feat: add typed boundary structs for plugin config, hooks, health, and notifications"
```

---

### Task 1.2: Create `internal/model/scheduler_types.go` (Payload types)

**Files:**
- Create: `internal/model/scheduler_payload.go`

- [ ] **Step 1: Create TaskPayload struct**

```go
package model

// TaskPayload is the typed parameter bag for scheduled tasks.
// Each task type uses known fields; Extra holds engine-specific params.
type TaskPayload struct {
	// Common fields
	Query       string   `json:"query,omitempty"`
	Engines     []string `json:"engines,omitempty"`
	PageSize    int      `json:"page_size,omitempty"`
	Format      string   `json:"format,omitempty"`
	DetectMode  string   `json:"detection_mode,omitempty"`
	MaxAgeDays  int      `json:"max_age_days,omitempty"`
	LowThresh   int      `json:"low_threshold,omitempty"`
	TimeoutSec  int      `json:"timeout_seconds,omitempty"`

	// ICP-specific
	Queries    []string `json:"queries,omitempty"`
	Type       string   `json:"type,omitempty"`
	Page       int      `json:"page,omitempty"`
	PageSizeICP int     `json:"page_size,omitempty"`

	// Batch screenshot
	URLs       []string `json:"urls,omitempty"`

	// Tamper check
	URL        string   `json:"url,omitempty"`

	// Cookie verify
	CookieFile string   `json:"cookie_file,omitempty"`

	Extra map[string]any `json:"extra,omitempty"`
}
```

- [ ] **Step 2: Create TaskOutput struct**

```go
// TaskOutput is the typed result returned by task handlers.
type TaskOutput struct {
	Message    string         `json:"message,omitempty"`
	Total      int            `json:"total,omitempty"`
	Success    int            `json:"success,omitempty"`
	Failed     int            `json:"failed,omitempty"`
	DurationMS int64          `json:"duration_ms,omitempty"`
	Extra      map[string]any `json:"extra,omitempty"`
}
```

- [ ] **Step 3: Run build check**

```bash
go build ./internal/model/...
```
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/model/scheduler_payload.go
git commit -m "feat: add typed TaskPayload and TaskOutput structs for scheduler"
```

---

### Task 1.3: Create `internal/model/bridge_types.go` (Extension bridge)

**Files:**
- Create: `internal/model/bridge_payload.go`

- [ ] **Step 1: Create CollectedDataItem struct**

```go
package model

// CollectedDataItem represents a single item extracted by the extension.
type CollectedDataItem struct {
	IP          string         `json:"ip,omitempty"`
	Port        int            `json:"port,omitempty"`
	Host        string         `json:"host,omitempty"`
	URL         string         `json:"url,omitempty"`
	Title       string         `json:"title,omitempty"`
	BodySnippet string         `json:"body_snippet,omitempty"`
	Server      string         `json:"server,omitempty"`
	StatusCode  int            `json:"status_code,omitempty"`
	Protocol    string         `json:"protocol,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}
```

- [ ] **Step 2: Create BridgeCollectedData struct**

```go
// BridgeCollectedData replaces map[string]interface{} in BridgeResult.StructuredCollectedData.
type BridgeCollectedData struct {
	Engine    string              `json:"engine,omitempty"`
	Total     int                 `json:"total,omitempty"`
	Items     []CollectedDataItem `json:"items,omitempty"`
	Extra     map[string]any      `json:"extra,omitempty"`
}
```

- [ ] **Step 3: Run build check**

```bash
go build ./internal/model/...
```
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/model/bridge_payload.go
git commit -m "feat: add typed CollectedDataItem and BridgeCollectedData structs for extension bridge"
```

---

### Task 1.4: Create `internal/model/api_responses.go` (Web API DTOs)

**Files:**
- Create: `internal/model/api_responses.go`

- [ ] **Step 1: Create common response wrappers**

```go
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
	BridgeConnected   bool   `json:"bridge_connected"`
	ExtensionOnline   bool   `json:"extension_online"`
	RouterExtHealthy  bool   `json:"router_ext_healthy"`
	LiveClients       int    `json:"live_clients"`
	Mode              string `json:"mode"`
	LastError         string `json:"last_error,omitempty"`
}
```

- [ ] **Step 2: Run build check**

```bash
go build ./internal/model/...
```
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/model/api_responses.go
git commit -m "feat: add typed API response structs for web handlers"
```

---

## Phase 2: Migrate Plugin Interfaces

### Task 2.1: Update Plugin.Initialize signature

**Files:**
- Modify: `internal/plugin/plugin.go:21`
- Modify: `internal/plugin/example_engine_plugin.go:55`
- Modify: `internal/plugin/processors/processors.go:60,231`
- Modify: `internal/plugin/processors/validation.go:63,241`
- Modify: `internal/plugin/manager.go`

- [ ] **Step 1: Update Plugin interface**

Change `Initialize(config map[string]interface{})` to `Initialize(config *model.PluginConfig)` in `internal/plugin/plugin.go:21`.

- [ ] **Step 2: Update all implementations**

Update `Initialize` in:
- `example_engine_plugin.go:55` — change parameter type
- `processors.go:60` (DeduplicationProcessor) — extract `config.Strategy` from typed struct
- `processors.go:231` (DataCleaningProcessor) — same
- `validation.go:63` (ValidationProcessor) — same
- `validation.go:241` (EnrichmentProcessor) — same

- [ ] **Step 3: Update PluginManager.LoadPlugin**

In `internal/plugin/manager.go`, find where `Initialize` is called and pass `*model.PluginConfig`.

- [ ] **Step 4: Run build check**

```bash
go build ./...
```
Expected: PASS (fix any compilation errors)

- [ ] **Step 5: Run tests**

```bash
go test ./internal/plugin/...
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/plugin/
git commit -m "refactor: change Plugin.Initialize to accept typed PluginConfig"
```

---

### Task 2.2: Update HookFunc signature

**Files:**
- Modify: `internal/plugin/hooks.go:36,68`
- Modify: all callers of TriggerHook

- [ ] **Step 1: Update HookFunc type**

Change `HookFunc func(pluginName string, data map[string]interface{}) error` to `HookFunc func(pluginName string, data *model.HookData) error` in `internal/plugin/hooks.go:36`.

- [ ] **Step 2: Update TriggerHook signature**

Change `TriggerHook(hookType HookType, pluginName string, data map[string]interface{}) error` to `TriggerHook(hookType HookType, pluginName string, data *model.HookData) error` in `internal/plugin/hooks.go:68`.

- [ ] **Step 3: Find and update all callers**

Search for `TriggerHook` calls and update them to pass `*model.HookData`.

- [ ] **Step 4: Run build check**

```bash
go build ./...
```
Expected: PASS

- [ ] **Step 5: Run tests**

```bash
go test ./internal/plugin/...
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/plugin/
git commit -m "refactor: change HookFunc and TriggerHook to use typed HookData"
```

---

### Task 2.3: Update HealthStatus.Details and NotificationMessage.Metadata

**Files:**
- Modify: `internal/plugin/plugin.go:83,99`
- Update all constructors of HealthStatus and NotificationMessage

- [ ] **Step 1: Update HealthStatus.Details**

Change `Details map[string]interface{}` to `Details *model.HealthDetails` in `internal/plugin/plugin.go:83`.

- [ ] **Step 2: Update NotificationMessage.Metadata**

Change `Metadata map[string]interface{}` to `Metadata *model.NotificationMetadata` in `internal/plugin/plugin.go:99`.

- [ ] **Step 3: Update all constructors**

Search for `HealthStatus{` and `NotificationMessage{` and update all call sites.

- [ ] **Step 4: Run build check**

```bash
go build ./...
```
Expected: PASS

- [ ] **Step 5: Run tests**

```bash
go test ./internal/plugin/...
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/plugin/
git commit -m "refactor: use typed HealthDetails and NotificationMetadata"
```

---

## Phase 3: Migrate Scheduler

### Task 3.1: Update ScheduledTask.Payload and TaskTemplate.Payload

**Files:**
- Modify: `internal/scheduler/scheduler_types.go:225,265`
- Modify: `internal/scheduler/executor_runners2.go` (all TaskHandler.Execute implementations)

- [ ] **Step 1: Update ScheduledTask.Payload**

Change `Payload map[string]interface{}` to `Payload *model.TaskPayload` in `scheduler_types.go:225`.

- [ ] **Step 2: Update TaskTemplate.Payload**

Change `Payload map[string]interface{}` to `Payload *model.TaskPayload` in `scheduler_types.go:265`.

- [ ] **Step 3: Update TaskHandler.Execute**

Change `Execute(ctx context.Context, payload map[string]interface{}) (string, error)` to `Execute(ctx context.Context, payload *model.TaskPayload) (string, error)` in `scheduler_types.go:288`.

- [ ] **Step 4: Update all TaskHandler implementations**

Search for all `func.*Execute.*payload map` and update signatures.

- [ ] **Step 5: Update DefaultTemplates**

Update all `Payload: map[string]interface{}{...}` in `DefaultTemplates()` to use `&model.TaskPayload{...}`.

- [ ] **Step 6: Run build check**

```bash
go build ./...
```
Expected: PASS

- [ ] **Step 7: Run tests**

```bash
go test ./internal/scheduler/...
```
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/scheduler/
git commit -m "refactor: use typed TaskPayload for scheduler tasks"
```

---

## Phase 4: Migrate Distributed Task Queue

### Task 4.1: Update TaskEnvelope, TaskResult, TaskRecord

**Files:**
- Modify: `internal/distributed/task_queue.go:29,43,62,75`

- [ ] **Step 1: Update TaskEnvelope.Payload**

Change `Payload map[string]interface{}` to `Payload *model.TaskPayload` in `task_queue.go:29`.

- [ ] **Step 2: Update TaskResult.Output**

Change `Output map[string]interface{}` to `Output *model.TaskOutput` in `task_queue.go:43`.

- [ ] **Step 3: Update TaskRecord.Payload and TaskRecord.Result**

Change `Payload map[string]interface{}` to `Payload *model.TaskPayload` and `Result map[string]interface{}` to `Result *model.TaskOutput` in `task_queue.go:62,75`.

- [ ] **Step 4: Update all callers**

Search for all places that construct or access these fields and update them.

- [ ] **Step 5: Run build check**

```bash
go build ./...
```
Expected: PASS

- [ ] **Step 6: Run tests**

```bash
go test ./internal/distributed/...
```
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/distributed/
git commit -m "refactor: use typed TaskPayload and TaskOutput in distributed queue"
```

---

## Phase 5: Migrate Bridge Types

### Task 5.1: Update BridgeResult.StructuredCollectedData

**Files:**
- Modify: `internal/screenshot/bridge_types.go:34`
- Modify: `internal/screenshot/bridge_service.go`
- Modify: `web/screenshot_bridge_handlers.go:201,276`

- [ ] **Step 1: Update BridgeResult.StructuredCollectedData**

Change `StructuredCollectedData map[string]interface{}` to `StructuredCollectedData *model.BridgeCollectedData` in `bridge_types.go:34`.

- [ ] **Step 2: Update bridge request struct in screenshot_bridge_handlers.go**

Change `StructuredCollectedData map[string]interface{}` to `StructuredCollectedData *model.BridgeCollectedData` in the anonymous struct at line 201.

- [ ] **Step 3: Update logBridgeCollectedData**

Update the function signature and body to use typed data.

- [ ] **Step 4: Update buildBridgeDiagnosticSnapshot**

Return `*model.BridgeDiagnosticSnapshot` instead of `map[string]interface{}`.

- [ ] **Step 5: Run build check**

```bash
go build ./...
```
Expected: PASS

- [ ] **Step 6: Run tests**

```bash
go test ./internal/screenshot/... ./web/...
```
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/screenshot/ web/screenshot_bridge_handlers.go
git commit -m "refactor: use typed BridgeCollectedData and BridgeDiagnosticSnapshot"
```

---

## Phase 6: Migrate Web API Handlers (Partial)

### Task 6.1: Migrate user_handlers.go

**Files:**
- Modify: `web/user_handlers.go:98,131,143,189,222,304,388`

- [ ] **Step 1: Replace inline map responses with model types**

Replace `map[string]interface{}{"user": map[string]interface{}{...}}` with `model.APIResponse{Data: model.UserInfo{...}}`.

- [ ] **Step 2: Run build check**

```bash
go build ./...
```
Expected: PASS

- [ ] **Step 3: Run tests**

```bash
go test ./web/...
```
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add web/user_handlers.go
git commit -m "refactor: use typed API responses in user handlers"
```

---

### Task 6.2: Migrate notification_handlers.go

**Files:**
- Modify: `web/notification_handlers.go:27,41,60,293,349,475`

- [ ] **Step 1: Replace inline map responses**

Use `model.NotificationChannelInfo` and `model.APIResponse`.

- [ ] **Step 2: Run build check**

```bash
go build ./...
```
Expected: PASS

- [ ] **Step 3: Run tests**

```bash
go test ./web/...
```
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add web/notification_handlers.go
git commit -m "refactor: use typed API responses in notification handlers"
```

---

### Task 6.3: Migrate node_handlers.go and node_task_handlers.go

**Files:**
- Modify: `web/node_handlers.go`
- Modify: `web/node_task_handlers.go`

- [ ] **Step 1: Replace inline map responses**

Use `model.NodeInfo` and `model.APIResponse`.

- [ ] **Step 2: Run build check**

```bash
go build ./...
```
Expected: PASS

- [ ] **Step 3: Run tests**

```bash
go test ./web/...
```
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add web/node_handlers.go web/node_task_handlers.go
git commit -m "refactor: use typed API responses in node handlers"
```

---

### Task 6.4: Migrate query_handlers.go (buildQueryAPIPayload)

**Files:**
- Modify: `web/query_handlers.go:71,124`

- [ ] **Step 1: Create QueryAPIPayload struct**

Add to `internal/model/api_responses.go`:

```go
type QueryAPIPayload struct {
	Query           string   `json:"query"`
	Engines         []string `json:"engines"`
	Status          string   `json:"status"`
	Results         any      `json:"results,omitempty"`
	Total           int      `json:"total,omitempty"`
	Page            int      `json:"page,omitempty"`
	PageSize        int      `json:"page_size,omitempty"`
	BrowserOutcome  string   `json:"browser_outcome,omitempty"`
	BrowserAction   string   `json:"browser_action,omitempty"`
	Error           string   `json:"error,omitempty"`
	Errors          []string `json:"errors,omitempty"`
}
```

- [ ] **Step 2: Update buildQueryAPIPayload**

Return `*model.QueryAPIPayload` instead of `map[string]interface{}`.

- [ ] **Step 3: Run build check**

```bash
go build ./...
```
Expected: PASS

- [ ] **Step 4: Run tests**

```bash
go test ./web/...
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/query_handlers.go internal/model/api_responses.go
git commit -m "refactor: use typed QueryAPIPayload in query handlers"
```

---

### Task 6.5: Migrate server.go template data and health endpoint

**Files:**
- Modify: `web/server.go:183,1114`

- [ ] **Step 1: Update dict template function**

Change `dict := make(map[string]interface{}, ...)` to `dict := make(map[string]any, ...)` (this is acceptable as template data).

- [ ] **Step 2: Update health endpoint**

Use `model.HealthResponse` instead of inline map.

- [ ] **Step 3: Run build check**

```bash
go build ./...
```
Expected: PASS

- [ ] **Step 4: Run tests**

```bash
go test ./web/...
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/server.go internal/model/api_responses.go
git commit -m "refactor: use typed HealthResponse and clean up template dict"
```

---

### Task 6.6: Migrate WebSocket message types

**Files:**
- Modify: `web/websocket_handlers.go:116,131,168,171,186,196,233,306,429`

- [ ] **Step 1: Create WSMessage struct**

Add to `internal/model/api_responses.go`:

```go
// WSMessage is the typed WebSocket message envelope.
type WSMessage struct {
	Type    string         `json:"type"`
	Query   string         `json:"query,omitempty"`
	Engines []string       `json:"engines,omitempty"`
	Error   string         `json:"error,omitempty"`
	Data    any            `json:"data,omitempty"`
	Extra   map[string]any `json:"extra,omitempty"`
}
```

- [ ] **Step 2: Update wsMessageLoop**

Change `var message map[string]interface{}` to `var message model.WSMessage`.

- [ ] **Step 3: Update writeJSON calls**

Replace `writeJSON(map[string]interface{}{"type": "pong"})` with `writeJSON(model.WSMessage{Type: "pong"})`.

- [ ] **Step 4: Update handleWebSocketQuery**

Change parameter type and update internal calls.

- [ ] **Step 5: Update parseWSQueryParams**

Accept `model.WSMessage` instead of `map[string]interface{}`.

- [ ] **Step 6: Run build check**

```bash
go build ./...
```
Expected: PASS

- [ ] **Step 7: Run tests**

```bash
go test ./web/...
```
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add web/websocket_handlers.go internal/model/api_responses.go
git commit -m "refactor: use typed WSMessage for WebSocket communication"
```

---

## Verification

### Task V1: Full build and test

- [ ] **Step 1: Run full build**

```bash
go build ./...
```
Expected: PASS

- [ ] **Step 2: Run full test suite with race detector**

```bash
go test -race ./...
```
Expected: PASS

- [ ] **Step 3: Run go vet**

```bash
go vet ./...
```
Expected: PASS

- [ ] **Step 4: Count remaining map[string]interface{}**

```bash
Select-String -Path "**\*.go" -Pattern "map\[string\]interface\{\}" | Measure-Object
```
Expected: Significantly reduced from 799 (target: < 200, remaining should be template data and test fixtures)

- [ ] **Step 5: Final commit if needed**

```bash
git add -A
git commit -m "chore: TD-4/L-05 strong typing migration complete"
```
