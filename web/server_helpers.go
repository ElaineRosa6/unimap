package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func parseEnginesParam(r *http.Request) []string {
	_ = r.ParseForm()

	seen := make(map[string]struct{})
	engines := make([]string, 0)
	for _, raw := range r.Form["engines"] {
		for _, part := range strings.Split(raw, ",") {
			engine := strings.TrimSpace(part)
			if engine == "" {
				continue
			}
			if _, ok := seen[engine]; ok {
				continue
			}
			seen[engine] = struct{}{}
			engines = append(engines, engine)
		}
	}

	return engines
}

func parseWSStringList(val interface{}) []string {
	if val == nil {
		return nil
	}

	sanitizeAndAppend := func(out []string, raw string) []string {
		for _, part := range strings.Split(raw, ",") {
			item := strings.TrimSpace(part)
			if item == "" {
				continue
			}
			out = append(out, item)
		}
		return out
	}

	switch v := val.(type) {
	case string:
		return sanitizeAndAppend(nil, v)
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = sanitizeAndAppend(out, item)
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = sanitizeAndAppend(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func parseWSInt(val interface{}, defaultValue int) int {
	if val == nil {
		return defaultValue
	}

	switch v := val.(type) {
	case float64:
		if v > 0 {
			return int(v)
		}
		return defaultValue
	case int:
		if v > 0 {
			return v
		}
		return defaultValue
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return defaultValue
		}
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
		return defaultValue
	default:
		return defaultValue
	}
}

func validateQueryInput(query string) error {
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("query cannot be empty")
	}
	if len(query) > 1000 {
		return fmt.Errorf("query is too long (maximum 1000 characters)")
	}
	for _, r := range query {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return fmt.Errorf("query contains invalid characters")
		}
	}
	return nil
}

func parseBoolValue(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseWSBool(raw interface{}) bool {
	switch value := raw.(type) {
	case bool:
		return value
	case string:
		return parseBoolValue(value)
	case float64:
		return value != 0
	case int:
		return value != 0
	default:
		return false
	}
}

func appendUniqueStrings(base []string, extra []string) []string {
	seen := make(map[string]struct{}, len(base)+len(extra))
	merged := make([]string, 0, len(base)+len(extra))
	for _, item := range base {
		if strings.TrimSpace(item) == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		merged = append(merged, item)
	}
	for _, item := range extra {
		if strings.TrimSpace(item) == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		merged = append(merged, item)
	}
	return merged
}

// maskAPIKey 屏蔽API密钥，用于日志输出与 GET /api/config 响应。
// 空字符串原样返回，便于前端区分「未配置」与「已配置但已脱敏」。
func maskAPIKey(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	if len(apiKey) <= 8 {
		return "****"
	}
	return apiKey[:4] + "****" + apiKey[len(apiKey)-4:]
}
