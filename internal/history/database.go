package history

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// Database manages the SQLite connection for operation history.
type Database struct {
	db *sql.DB
}

// NewDatabase opens or creates the SQLite database at dbPath.
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open history database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping history database: %w", err)
	}
	return &Database{db: db}, nil
}

// Close closes the database connection.
func (d *Database) Close() error {
	return d.db.Close()
}

// DB returns the underlying *sql.DB.
func (d *Database) DB() *sql.DB {
	return d.db
}

// InitSchema creates tables and indexes if they do not exist.
func (d *Database) InitSchema() error {
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS operation_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			operation_type TEXT NOT NULL,
			input TEXT NOT NULL,
			status TEXT NOT NULL,
			total_count INTEGER DEFAULT 0,
			summary TEXT,
			duration_ms INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create operation_history table: %w", err)
	}

	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS operation_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			history_id INTEGER NOT NULL,
			data TEXT NOT NULL,
			FOREIGN KEY (history_id) REFERENCES operation_history(id) ON DELETE CASCADE
		)
	`); err != nil {
		return fmt.Errorf("failed to create operation_results table: %w", err)
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_op_history_type ON operation_history(operation_type)`,
		`CREATE INDEX IF NOT EXISTS idx_op_history_created ON operation_history(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_op_results_history ON operation_results(history_id)`,
	}
	for _, idx := range indexes {
		if _, err := d.db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}
	return nil
}
