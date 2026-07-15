package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/unimap/project/internal/proxypool"
	"github.com/unimap/project/internal/utils/urlguard"
	"github.com/unimap/project/internal/utils/workerpool"
)

var defaultScanPorts = []int{80, 81, 443, 8000, 8080, 8443, 9000}

// ParsePortSpec parses comma-separated ports, inclusive ranges, or the
// aliases "all"/"full". The returned ports are unique and sorted.
func ParsePortSpec(spec string) ([]int, error) {
	spec = strings.ToLower(strings.TrimSpace(spec))
	if spec == "" {
		return nil, nil
	}
	if spec == "all" || spec == "full" {
		spec = "1-65535"
	}

	selected := make([]bool, 65536)
	for _, rawToken := range strings.Split(spec, ",") {
		token := strings.TrimSpace(rawToken)
		if token == "" {
			return nil, fmt.Errorf("empty port item")
		}
		start, end := 0, 0
		if strings.Contains(token, "-") {
			parts := strings.Split(token, "-")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid port range %q", token)
			}
			var err error
			start, err = strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid port range %q", token)
			}
			end, err = strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid port range %q", token)
			}
		} else {
			port, err := strconv.Atoi(token)
			if err != nil {
				return nil, fmt.Errorf("invalid port %q", token)
			}
			start, end = port, port
		}
		if start < 1 || end > 65535 || start > end {
			return nil, fmt.Errorf("port range %q is outside 1-65535", token)
		}
		for port := start; port <= end; port++ {
			selected[port] = true
		}
	}

	ports := make([]int, 0)
	for port := 1; port <= 65535; port++ {
		if selected[port] {
			ports = append(ports, port)
		}
	}
	return ports, nil
}

type URLPortScanSummary struct {
	Total         int `json:"total"`
	FormatValid   int `json:"formatValid"`
	InvalidFormat int `json:"invalidFormat"`
	CDNExcluded   int `json:"cdnExcluded"`
	Scanned       int `json:"scanned"`
	ResolveFailed int `json:"resolveFailed"`
	ScanFailed    int `json:"scanFailed"`
	Blocked       int `json:"blocked"`
}

type URLPortScanResult struct {
	Input                string           `json:"input"`
	URL                  string           `json:"url,omitempty"`
	Host                 string           `json:"host,omitempty"`
	Status               string           `json:"status"`
	Reason               string           `json:"reason,omitempty"`
	CDNDetected          bool             `json:"cdn_detected"`
	CDNReasons           []string         `json:"cdn_reasons,omitempty"`
	ResolvedIPs          []string         `json:"resolved_ips,omitempty"`
	ScannedIPs           []string         `json:"scanned_ips,omitempty"`
	OpenPorts            map[string][]int `json:"open_ports,omitempty"`
	AttemptedConnections int              `json:"attempted_connections,omitempty"`
	ExpectedConnections  int              `json:"expected_connections,omitempty"`
	DurationMS           int64            `json:"duration_ms,omitempty"`
}

type URLPortScanResponse struct {
	Summary    URLPortScanSummary  `json:"summary"`
	Ports      []int               `json:"ports,omitempty"`
	PortCount  int                 `json:"port_count"`
	DurationMS int64               `json:"duration_ms"`
	Results    []URLPortScanResult `json:"results"`
}

// PortScanOptions controls bounded target and TCP connection concurrency.
type PortScanOptions struct {
	TargetConcurrency int
	PortConcurrency   int
	ConnectTimeout    time.Duration
	ScanTimeout       time.Duration
}

type portScanTaskPayload struct {
	index int
	item  URLPortScanResult
}

type portScanTask struct {
	ctx             context.Context
	index           int
	input           string
	ports           []int
	proxyPool       *proxypool.Pool
	portConcurrency int
	connectTimeout  time.Duration
	scanTimeout     time.Duration
	resultChan      chan<- portScanTaskPayload
	wg              *sync.WaitGroup
}

func (t *portScanTask) Execute() error {
	defer t.wg.Done()

	normalizedURL, host, ok := t.validateAndResolveHost()
	if !ok {
		return nil
	}

	ips, ok := t.resolveHostIPs(normalizedURL, host)
	if !ok {
		return nil
	}

	if ok := t.checkCDNExclusion(normalizedURL, host, ips); !ok {
		return nil
	}

	t.scanAndReport(normalizedURL, host, ips)
	return nil
}

