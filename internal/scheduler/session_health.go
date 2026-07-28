package scheduler

import (
	"fmt"
	"sync"
	"time"
)

// FailureCategory classifies why a login/session check failed.
type FailureCategory string

const (
	FailureNone          FailureCategory = "none"           // No failure
	FailureCookieMissing FailureCategory = "cookie_missing" // No cookies configured
	FailureCookieExpired FailureCategory = "cookie_expired" // Cookies present but rejected
	FailureLoginWall     FailureCategory = "login_wall"     // Login page detected
	FailureCaptcha       FailureCategory = "captcha"        // CAPTCHA challenge detected
	FailurePageChanged   FailureCategory = "page_changed"   // Page structure changed (selector miss)
	FailureNetwork       FailureCategory = "network"        // Network/timeout error
	FailureUnknown       FailureCategory = "unknown"        // Unclassified failure
)

// ClassifyFailureReason maps a raw reason string from EngineLoginStatus to a FailureCategory.
func ClassifyFailureReason(reason, errMsg string) FailureCategory {
	switch reason {
	case "no_session":
		return FailureCookieMissing
	case "login_required":
		return FailureLoginWall
	case "browser_session", "cookie_configured":
		return FailureNone // These indicate success paths
	case "unsupported_engine":
		return FailureUnknown
	}
	// Classify by error message keywords
	if errMsg != "" {
		lower := toLower(errMsg)
		if contains(lower, "captcha") || contains(lower, "verify") || contains(lower, "验证码") {
			return FailureCaptcha
		}
		if contains(lower, "timeout") || contains(lower, "connection") || contains(lower, "dial") || contains(lower, "network") {
			return FailureNetwork
		}
		if contains(lower, "selector") || contains(lower, "not found") || contains(lower, "element") {
			return FailurePageChanged
		}
		if contains(lower, "401") || contains(lower, "403") || contains(lower, "unauthorized") || contains(lower, "forbidden") {
			return FailureCookieExpired
		}
	}
	if reason == "" {
		return FailureUnknown
	}
	return FailureUnknown
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// CircuitState represents the circuit breaker state for an engine.
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"    // Healthy, checks proceed normally
	CircuitOpen     CircuitState = "open"      // Tripped, browser tasks skipped
	CircuitHalfOpen CircuitState = "half_open" // Cooldown elapsed, one probe allowed
)

// EngineHealth tracks the session health of a single engine.
type EngineHealth struct {
	Engine            string          `json:"engine"`
	Circuit           CircuitState    `json:"circuit"`
	ConsecutiveFails  int             `json:"consecutive_fails"`
	LastFailure       FailureCategory `json:"last_failure"`
	LastFailureReason string          `json:"last_failure_reason"`
	LastFailTime      time.Time       `json:"last_fail_time"`
	LastSuccessTime   time.Time       `json:"last_success_time"`
	TotalChecks       int             `json:"total_checks"`
	TotalFailures     int             `json:"total_failures"`
}

// SessionHealthTracker tracks per-engine session health with circuit breaker logic.
type SessionHealthTracker struct {
	mu sync.RWMutex

	engines map[string]*EngineHealth

	// Circuit breaker configuration
	FailureThreshold int           // consecutive failures to trip (default 3)
	ResetDuration    time.Duration // cooldown before half-open (default 30min)
}

// NewSessionHealthTracker creates a tracker with default thresholds.
func NewSessionHealthTracker() *SessionHealthTracker {
	return &SessionHealthTracker{
		engines:          make(map[string]*EngineHealth),
		FailureThreshold: 3,
		ResetDuration:    30 * time.Minute,
	}
}

// RecordSuccess records a successful login check for an engine.
func (t *SessionHealthTracker) RecordSuccess(engine string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	h := t.getOrCreate(engine)
	h.ConsecutiveFails = 0
	h.Circuit = CircuitClosed
	h.LastFailure = FailureNone
	h.LastFailureReason = ""
	h.LastSuccessTime = time.Now()
	h.TotalChecks++
}

