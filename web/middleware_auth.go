package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/unimap/project/internal/logger"
)

// contextKey is a typed key for request context values.
type contextKey string

const (
	// contextKeyUserID is the authenticated user's database ID (int64).
	// 0 means legacy single-user mode or admin-token-only auth.
	// -1 means admin-token auth (synthetic admin, not from user DB).
	contextKeyUserID contextKey = "user_id"

	// adminSyntheticUserID is set in context when auth is via X-Admin-Token header.
	// getCurrentUser treats this as a superuser that bypasses role checks.
	adminSyntheticUserID int64 = -1
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
				userID := s.getSessionUserID(r)
				ctx := context.WithValue(r.Context(), contextKeyUserID, userID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Try X-Admin-Token header (API/CLI clients)
			token := r.Header.Get("X-Admin-Token")
			if token == "" {
				token = extractBearerToken(r.Header.Get("Authorization"))
			}
			adminToken := s.adminToken()
			if adminToken != "" && token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) == 1 {
				// Admin token auth: set synthetic userID so user management endpoints work
				ctx := context.WithValue(r.Context(), contextKeyUserID, adminSyntheticUserID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Check node auth tokens (distributed nodes use X-Node-Token)
			if s.isNodeAuthPath(r.URL.Path) && s.authenticateNodeToken(r) {
				ctx := context.WithValue(r.Context(), contextKeyUserID, int64(0))
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Authentication failed
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

// isNodeAuthPath returns true for distributed node endpoints that accept X-Node-Token.
func (s *Server) isNodeAuthPath(path string) bool {
	nodePaths := []string{
		"/api/v1/nodes/register",
		"/api/v1/nodes/heartbeat",
		"/api/v1/nodes/task/claim",
		"/api/v1/nodes/task/result",
		"/api/nodes/register",
		"/api/nodes/heartbeat",
		"/api/nodes/task/claim",
		"/api/nodes/task/result",
	}
	for _, p := range nodePaths {
		if path == p {
			return true
		}
	}
	return false
}

// authenticateNodeToken checks X-Node-Token against configured node auth tokens.
func (s *Server) authenticateNodeToken(r *http.Request) bool {
	if s.config == nil || !s.config.Distributed.Enabled {
		return false
	}
	nodeToken := r.Header.Get("X-Node-Token")
	if nodeToken == "" {
		return false
	}
	for _, configuredToken := range s.config.Distributed.NodeAuthTokens {
		if subtle.ConstantTimeCompare([]byte(nodeToken), []byte(configuredToken)) == 1 {
			return true
		}
	}
	return false
}

// isBrowserRequest checks if the request is from a browser (GET accepting HTML).
func isBrowserRequest(r *http.Request) bool {
	return r.Method == http.MethodGet &&
		strings.Contains(r.Header.Get("Accept"), "text/html")
}

// isScreenshotBridgePath returns true for paths under the screenshot bridge API.
func isScreenshotBridgePath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/screenshot/bridge/")
}

// isPublicPath returns true for paths that do not require authentication.
func (s *Server) isPublicPath(path string) bool {
	publicPrefixes := []string{
		"/static/",
		"/screenshots/",
		"/api/v1/screenshot/bridge/",
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
		"/api/v1/login",
		"/api/v1/logout",
	}
	for _, p := range publicExact {
		if path == p {
			return true
		}
	}
	// Registration is public only when no users exist (bootstrap mode)
	if path == "/api/v1/users/register" {
		return s.isRegistrationPublic()
	}
	return false
}

// isRegistrationPublic returns true if registration should be publicly accessible.
// This is only true when the user DB has zero accounts (bootstrap mode).
func (s *Server) isRegistrationPublic() bool {
	if s.userRepo == nil {
		return false
	}
	count, err := s.userRepo.Count()
	if err != nil {
		logger.Warnf("registration check: failed to count users: %v", err)
		return false
	}
	return count == 0
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
func (s *Server) adminToken() string {
	if s.config == nil || !s.config.Web.Auth.Enabled {
		return ""
	}
	token := s.config.Web.Auth.AdminToken
	if token != "" {
		return token
	}
	s.configMutex.Lock()
	defer s.configMutex.Unlock()
	if s.config.Web.Auth.AdminToken != "" {
		return s.config.Web.Auth.AdminToken
	}
	token = generateRandomToken()
	s.config.Web.Auth.AdminToken = token
	logger.Warnf("Admin token was not configured; auto-generated a random token: %s (save this to config.yaml)", maskTokenForLog(token))
	if s.configManager != nil {
		if err := s.configManager.Save(); err != nil {
			logger.Warnf("failed to persist auto-generated admin token: %v", err)
		}
	}
	return token
}
