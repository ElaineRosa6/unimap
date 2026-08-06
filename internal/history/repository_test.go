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

func TestPushStateLifecycle(t *testing.T) {
	repo := setupTestDB(t)

	// Never pushed -> empty, non-nil set.
	pushed, err := repo.LoadPushedAssetKeys("task-a")
	if err != nil {
		t.Fatalf("load empty pushed set: %v", err)
	}
	if pushed == nil || len(pushed) != 0 {
		t.Fatalf("expected empty non-nil pushed set, got %v", pushed)
	}

	// Record then load.
	if err := repo.RecordPushedAssets("task-a", []string{"1.1.1.1:80", "2.2.2.2:443"}); err != nil {
		t.Fatalf("record pushed assets: %v", err)
	}
	pushed, err = repo.LoadPushedAssetKeys("task-a")
	if err != nil {
		t.Fatalf("load pushed set: %v", err)
	}
	if len(pushed) != 2 {
		t.Fatalf("expected 2 pushed keys, got %v", pushed)
	}
	for _, key := range []string{"1.1.1.1:80", "2.2.2.2:443"} {
		if _, ok := pushed[key]; !ok {
			t.Fatalf("missing pushed key %q in %v", key, pushed)
		}
	}

	// Re-recording an existing key is idempotent (no duplicate row, no error).
	if err := repo.RecordPushedAssets("task-a", []string{"1.1.1.1:80", "3.3.3.3:443"}); err != nil {
		t.Fatalf("re-record pushed assets: %v", err)
	}
	pushed, err = repo.LoadPushedAssetKeys("task-a")
	if err != nil {
		t.Fatalf("load after upsert: %v", err)
	}
	if len(pushed) != 3 {
		t.Fatalf("expected 3 distinct pushed keys after upsert, got %v", pushed)
	}

	// Per-task isolation.
	pushedB, err := repo.LoadPushedAssetKeys("task-b")
	if err != nil {
		t.Fatalf("load pushed set for task-b: %v", err)
	}
	if len(pushedB) != 0 {
		t.Fatalf("task-b must be isolated from task-a, got %v", pushedB)
	}

	// Empty key list is a no-op.
	if err := repo.RecordPushedAssets("task-a", nil); err != nil {
		t.Fatalf("record empty keys: %v", err)
	}
}

func TestHistoryAndResultsPersistAfterDatabaseReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "history.db")
	firstDB, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("open first database: %v", err)
	}
	if err := firstDB.InitSchema(); err != nil {
		_ = firstDB.Close()
		t.Fatalf("initialize first database: %v", err)
	}
	firstRepo := NewRepository(firstDB.DB())
	id, err := firstRepo.CreateHistoryWithResults(&OperationHistory{
		OperationType: OpTypeQuery,
		Input:         `{"query":"port=443"}`,
		Status:        "success",
		TotalCount:    1,
	}, []OperationResult{{Data: `{"ip":"203.0.113.10","port":443}`}})
	if err != nil {
		_ = firstDB.Close()
		t.Fatalf("save history and results: %v", err)
	}
	if err := firstDB.Close(); err != nil {
		t.Fatalf("close first database: %v", err)
	}

	secondDB, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	defer secondDB.Close()
	if err := secondDB.InitSchema(); err != nil {
		t.Fatalf("initialize reopened database: %v", err)
	}
	secondRepo := NewRepository(secondDB.DB())
	history, err := secondRepo.GetHistory(id)
	if err != nil {
		t.Fatalf("get persisted history: %v", err)
	}
	if history == nil || history.Input != `{"query":"port=443"}` {
		t.Fatalf("unexpected persisted history: %#v", history)
	}
	results, err := secondRepo.GetResults(id)
	if err != nil {
		t.Fatalf("get persisted results: %v", err)
	}
	if len(results) != 1 || results[0].Data != `{"ip":"203.0.113.10","port":443}` {
		t.Fatalf("unexpected persisted results: %#v", results)
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

	repo.DeleteHistory(1)
	items, total, _ := repo.ListHistory("", 20, 0)
	if total != 1 || len(items) != 1 {
		t.Fatalf("after delete: expected 1, got %d", total)
	}

	repo.ClearHistory("")
	_, total, _ = repo.ListHistory("", 20, 0)
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
