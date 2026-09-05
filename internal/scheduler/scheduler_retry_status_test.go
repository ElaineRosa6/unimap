package scheduler

import (
	"context"
	"errors"
	"testing"
)

func TestSchedulerRetryStatus(t *testing.T) {
	for _, success := range []bool{false, true} {
		name := "ordinary-error-after-backoff"
		if success {
			name = "success-clears-error"
		}
		t.Run(name, func(t *testing.T) {
			s := NewScheduler("", "", 10)
			defer s.Stop()
			calls := 0
			h := retryCancelHandler{run: func(context.Context) (string, error) {
				calls++
				if success && calls == 2 {
					return "ok", nil
				}
				return "", errors.New("ordinary fixture error")
			}}
			r := s.executeTaskWithRetry(&ScheduledTask{ID: "fixture", Type: h.Type()}, h, 1, 1)
			wantStatus, wantError := "failed", "ordinary fixture error"
			if success {
				wantStatus, wantError = "success", ""
			}
			if calls != 2 || r.Status != wantStatus || r.Error != wantError || r.RetryCount != 1 {
				t.Fatalf("calls=%d record=%+v; want status=%s error=%q", calls, r, wantStatus, wantError)
			}
		})
	}
}

func TestSchedulerExpiredAttemptStatus(t *testing.T) {
	for _, reportError := range []bool{false, true} {
		name := "late-success"
		if reportError {
			name = "cooperative-timeout"
		}
		t.Run(name, func(t *testing.T) {
			s := NewScheduler("", "", 10)
			defer s.Stop()
			h := retryCancelHandler{run: func(ctx context.Context) (string, error) {
				<-ctx.Done()
				if reportError {
					return "", ctx.Err()
				}
				return "late result", nil
			}}
			r := s.executeTaskWithRetry(&ScheduledTask{ID: "fixture", Type: h.Type()}, h, 1, 0)
			if r.Status != "timeout" || r.Result != "" || r.Error == "" {
				t.Fatalf("expired attempt: %+v", r)
			}
		})
	}
}
