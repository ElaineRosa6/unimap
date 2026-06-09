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

// QuakeAdapter Quake引擎适配器
type QuakeAdapter struct {
	client  *resty.Client
	baseURL string
	apiKey  string
	qps     int
	timeout time.Duration
}

func quakeIsSuccessCode(code interface{}) bool {
	switch v := code.(type) {
	case nil:
		return true // some responses omit code on success
	case int:
		return v == 0 || v == 200
	case int64:
		return v == 0 || v == 200
	case float64:
		return int(v) == 0 || int(v) == 200
	case string:
		vv := strings.TrimSpace(v)
		return vv == "0" || vv == "200" || strings.EqualFold(vv, "success")
	default:
		return false
	}
}

// NewQuakeAdapter 创建Quake适配器
func NewQuakeAdapter(baseURL, apiKey string, qps int, timeout time.Duration) *QuakeAdapter {
	client := resty.New().
		SetTimeout(timeout).
		SetHeader("User-Agent", "unimap/1.0").
		SetHeader("X-QuakeToken", apiKey)

	return &QuakeAdapter{
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
		qps:     qps,
		timeout: timeout,
	}
}

// Name 返回引擎名称
func (q *QuakeAdapter) Name() string {
	return "quake"
}

// Translate 将UQL AST转换为Quake查询语法
func (q *QuakeAdapter) Translate(ast *model.UQLAST) (string, error) {
	if ast == nil || ast.Root == nil {
		return "", fmt.Errorf("invalid AST")
	}

	query := q.translateNode(ast.Root)
	return query, nil
}

func (q *QuakeAdapter) translateNode(node *model.UQLNode) string {
	if node == nil {
		return ""
	}

	switch node.Type {
	case "condition":
		field := node.Value
		if len(node.Children) >= 2 {
			op := node.Children[0].Value
			val := node.Children[1].Value

			if op == "IN" {
				values := strings.Split(val, ",")
				conditions := []string{}
				for _, v := range values {
					conditions = append(conditions, q.buildCondition(field, "=", v))
				}
				return "(" + strings.Join(conditions, " OR ") + ")"
			}

			return q.buildCondition(field, op, val)
		}

	case "logical":
		if len(node.Children) >= 2 {
			left := q.translateNode(node.Children[0])
			right := q.translateNode(node.Children[1])
			if node.Value == "OR" {
				return fmt.Sprintf("(%s OR %s)", left, right)
			}
			return fmt.Sprintf("(%s AND %s)", left, right)
		}
	}

	return ""
}

func (q *QuakeAdapter) buildCondition(field, op, value string) string {
	// 字段映射
	mapping := map[string]string{
		"body":        "response",
		"title":       "title",
		// "header" removed — Quake has no header field; falls through to passthrough
		"port":        "port",
		"protocol":    "service",
		"ip":          "ip",
		"country":     "country",
		"region":      "province",
		"city":        "city",
		"asn":         "asn",
		"org":         "org",
		"isp":         "isp",
		"domain":      "domain",
		"app":         "app",
		"os":          "os",
		"server":      "server",
		"host":        "domain",
		"url":         "url",
		"status_code": "status_code",
		"cert":        "cert",
	}

	if mapped, ok := mapping[field]; ok {
		field = mapped
	}

	escaped := escapeQuotes(value)

	if op == "!=" || op == "<>" {
		return fmt.Sprintf(`NOT %s:"%s"`, field, escaped)
	}
	// Quake 区间语法: port:[N TO M] / port:[N TO *]
	// 注意: Quake 的 [N TO *] 包含 N 本身，因此 > 和 >= 输出相同结果（Quake 无排他下界语法）
	if op == ">" || op == ">=" {
		return fmt.Sprintf(`%s:[%s TO *]`, field, escaped)
	}
	if op == "<" || op == "<=" {
		return fmt.Sprintf(`%s:[* TO %s]`, field, escaped)
	}

	// Quake syntax: field:"value"
	return fmt.Sprintf(`%s:"%s"`, field, escaped)
}

