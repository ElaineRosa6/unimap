package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNormalizeScanPorts(t *testing.T) {
	t.Run("default ports", func(t *testing.T) {
		ports := normalizeScanPorts(nil)
		if len(ports) == 0 {
			t.Fatalf("expected default ports")
		}
	})

	t.Run("dedupe and trim invalid", func(t *testing.T) {
		ports := normalizeScanPorts([]int{8080, 8080, -1, 0, 65536, 443})
		if len(ports) != 2 {
			t.Fatalf("expected 2 valid ports, got %d", len(ports))
		}
		if ports[0] != 443 || ports[1] != 8080 {
			t.Fatalf("unexpected normalized ports: %+v", ports)
		}
	})

	t.Run("preserves the full port range", func(t *testing.T) {
		ports, err := ParsePortSpec("all")
		if err != nil {
			t.Fatalf("ParsePortSpec(all): %v", err)
		}
		got := normalizeScanPorts(ports)
		if len(got) != 65535 || got[0] != 1 || got[len(got)-1] != 65535 {
			t.Fatalf("expected all 65535 ports, got len=%d first=%d last=%d", len(got), got[0], got[len(got)-1])
		}
	})
}

func TestScanHostPortsConcurrentUsesRealTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	open, attempted, err := scanHostPortsConcurrent(
		context.Background(), []string{"127.0.0.1"}, []int{port}, 4, time.Second, nil,
	)
	if err != nil {
		t.Fatalf("scanHostPortsConcurrent: %v", err)
	}
	if attempted != 1 || len(open["127.0.0.1"]) != 1 || open["127.0.0.1"][0] != port {
		t.Fatalf("expected real listener %d to be open; attempted=%d result=%v", port, attempted, open)
	}
}

func TestScanHostPortsConcurrentActuallyRunsConcurrently(t *testing.T) {
	var active atomic.Int32
	var maxActive atomic.Int32
	dial := func(ctx context.Context, ip string, port int, timeout time.Duration) bool {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			previous := maxActive.Load()
			if current <= previous || maxActive.CompareAndSwap(previous, current) {
				break
			}
		}
		select {
		case <-time.After(20 * time.Millisecond):
			return port%2 == 0
		case <-ctx.Done():
			return false
		}
	}

	ports := []int{1, 2, 3, 4, 5, 6, 7, 8}
	open, attempted, err := scanHostPortsConcurrent(
		context.Background(), []string{"192.0.2.1"}, ports, 4, time.Second, dial,
	)
	if err != nil {
		t.Fatalf("scanHostPortsConcurrent: %v", err)
	}
	if attempted != len(ports) || maxActive.Load() < 2 {
		t.Fatalf("expected concurrent execution; attempted=%d max_active=%d", attempted, maxActive.Load())
	}
	if got := open["192.0.2.1"]; len(got) != 4 || got[0] != 2 || got[3] != 8 {
		t.Fatalf("unexpected open ports: %v", got)
	}
}

func TestPortScanPlanDeduplicatesIPsAndBuildsCartesianProduct(t *testing.T) {
	results := []URLPortScanResult{
		{Input: "one.example", Status: "resolved", ResolvedIPs: []string{"192.0.2.10"}},
		{Input: "two.example", Status: "resolved", ResolvedIPs: []string{"192.0.2.10", "198.51.100.20"}},
	}
	plan := buildPortScanPlan(results, []int{80, 443})

	if len(plan.IPs) != 2 || plan.PlannedConnections != 4 || plan.DuplicateIPReferences != 1 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if plan.IPs[0] != "192.0.2.10" || plan.IPs[1] != "198.51.100.20" {
		t.Fatalf("plan IPs are not unique and sorted: %v", plan.IPs)
	}
}

func TestApplyAuthorizedScopeRejectsTargetWithUnlistedResolvedIP(t *testing.T) {
	networks, err := parseAuthorizedNetworks([]string{"192.0.2.10", "198.51.100.0/24"})
	if err != nil {
		t.Fatalf("parseAuthorizedNetworks: %v", err)
	}
	results := []URLPortScanResult{
		{Input: "allowed", Status: "resolved", ResolvedIPs: []string{"192.0.2.10", "198.51.100.20"}},
		{Input: "mixed", Status: "resolved", ResolvedIPs: []string{"192.0.2.10", "203.0.113.30"}},
	}
	applyAuthorizedScope(results, networks)

	if results[0].Status != "resolved" {
		t.Fatalf("allowed target status = %q", results[0].Status)
	}
	if results[1].Status != "not_authorized" || !strings.Contains(results[1].Reason, "203.0.113.30") {
		t.Fatalf("mixed target should be rejected with the unlisted IP, got %+v", results[1])
	}
}

