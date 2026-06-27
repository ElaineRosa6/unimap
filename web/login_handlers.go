package web

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/logger"
	"golang.org/x/crypto/bcrypt"
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

// handleLoginAPI validates credentials and sets session cookie (POST /api/v1/login).
func (s *Server) handleLoginAPI(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)
	if !loginRateLimiter.Allow(clientIP) {
		logger.Warnf("login rate limited: ip=%s", clientIP)
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many login attempts, please try again later"})
		return
	}
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	csrfToken := r.FormValue("csrf_token")

	if !s.validateLoginCSRF(w, r, csrfToken, clientIP) {
		return
	}

	loginUserID, ok := s.validateLoginCredentials(w, username, password, clientIP)
	if !ok {
		return
	}

	if err := s.setSessionCookieForUser(w, r, loginUserID); err != nil {
		logger.Errorf("login: failed to set session cookie: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	// Rotate CSRF token after successful login to prevent replay attacks
	newCSRF := generateCSRFToken()
	s.setCSRFCookie(w, r, newCSRF)
	logger.Infof("login successful: ip=%s username=%s userID=%d", clientIP, username, loginUserID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "redirect": "/"})
}

// validateLoginCSRF 验证 CSRF token
func (s *Server) validateLoginCSRF(w http.ResponseWriter, r *http.Request, csrfToken, clientIP string) bool {
	expectedCSRF := getCSRFToken(r)
	if expectedCSRF == "" || csrfToken == "" || !secureCompare(csrfToken, expectedCSRF) {
		logger.Warnf("login CSRF validation failed: ip=%s", clientIP)
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid CSRF token"})
		return false
	}
	return true
}

// validateLoginCredentials 验证登录凭据（用户数据库 → 配置文件降级）
// Always performs bcrypt comparison to prevent user enumeration via timing side-channel.
func (s *Server) validateLoginCredentials(w http.ResponseWriter, username, password, clientIP string) (int64, bool) {
	var dbUserFound bool
	var dbUserID int64
	var dbUserHash string
	var dbUserActive bool

	// 用户数据库（多用户模式）
	if s.userRepo != nil {
		user, err := s.userRepo.GetByUsername(username)
		if err != nil {
			logger.Errorf("login: db error: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return 0, false
		}
		if user != nil && user.Status == "active" {
			dbUserFound = true
			dbUserID = user.ID
			dbUserHash = user.PasswordHash
			dbUserActive = true
		}
	}

	// Always perform bcrypt comparison to equalize timing regardless of user existence.
	if dbUserActive {
		if err := bcrypt.CompareHashAndPassword([]byte(dbUserHash), []byte(password)); err == nil {
			return dbUserID, true
		}
	} else {
		// Dummy bcrypt to match timing of a real comparison
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$000000000000000000000000000000000000000000000000000000x"), []byte(password))
	}

	// 配置文件降级（单用户遗留模式）
	if s.config == nil || !dbUserFound {
		// Only check config if no DB user was found
		if !dbUserFound {
			if s.config == nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server configuration error"})
				return 0, false
			}
			expectedUser := s.config.Web.Auth.Username
			expectedHash := s.config.Web.Auth.PasswordHash
			if expectedUser == "" || expectedHash == "" {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "login not configured"})
				return 0, false
			}
			if secureCompare(username, expectedUser) && config.CheckPassword(password, expectedHash) {
				return 0, true
			}
		}
	}

	logger.Warnf("login failed: ip=%s username=%s", clientIP, username)
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid username or password"})
	return 0, false
}

// handleLogoutAPI clears the session cookie (POST /api/v1/logout).
func (s *Server) handleLogoutAPI(w http.ResponseWriter, r *http.Request) {
	s.clearSessionCookie(w, r)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// secureCompare performs constant-time string comparison.
func secureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
