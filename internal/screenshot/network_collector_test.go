package screenshot

import (
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
					"autonomous_system": {"asn": "AS9808", "name": "CMNET", "isp": "China Mobile"},
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
		{"fofa", false},
		{"shodan", false},
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
