package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/unimap/project/internal/config"
)

func TestIsBrowserRequest(t *testing.T) {
	tests := []struct {
		method, accept string
		want           bool
	}{
		{http.MethodGet, "text/html,application/xhtml+xml", true},
		{http.MethodGet, "application/json", false},
		{http.MethodPost, "text/html", false},
		{http.MethodGet, "", false},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(tt.method, "/", nil)
		r.Header.Set("Accept", tt.accept)
		got := isBrowserRequest(r)
		if got != tt.want {
			t.Errorf("isBrowserRequest(%s, %s) = %v, want %v", tt.method, tt.accept, got, tt.want)
		}
	}
}

func TestIsScreenshotBridgePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/api/v1/screenshot/bridge/health", true},
		{"/api/v1/screenshot/bridge/status", true},
		{"/api/v1/screenshot/bridge/pair", true},
		{"/api/v1/query", false},
		{"/api/v1/screenshot", false},
		{"/static/js/main.js", false},
	}
	for _, tt := range tests {
		got := isScreenshotBridgePath(tt.path)
		if got != tt.want {
			t.Errorf("isScreenshotBridgePath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestAuthenticateNodeToken_Disabled(t *testing.T) {
	s := &Server{config: &config.Config{}}
	s.config.Distributed.Enabled = false
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Node-Token", "test-token")
	if s.authenticateNodeToken(r) {
		t.Fatal("expected false when distributed disabled")
	}
}

func TestAuthenticateNodeToken_NilConfig(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Node-Token", "test-token")
	if s.authenticateNodeToken(r) {
		t.Fatal("expected false when config nil")
	}
}

func TestAuthenticateNodeToken_NoToken(t *testing.T) {
	s := &Server{config: &config.Config{}}
	s.config.Distributed.Enabled = true
	s.config.Distributed.NodeAuthTokens = map[string]string{"token-value": "node1"}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if s.authenticateNodeToken(r) {
		t.Fatal("expected false when no token header")
	}
}

func TestAuthenticateNodeToken_WrongToken(t *testing.T) {
	s := &Server{config: &config.Config{}}
	s.config.Distributed.Enabled = true
	s.config.Distributed.NodeAuthTokens = map[string]string{"token-value": "node1"}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Node-Token", "wrong-token")
	if s.authenticateNodeToken(r) {
		t.Fatal("expected false for wrong token")
	}
}

func TestAuthenticateNodeToken_CorrectToken(t *testing.T) {
	s := &Server{config: &config.Config{}}
	s.config.Distributed.Enabled = true
	// Code iterates values: for _, configuredToken := range NodeAuthTokens
	// So the value "node1" is what gets compared
	s.config.Distributed.NodeAuthTokens = map[string]string{"key": "node1"}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Node-Token", "node1")
	if !s.authenticateNodeToken(r) {
		t.Fatal("expected true for correct token")
	}
}

func TestGenerateRandomToken(t *testing.T) {
	token := generateRandomToken()
	if len(token) != 64 { // 32 bytes = 64 hex chars
		t.Fatalf("expected 64 hex chars, got %d", len(token))
	}
}

func TestGenerateRandomToken_Unique(t *testing.T) {
	t1 := generateRandomToken()
	t2 := generateRandomToken()
	if t1 == t2 {
		t.Fatal("tokens should be unique")
	}
}

func TestMaskTokenForLog_Short(t *testing.T) {
	got := maskTokenForLog("abc")
	if got != "****" {
		t.Fatalf("expected '****' for short token, got %q", got)
	}
}

func TestMaskTokenForLog_Long(t *testing.T) {
	got := maskTokenForLog("abcdefghijklmnop")
	if got != "abcd****mnop" {
		t.Fatalf("expected 'abcd****mnop', got %q", got)
	}
}

func TestMaskTokenForLog_Exact8(t *testing.T) {
	got := maskTokenForLog("12345678")
	// len <= 8 returns "****"
	if got != "****" {
		t.Fatalf("expected '****' for 8-char token, got %q", got)
	}
}

func TestIsRegistrationPublic_NilRepo(t *testing.T) {
	s := &Server{}
	if s.isRegistrationPublic() {
		t.Fatal("expected false when userRepo nil")
	}
}

func TestIsRegistrationPublic_EmptyDB(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	if !s.isRegistrationPublic() {
		t.Fatal("expected true when no users")
	}
}

func TestIsRegistrationPublic_HasUsers(t *testing.T) {
	repo := newMockUserRepo()
	repo.Create("admin", "hash", "admin")
	s := &Server{userRepo: repo}
	if s.isRegistrationPublic() {
		t.Fatal("expected false when users exist")
	}
}

func TestCSPNonceFromContext_Missing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	nonce := cspNonceFromContext(r.Context())
	if nonce != "" {
		t.Fatalf("expected empty nonce, got %q", nonce)
	}
}

func TestGenerateCSPNonce_Length(t *testing.T) {
	nonce := generateCSPNonce()
	// 16 bytes = 24 base64 chars (with padding) or 22 without padding
	if len(nonce) < 20 || len(nonce) > 32 {
		t.Fatalf("unexpected nonce length: %d (%q)", len(nonce), nonce)
	}
}

func TestGenerateCSPNonce_Unique(t *testing.T) {
	n1 := generateCSPNonce()
	n2 := generateCSPNonce()
	if n1 == n2 {
		t.Fatal("nonces should be unique")
	}
}

func TestOriginAllowedByList(t *testing.T) {
	tests := []struct {
		origin string
		list   []string
		want   bool
	}{
		{"http://localhost:8448", []string{"http://localhost:8448"}, true},
		{"http://localhost:8448", []string{"http://example.com"}, false},
		{"http://localhost:8448", []string{"*"}, true},
		{"", []string{"http://localhost:8448"}, false},
		{"http://localhost:8448", nil, false},
	}
	for _, tt := range tests {
		got := originAllowedByList(tt.origin, tt.list)
		if got != tt.want {
			t.Errorf("originAllowedByList(%q, %v) = %v, want %v", tt.origin, tt.list, got, tt.want)
		}
	}
}

func TestIsOriginAllowed(t *testing.T) {
	tests := []struct {
		origin, host string
		list         []string
		want         bool
	}{
		{"http://localhost:8448", "localhost:8448", []string{"http://localhost:8448"}, true},
		{"http://evil.com", "localhost:8448", []string{"http://localhost:8448"}, false},
		{"", "localhost:8448", []string{"http://localhost:8448"}, false},
	}
	for _, tt := range tests {
		got := isOriginAllowed(tt.origin, tt.host, tt.list)
		if got != tt.want {
			t.Errorf("isOriginAllowed(%q, %q, %v) = %v, want %v", tt.origin, tt.host, tt.list, got, tt.want)
		}
	}
}
