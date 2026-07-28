package scheduler

import (
	"testing"
	"time"
)

func TestSessionHealthTracker_SuccessResetsCircuit(t *testing.T) {
	tracker := NewSessionHealthTracker()
	tracker.FailureThreshold = 2

	tracker.RecordFailure("fofa", FailureNetwork, "timeout")
	tracker.RecordFailure("fofa", FailureNetwork, "timeout")

	h := tracker.GetHealth("fofa")
	if h.Circuit != CircuitOpen {
		t.Fatalf("expected open circuit, got %s", h.Circuit)
	}
	if !tracker.AllowBrowserTask("fofa") == true {
		// AllowBrowserTask should return false when open
	}
	if tracker.AllowBrowserTask("fofa") {
		t.Fatal("browser task should be blocked when circuit is open")
	}

	tracker.RecordSuccess("fofa")
	h = tracker.GetHealth("fofa")
	if h.Circuit != CircuitClosed {
		t.Fatalf("expected closed circuit after success, got %s", h.Circuit)
	}
	if h.ConsecutiveFails != 0 {
		t.Fatalf("expected 0 consecutive fails, got %d", h.ConsecutiveFails)
	}
	if !tracker.AllowBrowserTask("fofa") {
		t.Fatal("browser task should be allowed after recovery")
	}
}

func TestSessionHealthTracker_HalfOpenAfterCooldown(t *testing.T) {
	tracker := NewSessionHealthTracker()
	tracker.FailureThreshold = 1
	tracker.ResetDuration = 10 * time.Millisecond

	tracker.RecordFailure("hunter", FailureCookieExpired, "403")

	if tracker.AllowCheck("hunter") {
		t.Fatal("check should be blocked when circuit is open")
	}

	time.Sleep(15 * time.Millisecond)

	if !tracker.AllowCheck("hunter") {
		t.Fatal("check should be allowed after cooldown (half-open)")
	}
	h := tracker.GetHealth("hunter")
	if h.Circuit != CircuitHalfOpen {
		t.Fatalf("expected half-open, got %s", h.Circuit)
	}
}

func TestSessionHealthTracker_FailureClassification(t *testing.T) {
	tests := []struct {
		reason string
		errMsg string
		want   FailureCategory
	}{
		{"no_session", "", FailureCookieMissing},
		{"login_required", "", FailureLoginWall},
		{"", "connection timeout", FailureNetwork},
		{"", "captcha detected", FailureCaptcha},
		{"", "403 forbidden", FailureCookieExpired},
		{"", "selector not found", FailurePageChanged},
		{"", "something else", FailureUnknown},
		{"browser_session", "", FailureNone},
	}
	for _, tt := range tests {
		got := ClassifyFailureReason(tt.reason, tt.errMsg)
		if got != tt.want {
			t.Errorf("ClassifyFailureReason(%q, %q) = %q, want %q", tt.reason, tt.errMsg, got, tt.want)
		}
	}
}

func TestSessionHealthTracker_Summary(t *testing.T) {
	tracker := NewSessionHealthTracker()
	tracker.RecordSuccess("fofa")
	tracker.RecordFailure("hunter", FailureCookieExpired, "403")

	summary := tracker.Summary()
	if summary == "" || summary == "no engine health data" {
		t.Fatal("expected non-empty summary")
	}
}

func TestRecoveryHint(t *testing.T) {
	hints := []FailureCategory{
		FailureCookieMissing, FailureCookieExpired, FailureLoginWall,
		FailureCaptcha, FailurePageChanged, FailureNetwork, FailureUnknown,
	}
	for _, cat := range hints {
		hint := RecoveryHint(cat)
		if hint == "" {
			t.Errorf("RecoveryHint(%q) returned empty", cat)
		}
	}
}

func TestSessionHealthTracker_UnknownEngineAllowed(t *testing.T) {
	tracker := NewSessionHealthTracker()
	if !tracker.AllowBrowserTask("unknown_engine") {
		t.Fatal("unknown engine should be allowed")
	}
	if !tracker.AllowCheck("unknown_engine") {
		t.Fatal("unknown engine should be allowed")
	}
}
