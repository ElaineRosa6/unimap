package screenshot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		URLPattern:    "/api/visitor/search/query_string/quake_service",
		Method:        "POST",
		ParseResponse: parseQuakeNetworkResponse,
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

	allocCtx, allocCancel, err := m.newAllocator(ctx)
	if err != nil {
		return nil, err
	}
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	var mu sync.Mutex
	captured := &networkResponse{}
	respCh := make(chan struct{}, 1)

	chromedp.ListenTarget(browserCtx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventResponseReceived:
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

	if err := chromedp.Run(browserCtx, network.Enable()); err != nil {
		return nil, fmt.Errorf("enable network: %w", err)
	}

	logger.Infof("L1: navigating to %s for engine %s", searchURL, engine)
	if err := chromedp.Run(browserCtx, chromedp.Navigate(searchURL)); err != nil {
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
		return nil, fmt.Errorf("L1: timeout waiting for %s search API response", engine)
	}
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
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Total int `json:"total"`
			Hits  []struct {
				IP       string `json:"ip"`
				Port     int    `json:"port"`
				Hostname string `json:"hostname"`
				Service  struct {
					Name string `json:"name"`
				} `json:"service"`
				Transport string `json:"transport"`
				Title     struct {
					Title string `json:"title"`
				} `json:"title"`
				Location struct {
					CountryCode string `json:"country_code"`
					City        string `json:"city_cn"`
				} `json:"location"`
				AS struct {
					ASN  string `json:"asn"`
					Name string `json:"name"`
					ISP  string `json:"isp"`
				} `json:"autonomous_system"`
				Server string `json:"server"`
			} `json:"hits"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("failed to parse Quake response: %w", err)
	}
	if resp.Code != 0 {
		return nil, 0, fmt.Errorf("Quake API error: code=%d message=%s", resp.Code, resp.Message)
	}
	assets := make([]model.UnifiedAsset, 0, len(resp.Data.Hits))
	for _, hit := range resp.Data.Hits {
		proto := hit.Transport
		if proto == "" {
			proto = hit.Service.Name
		}
		assets = append(assets, model.UnifiedAsset{
			IP: hit.IP, Port: hit.Port, Protocol: proto, Host: hit.Hostname,
			Title: hit.Title.Title, CountryCode: hit.Location.CountryCode, City: hit.Location.City,
			ASN: hit.AS.ASN, Org: hit.AS.Name, ISP: hit.AS.ISP,
			Server: hit.Server, Source: "quake",
		})
	}
	return assets, resp.Data.Total, nil
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
