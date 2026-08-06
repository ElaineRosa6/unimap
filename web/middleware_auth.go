package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/unimap/project/internal/auth"
	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/logger"
)

// contextKey is a typed key for request context values.
type contextKey string

const (
	// contextKeyUserID is the authenticated user's database ID (int64).
	// 0 means legacy single-user mode or admin-token-only auth.
	// -1 means admin-token auth (synthetic admin, not from user DB).
	contextKeyUserID contextKey = "user_id"
	contextKeyUser   contextKey = "authenticated_user"

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
			if sessionToken, userID, sessionVersion := s.getSessionIdentity(r); sessionToken != "" {
				var authenticatedUser *auth.User
				if userID > 0 && s.userRepo != nil {
					user, err := s.userRepo.GetByID(userID)
					if err != nil {
						writeAPIError(w, http.StatusServiceUnavailable, "user_store_unavailable", "user database unavailable", nil)
						return
					}
					if user == nil || user.Status != "active" || user.SessionVersion != sessionVersion {
						s.clearSessionCookie(w, r)
						writeAPIError(w, http.StatusUnauthorized, "session_invalid", "session is no longer valid", nil)
						return
					}
					authenticatedUser = user
				}
				ctx := context.WithValue(r.Context(), contextKeyUserID, userID)
				if authenticatedUser != nil {
					ctx = context.WithValue(ctx, contextKeyUser, authenticatedUser)
				}
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
	cfg := s.currentConfig()
	if cfg == nil || !cfg.Distributed.Enabled {
		return false
	}
	nodeToken := r.Header.Get("X-Node-Token")
	if nodeToken == "" {
		return false
	}
	for _, configuredToken := range cfg.Distributed.NodeAuthTokens {
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
	s.configMutex.Lock()
	ephemeral := s.ephemeralAdminToken
	s.configMutex.Unlock()
	if ephemeral != "" {
		return ephemeral
	}
	cfg := s.currentConfig()
	if cfg == nil || !cfg.Web.Auth.Enabled {
		return ""
	}
	token := cfg.Web.Auth.AdminToken
	if token != "" {
		return token
	}
	token = generateRandomToken()
	// FINDING-006: do not log any token fragment (even masked) — just notify
	// the operator to check config.yaml for the persisted value.
	logger.Warnf("Admin token was not configured; auto-generated a random token and saved to config.yaml. See web.auth.admin_token.")
	committed, err := s.updateConfig(func(candidate *config.Config) error {
		if candidate.Web.Auth.AdminToken == "" {
			candidate.Web.Auth.AdminToken = token
		}
		return nil
	})
	if err != nil {
		logger.Warnf("failed to persist auto-generated admin token: %v", err)
		s.configMutex.Lock()
		if s.ephemeralAdminToken == "" {
			s.ephemeralAdminToken = token
		}
		token = s.ephemeralAdminToken
		s.configMutex.Unlock()
		return token
	}
	return committed.Web.Auth.AdminToken
}
