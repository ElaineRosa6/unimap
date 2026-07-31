package screenshot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/unimap/project/internal/collection"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/model"
)

// networkResponse 捕获的网络响应
type networkResponse struct {
	URL        string
	RequestID  network.RequestID
	StatusCode int
	Body       []byte
}

// l1SearchAPIConfig 每个引擎的 L1 搜索 API 配置
type l1SearchAPIConfig struct {
	URLPattern    string
	Method        string
	ParseResponse func(body []byte) ([]model.UnifiedAsset, int, error)
}

// l1SearchAPIs 定义支持 L1 采集的引擎搜索 API
var l1SearchAPIs = map[string]l1SearchAPIConfig{
	"zoomeye": {
		URLPattern:    "/api/search?",
		Method:        "GET",
		ParseResponse: parseZoomEyeNetworkResponse,
	},
	"hunter": {
		URLPattern:    "/api/search",
		Method:        "POST",
		ParseResponse: parseHunterNetworkResponse,
	},
	"quake": {
		URLPattern:    "/api/search/query_string/quake_service",
		Method:        "POST",
		ParseResponse: parseQuakeNetworkResponse,
	},
	// FOFA and Shodan L1 patterns added for P2 CDP grading prep (2026-08-01).
	// URL patterns and response parsers need real calibration during P2.
	"fofa": {
		URLPattern:    "/api/search",
		Method:        "GET",
		ParseResponse: parseFofaNetworkResponse,
	},
	"shodan": {
		URLPattern:    "/api/search",
		Method:        "GET",
		ParseResponse: parseShodanNetworkResponse,
	},
}

// IsL1Supported 检查引擎是否支持 L1 Network 采集
func IsL1Supported(engine string) bool {
	_, ok := l1SearchAPIs[strings.ToLower(engine)]
	return ok
}

// CollectViaNetwork 通过 CDP Network 拦截搜索 API 响应进行 L1 采集
func (m *Manager) CollectViaNetwork(ctx context.Context, engine, query, queryID string) ([]collection.CollectResult, error) {
	engineKey := strings.ToLower(engine)
	apiConfig, ok := l1SearchAPIs[engineKey]
	if !ok {
		return nil, fmt.Errorf("L1 network collection not supported for engine: %s", engine)
	}
	searchURL := m.BuildSearchEngineURL(engine, query)
	if searchURL == "" {
		return nil, fmt.Errorf("unsupported engine: %s", engine)
	}

	collectTimeout := m.timeout
	if collectTimeout <= 0 || collectTimeout > 60*time.Second {
		collectTimeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, collectTimeout)
	defer cancel()

	session, err := m.newGuardedBrowserSession(ctx, searchURL, "")
	if err != nil {
		return nil, err
	}
	defer session.Close()
	browserCtx := session.Context()

	if cookies := m.GetCookies(engine); len(cookies) > 0 {
		if err := chromedp.Run(browserCtx, setCookieActions(cookies, searchURL)...); err != nil {
			logger.Warnf("inject cookies failed for %s: %v", engine, err)
		}
	}

	var mu sync.Mutex
	captured := &networkResponse{}
	observedPaths := make(map[string]struct{})
	respCh := make(chan struct{}, 1)

	chromedp.ListenTarget(browserCtx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventResponseReceived:
			if e.Type == network.ResourceTypeXHR || e.Type == network.ResourceTypeFetch {
				label := networkResponseLabel(e.Response.URL)
				if label == "" {
					break
				}
				mu.Lock()
				if len(observedPaths) < 50 {
					observedPaths[label] = struct{}{}
				}
				mu.Unlock()
			}
			if strings.Contains(e.Response.URL, apiConfig.URLPattern) {
				mu.Lock()
				if captured.URL == "" {
					captured.URL = e.Response.URL
					captured.RequestID = e.RequestID
					captured.StatusCode = int(e.Response.Status)
				}
				mu.Unlock()
			}
		case *network.EventLoadingFinished:
			mu.Lock()
			needFetch := captured.URL != "" && captured.Body == nil && e.RequestID == captured.RequestID
			reqID := captured.RequestID
			mu.Unlock()
			if needFetch {
				go func() {
					var body []byte
					if err := chromedp.Run(browserCtx, chromedp.ActionFunc(func(ctx context.Context) error {
						b, err := network.GetResponseBody(reqID).Do(ctx)
						if err != nil {
							return err
						}
						body = b
						return nil
					})); err != nil {
						logger.Warnf("L1: failed to get response body: %v", err)
						return
					}
					mu.Lock()
					captured.Body = body
					mu.Unlock()
					select {
					case respCh <- struct{}{}:
					default:
					}
				}()
			}
		}
	})

	if err := session.Run(network.Enable()); err != nil {
		return nil, fmt.Errorf("enable network: %w", err)
	}

	logger.Infof("L1: navigating to %s for engine %s", searchURL, engine)
	if err := session.Run(chromedp.Navigate(searchURL)); err != nil {
		return nil, fmt.Errorf("navigate failed: %w", err)
	}

	select {
	case <-respCh:
		mu.Lock()
		resp := *captured
		mu.Unlock()
		if resp.Body == nil {
			return nil, fmt.Errorf("L1: failed to capture response body")
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("L1: API returned status %d", resp.StatusCode)
		}
		return m.buildL1Result(engine, query, resp, apiConfig.ParseResponse)
	case <-ctx.Done():
		mu.Lock()
		observed := make([]string, 0, len(observedPaths))
		for path := range observedPaths {
			observed = append(observed, path)
		}
		mu.Unlock()
		sort.Strings(observed)
		return nil, fmt.Errorf("L1: timeout waiting for %s search API response; observed=%v", engine, observed)
	}
}

