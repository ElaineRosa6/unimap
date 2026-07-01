package processors

import (
	"context"
	"net"
	"testing"

	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/plugin"
)

// ============================================================================
// ValidationProcessor 测试
// ============================================================================

// TestValidationProcessor_Metadata 验证插件元信息方法
func TestValidationProcessor_Metadata(t *testing.T) {
	p := NewValidationProcessor(false)

	if p.Name() != "validation_processor" {
		t.Errorf("Name() = %q, want %q", p.Name(), "validation_processor")
	}
	if p.Version() != "1.0.0" {
		t.Errorf("Version() = %q, want %q", p.Version(), "1.0.0")
	}
	if p.Type() != plugin.PluginTypeProcessor {
		t.Errorf("Type() = %q, want %q", p.Type(), plugin.PluginTypeProcessor)
	}
	if p.Priority() != 50 {
		t.Errorf("Priority() = %d, want 50", p.Priority())
	}
	if hs := p.Health(); !hs.Healthy {
		t.Errorf("Health() = %+v, want Healthy=true", hs)
	}
}

// TestValidationProcessor_IsValidIP 验证 IP 地址有效性（默认允许私有 IP）
func TestValidationProcessor_IsValidIP(t *testing.T) {
	p := NewValidationProcessor(false) // allowPrivateIP 默认为 true

	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"public ipv4", "1.1.1.1", true},
		{"public ipv4 2", "8.8.8.8", true},
		{"private ipv4 allowed by default", "192.168.1.1", true},
		{"loopback allowed by default", "127.0.0.1", true},
		{"public ipv6", "2001:db8::1", true},
		{"ipv6 loopback allowed", "::1", true},
		{"out of range octet", "256.1.1.1", false},
		{"too few octets", "1.1.1", false},
		{"not an ip", "not-an-ip", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.isValidIP(tt.ip); got != tt.want {
				t.Errorf("isValidIP(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

// TestValidationProcessor_IsPrivateIP 检查私有/保留 IP 范围识别
func TestValidationProcessor_IsPrivateIP(t *testing.T) {
	p := NewValidationProcessor(false)

	privateIPs := []string{
		"10.0.0.1",       // 10.0.0.0/8
		"172.16.0.1",     // 172.16.0.0/12 起始
		"172.31.255.255", // 172.16.0.0/12 结束
		"192.168.1.1",    // 192.168.0.0/16
		"127.0.0.1",      // 127.0.0.0/8
		"169.254.1.1",    // 169.254.0.0/16
		"::1",            // ::1/128
		"fc00::1",        // fc00::/7
		"fd00::1",        // fc00::/7 范围内
		"fe80::1",        // fe80::/10
	}
	for _, ip := range privateIPs {
		t.Run("private/"+ip, func(t *testing.T) {
			parsed := net.ParseIP(ip)
			if parsed == nil {
				t.Fatalf("net.ParseIP(%q) = nil, 解析失败", ip)
			}
			if !p.isPrivateIP(parsed) {
				t.Errorf("isPrivateIP(%q) = false, want true", ip)
			}
		})
	}

	publicIPs := []string{
		"8.8.8.8",              // 公网
		"1.1.1.1",              // 公网
		"172.15.0.1",           // 172.16/12 范围之下
		"172.32.0.1",           // 172.16/12 范围之上
		"11.0.0.1",             // 10/8 之外
		"2001:4860:4860::8888", // 公网 IPv6
	}
	for _, ip := range publicIPs {
		t.Run("public/"+ip, func(t *testing.T) {
			parsed := net.ParseIP(ip)
			if parsed == nil {
				t.Fatalf("net.ParseIP(%q) = nil, 解析失败", ip)
			}
			if p.isPrivateIP(parsed) {
				t.Errorf("isPrivateIP(%q) = true, want false", ip)
			}
		})
	}
}

// TestValidationProcessor_IsValidPort 端口号边界验证
func TestValidationProcessor_IsValidPort(t *testing.T) {
	p := NewValidationProcessor(false)

	tests := []struct {
		name string
		port int
		want bool
	}{
		{"zero", 0, false},
		{"one", 1, true},
		{"http", 80, true},
		{"https", 443, true},
		{"max valid", 65535, true},
		{"over max", 65536, false},
		{"negative", -1, false},
		{"large invalid", 100000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.isValidPort(tt.port); got != tt.want {
				t.Errorf("isValidPort(%d) = %v, want %v", tt.port, got, tt.want)
			}
		})
	}
}

// TestValidationProcessor_IsValidURL URL 格式验证
func TestValidationProcessor_IsValidURL(t *testing.T) {
	p := NewValidationProcessor(false)

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"valid http", "http://example.com", true},
		{"valid https", "https://example.com", true},
		{"valid https with path", "https://example.com/path", true},
		{"valid with port and query", "http://example.com:8080/path?query=1", true},
		{"missing scheme", "example.com", false},
		{"unsupported scheme", "ftp://example.com", false},
		{"scheme only", "http://", false},
		{"single char host", "http://a", false},
		{"malformed", "not a url", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.isValidURL(tt.url); got != tt.want {
				t.Errorf("isValidURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

// TestValidationProcessor_Process_StrictMode 严格模式下无效资产被丢弃
func TestValidationProcessor_Process_StrictMode(t *testing.T) {
	p := NewValidationProcessor(true) // 严格模式

	assets := []model.UnifiedAsset{
		{IP: "1.1.1.1", Port: 80, URL: "http://example.com"}, // 合法，保留
		{IP: "999.999.999.999", Port: 80, URL: ""},           // 无效 IP，丢弃
		{IP: "1.1.1.1", Port: 99999, URL: ""},                // 无效端口，丢弃
		{IP: "1.1.1.1", Port: 80, URL: "not-a-url"},          // 无效 URL，丢弃
	}

	got, err := p.Process(context.Background(), assets)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (严格模式下仅合法资产保留)", len(got))
	}
	if got[0].IP != "1.1.1.1" || got[0].Port != 80 {
		t.Errorf("got[0] = {IP:%s Port:%d}, want IP=1.1.1.1 Port=80", got[0].IP, got[0].Port)
	}
}

// TestValidationProcessor_Process_LenientMode 宽松模式下无效资产被保留
func TestValidationProcessor_Process_LenientMode(t *testing.T) {
	p := NewValidationProcessor(false) // 宽松模式

	assets := []model.UnifiedAsset{
		{IP: "1.1.1.1", Port: 80, URL: "http://example.com"}, // 合法
		{IP: "999.999.999.999", Port: 80, URL: ""},           // 无效 IP，但宽松模式保留
		{IP: "1.1.1.1", Port: 99999, URL: ""},                // 无效端口，但宽松模式保留
		{IP: "1.1.1.1", Port: 80, URL: "not-a-url"},          // 无效 URL，但宽松模式保留
	}

	got, err := p.Process(context.Background(), assets)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("len(got) = %d, want 4 (宽松模式下所有资产保留)", len(got))
	}
}

// TestValidationProcessor_AllowPrivateIP_False allowPrivateIP=false 时严格模式拒绝私有 IP
func TestValidationProcessor_AllowPrivateIP_False(t *testing.T) {
	p := NewValidationProcessor(true)
	p.allowPrivateIP = false // 白盒设置：禁止私有 IP

	assets := []model.UnifiedAsset{
		{IP: "192.168.1.1", Port: 80}, // 私有 IP，拒绝
		{IP: "10.0.0.1", Port: 80},    // 私有 IP，拒绝
		{IP: "8.8.8.8", Port: 80},     // 公网 IP，保留
	}

	got, err := p.Process(context.Background(), assets)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (私有 IP 应被拒绝)", len(got))
	}
	if got[0].IP != "8.8.8.8" {
		t.Errorf("got[0].IP = %q, want 8.8.8.8", got[0].IP)
	}
}

// TestValidationProcessor_AllowPrivateIP_True allowPrivateIP=true 时严格模式接受私有 IP
func TestValidationProcessor_AllowPrivateIP_True(t *testing.T) {
	p := NewValidationProcessor(true) // allowPrivateIP 默认为 true

	assets := []model.UnifiedAsset{
		{IP: "192.168.1.1", Port: 80}, // 私有 IP，但允许
		{IP: "10.0.0.1", Port: 80},    // 私有 IP，但允许
	}

	got, err := p.Process(context.Background(), assets)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (allowPrivateIP=true 时私有 IP 应被接受)", len(got))
	}
}

// TestValidationProcessor_Process_EmptyInput 空输入应返回空切片且无错误
func TestValidationProcessor_Process_EmptyInput(t *testing.T) {
	p := NewValidationProcessor(true)

	got, err := p.Process(context.Background(), []model.UnifiedAsset{})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

// ============================================================================
// EnrichmentProcessor 测试
// ============================================================================

// TestEnrichmentProcessor_Metadata 验证插件元信息方法
func TestEnrichmentProcessor_Metadata(t *testing.T) {
	p := NewEnrichmentProcessor()

	if p.Name() != "enrichment_processor" {
		t.Errorf("Name() = %q, want %q", p.Name(), "enrichment_processor")
	}
	if p.Version() != "1.0.0" {
		t.Errorf("Version() = %q, want %q", p.Version(), "1.0.0")
	}
	if p.Type() != plugin.PluginTypeProcessor {
		t.Errorf("Type() = %q, want %q", p.Type(), plugin.PluginTypeProcessor)
	}
	if p.Priority() != 80 {
		t.Errorf("Priority() = %d, want 80", p.Priority())
	}
	if hs := p.Health(); !hs.Healthy {
		t.Errorf("Health() = %+v, want Healthy=true", hs)
	}
}

// TestEnrichmentProcessor_GuessServiceType 基于端口推测服务类型，回退到 Protocol 再到 unknown
func TestEnrichmentProcessor_GuessServiceType(t *testing.T) {
	p := NewEnrichmentProcessor()

	tests := []struct {
		name     string
		port     int
		protocol string
		want     string
	}{
		{"http 80", 80, "", "http"},
		{"http 8080", 8080, "", "http"},
		{"http 8000", 8000, "", "http"},
		{"https 443", 443, "", "https"},
		{"https 8443", 8443, "", "https"},
		{"ftp 21", 21, "", "ftp"},
		{"ssh 22", 22, "", "ssh"},
		{"mysql 3306", 3306, "", "mysql"},
		{"postgresql 5432", 5432, "", "postgresql"},
		{"redis 6379", 6379, "", "redis"},
		{"mongodb 27017", 27017, "", "mongodb"},
		{"unknown port fallback to protocol", 1234, "custom-proto", "custom-proto"},
		{"unknown port no protocol fallback to unknown", 1234, "", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset := model.UnifiedAsset{Port: tt.port, Protocol: tt.protocol}
			if got := p.guessServiceType(asset); got != tt.want {
				t.Errorf("guessServiceType(port=%d, protocol=%q) = %q, want %q",
					tt.port, tt.protocol, got, tt.want)
			}
		})
	}
}

// TestEnrichmentProcessor_NormalizeCountryCode 国家代码应转为大写
func TestEnrichmentProcessor_NormalizeCountryCode(t *testing.T) {
	p := NewEnrichmentProcessor()

	tests := []struct {
		code string
		want string
	}{
		{"us", "US"},
		{"Cn", "CN"},
		{"DE", "DE"},
		{"jp", "JP"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := p.normalizeCountryCode(tt.code); got != tt.want {
				t.Errorf("normalizeCountryCode(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// TestEnrichmentProcessor_GenerateFingerprint 指纹格式应为 IP:Port:Protocol
func TestEnrichmentProcessor_GenerateFingerprint(t *testing.T) {
	p := NewEnrichmentProcessor()

	asset := model.UnifiedAsset{IP: "1.1.1.1", Port: 80, Protocol: "http"}
	if got := p.generateFingerprint(asset); got != "1.1.1.1:80:http" {
		t.Errorf("generateFingerprint() = %q, want %q", got, "1.1.1.1:80:http")
	}
}

// TestEnrichmentProcessor_Process_EnrichesAll 富化所有资产（不做过滤）
func TestEnrichmentProcessor_Process_EnrichesAll(t *testing.T) {
	p := NewEnrichmentProcessor()

	assets := []model.UnifiedAsset{
		{IP: "1.1.1.1", Port: 80, Protocol: "http", CountryCode: "us"},
		{IP: "2.2.2.2", Port: 443, Protocol: "https", CountryCode: "de"},
	}

	got, err := p.Process(context.Background(), assets)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (富化不应过滤资产)", len(got))
	}

	// 第一个资产：验证富化字段
	a0 := got[0]
	if a0.CountryCode != "US" {
		t.Errorf("got[0].CountryCode = %q, want US (应大写化)", a0.CountryCode)
	}
	if a0.Extra == nil {
		t.Fatalf("got[0].Extra = nil, 应被初始化并填充")
	}
	if st, ok := a0.Extra["service_type"].(string); !ok || st != "http" {
		t.Errorf("got[0].Extra[service_type] = %v, want \"http\"", a0.Extra["service_type"])
	}
	if fp, ok := a0.Extra["fingerprint"].(string); !ok || fp != "1.1.1.1:80:http" {
		t.Errorf("got[0].Extra[fingerprint] = %v, want \"1.1.1.1:80:http\"", a0.Extra["fingerprint"])
	}

	// 第二个资产：验证富化字段
	a1 := got[1]
	if a1.CountryCode != "DE" {
		t.Errorf("got[1].CountryCode = %q, want DE (应大写化)", a1.CountryCode)
	}
	if st, ok := a1.Extra["service_type"].(string); !ok || st != "https" {
		t.Errorf("got[1].Extra[service_type] = %v, want \"https\"", a1.Extra["service_type"])
	}
	if fp, ok := a1.Extra["fingerprint"].(string); !ok || fp != "2.2.2.2:443:https" {
		t.Errorf("got[1].Extra[fingerprint] = %v, want \"2.2.2.2:443:https\"", a1.Extra["fingerprint"])
	}
}

// TestEnrichmentProcessor_ExtraMapInitialized Extra 为 nil 时应被初始化
func TestEnrichmentProcessor_ExtraMapInitialized(t *testing.T) {
	p := NewEnrichmentProcessor()

	asset := model.UnifiedAsset{IP: "1.1.1.1", Port: 80, Protocol: "http"} // Extra 为 nil
	got, err := p.Process(context.Background(), []model.UnifiedAsset{asset})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Extra == nil {
		t.Fatal("Extra = nil, 应被初始化为非 nil map")
	}
	if _, ok := got[0].Extra["service_type"]; !ok {
		t.Error("Extra[service_type] 缺失, 应被填充")
	}
	if _, ok := got[0].Extra["fingerprint"]; !ok {
		t.Error("Extra[fingerprint] 缺失, 应被填充")
	}
}

// TestEnrichmentProcessor_EmptyInput 空输入应返回空切片且无错误
func TestEnrichmentProcessor_EmptyInput(t *testing.T) {
	p := NewEnrichmentProcessor()

	got, err := p.Process(context.Background(), []model.UnifiedAsset{})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}
