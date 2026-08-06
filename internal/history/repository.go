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
		_ = tx.Rollback()
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()
	for _, res := range results {
		if _, err := stmt.Exec(historyID, res.Data); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to insert operation_result: %w", err)
		}
	}
	return tx.Commit()
}

// CreateHistoryWithResults atomically stores an operation and all of its result rows.
func (r *Repository) CreateHistoryWithResults(h *OperationHistory, results []OperationResult) (int64, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin history transaction: %w", err)
	}
	rollback := func(err error) (int64, error) { _ = tx.Rollback(); return 0, err }
	result, err := tx.Exec(`INSERT INTO operation_history (operation_type, input, status, total_count, summary, duration_ms, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, h.OperationType, h.Input, h.Status, h.TotalCount, h.Summary, h.DurationMS, time.Now())
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

// ListHistory returns operation history records, optionally filtered by type.
func (r *Repository) ListHistory(opType string, limit, offset int) ([]OperationHistory, int, error) {
	if limit <= 0 {
		limit = 20
	}

	var total int
	var rows *sql.Rows
	var err error
	if opType != "" {
		if qErr := r.db.QueryRow("SELECT COUNT(*) FROM operation_history WHERE operation_type = ?", opType).Scan(&total); qErr != nil {
			return nil, 0, fmt.Errorf("failed to count operation_history: %w", qErr)
		}
		rows, err = r.db.Query(
			"SELECT id, operation_type, input, status, total_count, summary, duration_ms, created_at FROM operation_history WHERE operation_type = ? ORDER BY created_at DESC LIMIT ? OFFSET ?",
			opType, limit, offset,
		)
	} else {
		if qErr := r.db.QueryRow("SELECT COUNT(*) FROM operation_history").Scan(&total); qErr != nil {
			return nil, 0, fmt.Errorf("failed to count operation_history: %w", qErr)
		}
		rows, err = r.db.Query(
			"SELECT id, operation_type, input, status, total_count, summary, duration_ms, created_at FROM operation_history ORDER BY created_at DESC LIMIT ? OFFSET ?",
			limit, offset,
		)
	}
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

// LoadPushedAssetKeys returns the set of asset keys already pushed for a task.
// An empty result set (never-pushed task) yields an empty, non-nil map so
// callers can check membership without a nil-map guard.
func (r *Repository) LoadPushedAssetKeys(taskID string) (map[string]struct{}, error) {
	rows, err := r.db.Query("SELECT asset_key FROM notify_push_state WHERE task_id = ?", taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query notify_push_state: %w", err)
	}
	defer rows.Close()

	keys := make(map[string]struct{})
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("failed to scan notify_push_state: %w", err)
		}
		keys[key] = struct{}{}
	}
	return keys, rows.Err()
}

// RecordPushedAssets records newly pushed asset keys for a task. The operation
// is idempotent: an existing (task_id, asset_key) row refreshes last_pushed_at
// instead of erroring, so re-runs after a partial failure never double count.
func (r *Repository) RecordPushedAssets(taskID string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin push-state transaction: %w", err)
	}
	stmt, err := tx.Prepare(`
		INSERT INTO notify_push_state (task_id, asset_key)
		VALUES (?, ?)
		ON CONFLICT(task_id, asset_key)
		DO UPDATE SET last_pushed_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to prepare push-state statement: %w", err)
	}
	defer stmt.Close()
	for _, key := range keys {
		if _, err := stmt.Exec(taskID, key); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to insert notify_push_state: %w", err)
		}
	}
	return tx.Commit()
}
