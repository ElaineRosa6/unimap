package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var ErrBootstrapAlreadyCompleted = errors.New("user bootstrap already completed")

// UserDB manages the SQLite connection for user persistence.
type UserDB struct {
	db *sql.DB
}

// NewUserDB opens (or creates) the SQLite database at dbPath.
func NewUserDB(dbPath string) (*UserDB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open user database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping user database: %w", err)
	}
	return &UserDB{db: db}, nil
}

// Close closes the database connection.
func (d *UserDB) Close() error {
	return d.db.Close()
}

// DB returns the underlying *sql.DB.
func (d *UserDB) DB() *sql.DB {
	return d.db
}

// InitSchema creates the users table and index if they do not exist.
func (d *UserDB) InitSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT    NOT NULL UNIQUE,
		password_hash TEXT    NOT NULL,
		role          TEXT    NOT NULL DEFAULT 'readonly',
		status        TEXT    NOT NULL DEFAULT 'active',
		session_version INTEGER NOT NULL DEFAULT 0,
		created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
	`
	if _, err := d.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create users schema: %w", err)
	}
	var sessionVersionColumn int
	rows, err := d.db.Query(`PRAGMA table_info(users)`)
	if err != nil {
		return fmt.Errorf("inspect users schema: %w", err)
	}
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return fmt.Errorf("inspect users column: %w", err)
		}
		if name == "session_version" {
			sessionVersionColumn = 1
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate users schema: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close users schema inspection: %w", err)
	}
	if sessionVersionColumn == 0 {
		if _, err := d.db.Exec(`ALTER TABLE users ADD COLUMN session_version INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add users session_version: %w", err)
		}
	}
	return nil
}

// User represents a user account.
type User struct {
	ID             int64     `json:"id"`
	Username       string    `json:"username"`
	PasswordHash   string    `json:"-"` // never serialize
	Role           string    `json:"role"`
	Status         string    `json:"status"`
	SessionVersion int64     `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// UserRepository defines operations for user persistence.
type UserRepository interface {
	Create(username, passwordHash, role string) (*User, error)
	CreateBootstrapAdmin(username, passwordHash string) (*User, error)
	GetByID(id int64) (*User, error)
	GetByUsername(username string) (*User, error)
	List() ([]*User, error)
	Update(user *User) error
	Delete(id int64) error
	UpdatePassword(id int64, passwordHash string) error
	Count() (int, error)
}

func (r *userRepository) CreateBootstrapAdmin(username, passwordHash string) (*User, error) {
	now := time.Now()
	result, err := r.db.Exec(
		`INSERT INTO users (username, password_hash, role, status, created_at, updated_at)
		 SELECT ?, ?, 'admin', 'active', ?, ? WHERE NOT EXISTS (SELECT 1 FROM users)`,
		username, passwordHash, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create bootstrap admin: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("check bootstrap admin creation: %w", err)
	}
	if rowsAffected != 1 {
		return nil, ErrBootstrapAlreadyCompleted
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get bootstrap admin ID: %w", err)
	}
	return &User{ID: id, Username: username, PasswordHash: passwordHash, Role: "admin", Status: "active", CreatedAt: now, UpdatedAt: now}, nil
}

type userRepository struct {
	db *sql.DB
}

// NewUserRepository creates a repository backed by the given db.
func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(username, passwordHash, role string) (*User, error) {
	now := time.Now()
	result, err := r.db.Exec(
		`INSERT INTO users (username, password_hash, role, status, created_at, updated_at)
		 VALUES (?, ?, ?, 'active', ?, ?)`,
		username, passwordHash, role, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}
	return &User{
		ID:           id,
		Username:     username,
		PasswordHash: passwordHash,
		Role:         role,
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (r *userRepository) GetByID(id int64) (*User, error) {
	user := &User{}
	err := r.db.QueryRow(
		`SELECT id, username, password_hash, role, status, session_version, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.Status, &user.SessionVersion, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}
	return user, nil
}

func (r *userRepository) GetByUsername(username string) (*User, error) {
	user := &User{}
	err := r.db.QueryRow(
		`SELECT id, username, password_hash, role, status, session_version, created_at, updated_at
		 FROM users WHERE username = ?`, username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.Status, &user.SessionVersion, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}
	return user, nil
}

func (r *userRepository) List() ([]*User, error) {
	rows, err := r.db.Query(
		`SELECT id, username, password_hash, role, status, session_version, created_at, updated_at
		 FROM users ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &u.SessionVersion, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *userRepository) Update(user *User) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE users SET username = ?, role = ?, status = ?,
		 session_version = CASE WHEN status != 'disabled' AND ? = 'disabled' THEN session_version + 1 ELSE session_version END,
		 updated_at = ? WHERE id = ?`,
		user.Username, user.Role, user.Status, user.Status, now, user.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

func (r *userRepository) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}

func (r *userRepository) UpdatePassword(id int64, passwordHash string) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE users SET password_hash = ?, session_version = session_version + 1, updated_at = ? WHERE id = ?`,
		passwordHash, now, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	return nil
}

func (r *userRepository) Count() (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}
