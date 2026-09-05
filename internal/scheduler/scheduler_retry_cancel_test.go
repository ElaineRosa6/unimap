package scheduler

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unimap/project/internal/model"
)

type retryCancelHandler struct {
	run func(context.Context) (string, error)
}

func (retryCancelHandler) Type() TaskType { return TaskType("query") }
func (h retryCancelHandler) Execute(ctx context.Context, _ *model.TaskPayload) (string, error) {
	return h.run(ctx)
}

func TestSchedulerRetryCancellation(t *testing.T) {
	for _, mode := range []string{"before-run", "during-backoff"} {
		t.Run(mode, func(t *testing.T) {
			s := NewScheduler("", "", 10)
			defer s.Stop()
			var calls atomic.Int32
			first := make(chan struct{})
			h := retryCancelHandler{run: func(context.Context) (string, error) {
				if calls.Add(1) == 1 {
					close(first)
				}
				return "", errors.New("fixture retry error")
			}}
			if mode == "before-run" {
				s.cancel()
			}
			done := make(chan ExecutionRecord, 1)
			go func() { done <- s.executeTaskWithRetry(&ScheduledTask{ID: "fixture", Type: h.Type()}, h, 30, 1) }()
			if mode == "during-backoff" {
				select {
				case <-first:
				case <-time.After(3 * time.Second):
					t.Fatal("handler did not start")
				}
				// Give the failed attempt time to enter its two-second backoff.
				time.Sleep(50 * time.Millisecond)
				s.cancel()
			}
			var record ExecutionRecord
			select {
			case record = <-done:
			case <-time.After(500 * time.Millisecond):
				t.Error("cancellation did not interrupt retry backoff")
				select {
				case record = <-done:
				case <-time.After(4 * time.Second):
					t.Fatal("retry goroutine did not terminate")
				}
			}
			wantCalls := int32(0)
			if mode == "during-backoff" {
				wantCalls = 1
			}
			if calls.Load() != wantCalls {
				t.Errorf("handler calls=%d want=%d", calls.Load(), wantCalls)
			}
			if record.Status != "failed" || !strings.Contains(record.Error, context.Canceled.Error()) || record.RetryCount != 0 || record.FinishedAt == "" {
				t.Errorf("unexpected cancellation record: %+v", record)
			}
		})
	}
}

func TestSchedulerRetryStillSucceeds(t *testing.T) {
	s := NewScheduler("", "", 10)
	defer s.Stop()
	calls := 0
	h := retryCancelHandler{run: func(context.Context) (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("transient")
		}
		return "ok", nil
	}}
	r := s.executeTaskWithRetry(&ScheduledTask{ID: "fixture", Type: h.Type()}, h, 30, 1)
	if calls != 2 || r.Status != "success" || r.Result != "ok" || r.RetryCount != 1 {
		t.Fatalf("normal retry: calls=%d record=%+v", calls, r)
	}
}
