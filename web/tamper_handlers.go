package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/unimap/project/internal/exporter"
	"github.com/unimap/project/internal/service"
	"github.com/unimap/project/internal/tamper"
)

func (s *Server) tamperRuntimeConfig() service.TamperRuntimeConfig {
	cfg := service.TamperRuntimeConfig{}
	if current := s.currentConfig(); current != nil {
		cfg.PortScanEnabled = current.Tamper.PortScanEnabled
		cfg.InsecureSkipVerify = current.Tamper.InsecureSkipVerify
		if current.Tamper.PortScanTimeoutMs > 0 {
			cfg.PortScanTimeout = time.Duration(current.Tamper.PortScanTimeoutMs) * time.Millisecond
		}
	}
	return cfg
}

func (s *Server) handleTamperHistoryExport(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	limit := 10000
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 && value < limit {
			limit = value
		}
	}
	filter, err := tamperHistoryFilterFromRequest(r, limit, 0)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_time_range", err.Error(), nil)
		return
	}
	result, err := s.tamperApp.QueryHistory(filter)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "export_history_failed", "export history failed", nil)
		return
	}
	items := make([]exporter.TamperHistoryExportResult, 0, len(result.Records))
	for _, record := range result.Records {
		items = append(items, exporter.TamperHistoryExportResult{ID: record.ID, URL: record.URL, Status: record.Status, DetectionMode: record.DetectionMode, Tampered: record.Tampered, TamperedSegments: record.TamperedSegments, ChangesCount: record.ChangesCount, CheckTime: time.Unix(record.Timestamp, 0).Format(time.RFC3339)})
	}
	var body bytes.Buffer
	if err := exporter.ExportTamperHistoryJSON(&body, items); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "export_history_failed", "export history failed", nil)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=tamper-history.json")
	_, _ = w.Write(body.Bytes())
}

func (s *Server) tamperPageLoader(proxy string) service.TamperPageLoader {
	if s.screenshotMgr == nil {
		return nil
	}
	if strings.TrimSpace(proxy) == "" {
		return s.screenshotMgr
	}
	return tamper.BrowserPageLoaderFunc(func(ctx context.Context, targetURL string) (string, string, error) {
		return s.screenshotMgr.LoadPageWithProxy(ctx, targetURL, proxy)
	})
}

// handleTamperCheck 处理篡改检测请求
func (s *Server) handleTamperCheck(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !requireTrustedRequest(w, r, s.allowedOrigins()) {
		return
	}

	var req struct {
		URLs        []string `json:"urls"`
		Concurrency int      `json:"concurrency"`
		Mode        string   `json:"mode"`
	}

	if !decodeJSONBody(w, r, &req) {
		return
	}

	// Limit concurrency and URL count to prevent resource exhaustion
	const maxTamperConcurrency = 20
	const maxTamperURLs = 500
	if len(req.URLs) > maxTamperURLs {
		writeAPIError(w, http.StatusBadRequest, "too_many_urls", fmt.Sprintf("maximum %d URLs allowed", maxTamperURLs), nil)
		return
	}
	if req.Concurrency <= 0 || req.Concurrency > maxTamperConcurrency {
		req.Concurrency = maxTamperConcurrency
	}

	// SSRF: reject URLs that resolve to private/internal addresses
	for _, urlStr := range req.URLs {
		parsed, err := url.Parse(urlStr)
		if err != nil {
			continue
		}
		if isPrivateOrInternalIP(parsed.Hostname()) {
			writeAPIError(w, http.StatusForbidden, "blocked_url",
				"url resolves to private/internal address", nil)
			return
		}
	}

	proxy := s.selectRequestProxy()
	resp, err := s.tamperApp.Check(r.Context(), service.TamperCheckRequest{
		URLs:        req.URLs,
		Concurrency: req.Concurrency,
		Mode:        req.Mode,
	}, s.tamperPageLoader(proxy))
	if err != nil {
		s.reportRequestProxy(proxy, false)
		if strings.Contains(strings.ToLower(err.Error()), "no urls") {
			writeAPIError(w, http.StatusBadRequest, "no_urls_provided", "no URLs provided", nil)
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "tamper_check_failed", "tamper check failed", sanitizeError(err.Error()))
		return
	}
	s.reportRequestProxy(proxy, true)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"mode":    resp.Mode,
		"summary": resp.Summary,
		"results": resp.Results,
	})
}