func networkResponseLabel(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if !strings.Contains(host, "quake") && !strings.Contains(parsed.Path, "search") && !strings.Contains(parsed.Path, "/api/") {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host + parsed.EscapedPath()
}

// buildL1Result 解析 L1 捕获的响应并构建 CollectResult
func (m *Manager) buildL1Result(engine, query string, resp networkResponse, parseFn func([]byte) ([]model.UnifiedAsset, int, error)) ([]collection.CollectResult, error) {
	assets, total, err := parseFn(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("L1: failed to parse response: %w", err)
	}
	logger.Infof("L1: captured %d assets from %s (total=%d)", len(assets), engine, total)
	return []collection.CollectResult{{
		Engine: engine, Query: query, RawURL: resp.URL,
		Title: fmt.Sprintf("L1 Network: %s", engine), Timestamp: time.Now().Unix(),
		Assets: assets, Total: total, HasMore: len(assets) < total,
	}}, nil
}

// parseZoomEyeNetworkResponse 解析 ZoomEye 搜索 API 响应
func parseZoomEyeNetworkResponse(body []byte) ([]model.UnifiedAsset, int, error) {
	var resp struct {
		Total   int `json:"total"`
		Matches []struct {
			IP           string `json:"ip"`
			Port         int    `json:"portinfo.port"`
			Service      string `json:"portinfo.service"`
			Title        string `json:"title"`
			Domain       string `json:"domain"`
			Hostname     string `json:"hostname"`
			Country      string `json:"geoinfo.country.code"`
			City         string `json:"geoinfo.city"`
			Subdivisions string `json:"geoinfo.subdivisions"`
			Org          string `json:"organization"`
			ISP          string `json:"geoinfo.isp"`
			ASN          int    `json:"asn"`
			Banner       string `json:"portinfo.banner"`
			Server       string `json:"portinfo.header.server"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return parseZoomEyeNetworkResponseAlt(body)
	}
	if len(resp.Matches) == 0 {
		return parseZoomEyeNetworkResponseAlt(body)
	}
	assets := make([]model.UnifiedAsset, 0, len(resp.Matches))
	for _, m := range resp.Matches {
		a := model.UnifiedAsset{
			IP: m.IP, Port: m.Port, Protocol: m.Service, Host: m.Domain,
			Title: m.Title, CountryCode: m.Country, City: m.City,
			Region: m.Subdivisions, Org: m.Org, ISP: m.ISP, Source: "zoomeye",
		}
		if m.Hostname != "" && a.Host == "" {
			a.Host = m.Hostname
		}
		if m.ASN > 0 {
			a.ASN = fmt.Sprintf("%d", m.ASN)
		}
		if m.Server != "" {
			a.Server = m.Server
		}
		if m.Banner != "" {
			if len(m.Banner) > 200 {
				a.BodySnippet = m.Banner[:200]
			} else {
				a.BodySnippet = m.Banner
			}
		}
		assets = append(assets, a)
	}
	return assets, resp.Total, nil
}

// zoomEyeNetworkResult is a typed alternative parser for ZoomEye search API responses.
type zoomEyeNetworkResult struct {
	IP      string  `json:"ip"`
	Port    float64 `json:"port"`
	Service string  `json:"service"`
	Domain  string  `json:"domain"`
	Title   string  `json:"title"`
}

func parseZoomEyeNetworkResponseAlt(body []byte) ([]model.UnifiedAsset, int, error) {
	var resp struct {
		Total   int                    `json:"total"`
		Results []zoomEyeNetworkResult `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("failed to parse ZoomEye response: %w", err)
	}
	assets := make([]model.UnifiedAsset, 0, len(resp.Results))
	for _, item := range resp.Results {
		assets = append(assets, model.UnifiedAsset{
			Source:   "zoomeye",
			IP:       item.IP,
			Port:     int(item.Port),
			Protocol: item.Service,
			Host:     item.Domain,
			Title:    item.Title,
		})
	}
	return assets, resp.Total, nil
}

// parseHunterNetworkResponse 解析 Hunter 搜索 API 响应
func parseHunterNetworkResponse(body []byte) ([]model.UnifiedAsset, int, error) {
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Total int `json:"total"`
			Arr   []struct {
				IP           string `json:"ip"`
				Port         int    `json:"port"`
				Domain       string `json:"domain"`
				Protocol     string `json:"protocol"`
				WebTitle     string `json:"web_title"`
				StatusCode   int    `json:"status_code"`
				HeaderServer string `json:"header_server"`
				Country      string `json:"country"`
				Province     string `json:"province"`
				City         string `json:"city"`
				ISP          string `json:"isp"`
				ASNOrg       string `json:"as_org"`
				URL          string `json:"url"`
				ASN          string `json:"asn"`
			} `json:"arr"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("failed to parse Hunter response: %w", err)
	}
	if resp.Code != 200 {
		return nil, 0, fmt.Errorf("Hunter API error: code=%d message=%s", resp.Code, resp.Message)
	}
	assets := make([]model.UnifiedAsset, 0, len(resp.Data.Arr))
	for _, item := range resp.Data.Arr {
		assets = append(assets, model.UnifiedAsset{
			IP: item.IP, Port: item.Port, Protocol: item.Protocol, Host: item.Domain,
			Title: item.WebTitle, StatusCode: item.StatusCode, Server: item.HeaderServer,
			CountryCode: item.Country, Region: item.Province, City: item.City,
			ISP: item.ISP, Org: item.ASNOrg, URL: item.URL, ASN: item.ASN, Source: "hunter",
		})
	}
	collection.NormalizeAssets("hunter", assets)
	return assets, resp.Data.Total, nil
}

// parseQuakeNetworkResponse 解析 Quake 搜索 API 响应
func parseQuakeNetworkResponse(body []byte) ([]model.UnifiedAsset, int, error) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, 0, fmt.Errorf("failed to parse Quake response: %w", err)
	}
	code := quakeInt(root["code"])
	if code != 0 {
		return nil, 0, fmt.Errorf("Quake API error: code=%d message=%s", code, quakeString(root["message"]))
	}

	var hits []any
	total := 0
	switch data := root["data"].(type) {
	case []any:
		hits = data
	case map[string]any:
		hits, _ = data["hits"].([]any)
		total = quakeInt(data["total"])
	}
	if total == 0 {
		total = quakeIntAt(root, "meta.pagination.total", "meta.total")
	}

	assets := make([]model.UnifiedAsset, 0, len(hits))
	for _, rawHit := range hits {
		hit, ok := rawHit.(map[string]any)
		if !ok {
			continue
		}
		protocol := quakeStringAt(hit, "service.name", "transport")
		var extra map[string]interface{}
		if countryName := quakeStringAt(hit, "location.country_cn", "location.country"); countryName != "" {
			extra = map[string]interface{}{"country_name": countryName}
		}
		assets = append(assets, model.UnifiedAsset{
			IP:          quakeStringAt(hit, "ip"),
			Port:        quakeIntAt(hit, "port"),
			Protocol:    protocol,
			Host:        quakeStringAt(hit, "hostname", "service.http.host"),
			Title:       quakeStringAt(hit, "title.title", "service.http.title", "service.http.response.html_title"),
			CountryCode: quakeStringAt(hit, "location.country_code"),
			Region:      quakeStringAt(hit, "location.province_cn", "location.province"),
			City:        quakeStringAt(hit, "location.city_cn", "location.city"),
			ASN:         quakeStringAt(hit, "autonomous_system.asn", "asn"),
			Org:         quakeStringAt(hit, "autonomous_system.name", "org"),
			ISP:         quakeStringAt(hit, "autonomous_system.isp", "isp"),
			Server:      quakeStringAt(hit, "server", "service.http.response.headers.server"),
			Source:      "quake",
			Extra:       extra,
		})
	}
	collection.NormalizeAssets("quake", assets)
	return assets, total, nil
}

func quakeStringAt(root map[string]any, paths ...string) string {
	for _, path := range paths {
		var current any = root
		for _, part := range strings.Split(path, ".") {
			object, ok := current.(map[string]any)
			if !ok {
				current = nil
				break
			}
			current = object[part]
		}
		if value := quakeString(current); value != "" {
			return value
		}
	}
	return ""
}

func quakeIntAt(root map[string]any, paths ...string) int {
	for _, path := range paths {
		var current any = root
		for _, part := range strings.Split(path, ".") {
			object, ok := current.(map[string]any)
			if !ok {
				current = nil
				break
			}
			current = object[part]
		}
		if value := quakeInt(current); value != 0 {
			return value
		}
	}
	return 0
}

func quakeString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case json.Number:
		return typed.String()
	case []any:
		for _, item := range typed {
			if value := quakeString(item); value != "" {
				return value
			}
		}
	}
	return ""
}

func quakeInt(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := strconv.Atoi(typed.String())
		return parsed
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	}
	return 0
}

// parseFofaNetworkResponse 解析 FOFA 搜索 API 响应。
// FOFA Web 前端调用 /api/search 返回两种已知格式：
//   - 对象数组：{"error":false,"size":N,"results":[{"ip":"...","port":"80",...}]}
//   - 二维数组：{"error":false,"results":[["1.1.1.1","80","http","title","CN"],...]}
//
// 本解析器兼容两种格式。字段名基于 FOFA 公开 API 文档，P2 真实定级时需校准。
func parseFofaNetworkResponse(body []byte) ([]model.UnifiedAsset, int, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, 0, fmt.Errorf("failed to parse FOFA response: %w", err)
	}

	// Check error flag.
	if errRaw, ok := root["error"]; ok {
		var errVal bool
		if json.Unmarshal(errRaw, &errVal) == nil && errVal {
			msg := "unknown FOFA error"
			if emsg, ok := root["errmsg"]; ok {
				var s string
				if json.Unmarshal(emsg, &s) == nil && s != "" {
					msg = s
				}
			}
			return nil, 0, fmt.Errorf("FOFA API error: %s", msg)
		}
	}

	total := 0
	if sizeRaw, ok := root["size"]; ok {
		var n json.Number
		if json.Unmarshal(sizeRaw, &n) == nil {
			total, _ = strconv.Atoi(n.String())
		}
	}

	resultsRaw, ok := root["results"]
	if !ok {
		return nil, 0, fmt.Errorf("FOFA response missing results field")
	}

	// Try object array format first.
	var objResults []map[string]any
	if json.Unmarshal(resultsRaw, &objResults) == nil && len(objResults) > 0 {
		return parseFofaObjectResults(objResults, total)
	}

	// Fall back to 2D array format.
	var arrResults [][]string
	if json.Unmarshal(resultsRaw, &arrResults) == nil && len(arrResults) > 0 {
		return parseFofaArrayResults(arrResults, total)
	}

	return nil, 0, nil // empty results
}

func parseFofaObjectResults(results []map[string]any, total int) ([]model.UnifiedAsset, int, error) {
	var assets []model.UnifiedAsset
	for _, hit := range results {
		port := 0
		if p := quakeStringAt(hit, "port"); p != "" {
			port, _ = strconv.Atoi(p)
		}
		assets = append(assets, model.UnifiedAsset{
			IP:          quakeStringAt(hit, "ip", "host", "domain"),
			Port:        port,
			Protocol:    quakeStringAt(hit, "protocol", "service"),
			Host:        quakeStringAt(hit, "host", "domain"),
			Title:       quakeStringAt(hit, "title", "header"),
			CountryCode: quakeStringAt(hit, "country_code", "country"),
			Region:      quakeStringAt(hit, "region", "province"),
			City:        quakeStringAt(hit, "city"),
			Org:         quakeStringAt(hit, "org", "isp"),
			Server:      quakeStringAt(hit, "server", "product"),
			Source:      "fofa",
		})
	}
	collection.NormalizeAssets("fofa", assets)
	return assets, total, nil
}

// parseFofaArrayResults handles the legacy 2D array format where fields are
// positionally mapped: [ip, port, protocol, title, country, city, org, ...].
func parseFofaArrayResults(results [][]string, total int) ([]model.UnifiedAsset, int, error) {
	var assets []model.UnifiedAsset
	for _, row := range results {
		if len(row) < 2 {
			continue
		}
		asset := model.UnifiedAsset{Source: "fofa"}
		if len(row) > 0 {
			asset.IP = strings.TrimSpace(row[0])
		}
		if len(row) > 1 {
			asset.Port, _ = strconv.Atoi(strings.TrimSpace(row[1]))
		}
		if len(row) > 2 {
			asset.Protocol = strings.TrimSpace(row[2])
		}
		if len(row) > 3 {
			asset.Title = strings.TrimSpace(row[3])
		}
		if len(row) > 4 {
			asset.CountryCode = strings.TrimSpace(row[4])
		}
		if len(row) > 5 {
			asset.City = strings.TrimSpace(row[5])
		}
		if len(row) > 6 {
			asset.Org = strings.TrimSpace(row[6])
		}
		if asset.IP != "" || asset.Host != "" {
			assets = append(assets, asset)
		}
	}
	collection.NormalizeAssets("fofa", assets)
	return assets, total, nil
}

// parseShodanNetworkResponse 解析 Shodan 搜索 API 响应。
// Shodan Web 前端可能调用 /api/search 或内部端点。响应格式基于 Shodan 公开 API：
//
//	{"total":N,"matches":[{"ip_str":"...","port":80,"transport":"tcp",...}]}
//
// P2 真实定级时需校准实际 Web 前端调用的端点和字段。
func parseShodanNetworkResponse(body []byte) ([]model.UnifiedAsset, int, error) {
	var resp struct {
		Total   int `json:"total"`
		Matches []struct {
			IPStr     string   `json:"ip_str"`
			Port      int      `json:"port"`
			Transport string   `json:"transport"`
			Hostnames []string `json:"hostnames"`
			Location  struct {
				CountryCode string `json:"country_code"`
				CountryName string `json:"country_name"`
				City        string `json:"city"`
				RegionCode  string `json:"region_code"`
			} `json:"location"`
			HTTP struct {
				Title  string `json:"title"`
				Server string `json:"server"`
				Host   string `json:"host"`
			} `json:"http"`
			Org     string `json:"org"`
			ISP     string `json:"isp"`
			Product string `json:"product"`
			Version string `json:"version"`
		} `json:"matches"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("failed to parse Shodan response: %w", err)
	}
	if resp.Error != "" {
		return nil, 0, fmt.Errorf("Shodan API error: %s", resp.Error)
	}

	var assets []model.UnifiedAsset
	for _, m := range resp.Matches {
		host := ""
		if len(m.Hostnames) > 0 {
			host = m.Hostnames[0]
		}
		if host == "" {
			host = m.HTTP.Host
		}
		region := m.Location.RegionCode
		org := m.Org
		if org == "" {
			org = m.ISP
		}
		assets = append(assets, model.UnifiedAsset{
			IP:          m.IPStr,
			Port:        m.Port,
			Protocol:    m.Transport,
			Host:        host,
			Title:       m.HTTP.Title,
			CountryCode: m.Location.CountryCode,
			Region:      region,
			City:        m.Location.City,
			Org:         org,
			Server:      m.HTTP.Server,
			Source:      "shodan",
		})
	}
	collection.NormalizeAssets("shodan", assets)
	return assets, resp.Total, nil
}

// FetchSearchResultDirect 直接通过 HTTP 调用引擎搜索 API（不经过浏览器）
func FetchSearchResultDirect(engine, query string, page, pageSize int, apiKey string) ([]model.UnifiedAsset, int, error) {
	switch strings.ToLower(engine) {
	case "zoomeye":
		return fetchZoomEyeDirect(query, page, pageSize, apiKey)
	default:
		return nil, 0, fmt.Errorf("direct fetch not supported for engine: %s", engine)
	}
}

func fetchZoomEyeDirect(query string, page, pageSize int, apiKey string) ([]model.UnifiedAsset, int, error) {
	url := fmt.Sprintf("https://api.zoomeye.org/api/search?q=%s&page=%d&t=v4+v6+web", query, page)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("API-KEY", apiKey)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != 200 {
		return nil, 0, fmt.Errorf("ZoomEye API returned status %d", resp.StatusCode)
	}
	return parseZoomEyeNetworkResponse(body)
}
