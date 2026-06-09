package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/unimap/project/internal/auth"
	"github.com/unimap/project/internal/logger"
	"golang.org/x/crypto/bcrypt"
)

// userRepoGuard checks if userRepo is available; returns false and writes 503 if not.
func (s *Server) userRepoGuard(w http.ResponseWriter) bool {
	if s.userRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "user database unavailable")
		return false
	}
	return true
}

// handleRegister handles user registration (POST /api/v1/users/register).
// Public only when no users exist (bootstrap mode). After that, requires authentication.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.userRepoGuard(w) {
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	if len(req.Username) < 3 || len(req.Username) > 32 {
		writeError(w, http.StatusBadRequest, "username must be 3-32 characters")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	// If users already exist, require authentication (admin or logged-in user)
	count, _ := s.userRepo.Count()
	if count > 0 {
		currentUser := s.getCurrentUser(r)
		if currentUser == nil {
			writeError(w, http.StatusUnauthorized, "authentication required to register new users")
			return
		}
		if currentUser.Role != "admin" {
			writeError(w, http.StatusForbidden, "only admin can register new users")
			return
		}
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Errorf("register: failed to hash password: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// First user gets admin role, others get readonly
	role := "readonly"
	if count == 0 {
		role = "admin"
	}

	user, err := s.userRepo.Create(req.Username, string(hash), role)
	if err != nil {
		// Handle UNIQUE constraint violation (race condition)
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			writeError(w, http.StatusConflict, "username already exists")
			return
		}
		logger.Errorf("register: failed to create user: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	logger.Infof("user registered: %s (role: %s)", user.Username, user.Role)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"user": map[string]interface{}{
			"id":         user.ID,
			"username":   user.Username,
			"role":       user.Role,
			"status":     user.Status,
			"created_at": user.CreatedAt,
		},
	})
}

// handleListUsers handles listing all users (GET /api/v1/users).
// Requires admin role.
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.userRepoGuard(w) {
		return
	}
	if ok, resp := s.requireAdmin(r); !ok {
		writeError(w, http.StatusForbidden, resp)
		return
	}

	users, err := s.userRepo.List()
	if err != nil {
		logger.Errorf("list users: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	result := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		result = append(result, map[string]interface{}{
			"id":         u.ID,
			"username":   u.Username,
			"role":       u.Role,
			"status":     u.Status,
			"created_at": u.CreatedAt,
			"updated_at": u.UpdatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"users": result,
		"total": len(result),
	})
}

// handleGetUser handles getting a single user (GET /api/v1/users/{id}).
func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.userRepoGuard(w) {
		return
	}

	currentUser := s.getCurrentUser(r)
	if currentUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := r.PathValue("id")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	// Non-admin can only view themselves
	if currentUser.Role != "admin" && currentUser.ID != targetID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	user, err := s.userRepo.GetByID(targetID)
	if err != nil {
		logger.Errorf("get user: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user": map[string]interface{}{
			"id":         user.ID,
			"username":   user.Username,
			"role":       user.Role,
			"status":     user.Status,
			"created_at": user.CreatedAt,
			"updated_at": user.UpdatedAt,
		},
	})
}

