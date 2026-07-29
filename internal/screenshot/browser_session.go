package screenshot

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/chromedp/chromedp"
)

// guardedBrowserSession owns one browser Target whose requests are protected
// by the browser URL policy. Callers only need Run and Close; allocator,
// interceptor, and cancellation ordering stay local to this module.
type guardedBrowserSession struct {
	ctx           context.Context
	interceptor   *SSRFInterceptor
	browserCancel context.CancelFunc
	allocCancel   context.CancelFunc
	egressProxy   *browserEgressProxy
}

func (m *Manager) newGuardedBrowserSession(parent context.Context, targetURL, proxy string) (*guardedBrowserSession, error) {
	if err := m.validateBrowserURL(parent, targetURL); err != nil {
		return nil, err
	}
	if m.configuredUpstreamProxy(proxy) != "" {
		return nil, fmt.Errorf("ssrf: external browser proxy is not supported by the guarded egress path")
	}
	if m.configuredRemoteDebugURL() != "" {
		return nil, fmt.Errorf("ssrf: remote Chrome cannot prove guarded egress configuration")
	}
	factory := m.egressProxyFactory
	if factory == nil {
		factory = func(ctx context.Context) (*browserEgressProxy, error) {
			return newBrowserEgressProxy(ctx, net.DefaultResolver)
		}
	}
	egressProxy, err := factory(parent)
	if err != nil {
		return nil, err
	}

	var (
		allocCtx    context.Context
		allocCancel context.CancelFunc
	)
	allocCtx, allocCancel, err = m.newAllocatorWithProxy(parent, egressProxy.URL())
	if err != nil {
		_ = egressProxy.Close()
		return nil, err
	}

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	interceptor := newSSRFInterceptor(m.urlValidator)
	guardedCtx, err := interceptor.Enable(browserCtx)
	if err != nil {
		browserCancel()
		allocCancel()
		_ = egressProxy.Close()
		return nil, err
	}

	return &guardedBrowserSession{
		ctx:           guardedCtx,
		interceptor:   interceptor,
		browserCancel: browserCancel,
		allocCancel:   allocCancel,
		egressProxy:   egressProxy,
	}, nil
}

func (m *Manager) configuredUpstreamProxy(override string) string {
	if value := strings.TrimSpace(override); value != "" {
		return value
	}
	if m != nil {
		if value := strings.TrimSpace(m.proxyServer); value != "" {
			return value
		}
	}
	return strings.TrimSpace(os.Getenv("UNIMAP_CHROME_PROXY_SERVER"))
}

func (m *Manager) configuredRemoteDebugURL() string {
	if m != nil {
		if value := strings.TrimSpace(m.remoteDebugURL); value != "" {
			return value
		}
	}
	return strings.TrimSpace(os.Getenv("UNIMAP_CHROME_REMOTE_DEBUG_URL"))
}

func (s *guardedBrowserSession) Context() context.Context {
	if s == nil {
		return nil
	}
	return s.ctx
}

func (s *guardedBrowserSession) Run(actions ...chromedp.Action) error {
	if s == nil || s.ctx == nil {
		return fmt.Errorf("ssrf: browser session is not initialized")
	}
	runErr := chromedp.Run(s.ctx, actions...)
	if guardErr := s.interceptor.Err(); guardErr != nil {
		return guardErr
	}
	return runErr
}

func (s *guardedBrowserSession) Close() {
	if s == nil {
		return
	}
	if s.interceptor != nil {
		s.interceptor.Cancel()
	}
	if s.browserCancel != nil {
		s.browserCancel()
	}
	if s.allocCancel != nil {
		s.allocCancel()
	}
	if s.egressProxy != nil {
		_ = s.egressProxy.Close()
	}
}
