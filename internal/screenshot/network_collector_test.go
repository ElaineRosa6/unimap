package screenshot

import (
	"strings"
	"testing"
)

func TestParseZoomEyeNetworkResponse(t *testing.T) {
	body := []byte(`{
		"total": 2,
		"matches": [
			{
				"ip": "1.2.3.4",
				"portinfo.port": 80,
				"portinfo.service": "http",
				"title": "nginx",
				"domain": "example.com",
				"geoinfo.country.code": "US",
				"geoinfo.city": "New York",
				"organization": "Example Org",
				"asn": 12345
			},
			{
				"ip": "5.6.7.8",
				"portinfo.port": 443,
				"portinfo.service": "https",
				"title": "Apache",
				"hostname": "test.com",
				"geoinfo.country.code": "CN",
				"organization": "Test Org"
			}
		]
	}`)

	assets, total, err := parseZoomEyeNetworkResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(assets))
	}

	a := assets[0]
	if a.IP != "1.2.3.4" {
		t.Errorf("expected IP 1.2.3.4, got %s", a.IP)
	}
	if a.Port != 80 {
		t.Errorf("expected port 80, got %d", a.Port)
	}
	if a.Host != "example.com" {
		t.Errorf("expected host example.com, got %s", a.Host)
	}
	if a.CountryCode != "US" {
		t.Errorf("expected country US, got %s", a.CountryCode)
	}
	if a.Source != "zoomeye" {
		t.Errorf("expected source zoomeye, got %s", a.Source)
	}

	a2 := assets[1]
	if a2.Host != "test.com" {
		t.Errorf("expected host test.com, got %s", a2.Host)
	}
}

func TestParseZoomEyeNetworkResponse_ResultsFallback(t *testing.T) {
	body := []byte(`{
		"total": 1,
		"results": [
			{"ip": "8.8.8.8", "port": 53, "service": "dns", "domain": "dns.google", "title": "Google DNS"}
		]
	}`)

	assets, total, err := parseZoomEyeNetworkResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
	if assets[0].IP != "8.8.8.8" || assets[0].Port != 53 || assets[0].Host != "dns.google" {
		t.Fatalf("unexpected asset: %#v", assets[0])
	}
}

func TestParseHunterNetworkResponse(t *testing.T) {
	body := []byte(`{
		"code": 200,
		"message": "success",
		"data": {
			"total": 1,
			"arr": [
				{
					"ip": "10.0.0.1",
					"port": 8080,
					"domain": "internal.corp",
					"protocol": "http",
					"web_title": "Internal App",
					"status_code": 200,
					"header_server": "nginx/1.20",
					"country": "CN",
					"province": "Beijing",
					"city": "Beijing",
					"isp": "China Telecom",
					"as_org": "CT",
					"url": "http://internal.corp:8080",
					"asn": "AS4134"
				}
			]
		}
	}`)

	assets, total, err := parseHunterNetworkResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}

	a := assets[0]
	if a.IP != "10.0.0.1" {
		t.Errorf("expected IP 10.0.0.1, got %s", a.IP)
	}
	if a.Port != 8080 {
		t.Errorf("expected port 8080, got %d", a.Port)
	}
	if a.Server != "nginx/1.20" {
		t.Errorf("expected server nginx/1.20, got %s", a.Server)
	}
	if a.Source != "hunter" {
		t.Errorf("expected source hunter, got %s", a.Source)
	}
}

func TestParseHunterNetworkResponse_Error(t *testing.T) {
	body := []byte(`{"code": 401, "message": "auth failed", "data": null}`)

	_, _, err := parseHunterNetworkResponse(body)
	if err == nil {
		t.Fatal("expected error for auth failure")
	}
}

func TestParseQuakeNetworkResponse(t *testing.T) {
	body := []byte(`{
		"code": 0,
		"message": "success",
		"data": {
			"total": 1,
			"hits": [
				{
					"ip": "192.168.1.1",
					"port": 22,
					"hostname": "server.local",
					"transport": "tcp",
					"title": {"title": "SSH"},
					"location": {"country_cn": "中国", "province_cn": "北京", "city_cn": "北京", "country_code": "CN"},
					"autonomous_system": {"asn": "[REDACTED]", "name": "CMNET", "isp": "[REDACTED]"},
					"server": "OpenSSH"
				}
			]
		}
	}`)

	assets, total, err := parseQuakeNetworkResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}

	a := assets[0]
	if a.IP != "192.168.1.1" {
		t.Errorf("expected IP 192.168.1.1, got %s", a.IP)
	}
	if a.Port != 22 {
		t.Errorf("expected port 22, got %d", a.Port)
	}
	if a.Server != "OpenSSH" {
		t.Errorf("expected server OpenSSH, got %s", a.Server)
	}
	if a.Source != "quake" {
		t.Errorf("expected source quake, got %s", a.Source)
	}
}

func TestParseQuakeNetworkResponse_Error(t *testing.T) {
	body := []byte(`{"code": -1, "message": "unauthorized", "data": null}`)

	_, _, err := parseQuakeNetworkResponse(body)
	if err == nil {
		t.Fatal("expected error for unauthorized")
	}
}

