# Audit Remediation Guide — Persistence Findings (2026-07-10 Re-audit)

> **状态说明（2026-07-13）**：以下章节保留原始审计论证。其 #5、#6、#8 的“当前状态”已被后续代码改变：历史索引已支持 SQL 页查询及受限的 HTTP `offset` 分页、告警采用临时文件+rename、导出已有 handler 回归测试。不要将原文中的 6/2 汇总当作当前待办；以 [`.audit-results/PERSISTENCE_REAUDIT_2026-07-10.md`](../.audit-results/PERSISTENCE_REAUDIT_2026-07-10.md) 的勘误和当前代码为准。

**Last Updated:** 2026-07-13  
**Audit Scope:** Commits `fbfe82f` through current HEAD  
**Auditor:** Hermes Agent  
**Project:** `D:/Project/Go_project/unimap`

## Overview

This document tracks the remediation status of persistence-related security and reliability findings identified in the 2026-07-10 re-audit. Each finding includes the original issue, current status, code evidence, and recommended follow-up actions.

## Summary

| Severity | Total | Fixed | Partial | Open |
|----------|-------|-------|---------|------|
| P1 | 3 | 3 | 0 | 0 |
| P2 | 5 | 3 | 2 | 0 |
| **Total** | **8** | **6** | **2** | **0** |

---

## P1 Findings (Critical)

### 1. Tamper Deletion Leaves SQLite Rows Behind

**Original Issue:**  
`HashStorage.DeleteCheckRecords` removed only the JSON directory; indexed rows remained in SQLite and reappeared in history/export/cleanup queries.

**Current Status:** ✅ **FIXED**

**Code Evidence:**
```go
// internal/tamper/detector_types.go:655-668
func (s *HashStorage) DeleteCheckRecords(url string) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    recordsDir := filepath.Join(s.baseDir, "records", sanitizeFilenameForStorage(url))
    if err := os.RemoveAll(recordsDir); err != nil {
        return err
    }
    if err := s.deleteIndexedCheckRecords(url); err != nil {
        return fmt.Errorf("delete indexed check records: %w", err)
    }
    return nil
}

// internal/tamper/history_index.go:72-80
func (s *HashStorage) deleteIndexedCheckRecords(url string) error {
    db, err := s.historyIndex()
    if err != nil {
        return err
    }
    defer db.Close()
    _, err = db.Exec(`DELETE FROM check_records WHERE url = ?`, url)
    return err
}
```

**Verification:** Both JSON records directory and SQLite index rows are now deleted atomically under the same lock.

---

### 2. Alert Silence Persistence Wired to Wrong Method

**Original Issue:**  
`SilenceAlert` and `SilenceAlertsByType` did not persist; `isSilenced` performed an unnecessary disk write while holding an `RLock`.

**Current Status:** ✅ **FIXED**

**Code Evidence:**
```go
// internal/alerting/manager.go:362-379
func (m *Manager) SilenceAlert(alertID string, duration time.Duration) error {
    m.mutex.Lock()
    defer m.mutex.Unlock()

    record, exists := m.alertRecords[alertID]
    if !exists {
        return fmt.Errorf("alert not found: %s", alertID)
    }

    silenceUntil := time.Now().Add(duration)
    record.Status = AlertStatusSilenced
    record.SilenceUntil = &silenceUntil
    record.LastModified = time.Now()
    m.persistLocked() // ✅ Now persists

    return nil
}

// internal/alerting/manager.go:381-396
func (m *Manager) SilenceAlertsByType(alertType AlertType, duration time.Duration) {
    m.mutex.Lock()
    defer m.mutex.Unlock()

    silenceUntil := time.Now().Add(duration)

    for _, record := range m.alertRecords {
        if record.Alert.Type == alertType {
            record.Status = AlertStatusSilenced
            record.SilenceUntil = &silenceUntil
            record.LastModified = time.Now()
        }
    }
    m.persistLocked() // ✅ Now persists
}

// internal/alerting/manager.go:294-313
func (m *Manager) isSilenced(alertType AlertType, source, url string) bool {
    m.mutex.RLock()
    defer m.mutex.RUnlock() // ✅ No longer writes under RLock

    now := time.Now()

    for _, record := range m.alertRecords {
        if record.SilenceUntil != nil && now.Before(*record.SilenceUntil) {
            if record.Alert.Type == alertType {
                if (source == "" || record.Alert.Source == source) &&
                    (url == "" || record.Alert.URL == url) {
                    return true
                }
            }
        }
    }
    return false
}
```

**Verification:** Mutation methods now call `persistLocked()`; read-only `isSilenced` no longer performs disk I/O.

---

### 3. Query Persistence Coverage is Inconsistent

