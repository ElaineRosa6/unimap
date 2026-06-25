package tamper

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractHost(t *testing.T) {
	assert.Equal(t, "www.example.com", extractHost("https://www.example.com/page"))
	assert.Equal(t, "192.168.1.1", extractHost("http://192.168.1.1:8080/api"))
	assert.Equal(t, "", extractHost("not-a-url"))
}

func TestIsPortOpen(t *testing.T) {
	// 启动本地监听
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	addr := ln.Addr().(*net.TCPAddr)

	ctx := context.Background()

	// 监听的端口应返回 true
	assert.True(t, isPortOpen(ctx, "127.0.0.1", addr.Port, 500*time.Millisecond))

	// 未监听的端口应返回 false（使用高位端口）
	assert.False(t, isPortOpen(ctx, "127.0.0.1", 19999, 200*time.Millisecond))
}

func TestScanHostPorts(t *testing.T) {
	// 启动几个本地监听
	listeners := make([]net.Listener, 0, 3)
	defer func() {
		for _, l := range listeners {
			l.Close()
		}
	}()

	var ports []int
	for i := 0; i < 3; i++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		listeners = append(listeners, ln)
		ports = append(ports, ln.Addr().(*net.TCPAddr).Port)
	}

	ctx := context.Background()
	scanPorts := append(ports, 19998, 19999) // 加入两个未监听端口
	open := scanHostPorts(ctx, "127.0.0.1", scanPorts, 500*time.Millisecond)

	// 应只检测到监听的那 3 个端口
	assert.Len(t, open, 3)
	for _, p := range ports {
		assert.Contains(t, open, p)
	}
}

func TestComparePorts(t *testing.T) {
	tests := []struct {
		name     string
		baseline []int
		current  []int
		want     []PortChange
	}{
		{
			name:     "no changes",
			baseline: []int{80, 443},
			current:  []int{80, 443},
			want:     nil,
		},
		{
			name:     "new port opened",
			baseline: []int{80, 443},
			current:  []int{80, 443, 8080},
			want:     []PortChange{{Port: 8080, Change: "opened"}},
		},
		{
			name:     "port closed",
			baseline: []int{80, 443, 8080},
			current:  []int{80, 443},
			want:     []PortChange{{Port: 8080, Change: "closed"}},
		},
		{
			name:     "mixed changes",
			baseline: []int{80, 443},
			current:  []int{80, 8080},
			want:     []PortChange{{Port: 8080, Change: "opened"}, {Port: 443, Change: "closed"}},
		},
		{
			name:     "empty baseline",
			baseline: nil,
			current:  []int{80, 443},
			want:     []PortChange{{Port: 80, Change: "opened"}, {Port: 443, Change: "opened"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := comparePorts(tt.baseline, tt.current)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestDetector_PortScanEnabled_RunsOnCheck(t *testing.T) {
	// 启动本地 HTTP + 一个额外端口
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	extraPort := ln.Addr().(*net.TCPAddr).Port

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>Test</title></head><body><main><p>Hello</p></main></body></html>`))
	}))
	defer ts.Close()

	detector := NewDetector(DetectorConfig{
		DetectionMode:    DetectionModeRelaxed,
		PerformanceMode:  PerformanceModeFast,
		PortScanEnabled:  true,
		PortScanList:     []int{extraPort}, // 只扫这个额外端口
		PortScanTimeout:  500 * time.Millisecond,
	})

	ctx := context.Background()

	// 设置基线（端口扫描在 ComputePageHash 中触发）
	baseline, err := detector.ComputePageHash(ctx, ts.URL)
	require.NoError(t, err)
	require.NoError(t, detector.SaveBaseline(ts.URL, baseline))

	// 基线应记录开放端口
	assert.Contains(t, baseline.OpenPorts, extraPort, "基线应记录额外监听的端口")
	if len(baseline.OpenPorts) > 0 {
		t.Logf("Baseline open ports: %v", baseline.OpenPorts)
	}

	// 执行检测
	result, err := detector.CheckTampering(ctx, ts.URL)
	require.NoError(t, err)
	require.NotNil(t, result.CurrentHash)

	t.Logf("Status: %s, PortChanges: %v, CurrentPorts: %v, BaselinePorts: %v",
		result.Status, result.PortChanges, result.CurrentHash.OpenPorts, baseline.OpenPorts)
}

func TestNormalizePortList(t *testing.T) {
	// 空列表返回默认列表
	def := normalizePortList(nil)
	assert.NotEmpty(t, def)
	assert.Greater(t, len(def), 10, "默认端口列表应有合理数量")

	// 自定义列表去重+排序
	custom := normalizePortList([]int{443, 80, 443, 8080, 80})
	assert.Equal(t, []int{80, 443, 8080}, custom)

	// 非法端口被过滤
	filtered := normalizePortList([]int{0, 80, 99999, 443})
	assert.Equal(t, []int{80, 443}, filtered)
}

func TestNormalizePortTimeout(t *testing.T) {
	assert.Equal(t, 800*time.Millisecond, normalizePortTimeout(0))
	assert.Equal(t, 100*time.Millisecond, normalizePortTimeout(50*time.Millisecond))
	assert.Equal(t, 5*time.Second, normalizePortTimeout(10*time.Second))
	assert.Equal(t, 500*time.Millisecond, normalizePortTimeout(500*time.Millisecond))
}
