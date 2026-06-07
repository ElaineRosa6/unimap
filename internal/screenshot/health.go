package screenshot

import (
	"context"
	"net/http"
	"time"

	"github.com/unimap/project/internal/logger"
)

// HealthChecker probes a screenshot mode for liveness.
type HealthChecker interface {
	Check(ctx context.Context) (bool, error)
	Mode() string // "cdp" or "extension"
}

// LocalChromeFinder reports whether a local Chrome/Chromium binary is
// available on this host. Used by CDPHealthChecker when no remote debug URL
// is configured to distinguish "spawnable" from "Chrome not installed".
type LocalChromeFinder func() string

// CDPHealthChecker checks CDP mode health.
//
// Decision matrix (probed each tick):
//
//	RemoteDebugURL set + /json/version 200  → healthy
//	RemoteDebugURL set + endpoint offline    → unhealthy
//	RemoteDebugURL empty + local Chrome path → healthy (spawnable)
//	RemoteDebugURL empty + no Chrome found   → unhealthy
//
// Override is honored first when non-nil (test hook).
type CDPHealthChecker struct {
	RemoteDebugURL string
	// LocalChromeFinder, if non-nil, is invoked when RemoteDebugURL is empty.
	// It returns the resolved Chrome path or "" if not found. Defaults to
	// findChromePath when nil.
	LocalChromeFinder LocalChromeFinder
	// Override, if non-nil, overrides the actual health check result (for testing).
	Override *bool
}

func (c *CDPHealthChecker) Mode() string { return "cdp" }

func (c *CDPHealthChecker) Check(ctx context.Context) (bool, error) {
	if c.Override != nil {
		return *c.Override, nil
	}

	if c.RemoteDebugURL == "" {
		// No remote URL: rely on a local Chrome being launchable. Without one
		// we'd just fail at first capture, so report unhealthy now.
		finder := c.LocalChromeFinder
		if finder == nil {
			finder = findChromePath
		}
		if finder() == "" {
			logger.Debugf("CDP health: no remote debug URL and no local Chrome binary found")
			return false, nil
		}
		return true, nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, c.RemoteDebugURL+"/json/version", nil)
	if err != nil {
		return false, err
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Debugf("CDP health check failed for %s: %v", c.RemoteDebugURL, err)
		return false, nil
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// LiveClientProvider reports whether at least one extension client is
// currently paired/live with the bridge.
type LiveClientProvider func() bool

// LastActivityProvider returns the unix timestamp of the most recent
// extension activity (task pull or callback). Zero means "never".
type LastActivityProvider func() int64

// ExtensionHealthChecker checks Extension Bridge mode health.
//
// Beyond the BridgeService running, the checker now optionally requires:
//   - At least one live client (when LiveClient is non-nil).
//   - Recent activity within RecentActivityCutoff (when LastActivity is
//     non-nil and RecentActivityCutoff > 0). A zero LastActivity timestamp
//     is treated as "never seen", which is fine immediately after start;
//     it only fails the check once activity has been seen but is now stale.
//
// All extra signals are opt-in. When the providers are nil, behavior matches
// the previous "started + queue not overloaded" check so existing callers
// keep working.
type ExtensionHealthChecker struct {
	BridgeService *BridgeService
	IsMock        bool
	// LiveClient, if non-nil, reports whether any extension client is paired.
	LiveClient LiveClientProvider
	// LastActivity, if non-nil, returns the unix timestamp of the most recent
	// task pull or callback.
	LastActivity LastActivityProvider
	// RecentActivityCutoff, when >0 and LastActivity is set, requires the
	// most recent activity to be within this duration to count as healthy.
	RecentActivityCutoff time.Duration
	// Override, if non-nil, overrides the actual health check result (for testing).
	Override *bool
	// now is overridable in tests; defaults to time.Now.
	now func() time.Time
}

func (e *ExtensionHealthChecker) Mode() string { return "extension" }

func (e *ExtensionHealthChecker) Check(ctx context.Context) (bool, error) {
	if e.Override != nil {
		return *e.Override, nil
	}
	if e.BridgeService == nil {
		return false, nil
	}
	if !e.BridgeService.IsStarted() {
		return false, nil
	}
	// Mock client runs in-process — short-circuit network-level checks but
	// still honor live-client signal so tests can simulate "extension absent".
	if e.IsMock {
		if e.LiveClient != nil && !e.LiveClient() {
			return false, nil
		}
		return true, nil
	}
	// Overload check: if queue is excessively backed up, consider unhealthy
	queueLen := e.BridgeService.QueueLen()
	const maxQueueThreshold = 50
	if queueLen > maxQueueThreshold {
		return false, nil
	}

	if e.LiveClient != nil && !e.LiveClient() {
		return false, nil
	}

	if e.LastActivity != nil && e.RecentActivityCutoff > 0 {
		ts := e.LastActivity()
		if ts > 0 {
			now := time.Now
			if e.now != nil {
				now = e.now
			}
			if now().Unix()-ts > int64(e.RecentActivityCutoff.Seconds()) {
				return false, nil
			}
		}
	}

	return true, nil
}
