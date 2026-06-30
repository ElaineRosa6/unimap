package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-resty/resty/v2"
	"github.com/unimap/project/internal/metrics"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/requestid"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// ensureUTF8 converts GBK-encoded strings to UTF-8.
// On Windows, curl and some HTTP clients send Chinese characters in GBK encoding.
// The ICP sidecar expects UTF-8, so we need to normalize.
func ensureUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	decoded, _, err := transform.String(simplifiedchinese.GBK.NewDecoder(), s)
	if err != nil {
		return s
	}
	return decoded
}

type ICPQueryType string

const (
	ICPWeb      ICPQueryType = "web"
	ICPApp      ICPQueryType = "app"
	ICPMiniApp  ICPQueryType = "mapp"
	ICPKuaiApp  ICPQueryType = "kapp"
	ICPBWeb     ICPQueryType = "bweb"
	ICPBApp     ICPQueryType = "bapp"
	ICPBMiniApp ICPQueryType = "bmapp"
	ICPBKuaiApp ICPQueryType = "bkapp"
)

// ICPResult unified model for all ICP query types.
// Web queries return: domain, mainId, detailId, domainId, serviceId
// App/MiniApp/KuaiApp queries return: serviceName, dataId (no domain/mainId)
// Use json.RawMessage for fields that vary by query type to avoid unmarshal errors.
type ICPResult struct {
	Domain        string          `json:"domain"`
	ServiceName   string          `json:"serviceName"`
	ServiceType   json.RawMessage `json:"serviceType"`
	Licence       string          `json:"licence"`
	UpdateRecord  string          `json:"updateRecord"`
	LimitAccess   string          `json:"limitAccess"`
	ContentName   string          `json:"contentTypeName"`
	MainLicence   string          `json:"serviceLicence"`
	NatureName    string          `json:"natureName"`
	CityName      string          `json:"cityName"`
	UnitName      string          `json:"unitName"`
	LeaderName    string          `json:"leaderName"`
	MainID        json.RawMessage `json:"mainId"`
	DetailID      json.RawMessage `json:"detailId"`
	DomainID      json.RawMessage `json:"domainId"`
	ServiceID     json.RawMessage `json:"serviceId"`
	DataID        json.RawMessage `json:"dataId"`
	MainLicWeb    string          `json:"mainLicence"` // web queries use this field for main licence
	UpdateRecTime string          `json:"updateRecordTime"`
}

// serviceTypeStr returns the service type as a human-readable string.
func (r *ICPResult) serviceTypeStr() string {
	if len(r.ServiceType) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(r.ServiceType, &s); err == nil {
		return s
	}
	var n int
	if err := json.Unmarshal(r.ServiceType, &n); err == nil {
		return fmt.Sprintf("%d", n)
	}
	return string(r.ServiceType)
}

// unitName returns the best available unit/organization name.
func (r *ICPResult) unitName() string {
	if r.UnitName != "" {
		return r.UnitName
	}
	return ""
}

// title returns the best available title/service name.
func (r *ICPResult) title() string {
	if r.ContentName != "" {
		return r.ContentName
	}
	if r.ServiceName != "" {
		return r.ServiceName
	}
	return ""
}

// host returns the best available host/domain.
func (r *ICPResult) host() string {
	return r.Domain
}

// licence returns the best available licence number.
func (r *ICPResult) licence() string {
	if r.Licence != "" {
		return r.Licence
	}
	if r.MainLicWeb != "" {
		return r.MainLicWeb
	}
	return r.MainLicence
}

type icpAPIParams struct {
	List  []ICPResult `json:"list"`
	Total int         `json:"total"`
	Page  int         `json:"pageNum"`
	Size  int         `json:"pageSize"`
	Pages int         `json:"pages"`
}

type icpAPIResponse struct {
	Code   int           `json:"code"`
	Msg    string        `json:"msg"`
	Params *icpAPIParams `json:"params"`
	Total  int           `json:"total"`
	EndID  string        `json:"endId"`
	List   []ICPResult   `json:"list"`
}

type ICPConfig struct {
	Enabled          bool
	BaseURL          string
	Timeout          int
	DefaultType      string
	DatabasePath     string
	RetryOnCaptcha   bool
	MaxRetries       int
	APIKey           string
	CircuitThreshold int
	CircuitResetDur  time.Duration
}

