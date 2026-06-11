package adapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/utils"
)

// HunterAdapter Hunter引擎适配器
type HunterAdapter struct {
	client  *resty.Client
	baseURL string
	apiKey  string
	qps     int
	timeout time.Duration

	// 请求节流：保证相邻请求间隔 >= 1/qps，避免并发查询对 Hunter 造成
	// 突发流量触发"请求太多啦"。qps<=0 时不限流。
	rateMu  sync.Mutex
	lastReq time.Time
}

// NewHunterAdapter 创建Hunter适配器
func NewHunterAdapter(baseURL, apiKey string, qps int, timeout time.Duration) *HunterAdapter {
	client := resty.New().
		SetTimeout(timeout).
		SetHeader("User-Agent", "unimap/1.0")

	return &HunterAdapter{
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
		qps:     qps,
		timeout: timeout,
	}
}

// waitForRate 在发起请求前等待，确保相邻请求间隔满足 qps 限制。
// 尊重 ctx 取消；qps<=0 时立即返回。
func (h *HunterAdapter) waitForRate(ctx context.Context) error {
	if h.qps <= 0 {
		return nil
	}

	minInterval := time.Second / time.Duration(h.qps)

	h.rateMu.Lock()
	now := time.Now()
	var wait time.Duration
	if !h.lastReq.IsZero() {
		if elapsed := now.Sub(h.lastReq); elapsed < minInterval {
			wait = minInterval - elapsed
		}
	}
	// 预占下一个时隙，避免持锁期间睡眠阻塞其它 goroutine
	h.lastReq = now.Add(wait)
	h.rateMu.Unlock()

	if wait <= 0 {
		return nil
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Name 返回引擎名称
func (h *HunterAdapter) Name() string {
	return "hunter"
}

// Translate 将UQL AST转换为Hunter查询语法
func (h *HunterAdapter) Translate(ast *model.UQLAST) (string, error) {
	if ast == nil || ast.Root == nil {
		return "", fmt.Errorf("invalid AST")
	}

	query := h.translateNode(ast.Root)
	return query, nil
}

func (h *HunterAdapter) translateNode(node *model.UQLNode) string {
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
					conditions = append(conditions, h.buildCondition(field, "=", v))
				}
				return "(" + strings.Join(conditions, " || ") + ")"
			}

			return h.buildCondition(field, op, val)
		}

	case "logical":
		if len(node.Children) >= 2 {
			left := h.translateNode(node.Children[0])
			right := h.translateNode(node.Children[1])
			if node.Value == "OR" {
				return fmt.Sprintf("(%s || %s)", left, right)
			}
			return fmt.Sprintf("(%s && %s)", left, right)
		}
	}

	return ""
}

func (h *HunterAdapter) buildCondition(field, op, value string) string {
	// Hunter 字段映射（点命名空间: web.*, ip.*, app.*, header.*）
	mapping := map[string]string{
		"body":        "web.body",
		"title":       "web.title",
		"header":      "header",
		"port":        "port",
		"protocol":    "protocol",
		"ip":          "ip",
		"country":     "ip.country",
		"region":      "ip.province",
		"city":        "ip.city",
		"asn":         "asn",
		"org":         "ip.org",
		"isp":         "ip.isp",
		"domain":      "domain",
		"status_code": "web.status_code",
		"os":          "ip.os",
		"app":         "app.name",
		"server":      "header.server",
		"host":        "domain",
		"cert":        "cert",
	}

	mappedField, ok := mapping[field]
	if !ok {
		mappedField = field
	}

	switch op {
	case "==":
		return fmt.Sprintf(`%s=="%s"`, mappedField, escapeQuotes(value))
	case "!=", "<>":
		return fmt.Sprintf(`%s!="%s"`, mappedField, escapeQuotes(value))
	case ">":
		return fmt.Sprintf(`%s>"%s"`, mappedField, escapeQuotes(value))
	case ">=":
		return fmt.Sprintf(`%s>="%s"`, mappedField, escapeQuotes(value))
	case "<":
		return fmt.Sprintf(`%s<"%s"`, mappedField, escapeQuotes(value))
	case "<=":
		return fmt.Sprintf(`%s<="%s"`, mappedField, escapeQuotes(value))
	default:
		// =, CONTAINS 等均为模糊匹配
		return fmt.Sprintf(`%s="%s"`, mappedField, escapeQuotes(value))
	}
}

