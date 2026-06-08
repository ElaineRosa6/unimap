package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/unimap/project/internal/model"
)

// ===== DayDayMapAdapter: Constructor, Name, IsWebOnly =====

func TestDayDayMapAdapter_Name(t *testing.T) {
	a := NewDayDayMapAdapter("https://www.daydaymap.com", "key", 3, 30*time.Second)
	if got := a.Name(); got != "daydaymap" {
		t.Errorf("Name() = %q, want %q", got, "daydaymap")
	}
}

func TestDayDayMapAdapter_IsWebOnly(t *testing.T) {
	a := NewDayDayMapAdapter("https://www.daydaymap.com", "key", 3, 30*time.Second)
	if a.IsWebOnly() {
		t.Error("expected IsWebOnly() = false")
	}
}

// ===== DayDayMapAdapter: Translate =====

func TestDayDayMapAdapter_Translate(t *testing.T) {
	a := NewDayDayMapAdapter("https://www.daydaymap.com", "key", 3, 30*time.Second)

	tests := []struct {
		name    string
		ast     *model.UQLAST
		want    string
		wantErr bool
	}{
		{
			name:    "nil AST",
			ast:     nil,
			wantErr: true,
		},
		{
			name:    "nil root",
			ast:     &model.UQLAST{Root: nil},
			wantErr: true,
		},
		{
			name: "simple condition port=80",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "port",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "80"},
				},
			}},
			want: `ip.port="80"`,
		},
		{
			name: "not equal country!=CN",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "country",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "!="},
					{Type: "value", Value: "CN"},
				},
			}},
			want: `ip.country!="CN"`,
		},
		{
			name: "AND logical",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "logical",
				Value: "AND",
				Children: []*model.UQLNode{
					{Type: "condition", Value: "port", Children: []*model.UQLNode{
						{Type: "operator", Value: "="},
						{Type: "value", Value: "80"},
					}},
					{Type: "condition", Value: "ip", Children: []*model.UQLNode{
						{Type: "operator", Value: "="},
						{Type: "value", Value: "1.2.3.4"},
					}},
				},
			}},
			want: `(ip.port="80" && ip="1.2.3.4")`,
		},
		{
			name: "OR logical",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "logical",
				Value: "OR",
				Children: []*model.UQLNode{
					{Type: "condition", Value: "port", Children: []*model.UQLNode{
						{Type: "operator", Value: "="},
						{Type: "value", Value: "80"},
					}},
					{Type: "condition", Value: "port", Children: []*model.UQLNode{
						{Type: "operator", Value: "="},
						{Type: "value", Value: "443"},
					}},
				},
			}},
			want: `(ip.port="80" || ip.port="443")`,
		},
		{
			name: "IN operator expands to OR",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "port",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "IN"},
					{Type: "value", Value: "80,443"},
				},
			}},
			want: `(ip.port="80" || ip.port="443")`,
		},
		{
			name: "field mapping body",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "body",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "login"},
				},
			}},
			want: `web.body="login"`,
		},
		{
			name: "field mapping title",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "title",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "admin"},
				},
			}},
			want: `web.title="admin"`,
		},
		{
			name: "field mapping server",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "server",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "nginx"},
				},
			}},
			want: `web.server="nginx"`,
		},
		{
			name: "field mapping cert.subject.cn",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "cert.subject.cn",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "baidu.com"},
				},
			}},
			want: `cert.subject.cn="baidu.com"`,
		},
		{
			name: "field mapping country",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "country",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "CN"},
				},
			}},
			want: `ip.country="CN"`,
		},
		{
			name: "unknown field passthrough",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "custom_field",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "test"},
				},
			}},
			want: `custom_field="test"`,
		},
		{
			name: "nested logical",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "logical",
				Value: "AND",
				Children: []*model.UQLNode{
					{Type: "logical", Value: "OR", Children: []*model.UQLNode{
						{Type: "condition", Value: "port", Children: []*model.UQLNode{
							{Type: "operator", Value: "="},
							{Type: "value", Value: "80"},
						}},
						{Type: "condition", Value: "port", Children: []*model.UQLNode{
							{Type: "operator", Value: "="},
							{Type: "value", Value: "443"},
						}},
					}},
					{Type: "condition", Value: "country", Children: []*model.UQLNode{
						{Type: "operator", Value: "="},
						{Type: "value", Value: "CN"},
					}},
				},
			}},
			want: `((ip.port="80" || ip.port="443") && ip.country="CN")`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := a.Translate(tt.ast)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Translate() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ===== DayDayMapAdapter: Search =====

