package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/utils"
)

const (
	// OnypheDefaultTimeout Onyphe默认超时
	OnypheDefaultTimeout = 30 * time.Second
	// OnypheDefaultQPS Onyphe默认QPS
	OnypheDefaultQPS = 1
)

// OnypheAdapter Onyphe引擎适配器
type OnypheAdapter struct {
	client  *resty.Client
	baseURL string
	apiKey  string
	qps     int
	timeout time.Duration
}

// NewOnypheAdapter 创建Onyphe适配器
func NewOnypheAdapter(baseURL, apiKey string, qps int, timeout time.Duration) *OnypheAdapter {
	client := resty.New().
		SetTimeout(timeout).
		SetHeader("User-Agent", "unimap/1.0")

	return &OnypheAdapter{
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
		qps:     qps,
		timeout: timeout,
	}
}

// Name 返回引擎名称
func (o *OnypheAdapter) Name() string {
	return "onyphe"
}

// IsWebOnly 检查是否为 Web-only 模式
func (o *OnypheAdapter) IsWebOnly() bool {
	return false
}

// Translate 将UQL AST转换为Onyphe查询语法 (OQL)
// OQL: field:value, + for AND, OR for OR, -field:value for NOT
func (o *OnypheAdapter) Translate(ast *model.UQLAST) (string, error) {
	if ast == nil || ast.Root == nil {
		return "", fmt.Errorf("invalid AST")
	}

	query := o.translateNode(ast.Root)
	return query, nil
}

func (o *OnypheAdapter) translateNode(node *model.UQLNode) string {
	if node == nil {
		return ""
	}

	switch node.Type {
	case "condition":
		field := node.Value
		if len(node.Children) >= 2 {
			op := node.Children[0].Value
			val := node.Children[1].Value

			mappedField := o.mapField(field)

			if op == "IN" {
				// 同字段 OR: field:val1 OR field:val2 OR ...
				values := strings.Split(val, ",")
				clauses := make([]string, 0, len(values))
				for _, v := range values {
					clauses = append(clauses, fmt.Sprintf("%s:%s", mappedField, onypheQuote(strings.TrimSpace(v))))
				}
				return strings.Join(clauses, " OR ")
			}

			if op == "!=" || op == "<>" {
				return fmt.Sprintf("-%s:%s", mappedField, onypheQuote(val))
			}
			// OQL 比较操作符: field:>value, field:>=value, field:<value, field:<=value
			if op == ">" || op == ">=" || op == "<" || op == "<=" {
				return fmt.Sprintf("%s:%s%s", mappedField, op, onypheQuote(val))
			}
			return fmt.Sprintf("%s:%s", mappedField, onypheQuote(val))
		}

	case "logical":
		if len(node.Children) >= 2 {
			left := o.translateNode(node.Children[0])
			right := o.translateNode(node.Children[1])
			if node.Value == "OR" {
				return fmt.Sprintf("%s OR %s", left, right)
			}
			// AND = + 连接（OQL 原生语法）
			return fmt.Sprintf("%s +%s", left, right)
		}
	}

	return ""
}

