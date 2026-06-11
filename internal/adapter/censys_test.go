package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/unimap/project/internal/model"
)

// ===== CensysAdapter: Constructor, Name, IsWebOnly =====

func TestCensysAdapter_Name(t *testing.T) {
	a := NewCensysAdapter("https://search.censys.io", "id", "secret", 3, 30*time.Second)
	if got := a.Name(); got != "censys" {
		t.Errorf("Name() = %q, want %q", got, "censys")
	}
}

func TestCensysAdapter_IsWebOnly(t *testing.T) {
	a := NewCensysAdapter("https://search.censys.io", "id", "secret", 3, 30*time.Second)
	if a.IsWebOnly() {
		t.Error("expected IsWebOnly() = false")
	}
}

// ===== censysQuote =====

func TestCensysQuote(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want string
	}{
		{"empty", "", ""},
		{"simple number", "80", "80"},
		{"ip address", "1.2.3.4", "1.2.3.4"},
		{"country code", "US", "US"},
		{"with slash", "/api/v1", "/api/v1"},
		{"with space", "Beijing University", `"Beijing University"`},
		{"with double quote", `hello"world`, `"hello\"world"`},
		{"special chars", "Cisco ASA", `"Cisco ASA"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := censysQuote(tt.val)
			if got != tt.want {
				t.Errorf("censysQuote(%q) = %q, want %q", tt.val, got, tt.want)
			}
		})
	}
}

// ===== CensysAdapter: Translate =====

func TestCensysAdapter_Translate(t *testing.T) {
	a := NewCensysAdapter("https://search.censys.io", "id", "secret", 3, 30*time.Second)

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
			name: "simple condition",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "port",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "80"},
				},
			}},
			want: `services.port:80`,
		},
		{
			name: "not equal uses NOT prefix",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "country",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "!="},
					{Type: "value", Value: "CN"},
				},
			}},
			want: `NOT location.country_code:CN`,
		},
		{
			name: "AND logical with parentheses",
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
			want: `(services.port:80 AND ip:1.2.3.4)`,
		},
		{
			name: "OR logical native support",
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
			want: `(services.port:80 OR services.port:443)`,
		},
		{
			name: "IN operator expands to OR",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "port",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "IN"},
					{Type: "value", Value: "80,443,8080"},
				},
			}},
			want: `(services.port:80 OR services.port:443 OR services.port:8080)`,
		},
		{
			name: "greater than",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "port",
				Children: []*model.UQLNode{
					{Type: "operator", Value: ">"},
					{Type: "value", Value: "80"},
				},
			}},
			want: `services.port:>80`,
		},
		{
			name: "less than or equal",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "port",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "<="},
					{Type: "value", Value: "443"},
				},
			}},
			want: `services.port:<=443`,
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
			want: `services.http.response.body:login`,
		},
		{
			name: "field mapping title",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "title",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "nginx"},
				},
			}},
			want: `services.http.response.html_title:nginx`,
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
			want: `services.http.response.headers.Server:nginx`,
		},
		{
			name: "field mapping protocol",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "protocol",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "HTTP"},
				},
			}},
			want: `services.service_name:HTTP`,
		},
		{
			name: "field mapping asn",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "asn",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "136800"},
				},
			}},
			want: `autonomous_system.asn:136800`,
		},
		{
			name: "field mapping org",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "org",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "Google"},
				},
			}},
			want: `autonomous_system.name:Google`,
		},
		{
			name: "field mapping isp maps to autonomous_system.name",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "isp",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "Cloudflare"},
				},
			}},
			want: `autonomous_system.name:Cloudflare`,
		},
		{
			name: "field mapping domain",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "domain",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "example.com"},
				},
			}},
			want: `dns.names:example.com`,
		},
		{
			name: "field mapping status_code",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "status_code",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "200"},
				},
			}},
			want: `services.http.response.status_code:200`,
		},
		{
			name: "field mapping os",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "os",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "Linux"},
				},
			}},
			want: `operating_system:Linux`,
		},
		{
			name: "field mapping app",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "app",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "Apache"},
				},
			}},
			want: `services.software.product:Apache`,
		},
		{
			name: "field mapping cert",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "cert",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "baidu.com"},
				},
			}},
			want: `services.tls.certificates.leaf.subject:baidu.com`,
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
			want: `custom_field:test`,
		},
		{
			name: "value with space gets quoted",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "org",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "Beijing University"},
				},
			}},
			want: `autonomous_system.name:"Beijing University"`,
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
			want: `((services.port:80 OR services.port:443) AND location.country_code:CN)`,
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

// ===== CensysAdapter: Search =====

func TestCensysAdapter_Search(t *testing.T) {
	t.Run("empty credentials", func(t *testing.T) {
		a := NewCensysAdapter("https://search.censys.io", "", "", 3, 30*time.Second)
		result, err := a.Search(context.Background(), "test", 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("expected error in result for empty credentials")
		}
	})

	t.Run("successful search", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify Basic Auth
			user, pass, ok := r.BasicAuth()
			if !ok || user != "testid" || pass != "testsecret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"result": {
					"hits": [
						{"ip": "1.2.3.4", "services": [{"port": 80, "service_name": "HTTP"}], "location": {"country_code": "US"}},
						{"ip": "5.6.7.8", "services": [{"port": 443, "service_name": "HTTPS"}], "location": {"country_code": "CN"}}
					],
					"total": 2,
					"links": {"next": "cursor123"}
				}
			}`))
		}))
		defer server.Close()

		a := NewCensysAdapter(server.URL, "testid", "testsecret", 3, 30*time.Second)
		result, err := a.Search(context.Background(), "services.port:80", 1, 10)
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
		if !result.HasMore {
			t.Error("expected HasMore = true (cursor present)")
		}
	})

	t.Run("no cursor means no more results", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"result": {
					"hits": [{"ip": "1.2.3.4", "services": []}],
					"total": 1,
					"links": {}
				}
			}`))
		}))
		defer server.Close()

		a := NewCensysAdapter(server.URL, "id", "secret", 3, 30*time.Second)
		result, err := a.Search(context.Background(), "test", 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.HasMore {
			t.Error("expected HasMore = false (no cursor)")
		}
	})

	t.Run("HTTP error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer server.Close()

		a := NewCensysAdapter(server.URL, "id", "secret", 3, 30*time.Second)
		result, err := a.Search(context.Background(), "test", 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("expected error result for HTTP 500")
		}
	})

	t.Run("page > 1 uses per_page*page approximation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			perPage := r.URL.Query().Get("per_page")
			if perPage != "20" {
				t.Errorf("expected per_page=20 for page 2 with pageSize 10, got %s", perPage)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"result": {
					"hits": [
						{"ip": "1.2.3.4", "services": []},
						{"ip": "5.6.7.8", "services": []}
					],
					"total": 2,
					"links": {}
				}
			}`))
		}))
		defer server.Close()

		a := NewCensysAdapter(server.URL, "id", "secret", 3, 30*time.Second)
		result, err := a.Search(context.Background(), "test", 2, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error != "" {
			t.Fatalf("expected success, got: %s", result.Error)
		}
	})
}

