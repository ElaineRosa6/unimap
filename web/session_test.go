package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/unimap/project/internal/config"
)

func TestDeriveSessionKey(t *testing.T) {
	s := &Server{}
	s.config = &config.Config{}
	s.config.Web.Auth.AdminToken = "test-token-123"
	key := s.deriveSessionKey()
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}
}

func TestEncryptDecryptToken_RoundTrip(t *testing.T) {
	s := &Server{}
	s.config = &config.Config{}
	s.config.Web.Auth.AdminToken = "secret-admin-token"
	original := "test-payload"
	encrypted, err := s.encryptToken(original)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	decrypted, err := s.decryptToken(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != original {
		t.Fatalf("round trip failed: got %q, want %q", decrypted, original)
	}
}

func TestDecryptToken_InvalidBase64(t *testing.T) {
	s := &Server{}
	s.config = &config.Config{}
	s.config.Web.Auth.AdminToken = "test-token"
	_, err := s.decryptToken("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecryptToken_TooShort(t *testing.T) {
	s := &Server{}
	s.config = &config.Config{}
	s.config.Web.Auth.AdminToken = "test-token"
	// Valid base64 but too short for AES-GCM
	short := "AAAA"
	_, err := s.decryptToken(short)
	if err == nil {
		t.Fatal("expected error for too-short ciphertext")
	}
}

func TestIsSecure_TLS(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No TLS set
	if isSecure(req) {
		t.Fatal("expected false without TLS")
	}
}

func TestIsSecure_XForwardedProto(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	if !isSecure(req) {
		t.Fatal("expected true with X-Forwarded-Proto: https")
	}
}

func TestIsSecure_XForwardedProtoHTTP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	if isSecure(req) {
		t.Fatal("expected false with X-Forwarded-Proto: http")
	}
}

func TestSetSessionCookie_SetsCookie(t *testing.T) {
	s := &Server{}
	s.config = &config.Config{}
	s.config.Web.Auth.AdminToken = "test-admin-token"
	s.staticVersion = "1.0"
	s.revocationStore = newSessionRevocationStore()
	defer s.revocationStore.Stop()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	err := s.setSessionCookie(w, r)
	if err != nil {
		t.Fatalf("setSessionCookie: %v", err)
	}
	cookies := w.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			found = true
			if c.Value == "" {
				t.Fatal("session cookie value is empty")
			}
			if !strings.Contains(c.Value, ":") {
				t.Fatal("session cookie should contain colon separator")
			}
			if c.MaxAge != sessionMaxAge {
				t.Fatalf("expected MaxAge=%d, got %d", sessionMaxAge, c.MaxAge)
			}
			if !c.HttpOnly {
				t.Fatal("session cookie should be HttpOnly")
			}
			break
		}
	}
	if !found {
		t.Fatal("session cookie not set")
	}
}

func TestSetSessionCookieForUser_SetsUserID(t *testing.T) {
	s := &Server{}
	s.config = &config.Config{}
	s.config.Web.Auth.AdminToken = "test-admin-token"
	s.staticVersion = "1.0"
	s.revocationStore = newSessionRevocationStore()
	defer s.revocationStore.Stop()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	err := s.setSessionCookieForUser(w, r, 42)
	if err != nil {
		t.Fatalf("setSessionCookieForUser: %v", err)
	}
	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			// Decrypt and verify userID
			parts := strings.SplitN(c.Value, ":", 2)
			if len(parts) != 2 {
				t.Fatal("invalid cookie format")
			}
			decrypted, err := s.decryptToken(parts[1])
			if err != nil {
				t.Fatalf("decrypt: %v", err)
			}
			if !strings.HasPrefix(decrypted, "42:") {
				t.Fatalf("expected userID=42 prefix, got %q", decrypted)
			}
			return
		}
	}
	t.Fatal("session cookie not found")
}

func TestGetSessionToken_NoCookie(t *testing.T) {
	s := &Server{}
	s.config = &config.Config{}
	s.config.Web.Auth.AdminToken = "test-token"
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	token := s.getSessionToken(r)
	if token != "" {
		t.Fatalf("expected empty token, got %q", token)
	}
}

func TestGetSessionToken_InvalidFormat(t *testing.T) {
	s := &Server{}
	s.config = &config.Config{}
	s.config.Web.Auth.AdminToken = "test-token"
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "no-colon"})
	token := s.getSessionToken(r)
	if token != "" {
		t.Fatalf("expected empty token for invalid format, got %q", token)
	}
}

func TestGetSessionUserID_NoCookie(t *testing.T) {
	s := &Server{}
	s.config = &config.Config{}
	s.config.Web.Auth.AdminToken = "test-token"
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	userID := s.getSessionUserID(r)
	if userID != 0 {
		t.Fatalf("expected 0, got %d", userID)
	}
}

func TestGetSessionID_NoCookie(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	sessionID := s.getSessionID(r)
	if sessionID != "" {
		t.Fatalf("expected empty, got %q", sessionID)
	}
}

func TestGetSessionID_InvalidFormat(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "no-colon"})
	sessionID := s.getSessionID(r)
	if sessionID != "" {
		t.Fatalf("expected empty for invalid format, got %q", sessionID)
	}
}

