# UQL 查询历史服务端持久化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use compose:subagent (recommended) or compose:execute to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add server-side persistence for all operation history (UQL queries, ICP, port scans, etc.) with results, replacing frontend-only localStorage.

**Architecture:** New `internal/history` package with SQLite database, repository pattern (following ICP database module), and REST API endpoints in `web/`. Frontend migrated from localStorage to API calls.

**Tech Stack:** SQLite (go-sqlite3), net/http handlers, JSON serialization

---

## File Structure

| File | Responsibility |
|------|---------------|
| `internal/history/database.go` | SQLite connection, schema init |
| `internal/history/repository.go` | CRUD operations for operation_history + operation_results |
| `internal/history/models.go` | Go struct definitions |
| `internal/history/repository_test.go` | Repository unit tests |
| `web/history_handlers.go` | HTTP handlers for history API |
| `web/server.go` | Route registration |
| `web/static/js/main.js` | Frontend: save/load from API |
| `configs/config.yaml` | History database path config |
| `internal/config/config_types.go` | Config struct update |

---

### Task 1: Create history models and database schema

**Files:**
- Create: `internal/history/models.go`
- Create: `internal/history/database.go`

- [ ] **Step 1: Create models.go**

```go
package history

import "time"

// OperationType identifies the type of operation.
type OperationType string

const (
	OpTypeQuery      OperationType = "query"
	OpTypeICPQuery   OperationType = "icp_query"
	OpTypePortScan   OperationType = "port_scan"
	OpTypeScreenshot OperationType = "screenshot"
	OpTypeTamperCheck OperationType = "tamper_check"
)

// OperationHistory represents one executed operation.
type OperationHistory struct {
	ID           int64         `json:"id"`
	OperationType OperationType `json:"operation_type"`
	Input        string        `json:"input"`         // JSON: query params
	Status       string        `json:"status"`        // success/failed/partial
	TotalCount   int           `json:"total_count"`
	Summary      string        `json:"summary,omitempty"` // JSON: stats
	DurationMS   int64         `json:"duration_ms,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
}

// OperationResult represents a single result row for an operation.
type OperationResult struct {
	ID        int64  `json:"id"`
	HistoryID int64  `json:"history_id"`
	Data      string `json:"data"` // JSON: arbitrary result data
}
```

- [ ] **Step 2: Create database.go**

```go
package history

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// Database manages the SQLite connection for operation history.
type Database struct {
	db *sql.DB
}

// NewDatabase opens or creates the SQLite database at dbPath.
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open history database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping history database: %w", err)
	}
	return &Database{db: db}, nil
}

// Close closes the database connection.
func (d *Database) Close() error {
	return d.db.Close()
}

// DB returns the underlying *sql.DB.
func (d *Database) DB() *sql.DB {
	return d.db
}

// InitSchema creates tables and indexes if they do not exist.
func (d *Database) InitSchema() error {
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS operation_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			operation_type TEXT NOT NULL,
			input TEXT NOT NULL,
			status TEXT NOT NULL,
			total_count INTEGER DEFAULT 0,
			summary TEXT,
			duration_ms INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create operation_history table: %w", err)
	}

	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS operation_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			history_id INTEGER NOT NULL,
			data TEXT NOT NULL,
			FOREIGN KEY (history_id) REFERENCES operation_history(id) ON DELETE CASCADE
		)
	`); err != nil {
		return fmt.Errorf("failed to create operation_results table: %w", err)
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_op_history_type ON operation_history(operation_type)`,
		`CREATE INDEX IF NOT EXISTS idx_op_history_created ON operation_history(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_op_results_history ON operation_results(history_id)`,
	}
	for _, idx := range indexes {
		if _, err := d.db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}
	return nil
}
```

- [ ] **Step 3: Run build check**

```bash
go build ./internal/history/...
```
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/history/
git commit -m "feat: add history database and models for operation persistence"
```

---

### Task 2: Create repository with CRUD operations

**Files:**
- Create: `internal/history/repository.go`
- Create: `internal/history/repository_test.go`

- [ ] **Step 1: Create repository.go**

