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
	// GreyNoiseDefaultTimeout GreyNoise默认超时
	GreyNoiseDefaultTimeout = 30 * time.Second
	// GreyNoiseDefaultQPS GreyNoise默认QPS
	GreyNoiseDefaultQPS = 1
)

// GreyNoiseAdapter GreyNoise引擎适配器
type GreyNoiseAdapter struct {
	client  *resty.Client
	baseURL string
	apiKey  string
	qps     int
	timeout time.Duration
}

// NewGreyNoiseAdapter 创建GreyNoise适配器
func NewGreyNoiseAdapter(baseURL, apiKey string, qps int, timeout time.Duration) *GreyNoiseAdapter {
	client := resty.New().
		SetTimeout(timeout).
		SetHeader("User-Agent", "unimap/1.0")

	return &GreyNoiseAdapter{
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
		qps:     qps,
		timeout: timeout,
	}
}

// Name 返回引擎名称
func (g *GreyNoiseAdapter) Name() string {
	return "greynoise"
}

// IsWebOnly 检查是否为 Web-only 模式
func (g *GreyNoiseAdapter) IsWebOnly() bool {
	return false
}

// greyNoiseQuote 对 GreyNoise 值加引号：含空格或特殊字符时包裹双引号，否则原样返回。
func greyNoiseQuote(v string) string {
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

// mapField 映射统一字段到GreyNoise字段
func (g *GreyNoiseAdapter) mapField(field string) string {
	mapping := map[string]string{
		"ip":          "ip",
		"class":       "classification",
		"classification": "classification",
		"tag":         "tags",
		"tags":        "tags",
		"org":         "metadata.organization",
		"isp":         "metadata.organization",
		"os":          "metadata.os",
		"country":     "metadata.country",
		"city":        "metadata.city",
		"category":    "metadata.category",
		"port":        "raw_data.scan.port",
		"protocol":    "raw_data.scan.protocol",
		"noise":       "noise",
		"riot":        "riot",
		"spoofable":   "spoofable",
		"vpn":         "vpn_service",
		"vpn_service": "vpn_service",
		"first_seen":  "first_seen",
		"last_seen":   "last_seen",
		"asn":         "metadata.asn",
	}

	if mapped, ok := mapping[field]; ok {
		return mapped
	}
	// title, body, header, server, cert, domain, hostname, url, app, status_code 等无对应GNQL字段
	// 对于威胁情报引擎，这些字段通常不适用，记录警告后 passthrough
	logger.Debugf("GreyNoise: unmapped field %q, passing through", field)
	return field
}

// Translate 将UQL AST转换为GreyNoise查询语法 (GNQL)
// GNQL: field:value, 空格为AND, OR为OR, -field:value为NOT
func (g *GreyNoiseAdapter) Translate(ast *model.UQLAST) (string, error) {
	if ast == nil || ast.Root == nil {
		return "", fmt.Errorf("invalid AST")
	}

	query := g.translateNode(ast.Root)
	return query, nil
}

func (g *GreyNoiseAdapter) translateNode(node *model.UQLNode) string {
	if node == nil {
		return ""
	}

	switch node.Type {
	case "condition":
		field := node.Value
		if len(node.Children) >= 2 {
			op := node.Children[0].Value
			val := node.Children[1].Value

			mappedField := g.mapField(field)

			if op == "IN" {
				// 同字段 OR: field:val1 OR field:val2 OR ...
				values := strings.Split(val, ",")
				clauses := make([]string, 0, len(values))
				for _, v := range values {
					clauses = append(clauses, fmt.Sprintf("%s:%s", mappedField, greyNoiseQuote(strings.TrimSpace(v))))
				}
				return strings.Join(clauses, " OR ")
			}

			if op == "!=" || op == "<>" {
				return fmt.Sprintf("-%s:%s", mappedField, greyNoiseQuote(val))
			}
			// GNQL 不支持比较操作符 > >= < <=，但保留语法兼容
			if op == ">" || op == ">=" || op == "<" || op == "<=" {
				logger.Warnf("GreyNoise: comparison operator %q not supported in GNQL, using equality", op)
				return fmt.Sprintf("%s:%s", mappedField, greyNoiseQuote(val))
			}
			return fmt.Sprintf("%s:%s", mappedField, greyNoiseQuote(val))
		}

	case "logical":
		if len(node.Children) >= 2 {
			left := g.translateNode(node.Children[0])
			right := g.translateNode(node.Children[1])
			if node.Value == "OR" {
				return fmt.Sprintf("%s OR %s", left, right)
			}
			// AND = 空格连接（GNQL 原生语法）
			return fmt.Sprintf("%s %s", left, right)
		}
	}

	return ""
}

// GreyNoiseSearchResponse GreyNoise API搜索响应
type GreyNoiseSearchResponse struct {
	Complete bool                   `json:"complete"`
	Count    int                    `json:"count"`
	Data     []json.RawMessage      `json:"data"`
	Message  string                 `json:"message,omitempty"`
	Query    string                 `json:"query"`
	Scroll   string                 `json:"scroll,omitempty"`
}

// Search 执行GreyNoise搜索
func (g *GreyNoiseAdapter) Search(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error) {
	if g.apiKey == "" {
		return &model.EngineResult{EngineName: g.Name(), Error: "GreyNoise API key not configured"}, nil
	}
	var engineResult *model.EngineResult
	err := utils.Retry(g.searchRetryConfig(), func() error {
		return g.executeGreyNoiseSearch(query, page, pageSize, &engineResult)
	})
	if err != nil {
		return &model.EngineResult{EngineName: g.Name(), Error: fmt.Sprintf("search error: %v", err)}, nil
	}
	return engineResult, nil
}

func (g *GreyNoiseAdapter) searchRetryConfig() utils.RetryConfig {
	return utils.RetryConfig{
		MaxRetries: 3, BaseDelay: 100 * time.Millisecond, MaxDelay: 2 * time.Second,
		Exponential: true, Jitter: true,
		RetryableFunc: func(err error) bool {
			errStr := err.Error()
			if strings.Contains(errStr, "HTTP 401") || strings.Contains(errStr, "HTTP 403") {
				return false
			}
			return true
		},
	}
}

// executeGreyNoiseSearch 执行单次 GreyNoise API 调用
func (g *GreyNoiseAdapter) executeGreyNoiseSearch(query string, page, pageSize int, result **model.EngineResult) error {
	searchURL := fmt.Sprintf("%s/v3/experimental/gnql", g.baseURL)
	resp, err := g.client.R().
		SetHeader("key", g.apiKey).
		SetQueryParams(map[string]string{
			"query": query, "size": fmt.Sprintf("%d", pageSize),
		}).
		Get(searchURL)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode(), sanitizeBody(resp.String()))
	}
	return parseGreyNoiseSearchResponse(resp.Body(), pageSize, g.Name(), result)
}

