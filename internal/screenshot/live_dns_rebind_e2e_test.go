//go:build live_dns_e2e

package screenshot

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/unimap/project/internal/utils/urlguard"
)

// TestLiveBrowserEgressRejectsDNSRebinding requires an explicitly authorized
// DNS test fixture. The fixture must:
//   - reset the target hostname to a public service and zero the private sink;
//   - flip that same hostname to a private/loopback sink;
//   - expose {"hits":N} for the private sink through a separate public HTTPS
//     control endpoint.
//
// Control credentials and response bodies are never logged.
func TestLiveBrowserEgressRejectsDNSRebinding(t *testing.T) {
	targetURL := requireLiveDNSURL(t, "UNIMAP_LIVE_DNS_TARGET_URL", []string{"http", "https"})
	resetURL := requireLiveDNSURL(t, "UNIMAP_LIVE_DNS_RESET_URL", []string{"https"})
	flipURL := requireLiveDNSURL(t, "UNIMAP_LIVE_DNS_FLIP_URL", []string{"https"})
	hitsURL := requireLiveDNSURL(t, "UNIMAP_LIVE_DNS_PRIVATE_HITS_URL", []string{"https"})
	controlToken := strings.TrimSpace(os.Getenv("UNIMAP_LIVE_DNS_CONTROL_TOKEN"))
	if controlToken == "" {
		t.Fatal("UNIMAP_LIVE_DNS_CONTROL_TOKEN is required")
	}

	controlClient := urlguard.SafeHTTPClient(urlguard.CheckOptions{}, 20*time.Second)
	postLiveDNSControl(t, controlClient, resetURL, controlToken)
	t.Cleanup(func() {
		postLiveDNSControl(t, controlClient, resetURL, controlToken)
	})

	target, err := url.Parse(targetURL)
	if err != nil {
		t.Fatalf("parse DNS target URL: %v", err)
	}
	targetAddress := liveDNSTargetAddress(target)
	waitForLiveDNSState(t, targetAddress, false, 120*time.Second)

	proxy, err := newBrowserEgressProxy(t.Context(), net.DefaultResolver)
	if err != nil {
		t.Fatalf("start guarded browser egress proxy: %v", err)
	}
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatalf("parse guarded proxy URL: %v", err)
	}
	transport := &http.Transport{
		Proxy:               http.ProxyURL(proxyURL),
		DisableKeepAlives:   true,
		TLSHandshakeTimeout: 15 * time.Second,
	}
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second}

	resp, err := client.Get(targetURL) // #nosec G107 -- explicit live acceptance target
	if err != nil {
		t.Fatalf("public DNS state did not connect through guarded proxy: %v", err)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		t.Fatalf("public DNS state returned status %d", resp.StatusCode)
	}

	beforeHits := readLiveDNSPrivateHits(t, controlClient, hitsURL, controlToken)
	postLiveDNSControl(t, controlClient, flipURL, controlToken)
	waitForLiveDNSState(t, targetAddress, true, 120*time.Second)

	resp, requestErr := client.Get(targetURL) // #nosec G107 -- explicit live acceptance target
	if requestErr == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
		_ = resp.Body.Close()
		if resp.StatusCode < 400 {
			t.Fatalf("restricted DNS state unexpectedly returned status %d", resp.StatusCode)
		}
	}

	afterHits := readLiveDNSPrivateHits(t, controlClient, hitsURL, controlToken)
	if afterHits != beforeHits {
		t.Fatalf("private sink hits changed from %d to %d; guarded proxy connected after DNS rebinding", beforeHits, afterHits)
	}
	t.Logf("LIVE_DNS_REBIND success public_connect=true restricted_connect=false private_hits_delta=0")
}

func requireLiveDNSURL(t *testing.T, envName string, schemes []string) string {
	t.Helper()
	rawURL := strings.TrimSpace(os.Getenv(envName))
	if rawURL == "" {
		t.Fatalf("%s is required", envName)
	}
	parsed, err := urlguard.Check(rawURL, urlguard.CheckOptions{AllowedSchemes: schemes})
	if err != nil {
		t.Fatalf("%s is invalid: %v", envName, err)
	}
	if urlguard.IsInternalHost(t.Context(), parsed.Hostname()) {
		t.Fatalf("%s initially resolves to an internal or restricted address", envName)
	}
	return parsed.String()
}

func liveDNSTargetAddress(target *url.URL) string {
	if target.Port() != "" {
		return net.JoinHostPort(target.Hostname(), target.Port())
	}
	if strings.EqualFold(target.Scheme, "https") {
		return net.JoinHostPort(target.Hostname(), "443")
	}
	return net.JoinHostPort(target.Hostname(), "80")
}

func waitForLiveDNSState(t *testing.T, address string, restricted bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		_, err := resolveBrowserDialTargets(t.Context(), net.DefaultResolver, address)
		lastErr = err
		if restricted {
			if err != nil && strings.Contains(strings.ToLower(err.Error()), "restricted") {
				return
			}
		} else if err == nil {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("DNS state did not become restricted=%t before timeout: %v", restricted, lastErr)
}

func postLiveDNSControl(t *testing.T, client *http.Client, endpoint, token string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, endpoint, nil)
	if err != nil {
		t.Fatalf("create DNS control request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("call DNS control endpoint: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("DNS control endpoint returned status %d", resp.StatusCode)
	}
}

func readLiveDNSPrivateHits(t *testing.T, client *http.Client, endpoint, token string) int64 {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatalf("create private-hit counter request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("read private-hit counter: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		t.Fatalf("private-hit counter returned status %d", resp.StatusCode)
	}
	var body struct {
		Hits json.Number `json:"hits"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 4096))
	decoder.UseNumber()
	if err := decoder.Decode(&body); err != nil {
		t.Fatalf("decode private-hit counter: %v", err)
	}
	hits, err := strconv.ParseInt(body.Hits.String(), 10, 64)
	if err != nil || hits < 0 {
		t.Fatalf("private-hit counter is invalid: %v", err)
	}
	return hits
}
