package adapter

import (
	"context"
	"encoding/base64"
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

const (
	// FofaDefaultTimeout FOFA默认超时
	FofaDefaultTimeout = 30 * time.Second
	// FofaDefaultQPS FOFA默认QPS
	FofaDefaultQPS = 3
)

// FofaItem is a single result item from the Fofa API.
// The API returns rows as arrays ([][]interface{}); field order
// depends on the "fields" query parameter.
type FofaItem struct {
	IP         string  `json:"ip"`
	Port       float64 `json:"port"` // JSON numbers decode as float64
	Protocol   string  `json:"protocol"`
	Domain     string  `json:"domain"`
	Title      string  `json:"title"`
	Server     string  `json:"server"`
	Header     string  `json:"header"`
	Body       string  `json:"body"`
	Country    string  `json:"country"`
	Region     string  `json:"region"`
	City       string  `json:"city"`
	ASN        string  `json:"asn"`
	Org        string  `json:"org"`
	ISP        string  `json:"isp"`
	StatusCode float64 `json:"status_code"`
}

// FofaAdapter FOFA引擎适配器
type FofaAdapter struct {
	client  *resty.Client
	baseURL string
	apiKey  string
	email   string
	qps     int
	timeout time.Duration
}

// NewFofaAdapter 创建FOFA适配器
func NewFofaAdapter(baseURL, apiKey, email string, qps int, timeout time.Duration) *FofaAdapter {
	client := resty.New().
		SetTimeout(timeout).
		SetHeader("User-Agent", "unimap/1.0")

	return &FofaAdapter{
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
		email:   email,
		qps:     qps,
		timeout: timeout,
	}
}

// Name 返回引擎名称
func (f *FofaAdapter) Name() string {
	return "fofa"
}

// Translate 将UQL AST转换为FOFA查询语法
func (f *FofaAdapter) Translate(ast *model.UQLAST) (string, error) {
	if ast == nil || ast.Root == nil {
		return "", fmt.Errorf("invalid AST")
	}

	// FOFA使用类似ES的查询语法
	// 简单实现：遍历AST构建查询字符串
	query := f.translateNode(ast.Root)
	return query, nil
}

func (f *FofaAdapter) translateNode(node *model.UQLNode) string {
	if node == nil {
		return ""
	}

	switch node.Type {
	case "condition":
		// field= value 或 field IN [values]
		field := node.Value
		if len(node.Children) >= 2 {
			op := node.Children[0].Value
			val := node.Children[1].Value

			if op == "IN" {
				// FOFA不支持IN语法，需要转换为多个OR
				values := strings.Split(val, ",")
				conditions := []string{}
				for _, v := range values {
					conditions = append(conditions, fmt.Sprintf(`%s="%s"`, f.mapField(field), v))
				}
				return "(" + strings.Join(conditions, " || ") + ")"
			}

			// 处理特殊字段映射
			field = f.mapField(field)

			if op == "==" {
				return fmt.Sprintf(`%s=="%s"`, field, val)
			}
			if op == "=" || strings.ToUpper(op) == "CONTAINS" {
				return fmt.Sprintf(`%s="%s"`, field, val)
			}
			if op == "!=" || op == "<>" {
				return fmt.Sprintf(`%s!="%s"`, field, val)
			}
			// Fallback
			return fmt.Sprintf(`%s="%s"`, field, val)
		}

	case "logical":
		if len(node.Children) >= 2 {
			left := f.translateNode(node.Children[0])
			right := f.translateNode(node.Children[1])
			if node.Value == "OR" {
				return fmt.Sprintf("(%s || %s)", left, right)
			}
			return fmt.Sprintf("(%s && %s)", left, right)
		}
	}

	return ""
}

// safeRowField safely extracts a field from a row by index, returning nil if out of bounds.
func safeRowField(row []interface{}, idx int) interface{} {
	if idx < len(row) {
		return row[idx]
	}
	return nil
}

// fofaRowToItem maps a Fofa API row ([]interface{}) to a FofaItem struct
// using the provided field name order.
func fofaRowToItem(row []interface{}, fieldNames []string) *FofaItem {
	item := &FofaItem{}
	for j, name := range fieldNames {
		if j >= len(row) {
			break
		}
		v := row[j]
		if v == nil {
			continue
		}
		switch name {
		case "ip":
			item.IP, _ = v.(string)
		case "port":
			if f, ok := v.(float64); ok {
				item.Port = f
			}
		case "protocol":
			item.Protocol, _ = v.(string)
		case "domain":
			item.Domain, _ = v.(string)
		case "title":
			item.Title, _ = v.(string)
		case "server":
			item.Server, _ = v.(string)
		case "header":
			item.Header, _ = v.(string)
		case "body":
			item.Body, _ = v.(string)
		case "country":
			item.Country, _ = v.(string)
		case "region":
			item.Region, _ = v.(string)
		case "city":
			item.City, _ = v.(string)
		case "asn":
			item.ASN, _ = v.(string)
		case "org":
			item.Org, _ = v.(string)
		case "isp":
			item.ISP, _ = v.(string)
		case "status_code":
			if f, ok := v.(float64); ok {
				item.StatusCode = f
			}
		}
	}
	return item
}

// mapField 映射统一字段到FOFA字段
func (f *FofaAdapter) mapField(field string) string {
	mapping := map[string]string{
		"body":        "body",
		"title":       "title",
		"header":      "header",
		"port":        "port",
		"protocol":    "protocol",
		"ip":          "ip",
		"country":     "country",
		"region":      "region",
		"city":        "city",
		"asn":         "asn",
		"org":         "org",
		// "isp" removed — FOFA has no isp field (B-1a)
		"domain":      "domain",
		"host":        "host",
		"server":      "server",
		"status_code": "status_code",
		"os":          "os",
		"app":         "app",
		"cert":            "cert",
		"cert.subject.cn": "cert.subject.cn",
		"cert.issuer.cn":  "cert.issuer.cn",
		"url":         "host",
	}

	if mapped, ok := mapping[field]; ok {
		return mapped
	}
	return field
}

// searchWithFields 使用指定字段执行FOFA搜索
func (f *FofaAdapter) searchWithFields(url, encodedQuery string, page, pageSize int, fields string) (*resty.Response, error) {
	return f.client.R().
		SetQueryParams(map[string]string{
			"email":   f.email,
			"key":     f.apiKey,
			"qbase64": encodedQuery,
			"page":    fmt.Sprintf("%d", page),
			"size":    fmt.Sprintf("%d", pageSize),
			"fields":  fields,
		}).
		Get(url)
}

// Search 执行FOFA搜索
func (f *FofaAdapter) Search(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error) {
	if f.apiKey == "" || f.email == "" {
		return &model.EngineResult{EngineName: f.Name(), Error: "FOFA API key or email not configured"}, nil
	}
	var engineResult *model.EngineResult
	err := utils.Retry(fofaSearchRetryConfig(), func() error {
		return f.executeFofaSearch(query, page, pageSize, &engineResult)
	})
	if err != nil {
		return &model.EngineResult{EngineName: f.Name(), Error: fmt.Sprintf("search error: %v", err)}, nil
	}
	return engineResult, nil
}

func fofaSearchRetryConfig() utils.RetryConfig {
	return utils.RetryConfig{
		MaxRetries: 3, BaseDelay: 100 * time.Millisecond, MaxDelay: 2 * time.Second,
		Exponential: true, Jitter: true,
		RetryableFunc: func(err error) bool {
			errStr := err.Error()
			return !strings.Contains(errStr, "HTTP 401") && !strings.Contains(errStr, "HTTP 403") && !strings.Contains(errStr, "820031")
		},
	}
}

// executeFofaSearch 执行单次 FOFA API 调用（含字段权限降级）
func (f *FofaAdapter) executeFofaSearch(query string, page, pageSize int, result **model.EngineResult) error {
	encodedQuery := base64.StdEncoding.EncodeToString([]byte(query))
	url := fmt.Sprintf("%s/api/v1/search/all", f.baseURL)
	allFields := "ip,port,protocol,domain,title,server,header,country,region,city,asn,org,isp,status_code"
	activeFields := allFields

	resp, err := f.searchWithFields(url, encodedQuery, page, pageSize, allFields)
	if err == nil && resp.StatusCode() == 200 {
		if degraded := f.degradeFieldsOnPermissionError(resp.Body()); degraded != "" {
			activeFields = degraded
			resp, err = f.searchWithFields(url, encodedQuery, page, pageSize, activeFields)
		}
	}
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String())
	}
	return parseFofaSearchResponse(resp.Body(), activeFields, page, pageSize, f.Name(), result)
}