// Search 执行Hunter搜索
func (h *HunterAdapter) Search(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error) {
	if h.apiKey == "" {
		return &model.EngineResult{EngineName: h.Name(), Error: "Hunter API key not configured"}, nil
	}
	var engineResult *model.EngineResult
	err := utils.Retry(h.searchRetryConfig(), func() error {
		if rateErr := h.waitForRate(ctx); rateErr != nil {
			return fmt.Errorf("hunter rate wait cancelled: %w", rateErr)
		}
		return h.executeHunterSearch(query, page, pageSize, &engineResult)
	})
	if err != nil {
		return &model.EngineResult{EngineName: h.Name(), Error: fmt.Sprintf("search error: %v", err)}, nil
	}
	return engineResult, nil
}

func (h *HunterAdapter) searchRetryConfig() utils.RetryConfig {
	return utils.RetryConfig{
		MaxRetries: 3, BaseDelay: 100 * time.Millisecond, MaxDelay: 2 * time.Second,
		Exponential: true, Jitter: true,
	}
}

// executeHunterSearch 执行单次 Hunter API 调用
func (h *HunterAdapter) executeHunterSearch(query string, page, pageSize int, result **model.EngineResult) error {
	baseURL := strings.TrimRight(h.baseURL, "/")
	encodedQuery := base64.URLEncoding.EncodeToString([]byte(query))
	resp, err := h.client.R().SetQueryParams(map[string]string{
		"api-key": h.apiKey, "search": encodedQuery,
		"page": fmt.Sprintf("%d", page), "page_size": fmt.Sprintf("%d", pageSize), "is_web": "0",
	}).Get(fmt.Sprintf("%s/openApi/search", baseURL))
	if err != nil {
		return fmt.Errorf("hunter request error: %w", err)
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("hunter HTTP error %d: %s", resp.StatusCode(), resp.String())
	}
	return parseHunterSearchResponse(resp.Body(), page, pageSize, h.Name(), result)
}

