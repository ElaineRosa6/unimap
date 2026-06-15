package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/unimap/project/internal/auth"
	"github.com/unimap/project/internal/model"
)

// contextWithUserID creates a context with the given user ID for testing.
func contextWithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, contextKeyUserID, userID)
}

// mockUserRepo is a minimal in-memory user repository for testing.
type mockUserRepo struct {
	users  map[int64]*auth.User
	nextID int64
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[int64]*auth.User), nextID: 1}
}

func (m *mockUserRepo) Create(username, passwordHash, role string) (*auth.User, error) {
	u := &auth.User{
		ID:           m.nextID,
		Username:     username,
		PasswordHash: passwordHash,
		Role:         role,
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	m.users[u.ID] = u
	m.nextID++
	return u, nil
}

func (m *mockUserRepo) GetByID(id int64) (*auth.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *mockUserRepo) GetByUsername(username string) (*auth.User, error) {
	for _, u := range m.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) List() ([]*auth.User, error) {
	users := make([]*auth.User, 0, len(m.users))
	for _, u := range m.users {
		users = append(users, u)
	}
	return users, nil
}

func (m *mockUserRepo) Update(user *auth.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) Delete(id int64) error {
	delete(m.users, id)
	return nil
}

func (m *mockUserRepo) UpdatePassword(id int64, passwordHash string) error {
	if u, ok := m.users[id]; ok {
		u.PasswordHash = passwordHash
	}
	return nil
}

func (m *mockUserRepo) Count() (int, error) {
	return len(m.users), nil
}

func TestUserRepoGuard_Nil(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	if s.userRepoGuard(rec) {
		t.Fatal("expected false when userRepo is nil")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestUserRepoGuard_OK(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	rec := httptest.NewRecorder()
	if !s.userRepoGuard(rec) {
		t.Fatal("expected true when userRepo is set")
	}
}

func TestHandleRegister_MethodNotAllowed(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/register", nil)
	s.handleRegister(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleRegister_NoRepo(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", nil)
	s.handleRegister(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleRegister_InvalidBody(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewReader([]byte("invalid")))
	s.handleRegister(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleRegister_EmptyUsername(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"username": "", "password": "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewReader(body))
	s.handleRegister(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleRegister_ShortUsername(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"username": "ab", "password": "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewReader(body))
	s.handleRegister(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleRegister_ShortPassword(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "short"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewReader(body))
	s.handleRegister(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleRegister_FirstUserGetsAdmin(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewReader(body))
	s.handleRegister(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var resp model.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if !resp.Success {
		t.Fatal("expected success=true")
	}
}

func TestHandleRegister_NonAdminRequiresAuth(t *testing.T) {
	repo := newMockUserRepo()
	repo.Create("existing", "hash", "admin")
	s := &Server{userRepo: repo}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"username": "newuser", "password": "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", bytes.NewReader(body))
	s.handleRegister(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleListUsers_MethodNotAllowed(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", nil)
	s.handleListUsers(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleListUsers_NoRepo(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	s.handleListUsers(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleListUsers_NotAdmin(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	// No context key set => no user => 403
	s.handleListUsers(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestHandleGetUser_MethodNotAllowed(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/1", nil)
	s.handleGetUser(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleGetUser_NoRepo(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	s.handleGetUser(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleDeleteUser_MethodNotAllowed(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	s.handleDeleteUser(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleDeleteUser_NoRepo(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/1", nil)
	s.handleDeleteUser(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestGetCurrentUser_NoContext(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	user := s.getCurrentUser(req)
	if user != nil {
		t.Fatal("expected nil user")
	}
}

func TestGetCurrentUser_AdminSynthetic(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := req.Context()
	ctx = contextWithUserID(ctx, adminSyntheticUserID)
	req = req.WithContext(ctx)
	user := s.getCurrentUser(req)
	if user == nil {
		t.Fatal("expected synthetic admin user")
	}
	if user.Role != "admin" {
		t.Fatalf("expected admin role, got %s", user.Role)
	}
}

func TestGetCurrentUser_ZeroUserID(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := req.Context()
	ctx = contextWithUserID(ctx, 0)
	req = req.WithContext(ctx)
	user := s.getCurrentUser(req)
	if user != nil {
		t.Fatal("expected nil for userID=0")
	}
}

func TestRequireAdmin_NoUser(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ok, msg := s.requireAdmin(req)
	if ok {
		t.Fatal("expected false")
	}
	if msg != "unauthorized" {
		t.Fatalf("expected 'unauthorized', got %q", msg)
	}
}

func TestApplyUserUpdateFields_EmptyUsername(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	user := &auth.User{ID: 1, Username: "admin"}
	empty := ""
	req := struct {
		Username *string `json:"username"`
		Role     *string `json:"role"`
		Status   *string `json:"status"`
	}{Username: &empty}
	currentUser := &auth.User{ID: 1, Role: "admin"}
	rec := httptest.NewRecorder()
	hasErr := applyUserUpdateFields(rec, user, req, currentUser, 1, s)
	if !hasErr {
		t.Fatal("expected error for empty username")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestApplyUserUpdateFields_InvalidRole(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	user := &auth.User{ID: 1, Username: "admin", Role: "admin"}
	badRole := "superadmin"
	req := struct {
		Username *string `json:"username"`
		Role     *string `json:"role"`
		Status   *string `json:"status"`
	}{Role: &badRole}
	currentUser := &auth.User{ID: 1, Role: "admin"}
	rec := httptest.NewRecorder()
	hasErr := applyUserUpdateFields(rec, user, req, currentUser, 1, s)
	if !hasErr {
		t.Fatal("expected error for invalid role")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestApplyUserUpdateFields_InvalidStatus(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	user := &auth.User{ID: 1, Username: "admin", Status: "active"}
	badStatus := "deleted"
	req := struct {
		Username *string `json:"username"`
		Role     *string `json:"role"`
		Status   *string `json:"status"`
	}{Status: &badStatus}
	currentUser := &auth.User{ID: 1, Role: "admin"}
	rec := httptest.NewRecorder()
	hasErr := applyUserUpdateFields(rec, user, req, currentUser, 1, s)
	if !hasErr {
		t.Fatal("expected error for invalid status")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestApplyUserUpdateFields_NonAdminRoleChange(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	user := &auth.User{ID: 1, Username: "user1", Role: "readonly"}
	newRole := "admin"
	req := struct {
		Username *string `json:"username"`
		Role     *string `json:"role"`
		Status   *string `json:"status"`
	}{Role: &newRole}
	currentUser := &auth.User{ID: 2, Role: "readonly"}
	rec := httptest.NewRecorder()
	hasErr := applyUserUpdateFields(rec, user, req, currentUser, 1, s)
	if !hasErr {
		t.Fatal("expected error for non-admin role change")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestApplyUserUpdateFields_NonAdminStatusChange(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	user := &auth.User{ID: 1, Username: "user1", Status: "active"}
	newStatus := "disabled"
	req := struct {
		Username *string `json:"username"`
		Role     *string `json:"role"`
		Status   *string `json:"status"`
	}{Status: &newStatus}
	currentUser := &auth.User{ID: 2, Role: "readonly"}
	rec := httptest.NewRecorder()
	hasErr := applyUserUpdateFields(rec, user, req, currentUser, 1, s)
	if !hasErr {
		t.Fatal("expected error for non-admin status change")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestApplyUserUpdateFields_ShortUsername(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	user := &auth.User{ID: 1, Username: "admin"}
	shortName := "ab"
	req := struct {
		Username *string `json:"username"`
		Role     *string `json:"role"`
		Status   *string `json:"status"`
	}{Username: &shortName}
	currentUser := &auth.User{ID: 1, Role: "admin"}
	rec := httptest.NewRecorder()
	hasErr := applyUserUpdateFields(rec, user, req, currentUser, 1, s)
	if !hasErr {
		t.Fatal("expected error for short username")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestApplyUserUpdateFields_DuplicateUsername(t *testing.T) {
	repo := newMockUserRepo()
	repo.Create("existing", "hash", "readonly")
	s := &Server{userRepo: repo}
	user := &auth.User{ID: 2, Username: "admin"}
	takenName := "existing"
	req := struct {
		Username *string `json:"username"`
		Role     *string `json:"role"`
		Status   *string `json:"status"`
	}{Username: &takenName}
	currentUser := &auth.User{ID: 2, Role: "admin"}
	rec := httptest.NewRecorder()
	hasErr := applyUserUpdateFields(rec, user, req, currentUser, 2, s)
	if !hasErr {
		t.Fatal("expected error for duplicate username")
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestApplyUserUpdateFields_Success(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	user := &auth.User{ID: 1, Username: "admin", Role: "admin", Status: "active"}
	newRole := "operator"
	newStatus := "disabled"
	newName := "newadmin"
	req := struct {
		Username *string `json:"username"`
		Role     *string `json:"role"`
		Status   *string `json:"status"`
	}{Role: &newRole, Status: &newStatus, Username: &newName}
	currentUser := &auth.User{ID: 1, Role: "admin"}
	rec := httptest.NewRecorder()
	hasErr := applyUserUpdateFields(rec, user, req, currentUser, 1, s)
	if hasErr {
		t.Fatal("expected no error")
	}
	if user.Role != "operator" {
		t.Fatalf("expected role=operator, got %s", user.Role)
	}
	if user.Status != "disabled" {
		t.Fatalf("expected status=disabled, got %s", user.Status)
	}
	if user.Username != "newadmin" {
		t.Fatalf("expected username=newadmin, got %s", user.Username)
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "test error")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var resp model.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Error != "test error" {
		t.Fatalf("expected error message, got %v", resp.Error)
	}
}