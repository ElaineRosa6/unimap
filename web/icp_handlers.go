package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/unimap/project/internal/adapter"
	icpdb "github.com/unimap/project/internal/icp/database"
)

// handleICPPage renders the ICP query page (GET /icp).
func (s *Server) handleICPPage(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	types := make([]map[string]string, 0, len(adapter.AllICPQueryTypes()))
	for _, t := range adapter.AllICPQueryTypes() {
		types = append(types, map[string]string{
			"value": string(t),
			"label": adapter.ICPTypeLabel(t),
		})
	}

	defaultType := "web"
	icpEnabled := false
	if s.config != nil {
		s.configMutex.Lock()
		if v := strings.TrimSpace(s.config.ICP.DefaultType); v != "" {
			defaultType = v
		}
		icpEnabled = s.config.ICP.Enabled
		s.configMutex.Unlock()
	}

	if !s.renderTemplateWithNonce(r, w, http.StatusInternalServerError, "icp-page", map[string]interface{}{
		"types":         types,
		"defaultType":   defaultType,
		"icpEnabled":    icpEnabled,
		"staticVersion": s.staticVersion,
	}) {
		return
	}
}

// handleICPQuery handles GET /api/icp/query?type=web&search=xxx&page=1&page_size=20.
func (s *Server) handleICPQuery(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	if s.config == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "config_not_loaded", "config not loaded", nil)
		return
	}

	s.configMutex.Lock()
	enabled := s.config.ICP.Enabled
	baseURL := strings.TrimSpace(s.config.ICP.BaseURL)
	apiKey := s.config.ICP.APIKey
	defaultType := s.config.ICP.DefaultType
	s.configMutex.Unlock()

	if !enabled {
		writeAPIError(w, http.StatusServiceUnavailable, "icp_disabled",
			"ICP query is disabled; enable it in settings", nil)
		return
	}
	if baseURL == "" {
		writeAPIError(w, http.StatusServiceUnavailable, "icp_not_configured",
			"ICP base_url is not configured", nil)
		return
	}

	queryType := strings.TrimSpace(r.URL.Query().Get("type"))
	if queryType == "" {
		queryType = defaultType
	}
	if !adapter.IsValidICPQueryType(queryType) {
		writeAPIError(w, http.StatusBadRequest, "invalid_type",
			"invalid ICP query type", map[string]string{"type": queryType})
		return
	}

	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if search == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_search",
			"search parameter is required", nil)
		return
	}
	if len(search) > 256 {
		writeAPIError(w, http.StatusBadRequest, "search_too_long",
			"search must be 256 chars or fewer", nil)
		return
	}

	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("page_size"), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	results, total, err := adapter.ICPSearch(baseURL, apiKey, adapter.ICPSearchRequest{
		Query:    search,
		Type:     queryType,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, "icp_query_failed",
			sanitizeError(err.Error()), nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"type":      queryType,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"results":   results,
	})
}

// parsePositiveInt parses a positive integer from a query string value; returns
// fallback on empty / invalid / non-positive input.
func parsePositiveInt(raw string, fallback int) int {
	v := strings.TrimSpace(raw)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// handleICPHistory handles GET /api/icp/history?task_id=xxx&keyword=xxx&type=web&limit=20.
func (s *Server) handleICPHistory(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if s.icpRepo == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "icp_db_not_available",
			"ICP result database is not available", nil)
		return
	}

	taskID := r.URL.Query().Get("task_id")
	keyword := r.URL.Query().Get("keyword")
	queryType := strings.TrimSpace(r.URL.Query().Get("type"))
	if queryType == "" {
		queryType = "web"
	}
	limit := parsePositiveInt(r.URL.Query().Get("limit"), 20)

	var runs []*icpdb.ICPQueryRun
	var err error

	if taskID != "" {
		runs, err = s.icpRepo.GetRunsByTaskID(taskID, limit)
	} else if keyword != "" {
		runs, err = s.icpRepo.GetRunsByKeyword(keyword, queryType, limit)
	} else {
		writeAPIError(w, http.StatusBadRequest, "missing_param",
			"task_id or keyword parameter is required", nil)
		return
	}

	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "db_error",
			"failed to query ICP history", nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"runs":    runs,
	})
}

