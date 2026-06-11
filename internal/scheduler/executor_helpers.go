package scheduler

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// sanitizeUTF8 ensures s is valid UTF-8, converting from GBK if necessary.
// This prevents garbled text (mojibake) in notification channels that assume
// valid UTF-8 (Feishu, DingTalk, WeCom).
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

// extractStrings pulls a string slice from payload[key], falling back to def.
func extractStrings(payload map[string]interface{}, key string, def []string) []string {
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

func extractInt(payload map[string]interface{}, key string, def int) int {
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

func extractString(payload map[string]interface{}, key string, def string) string {
	v, ok := payload[key]
	if !ok {
		return def
	}
	if s, ok := v.(string); ok {
		return s
	}
	return def
}

func extractBool(payload map[string]interface{}, key string, def bool) bool {
	v, ok := payload[key]
	if !ok {
		return def
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