// ===== CensysAdapter: Normalize =====

func TestCensysAdapter_Normalize(t *testing.T) {
	a := NewCensysAdapter("https://search.censys.io", "id", "secret", 3, 30*time.Second)

	t.Run("empty result", func(t *testing.T) {
		results, err := a.Normalize(&model.EngineResult{RawData: []interface{}{}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 assets, got %d", len(results))
		}
	})

	t.Run("host with multiple services produces multiple assets", func(t *testing.T) {
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"ip": "1.2.3.4",
				"location": map[string]interface{}{
					"country_code": "US",
					"province":     "California",
					"city":         "San Francisco",
				},
				"autonomous_system": map[string]interface{}{
					"asn":  "AS13335",
					"name": "Cloudflare",
				},
				"dns": map[string]interface{}{
					"names": []interface{}{"example.com", "www.example.com"},
				},
				"services": []interface{}{
					map[string]interface{}{
						"port":         float64(80),
						"service_name": "HTTP",
						"http": map[string]interface{}{
							"response": map[string]interface{}{
								"html_title":  "Example",
								"status_code": float64(200),
								"body":        strings.Repeat("x", 500),
								"headers": map[string]interface{}{
									"Server": "nginx",
								},
							},
						},
					},
					map[string]interface{}{
						"port":         float64(443),
						"service_name": "HTTPS",
						"http": map[string]interface{}{
							"response": map[string]interface{}{
								"html_title":  "Secure Example",
								"status_code": float64(200),
							},
						},
					},
				},
			},
		}}
		assets, err := a.Normalize(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 2 {
			t.Fatalf("expected 2 assets (one per service), got %d", len(assets))
		}

		// First service: HTTP
		if assets[0].IP != "1.2.3.4" {
			t.Errorf("IP = %q, want %q", assets[0].IP, "1.2.3.4")
		}
		if assets[0].Port != 80 {
			t.Errorf("Port = %d, want 80", assets[0].Port)
		}
		if assets[0].Protocol != "HTTP" {
			t.Errorf("Protocol = %q, want %q", assets[0].Protocol, "HTTP")
		}
		if assets[0].Title != "Example" {
			t.Errorf("Title = %q, want %q", assets[0].Title, "Example")
		}
		if assets[0].Server != "nginx" {
			t.Errorf("Server = %q, want %q", assets[0].Server, "nginx")
		}
		if assets[0].StatusCode != 200 {
			t.Errorf("StatusCode = %d, want 200", assets[0].StatusCode)
		}
		if len(assets[0].BodySnippet) > 200 {
			t.Errorf("BodySnippet too long: %d chars", len(assets[0].BodySnippet))
		}
		if assets[0].CountryCode != "US" {
			t.Errorf("CountryCode = %q, want %q", assets[0].CountryCode, "US")
		}
		if assets[0].Region != "California" {
			t.Errorf("Region = %q, want %q", assets[0].Region, "California")
		}
		if assets[0].City != "San Francisco" {
			t.Errorf("City = %q, want %q", assets[0].City, "San Francisco")
		}
		if assets[0].ASN != "AS13335" {
			t.Errorf("ASN = %q, want %q", assets[0].ASN, "AS13335")
		}
		if assets[0].Org != "Cloudflare" {
			t.Errorf("Org = %q, want %q", assets[0].Org, "Cloudflare")
		}
		if assets[0].ISP != "Cloudflare" {
			t.Errorf("ISP = %q, want %q", assets[0].ISP, "Cloudflare")
		}
		if assets[0].Host != "example.com" {
			t.Errorf("Host = %q, want %q", assets[0].Host, "example.com")
		}

		// Second service: HTTPS
		if assets[1].Port != 443 {
			t.Errorf("Port = %d, want 443", assets[1].Port)
		}
		if assets[1].Title != "Secure Example" {
			t.Errorf("Title = %q, want %q", assets[1].Title, "Secure Example")
		}
	})

	t.Run("no services emits one asset with host info only", func(t *testing.T) {
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"ip": "10.0.0.1",
				"location": map[string]interface{}{
					"country_code": "CN",
				},
			},
		}}
		assets, err := a.Normalize(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 1 {
			t.Fatalf("expected 1 asset (no services), got %d", len(assets))
		}
		if assets[0].IP != "10.0.0.1" {
			t.Errorf("IP = %q, want %q", assets[0].IP, "10.0.0.1")
		}
		if assets[0].CountryCode != "CN" {
			t.Errorf("CountryCode = %q, want %q", assets[0].CountryCode, "CN")
		}
	})

	t.Run("no ip skipped", func(t *testing.T) {
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"services": []interface{}{
					map[string]interface{}{"port": float64(80)},
				},
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

	t.Run("port as int type", func(t *testing.T) {
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"ip": "1.2.3.4",
				"services": []interface{}{
					map[string]interface{}{"port": int(443)},
				},
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
				"ip": "1.2.3.4",
				"dns": map[string]interface{}{
					"names": []interface{}{"example.com"},
				},
				"services": []interface{}{
					map[string]interface{}{
						"port":         float64(443),
						"service_name": "HTTPS",
					},
				},
			},
		}}
		assets, err := a.Normalize(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(assets))
		}
		// Should use host in URL when available
		if !strings.Contains(assets[0].URL, "example.com") {
			t.Errorf("URL = %q, expected to contain 'example.com'", assets[0].URL)
		}
		if !strings.HasPrefix(assets[0].URL, "https://") {
			t.Errorf("URL = %q, expected https scheme", assets[0].URL)
		}
	})

	t.Run("URL construction without host uses IP", func(t *testing.T) {
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"ip": "1.2.3.4",
				"services": []interface{}{
					map[string]interface{}{
						"port":         float64(80),
						"service_name": "HTTP",
					},
				},
			},
		}}
		assets, err := a.Normalize(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(assets[0].URL, "http://1.2.3.4:") {
			t.Errorf("URL = %q, expected http://1.2.3.4:80", assets[0].URL)
		}
	})

	t.Run("empty services slice behaves like no services", func(t *testing.T) {
		result := &model.EngineResult{RawData: []interface{}{
			map[string]interface{}{
				"ip":       "1.2.3.4",
				"services": []interface{}{},
			},
		}}
		assets, err := a.Normalize(result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 1 {
			t.Fatalf("expected 1 asset (empty services), got %d", len(assets))
		}
	})
}

