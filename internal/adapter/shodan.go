package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/utils"
)

// ShodanAdapter Shodan引擎适配器
type ShodanAdapter struct {
	client  *resty.Client
	baseURL string
	apiKey  string
	qps     int
	timeout time.Duration
}

// ShodanSearchResponse is the Shodan Host Search API response.
type ShodanSearchResponse struct {
	Matches []ShodanMatch `json:"matches"`
	Total   int           `json:"total"`
	Error   string        `json:"error,omitempty"`
}

// ShodanMatch is a single Shodan result from the Host Search API.
type ShodanMatch struct {
	IP        string            `json:"ip_str"`
	Port      int               `json:"port"`
	Transport string            `json:"transport"`
	Hostnames []string          `json:"hostnames"`
	Domain    string            `json:"domain"`
	Title     string            `json:"title"`
	Server    string            `json:"server"`
	HTTP      map[string]string `json:"http"`
	Status    int               `json:"status"`
	Country   string            `json:"country_code"`
	Region    string            `json:"region_code"`
	City      string            `json:"city"`
	ASN       string            `json:"asn"`
	Org       string            `json:"org"`
	ISP       string            `json:"isp"`
	OS        string            `json:"os"`
	Product   string            `json:"product"`
	Version   string            `json:"version"`
	Data      string            `json:"data"`
}

// NewShodanAdapter 创建Shodan适配器
func NewShodanAdapter(baseURL, apiKey string, qps int, timeout time.Duration) *ShodanAdapter {
	client := resty.New().
		SetTimeout(timeout).
		SetHeader("User-Agent", "unimap/1.0")

	return &ShodanAdapter{
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
		qps:     qps,
		timeout: timeout,
	}
}

// Name 返回引擎名称
func (s *ShodanAdapter) Name() string {
	return "shodan"
}

// Translate 将UQL AST转换为Shodan查询语法
func (s *ShodanAdapter) Translate(ast *model.UQLAST) (string, error) {
	if ast == nil || ast.Root == nil {
		return "", fmt.Errorf("invalid AST")
	}

	// Shodan使用类似ES的查询语法
	// 简单实现：遍历AST构建查询字符串
	query := s.translateNode(ast.Root)
	return query, nil
}

func (s *ShodanAdapter) translateNode(node *model.UQLNode) string {
	if node == nil {
		return ""
	}

	switch node.Type {
	case "condition":
		field := node.Value
		if len(node.Children) >= 2 {
			op := node.Children[0].Value
			val := node.Children[1].Value

			mappedField := s.mapField(field)

			if op == "IN" {
				// 同字段 OR 用逗号: port:80,443
				values := strings.Split(val, ",")
				quoted := make([]string, 0, len(values))
				for _, v := range values {
					quoted = append(quoted, shodanQuote(strings.TrimSpace(v)))
				}
				return fmt.Sprintf("%s:%s", mappedField, strings.Join(quoted, ","))
			}

			if op == "!=" || op == "<>" {
				return fmt.Sprintf("-%s:%s", mappedField, shodanQuote(val))
			}
			// Shodan 比较操作符: field:>value, field:>=value, field:<value, field:<=value
			if op == ">" || op == ">=" || op == "<" || op == "<=" {
				return fmt.Sprintf("%s:%s%s", mappedField, op, shodanQuote(val))
			}
			return fmt.Sprintf("%s:%s", mappedField, shodanQuote(val))
		}

	case "logical":
		if len(node.Children) >= 2 {
			left := s.translateNode(node.Children[0])
			right := s.translateNode(node.Children[1])
			if node.Value == "OR" {
				// Shodan 不支持跨字段 OR / 括号 — 降级为 AND 语义（结果集更小但安全）
				logger.Warnf("Shodan adapter: OR between (%s) and (%s) degraded to AND (Shodan does not support cross-field OR)", left, right)
			}
			// AND = 空格连接（Shodan 原生语法）
			return fmt.Sprintf("%s %s", left, right)
		}
	}

	return ""
}