func TestExecutePortScanPlanScansEachUniqueIPPortPairOnce(t *testing.T) {
	results := []URLPortScanResult{
		{Input: "one.example", Status: "resolved", ResolvedIPs: []string{"192.0.2.10"}},
		{Input: "two.example", Status: "resolved", ResolvedIPs: []string{"192.0.2.10", "198.51.100.20"}},
	}
	ports := []int{80, 443}
	plan := buildPortScanPlan(results, ports)
	var calls atomic.Int32
	dial := func(ctx context.Context, ip string, port int, timeout time.Duration) bool {
		calls.Add(1)
		return ip == "192.0.2.10" && port == 443
	}

	outcome := executePortScanPlan(context.Background(), plan, ports, 4, time.Second, dial)
	if outcome.Err != nil {
		t.Fatalf("executePortScanPlan: %v", outcome.Err)
	}
	if calls.Load() != 4 || outcome.AttemptedConnections != 4 {
		t.Fatalf("expected exactly four unique IP×port attempts, calls=%d outcome=%+v", calls.Load(), outcome)
	}
	if got := outcome.OpenPorts["192.0.2.10"]; len(got) != 1 || got[0] != 443 {
		t.Fatalf("unexpected open ports: %v", outcome.OpenPorts)
	}
}

func TestApplyPortScanOutcomeAttributesSharedIPResults(t *testing.T) {
	results := []URLPortScanResult{
		{Input: "one.example", Status: "resolved", ResolvedIPs: []string{"192.0.2.10"}},
		{Input: "two.example", Status: "resolved", ResolvedIPs: []string{"192.0.2.10", "198.51.100.20"}},
	}
	outcome := portScanExecution{
		OpenPorts: map[string][]int{
			"192.0.2.10":    {443},
			"198.51.100.20": {},
		},
		AttemptedByIP:        map[string]int{"192.0.2.10": 2, "198.51.100.20": 2},
		AttemptedConnections: 4,
	}
	applyPortScanOutcome(results, []int{80, 443}, outcome, 25)

	if results[0].Status != "scanned" || results[0].AttemptedConnections != 2 || results[0].ExpectedConnections != 2 {
		t.Fatalf("unexpected first target attribution: %+v", results[0])
	}
	if results[1].Status != "scanned" || results[1].AttemptedConnections != 4 || results[1].ExpectedConnections != 4 {
		t.Fatalf("unexpected second target attribution: %+v", results[1])
	}
	if got := results[0].OpenPorts["192.0.2.10"]; len(got) != 1 || got[0] != 443 {
		t.Fatalf("shared IP result not attributed: %v", results[0].OpenPorts)
	}
}

func TestCDNHelpers(t *testing.T) {
	if !isLikelyCDNString("cloudflare edge") {
		t.Fatalf("expected cloudflare marker to be detected")
	}
	if isLikelyCDNString("internal-app") {
		t.Fatalf("unexpected CDN marker for normal text")
	}
	if !isLikelyCDNIP("104.16.1.2") {
		t.Fatalf("expected known cloudflare range to match")
	}
	if isLikelyCDNIP("127.0.0.1") {
		t.Fatalf("localhost should not be treated as CDN")
	}
}

func TestScanURLPorts_LocalhostScanned(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{Handler: mux}
	go func() {
		_ = srv.Serve(ln)
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	target := fmt.Sprintf("http://127.0.0.1:%d", port)

	app := NewMonitorAppService(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := app.ScanURLPorts(ctx, []string{target}, []int{port}, 1)
	if err != nil {
		t.Fatalf("ScanURLPorts failed: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected one result, got %d", len(resp.Results))
	}
	if resp.Results[0].Status != "blocked" {
		t.Fatalf("expected status blocked, got %s", resp.Results[0].Status)
	}
}

func TestScanURLPorts_CDNExcludedByHeader(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Via", "cloudflare")
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{Handler: mux}
	go func() {
		_ = srv.Serve(ln)
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	target := fmt.Sprintf("http://127.0.0.1:%d", port)

	app := NewMonitorAppService(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := app.ScanURLPorts(ctx, []string{target}, []int{port}, 1)
	if err != nil {
		t.Fatalf("ScanURLPorts failed: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected one result, got %d", len(resp.Results))
	}
	if resp.Results[0].Status != "blocked" {
		t.Fatalf("expected status blocked, got %s", resp.Results[0].Status)
	}
}