type ICPAdapter struct {
	client    *resty.Client
	baseURL   string
	queryType ICPQueryType
	apiKey    string
	timeout   time.Duration
}

func NewICPAdapter(cfg ICPConfig, queryType ICPQueryType) *ICPAdapter {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	client := resty.New().SetTimeout(timeout).SetRetryCount(2).SetRetryWaitTime(1 * time.Second)
	if cfg.APIKey != "" {
		client.SetHeader("X-ICP-API-Key", cfg.APIKey)
	}
	return &ICPAdapter{client: client, baseURL: cfg.BaseURL, queryType: queryType, apiKey: cfg.APIKey, timeout: timeout}
}

func (a *ICPAdapter) Name() string {
	return fmt.Sprintf("icp-%s", a.queryType)
}

func (a *ICPAdapter) Translate(ast *model.UQLAST) (string, error) {
	if ast == nil || ast.Root == nil {
		return "", fmt.Errorf("invalid AST")
	}
	query, err := a.extractSearchTerm(ast.Root)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("ICP query requires one of icp.domain, icp.company, icp.licence, domain, host, or org")
	}
	return query, nil
}

func (a *ICPAdapter) Search(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error) {
	return a.SearchWithContext(ctx, query, page, pageSize)
}

func (a *ICPAdapter) SearchWithContext(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	result, err := a.doSearch(ctx, query, page, pageSize)

	qt := string(a.queryType)
	if err != nil {
		metrics.IncICPQuery(qt, "error")
		metrics.ObserveICPQueryDuration(qt, time.Since(start))
		return nil, err
	}
	metrics.IncICPQuery(qt, "success")
	metrics.ObserveICPQueryDuration(qt, time.Since(start))
	return result, nil
}

func (a *ICPAdapter) extractSearchTerm(node *model.UQLNode) (string, error) {
	var query string
	err := a.walkICPConditions(node, &query)
	return query, err
}

func (a *ICPAdapter) walkICPConditions(node *model.UQLNode, query *string) error {
	if node == nil {
		return nil
	}
	if node.Type == "condition" && len(node.Children) >= 2 {
		field := strings.ToLower(strings.TrimSpace(node.Value))
		op := strings.ToUpper(strings.TrimSpace(node.Children[0].Value))
		value := strings.TrimSpace(node.Children[1].Value)

		if field == "icp.type" {
			if op != "=" && op != "==" {
				return fmt.Errorf("icp.type only supports equality")
			}
			if value != "" && !strings.EqualFold(value, string(a.queryType)) {
				return fmt.Errorf("icp.type %q does not match selected engine %s", value, a.Name())
			}
			return nil
		}

		if isICPSearchField(field) {
			if op != "=" && op != "==" && op != "CONTAINS" && op != "~=" && op != "IN" {
				return fmt.Errorf("%s does not support operator %s for ICP search", field, op)
			}
			if strings.TrimSpace(*query) != "" {
				return nil
			}
			if op == "IN" {
				for _, candidate := range strings.Split(value, ",") {
					if candidate = strings.TrimSpace(candidate); candidate != "" {
						*query = candidate
						return nil
					}
				}
				return nil
			}
			*query = value
			return nil
		}
	}

	for _, child := range node.Children {
		if err := a.walkICPConditions(child, query); err != nil {
			return err
		}
	}
	return nil
}

func isICPSearchField(field string) bool {
	switch field {
	case "icp.domain", "icp.company", "icp.licence", "domain", "host", "org":
		return true
	default:
		return false
	}
}

