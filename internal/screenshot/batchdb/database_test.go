package batchdb

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/unimap/project/internal/screenshot"
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

func TestSaveAndGetJob(t *testing.T) {
	repo := setupTestDB(t)

	started := time.Now()
	job := &BatchJobRecord{
		ID:        "batch_123",
		Status:    "completed",
		Total:     3,
		Completed: 3,
		Success:   2,
		Failed:    1,
		Results: []screenshot.BatchScreenshotResult{
			{URL: "http://a.com", Success: true, FilePath: "/screenshots/batch_123/a.png", Timestamp: started.Unix()},
			{URL: "http://b.com", Success: true, FilePath: "/screenshots/batch_123/b.png", Timestamp: started.Unix()},
			{URL: "http://c.com", Success: false, Error: "timeout", Timestamp: started.Unix()},
		},
		StartedAt: started,
	}
	ended := started.Add(5 * time.Second)
	job.EndedAt = &ended

	if err := repo.SaveJob(job); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}

	got, err := repo.GetJob("batch_123")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got == nil {
		t.Fatal("expected job, got nil")
	}
	if got.Status != "completed" || got.Total != 3 || got.Success != 2 || got.Failed != 1 {
		t.Fatalf("unexpected fields: %+v", got)
	}
	if len(got.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got.Results))
	}
	if got.Results[0].URL != "http://a.com" || !got.Results[0].Success {
		t.Fatalf("unexpected first result: %+v", got.Results[0])
	}
	if got.Results[2].Error != "timeout" {
		t.Fatalf("unexpected third result error: %q", got.Results[2].Error)
	}
	if got.EndedAt == nil || !got.EndedAt.After(got.StartedAt) {
		t.Fatalf("unexpected ended_at: %+v", got.EndedAt)
	}
}

func TestSaveJob_Upsert(t *testing.T) {
	repo := setupTestDB(t)

	job := &BatchJobRecord{
		ID:        "batch_456",
		Status:    "running",
		Total:     2,
		StartedAt: time.Now(),
	}
	if err := repo.SaveJob(job); err != nil {
		t.Fatalf("SaveJob (insert): %v", err)
	}

	// Update same ID — should replace, not duplicate
	job.Status = "completed"
	job.Completed = 2
	job.Success = 2
	if err := repo.SaveJob(job); err != nil {
		t.Fatalf("SaveJob (upsert): %v", err)
	}

	jobs, err := repo.ListJobs(10)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job after upsert, got %d", len(jobs))
	}
	if jobs[0].Status != "completed" {
		t.Fatalf("expected status=completed after upsert, got %q", jobs[0].Status)
	}
}

func TestListJobs_OrderedByStartedAtDesc(t *testing.T) {
	repo := setupTestDB(t)

	base := time.Now()
	jobs := []*BatchJobRecord{
		{ID: "old", Status: "completed", Total: 1, StartedAt: base.Add(-2 * time.Hour)},
		{ID: "new", Status: "completed", Total: 1, StartedAt: base.Add(-1 * time.Hour)},
		{ID: "newest", Status: "completed", Total: 1, StartedAt: base},
	}
	for _, j := range jobs {
		if err := repo.SaveJob(j); err != nil {
			t.Fatalf("SaveJob %s: %v", j.ID, err)
		}
	}

	got, err := repo.ListJobs(10)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(got))
	}
	if got[0].ID != "newest" || got[1].ID != "new" || got[2].ID != "old" {
		t.Fatalf("expected newest/new/old order, got %s/%s/%s", got[0].ID, got[1].ID, got[2].ID)
	}
}

func TestGetJob_NotFound(t *testing.T) {
	repo := setupTestDB(t)
	got, err := repo.GetJob("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for nonexistent job, got %+v", got)
	}
}

func TestDeleteJob(t *testing.T) {
	repo := setupTestDB(t)
	job := &BatchJobRecord{ID: "batch_del", Status: "completed", Total: 1, StartedAt: time.Now()}
	if err := repo.SaveJob(job); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}
	if err := repo.DeleteJob("batch_del"); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}
	got, err := repo.GetJob("batch_del")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil after delete, got %+v", got)
	}
}

func TestSaveJob_EmptyResults(t *testing.T) {
	repo := setupTestDB(t)
	job := &BatchJobRecord{
		ID:        "batch_empty",
		Status:    "failed",
		Total:     0,
		Error:     "no valid URLs",
		StartedAt: time.Now(),
	}
	if err := repo.SaveJob(job); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}
	got, err := repo.GetJob("batch_empty")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got == nil {
		t.Fatal("expected job, got nil")
	}
	if len(got.Results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(got.Results))
	}
	if got.Error != "no valid URLs" {
		t.Fatalf("expected error msg, got %q", got.Error)
	}
}
