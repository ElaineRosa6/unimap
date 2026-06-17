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

	repo.DeleteHistory(1)
	items, total, _ := repo.ListHistory("", 20, 0)
	if total != 1 || len(items) != 1 {
		t.Fatalf("after delete: expected 1, got %d", total)
	}

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
