package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database manages the SQLite connection for ICP result persistence.
type Database struct {
	db *sql.DB
}

// NewDatabase opens (or creates) the SQLite database at dbPath.
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open ICP database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping ICP database: %w", err)
	}
	return &Database{db: db}, nil
}

// Close closes the database connection.
func (d *Database) Close() error {
	return d.db.Close()
}

// DB returns the underlying *sql.DB for repository constructors.
func (d *Database) DB() *sql.DB {
	return d.db
}

// InitSchema creates tables and indexes if they do not exist.
func (d *Database) InitSchema() error {
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS icp_query_runs (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id      TEXT    NOT NULL DEFAULT '',
			query_keyword TEXT   NOT NULL,
			query_type   TEXT    NOT NULL DEFAULT 'web',
			page         INTEGER NOT NULL DEFAULT 1,
			page_size    INTEGER NOT NULL DEFAULT 20,
			total_records INTEGER NOT NULL DEFAULT 0,
			result_count INTEGER NOT NULL DEFAULT 0,
			error_msg    TEXT    NOT NULL DEFAULT '',
			started_at   TIMESTAMP NOT NULL,
			created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create icp_query_runs table: %w", err)
	}

	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS icp_results (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id          INTEGER NOT NULL REFERENCES icp_query_runs(id) ON DELETE CASCADE,
			domain          TEXT    NOT NULL DEFAULT '',
			service_name    TEXT    NOT NULL DEFAULT '',
			licence         TEXT    NOT NULL DEFAULT '',
			unit_name       TEXT    NOT NULL DEFAULT '',
			nature_name     TEXT    NOT NULL DEFAULT '',
			city_name       TEXT    NOT NULL DEFAULT '',
			content_name    TEXT    NOT NULL DEFAULT '',
			main_licence    TEXT    NOT NULL DEFAULT '',
			update_record   TEXT    NOT NULL DEFAULT '',
			limit_access    TEXT    NOT NULL DEFAULT '',
			leader_name     TEXT    NOT NULL DEFAULT '',
			service_type_raw TEXT   NOT NULL DEFAULT '',
			main_id_raw     TEXT    NOT NULL DEFAULT '',
			detail_id_raw   TEXT    NOT NULL DEFAULT '',
			domain_id_raw   TEXT    NOT NULL DEFAULT '',
			service_id_raw  TEXT    NOT NULL DEFAULT '',
			data_id_raw     TEXT    NOT NULL DEFAULT '',
			update_rec_time TEXT    NOT NULL DEFAULT '',
			fetched_at      TIMESTAMP NOT NULL,
			created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create icp_results table: %w", err)
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_icp_runs_task_id    ON icp_query_runs(task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_icp_runs_started_at ON icp_query_runs(started_at)`,
		`CREATE INDEX IF NOT EXISTS idx_icp_runs_keyword_type ON icp_query_runs(query_keyword, query_type)`,
		`CREATE INDEX IF NOT EXISTS idx_icp_results_run_id  ON icp_results(run_id)`,
		`CREATE INDEX IF NOT EXISTS idx_icp_results_domain  ON icp_results(domain)`,
		`CREATE INDEX IF NOT EXISTS idx_icp_results_licence ON icp_results(licence)`,
		`CREATE INDEX IF NOT EXISTS idx_icp_results_unit    ON icp_results(unit_name)`,
	}
	for _, idx := range indexes {
		if _, err := d.db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}
	return nil
}

// ICPQueryRun represents one scheduled ICP query execution.
type ICPQueryRun struct {
	ID           int64     `json:"id"`
	TaskID       string    `json:"task_id"`
	QueryKeyword string    `json:"query_keyword"`
	QueryType    string    `json:"query_type"`
	Page         int       `json:"page"`
	PageSize     int       `json:"page_size"`
	TotalRecords int       `json:"total_records"`
	ResultCount  int       `json:"result_count"`
	ErrorMsg     string    `json:"error_msg,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// ICPResultRow represents a single ICP record persisted in SQLite.
type ICPResultRow struct {
	ID             int64     `json:"id"`
	RunID          int64     `json:"run_id"`
	Domain         string    `json:"domain"`
	ServiceName    string    `json:"service_name"`
	Licence        string    `json:"licence"`
	UnitName       string    `json:"unit_name"`
	NatureName     string    `json:"nature_name"`
	CityName       string    `json:"city_name"`
	ContentName    string    `json:"content_name"`
	MainLicence    string    `json:"main_licence"`
	UpdateRecord   string    `json:"update_record"`
	LimitAccess    string    `json:"limit_access"`
	LeaderName     string    `json:"leader_name"`
	ServiceTypeRaw string    `json:"service_type_raw,omitempty"`
	MainIDRaw      string    `json:"main_id_raw,omitempty"`
	DetailIDRaw    string    `json:"detail_id_raw,omitempty"`
	DomainIDRaw    string    `json:"domain_id_raw,omitempty"`
	ServiceIDRaw   string    `json:"service_id_raw,omitempty"`
	DataIDRaw      string    `json:"data_id_raw,omitempty"`
	UpdateRecTime  string    `json:"update_rec_time"`
	FetchedAt      time.Time `json:"fetched_at"`
	CreatedAt      time.Time `json:"created_at"`
}
