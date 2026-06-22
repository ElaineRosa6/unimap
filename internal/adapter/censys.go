package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/utils"
)

// CensysAdapter Censys引擎适配器
type CensysAdapter struct {
	client     *resty.Client
	baseURL    string
	apiID      string
	apiSecret  string
	useBearer  bool // true when using new-format personal API key (Bearer auth)
	qps        int
	timeout    time.Duration
}

// NewCensysAdapter 创建Censys适配器
func NewCensysAdapter(baseURL, apiID, apiSecret string, qps int, timeout time.Duration) *CensysAdapter {
	client := resty.New().
		SetTimeout(timeout).
		SetHeader("User-Agent", "unimap/1.0")

	// New-format Censys personal API keys start with "censys_" and use Bearer
	// auth instead of the legacy API_ID:API_Secret Basic Auth.
	useBearer := apiSecret == "" && strings.HasPrefix(apiID, "censys_")

	return &CensysAdapter{
		client:     client,
		baseURL:    baseURL,
		apiID:      apiID,
		apiSecret:  apiSecret,
		useBearer:  useBearer,
		qps:        qps,
		timeout:    timeout,
	}
}

// setAuth applies the appropriate authentication to a resty request.
func (c *CensysAdapter) setAuth(r *resty.Request) *resty.Request {
	if c.useBearer {
		return r.SetAuthToken(c.apiID)
	}
	return r.SetBasicAuth(c.apiID, c.apiSecret)
}

// Name 返回引擎名称
func (c *CensysAdapter) Name() string {
	return "censys"
}

// IsWebOnly 检查是否为 Web-only 模式
func (c *CensysAdapter) IsWebOnly() bool {
	return false
}

// censysQuote 对 Censys 值加引号：含特殊字符时包裹双引号，否则原样返回。
// 使用手动转义而非 fmt.Sprintf("%q")，避免 Go 风格的 \n \t 等转义被 Censys 误解。
func censysQuote(v string) string {
	if v == "" {
		return v
	}
	// 纯字母/数字/点/连字符/下划线/冒号/斜杠组合（如 IP、端口、国家码、路径）不加引号
	for _, c := range v {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_' || c == ':' || c == '/') {
			return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
		}
	}
	return v
}

// mapField 映射统一字段到Censys字段
func (c *CensysAdapter) mapField(field string) string {
	mapping := map[string]string{
		"body":        "services.http.response.body",
		"title":       "services.http.response.html_title",
		"header":      "services.http.response.headers.raw",
		"port":        "services.port",
		"protocol":    "services.service_name",
		"ip":          "ip",
		"country":     "location.country_code",
		"region":      "location.province",
		"city":        "location.city",
		"asn":         "autonomous_system.asn",
		"org":         "autonomous_system.name",
		"isp":         "autonomous_system.name",
		"domain":      "dns.names",
		"host":        "dns.names",
		"server":      "services.http.response.headers.Server",
		"status_code": "services.http.response.status_code",
		"os":          "operating_system",
		"app":         "services.software.product",
		"cert":        "services.tls.certificates.leaf.subject",
		"url":         "dns.names",
	}

	if mapped, ok := mapping[field]; ok {
		return mapped
	}
	return field
}

// Translate 将UQL AST转换为Censys查询语法
func (c *CensysAdapter) Translate(ast *model.UQLAST) (string, error) {
	if ast == nil || ast.Root == nil {
		return "", fmt.Errorf("invalid AST")
	}

	query := c.translateNode(ast.Root)
	return query, nil
}

