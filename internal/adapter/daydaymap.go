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
	// DayDayMapDefaultTimeout DayDayMap默认超时
	DayDayMapDefaultTimeout = 30 * time.Second
	// DayDayMapDefaultQPS DayDayMap默认QPS
	DayDayMapDefaultQPS = 3
)

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
		"body":        "web.body",
		"title":       "web.title",
		"header":      "web.header",
		"port":        "ip.port",
		"protocol":    "protocol.service",
		"ip":          "ip",
		"country":     "ip.country",
		"region":      "ip.province",
		"city":        "ip.city",
		"asn":         "asn.number",
		"org":         "org.name",
		"isp":         "ip.isp",
		"domain":      "domain",
		"host":        "domain",
		"server":      "web.server",
		"status_code": "web.status_code",
		"os":          "ip.os",
		"app":         "app.name",
		"cert":            "cert.subject",
		"cert.subject.cn": "cert.subject.cn",
		"cert.issuer.cn":  "cert.issuer.cn",
		"url":         "domain",
	}

	if mapped, ok := mapping[field]; ok {
		return mapped
	}
	return field
}

// Search 执行DayDayMap搜索
func (d *DayDayMapAdapter) Search(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error) {
	if d.apiKey == "" {
		return &model.EngineResult{
			EngineName: d.Name(),
			Error:      "DayDayMap API key not configured",
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
			// 非临时性错误不重试：认证失败、余额不足
			if strings.Contains(errStr, "HTTP 401") ||
				strings.Contains(errStr, "HTTP 403") {
				return false
			}
			// 其他错误（网络、5xx、429限流等）可重试
			return true
		},
	}

	err := utils.Retry(retryConfig, func() error {
		searchURL := fmt.Sprintf("%s/api/v1/search", d.baseURL)

		resp, err := d.client.R().
			SetQueryParams(map[string]string{
				"apikey":   d.apiKey,
				"query":    query,
				"page":     fmt.Sprintf("%d", page),
				"pagesize": fmt.Sprintf("%d", pageSize),
			}).
			Get(searchURL)

		if err != nil {
			return err
		}

		if resp.StatusCode() != 200 {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode(), sanitizeBody(resp.String()))
		}

		var result struct {
			Code    int             `json:"code"`
			Message string          `json:"message"`
			Data    json.RawMessage `json:"data"`
			Total   int             `json:"total"`
		}

		if err := json.Unmarshal(resp.Body(), &result); err != nil {
			return err
		}

		if result.Code != 0 && result.Code != 200 {
			errMsg := result.Message
			if errMsg == "" {
				errMsg = "DayDayMap API reported an error (unknown cause)"
			}
			return fmt.Errorf("DayDayMap API error: %s", errMsg)
		}

		// 解析 data 数组
		var dataItems []map[string]interface{}
		if err := json.Unmarshal(result.Data, &dataItems); err != nil {
			return fmt.Errorf("parse data error: %w", err)
		}

		rawData := make([]interface{}, 0, len(dataItems))
		for _, item := range dataItems {
			rawData = append(rawData, item)
		}

		engineResult = &model.EngineResult{
			EngineName: d.Name(),
			RawData:    rawData,
			Total:      result.Total,
			Page:       page,
			HasMore:    (page * pageSize) < result.Total,
		}

		return nil
	})

	if err != nil {
		return &model.EngineResult{
			EngineName: d.Name(),
			Error:      fmt.Sprintf("search error: %v", err),
		}, nil
	}

	return engineResult, nil
}

// Normalize 标准化DayDayMap结果
func (d *DayDayMapAdapter) Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error) {
	if raw == nil || len(raw.RawData) == 0 {
		return []model.UnifiedAsset{}, nil
	}
	assets := make([]model.UnifiedAsset, 0, len(raw.RawData))
	for _, item := range raw.RawData {
		data, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if asset := d.normalizeDayDayMapItem(data); asset != nil {
			assets = append(assets, *asset)
		}
	}
	return assets, nil
}

// normalizeDayDayMapItem 解析单条 DayDayMap 数据
func (d *DayDayMapAdapter) normalizeDayDayMapItem(data map[string]interface{}) *model.UnifiedAsset {
	ip, _ := data["ip"].(string)
	if ip == "" {
		return nil
	}
	asset := &model.UnifiedAsset{IP: ip, Source: d.Name()}
	getStr := func(k string) string { v, _ := data[k].(string); return v }
	getInt := func(k string) int {
		if v, ok := data[k].(float64); ok { return int(v) }
		if v, ok := data[k].(int); ok { return v }
		return 0
	}

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
	asset.Region = getStr("province")
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

// GetQuota 获取DayDayMap配额信息
func (d *DayDayMapAdapter) GetQuota() (*model.QuotaInfo, error) {
	if d.apiKey == "" {
		return nil, fmt.Errorf("DayDayMap API key not configured")
	}

	quotaURL := fmt.Sprintf("%s/api/v1/user/info", d.baseURL)

	resp, err := d.client.R().
		SetQueryParams(map[string]string{
			"apikey": d.apiKey,
		}).
		Get(quotaURL)

	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), sanitizeBody(resp.String()))
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			RemainQuota int `json:"remain_quota"`
			TotalQuota  int `json:"total_quota"`
			UsedQuota   int `json:"used_quota"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	if result.Code != 0 && result.Code != 200 {
		return nil, fmt.Errorf("DayDayMap API error: %s", result.Message)
	}

	remaining := result.Data.RemainQuota
	total := result.Data.TotalQuota
	used := result.Data.UsedQuota
	if total <= 0 && remaining > 0 {
		total = remaining + used
	}

	return &model.QuotaInfo{
		Remaining: remaining,
		Total:     total,
		Used:      used,
		Unit:      "queries",
		Expiry:    "",
	}, nil
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
