package adapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/utils"
)

// ZoomEyeAdapter ZoomEye引擎适配器
type ZoomEyeAdapter struct {
	client  *resty.Client
	baseURL string
	apiKey  string
	qps     int
	timeout time.Duration
}

// ZoomEyeItem is a single result item from ZoomEye v2 search API.
type ZoomEyeItem struct {
	IP       string      `json:"ip"`
	Port     float64     `json:"port"`    // float64 in JSON, may be number or string
	Service  string      `json:"service"` // e.g. "http", "ssh"
	Banner   string      `json:"banner"`
	Title    string      `json:"title"`
	Server   string      `json:"server"`
	ASN      json.Number `json:"asn"` // numeric in ZoomEye API, json.Number handles both
	Org      string      `json:"org"`
	ISP      string      `json:"isp"`
	OS       string      `json:"os"`
	Product  string      `json:"product"`
	Version  string      `json:"version"`
	Device   string      `json:"device"`
	App      string      `json:"app"`
	Body     string      `json:"body"`
	Header   string      `json:"header"`
	Country  string      `json:"country"`
	City     string      `json:"city"`
	Timezone string      `json:"timezone"`
	Hostname string      `json:"hostname"`
	Domain   string      `json:"domain"`
	LastSeen string      `json:"last_seen"`
	URL      string      `json:"url"`
	// Dot-notation fields (newer API format)
	CountryName  string `json:"country.name"`
	CountryCode  string `json:"country.code"`
	ProvinceName string `json:"province.name"`
	CityName     string `json:"city.name"`
	OrgName      string `json:"organization.name"`
	ISPName      string `json:"isp.name"`
	ServerName   string `json:"header.server.name"`
	// Nested objects — variable structure, kept as map for flexibility
	PortInfo map[string]interface{} `json:"portinfo"`
	GeoInfo  map[string]interface{} `json:"geoinfo"`
}

// ZoomEyeSearchResponse is the ZoomEye v2 search API response.
type ZoomEyeSearchResponse struct {
	Code    int               `json:"code"`
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Total   int               `json:"total"`
	Query   string            `json:"query"`
	Data    []json.RawMessage `json:"data"`
}

// zoomEyeSearchRequest is the JSON body for POST /v2/search.
type zoomEyeSearchRequest struct {
	QBase64  string `json:"qbase64"`
	Page     int    `json:"page"`
	PageSize int    `json:"pagesize"`
}

// NewZoomEyeAdapter 创建ZoomEye适配器
func NewZoomEyeAdapter(baseURL, apiKey string, qps int, timeout time.Duration) *ZoomEyeAdapter {
	client := resty.New().
		SetTimeout(timeout).
		SetHeader("User-Agent", "unimap/1.0").
		SetHeader("API-KEY", apiKey)

	return &ZoomEyeAdapter{
		client:  client,
		baseURL: baseURL,
		apiKey:  apiKey,
		qps:     qps,
		timeout: timeout,
	}
}

// Name 返回引擎名称
func (z *ZoomEyeAdapter) Name() string {
	return "zoomeye"
}

// Translate 将UQL AST转换为ZoomEye查询语法
func (z *ZoomEyeAdapter) Translate(ast *model.UQLAST) (string, error) {
	if ast == nil || ast.Root == nil {
		return "", fmt.Errorf("invalid AST")
	}

	query := z.translateNode(ast.Root)
	return query, nil
}

func (z *ZoomEyeAdapter) translateNode(node *model.UQLNode) string {
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
					conditions = append(conditions, z.buildCondition(field, "=", v))
				}
				return "(" + strings.Join(conditions, " || ") + ")"
			}

			return z.buildCondition(field, op, val)
		}

	case "logical":
		if len(node.Children) >= 2 {
			left := z.translateNode(node.Children[0])
			right := z.translateNode(node.Children[1])
			if node.Value == "OR" {
				return fmt.Sprintf("(%s || %s)", left, right)
			}
			return fmt.Sprintf("(%s && %s)", left, right)
		}
	}

	return ""
}

