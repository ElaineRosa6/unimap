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

// QuakeItem is a single result item from the Quake v3 search API.
type QuakeItem struct {
	IP       string         `json:"ip"`
	Port     float64        `json:"port"`
	Hostname string         `json:"hostname"`
	Domain   string         `json:"domain"`
	URL      string         `json:"url"`
	Service  *QuakeService  `json:"service,omitempty"`
	Location *QuakeLocation `json:"location,omitempty"`
}

// QuakeService holds the nested service info in a Quake result.
type QuakeService struct {
	Name       string     `json:"name"`
	HTTP       *QuakeHTTP `json:"http,omitempty"`
	StatusCode float64    `json:"status_code"`
}

// QuakeHTTP holds the HTTP response info inside a Quake service.
type QuakeHTTP struct {
	Title      string  `json:"title"`
	Server     string  `json:"server"`
	StatusCode float64 `json:"status_code"`
}

// QuakeLocation holds geographic location info from a Quake result.
type QuakeLocation struct {
	CountryCode string `json:"country_code"`
	CityCN      string `json:"city_cn"`
	ProvinceCN  string `json:"province_cn"`
}

// quakeSearchRequest is the JSON body for POST /v3/search/quake_service.
type quakeSearchRequest struct {
	Query string `json:"query"`
	Start int    `json:"start"`
	Size  int    `json:"size"`
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
		"body":  "response",
		"title": "title",
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
			return true
		},
	}

	err := utils.Retry(retryConfig, func() error {
		url := fmt.Sprintf("%s/v3/search/quake_service", q.baseURL)

		reqBody := quakeSearchRequest{
			Query: query,
			Start: (page - 1) * pageSize,
			Size:  pageSize,
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

		// Parse Quake response — data can be array or object depending on API version
		var result struct {
			Code    interface{}     `json:"code"`
			Message string          `json:"message"`
			Data    json.RawMessage `json:"data"`
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

		// Try to unmarshal data as []QuakeItem first
		var typedAssets []QuakeItem
		if err := json.Unmarshal(result.Data, &typedAssets); err != nil {
			// Retry: data might be an object wrapping a list
			var obj struct {
				List []QuakeItem `json:"list"`
			}
			if err2 := json.Unmarshal(result.Data, &obj); err2 == nil && len(obj.List) > 0 {
				typedAssets = obj.List
			} else {
				// Try other common keys
				var raw map[string]interface{}
				if json.Unmarshal(result.Data, &raw) == nil {
					for _, key := range []string{"list", "service_list", "data", "items", "records", "services"} {
						if arrJSON, err3 := json.Marshal(raw[key]); err3 == nil {
							var arr []QuakeItem
							if json.Unmarshal(arrJSON, &arr) == nil && len(arr) > 0 {
								typedAssets = arr
								break
							}
						}
					}
				}
				if len(typedAssets) == 0 {
					logger.Warnf("Quake response data could not be parsed as item array")
				}
			}
		}

		rawData := make([]interface{}, len(typedAssets))
		for i := range typedAssets {
			rawData[i] = &typedAssets[i]
		}

		engineResult = &model.EngineResult{
			EngineName: q.Name(),
			RawData:    rawData,
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
		qi, ok := item.(*QuakeItem)
		if !ok {
			continue
		}
		if asset := normalizeQuakeItem(qi, q.Name()); asset != nil {
			assets = append(assets, *asset)
		}
	}

	return assets, nil
}

// normalizeQuakeItem converts a parsed QuakeItem to a UnifiedAsset.
func normalizeQuakeItem(qi *QuakeItem, source string) *model.UnifiedAsset {
	if qi == nil || qi.IP == "" {
		return nil
	}
	asset := &model.UnifiedAsset{
		Source: source,
		IP:     qi.IP,
		Port:   int(qi.Port),
		Host:   firstNonEmpty(qi.Hostname, qi.Domain),
	}
	// Service info
	if qi.Service != nil {
		if qi.Service.Name != "" {
			asset.Protocol = qi.Service.Name
		}
		if qi.Service.HTTP != nil {
			h := qi.Service.HTTP
			asset.Title = h.Title
			asset.Server = h.Server
			if h.StatusCode > 0 {
				asset.StatusCode = int(h.StatusCode)
			}
		}
		if qi.Service.StatusCode > 0 && asset.StatusCode == 0 {
			asset.StatusCode = int(qi.Service.StatusCode)
		}
	}
	// Location
	if qi.Location != nil {
		asset.CountryCode = qi.Location.CountryCode
		asset.City = qi.Location.CityCN
		asset.Region = qi.Location.ProvinceCN
	}

	if asset.IP != "" || asset.Host != "" {
		return asset
	}
	return nil
}

// firstNonEmpty returns the first non-empty string from the given options.
func firstNonEmpty(opts ...string) string {
	for _, s := range opts {
		if s != "" {
			return s
		}
	}
	return ""
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
