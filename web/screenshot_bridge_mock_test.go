package web

import (
	"context"
	"testing"
	"time"

	"github.com/unimap/project/internal/screenshot"
)

func TestBridgeMockClient_SubmitTask(t *testing.T) {
	client := newBridgeMockClient()
	task := screenshot.BridgeTask{RequestID: "req-1", URL: "https://example.com"}
	err := client.SubmitTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(client.pending))
	}
}

func TestBridgeMockClient_NextTask(t *testing.T) {
	client := newBridgeMockClient()
	client.SubmitTask(context.Background(), screenshot.BridgeTask{RequestID: "req-1"})
	client.SubmitTask(context.Background(), screenshot.BridgeTask{RequestID: "req-2"})

	task, ok := client.NextTask()
	if !ok {
		t.Fatal("expected task")
	}
	if task.RequestID != "req-1" {
		t.Fatalf("expected req-1, got %q", task.RequestID)
	}
	if len(client.pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(client.pending))
	}
	if len(client.dispatched) != 1 {
		t.Fatalf("expected 1 dispatched, got %d", len(client.dispatched))
	}
}

func TestBridgeMockClient_NextTask_Empty(t *testing.T) {
	client := newBridgeMockClient()
	_, ok := client.NextTask()
	if ok {
		t.Fatal("expected no task")
	}
}

func TestBridgeMockClient_PushResult_NoWaiter(t *testing.T) {
	client := newBridgeMockClient()
	client.SubmitTask(context.Background(), screenshot.BridgeTask{RequestID: "req-1"})
	client.NextTask()

	result := screenshot.BridgeResult{RequestID: "req-1", Success: true}
	client.PushResult(result)

	if len(client.results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(client.results))
	}
}

func TestBridgeMockClient_PushResult_WithDuration(t *testing.T) {
	client := newBridgeMockClient()
	client.SubmitTask(context.Background(), screenshot.BridgeTask{RequestID: "req-1"})
	client.NextTask()

	result := screenshot.BridgeResult{RequestID: "req-1", DurationMS: 0}
	client.PushResult(result)

	// DurationMS should be set to 1 if 0
	res := client.results["req-1"]
	if res.DurationMS != 1 {
		t.Fatalf("expected DurationMS=1, got %d", res.DurationMS)
	}
}

func TestBridgeMockClient_AwaitResult_AlreadyPushed(t *testing.T) {
	client := newBridgeMockClient()
	client.SubmitTask(context.Background(), screenshot.BridgeTask{RequestID: "req-1"})
	client.NextTask()
	client.PushResult(screenshot.BridgeResult{RequestID: "req-1", Success: true})

	result, err := client.AwaitResult(context.Background(), "req-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

func TestBridgeMockClient_AwaitResult_ContextCanceled(t *testing.T) {
	client := newBridgeMockClient()
	client.SubmitTask(context.Background(), screenshot.BridgeTask{RequestID: "req-1"})
	client.NextTask()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.AwaitResult(ctx, "req-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBridgeMockClient_Stats(t *testing.T) {
	client := newBridgeMockClient()
	client.SubmitTask(context.Background(), screenshot.BridgeTask{RequestID: "req-1"})
	client.SubmitTask(context.Background(), screenshot.BridgeTask{RequestID: "req-2"})

	pending, waiters := client.Stats()
	if pending != 2 {
		t.Fatalf("expected 2 pending, got %d", pending)
	}
	if waiters != 0 {
		t.Fatalf("expected 0 waiters, got %d", waiters)
	}
}

func TestBridgeMockClient_Stats_Nil(t *testing.T) {
	var client *bridgeMockClient
	pending, waiters := client.Stats()
	if pending != 0 || waiters != 0 {
		t.Fatal("expected 0, 0 for nil client")
	}
}

func TestBridgeMockClient_TaskForRequest_Pending(t *testing.T) {
	client := newBridgeMockClient()
	client.SubmitTask(context.Background(), screenshot.BridgeTask{RequestID: "req-1"})

	task, ok := client.TaskForRequest("req-1")
	if !ok {
		t.Fatal("expected task")
	}
	if task.RequestID != "req-1" {
		t.Fatalf("expected req-1, got %q", task.RequestID)
	}
}

func TestBridgeMockClient_TaskForRequest_Dispatched(t *testing.T) {
	client := newBridgeMockClient()
	client.SubmitTask(context.Background(), screenshot.BridgeTask{RequestID: "req-1"})
	client.NextTask()

	task, ok := client.TaskForRequest("req-1")
	if !ok {
		t.Fatal("expected task")
	}
	if task.RequestID != "req-1" {
		t.Fatalf("expected req-1, got %q", task.RequestID)
	}
}

func TestBridgeMockClient_TaskForRequest_NotFound(t *testing.T) {
	client := newBridgeMockClient()
	_, ok := client.TaskForRequest("nonexistent")
	if ok {
		t.Fatal("expected no task")
	}
}

func TestBridgeMockClient_TaskForRequest_Nil(t *testing.T) {
	var client *bridgeMockClient
	_, ok := client.TaskForRequest("req-1")
	if ok {
		t.Fatal("expected no task for nil client")
	}
}

func TestBridgeMockClient_NextTask_Nil(t *testing.T) {
	var client *bridgeMockClient
	_, ok := client.NextTask()
	if ok {
		t.Fatal("expected no task for nil client")
	}
}

func TestBridgeMockClient_PushResult_Nil(t *testing.T) {
	var client *bridgeMockClient
	client.PushResult(screenshot.BridgeResult{}) // should not panic
}

func TestBridgeMockClient_SubmitTask_Nil(t *testing.T) {
	var client *bridgeMockClient
	err := client.SubmitTask(context.Background(), screenshot.BridgeTask{})
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestBridgeMockClient_RemoveFromOrder(t *testing.T) {
	client := newBridgeMockClient()
	client.SubmitTask(context.Background(), screenshot.BridgeTask{RequestID: "req-1"})
	client.SubmitTask(context.Background(), screenshot.BridgeTask{RequestID: "req-2"})
	client.SubmitTask(context.Background(), screenshot.BridgeTask{RequestID: "req-3"})

	// Remove req-2 before it's dispatched
	client.PushResult(screenshot.BridgeResult{RequestID: "req-2"})

	// NextTask should skip req-2
	task, ok := client.NextTask()
	if !ok {
		t.Fatal("expected task")
	}
	if task.RequestID != "req-1" {
		t.Fatalf("expected req-1, got %q", task.RequestID)
	}
}