// Search 执行搜索
func (q *QuakeAdapter) Search(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error) {
	var engineResult *model.EngineResult

	retryConfig := utils.RetryConfig{
		MaxRetries:  3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    2 * time.Second,
		Exponential: true,
		Jitter:      true,
		RetryableFunc: func(err error) bool {
			// 网络错误可重试
			return true
		},
	}

	err := utils.Retry(retryConfig, func() error {
		// Quake API endpoint: /v3/search/quake_service
		url := fmt.Sprintf("%s/v3/search/quake_service", q.baseURL)

		reqBody := map[string]interface{}{
			"query": query,
			"start": (page - 1) * pageSize,
			"size":  pageSize,
		}

		resp, err := q.client.R().
			SetBody(reqBody).
			Post(url)

		if err != nil {
			return err
		}

		if resp.StatusCode() != 200 {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String())
		}

		// 解析Quake响应
		var result struct {
			Code    interface{}   `json:"code"` // Can be int or string depending on version/error
			Message string        `json:"message"`
			Data    []interface{} `json:"data"`
			Meta    struct {
				Pagination struct {
					Total int `json:"total"`
					Count int `json:"count"`
				} `json:"pagination"`
			} `json:"meta"`
		}

		if err := json.Unmarshal(resp.Body(), &result); err != nil {
			return err
		}

		if !quakeIsSuccessCode(result.Code) {
			return fmt.Errorf("quake API error (code=%v): %s", result.Code, result.Message)
		}

		engineResult = &model.EngineResult{
			EngineName: q.Name(),
			RawData:    result.Data,
			Total:      result.Meta.Pagination.Total,
			Page:       page,
			HasMore:    (result.Meta.Pagination.Total > page*pageSize),
		}

		return nil
	})

	if err != nil {
		return &model.EngineResult{
			EngineName: q.Name(),
			Error:      fmt.Sprintf("search error: %v", err),
		}, nil
	}

	return engineResult, nil
}

// Normalize 标准化结果
func (q *QuakeAdapter) Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error) {
	assets := make([]model.UnifiedAsset, 0, len(raw.RawData))

	if raw == nil || len(raw.RawData) == 0 {
		return assets, nil
	}

	for _, item := range raw.RawData {
		data, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// 创建新的资产对象
		asset := &model.UnifiedAsset{
			Source: q.Name(),
		}

		if ip, ok := data["ip"].(string); ok {
			asset.IP = ip
		}
		if port, ok := data["port"].(float64); ok {
			asset.Port = int(port)
		}

		if service, ok := data["service"].(map[string]interface{}); ok {
			if name, ok := service["name"].(string); ok {
				asset.Protocol = name
			}
			if http, ok := service["http"].(map[string]interface{}); ok {
				if title, ok := http["title"].(string); ok {
					asset.Title = title
				}
				if server, ok := http["server"].(string); ok {
					asset.Server = server
				}
				if statusCode, ok := http["status_code"].(float64); ok {
					asset.StatusCode = int(statusCode)
				}
			}
		}

		if location, ok := data["location"].(map[string]interface{}); ok {
			if country, ok := location["country_code"].(string); ok {
				asset.CountryCode = country
			}
			if city, ok := location["city_cn"].(string); ok {
				asset.City = city
			}
			if province, ok := location["province_cn"].(string); ok {
				asset.Region = province
			}
		}

		if asset.IP != "" || asset.Host != "" {
			assets = append(assets, *asset)
		}
	}

	return assets, nil
}

// GetQuota 获取Quake配额信息
func (q *QuakeAdapter) GetQuota() (*model.QuotaInfo, error) {
	if q.apiKey == "" {
		return nil, fmt.Errorf("Quake API key not configured")
	}
	url := fmt.Sprintf("%s/v3/user/info", q.baseURL)
	resp, err := q.client.R().SetQueryParam("key", q.apiKey).Get(url)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String())
	}
	return parseQuakeQuotaResponse(resp.Body())
}

