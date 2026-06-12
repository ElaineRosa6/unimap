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

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM operation_history %s", where)
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count operation_history: %w", err)
	}

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
