//go:build headless_e2e

package tamper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDetectorWithCDP 创建一个配置了 CI 安全 chromedp allocator 的 Detector。
// GitHub Actions / 容器环境以 root 运行 Chrome，需要 --no-sandbox 才能启动，
// 否则 Chrome 启动时收到 SIGABRT（"chrome failed to start: Received signal 6"）；
// disable-dev-shm-usage 避免 /dev/shm 过小导致的崩溃。仅用于 chromedp 模式测试。
func newTestDetectorWithCDP(t *testing.T, cfg DetectorConfig) *Detector {
	t.Helper()
	d := NewDetector(cfg)
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("disable-dev-shm-usage", "true"),
	)
	if chromePath := strings.TrimSpace(os.Getenv("UNIMAP_CHROME_PATH")); chromePath != "" {
		opts = append(opts, chromedp.ExecPath(chromePath))
	}
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	t.Cleanup(allocCancel)
	d.SetAllocator(context.Background(), allocCtx, allocCancel)
	return d
}

// TestRelaxed_VersionedJSFiles_NoFalsePositive 验证：
// 版本化 JS 文件名在 relaxed 模式不应导致误报
func TestRelaxed_VersionedJSFiles_NoFalsePositive(t *testing.T) {
	var versionCounter atomic.Int64

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hash := fmt.Sprintf("a%08x", versionCounter.Add(1))
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<title>SPA App</title>
	<link rel="stylesheet" href="/static/app.%s.css">
</head>
<body>
<main><h1>Hello World</h1></main>
<script src="/static/runtime.%s.js"></script>
<script src="/static/vendor.%s.js"></script>
<script src="/static/app.%s.js"></script>
</body>
</html>`, hash, hash, hash, hash)
	}))
	defer ts.Close()

	detector := newTestDetectorWithCDP(t, DetectorConfig{
		DetectionMode:   DetectionModeRelaxed,
		PerformanceMode: PerformanceModeBalanced,
	})

	ctx := context.Background()

	// retryComputePageHash 对 chromedp WebSocket 超时等瞬态错误做重试
	retryComputePageHash := func(targetURL string) (*PageHashResult, error) {
		var result *PageHashResult
		var err error
		for attempt := 0; attempt < 3; attempt++ {
			result, err = detector.ComputePageHash(ctx, targetURL)
			if err == nil {
				return result, nil
			}
			if strings.Contains(err.Error(), "websocket url timeout") ||
				strings.Contains(err.Error(), "connection refused") {
				t.Logf("ComputePageHash transient error (attempt %d/3): %v", attempt+1, err)
				time.Sleep(2 * time.Second)
				continue
			}
			return nil, err
		}
		return result, err
	}

	baseline, err := retryComputePageHash(ts.URL)
	require.NoError(t, err)
	require.NoError(t, detector.SaveBaseline(ts.URL, baseline))

	clearPageHashCache(detector)

	result, err := detector.CheckTampering(ctx, ts.URL)
	require.NoError(t, err)

	assert.False(t, result.Tampered, "版本化 JS 文件名在relaxed模式不应误报")
	t.Logf("Status: %s, Tampered: %v, Segments: %v",
		result.Status, result.Tampered, result.TamperedSegments)
}

// TestRelaxed_SSRHydrationData_NoFalsePositive 验证：
// Next.js/Nuxt SSR 水合数据差异在 relaxed 模式不应误报
func TestRelaxed_SSRHydrationData_NoFalsePositive(t *testing.T) {
	counter := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter++
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>SSR Page</title></head>
<body>
<div id="__next">
	<h1>Server Rendered Page</h1>
	<p>Build: %d</p>
</div>
<script>window.__NEXT_DATA__ = {"props":{"pageProps":{"buildId":"%d"}},"page":"/","query":{},"buildId":"%d"};</script>
<script>window.__NUXT__ = {"data":[],"state":{},"_errors":{},"serverRendered":true,"config":{}};</script>
</body>
</html>`, counter, counter, counter)
	}))
	defer ts.Close()

	detector := newTestDetectorWithCDP(t, DetectorConfig{
		DetectionMode:   DetectionModeRelaxed,
		PerformanceMode: PerformanceModeBalanced,
	})

	ctx := context.Background()

	baseline, err := detector.ComputePageHash(ctx, ts.URL)
	require.NoError(t, err)
	require.NoError(t, detector.SaveBaseline(ts.URL, baseline))

	clearPageHashCache(detector)

	result, err := detector.CheckTampering(ctx, ts.URL)
	require.NoError(t, err)

	assert.False(t, result.Tampered, "SSR水合数据差异在relaxed模式不应误报")
	t.Logf("Status: %s, Tampered: %v", result.Status, result.Tampered)
}

