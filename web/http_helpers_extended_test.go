package web

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetAllowedExtensionIDs(t *testing.T) {
	// Reset state
	extIDMu.Lock()
	extIDSet = nil
	extIDLoaded = false
	extIDMu.Unlock()

	SetAllowedExtensionIDs([]string{"abc123", "def456"})
	extIDMu.RLock()
	loaded := extIDLoaded
	ids := extIDSet
	extIDMu.RUnlock()

	if !loaded {
		t.Fatal("expected extIDLoaded=true")
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d", len(ids))
	}
	if _, ok := ids["abc123"]; !ok {
		t.Fatal("expected abc123 in set")
	}
	if _, ok := ids["def456"]; !ok {
		t.Fatal("expected def456 in set")
	}
}

func TestSetAllowedExtensionIDs_Empty(t *testing.T) {
	SetAllowedExtensionIDs([]string{})
	extIDMu.RLock()
	loaded := extIDLoaded
	ids := extIDSet
	extIDMu.RUnlock()

	if !loaded {
		t.Fatal("expected extIDLoaded=true")
	}
	if len(ids) != 0 {
		t.Fatalf("expected empty set, got %d", len(ids))
	}
}

func TestSetAllowedExtensionIDs_WithWhitespace(t *testing.T) {
	SetAllowedExtensionIDs([]string{"  abc123  ", "", "  "})
	extIDMu.RLock()
	ids := extIDSet
	extIDMu.RUnlock()

	if len(ids) != 1 {
		t.Fatalf("expected 1 ID (empty/whitespace filtered), got %d", len(ids))
	}
	if _, ok := ids["abc123"]; !ok {
		t.Fatal("expected abc123 in set")
	}
}

