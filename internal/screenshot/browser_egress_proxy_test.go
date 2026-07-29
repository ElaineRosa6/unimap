package screenshot

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBrowserEgressProxyRejectsRestrictedTargetBeforeConnect(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits.Add(1)
	}))
	defer target.Close()

	proxy, err := newBrowserEgressProxy(t.Context(), net.DefaultResolver)
	if err != nil {
		t.Fatalf("start egress proxy: %v", err)
	}
	defer proxy.Close()

	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatalf("parse proxy URL: %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   3 * time.Second,
	}
	resp, err := client.Get(target.URL)
	if err == nil {
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode < 400 {
			t.Fatalf("restricted target returned status %d", resp.StatusCode)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("restricted target received %d connections, want 0", hits.Load())
	}
}

func TestResolveBrowserDialTargetsFailsClosed(t *testing.T) {
	tests := []struct {
		name     string
		resolver stubBrowserResolver
	}{
		{name: "dns error", resolver: stubBrowserResolver{err: context.DeadlineExceeded}},
		{name: "empty", resolver: stubBrowserResolver{}},
		{name: "mixed", resolver: stubBrowserResolver{ips: []net.IPAddr{
			{IP: net.ParseIP("93.184.216.34")},
			{IP: net.ParseIP("127.0.0.1")},
		}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveBrowserDialTargets(t.Context(), tt.resolver, "example.test:443")
			if err == nil || !strings.Contains(err.Error(), "ssrf:") {
				t.Fatalf("resolveBrowserDialTargets error = %v, want SSRF failure", err)
			}
		})
	}
}