func TestDayDayMapAdapter_Search(t *testing.T) {
	t.Run("empty api key", func(t *testing.T) {
		a := NewDayDayMapAdapter("https://www.daydaymap.com", "", 3, 30*time.Second)
		result, err := a.Search(context.Background(), "test", 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("expected error in result for empty api key")
		}
	})

	t.Run("successful search", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify apikey param
			if r.URL.Query().Get("apikey") != "testkey" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"code": 0,
				"message": "success",
				"data": [
					{"ip": "1.2.3.4", "port": 80, "protocol": "http", "domain": "example.com", "title": "Test", "country": "US"},
					{"ip": "5.6.7.8", "port": 443, "protocol": "https", "domain": "test.com", "title": "Secure", "country": "CN"}
				],
				"total": 2
			}`))
		}))
		defer server.Close()

		a := NewDayDayMapAdapter(server.URL, "testkey", 3, 30*time.Second)
		result, err := a.Search(context.Background(), "ip.port=\"80\"", 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error != "" {
			t.Fatalf("expected success, got: %s", result.Error)
		}
		if len(result.RawData) != 2 {
			t.Fatalf("expected 2 results, got %d", len(result.RawData))
		}
		if result.Total != 2 {
			t.Errorf("Total = %d, want 2", result.Total)
		}
		if result.HasMore {
			t.Error("expected HasMore = false")
		}
	})

	t.Run("has more when total > page*pageSize", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"code": 0,
				"message": "success",
				"data": [{"ip": "1.2.3.4", "port": 80}],
				"total": 100
			}`))
		}))
		defer server.Close()

		a := NewDayDayMapAdapter(server.URL, "testkey", 3, 30*time.Second)
		result, err := a.Search(context.Background(), "test", 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.HasMore {
			t.Error("expected HasMore = true (total 100 > page*pageSize 10)")
		}
	})

	t.Run("API error code", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code": 1001, "message": "Invalid API key", "data": [], "total": 0}`))
		}))
		defer server.Close()

		a := NewDayDayMapAdapter(server.URL, "testkey", 3, 30*time.Second)
		result, err := a.Search(context.Background(), "test", 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("expected error in result for API error code")
		}
	})

	t.Run("HTTP error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer server.Close()

		a := NewDayDayMapAdapter(server.URL, "testkey", 3, 30*time.Second)
		result, err := a.Search(context.Background(), "test", 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("expected error result for HTTP 500")
		}
	})
}

// ===== DayDayMapAdapter: Normalize =====

