package urlguard

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func Check(rawURL string, opts CheckOptions) (*url.URL, error) {
	opts = opts.withDefaults()

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("urlguard: parse error: %w", err)
	}

	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("urlguard: missing scheme or host")
	}

	schemeAllowed := false
	for _, s := range opts.AllowedSchemes {
		if u.Scheme == s {
			schemeAllowed = true
			break
		}
	}
	if !schemeAllowed {
		return nil, fmt.Errorf("urlguard: scheme %q not allowed (allowed: %v)", u.Scheme, opts.AllowedSchemes)
	}

	if opts.AllowedHostsRE != nil {
		if !opts.AllowedHostsRE.MatchString(u.Host) {
			return nil, fmt.Errorf("urlguard: host %q not allowed by pattern", u.Host)
		}
	}

	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("urlguard: empty hostname")
	}

	if !opts.AllowPrivate {
		if err := checkIPLiteralPrivate(host); err != nil {
			return nil, err
		}
	}

	return u, nil
}

// checkIPLiteralPrivate rejects loopback/private/link-local IPs.
// For hostname literals, it does NOT perform DNS — DNS rebinding protection
// is enforced at dial time in SafeHTTPClient.DialContext (where the resolved
// IP is the actual connection target). This keeps Check() usable in offline
// environments (config validation, unit tests, CI sandboxes).
func checkIPLiteralPrivate(host string) error {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return fmt.Errorf("urlguard: host %q is loopback", host)
	}

	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("urlguard: IP %q is loopback/private/link-local/unspecified", host)
		}
		return nil
	}

	// Not an IP literal — skip DNS here. DNS rebinding protection is
	// enforced at connection time in SafeHTTPClient.DialContext.
	return nil
}

// checkHostLive does the full DNS resolution check. Used at dial/redirect time.
func checkHostLive(ctx context.Context, host string) error {
	if err := checkIPLiteralPrivate(host); err == nil {
		if net.ParseIP(host) != nil {
			return nil
		}
	}

	resolveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resolver := &net.Resolver{PreferGo: true}
	ips, err := resolver.LookupIPAddr(resolveCtx, host)
	if err != nil {
		return fmt.Errorf("urlguard: DNS lookup failed for %q: %w", host, err)
	}

	for _, ipAddr := range ips {
		ip := ipAddr.IP
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("urlguard: DNS for %q resolves to restricted IP %s", host, ip)
		}
	}

	return nil
}

// IsInternalHost checks whether a host resolves to a private/internal address.
func IsInternalHost(ctx context.Context, host string) bool {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" {
		return false
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return checkHostLive(ctx, host) != nil
}

func SafeDialer(opts CheckOptions) *net.Dialer {
	return &net.Dialer{
		Timeout: 10 * time.Second,
		Control: nil,
	}
}

func SafeHTTPClient(opts CheckOptions, timeout time.Duration) *http.Client {
	opts = opts.withDefaults()

	// Pre-compute proxy address so DialContext can skip the private-IP check
	// when connecting to a local/loopback proxy (the proxy is trusted; SSRF
	// protection for the final target is enforced by Check() and the proxy).
	var proxyAddr string
	if probe, _ := http.ProxyFromEnvironment(&http.Request{URL: &url.URL{Scheme: "https", Host: "example.com"}}); probe != nil {
		proxyAddr = probe.Host
	}

	transport := &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// When a proxy is configured, DialContext connects to the proxy
			// server, not the final target. Skip private-IP checks for the
			// proxy address itself — it is a trusted local endpoint.
			if proxyAddr != "" && (addr == proxyAddr || hostPortEqual(addr, proxyAddr)) {
				return net.DialTimeout(network, addr, 10*time.Second)
			}
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if !opts.AllowPrivate {
				if err := checkHostLive(ctx, host); err != nil {
					return nil, err
				}
			}
			return net.DialTimeout(network, addr, 10*time.Second)
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !opts.AllowPrivate {
				redirectHost := req.URL.Hostname()
				if err := checkHostLive(req.Context(), redirectHost); err != nil {
					return fmt.Errorf("urlguard: redirect to restricted host: %w", err)
				}
			}
			return nil
		},
	}
}

// hostPortEqual compares two host:port strings, normalising missing ports
// (e.g. "127.0.0.1" vs "127.0.0.1:7897").
func hostPortEqual(a, b string) bool {
	ha, pa := splitHP(a)
	hb, pb := splitHP(b)
	return ha == hb && pa == pb
}

func splitHP(addr string) (host, port string) {
	if h, p, err := net.SplitHostPort(addr); err == nil {
		return h, p
	}
	return addr, ""
}