func (a *ICPAdapter) doSearch(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error) {
	var resp icpAPIResponse
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	req := a.client.R().SetContext(ctx).
		SetQueryParam("search", ensureUTF8(query)).
		SetQueryParam("pageNum", fmt.Sprintf("%d", page)).
		SetQueryParam("pageSize", fmt.Sprintf("%d", pageSize)).
		SetResult(&resp)
	if reqID := requestIDFromContext(ctx); reqID != "" {
		req.SetHeader("X-Request-ID", reqID)
	}
	httpResp, err := req.Get(fmt.Sprintf("%s/query/%s", strings.TrimRight(a.baseURL, "/"), a.queryType))
	if err != nil {
		return nil, fmt.Errorf("ICP query request failed: %w", err)
	}
	if httpResp.StatusCode() != 200 {
		return nil, fmt.Errorf("ICP API returned HTTP %d: %s", httpResp.StatusCode(), httpResp.String())
	}
	if resp.Code != 200 {
		if strings.Contains(strings.ToLower(resp.Msg), "captcha") || strings.Contains(resp.Msg, "验证码") {
			metrics.IncICPCaptchaFailure(string(a.queryType))
		}
		return nil, fmt.Errorf("ICP API error: %s", resp.Msg)
	}

	// ICP service returns: {"code": 200, "params": {"list": [...], "total": N}}
	// Legacy format: {"code": 200, "list": [...], "total": N}
	var results []ICPResult
	var total int
	if resp.Params != nil && len(resp.Params.List) > 0 {
		results = resp.Params.List
		total = resp.Params.Total
	} else {
		results = resp.List
		total = resp.Total
	}

	assets := make([]model.UnifiedAsset, 0, len(results))
	rawData := make([]interface{}, 0, len(results))
	for _, r := range results {
		rawData = append(rawData, r)
		asset := model.UnifiedAsset{
			Host:   r.host(),
			Title:  r.title(),
			Org:    r.unitName(),
			Source: a.Name(),
			Extra: map[string]interface{}{
				"icp_licence":       r.licence(),
				"icp_main_licence":  r.MainLicence,
				"icp_nature":        r.NatureName,
				"icp_city":          r.CityName,
				"icp_service_type":  r.serviceTypeStr(),
				"icp_detail_id":     string(r.DetailID),
				"icp_update_record": r.UpdateRecTime,
				"icp_limit_access":  r.LimitAccess,
				"icp_domain":        r.Domain,
				"icp_service_name":  r.ServiceName,
				"icp_data_id":       string(r.DataID),
			},
		}
		assets = append(assets, asset)
	}
	return &model.EngineResult{EngineName: a.Name(), Total: total, RawData: rawData, Page: page, HasMore: len(results) >= pageSize}, nil
}

func (a *ICPAdapter) Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error) {
	if raw == nil || len(raw.RawData) == 0 {
		return []model.UnifiedAsset{}, nil
	}
	assets := make([]model.UnifiedAsset, 0, len(raw.RawData))
	for _, item := range raw.RawData {
		r, ok := item.(ICPResult)
		if !ok {
			continue
		}
		assets = append(assets, model.UnifiedAsset{
			Host:   r.host(),
			Title:  r.title(),
			Org:    r.unitName(),
			Source: a.Name(),
			Extra: map[string]interface{}{
				"icp_licence":       r.licence(),
				"icp_main_licence":  r.MainLicence,
				"icp_nature":        r.NatureName,
				"icp_city":          r.CityName,
				"icp_service_type":  r.serviceTypeStr(),
				"icp_detail_id":     string(r.DetailID),
				"icp_update_record": r.UpdateRecTime,
				"icp_limit_access":  r.LimitAccess,
				"icp_domain":        r.Domain,
				"icp_service_name":  r.ServiceName,
				"icp_data_id":       string(r.DataID),
			},
		})
	}
	return assets, nil
}

func (a *ICPAdapter) GetQuota() (*model.QuotaInfo, error) {
	return &model.QuotaInfo{Remaining: -1, Total: -1, Unit: "unlimited"}, nil
}

func (a *ICPAdapter) IsWebOnly() bool {
	return false
}

func AllICPQueryTypes() []ICPQueryType {
	return []ICPQueryType{ICPWeb, ICPApp, ICPMiniApp, ICPKuaiApp, ICPBWeb, ICPBApp, ICPBMiniApp, ICPBKuaiApp}
}

func ICPTypeLabel(t ICPQueryType) string {
	labels := map[ICPQueryType]string{
		ICPWeb: "网站备案", ICPApp: "APP备案", ICPMiniApp: "小程序备案", ICPKuaiApp: "快应用备案",
		ICPBWeb: "网站黑名单", ICPBApp: "APP黑名单", ICPBMiniApp: "小程序黑名单", ICPBKuaiApp: "快应用黑名单",
	}
	if l, ok := labels[t]; ok {
		return l
	}
	return string(t)
}

