package workerpool

import (
	"testing"
	"time"
)

func TestIdlePoolStopsWithoutMonitoringTick(t *testing.T) {
	for name, create := range map[string]func() *Pool{
		"fixed":   func() *Pool { return NewPool(1) },
		"dynamic": func() *Pool { return NewDynamicPool(1, 4) },
	} {
		t.Run(name, func(t *testing.T) {
			p := create()
			p.Start()
			done := make(chan struct{})
			started := time.Now()
			go func() { p.Stop(); close(done) }()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Error("idle pool Stop waits for load-monitor ticker")
				select {
				case <-done:
				case <-time.After(7 * time.Second):
					t.Fatal("pool did not terminate")
				}
			}
			t.Logf("Stop duration: %v", time.Since(started))
			if _, ok := <-p.Results(); ok {
				t.Error("results channel remains open")
			}
			p.Stop() // idempotent shutdown is unchanged.
		})
	}
}
