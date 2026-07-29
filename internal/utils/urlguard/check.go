package urlguard

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var restrictedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

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
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("urlguard: host %q is loopback", host)
	}

	ip := net.ParseIP(host)
	if ip != nil {
		if isRestrictedIP(ip) {
			return fmt.Errorf("urlguard: IP %q is restricted", host)
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
		if isRestrictedIP(ip) {
			return fmt.Errorf("urlguard: DNS for %q resolves to restricted IP %s", host, ip)
		}
	}

	return nil
}

func isRestrictedIP(ip net.IP) bool {
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
	for _, prefix := range restrictedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
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
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	}
	if opts.AllowPrivate {
		transport.Proxy = http.ProxyFromEnvironment
		transport.DialContext = dialer.DialContext
	} else {
		// Public-only clients cannot trust an ambient proxy because it would
		// resolve and dial the final target outside this process's policy.
		transport.Proxy = nil
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return safeDialContext(ctx, net.DefaultResolver, dialer.DialContext, network, addr)
		}
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

type safeHostResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type safeDialFunc func(context.Context, string, string) (net.Conn, error)

func safeDialContext(ctx context.Context, resolver safeHostResolver, dial safeDialFunc, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil || strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
		return nil, fmt.Errorf("urlguard: invalid dial target")
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")

	var ips []net.IPAddr
	if literal := net.ParseIP(host); literal != nil {
		ips = []net.IPAddr{{IP: literal}}
	} else {
		ips, err = resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("urlguard: DNS lookup failed for %q: %w", host, err)
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("urlguard: DNS for %q returned no addresses", host)
	}

	targets := make([]string, 0, len(ips))
	for _, ipAddr := range ips {
		if isRestrictedIP(ipAddr.IP) {
			return nil, fmt.Errorf("urlguard: DNS for %q resolves to restricted IP %s", host, ipAddr.IP)
		}
		targets = append(targets, net.JoinHostPort(ipAddr.IP.String(), port))
	}

	var lastErr error
	for _, target := range targets {
		conn, dialErr := dial(ctx, network, target)
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	return nil, fmt.Errorf("urlguard: dial validated target: %w", lastErr)
}