// validateAndResolveHost normalizes the URL and resolves the hostname.
func (t *portScanTask) validateAndResolveHost() (normalizedURL, host string, ok bool) {
	normalizedURL, normalizeErr := normalizeMonitorURLForService(t.input)
	if normalizeErr != nil {
		t.sendSimpleResult(t.input, "", "invalid_format", normalizeErr.Error())
		return "", "", false
	}
	parsed, err := url.Parse(normalizedURL)
	if err != nil || strings.TrimSpace(parsed.Hostname()) == "" {
		t.sendSimpleResult(t.input, normalizedURL, "invalid_format", "missing host")
		return "", "", false
	}
	host = strings.TrimSpace(parsed.Hostname())
	if urlguard.IsInternalHost(t.ctx, host) {
		t.sendHostResult(t.input, normalizedURL, host, "blocked", "target resolves to private/internal address (SSRF protection)")
		return "", "", false
	}
	return normalizedURL, host, true
}

// resolveHostIPs resolves IPv4 addresses for the host.
func (t *portScanTask) resolveHostIPs(normalizedURL, host string) ([]string, bool) {
	resolveCtx, resolveCancel := context.WithTimeout(t.ctx, 6*time.Second)
	defer resolveCancel()
	ips, resolveErr := resolveIPv4Addresses(resolveCtx, host)
	if resolveErr != nil {
		t.sendHostResult(t.input, normalizedURL, host, "resolve_failed", resolveErr.Error())
		return nil, false
	}
	return ips, true
}

// checkCDNExclusion checks if the target is behind a CDN.
func (t *portScanTask) checkCDNExclusion(normalizedURL, host string, ips []string) bool {
	cdnDetected, cdnReasons := detectCDNForTarget(t.ctx, normalizedURL, host, ips, t.proxyPool)
	if cdnDetected {
		t.resultChan <- portScanTaskPayload{index: t.index, item: URLPortScanResult{
			Input: t.input, URL: normalizedURL, Host: host,
			Status: "cdn_excluded", Reason: "cdn detected, port scan excluded",
			CDNDetected: true, CDNReasons: cdnReasons, ResolvedIPs: ips,
		}}
		return false
	}
	return true
}

// scanAndReport performs the port scan and sends the result.
func (t *portScanTask) scanAndReport(normalizedURL, host string, ips []string) {
	started := time.Now()
	scanCtx, scanCancel := context.WithTimeout(t.ctx, t.scanTimeout)
	defer scanCancel()
	openPorts, attempted, scanErr := scanHostPortsConcurrent(scanCtx, ips, t.ports, t.portConcurrency, t.connectTimeout, nil)
	expected := len(ips) * len(t.ports)
	durationMS := time.Since(started).Milliseconds()
	if scanErr != nil {
		t.resultChan <- portScanTaskPayload{index: t.index, item: URLPortScanResult{
			Input: t.input, URL: normalizedURL, Host: host,
			Status: "scan_failed", Reason: fmt.Sprintf("scan incomplete: %v (%d/%d connections attempted)", scanErr, attempted, expected),
			CDNDetected: false, ResolvedIPs: ips, OpenPorts: openPorts,
			AttemptedConnections: attempted, ExpectedConnections: expected, DurationMS: durationMS,
		}}
		return
	}
	scannedIPs := make([]string, 0, len(openPorts))
	for ip := range openPorts {
		scannedIPs = append(scannedIPs, ip)
	}
	sort.Strings(scannedIPs)
	t.resultChan <- portScanTaskPayload{index: t.index, item: URLPortScanResult{
		Input: t.input, URL: normalizedURL, Host: host,
		Status: "scanned", CDNDetected: false,
		ResolvedIPs: ips, ScannedIPs: scannedIPs, OpenPorts: openPorts,
		AttemptedConnections: attempted, ExpectedConnections: expected, DurationMS: durationMS,
	}}
}

// sendSimpleResult sends a result with only input, URL, status, and reason.
func (t *portScanTask) sendSimpleResult(input, normalizedURL, status, reason string) {
	t.resultChan <- portScanTaskPayload{index: t.index, item: URLPortScanResult{
		Input: input, URL: normalizedURL, Status: status, CDNDetected: false, Reason: reason,
	}}
}

// sendHostResult sends a result that includes the host field.
func (t *portScanTask) sendHostResult(input, normalizedURL, host, status, reason string) {
	t.resultChan <- portScanTaskPayload{index: t.index, item: URLPortScanResult{
		Input: input, URL: normalizedURL, Host: host, Status: status, CDNDetected: false, Reason: reason,
	}}
}

// ScanURLPorts executes URL->IP resolution, CDN exclusion and port scanning for non-CDN targets.
func (s *MonitorAppService) ScanURLPorts(ctx context.Context, urls []string, ports []int, concurrency int) (*URLPortScanResponse, error) {
	return s.ScanURLPortsWithOptions(ctx, urls, ports, PortScanOptions{TargetConcurrency: concurrency})
}

