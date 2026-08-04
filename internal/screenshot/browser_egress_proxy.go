package screenshot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	xproxy "golang.org/x/net/proxy"
)

// browserEgressProxy is a loopback-only forward proxy. Unlike a Fetch-domain
// pre-check, its dialer validates and pins the actual IP used for every TCP
// connection, closing the DNS rebinding window between validation and dial.
type browserEgressProxy struct {
	listener net.Listener
	server   *http.Server
	once     sync.Once
	done     chan struct{}
}

type browserDialTargetResolver func(context.Context, string) ([]string, error)
type browserPinnedDialer func(context.Context, string, string) (net.Conn, error)

func newBrowserEgressProxy(parent context.Context, resolver browserHostResolver) (*browserEgressProxy, error) {
	return newBrowserEgressProxyWithUpstream(parent, resolver, "")
}

// newBrowserEgressProxyWithUpstream keeps DNS validation and address pinning
// inside UniMap, then optionally sends the already-pinned public IP through a
// loopback SOCKS5 proxy. The SOCKS server never receives the original host, so
// it cannot re-resolve the name to a private address.
func newBrowserEgressProxyWithUpstream(parent context.Context, resolver browserHostResolver, upstream string) (*browserEgressProxy, error) {
	dial, err := browserPinnedDialerForUpstream(upstream)
	if err != nil {
		return nil, err
	}
	return newBrowserEgressProxyWithDialResolver(parent, func(ctx context.Context, addr string) ([]string, error) {
		return resolveBrowserDialTargets(ctx, resolver, addr)
	}, dial)
}

func newBrowserEgressProxyWithDialResolver(parent context.Context, resolveTargets browserDialTargetResolver, dial browserPinnedDialer) (*browserEgressProxy, error) {
	if parent == nil {
		parent = context.Background()
	}
	if resolveTargets == nil {
		return nil, fmt.Errorf("ssrf: browser egress target resolver is unavailable")
	}
	if dial == nil {
		return nil, fmt.Errorf("ssrf: browser egress dialer is unavailable")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("ssrf: start browser egress proxy: %w", err)
	}

	proxy := &browserEgressProxy{listener: listener, done: make(chan struct{})}
	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     false,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialValidatedBrowserTarget(ctx, resolveTargets, dial, network, addr)
		},
	}
	reverseProxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.RequestURI = ""
			req.Header.Del("Proxy-Connection")
			if req.URL.Scheme == "" {
				req.URL.Scheme = "http"
			}
			if req.URL.Host == "" {
				req.URL.Host = req.Host
			}
		},
		Transport: transport,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "browser egress request denied", http.StatusBadGateway)
		},
	}
	proxy.server = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodConnect {
				proxy.handleConnect(w, req, resolveTargets, dial)
				return
			}
			reverseProxy.ServeHTTP(w, req)
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		_ = proxy.server.Serve(listener)
	}()
	go func() {
		select {
		case <-parent.Done():
			_ = proxy.Close()
		case <-proxy.done:
		}
	}()
	return proxy, nil
}

func (p *browserEgressProxy) URL() string {
	if p == nil || p.listener == nil {
		return ""
	}
	return "http://" + p.listener.Addr().String()
}

func (p *browserEgressProxy) Close() error {
	if p == nil {
		return nil
	}
	var closeErr error
	p.once.Do(func() {
		if p.done != nil {
			close(p.done)
		}
		if p.server != nil {
			closeErr = p.server.Close()
			if errors.Is(closeErr, http.ErrServerClosed) {
				closeErr = nil
			}
		} else if p.listener != nil {
			closeErr = p.listener.Close()
		}
	})
	return closeErr
}

func (p *browserEgressProxy) handleConnect(w http.ResponseWriter, req *http.Request, resolveTargets browserDialTargetResolver, dial browserPinnedDialer) {
	target := strings.TrimSpace(req.Host)
	if !strings.Contains(target, ":") {
		target = net.JoinHostPort(target, "443")
	}
	upstream, err := dialValidatedBrowserTarget(req.Context(), resolveTargets, dial, "tcp", target)
	if err != nil {
		http.Error(w, "browser egress connection denied", http.StatusForbidden)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		_ = upstream.Close()
		http.Error(w, "browser egress tunneling unavailable", http.StatusInternalServerError)
		return
	}
	client, _, err := hijacker.Hijack()
	if err != nil {
		_ = upstream.Close()
		return
	}
	defer client.Close()
	defer upstream.Close()
	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}

	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(upstream, client)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(client, upstream)
		done <- struct{}{}
	}()
	<-done
}

func dialValidatedBrowserTarget(ctx context.Context, resolveTargets browserDialTargetResolver, dial browserPinnedDialer, network, addr string) (net.Conn, error) {
	targets, err := resolveTargets(ctx, addr)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, target := range targets {
		conn, dialErr := dial(ctx, network, target)
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	return nil, fmt.Errorf("ssrf: dial validated browser target: %w", lastErr)
}

func browserPinnedDialerForUpstream(raw string) (browserPinnedDialer, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		direct := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		return direct.DialContext, nil
	}

	parsed, err := url.Parse(value)
	if err != nil || !strings.EqualFold(parsed.Scheme, "socks5") {
		return nil, fmt.Errorf("ssrf: guarded browser upstream must use socks5://")
	}
	host := strings.TrimSpace(parsed.Hostname())
	port := strings.TrimSpace(parsed.Port())
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() || port == "" {
		return nil, fmt.Errorf("ssrf: guarded browser SOCKS5 upstream must be a loopback IP and explicit port")
	}

	var auth *xproxy.Auth
	if parsed.User != nil {
		password, _ := parsed.User.Password()
		auth = &xproxy.Auth{User: parsed.User.Username(), Password: password}
	}
	forward := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	socksDialer, err := xproxy.SOCKS5("tcp", net.JoinHostPort(ip.String(), port), auth, forward)
	if err != nil {
		return nil, fmt.Errorf("ssrf: initialize guarded browser SOCKS5 upstream: %w", err)
	}
	contextDialer, ok := socksDialer.(xproxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("ssrf: guarded browser SOCKS5 dialer lacks context support")
	}
	return contextDialer.DialContext, nil
}

func resolveBrowserDialTargets(ctx context.Context, resolver browserHostResolver, addr string) ([]string, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		return nil, fmt.Errorf("ssrf: invalid browser egress target")
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" || port == "" {
		return nil, fmt.Errorf("ssrf: invalid browser egress target")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isRestrictedBrowserIP(ip) {
			return nil, fmt.Errorf("ssrf: browser egress target is restricted")
		}
		return []string{net.JoinHostPort(ip.String(), port)}, nil
	}
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("ssrf: resolve browser egress host: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("ssrf: browser egress host resolved to no addresses")
	}
	targets := make([]string, 0, len(ips))
	for _, addr := range ips {
		if isRestrictedBrowserIP(addr.IP) {
			return nil, fmt.Errorf("ssrf: browser egress host resolves to a restricted address")
		}
		targets = append(targets, net.JoinHostPort(addr.IP.String(), port))
	}
	return targets, nil
}
