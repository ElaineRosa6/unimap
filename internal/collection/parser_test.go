package collection

import (
	"testing"

	"github.com/unimap/project/internal/model"
)

func TestExtractPortFromHost(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantPort int
		wantHost string
	}{
		{"standard host:port", "1.2.3.4:8080", 8080, "1.2.3.4"},
		{"standard domain:port", "example.com:443", 443, "example.com"},
		{"no port", "1.2.3.4", 0, "1.2.3.4"},
		{"empty string", "", 0, ""},
		{"colon only", ":", 0, ":"},
		{"port zero", "1.2.3.4:0", 0, "1.2.3.4:0"},
		{"port 65535", "1.2.3.4:65535", 65535, "1.2.3.4"},
		{"port too large", "1.2.3.4:70000", 0, "1.2.3.4:70000"},
		{"non-numeric port", "1.2.3.4:abc", 0, "1.2.3.4:abc"},
		{"bracketed ipv6 with port", "[2001:db8::1]:443", 443, "2001:db8::1"},
		{"bare ipv6 no port", "2001:db8::1", 0, "2001:db8::1"},
		{"bare ipv6 ending with digits", "2001:db8::1:443", 0, "2001:db8::1:443"},
		{"bracketed ipv6 non-numeric port", "[2001:db8::1]:abc", 0, "[2001:db8::1]:abc"},
		{"multiple colons non-standard", "a:b:8080", 0, "a:b:8080"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPort, gotHost := extractPortFromHost(tt.input)
			if gotPort != tt.wantPort || gotHost != tt.wantHost {
				t.Errorf("extractPortFromHost(%q) = (%d, %q), want (%d, %q)",
					tt.input, gotPort, gotHost, tt.wantPort, tt.wantHost)
			}
		})
	}
}

func TestDefaultPortForProtocol(t *testing.T) {
	tests := []struct {
		proto string
		want  int
	}{
		{"http", 80},
		{"HTTP", 80},
		{"http/server", 80},
		{"https", 443},
		{"ssl/http", 443},
		{"tls", 443},
		{"ssh", 22},
		{"ftp", 21},
		{"smtp", 25},
		{"smtps", 25},
		{"pop3", 110},
		{"imap", 143},
		{"mysql", 3306},
		{"rdp", 3389},
		{"ms-wbt-server", 3389},
		{"smb", 445},
		{"dns", 53},
		{"redis", 6379},
		{"unknown", 0},
		{"", 0},
		{"  http  ", 80},
	}
	for _, tt := range tests {
		t.Run(tt.proto, func(t *testing.T) {
			if got := defaultPortForProtocol(tt.proto); got != tt.want {
				t.Errorf("defaultPortForProtocol(%q) = %d, want %d", tt.proto, got, tt.want)
			}
		})
	}
}

func TestParseIntField(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
		key  string
		want int
	}{
		{"float64", map[string]interface{}{"port": float64(8080)}, "port", 8080},
		{"string", map[string]interface{}{"port": "443"}, "port", 443},
		{"string with spaces", map[string]interface{}{"port": "  80  "}, "port", 80},
		{"invalid string", map[string]interface{}{"port": "abc"}, "port", 0},
		{"missing key", map[string]interface{}{}, "port", 0},
		{"zero", map[string]interface{}{"port": float64(0)}, "port", 0},
		{"negative", map[string]interface{}{"port": float64(-1)}, "port", -1},
		{"bool", map[string]interface{}{"port": true}, "port", 0},
		{"nil", map[string]interface{}{"port": nil}, "port", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseIntField(tt.data, tt.key); got != tt.want {
				t.Errorf("ParseIntField(%v, %q) = %d, want %d", tt.data, tt.key, got, tt.want)
			}
		})
	}
}

