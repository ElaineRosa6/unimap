//go:build headless_e2e

package screenshot

import (
	"bytes"
	"context"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHeadlessChromeExecutesJavaScriptAndCapturesPNG(t *testing.T) {
	chromePath := os.Getenv("UNIMAP_CHROME_PATH")
	if chromePath == "" {
		t.Fatal("UNIMAP_CHROME_PATH is required for the headless integration test")
	}
	if healthy, err := (&CDPHealthChecker{ConfiguredChromePath: chromePath}).Check(t.Context()); err != nil || !healthy {
		t.Fatalf("Chrome readiness probe failed: healthy=%v err=%v", healthy, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><body><div id="state">loading</div><script>document.getElementById('state').textContent='ready'</script></body></html>`))
	}))
	defer server.Close()

	mgr := NewManager(Config{
		BaseDir:     t.TempDir(),
		ChromePath:  chromePath,
		UserDataDir: filepath.Join(t.TempDir(), "profile"),
		Headless:    true,
		Timeout:     20 * time.Second,
		WaitTime:    100 * time.Millisecond,
		MaxSessions: 1,
	})
	allowHeadlessFixtureOrigin(t, mgr, server.URL)

	image, err := mgr.CaptureScreenshot(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatalf("headless capture failed: %v", err)
	}
	if _, err := png.DecodeConfig(bytes.NewReader(image)); err != nil {
		t.Fatalf("capture is not a valid PNG: %v", err)
	}

	var title, html string
	if err := mgr.loadPageContent(context.Background(), server.URL, nil, &title, &html); err != nil {
		t.Fatalf("headless DOM collection failed: %v", err)
	}
	if !bytes.Contains([]byte(html), []byte(`id="state">ready`)) {
		t.Fatalf("JavaScript result missing from collected DOM: %s", html)
	}
}

func TestHeadlessDOMCollectionBlocksPrivateRedirect(t *testing.T) {
	chromePath := os.Getenv("UNIMAP_CHROME_PATH")
	if chromePath == "" {
		t.Fatal("UNIMAP_CHROME_PATH is required for the headless integration test")
	}

	var blockedHits atomic.Int64
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		blockedHits.Add(1)
		_, _ = w.Write([]byte("<html><body>internal</body></html>"))
	}))
	defer blocked.Close()

	entry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, blocked.URL, http.StatusFound)
	}))
	defer entry.Close()

	mgr := NewManager(Config{
		BaseDir:     t.TempDir(),
		ChromePath:  chromePath,
		UserDataDir: filepath.Join(t.TempDir(), "profile"),
		Headless:    true,
		Timeout:     20 * time.Second,
		WaitTime:    10 * time.Millisecond,
		MaxSessions: 1,
	})
	allowHeadlessFixtureOrigin(t, mgr, entry.URL)

	var title, html string
	err := mgr.loadPageContent(context.Background(), entry.URL, nil, &title, &html)
	if err == nil {
		t.Fatal("DOM collection followed a redirect to a blocked loopback origin")
	}
	if !strings.Contains(err.Error(), "ssrf:") {
		t.Fatalf("blocked redirect returned a non-SSRF error: %v", err)
	}
	if got := blockedHits.Load(); got != 0 {
		t.Fatalf("blocked origin received %d request(s), want 0", got)
	}
}

func TestHeadlessDOMCollectionBlocksPrivateSubresources(t *testing.T) {
	chromePath := os.Getenv("UNIMAP_CHROME_PATH")
	if chromePath == "" {
		t.Fatal("UNIMAP_CHROME_PATH is required for the headless integration test")
	}

	var blockedHits atomic.Int64
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		blockedHits.Add(1)
		_, _ = w.Write([]byte("restricted"))
	}))
	defer blocked.Close()

	entry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><html><head><link rel="stylesheet" href="` + blocked.URL + `/private.css"></head><body><img src="` + blocked.URL + `/private.png"></body></html>`))
	}))
	defer entry.Close()

	mgr := NewManager(Config{
		BaseDir:     t.TempDir(),
		ChromePath:  chromePath,
		UserDataDir: filepath.Join(t.TempDir(), "profile"),
		Headless:    true,
		Timeout:     20 * time.Second,
		WaitTime:    10 * time.Millisecond,
		MaxSessions: 1,
	})
	allowHeadlessFixtureOrigin(t, mgr, entry.URL)

	var title, html string
	err := mgr.loadPageContent(context.Background(), entry.URL, nil, &title, &html)
	if err == nil {
		t.Fatal("DOM collection accepted restricted subresources")
	}
	if !strings.Contains(err.Error(), "ssrf:") {
		t.Fatalf("blocked subresource returned a non-SSRF error: %v", err)
	}
	if got := blockedHits.Load(); got != 0 {
		t.Fatalf("blocked subresource origin received %d request(s), want 0", got)
	}
}

func allowHeadlessFixtureOrigin(t *testing.T, mgr *Manager, rawOrigin string) {
	t.Helper()
	allowed, err := url.Parse(rawOrigin)
	if err != nil {
		t.Fatalf("parse fixture origin: %v", err)
	}
	mgr.urlValidator = func(ctx context.Context, rawURL string) error {
		candidate, err := url.Parse(rawURL)
		if err == nil && candidate.Scheme == allowed.Scheme && candidate.Host == allowed.Host {
			return nil
		}
		return ValidateBrowserURLLive(ctx, rawURL)
	}
	mgr.egressProxyFactory = func(ctx context.Context) (*browserEgressProxy, error) {
		return newBrowserEgressProxyWithDialResolver(ctx, func(resolveCtx context.Context, addr string) ([]string, error) {
			if addr == allowed.Host {
				return []string{addr}, nil
			}
			return resolveBrowserDialTargets(resolveCtx, net.DefaultResolver, addr)
		})
	}
}
