package batchdb

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// Database manages the SQLite connection for screenshot batch job metadata.
type Database struct {
	db *sql.DB
}

// NewDatabase opens or creates the SQLite database at dbPath.
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open screenshot batch database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping screenshot batch database: %w", err)
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

// InitSchema creates the screenshot_batch_jobs table and indexes if they do not exist.
func (d *Database) InitSchema() error {
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS screenshot_batch_jobs (
			id          TEXT PRIMARY KEY,
			status      TEXT NOT NULL,
			total       INTEGER NOT NULL DEFAULT 0,
			completed   INTEGER NOT NULL DEFAULT 0,
			success     INTEGER NOT NULL DEFAULT 0,
			failed      INTEGER NOT NULL DEFAULT 0,
			error_msg   TEXT NOT NULL DEFAULT '',
			results     TEXT NOT NULL DEFAULT '[]',
			started_at  DATETIME NOT NULL,
			ended_at    DATETIME
		)
	`); err != nil {
		return fmt.Errorf("failed to create screenshot_batch_jobs table: %w", err)
	}

	if _, err := d.db.Exec(
		`CREATE INDEX IF NOT EXISTS idx_screenshot_batch_started ON screenshot_batch_jobs(started_at DESC)`,
	); err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}
	return nil
}
