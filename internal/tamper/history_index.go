package tamper

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func (s *HashStorage) historyIndex() (*sql.DB, error) {
	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", filepath.Join(s.baseDir, "check_records.db")+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS check_records (id TEXT PRIMARY KEY, url TEXT NOT NULL, timestamp INTEGER NOT NULL, payload TEXT NOT NULL); CREATE INDEX IF NOT EXISTS idx_check_records_url_time ON check_records(url, timestamp DESC);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (s *HashStorage) indexCheckRecord(record *CheckRecord) error {
	db, err := s.historyIndex()
	if err != nil {
		return err
	}
	defer db.Close()
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT OR REPLACE INTO check_records(id,url,timestamp,payload) VALUES(?,?,?,?)`, record.ID, record.URL, record.Timestamp, string(payload))
	return err
}

func (s *HashStorage) listIndexedCheckRecords() (map[string][]*CheckRecord, error) {
	db, err := s.historyIndex()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT payload FROM check_records ORDER BY timestamp DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string][]*CheckRecord)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var record CheckRecord
		if err := json.Unmarshal([]byte(payload), &record); err != nil {
			continue
		}
		result[record.URL] = append(result[record.URL], &record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate check record index: %w", err)
	}
	return result, nil
}

func (s *HashStorage) deleteIndexedCheckRecords(url string) error {
	db, err := s.historyIndex()
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`DELETE FROM check_records WHERE url = ?`, url)
	return err
}
