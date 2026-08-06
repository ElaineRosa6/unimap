package adapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	// backupAPIKey 备用 key：主 key 遇到鉴权/限流/欠费失败时自动切换重试
	backupAPIKey string
	qps          int
	timeout      time.Duration

	// 请求节流：保证相邻请求间隔 >= 1/qps，避免并发查询对 Hunter 造成
	// 突发流量触发"请求太多啦"。qps<=0 时不限流。
	rateMu  sync.Mutex
	lastReq time.Time
}

// HunterItem is a single result item from the Hunter API.
type HunterItem struct {
	IP           string  `json:"ip"`
	Port         float64 `json:"port"` // float64 in JSON
	Protocol     string  `json:"protocol"`
	Domain       string  `json:"domain"`
	WebTitle     string  `json:"web_title"`
	HeaderServer string  `json:"header_server"`
	StatusCode   float64 `json:"status_code"`
	Country      string  `json:"country"`
	Province     string  `json:"province"`
	City         string  `json:"city"`
	ISP          string  `json:"isp"`
	Org          string  `json:"as_org"`
	URL          string  `json:"url"`
	// Legacy nested fields for fallback
	Web      map[string]interface{} `json:"web"`
	Location map[string]interface{} `json:"location"`
	// Extra preserves any top-level keys Hunter returns that this struct does
	// not declare (e.g. updated_at), so they are not silently dropped.
	Extra map[string]interface{} `json:"-"`
}

// UnmarshalJSON captures unknown top-level keys into Extra instead of
// dropping them, while decoding declared fields as usual.
func (m *HunterItem) UnmarshalJSON(data []byte) error {
	type alias HunterItem
	var aux alias
	extra, err := rawUnknown(data, &aux)
	if err != nil {
		return err
	}
	*m = HunterItem(aux)
	m.Extra = extra
	return nil
}

// NewHunterAdapter 创建Hunter适配器
func NewHunterAdapter(baseURL, apiKey, backupAPIKey string, qps int, timeout time.Duration) *HunterAdapter {
	client := resty.New().
		SetTimeout(timeout).
		SetHeader("User-Agent", "unimap/1.0")

	return &HunterAdapter{
		client:       client,
		baseURL:      baseURL,
		apiKey:       apiKey,
		backupAPIKey: backupAPIKey,
		qps:          qps,
		timeout:      timeout,
	}
}

// activeAPIKeys 返回依次尝试的 API key 列表（主 key + 备用 key）。
// 备用 key 为空或与主 key 相同时不重复。
func (h *HunterAdapter) activeAPIKeys() []string {
	keys := []string{h.apiKey}
	if h.backupAPIKey != "" && h.backupAPIKey != h.apiKey {
		keys = append(keys, h.backupAPIKey)
	}
	return keys
}

// hunterKeyError 表示与特定 API key 相关的失败（401 鉴权 / 402 欠费 / 429 限流）。
// 这类失败换用备用 key 可能成功，用于驱动主/备用 key 自动切换。
type hunterKeyError struct {
	code int
	msg  string
}

func (e *hunterKeyError) Error() string {
	return fmt.Sprintf("hunter key error %d: %s", e.code, e.msg)
}

// isHunterKeyError 判断错误是否为 key 级失败。
func isHunterKeyError(err error) bool {
	var keyErr *hunterKeyError
	return errors.As(err, &keyErr)
}

// classifyHunterStatus 将 HTTP/业务状态码归类为 key 级失败（401/402/429）。
// 其他状态码返回 nil，由调用方按普通错误处理。
func classifyHunterStatus(code int, body string) *hunterKeyError {
	switch code {
	case 401, 402, 429:
		return &hunterKeyError{code: code, msg: body}
	default:
		return nil
	}
}

