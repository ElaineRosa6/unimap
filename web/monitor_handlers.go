package web

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xuri/excelize/v2"
)

// 预编译正则表达式，避免每次调用时重新编译
var reURLPattern = regexp.MustCompile(`^(https?://)?([\w.-]+)(:\d+)?(/.*)?$`)

// allowedUploadMIME maps file extensions to their expected MIME types
var allowedUploadMIME = map[string][]string{
	".xlsx": {"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "application/zip", "application/octet-stream"},
	".xls":  {"application/vnd.ms-excel", "application/octet-stream"},
	".csv":  {"text/csv", "application/csv", "text/plain", "text/comma-separated-values", "application/octet-stream"},
	".txt":  {"text/plain", "application/octet-stream"},
}

// sanitizeFilename removes path components and dangerous characters from filenames
func sanitizeFilename(name string) string {
	// Strip directory components
	name = filepath.Base(name)
	// Replace path separators and null bytes
	name = strings.ReplaceAll(name, "\x00", "")
	// Only allow: alphanumeric, dots, hyphens, underscores
	sanitized := strings.Builder{}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			sanitized.WriteRune(r)
		}
	}
	return sanitized.String()
}

// validateUploadMIME checks the file's Content-Type against allowed types for the extension
func validateUploadMIME(filename string, headerContentType string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	allowed, ok := allowedUploadMIME[ext]
	if !ok {
		return fmt.Errorf("unsupported file format: %s", ext)
	}
	if headerContentType == "" {
		return nil // No MIME header provided, skip validation
	}
	ct := strings.ToLower(strings.TrimSpace(headerContentType))
	// Strip parameters (e.g. "text/csv; charset=utf-8" → "text/csv")
	if idx := strings.IndexByte(ct, ';'); idx != -1 {
		ct = ct[:idx]
	}
	for _, a := range allowed {
		if ct == a {
			return nil
		}
	}
	return fmt.Errorf("mime type mismatch: got %q, expected one of %v for extension %s", ct, allowed, ext)
}

// handleScreenshot 处理截图请求
func (s *Server) handleMonitorPage(w http.ResponseWriter, r *http.Request) {
	if !s.renderTemplateWithNonce(r, w, http.StatusInternalServerError, "monitor.html", map[string]interface{}{
		"staticVersion": s.staticVersion,
	}) {
		return
	}
}

func (s *Server) handlePortScanPage(w http.ResponseWriter, r *http.Request) {
	if !s.renderTemplateWithNonce(r, w, http.StatusInternalServerError, "port-scan.html", map[string]interface{}{
		"staticVersion": s.staticVersion,
	}) {
		return
	}
}

// handleImportURLs 处理URL文件导入
func (s *Server) handleImportURLs(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}

	// 解析multipart表单
	maxMultipartMemory := int64(10 << 20)
	if s.config != nil && s.config.Web.RequestLimits.MaxMultipartMemory > 0 {
		maxMultipartMemory = s.config.Web.RequestLimits.MaxMultipartMemory
	}
	err := r.ParseMultipartForm(maxMultipartMemory)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_multipart_form", "failed to parse form", err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "file_required", "failed to get file", err.Error())
		return
	}
	defer file.Close()

	safeName := sanitizeFilename(header.Filename)
	if safeName == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_filename", "filename contains no valid characters", nil)
		return
	}

	if validateErr := validateUploadMIME(safeName, header.Header.Get("Content-Type")); validateErr != nil {
		writeAPIError(w, http.StatusBadRequest, "mime_mismatch", validateErr.Error(), nil)
		return
	}

	fileName := strings.ToLower(safeName)
	var urls []string

	if strings.HasSuffix(fileName, ".xlsx") || strings.HasSuffix(fileName, ".xls") {
		// 解析Excel文件
		urls, err = parseExcelFile(file)
	} else if strings.HasSuffix(fileName, ".csv") {
		// 解析CSV文件
		urls, err = parseCSVFile(file)
	} else if strings.HasSuffix(fileName, ".txt") {
		// 解析TXT文件
		urls, err = parseTXTFile(file)
	} else {
		writeAPIError(w, http.StatusBadRequest, "unsupported_file_format", "unsupported file format", nil)
		return
	}

	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "file_parse_failed", "failed to parse file", sanitizeError(err.Error()))
		return
	}

	// 过滤有效URL
	validUrls := filterValidURLs(urls)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":    len(urls),
		"valid":    len(validUrls),
		"urls":     validUrls,
		"filename": header.Filename,
	})
}

