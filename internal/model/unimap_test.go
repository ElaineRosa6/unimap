package model

import (
	"encoding/json"
	"testing"
)

// TestUnifiedAsset_JSONRoundTrip 验证 UnifiedAsset 的 JSON 序列化/反序列化往返
func TestUnifiedAsset_JSONRoundTrip(t *testing.T) {
	original := UnifiedAsset{
		IP:          "192.168.1.1",
		Port:        8080,
		Protocol:    "http",
		Host:        "example.com",
		URL:         "http://example.com:8080",
		Title:       "测试页面",
		BodySnippet: "页面内容片段",
		Server:      "nginx/1.24",
		Headers:     map[string]string{"Content-Type": "text/html", "X-Custom": "value"},
		StatusCode:  200,
		CountryCode: "CN",
		Region:      "Beijing",
		City:        "Beijing",
		ASN:         "AS12345",
		Org:         "Test Org",
		ISP:         "Test ISP",
		LastSeen:    "2026-06-30",
		Source:      "fofa",
		Extra:       map[string]interface{}{"custom_field": "custom_value", "count": 42},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded UnifiedAsset
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// 逐字段验证
	if decoded.IP != original.IP {
		t.Errorf("IP: got %q, want %q", decoded.IP, original.IP)
	}
	if decoded.Port != original.Port {
		t.Errorf("Port: got %d, want %d", decoded.Port, original.Port)
	}
	if decoded.Protocol != original.Protocol {
		t.Errorf("Protocol: got %q, want %q", decoded.Protocol, original.Protocol)
	}
	if decoded.Host != original.Host {
		t.Errorf("Host: got %q, want %q", decoded.Host, original.Host)
	}
	if decoded.URL != original.URL {
		t.Errorf("URL: got %q, want %q", decoded.URL, original.URL)
	}
	if decoded.Title != original.Title {
		t.Errorf("Title: got %q, want %q", decoded.Title, original.Title)
	}
	if decoded.StatusCode != original.StatusCode {
		t.Errorf("StatusCode: got %d, want %d", decoded.StatusCode, original.StatusCode)
	}
	if decoded.CountryCode != original.CountryCode {
		t.Errorf("CountryCode: got %q, want %q", decoded.CountryCode, original.CountryCode)
	}
	if decoded.Source != original.Source {
		t.Errorf("Source: got %q, want %q", decoded.Source, original.Source)
	}
	if len(decoded.Headers) != len(original.Headers) {
		t.Errorf("Headers length: got %d, want %d", len(decoded.Headers), len(original.Headers))
	}
	if len(decoded.Extra) != len(original.Extra) {
		t.Errorf("Extra length: got %d, want %d", len(decoded.Extra), len(original.Extra))
	}
}

// TestUnifiedAsset_EmptyJSON 验证空 UnifiedAsset 的 JSON 往返
func TestUnifiedAsset_EmptyJSON(t *testing.T) {
	original := UnifiedAsset{}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded UnifiedAsset
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.IP != "" || decoded.Port != 0 || decoded.URL != "" {
		t.Errorf("Empty asset should round-trip to zero values, got %+v", decoded)
	}
}

// TestQueryAPIPayload_JSONRoundTrip 验证 QueryAPIPayload 的 JSON 序列化/反序列化
func TestQueryAPIPayload_JSONRoundTrip(t *testing.T) {
	original := QueryAPIPayload{
		Query:          "ip=1.2.3.4",
		Engines:        []string{"fofa", "shodan", "zoomeye"},
		Status:         "success",
		Results:        []UnifiedAsset{{IP: "1.2.3.4", Port: 80}},
		Total:          1,
		Page:           1,
		PageSize:       20,
		BrowserOutcome: "ok",
		BrowserAction:  "screenshot",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded QueryAPIPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Query != original.Query {
		t.Errorf("Query: got %q, want %q", decoded.Query, original.Query)
	}
	if decoded.Status != original.Status {
		t.Errorf("Status: got %q, want %q", decoded.Status, original.Status)
	}
	if decoded.Total != original.Total {
		t.Errorf("Total: got %d, want %d", decoded.Total, original.Total)
	}
	if len(decoded.Engines) != len(original.Engines) {
		t.Errorf("Engines length: got %d, want %d", len(decoded.Engines), len(original.Engines))
	}
}

// TestAPIResponse_Omitempty 验证 APIResponse 的 omitempty 标签行为
func TestAPIResponse_Omitempty(t *testing.T) {
	// success=true, 无 error/message/data → 只应有 success 字段
	resp := APIResponse{Success: true}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	if _, exists := m["error"]; exists {
		t.Error("error field should be omitted when empty")
	}
	if _, exists := m["message"]; exists {
		t.Error("message field should be omitted when empty")
	}
	if _, exists := m["data"]; exists {
		t.Error("data field should be omitted when nil")
	}
	if m["success"] != true {
		t.Error("success field should be true")
	}
}

// TestEngineResult_JSONRoundTrip 验证 EngineResult 的 JSON 往返
func TestEngineResult_JSONRoundTrip(t *testing.T) {
	original := EngineResult{
		EngineName: "fofa",
		Total:      100,
		Page:       1,
		HasMore:    true,
		NormalizedData: []UnifiedAsset{
			{IP: "1.2.3.4", Port: 443, Protocol: "https"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded EngineResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.EngineName != original.EngineName {
		t.Errorf("EngineName: got %q, want %q", decoded.EngineName, original.EngineName)
	}
	if decoded.Total != original.Total {
		t.Errorf("Total: got %d, want %d", decoded.Total, original.Total)
	}
	if !decoded.HasMore {
		t.Error("HasMore should be true")
	}
	if len(decoded.NormalizedData) != 1 {
		t.Fatalf("NormalizedData length: got %d, want 1", len(decoded.NormalizedData))
	}
	if decoded.NormalizedData[0].IP != "1.2.3.4" {
		t.Errorf("NormalizedData[0].IP: got %q, want %q", decoded.NormalizedData[0].IP, "1.2.3.4")
	}
}

// TestQuotaInfo_JSONRoundTrip 验证 QuotaInfo 的 JSON 往返
func TestQuotaInfo_JSONRoundTrip(t *testing.T) {
	original := QuotaInfo{
		Remaining: 500,
		Total:     1000,
		Used:      500,
		Unit:      "times",
		Expiry:    "2026-12-31",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded QuotaInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Remaining != original.Remaining {
		t.Errorf("Remaining: got %d, want %d", decoded.Remaining, original.Remaining)
	}
	if decoded.Total != original.Total {
		t.Errorf("Total: got %d, want %d", decoded.Total, original.Total)
	}
	if decoded.Unit != original.Unit {
		t.Errorf("Unit: got %q, want %q", decoded.Unit, original.Unit)
	}
}

// TestFOFAOfficialWebURL 验证常量值
func TestFOFAOfficialWebURL(t *testing.T) {
	if FOFAOfficialWebURL != "https://fofa.info" {
		t.Errorf("FOFAOfficialWebURL = %q, want %q", FOFAOfficialWebURL, "https://fofa.info")
	}
}