func TestExtractExtraFields(t *testing.T) {
	t.Run("known fields filtered", func(t *testing.T) {
		item := map[string]interface{}{
			"ip":     "1.2.3.4",
			"port":   float64(80),
			"title":  "test",
			"custom": "value",
			"extra2": float64(42),
		}
		extra := ExtractExtraFields(item)
		if extra == nil {
			t.Fatal("expected non-nil extra")
		}
		if _, ok := extra["ip"]; ok {
			t.Error("ip should be filtered")
		}
		if extra["custom"] != "value" {
			t.Errorf("custom = %v, want 'value'", extra["custom"])
		}
		if extra["extra2"] != float64(42) {
			t.Errorf("extra2 = %v, want 42", extra["extra2"])
		}
	})

	t.Run("all known fields returns nil", func(t *testing.T) {
		item := map[string]interface{}{
			"ip": "1.2.3.4", "port": float64(80), "title": "t",
		}
		extra := ExtractExtraFields(item)
		if extra != nil {
			t.Errorf("expected nil, got %v", extra)
		}
	})

	t.Run("empty item returns nil", func(t *testing.T) {
		extra := ExtractExtraFields(map[string]interface{}{})
		if extra != nil {
			t.Errorf("expected nil, got %v", extra)
		}
	})
}