func TestDayDayMapAdapter_Normalize(t *testing.T) {
	a := NewDayDayMapAdapter("https://www.daydaymap.com", "key", 3, 30*time.Second)

	t.Run("empty result", func(t *testing.T) {
		results, err := a.Normalize(&model.EngineResult{RawData: []interface{}{}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 assets, got %d", len(results))
		}
	})

	t.Run("full fields", func(t *testing.T) {
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"ip":          "1.2.3.4",
				"port":        float64(80),
				"protocol":    "http",
				"domain":      "example.com",
				"title":       "Example",
				"server":      "nginx",
				"body":        "hello world",
				"status_code": float64(200),
				"country":     "US",
				"province":    "California",
				"city":        "San Francisco",
				"asn":         "AS13335",
				"org":         "Cloudflare",
				"isp":         "Cloudflare",
			},
		}}
		assets, err := a.Normalize(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(assets))
		}

		asset := assets[0]
		if asset.IP != "1.2.3.4" {
			t.Errorf("IP = %q, want %q", asset.IP, "1.2.3.4")
		}
		if asset.Port != 80 {
			t.Errorf("Port = %d, want 80", asset.Port)
		}
		if asset.Protocol != "http" {
			t.Errorf("Protocol = %q, want %q", asset.Protocol, "http")
		}
		if asset.Host != "example.com" {
			t.Errorf("Host = %q, want %q", asset.Host, "example.com")
		}
		if asset.Title != "Example" {
			t.Errorf("Title = %q, want %q", asset.Title, "Example")
		}
		if asset.Server != "nginx" {
			t.Errorf("Server = %q, want %q", asset.Server, "nginx")
		}
		if asset.BodySnippet != "hello world" {
			t.Errorf("BodySnippet = %q, want %q", asset.BodySnippet, "hello world")
		}
		if asset.StatusCode != 200 {
			t.Errorf("StatusCode = %d, want 200", asset.StatusCode)
		}
		if asset.CountryCode != "US" {
			t.Errorf("CountryCode = %q, want %q", asset.CountryCode, "US")
		}
		if asset.Region != "California" {
			t.Errorf("Region = %q, want %q", asset.Region, "California")
		}
		if asset.City != "San Francisco" {
			t.Errorf("City = %q, want %q", asset.City, "San Francisco")
		}
		if asset.ASN != "AS13335" {
			t.Errorf("ASN = %q, want %q", asset.ASN, "AS13335")
		}
		if asset.Org != "Cloudflare" {
			t.Errorf("Org = %q, want %q", asset.Org, "Cloudflare")
		}
		if asset.ISP != "Cloudflare" {
			t.Errorf("ISP = %q, want %q", asset.ISP, "Cloudflare")
		}
		if asset.Source != "daydaymap" {
			t.Errorf("Source = %q, want %q", asset.Source, "daydaymap")
		}
	})

	t.Run("no IP skipped", func(t *testing.T) {
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"port": float64(80),
				"host": "example.com",
			},
		}}
		assets, err := a.Normalize(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 0 {
			t.Errorf("expected 0 assets (no ip), got %d", len(assets))
		}
	})

	t.Run("body truncated to 200 chars", func(t *testing.T) {
		longBody := ""
		for i := 0; i < 500; i++ {
			longBody += "a"
		}
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"ip":   "1.2.3.4",
				"port": float64(80),
				"body": longBody,
			},
		}}
		assets, err := a.Normalize(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(assets))
		}
		if len(assets[0].BodySnippet) != 200 {
			t.Errorf("BodySnippet length = %d, want 200", len(assets[0].BodySnippet))
		}
	})

	t.Run("port as int type", func(t *testing.T) {
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"ip":   "1.2.3.4",
				"port": int(443),
			},
		}}
		assets, err := a.Normalize(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 1 || assets[0].Port != 443 {
			t.Errorf("Port = %d, want 443", assets[0].Port)
		}
	})

	t.Run("URL construction with host", func(t *testing.T) {
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"ip":       "1.2.3.4",
				"port":     float64(443),
				"protocol": "https",
				"domain":   "example.com",
			},
		}}
		assets, err := a.Normalize(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(assets))
		}
		if assets[0].URL != "https://example.com:443" {
			t.Errorf("URL = %q, want %q", assets[0].URL, "https://example.com:443")
		}
	})

	t.Run("URL construction without host uses IP", func(t *testing.T) {
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"ip":       "1.2.3.4",
				"port":     float64(80),
				"protocol": "http",
			},
		}}
		assets, err := a.Normalize(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if assets[0].URL != "http://1.2.3.4:80" {
			t.Errorf("URL = %q, want %q", assets[0].URL, "http://1.2.3.4:80")
		}
	})

	t.Run("default protocol for port 443", func(t *testing.T) {
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"ip":   "1.2.3.4",
				"port": float64(443),
			},
		}}
		assets, err := a.Normalize(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if assets[0].Protocol != "https" {
			t.Errorf("Protocol = %q, want %q", assets[0].Protocol, "https")
		}
	})

	t.Run("default protocol for other ports", func(t *testing.T) {
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"ip":   "1.2.3.4",
				"port": float64(8080),
			},
		}}
		assets, err := a.Normalize(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if assets[0].Protocol != "http" {
			t.Errorf("Protocol = %q, want %q", assets[0].Protocol, "http")
		}
	})

	t.Run("only host no port", func(t *testing.T) {
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"ip":     "1.2.3.4",
				"domain": "example.com",
			},
		}}
		assets, err := a.Normalize(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(assets))
		}
		if assets[0].Host != "example.com" {
			t.Errorf("Host = %q, want %q", assets[0].Host, "example.com")
		}
	})

	t.Run("only IP no port no host", func(t *testing.T) {
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"ip": "1.2.3.4",
			},
		}}
		assets, err := a.Normalize(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(assets))
		}
		if assets[0].IP != "1.2.3.4" {
			t.Errorf("IP = %q, want %q", assets[0].IP, "1.2.3.4")
		}
	})

	t.Run("status_code as int type", func(t *testing.T) {
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"ip":          "1.2.3.4",
				"port":        float64(80),
				"status_code": int(404),
			},
		}}
		assets, err := a.Normalize(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if assets[0].StatusCode != 404 {
			t.Errorf("StatusCode = %d, want 404", assets[0].StatusCode)
		}
	})

	t.Run("nil result", func(t *testing.T) {
		results, err := a.Normalize(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 assets for nil result, got %d", len(results))
		}
	})
}

