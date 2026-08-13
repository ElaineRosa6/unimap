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
	if recErr := repo.RecordPushedAssets("task-a", []string{"1.1.1.1:80", "2.2.2.2:443"}); recErr != nil {
		t.Fatalf("record pushed assets: %v", recErr)
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
	if recErr := repo.RecordPushedAssets("task-a", []string{"1.1.1.1:80", "3.3.3.3:443"}); recErr != nil {
		t.Fatalf("re-record pushed assets: %v", recErr)
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
	if initErr := firstDB.InitSchema(); initErr != nil {
		_ = firstDB.Close()
		t.Fatalf("initialize first database: %v", initErr)
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
	if closeErr := firstDB.Close(); closeErr != nil {
		t.Fatalf("close first database: %v", closeErr)
	}

	secondDB, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	defer secondDB.Close()
	if initErr := secondDB.InitSchema(); initErr != nil {
		t.Fatalf("initialize reopened database: %v", initErr)
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

func TestNotificationPushLogRoundTrip(t *testing.T) {
	repo := setupTestDB(t)

	// Empty before any push.
	logs, err := repo.ListNotificationLogs(10)
	if err != nil {
		t.Fatalf("list empty push logs: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("expected no push logs, got %d", len(logs))
	}

	first := NotificationPushLog{
		TaskID:        "task-a",
		TaskName:      "FOFA 每日巡检",
		ChannelIDs:    []string{"builtin-log", "wecom_ynmobile"},
		Status:        "success",
		ResultCount:   120,
		ResultSummary: "**查询完成｜引擎: fofa ｜ 新增 120 条（去重后）**",
	}
	second := NotificationPushLog{
		TaskID:        "task-b",
		TaskName:      "Hunter 精简巡检",
		ChannelIDs:    []string{"wecom_ynmobile"},
		Status:        "failed",
		ResultCount:   0,
		ResultSummary: "无新增资产（已全部推送过）",
		Error:         "webhook 40058 截断",
	}
	for _, l := range []NotificationPushLog{first, second} {
		if recordErr := repo.RecordNotificationLog(l); recordErr != nil {
			t.Fatalf("record push log: %v", recordErr)
		}
	}

	logs, err = repo.ListNotificationLogs(10)
	if err != nil {
		t.Fatalf("list push logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 push logs, got %d", len(logs))
	}
	// Newest first.
	if logs[0].TaskID != "task-b" || logs[1].TaskID != "task-a" {
		t.Fatalf("expected newest-first order, got %s then %s", logs[0].TaskID, logs[1].TaskID)
	}
	// ChannelIDs JSON round-trip.
	got := logs[0]
	if len(got.ChannelIDs) != 1 || got.ChannelIDs[0] != "wecom_ynmobile" {
		t.Fatalf("unexpected channel_ids: %v", got.ChannelIDs)
	}
	if got.Status != "failed" || got.Error != "webhook 40058 截断" {
		t.Fatalf("unexpected log fields: %#v", got)
	}
	if got.ResultCount != 0 || got.ResultSummary != "无新增资产（已全部推送过）" {
		t.Fatalf("unexpected result fields: %#v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("expected created_at to be set")
	}

	// Second row keeps multi-channel list.
	gotFirst := logs[1]
	if len(gotFirst.ChannelIDs) != 2 || gotFirst.ChannelIDs[1] != "wecom_ynmobile" {
		t.Fatalf("unexpected first row channel_ids: %v", gotFirst.ChannelIDs)
	}
}

func TestNotificationPushLogLimit(t *testing.T) {
	repo := setupTestDB(t)
	for i := range 3 {
		if err := repo.RecordNotificationLog(NotificationPushLog{
			TaskID: "t", TaskName: "n", ChannelIDs: []string{"log"}, Status: "success",
		}); err != nil {
			t.Fatalf("record push log %d: %v", i, err)
		}
	}

	// limit<=0 or >500 falls back to 100; here it means all 3 rows.
	for _, lim := range []int{0, -1, 600} {
		logs, err := repo.ListNotificationLogs(lim)
		if err != nil {
			t.Fatalf("list with limit=%d: %v", lim, err)
		}
		if len(logs) != 3 {
			t.Fatalf("limit=%d: expected 3 rows, got %d", lim, len(logs))
		}
	}

	// Positive limit truncates newest first.
	logs, err := repo.ListNotificationLogs(2)
	if err != nil {
		t.Fatalf("list with limit=2: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("limit=2: expected 2 rows, got %d", len(logs))
	}
}