// degradeFieldsOnPermissionError 检查是否为字段权限错误，返回降级字段或空串
func (f *FofaAdapter) degradeFieldsOnPermissionError(body []byte) string {
	var errCheck struct {
		Err    interface{} `json:"error"`
		ErrMsg string      `json:"errmsg"`
	}
	if json.Unmarshal(body, &errCheck) != nil {
		return ""
	}
	errMsg := errCheck.ErrMsg
	if errMsg == "" {
		if s, ok := errCheck.Err.(string); ok {
			errMsg = s
		}
	}
	if strings.Contains(errMsg, "没有权限") || strings.Contains(errMsg, "820001") {
		logger.Warnf("fofa: 字段权限不足，降级到基础字段重试: %s", errMsg)
		return "ip,port,protocol,domain,title,server,country"
	}
	return ""
}

// parseFofaSearchResponse 解析 FOFA 搜索响应
func parseFofaSearchResponse(body []byte, activeFields string, page, pageSize int, engineName string, result **model.EngineResult) error {
	var resp struct {
		Mode    string          `json:"mode"`
		Results [][]interface{} `json:"results"`
		Total   int             `json:"total"`
		Err     interface{}     `json:"error"`
		ErrMsg  string          `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	hasError := false
	errMsg := ""
	if b, ok := resp.Err.(bool); ok {
		hasError = b
	} else if s, ok := resp.Err.(string); ok && s != "" && s != "false" {
		hasError = true
		errMsg = s
	}
	if hasError {
		if errMsg == "" {
			errMsg = resp.ErrMsg
		}
		if errMsg == "" {
			errMsg = "FOFA API reported an error (unknown cause)"
		}
		return fmt.Errorf("FOFA API error: %s", errMsg)
	}
	fieldNames := strings.Split(activeFields, ",")
	rawData := make([]interface{}, len(resp.Results))
	for i, row := range resp.Results {
		rawData[i] = fofaRowToItem(row, fieldNames)
	}
	*result = &model.EngineResult{
		EngineName: engineName, RawData: rawData, Total: resp.Total,
		Page: page, HasMore: (page * pageSize) < resp.Total,
	}
	return nil
}

// Normalize 标准化FOFA结果
func (f *FofaAdapter) Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error) {
	assets := make([]model.UnifiedAsset, 0, len(raw.RawData))
	if raw == nil || len(raw.RawData) == 0 {
		return assets, nil
	}
	for _, item := range raw.RawData {
		data, ok := item.(*FofaItem)
		if !ok {
			continue
		}
		if asset := normalizeFofaItem(data); asset != nil {
			assets = append(assets, *asset)
		}
	}
	return assets, nil
}

// normalizeFofaItem 解析单条 FOFA 数据
func normalizeFofaItem(item *FofaItem) *model.UnifiedAsset {
	if item == nil || item.IP == "" {
		return nil
	}
	asset := &model.UnifiedAsset{
		Source:     "fofa",
		IP:         item.IP,
		Port:       int(item.Port),
		Protocol:   item.Protocol,
		Host:       item.Domain,
		Title:      item.Title,
		Server:     item.Server,
		CountryCode: item.Country,
		Region:     item.Region,
		City:       item.City,
		ASN:        item.ASN,
		Org:        item.Org,
		ISP:        item.ISP,
		StatusCode: int(item.StatusCode),
	}
	// Body snippet: prefer body field, fall back to header
	snippet := item.Body
	if snippet == "" {
		snippet = item.Header
	}
	if len(snippet) > 200 {
		asset.BodySnippet = snippet[:200]
	} else {
		asset.BodySnippet = snippet
	}

	if asset.IP != "" && asset.Port > 0 {
		buildFofaURL(asset)
		return asset
	}
	if asset.Host != "" {
		return asset
	}
	return nil
}

// buildFofaURL 从 IP/Port/Protocol 构建 URL
func buildFofaURL(asset *model.UnifiedAsset) {
	if asset.Protocol == "" {
		if asset.Port == 443 { asset.Protocol = "https" } else { asset.Protocol = "http" }
	}
	u := &url.URL{Scheme: asset.Protocol}
	if asset.Host != "" {
		u.Host = fmt.Sprintf("%s:%d", asset.Host, asset.Port)
	} else {
		u.Host = fmt.Sprintf("%s:%d", asset.IP, asset.Port)
	}
	asset.URL = u.String()
}

// GetQuota 获取FOFA配额信息
func (f *FofaAdapter) GetQuota() (*model.QuotaInfo, error) {
	if f.apiKey == "" || f.email == "" {
		return nil, fmt.Errorf("FOFA API key or email not configured")
	}

	// FOFA API endpoint for user info (contains quota)
	url := fmt.Sprintf("%s/api/v1/info/my", f.baseURL)

	resp, err := f.client.R().
		SetQueryParams(map[string]string{
			"email": f.email,
			"key":   f.apiKey,
		}).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), sanitizeBody(resp.String()))
	}

	// 记录响应体，方便调试
	logger.Debugf("FOFA quota response: %s", sanitizeBody(resp.String()))

	// FOFA quota response structure
	var result struct {
		Error           bool   `json:"error"`
		Email           string `json:"email"`
		Username        string `json:"username"`
		Category        string `json:"category"`
		IsVIP           bool   `json:"isvip"`
		VIPLevel        int    `json:"vip_level"`
		RemainFreePoint int    `json:"remain_free_point"`
		RemainAPIQuery  int    `json:"remain_api_query"`
		RemainAPIData   int    `json:"remain_api_data"`
		Message         string `json:"message"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	if result.Error {
		return nil, fmt.Errorf("%s", result.Message)
	}

	// 计算配额信息 - 简化逻辑，直接使用API返回的值
	// FOFA API响应结构：
	// - remain_free_point: 剩余免费点数
	// - remain_api_query: 剩余API查询次数
	// - remain_api_data: 剩余API数据条数

	total := 0
	remain := 0

	// 优先使用API查询次数
	if result.RemainAPIQuery > 0 {
		remain = result.RemainAPIQuery
		// 对于付费用户，假设总配额为剩余+已用（保守估计）
		// 由于API不直接返回总数，使用剩余作为最小估计
		total = remain
	} else if result.RemainFreePoint > 0 {
		// 使用免费点数
		remain = result.RemainFreePoint
		total = remain
	}

	// 计算已用配额（如果有总数）
	used := 0
	if total > 0 {
		used = total - remain
	}

	// 确保数值合理
	if remain < 0 {
		remain = 0
	}
	if used < 0 {
		used = 0
	}

	// 仅记录必要的配额信息（不记录敏感用户详情）
	logger.Debugf("FOFA quota: total=%d, used=%d, remain=%d", total, used, remain)

	return &model.QuotaInfo{
		Remaining: remain,
		Total:     total,
		Used:      used,
		Unit:      "queries",
		Expiry:    "", // FOFA API doesn't return expiry info
	}, nil
}

// IsWebOnly 检查是否为 Web-only 模式
func (f *FofaAdapter) IsWebOnly() bool {
	return false
}

// FofaAdapterWebOnly FOFA Web-only模式适配器
type FofaAdapterWebOnly struct {
	*WebOnlyAdapterBase
}

// NewFofaAdapterWebOnly 创建FOFA Web-only适配器
func NewFofaAdapterWebOnly() *FofaAdapterWebOnly {
	baseAdapter := NewFofaAdapter("", "", "", FofaDefaultQPS, FofaDefaultTimeout)
	return &FofaAdapterWebOnly{
		WebOnlyAdapterBase: NewWebOnlyAdapterBase(baseAdapter, "fofa"),
	}
}
