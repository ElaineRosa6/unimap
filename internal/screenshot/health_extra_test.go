package screenshot

import (
	"context"
	"testing"
	"time"
)

// TestExtensionHealthChecker_LiveClientGate verifies a paired-client gate.
// When LiveClient is provided and reports false, the checker must report
// unhealthy even though the bridge service is started and queue is fine.
func TestExtensionHealthChecker_LiveClientGate(t *testing.T) {
	svc := NewBridgeService(&mockBridgeClient{}, 5, 30*time.Second)
	svc.Start(context.Background())
	defer svc.Stop()

	tests := []struct {
		name       string
		liveClient bool
		want       bool
	}{
		{"no live client -> unhealthy", false, false},
		{"live client present -> healthy", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &ExtensionHealthChecker{
				BridgeService: svc,
				LiveClient:    func() bool { return tt.liveClient },
			}
			got, err := e.Check(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Check() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestExtensionHealthChecker_RecentActivity verifies the recent-activity
// freshness gate. A zero timestamp is interpreted as "never seen" and is
// permitted (so freshly-started services don't immediately fail), while
// a stale-but-nonzero timestamp must fail.
func TestExtensionHealthChecker_RecentActivity(t *testing.T) {
	svc := NewBridgeService(&mockBridgeClient{}, 5, 30*time.Second)
	svc.Start(context.Background())
	defer svc.Stop()

	fixedNow := time.Unix(2_000_000_000, 0)
	cutoff := 60 * time.Second

	tests := []struct {
		name        string
		lastUnix    int64
		want        bool
		description string
	}{
		{"never seen activity -> healthy", 0, true, "0 means no activity ever; allowed"},
		{"recent activity -> healthy", fixedNow.Unix() - 30, true, "30s ago, within 60s cutoff"},
		{"stale activity -> unhealthy", fixedNow.Unix() - 120, false, "2min ago, beyond cutoff"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &ExtensionHealthChecker{
				BridgeService:        svc,
				LiveClient:           func() bool { return true },
				LastActivity:         func() int64 { return tt.lastUnix },
				RecentActivityCutoff: cutoff,
				now:                  func() time.Time { return fixedNow },
			}
			got, err := e.Check(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Check() = %v, want %v (%s)", got, tt.want, tt.description)
			}
		})
	}
}

// TestExtensionHealthChecker_NilLiveClient_TreatedAsAbsent verifies that a
// bridge service started without a LiveClient provider is reported as
// unhealthy. "Service started" alone does not prove any extension is paired.
func TestExtensionHealthChecker_NilLiveClient_TreatedAsAbsent(t *testing.T) {
	svc := NewBridgeService(&mockBridgeClient{}, 5, 30*time.Second)
	svc.Start(context.Background())
	defer svc.Stop()

	e := &ExtensionHealthChecker{BridgeService: svc}
	ok, err := e.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false: bridge started but no live-client signal")
	}
}

// TestExtensionHealthChecker_MockHonorsLiveClient verifies that even in mock
// mode the LiveClient gate still applies. The mock fast-path used to always
// return true; tests now pin that the gate is observable.
func TestExtensionHealthChecker_MockHonorsLiveClient(t *testing.T) {
	svc := NewBridgeService(&mockBridgeClient{}, 5, 30*time.Second)
	svc.Start(context.Background())
	defer svc.Stop()

	e := &ExtensionHealthChecker{
		BridgeService: svc,
		IsMock:        true,
		LiveClient:    func() bool { return false },
	}
	ok, err := e.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("mock mode must still honor LiveClient=false")
	}
}
