package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/unimap/project/internal/history"
)

// HistorySaveRequest is the request body for saving history.
type HistorySaveRequest struct {
	OperationType string        `json:"operation_type"`
	Input         interface{}   `json:"input"`
	Status        string        `json:"status"`
	TotalCount    int           `json:"total_count"`
	Summary       interface{}   `json:"summary,omitempty"`
	DurationMS    int64         `json:"duration_ms,omitempty"`
	Results       []interface{} `json:"results,omitempty"`
}

// handleHistorySave handles POST /api/v1/history/save
func (s *Server) handleHistorySave(w http.ResponseWriter, r *http.Request) {
	if s.historyRepo == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "history_disabled", "history is not enabled", nil)
		return
	}
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required", nil)
		return
	}

	var req HistorySaveRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.OperationType == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_type", "operation_type is required", nil)
		return
	}

	inputJSON, _ := json.Marshal(req.Input)
	summaryJSON, _ := json.Marshal(req.Summary)

	h := &history.OperationHistory{
		OperationType: history.OperationType(req.OperationType),
		Input:         string(inputJSON),
		Status:        req.Status,
		TotalCount:    req.TotalCount,
		Summary:       string(summaryJSON),
		DurationMS:    req.DurationMS,
	}

	id, err := s.historyRepo.CreateHistory(h)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "db_error", err.Error(), nil)
		return
	}

	maxResults := 1000
	if s.config != nil && s.config.History.MaxResults > 0 {
		maxResults = s.config.History.MaxResults
	}
	if len(req.Results) > maxResults {
		req.Results = req.Results[:maxResults]
	}
	if len(req.Results) > 0 {
		results := make([]history.OperationResult, len(req.Results))
		for i, r := range req.Results {
			data, _ := json.Marshal(r)
			results[i] = history.OperationResult{Data: string(data)}
		}
		if err := s.historyRepo.CreateResults(id, results); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "db_error", err.Error(), nil)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
	})
}

// handleHistoryListOrClear handles GET/DELETE /api/v1/history
func (s *Server) handleHistoryListOrClear(w http.ResponseWriter, r *http.Request) {
	if s.historyRepo == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "history_disabled", "history is not enabled", nil)
		return
	}

	switch r.Method {
	case http.MethodGet:
		opType := r.URL.Query().Get("type")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if limit <= 0 {
			limit = 20
		}

		items, total, err := s.historyRepo.ListHistory(opType, limit, offset)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "db_error", err.Error(), nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"total":   total,
			"items":   items,
		})

	case http.MethodDelete:
		opType := r.URL.Query().Get("type")
		if err := s.historyRepo.ClearHistory(opType); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "db_error", err.Error(), nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})

	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET or DELETE required", nil)
	}
}

// handleHistoryGetOrDelete handles GET/DELETE /api/v1/history/{id}
func (s *Server) handleHistoryGetOrDelete(w http.ResponseWriter, r *http.Request) {
	if s.historyRepo == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "history_disabled", "history is not enabled", nil)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/history/")
	if idStr == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_id", "history ID is required", nil)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_id", "invalid history ID", nil)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h, err := s.historyRepo.GetHistory(id)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "db_error", err.Error(), nil)
			return
		}
		if h == nil {
			writeAPIError(w, http.StatusNotFound, "not_found", "history not found", nil)
			return
		}
		results, err := s.historyRepo.GetResults(id)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "db_error", err.Error(), nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"history": h,
			"results": results,
		})

	case http.MethodDelete:
		if err := s.historyRepo.DeleteHistory(id); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "db_error", err.Error(), nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})

	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET or DELETE required", nil)
	}
}
