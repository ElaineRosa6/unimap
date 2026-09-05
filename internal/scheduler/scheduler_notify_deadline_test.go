package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/unimap/project/internal/notify"
)

type deadlineNotifyChannel struct {
	calls       int
	acknowledge bool
}

func (*deadlineNotifyChannel) ID() string      { return "deadline-fixture" }
func (*deadlineNotifyChannel) Type() string    { return "fixture" }
func (*deadlineNotifyChannel) IsEnabled() bool { return true }
func (*deadlineNotifyChannel) Close() error    { return nil }
func (c *deadlineNotifyChannel) Send(ctx context.Context, _ notify.TaskNotification) error {
	c.calls++
	<-ctx.Done()
	if c.acknowledge {
		return nil
	}
	// Some channel implementations replace the transport's context error.
	return errors.New("fixture transport failure after deadline")
}

func TestNotifyDeadlineNotRetried(t *testing.T) {
	for _, acknowledge := range []bool{false, true} {
		name := "obscured-timeout"
		if acknowledge {
			name = "acknowledged"
		}
		t.Run(name, func(t *testing.T) {
			ch := &deadlineNotifyChannel{acknowledge: acknowledge}
			err := sendNotifyChannelWithRetry(ch, notify.TaskNotification{}, 25*time.Millisecond, 1)
			if ch.calls != 1 {
				t.Errorf("send calls=%d; deadline must not cause duplicate delivery", ch.calls)
			}
			if acknowledge {
				if err != nil {
					t.Errorf("acknowledged send: %v", err)
				}
			} else if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("deadline error=%v", err)
			}
		})
	}
}
