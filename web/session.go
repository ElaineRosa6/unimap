package web

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookieName = "unimap_session"
	csrfCookieName    = "unimap_csrf"
	sessionPepper     = "unimap-session-pepper-v1"
	sessionMaxAge     = 86400 // 24 hours
	csrfMaxAge        = 3600  // 1 hour
)

// sessionRevocationStore tracks revoked session IDs for server-side invalidation.
type sessionRevocationStore struct {
	mu      sync.RWMutex
	revoked map[string]time.Time // session ID -> expiry time
	stopCh  chan struct{}
}

func newSessionRevocationStore() *sessionRevocationStore {
	s := &sessionRevocationStore{
		revoked: make(map[string]time.Time),
		stopCh:  make(chan struct{}),
	}
	go s.cleanupLoop()
	return s
}

func (s *sessionRevocationStore) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.stopCh:
			return
		}
	}
}

func (s *sessionRevocationStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, expiry := range s.revoked {
		if now.After(expiry) {
			delete(s.revoked, id)
		}
	}
}

// Revoke adds a session ID to the revocation list with a TTL.
func (s *sessionRevocationStore) Revoke(sessionID string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revoked[sessionID] = time.Now().Add(ttl)
}

// IsRevoked checks whether a session ID has been revoked.
func (s *sessionRevocationStore) IsRevoked(sessionID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.revoked[sessionID]
	return ok
}

func (s *sessionRevocationStore) Stop() {
	close(s.stopCh)
}

// deriveSessionKey derives a 32-byte AES key from adminToken + pepper.
func (s *Server) deriveSessionKey() []byte {
	h := sha256.Sum256([]byte(s.adminToken() + sessionPepper))
	return h[:]
}

// encryptToken encrypts the admin token with AES-GCM, returns base64 string.
func (s *Server) encryptToken(token string) (string, error) {
	key := s.deriveSessionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(token), nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// decryptToken decrypts the cookie value back to the admin token.
func (s *Server) decryptToken(encrypted string) (string, error) {
	key := s.deriveSessionKey()
	ciphertext, err := base64.URLEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("gcm open: %w", err)
	}
	return string(plaintext), nil
}

// isSecure returns true if the request arrived over TLS directly or via a
// reverse proxy that sets the X-Forwarded-Proto header to "https".
func isSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// setSessionCookie creates a new session with a random session ID and encrypted admin token.
// Cookie format: "sessionID:encryptedToken"
func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request) error {
	return s.setSessionCookieForUser(w, r, 0)
}

// setSessionCookieForUser creates a session for a specific user.
// Cookie format: "sessionID:encryptedPayload" where payload = "userID:adminToken"
// userID=0 means legacy single-user mode (no user DB).
func (s *Server) setSessionCookieForUser(w http.ResponseWriter, r *http.Request, userID int64) error {
	sessionID := generateSessionID()
	payload := fmt.Sprintf("%d:%s", userID, s.adminToken())
	encrypted, err := s.encryptToken(payload)
	if err != nil {
		return fmt.Errorf("encrypt token: %w", err)
	}
	cookieValue := sessionID + ":" + encrypted
	secure := isSecure(r)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    cookieValue,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   sessionMaxAge,
	})
	return nil
}

// getSessionToken extracts and decrypts the session cookie. Returns "" if invalid or revoked.
func (s *Server) getSessionToken(r *http.Request) string {
	token, _ := s.getSessionInfo(r)
	return token
}

// getSessionUserID extracts the user ID from the session cookie. Returns 0 if unavailable.
func (s *Server) getSessionUserID(r *http.Request) int64 {
	_, userID := s.getSessionInfo(r)
	return userID
}

// getSessionInfo extracts both admin token and user ID from the session cookie.
// Handles both new format ("userID:adminToken") and legacy format ("adminToken").
func (s *Server) getSessionInfo(r *http.Request) (string, int64) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return "", 0
	}
	parts := strings.SplitN(cookie.Value, ":", 2)
	if len(parts) != 2 {
		return "", 0
	}
	sessionID := parts[0]
	if s.revocationStore != nil && s.revocationStore.IsRevoked(sessionID) {
		return "", 0
	}
	decrypted, err := s.decryptToken(parts[1])
	if err != nil {
		return "", 0
	}
	// New format: "userID:adminToken"
	if idx := strings.Index(decrypted, ":"); idx > 0 {
		userIDStr := decrypted[:idx]
		token := decrypted[idx+1:]
		var userID int64
		for _, c := range userIDStr {
			if c >= '0' && c <= '9' {
				userID = userID*10 + int64(c-'0')
			} else {
				// Not a number, treat as legacy format
				return decrypted, 0
			}
		}
		return token, userID
	}
	// Legacy format: just the admin token
	return decrypted, 0
}

// getSessionID extracts the session ID from the cookie. Returns "" if missing.
func (s *Server) getSessionID(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return ""
	}
	parts := strings.SplitN(cookie.Value, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[0]
}

// clearSessionCookie expires the session cookie and revokes the session ID server-side.
func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	sessionID := s.getSessionID(r)
	if s.revocationStore != nil && sessionID != "" {
		s.revocationStore.Revoke(sessionID, 24*time.Hour)
	}
	secure := isSecure(r)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// generateCSRFToken generates a random 32-byte hex CSRF token.
func generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// setCSRFCookie sets the CSRF cookie (readable by JS).
func (s *Server) setCSRFCookie(w http.ResponseWriter, r *http.Request, token string) {
	secure := isSecure(r)
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false, // JS needs to read this
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   csrfMaxAge,
	})
}

// getCSRFToken reads the CSRF cookie.
func getCSRFToken(r *http.Request) string {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		return ""
	}
	return cookie.Value
}
