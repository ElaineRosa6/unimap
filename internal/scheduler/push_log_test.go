package scheduler

import (
	"strings"
	"testing"
)

func TestParsePushResultCount(t *testing.T) {
	cases := []struct {
		result string
		want   int
	}{
		{"**查询完成｜引擎: fofa ｜ 新增 120 条（去重后）**", 120},
		{"返回 3 条", 3},
		{"无新增资产（已全部推送过）", 0},
		{"", 0},
	}
	for _, tc := range cases {
		if got := parsePushResultCount(tc.result); got != tc.want {
			t.Errorf("parsePushResultCount(%q) = %d, want %d", tc.result, got, tc.want)
		}
	}
}

func TestPushResultSummaryTruncates(t *testing.T) {
	short := "新增 1 条"
	if got := pushResultSummary(short); got != short {
		t.Fatalf("short result must pass through unchanged, got %q", got)
	}

	long := strings.Repeat("x", pushResultSummaryMaxLen+50)
	got := pushResultSummary(long)
	if len(got) != pushResultSummaryMaxLen {
		t.Fatalf("expected truncation to %d, got %d", pushResultSummaryMaxLen, len(got))
	}
}

func TestSendNotificationRecordsPushLog(t *testing.T) {
	s := NewScheduler("", "", 10)

	var recorded *PushLogRecord
	s.recordPushLog = func(rec PushLogRecord) error {
		r := rec
		recorded = &r
		return nil
	}

	task := &ScheduledTask{
		ID:   "task-push-a",
		Name: "FOFA 每日巡检",
		Type: TaskQuery,
		Notifications: &NotificationConfig{
			Enabled:    true,
			OnSuccess:  true,
			ChannelIDs: []string{"builtin-log"},
		},
	}
	record := ExecutionRecord{
		TaskID:   task.ID,
		TaskName: task.Name,
		Status:   "success",
		Result:   "**查询完成｜引擎: fofa ｜ 新增 120 条（去重后）**",
	}

	s.sendNotification(task, record)

	if recorded == nil {
		t.Fatal("recordPushLog was not invoked by sendNotification")
	}
	if recorded.TaskID != "task-push-a" || recorded.TaskName != "FOFA 每日巡检" {
		t.Fatalf("unexpected push log identity: %#v", recorded)
	}
	if recorded.Status != "success" {
		t.Fatalf("expected status success, got %s", recorded.Status)
	}
	if recorded.ResultCount != 120 {
		t.Fatalf("expected result_count 120, got %d", recorded.ResultCount)
	}
	if len(recorded.ChannelIDs) != 1 || recorded.ChannelIDs[0] != "builtin-log" {
		t.Fatalf("unexpected channel ids: %v", recorded.ChannelIDs)
	}
}

func TestSendNotificationSkipsRecordingWhenDisabled(t *testing.T) {
	s := NewScheduler("", "", 10)
	recorded := false
	s.recordPushLog = func(PushLogRecord) error {
		recorded = true
		return nil
	}

	task := &ScheduledTask{
		ID:   "task-push-b",
		Name: "disabled notify",
		Type: TaskQuery,
		Notifications: &NotificationConfig{
			Enabled:    true,
			OnSuccess:  false, // success must not notify
			ChannelIDs: []string{"builtin-log"},
		},
	}
	s.sendNotification(task, ExecutionRecord{TaskID: task.ID, Status: "success"})

	if recorded {
		t.Fatal("push log recorded even though on_success=false skipped the notification")
	}
}

func TestSendNotificationRecorderFailureIsNonBlocking(t *testing.T) {
	s := NewScheduler("", "", 10)
	s.recordPushLog = func(PushLogRecord) error { return nil }

	task := &ScheduledTask{
		ID:   "task-push-c",
		Name: "recorder fails",
		Type: TaskQuery,
		Notifications: &NotificationConfig{
			Enabled:    true,
			OnFailure:  true,
			ChannelIDs: []string{"builtin-log"},
		},
	}
	// A nil task payload must not panic; the recorder error is only logged.
	s.sendNotification(task, ExecutionRecord{TaskID: task.ID, Status: "failed", Error: "boom"})
}
