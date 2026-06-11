package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/unimap/project/internal/model"
)

// ===== GreyNoiseAdapter =====

func TestGreyNoiseAdapter_Name(t *testing.T) {
	a := NewGreyNoiseAdapter("https://api.greynoise.io", "key", 1, 30*time.Second)
	if got := a.Name(); got != "greynoise" {
		t.Errorf("Name() = %q, want %q", got, "greynoise")
	}
}

func TestGreyNoiseAdapter_IsWebOnly(t *testing.T) {
	a := NewGreyNoiseAdapter("https://api.greynoise.io", "key", 1, 30*time.Second)
	if a.IsWebOnly() {
		t.Error("expected IsWebOnly() = false")
	}
}

func TestGreyNoiseAdapter_Translate(t *testing.T) {
	a := NewGreyNoiseAdapter("https://api.greynoise.io", "key", 1, 30*time.Second)

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
		// --- 基本条件 ---
		{
			name: "ip",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "ip",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "1.1.1.1"},
				},
			}},
			want: `ip:1.1.1.1`,
		},
		{
			name: "classification",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "classification",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "malicious"},
				},
			}},
			want: `classification:malicious`,
		},
		{
			name: "class alias maps to classification",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "class",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "suspicious"},
				},
			}},
			want: `classification:suspicious`,
		},
		{
			name: "tag",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "tags",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "GPON Router Auth Bypass"},
				},
			}},
			want: `tags:"GPON Router Auth Bypass"`,
		},
		{
			name: "org",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "org",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "Shodan"},
				},
			}},
			want: `metadata.organization:Shodan`,
		},
		{
			name: "os",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "os",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "Linux"},
				},
			}},
			want: `metadata.os:Linux`,
		},
		{
			name: "country",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "country",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "US"},
				},
			}},
			want: `metadata.country:US`,
		},
		{
			name: "port",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "port",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "80"},
				},
			}},
			want: `raw_data.scan.port:80`,
		},
		{
			name: "noise true",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "noise",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "true"},
				},
			}},
			want: `noise:true`,
		},
		{
			name: "riot true",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "riot",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "true"},
				},
			}},
			want: `riot:true`,
		},
		{
			name: "vpn_service",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "vpn",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "NordVPN"},
				},
			}},
			want: `vpn_service:NordVPN`,
		},
		{
			name: "first_seen",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "first_seen",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "="},
					{Type: "value", Value: "2024-01-01"},
				},
			}},
			want: `first_seen:2024-01-01`,
		},
		// --- NOT 操作符 ---
		{
			name: "NOT with !=",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "classification",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "!="},
					{Type: "value", Value: "benign"},
				},
			}},
			want: `-classification:benign`,
		},
		{
			name: "NOT with <>",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "classification",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "<>"},
					{Type: "value", Value: "unknown"},
				},
			}},
			want: `-classification:unknown`,
		},
		// --- IN 操作符 ---
		{
			name: "IN same field OR",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "classification",
				Children: []*model.UQLNode{
					{Type: "operator", Value: "IN"},
					{Type: "value", Value: "malicious,suspicious"},
				},
			}},
			want: `classification:malicious OR classification:suspicious`,
		},
		// --- 逻辑组合 ---
		{
			name: "AND combination",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type: "logical", Value: "AND",
				Children: []*model.UQLNode{
					{Type: "condition", Value: "classification", Children: []*model.UQLNode{
						{Type: "operator", Value: "="},
						{Type: "value", Value: "malicious"},
					}},
					{Type: "condition", Value: "port", Children: []*model.UQLNode{
						{Type: "operator", Value: "="},
						{Type: "value", Value: "80"},
					}},
				},
			}},
			want: `classification:malicious raw_data.scan.port:80`,
		},
		{
			name: "OR combination",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type: "logical", Value: "OR",
				Children: []*model.UQLNode{
					{Type: "condition", Value: "classification", Children: []*model.UQLNode{
						{Type: "operator", Value: "="},
						{Type: "value", Value: "malicious"},
					}},
					{Type: "condition", Value: "classification", Children: []*model.UQLNode{
						{Type: "operator", Value: "="},
						{Type: "value", Value: "suspicious"},
					}},
				},
			}},
			want: `classification:malicious OR classification:suspicious`,
		},
		// --- 比较操作符降级 ---
		{
			name: "comparison operator > degrades to equality",
			ast: &model.UQLAST{Root: &model.UQLNode{
				Type:  "condition",
				Value: "port",
				Children: []*model.UQLNode{
					{Type: "operator", Value: ">"},
					{Type: "value", Value: "80"},
				},
			}},
			want: `raw_data.scan.port:80`,
		},
		// --- unmapped field passthrough ---
		{
			name: "unmapped field passthrough",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := a.Translate(tt.ast)
			if (err != nil) != tt.wantErr {
				t.Errorf("Translate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Translate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGreyNoiseAdapter_Search(t *testing.T) {
	t.Run("empty api key", func(t *testing.T) {
		a := NewGreyNoiseAdapter("https://api.greynoise.io", "", 1, 30*time.Second)
		result, err := a.Search(context.Background(), "test", 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("expected error for empty api key")
		}
	})

	t.Run("successful search", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("key") != "testkey123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			resp := GreyNoiseSearchResponse{
				Complete: true,
				Count:    2,
				Query:    "classification:malicious",
				Data: []json.RawMessage{
					json.RawMessage(`{"ip":"1.1.1.1","classification":"malicious","noise":true,"riot":false,"metadata":{"organization":"TestOrg","country":"US"}}`),
					json.RawMessage(`{"ip":"2.2.2.2","classification":"malicious","noise":true,"riot":false,"metadata":{"organization":"AnotherOrg","country":"CN"}}`),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		a := NewGreyNoiseAdapter(server.URL, "testkey123", 1, 30*time.Second)
		result, err := a.Search(context.Background(), "classification:malicious", 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error != "" {
			t.Fatalf("search error: %s", result.Error)
		}
		if result.Total != 2 {
			t.Errorf("Total = %d, want 2", result.Total)
		}
		if len(result.RawData) != 2 {
			t.Errorf("RawData len = %d, want 2", len(result.RawData))
		}
	})

	t.Run("HTTP error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"message":"Forbidden"}`))
		}))
		defer server.Close()

		a := NewGreyNoiseAdapter(server.URL, "key", 1, 30*time.Second)
		result, err := a.Search(context.Background(), "test", 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("expected error for HTTP 403")
		}
	})

	t.Run("API error message", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"message":"Invalid query","count":0,"data":[]}`))
		}))
		defer server.Close()

		a := NewGreyNoiseAdapter(server.URL, "key", 1, 30*time.Second)
		result, err := a.Search(context.Background(), "bad query", 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("expected error for API error message")
		}
	})

	t.Run("server connection error", func(t *testing.T) {
		a := NewGreyNoiseAdapter("http://localhost:1", "key", 1, 30*time.Second)
		result, err := a.Search(context.Background(), "test", 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("expected error for connection failure")
		}
	})
}

func TestGreyNoiseAdapter_Normalize(t *testing.T) {
	a := NewGreyNoiseAdapter("https://api.greynoise.io", "key", 1, 30*time.Second)

	t.Run("empty result", func(t *testing.T) {
		assets, err := a.Normalize(&model.EngineResult{RawData: []interface{}{}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 0 {
			t.Errorf("expected 0 assets, got %d", len(assets))
		}
	})

	t.Run("nil result", func(t *testing.T) {
		assets, err := a.Normalize(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 0 {
			t.Errorf("expected 0 assets, got %d", len(assets))
		}
	})

	t.Run("malicious IP with metadata", func(t *testing.T) {
		raw := &model.EngineResult{
			RawData: []interface{}{
				map[string]interface{}{
					"ip":             "1.1.1.1",
					"classification": "malicious",
					"noise":          true,
					"riot":           false,
					"spoofable":      false,
					"tags":           []interface{}{"GPON Router Auth Bypass", "SSH Brute-Force"},
					"metadata": map[string]interface{}{
						"organization": "Shodan",
						"country":      "US",
						"city":         "New York",
						"asn":          "AS13335",
						"os":           "Linux",
					},
					"raw_data": map[string]interface{}{
						"scan": map[string]interface{}{
							"port":     float64(80),
							"protocol": "HTTP",
						},
					},
				},
			},
		}

		assets, err := a.Normalize(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(assets))
		}

		asset := assets[0]
		if asset.IP != "1.1.1.1" {
			t.Errorf("IP = %q, want %q", asset.IP, "1.1.1.1")
		}
		if asset.Port != 80 {
			t.Errorf("Port = %d, want 80", asset.Port)
		}
		if asset.Protocol != "HTTP" {
			t.Errorf("Protocol = %q, want %q", asset.Protocol, "HTTP")
		}
		if asset.Org != "Shodan" {
			t.Errorf("Org = %q, want %q", asset.Org, "Shodan")
		}
		if asset.CountryCode != "US" {
			t.Errorf("CountryCode = %q, want %q", asset.CountryCode, "US")
		}
		if asset.City != "New York" {
			t.Errorf("City = %q, want %q", asset.City, "New York")
		}
		if asset.ASN != "AS13335" {
			t.Errorf("ASN = %q, want %q", asset.ASN, "AS13335")
		}
		if asset.Server != "Linux" {
			t.Errorf("Server = %q, want %q", asset.Server, "Linux")
		}
		if !strings.Contains(asset.Title, "malicious") {
			t.Errorf("Title should contain classification, got %q", asset.Title)
		}
		if !strings.Contains(asset.Title, "GPON Router Auth Bypass") {
			t.Errorf("Title should contain tags, got %q", asset.Title)
		}
		if asset.URL == "" {
			t.Error("expected non-empty URL")
		}
		if !strings.Contains(asset.BodySnippet, "noise=true") {
			t.Errorf("BodySnippet should contain noise, got %q", asset.BodySnippet)
		}
	})

	t.Run("minimal IP only", func(t *testing.T) {
		raw := &model.EngineResult{
			RawData: []interface{}{
				map[string]interface{}{
					"ip":             "10.0.0.1",
					"classification": "unknown",
					"noise":          false,
					"riot":           false,
				},
			},
		}

		assets, err := a.Normalize(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(assets))
		}

		asset := assets[0]
		if asset.IP != "10.0.0.1" {
			t.Errorf("IP = %q, want %q", asset.IP, "10.0.0.1")
		}
		if asset.Port != 0 {
			t.Errorf("Port = %d, want 0", asset.Port)
		}
	})

	t.Run("benign riot IP", func(t *testing.T) {
		raw := &model.EngineResult{
			RawData: []interface{}{
				map[string]interface{}{
					"ip":             "8.8.8.8",
					"classification": "benign",
					"noise":          false,
					"riot":           true,
					"spoofable":      false,
					"metadata": map[string]interface{}{
						"organization": "Google LLC",
						"country":      "US",
					},
				},
			},
		}

		assets, err := a.Normalize(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(assets))
		}

		asset := assets[0]
		if asset.Org != "Google LLC" {
			t.Errorf("Org = %q, want %q", asset.Org, "Google LLC")
		}
		if !strings.Contains(asset.BodySnippet, "riot=true") {
			t.Errorf("BodySnippet should contain riot=true, got %q", asset.BodySnippet)
		}
	})

	t.Run("skip invalid item", func(t *testing.T) {
		raw := &model.EngineResult{
			RawData: []interface{}{
				"invalid",
				map[string]interface{}{
					"ip": "1.1.1.1",
				},
			},
		}

		assets, err := a.Normalize(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 1 {
			t.Errorf("expected 1 asset (invalid skipped), got %d", len(assets))
		}
	})

	t.Run("skip empty IP", func(t *testing.T) {
		raw := &model.EngineResult{
			RawData: []interface{}{
				map[string]interface{}{
					"classification": "malicious",
				},
			},
		}

		assets, err := a.Normalize(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 0 {
			t.Errorf("expected 0 assets (empty IP skipped), got %d", len(assets))
		}
	})

	t.Run("spoofable flag", func(t *testing.T) {
		raw := &model.EngineResult{
			RawData: []interface{}{
				map[string]interface{}{
					"ip":             "3.3.3.3",
					"classification": "suspicious",
					"noise":          true,
					"riot":           false,
					"spoofable":      true,
				},
			},
		}

		assets, err := a.Normalize(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(assets))
		}
		if !strings.Contains(assets[0].BodySnippet, "spoofable=true") {
			t.Errorf("BodySnippet should contain spoofable=true, got %q", assets[0].BodySnippet)
		}
	})

	t.Run("port 443 uses https", func(t *testing.T) {
		raw := &model.EngineResult{
			RawData: []interface{}{
				map[string]interface{}{
					"ip": "4.4.4.4",
					"raw_data": map[string]interface{}{
						"scan": map[string]interface{}{
							"port":     float64(443),
							"protocol": "HTTPS",
						},
					},
				},
			},
		}

		assets, err := a.Normalize(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(assets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(assets))
		}
		if !strings.HasPrefix(strings.ToLower(assets[0].URL), "https://") {
			t.Errorf("URL should start with https://, got %q", assets[0].URL)
		}
	})
}

func TestGreyNoiseAdapter_GetQuota(t *testing.T) {
	t.Run("empty api key", func(t *testing.T) {
		a := NewGreyNoiseAdapter("https://api.greynoise.io", "", 1, 30*time.Second)
		_, err := a.GetQuota()
		if err == nil {
			t.Error("expected error for empty api key")
		}
	})

	t.Run("successful quota", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"queries_remaining":900,"queries_used":100,"queries_total":1000,"rate_limit":"50/day"}`))
		}))
		defer server.Close()

		a := NewGreyNoiseAdapter(server.URL, "testkey", 1, 30*time.Second)
		quota, err := a.GetQuota()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if quota.Remaining != 900 {
			t.Errorf("Remaining = %d, want 900", quota.Remaining)
		}
		if quota.Total != 1000 {
			t.Errorf("Total = %d, want 1000", quota.Total)
		}
		if quota.Used != 100 {
			t.Errorf("Used = %d, want 100", quota.Used)
		}
		if quota.Unit != "queries" {
			t.Errorf("Unit = %q, want %q", quota.Unit, "queries")
		}
	})

	t.Run("HTTP error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		a := NewGreyNoiseAdapter(server.URL, "key", 1, 30*time.Second)
		_, err := a.GetQuota()
		if err == nil {
			t.Error("expected error for HTTP 401")
		}
	})
}

// ===== GreyNoiseAdapterWebOnly =====

func TestNewGreyNoiseAdapterWebOnly(t *testing.T) {
	a := NewGreyNoiseAdapterWebOnly()
	if a == nil {
		t.Fatal("expected non-nil adapter")
	}
	if a.Name() != "greynoise" {
		t.Errorf("Name() = %q, want %q", a.Name(), "greynoise")
	}
	if !a.IsWebOnly() {
		t.Error("expected IsWebOnly() = true")
	}
}