// handleTamperBaseline 处理基线设置请求
func (s *Server) handleTamperBaseline(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !requireTrustedRequest(w, r, s.allowedOrigins()) {
		return
	}

	var req struct {
		URLs        []string `json:"urls"`
		Concurrency int      `json:"concurrency"`
	}

	if !decodeJSONBody(w, r, &req) {
		return
	}

	// Limit concurrency and URL count to prevent resource exhaustion
	const maxTamperConcurrency = 20
	const maxTamperURLs = 500
	if len(req.URLs) > maxTamperURLs {
		writeAPIError(w, http.StatusBadRequest, "too_many_urls", fmt.Sprintf("maximum %d URLs allowed", maxTamperURLs), nil)
		return
	}
	if req.Concurrency <= 0 || req.Concurrency > maxTamperConcurrency {
		req.Concurrency = maxTamperConcurrency
	}

	// SSRF: reject URLs that resolve to private/internal addresses
	for _, urlStr := range req.URLs {
		parsed, err := url.Parse(urlStr)
		if err != nil {
			continue
		}
		if isPrivateOrInternalIP(parsed.Hostname()) {
			writeAPIError(w, http.StatusForbidden, "blocked_url",
				"url resolves to private/internal address", nil)
			return
		}
	}

	proxy := s.selectRequestProxy()
	resp, err := s.tamperApp.SetBaseline(r.Context(), service.TamperBaselineRequest{
		URLs:        req.URLs,
		Concurrency: req.Concurrency,
	}, s.tamperPageLoader(proxy))
	if err != nil {
		s.reportRequestProxy(proxy, false)
		if strings.Contains(strings.ToLower(err.Error()), "no urls") {
			writeAPIError(w, http.StatusBadRequest, "no_urls_provided", "no URLs provided", nil)
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "set_baseline_failed", "set baseline failed", sanitizeError(err.Error()))
		return
	}
	s.reportRequestProxy(proxy, true)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"summary": resp.Summary,
		"results": resp.Results,
	})
}

// handleTamperBaselineList 处理基线列表请求
func (s *Server) handleTamperBaselineList(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	urls, err := s.tamperApp.ListBaselines()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "list_baselines_failed", "list baselines failed", sanitizeError(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"urls":    urls,
		"count":   len(urls),
	})
}

// handleTamperBaselineDelete 处理删除基线请求
func (s *Server) handleTamperBaselineDelete(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodDelete) {
		return
	}
	if !requireTrustedRequest(w, r, s.allowedOrigins()) {
		return
	}

	urlValue := strings.TrimSpace(r.URL.Query().Get("url"))
	if urlValue == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_url", "URL is required", nil)
		return
	}

	if err := s.tamperApp.DeleteBaseline(urlValue); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "delete_baseline_failed", "delete baseline failed", sanitizeError(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Baseline for %s deleted", urlValue),
		"url":     urlValue,
	})
}

// handleTamperHistory 处理检测历史记录请求
func (s *Server) handleTamperHistory(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	limit := 200
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		if v, err := strconv.Atoi(rawLimit); err == nil && v > 0 {
			if v > 1000 {
				v = 1000
			}
			limit = v
		}
	}
	offset := 0
	if rawOffset := strings.TrimSpace(r.URL.Query().Get("offset")); rawOffset != "" {
		if v, err := strconv.Atoi(rawOffset); err == nil && v > 0 {
			const maxTamperHistoryOffset = 100000
			if v > maxTamperHistoryOffset {
				v = maxTamperHistoryOffset
			}
			offset = v
		}
	}

	filter, err := tamperHistoryFilterFromRequest(r, limit, offset)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_time_range", err.Error(), nil)
		return
	}

	result, err := s.tamperApp.QueryHistory(filter)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "list_history_failed", "list history failed", sanitizeError(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   result.Count,
		"records": result.Records,
		"urls":    result.URLOptions,
	})
}

func tamperHistoryFilterFromRequest(r *http.Request, limit, offset int) (service.HistoryFilter, error) {
	startTime, err := parseTamperHistoryTime(strings.TrimSpace(r.URL.Query().Get("start_time")))
	if err != nil {
		return service.HistoryFilter{}, fmt.Errorf("invalid start_time: %w", err)
	}
	endTime, err := parseTamperHistoryTime(strings.TrimSpace(r.URL.Query().Get("end_time")))
	if err != nil {
		return service.HistoryFilter{}, fmt.Errorf("invalid end_time: %w", err)
	}
	if startTime > 0 && endTime > 0 && startTime > endTime {
		return service.HistoryFilter{}, fmt.Errorf("start_time must not be later than end_time")
	}
	return service.HistoryFilter{
		URLFilter:   strings.TrimSpace(r.URL.Query().Get("url")),
		TypeFilter:  strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type"))),
		ModeFilter:  strings.ToLower(strings.TrimSpace(r.URL.Query().Get("mode"))),
		QueryFilter: strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q"))),
		StartTime:   startTime,
		EndTime:     endTime,
		Limit:       limit,
		Offset:      offset,
	}, nil
}

func parseTamperHistoryTime(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	if unixTime, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if unixTime <= 0 {
			return 0, fmt.Errorf("must be a positive Unix timestamp")
		}
		return unixTime, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return 0, fmt.Errorf("must be Unix seconds or RFC3339")
	}
	unixTime := parsed.Unix()
	if unixTime <= 0 {
		return 0, fmt.Errorf("must be later than 1970-01-01T00:00:00Z")
	}
	return unixTime, nil
}

// handleTamperHistoryDelete 处理删除指定URL的检测历史请求
func (s *Server) handleTamperHistoryDelete(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodDelete) {
		return
	}
	if !requireTrustedRequest(w, r, s.allowedOrigins()) {
		return
	}

	urlValue := strings.TrimSpace(r.URL.Query().Get("url"))
	if urlValue == "" {
		writeAPIError(w, http.StatusBadRequest, "missing_url", "URL is required", nil)
		return
	}

	if err := s.tamperApp.DeleteCheckRecords(urlValue); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "delete_history_failed", "delete history failed", sanitizeError(err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"url":     urlValue,
	})
}