func TestParseAssetItem(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		item := map[string]interface{}{
			"url":          "https://example.com",
			"title":        "Example",
			"ip":           "1.2.3.4",
			"port":         float64(443),
			"protocol":     "https",
			"host":         "example.com",
			"body_snippet": "snippet",
			"server":       "nginx",
			"status_code":  float64(200),
			"country_code": "US",
			"region":       "California",
			"city":         "San Francisco",
			"asn":          "AS1234",
			"org":          "Example Inc",
			"isp":          "Comcast",
		}
		asset := ParseAssetItem(item, "fofa")
		if asset.Source != "fofa" {
			t.Errorf("Source = %q, want fofa", asset.Source)
		}
		if asset.IP != "1.2.3.4" {
			t.Errorf("IP = %q", asset.IP)
		}
		if asset.Port != 443 {
			t.Errorf("Port = %d, want 443", asset.Port)
		}
		if asset.Host != "example.com" {
			t.Errorf("Host = %q", asset.Host)
		}
		if asset.Title != "Example" {
			t.Errorf("Title = %q", asset.Title)
		}
		if asset.CountryCode != "US" {
			t.Errorf("CountryCode = %q", asset.CountryCode)
		}
	})

	t.Run("port extracted from host:port", func(t *testing.T) {
		item := map[string]interface{}{
			"host": "1.2.3.4:8080",
		}
		asset := ParseAssetItem(item, "test")
		if asset.Port != 8080 {
			t.Errorf("Port = %d, want 8080", asset.Port)
		}
		if asset.Host != "1.2.3.4" {
			t.Errorf("Host = %q, want 1.2.3.4", asset.Host)
		}
	})

	t.Run("port extracted from ip:port", func(t *testing.T) {
		item := map[string]interface{}{
			"ip": "1.2.3.4:443",
		}
		asset := ParseAssetItem(item, "test")
		if asset.Port != 443 {
			t.Errorf("Port = %d, want 443", asset.Port)
		}
		if asset.IP != "1.2.3.4" {
			t.Errorf("IP = %q, want 1.2.3.4", asset.IP)
		}
	})

	t.Run("port from protocol string", func(t *testing.T) {
		item := map[string]interface{}{
			"protocol": "8081 http",
		}
		asset := ParseAssetItem(item, "test")
		if asset.Port != 8081 {
			t.Errorf("Port = %d, want 8081", asset.Port)
		}
	})

	t.Run("port from protocol name", func(t *testing.T) {
		item := map[string]interface{}{
			"protocol": "ssh",
		}
		asset := ParseAssetItem(item, "test")
		if asset.Port != 22 {
			t.Errorf("Port = %d, want 22", asset.Port)
		}
	})

	t.Run("banner fallback to body_snippet", func(t *testing.T) {
		item := map[string]interface{}{
			"banner": "HTTP/1.1 200 OK",
		}
		asset := ParseAssetItem(item, "test")
		if asset.BodySnippet != "HTTP/1.1 200 OK" {
			t.Errorf("BodySnippet = %q", asset.BodySnippet)
		}
	})

	t.Run("body_snippet preferred over banner", func(t *testing.T) {
		item := map[string]interface{}{
			"body_snippet": "preferred",
			"banner":       "ignored",
		}
		asset := ParseAssetItem(item, "test")
		if asset.BodySnippet != "preferred" {
			t.Errorf("BodySnippet = %q, want 'preferred'", asset.BodySnippet)
		}
	})

	t.Run("country fallback from country field", func(t *testing.T) {
		item := map[string]interface{}{
			"country": "CN",
		}
		asset := ParseAssetItem(item, "test")
		if asset.CountryCode != "CN" {
			t.Errorf("CountryCode = %q, want CN", asset.CountryCode)
		}
	})

	t.Run("country_code takes precedence over country", func(t *testing.T) {
		item := map[string]interface{}{
			"country_code": "US",
			"country":      "CN",
		}
		asset := ParseAssetItem(item, "test")
		if asset.CountryCode != "US" {
			t.Errorf("CountryCode = %q, want US", asset.CountryCode)
		}
	})

	t.Run("product fallback to title", func(t *testing.T) {
		item := map[string]interface{}{
			"product": "Apache",
		}
		asset := ParseAssetItem(item, "test")
		if asset.Title != "Apache" {
			t.Errorf("Title = %q, want Apache", asset.Title)
		}
	})

	t.Run("product does not override existing title", func(t *testing.T) {
		item := map[string]interface{}{
			"title":   "Existing",
			"product": "Apache",
		}
		asset := ParseAssetItem(item, "test")
		if asset.Title != "Existing" {
			t.Errorf("Title = %q, want Existing", asset.Title)
		}
	})

	t.Run("shodan timestamp mapped to last_seen", func(t *testing.T) {
		item := map[string]interface{}{
			"ip":        "1.2.3.4",
			"port":      float64(80),
			"timestamp": "2026-06-14T17:33:27.232948",
		}
		asset := ParseAssetItem(item, "shodan")
		if asset.LastSeen != "2026-06-14T17:33:27.232948" {
			t.Errorf("LastSeen = %q, want 2026-06-14T17:33:27.232948", asset.LastSeen)
		}
		// timestamp is a known field, must NOT appear in Extra
		if asset.Extra != nil {
			if _, ok := asset.Extra["timestamp"]; ok {
				t.Error("timestamp should not be in Extra (it is a known field mapped to LastSeen)")
			}
		}
	})

	t.Run("last_seen key preferred over timestamp", func(t *testing.T) {
		item := map[string]interface{}{
			"ip":        "1.2.3.4",
			"last_seen": "2026-06-15T10:00:00",
			"timestamp": "2026-06-14T17:33:27",
		}
		asset := ParseAssetItem(item, "shodan")
		if asset.LastSeen != "2026-06-15T10:00:00" {
			t.Errorf("LastSeen = %q, want 2026-06-15T10:00:00 (last_seen takes precedence)", asset.LastSeen)
		}
	})

	t.Run("unknown fields go to extra", func(t *testing.T) {
		item := map[string]interface{}{
			"ip":      "1.2.3.4",
			"custom1": "val1",
			"custom2": float64(42),
		}
		asset := ParseAssetItem(item, "test")
		if asset.Extra == nil {
			t.Fatal("expected non-nil Extra")
		}
		if asset.Extra["custom1"] != "val1" {
			t.Errorf("Extra[custom1] = %v", asset.Extra["custom1"])
		}
		if asset.Extra["custom2"] != float64(42) {
			t.Errorf("Extra[custom2] = %v", asset.Extra["custom2"])
		}
		if _, ok := asset.Extra["ip"]; ok {
			t.Error("ip should not be in Extra")
		}
	})

	t.Run("empty item", func(t *testing.T) {
		asset := ParseAssetItem(map[string]interface{}{}, "test")
		if asset.Source != "test" {
			t.Errorf("Source = %q", asset.Source)
		}
		if asset.Port != 0 {
			t.Errorf("Port = %d, want 0", asset.Port)
		}
	})
}

