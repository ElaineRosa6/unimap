package workerpool

import (
	"errors"
	"testing"
	"time"
)

type heldErrorTask struct{ started, release chan struct{} }

func (task *heldErrorTask) Execute() error {
	close(task.started)
	<-task.release
	return errors.New("fixture late error")
}

func TestPoolLateErrorAfterStopTimeout(t *testing.T) {
	p := NewPool(1)
	p.Start()
	task := &heldErrorTask{started: make(chan struct{}), release: make(chan struct{})}
	p.Submit(task)
	select {
	case <-task.started:
	case <-time.After(time.Second):
		t.Fatal("task did not start")
	}
	p.StopWithTimeout(10 * time.Millisecond)
	select {
	case _, ok := <-p.Results():
		if !ok {
			t.Error("results closed while producer still running")
		}
	default:
	}
	close(task.release)
	finished := make(chan struct{})
	go func() { p.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("late producer did not exit")
	}
	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-p.Results():
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("results not closed after producers exited")
		}
	}
}

func TestPoolStopUnblocksFullErrorQueue(t *testing.T) {
	p := NewPool(1)
	p.Start()
	// Fill without a reader, then make a worker try to publish one more error.
	for i := 0; i < cap(p.results); i++ {
		p.results <- errors.New("buffer fixture")
	}
	task := &heldErrorTask{started: make(chan struct{}), release: make(chan struct{})}
	p.Submit(task)
	<-task.started
	close(task.release)
	done := make(chan struct{})
	go func() { p.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("shutdown blocked on unconsumed errors")
		// Drain to allow cleanup even on the original implementation.
		for range p.Results() {
		}
		<-done
	}
}