// onypheQuote 对 Onyphe 值加引号：含空格或特殊字符时包裹双引号，否则原样返回。
func onypheQuote(v string) string {
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

// mapField 映射统一字段到Onyphe字段
// 注意：Onyphe OQL 不支持 country/city/os 等地理字段，这些会作为 passthrough 处理
func (o *OnypheAdapter) mapField(field string) string {
	mapping := map[string]string{
		"ip":       "ip",
		"port":     "port",
		"domain":   "domain",
		"hostname": "hostname",
		"host":     "hostname",
		"url":      "domain",
		"asn":      "asn",
		"org":      "organization",
		"isp":      "organization",
		"app":      "product",
	}

	if mapped, ok := mapping[field]; ok {
		return mapped
	}
	// body, title, header, server, cert, protocol, status_code, country, city, os 等无对应OQL字段，passthrough
	return field
}

// OnypheSearchResponse Onyphe API搜索响应
type OnypheSearchResponse struct {
	Results  []json.RawMessage `json:"results"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	MaxPage  int               `json:"max_page"`
	Error    string            `json:"error,omitempty"`
	ErrMsg   string            `json:"errormsg,omitempty"`
	Count    int               `json:"count"`
}

// Search 执行Onyphe搜索
func (o *OnypheAdapter) Search(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error) {
	if o.apiKey == "" {
		return &model.EngineResult{
			EngineName: o.Name(),
			Error:      "Onyphe API key not configured",
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
			if strings.Contains(errStr, "HTTP 401") ||
				strings.Contains(errStr, "HTTP 403") {
				return false
			}
			return true
		},
	}

	err := utils.Retry(retryConfig, func() error {
		searchURL := fmt.Sprintf("%s/api/v2/simple/search", o.baseURL)

		resp, err := o.client.R().
			SetHeader("Authorization", fmt.Sprintf("apikey %s", o.apiKey)).
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

		var result OnypheSearchResponse
		if err := json.Unmarshal(resp.Body(), &result); err != nil {
			return err
		}

		if result.Error != "" {
			return fmt.Errorf("Onyphe API error: %s", result.Error)
		}
		if result.ErrMsg != "" {
			return fmt.Errorf("Onyphe API error: %s", result.ErrMsg)
		}

		rawData := make([]interface{}, 0, len(result.Results))
		for _, item := range result.Results {
			var data map[string]interface{}
			if err := json.Unmarshal(item, &data); err != nil {
				continue
			}
			rawData = append(rawData, data)
		}

		engineResult = &model.EngineResult{
			EngineName: o.Name(),
			RawData:    rawData,
			Total:      result.Total,
			Page:       result.Page,
			HasMore:    result.Page < result.MaxPage,
		}

		return nil
	})

	if err != nil {
		return &model.EngineResult{
			EngineName: o.Name(),
			Error:      fmt.Sprintf("search error: %v", err),
		}, nil
	}

	return engineResult, nil
}

// Normalize 标准化Onyphe结果
func (o *OnypheAdapter) Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error) {
	assets := make([]model.UnifiedAsset, 0, len(raw.RawData))
	if raw == nil || len(raw.RawData) == 0 {
		return assets, nil
	}
	for _, item := range raw.RawData {
		data, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if asset := o.normalizeOnypheItem(data); asset != nil {
			assets = append(assets, *asset)
		}
	}
	return assets, nil
}

// normalizeOnypheItem 解析单条 Onyphe 数据
func (o *OnypheAdapter) normalizeOnypheItem(data map[string]interface{}) *model.UnifiedAsset {
	ip, _ := data["ip"].(string)
	if ip == "" {
		return nil
	}
	asset := &model.UnifiedAsset{IP: ip, Source: o.Name()}
	getStr := func(k string) string { v, _ := data[k].(string); return v }
	getInt := func(k string) int {
		if v, ok := data[k].(float64); ok { return int(v) }
		if v, ok := data[k].(int); ok { return v }
		return 0
	}

	asset.Port = getInt("port")
	if proto := getStr("transport"); proto != "" {
		asset.Protocol = proto
	} else {
		asset.Protocol = getStr("protocol")
	}
	if domain := getStr("domain"); domain != "" {
		asset.Host = domain
	} else {
		asset.Host = getStr("hostname")
	}
	asset.Title = getStr("product")
	asset.Server = getStr("server")
	if body := getStr("content"); len(body) > 200 {
		asset.BodySnippet = body[:200]
	} else {
		asset.BodySnippet = body
	}
	asset.StatusCode = getInt("status")
	asset.CountryCode = getStr("country")
	asset.City = getStr("city")
	if asn := getStr("asn"); asn != "" {
		asset.ASN = asn
	} else if asnF, ok := data["asn"].(float64); ok {
		asset.ASN = fmt.Sprintf("AS%d", int(asnF))
	}
	if org := getStr("organization"); org != "" {
		asset.Org = org
		asset.ISP = org
	}

	if asset.IP != "" && asset.Port > 0 {
		buildAssetURL(asset)
		asset.Extra = data
		return asset
	}
	if asset.Host != "" {
		asset.Extra = data
		return asset
	}
	return nil
}

// GetQuota 获取Onyphe配额信息
func (o *OnypheAdapter) GetQuota() (*model.QuotaInfo, error) {
	if o.apiKey == "" {
		return nil, fmt.Errorf("Onyphe API key not configured")
	}

	quotaURL := fmt.Sprintf("%s/api/v2/user", o.baseURL)

	resp, err := o.client.R().
		SetHeader("Authorization", fmt.Sprintf("apikey %s", o.apiKey)).
		Get(quotaURL)

	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}

	if resp.StatusCode() != 200 {
		var apiErr struct {
			Error   string `json:"error"`
			ErrMsg  string `json:"errormsg"`
		}
		if err := json.Unmarshal(resp.Body(), &apiErr); err == nil {
			errMsg := apiErr.ErrMsg
			if errMsg == "" {
				errMsg = apiErr.Error
			}
			if strings.TrimSpace(errMsg) != "" {
				return nil, fmt.Errorf("Onyphe API error: %s", strings.TrimSpace(errMsg))
			}
		}
		return nil, fmt.Errorf("Onyphe API HTTP %d", resp.StatusCode())
	}

	var result struct {
		Plan        string `json:"plan"`
		QuotaUsed   int    `json:"quota_used"`
		QuotaMax    int    `json:"quota_max"`
		Error       string `json:"error,omitempty"`
		ErrMsg      string `json:"errormsg,omitempty"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("Onyphe API error: %s", result.Error)
	}
	if result.ErrMsg != "" {
		return nil, fmt.Errorf("Onyphe API error: %s", result.ErrMsg)
	}

	remaining := result.QuotaMax - result.QuotaUsed
	if remaining < 0 {
		remaining = 0
	}

	return &model.QuotaInfo{
		Remaining: remaining,
		Total:     result.QuotaMax,
		Used:      result.QuotaUsed,
		Unit:      "queries",
		Expiry:    "",
	}, nil
}

// OnypheAdapterWebOnly Onyphe Web-only模式适配器
type OnypheAdapterWebOnly struct {
	*WebOnlyAdapterBase
}

// NewOnypheAdapterWebOnly 创建Onyphe Web-only适配器
func NewOnypheAdapterWebOnly() *OnypheAdapterWebOnly {
	baseAdapter := NewOnypheAdapter("", "", OnypheDefaultQPS, OnypheDefaultTimeout)
	return &OnypheAdapterWebOnly{
		WebOnlyAdapterBase: NewWebOnlyAdapterBase(baseAdapter, "onyphe"),
	}
}