func TestParseStructuredCollectedData(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		assets, total, hasMore := ParseStructuredCollectedData(map[string]interface{}{}, "fofa")
		if len(assets) != 0 {
			t.Errorf("expected 0 assets, got %d", len(assets))
		}
		if total != 0 {
			t.Errorf("total = %d, want 0", total)
		}
		if hasMore {
			t.Error("hasMore = true, want false")
		}
	})

	t.Run("with items", func(t *testing.T) {
		data := map[string]interface{}{
			"total":    float64(100),
			"has_more": true,
			"items": []interface{}{
				map[string]interface{}{
					"ip":    "1.1.1.1",
					"port":  float64(80),
					"title": "test",
				},
				map[string]interface{}{
					"ip":   "2.2.2.2",
					"port": float64(443),
				},
			},
		}
		assets, total, hasMore := ParseStructuredCollectedData(data, "hunter")
		if total != 100 {
			t.Errorf("total = %d, want 100", total)
		}
		if !hasMore {
			t.Error("hasMore = false, want true")
		}
		if len(assets) != 2 {
			t.Fatalf("expected 2 assets, got %d", len(assets))
		}
		if assets[0].IP != "1.1.1.1" || assets[0].Port != 80 {
			t.Errorf("asset[0] = %+v", assets[0])
		}
		if assets[1].IP != "2.2.2.2" || assets[1].Port != 443 {
			t.Errorf("asset[1] = %+v", assets[1])
		}
	})

	t.Run("missing items key", func(t *testing.T) {
		data := map[string]interface{}{
			"total": float64(10),
		}
		assets, total, _ := ParseStructuredCollectedData(data, "test")
		if len(assets) != 0 || total != 10 {
			t.Errorf("got %d assets, total %d", len(assets), total)
		}
	})

	t.Run("malformed items skipped", func(t *testing.T) {
		data := map[string]interface{}{
			"items": []interface{}{
				"not-a-map",
				float64(42),
				map[string]interface{}{"ip": "3.3.3.3"},
			},
		}
		assets, _, _ := ParseStructuredCollectedData(data, "test")
		if len(assets) != 1 {
			t.Errorf("expected 1 asset (only valid item), got %d", len(assets))
		}
	})

	t.Run("items not a slice", func(t *testing.T) {
		data := map[string]interface{}{
			"items": "not-a-slice",
		}
		assets, _, _ := ParseStructuredCollectedData(data, "test")
		if len(assets) != 0 {
			t.Errorf("expected 0 assets, got %d", len(assets))
		}
	})
}

