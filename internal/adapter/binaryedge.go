package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/utils"
)

const (
	// BinaryEdgeDefaultTimeout BinaryEdge默认超时
	BinaryEdgeDefaultTimeout = 30 * time.Second
	// BinaryEdgeDefaultQPS BinaryEdge默认QPS
	BinaryEdgeDefaultQPS = 2
)

// BinaryEdgeAdapter BinaryEdge引擎适配器
type BinaryEdgeAdapter struct {
	client  *resty.Client
	baseURL string
	apiKey  string
	qps     int
	timeout time.Duration
}

// NewBinaryEdgeAdapter 创建BinaryEdge适配器
func NewBinaryEdgeAdapter(baseURL, apiKey string, qps int, timeout time.Duration) *BinaryEdgeAdapter {
	client := resty.New().
		SetTimeout(timeout).
		SetHeader("User-Agent", "unimap/1.0")

	return &BinaryEdgeAdapter{
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
		qps:     qps,
		timeout: timeout,
	}
}

// Name 返回引擎名称
func (b *BinaryEdgeAdapter) Name() string {
	return "binaryedge"
}

// IsWebOnly 检查是否为 Web-only 模式
func (b *BinaryEdgeAdapter) IsWebOnly() bool {
	return false
}

// Translate 将UQL AST转换为BinaryEdge查询语法（Shodan兼容）
func (b *BinaryEdgeAdapter) Translate(ast *model.UQLAST) (string, error) {
	if ast == nil || ast.Root == nil {
		return "", fmt.Errorf("invalid AST")
	}

	query := b.translateNode(ast.Root)
	return query, nil
}

func (b *BinaryEdgeAdapter) translateNode(node *model.UQLNode) string {
	if node == nil {
		return ""
	}

	switch node.Type {
	case "condition":
		field := node.Value
		if len(node.Children) >= 2 {
			op := node.Children[0].Value
			val := node.Children[1].Value

			mappedField := b.mapField(field)

			if op == "IN" {
				// 同字段 OR 用逗号: port:80,443
				values := strings.Split(val, ",")
				quoted := make([]string, 0, len(values))
				for _, v := range values {
					quoted = append(quoted, binaryEdgeQuote(strings.TrimSpace(v)))
				}
				return fmt.Sprintf("%s:%s", mappedField, strings.Join(quoted, ","))
			}

			if op == "!=" || op == "<>" {
				return fmt.Sprintf("-%s:%s", mappedField, binaryEdgeQuote(val))
			}
			// BinaryEdge 比较操作符: field:>value, field:>=value, field:<value, field:<=value
			if op == ">" || op == ">=" || op == "<" || op == "<=" {
				return fmt.Sprintf("%s:%s%s", mappedField, op, binaryEdgeQuote(val))
			}
			return fmt.Sprintf("%s:%s", mappedField, binaryEdgeQuote(val))
		}

	case "logical":
		if len(node.Children) >= 2 {
			left := b.translateNode(node.Children[0])
			right := b.translateNode(node.Children[1])
			if node.Value == "OR" {
				// BinaryEdge 不支持跨字段 OR / 括号 — 降级为 AND 语义
				logger.Warnf("BinaryEdge adapter: OR between (%s) and (%s) degraded to AND (BinaryEdge does not support cross-field OR)", left, right)
			}
			// AND = 空格连接（BinaryEdge 原生语法）
			return fmt.Sprintf("%s %s", left, right)
		}
	}

	return ""
}

// binaryEdgeQuote 对 BinaryEdge 值加引号：含空格或特殊字符时包裹双引号，否则原样返回。
func binaryEdgeQuote(v string) string {
	if v == "" {
		return v
	}
	for _, c := range v {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_' || c == ':') {
			return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
		}
	}
	return v
}

// mapField 映射统一字段到BinaryEdge字段
func (b *BinaryEdgeAdapter) mapField(field string) string {
	mapping := map[string]string{
		"body":        "body",
		"title":       "title",
		"header":      "header",
		"port":        "port",
		"ip":          "ip",
		"country":     "country",
		"asn":         "asn",
		"domain":      "domain",
		"os":          "os",
		"app":         "product",
		"cert":        "cert",
		"server":      "header",
		"host":        "domain",
		"url":         "domain",
		"region":      "country",
		"city":        "city",
		"org":         "org",
		"isp":         "isp",
		"protocol":    "protocol",
		"status_code": "status_code",
	}

	if mapped, ok := mapping[field]; ok {
		return mapped
	}
	return field
}