// RecordFailure records a failed login check for an engine.
func (t *SessionHealthTracker) RecordFailure(engine string, category FailureCategory, reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	h := t.getOrCreate(engine)
	h.ConsecutiveFails++
	h.LastFailure = category
	h.LastFailureReason = reason
	h.LastFailTime = time.Now()
	h.TotalChecks++
	h.TotalFailures++

	if h.ConsecutiveFails >= t.FailureThreshold {
		h.Circuit = CircuitOpen
	}
}

// AllowCheck returns whether a login check should proceed for the given engine.
// When the circuit is open, checks are blocked until ResetDuration elapses (half-open).
func (t *SessionHealthTracker) AllowCheck(engine string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	h := t.getOrCreate(engine)
	switch h.Circuit {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(h.LastFailTime) > t.ResetDuration {
			h.Circuit = CircuitHalfOpen
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	}
	return true
}

// AllowBrowserTask returns whether browser tasks (CDP/Bridge) should proceed for the engine.
// This is stricter than AllowCheck: when the circuit is open, browser tasks are skipped
// but login checks are still allowed (to detect recovery).
func (t *SessionHealthTracker) AllowBrowserTask(engine string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	h, ok := t.engines[engine]
	if !ok {
		return true
	}
	return h.Circuit != CircuitOpen
}

// GetHealth returns a snapshot of an engine's health.
func (t *SessionHealthTracker) GetHealth(engine string) *EngineHealth {
	t.mu.RLock()
	defer t.mu.RUnlock()

	h, ok := t.engines[engine]
	if !ok {
		return &EngineHealth{Engine: engine, Circuit: CircuitClosed}
	}
	cp := *h
	return &cp
}

// Summary returns a human-readable health summary for all tracked engines.
func (t *SessionHealthTracker) Summary() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.engines) == 0 {
		return "no engine health data"
	}

	var b []byte
	for engine, h := range t.engines {
		status := "✅"
		if h.Circuit == CircuitOpen {
			status = "🔴"
		} else if h.Circuit == CircuitHalfOpen {
			status = "🟡"
		} else if h.ConsecutiveFails > 0 {
			status = "⚠️"
		}

		line := fmt.Sprintf("%s %s: circuit=%s", status, engine, h.Circuit)
		if h.LastSuccessTime.IsZero() {
			line += ", never succeeded"
		} else {
			line += fmt.Sprintf(", last_ok=%s", h.LastSuccessTime.Format("01-02 15:04"))
		}
		if h.ConsecutiveFails > 0 {
			line += fmt.Sprintf(", fails=%d (%s)", h.ConsecutiveFails, h.LastFailure)
		}
		b = append(b, []byte(line+"\n")...)
	}
	return string(b)
}

// RecoveryHint returns actionable recovery guidance based on the failure category.
func RecoveryHint(category FailureCategory) string {
	switch category {
	case FailureCookieMissing:
		return "请在设置页配置引擎 Cookie，或通过 Bridge 扩展自动获取"
	case FailureCookieExpired:
		return "Cookie 已过期，请重新登录引擎网站并更新 Cookie"
	case FailureLoginWall:
		return "检测到登录墙，请检查 Cookie 是否有效或手动登录后重试"
	case FailureCaptcha:
		return "检测到验证码，请手动完成验证后重试"
	case FailurePageChanged:
		return "引擎页面结构可能已改版，请检查选择器是否需要更新"
	case FailureNetwork:
		return "网络连接失败，请检查代理配置和网络连通性"
	default:
		return "请检查引擎配置和日志获取详细信息"
	}
}

func (t *SessionHealthTracker) getOrCreate(engine string) *EngineHealth {
	h, ok := t.engines[engine]
	if !ok {
		h = &EngineHealth{Engine: engine, Circuit: CircuitClosed}
		t.engines[engine] = h
	}
	return h
}