// parseGreyNoiseSearchResponse 解析 GreyNoise 搜索响应
func parseGreyNoiseSearchResponse(body []byte, pageSize int, engineName string, result **model.EngineResult) error {
	var resp GreyNoiseSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if resp.Message != "" && resp.Count == 0 {
		return fmt.Errorf("GreyNoise API error: %s", resp.Message)
	}
	rawData := make([]interface{}, 0, len(resp.Data))
	for _, item := range resp.Data {
		var data map[string]interface{}
		if err := json.Unmarshal(item, &data); err != nil {
			continue
		}
		rawData = append(rawData, data)
	}
	*result = &model.EngineResult{
		EngineName: engineName, RawData: rawData, Total: resp.Count,
		Page: 1, HasMore: !resp.Complete && resp.Scroll != "",
	}
	return nil
}

// Normalize 标准化GreyNoise结果
func (g *GreyNoiseAdapter) Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error) {
	if raw == nil || len(raw.RawData) == 0 {
		return []model.UnifiedAsset{}, nil
	}
	assets := make([]model.UnifiedAsset, 0, len(raw.RawData))
	for _, item := range raw.RawData {
		data, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if asset := g.normalizeGreyNoiseItem(data); asset != nil {
			assets = append(assets, *asset)
		}
	}
	return assets, nil
}