// Search 执行BinaryEdge搜索
func (b *BinaryEdgeAdapter) Search(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error) {
	if b.apiKey == "" {
		return &model.EngineResult{
			EngineName: b.Name(),
			Error:      "BinaryEdge API key not configured",
		}, nil
	}

	var engineResult *model.EngineResult

	retryConfig := utils.RetryConfig{
		MaxRetries:  3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    2 * time.Second,
		Exponential: true,
		Jitter:      true,
		RetryableFunc: func(err error) bool {
			errStr := err.Error()
			// 非临时性错误不重试：认证失败
			if strings.Contains(errStr, "HTTP 401") ||
				strings.Contains(errStr, "HTTP 403") {
				return false
			}
			return true
		},
	}

	err := utils.Retry(retryConfig, func() error {
		searchURL := fmt.Sprintf("%s/v2/query/search", b.baseURL)

		resp, err := b.client.R().
			SetHeader("X-Key", b.apiKey).
			SetQueryParams(map[string]string{
				"query": query,
				"page":  fmt.Sprintf("%d", page),
			}).
			Get(searchURL)

		if err != nil {
			return err
		}

		if resp.StatusCode() != 200 {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode(), sanitizeBody(resp.String()))
		}

		var result struct {
			Events []json.RawMessage `json:"events"`
			Total  int              `json:"total"`
		}

		if err := json.Unmarshal(resp.Body(), &result); err != nil {
			return err
		}

		rawData := make([]interface{}, 0, len(result.Events))
		for _, event := range result.Events {
			var item map[string]interface{}
			if err := json.Unmarshal(event, &item); err != nil {
				continue
			}
			rawData = append(rawData, item)
		}

		engineResult = &model.EngineResult{
			EngineName: b.Name(),
			RawData:    rawData,
			Total:      result.Total,
			Page:       page,
			HasMore:    (page * pageSize) < result.Total,
		}

		return nil
	})

	if err != nil {
		return &model.EngineResult{
			EngineName: b.Name(),
			Error:      fmt.Sprintf("search error: %v", err),
		}, nil
	}

	return engineResult, nil
}

// Normalize 标准化BinaryEdge结果
func (b *BinaryEdgeAdapter) Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error) {
	if raw == nil || len(raw.RawData) == 0 {
		return []model.UnifiedAsset{}, nil
	}
	assets := make([]model.UnifiedAsset, 0, len(raw.RawData))
	for _, item := range raw.RawData {
		data, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if asset := b.normalizeBinaryEdgeItem(data); asset != nil {
			assets = append(assets, *asset)
		}
	}
	return assets, nil
}

// normalizeBinaryEdgeItem 解析单条 BinaryEdge 数据
func (b *BinaryEdgeAdapter) normalizeBinaryEdgeItem(data map[string]interface{}) *model.UnifiedAsset {
	asset := &model.UnifiedAsset{Source: b.Name()}
	getStr := func(k string) string { v, _ := data[k].(string); return v }
	getInt := func(k string) int {
		if v, ok := data[k].(float64); ok { return int(v) }
		if v, ok := data[k].(int); ok { return v }
		return 0
	}

	asset.IP = getStr("ip")
	asset.Port = getInt("port")
	asset.Protocol = getStr("protocol")
	asset.Host = getStr("domain")
	asset.Title = getStr("title")
	asset.Server = getStr("server")
	if body := getStr("body"); len(body) > 200 {
		asset.BodySnippet = body[:200]
	} else {
		asset.BodySnippet = body
	}
	asset.StatusCode = getInt("status_code")
	asset.CountryCode = getStr("country")
	asset.City = getStr("city")
	asset.ASN = getStr("asn")
	asset.Org = getStr("org")
	asset.ISP = getStr("isp")

	if asset.IP != "" && asset.Port > 0 {
		buildAssetURL(asset)
		asset.Extra = data
		return asset
	}
	if asset.Host != "" || asset.IP != "" {
		asset.Extra = data
		return asset
	}
	return nil
}

// GetQuota 获取BinaryEdge配额信息
func (b *BinaryEdgeAdapter) GetQuota() (*model.QuotaInfo, error) {
	if b.apiKey == "" {
		return nil, fmt.Errorf("BinaryEdge API key not configured")
	}

	quotaURL := fmt.Sprintf("%s/v2/user/subscription", b.baseURL)

	resp, err := b.client.R().
		SetHeader("X-Key", b.apiKey).
		Get(quotaURL)

	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), sanitizeBody(resp.String()))
	}

	var result struct {
		Plan struct {
			Queries struct {
				Allowed int `json:"allowed"`
				Used    int `json:"used"`
			} `json:"queries"`
		} `json:"plan"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	remaining := result.Plan.Queries.Allowed - result.Plan.Queries.Used
	if remaining < 0 {
		remaining = 0
	}

	return &model.QuotaInfo{
		Remaining: remaining,
		Total:     result.Plan.Queries.Allowed,
		Used:      result.Plan.Queries.Used,
		Unit:      "queries",
		Expiry:    "",
	}, nil
}

// BinaryEdgeAdapterWebOnly BinaryEdge Web-only模式适配器
type BinaryEdgeAdapterWebOnly struct {
	*WebOnlyAdapterBase
}

// NewBinaryEdgeAdapterWebOnly 创建BinaryEdge Web-only适配器
func NewBinaryEdgeAdapterWebOnly() *BinaryEdgeAdapterWebOnly {
	baseAdapter := NewBinaryEdgeAdapter("", "", BinaryEdgeDefaultQPS, BinaryEdgeDefaultTimeout)
	return &BinaryEdgeAdapterWebOnly{
		WebOnlyAdapterBase: NewWebOnlyAdapterBase(baseAdapter, "binaryedge"),
	}
}