func TestExtractExtensionID(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"chrome-extension://abc123/", "abc123"},
		{"chrome-extension://abc123", "abc123"},
		{"chrome-extension://abc123/some/path", "abc123"},
		{"https://example.com", ""},
		{"", ""},
		{"chrome-extension://", ""},
	}
	for _, tt := range tests {
		got := extractExtensionID(tt.input)
		if got != tt.want {
			t.Errorf("extractExtensionID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsChromeExtensionAllowed_NotLoaded(t *testing.T) {
	extIDMu.Lock()
	extIDLoaded = false
	extIDMu.Unlock()

	if !isChromeExtensionAllowed("chrome-extension://anyid/") {
		t.Fatal("expected true when not loaded (backward compat)")
	}
}

func TestIsChromeExtensionAllowed_EmptySet(t *testing.T) {
	SetAllowedExtensionIDs([]string{})

	if !isChromeExtensionAllowed("chrome-extension://anyid/") {
		t.Fatal("expected true when empty set (backward compat)")
	}
}

func TestIsChromeExtensionAllowed_Allowed(t *testing.T) {
	SetAllowedExtensionIDs([]string{"abc123"})

	if !isChromeExtensionAllowed("chrome-extension://abc123/") {
		t.Fatal("expected true for allowed ID")
	}
}

func TestIsChromeExtensionAllowed_NotAllowed(t *testing.T) {
	SetAllowedExtensionIDs([]string{"abc123"})

	if isChromeExtensionAllowed("chrome-extension://xyz789/") {
		t.Fatal("expected false for non-allowed ID")
	}
}

func TestIsChromeExtensionAllowed_NonExtensionOrigin(t *testing.T) {
	SetAllowedExtensionIDs([]string{"abc123"})

	if isChromeExtensionAllowed("https://example.com") {
		t.Fatal("expected false for non-extension origin")
	}
}

func TestWriteAPIError_Extended(t *testing.T) {
	rec := httptest.NewRecorder()
	writeAPIError(rec, http.StatusBadRequest, "test_code", "test message", "detail")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "test_code") {
		t.Fatal("expected code in response")
	}
	if !strings.Contains(body, "test message") {
		t.Fatal("expected message in response")
	}
}

func TestRequireMethod_OK(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if !requireMethod(rec, req, http.MethodGet) {
		t.Fatal("expected true for matching method")
	}
}

func TestRequireMethod_Wrong(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	if requireMethod(rec, req, http.MethodGet) {
		t.Fatal("expected false for non-matching method")
	}
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestRequireTrustedRequest_Trusted_Extended(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://localhost:8448")
	if !requireTrustedRequest(rec, req, []string{"http://localhost:8448"}) {
		t.Fatal("expected true for trusted origin")
	}
}

func TestRequireTrustedRequest_Untrusted_Extended(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://evil.com")
	if requireTrustedRequest(rec, req, []string{"http://localhost:8448"}) {
		t.Fatal("expected false for untrusted origin")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestDecodeJSONReader_Valid(t *testing.T) {
	var dst map[string]string
	err := decodeJSONReader(strings.NewReader(`{"key":"value"}`), &dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst["key"] != "value" {
		t.Fatalf("expected key=value, got %v", dst)
	}
}

func TestDecodeJSONReader_UnknownField(t *testing.T) {
	var dst struct{ Name string }
	err := decodeJSONReader(strings.NewReader(`{"Name":"test","Unknown":"val"}`), &dst)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestDecodeJSONReader_TrailingGarbage(t *testing.T) {
	var dst map[string]string
	err := decodeJSONReader(strings.NewReader(`{"key":"value"} extra`), &dst)
	if err == nil {
		t.Fatal("expected error for trailing garbage")
	}
}

func TestIsSameHostURL_Extended(t *testing.T) {
	tests := []struct {
		rawURL, host string
		want         bool
	}{
		{"http://localhost:8448/path", "localhost:8448", true},
		{"http://localhost:8448/path", "localhost:9999", false},
		{"", "localhost:8448", false},
		{"invalid-url", "localhost:8448", false},
	}
	for _, tt := range tests {
		got := isSameHostURL(tt.rawURL, tt.host)
		if got != tt.want {
			t.Errorf("isSameHostURL(%q, %q) = %v, want %v", tt.rawURL, tt.host, got, tt.want)
		}
	}
}

func TestNormalizeOrigin(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"http://localhost:8448", "http://localhost:8448"},
		{"http://localhost:8448/path", "http://localhost:8448"},
		{"https://example.com", "https://example.com"},
		{"", ""},
		{"not-a-url", ""},
		{"://bad", ""},
	}
	for _, tt := range tests {
		got := normalizeOrigin(tt.input)
		if got != tt.want {
			t.Errorf("normalizeOrigin(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsPrivateOrInternalHost(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"[::1]", true},
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"8.8.8.8", false},
		{"example.com", false},
	}
	for _, tt := range tests {
		got := isPrivateOrInternalHost(ctx, tt.host)
		if got != tt.want {
			t.Errorf("isPrivateOrInternalHost(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}

func TestIsBlockedIP(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"8.8.8.8", false},
	}
	for _, tt := range tests {
		parsed := net.ParseIP(tt.ip)
		if parsed == nil {
			if tt.want {
				t.Errorf("isBlockedIP(%q) = false (unparseable), want true", tt.ip)
			}
			continue
		}
		got := isBlockedIP(parsed)
		if got != tt.want {
			t.Errorf("isBlockedIP(%q) = %v, want %v", tt.ip, got, tt.want)
		}
	}
}

func TestIsPrivateOrInternalIP(t *testing.T) {
	tests := []struct {
		ip string
		want bool
	}{
		{"127.0.0.1", true},
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"8.8.8.8", false},
		{"", false},
		// "not-an-ip" triggers DNS lookup which fails => returns true (unsafe)
	}
	for _, tt := range tests {
		got := isPrivateOrInternalIP(tt.ip)
		if got != tt.want {
			t.Errorf("isPrivateOrInternalIP(%q) = %v, want %v", tt.ip, got, tt.want)
		}
	}
}

func TestSanitizeError(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"normal text", "normal text"},
		{"", ""},
		{"error\ngoroutine 1 [running]:\nruntime.main", "error"},
		{
			strings.Repeat("long error ", 100),
			strings.Repeat("long error ", 100)[:500] + "...",
		},
	}
	for _, tt := range tests {
		got := sanitizeError(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeError(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