func TestParseStructuredCollectedDataFromItems(t *testing.T) {
	t.Run("empty items", func(t *testing.T) {
		assets, total, hasMore := ParseStructuredCollectedDataFromItems(nil, "fofa", true)
		if len(assets) != 0 || total != 0 {
			t.Errorf("got %d assets, total %d", len(assets), total)
		}
		if !hasMore {
			t.Error("hasMore should be preserved as true even with empty items")
		}
	})

	t.Run("all fields mapped", func(t *testing.T) {
		items := []model.CollectedDataItem{
			{
				IP:          "1.2.3.4",
				Port:        80,
				Protocol:    "http",
				Host:        "example.com",
				URL:         "https://example.com",
				Title:       "Example",
				BodySnippet: "snippet",
				Server:      "nginx",
				StatusCode:  200,
				CountryCode: "US",
				Region:      "California",
				City:        "SF",
				ASN:         "AS1234",
				Org:         "Org",
				ISP:         "ISP",
			},
		}
		assets, total, hasMore := ParseStructuredCollectedDataFromItems(items, "shodan", false)
		if total != 1 || hasMore {
			t.Errorf("total=%d, hasMore=%v", total, hasMore)
		}
		a := assets[0]
		if a.IP != "1.2.3.4" || a.Port != 80 || a.Host != "example.com" {
			t.Errorf("basic fields: IP=%q Port=%d Host=%q", a.IP, a.Port, a.Host)
		}
		if a.CountryCode != "US" || a.Region != "California" || a.City != "SF" {
			t.Errorf("geo fields: CountryCode=%q Region=%q City=%q", a.CountryCode, a.Region, a.City)
		}
		if a.Source != "shodan" {
			t.Errorf("Source = %q, want shodan", a.Source)
		}
	})

	t.Run("product maps to title when empty", func(t *testing.T) {
		items := []model.CollectedDataItem{
			{IP: "1.1.1.1", Product: "Apache"},
		}
		assets, _, _ := ParseStructuredCollectedDataFromItems(items, "hunter", false)
		if assets[0].Title != "Apache" {
			t.Errorf("Title = %q, want Apache", assets[0].Title)
		}
	})

	t.Run("product does not override title", func(t *testing.T) {
		items := []model.CollectedDataItem{
			{IP: "1.1.1.1", Title: "Existing", Product: "Apache"},
		}
		assets, _, _ := ParseStructuredCollectedDataFromItems(items, "hunter", false)
		if assets[0].Title != "Existing" {
			t.Errorf("Title = %q, want Existing", assets[0].Title)
		}
	})

	t.Run("port from host:port", func(t *testing.T) {
		items := []model.CollectedDataItem{
			{Host: "1.2.3.4:8080"},
		}
		assets, _, _ := ParseStructuredCollectedDataFromItems(items, "test", false)
		if assets[0].Port != 8080 || assets[0].Host != "1.2.3.4" {
			t.Errorf("Port=%d Host=%q", assets[0].Port, assets[0].Host)
		}
	})

	t.Run("port from ip:port", func(t *testing.T) {
		items := []model.CollectedDataItem{
			{IP: "1.2.3.4:443"},
		}
		assets, _, _ := ParseStructuredCollectedDataFromItems(items, "test", false)
		if assets[0].Port != 443 || assets[0].IP != "1.2.3.4" {
			t.Errorf("Port=%d IP=%q", assets[0].Port, assets[0].IP)
		}
	})

	t.Run("port from protocol name", func(t *testing.T) {
		items := []model.CollectedDataItem{
			{Protocol: "ssh"},
		}
		assets, _, _ := ParseStructuredCollectedDataFromItems(items, "test", false)
		if assets[0].Port != 22 {
			t.Errorf("Port = %d, want 22", assets[0].Port)
		}
	})

	t.Run("has_more propagated", func(t *testing.T) {
		items := []model.CollectedDataItem{{IP: "1.1.1.1"}}
		_, _, hasMore := ParseStructuredCollectedDataFromItems(items, "test", true)
		if !hasMore {
			t.Error("hasMore should be true")
		}
	})

	t.Run("last_seen from collected item", func(t *testing.T) {
		items := []model.CollectedDataItem{
			{IP: "1.2.3.4", Port: 80, LastSeen: "2026-06-14T17:33:27"},
		}
		assets, _, _ := ParseStructuredCollectedDataFromItems(items, "shodan", false)
		if len(assets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(assets))
		}
		if assets[0].LastSeen != "2026-06-14T17:33:27" {
			t.Errorf("LastSeen = %q, want 2026-06-14T17:33:27", assets[0].LastSeen)
		}
		if assets[0].Source != "shodan" {
			t.Errorf("Source = %q, want shodan", assets[0].Source)
		}
	})
}

func TestNormalizeAssets_Hunter(t *testing.T) {
	assets := []model.UnifiedAsset{
		{
			CountryCode: "成都市",
			Host:        "不看空域名 -",
			Title:       "Dovecot imapd企业办公 邮件系统 开源 Dovecot imapd",
		},
	}

	NormalizeAssets("hunter", assets)

	if assets[0].CountryCode != "中国" {
		t.Fatalf("CountryCode = %q, want 中国", assets[0].CountryCode)
	}
	if assets[0].Host != "" {
		t.Fatalf("Host = %q, want empty", assets[0].Host)
	}
	if assets[0].Title != "Dovecot imapd" {
		t.Fatalf("Title = %q, want Dovecot imapd", assets[0].Title)
	}
}

func TestNormalizeAssets_NonHunterNoop(t *testing.T) {
	assets := []model.UnifiedAsset{{
		CountryCode: "成都市",
		Host:        "不看空域名 -",
		Title:       "Dovecot imapd企业办公 邮件系统 开源 Dovecot imapd",
	}}

	NormalizeAssets("fofa", assets)

	if assets[0].CountryCode != "成都市" || assets[0].Host != "不看空域名 -" || assets[0].Title != "Dovecot imapd企业办公 邮件系统 开源 Dovecot imapd" {
		t.Fatalf("expected non-hunter assets to remain unchanged, got %+v", assets[0])
	}
}

