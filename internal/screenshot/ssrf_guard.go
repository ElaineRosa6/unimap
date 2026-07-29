package screenshot

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"sync"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/utils/urlguard"
)

// ssrfCheckOptions are the urlguard options used for browser URL validation.
// Private, loopback, link-local and unspecified addresses are rejected.
var ssrfCheckOptions = urlguard.CheckOptions{
	AllowPrivate:   false,
	AllowedSchemes: []string{"http", "https"},
}

var restrictedBrowserPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),  // IETF protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),  // documentation
	netip.MustParsePrefix("198.18.0.0/15"), // benchmarking
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"), // reserved
	netip.MustParsePrefix("2001:db8::/32"),
}

type browserHostResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type browserURLValidator func(context.Context, string) error

// ValidateBrowserURL performs a static SSRF check on a URL before it is used
// for browser navigation (CDP or Extension). It rejects private, loopback,
// link-local and unspecified IP addresses, and non-HTTP(S) schemes.
//
// This is a pre-navigation gate. For full per-hop protection against redirects
// and DNS rebinding in CDP mode, use EnableFetchInterception as well.
func ValidateBrowserURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fmt.Errorf("ssrf: empty URL")
	}
	checkedURL, err := urlguard.Check(rawURL, ssrfCheckOptions)
	if err != nil {
		return fmt.Errorf("ssrf: blocked URL %q: %w", browserURLLabel(rawURL), err)
	}
	host := strings.TrimSuffix(strings.ToLower(checkedURL.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("ssrf: blocked URL %q: localhost hostname is not allowed", browserURLLabel(rawURL))
	}
	if ip := net.ParseIP(host); ip != nil && isRestrictedBrowserIP(ip) {
		return fmt.Errorf("ssrf: blocked URL %q: restricted IP %s", browserURLLabel(rawURL), ip)
	}
	return nil
}

// ValidateBrowserURLLive adds a live DNS check to the static browser URL
// validation. Every resolved address must be publicly routable.
func ValidateBrowserURLLive(ctx context.Context, rawURL string) error {
	return validateBrowserURLWithResolver(ctx, rawURL, net.DefaultResolver)
}

func validateBrowserURLWithResolver(ctx context.Context, rawURL string, resolver browserHostResolver) error {
	if err := ValidateBrowserURL(rawURL); err != nil {
		return err
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("ssrf: parse URL %q for DNS validation: %w", browserURLLabel(rawURL), err)
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if ip := net.ParseIP(host); ip != nil {
		if isRestrictedBrowserIP(ip) {
			return fmt.Errorf("ssrf: blocked URL %q: restricted IP %s", browserURLLabel(rawURL), ip)
		}
		return nil
	}
	if resolver == nil {
		return fmt.Errorf("ssrf: DNS resolver is unavailable")
	}
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("ssrf: resolve host %q: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("ssrf: host %q resolved to no addresses", host)
	}
	for _, addr := range ips {
		if isRestrictedBrowserIP(addr.IP) {
			return fmt.Errorf("ssrf: host %q resolves to restricted IP %s", host, addr.IP)
		}
	}
	return nil
}

func isRestrictedBrowserIP(ip net.IP) bool {
	if ip == nil ||
		ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		!ip.IsGlobalUnicast() {
		return true
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()
	for _, prefix := range restrictedBrowserPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func browserURLLabel(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "<invalid>"
	}
	return (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String()
}

// SSRFInterceptor provides per-hop SSRF protection for a CDP browser context
// by intercepting all navigation and subresource requests via the Fetch domain.
// Each request URL is validated against urlguard before being allowed to proceed.
// Blocked requests are failed with AccessDenied.
//
// Usage:
//
//	interceptor := NewSSRFInterceptor()
//	if err := interceptor.Enable(browserCtx); err != nil { ... }
//	defer interceptor.Cancel()
//	// ... proceed with chromedp.Run navigation ...
type SSRFInterceptor struct {
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	mu            sync.Mutex
	blocked       []string // URLs that were blocked (for diagnostics)
	fatalErr      error
	validator     browserURLValidator
	queueCapacity int
	pausedCh      chan *fetch.EventRequestPaused
	continueFn    func(context.Context, fetch.RequestID) error
	failFn        func(context.Context, fetch.RequestID) error
}

// NewSSRFInterceptor creates a new SSRF interceptor.
func NewSSRFInterceptor() *SSRFInterceptor {
	return newSSRFInterceptor(ValidateBrowserURLLive)
}

func newSSRFInterceptor(validator browserURLValidator) *SSRFInterceptor {
	if validator == nil {
		validator = ValidateBrowserURLLive
	}
	return &SSRFInterceptor{
		validator:     validator,
		queueCapacity: 256,
	}
}

// Enable activates fetch-domain request interception and returns the child
// browser context that callers must use for all subsequent actions. Every
// network request is paused at the Request stage and validated before Chrome
// can continue it.
//
// The interceptor runs until Cancel is called or the browser context is done.
func (s *SSRFInterceptor) Enable(browserCtx context.Context) (context.Context, error) {
	if browserCtx == nil {
		return nil, fmt.Errorf("ssrf: nil browser context")
	}
	// The first Run allocates Chrome and attaches a Target. No business
	// navigation has happened yet (the target is about:blank), but subsequent
	// CDP domain commands now have a valid Target executor.
	if err := chromedp.Run(browserCtx); err != nil {
		return nil, fmt.Errorf("ssrf: initialize browser target: %w", err)
	}

	guardCtx, cancel := context.WithCancel(browserCtx)
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	patterns := []*fetch.RequestPattern{
		{
			URLPattern:   "*",
			RequestStage: fetch.RequestStageRequest,
		},
	}

	queueCapacity := s.queueCapacity
	if queueCapacity <= 0 {
		queueCapacity = 1
	}
	ch := make(chan *fetch.EventRequestPaused, queueCapacity)
	s.mu.Lock()
	s.pausedCh = ch
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.listen(guardCtx, ch)
	}()

	chromedp.ListenTarget(guardCtx, func(ev interface{}) {
		e, ok := ev.(*fetch.EventRequestPaused)
		if !ok {
			return
		}
		s.enqueuePaused(e)
	})

	if err := chromedp.Run(guardCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		return fetch.Enable().WithPatterns(patterns).Do(ctx)
	})); err != nil {
		s.failClosed(fmt.Errorf("ssrf: enable fetch interception: %w", err))
		s.wg.Wait()
		return nil, fmt.Errorf("ssrf: enable fetch interception: %w", err)
	}

	return guardCtx, nil
}