func (z *ZoomEyeAdapter) buildCondition(field, op, value string) string {
	// 字段映射 — v2 API 语法: field="value" (非 v1 的 +field:"value")
	mapping := map[string]string{
		"body":        "http.body",
		"title":       "title",
		"header":      "http.header",
		"port":        "port",
		"protocol":    "service",
		"ip":          "ip",
		"country":     "country",
		"region":      "subdivisions",
		"city":        "city",
		"asn":         "asn",
		"org":         "org",
		"isp":         "isp",
		"domain":      "domain",
		"app":         "app",
		"os":          "os",
		"device":      "device",
		"banner":      "banner",
		"server":      "http.header.server",
		"host":        "hostname",
		"url":         "site",
		"status_code": "http.header.status_code",
		"cert":        "ssl",
	}

	if mapped, ok := mapping[field]; ok {
		field = mapped
	}

	escaped := escapeQuotes(value)

	switch op {
	case "==":
		return fmt.Sprintf(`%s=="%s"`, field, escaped)
	case "!=", "<>":
		return fmt.Sprintf(`%s!="%s"`, field, escaped)
	case ">", ">=", "<", "<=":
		// ZoomEye 不支持比较运算符，降级为等值查询
		logger.Warnf("zoomeye: comparison operator %q not supported, falling back to = for field %s", op, field)
		return fmt.Sprintf(`%s="%s"`, field, escaped)
	default:
		// =, CONTAINS 等均为模糊匹配
		return fmt.Sprintf(`%s="%s"`, field, escaped)
	}
}

// Search 执行搜索
func (z *ZoomEyeAdapter) Search(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error) {
	var engineResult *model.EngineResult
	err := utils.Retry(z.searchRetryConfig(), func() error {
		return z.executeZoomEyeSearch(query, page, pageSize, &engineResult)
	})
	if err != nil {
		return &model.EngineResult{EngineName: z.Name(), Error: err.Error()}, nil
	}
	return engineResult, nil
}

func (z *ZoomEyeAdapter) searchRetryConfig() utils.RetryConfig {
	return utils.RetryConfig{
		MaxRetries: 3, BaseDelay: 100 * time.Millisecond, MaxDelay: 2 * time.Second,
		Exponential: true, Jitter: true,
		RetryableFunc: func(err error) bool {
			errMsg := err.Error()
			return !strings.Contains(errMsg, "402") && !strings.Contains(errMsg, "Payment Required")
		},
	}
}

// executeZoomEyeSearch 执行单次 ZoomEye API 调用
func (z *ZoomEyeAdapter) executeZoomEyeSearch(query string, page, pageSize int, result **model.EngineResult) error {
	url := fmt.Sprintf("%s/v2/search", z.baseURL)

	encodedQuery := base64.StdEncoding.EncodeToString([]byte(query))
	encodedQuery = strings.ReplaceAll(encodedQuery, "+", "-")
	encodedQuery = strings.ReplaceAll(encodedQuery, "/", "_")
	encodedQuery = strings.TrimRight(encodedQuery, "=")

	reqBody := zoomEyeSearchRequest{
		QBase64:  encodedQuery,
		Page:     page,
		PageSize: pageSize,
	}

	logger.Debugf("ZoomEye search request: URL=%s, Query=%s, EncodedQuery=%s, Page=%d, PageSize=%d", url, query, encodedQuery, page, pageSize)

	resp, err := z.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(reqBody).
		Post(url)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 {
		if resp.StatusCode() == 402 {
			return fmt.Errorf("ZoomEye API Payment Required (402): %s. Please check if your account is mobile-verified or if you have sufficient quota/credits.", resp.String())
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String())
	}
	return parseZoomEyeSearchResponse(resp.Body(), page, pageSize, z.Name(), result)
}