func TestParseExtractedAssets(t *testing.T) {
	t.Run("basic extraction", func(t *testing.T) {
		raw := []map[string]interface{}{
			{
				"ip":       "1.1.1.1",
				"port":     float64(80),
				"protocol": "http",
				"host":     "example.com",
				"title":    "Test",
				"server":   "nginx",
				"country":  "US",
				"org":      "Org",
			},
		}
		assets := ParseExtractedAssets(raw, "fofa")
		if len(assets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(assets))
		}
		a := assets[0]
		if a.IP != "1.1.1.1" || a.Port != 80 || a.Host != "example.com" {
			t.Errorf("basic fields: IP=%q Port=%d Host=%q", a.IP, a.Port, a.Host)
		}
		if a.Source != "fofa" {
			t.Errorf("Source = %q", a.Source)
		}
	})

	t.Run("port from host:port", func(t *testing.T) {
		raw := []map[string]interface{}{
			{"host": "1.2.3.4:9090"},
		}
		assets := ParseExtractedAssets(raw, "test")
		if len(assets) != 1 || assets[0].Port != 9090 || assets[0].Host != "1.2.3.4" {
			t.Errorf("got %+v", assets)
		}
	})

	t.Run("port from ip:port", func(t *testing.T) {
		raw := []map[string]interface{}{
			{"ip": "1.2.3.4:443"},
		}
		assets := ParseExtractedAssets(raw, "test")
		if len(assets) != 1 || assets[0].Port != 443 || assets[0].IP != "1.2.3.4" {
			t.Errorf("got %+v", assets)
		}
	})

	t.Run("port from protocol", func(t *testing.T) {
		raw := []map[string]interface{}{
			{"ip": "1.1.1.1", "protocol": "https"},
		}
		assets := ParseExtractedAssets(raw, "test")
		if len(assets) != 1 || assets[0].Port != 443 {
			t.Errorf("got %+v", assets)
		}
	})

	t.Run("empty row skipped", func(t *testing.T) {
		raw := []map[string]interface{}{
			{},
			{"ip": "1.1.1.1"},
		}
		assets := ParseExtractedAssets(raw, "test")
		if len(assets) != 1 {
			t.Errorf("expected 1 asset, got %d", len(assets))
		}
	})

	t.Run("all geo fields", func(t *testing.T) {
		raw := []map[string]interface{}{
			{
				"ip":     "1.1.1.1",
				"region": "TestRegion",
				"city":   "TestCity",
				"asn":    "AS999",
				"org":    "TestOrg",
				"isp":    "TestISP",
				"source": "override",
			},
		}
		assets := ParseExtractedAssets(raw, "test")
		a := assets[0]
		if a.Region != "TestRegion" || a.City != "TestCity" || a.ASN != "AS999" {
			t.Errorf("geo: Region=%q City=%q ASN=%q", a.Region, a.City, a.ASN)
		}
		if a.Org != "TestOrg" || a.ISP != "TestISP" {
			t.Errorf("org/isp: Org=%q ISP=%q", a.Org, a.ISP)
		}
		if a.Source != "override" {
			t.Errorf("Source = %q, want override", a.Source)
		}
	})

	t.Run("timestamp and banner mapped", func(t *testing.T) {
		raw := []map[string]interface{}{
			{
				"ip":        "1.2.3.4",
				"port":      float64(80),
				"timestamp": "2026-06-14T17:33:27",
				"banner":    "+OK Dovecot ready.",
			},
		}
		assets := ParseExtractedAssets(raw, "shodan")
		if len(assets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(assets))
		}
		a := assets[0]
		if a.LastSeen != "2026-06-14T17:33:27" {
			t.Errorf("LastSeen = %q, want 2026-06-14T17:33:27", a.LastSeen)
		}
		if a.BodySnippet != "+OK Dovecot ready." {
			t.Errorf("BodySnippet = %q, want +OK Dovecot ready.", a.BodySnippet)
		}
	})

	t.Run("body_snippet preferred over banner", func(t *testing.T) {
		raw := []map[string]interface{}{
			{
				"ip":           "1.2.3.4",
				"port":         float64(80),
				"body_snippet": "HTTP/1.1 200 OK",
				"banner":       "fallback banner",
			},
		}
		assets := ParseExtractedAssets(raw, "test")
		if assets[0].BodySnippet != "HTTP/1.1 200 OK" {
			t.Errorf("BodySnippet = %q, want HTTP/1.1 200 OK", assets[0].BodySnippet)
		}
	})
}