// TestRelaxed_InjectedMaliciousIframe_DetectsTamper 验证：
// 注入隐藏 iframe 在 relaxed 模式应正确检出（隐藏 iframe 是 relaxed 模式的敏感标记）
func TestRelaxed_InjectedMaliciousIframe_DetectsTamper(t *testing.T) {
	htmlBase := `<!DOCTYPE html>
<html>
<head><title>Normal Page</title></head>
<body>
<main>
	<h1>Welcome</h1>
	<p>This is a normal page.</p>
</main>
<script src="/static/app.js"></script>
</body>
</html>`

	requestCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			_, _ = fmt.Fprint(w, htmlBase)
		} else {
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Normal Page</title></head>
<body>
<main>
	<h1>Welcome</h1>
	<p>This is a normal page.</p>
</main>
<iframe src="https://evil.com/phish" style="display:none;width:0;height:0"></iframe>
<script src="/static/app.js"></script>
</body>
</html>`)
		}
	}))
	defer ts.Close()

	detector := newTestDetectorWithCDP(t, DetectorConfig{
		DetectionMode:   DetectionModeRelaxed,
		PerformanceMode: PerformanceModeBalanced,
	})

	ctx := context.Background()

	baseline, err := detector.ComputePageHash(ctx, ts.URL)
	require.NoError(t, err)
	require.NoError(t, detector.SaveBaseline(ts.URL, baseline))

	clearPageHashCache(detector)

	result, err := detector.CheckTampering(ctx, ts.URL)
	require.NoError(t, err)

	assert.True(t, result.Tampered, "注入隐藏iframe在relaxed模式应检出篡改")
	assert.Equal(t, "suspicious", result.Status)
	assert.Contains(t, result.SuspiciousFlags, "hidden_iframe_detected")
	t.Logf("Status: %s, Tampered: %v, Flags: %v",
		result.Status, result.Tampered, result.SuspiciousFlags)
}

// TestRelaxed_SignificantMainContentChange_DetectsTamper 验证：
// 大幅修改 main 段内容在 relaxed 模式应正确检出（main 是 critical stable segment）
func TestRelaxed_SignificantMainContentChange_DetectsTamper(t *testing.T) {
	requestCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Normal</title></head>
<body><main><h1>Welcome</h1><p>Normal content.</p></main></body></html>`)
		} else {
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Normal</title></head>
<body><main><h1>HACKED</h1><p>This page has been defaced!</p></main></body></html>`)
		}
	}))
	defer ts.Close()

	detector := newTestDetectorWithCDP(t, DetectorConfig{
		DetectionMode:   DetectionModeRelaxed,
		PerformanceMode: PerformanceModeBalanced,
	})

	ctx := context.Background()

	baseline, err := detector.ComputePageHash(ctx, ts.URL)
	require.NoError(t, err)
	require.NoError(t, detector.SaveBaseline(ts.URL, baseline))

	clearPageHashCache(detector)

	result, err := detector.CheckTampering(ctx, ts.URL)
	require.NoError(t, err)

	assert.True(t, result.Tampered, "大幅修改main段在relaxed模式应检出篡改")
	assert.Equal(t, "tampered", result.Status)
	t.Logf("Status: %s, Tampered: %v, Segments: %v",
		result.Status, result.Tampered, result.TamperedSegments)
}

// TestStrict_MD5Change_DetectsTamper 验证：
// strict 模式下 SimpleMD5Hash 变化仍应触发篡改
func TestStrict_MD5Change_DetectsTamper(t *testing.T) {
	requestCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Page</title></head>
<body><main><p>Version A</p></main></body></html>`)
		} else {
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Page</title></head>
<body><main><p>Version B</p></main></body></html>`)
		}
	}))
	defer ts.Close()

	detector := newTestDetectorWithCDP(t, DetectorConfig{
		DetectionMode: DetectionModeStrict,
	})

	ctx := context.Background()

	baseline, err := detector.ComputePageHash(ctx, ts.URL)
	require.NoError(t, err)
	require.NoError(t, detector.SaveBaseline(ts.URL, baseline))

	clearPageHashCache(detector)

	result, err := detector.CheckTampering(ctx, ts.URL)
	require.NoError(t, err)

	assert.True(t, result.Tampered, "strict模式：内容变化应检出篡改")
	assert.Equal(t, "tampered", result.Status)
}

// TestNormalDynamic_DoesNotSetTampered 验证：
// normal_dynamic 状态下 Tampered 必须为 false（回归此前 normal_dynamic 设 Tampered=true 的 bug）
func TestNormalDynamic_DoesNotSetTampered(t *testing.T) {
	counter := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter++
		now := time.Now().UTC().Format(time.RFC3339) // ISO 8601，可被 reTimestamp 归一化
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<main>
	<h1>Fixed Content</h1>
	<p>This paragraph never changes.</p>
	<p>Generated: %s</p>
</main>
<script>var _hmt = _hmt || []; _hmt.push(['_trackPageview', %d]);</script>
</body>
</html>`, now, counter)
	}))
	defer ts.Close()

	detector := newTestDetectorWithCDP(t, DetectorConfig{
		DetectionMode:   DetectionModeRelaxed,
		PerformanceMode: PerformanceModeBalanced,
	})

	ctx := context.Background()

	baseline, err := detector.ComputePageHash(ctx, ts.URL)
	require.NoError(t, err)
	require.NoError(t, detector.SaveBaseline(ts.URL, baseline))

	clearPageHashCache(detector)

	result, err := detector.CheckTampering(ctx, ts.URL)
	require.NoError(t, err)

	// 关键断言：normal_dynamic 状态下 Tampered 必须是 false
	assert.False(t, result.Tampered,
		"normal_dynamic状态下Tampered必须为false（回归：此前有bug设置Tampered=true）")
	assert.Contains(t, []string{"normal", "normal_dynamic"}, result.Status)
	t.Logf("Status: %s, Tampered: %v", result.Status, result.Tampered)
}