func (c *CensysAdapter) translateNode(node *model.UQLNode) string {
	if node == nil {
		return ""
	}

	switch node.Type {
	case "condition":
		field := node.Value
		if len(node.Children) >= 2 {
			op := node.Children[0].Value
			val := node.Children[1].Value

			mappedField := c.mapField(field)

			if op == "IN" {
				// 同字段 OR: (field:val1 OR field:val2 OR ...)
				values := strings.Split(val, ",")
				clauses := make([]string, 0, len(values))
				for _, v := range values {
					clauses = append(clauses, fmt.Sprintf("%s:%s", mappedField, censysQuote(strings.TrimSpace(v))))
				}
				return "(" + strings.Join(clauses, " OR ") + ")"
			}

			if op == "!=" || op == "<>" {
				// Censys uses NOT field:value (not -field:value)
				return fmt.Sprintf("NOT %s:%s", mappedField, censysQuote(val))
			}
			// Censys 比较操作符: field:>value, field:>=value, field:<value, field:<=value
			if op == ">" || op == ">=" || op == "<" || op == "<=" {
				return fmt.Sprintf("%s:%s%s", mappedField, op, censysQuote(val))
			}
			return fmt.Sprintf("%s:%s", mappedField, censysQuote(val))
		}

	case "logical":
		if len(node.Children) >= 2 {
			left := c.translateNode(node.Children[0])
			right := c.translateNode(node.Children[1])
			if node.Value == "OR" {
				// Censys 原生支持 OR
				return fmt.Sprintf("(%s OR %s)", left, right)
			}
			// AND = 空格连接（Censys 原生语法）
			return fmt.Sprintf("(%s AND %s)", left, right)
		}
	}

	return ""
}

// CensysSearchResult Censys搜索结果
type CensysSearchResult struct {
	Result struct {
		Hits  []json.RawMessage `json:"hits"`
		Total int               `json:"total"`
		Links struct {
			Next string `json:"next"`
		} `json:"links"`
	} `json:"result"`
}

// Search 执行Censys搜索（v3 API, Bearer token）
func (c *CensysAdapter) Search(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error) {
	if c.apiID == "" {
		return &model.EngineResult{EngineName: c.Name(), Error: "Censys API key not configured"}, nil
	}
	var engineResult *model.EngineResult
	err := utils.Retry(c.searchRetryConfig(), func() error {
		return c.executeCensysSearch(ctx, query, page, pageSize, &engineResult)
	})
	if err != nil {
		return &model.EngineResult{EngineName: c.Name(), Error: fmt.Sprintf("search error: %v", err)}, nil
	}
	return engineResult, nil
}

func (c *CensysAdapter) searchRetryConfig() utils.RetryConfig {
	return utils.RetryConfig{
		MaxRetries: 3, BaseDelay: 100 * time.Millisecond, MaxDelay: 2 * time.Second,
		Exponential: true, Jitter: true, RetryableFunc: func(err error) bool { return true },
	}
}

// executeCensysSearch 执行 Censys v3 单 IP 查询
// Free tier supports GET /v3/global/asset/host/{ip} with Bearer token.
// Keyword/bulk search requires API ID+Secret (v2) or paid plan (v3).
func (c *CensysAdapter) executeCensysSearch(ctx context.Context, query string, page, pageSize int, result **model.EngineResult) error {
	// Extract IP from query — v3 free tier only supports single-host lookups
	ip := extractIP(query)
	if ip == "" {
		// Try as a bare IP
		ip = strings.TrimSpace(query)
		if net.ParseIP(ip) == nil {
			return fmt.Errorf("Censys free tier only supports IP-based queries (host lookup)")
		}
	}

	searchURL := fmt.Sprintf("%s/v3/global/asset/host/%s", c.baseURL, ip)
	resp, err := c.setAuth(c.client.R()).
		SetHeader("Accept", "application/json").
		Get(searchURL)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode(), sanitizeBody(resp.String()))
	}
	return parseCensysV3HostResponse(resp.Body(), page, pageSize, c.Name(), result)
}

// extractIP tries to extract an IPv4 or IPv6 address from a Censys query string.
func extractIP(query string) string {
	// Common patterns: "ip:8.8.8.8", "ip=8.8.8.8", bare "8.8.8.8"
	candidates := strings.FieldsFunc(query, func(r rune) bool {
		return r == ':' || r == '=' || r == ' ' || r == '"' || r == '\''
	})
	for _, c := range candidates {
		if ip := net.ParseIP(c); ip != nil {
			return c
		}
	}
	return ""
}