// ===== DayDayMapAdapter: GetQuota =====

func TestDayDayMapAdapter_GetQuota(t *testing.T) {
	t.Run("empty api key", func(t *testing.T) {
		a := NewDayDayMapAdapter("https://www.daydaymap.com", "", 3, 30*time.Second)
		_, err := a.GetQuota()
		if err == nil {
			t.Error("expected error for empty api key")
		}
	})

	t.Run("successful quota", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("apikey") != "testkey" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"code": 0,
				"message": "success",
				"data": {
					"remain_quota": 500,
					"total_quota": 1000,
					"used_quota": 500
				}
			}`))
		}))
		defer server.Close()

		a := NewDayDayMapAdapter(server.URL, "testkey", 3, 30*time.Second)
		quota, err := a.GetQuota()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if quota == nil {
			t.Fatal("expected quota info, got nil")
		}
		if quota.Total != 1000 {
			t.Errorf("Total = %d, want 1000", quota.Total)
		}
		if quota.Remaining != 500 {
			t.Errorf("Remaining = %d, want 500", quota.Remaining)
		}
		if quota.Used != 500 {
			t.Errorf("Used = %d, want 500", quota.Used)
		}
		if quota.Unit != "queries" {
			t.Errorf("Unit = %q, want %q", quota.Unit, "queries")
		}
	})

	t.Run("API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code": 1001, "message": "Invalid API key", "data": {}}`))
		}))
		defer server.Close()

		a := NewDayDayMapAdapter(server.URL, "testkey", 3, 30*time.Second)
		_, err := a.GetQuota()
		if err == nil {
			t.Error("expected error for API error")
		}
	})
}

// ===== DayDayMapAdapterWebOnly =====

func TestNewDayDayMapAdapterWebOnly(t *testing.T) {
	a := NewDayDayMapAdapterWebOnly()
	if a == nil {
		t.Fatal("expected non-nil adapter")
	}
	if !a.IsWebOnly() {
		t.Error("expected IsWebOnly() = true")
	}
	if a.Name() != "daydaymap" {
		t.Errorf("Name() = %q, want %q", a.Name(), "daydaymap")
	}
}
