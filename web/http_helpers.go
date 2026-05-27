package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/unimap/project/internal/logger"
)

// Chrome extension origin restriction state.
// SetAllowedExtensionIDs must be called during server startup before serving requests.
// If not called (e.g., in tests), all chrome-extension:// origins are allowed for backward compatibility.
var (
	extIDMu     sync.RWMutex
	extIDSet    map[string]struct{} // non-nil + non-empty = restrict to these IDs
	extIDLoaded bool
)

// SetAllowedExtensionIDs configures which chrome-extension:// origin IDs are permitted.
// Pass nil or an empty slice to allow all extensions (backward-compatible default).
func SetAllowedExtensionIDs(ids []string) {
	m := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			m[id] = struct{}{}
		}
	}
	extIDMu.Lock()
	extIDSet = m
	extIDLoaded = true
	extIDMu.Unlock()
	if len(m) == 0 {
		logger.Warn("web.cors.allowed_extension_ids is empty; all chrome-extension:// origins will be allowed (backward compatibility). Configure allowed_extension_ids to restrict.")
	} else {
		snapshot := make([]string, 0, len(m))
		for k := range m {
			snapshot = append(snapshot, k)
		}
		logger.Infof("Chrome extension origin restriction enabled, allowed IDs: %v", snapshot)
	}
}

// extractExtensionID parses the extension ID from a chrome-extension:// origin.
// Returns "" if the origin is not a valid chrome-extension:// URL.
func extractExtensionID(origin string) string {
	const prefix = "chrome-extension://"
	if !strings.HasPrefix(origin, prefix) {
		return ""
	}
	rest := origin[len(prefix):]
	if idx := strings.Index(rest, "/"); idx != -1 {
		rest = rest[:idx]
	}
	return strings.TrimSpace(rest)
}

// isChromeExtensionAllowed checks whether a chrome-extension:// origin is permitted.
// Returns true if no restriction is configured (backward compatible).
func isChromeExtensionAllowed(origin string) bool {
	extIDMu.RLock()
	loaded := extIDLoaded
	ids := extIDSet
	extIDMu.RUnlock()

	if !loaded || len(ids) == 0 {
		return true // not configured or empty — allow all (backward compat)
	}
	id := extractExtensionID(origin)
	if id == "" {
		return false
	}
	_, allowed := ids[id]
	return allowed
}

