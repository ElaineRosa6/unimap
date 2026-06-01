package database

import (
	"fmt"
	"sort"
	"time"

	"github.com/unimap/project/internal/logger"
)

// migration records a single schema change. Add new entries at the bottom.
type migration struct {
	Version     int
	Description string
	Up          string
	Down        string
}

// migrations is the ordered list of schema changes, append-only.
// Version 0 creates the version-tracking table itself.
// Version 1 is the initial schema (extracted from InitSchema).
var migrations = []migration{
	{
		Version:     1,
		Description: "initial schema: icp_query_runs + icp_results + indexes",
		Up: `
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
);

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
);

CREATE INDEX IF NOT EXISTS idx_icp_runs_task_id    ON icp_query_runs(task_id);
CREATE INDEX IF NOT EXISTS idx_icp_runs_started_at ON icp_query_runs(started_at);
CREATE INDEX IF NOT EXISTS idx_icp_runs_keyword_type ON icp_query_runs(query_keyword, query_type);
CREATE INDEX IF NOT EXISTS idx_icp_results_run_id  ON icp_results(run_id);
CREATE INDEX IF NOT EXISTS idx_icp_results_domain  ON icp_results(domain);
CREATE INDEX IF NOT EXISTS idx_icp_results_licence ON icp_results(licence);
CREATE INDEX IF NOT EXISTS idx_icp_results_unit    ON icp_results(unit_name);
`,
		Down: `
DROP INDEX IF EXISTS idx_icp_results_unit;
DROP INDEX IF EXISTS idx_icp_results_licence;
DROP INDEX IF EXISTS idx_icp_results_domain;
DROP INDEX IF EXISTS idx_icp_results_run_id;
DROP INDEX IF EXISTS idx_icp_runs_keyword_type;
DROP INDEX IF EXISTS idx_icp_runs_started_at;
DROP INDEX IF EXISTS idx_icp_runs_task_id;
DROP TABLE IF EXISTS icp_results;
DROP TABLE IF EXISTS icp_query_runs;
`,
	},
}

// currentVersion returns the latest migration version.
func currentVersion() int {
	if len(migrations) == 0 {
		return 0
	}
	return migrations[len(migrations)-1].Version
}

// InitSchemaWithMigration ensures the migration tracking table exists and
// applies any pending migrations. Safe to call on an existing database.
func (d *Database) InitSchemaWithMigration() error {
	// Step 1: create migration tracking table if not exists
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     INTEGER PRIMARY KEY,
			description TEXT    NOT NULL DEFAULT '',
			applied_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Step 2: read current version
	var current int
	row := d.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")
	if err := row.Scan(&current); err != nil {
		// Table might not exist yet (pre-migration database), start from 0
		current = 0
	}

	latest := currentVersion()
	if current == latest {
		logger.Debugf("ICP database schema is up to date (version %d)", current)
		return nil
	}

	logger.Infof("ICP database schema migration: current=%d, target=%d", current, latest)

	// Step 3: apply pending migrations
	for _, m := range migrations {
		if m.Version <= current {
			continue
		}
		logger.Infof("Applying ICP database migration v%d: %s", m.Version, m.Description)

		if _, err := d.db.Exec(m.Up); err != nil {
			return fmt.Errorf("migration v%d up failed: %w", m.Version, err)
		}

		if _, err := d.db.Exec(
			"INSERT INTO schema_migrations (version, description, applied_at) VALUES (?, ?, ?)",
			m.Version, m.Description, time.Now(),
		); err != nil {
			return fmt.Errorf("migration v%d record failed: %w", m.Version, err)
		}
	}

	logger.Infof("ICP database schema migration complete (version %d)", latest)
	return nil
}

// RollbackMigration rolls back the latest applied migration. Returns the
// rolled-back version, or 0 if there is nothing to roll back.
func (d *Database) RollbackMigration() (int, error) {
	var current int
	row := d.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")
	if err := row.Scan(&current); err != nil {
		return 0, fmt.Errorf("failed to read migration version: %w", err)
	}
	if current == 0 {
		return 0, nil
	}

	// Find the migration
	idx := sort.Search(len(migrations), func(i int) bool {
		return migrations[i].Version >= current
	})
	if idx >= len(migrations) || migrations[idx].Version != current {
		return 0, fmt.Errorf("migration v%d not found in registry", current)
	}
	m := migrations[idx]

	logger.Warnf("Rolling back ICP database migration v%d: %s", m.Version, m.Description)

	if _, err := d.db.Exec(m.Down); err != nil {
		return 0, fmt.Errorf("migration v%d down failed: %w", m.Version, err)
	}

	if _, err := d.db.Exec("DELETE FROM schema_migrations WHERE version = ?", m.Version); err != nil {
		return 0, fmt.Errorf("migration v%d cleanup failed: %w", m.Version, err)
	}

	return current - 1, nil
}

// MigrationVersion returns the currently applied schema version.
func (d *Database) MigrationVersion() (int, error) {
	var current int
	row := d.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")
	if err := row.Scan(&current); err != nil {
		return 0, fmt.Errorf("failed to read migration version: %w", err)
	}
	return current, nil
}