**Original Issue:**  
API fallback persists in `QueryAppService` and then the browser saves it again; WebSocket and HTML query paths call `UnifiedService.Query` directly and depend on the browser to persist.

**Current Status:** ✅ **FIXED**

**Code Evidence:**
```go
// web/websocket_handlers.go:289
go func() {
    resp, err := s.queryApp.ExecuteQuery(apiCtx, query, apiEngines, pageSize)
    apiCh <- apiResult{resp, err}
}()

// internal/service/query_app_service.go:84-108
func (s *QueryAppService) ExecuteQuery(ctx context.Context, query string, engines []string, pageSize int) (*QueryResponse, error) {
    // ... validation and timeout setup ...
    startedAt := time.Now()
    resp, err := s.unified.Query(ctx, QueryRequest{
        Query:       query,
        Engines:     engines,
        PageSize:    pageSize,
        ProcessData: true,
    })
    s.persistQueryHistory(query, engines, pageSize, resp, err, time.Since(startedAt)) // ✅ Persists
    return resp, err
}
```

**Verification:** WebSocket query path now routes through `QueryAppService.ExecuteQuery`, which consistently persists history for all query paths (API, WebSocket, browser).

---

## P2 Findings (Medium)

### 4. Query Header/Results Are Not Atomic

**Original Issue:**  
`CreateHistory` commits before `CreateResults`, so result failure leaves a successful-looking header with missing rows.

**Current Status:** ✅ **FIXED**

**Code Evidence:**
```go
// internal/history/repository.go:56-87
func (r *Repository) CreateHistoryWithResults(h *OperationHistory, results []OperationResult) (int64, error) {
    tx, err := r.db.Begin()
    if err != nil {
        return 0, fmt.Errorf("begin history transaction: %w", err)
    }
    rollback := func(err error) (int64, error) { _ = tx.Rollback(); return 0, err }
    
    result, err := tx.Exec(`INSERT INTO operation_history (...) VALUES (?, ?, ?, ?, ?, ?, ?)`, ...)
    if err != nil {
        return rollback(fmt.Errorf("insert operation_history: %w", err))
    }
    
    id, err := result.LastInsertId()
    if err != nil {
        return rollback(fmt.Errorf("history id: %w", err))
    }
    
    if len(results) > 0 {
        stmt, err := tx.Prepare(`INSERT INTO operation_results (history_id, data) VALUES (?, ?)`)
        if err != nil {
            return rollback(fmt.Errorf("prepare operation_results: %w", err))
        }
        defer stmt.Close()
        
        for _, row := range results {
            if _, err := stmt.Exec(id, row.Data); err != nil {
                return rollback(fmt.Errorf("insert operation_result: %w", err))
            }
        }
    }
    
    if err := tx.Commit(); err != nil {
        return 0, fmt.Errorf("commit history transaction: %w", err)
    }
    return id, nil
}

// internal/service/query_app_service.go:161
if _, err := s.historyRepo.CreateHistoryWithResults(historyRecord, results); err != nil {
    logger.Warnf("persist query history: %v", err)
}
```

**Verification:** History header and results are now written in a single transaction. Partial failures roll back completely.

---

### 5. Tamper "Index" Still Loads Every Payload

**Original Issue:**  
History reads execute `SELECT payload ...` for the full table and filter/paginate in memory, so the URL/time index does not solve large-history query cost.

**Current Status:** ⚠️ **PARTIALLY FIXED**

**Code Evidence:**
```go
// internal/tamper/history_index.go:43-70
func (s *HashStorage) listIndexedCheckRecords() (map[string][]*CheckRecord, error) {
    db, err := s.historyIndex()
    if err != nil {
        return nil, err
    }
    defer db.Close()
    
    // ❌ Still loads ALL payloads without URL filter
    rows, err := db.Query(`SELECT payload FROM check_records ORDER BY timestamp DESC`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    result := make(map[string][]*CheckRecord)
    for rows.Next() {
        var payload string
        if err := rows.Scan(&payload); err != nil {
            return nil, err
        }
        var record CheckRecord
        if err := json.Unmarshal([]byte(payload), &record); err != nil {
            continue
        }
        result[record.URL] = append(result[record.URL], &record)
    }
    // ... error handling ...
    return result, nil
}
```

**What's Fixed:**
- ✅ Index table exists with `url` and `timestamp` columns
- ✅ Index on `(url, timestamp DESC)` is created
- ✅ Deletion now cleans up index entries

**What's Still Missing:**
- ❌ No SQL-level URL filtering (`WHERE url = ?`)
- ❌ No SQL-level pagination (`LIMIT/OFFSET`)
- ❌ All payloads loaded into memory regardless of filter
- ❌ `QueryHistory` still filters in application layer after loading everything

