package workerpool

import (
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestPoolLifecycleResizeAfterStop(t *testing.T) {
	p := NewDynamicPool(1, 4)
	p.Start()
	p.SetConcurrency(4)
	p.Stop()
	defer func() {
		if v := recover(); v != nil {
			t.Errorf("resize after Stop panicked: %v", v)
		}
	}()
	before := p.GetConcurrency()
	p.SetConcurrency(1)
	if p.GetConcurrency() != before {
		t.Error("stopped pool accepted resize")
	}
	p.SetConcurrency(4)
	p.Wait()
}

func TestPoolLifecycleRestartIsNoOp(t *testing.T) {
	p := NewPool(1)
	p.Start()
	p.Stop()
	p.Start()
	if atomic.LoadInt32(&p.running) != 0 {
		t.Fatal("closed pool was restarted")
	}
	p.Submit(&MockTask{})
	p.Stop()
	p.Wait()
}

func TestPoolLifecycleConfigureBeforeStart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := NewDynamicPool(1, 4)
		p.SetConcurrency(4)
		started := make(chan struct{}, 4)
		release := make(chan struct{})
		// Inspect the internal queue to prove configuration alone spawns no worker.
		for i := 0; i < 4; i++ {
			p.tasks <- submitStopTask(func() error { started <- struct{}{}; <-release; return nil })
		}
		synctest.Wait()
		if len(started) != 0 {
			t.Errorf("workers ran before Start: %d", len(started))
		}
		p.Start()
		synctest.Wait()
		if len(started) != 4 {
			t.Errorf("Start ignored configured concurrency: started=%d", len(started))
		}
		close(release)
		p.Stop()
	})
}

func TestPoolLifecycleConcurrentOperations(t *testing.T) {
	for i := 0; i < 50; i++ {
		p := NewDynamicPool(1, 4)
		if i%2 == 0 {
			p.Start()
		}
		gate := make(chan struct{})
		var wg sync.WaitGroup
		operations := []func(){
			p.Start,
			func() { p.SetConcurrency(4); p.SetConcurrency(1) },
			p.adjustConcurrency,
			func() { p.StopWithTimeout(time.Second) },
			func() { p.Submit(&MockTask{}) },
		}
		for _, op := range operations {
			wg.Add(1)
			go func() { defer wg.Done(); <-gate; op() }()
		}
		close(gate)
		wg.Wait()
		p.Stop()
		p.Wait()
		if atomic.LoadInt32(&p.running) != 0 {
			t.Fatal("pool revived during concurrent shutdown")
		}
		if _, ok := <-p.Results(); ok {
			t.Fatal("Results not closed after workers exit")
		}
	}
}

func TestPoolLifecycleNoRevivalWhileStopping(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := NewDynamicPool(1, 4)
		p.Start()
		started, release := make(chan struct{}), make(chan struct{})
		p.Submit(submitStopTask(func() error { close(started); <-release; return nil }))
		<-started
		p.StopWithTimeout(10 * time.Millisecond)
		p.SetConcurrency(4)
		p.Start()
		if atomic.LoadInt32(&p.running) != 0 || p.GetConcurrency() != 1 {
			t.Error("timed-out shutdown allowed worker creation")
		}
		close(release)
		for range p.Results() {
		}
		p.Wait()
	})
}