// ScanURLPortsWithOptions executes a bounded concurrent TCP port scan.
func (s *MonitorAppService) ScanURLPortsWithOptions(ctx context.Context, urls []string, ports []int, options PortScanOptions) (*URLPortScanResponse, error) {
	if len(urls) == 0 {
		return nil, fmt.Errorf("no URLs provided")
	}
	normalizedPorts := normalizeScanPorts(ports)
	options = normalizePortScanOptions(options, len(normalizedPorts))
	started := time.Now()
	results := make([]URLPortScanResult, len(urls))

	pool := workerpool.NewPool(options.TargetConcurrency)
	pool.Start()

	resultChan := make(chan portScanTaskPayload, len(urls))
	var wg sync.WaitGroup

	for i, rawURL := range urls {
		wg.Add(1)
		pool.Submit(&portScanTask{
			ctx:             ctx,
			index:           i,
			input:           rawURL,
			ports:           normalizedPorts,
			proxyPool:       s.proxyPool,
			portConcurrency: options.PortConcurrency,
			connectTimeout:  options.ConnectTimeout,
			scanTimeout:     options.ScanTimeout,
			resultChan:      resultChan,
			wg:              &wg,
		})
	}

	go func() {
		wg.Wait()
		pool.Stop()
		close(resultChan)
	}()

	for item := range resultChan {
		results[item.index] = item.item
	}

	summary := URLPortScanSummary{Total: len(results)}
	for _, result := range results {
		switch result.Status {
		case "invalid_format":
			summary.InvalidFormat++
		case "resolve_failed":
			summary.FormatValid++
			summary.ResolveFailed++
		case "cdn_excluded":
			summary.FormatValid++
			summary.CDNExcluded++
		case "scan_failed":
			summary.FormatValid++
			summary.ScanFailed++
		case "blocked":
			summary.FormatValid++
			summary.Blocked++
		case "scanned":
			summary.FormatValid++
			summary.Scanned++
		}
	}

	responsePorts := normalizedPorts
	if len(responsePorts) > 1024 {
		responsePorts = nil
	}
	return &URLPortScanResponse{
		Summary: summary, Ports: responsePorts, PortCount: len(normalizedPorts),
		DurationMS: time.Since(started).Milliseconds(), Results: results,
	}, nil
}

func normalizePortScanOptions(options PortScanOptions, portCount int) PortScanOptions {
	if options.TargetConcurrency <= 0 || options.TargetConcurrency > 10 {
		options.TargetConcurrency = 3
	}
	maxPerTarget := 1024 / options.TargetConcurrency
	if maxPerTarget < 32 {
		maxPerTarget = 32
	}
	if options.PortConcurrency <= 0 {
		options.PortConcurrency = 256
	}
	if options.PortConcurrency > maxPerTarget {
		options.PortConcurrency = maxPerTarget
	}
	if options.ConnectTimeout <= 0 || options.ConnectTimeout > 10*time.Second {
		options.ConnectTimeout = 800 * time.Millisecond
	}
	if options.ScanTimeout <= 0 {
		options.ScanTimeout = 60 * time.Second
		if portCount > 1024 {
			options.ScanTimeout = 5 * time.Minute
		}
	}
	if options.ScanTimeout > 15*time.Minute {
		options.ScanTimeout = 15 * time.Minute
	}
	return options
}

func normalizeScanPorts(ports []int) []int {
	if len(ports) == 0 {
		out := make([]int, len(defaultScanPorts))
		copy(out, defaultScanPorts)
		return out
	}

	seen := make(map[int]struct{}, len(ports))
	out := make([]int, 0, len(ports))
	for _, p := range ports {
		if p < 1 || p > 65535 {
			continue
		}
		if _, exists := seen[p]; exists {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}

	if len(out) == 0 {
		out = append(out, defaultScanPorts...)
	}
	sort.Ints(out)
	return out
}

func resolveIPv4Addresses(ctx context.Context, host string) ([]string, error) {
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no ipv4 address resolved")
	}
	out := make([]string, 0, len(ips))
	seen := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		ipText := strings.TrimSpace(ip.String())
		if ipText == "" {
			continue
		}
		if _, exists := seen[ipText]; exists {
			continue
		}
		seen[ipText] = struct{}{}
		out = append(out, ipText)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, fmt.Errorf("no ipv4 address resolved")
	}
	return out, nil
}

func detectCDNForTarget(ctx context.Context, targetURL, host string, ips []string, pool *proxypool.Pool) (bool, []string) {
	reasons := make([]string, 0, 4)

	if cname, err := net.LookupCNAME(host); err == nil {
		lower := strings.ToLower(strings.TrimSpace(cname))
		if lower != "" && isLikelyCDNString(lower) {
			reasons = append(reasons, "cname indicates cdn")
		}
	}

	if len(ips) >= 4 {
		reasons = append(reasons, "multiple edge ips resolved")
	}

	for _, ip := range ips {
		if isLikelyCDNIP(ip) {
			reasons = append(reasons, "ip in known cdn range")
			break
		}
	}

	if cdnByHeader := detectCDNByHTTPHeaders(ctx, targetURL, pool); cdnByHeader {
		reasons = append(reasons, "http headers indicate cdn")
	}

	return len(reasons) > 0, dedupeStrings(reasons)
}

