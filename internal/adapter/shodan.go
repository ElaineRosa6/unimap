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
		"header":      "http.html",
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
		"server":      "product",
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
		return &model.EngineResult{
			EngineName: s.Name(),
			Error:      "Shodan API key not configured",
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
			// 网络错误可重试
			return true
		},
	}

	err := utils.Retry(retryConfig, func() error {
		// Shodan API endpoint for search
		url := fmt.Sprintf("%s/shodan/host/search", s.baseURL)

		resp, err := s.client.R().
			SetQueryParams(map[string]string{
				"key":   s.apiKey,
				"query": query,
				"page":  fmt.Sprintf("%d", page),
				"limit": fmt.Sprintf("%d", pageSize),
			}).
			Get(url)

		if err != nil {
			return err
		}

		if resp.StatusCode() != 200 {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String())
		}

		var result struct {
			Matches []struct {
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
			} `json:"matches"`
			Total int    `json:"total"`
			Error string `json:"error,omitempty"`
		}

		if err := json.Unmarshal(resp.Body(), &result); err != nil {
			return err
		}

		if result.Error != "" {
			return fmt.Errorf("%s", result.Error)
		}

		// 转换为通用格式
		rawData := []interface{}{}
		for _, match := range result.Matches {
			data := map[string]interface{}{
				"ip":          match.IP,
				"port":        match.Port,
				"protocol":    match.Transport,
				"domain":      match.Domain,
				"hostnames":   match.Hostnames,
				"title":       match.Title,
				"server":      match.Server,
				"http":        match.HTTP,
				"status_code": match.Status,
				"country":     match.Country,
				"region":      match.Region,
				"city":        match.City,
				"asn":         match.ASN,
				"org":         match.Org,
				"isp":         match.ISP,
				"os":          match.OS,
				"product":     match.Product,
				"version":     match.Version,
				"data":        match.Data,
			}
			rawData = append(rawData, data)
		}

		engineResult = &model.EngineResult{
			EngineName: s.Name(),
			RawData:    rawData,
			Total:      result.Total,
			Page:       page,
			HasMore:    (page * pageSize) < result.Total,
		}

		return nil
	})

	if err != nil {
		return &model.EngineResult{
			EngineName: s.Name(),
			Error:      fmt.Sprintf("search error: %v", err),
		}, nil
	}

	return engineResult, nil
}

// Normalize 标准化Shodan结果
func (s *ShodanAdapter) Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error) {
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
			Source: s.Name(),
		}

		// 提取字段
		if ip, ok := data["ip"].(string); ok {
			asset.IP = ip
		}
		if port, ok := data["port"].(float64); ok {
			asset.Port = int(port)
		} else if port, ok := data["port"].(int); ok {
			asset.Port = port
		}
		if proto, ok := data["protocol"].(string); ok {
			asset.Protocol = proto
		}
		if domain, ok := data["domain"].(string); ok {
			asset.Host = domain
		} else if hostnames, ok := data["hostnames"].([]interface{}); ok && len(hostnames) > 0 {
			if hostname, ok := hostnames[0].(string); ok {
				asset.Host = hostname
			}
		}
		if title, ok := data["title"].(string); ok {
			asset.Title = title
		}
		if server, ok := data["server"].(string); ok {
			asset.Server = server
		}
		if body, ok := data["data"].(string); ok {
			if len(body) > 200 {
				asset.BodySnippet = body[:200]
			} else {
				asset.BodySnippet = body
			}
		}
		if status, ok := data["status_code"].(float64); ok {
			asset.StatusCode = int(status)
		} else if status, ok := data["status_code"].(int); ok {
			asset.StatusCode = status
		}

		// 地理信息
		if country, ok := data["country"].(string); ok {
			asset.CountryCode = country
		}
		if region, ok := data["region"].(string); ok {
			asset.Region = region
		}
		if city, ok := data["city"].(string); ok {
			asset.City = city
		}
		if asn, ok := data["asn"].(string); ok {
			asset.ASN = asn
		}
		if org, ok := data["org"].(string); ok {
			asset.Org = org
		}
		if isp, ok := data["isp"].(string); ok {
			asset.ISP = isp
		}

		// 构建URL
		if asset.IP != "" && asset.Port > 0 {
			if asset.Protocol == "" {
				if asset.Port == 443 {
					asset.Protocol = "https"
				} else {
					asset.Protocol = "http"
				}
			}

			// 使用 url.URL 结构体安全构建 URL
			u := &url.URL{
				Scheme: asset.Protocol,
			}
			if asset.Host != "" {
				u.Host = fmt.Sprintf("%s:%d", asset.Host, asset.Port)
			} else {
				u.Host = fmt.Sprintf("%s:%d", asset.IP, asset.Port)
			}
			asset.URL = u.String()

			asset.Extra = data
			assets = append(assets, *asset)
		} else if asset.Host != "" {
			// Keep hostname-only assets (e.g. CDN-backed sites)
			asset.Extra = data
			assets = append(assets, *asset)
		}
	}

	return assets, nil
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