// normalizeGreyNoiseItem 解析单条 GreyNoise 数据
func (g *GreyNoiseAdapter) normalizeGreyNoiseItem(data map[string]interface{}) *model.UnifiedAsset {
	ip, _ := data["ip"].(string)
	if ip == "" {
		return nil
	}
	asset := &model.UnifiedAsset{IP: ip, Source: g.Name()}
	getInt := func(k string, parent map[string]interface{}) int {
		if v, ok := parent[k].(float64); ok { return int(v) }
		if v, ok := parent[k].(int); ok { return v }
		return 0
	}

	// 分类 + 标签
	if classification, ok := data["classification"].(string); ok {
		asset.Title = classification
	}
	if tags, ok := data["tags"].([]interface{}); ok && len(tags) > 0 {
		tagStrs := make([]string, 0, len(tags))
		for _, t := range tags {
			if s, ok := t.(string); ok {
				tagStrs = append(tagStrs, s)
			}
		}
		if len(tagStrs) > 0 {
			if asset.Title != "" {
				asset.Title += " | " + strings.Join(tagStrs, ", ")
			} else {
				asset.Title = strings.Join(tagStrs, ", ")
			}
		}
	}

	// 元数据
	if metadata, ok := data["metadata"].(map[string]interface{}); ok {
		if org, ok := metadata["organization"].(string); ok {
			asset.Org = org
			asset.ISP = org
		}
		if country, ok := metadata["country"].(string); ok {
			asset.CountryCode = country
		}
		if city, ok := metadata["city"].(string); ok {
			asset.City = city
		}
		if asn, ok := metadata["asn"].(string); ok {
			asset.ASN = asn
		} else if asnF, ok := metadata["asn"].(float64); ok {
			asset.ASN = fmt.Sprintf("AS%d", int(asnF))
		}
		if os, ok := metadata["os"].(string); ok {
			asset.Server = os
		}
	}

	// 端口/协议
	if rawData, ok := data["raw_data"].(map[string]interface{}); ok {
		if scan, ok := rawData["scan"].(map[string]interface{}); ok {
			asset.Port = getInt("port", scan)
			if proto, ok := scan["protocol"].(string); ok {
				asset.Protocol = proto
			}
		}
	}

	// URL
	if asset.IP != "" && asset.Port > 0 {
		buildAssetURL(asset)
	}

	// Body snippet: noise/riot 状态
	snippet := fmt.Sprintf("noise=%v riot=%v", data["noise"], data["riot"])
	if spoofable, ok := data["spoofable"].(bool); ok && spoofable {
		snippet += " spoofable=true"
	}
	asset.BodySnippet = snippet

	asset.Extra = data
	return asset
}

// GetQuota 获取GreyNoise配额信息
func (g *GreyNoiseAdapter) GetQuota() (*model.QuotaInfo, error) {
	if g.apiKey == "" {
		return nil, fmt.Errorf("GreyNoise API key not configured")
	}

	quotaURL := fmt.Sprintf("%s/v3/user/quota", g.baseURL)

	resp, err := g.client.R().
		SetHeader("key", g.apiKey).
		Get(quotaURL)

	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("GreyNoise API HTTP %d", resp.StatusCode())
	}

	var result struct {
		QueriesRemaining int    `json:"queries_remaining"`
		QueriesUsed      int    `json:"queries_used"`
		QueriesTotal     int    `json:"queries_total"`
		RateLimit        string `json:"rate_limit,omitempty"`
		Reset            string `json:"reset,omitempty"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	return &model.QuotaInfo{
		Remaining: result.QueriesRemaining,
		Total:     result.QueriesTotal,
		Used:      result.QueriesUsed,
		Unit:      "queries",
		Expiry:    result.Reset,
	}, nil
}

// GreyNoiseAdapterWebOnly GreyNoise Web-only模式适配器
type GreyNoiseAdapterWebOnly struct {
	*WebOnlyAdapterBase
}

// NewGreyNoiseAdapterWebOnly 创建GreyNoise Web-only适配器
func NewGreyNoiseAdapterWebOnly() *GreyNoiseAdapterWebOnly {
	baseAdapter := NewGreyNoiseAdapter("", "", GreyNoiseDefaultQPS, GreyNoiseDefaultTimeout)
	return &GreyNoiseAdapterWebOnly{
		WebOnlyAdapterBase: NewWebOnlyAdapterBase(baseAdapter, "greynoise"),
	}
}
