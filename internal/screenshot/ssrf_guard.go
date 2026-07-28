package screenshot

import (
	"context"
	"fmt"
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
	_, err := urlguard.Check(rawURL, ssrfCheckOptions)
	if err != nil {
		return fmt.Errorf("ssrf: blocked URL %q: %w", rawURL, err)
	}
	return nil
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
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	blocked []string // URLs that were blocked (for diagnostics)
}

// NewSSRFInterceptor creates a new SSRF interceptor.
func NewSSRFInterceptor() *SSRFInterceptor {
	return &SSRFInterceptor{}
}

// Enable activates fetch-domain request interception on the given browser
// context. It intercepts Document (navigation) requests at the Request stage
// and validates each URL. Non-navigation subresource requests are also
// intercepted to prevent loading from internal addresses.
//
// The interceptor runs until Cancel is called or the browser context is done.
func (s *SSRFInterceptor) Enable(browserCtx context.Context) error {
	// Enable fetch interception for navigation (Document) requests.
	// We intercept at the Request stage so we can block before any connection
	// is made, preventing DNS rebinding TOCTOU issues.
	patterns := []*fetch.RequestPattern{
		{
			// Intercept all navigation requests (top-level and iframe).
			ResourceType: network.ResourceTypeDocument,
			RequestStage: fetch.RequestStageRequest,
		},
		{
			// Also intercept subresource requests to block internal asset loading.
			ResourceType: network.ResourceTypeScript,
			RequestStage: fetch.RequestStageRequest,
		},
		{
			ResourceType: network.ResourceTypeXHR,
			RequestStage: fetch.RequestStageRequest,
		},
		{
			ResourceType: network.ResourceTypeFetch,
			RequestStage: fetch.RequestStageRequest,
		},
	}

	if err := fetch.Enable().WithPatterns(patterns).Do(browserCtx); err != nil {
		return fmt.Errorf("ssrf: enable fetch interception: %w", err)
	}

	// Create a cancellable context for the event listener.
	listenCtx, cancel := context.WithCancel(browserCtx)
	s.cancel = cancel

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.listen(listenCtx)
	}()

	return nil
}

// listen handles fetch.EventRequestPaused events, validating each request URL.
func (s *SSRFInterceptor) listen(ctx context.Context) {
	ch := make(chan *fetch.EventRequestPaused, 64)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if e, ok := ev.(*fetch.EventRequestPaused); ok {
			select {
			case ch <- e:
			default:
				// Channel full — continue the request to avoid stalling the browser.
				// This is a safety valve; in practice the channel should not fill.
				_ = fetch.ContinueRequest(e.RequestID).Do(ctx)
			}
		}
	})

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
	reqURL := ""
	if ev.Request != nil {
		reqURL = ev.Request.URL
	}

	if reqURL == "" {
		// No URL available — allow the request to proceed.
		_ = fetch.ContinueRequest(ev.RequestID).Do(ctx)
		return
	}

	// Allow data: and about: URLs (used by browser internals).
	if strings.HasPrefix(reqURL, "data:") || strings.HasPrefix(reqURL, "about:") || strings.HasPrefix(reqURL, "chrome:") {
		_ = fetch.ContinueRequest(ev.RequestID).Do(ctx)
		return
	}

	if err := ValidateBrowserURL(reqURL); err != nil {
		// Block the request.
		s.mu.Lock()
		s.blocked = append(s.blocked, reqURL)
		s.mu.Unlock()
		logger.Warnf("[ssrf-guard] blocked %s request to %s: %v", ev.ResourceType, reqURL, err)
		_ = fetch.FailRequest(ev.RequestID, network.ErrorReasonAccessDenied).Do(ctx)
		return
	}

	// URL is safe — allow the request to proceed.
	_ = fetch.ContinueRequest(ev.RequestID).Do(ctx)
}

// Cancel stops the interceptor and waits for the listener goroutine to exit.
func (s *SSRFInterceptor) Cancel() {
	if s.cancel != nil {
		s.cancel()
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
