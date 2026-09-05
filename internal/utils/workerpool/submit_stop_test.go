package workerpool

import (
	"testing"
	"testing/synctest"
	"time"
)

type submitStopTask func() error

func (f submitStopTask) Execute() error { return f() }

func TestPoolStopReleasesBlockedSubmit(t *testing.T) {
	for name, makePool := range map[string]func() *Pool{
		"fixed":   func() *Pool { return NewPool(1) },
		"dynamic": func() *Pool { return NewDynamicPool(1, 2) },
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				p := makePool()
				p.Start()
				release := make(chan struct{})
				started := make(chan struct{})
				p.Submit(submitStopTask(func() error { close(started); <-release; return nil }))
				<-started
				for i := 0; i < cap(p.tasks); i++ {
					p.Submit(&MockTask{})
				}
				returned := make(chan any, 1)
				go func() {
					defer func() { returned <- recover() }()
					p.Submit(&MockTask{})
				}()
				// The worker and the extra submitter are now durably blocked.
				synctest.Wait()
				select {
				case <-returned:
					t.Fatal("Submit did not block on a full queue")
				default:
				}
				p.StopWithTimeout(10 * time.Millisecond)
				synctest.Wait()
				select {
				case v := <-returned:
					if v != nil {
						t.Errorf("blocked Submit panicked during Stop: %v", v)
					}
				default:
					t.Error("Stop left Submit blocked")
				}
				close(release)
				for range p.Results() {
				}
				// Calls after shutdown remain harmless, including a full buffered queue.
				p.Submit(&MockTask{})
				p.Stop()
			})
		})
	}
}
