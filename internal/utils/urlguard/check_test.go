package urlguard

import (
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
