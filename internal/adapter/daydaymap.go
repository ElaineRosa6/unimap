package adapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/utils"
)

const (
	// DayDayMapDefaultTimeout DayDayMap默认超时
	DayDayMapDefaultTimeout = 30 * time.Second
	// DayDayMapDefaultQPS DayDayMap默认QPS
	DayDayMapDefaultQPS = 3
)

// DayDayMapItem is a single result item from the DayDayMap API.
// API returns list items as JSON objects with snake_case keys.
type DayDayMapItem struct {
	IP         string  `json:"ip"`
	Port       float64 `json:"port"`
	Protocol   string  `json:"protocol"`
	Domain     string  `json:"domain"`
	Title      string  `json:"title"`
	Server     string  `json:"server"`
	Body       string  `json:"body"`
	StatusCode float64 `json:"status_code"`
	Country    string  `json:"country"`
	Province   string  `json:"province"`
	City       string  `json:"city"`
	ASN        string  `json:"asn"`
	Org        string  `json:"org"`
	ISP        string  `json:"isp"`
}

// DayDayMapAdapter DayDayMap引擎适配器
type DayDayMapAdapter struct {
	client  *resty.Client
	baseURL string
	apiKey  string
	qps     int
	timeout time.Duration
}

// NewDayDayMapAdapter 创建DayDayMap适配器
func NewDayDayMapAdapter(baseURL, apiKey string, qps int, timeout time.Duration) *DayDayMapAdapter {
	client := resty.New().
		SetTimeout(timeout).
		SetHeader("User-Agent", "unimap/1.0")

	return &DayDayMapAdapter{
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
		qps:     qps,
		timeout: timeout,
	}
}

// Name 返回引擎名称
func (d *DayDayMapAdapter) Name() string {
	return "daydaymap"
}

// IsWebOnly 检查是否为 Web-only 模式
func (d *DayDayMapAdapter) IsWebOnly() bool {
	return false
}

// Translate 将UQL AST转换为DayDayMap查询语法
func (d *DayDayMapAdapter) Translate(ast *model.UQLAST) (string, error) {
	if ast == nil || ast.Root == nil {
		return "", fmt.Errorf("invalid AST")
	}

	query := d.translateNode(ast.Root)
	return query, nil
}

func (d *DayDayMapAdapter) translateNode(node *model.UQLNode) string {
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
				// DayDayMap不支持IN语法，转换为多个OR
				values := strings.Split(val, ",")
				conditions := []string{}
				for _, v := range values {
					conditions = append(conditions, fmt.Sprintf(`%s="%s"`, d.mapField(field), v))
				}
				return "(" + strings.Join(conditions, " || ") + ")"
			}

			// 处理字段映射
			field = d.mapField(field)

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
			left := d.translateNode(node.Children[0])
			right := d.translateNode(node.Children[1])
			if node.Value == "OR" {
				return fmt.Sprintf("(%s || %s)", left, right)
			}
			return fmt.Sprintf("(%s && %s)", left, right)
		}
	}

	return ""
}

// mapField 映射统一字段到DayDayMap字段
func (d *DayDayMapAdapter) mapField(field string) string {
	mapping := map[string]string{
		"body":            "web.body",
		"title":           "web.title",
		"header":          "web.header",
		"port":            "ip.port",
		"protocol":        "protocol.service",
		"ip":              "ip",
		"country":         "ip.country",
		"region":          "ip.province",
		"city":            "ip.city",
		"asn":             "asn.number",
		"org":             "org.name",
		"isp":             "ip.isp",
		"domain":          "domain",
		"host":            "domain",
		"server":          "web.server",
		"status_code":     "web.status_code",
		"os":              "ip.os",
		"app":             "app.name",
		"cert":            "cert.subject",
		"cert.subject.cn": "cert.subject.cn",
		"cert.issuer.cn":  "cert.issuer.cn",
		"url":             "domain",
	}

	if mapped, ok := mapping[field]; ok {
		return mapped
	}
	return field
}

// Search 执行DayDayMap搜索
func (d *DayDayMapAdapter) Search(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error) {
	if d.apiKey == "" {
		return &model.EngineResult{EngineName: d.Name(), Error: "DayDayMap API key not configured"}, nil
	}
	var engineResult *model.EngineResult
	err := utils.Retry(d.searchRetryConfig(), func() error {
		return d.executeDayDayMapSearch(query, page, pageSize, &engineResult)
	})
	if err != nil {
		return &model.EngineResult{EngineName: d.Name(), Error: fmt.Sprintf("search error: %v", err)}, nil
	}
	return engineResult, nil
}