func TestClearSessionCookie_ClearsCookie(t *testing.T) {
	s := &Server{}
	s.config = &config.Config{}
	s.config.Web.Auth.AdminToken = "test-token"
	s.revocationStore = newSessionRevocationStore()
	defer s.revocationStore.Stop()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	s.clearSessionCookie(w, r)
	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			if c.Value != "" {
				t.Fatal("expected empty cookie value")
			}
			if c.MaxAge != -1 {
				t.Fatalf("expected MaxAge=-1, got %d", c.MaxAge)
			}
			return
		}
	}
	t.Fatal("session cookie not found")
}

func TestGenerateSessionID_Length(t *testing.T) {
	id := generateSessionID()
	// 16 bytes = 32 hex chars
	if len(id) != 32 {
		t.Fatalf("expected 32 chars, got %d", len(id))
	}
}

func TestGenerateSessionID_Unique(t *testing.T) {
	id1 := generateSessionID()
	id2 := generateSessionID()
	if id1 == id2 {
		t.Fatal("session IDs should be unique")
	}
}

func TestGenerateCSRFToken_Length(t *testing.T) {
	token := generateCSRFToken()
	// 32 bytes = 64 hex chars
	if len(token) != 64 {
		t.Fatalf("expected 64 chars, got %d", len(token))
	}
}

func TestGenerateCSRFToken_Unique(t *testing.T) {
	t1 := generateCSRFToken()
	t2 := generateCSRFToken()
	if t1 == t2 {
		t.Fatal("CSRF tokens should be unique")
	}
}

func TestSetCSRFCookie_SetsCookie(t *testing.T) {
	s := &Server{}
	s.config = &config.Config{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	s.setCSRFCookie(w, r, "test-csrf-token")
	cookies := w.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == csrfCookieName {
			found = true
			if c.Value != "test-csrf-token" {
				t.Fatalf("expected 'test-csrf-token', got %q", c.Value)
			}
			if c.HttpOnly {
				t.Fatal("CSRF cookie should NOT be HttpOnly (JS needs to read it)")
			}
			if c.MaxAge != csrfMaxAge {
				t.Fatalf("expected MaxAge=%d, got %d", csrfMaxAge, c.MaxAge)
			}
			break
		}
	}
	if !found {
		t.Fatal("CSRF cookie not set")
	}
}

func TestGetCSRFToken_NoCookie(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	token := getCSRFToken(r)
	if token != "" {
		t.Fatalf("expected empty, got %q", token)
	}
}

func TestGetCSRFToken_WithCookie(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "my-csrf"})
	token := getCSRFToken(r)
	if token != "my-csrf" {
		t.Fatalf("expected 'my-csrf', got %q", token)
	}
}

func TestNewSessionRevocationStore(t *testing.T) {
	store := newSessionRevocationStore()
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if store.revoked == nil {
		t.Fatal("expected non-nil revoked map")
	}
	store.Stop()
}

func TestSessionRevocationStore_RevokeAndIsRevoked(t *testing.T) {
	store := newSessionRevocationStore()
	defer store.Stop()
	store.Revoke("session-123", 1*time.Hour)
	if !store.IsRevoked("session-123") {
		t.Fatal("expected session to be revoked")
	}
	if store.IsRevoked("session-456") {
		t.Fatal("unexpected session revoked")
	}
}

func TestSessionRevocationStore_Cleanup(t *testing.T) {
	store := newSessionRevocationStore()
	defer store.Stop()
	// Revoke with very short TTL
	store.Revoke("short-lived", 1*time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	store.cleanup()
	if store.IsRevoked("short-lived") {
		t.Fatal("expected expired session to be cleaned up")
	}
}

func TestSessionRevocationStore_StopIdempotent(t *testing.T) {
	// Stop() uses sync.Once to safely close the stopCh.
	// Calling Stop multiple times should not panic.
	store := newSessionRevocationStore()
	store.Stop()
	store.Stop() // second call should be a no-op
}

func TestSetSessionCookieForUser_SetsRevocationStore(t *testing.T) {
	s := &Server{}
	s.config = &config.Config{}
	s.config.Web.Auth.AdminToken = "test-token"
	s.staticVersion = "1.0"
	s.revocationStore = newSessionRevocationStore()
	defer s.revocationStore.Stop()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	err := s.setSessionCookieForUser(w, r, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetSessionInfo_LegacyFormat(t *testing.T) {
	s := &Server{}
	s.config = &config.Config{}
	s.config.Web.Auth.AdminToken = "test-admin-token"
	s.revocationStore = newSessionRevocationStore()
	defer s.revocationStore.Stop()
	// Create a cookie with legacy format (no userID prefix)
	sessionID := generateSessionID()
	payload := "legacy-token"
	encrypted, err := s.encryptToken(payload)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	cookieValue := sessionID + ":" + encrypted
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookieValue})
	token, userID := s.getSessionInfo(r)
	if token != payload {
		t.Fatalf("expected token=%q, got %q", payload, token)
	}
	if userID != 0 {
		t.Fatalf("expected userID=0 for legacy, got %d", userID)
	}
}