// parseQuakeQuotaResponse 防御性解析 Quake 配额响应（支持多种响应结构）
func parseQuakeQuotaResponse(body []byte) (*model.QuotaInfo, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	code := raw["code"]
	message, _ := raw["message"].(string)
	if !quakeIsSuccessCode(code) {
		return nil, fmt.Errorf("quake API error (code=%v): %s", code, message)
	}

	total, used, remain, found := extractQuakeQuotaValues(raw)
	if !found {
		logger.Warnf("Quake quota response structure different than expected, using default values: %v", raw)
		return &model.QuotaInfo{Unit: "queries"}, nil
	}
	return &model.QuotaInfo{Remaining: remain, Total: total, Used: used, Unit: "queries"}, nil
}

// extractQuakeQuotaValues 尝试从多种响应结构中提取配额值
func extractQuakeQuotaValues(raw map[string]interface{}) (total, used, remain int, found bool) {
	qInt := func(v interface{}) (int, bool) {
		switch vv := v.(type) {
		case int:
			return vv, true
		case int64:
			return int(vv), true
		case float64:
			return int(vv), true
		case string:
			vv = strings.TrimSpace(vv)
			if vv == "" {
				return 0, false
			}
			var n int
			_, err := fmt.Sscanf(vv, "%d", &n)
			return n, err == nil
		default:
			return 0, false
		}
	}
	qMap := func(v interface{}) (map[string]interface{}, bool) {
		m, ok := v.(map[string]interface{})
		return m, ok
	}
	extractFromLimit := func(ql map[string]interface{}) bool {
		t, tOk := qInt(ql["total"])
		u, uOk := qInt(ql["used"])
		r, rOk := qInt(ql["remain"])
		if tOk || uOk || rOk {
			total, used, remain = t, u, r
			return true
		}
		return false
	}

	// 结构1: data.resource.query_limit / queryLimit
	if data, ok := qMap(raw["data"]); ok {
		if resource, ok := qMap(data["resource"]); ok {
			if ql, ok := qMap(resource["query_limit"]); ok && extractFromLimit(ql) {
				return total, used, remain, true
			}
			if ql, ok := qMap(resource["queryLimit"]); ok && extractFromLimit(ql) {
				return total, used, remain, true
			}
		}
		// 结构2: data.query_limit / queryLimit
		if ql, ok := qMap(data["query_limit"]); ok && extractFromLimit(ql) {
			return total, used, remain, true
		}
		if ql, ok := qMap(data["queryLimit"]); ok && extractFromLimit(ql) {
			return total, used, remain, true
		}
		// 结构3: data.credit + data.month_remaining_credit
		t, tOk := qInt(data["credit"])
		r, rOk := qInt(data["month_remaining_credit"])
		if tOk && rOk {
			logger.Infof("Quake quota: total=%d, used=%d, remain=%d", t, t-r, r)
			return t, t - r, r, true
		}
	}
	// 结构4: raw.query_limit / queryLimit
	if ql, ok := qMap(raw["query_limit"]); ok && extractFromLimit(ql) {
		return total, used, remain, true
	}
	if ql, ok := qMap(raw["queryLimit"]); ok && extractFromLimit(ql) {
		return total, used, remain, true
	}
	return 0, 0, 0, false
}

// IsWebOnly 检查是否为 Web-only 模式
func (q *QuakeAdapter) IsWebOnly() bool {
	return false
}

// QuakeAdapterWebOnly Quake Web-only模式适配器
type QuakeAdapterWebOnly struct {
	*WebOnlyAdapterBase
}

// NewQuakeAdapterWebOnly 创建Quake Web-only适配器
func NewQuakeAdapterWebOnly() *QuakeAdapterWebOnly {
	baseAdapter := NewQuakeAdapter("", "", 3, 30*time.Second)
	return &QuakeAdapterWebOnly{
		WebOnlyAdapterBase: NewWebOnlyAdapterBase(baseAdapter, "quake"),
	}
}
