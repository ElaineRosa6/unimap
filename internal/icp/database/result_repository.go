package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/unimap/project/internal/adapter"
)

// ICPResultRepository defines operations for persisting and querying ICP results.
type ICPResultRepository interface {
	// SaveRun inserts a query run and returns its ID.
	SaveRun(run *ICPQueryRun) (int64, error)
	// SaveResults batch-inserts ICP results for a given run.
	SaveResults(runID int64, results []adapter.ICPResult, fetchedAt time.Time) error
	// GetRunsByTaskID returns recent runs for a task, ordered by started_at DESC.
	GetRunsByTaskID(taskID string, limit int) ([]*ICPQueryRun, error)
	// GetRunsByKeyword returns recent runs for a keyword+type combo.
	GetRunsByKeyword(keyword, queryType string, limit int) ([]*ICPQueryRun, error)
	// GetResultsByRunID returns all results for a specific run.
	GetResultsByRunID(runID int64) ([]*ICPResultRow, error)
	// GetLatestResults returns results from the most recent run for a keyword+type.
	GetLatestResults(keyword, queryType string) ([]*ICPResultRow, error)
	// GetPreviousResults returns results from the run immediately before the given time.
	GetPreviousResults(keyword, queryType string, before time.Time) ([]*ICPResultRow, error)
	// CleanupOldRuns deletes runs (and their results via CASCADE) older than the cutoff.
	CleanupOldRuns(before time.Time) (int64, error)
}

type resultRepository struct {
	db *sql.DB
}

// NewICPResultRepository creates a repository backed by the given db.
func NewICPResultRepository(db *sql.DB) ICPResultRepository {
	return &resultRepository{db: db}
}

func (r *resultRepository) SaveRun(run *ICPQueryRun) (int64, error) {
	query := `INSERT INTO icp_query_runs
		(task_id, query_keyword, query_type, page, page_size, total_records, result_count, error_msg, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := r.db.Exec(query,
		run.TaskID, run.QueryKeyword, run.QueryType,
		run.Page, run.PageSize, run.TotalRecords, run.ResultCount,
		run.ErrorMsg, run.StartedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to save ICP query run: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get run ID: %w", err)
	}
	run.ID = id
	return id, nil
}

func (r *resultRepository) SaveResults(runID int64, results []adapter.ICPResult, fetchedAt time.Time) error {
	if len(results) == 0 {
		return nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO icp_results
		(run_id, domain, service_name, licence, unit_name, nature_name, city_name,
		 content_name, main_licence, update_record, limit_access, leader_name,
		 service_type_raw, main_id_raw, detail_id_raw, domain_id_raw,
		 service_id_raw, data_id_raw, update_rec_time, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, res := range results {
		_, err := stmt.Exec(
			runID, res.Domain, res.ServiceName, res.Licence, res.UnitName,
			res.NatureName, res.CityName, res.ContentName, res.MainLicence,
			res.UpdateRecord, res.LimitAccess, res.LeaderName,
			rawJSON(res.ServiceType), rawJSON(res.MainID), rawJSON(res.DetailID),
			rawJSON(res.DomainID), rawJSON(res.ServiceID), rawJSON(res.DataID),
			res.UpdateRecTime, fetchedAt,
		)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to insert ICP result: %w", err)
		}
	}
	return tx.Commit()
}

func (r *resultRepository) GetRunsByTaskID(taskID string, limit int) ([]*ICPQueryRun, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(
		`SELECT id, task_id, query_keyword, query_type, page, page_size,
		        total_records, result_count, error_msg, started_at, created_at
		 FROM icp_query_runs WHERE task_id = ? ORDER BY started_at DESC LIMIT ?`,
		taskID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query runs by task ID: %w", err)
	}
	defer rows.Close()
	return scanRuns(rows)
}

func (r *resultRepository) GetRunsByKeyword(keyword, queryType string, limit int) ([]*ICPQueryRun, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(
		`SELECT id, task_id, query_keyword, query_type, page, page_size,
		        total_records, result_count, error_msg, started_at, created_at
		 FROM icp_query_runs WHERE query_keyword = ? AND query_type = ?
		 ORDER BY started_at DESC LIMIT ?`,
		keyword, queryType, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query runs by keyword: %w", err)
	}
	defer rows.Close()
	return scanRuns(rows)
}

func (r *resultRepository) GetResultsByRunID(runID int64) ([]*ICPResultRow, error) {
	rows, err := r.db.Query(
		`SELECT id, run_id, domain, service_name, licence, unit_name, nature_name,
		        city_name, content_name, main_licence, update_record, limit_access,
		        leader_name, service_type_raw, main_id_raw, detail_id_raw,
		        domain_id_raw, service_id_raw, data_id_raw, update_rec_time,
		        fetched_at, created_at
		 FROM icp_results WHERE run_id = ? ORDER BY id`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query results by run ID: %w", err)
	}
	defer rows.Close()
	return scanResults(rows)
}

func (r *resultRepository) GetLatestResults(keyword, queryType string) ([]*ICPResultRow, error) {
	var runID int64
	err := r.db.QueryRow(
		`SELECT id FROM icp_query_runs
		 WHERE query_keyword = ? AND query_type = ? AND error_msg = ''
		 ORDER BY started_at DESC LIMIT 1`,
		keyword, queryType,
	).Scan(&runID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find latest run: %w", err)
	}
	return r.GetResultsByRunID(runID)
}

func (r *resultRepository) GetPreviousResults(keyword, queryType string, before time.Time) ([]*ICPResultRow, error) {
	var runID int64
	err := r.db.QueryRow(
		`SELECT id FROM icp_query_runs
		 WHERE query_keyword = ? AND query_type = ? AND error_msg = '' AND started_at < ?
		 ORDER BY started_at DESC LIMIT 1`,
		keyword, queryType, before,
	).Scan(&runID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find previous run: %w", err)
	}
	return r.GetResultsByRunID(runID)
}

func (r *resultRepository) CleanupOldRuns(before time.Time) (int64, error) {
	result, err := r.db.Exec(
		`DELETE FROM icp_query_runs WHERE started_at < ?`, before,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old runs: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// rawJSON converts json.RawMessage to a string for SQLite TEXT storage.
func rawJSON(v json.RawMessage) string {
	if len(v) == 0 {
		return ""
	}
	return string(v)
}

func scanRuns(rows *sql.Rows) ([]*ICPQueryRun, error) {
	var runs []*ICPQueryRun
	for rows.Next() {
		var run ICPQueryRun
		if err := rows.Scan(
			&run.ID, &run.TaskID, &run.QueryKeyword, &run.QueryType,
			&run.Page, &run.PageSize, &run.TotalRecords, &run.ResultCount,
			&run.ErrorMsg, &run.StartedAt, &run.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan run: %w", err)
		}
		runs = append(runs, &run)
	}
	return runs, rows.Err()
}

func scanResults(rows *sql.Rows) ([]*ICPResultRow, error) {
	var results []*ICPResultRow
	for rows.Next() {
		var row ICPResultRow
		if err := rows.Scan(
			&row.ID, &row.RunID, &row.Domain, &row.ServiceName, &row.Licence,
			&row.UnitName, &row.NatureName, &row.CityName, &row.ContentName,
			&row.MainLicence, &row.UpdateRecord, &row.LimitAccess, &row.LeaderName,
			&row.ServiceTypeRaw, &row.MainIDRaw, &row.DetailIDRaw, &row.DomainIDRaw,
			&row.ServiceIDRaw, &row.DataIDRaw, &row.UpdateRecTime,
			&row.FetchedAt, &row.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, &row)
	}
	return results, rows.Err()
}
