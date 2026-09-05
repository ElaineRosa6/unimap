package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestQueryContextPreservesValuesAndCause(t *testing.T) {
	type traceKey struct{}
	parent, cancelDeadline := context.WithTimeout(context.WithValue(context.Background(), traceKey{}, "trace-fixture"), time.Hour)
	defer cancelDeadline()
	parent, cancelCause := context.WithCancelCause(parent)
	defer cancelCause(nil)
	child := withoutDeadline(parent)
	if got := child.Value(traceKey{}); got != "trace-fixture" {
		t.Errorf("context value=%v", got)
	}
	if _, ok := child.Deadline(); ok {
		t.Error("deadline remains visible")
	}
	cause := errors.New("fixture caller cancellation")
	cancelCause(cause)
	select {
	case <-child.Done():
	case <-time.After(time.Second):
		t.Fatal("cancellation not propagated")
	}
	if !errors.Is(context.Cause(child), cause) {
		t.Errorf("cause=%v want=%v", context.Cause(child), cause)
	}
}

func TestQueryContextPreservesParentExpiry(t *testing.T) {
	parent, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	child := withoutDeadline(parent)
	select {
	case <-child.Done():
	case <-time.After(time.Second):
		t.Fatal("expired parent not propagated")
	}
	if child.Err() != context.DeadlineExceeded {
		t.Errorf("expiry error=%v", child.Err())
	}
}
