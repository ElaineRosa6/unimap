package urlguard

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"
)

func TestCheck_ValidHTTPS(t *testing.T) {
	u, err := Check("https://example.com/api/v1", CheckOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Scheme != "https" {
		t.Errorf("expected scheme https, got %s", u.Scheme)
	}
	if u.Host != "example.com" {
		t.Errorf("expected host example.com, got %s", u.Host)
	}
}

func TestCheck_ValidHTTP(t *testing.T) {
	u, err := Check("http://example.com", CheckOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Scheme != "http" {
		t.Errorf("expected scheme http, got %s", u.Scheme)
	}
}

func TestCheck_RejectInvalidScheme(t *testing.T) {
	_, err := Check("ftp://example.com/file", CheckOptions{})
	if err == nil {
		t.Fatal("expected error for ftp scheme")
	}
}

func TestCheck_RejectLoopback(t *testing.T) {
	loopbacks := []string{
		"http://127.0.0.1/api",
		"http://localhost/api",
		"http://::1/api",
		"http://0.0.0.0/api",
	}
	for _, rawURL := range loopbacks {
		_, err := Check(rawURL, CheckOptions{})
		if err == nil {
			t.Errorf("expected error for loopback URL %s", rawURL)
		}
	}
}

func TestCheck_RejectPrivate(t *testing.T) {
	privates := []string{
		"http://10.0.0.1/api",
		"http://192.168.1.1/api",
		"http://172.16.0.1/api",
		"http://172.31.255.255/api",
	}
	for _, rawURL := range privates {
		_, err := Check(rawURL, CheckOptions{})
		if err == nil {
			t.Errorf("expected error for private URL %s", rawURL)
		}
	}
}

func TestCheck_RejectLinkLocal(t *testing.T) {
	_, err := Check("http://169.254.169.254/latest/meta-data/", CheckOptions{})
	if err == nil {
		t.Fatal("expected error for link-local URL")
	}
}

func TestCheck_AllowPrivateOpt(t *testing.T) {
	u, err := Check("http://10.0.0.1/api", CheckOptions{AllowPrivate: true})
	if err != nil {
		t.Fatalf("expected no error with AllowPrivate=true, got: %v", err)
	}
	if u.Host != "10.0.0.1" {
		t.Errorf("expected host 10.0.0.1, got %s", u.Host)
	}
}

func TestCheck_AllowedHostsRegex(t *testing.T) {
	re := regexp.MustCompile(`.*\.dingtalk\.com$`)
	_, err := Check("https://oapi.dingtalk.com/robot/send", CheckOptions{AllowedHostsRE: re})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = Check("https://evil.com/robot/send", CheckOptions{AllowedHostsRE: re})
	if err == nil {
		t.Fatal("expected error for non-matching host")
	}
}

func TestCheck_MissingHost(t *testing.T) {
	_, err := Check("https:///path", CheckOptions{})
	if err == nil {
		t.Fatal("expected error for missing host")
	}
}

func TestCheck_ParseError(t *testing.T) {
	_, err := Check("://invalid", CheckOptions{})
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestSafeHTTPClient_Basic(t *testing.T) {
	client := SafeHTTPClient(CheckOptions{}, 10*time.Second)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", client.Timeout)
	}
}

func TestSafeHTTPClient_RejectRedirectToPrivate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.1/secret", http.StatusFound)
	}))
	defer server.Close()

	client := SafeHTTPClient(CheckOptions{}, 5*time.Second)
	resp, err := client.Get(server.URL)
	if err == nil {
		if resp != nil {
			resp.Body.Close()
		}
		t.Fatal("expected error for redirect to private IP")
	}
}

func TestCheckOptions_withDefaults(t *testing.T) {
	opts := CheckOptions{}.withDefaults()
	if len(opts.AllowedSchemes) != 2 {
		t.Errorf("expected 2 default schemes, got %d", len(opts.AllowedSchemes))
	}
	if opts.AllowedSchemes[0] != "http" || opts.AllowedSchemes[1] != "https" {
		t.Errorf("unexpected default schemes: %v", opts.AllowedSchemes)
	}
}