func TestParseQuakeNetworkResponse_CurrentArrayShape(t *testing.T) {
	body := []byte(`{
		"code": 0,
		"message": "Successful.",
		"data": [{
			"ip": "203.0.113.10",
			"port": 443,
			"hostname": ["example.test"],
			"transport": "tcp",
			"service": {
				"name": "https",
				"http": {"title": "Example TLS Service"}
			},
			"location": {"country_code": "CN", "city_cn": "北京"},
			"asn": 64500,
			"org": "Example Org",
			"isp": "Example ISP"
		}],
		"meta": {"pagination": {"total": 123}}
	}`)
	assets, total, err := parseQuakeNetworkResponse(body)
	if err != nil {
		t.Fatalf("parse current Quake response: %v", err)
	}
	if total != 123 || len(assets) != 1 {
		t.Fatalf("total/assets = %d/%d, want 123/1", total, len(assets))
	}
	got := assets[0]
	if got.IP != "203.0.113.10" || got.Port != 443 || got.Host != "example.test" {
		t.Fatalf("unexpected asset: %#v", got)
	}
	if got.Title != "Example TLS Service" || got.ASN != "64500" {
		t.Fatalf("nested fields not normalized: %#v", got)
	}
}

func TestParseQuakeNetworkResponseDoesNotUseCountryNameAsCode(t *testing.T) {
	body := []byte(`{"code":0,"data":[{"ip":"203.0.113.10","port":80,"location":{"country_cn":"中国"}}]}`)
	assets, _, err := parseQuakeNetworkResponse(body)
	if err != nil || len(assets) != 1 {
		t.Fatalf("parse Quake response: assets=%d err=%v", len(assets), err)
	}
	if assets[0].CountryCode != "" {
		t.Fatalf("CountryCode = %q, want empty without a code field", assets[0].CountryCode)
	}
	if assets[0].Extra["country_name"] != "中国" {
		t.Fatalf("country name extra = %#v", assets[0].Extra)
	}
}

func TestIsL1Supported(t *testing.T) {
	tests := []struct {
		engine   string
		expected bool
	}{
		{"zoomeye", true},
		{"ZoomEye", true},
		{"hunter", true},
		{"Hunter", true},
		{"quake", true},
		{"Quake", true},
		{"fofa", true},
		{"shodan", true},
		{"censys", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.engine, func(t *testing.T) {
			if got := IsL1Supported(tt.engine); got != tt.expected {
				t.Errorf("IsL1Supported(%q) = %v, want %v", tt.engine, got, tt.expected)
			}
		})
	}
}

func TestQuakeL1PatternMatchesCurrentSearchEndpoint(t *testing.T) {
	cfg := l1SearchAPIs["quake"]
	current := "https://quake.360.net/api/search/query_string/quake_service"
	if !strings.Contains(current, cfg.URLPattern) {
		t.Fatalf("Quake L1 pattern %q does not match %q", cfg.URLPattern, current)
	}
	if strings.Contains(cfg.URLPattern, "/visitor/") {
		t.Fatalf("Quake L1 pattern still uses retired visitor endpoint: %q", cfg.URLPattern)
	}
}

// ---------------------------------------------------------------------------
// FOFA L1 parser tests
// ---------------------------------------------------------------------------

func TestParseFofaNetworkResponse_ObjectFormat(t *testing.T) {
	body := []byte(`{
		"error": false,
		"size": 2,
		"results": [
			{"ip": "203.0.113.1", "port": "443", "protocol": "https", "title": "Example Corp", "country_code": "CN", "region": "Beijing", "city": "Beijing", "org": "ChinaNet", "server": "nginx"},
			{"ip": "198.51.100.2", "port": "80", "protocol": "http", "title": "Test Site", "country_code": "US", "host": "test.example.com"}
		]
	}`)
	assets, total, err := parseFofaNetworkResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(assets) != 2 {
		t.Fatalf("len(assets) = %d, want 2", len(assets))
	}
	a := assets[0]
	if a.IP != "203.0.113.1" || a.Port != 443 || a.Protocol != "https" {
		t.Errorf("asset[0] = %+v, want IP=203.0.113.1 Port=443 Protocol=https", a)
	}
	if a.Title != "Example Corp" || a.CountryCode != "CN" || a.Org != "ChinaNet" {
		t.Errorf("asset[0] metadata = %+v", a)
	}
	if a.Source != "fofa" {
		t.Errorf("asset[0].Source = %q, want fofa", a.Source)
	}
	b := assets[1]
	if b.Host != "test.example.com" || b.IP != "198.51.100.2" {
		t.Errorf("asset[1] = %+v, want Host=test.example.com IP=198.51.100.2", b)
	}
}

