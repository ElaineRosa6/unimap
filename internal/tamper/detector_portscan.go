package tamper

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/unimap/project/internal/logger"
)

// scanHostPorts 扫描目标主机的开放端口（并发 TCP dial）
func scanHostPorts(ctx context.Context, host string, ports []int, timeout time.Duration) []int {
	if len(ports) == 0 {
		return nil
	}

	var open []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 控制并发数的信号量
	sem := make(chan struct{}, 50)

	for _, port := range ports {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if isPortOpen(ctx, host, p, timeout) {
				mu.Lock()
				open = append(open, p)
				mu.Unlock()
			}
		}(port)
	}

	wg.Wait()
	sort.Ints(open)
	return open
}

// isPortOpen 检查单端口是否开放
func isPortOpen(ctx context.Context, host string, port int, timeout time.Duration) bool {
	target := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// extractHost 从 URL 中提取主机名
func extractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// runPortScanIfEnabled 在巡检时执行端口扫描（异步采集，不阻塞主流程）
func (d *Detector) runPortScanIfEnabled(ctx context.Context, targetURL string, result *PageHashResult) {
	if !d.portScanEnabled || len(d.portScanList) == 0 {
		return
	}

	host := extractHost(targetURL)
	if host == "" {
		return
	}

	// 端口扫描有独立超时（最长 30s）
	scanCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	start := time.Now()
	openPorts := scanHostPorts(scanCtx, host, d.portScanList, d.portScanTimeout)
	elapsed := time.Since(start)

	result.OpenPorts = openPorts
	logger.CtxDebugf(ctx, "port scan %s: %d ports scanned, %d open in %v",
		host, len(d.portScanList), len(openPorts), elapsed)
}

// comparePorts 对比基线和当前端口，返回变化列表
func comparePorts(baseline, current []int) []PortChange {
	baselineSet := make(map[int]struct{}, len(baseline))
	for _, p := range baseline {
		baselineSet[p] = struct{}{}
	}
	currentSet := make(map[int]struct{}, len(current))
	for _, p := range current {
		currentSet[p] = struct{}{}
	}

	var changes []PortChange

	// 新开放的端口
	for _, p := range current {
		if _, exists := baselineSet[p]; !exists {
			changes = append(changes, PortChange{Port: p, Change: "opened"})
		}
	}

	// 关闭的端口
	for _, p := range baseline {
		if _, exists := currentSet[p]; !exists {
			changes = append(changes, PortChange{Port: p, Change: "closed"})
		}
	}

	return changes
}
