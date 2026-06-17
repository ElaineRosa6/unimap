package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleUpdateUser_MethodNotAllowed(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	s.handleUpdateUser(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleUpdateUser_NoRepo(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/1", nil)
	s.handleUpdateUser(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleUpdateUser_NoAuth(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/1", nil)
	s.handleUpdateUser(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleUpdateUser_InvalidID(t *testing.T) {
	repo := newMockUserRepo()
	user, _ := repo.Create("user1", "hash", "readonly")
	s := &Server{userRepo: repo}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/abc", nil)
	req.SetPathValue("id", "abc")
	ctx := req.Context()
	ctx = contextWithUserID(ctx, user.ID)
	req = req.WithContext(ctx)
	s.handleUpdateUser(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleUpdateUser_Forbidden_NonAdmin(t *testing.T) {
	repo := newMockUserRepo()
	repo.Create("user1", "hash", "readonly")
	s := &Server{userRepo: repo}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/2", nil)
	req.SetPathValue("id", "2")
	ctx := req.Context()
	ctx = contextWithUserID(ctx, 1) // user1
	req = req.WithContext(ctx)
	s.handleUpdateUser(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestHandleUpdateUser_NotFound(t *testing.T) {
	repo := newMockUserRepo()
	admin, _ := repo.Create("admin", "hash", "admin")
	s := &Server{userRepo: repo}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"username": "newname"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/999", bytes.NewReader(body))
	req.SetPathValue("id", "999")
	ctx := req.Context()
	ctx = contextWithUserID(ctx, admin.ID) // admin
	req = req.WithContext(ctx)
	s.handleUpdateUser(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleUpdateUser_InvalidBody(t *testing.T) {
	repo := newMockUserRepo()
	admin, _ := repo.Create("admin", "hash", "admin")
	s := &Server{userRepo: repo}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/1", bytes.NewReader([]byte("invalid")))
	ctx := req.Context()
	ctx = contextWithUserID(ctx, admin.ID)
	req = req.WithContext(ctx)
	s.handleUpdateUser(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleUpdateUser_Success(t *testing.T) {
	repo := newMockUserRepo()
	admin, _ := repo.Create("admin", "hash", "admin")
	s := &Server{userRepo: repo}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"username": "newadmin"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/1", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	ctx := req.Context()
	ctx = contextWithUserID(ctx, admin.ID)
	req = req.WithContext(ctx)
	s.handleUpdateUser(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if !resp["success"].(bool) {
		t.Fatal("expected success=true")
	}
}

func TestChangeUserPassword_MethodNotAllowed(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1/password", nil)
	s.handleChangeUserPassword(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestChangeUserPassword_NoRepo(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/1/password", nil)
	s.handleChangeUserPassword(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestChangeUserPassword_NoAuth(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/1/password", nil)
	s.handleChangeUserPassword(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestChangeUserPassword_InvalidID(t *testing.T) {
	repo := newMockUserRepo()
	user, _ := repo.Create("user1", "hash", "readonly")
	s := &Server{userRepo: repo}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/abc/password", nil)
	req.SetPathValue("id", "abc")
	ctx := req.Context()
	ctx = contextWithUserID(ctx, user.ID)
	req = req.WithContext(ctx)
	s.handleChangeUserPassword(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestChangeUserPassword_ShortPassword(t *testing.T) {
	repo := newMockUserRepo()
	user, _ := repo.Create("user1", "hash", "readonly")
	s := &Server{userRepo: repo}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"old_password": "old", "new_password": "short"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/1/password", bytes.NewReader(body))
	ctx := req.Context()
	ctx = contextWithUserID(ctx, user.ID)
	req = req.WithContext(ctx)
	s.handleChangeUserPassword(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestChangeUserPassword_InvalidBody(t *testing.T) {
	repo := newMockUserRepo()
	user, _ := repo.Create("user1", "hash", "readonly")
	s := &Server{userRepo: repo}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/1/password", bytes.NewReader([]byte("invalid")))
	ctx := req.Context()
	ctx = contextWithUserID(ctx, user.ID)
	req = req.WithContext(ctx)
	s.handleChangeUserPassword(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestChangeUserPassword_Forbidden_NonAdmin(t *testing.T) {
	repo := newMockUserRepo()
	user1, _ := repo.Create("user1", "hash", "readonly")
	user2, _ := repo.Create("user2", "hash", "readonly")
	s := &Server{userRepo: repo}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"old_password": "old", "new_password": "newpassword123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/2/password", bytes.NewReader(body))
	req.SetPathValue("id", "2")
	ctx := req.Context()
	ctx = contextWithUserID(ctx, user1.ID) // user1 trying to change user2's password
	req = req.WithContext(ctx)
	s.handleChangeUserPassword(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	_ = user2
}

func TestChangeUserPassword_AdminNoOldPassword(t *testing.T) {
	repo := newMockUserRepo()
	admin, _ := repo.Create("admin", "hash", "admin")
	s := &Server{userRepo: repo}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"new_password": "newpassword123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/1/password", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	ctx := req.Context()
	ctx = contextWithUserID(ctx, admin.ID)
	req = req.WithContext(ctx)
	s.handleChangeUserPassword(rec, req)
	// Admin can change without old password
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeUserPassword_WrongOldPassword(t *testing.T) {
	repo := newMockUserRepo()
	user, _ := repo.Create("user1", "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012", "readonly")
	s := &Server{userRepo: repo}
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"old_password": "wrong", "new_password": "newpassword123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/1/password", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	ctx := req.Context()
	ctx = contextWithUserID(ctx, user.ID)
	req = req.WithContext(ctx)
	s.handleChangeUserPassword(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAdmin_Admin(t *testing.T) {
	s := &Server{userRepo: newMockUserRepo()}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := req.Context()
	ctx = contextWithUserID(ctx, adminSyntheticUserID)
	req = req.WithContext(ctx)
	ok, _ := s.requireAdmin(req)
	if !ok {
		t.Fatal("expected true for admin")
	}
}

func TestRequireAdmin_NonAdmin(t *testing.T) {
	repo := newMockUserRepo()
	user, _ := repo.Create("user1", "hash", "readonly")
	s := &Server{userRepo: repo}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := req.Context()
	ctx = contextWithUserID(ctx, user.ID)
	req = req.WithContext(ctx)
	ok, msg := s.requireAdmin(req)
	if ok {
		t.Fatal("expected false for non-admin")
	}
	if msg != "admin role required" {
		t.Fatalf("expected 'admin role required', got %q", msg)
	}
}