func TestParseFofaNetworkResponse_ArrayFormat(t *testing.T) {
	body := []byte(`{
		"error": false,
		"size": 3,
		"results": [
			["192.0.2.1", "8080", "http", "Dashboard", "CN", "Shanghai", "Aliyun"],
			["192.0.2.2", "22", "ssh", "", "US", "Virginia", "AWS"]
		]
	}`)
	assets, total, err := parseFofaNetworkResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(assets) != 2 {
		t.Fatalf("len(assets) = %d, want 2", len(assets))
	}
	if assets[0].IP != "192.0.2.1" || assets[0].Port != 8080 || assets[0].Title != "Dashboard" {
		t.Errorf("asset[0] = %+v", assets[0])
	}
	if assets[1].CountryCode != "US" || assets[1].Org != "AWS" {
		t.Errorf("asset[1] = %+v", assets[1])
	}
}

func TestParseFofaNetworkResponse_Error(t *testing.T) {
	body := []byte(`{"error": true, "errmsg": "query syntax error"}`)
	_, _, err := parseFofaNetworkResponse(body)
	if err == nil {
		t.Fatal("expected error for FOFA error response")
	}
	if !strings.Contains(err.Error(), "query syntax error") {
		t.Errorf("error = %q, want contains 'query syntax error'", err)
	}
}

func TestParseFofaNetworkResponse_Empty(t *testing.T) {
	body := []byte(`{"error": false, "size": 0, "results": []}`)
	assets, _, err := parseFofaNetworkResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assets) != 0 {
		t.Errorf("len(assets) = %d, want 0", len(assets))
	}
}

// ---------------------------------------------------------------------------
// Shodan L1 parser tests
// ---------------------------------------------------------------------------

func TestParseShodanNetworkResponse(t *testing.T) {
	body := []byte(`{
		"total": 1500,
		"matches": [
			{
				"ip_str": "203.0.113.10",
				"port": 443,
				"transport": "tcp",
				"hostnames": ["mail.example.com"],
				"location": {"country_code": "DE", "country_name": "Germany", "city": "Frankfurt", "region_code": "HE"},
				"http": {"title": "Mail Server", "server": "Apache/2.4", "host": "mail.example.com"},
				"org": "Deutsche Telekom",
				"isp": "DTAG",
				"product": "Apache",
				"version": "2.4"
			},
			{
				"ip_str": "198.51.100.20",
				"port": 22,
				"transport": "tcp",
				"hostnames": [],
				"location": {"country_code": "US", "country_name": "United States"},
				"http": {},
				"org": "Amazon",
				"isp": "AWS"
			}
		]
	}`)
	assets, total, err := parseShodanNetworkResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1500 {
		t.Errorf("total = %d, want 1500", total)
	}
	if len(assets) != 2 {
		t.Fatalf("len(assets) = %d, want 2", len(assets))
	}
	a := assets[0]
	if a.IP != "203.0.113.10" || a.Port != 443 || a.Protocol != "tcp" {
		t.Errorf("asset[0] = %+v, want IP=203.0.113.10 Port=443 Protocol=tcp", a)
	}
	if a.Host != "mail.example.com" || a.Title != "Mail Server" {
		t.Errorf("asset[0] host/title = %+v", a)
	}
	if a.CountryCode != "DE" || a.City != "Frankfurt" || a.Org != "Deutsche Telekom" {
		t.Errorf("asset[0] location = %+v", a)
	}
	if a.Source != "shodan" {
		t.Errorf("asset[0].Source = %q, want shodan", a.Source)
	}
	b := assets[1]
	if b.IP != "198.51.100.20" || b.Port != 22 {
		t.Errorf("asset[1] = %+v", b)
	}
	if b.Host != "" {
		t.Errorf("asset[1].Host = %q, want empty (no hostnames)", b.Host)
	}
	if b.Org != "Amazon" {
		t.Errorf("asset[1].Org = %q, want Amazon", b.Org)
	}
}

func TestParseShodanNetworkResponse_Error(t *testing.T) {
	body := []byte(`{"error": "Invalid API key", "total": 0, "matches": []}`)
	_, _, err := parseShodanNetworkResponse(body)
	if err == nil {
		t.Fatal("expected error for Shodan error response")
	}
	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("error = %q, want contains 'Invalid API key'", err)
	}
}

func TestParseShodanNetworkResponse_Empty(t *testing.T) {
	body := []byte(`{"total": 0, "matches": []}`)
	assets, total, err := parseShodanNetworkResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 || len(assets) != 0 {
		t.Errorf("total=%d assets=%d, want 0/0", total, len(assets))
	}
}

// ---------------------------------------------------------------------------
// L1 support matrix
// ---------------------------------------------------------------------------

func TestIsL1Supported_AllStableEngines(t *testing.T) {
	engines := []string{"fofa", "hunter", "zoomeye", "quake", "shodan"}
	for _, e := range engines {
		if !IsL1Supported(e) {
			t.Errorf("IsL1Supported(%q) = false, want true", e)
		}
	}
	if IsL1Supported("censys") {
		t.Error("IsL1Supported(censys) = true, want false (not yet implemented)")
	}
	if IsL1Supported("daydaymap") {
		t.Error("IsL1Supported(daydaymap) = true, want false (not yet implemented)")
	}
}
