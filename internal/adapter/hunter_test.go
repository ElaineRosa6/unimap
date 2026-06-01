package adapter

import (
	"context"
	"testing"
	"time"
)

// TestHunterWaitForRate verifies the qps throttle enforces a minimum interval
// between consecutive requests so concurrent queries don't burst Hunter.
func TestHunterWaitForRate(t *testing.T) {
	t.Run("no throttle when qps<=0", func(t *testing.T) {
		h := NewHunterAdapter("http://example.com", "key", 0, time.Second)
		start := time.Now()
		for i := 0; i < 5; i++ {
			if err := h.waitForRate(context.Background()); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
			t.Errorf("expected no throttle, but waited %v", elapsed)
		}
	})

	t.Run("enforces minimum interval at qps=10", func(t *testing.T) {
		h := NewHunterAdapter("http://example.com", "key", 10, time.Second)
		// minInterval = 100ms; 3 sequential calls => ~200ms total (first is free)
		start := time.Now()
		for i := 0; i < 3; i++ {
			if err := h.waitForRate(context.Background()); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		elapsed := time.Since(start)
		if elapsed < 180*time.Millisecond {
			t.Errorf("expected >=180ms for 3 calls at qps=10, got %v", elapsed)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		h := NewHunterAdapter("http://example.com", "key", 1, time.Second)
		// First call reserves the slot immediately.
		if err := h.waitForRate(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Second call must wait ~1s; cancel quickly and confirm it returns the ctx error.
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		start := time.Now()
		if err := h.waitForRate(ctx); err == nil {
			t.Error("expected context error, got nil")
		}
		if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
			t.Errorf("expected fast cancellation, waited %v", elapsed)
		}
	})
}
