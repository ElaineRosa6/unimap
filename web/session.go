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

// sessionRevocationStore tracks revoked session tokens for server-side invalidation.
type sessionRevocationStore struct {
	mu      sync.RWMutex
	revoked map[string]time.Time // token hash -> expiry time
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
	for tokenHash, expiry := range s.revoked {
		if now.After(expiry) {
			delete(s.revoked, tokenHash)
		}
	}
}

func (s *sessionRevocationStore) Revoke(token string, ttl time.Duration) {
	h := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(h[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revoked[tokenHash] = time.Now().Add(ttl)
}

func (s *sessionRevocationStore) IsRevoked(token string) bool {
	h := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(h[:])
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.revoked[tokenHash]
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

// setSessionCookie sets the HttpOnly session cookie with encrypted admin token.
func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request) error {
	encrypted, err := s.encryptToken(s.adminToken())
	if err != nil {
		return fmt.Errorf("encrypt token: %w", err)
	}
	secure := r.TLS != nil
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    encrypted,
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
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return ""
	}
	token, err := s.decryptToken(cookie.Value)
	if err != nil {
		return ""
	}
	if s.revocationStore != nil && s.revocationStore.IsRevoked(token) {
		return ""
	}
	return token
}

// clearSessionCookie expires the session cookie and revokes the token server-side.
func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	token := s.getSessionToken(r)
	if s.revocationStore != nil && token != "" {
		s.revocationStore.Revoke(token, 24*time.Hour)
	}
	secure := r.TLS != nil
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
	secure := r.TLS != nil
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
