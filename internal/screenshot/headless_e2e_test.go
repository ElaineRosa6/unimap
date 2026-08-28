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
	waitForChromeReady(t, chromePath)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><body><div id="state">loading</div><script>document.getElementById('state').textContent='ready'</script></body></html>`))
	}))
	defer server.Close()

	profileDir := softChromeProfileDir(t)
	mgr := NewManager(Config{
		BaseDir:     softTempDir(t),
		ChromePath:  chromePath,
		UserDataDir: profileDir,
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
	waitForChromeReady(t, chromePath)

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
		BaseDir:     softTempDir(t),
		ChromePath:  chromePath,
		UserDataDir: softChromeProfileDir(t),
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
	waitForChromeReady(t, chromePath)

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
		BaseDir:     softTempDir(t),
		ChromePath:  chromePath,
		UserDataDir: softChromeProfileDir(t),
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

// waitForChromeReady retries the CDP readiness probe with backoff. CI runners
// occasionally stall on the first chrome --version probe (3s timeout), which
// previously failed the suite immediately with healthy=false err=<nil>.
func waitForChromeReady(t *testing.T, chromePath string) {
	t.Helper()
	checker := &CDPHealthChecker{
		ConfiguredChromePath: chromePath,
		ChromeProbe: func(ctx context.Context, path string) bool {
			probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			return validateChromeBinary(probeCtx, path)
		},
	}

	deadline := time.Now().Add(45 * time.Second)
	var lastHealthy bool
	var lastErr error
	for attempt := 0; time.Now().Before(deadline); attempt++ {
		lastHealthy, lastErr = checker.Check(t.Context())
		if lastErr == nil && lastHealthy {
			return
		}
		sleep := time.Duration(200*(1<<min(attempt, 4))) * time.Millisecond
		if sleep > 2*time.Second {
			sleep = 2 * time.Second
		}
		select {
		case <-t.Context().Done():
			t.Fatalf("Chrome readiness probe canceled: healthy=%v err=%v", lastHealthy, lastErr)
		case <-time.After(sleep):
		}
	}
	t.Fatalf("Chrome readiness probe failed after retries: healthy=%v err=%v", lastHealthy, lastErr)
}

// softTempDir is like t.TempDir but cleanup failures (e.g. Chrome locks) are
// logged and do not fail the suite.
func softTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "unimap-e2e-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Logf("soft temp cleanup ignored: %v", err)
		}
	})
	return dir
}

func softChromeProfileDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(softTempDir(t), "profile")
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
		direct := &net.Dialer{Timeout: 10 * time.Second}
		return newBrowserEgressProxyWithDialResolver(ctx, func(resolveCtx context.Context, addr string) ([]string, error) {
			if addr == allowed.Host {
				return []string{addr}, nil
			}
			return resolveBrowserDialTargets(resolveCtx, net.DefaultResolver, addr)
		}, direct.DialContext)
	}
}
