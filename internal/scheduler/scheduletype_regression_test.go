package scheduler

import (
	"testing"
	"time"

	"github.com/unimap/project/internal/model"
)

// These tests guard the ScheduleType validation gaps fixed after the
// qa-security-audit (FINDING-001 .. FINDING-004). They assert that the
// UpdateTask and EnableTask paths enforce the SAME rules as AddTask.

// FINDING-001: UpdateTask must reject an unknown schedule_type (AddTask does).
func TestUpdateTask_RejectsBogusScheduleType(t *testing.T) {
	s := NewScheduler("", "", 10)
	s.RegisterHandler(&testHandler{typ: TaskQuery})
	s.Start()
	defer s.Stop()

	orig := &ScheduledTask{
		Name: "t", Type: TaskQuery, Enabled: false,
		CronExpr: "0 0 * * * *", ScheduleType: "cron",
	}
	if err := s.AddTask(orig); err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	upd := &ScheduledTask{
		ID: orig.ID, Name: "t", Type: TaskQuery, Enabled: false,
		CronExpr: "0 0 * * * *", ScheduleType: "totally-bogus-type",
	}
	if err := s.UpdateTask(upd); err == nil {
		t.Fatal("UpdateTask accepted unknown schedule_type; expected error")
	}
}

// FINDING-002: UpdateTask must reject a once task with a past RunAt.
func TestUpdateTask_RejectsPastRunAt(t *testing.T) {
	s := NewScheduler("", "", 10)
	s.RegisterHandler(&testHandler{typ: TaskQuery})
	s.Start()
	defer s.Stop()

	orig := &ScheduledTask{
		Name: "t", Type: TaskQuery, Enabled: false,
		CronExpr: "0 0 * * * *", ScheduleType: "cron",
	}
	if err := s.AddTask(orig); err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	past := time.Now().Add(-1 * time.Hour)
	upd := &ScheduledTask{
		ID: orig.ID, Name: "t", Type: TaskQuery, Enabled: false,
		ScheduleType: "once", RunAt: &past,
	}
	if err := s.UpdateTask(upd); err == nil {
		t.Fatal("UpdateTask accepted past RunAt; expected error")
	}
}

// FINDING-002 (positive): UpdateTask accepts a once task with a future RunAt.
func TestUpdateTask_AcceptsFutureRunAt(t *testing.T) {
	s := NewScheduler("", "", 10)
	s.RegisterHandler(&testHandler{typ: TaskQuery})
	s.Start()
	defer s.Stop()

	orig := &ScheduledTask{
		Name: "t", Type: TaskQuery, Enabled: false,
		CronExpr: "0 0 * * * *", ScheduleType: "cron",
	}
	if err := s.AddTask(orig); err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	future := time.Now().Add(2 * time.Hour)
	upd := &ScheduledTask{
		ID: orig.ID, Name: "t", Type: TaskQuery, Enabled: false,
		ScheduleType: "once", RunAt: &future,
	}
	if err := s.UpdateTask(upd); err != nil {
		t.Fatalf("UpdateTask rejected future RunAt: %v", err)
	}
}

// FINDING-003: UpdateTask must reject a delay task with non-positive
// DelaySeconds when no RunAt is present.
func TestUpdateTask_RejectsZeroDelay(t *testing.T) {
	s := NewScheduler("", "", 10)
	s.RegisterHandler(&testHandler{typ: TaskQuery})
	s.Start()
	defer s.Stop()

	orig := &ScheduledTask{
		Name: "t", Type: TaskQuery, Enabled: false,
		CronExpr: "0 0 * * * *", ScheduleType: "cron",
	}
	if err := s.AddTask(orig); err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	upd := &ScheduledTask{
		ID: orig.ID, Name: "t", Type: TaskQuery, Enabled: false,
		ScheduleType: "delay", DelaySeconds: 0,
	}
	if err := s.UpdateTask(upd); err == nil {
		t.Fatal("UpdateTask accepted delay with DelaySeconds=0; expected error")
	}
}

// FINDING-003 (positive): UpdateTask accepts a delay task with positive delay.
func TestUpdateTask_AcceptsPositiveDelay(t *testing.T) {
	s := NewScheduler("", "", 10)
	s.RegisterHandler(&testHandler{typ: TaskQuery})
	s.Start()
	defer s.Stop()

	orig := &ScheduledTask{
		Name: "t", Type: TaskQuery, Enabled: false,
		CronExpr: "0 0 * * * *", ScheduleType: "cron",
	}
	if err := s.AddTask(orig); err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	upd := &ScheduledTask{
		ID: orig.ID, Name: "t", Type: TaskQuery, Enabled: false,
		ScheduleType: "delay", DelaySeconds: 60,
	}
	if err := s.UpdateTask(upd); err != nil {
		t.Fatalf("UpdateTask rejected positive delay: %v", err)
	}
}

