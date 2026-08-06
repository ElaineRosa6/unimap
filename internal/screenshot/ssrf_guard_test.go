package screenshot

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
)

type stubBrowserResolver struct {
	ips []net.IPAddr
	err error
}

func (r stubBrowserResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return r.ips, r.err
}

func TestValidateBrowserURL_ValidPublic(t *testing.T) {
	valid := []string{
		"https://fofa.info/result?qbase64=dGVzdA==",
		"https://en.hunter.how/search?search-value=test",
		"https://www.zoomeye.org/searchResult?q=test",
		"https://quake.360.net/quake/#/searchResult?searchVal=test",
		"https://www.shodan.io/search?query=test",
		"http://93.184.216.34/",
		"https://example.com:8443/path",
	}
	for _, u := range valid {
		if err := ValidateBrowserURL(u); err != nil {
			t.Errorf("ValidateBrowserURL(%q) = %v, want nil", u, err)
		}
	}
}

func TestValidateBrowserURL_BlocksPrivate(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1/",
		"http://127.0.0.1:8080/admin",
		"https://localhost/",
		"http://10.0.0.1/",
		"http://172.16.0.1/",
		"http://192.168.1.1/",
		"http://169.254.169.254/latest/meta-data/",
		"http://100.64.0.1/",
		"http://198.18.0.1/",
		"http://239.255.255.250/",
		"http://[::1]/",
		"http://0.0.0.0/",
		"ftp://example.com/",
		"file:///etc/passwd",
		"",
		"   ",
		"http://LOCALHOST/",
		"http://localhost./",
		"http://service.localhost/",
	}
	for _, u := range blocked {
		if err := ValidateBrowserURL(u); err == nil {
			t.Errorf("ValidateBrowserURL(%q) = nil, want error", u)
		}
	}
}

func TestValidateBrowserURL_BlocksCloudMetadata(t *testing.T) {
	// Cloud metadata endpoints must be blocked.
	metadata := []string{
		"http://169.254.169.254/",           // AWS/GCP/Azure
		"http://169.254.169.254:80/latest/", // with port
		"http://metadata.google.internal/",  // GCP DNS name resolves to 169.254.169.254
	}
	for _, u := range metadata[:2] {
		if err := ValidateBrowserURL(u); err == nil {
			t.Errorf("ValidateBrowserURL(%q) = nil, want error (cloud metadata)", u)
		}
	}
	// metadata.google.internal is a hostname — static check cannot block it
	// (requires DNS resolution). This is a known limitation documented in AGENTS.md.
	_ = metadata[2]
}

func TestValidateBrowserURL_ErrorRedactsQuery(t *testing.T) {
	err := ValidateBrowserURL("http://127.0.0.1/admin?token=do-not-log")
	if err == nil {
		t.Fatal("loopback URL must be rejected")
	}
	if strings.Contains(err.Error(), "do-not-log") {
		t.Fatalf("validation error leaked URL query: %v", err)
	}
}

func TestValidateBrowserURLWithResolver_BlocksResolvedPrivateAddress(t *testing.T) {
	resolver := stubBrowserResolver{ips: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}}
	if err := validateBrowserURLWithResolver(context.Background(), "https://public-name.example/", resolver); err == nil {
		t.Fatal("hostname resolving to loopback must be rejected")
	}
}

func TestValidateBrowserURLWithResolver_AllowsResolvedPublicAddress(t *testing.T) {
	resolver := stubBrowserResolver{ips: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}}
	if err := validateBrowserURLWithResolver(context.Background(), "https://public-name.example/", resolver); err != nil {
		t.Fatalf("public hostname rejected: %v", err)
	}
}

func TestValidateBrowserURLWithResolver_FailsClosed(t *testing.T) {
	tests := []struct {
		name     string
		resolver stubBrowserResolver
	}{
		{
			name:     "resolver error",
			resolver: stubBrowserResolver{err: errors.New("lookup unavailable")},
		},
		{
			name:     "no addresses",
			resolver: stubBrowserResolver{},
		},
		{
			name: "mixed public and private",
			resolver: stubBrowserResolver{ips: []net.IPAddr{
				{IP: net.ParseIP("93.184.216.34")},
				{IP: net.ParseIP("10.0.0.1")},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateBrowserURLWithResolver(context.Background(), "https://public-name.example/", tt.resolver); err == nil {
				t.Fatal("unsafe or indeterminate DNS result must be rejected")
			}
		})
	}
}

func TestSSRFInterceptor_NewAndCancel(t *testing.T) {
	interceptor := NewSSRFInterceptor()
	if interceptor == nil {
		t.Fatal("NewSSRFInterceptor() = nil")
	}
	// Cancel without Enable should not panic.
	interceptor.Cancel()

	blocked := interceptor.BlockedURLs()
	if len(blocked) != 0 {
		t.Errorf("BlockedURLs() = %v, want empty", blocked)
	}
}

func TestSSRFInterceptorQueueOverflowFailsClosed(t *testing.T) {
	interceptor := newSSRFInterceptor(func(context.Context, string) error { return nil })
	interceptor.pausedCh = make(chan *fetch.EventRequestPaused, 1)
	interceptor.enqueuePaused(&fetch.EventRequestPaused{})
	interceptor.enqueuePaused(&fetch.EventRequestPaused{})
	if err := interceptor.Err(); err == nil || !strings.Contains(err.Error(), "queue overflow") {
		t.Fatalf("Err() = %v, want queue overflow", err)
	}
}

func TestSSRFInterceptorCommandFailuresFailClosed(t *testing.T) {
	t.Run("continue", func(t *testing.T) {
		interceptor := newSSRFInterceptor(func(context.Context, string) error { return nil })
		interceptor.continueFn = func(context.Context, fetch.RequestID) error {
			return errors.New("continue unavailable")
		}
		interceptor.handlePaused(t.Context(), &fetch.EventRequestPaused{
			RequestID: "request-1",
			Request:   &network.Request{URL: "https://example.com/asset.png"},
		})
		if err := interceptor.Err(); err == nil || !strings.Contains(err.Error(), "continue validated request") {
			t.Fatalf("Err() = %v, want continue failure", err)
		}
	})

	t.Run("fail", func(t *testing.T) {
		interceptor := newSSRFInterceptor(func(context.Context, string) error {
			return errors.New("blocked by policy")
		})
		interceptor.failFn = func(context.Context, fetch.RequestID) error {
			return errors.New("fail unavailable")
		}
		interceptor.handlePaused(t.Context(), &fetch.EventRequestPaused{
			RequestID: "request-2",
			Request:   &network.Request{URL: "https://example.com/private.png"},
		})
		if err := interceptor.Err(); err == nil || !strings.Contains(err.Error(), "fail blocked request") {
			t.Fatalf("Err() = %v, want fail-command failure", err)
		}
	})
}