// parseZoomEyeSearchResponse 解析 ZoomEye 搜索响应
func parseZoomEyeSearchResponse(body []byte, page, pageSize int, engineName string, result **model.EngineResult) error {
	var resp struct {
		Code    int           `json:"code"`
		Error   string        `json:"error"`
		Message string        `json:"message"`
		Total   int           `json:"total"`
		Query   string        `json:"query"`
		Data    []ZoomEyeItem `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if resp.Code != 60000 {
		errorMsg := fmt.Sprintf("ZoomEye API error (code=%d, error=%s): %s", resp.Code, resp.Error, resp.Message)
		if resp.Code == 50000 && resp.Error == "credits_insufficient" {
			errorMsg = fmt.Sprintf("ZoomEye API credits insufficient: %s. Please check your account balance or upgrade your plan.", resp.Message)
		}
		return fmt.Errorf("%s", errorMsg)
	}
	rawData := make([]interface{}, len(resp.Data))
	for i := range resp.Data {
		rawData[i] = &resp.Data[i]
	}
	*result = &model.EngineResult{
		EngineName: engineName, RawData: rawData, Total: resp.Total,
		Page: page, HasMore: (page * pageSize) < resp.Total,
	}
	return nil
}

// Normalize 标准化结果
func (z *ZoomEyeAdapter) Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error) {
	assets := make([]model.UnifiedAsset, 0, len(raw.RawData))
	if raw == nil || len(raw.RawData) == 0 {
		return assets, nil
	}
	for _, item := range raw.RawData {
		it, ok := item.(*ZoomEyeItem)
		if !ok {
			continue
		}
		if asset := normalizeZoomEyeItem(it, z.Name()); asset != nil {
			assets = append(assets, *asset)
		}
	}
	return assets, nil
}

// normalizeZoomEyeItem 解析单条 ZoomEye 数据为资产对象
func normalizeZoomEyeItem(it *ZoomEyeItem, source string) *model.UnifiedAsset {
	if it == nil || (it.IP == "" && it.URL == "" && it.Hostname == "" && it.Domain == "") {
		return nil
	}
	asset := &model.UnifiedAsset{Source: source, IP: it.IP}

	parseZoomEyePortAndService(it, asset)
	parseZoomEyeTitle(it, asset)
	parseZoomEyeBasicFields(it, asset)
	parseZoomEyeGeo(it, asset)
	parseZoomEyeNetwork(it, asset)
	parseZoomEyeExtra(it, asset)

	return asset
}

// parseZoomEyePortAndService 解析端口和服务（支持顶层和 portinfo 两种格式）
func parseZoomEyePortAndService(it *ZoomEyeItem, asset *model.UnifiedAsset) {
	if it.Port > 0 {
		asset.Port = int(it.Port)
	}
	if it.Service != "" {
		asset.Protocol = it.Service
	}
	// 旧格式：在 portinfo 中
	if it.PortInfo != nil {
		if asset.Port == 0 {
			if port, ok := it.PortInfo["port"].(float64); ok {
				asset.Port = int(port)
			}
		}
		if asset.Protocol == "" {
			if service, ok := it.PortInfo["service"].(string); ok {
				asset.Protocol = service
			}
		}
		if asset.Title == "" {
			if title, ok := it.PortInfo["title"].(string); ok {
				asset.Title = title
			}
		}
		if asset.BodySnippet == "" {
			if banner, ok := it.PortInfo["banner"].(string); ok {
				asset.BodySnippet = banner
			}
		}
	}
}

// parseZoomEyeTitle 解析标题。
// ZoomEye API 返回的 title 可能包含元数据前缀（如 "CN 北京 公司名 AS12345 真正标题"），
// 需要清理掉国家/城市/ASN/组织等前缀。
func parseZoomEyeTitle(it *ZoomEyeItem, asset *model.UnifiedAsset) {
	asset.Title = cleanZoomEyeTitle(it.Title)
}

// cleanZoomEyeTitle 清理 ZoomEye title 中的元数据前缀。
// ZoomEye 的 title 字段可能包含国家代码、城市名、ASN、组织名等前缀，
// 这些信息已经在其他字段（country_code/region/org/asn）中提取，需要从 title 中移除。
func cleanZoomEyeTitle(title string) string {
	if title == "" {
		return ""
	}
	// 移除开头的 2 位国家代码（如 CN、US）
	if len(title) >= 3 && title[0] >= 'A' && title[0] <= 'Z' && title[1] >= 'A' && title[1] <= 'Z' && title[2] == ' ' {
		title = title[3:]
	}
	// 移除常见中文城市名前缀
	cities := []string{"北京", "上海", "广州", "深圳", "杭州", "成都", "武汉", "南京", "西安", "重庆", "天津", "苏州", "长沙", "郑州", "青岛", "大连", "厦门", "宁波", "东莞", "无锡", "佛山"}
	for _, city := range cities {
		if strings.HasPrefix(title, city+" ") {
			title = title[len(city)+1:]
			break
		}
	}
	// 移除 ASN 前缀（如 AS12345）
	if strings.HasPrefix(title, "AS") {
		for i := 2; i < len(title); i++ {
			if title[i] < '0' || title[i] > '9' {
				if title[i] == ' ' {
					title = title[i+1:]
				}
				break
			}
		}
	}
	// 移除组织名前缀（包含公司/集团/科技等关键词）
	orgKeywords := []string{"公司", "集团", "有限", "股份", "科技", "网络", "信息", "技术", "企业", "机构", "组织"}
	for _, keyword := range orgKeywords {
		idx := strings.Index(title, keyword)
		if idx > 0 && idx < 20 {
			// 找到关键词后的空格
			afterOrg := title[idx+len(keyword):]
			if len(afterOrg) > 0 && afterOrg[0] == ' ' {
				title = afterOrg[1:]
			}
			break
		}
	}
	return strings.TrimSpace(title)
}

// parseZoomEyeBasicFields 解析 banner、server、url、domain 等基础字段
func parseZoomEyeBasicFields(it *ZoomEyeItem, asset *model.UnifiedAsset) {
	if it.Banner != "" {
		asset.BodySnippet = it.Banner
	}
	if it.ServerName != "" {
		asset.Server = it.ServerName
	}
	if it.URL != "" {
		asset.URL = it.URL
	}
	if it.Domain != "" {
		asset.Host = it.Domain
	} else if it.Hostname != "" {
		asset.Host = it.Hostname
	}
}

// parseZoomEyeGeo 解析地理位置信息（支持新格式点号字段和旧格式 geoinfo）
func parseZoomEyeGeo(it *ZoomEyeItem, asset *model.UnifiedAsset) {
	// 新格式：点号分隔字段
	if it.CountryName != "" {
		ensureZoomEyeExtra(asset)["country"] = it.CountryName
	}
	if it.CountryCode != "" {
		asset.CountryCode = it.CountryCode
	}
	if it.ProvinceName != "" {
		asset.Region = it.ProvinceName
	}
	if it.CityName != "" {
		asset.City = it.CityName
	}
	// 旧格式：geoinfo 结构
	if it.GeoInfo != nil {
		if asset.CountryCode == "" {
			if country, ok := it.GeoInfo["country"].(map[string]interface{}); ok {
				if code, ok := country["code"].(string); ok {
					asset.CountryCode = code
				}
			}
		}
		if asset.City == "" {
			if city, ok := it.GeoInfo["city"].(string); ok {
				asset.City = city
			}
		}
		if asset.Region == "" {
			if subdivisions, ok := it.GeoInfo["subdivisions"].(string); ok {
				asset.Region = subdivisions
			}
		}
	}
}

// parseZoomEyeNetwork 解析 ASN、组织和 ISP 信息
func parseZoomEyeNetwork(it *ZoomEyeItem, asset *model.UnifiedAsset) {
	// ASN: ZoomEye API returns numeric, json.Number handles both number and string
	if asn := it.ASN.String(); asn != "" {
		asset.ASN = asn
	}
	// Organization
	if it.OrgName != "" {
		asset.Org = it.OrgName
	} else if it.Org != "" {
		asset.Org = it.Org
	}
	// ISP
	if it.ISPName != "" {
		asset.ISP = it.ISPName
	} else if it.ISP != "" {
		asset.ISP = it.ISP
	}
	// Timestamp from Extension DOM extraction (last_seen) or API response (timestamp/icon-time)
	if it.LastSeen != "" {
		asset.LastSeen = it.LastSeen
	}
}

// parseZoomEyeExtra 解析 os/product/app/version/device/body/header 等扩展字段
func parseZoomEyeExtra(it *ZoomEyeItem, asset *model.UnifiedAsset) {
	setExtra := func(key, val string) {
		if val != "" {
			ensureZoomEyeExtra(asset)[key] = val
		}
	}
	setExtra("os", it.OS)
	setExtra("product", it.Product)
	setExtra("app", it.App)
	setExtra("version", it.Version)
	setExtra("device", it.Device)
	setExtra("body", it.Body)
	setExtra("header", it.Header)
}

// ensureZoomEyeExtra 确保 Extra map 已初始化
func ensureZoomEyeExtra(asset *model.UnifiedAsset) map[string]interface{} {
	if asset.Extra == nil {
		asset.Extra = make(map[string]interface{})
	}
	return asset.Extra
}

// GetQuota 获取ZoomEye配额信息
func (z *ZoomEyeAdapter) GetQuota() (*model.QuotaInfo, error) {
	if z.apiKey == "" {
		return nil, fmt.Errorf("ZoomEye API key not configured")
	}

	// ZoomEye API endpoint for quota info
	url := fmt.Sprintf("%s/resources-info", z.baseURL)

	resp, err := z.client.R().
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String())
	}

	// 打印响应体，方便调试
	logger.Debugf("ZoomEye quota response: %s", resp.String())

	// ZoomEye quota response structure
	var result struct {
		Code      int    `json:"code"`
		Plan      string `json:"plan"`
		Resources struct {
			Search   int    `json:"search"`
			Interval string `json:"interval"`
		} `json:"resources"`
		UserInfo struct {
			Name      string `json:"name"`
			Role      string `json:"role"`
			ExpiredAt string `json:"expired_at"`
		} `json:"user_info"`
		QuotaInfo struct {
			RemainFreeQuota  int `json:"remain_free_quota"`
			RemainPayQuota   int `json:"remain_pay_quota"`
			RemainTotalQuota int `json:"remain_total_quota"`
		} `json:"quota_info"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	if result.Code != 60000 {
		return nil, fmt.Errorf("ZoomEye API error code: %d", result.Code)
	}

	// 计算配额信息
	// ZoomEye的响应中，quota_info.remain_total_quota是剩余的总配额，resources.search是总配额
	total := result.Resources.Search
	remain := result.QuotaInfo.RemainTotalQuota
	used := total - remain

	// 打印解析后的配额信息
	logger.Infof("ZoomEye quota: total=%d, used=%d, remain=%d", total, used, remain)

	return &model.QuotaInfo{
		Remaining: remain,
		Total:     total,
		Used:      used,
		Unit:      "queries",
		Expiry:    result.UserInfo.ExpiredAt,
	}, nil
}

// IsWebOnly 检查是否为 Web-only 模式
func (z *ZoomEyeAdapter) IsWebOnly() bool {
	return false
}

// ZoomEyeAdapterWebOnly ZoomEye Web-only模式适配器
type ZoomEyeAdapterWebOnly struct {
	*WebOnlyAdapterBase
}

// NewZoomEyeAdapterWebOnly 创建ZoomEye Web-only适配器
func NewZoomEyeAdapterWebOnly() *ZoomEyeAdapterWebOnly {
	baseAdapter := NewZoomEyeAdapter("", "", 3, 30*time.Second)
	return &ZoomEyeAdapterWebOnly{
		WebOnlyAdapterBase: NewWebOnlyAdapterBase(baseAdapter, "zoomeye"),
	}
}