// FINDING-004: EnableTask must reject an expired once task that never ran
// (LastRunAt == nil), instead of silently executing it immediately.
func TestEnableTask_RejectsExpiredNeverRunOnceTask(t *testing.T) {
	s := NewScheduler("", "", 10)
	h := &testHandler{typ: TaskQuery}
	s.RegisterHandler(h)
	s.Start()
	defer s.Stop()

	past := time.Now().Add(-1 * time.Hour)
	task := &ScheduledTask{
		Name: "expired-once", Type: TaskQuery, Enabled: false,
		ScheduleType: "once", RunAt: &past, LastRunAt: nil,
	}
	s.mu.Lock()
	s.tasks[task.ID] = task
	s.mu.Unlock()

	if err := s.EnableTask(task.ID); err == nil {
		t.Fatal("EnableTask accepted expired never-run once task; expected error")
	}
	time.Sleep(200 * time.Millisecond)
	if c := h.execCount.Load(); c != 0 {
		t.Fatalf("expired once task executed %d time(s); expected 0", c)
	}
}

// FINDING-004 (positive): EnableTask still rejects an expired once task that
// already ran (LastRunAt != nil) — the pre-existing behavior.
func TestEnableTask_RejectsExpiredRunOnceTask(t *testing.T) {
	s := NewScheduler("", "", 10)
	s.RegisterHandler(&testHandler{typ: TaskQuery})
	s.Start()
	defer s.Stop()

	past := time.Now().Add(-1 * time.Hour)
	ran := time.Now().Add(-30 * time.Minute)
	task := &ScheduledTask{
		Name: "ran-once", Type: TaskQuery, Enabled: false,
		ScheduleType: "once", RunAt: &past, LastRunAt: &ran,
	}
	s.mu.Lock()
	s.tasks[task.ID] = task
	s.mu.Unlock()

	if err := s.EnableTask(task.ID); err == nil {
		t.Fatal("EnableTask accepted expired already-run once task; expected error")
	}
}

// FINDING-005: persisted tasks survive a write and reload (atomic write
// preserves a valid JSON file). Verifies the tmp+rename path produces a
// loadable file.
func TestAtomicPersist_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	taskPath := dir + "/tasks.json"
	histPath := dir + "/history.json"
	s := NewScheduler(taskPath, histPath, 10)
	s.RegisterHandler(&testHandler{typ: TaskQuery})
	s.Start()

	future := time.Now().Add(2 * time.Hour)
	if err := s.AddTask(&ScheduledTask{
		Name: "persist-once", Type: TaskQuery, Enabled: false,
		ScheduleType: "once", RunAt: &future,
	}); err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	s.Stop()

	// Reload into a fresh scheduler — tasks must be intact.
	s2 := NewScheduler(taskPath, histPath, 10)
	s2.RegisterHandler(&testHandler{typ: TaskQuery})
	if err := s2.Load(); err != nil {
		t.Fatalf("Load after atomic write: %v", err)
	}
	s2.Start()
	defer s2.Stop()
	if len(s2.tasks) != 1 {
		t.Fatalf("expected 1 reloaded task, got %d", len(s2.tasks))
	}
}

// FINDING-006: notification payload must redact cookie_file and sensitive
// extra keys.
func TestPayloadRedaction(t *testing.T) {
	payload := &model.TaskPayload{
		CookieFile: "/secret/cookies.txt",
		Extra: map[string]any{
			"query":    "example.com",
			"api_key":  "sk-super-secret",
			"token":    "abc123",
			"normal":   42,
			"password": "hunter2",
		},
	}
	m := payloadToMap(payload)
	if m == nil {
		t.Fatal("expected non-nil map")
	}
	if m["cookie_file"] != "[REDACTED]" {
		t.Errorf("cookie_file not redacted: %v", m["cookie_file"])
	}
	extra, ok := m["extra"].(map[string]interface{})
	if !ok {
		t.Fatal("extra map missing or wrong type")
	}
	for _, key := range []string{"api_key", "token", "password"} {
		if extra[key] != "[REDACTED]" {
			t.Errorf("%s not redacted: %v", key, extra[key])
		}
	}
	// Non-sensitive fields preserved. JSON round-trip turns numbers into
	// float64, so compare against that.
	if extra["query"] != "example.com" {
		t.Errorf("query altered: %v", extra["query"])
	}
	if extra["normal"] != float64(42) {
		t.Errorf("normal altered: %v", extra["normal"])
	}
}