// parseCensysV3HostResponse parses the v3 single-host lookup response
// Response: {"result":{"resource":{"ip":"8.8.8.8", "services":[...], ...}}}
func parseCensysV3HostResponse(body []byte, page, pageSize int, engineName string, result **model.EngineResult) error {
	var resp struct {
		Result struct {
			Resource struct {
				IP                string                     `json:"ip"`
				Location          map[string]interface{}     `json:"location"`
				AutonomousSystem  map[string]interface{}     `json:"autonomous_system"`
				Services          []map[string]interface{}   `json:"services"`
				LastUpdatedAt     string                     `json:"last_updated_at"`
			} `json:"resource"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}

	resource := resp.Result.Resource
	if resource.IP == "" {
		return fmt.Errorf("Censys v3 host response missing IP")
	}

	// Build raw data: each service becomes an entry with the host info merged
	rawData := make([]interface{}, 0, len(resource.Services)+1)
	for _, svc := range resource.Services {
		entry := map[string]interface{}{
			"ip": resource.IP,
		}
		for k, v := range svc {
			entry[k] = v
		}
		if resource.Location != nil {
			entry["location"] = resource.Location
		}
		if resource.AutonomousSystem != nil {
			entry["autonomous_system"] = resource.AutonomousSystem
		}
		rawData = append(rawData, entry)
	}
	// If no services, still return the host itself
	if len(rawData) == 0 {
		rawData = append(rawData, map[string]interface{}{
			"ip":                 resource.IP,
			"location":           resource.Location,
			"autonomous_system":  resource.AutonomousSystem,
			"last_updated_at":    resource.LastUpdatedAt,
		})
	}

	total := len(rawData)
	*result = &model.EngineResult{
		EngineName: engineName, RawData: rawData, Total: total,
		Page: page, HasMore: false,
	}
	return nil
}

// Normalize 标准化Censys结果
// Censys hosts have nested services[]. Each host can produce multiple UnifiedAssets (one per service).
func (c *CensysAdapter) Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error) {
	assets := make([]model.UnifiedAsset, 0, len(raw.RawData))
	if raw == nil || len(raw.RawData) == 0 {
		return assets, nil
	}
	for _, item := range raw.RawData {
		data, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		ip, _ := data["ip"].(string)
		if ip == "" {
			continue
		}
		assets = append(assets, c.normalizeCensysHost(data, ip)...)
	}
	return assets, nil
}

// normalizeCensysHost 解析单个 Censys 主机（可能产生多个资产，每个 service 一个）
func (c *CensysAdapter) normalizeCensysHost(data map[string]interface{}, ip string) []model.UnifiedAsset {
	cc, region, city := extractCensysLocation(data)
	asn, org := extractCensysNetwork(data)
	host := extractCensysHost(data)

	base := model.UnifiedAsset{
		IP: ip, Source: c.Name(), CountryCode: cc, Region: region, City: city,
		ASN: asn, Org: org, ISP: org, Host: host, Extra: data,
	}

	services, _ := data["services"].([]interface{})
	if len(services) == 0 {
		return []model.UnifiedAsset{base}
	}

	assets := make([]model.UnifiedAsset, 0, len(services))
	for _, svc := range services {
		svcData, ok := svc.(map[string]interface{})
		if !ok {
			continue
		}
		asset := base // copy base fields
		c.applyCensysServiceFields(svcData, &asset)
		assets = append(assets, asset)
	}
	return assets
}

// extractCensysLocation 从主机数据提取地理位置
func extractCensysLocation(data map[string]interface{}) (countryCode, region, city string) {
	if loc, ok := data["location"].(map[string]interface{}); ok {
		countryCode, _ = loc["country_code"].(string)
		region, _ = loc["province"].(string)
		city, _ = loc["city"].(string)
	}
	return
}

// extractCensysNetwork 从主机数据提取 ASN 和组织
func extractCensysNetwork(data map[string]interface{}) (asn, org string) {
	if as, ok := data["autonomous_system"].(map[string]interface{}); ok {
		if v, ok := as["asn"].(string); ok {
			asn = v
		} else if v, ok := as["asn"].(float64); ok {
			asn = fmt.Sprintf("AS%d", int(v))
		}
		org, _ = as["name"].(string)
	}
	return
}

// extractCensysHost 从 DNS 数据提取主机名
func extractCensysHost(data map[string]interface{}) string {
	if dns, ok := data["dns"].(map[string]interface{}); ok {
		if names, ok := dns["names"].([]interface{}); ok && len(names) > 0 {
			if name, ok := names[0].(string); ok {
				return name
			}
		}
	}
	return ""
}

// applyCensysServiceFields 将单个 service 的字段应用到资产
func (c *CensysAdapter) applyCensysServiceFields(svcData map[string]interface{}, asset *model.UnifiedAsset) {
	if port, ok := svcData["port"].(float64); ok {
		asset.Port = int(port)
	} else if port, ok := svcData["port"].(int); ok {
		asset.Port = port
	}
	if proto, ok := svcData["service_name"].(string); ok {
		asset.Protocol = proto
	}
	c.parseCensysHTTPResponse(svcData, asset)
	c.parseCensysTLS(svcData, asset)
	c.parseCensysSoftware(svcData, asset)
	buildCensysURL(asset)
}

// parseCensysHTTPResponse 解析 HTTP 响应字段
func (c *CensysAdapter) parseCensysHTTPResponse(svcData map[string]interface{}, asset *model.UnifiedAsset) {
	httpResp, ok := svcData["http"].(map[string]interface{})
	if !ok {
		return
	}
	resp, ok := httpResp["response"].(map[string]interface{})
	if !ok {
		return
	}
	if title, ok := resp["html_title"].(string); ok {
		asset.Title = title
	}
	if status, ok := resp["status_code"].(float64); ok {
		asset.StatusCode = int(status)
	} else if status, ok := resp["status_code"].(int); ok {
		asset.StatusCode = status
	}
	if body, ok := resp["body"].(string); ok {
		if len(body) > 200 {
			asset.BodySnippet = body[:200]
		} else {
			asset.BodySnippet = body
		}
	}
	if headers, ok := resp["headers"].(map[string]interface{}); ok {
		if server, ok := headers["Server"].(string); ok {
			asset.Server = server
		}
	}
}

// parseCensysTLS 解析 TLS 证书信息
func (c *CensysAdapter) parseCensysTLS(svcData map[string]interface{}, asset *model.UnifiedAsset) {
	tls, ok := svcData["tls"].(map[string]interface{})
	if !ok {
		return
	}
	certs, ok := tls["certificates"].(map[string]interface{})
	if !ok {
		return
	}
	leaf, ok := certs["leaf"].(map[string]interface{})
	if !ok {
		return
	}
	if subject, ok := leaf["subject"].(string); ok && asset.Host == "" {
		asset.Host = subject
	}
}

// parseCensysSoftware 解析软件信息
func (c *CensysAdapter) parseCensysSoftware(svcData map[string]interface{}, asset *model.UnifiedAsset) {
	sw, ok := svcData["software"].([]interface{})
	if !ok || len(sw) == 0 {
		return
	}
	if prod, ok := sw[0].(map[string]interface{}); ok {
		if name, ok := prod["product"].(string); ok {
			asset.Title = name
		}
	}
}

// buildCensysURL 从 IP/Port/Protocol 构建 URL
func buildCensysURL(asset *model.UnifiedAsset) {
	if asset.IP == "" || asset.Port <= 0 {
		return
	}
	if asset.Protocol == "" {
		if asset.Port == 443 { asset.Protocol = "https" } else { asset.Protocol = "http" }
	}
	u := &url.URL{Scheme: strings.ToLower(asset.Protocol)}
	if asset.Host != "" {
		u.Host = fmt.Sprintf("%s:%d", asset.Host, asset.Port)
	} else {
		u.Host = fmt.Sprintf("%s:%d", asset.IP, asset.Port)
	}
	asset.URL = u.String()
}

// GetQuota 获取Censys配额信息（v3 免费版无独立配额端点）
func (c *CensysAdapter) GetQuota() (*model.QuotaInfo, error) {
	return nil, fmt.Errorf("Censys free tier: quota API not available")
}

// CensysAdapterWebOnly Censys Web-only模式适配器
type CensysAdapterWebOnly struct {
	*WebOnlyAdapterBase
}

// NewCensysAdapterWebOnly 创建Censys Web-only适配器
func NewCensysAdapterWebOnly() *CensysAdapterWebOnly {
	baseAdapter := NewCensysAdapter("", "", "", 3, 30*time.Second)
	return &CensysAdapterWebOnly{
		WebOnlyAdapterBase: NewWebOnlyAdapterBase(baseAdapter, "censys"),
	}
}
