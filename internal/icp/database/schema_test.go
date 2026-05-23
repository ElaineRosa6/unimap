package database

import (
	"testing"
	"time"

	"github.com/unimap-icp-hunter/project/internal/adapter"
)

func setupTestDB(t *testing.T) (*Database, func()) {
	t.Helper()
	dir := t.TempDir()
	db, err := NewDatabase(dir + "/test.db")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.InitSchema(); err != nil {
		db.Close()
		t.Fatalf("failed to init schema: %v", err)
	}
	return db, func() { db.Close() }
}

func TestDatabase_NewAndInitSchema(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	if db.db == nil {
		t.Fatal("expected non-nil db")
	}
}

func TestICPResultRepository_SaveAndQueryRun(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewICPResultRepository(db.db)

	run := &ICPQueryRun{
		TaskID:       "task-1",
		QueryKeyword: "example.com",
		QueryType:    "web",
		Page:         1,
		PageSize:     20,
		TotalRecords: 5,
		ResultCount:  3,
		StartedAt:    time.Now(),
	}

	id, err := repo.SaveRun(run)
	if err != nil {
		t.Fatalf("SaveRun failed: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive run ID, got %d", id)
	}
	if run.ID != id {
		t.Errorf("run.ID not updated, expected %d got %d", id, run.ID)
	}
}

func TestICPResultRepository_SaveResults(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewICPResultRepository(db.db)

	// Insert a run first.
	run := &ICPQueryRun{
		TaskID:       "t1", QueryKeyword: "test.com", QueryType: "web",
		Page: 1, PageSize: 20, TotalRecords: 3, ResultCount: 2,
		StartedAt: time.Now(),
	}
	runID, err := repo.SaveRun(run)
	if err != nil {
		t.Fatalf("SaveRun failed: %v", err)
	}

	results := []adapter.ICPResult{
		{
			Domain:      "test1.com",
			Licence:     "京ICP备00000001号",
			UnitName:    "公司A",
			NatureName:  "企业",
			CityName:    "北京",
			MainLicence: "京ICP备00000001号-1",
		},
		{
			Domain:      "test2.com",
			Licence:     "京ICP备00000002号",
			UnitName:    "公司B",
			NatureName:  "企业",
			CityName:    "上海",
			MainLicence: "京ICP备00000002号-1",
		},
	}

	if err := repo.SaveResults(runID, results, time.Now()); err != nil {
		t.Fatalf("SaveResults failed: %v", err)
	}

	// Verify results count.
	rows, err := repo.GetResultsByRunID(runID)
	if err != nil {
		t.Fatalf("GetResultsByRunID failed: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 results, got %d", len(rows))
	}
	if rows[0].Domain != "test1.com" {
		t.Errorf("expected domain test1.com, got %s", rows[0].Domain)
	}
}

func TestICPResultRepository_GetLatestResults(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	repo := NewICPResultRepository(db.db)

	// Run 1.
	run1 := &ICPQueryRun{
		TaskID: "t1", QueryKeyword: "domain.com", QueryType: "web",
		Page: 1, PageSize: 20, TotalRecords: 1, ResultCount: 1,
		StartedAt: time.Now().Add(-time.Hour),
	}
	id1, _ := repo.SaveRun(run1)
	repo.SaveResults(id1, []adapter.ICPResult{{Domain: "domain.com", Licence: "old-licence"}}, time.Now().Add(-time.Hour))

	// Run 2 (more recent).
	run2 := &ICPQueryRun{
		TaskID: "t1", QueryKeyword: "domain.com", QueryType: "web",
		Page: 1, PageSize: 20, TotalRecords: 1, ResultCount: 1,
		StartedAt: time.Now(),
	}
	id2, _ := repo.SaveRun(run2)
	repo.SaveResults(id2, []adapter.ICPResult{{Domain: "domain.com", Licence: "new-licence"}}, time.Now())

	latest, err := repo.GetLatestResults("domain.com", "web")
	if err != nil {
		t.Fatalf("GetLatestResults failed: %v", err)
	}
	if len(latest) != 1 {
		t.Fatalf("expected 1 latest result, got %d", len(latest))
	}
	if latest[0].Licence != "new-licence" {
		t.Errorf("expected new-licence, got %s", latest[0].Licence)
	}
}

func TestICPResultRepository_GetPreviousResults(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	repo := NewICPResultRepository(db.db)

	now := time.Now()

	run1 := &ICPQueryRun{
		TaskID: "t1", QueryKeyword: "domain.com", QueryType: "web",
		Page: 1, PageSize: 20, TotalRecords: 1, ResultCount: 1,
		StartedAt: now.Add(-2 * time.Hour),
	}
	id1, _ := repo.SaveRun(run1)
	repo.SaveResults(id1, []adapter.ICPResult{{Domain: "domain.com", Licence: "v1"}}, now.Add(-2*time.Hour))

	run2 := &ICPQueryRun{
		TaskID: "t1", QueryKeyword: "domain.com", QueryType: "web",
		Page: 1, PageSize: 20, TotalRecords: 1, ResultCount: 1,
		StartedAt: now.Add(-time.Hour),
	}
	id2, _ := repo.SaveRun(run2)
	repo.SaveResults(id2, []adapter.ICPResult{{Domain: "domain.com", Licence: "v2"}}, now.Add(-time.Hour))

	prev, err := repo.GetPreviousResults("domain.com", "web", run2.StartedAt)
	if err != nil {
		t.Fatalf("GetPreviousResults failed: %v", err)
	}
	if len(prev) != 1 {
		t.Fatalf("expected 1 previous result, got %d", len(prev))
	}
	if prev[0].Licence != "v1" {
		t.Errorf("expected v1, got %s", prev[0].Licence)
	}
}

func TestICPResultRepository_CleanupOldRuns(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	repo := NewICPResultRepository(db.db)

	oldRun := &ICPQueryRun{
		TaskID: "t1", QueryKeyword: "old.com", QueryType: "web",
		Page: 1, PageSize: 20, TotalRecords: 0, ResultCount: 0,
		StartedAt: time.Now().Add(-24 * time.Hour),
	}
	repo.SaveRun(oldRun)

	newRun := &ICPQueryRun{
		TaskID: "t1", QueryKeyword: "new.com", QueryType: "web",
		Page: 1, PageSize: 20, TotalRecords: 0, ResultCount: 0,
		StartedAt: time.Now(),
	}
	repo.SaveRun(newRun)

	n, err := repo.CleanupOldRuns(time.Now().Add(-12 * time.Hour))
	if err != nil {
		t.Fatalf("CleanupOldRuns failed: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 deleted, got %d", n)
	}
}

func TestICPResultRepository_SaveResults_Empty(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	repo := NewICPResultRepository(db.db)

	err := repo.SaveResults(1, nil, time.Now())
	if err != nil {
		t.Fatalf("SaveResults with empty slice should return nil: %v", err)
	}
}

func TestRawJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"nil raw", "", ""},
		{"empty string", "", ""},
		{"json string", `"web"`, `"web"`},
		{"json number", `123`, `123`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw adapter.ICPResult
			if tt.in == "" {
				got := rawJSON(nil)
				if got != tt.want {
					t.Errorf("rawJSON(nil) = %q, want %q", got, tt.want)
				}
			} else {
				_ = raw
				got := rawJSON([]byte(tt.in))
				if got != tt.want {
					t.Errorf("rawJSON(%q) = %q, want %q", tt.in, got, tt.want)
				}
			}
		})
	}
}
