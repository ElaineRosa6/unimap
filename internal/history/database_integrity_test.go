package history

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestHistoryDeletionCascades(t *testing.T) {
	for _, mode := range []string{"single", "filtered", "all"} {
		t.Run(mode, func(t *testing.T) {
			r := setupTestDB(t)
			ids := make([]int64, 0, 2)
			for _, typ := range []OperationType{OpTypeQuery, OpTypeScreenshot} {
				id, err := r.CreateHistoryWithResults(&OperationHistory{OperationType: typ, Input: "fixture", Status: "success"}, []OperationResult{{Data: "one"}, {Data: "two"}})
				if err != nil {
					t.Fatal(err)
				}
				ids = append(ids, id)
			}
			var err error
			switch mode {
			case "single":
				err = r.DeleteHistory(ids[0])
			case "filtered":
				err = r.ClearHistory(string(OpTypeQuery))
			case "all":
				err = r.ClearHistory("")
			}
			if err != nil {
				t.Fatal(err)
			}
			for i, id := range ids {
				want := 0
				if mode != "all" && i == 1 {
					want = 2
				}
				rows, err := r.GetResults(id)
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) != want {
					t.Errorf("history %d: retained %d result rows, want %d", id, len(rows), want)
				}
			}
		})
	}
}

func TestHistoryForeignKeysEveryConnection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	for pass := 0; pass < 2; pass++ {
		func() {
			db, err := NewDatabase(path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := db.InitSchema(); err != nil {
				t.Fatal(err)
			}
			db.DB().SetMaxOpenConns(3)
			var held []*sql.Conn
			defer func() {
				for _, c := range held {
					c.Close()
				}
			}()
			for i := 0; i < 3; i++ {
				c, err := db.DB().Conn(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				held = append(held, c)
				var enabled int
				if err := c.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&enabled); err != nil {
					t.Fatal(err)
				}
				if enabled != 1 {
					t.Errorf("reopen=%d connection=%d foreign_keys=%d, want 1", pass, i, enabled)
				}
				if _, err := c.ExecContext(context.Background(), "INSERT INTO operation_results(history_id,data) VALUES (?,?)", 99999, "orphan"); err == nil {
					t.Errorf("reopen=%d connection=%d accepted orphan", pass, i)
				}
			}
		}()
	}
}