func (d *DayDayMapAdapter) searchRetryConfig() utils.RetryConfig {
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

// dayDayMapSearchRequest is the JSON body for POST /api/v1/raymap/search/all.
type dayDayMapSearchRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Keyword  string `json:"keyword"`
}

// executeDayDayMapSearch 执行单次 DayDayMap API 调用
// API: POST /api/v1/raymap/search/all, header api-key, JSON body with base64 keyword
func (d *DayDayMapAdapter) executeDayDayMapSearch(query string, page, pageSize int, result **model.EngineResult) error {
	searchURL := fmt.Sprintf("%s/api/v1/raymap/search/all", d.baseURL)
	// DayDayMap requires the search keyword to be base64-encoded
	keyword := base64.StdEncoding.EncodeToString([]byte(query))
	resp, err := d.client.R().
		SetHeader("api-key", d.apiKey).
		SetHeader("Content-Type", "application/json").
		SetBody(dayDayMapSearchRequest{
			Page:     page,
			PageSize: pageSize,
			Keyword:  keyword,
		}).
		Post(searchURL)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode(), sanitizeBody(resp.String()))
	}
	return parseDayDayMapSearchResponse(resp.Body(), page, pageSize, d.Name(), result)
}

// parseDayDayMapSearchResponse 解析 DayDayMap 搜索响应
// Response: {"code":200, "data":{"list":[...], "total":N, "page":1, "page_size":10}, "msg":"检索成功"}
func parseDayDayMapSearchResponse(body []byte, page, pageSize int, engineName string, result **model.EngineResult) error {
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			List  []DayDayMapItem `json:"list"`
			Total int             `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if resp.Code != 200 {
		errMsg := resp.Msg
		if errMsg == "" {
			errMsg = "DayDayMap API reported an error (unknown cause)"
		}
		return fmt.Errorf("DayDayMap API error: %s", errMsg)
	}
	rawData := make([]interface{}, len(resp.Data.List))
	for i := range resp.Data.List {
		rawData[i] = &resp.Data.List[i]
	}
	*result = &model.EngineResult{
		EngineName: engineName, RawData: rawData, Total: resp.Data.Total,
		Page: page, HasMore: (page * pageSize) < resp.Data.Total,
	}
	return nil
}

// Normalize 标准化DayDayMap结果
func (d *DayDayMapAdapter) Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error) {
	if raw == nil || len(raw.RawData) == 0 {
		return []model.UnifiedAsset{}, nil
	}
	assets := make([]model.UnifiedAsset, 0, len(raw.RawData))
	for _, item := range raw.RawData {
		data, ok := item.(*DayDayMapItem)
		if !ok {
			continue
		}
		if asset := normalizeDayDayMapItem(data, d.Name()); asset != nil {
			assets = append(assets, *asset)
		}
	}
	return assets, nil
}

// normalizeDayDayMapItem 解析单条 DayDayMap 数据
func normalizeDayDayMapItem(item *DayDayMapItem, source string) *model.UnifiedAsset {
	if item == nil || item.IP == "" {
		return nil
	}
	asset := &model.UnifiedAsset{
		Source:      source,
		IP:          item.IP,
		Port:        int(item.Port),
		Protocol:    item.Protocol,
		Host:        item.Domain,
		Title:       item.Title,
		Server:      item.Server,
		StatusCode:  int(item.StatusCode),
		CountryCode: item.Country,
		Region:      item.Province,
		City:        item.City,
		ASN:         item.ASN,
		Org:         item.Org,
		ISP:         item.ISP,
	}
	// Body snippet
	if len(item.Body) > 200 {
		asset.BodySnippet = item.Body[:200]
	} else {
		asset.BodySnippet = item.Body
	}

	if asset.IP != "" && asset.Port > 0 {
		buildAssetURL(asset)
		return asset
	}
	if asset.Host != "" || asset.IP != "" {
		return asset
	}
	return nil
}

// GetQuota 获取DayDayMap配额信息
// DayDayMap API 不提供独立的配额查询端点；返回不可用。
func (d *DayDayMapAdapter) GetQuota() (*model.QuotaInfo, error) {
	return nil, fmt.Errorf("DayDayMap quota API not available")
}

// DayDayMapAdapterWebOnly DayDayMap Web-only模式适配器
type DayDayMapAdapterWebOnly struct {
	*WebOnlyAdapterBase
}

// NewDayDayMapAdapterWebOnly 创建DayDayMap Web-only适配器
func NewDayDayMapAdapterWebOnly() *DayDayMapAdapterWebOnly {
	baseAdapter := NewDayDayMapAdapter("", "", DayDayMapDefaultQPS, DayDayMapDefaultTimeout)
	return &DayDayMapAdapterWebOnly{
		WebOnlyAdapterBase: NewWebOnlyAdapterBase(baseAdapter, "daydaymap"),
	}
}