func TestCheckOptions_withDefaults_PreservesExisting(t *testing.T) {
	opts := CheckOptions{AllowedSchemes: []string{"https"}}.withDefaults()
	if len(opts.AllowedSchemes) != 1 || opts.AllowedSchemes[0] != "https" {
		t.Errorf("expected existing schemes to be preserved, got %v", opts.AllowedSchemes)
	}
}

func TestSafeDialer_Creation(t *testing.T) {
	d := SafeDialer(CheckOptions{})
	if d == nil {
		t.Fatal("expected non-nil dialer")
	}
	if d.Timeout != 10*time.Second {
		t.Errorf("expected 10s timeout, got %v", d.Timeout)
	}
}

func TestSafeHTTPClient_AllowPrivateRedirect(t *testing.T) {
	// With AllowPrivate=true, redirect to private IP should be allowed
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.1/secret", http.StatusFound)
	}))
	defer server.Close()

	client := SafeHTTPClient(CheckOptions{AllowPrivate: true}, 5*time.Second)
	// Should not error on redirect to private IP when AllowPrivate is true
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error with AllowPrivate=true: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}
}

func TestSafeHTTPClient_TLSMinVersion(t *testing.T) {
	client := SafeHTTPClient(CheckOptions{}, 5*time.Second)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if transport.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("expected TLS 1.2 minimum, got %v", transport.TLSClientConfig.MinVersion)
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := defaultOptions()
	if opts.AllowPrivate {
		t.Error("expected AllowPrivate=false")
	}
	if len(opts.AllowedSchemes) != 2 {
		t.Fatalf("expected 2 schemes, got %d", len(opts.AllowedSchemes))
	}
}

func TestCheck_ValidHTTP_IPHost(t *testing.T) {
	// Public IP (not private) should pass Check
	u, err := Check("https://8.8.8.8/dns-query", CheckOptions{})
	if err != nil {
		t.Fatalf("unexpected error for public IP: %v", err)
	}
	if u.Host != "8.8.8.8" {
		t.Errorf("expected host 8.8.8.8, got %s", u.Host)
	}
}

func TestCheck_RejectPrivateIPHost(t *testing.T) {
	_, err := Check("https://192.168.1.1/admin", CheckOptions{})
	if err == nil {
		t.Fatal("expected error for private IP")
	}
}

func TestCheck_OptionsOverrides(t *testing.T) {
	// Custom AllowedSchemes should override defaults
	_, err := Check("ftp://example.com/file", CheckOptions{AllowedSchemes: []string{"ftp"}})
	if err != nil {
		t.Fatalf("unexpected error with ftp in AllowedSchemes: %v", err)
	}
}

func TestCheck_EmptyURL(t *testing.T) {
	_, err := Check("", CheckOptions{})
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestSafeHTTPClient_CheckRedirect_AllowPrivate(t *testing.T) {
	// With AllowPrivate=true, redirect to loopback should succeed
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.1/target", http.StatusFound)
	}))
	defer server.Close()

	client := SafeHTTPClient(CheckOptions{AllowPrivate: true}, 5*time.Second)
	// Redirect should not be blocked
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error with AllowPrivate redirect: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}
}

func TestSafeHTTPClient_WithCustomSchemes(t *testing.T) {
	client := SafeHTTPClient(CheckOptions{AllowedSchemes: []string{"https"}}, 5*time.Second)
	if client.Timeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", client.Timeout)
	}
}

func TestCheck_IPv6Public(t *testing.T) {
	// A public IPv6 address should pass (not loopback/private)
	u, err := Check("https://[2606:4700:4700::1111]/dns", CheckOptions{})
	if err != nil {
		t.Fatalf("unexpected error for public IPv6: %v", err)
	}
	if u.Host != "[2606:4700:4700::1111]" {
		t.Errorf("expected host [2606:4700:4700::1111], got %s", u.Host)
	}
}

func TestCheck_RejectIPv6Loopback(t *testing.T) {
	_, err := Check("https://[::1]/api", CheckOptions{})
	if err == nil {
		t.Fatal("expected error for IPv6 loopback")
	}
}

func TestCheck_RejectUnspecifiedIP(t *testing.T) {
	_, err := Check("https://0.0.0.0/admin", CheckOptions{})
	if err == nil {
		t.Fatal("expected error for unspecified IP")
	}
}