func (s *SSRFInterceptor) enqueuePaused(ev *fetch.EventRequestPaused) {
	s.mu.Lock()
	ch := s.pausedCh
	s.mu.Unlock()
	if ch == nil {
		s.failClosed(fmt.Errorf("ssrf: request interception queue is unavailable"))
		return
	}
	select {
	case ch <- ev:
	default:
		s.failClosed(fmt.Errorf("ssrf: request interception queue overflow"))
	}
}

// listen handles fetch.EventRequestPaused events, validating each request URL.
func (s *SSRFInterceptor) listen(ctx context.Context, ch <-chan *fetch.EventRequestPaused) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-ch:
			s.handlePaused(ctx, ev)
		}
	}
}

// handlePaused validates a paused request and either continues or blocks it.
func (s *SSRFInterceptor) handlePaused(ctx context.Context, ev *fetch.EventRequestPaused) {
	if ev == nil {
		s.failClosed(fmt.Errorf("ssrf: nil paused request event"))
		return
	}
	reqURL := ""
	if ev.Request != nil {
		reqURL = ev.Request.URL
	}

	if reqURL == "" {
		s.blockRequest(ctx, ev, fmt.Errorf("ssrf: paused request has no URL"))
		return
	}

	// Allow data: and about: URLs (used by browser internals).
	if strings.HasPrefix(reqURL, "data:") || strings.HasPrefix(reqURL, "about:") || strings.HasPrefix(reqURL, "chrome:") {
		if err := s.continueRequest(ctx, ev.RequestID); err != nil {
			s.failClosed(fmt.Errorf("ssrf: continue browser-internal request: %w", err))
		}
		return
	}

	if err := s.validator(ctx, reqURL); err != nil {
		s.blockRequest(ctx, ev, err)
		return
	}

	if err := s.continueRequest(ctx, ev.RequestID); err != nil {
		s.failClosed(fmt.Errorf("ssrf: continue validated request: %w", err))
	}
}

func (s *SSRFInterceptor) continueRequest(ctx context.Context, requestID fetch.RequestID) error {
	if s.continueFn != nil {
		return s.continueFn(ctx, requestID)
	}
	return chromedp.Run(ctx, chromedp.ActionFunc(func(execCtx context.Context) error {
		return fetch.ContinueRequest(requestID).Do(execCtx)
	}))
}

func (s *SSRFInterceptor) blockRequest(ctx context.Context, ev *fetch.EventRequestPaused, reason error) {
	reqURL := ""
	if ev.Request != nil {
		reqURL = ev.Request.URL
	}
	s.mu.Lock()
	s.blocked = append(s.blocked, reqURL)
	s.mu.Unlock()
	logger.Warnf("[ssrf-guard] blocked %s request to %s: %v", ev.ResourceType, browserURLLabel(reqURL), reason)
	failRequest := s.failFn
	if failRequest == nil {
		failRequest = func(runCtx context.Context, requestID fetch.RequestID) error {
			return chromedp.Run(runCtx, chromedp.ActionFunc(func(execCtx context.Context) error {
				return fetch.FailRequest(requestID, network.ErrorReasonAccessDenied).Do(execCtx)
			}))
		}
	}
	if err := failRequest(ctx, ev.RequestID); err != nil && ctx.Err() == nil {
		s.failClosed(fmt.Errorf("ssrf: fail blocked request: %w", err))
		return
	}
	s.recordFatal(fmt.Errorf("ssrf: blocked browser request: %w", reason))
}

func (s *SSRFInterceptor) recordFatal(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fatalErr == nil {
		s.fatalErr = err
	}
}

func (s *SSRFInterceptor) failClosed(err error) {
	s.recordFatal(err)
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Cancel stops the interceptor and waits for the listener goroutine to exit.
func (s *SSRFInterceptor) Cancel() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		s.wg.Wait()
	}
}

// BlockedURLs returns a copy of the URLs that were blocked during interception.
func (s *SSRFInterceptor) BlockedURLs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.blocked))
	copy(out, s.blocked)
	return out
}

// Err reports a fatal interceptor failure, such as queue overflow or a CDP
// command failure. Callers should treat any non-nil value as task failure.
func (s *SSRFInterceptor) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fatalErr
}
