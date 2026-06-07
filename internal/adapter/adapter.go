package adapter

import (
	"context"
	"regexp"
	"strings"

	"github.com/unimap/project/internal/model"
)

// maxLogBodyLen is the maximum number of characters from a response body
// that may appear in log output or error messages.
const maxLogBodyLen = 200

// tokenPattern matches long hex or base64-like strings that could be
// API keys, tokens, or secrets embedded in response bodies.
var tokenPattern = regexp.MustCompile(`(?i)[0-9a-f]{32,}|[a-z0-9+/=_-]{40,}`)

// sanitizeBody truncates a response body for safe logging and masks any
// substring that looks like a token or secret. Use this whenever an HTTP
// response body must appear in a log line or error message.
func sanitizeBody(body string) string {
	if len(body) > maxLogBodyLen {
		body = body[:maxLogBodyLen] + "...(truncated)"
	}
	return tokenPattern.ReplaceAllString(body, "***")
}

// escapeQuotes escapes embedded double quotes in a value string.
// Used by adapters that wrap values in double quotes to prevent syntax breakage.
func escapeQuotes(v string) string {
	return strings.ReplaceAll(v, `"`, `\"`)
}

// EngineAdapter 引擎适配器接口
type EngineAdapter interface {
	// Name 返回引擎标识
	Name() string

	// Translate 将UQL AST转换为引擎专用查询串
	Translate(ast *model.UQLAST) (string, error)

	// Search 执行搜索并返回原生结果
	Search(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error)

	// Normalize 将原生结果映射到统一资产模型
	Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error)

	// GetQuota 获取引擎配额信息
	GetQuota() (*model.QuotaInfo, error)

	// IsWebOnly 检查是否为 Web-only 模式
	IsWebOnly() bool
}
