package web

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/logger"
)

// loginRateLimiter provides brute force protection for the login endpoint.
// 5 attempts per 15 minutes per IP.
var loginRateLimiter = NewRateLimiter(5, 15*time.Minute)

// handleLoginPage renders the login form (GET /login).
func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	// If already authenticated, redirect to home
	if s.getSessionToken(r) != "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Check X-Admin-Token header too (API clients)
	token := r.Header.Get("X-Admin-Token")
	if token != "" && s.adminToken() != "" {
		if secureCompare(token, s.adminToken()) {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
	}

	// Generate CSRF token and set cookie
	csrfToken := generateCSRFToken()
	s.setCSRFCookie(w, r, csrfToken)

	data := map[string]interface{}{
		"staticVersion": s.staticVersion,
		"CSRFToken":     csrfToken,
		"Error":         "",
	}
	s.renderTemplateWithNonce(r, w, http.StatusOK, "login.html", data)
}

// handleLoginAPI validates credentials and sets session cookie (POST /api/login).
func (s *Server) handleLoginAPI(w http.ResponseWriter, r *http.Request) {
	// Rate limiting
	clientIP := getClientIP(r)
	if !loginRateLimiter.Allow(clientIP) {
		logger.Warnf("login rate limited: ip=%s", clientIP)
		writeJSON(w, http.StatusTooManyRequests, map[string]string{
			"error": "too many login attempts, please try again later",
		})
		return
	}

	// Parse form
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	csrfToken := r.FormValue("csrf_token")

	// Validate CSRF
	expectedCSRF := getCSRFToken(r)
	if expectedCSRF == "" || csrfToken == "" || !secureCompare(csrfToken, expectedCSRF) {
		logger.Warnf("login CSRF validation failed: ip=%s", clientIP)
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "invalid CSRF token",
		})
		return
	}

	// Validate credentials
	if s.config == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "server configuration error",
		})
		return
	}

	expectedUser := s.config.Web.Auth.Username
	expectedHash := s.config.Web.Auth.PasswordHash

	if expectedUser == "" || expectedHash == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "login not configured",
		})
		return
	}

	if !secureCompare(username, expectedUser) || !config.CheckPassword(password, expectedHash) {
		logger.Warnf("login failed: ip=%s username=%s", clientIP, username)
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "invalid username or password",
		})
		return
	}

	// Set session cookie
	if err := s.setSessionCookie(w, r); err != nil {
		logger.Errorf("login: failed to set session cookie: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "internal server error",
		})
		return
	}

	logger.Infof("login successful: ip=%s username=%s", clientIP, username)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"redirect": "/",
	})
}

// handleLogoutAPI clears the session cookie (POST /api/logout).
func (s *Server) handleLogoutAPI(w http.ResponseWriter, r *http.Request) {
	s.clearSessionCookie(w, r)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// secureCompare performs constant-time string comparison.
func secureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
