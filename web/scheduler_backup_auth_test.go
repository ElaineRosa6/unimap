package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/unimap/project/internal/auth"
	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/scheduler"
)

func protectedBackupTaskServer(t *testing.T) (*Server, *scheduler.ScheduledTask) {
	t.Helper()
	sched := scheduler.NewScheduler(t.TempDir()+"/tasks.json", t.TempDir()+"/history.json", 500)
	sched.Start()
	t.Cleanup(sched.Stop)
	sched.RegisterHandler(scheduler.NewBackupRunner())

	task := &scheduler.ScheduledTask{
		Name:     "protected backup",
		Type:     scheduler.TaskBackup,
		Enabled:  true,
		CronExpr: "0 0 2 * * *",
	}
	if err := sched.AddTask(task); err != nil {
		t.Fatalf("add backup task: %v", err)
	}

	cfg := &config.Config{}
	cfg.Web.Auth.Enabled = true
	return &Server{
		config:    cfg,
		scheduler: sched,
		userRepo: &mockUserRepo{users: map[int64]*auth.User{
			42: {ID: 42, Username: "operator", Role: "operator", Status: "active"},
		}},
	}, task
}

func backupTaskMutationRequest(path, taskID string) *http.Request {
	body, _ := json.Marshal(map[string]string{"id": taskID})
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8448")
	return req.WithContext(contextWithUserID(req.Context(), 42))
}

func TestHandleDeleteTask_BackupRequiresAdmin(t *testing.T) {
	s, task := protectedBackupTaskServer(t)
	rec := httptest.NewRecorder()

	s.handleDeleteTask(rec, backupTaskMutationRequest("/api/v1/scheduler/tasks/delete", task.ID))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin backup task deletion, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := s.scheduler.GetTask(task.ID); err != nil {
		t.Fatalf("backup task was deleted despite authorization failure: %v", err)
	}
}

func TestHandleDisableTask_BackupRequiresAdmin(t *testing.T) {
	s, task := protectedBackupTaskServer(t)
	rec := httptest.NewRecorder()

	s.handleDisableTask(rec, backupTaskMutationRequest("/api/v1/scheduler/tasks/disable", task.ID))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin backup task disabling, got %d: %s", rec.Code, rec.Body.String())
	}
	updated, err := s.scheduler.GetTask(task.ID)
	if err != nil {
		t.Fatalf("get backup task after authorization failure: %v", err)
	}
	if !updated.Enabled {
		t.Fatal("backup task was disabled despite authorization failure")
	}
}
