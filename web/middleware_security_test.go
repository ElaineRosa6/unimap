package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCspNonceFromContext_Empty(t *testing.T) {
	got := cspNonceFromContext(context.Background())
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestCspNonceFromContext_WithValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), cspNonceKey{}, "test-nonce")
	got := cspNonceFromContext(ctx)
	if got != "test-nonce" {
		t.Fatalf("expected test-nonce, got %q", got)
	}
}

func TestCspNonceFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), cspNonceKey{}, 12345)
	got := cspNonceFromContext(ctx)
	if got != "" {
		t.Fatalf("expected empty string for wrong type, got %q", got)
	}
}

func TestSecurityMiddleware_SetsCSPNonce(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := securityMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "nonce-") {
		t.Errorf("expected CSP to contain nonce, got %q", csp)
	}
}
