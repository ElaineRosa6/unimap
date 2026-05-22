package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/unimap-icp-hunter/project/internal/adapter"
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