```go
package history

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides CRUD for operation history.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// CreateHistory inserts a new operation history record and returns its ID.
func (r *Repository) CreateHistory(h *OperationHistory) (int64, error) {
	result, err := r.db.Exec(
		`INSERT INTO operation_history (operation_type, input, status, total_count, summary, duration_ms, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		h.OperationType, h.Input, h.Status, h.TotalCount, h.Summary, h.DurationMS, time.Now(),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert operation_history: %w", err)
	}
	return result.LastInsertId()
}

// CreateResults bulk-inserts operation results for a given history ID.
func (r *Repository) CreateResults(historyID int64, results []OperationResult) error {
	if len(results) == 0 {
		return nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO operation_results (history_id, data) VALUES (?, ?)`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()
	for _, res := range results {
		if _, err := stmt.Exec(historyID, res.Data); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to insert operation_result: %w", err)
		}
	}
	return tx.Commit()
}

// ListHistory returns operation history records, optionally filtered by type.
func (r *Repository) ListHistory(opType string, limit, offset int) ([]OperationHistory, int, error) {
	if limit <= 0 {
		limit = 20
	}
	where := ""
	args := []interface{}{}
	if opType != "" {
		where = "WHERE operation_type = ?"
		args = append(args, opType)
	}

	// Count total
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM operation_history %s", where)
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count operation_history: %w", err)
	}

	// Fetch records
	query := fmt.Sprintf(
		"SELECT id, operation_type, input, status, total_count, summary, duration_ms, created_at FROM operation_history %s ORDER BY created_at DESC LIMIT ? OFFSET ?",
		where,
	)
	args = append(args, limit, offset)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query operation_history: %w", err)
	}
	defer rows.Close()

	var items []OperationHistory
	for rows.Next() {
		var h OperationHistory
		if err := rows.Scan(&h.ID, &h.OperationType, &h.Input, &h.Status, &h.TotalCount, &h.Summary, &h.DurationMS, &h.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan operation_history: %w", err)
		}
		items = append(items, h)
	}
	return items, total, nil
}

// GetHistory returns a single operation history by ID.
func (r *Repository) GetHistory(id int64) (*OperationHistory, error) {
	var h OperationHistory
	err := r.db.QueryRow(
		"SELECT id, operation_type, input, status, total_count, summary, duration_ms, created_at FROM operation_history WHERE id = ?",
		id,
	).Scan(&h.ID, &h.OperationType, &h.Input, &h.Status, &h.TotalCount, &h.Summary, &h.DurationMS, &h.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get operation_history: %w", err)
	}
	return &h, nil
}

// GetResults returns all results for a given history ID.
func (r *Repository) GetResults(historyID int64) ([]OperationResult, error) {
	rows, err := r.db.Query(
		"SELECT id, history_id, data FROM operation_results WHERE history_id = ? ORDER BY id",
		historyID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query operation_results: %w", err)
	}
	defer rows.Close()

	var results []OperationResult
	for rows.Next() {
		var res OperationResult
		if err := rows.Scan(&res.ID, &res.HistoryID, &res.Data); err != nil {
			return nil, fmt.Errorf("failed to scan operation_result: %w", err)
		}
		results = append(results, res)
	}
	return results, nil
}

// DeleteHistory deletes a single history record and its results (CASCADE).
func (r *Repository) DeleteHistory(id int64) error {
	_, err := r.db.Exec("DELETE FROM operation_history WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete operation_history: %w", err)
	}
	return nil
}

// ClearHistory deletes all history, optionally filtered by type.
func (r *Repository) ClearHistory(opType string) error {
	if opType != "" {
		_, err := r.db.Exec("DELETE FROM operation_history WHERE operation_type = ?", opType)
		return err
	}
	_, err := r.db.Exec("DELETE FROM operation_history")
	return err
}
```

- [ ] **Step 2: Create repository_test.go**

```go
package history

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) *Repository {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		os.Remove(dbPath)
	})
	if err := db.InitSchema(); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}
	return NewRepository(db.DB())
}

func TestCreateAndListHistory(t *testing.T) {
	repo := setupTestDB(t)

	id, err := repo.CreateHistory(&OperationHistory{
		OperationType: OpTypeQuery,
		Input:         `{"query":"port=80","engines":["fofa"]}`,
		Status:        "success",
		TotalCount:    10,
		Summary:       `{"fofa":10}`,
		DurationMS:    1500,
	})
	if err != nil {
		t.Fatalf("CreateHistory: %v", err)
	}
	if id <= 0 {
		t.Fatal("expected positive ID")
	}

	items, total, err := repo.ListHistory("", 20, 0)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected 1 item, got %d (total=%d)", len(items), total)
	}
	if items[0].OperationType != OpTypeQuery {
		t.Errorf("expected query type, got %s", items[0].OperationType)
	}
}

func TestCreateAndGetResults(t *testing.T) {
	repo := setupTestDB(t)

	id, _ := repo.CreateHistory(&OperationHistory{
		OperationType: OpTypeQuery,
		Input:         `{"query":"test"}`,
		Status:        "success",
		TotalCount:    2,
	})

	err := repo.CreateResults(id, []OperationResult{
		{Data: `{"ip":"1.1.1.1","port":80}`},
		{Data: `{"ip":"2.2.2.2","port":443}`},
	})
	if err != nil {
		t.Fatalf("CreateResults: %v", err)
	}

	results, err := repo.GetResults(id)
	if err != nil {
		t.Fatalf("GetResults: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestDeleteAndClear(t *testing.T) {
	repo := setupTestDB(t)

	repo.CreateHistory(&OperationHistory{OperationType: OpTypeQuery, Input: "q1", Status: "success"})
	repo.CreateHistory(&OperationHistory{OperationType: OpTypeICPQuery, Input: "q2", Status: "success"})

	// Delete single
	repo.DeleteHistory(1)
	items, total, _ := repo.ListHistory("", 20, 0)
	if total != 1 || len(items) != 1 {
		t.Fatalf("after delete: expected 1, got %d", total)
	}

	// Clear all
	repo.ClearHistory("")
	items, total, _ = repo.ListHistory("", 20, 0)
	if total != 0 {
		t.Fatalf("after clear: expected 0, got %d", total)
	}
}

func TestListHistoryByType(t *testing.T) {
	repo := setupTestDB(t)

	repo.CreateHistory(&OperationHistory{OperationType: OpTypeQuery, Input: "q1", Status: "success"})
	repo.CreateHistory(&OperationHistory{OperationType: OpTypeICPQuery, Input: "q2", Status: "success"})
	repo.CreateHistory(&OperationHistory{OperationType: OpTypeQuery, Input: "q3", Status: "success"})

	items, total, _ := repo.ListHistory("query", 20, 0)
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected 2 query items, got %d", total)
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test -race ./internal/history/...
```
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/history/
git commit -m "feat: add history repository with CRUD and tests"
```

---

### Task 3: Add config support and wire up database in server

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`
- Modify: `web/server.go`
- Modify: `configs/config.yaml.example`

- [ ] **Step 1: Add History config to config_types.go**

Add after the existing config sections:
```go
type HistoryConfig struct {
	Enabled    bool   `yaml:"enabled"`
	DatabasePath string `yaml:"database_path"` // default ./data/history.db
	MaxResults int    `yaml:"max_results"`    // max results per query to store, default 1000
}
```

- [ ] **Step 2: Add default in config_defaults.go**

```go
config.History.Enabled = true
config.History.DatabasePath = "./data/history.db"
config.History.MaxResults = 1000
```

- [ ] **Step 3: Add to Server struct and initialization in server.go**

In the `Server` struct, add:
```go
historyDB     *history.Database
historyRepo   *history.Repository
```

In `NewServer`, after existing DB init:
```go
// Init history database
if cfg != nil && cfg.History.Enabled {
    hdb, err := history.NewDatabase(cfg.History.DatabasePath)
    if err != nil {
        logger.Warnf("Failed to open history database: %v", err)
    } else {
        if err := hdb.InitSchema(); err != nil {
            logger.Warnf("Failed to init history schema: %v", err)
        } else {
            srv.historyDB = hdb
            srv.historyRepo = history.NewRepository(hdb.DB())
        }
    }
}
```

- [ ] **Step 4: Register routes in server.go**

```go
if srv.historyRepo != nil {
    mux.HandleFunc("/api/v1/history", srv.handleHistoryListOrClear)
    mux.HandleFunc("/api/v1/history/", srv.handleHistoryGetOrDelete)
    mux.HandleFunc("/api/v1/history/save", srv.handleHistorySave)
}
```

- [ ] **Step 5: Add graceful shutdown**

In the `Shutdown` method, add:
```go
if s.historyDB != nil {
    s.historyDB.Close()
}
```

- [ ] **Step 6: Run build check**

```bash
go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add internal/config/ web/server.go configs/
git commit -m "feat: add history config and wire up database in server"
```

---

### Task 4: Implement history API handlers

**Files:**
- Create: `web/history_handlers.go`

- [ ] **Step 1: Create history_handlers.go**

```go
package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/unimap/project/internal/history"
)

// HistorySaveRequest is the request body for saving history.
type HistorySaveRequest struct {
	OperationType string        `json:"operation_type"`
	Input         interface{}   `json:"input"`       // will be JSON-marshaled
	Status        string        `json:"status"`
	TotalCount    int           `json:"total_count"`
	Summary       interface{}   `json:"summary,omitempty"`
	DurationMS    int64         `json:"duration_ms,omitempty"`
	Results       []interface{} `json:"results,omitempty"` // will be JSON-marshaled
}

// handleHistorySave handles POST /api/v1/history/save
func (s *Server) handleHistorySave(w http.ResponseWriter, r *http.Request) {
	if s.historyRepo == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "history_disabled", "history is not enabled", nil)
		return
	}
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required", nil)
		return
	}

	var req HistorySaveRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.OperationType == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_type", "operation_type is required", nil)
		return
	}

	inputJSON, _ := json.Marshal(req.Input)
	summaryJSON, _ := json.Marshal(req.Summary)

	h := &history.OperationHistory{
		OperationType: history.OperationType(req.OperationType),
		Input:         string(inputJSON),
		Status:        req.Status,
		TotalCount:    req.TotalCount,
		Summary:       string(summaryJSON),
		DurationMS:    req.DurationMS,
	}

	id, err := s.historyRepo.CreateHistory(h)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "db_error", err.Error(), nil)
		return
	}

	// Save results (capped by MaxResults)
	maxResults := 1000
	if s.config != nil && s.config.History.MaxResults > 0 {
		maxResults = s.config.History.MaxResults
	}
	if len(req.Results) > maxResults {
		req.Results = req.Results[:maxResults]
	}
	if len(req.Results) > 0 {
		results := make([]history.OperationResult, len(req.Results))
		for i, r := range req.Results {
			data, _ := json.Marshal(r)
			results[i] = history.OperationResult{Data: string(data)}
		}
		if err := s.historyRepo.CreateResults(id, results); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "db_error", err.Error(), nil)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
	})
}

// handleHistoryListOrClear handles GET/DELETE /api/v1/history
func (s *Server) handleHistoryListOrClear(w http.ResponseWriter, r *http.Request) {
	if s.historyRepo == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "history_disabled", "history is not enabled", nil)
		return
	}

	switch r.Method {
	case http.MethodGet:
		opType := r.URL.Query().Get("type")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if limit <= 0 {
			limit = 20
		}

		items, total, err := s.historyRepo.ListHistory(opType, limit, offset)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "db_error", err.Error(), nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"total":   total,
			"items":   items,
		})

	case http.MethodDelete:
		opType := r.URL.Query().Get("type")
		if err := s.historyRepo.ClearHistory(opType); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "db_error", err.Error(), nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})

	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET or DELETE required", nil)
	}
}

// handleHistoryGetOrDelete handles GET/DELETE /api/v1/history/:id
func (s *Server) handleHistoryGetOrDelete(w http.ResponseWriter, r *http.Request) {
	if s.historyRepo == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "history_disabled", "history is not enabled", nil)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/history/")
	if idStr == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_id", "history ID is required", nil)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_id", "invalid history ID", nil)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h, err := s.historyRepo.GetHistory(id)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "db_error", err.Error(), nil)
			return
		}
		if h == nil {
			writeAPIError(w, http.StatusNotFound, "not_found", "history not found", nil)
			return
		}
		results, err := s.historyRepo.GetResults(id)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "db_error", err.Error(), nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"history": h,
			"results": results,
		})

	case http.MethodDelete:
		if err := s.historyRepo.DeleteHistory(id); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "db_error", err.Error(), nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})

	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET or DELETE required", nil)
	}
}
```

- [ ] **Step 2: Run build check**

```bash
go build ./web/...
```

- [ ] **Step 3: Commit**

```bash
git add web/history_handlers.go
git commit -m "feat: add history API handlers (save/list/get/delete)"
```

---

### Task 5: Frontend — auto-save query results and migrate history UI

**Files:**
- Modify: `web/static/js/main.js`

- [ ] **Step 1: Add auto-save after successful query**

After the query succeeds and results are displayed, add a call to save history:

```javascript
// After query results are rendered, save to server history
function saveQueryToServerHistory(query, engines, result) {
    const input = { query, engines };
    const summary = result.engine_stats || {};
    const results = (result.assets || []).slice(0, 1000).map(a => ({
        ip: a.ip, port: a.port, protocol: a.protocol, host: a.host,
        url: a.url, title: a.title, server: a.server,
        status_code: a.status_code, country_code: a.country_code,
        source: a.source
    }));
    
    apiFetch('/api/v1/history/save', {
        method: 'POST',
        body: JSON.stringify({
            operation_type: 'query',
            input: input,
            status: result.errors && result.errors.length > 0 ? 'partial' : 'success',
            total_count: result.total_count || 0,
            summary: summary,
            results: results
        })
    }).catch(err => console.error('Failed to save query history:', err));
}
```

- [ ] **Step 2: Migrate history modal to load from server**

Replace the localStorage-based history with server API calls:

```javascript
function loadServerHistory(type) {
    return apiFetch(`/api/v1/history?type=${type || ''}&limit=50`)
        .then(data => data.items || []);
}

function clearServerHistory(type) {
    return apiFetch(`/api/v1/history?type=${type || ''}`, { method: 'DELETE' });
}

function deleteServerHistoryItem(id) {
    return apiFetch(`/api/v1/history/${id}`, { method: 'DELETE' });
}
```

- [ ] **Step 3: Update openHistoryModal to use server data**

Modify `openHistoryModal` to call `loadServerHistory` instead of `getQueryHistory`.

- [ ] **Step 4: Run build check**

```bash
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add web/static/js/main.js
git commit -m "feat: migrate frontend history to server-side persistence"
```

---

### Task 6: Integration tests

**Files:**
- Create: `web/history_handlers_test.go`

- [ ] **Step 1: Create handler tests**

```go
package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/unimap/project/internal/history"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test_history.db")
	db, err := history.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		os.Remove(dbPath)
	})
	if err := db.InitSchema(); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}
	return &Server{
		historyRepo: history.NewRepository(db.DB()),
		historyDB:   db,
	}
}

func TestHandleHistorySave(t *testing.T) {
	s := setupTestServer(t)

	body, _ := json.Marshal(HistorySaveRequest{
		OperationType: "query",
		Input:         map[string]interface{}{"query": "port=80", "engines": []string{"fofa"}},
		Status:        "success",
		TotalCount:    10,
		Summary:       map[string]interface{}{"fofa": 10},
		Results:       []interface{}{map[string]interface{}{"ip": "1.1.1.1", "port": 80}},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/history/save", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleHistorySave(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleHistoryList(t *testing.T) {
	s := setupTestServer(t)

	// Save one record
	body, _ := json.Marshal(HistorySaveRequest{
		OperationType: "query",
		Input:         "test",
		Status:        "success",
	})
	saveReq := httptest.NewRequest(http.MethodPost, "/api/v1/history/save", bytes.NewReader(body))
	saveReq.Header.Set("Content-Type", "application/json")
	saveW := httptest.NewRecorder()
	s.handleHistorySave(saveW, saveReq)

	// List
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/history", nil)
	listW := httptest.NewRecorder()
	s.handleHistoryListOrClear(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listW.Code)
	}
}
```

- [ ] **Step 2: Run tests**

```bash
go test -race ./web/...
```

- [ ] **Step 3: Commit**

```bash
git add web/history_handlers_test.go
git commit -m "test: add history API integration tests"
```
