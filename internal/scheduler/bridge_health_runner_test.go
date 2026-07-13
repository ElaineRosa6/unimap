package scheduler

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/unimap/project/internal/screenshot"
)

func TestBridgeHealthCheckRunnerWithProvider_ResolvesServiceAtExecution(t *testing.T) {
	var bridgeSvc *screenshot.BridgeService
	r := NewBridgeHealthCheckRunnerWithProvider(func() *screenshot.BridgeService {
		return bridgeSvc
	})

	client := &mockBridgeSchedulerClient{}
	bridgeSvc = screenshot.NewBridgeService(client, 5, 5*time.Second)
	bridgeSvc.Start(context.Background())
	t.Cleanup(bridgeSvc.Stop)

	result, err := r.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error after bridge initialization: %v", err)
	}
	if !strings.Contains(result, "运行中") {
		t.Errorf("result should mention 运行中: %s", result)
	}
}