// handleICPHistoryResults handles GET /api/icp/history/results?run_id=123.
func (s *Server) handleICPHistoryResults(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if s.icpRepo == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "icp_db_not_available",
			"ICP result database is not available", nil)
		return
	}

	runIDStr := r.URL.Query().Get("run_id")
	runID, err := strconv.ParseInt(runIDStr, 10, 64)
	if err != nil || runID <= 0 {
		writeAPIError(w, http.StatusBadRequest, "invalid_run_id",
			"run_id must be a positive integer", nil)
		return
	}

	results, err := s.icpRepo.GetResultsByRunID(runID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "db_error",
			"failed to query ICP results", nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"run_id":  runID,
		"results": results,
	})
}

// handleICPCompare handles GET /api/icp/compare?keyword=xxx&type=web.
// Returns latest and previous results for side-by-side comparison.
func (s *Server) handleICPCompare(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if s.icpRepo == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "icp_db_not_available",
			"ICP result database is not available", nil)
		return
	}

	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if keyword == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_keyword",
			"keyword parameter is required", nil)
		return
	}
	queryType := strings.TrimSpace(r.URL.Query().Get("type"))
	if queryType == "" {
		queryType = "web"
	}

	latest, err := s.icpRepo.GetLatestResults(keyword, queryType)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "db_error",
			"failed to query latest ICP results", nil)
		return
	}

	var previous []*icpdb.ICPResultRow
	if len(latest) > 0 {
		previous, err = s.icpRepo.GetPreviousResults(keyword, queryType, latest[0].FetchedAt)
		if err != nil {
			previous = nil
		}
	}

	changes := compareICPResults(latest, previous)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"keyword":  keyword,
		"type":     queryType,
		"latest":   latest,
		"previous": previous,
		"changes":  changes,
	})
}

// ICPChange represents a field-level change between two result snapshots.
type ICPChange struct {
	Domain string `json:"domain"`
	Field  string `json:"field"`
	Old    string `json:"old,omitempty"`
	New    string `json:"new,omitempty"`
}

func compareICPResults(latest, previous []*icpdb.ICPResultRow) []ICPChange {
	if len(previous) == 0 {
		return nil
	}
	prevMap := make(map[string]*icpdb.ICPResultRow, len(previous))
	for _, p := range previous {
		if p.Domain != "" {
			prevMap[p.Domain] = p
		}
	}

	var changes []ICPChange
	compareFields := []struct {
		name string
		get  func(*icpdb.ICPResultRow) string
	}{
		{"licence", func(r *icpdb.ICPResultRow) string { return r.Licence }},
		{"unit_name", func(r *icpdb.ICPResultRow) string { return r.UnitName }},
		{"update_record", func(r *icpdb.ICPResultRow) string { return r.UpdateRecord }},
		{"nature_name", func(r *icpdb.ICPResultRow) string { return r.NatureName }},
		{"main_licence", func(r *icpdb.ICPResultRow) string { return r.MainLicence }},
		{"limit_access", func(r *icpdb.ICPResultRow) string { return r.LimitAccess }},
	}

	for _, l := range latest {
		if l.Domain == "" {
			continue
		}
		p, ok := prevMap[l.Domain]
		if !ok {
			changes = append(changes, ICPChange{Domain: l.Domain, Field: "_new", Old: "", New: l.Domain})
			continue
		}
		for _, f := range compareFields {
			oldVal := f.get(p)
			newVal := f.get(l)
			if oldVal != newVal {
				changes = append(changes, ICPChange{
					Domain: l.Domain, Field: f.name, Old: oldVal, New: newVal,
				})
			}
		}
	}

	prevDomains := make(map[string]bool, len(latest))
	for _, l := range latest {
		prevDomains[l.Domain] = true
	}
	for _, p := range previous {
		if p.Domain != "" && !prevDomains[p.Domain] {
			changes = append(changes, ICPChange{Domain: p.Domain, Field: "_removed", Old: p.Domain, New: ""})
		}
	}

	return changes
}

