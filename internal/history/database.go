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

	// notify_push_state records which asset keys have already been pushed for
	// each scheduled task, so incremental ("only new") pushes can deduplicate.
	// task_id is the stable task name (survives re-creation), asset_key is the
	// ip[:port] fingerprint.
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS notify_push_state (
			task_id TEXT NOT NULL,
			asset_key TEXT NOT NULL,
			first_pushed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_pushed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (task_id, asset_key)
		)
	`); err != nil {
		return fmt.Errorf("failed to create notify_push_state table: %w", err)
	}

	// notification_push_log is an append-only audit trail of every notification
	// push from a scheduled task: when, which task, to which channels, and the
	// outcome. notify_push_state above is the dedup state used by only_new; this
	// table records the push events themselves so past pushes stay inspectable
	// even after the in-memory scheduler history is trimmed.
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS notification_push_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id TEXT NOT NULL,
			task_name TEXT NOT NULL,
			channel_ids TEXT NOT NULL,
			status TEXT NOT NULL,
			result_count INTEGER NOT NULL DEFAULT 0,
			result_summary TEXT,
			error TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create notification_push_log table: %w", err)
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_op_history_type ON operation_history(operation_type)`,
		`CREATE INDEX IF NOT EXISTS idx_op_history_created ON operation_history(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_op_results_history ON operation_results(history_id)`,
		`CREATE INDEX IF NOT EXISTS idx_push_state_task ON notify_push_state(task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_push_log_task ON notification_push_log(task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_push_log_created ON notification_push_log(created_at)`,
	}
	for _, idx := range indexes {
		if _, err := d.db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}
	return nil
}