func detectCDNByHTTPHeaders(ctx context.Context, targetURL string, pool *proxypool.Pool) bool {
	selectedProxy := ""
	if pool != nil {
		if proxyAddr, ok := pool.Select(); ok {
			selectedProxy = proxyAddr
		}
	}

	client, err := buildReachabilityHTTPClient(selectedProxy)
	if err != nil {
		if pool != nil && selectedProxy != "" {
			pool.Report(selectedProxy, false)
		}
		return false
	}

	success := false
	defer func() {
		if pool != nil && selectedProxy != "" {
			pool.Report(selectedProxy, success)
		}
	}()

	headCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, reqErr := http.NewRequestWithContext(headCtx, http.MethodHead, targetURL, nil)
	if reqErr != nil {
		return false
	}
	resp, doErr := client.Do(req)
	if doErr != nil {
		return false
	}
	defer resp.Body.Close()
	success = true

	for k, values := range resp.Header {
		lowerKey := strings.ToLower(strings.TrimSpace(k))
		if isLikelyCDNString(lowerKey) {
			return true
		}
		for _, v := range values {
			if isLikelyCDNString(strings.ToLower(strings.TrimSpace(v))) {
				return true
			}
		}
	}
	return isLikelyCDNString(strings.ToLower(strings.TrimSpace(resp.Header.Get("Server"))))
}

func scanHostPorts(ctx context.Context, ips []string, ports []int) (map[string][]int, error) {
	result, _, err := scanHostPortsConcurrent(ctx, ips, ports, 64, 1200*time.Millisecond, nil)
	return result, err
}

type tcpPortDialFunc func(context.Context, string, int, time.Duration) bool

type tcpPortJob struct {
	ip   string
	port int
}

// scanHostPortsConcurrent scans real TCP endpoints with a bounded worker pool.
// A nil dial function selects the production net.Dialer implementation.
func scanHostPortsConcurrent(ctx context.Context, ips []string, ports []int, concurrency int, timeout time.Duration, dial tcpPortDialFunc) (map[string][]int, int, error) {
	result := make(map[string][]int, len(ips))
	for _, ip := range ips {
		result[ip] = []int{}
	}
	if len(ips) == 0 || len(ports) == 0 {
		return result, 0, nil
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	total := len(ips) * len(ports)
	if concurrency > total {
		concurrency = total
	}
	if dial == nil {
		dial = isTCPPortOpen
	}

	jobs := make(chan tcpPortJob)
	openResults := make(chan tcpPortJob)
	var attempted atomic.Int64
	var workers sync.WaitGroup
	workers.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer workers.Done()
			for job := range jobs {
				if ctx.Err() != nil {
					return
				}
				attempted.Add(1)
				if dial(ctx, job.ip, job.port, timeout) {
					select {
					case openResults <- job:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, ip := range ips {
			for _, port := range ports {
				select {
				case jobs <- tcpPortJob{ip: ip, port: port}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	go func() {
		workers.Wait()
		close(openResults)
	}()

	for open := range openResults {
		result[open.ip] = append(result[open.ip], open.port)
	}
	for ip := range result {
		sort.Ints(result[ip])
	}
	return result, int(attempted.Load()), ctx.Err()
}

func isTCPPortOpen(ctx context.Context, ip string, port int, timeout time.Duration) bool {
	target := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, item := range in {
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		if _, exists := seen[v]; exists {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func isLikelyCDNString(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	needle := []string{
		"cdn", "cloudflare", "cloudfront", "akamai", "fastly", "edgekey", "edgesuite",
		"incapsula", "sucuri", "stackpath", "qcloud", "aliyuncs", "tencent", "wangsu",
		"chinacache", "cache", "cf-ray", "x-cache", "x-served-by", "via",
	}
	lower := strings.ToLower(text)
	for _, k := range needle {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}

func isLikelyCDNIP(ipText string) bool {
	ip := net.ParseIP(strings.TrimSpace(ipText))
	if ip == nil {
		return false
	}
	cidrs := []string{
		"104.16.0.0/12",  // Cloudflare
		"172.64.0.0/13",  // Cloudflare
		"23.235.32.0/20", // Fastly
		"151.101.0.0/16", // Fastly
		"13.32.0.0/15",   // CloudFront
		"13.224.0.0/14",  // CloudFront
		"23.0.0.0/12",    // Akamai common edge
	}
	for _, cidrText := range cidrs {
		_, cidr, err := net.ParseCIDR(cidrText)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}