func IsValidICPQueryType(raw string) bool {
	for _, t := range AllICPQueryTypes() {
		if raw == string(t) {
			return true
		}
	}
	return false
}

func (a *ICPAdapter) HealthCheck(ctx context.Context) error {
	resp, err := a.client.R().SetContext(ctx).Get(strings.TrimRight(a.baseURL, "/"))
	if err != nil {
		return fmt.Errorf("ICP health check failed: %w", err)
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("ICP health check returned HTTP %d", resp.StatusCode())
	}
	return nil
}

type ICPSearchRequest struct {
	Query    string `json:"query"`
	Type     string `json:"type"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

func ICPSearch(baseURL string, apiKey string, req ICPSearchRequest) ([]ICPResult, int, error) {
	return ICPSearchWithContext(context.Background(), baseURL, apiKey, req)
}

func ICPSearchWithContext(ctx context.Context, baseURL string, apiKey string, req ICPSearchRequest) ([]ICPResult, int, error) {
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	if !IsValidICPQueryType(req.Type) {
		return nil, 0, fmt.Errorf("invalid ICP query type: %s", req.Type)
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	client := resty.New().SetTimeout(30 * time.Second)
	if apiKey != "" {
		client.SetHeader("X-ICP-API-Key", apiKey)
	}
	var resp icpAPIResponse
	httpResp, err := client.R().SetContext(ctx).
		SetQueryParam("search", ensureUTF8(req.Query)).
		SetQueryParam("pageNum", fmt.Sprintf("%d", req.Page)).
		SetQueryParam("pageSize", fmt.Sprintf("%d", req.PageSize)).
		SetResult(&resp).
		Get(fmt.Sprintf("%s/query/%s", strings.TrimRight(baseURL, "/"), req.Type))
	if err != nil {
		return nil, 0, fmt.Errorf("ICP query request failed: %w", err)
	}
	if httpResp.StatusCode() != 200 {
		return nil, 0, fmt.Errorf("ICP API returned HTTP %d: %s", httpResp.StatusCode(), httpResp.String())
	}
	if resp.Code != 200 {
		return nil, 0, fmt.Errorf("ICP API error: %s", resp.Msg)
	}

	// Use params.list if available (new format), fall back to top-level list
	if resp.Params != nil && len(resp.Params.List) > 0 {
		return resp.Params.List, resp.Params.Total, nil
	}
	return resp.List, resp.Total, nil
}

// ICPTypeGroup 表示单个类型的查询结果分组，用于多类型查询返回。
type ICPTypeGroup struct {
	Type    string      `json:"type"`
	Label   string      `json:"label"`
	Total   int         `json:"total"`
	Results []ICPResult `json:"results"`
	Error   string      `json:"error,omitempty"`
}

// ICPSearchMultiType 并发查询多个 ICP 备案类型。
// 每个 type 独立请求 sidecar，任一 type 失败只记入该组 Error，不影响其它组。
// 返回结果按 types 入参顺序排列。
func ICPSearchMultiType(ctx context.Context, baseURL, apiKey, query string, types []string, page, pageSize int) ([]ICPTypeGroup, error) {
	if len(types) == 0 {
		return nil, fmt.Errorf("no ICP types provided")
	}

	groups := make([]ICPTypeGroup, len(types))
	var wg sync.WaitGroup

	for i, t := range types {
		wg.Add(1)
		go func(idx int, tp string) {
			defer wg.Done()
			results, total, err := ICPSearchWithContext(ctx, baseURL, apiKey, ICPSearchRequest{
				Query:    query,
				Type:     tp,
				Page:     page,
				PageSize: pageSize,
			})
			g := ICPTypeGroup{Type: tp, Label: ICPTypeLabel(ICPQueryType(tp))}
			if err != nil {
				g.Error = err.Error()
			} else {
				g.Results = results
				g.Total = total
			}
			groups[idx] = g
		}(i, t)
	}

	wg.Wait()
	return groups, nil
}

func requestIDFromContext(ctx context.Context) string {
	return requestid.FromContext(ctx)
}
