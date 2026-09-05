package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHistorySaveAtomicResultLimit(t *testing.T) {
	for _, count := range []int{0, 1, 1001} {
		s := setupTestServer(t)
		results := make([]interface{}, count)
		for i := range results {
			results[i] = i
		}
		body, err := json.Marshal(HistorySaveRequest{OperationType: "query", Status: "success", Results: results})
		if err != nil {
			t.Fatal(err)
		}
		req := withAdminContext(httptest.NewRequest(http.MethodPost, "/api/v1/history/save", bytes.NewReader(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.handleHistorySave(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("count=%d status=%d", count, w.Code)
		}
		items, total, err := s.historyRepo.ListHistory("", 20, 0)
		if err != nil || total != 1 || len(items) != 1 {
			t.Fatalf("history total=%d items=%d err=%v", total, len(items), err)
		}
		stored, err := s.historyRepo.GetResults(items[0].ID)
		want := count
		if want > 1000 {
			want = 1000
		}
		if err != nil || len(stored) != want {
			t.Fatalf("results=%d want=%d err=%v", len(stored), want, err)
		}
	}
}

func TestHistorySaveAtomicFailureAndRetry(t *testing.T) {
	s := setupTestServer(t)
	// Fail on the second result, after a first result has been inserted.
	_, err := s.historyDB.DB().Exec(`CREATE TRIGGER reject_result BEFORE INSERT ON operation_results
		WHEN NEW.data = '"reject"' BEGIN SELECT RAISE(ABORT, 'fixture result failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	save := func() *httptest.ResponseRecorder {
		req := withAdminContext(httptest.NewRequest(http.MethodPost, "/api/v1/history/save",
			bytes.NewBufferString(`{"operation_type":"query","input":"fixture","status":"success","results":["first","reject"]}`)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.handleHistorySave(w, req)
		return w
	}
	w := save()
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	assertCounts := func(wantHistory, wantResults int) {
		t.Helper()
		for _, tc := range []struct {
			table string
			want  int
		}{{"operation_history", wantHistory}, {"operation_results", wantResults}} {
			var n int
			if err := s.historyDB.DB().QueryRow("SELECT COUNT(*) FROM " + tc.table).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != tc.want {
				t.Errorf("%s count=%d, want %d", tc.table, n, tc.want)
			}
		}
	}
	assertCounts(0, 0)
	if _, err := s.historyDB.DB().Exec("DROP TRIGGER reject_result"); err != nil {
		t.Fatal(err)
	}
	w = save()
	if w.Code != http.StatusOK {
		t.Fatalf("retry status=%d body=%s", w.Code, w.Body.String())
	}
	assertCounts(1, 2)
}
