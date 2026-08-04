package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/unimap/project/internal/notify"
)

type retryNotifyChannel struct {
	mu         sync.Mutex
	attempts   int
	failFor    int
	timeoutErr bool
}

func (c *retryNotifyChannel) ID() string      { return "retry" }
func (c *retryNotifyChannel) Type() string    { return "fixture" }
func (c *retryNotifyChannel) IsEnabled() bool { return true }
func (c *retryNotifyChannel) Close() error    { return nil }
func (c *retryNotifyChannel) Send(context.Context, notify.TaskNotification) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.attempts++
	if c.timeoutErr {
		// Simulate a channel whose request may already have been delivered but
		// whose response hit the send timeout (context.DeadlineExceeded).
		return context.DeadlineExceeded
	}
	if c.attempts <= c.failFor {
		return errors.New("transient fixture failure")
	}
	return nil
}

func TestSendNotifyChannelWithRetryRecoversTransientFailure(t *testing.T) {
	channel := &retryNotifyChannel{failFor: 1}
	if err := sendNotifyChannelWithRetry(channel, notify.TaskNotification{}, time.Second, 2); err != nil {
		t.Fatalf("retry notification: %v", err)
	}
	if channel.attempts != 2 {
		t.Fatalf("attempts=%d want=2", channel.attempts)
	}
}

func TestSendNotifyChannelWithRetryDoesNotRetryTimeout(t *testing.T) {
	// A timeout means the message may already have reached the channel; sending
	// it again would duplicate the notification, so retry must be skipped.
	channel := &retryNotifyChannel{timeoutErr: true}
	err := sendNotifyChannelWithRetry(channel, notify.TaskNotification{}, time.Second, 3)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error = %v, want context.DeadlineExceeded", err)
	}
	if channel.attempts != 1 {
		t.Fatalf("timeout error must not be retried, attempts=%d want=1", channel.attempts)
	}
}