// handleUpdateUser handles updating a user (PUT /api/v1/users/{id}).
func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.userRepoGuard(w) { return }
	currentUser := s.getCurrentUser(r)
	if currentUser == nil { writeError(w, http.StatusUnauthorized, "unauthorized"); return }
	idStr := r.PathValue("id")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil { writeError(w, http.StatusBadRequest, "invalid user ID"); return }
	if currentUser.Role != "admin" && currentUser.ID != targetID { writeError(w, http.StatusForbidden, "forbidden"); return }
	user, err := s.userRepo.GetByID(targetID)
	if err != nil { logger.Errorf("update user: %v", err); writeError(w, http.StatusInternalServerError, "internal error"); return }
	if user == nil { writeError(w, http.StatusNotFound, "user not found"); return }
	var req struct { Username *string `json:"username"`; Role *string `json:"role"`; Status *string `json:"status"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeError(w, http.StatusBadRequest, "invalid request body"); return }
	if applyUserUpdateFields(w, user, req, currentUser, targetID, s) { return }
	if err := s.userRepo.Update(user); err != nil { logger.Errorf("update user: %v", err); writeError(w, http.StatusInternalServerError, "failed to update user"); return }
	logger.Infof("user updated: %s (id=%d)", user.Username, user.ID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"user": map[string]interface{}{
		"id": user.ID, "username": user.Username, "role": user.Role, "status": user.Status,
		"created_at": user.CreatedAt, "updated_at": user.UpdatedAt,
	}})
}

// applyUserUpdateFields 应用用户更新字段，有错误时写入 HTTP 响应并返回 true
func applyUserUpdateFields(w http.ResponseWriter, user *auth.User, req struct {
	Username *string `json:"username"`; Role *string `json:"role"`; Status *string `json:"status"`
}, currentUser *auth.User, targetID int64, s *Server) (hasError bool) {
	if req.Role != nil {
		if currentUser.Role != "admin" { writeError(w, http.StatusForbidden, "only admin can change roles"); return true }
		if !map[string]bool{"admin": true, "operator": true, "readonly": true}[*req.Role] {
			writeError(w, http.StatusBadRequest, "invalid role: must be admin, operator, or readonly"); return true
		}
		user.Role = *req.Role
	}
	if req.Status != nil {
		if currentUser.Role != "admin" { writeError(w, http.StatusForbidden, "only admin can change status"); return true }
		if *req.Status != "active" && *req.Status != "disabled" {
			writeError(w, http.StatusBadRequest, "invalid status: must be active or disabled"); return true
		}
		user.Status = *req.Status
	}
	if req.Username != nil {
		newName := strings.TrimSpace(*req.Username)
		if newName == "" { writeError(w, http.StatusBadRequest, "username cannot be empty"); return true }
		if len(newName) < 3 || len(newName) > 32 { writeError(w, http.StatusBadRequest, "username must be 3-32 characters"); return true }
		existing, _ := s.userRepo.GetByUsername(newName)
		if existing != nil && existing.ID != targetID { writeError(w, http.StatusConflict, "username already taken"); return true }
		user.Username = newName
	}
	return false
}

// handleDeleteUser handles deleting a user (DELETE /api/v1/users/{id}).
// Admin only. Cannot delete yourself.
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.userRepoGuard(w) {
		return
	}

	if ok, resp := s.requireAdmin(r); !ok {
		writeError(w, http.StatusForbidden, resp)
		return
	}

	currentUser := s.getCurrentUser(r)
	idStr := r.PathValue("id")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	if currentUser != nil && currentUser.ID == targetID {
		writeError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	user, err := s.userRepo.GetByID(targetID)
	if err != nil {
		logger.Errorf("delete user: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if err := s.userRepo.Delete(targetID); err != nil {
		logger.Errorf("delete user: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}

	logger.Infof("user deleted: %s (id=%d)", user.Username, user.ID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "user deleted",
	})
}

// handleChangeUserPassword handles password change (POST /api/v1/users/{id}/password).
func (s *Server) handleChangeUserPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.userRepoGuard(w) {
		return
	}

	currentUser := s.getCurrentUser(r)
	if currentUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := r.PathValue("id")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	if currentUser.Role != "admin" && currentUser.ID != targetID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}

	user, err := s.userRepo.GetByID(targetID)
	if err != nil {
		logger.Errorf("change password: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	// Non-admin must provide old password
	if currentUser.Role != "admin" {
		if req.OldPassword == "" {
			writeError(w, http.StatusBadRequest, "old password is required")
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
			writeError(w, http.StatusUnauthorized, "incorrect old password")
			return
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		logger.Errorf("change password: failed to hash: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := s.userRepo.UpdatePassword(targetID, string(hash)); err != nil {
		logger.Errorf("change password: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	logger.Infof("password changed for user: %s (id=%d)", user.Username, user.ID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "password updated",
	})
}

// getCurrentUser extracts the authenticated user from the request context.
// Returns a synthetic admin user for admin-token-authenticated requests.
func (s *Server) getCurrentUser(r *http.Request) *auth.User {
	userID, ok := r.Context().Value(contextKeyUserID).(int64)
	if !ok {
		return nil
	}

	// Admin token auth: return synthetic admin user
	if userID == adminSyntheticUserID {
		return &auth.User{
			ID:       adminSyntheticUserID,
			Username: "admin (token)",
			Role:     "admin",
			Status:   "active",
		}
	}

	// No user DB or userID=0 (legacy mode): cannot resolve user
	if userID == 0 || s.userRepo == nil {
		return nil
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil || user == nil {
		return nil
	}
	return user
}

// requireAdmin checks if the current user has admin role.
func (s *Server) requireAdmin(r *http.Request) (bool, string) {
	user := s.getCurrentUser(r)
	if user == nil {
		return false, "unauthorized"
	}
	if user.Role != "admin" {
		return false, "admin role required"
	}
	return true, ""
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
