package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/unimap/project/internal/history"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test_history.db")
	db, err := history.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		os.Remove(dbPath)
	})
	if err := db.InitSchema(); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}
	return &Server{
		historyRepo: history.NewRepository(db.DB()),
		historyDB:   db,
	}
}

func TestHandleHistorySave(t *testing.T) {
	s := setupTestServer(t)

	body, _ := json.Marshal(HistorySaveRequest{
		OperationType: "query",
		Input:         map[string]interface{}{"query": "port=80", "engines": []string{"fofa"}},
		Status:        "success",
		TotalCount:    10,
		Summary:       map[string]interface{}{"fofa": 10},
		Results:       []interface{}{map[string]interface{}{"ip": "1.1.1.1", "port": 80}},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/history/save", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleHistorySave(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleHistoryList(t *testing.T) {
	s := setupTestServer(t)

	body, _ := json.Marshal(HistorySaveRequest{
		OperationType: "query",
		Input:         "test",
		Status:        "success",
	})
	saveReq := httptest.NewRequest(http.MethodPost, "/api/v1/history/save", bytes.NewReader(body))
	saveReq.Header.Set("Content-Type", "application/json")
	saveW := httptest.NewRecorder()
	s.handleHistorySave(saveW, saveReq)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/history", nil)
	listW := httptest.NewRecorder()
	s.handleHistoryListOrClear(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listW.Code)
	}
}

func TestHandleHistoryGet(t *testing.T) {
	s := setupTestServer(t)

	body, _ := json.Marshal(HistorySaveRequest{
		OperationType: "query",
		Input:         "test",
		Status:        "success",
		Results:       []interface{}{map[string]interface{}{"ip": "1.1.1.1"}},
	})
	saveReq := httptest.NewRequest(http.MethodPost, "/api/v1/history/save", bytes.NewReader(body))
	saveReq.Header.Set("Content-Type", "application/json")
	saveW := httptest.NewRecorder()
	s.handleHistorySave(saveW, saveReq)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/history/1", nil)
	getW := httptest.NewRecorder()
	s.handleHistoryGetOrDelete(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", getW.Code, getW.Body.String())
	}
}

func TestHandleHistoryDelete(t *testing.T) {
	s := setupTestServer(t)

	body, _ := json.Marshal(HistorySaveRequest{
		OperationType: "query",
		Input:         "test",
		Status:        "success",
	})
	saveReq := httptest.NewRequest(http.MethodPost, "/api/v1/history/save", bytes.NewReader(body))
	saveReq.Header.Set("Content-Type", "application/json")
	saveW := httptest.NewRecorder()
	s.handleHistorySave(saveW, saveReq)

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/history/1", nil)
	delW := httptest.NewRecorder()
	s.handleHistoryGetOrDelete(delW, delReq)

	if delW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", delW.Code)
	}
}