type apiErrorPayload struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type apiErrorResponse struct {
	Success bool            `json:"success"`
	Error   apiErrorPayload `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logger.Errorf("failed to encode JSON response: %v", err)
	}
}

func writeAPIError(w http.ResponseWriter, status int, code, message string, details interface{}) {
	writeJSON(w, status, apiErrorResponse{
		Success: false,
		Error: apiErrorPayload{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", map[string]string{"expected": method})
		return false
	}
	return true
}

func requireTrustedRequest(w http.ResponseWriter, r *http.Request, allowedOrigins []string) bool {
	if !isTrustedRequest(r, allowedOrigins) {
		writeAPIError(w, http.StatusForbidden, "forbidden_origin", "origin not allowed", nil)
		return false
	}
	return true
}

// decodeJSONReader decodes JSON from an io.Reader with strict validation:
// unknown fields are rejected, trailing garbage is rejected.
// Returns the raw error; callers should classify and report it appropriately.
func decodeJSONReader(r io.Reader, dst interface{}) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("request body must contain only one JSON object")
	}
	return nil
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	if err := decodeJSONReader(r.Body, dst); err != nil {
		if errors.Is(err, io.EOF) {
			writeAPIError(w, http.StatusBadRequest, "invalid_request_body", "request body is required", nil)
			return false
		}
		if strings.Contains(strings.ToLower(err.Error()), "request body too large") {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds configured limit", nil)
			return false
		}
		writeAPIError(w, http.StatusBadRequest, "invalid_request_body", "invalid JSON request body", err.Error())
		return false
	}
	return true
}

func isSameHostURL(rawURL string, host string) bool {
	if strings.TrimSpace(rawURL) == "" {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, host)
}

func normalizeOrigin(origin string) string {
	u, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.ToLower(strings.TrimRight(u.String(), "/"))
}

func originAllowedByList(origin string, allowedOrigins []string) bool {
	normalized := normalizeOrigin(origin)
	if normalized == "" {
		return false
	}
	for _, allowed := range allowedOrigins {
		allowed = strings.TrimSpace(allowed)
		if allowed == "*" {
			return true
		}
		if normalizeOrigin(allowed) == normalized {
			return true
		}
	}
	return false
}

func isOriginAllowed(origin, host string, allowedOrigins []string) bool {
	if strings.TrimSpace(origin) == "" {
		return false
	}
	if isSameHostURL(origin, host) {
		return true
	}
	// Allow Chrome extension origins only if the extension ID is permitted.
	if strings.HasPrefix(origin, "chrome-extension://") {
		return isChromeExtensionAllowed(origin)
	}
	return originAllowedByList(origin, allowedOrigins)
}

func isTrustedRequest(r *http.Request, allowedOrigins []string) bool {
	origin := r.Header.Get("Origin")
	referer := r.Header.Get("Referer")

	// 对状态变更操作（POST, PUT, PATCH, DELETE）要求必须有 Origin 或 Referer
	isStateChange := r.Method == http.MethodPost ||
		r.Method == http.MethodPut ||
		r.Method == http.MethodPatch ||
		r.Method == http.MethodDelete

	if isStateChange && strings.TrimSpace(origin) == "" && strings.TrimSpace(referer) == "" {
		return false
	}

	if strings.TrimSpace(origin) == "" && strings.TrimSpace(referer) == "" {
		// Keep compatibility for non-browser clients.
		return true
	}
	if isOriginAllowed(origin, r.Host, allowedOrigins) {
		return true
	}
	return isOriginAllowed(referer, r.Host, allowedOrigins)
}

func requestSizeLimitMiddleware(maxBodyBytes int64) func(http.Handler) http.Handler {
	if maxBodyBytes <= 0 {
		maxBodyBytes = 10 * 1024 * 1024
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			isWebSocket := strings.Contains(r.Header.Get("Connection"), "Upgrade") &&
				strings.EqualFold(r.Header.Get("Upgrade"), "websocket")

			if !isWebSocket {
				switch r.Method {
				case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
					if r.ContentLength > maxBodyBytes {
						writeAPIError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds configured limit", map[string]string{"max_body_bytes": strconv.FormatInt(maxBodyBytes, 10)})
						return
					}
					r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isPrivateOrInternalHost 检查主机名是否为私有/回环/内部地址，对域名做 DNS 解析
func isPrivateOrInternalHost(ctx context.Context, host string) bool {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" {
		return false
	}
	// 去除端口
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	// 检查常见主机名
	lower := strings.ToLower(host)
	if lower == "localhost" || lower == "127.0.0.1" || lower == "::1" || lower == "0.0.0.0" {
		return true
	}
	// 如果是字面 IP，直接检查
	if ip := net.ParseIP(host); ip != nil {
		return isBlockedIP(ip)
	}
	// 对域名做 DNS 解析，所有解析结果都必须是公网 IP
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return true // DNS 解析失败视为不安全
	}
	for _, addr := range ips {
		if isBlockedIP(addr.IP) {
			return true
		}
	}
	return false
}

func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

// isPrivateOrInternalIP 兼容旧接口，内部委托给 isPrivateOrInternalHost
func isPrivateOrInternalIP(host string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return isPrivateOrInternalHost(ctx, host)
}

func corsMiddleware(allowedOrigins, allowedMethods, allowedHeaders, exposedHeaders []string, allowCredentials bool, maxAge int) func(http.Handler) http.Handler {
	if len(allowedMethods) == 0 {
		allowedMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	}
	if len(allowedHeaders) == 0 {
		allowedHeaders = []string{"Content-Type", "Authorization", "X-Requested-With", "X-WebSocket-Token", "X-Request-Id"}
	}
	if maxAge < 0 {
		maxAge = 0
	}

	methodHeader := strings.Join(allowedMethods, ", ")
	headerHeader := strings.Join(allowedHeaders, ", ")
	exposedHeader := strings.Join(exposedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Bridge routes have their own auth (loopback + bearer token),
			// skip CORS restrictions for them (needed for browser extensions).
			if isScreenshotBridgePath(r.URL.Path) {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", methodHeader)
				w.Header().Set("Access-Control-Allow-Headers", headerHeader)
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin != "" && isOriginAllowed(origin, r.Host, allowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				if allowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				w.Header().Set("Vary", "Origin")
				if exposedHeader != "" {
					w.Header().Set("Access-Control-Expose-Headers", exposedHeader)
				}
			}

			if r.Method == http.MethodOptions {
				if origin == "" || !isOriginAllowed(origin, r.Host, allowedOrigins) {
					writeAPIError(w, http.StatusForbidden, "forbidden_origin", "origin not allowed", nil)
					return
				}
				// Ensure CORS headers are set on the preflight response itself
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", methodHeader)
				w.Header().Set("Access-Control-Allow-Headers", headerHeader)
				if allowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				w.Header().Set("Vary", "Origin")
				if maxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", strconv.Itoa(maxAge))
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// sanitizeError removes internal details from error messages before returning to client
func sanitizeError(err string) string {
	if err == "" {
		return ""
	}
	// Strip common internal patterns: file paths, stack traces, connection strings
	sanitized := err
	// Remove Go stack trace patterns
	if idx := strings.Index(sanitized, "\ngoroutine "); idx != -1 {
		sanitized = sanitized[:idx]
	}
	if idx := strings.Index(sanitized, "\nruntime."); idx != -1 {
		sanitized = sanitized[:idx]
	}
	// Truncate very long errors
	if len(sanitized) > 500 {
		sanitized = sanitized[:500] + "..."
	}
	return sanitized
}