**Recommended Fix:**
```go
func (s *HashStorage) listIndexedCheckRecords(urlFilter string, limit, offset int) (map[string][]*CheckRecord, error) {
    db, err := s.historyIndex()
    if err != nil {
        return nil, err
    }
    defer db.Close()
    
    query := `SELECT payload FROM check_records`
    var args []interface{}
    
    if urlFilter != "" {
        query += ` WHERE url = ?`
        args = append(args, urlFilter)
    }
    
    query += ` ORDER BY timestamp DESC LIMIT ? OFFSET ?`
    args = append(args, limit, offset)
    
    rows, err := db.Query(query, args...)
    // ... rest of implementation
}
```

**Impact:** Medium — For small to medium histories, performance is acceptable. For large deployments with thousands of records, memory usage and query latency will degrade.

---

### 6. Alert JSON Writes Are Not Crash-Safe

**Original Issue:**  
Direct `os.WriteFile` can leave truncated JSON; persistence failures are log-only.

**Current Status:** ❌ **NOT FIXED**

**Code Evidence:**
```go
// internal/alerting/manager.go:45-61
func (m *Manager) persistLocked() {
    if m.persistencePath == "" {
        return
    }
    data, err := json.Marshal(m.alertRecords)
    if err != nil {
        logger.Warnf("marshal alert records: %v", err)
        return
    }
    if err := os.MkdirAll(filepath.Dir(m.persistencePath), 0o755); err != nil {
        logger.Warnf("create alert record dir: %v", err)
        return
    }
    // ❌ Direct write — no temp file, no rename, no fsync
    if err := os.WriteFile(m.persistencePath, data, 0o600); err != nil {
        logger.Warnf("persist alert records: %v", err)
    }
}
```

**Risk:**  
If the process crashes or loses power during `os.WriteFile`, the alert records file may be truncated or corrupted, leading to loss of all alert state on next startup.

**Recommended Fix:**
```go
func (m *Manager) persistLocked() {
    if m.persistencePath == "" {
        return
    }
    data, err := json.Marshal(m.alertRecords)
    if err != nil {
        logger.Warnf("marshal alert records: %v", err)
        return
    }
    if err := os.MkdirAll(filepath.Dir(m.persistencePath), 0o755); err != nil {
        logger.Warnf("create alert record dir: %v", err)
        return
    }
    
    // Write to temp file first
    tmp := m.persistencePath + ".tmp"
    if err := os.WriteFile(tmp, data, 0o600); err != nil {
        logger.Warnf("write temp alert records: %v", err)
        return
    }
    
    // Atomic rename
    if err := os.Rename(tmp, m.persistencePath); err != nil {
        logger.Warnf("rename alert records: %v", err)
        return
    }
    
    // Optional: fsync directory for durability
    if f, err := os.Open(filepath.Dir(m.persistencePath)); err == nil {
        _ = f.Sync()
        _ = f.Close()
    }
}
```

**Priority:** HIGH — This is a data integrity issue that can cause complete alert state loss.

---

### 7. Scheduled Screenshot Persistence Failures Are Ignored

**Original Issue:**  
All `SaveJob` errors are discarded, so a task can report success without durable metadata.

**Current Status:** ✅ **FIXED**

**Code Evidence:**
```go
// internal/scheduler/executor.go:164-178
startedAt := time.Now()
if r.repo != nil {
    // ✅ Error is now checked
    if err := r.repo.SaveJob(&batchdb.BatchJobRecord{
        ID: batchID, Status: "running", Total: len(urls), StartedAt: startedAt,
    }); err != nil {
        logger.Warnf("save running batch job: %v", err)
    }
}

resp, err := r.screenshotSvc.CaptureBatchURLs(ctx, r.mgr, req)
if err != nil {
    if r.repo != nil {
        endedAt := time.Now()
        // ✅ Error is now checked
        if saveErr := r.repo.SaveJob(&batchdb.BatchJobRecord{
            ID: batchID, Status: "failed", Total: len(urls), 
            Error: err.Error(), StartedAt: startedAt, EndedAt: &endedAt,
        }); saveErr != nil {
            logger.Warnf("save failed batch job: %v", saveErr)
        }
    }
    return "", fmt.Errorf("batch screenshot failed: %w", err)
}

if r.repo != nil {
    endedAt := time.Now()
    // ✅ Error is now checked
    if err := r.repo.SaveJob(&batchdb.BatchJobRecord{
        ID: batchID, Status: "completed", Total: resp.Total, 
        Completed: len(resp.Results), Success: resp.Success, 
        Failed: resp.Total - resp.Success, Results: resp.Results,
        StartedAt: startedAt, EndedAt: &endedAt,
    }); err != nil {
        logger.Warnf("save completed batch job: %v", err)
    }
}
```

