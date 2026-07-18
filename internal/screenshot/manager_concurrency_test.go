package screenshot

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBrowserSessionLimitBlocksAndHonorsContext(t *testing.T) {
	mgr := NewManager(Config{MaxSessions: 1})
	release, err := mgr.acquireBrowserSlot(context.Background())
	if err != nil {
		t.Fatalf("acquire first slot: %v", err)
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := mgr.acquireBrowserSlot(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second acquire error = %v, want deadline exceeded", err)
	}

	release()
	secondRelease, err := mgr.acquireBrowserSlot(context.Background())
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	secondRelease()
}

func TestBrowserSessionReleaseIsIdempotent(t *testing.T) {
	mgr := NewManager(Config{MaxSessions: 1})
	release, err := mgr.acquireBrowserSlot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
	release()
}

func TestPersistentChromeProfileForcesSingleSession(t *testing.T) {
	mgr := NewManager(Config{UserDataDir: t.TempDir(), MaxSessions: 4})
	if got := cap(mgr.sessionSlots); got != 1 {
		t.Fatalf("session capacity = %d, want 1 for a shared Chrome profile", got)
	}
}
