//go:build headless_e2e

package screenshot

import (
	"bytes"
	"context"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
