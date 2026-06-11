package auth

import (
	"path/filepath"
	"testing"
)

func TestNewUserDB(t *testing.T) {
	t.Run("creates database successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")
		udb, err := NewUserDB(dbPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer udb.Close()

		if udb.DB() == nil {
			t.Error("expected non-nil DB")
		}
	})

	t.Run("invalid path returns error", func(t *testing.T) {
		_, err := NewUserDB("/nonexistent/path/test.db")
		if err == nil {
			t.Fatal("expected error for invalid path")
		}
	})
}

func TestUserDB_InitSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	udb, err := NewUserDB(dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer udb.Close()

	err = udb.InitSchema()
	if err != nil {
		t.Fatalf("unexpected error initializing schema: %v", err)
	}

	// Running again should succeed (idempotent)
	err = udb.InitSchema()
	if err != nil {
		t.Fatalf("unexpected error on second schema init: %v", err)
	}
}

func TestUserRepository_CRUD(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	udb, err := NewUserDB(dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer udb.Close()

	if err := udb.InitSchema(); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	repo := NewUserRepository(udb.DB())

	t.Run("Create", func(t *testing.T) {
		user, err := repo.Create("testuser", "hash123", "operator")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Username != "testuser" {
			t.Errorf("expected username 'testuser', got %q", user.Username)
		}
		if user.Role != "operator" {
			t.Errorf("expected role 'operator', got %q", user.Role)
		}
		if user.Status != "active" {
			t.Errorf("expected status 'active', got %q", user.Status)
		}
	})

	t.Run("GetByUsername", func(t *testing.T) {
		user, err := repo.GetByUsername("testuser")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user == nil {
			t.Fatal("expected user to be found")
		}
		if user.Username != "testuser" {
			t.Errorf("expected username 'testuser', got %q", user.Username)
		}
	})

	t.Run("GetByID", func(t *testing.T) {
		user, _ := repo.GetByUsername("testuser")
		found, err := repo.GetByID(user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found == nil {
			t.Fatal("expected user to be found")
		}
		if found.ID != user.ID {
			t.Errorf("expected ID %d, got %d", user.ID, found.ID)
		}
	})

	t.Run("GetByID_NotFound", func(t *testing.T) {
		found, err := repo.GetByID(99999)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != nil {
			t.Error("expected nil for non-existent user")
		}
	})

	t.Run("GetByUsername_NotFound", func(t *testing.T) {
		found, err := repo.GetByUsername("nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != nil {
			t.Error("expected nil for non-existent user")
		}
	})

	t.Run("List", func(t *testing.T) {
		users, err := repo.List()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(users) != 1 {
			t.Errorf("expected 1 user, got %d", len(users))
		}
	})

	t.Run("Count", func(t *testing.T) {
		count, err := repo.Count()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 1 {
			t.Errorf("expected count 1, got %d", count)
		}
	})

	t.Run("Update", func(t *testing.T) {
		user, _ := repo.GetByUsername("testuser")
		user.Role = "admin"
		user.Status = "inactive"
		err := repo.Update(user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		updated, _ := repo.GetByID(user.ID)
		if updated.Role != "admin" {
			t.Errorf("expected role 'admin', got %q", updated.Role)
		}
		if updated.Status != "inactive" {
			t.Errorf("expected status 'inactive', got %q", updated.Status)
		}
	})

	t.Run("UpdatePassword", func(t *testing.T) {
		user, _ := repo.GetByUsername("testuser")
		err := repo.UpdatePassword(user.ID, "newhash456")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		updated, _ := repo.GetByID(user.ID)
		if updated.PasswordHash != "newhash456" {
			t.Errorf("expected password hash 'newhash456', got %q", updated.PasswordHash)
		}
	})

	t.Run("CreateDuplicate", func(t *testing.T) {
		_, err := repo.Create("testuser", "hash", "readonly")
		if err == nil {
			t.Fatal("expected error for duplicate username")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		user, _ := repo.GetByUsername("testuser")
		err := repo.Delete(user.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		found, _ := repo.GetByID(user.ID)
		if found != nil {
			t.Error("expected user to be deleted")
		}
	})

	t.Run("ListEmpty", func(t *testing.T) {
		users, err := repo.List()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(users) != 0 {
			t.Errorf("expected 0 users, got %d", len(users))
		}
	})

	t.Run("CountEmpty", func(t *testing.T) {
		count, err := repo.Count()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("expected count 0, got %d", count)
		}
	})
}