// withKeyFailover 依次用主/备用 key 执行 fn，仅在 fn 返回 key 级失败时切换 key；
// 网络等瞬时错误由 fn 内部重试处理，不在此处切换。
func (h *HunterAdapter) withKeyFailover(fn func(key string) error) error {
	keys := h.activeAPIKeys()
	for i, key := range keys {
		err := fn(key)
		if err == nil {
			return nil
		}
		if isHunterKeyError(err) && i < len(keys)-1 {
			logger.Warnf("Hunter API key #%d failed (%v); trying backup key", i+1, err)
			continue
		}
		return err
	}
	return fmt.Errorf("hunter: all api keys exhausted")
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
	err := h.withKeyFailover(func(key string) error {
		return utils.Retry(h.searchRetryConfig(), func() error {
			if rateErr := h.waitForRate(ctx); rateErr != nil {
				return fmt.Errorf("hunter rate wait cancelled: %w", rateErr)
			}
			return h.executeHunterSearch(query, page, pageSize, key, &engineResult)
		})
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
		// key 级失败（鉴权/限流/欠费）不重试当前 key，交给 withKeyFailover 切换备用 key
		RetryableFunc: func(err error) bool {
			if isHunterKeyError(err) {
				return false
			}
			return utils.IsRetryableError(err)
		},
	}
}

// executeHunterSearch 执行单次 Hunter API 调用
func (h *HunterAdapter) executeHunterSearch(query string, page, pageSize int, apiKey string, result **model.EngineResult) error {
	baseURL := strings.TrimRight(h.baseURL, "/")
	encodedQuery := base64.URLEncoding.EncodeToString([]byte(query))
	resp, err := h.client.R().SetQueryParams(map[string]string{
		"api-key": apiKey, "search": encodedQuery,
		"page": fmt.Sprintf("%d", page), "page_size": fmt.Sprintf("%d", pageSize), "is_web": "0",
	}).Get(fmt.Sprintf("%s/openApi/search", baseURL))
	if err != nil {
		return fmt.Errorf("hunter request error: %w", err)
	}
	if resp.StatusCode() != 200 {
		if keyErr := classifyHunterStatus(resp.StatusCode(), sanitizeBody(resp.String())); keyErr != nil {
			return keyErr
		}
		return fmt.Errorf("hunter HTTP error %d: %s", resp.StatusCode(), sanitizeBody(resp.String()))
	}
	return parseHunterSearchResponse(resp.Body(), page, pageSize, h.Name(), result)
}

// parseHunterSearchResponse 解析 Hunter 搜索响应
func parseHunterSearchResponse(body []byte, page, pageSize int, engineName string, result **model.EngineResult) error {
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Total int          `json:"total"`
			Items []HunterItem `json:"arr"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("hunter response parse error: %w", err)
	}
	if resp.Code != 200 {
		if keyErr := classifyHunterStatus(resp.Code, resp.Message); keyErr != nil {
			return keyErr
		}
		return fmt.Errorf("hunter API error: %s", resp.Message)
	}
	rawData := make([]interface{}, len(resp.Data.Items))
	for i := range resp.Data.Items {
		rawData[i] = &resp.Data.Items[i]
	}
	*result = &model.EngineResult{
		EngineName: engineName, RawData: rawData, Total: resp.Data.Total,
		Page: page, HasMore: (page * pageSize) < resp.Data.Total,
	}
	return nil
}

// Normalize 标准化Hunter结果
func (h *HunterAdapter) Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error) {
	if raw == nil || len(raw.RawData) == 0 {
		return []model.UnifiedAsset{}, nil
	}
	assets := make([]model.UnifiedAsset, 0, len(raw.RawData))
	for _, item := range raw.RawData {
		m, ok := item.(*HunterItem)
		if !ok {
			continue
		}
		if asset := normalizeHunterMatch(m); asset != nil {
			assets = append(assets, *asset)
		}
	}
	return assets, nil
}

// normalizeHunterMatch converts a parsed HunterItem to a UnifiedAsset.
func normalizeHunterMatch(m *HunterItem) *model.UnifiedAsset {
	if m == nil || (m.IP == "" && m.Domain == "") {
		return nil
	}
	asset := &model.UnifiedAsset{Source: "hunter",
		IP: m.IP, Port: int(m.Port), Protocol: m.Protocol, Host: m.Domain,
		Title: m.WebTitle, Server: m.HeaderServer, StatusCode: int(m.StatusCode),
		CountryCode: m.Country, Region: m.Province, City: m.City,
		ISP: m.ISP, Org: m.Org, URL: m.URL,
	}

	// Legacy nested fields as fallback when flat fields are empty
	if asset.IP == "" {
		parseHunterLegacyFields(m, asset)
	}
	// Preserve raw nested structures (legacy web/location objects) so any keys
	// not extracted above still survive persistence.
	if m.Web != nil {
		mergeAssetExtra(asset, map[string]interface{}{"web": m.Web})
	}
	if m.Location != nil {
		mergeAssetExtra(asset, map[string]interface{}{"location": m.Location})
	}
	// Capture unknown top-level fields (e.g. updated_at) and promote any
	// timestamp key to LastSeen.
	applyExtras(asset, m.Extra)

	ensureHunterURL(asset)
	if asset.IP != "" || asset.Host != "" {
		return asset
	}
	return nil
}

// parseHunterLegacyFields 解析旧版嵌套结构（web/location 子对象）
func parseHunterLegacyFields(m *HunterItem, asset *model.UnifiedAsset) {
	if m.Web != nil {
		setStr := func(key string, target *string) {
			if v, ok := m.Web[key].(string); ok {
				*target = v
			}
		}
		setStr("ip", &asset.IP)
		setStr("protocol", &asset.Protocol)
		setStr("domain", &asset.Host)
		setStr("title", &asset.Title)
		setStr("server", &asset.Server)
		if v, ok := m.Web["port"].(float64); ok {
			asset.Port = int(v)
		}
		if v, ok := m.Web["status_code"].(float64); ok {
			asset.StatusCode = int(v)
		}
	}
	if m.Location != nil {
		if v, ok := m.Location["country_cn"].(string); ok {
			asset.CountryCode = v
		}
		if v, ok := m.Location["province_cn"].(string); ok {
			asset.Region = v
		}
		if v, ok := m.Location["city_cn"].(string); ok {
			asset.City = v
		}
	}
}

// ensureHunterURL 确保资产有 URL（从 IP/Port/Protocol 构建）
func ensureHunterURL(asset *model.UnifiedAsset) {
	if asset.URL != "" || asset.IP == "" || asset.Port == 0 {
		return
	}
	proto := asset.Protocol
	if proto == "" {
		if asset.Port == 443 {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := asset.IP
	if asset.Host != "" {
		host = asset.Host
	}
	scheme := "http"
	if strings.HasPrefix(proto, "https") || asset.Port == 443 {
		scheme = "https"
	}
	u := &url.URL{Scheme: scheme, Host: fmt.Sprintf("%s:%d", host, asset.Port)}
	asset.URL = u.String()
}

// GetQuota 获取Hunter配额信息
func (h *HunterAdapter) GetQuota() (*model.QuotaInfo, error) {
	if h.apiKey == "" {
		return nil, fmt.Errorf("Hunter API key not configured")
	}
	var quota *model.QuotaInfo
	err := h.withKeyFailover(func(key string) error {
		q, qErr := h.getQuotaWithKey(key)
		if qErr != nil {
			return qErr
		}
		quota = q
		return nil
	})
	if err != nil {
		return nil, err
	}
	return quota, nil
}

// getQuotaWithKey 用指定 key 查询配额
func (h *HunterAdapter) getQuotaWithKey(apiKey string) (*model.QuotaInfo, error) {
	// Hunter API endpoint for quota info
	baseURL := strings.TrimRight(h.baseURL, "/")
	// NOTE: Hunter uses camelCase path: /openApi/userInfo
	url := fmt.Sprintf("%s/openApi/userInfo", baseURL)

	resp, err := h.client.R().
		SetQueryParam("api-key", apiKey).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}

	if resp.StatusCode() != 200 {
		if keyErr := classifyHunterStatus(resp.StatusCode(), sanitizeBody(resp.String())); keyErr != nil {
			return nil, keyErr
		}
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
		if keyErr := classifyHunterStatus(result.Code, result.Message); keyErr != nil {
			return nil, keyErr
		}
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
	baseAdapter := NewHunterAdapter("", "", "", 3, 30*time.Second)
	return &HunterAdapterWebOnly{
		WebOnlyAdapterBase: NewWebOnlyAdapterBase(baseAdapter, "hunter"),
	}
}