func (s *Server) handleURLReachability(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}

	var req struct {
		URLs        []string `json:"urls"`
		Concurrency int      `json:"concurrency"`
	}

	if !decodeJSONBody(w, r, &req) {
		return
	}

	if len(req.URLs) == 0 {
		writeAPIError(w, http.StatusBadRequest, "no_urls_provided", "no URLs provided", nil)
		return
	}

	if s.monitorApp == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "monitor_service_unavailable", "monitor app service not initialized", nil)
		return
	}

	// 检查所有URL是否指向内网地址
	for _, urlStr := range req.URLs {
		parsed, err := url.Parse(urlStr)
		if err != nil {
			continue
		}
		if isPrivateOrInternalIP(parsed.Hostname()) {
			writeAPIError(w, http.StatusForbidden, "blocked_url", "target url resolves to private/internal address", nil)
			return
		}
	}

	response, err := s.monitorApp.CheckURLReachability(r.Context(), req.URLs, req.Concurrency)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "reachability_check_failed", "url reachability check failed", sanitizeError(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"summary": response.Summary,
		"results": response.Results,
	})
}

func (s *Server) handleURLPortScan(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if s.monitorApp == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "monitor_not_available", "monitor service not available", nil)
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}

	var req struct {
		URLs        []string `json:"urls"`
		Ports       []int    `json:"ports"`
		Concurrency int      `json:"concurrency"`
	}

	if !decodeJSONBody(w, r, &req) {
		return
	}

	if len(req.URLs) == 0 {
		writeAPIError(w, http.StatusBadRequest, "no_urls_provided", "no URLs provided", nil)
		return
	}

	// 检查所有URL是否指向内网地址
	for _, urlStr := range req.URLs {
		parsed, err := url.Parse(urlStr)
		if err != nil {
			continue
		}
		if isPrivateOrInternalIP(parsed.Hostname()) {
			writeAPIError(w, http.StatusForbidden, "blocked_url", "target url resolves to private/internal address", nil)
			return
		}
	}

	response, err := s.monitorApp.ScanURLPorts(r.Context(), req.URLs, req.Ports, req.Concurrency)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "url_port_scan_failed", "url port scan failed", sanitizeError(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"summary": response.Summary,
		"ports":   response.Ports,
		"results": response.Results,
	})
}

// parseExcelFile 解析Excel文件
func parseExcelFile(file io.Reader) ([]string, error) {
	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// 获取第一个工作表
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("no sheet found")
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	var urls []string
	for i, row := range rows {
		if i == 0 {
			// 跳过表头
			continue
		}
		if len(row) > 0 && row[0] != "" {
			urls = append(urls, strings.TrimSpace(row[0]))
		}
	}

	return urls, nil
}

// parseCSVFile 解析CSV文件
func parseCSVFile(file io.Reader) ([]string, error) {
	reader := csv.NewReader(file)
	var urls []string
	isFirstRow := true

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if isFirstRow {
			isFirstRow = false
			// 检查是否是表头
			if len(record) > 0 && (strings.ToLower(record[0]) == "url" ||
				strings.ToLower(record[0]) == "address" ||
				strings.ToLower(record[0]) == "网址") {
				continue
			}
		}

		if len(record) > 0 && record[0] != "" {
			urls = append(urls, strings.TrimSpace(record[0]))
		}
	}

	return urls, nil
}

// parseTXTFile 解析TXT文件
func parseTXTFile(file io.Reader) ([]string, error) {
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var urls []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			urls = append(urls, line)
		}
	}

	return urls, nil
}

// filterValidURLs 过滤有效URL
func filterValidURLs(urls []string) []string {
	var valid []string
	seen := make(map[string]bool)

	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" || seen[u] {
			continue
		}

		// 简单URL验证（使用预编译正则）
		if reURLPattern.MatchString(u) {
			valid = append(valid, u)
			seen[u] = true
		}
	}

	return valid
}
