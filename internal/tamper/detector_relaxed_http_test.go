package tamper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearPageHashCache clears the five-minute page-hash cache between requests
// when a test intentionally changes the served document.
func clearPageHashCache(d *Detector) {
	d.cacheMu.Lock()
	d.cache = make(map[string]*cacheEntry)
	d.cacheMu.Unlock()
}

func TestRelaxed_TimeBasedDynamicContent_NoFalsePositive(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		now := time.Now().UTC().Format(time.RFC3339)
		n := time.Now().UnixNano()
		rid := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
			n&0xffffffff, (n>>32)&0xffff, (n>>16)&0xffff, n&0xffff, n&0xffffffffffff)
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>Test Page</title></head><body>
<main><h1>Dynamic Content Page</h1><p>Current time: %s</p><p>Request ID: %s</p></main>
<script>var _hmt = _hmt || []; _hmt.push(['_trackPageview', %d]);</script></body></html>`, now, rid, n)
	}))
	defer ts.Close()

	detector := NewDetector(DetectorConfig{DetectionMode: DetectionModeRelaxed, PerformanceMode: PerformanceModeFast})
	baseline, err := detector.ComputePageHash(t.Context(), ts.URL)
	require.NoError(t, err)
	require.NoError(t, detector.SaveBaseline(ts.URL, baseline))
	clearPageHashCache(detector)
	result, err := detector.CheckTampering(t.Context(), ts.URL)
	require.NoError(t, err)
	assert.False(t, result.Tampered)
	assert.Contains(t, []string{"normal", "normal_dynamic"}, result.Status)
}

func TestRelaxed_CompletelyUnchangedPage_ReturnsNormal(t *testing.T) {
	const staticHTML = `<!DOCTYPE html><html><head><title>Static Page</title></head><body><main><h1>Static Content</h1><p>This never changes.</p></main></body></html>`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, staticHTML)
	}))
	defer ts.Close()

	detector := NewDetector(DetectorConfig{DetectionMode: DetectionModeRelaxed, PerformanceMode: PerformanceModeFast})
	ctx := context.Background()
	baseline, err := detector.ComputePageHash(ctx, ts.URL)
	require.NoError(t, err)
	require.NoError(t, detector.SaveBaseline(ts.URL, baseline))
	result, err := detector.CheckTampering(ctx, ts.URL)
	require.NoError(t, err)
	assert.False(t, result.Tampered)
	assert.Equal(t, "normal", result.Status)
}
