package scheduler

import (
	"strings"
	"unicode/utf8"

	"github.com/unimap/project/internal/model"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// sanitizeUTF8 ensures s is valid UTF-8, converting from GBK if necessary.
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	decoded, _, err := transform.String(simplifiedchinese.GBK.NewDecoder(), s)
	if err == nil && utf8.ValidString(decoded) {
		return decoded
	}
	return strings.ToValidUTF8(s, "�")
}

// extractStringsFromMap pulls a string slice from map[key], falling back to def.
func extractStringsFromMap(payload map[string]any, key string, def []string) []string {
	v, ok := payload[key]
	if !ok {
		return def
	}
	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if val == "" {
			return def
		}
		return []string{val}
	default:
		return def
	}
}

func extractIntFromMap(payload map[string]any, key string, def int) int {
	v, ok := payload[key]
	if !ok {
		return def
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	default:
		return def
	}
}

func extractStringFromMap(payload map[string]any, key string, def string) string {
	v, ok := payload[key]
	if !ok {
		return def
	}
	if s, ok := v.(string); ok {
		return s
	}
	return def
}

func extractBoolFromMap(payload map[string]any, key string, def bool) bool {
	v, ok := payload[key]
	if !ok {
		return def
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

// extractString gets a string from payload or its Extra map.
func extractString(payload *model.TaskPayload, key string, def string) string {
	if payload == nil {
		return def
	}
	switch key {
	case "query":
		return payload.Query
	case "format":
		return payload.Format
	case "detection_mode":
		return payload.DetectMode
	case "type":
		return payload.Type
	case "url":
		return payload.URL
	case "cookie_file":
		return payload.CookieFile
	case "engine":
		return extractStringFromMap(payload.Extra, key, def)
	case "query_id":
		return extractStringFromMap(payload.Extra, key, def)
	case "batch_id":
		return extractStringFromMap(payload.Extra, key, def)
	case "strategy":
		return extractStringFromMap(payload.Extra, key, def)
	}
	return extractStringFromMap(payload.Extra, key, def)
}

func extractStrings(payload *model.TaskPayload, key string, def []string) []string {
	if payload == nil {
		return def
	}
	switch key {
	case "engines":
		if len(payload.Engines) > 0 {
			return payload.Engines
		}
	case "queries":
		if len(payload.Queries) > 0 {
			return payload.Queries
		}
	case "urls":
		if len(payload.URLs) > 0 {
			return payload.URLs
		}
	}
	if payload.Extra != nil {
		return extractStringsFromMap(payload.Extra, key, def)
	}
	return def
}

func extractInt(payload *model.TaskPayload, key string, def int) int {
	if payload == nil {
		return def
	}
	switch key {
	case "page_size":
		if payload.PageSize > 0 {
			return payload.PageSize
		}
	case "icp_page_size":
		if payload.PageSizeICP > 0 {
			return payload.PageSizeICP
		}
	case "max_age_days":
		if payload.MaxAgeDays > 0 {
			return payload.MaxAgeDays
		}
	case "low_threshold":
		if payload.LowThresh > 0 {
			return payload.LowThresh
		}
	case "page":
		if payload.Page > 0 {
			return payload.Page
		}
	case "timeout_seconds":
		if payload.TimeoutSec > 0 {
			return payload.TimeoutSec
		}
	}
	if payload.Extra != nil {
		return extractIntFromMap(payload.Extra, key, def)
	}
	return def
}

func extractBool(payload *model.TaskPayload, key string, def bool) bool {
	if payload == nil {
		return def
	}
	switch key {
	case "fail_fast":
		return extractBoolFromMap(payload.Extra, key, def)
	}
	if payload.Extra != nil {
		return extractBoolFromMap(payload.Extra, key, def)
	}
	return def
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
