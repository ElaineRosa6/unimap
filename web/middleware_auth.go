package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/unimap/project/internal/logger"
)

// adminAuthMiddleware returns a middleware that requires authentication
// for all requests except public paths. Supports session cookie and X-Admin-Token header.
func (s *Server) adminAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for public paths
			if s.isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Try session cookie first (set by login page)
			if s.getSessionToken(r) != "" {
				next.ServeHTTP(w, r)
				return
			}

			// Try X-Admin-Token header (API/CLI clients)
			token := r.Header.Get("X-Admin-Token")
			if token == "" {
				token = extractBearerToken(r.Header.Get("Authorization"))
			}
			adminToken := s.adminToken()
			if adminToken != "" && token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) == 1 {
				next.ServeHTTP(w, r)
				return
			}

			// Authentication failed
			// Browser page requests → redirect to /login
			// API requests → JSON 401
			if isBrowserRequest(r) {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "unauthorized: valid admin token required",
			})
		})
	}
}

// isBrowserRequest checks if the request is from a browser (GET accepting HTML).
func isBrowserRequest(r *http.Request) bool {
	return r.Method == http.MethodGet &&
		strings.Contains(r.Header.Get("Accept"), "text/html")
}

// isScreenshotBridgePath returns true for paths under the screenshot bridge API.
// Bridge routes have their own auth (loopback + bearer token) and need to bypass
// CORS restrictions for browser extension access.
func isScreenshotBridgePath(path string) bool {
	return strings.HasPrefix(path, "/api/screenshot/bridge/")
}

// isPublicPath returns true for paths that do not require authentication.
func (s *Server) isPublicPath(path string) bool {
	publicPrefixes := []string{
		"/static/",
		"/screenshots/",
		"/api/screenshot/bridge/", // bridge has its own auth (loopback + bearer token)
	}
	for _, prefix := range publicPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	publicExact := []string{
		"/health",
		"/health/ready",
		"/health/live",
		"/login",
		"/api/login",
		"/api/logout",
	}
	for _, p := range publicExact {
		if path == p {
			return true
		}
	}
	return false
}

func generateRandomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		logger.Fatalf("failed to generate cryptographically secure random token: %v", err)
	}
	return hex.EncodeToString(b)
}

func maskTokenForLog(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}

// adminToken returns the configured admin token.
// If auth is enabled but no token is configured, a random token is auto-generated
// on first call and cached for the server lifetime.
func (s *Server) adminToken() string {
	if s.config == nil || !s.config.Web.Auth.Enabled {
		return ""
	}
	token := s.config.Web.Auth.AdminToken
	if token != "" {
		return token
	}
	// Auto-generate a random token if none configured
	s.configMutex.Lock()
	defer s.configMutex.Unlock()
	// Double-check after acquiring lock
	if s.config.Web.Auth.AdminToken != "" {
		return s.config.Web.Auth.AdminToken
	}
	token = generateRandomToken()
	s.config.Web.Auth.AdminToken = token
	logger.Warnf("Admin token was not configured; auto-generated a random token: %s (save this to config.yaml)", maskTokenForLog(token))
	return token
}