// ===== CensysAdapter: GetQuota =====

func TestCensysAdapter_GetQuota(t *testing.T) {
	t.Run("empty credentials", func(t *testing.T) {
		a := NewCensysAdapter("https://search.censys.io", "", "", 3, 30*time.Second)
		_, err := a.GetQuota()
		if err == nil {
			t.Error("expected error for empty credentials")
		}
	})

	t.Run("successful quota", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify Basic Auth
			user, pass, ok := r.BasicAuth()
			if !ok || user != "testid" || pass != "testsecret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"quota": {"used": 50, "total": 100, "remaining": 50}}`))
		}))
		defer server.Close()

		a := NewCensysAdapter(server.URL, "testid", "testsecret", 3, 30*time.Second)
		quota, err := a.GetQuota()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if quota == nil {
			t.Fatal("expected quota info, got nil")
		}
		if quota.Total != 100 {
			t.Errorf("Total = %d, want 100", quota.Total)
		}
		if quota.Remaining != 50 {
			t.Errorf("Remaining = %d, want 50", quota.Remaining)
		}
		if quota.Used != 50 {
			t.Errorf("Used = %d, want 50", quota.Used)
		}
		if quota.Unit != "queries" {
			t.Errorf("Unit = %q, want %q", quota.Unit, "queries")
		}
	})

	t.Run("json api error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Invalid API credentials"}`))
		}))
		defer server.Close()

		a := NewCensysAdapter(server.URL, "id", "secret", 3, 30*time.Second)
		_, err := a.GetQuota()
		if err == nil || !strings.Contains(err.Error(), "Invalid API credentials") {
			t.Fatalf("expected Censys API error, got %v", err)
		}
	})
}

// ===== CensysAdapterWebOnly =====

func TestNewCensysAdapterWebOnly(t *testing.T) {
	a := NewCensysAdapterWebOnly()
	if a == nil {
		t.Fatal("expected non-nil adapter")
	}
	if !a.IsWebOnly() {
		t.Error("expected IsWebOnly() = true")
	}
	if a.Name() != "censys" {
		t.Errorf("Name() = %q, want %q", a.Name(), "censys")
	}
}
