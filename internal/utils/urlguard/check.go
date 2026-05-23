package urlguard

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
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
		if err := checkHostPrivate(host); err != nil {
			return nil, err
		}
	}

	return u, nil
}

func checkHostPrivate(host string) error {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resolver := &net.Resolver{PreferGo: true}
	ips, err := resolver.LookupIPAddr(ctx, host)
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

func SafeDialer(opts CheckOptions) *net.Dialer {
	return &net.Dialer{
		Timeout: 10 * time.Second,
		Control: nil,
	}
}

func SafeHTTPClient(opts CheckOptions, timeout time.Duration) *http.Client {
	opts = opts.withDefaults()

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if !opts.AllowPrivate {
				if err := checkHostPrivate(host); err != nil {
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
				if err := checkHostPrivate(redirectHost); err != nil {
					return fmt.Errorf("urlguard: redirect to restricted host: %w", err)
				}
			}
			return nil
		},
	}
}