// parseHunterSearchResponse 解析 Hunter 搜索响应
func parseHunterSearchResponse(body []byte, page, pageSize int, engineName string, result **model.EngineResult) error {
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Total int                      `json:"total"`
			Items []map[string]interface{} `json:"arr"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("hunter response parse error: %w", err)
	}
	if resp.Code != 200 {
		switch resp.Code {
		case 401:
			return fmt.Errorf("hunter authentication error: %s", resp.Message)
		case 429:
			return fmt.Errorf("hunter rate limit exceeded: %s", resp.Message)
		case 402:
			return fmt.Errorf("hunter payment required: %s", resp.Message)
		default:
			return fmt.Errorf("hunter API error: %s", resp.Message)
		}
	}
	rawData := make([]interface{}, len(resp.Data.Items))
	for i, item := range resp.Data.Items {
		rawData[i] = item
	}
	*result = &model.EngineResult{
		EngineName: engineName, RawData: rawData, Total: resp.Data.Total,
		Page: page, HasMore: (page * pageSize) < resp.Data.Total,
	}
	return nil
}

// Normalize 标准化Hunter结果
func (h *HunterAdapter) Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error) {
	assets := make([]model.UnifiedAsset, 0, len(raw.RawData))
	if raw == nil || len(raw.RawData) == 0 {
		return assets, nil
	}
	for _, item := range raw.RawData {
		data, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if asset := h.normalizeHunterItem(data); asset != nil {
			assets = append(assets, *asset)
		}
	}
	return assets, nil
}

// normalizeHunterItem 解析单条 Hunter 数据
func (h *HunterAdapter) normalizeHunterItem(data map[string]interface{}) *model.UnifiedAsset {
	asset := &model.UnifiedAsset{Source: h.Name(), Extra: data}
	getStr := func(k string) string { v, _ := data[k].(string); return v }
	getInt := func(k string) int {
		if v, ok := data[k].(float64); ok { return int(v) }
		if v, ok := data[k].(int); ok { return v }
		return 0
	}

	// 扁平结构（新版 API）
	asset.IP = getStr("ip")
	asset.Port = getInt("port")
	asset.Protocol = getStr("protocol")
	asset.Host = getStr("domain")
	asset.Title = getStr("web_title")
	asset.Server = getStr("header_server")
	asset.StatusCode = getInt("status_code")
	asset.CountryCode = getStr("country")
	asset.Region = getStr("province")
	asset.City = getStr("city")
	asset.ISP = getStr("isp")
	asset.Org = getStr("as_org")
	asset.URL = getStr("url")

	if asset.IP == "" {
		h.parseHunterLegacyFields(data, asset)
		if asset.IP == "" { asset.IP = getStr("ip") }
		if asset.Port == 0 { asset.Port = getInt("port") }
	}

	ensureHunterURL(asset)
	if asset.IP != "" || asset.Host != "" {
		return asset
	}
	return nil
}

// parseHunterLegacyFields 解析旧版嵌套结构（web/location 子对象）
func (h *HunterAdapter) parseHunterLegacyFields(data map[string]interface{}, asset *model.UnifiedAsset) {
	if web, ok := data["web"].(map[string]interface{}); ok {
		setStr := func(key string, target *string) {
			if v, ok := web[key].(string); ok { *target = v }
		}
		setStr("ip", &asset.IP)
		setStr("protocol", &asset.Protocol)
		setStr("domain", &asset.Host)
		setStr("title", &asset.Title)
		setStr("server", &asset.Server)
		if v, ok := web["port"].(float64); ok { asset.Port = int(v) }
		if v, ok := web["status_code"].(float64); ok { asset.StatusCode = int(v) }
	}
	if loc, ok := data["location"].(map[string]interface{}); ok {
		if v, ok := loc["country_cn"].(string); ok { asset.CountryCode = v }
		if v, ok := loc["province_cn"].(string); ok { asset.Region = v }
		if v, ok := loc["city_cn"].(string); ok { asset.City = v }
	}
}

// ensureHunterURL 确保资产有 URL（从 IP/Port/Protocol 构建）
func ensureHunterURL(asset *model.UnifiedAsset) {
	if asset.URL != "" || asset.IP == "" || asset.Port == 0 {
		return
	}
	proto := asset.Protocol
	if proto == "" {
		if asset.Port == 443 { proto = "https" } else { proto = "http" }
	}
	host := asset.IP
	if asset.Host != "" { host = asset.Host }
	scheme := "http"
	if strings.HasPrefix(proto, "https") || asset.Port == 443 { scheme = "https" }
	u := &url.URL{Scheme: scheme, Host: fmt.Sprintf("%s:%d", host, asset.Port)}
	asset.URL = u.String()
}

// GetQuota 获取Hunter配额信息
func (h *HunterAdapter) GetQuota() (*model.QuotaInfo, error) {
	if h.apiKey == "" {
		return nil, fmt.Errorf("Hunter API key not configured")
	}

	// Hunter API endpoint for quota info
	baseURL := strings.TrimRight(h.baseURL, "/")
	// NOTE: Hunter uses camelCase path: /openApi/userInfo
	url := fmt.Sprintf("%s/openApi/userInfo", baseURL)

	resp, err := h.client.R().
		SetQueryParam("api-key", h.apiKey).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), sanitizeBody(resp.String()))
	}

	// 打印响应体，方便调试
	logger.Debugf("Hunter quota response: %s", sanitizeBody(resp.String()))

	// Hunter quota response structure
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			RestFreePoint   int `json:"rest_free_point"`
			DayFreePoint    int `json:"day_free_point"`
			RestEquityPoint int `json:"rest_equity_point"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	if result.Code != 200 {
		return nil, fmt.Errorf("%s", result.Message)
	}

	// 计算配额信息
	// Hunter的响应中，RestFreePoint是剩余的免费点数，DayFreePoint是每日免费点数
	total := result.Data.DayFreePoint
	remain := result.Data.RestFreePoint

	// 边界检查：确保数值合理
	if remain < 0 {
		remain = 0
	}
	if total < 0 {
		total = 0
	}

	// 计算已用配额，确保不会出现负数
	used := total - remain
	if used < 0 {
		used = 0
	}

	// 如果剩余大于总数，调整总数
	if remain > total {
		total = remain
		used = 0
	}

	// 打印解析后的配额信息
	logger.Infof("Hunter quota: total=%d, used=%d, remain=%d", total, used, remain)

	return &model.QuotaInfo{
		Remaining: remain,
		Total:     total,
		Used:      used,
		Unit:      "queries",
		Expiry:    "", // Hunter API doesn't return expiry info
	}, nil
}

// IsWebOnly 检查是否为 Web-only 模式
func (h *HunterAdapter) IsWebOnly() bool {
	return false
}

// HunterAdapterWebOnly Hunter Web-only模式适配器
type HunterAdapterWebOnly struct {
	*WebOnlyAdapterBase
}

// NewHunterAdapterWebOnly 创建Hunter Web-only适配器
func NewHunterAdapterWebOnly() *HunterAdapterWebOnly {
	baseAdapter := NewHunterAdapter("", "", 3, 30*time.Second)
	return &HunterAdapterWebOnly{
		WebOnlyAdapterBase: NewWebOnlyAdapterBase(baseAdapter, "hunter"),
	}
}