// shodanQuote 对 Shodan 值加引号：含空格或特殊字符时包裹双引号，否则原样返回。
// 使用手动转义而非 fmt.Sprintf("%q")，避免 Go 风格的 \n \t 等转义被 Shodan 误解。
func shodanQuote(v string) string {
	if v == "" {
		return v
	}
	// 纯数字或字母/数字/点/连字符组合（如 IP、端口、国家码）不加引号
	for _, c := range v {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_' || c == ':') {
			return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
		}
	}
	return v
}

// mapField 映射统一字段到Shodan字段
func (s *ShodanAdapter) mapField(field string) string {
	mapping := map[string]string{
		"body":        "http.html",
		"title":       "http.title",
		"header":      "http.headers_hash",
		"port":        "port",
		"protocol":    "transport",
		"ip":          "ip",
		"country":     "country",
		"region":      "region",
		"city":        "city",
		"asn":         "asn",
		"org":         "org",
		"isp":         "isp",
		"domain":      "domain",
		"host":        "hostname",
		"server":      "http.server",
		"status_code": "http.status",
		"os":          "os",
		"app":         "product",
		"cert":        "ssl",
	}

	if mapped, ok := mapping[field]; ok {
		return mapped
	}
	return field
}

// Search 执行Shodan搜索
func (s *ShodanAdapter) Search(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error) {
	if s.apiKey == "" {
		return &model.EngineResult{EngineName: s.Name(), Error: "Shodan API key not configured"}, nil
	}
	var engineResult *model.EngineResult
	err := utils.Retry(s.searchRetryConfig(), func() error {
		return s.executeShodanSearch(query, page, pageSize, &engineResult)
	})
	if err != nil {
		return &model.EngineResult{EngineName: s.Name(), Error: fmt.Sprintf("search error: %v", err)}, nil
	}
	return engineResult, nil
}

func (s *ShodanAdapter) searchRetryConfig() utils.RetryConfig {
	return utils.RetryConfig{
		MaxRetries: 3, BaseDelay: 100 * time.Millisecond, MaxDelay: 2 * time.Second,
		Exponential: true, Jitter: true, RetryableFunc: func(err error) bool { return true },
	}
}

// executeShodanSearch 执行单次 Shodan API 调用
func (s *ShodanAdapter) executeShodanSearch(query string, page, pageSize int, result **model.EngineResult) error {
	resp, err := s.client.R().SetQueryParams(map[string]string{
		"key": s.apiKey, "query": query, "page": fmt.Sprintf("%d", page), "limit": fmt.Sprintf("%d", pageSize),
	}).Get(fmt.Sprintf("%s/shodan/host/search", s.baseURL))
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String())
	}
	return parseShodanSearchResponse(resp.Body(), page, pageSize, s.Name(), result)
}

// parseShodanSearchResponse 解析 Shodan 搜索响应
func parseShodanSearchResponse(body []byte, page, pageSize int, engineName string, result **model.EngineResult) error {
	var resp ShodanSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("%s", resp.Error)
	}
	rawData := make([]interface{}, len(resp.Matches))
	for i := range resp.Matches {
		rawData[i] = &resp.Matches[i]
	}
	*result = &model.EngineResult{
		EngineName: engineName, RawData: rawData, Total: resp.Total,
		Page: page, HasMore: (page * pageSize) < resp.Total,
	}
	return nil
}

// Normalize 标准化Shodan结果
func (s *ShodanAdapter) Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error) {
	if raw == nil || len(raw.RawData) == 0 {
		return []model.UnifiedAsset{}, nil
	}

	assets := make([]model.UnifiedAsset, 0, len(raw.RawData))
	for _, item := range raw.RawData {
		m, ok := item.(*ShodanMatch)
		if !ok {
			continue
		}
		if asset := normalizeShodanMatch(m); asset != nil {
			assets = append(assets, *asset)
		}
	}
	return assets, nil
}

