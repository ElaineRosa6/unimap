package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/unimap/project/internal/auth"
	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/scheduler"
)

// setupScheduler creates a scheduler with the "query" handler registered for tests
func setupScheduler(t *testing.T) *scheduler.Scheduler {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "scheduler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})
	sched := scheduler.NewScheduler(tmpDir+"/tasks.json", tmpDir+"/history.json", 500)
	sched.Start()
	sched.RegisterHandler(&mockQueryHandler{})
	return sched
}

// mockQueryHandler is a minimal handler for testing
type mockQueryHandler struct{}

func (h *mockQueryHandler) Type() scheduler.TaskType { return scheduler.TaskQuery }
func (h *mockQueryHandler) Execute(ctx context.Context, payload *model.TaskPayload) (string, error) {
	return "mock query result", nil
}

func TestHandleCreateTask_NoScheduler_Returns503(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"name":      "test",
		"type":      "query",
		"cron_expr": "0 * * * *",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleCreateTask(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleCreateTask_GetMethod_Returns405(t *testing.T) {
	storePath := t.TempDir() + "/tasks.json"
	historyPath := t.TempDir() + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	defer sched.Stop()

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/tasks", nil)
	s.handleCreateTask(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleCreateTask_EmptyName_Returns400(t *testing.T) {
	storePath := t.TempDir() + "/tasks.json"
	historyPath := t.TempDir() + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	defer sched.Stop()

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"name":      "",
		"type":      "query",
		"cron_expr": "0 * * * *",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleCreateTask(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleCreateTask_EmptyCron_Returns400(t *testing.T) {
	storePath := t.TempDir() + "/tasks.json"
	historyPath := t.TempDir() + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	defer sched.Stop()

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"name":      "test",
		"type":      "query",
		"cron_expr": "",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleCreateTask(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleCreateTask_Success(t *testing.T) {
	sched := setupScheduler(t)

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"name":      "test task",
		"type":      "query",
		"enabled":   true,
		"cron_expr": "0 * * * *",
		"payload":   map[string]interface{}{"query": "country=\"CN\""},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleCreateTask(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["message"] != "task created" {
		t.Fatalf("expected 'task created', got %v", resp["message"])
	}
	if resp["id"] == "" {
		t.Fatal("expected non-empty task ID")
	}
}

func TestHandleCreateTask_MissingRunnerFieldReturnsClearError(t *testing.T) {
	sched := setupScheduler(t)
	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"name":      "screenshots",
		"type":      "batch_screenshot",
		"cron_expr": "0 * * * *",
		"payload":   map[string]interface{}{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")

	s.handleCreateTask(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{"batch_screenshot", "urls", "targets"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("error response %q does not contain %q", rec.Body.String(), want)
		}
	}
}

func TestHandleCreateTask_PreservesBackupPayloadFields(t *testing.T) {
	storePath := t.TempDir() + "/tasks.json"
	historyPath := t.TempDir() + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	defer sched.Stop()
	sched.RegisterHandler(scheduler.NewBackupRunner())

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"name":      "scheduled backup",
		"type":      "backup",
		"enabled":   true,
		"cron_expr": "0 0 2 * * *",
		"payload": map[string]interface{}{
			"sources":     []string{"configs", "data"},
			"output_dir":  "backups",
			"prefix":      "nightly",
			"max_backups": 3,
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")

	s.handleCreateTask(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	task, err := sched.GetTask(resp["id"].(string))
	if err != nil {
		t.Fatalf("get created task: %v", err)
	}
	if task.Payload == nil || task.Payload.Extra == nil {
		t.Fatal("backup payload extra fields were discarded")
	}
	if got := task.Payload.Extra["prefix"]; got != "nightly" {
		t.Errorf("prefix = %v, want nightly", got)
	}
	if got := task.Payload.Extra["max_backups"]; got != float64(3) {
		t.Errorf("max_backups = %v, want 3", got)
	}
	if got, ok := task.Payload.Extra["sources"].([]interface{}); !ok || len(got) != 2 {
		t.Errorf("sources = %#v, want two entries", task.Payload.Extra["sources"])
	}
}

func TestMapToTaskPayloadPreservesNotificationDetailLimit(t *testing.T) {
	payload := mapToTaskPayload(map[string]any{
		"query":                     `port="443"`,
		"notification_detail_limit": 25,
	})
	if payload.NotificationDetailLimit != 25 {
		t.Fatalf("notification detail limit = %d, want 25", payload.NotificationDetailLimit)
	}
	if payload.Extra != nil {
		if _, duplicated := payload.Extra["notification_detail_limit"]; duplicated {
			t.Fatal("typed notification detail limit was also duplicated into Extra")
		}
	}
}

func TestMapToTaskPayloadNormalizesCommaSeparatedURLs(t *testing.T) {
	payload := mapToTaskPayload(map[string]any{
		"urls": "https://a.test, https://b.test",
	})
	want := []string{"https://a.test", "https://b.test"}
	if !slices.Equal(payload.URLs, want) {
		t.Fatalf("URLs = %#v, want %#v", payload.URLs, want)
	}
}

func TestHandleCreateTask_BackupRequiresAdmin(t *testing.T) {
	storePath := t.TempDir() + "/tasks.json"
	historyPath := t.TempDir() + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	defer sched.Stop()
	sched.RegisterHandler(scheduler.NewBackupRunner())

	cfg := &config.Config{}
	cfg.Web.Auth.Enabled = true
	s := &Server{
		config:    cfg,
		scheduler: sched,
		userRepo: &mockUserRepo{users: map[int64]*auth.User{
			42: {ID: 42, Username: "operator", Role: "operator", Status: "active"},
		}},
	}
	body, _ := json.Marshal(map[string]interface{}{
		"name":      "exfiltration attempt",
		"type":      "backup",
		"enabled":   true,
		"cron_expr": "0 0 2 * * *",
		"payload": map[string]interface{}{
			"sources":    []string{"configs/config.yaml"},
			"output_dir": "web/static",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	req = req.WithContext(contextWithUserID(req.Context(), 42))
	rec := httptest.NewRecorder()

	s.handleCreateTask(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin backup task creation, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := len(sched.ListTasks()); got != 0 {
		t.Fatalf("backup task was created despite authorization failure: %d tasks", got)
	}
}

func TestHandleListTasks_NoScheduler_Returns503(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/tasks", nil)
	s.handleListTasks(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleListTasks_Empty(t *testing.T) {
	storePath := t.TempDir() + "/tasks.json"
	historyPath := t.TempDir() + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	defer sched.Stop()

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/tasks", nil)
	s.handleListTasks(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandleListTasks_AfterCreate(t *testing.T) {
	sched := setupScheduler(t)
	defer sched.Stop()

	// 先创建一个任务
	task := &scheduler.ScheduledTask{
		Name:     "list-test",
		Type:     "query",
		Enabled:  true,
		CronExpr: "0 * * * *",
	}
	sched.AddTask(task)

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/tasks", nil)
	s.handleListTasks(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var tasks []map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&tasks)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0]["name"] != "list-test" {
		t.Fatalf("expected name 'list-test', got %v", tasks[0]["name"])
	}
}

func TestHandleUpdateTask_NoScheduler_Returns503(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"id":        "some-id",
		"name":      "updated",
		"cron_expr": "0 * * * *",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleUpdateTask(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleUpdateTask_MissingID_Returns400(t *testing.T) {
	storePath := t.TempDir() + "/tasks.json"
	historyPath := t.TempDir() + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	defer sched.Stop()

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"name":      "updated",
		"cron_expr": "0 * * * *",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleUpdateTask(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleUpdateTask_NotFound_Returns404(t *testing.T) {
	storePath := t.TempDir() + "/tasks.json"
	historyPath := t.TempDir() + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	defer sched.Stop()

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"id":        "nonexistent",
		"name":      "updated",
		"cron_expr": "0 * * * *",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleUpdateTask(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleRunTaskNow_NoScheduler_Returns503(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"id": "some-id"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleRunTaskNow(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleRunTaskNow_MissingID_Returns400(t *testing.T) {
	storePath := t.TempDir() + "/tasks.json"
	historyPath := t.TempDir() + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	defer sched.Stop()

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleRunTaskNow(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleRunTaskNow_NotFound_Returns404(t *testing.T) {
	storePath := t.TempDir() + "/tasks.json"
	historyPath := t.TempDir() + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	defer sched.Stop()

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"id": "nonexistent"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleRunTaskNow(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleRunTaskNow_Success(t *testing.T) {
	sched := setupScheduler(t)
	defer sched.Stop()

	// 先创建任务
	task := &scheduler.ScheduledTask{
		Name:     "run-now-test",
		Type:     "query",
		Enabled:  true,
		CronExpr: "0 * * * *",
	}
	sched.AddTask(task)

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"id": task.ID})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleRunTaskNow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["message"] != "task scheduled for immediate execution" {
		t.Fatalf("unexpected message: %v", resp["message"])
	}
}

func TestHandleDeleteTask_NoScheduler_Returns503(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"id": "some-id"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleDeleteTask(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleTaskHistory_NoScheduler_Returns503(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/history", nil)
	s.handleTaskHistory(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleTaskHistory_Empty(t *testing.T) {
	storePath := t.TempDir() + "/tasks.json"
	historyPath := t.TempDir() + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	defer sched.Stop()

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/history", nil)
	s.handleTaskHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandleEnableTask_NoScheduler_Returns503(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"id": "some-id"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/enable", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleEnableTask(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleDisableTask_NoScheduler_Returns503(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"id": "some-id"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/disable", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleDisableTask(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleSchedulerPage(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/scheduler", nil)
	s.handleSchedulerPage(rec, req)

	// 模板可能不存在，但不应 panic
}

func TestHandleGetTask_NoScheduler_Returns503(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/tasks?id=1", nil)
	s.handleGetTask(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleGetTask_MissingID_Returns400(t *testing.T) {
	storePath := t.TempDir() + "/tasks.json"
	historyPath := t.TempDir() + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	defer sched.Stop()

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/tasks", nil)
	s.handleGetTask(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleCreateTask_BadJSON_Returns400(t *testing.T) {
	storePath := t.TempDir() + "/tasks.json"
	historyPath := t.TempDir() + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	defer sched.Stop()

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleCreateTask(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestWriteSchedulerJSONError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeSchedulerJSONError(rec, http.StatusBadRequest, "test error")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "test error" {
		t.Fatalf("expected 'test error', got %v", resp["error"])
	}
}

func TestWriteJSON_SchedulerTest(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]string{"key": "value"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandleTaskHistory_WithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := tmpDir + "/tasks.json"
	historyPath := tmpDir + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	defer sched.Stop()

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/history?limit=10", nil)
	s.handleTaskHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

// 验证 scheduler 初始化不泄漏文件描述符
func TestScheduler_CleanupOnClose(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := tmpDir + "/tasks.json"
	historyPath := tmpDir + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	sched.Stop()

	// 再次调用 Stop 不应 panic
	sched.Stop()
}

func TestCreateTask_InvalidCronExpression(t *testing.T) {
	storePath := t.TempDir() + "/tasks.json"
	historyPath := t.TempDir() + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	defer sched.Stop()

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"name":      "bad-cron",
		"type":      "query",
		"cron_expr": "not a valid cron",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleCreateTask(rec, req)

	// 无效 cron 应被拒绝
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid cron, got %d", rec.Code)
	}
}

func TestHandleRunTaskNow_GetMethod_Returns405(t *testing.T) {
	storePath := t.TempDir() + "/tasks.json"
	historyPath := t.TempDir() + "/history.json"
	sched := scheduler.NewScheduler(storePath, historyPath, 500)
	sched.Start()
	defer sched.Stop()

	s := &Server{scheduler: sched}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/tasks/run", nil)
	s.handleRunTaskNow(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

// ============================================================
// validateTaskPayload tests
// ============================================================

func TestValidateTaskPayload_Nil(t *testing.T) {
	if err := validateTaskPayload(scheduler.TaskScreenshotCleanup, nil); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidateTaskPayload_TooManyKeys(t *testing.T) {
	payload := make(map[string]interface{})
	for i := 0; i < maxPayloadKeys+1; i++ {
		payload[fmt.Sprintf("key%d", i)] = "value"
	}
	if err := validateTaskPayload(scheduler.TaskScreenshotCleanup, payload); err == nil {
		t.Fatal("expected error for too many keys")
	}
}

func TestValidateTaskPayload_Valid(t *testing.T) {
	payload := map[string]interface{}{"query": "test", "limit": 10}
	if err := validateTaskPayload(scheduler.TaskQuery, payload); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidateTaskPayload_AcceptsCompatibleShapes(t *testing.T) {
	tests := []struct {
		name     string
		taskType scheduler.TaskType
		payload  map[string]interface{}
	}{
		{"legacy targets string", scheduler.TaskPortScan, map[string]interface{}{"targets": "https://a.test,https://b.test"}},
		{"comma separated sources", scheduler.TaskBackup, map[string]interface{}{"sources": "config,baseline"}},
		{"legacy extra query", scheduler.TaskQuery, map[string]interface{}{"extra": map[string]interface{}{"query": "domain=example.com"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateTaskPayload(tt.taskType, tt.payload); err != nil {
				t.Fatalf("expected compatible payload to pass: %v", err)
			}
		})
	}
}

func TestValidateTaskPayload_InvalidWebhookScheme(t *testing.T) {
	payload := map[string]interface{}{"webhook_url": "ftp://example.com/webhook"}
	if err := validateTaskPayload(scheduler.TaskScreenshotCleanup, payload); err == nil {
		t.Fatal("expected error for invalid webhook scheme")
	}
}

// ============================================================
// handleDeleteTask additional tests
// ============================================================

func TestHandleDeleteTask_MethodNotAllowed(t *testing.T) {
	s := &Server{scheduler: setupScheduler(t)}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/tasks/delete", nil)
	s.handleDeleteTask(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleDeleteTask_MissingID(t *testing.T) {
	s := &Server{scheduler: setupScheduler(t)}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"id": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleDeleteTask(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleDeleteTask_NotFound(t *testing.T) {
	s := &Server{scheduler: setupScheduler(t)}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"id": "nonexistent"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleDeleteTask(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleDeleteTask_Success(t *testing.T) {
	sched := setupScheduler(t)
	s := &Server{scheduler: sched}
	// Create a task
	createBody, _ := json.Marshal(map[string]interface{}{
		"name": "del-test", "type": "query", "cron_expr": "0 * * * *",
		"payload": map[string]interface{}{"query": "test"},
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Origin", "http://localhost:8448")
	createRec := httptest.NewRecorder()
	s.handleCreateTask(createRec, createReq)
	var createResp map[string]interface{}
	json.NewDecoder(createRec.Body).Decode(&createResp)
	taskID := createResp["id"].(string)

	deleteBody, _ := json.Marshal(map[string]interface{}{"id": taskID})
	deleteReq := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/delete", bytes.NewReader(deleteBody))
	deleteReq.Header.Set("Content-Type", "application/json")
	deleteReq.Header.Set("Origin", "http://localhost:8448")
	deleteRec := httptest.NewRecorder()
	s.handleDeleteTask(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", deleteRec.Code)
	}
}

// ============================================================
// handleDisableTask tests
// ============================================================

func TestHandleDisableTask_MethodNotAllowed(t *testing.T) {
	s := &Server{scheduler: setupScheduler(t)}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/tasks/disable", nil)
	s.handleDisableTask(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleDisableTask_MissingID(t *testing.T) {
	s := &Server{scheduler: setupScheduler(t)}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"id": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/disable", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleDisableTask(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleDisableTask_Success(t *testing.T) {
	sched := setupScheduler(t)
	s := &Server{scheduler: sched}
	createBody, _ := json.Marshal(map[string]interface{}{
		"name": "disable-test", "type": "query", "cron_expr": "0 * * * *",
		"payload": map[string]interface{}{"query": "test"},
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Origin", "http://localhost:8448")
	createRec := httptest.NewRecorder()
	s.handleCreateTask(createRec, createReq)
	var createResp map[string]interface{}
	json.NewDecoder(createRec.Body).Decode(&createResp)
	taskID := createResp["id"].(string)

	body, _ := json.Marshal(map[string]interface{}{"id": taskID})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/disable", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	rec := httptest.NewRecorder()
	s.handleDisableTask(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

// ============================================================
// handleEnableTask tests
// ============================================================

func TestHandleEnableTask_MethodNotAllowed(t *testing.T) {
	s := &Server{scheduler: setupScheduler(t)}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scheduler/tasks/enable", nil)
	s.handleEnableTask(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleEnableTask_MissingID(t *testing.T) {
	s := &Server{scheduler: setupScheduler(t)}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{"id": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/enable", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	s.handleEnableTask(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleEnableTask_Success(t *testing.T) {
	sched := setupScheduler(t)
	s := &Server{scheduler: sched}
	createBody, _ := json.Marshal(map[string]interface{}{
		"name": "enable-test", "type": "query", "cron_expr": "0 * * * *", "enabled": false,
		"payload": map[string]interface{}{"query": "test"},
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Origin", "http://localhost:8448")
	createRec := httptest.NewRecorder()
	s.handleCreateTask(createRec, createReq)
	var createResp map[string]interface{}
	json.NewDecoder(createRec.Body).Decode(&createResp)
	taskID := createResp["id"].(string)

	body, _ := json.Marshal(map[string]interface{}{"id": taskID})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduler/tasks/enable", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	rec := httptest.NewRecorder()
	s.handleEnableTask(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
