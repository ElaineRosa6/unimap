package batchdb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/unimap/project/internal/screenshot"
)

// BatchJobRecord mirrors the in-memory batchJob struct for persistence.
// Results is stored as a JSON array of screenshot.BatchScreenshotResult.
type BatchJobRecord struct {
	ID        string                             `json:"id"`
	Status    string                             `json:"status"`
	Total     int                                `json:"total"`
	Completed int                                `json:"completed"`
	Success   int                                `json:"success"`
	Failed    int                                `json:"failed"`
	Error     string                             `json:"error,omitempty"`
	Results   []screenshot.BatchScreenshotResult `json:"results,omitempty"`
	StartedAt time.Time                          `json:"started_at"`
	EndedAt   *time.Time                         `json:"ended_at,omitempty"`
}

// Repository provides CRUD for screenshot batch job metadata.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// SaveJob upserts a batch job record (INSERT OR REPLACE).
func (r *Repository) SaveJob(job *BatchJobRecord) error {
	resultsJSON, err := json.Marshal(job.Results)
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	var endedAt interface{}
	if job.EndedAt != nil {
		endedAt = *job.EndedAt
	}

	_, err = r.db.Exec(
		`INSERT OR REPLACE INTO screenshot_batch_jobs
			(id, status, total, completed, success, failed, error_msg, results, started_at, ended_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.Status, job.Total, job.Completed, job.Success, job.Failed,
		job.Error, string(resultsJSON), job.StartedAt, endedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save batch job: %w", err)
	}
	return nil
}

// GetJob returns a single batch job by ID.
func (r *Repository) GetJob(id string) (*BatchJobRecord, error) {
	var rec BatchJobRecord
	var resultsJSON string
	var endedAt sql.NullTime

	err := r.db.QueryRow(
		`SELECT id, status, total, completed, success, failed, error_msg, results, started_at, ended_at
		 FROM screenshot_batch_jobs WHERE id = ?`, id,
	).Scan(&rec.ID, &rec.Status, &rec.Total, &rec.Completed, &rec.Success, &rec.Failed,
		&rec.Error, &resultsJSON, &rec.StartedAt, &endedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get batch job: %w", err)
	}

	if endedAt.Valid {
		t := endedAt.Time
		rec.EndedAt = &t
	}
	if resultsJSON != "" && resultsJSON != "[]" {
		if err := json.Unmarshal([]byte(resultsJSON), &rec.Results); err != nil {
			return nil, fmt.Errorf("failed to unmarshal results: %w", err)
		}
	}
	return &rec, nil
}

// ListJobs returns recent batch jobs ordered by started_at DESC.
func (r *Repository) ListJobs(limit int) ([]*BatchJobRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(
		`SELECT id, status, total, completed, success, failed, error_msg, results, started_at, ended_at
		 FROM screenshot_batch_jobs ORDER BY started_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list batch jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*BatchJobRecord
	for rows.Next() {
		var rec BatchJobRecord
		var resultsJSON string
		var endedAt sql.NullTime
		if err := rows.Scan(&rec.ID, &rec.Status, &rec.Total, &rec.Completed, &rec.Success, &rec.Failed,
			&rec.Error, &resultsJSON, &rec.StartedAt, &endedAt); err != nil {
			return nil, fmt.Errorf("failed to scan batch job: %w", err)
		}
		if endedAt.Valid {
			t := endedAt.Time
			rec.EndedAt = &t
		}
		if resultsJSON != "" && resultsJSON != "[]" {
			if err := json.Unmarshal([]byte(resultsJSON), &rec.Results); err != nil {
				return nil, fmt.Errorf("failed to unmarshal results: %w", err)
			}
		}
		jobs = append(jobs, &rec)
	}
	return jobs, rows.Err()
}

// DeleteJob removes a batch job record by ID.
func (r *Repository) DeleteJob(id string) error {
	_, err := r.db.Exec(`DELETE FROM screenshot_batch_jobs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete batch job: %w", err)
	}
	return nil
}
