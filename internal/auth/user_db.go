package auth

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

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
		created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
	`
	if _, err := d.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create users schema: %w", err)
	}
	return nil
}

// User represents a user account.
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // never serialize
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserRepository defines operations for user persistence.
type UserRepository interface {
	Create(username, passwordHash, role string) (*User, error)
	GetByID(id int64) (*User, error)
	GetByUsername(username string) (*User, error)
	List() ([]*User, error)
	Update(user *User) error
	Delete(id int64) error
	UpdatePassword(id int64, passwordHash string) error
	Count() (int, error)
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
		`SELECT id, username, password_hash, role, status, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt)
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
		`SELECT id, username, password_hash, role, status, created_at, updated_at
		 FROM users WHERE username = ?`, username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt)
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
		`SELECT id, username, password_hash, role, status, created_at, updated_at
		 FROM users ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *userRepository) Update(user *User) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE users SET username = ?, role = ?, status = ?, updated_at = ? WHERE id = ?`,
		user.Username, user.Role, user.Status, now, user.ID,
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
		`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
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