// normalizeShodanMatch converts a parsed Shodan API match to a UnifiedAsset.
func normalizeShodanMatch(m *ShodanMatch) *model.UnifiedAsset {
	if m == nil || m.IP == "" {
		return nil
	}
	asset := &model.UnifiedAsset{Source: "shodan",
		IP: m.IP, Port: m.Port, Protocol: m.Transport, Host: m.Domain,
		Title: m.Title, Server: m.Server,
		StatusCode: m.Status, CountryCode: m.Country, Region: m.Region, City: m.City,
		ASN: m.ASN, Org: m.Org, ISP: m.ISP,
	}
	// Use first hostname if no domain
	if asset.Host == "" && len(m.Hostnames) > 0 {
		asset.Host = m.Hostnames[0]
	}
	// Body snippet from data field
	if len(m.Data) > 200 {
		asset.BodySnippet = m.Data[:200]
	} else {
		asset.BodySnippet = m.Data
	}
	// Shodan Host Search v1 does not include per-result timestamps;
	// LastSeen is filled from the Extension's browser DOM extraction path.
	_ = m.OS // OS field available if needed in the future
	_ = m.Product
	_ = m.Version
	_ = m.HTTP

	if asset.IP != "" && asset.Port > 0 {
		buildShodanURL(asset)
		return asset
	}
	if asset.Host != "" {
		return asset
	}
	return nil
}

// buildShodanURL 从 IP/Port/Protocol 构建 URL
func buildShodanURL(asset *model.UnifiedAsset) {
	if asset.Protocol == "" {
		if asset.Port == 443 {
			asset.Protocol = "https"
		} else {
			asset.Protocol = "http"
		}
	}
	u := &url.URL{Scheme: asset.Protocol}
	if asset.Host != "" {
		u.Host = fmt.Sprintf("%s:%d", asset.Host, asset.Port)
	} else {
		u.Host = fmt.Sprintf("%s:%d", asset.IP, asset.Port)
	}
	asset.URL = u.String()
}

// GetQuota 获取Shodan配额信息
func (s *ShodanAdapter) GetQuota() (*model.QuotaInfo, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("Shodan API key not configured")
	}

	// Shodan API endpoint for API info (contains quota)
	url := fmt.Sprintf("%s/api-info", s.baseURL)

	resp, err := s.client.R().
		SetQueryParams(map[string]string{
			"key": s.apiKey,
		}).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}

	if resp.StatusCode() != 200 {
		var apiErr struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(resp.Body(), &apiErr); err == nil && strings.TrimSpace(apiErr.Error) != "" {
			return nil, fmt.Errorf("Shodan API error: %s", strings.TrimSpace(apiErr.Error))
		}
		return nil, fmt.Errorf("Shodan API HTTP %d", resp.StatusCode())
	}

	// Shodan API info response structure
	var result struct {
		Plan         string `json:"plan"`
		Credits      int    `json:"query_credits"`
		ScanCredits  int    `json:"scan_credits"`
		MonitoredIPs int    `json:"monitored_ips"`
		Unlocked     bool   `json:"unlocked"`
		Error        string `json:"error"`
		UsageLimits  struct {
			Credits     int `json:"query_credits"`
			ScanCredits int `json:"scan_credits"`
		} `json:"usage_limits"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	if strings.TrimSpace(result.Error) != "" {
		return nil, fmt.Errorf("Shodan API error: %s", strings.TrimSpace(result.Error))
	}

	total := result.UsageLimits.Credits
	if total < 0 {
		total = 0 // Shodan uses negative values for unlimited/unknown limits.
	}
	used := 0
	if total > 0 {
		used = total - result.Credits
		if used < 0 {
			used = 0
		}
	}

	return &model.QuotaInfo{
		Remaining: result.Credits,
		Total:     total,
		Used:      used,
		Unit:      "query credits",
		Expiry:    "", // Shodan API doesn't return expiry info
	}, nil
}

// IsWebOnly 检查是否为 Web-only 模式
func (s *ShodanAdapter) IsWebOnly() bool {
	return false
}

// ShodanAdapterWebOnly Shodan Web-only模式适配器
type ShodanAdapterWebOnly struct {
	*WebOnlyAdapterBase
}

// NewShodanAdapterWebOnly 创建Shodan Web-only适配器
func NewShodanAdapterWebOnly() *ShodanAdapterWebOnly {
	baseAdapter := NewShodanAdapter("", "", 3, 30*time.Second)
	return &ShodanAdapterWebOnly{
		WebOnlyAdapterBase: NewWebOnlyAdapterBase(baseAdapter, "shodan"),
	}
}