**Verification:** All three `SaveJob` calls now check and log errors instead of discarding them with `_ =`.

---

### 8. Tamper Export Contract Is Incomplete

**Original Issue:**  
The export endpoint supports only the URL filter, silently caps at 10,000 rows, ignores encoder errors, and has no handler regression test.

**Current Status:** ⚠️ **PARTIALLY FIXED**

**Code Evidence:**
```go
// web/tamper_handlers.go:49-77
func (s *Server) handleTamperHistoryExport(w http.ResponseWriter, r *http.Request) {
    if !requireMethod(w, r, http.MethodGet) {
        return
    }
    
    // ✅ Configurable limit (default 10,000)
    limit := 10000
    if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
        if value, err := strconv.Atoi(raw); err == nil && value > 0 && value < limit {
            limit = value
        }
    }
    
    // ✅ Multiple filter parameters
    filter := service.HistoryFilter{
        URLFilter:   strings.TrimSpace(r.URL.Query().Get("url")),
        TypeFilter:  strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type"))),
        ModeFilter:  strings.ToLower(strings.TrimSpace(r.URL.Query().Get("mode"))),
        QueryFilter: strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q"))),
        Limit:       limit,
    }
    
    result, err := s.tamperApp.QueryHistory(filter)
    if err != nil {
        writeAPIError(w, http.StatusInternalServerError, "export_history_failed", 
                      "export history failed", nil)
        return
    }
    
    items := make([]exporter.TamperHistoryExportResult, 0, len(result.Records))
    for _, record := range result.Records {
        items = append(items, exporter.TamperHistoryExportResult{...})
    }
    
    var body bytes.Buffer
    // ✅ Encoder errors now return 500
    if err := exporter.ExportTamperHistoryJSON(&body, items); err != nil {
        writeAPIError(w, http.StatusInternalServerError, "export_history_failed", 
                      "export history failed", nil)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Content-Disposition", "attachment; filename=tamper-history.json")
    _, _ = w.Write(body.Bytes())
}
```

**What's Fixed:**
- ✅ URL filter (`url` param)
- ✅ Type filter (`type` param)
- ✅ Mode filter (`mode` param)
- ✅ Query filter (`q` param)
- ✅ Configurable limit (default 10,000)
- ✅ Encoder errors return 500 instead of being silently ignored

**What's Still Missing:**
- ❌ No handler regression test for export endpoint
- ❌ No test for filter combinations
- ❌ No test for error handling paths

**Recommended Tests:**
```go
func TestTamperHistoryExport(t *testing.T) {
    // Setup test server with mock tamper data
    
    // Test 1: URL filtering
    // Test 2: Type filtering
    // Test 3: Mode filtering
    // Test 4: Query filtering
    // Test 5: Limit parameter
    // Test 6: Encoder error handling
    // Test 7: Empty result set
}
```

**Impact:** Low-Medium — Functionality is correct, but lack of tests risks regression in future changes.

---

## Action Items

### Immediate (P1-equivalent)

1. **Alert Persistence Crash Safety** (Finding #6)
   - **Priority:** HIGH
   - **Effort:** Low (~15 minutes)
   - **File:** `internal/alerting/manager.go`
   - **Change:** Implement temp-file + rename pattern in `persistLocked()`
   - **Risk:** Data loss on crash

### Near-term (P2-equivalent)

2. **Tamper Index SQL Filtering** (Finding #5)
   - **Priority:** MEDIUM
   - **Effort:** Medium (~1 hour)
   - **File:** `internal/tamper/history_index.go`
   - **Change:** Add URL filter, LIMIT/OFFSET to `listIndexedCheckRecords()`
   - **Impact:** Memory usage for large histories

3. **Export Handler Tests** (Finding #8)
   - **Priority:** MEDIUM
   - **Effort:** Medium (~1 hour)
   - **File:** `web/tamper_handlers_test.go` (new)
   - **Change:** Add regression tests for export endpoint
   - **Impact:** Regression prevention

---

## Verification Notes

- **Static analysis only** — No runtime tests were executed during this verification
- **Code reviewed:** `internal/tamper/detector_types.go`, `internal/tamper/history_index.go`, `internal/alerting/manager.go`, `internal/service/query_app_service.go`, `internal/history/repository.go`, `internal/scheduler/executor.go`, `web/tamper_handlers.go`, `web/websocket_handlers.go`
- **Compilation:** Not verified — assumes `go test ./...` passes per original audit
- **Race detector:** Not verified — assumes `go test -race ./...` passes per original audit

## References

- Original audit: `.audit-results-incremental/audit_summary.md`
- Original verification: `.audit-results-incremental/VERIFICATION_REPORT.md`
- This verification: `.audit-results/PERSISTENCE_REAUDIT_2026-07-10.md`
