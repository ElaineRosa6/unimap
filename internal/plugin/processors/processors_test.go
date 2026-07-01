package processors

import (
	"context"
	"testing"

	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/plugin"
)

// ============================================================================
// DeduplicationProcessor 测试
// ============================================================================

// TestDeduplicationProcessor_Metadata 验证插件元信息方法
func TestDeduplicationProcessor_Metadata(t *testing.T) {
	p := NewDeduplicationProcessor(StrategyIPPort)

	if p.Name() != "deduplication_processor" {
		t.Errorf("Name() = %q, want %q", p.Name(), "deduplication_processor")
	}
	if p.Version() != "1.0.0" {
		t.Errorf("Version() = %q, want %q", p.Version(), "1.0.0")
	}
	if p.Type() != plugin.PluginTypeProcessor {
		t.Errorf("Type() = %q, want %q", p.Type(), plugin.PluginTypeProcessor)
	}
	if p.Priority() != 100 {
		t.Errorf("Priority() = %d, want 100", p.Priority())
	}
	if hs := p.Health(); !hs.Healthy {
		t.Errorf("Health() = %+v, want Healthy=true", hs)
	}
}

// TestDeduplicationProcessor_StrategyIPPort 基于 IP:Port 去重
func TestDeduplicationProcessor_StrategyIPPort(t *testing.T) {
	p := NewDeduplicationProcessor(StrategyIPPort)
	assets := []model.UnifiedAsset{
		{IP: "1.1.1.1", Port: 80},
		{IP: "1.1.1.1", Port: 80},  // 重复，应被去除
		{IP: "1.1.1.1", Port: 443}, // 不同端口，保留
		{IP: "2.2.2.2", Port: 80},  // 不同 IP，保留
	}

	got, err := p.Process(context.Background(), assets)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	// 去重保留首个出现的资产
	if got[0].IP != "1.1.1.1" || got[0].Port != 80 {
		t.Errorf("got[0] = {IP:%s Port:%d}, want IP=1.1.1.1 Port=80", got[0].IP, got[0].Port)
	}
	if got[1].IP != "1.1.1.1" || got[1].Port != 443 {
		t.Errorf("got[1] = {IP:%s Port:%d}, want IP=1.1.1.1 Port=443", got[1].IP, got[1].Port)
	}
	if got[2].IP != "2.2.2.2" || got[2].Port != 80 {
		t.Errorf("got[2] = {IP:%s Port:%d}, want IP=2.2.2.2 Port=80", got[2].IP, got[2].Port)
	}
}

// TestDeduplicationProcessor_StrategyURL 基于 URL 去重，空 URL 始终保留
func TestDeduplicationProcessor_StrategyURL(t *testing.T) {
	p := NewDeduplicationProcessor(StrategyURL)
	assets := []model.UnifiedAsset{
		{IP: "1.1.1.1", Port: 80, URL: "http://example.com"},
		{IP: "2.2.2.2", Port: 80, URL: ""},                   // 空 URL，始终保留
		{IP: "3.3.3.3", Port: 80, URL: ""},                   // 空 URL，始终保留
		{IP: "4.4.4.4", Port: 80, URL: "http://example.com"}, // 重复 URL，去除
		{IP: "5.5.5.5", Port: 80, URL: "http://other.com"},   // 不同 URL，保留
	}

	got, err := p.Process(context.Background(), assets)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("len(got) = %d, want 4 (空 URL 始终保留，重复 URL 去除)", len(got))
	}
	if got[0].IP != "1.1.1.1" {
		t.Errorf("got[0].IP = %q, want 1.1.1.1", got[0].IP)
	}
	if got[1].IP != "2.2.2.2" {
		t.Errorf("got[1].IP = %q, want 2.2.2.2 (空 URL 应保留)", got[1].IP)
	}
	if got[2].IP != "3.3.3.3" {
		t.Errorf("got[2].IP = %q, want 3.3.3.3 (空 URL 应保留)", got[2].IP)
	}
	if got[3].IP != "5.5.5.5" {
		t.Errorf("got[3].IP = %q, want 5.5.5.5", got[3].IP)
	}
}

// TestDeduplicationProcessor_StrategyHost 基于 Host 去重，空 Host 回退到 IP
func TestDeduplicationProcessor_StrategyHost(t *testing.T) {
	p := NewDeduplicationProcessor(StrategyHost)
	assets := []model.UnifiedAsset{
		{IP: "1.1.1.1", Port: 80, Host: "example.com"},
		{IP: "2.2.2.2", Port: 80, Host: ""},            // 空 Host，回退到 IP 作为键
		{IP: "2.2.2.2", Port: 80, Host: ""},            // 空 Host，IP 重复，去除
		{IP: "3.3.3.3", Port: 80, Host: "example.com"}, // Host 重复，去除
	}

	got, err := p.Process(context.Background(), assets)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Host != "example.com" {
		t.Errorf("got[0].Host = %q, want example.com", got[0].Host)
	}
	if got[1].IP != "2.2.2.2" {
		t.Errorf("got[1].IP = %q, want 2.2.2.2 (空 Host 回退到 IP)", got[1].IP)
	}
}

// TestDeduplicationProcessor_StrategyAdvanced 高级去重，综合 IP/Port/Protocol/Host
func TestDeduplicationProcessor_StrategyAdvanced(t *testing.T) {
	p := NewDeduplicationProcessor(StrategyAdvanced)
	base := model.UnifiedAsset{IP: "1.1.1.1", Port: 80, Protocol: "http", Host: "a.com"}
	assets := []model.UnifiedAsset{
		base, // 保留
		base, // 完全重复，去除
		{IP: "1.1.1.1", Port: 80, Protocol: "http", Host: "b.com"},  // Host 不同，保留
		{IP: "1.1.1.1", Port: 80, Protocol: "https", Host: "a.com"}, // Protocol 不同，保留
		{IP: "1.1.1.1", Port: 443, Protocol: "http", Host: "a.com"}, // Port 不同，保留
		{IP: "2.2.2.2", Port: 80, Protocol: "http", Host: "a.com"},  // IP 不同，保留
	}

	got, err := p.Process(context.Background(), assets)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("len(got) = %d, want 5 (仅完全重复的资产被去除)", len(got))
	}
	if got[0].Host != "a.com" {
		t.Errorf("got[0].Host = %q, want a.com", got[0].Host)
	}
}

// TestDeduplicationProcessor_EmptyInput 空输入应返回空切片
func TestDeduplicationProcessor_EmptyInput(t *testing.T) {
	p := NewDeduplicationProcessor(StrategyIPPort)

	t.Run("empty slice", func(t *testing.T) {
		got, err := p.Process(context.Background(), []model.UnifiedAsset{})
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("len(got) = %d, want 0", len(got))
		}
	})

	t.Run("nil slice", func(t *testing.T) {
		got, err := p.Process(context.Background(), nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("len(got) = %d, want 0", len(got))
		}
	})
}

// TestDeduplicationProcessor_UnknownStrategyFallsBackToIPPort 未知策略回退到 IP:Port 去重
func TestDeduplicationProcessor_UnknownStrategyFallsBackToIPPort(t *testing.T) {
	p := NewDeduplicationProcessor(DeduplicationStrategy("unknown-strategy"))
	assets := []model.UnifiedAsset{
		{IP: "1.1.1.1", Port: 80},
		{IP: "1.1.1.1", Port: 80}, // 未知策略回退到 IP:Port，重复去除
		{IP: "2.2.2.2", Port: 80}, // 保留
	}

	got, err := p.Process(context.Background(), assets)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (未知策略应回退到 IP:Port 去重)", len(got))
	}
	if got[0].IP != "1.1.1.1" || got[0].Port != 80 {
		t.Errorf("got[0] = {IP:%s Port:%d}, want IP=1.1.1.1 Port=80", got[0].IP, got[0].Port)
	}
	if got[1].IP != "2.2.2.2" {
		t.Errorf("got[1].IP = %q, want 2.2.2.2", got[1].IP)
	}
}

// TestDeduplicationProcessor_Initialize_StrategyFromConfig 通过 Initialize 从配置读取策略
func TestDeduplicationProcessor_Initialize_StrategyFromConfig(t *testing.T) {
	p := NewDeduplicationProcessor(StrategyIPPort)
	if err := p.Initialize(&model.PluginConfig{
		Extra: map[string]any{"strategy": "host"},
	}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	// 配置应将策略切换为 host：相同 Host 的资产被去除
	assets := []model.UnifiedAsset{
		{IP: "1.1.1.1", Port: 80, Host: "example.com"},
		{IP: "2.2.2.2", Port: 80, Host: "example.com"}, // Host 重复，去除
	}
	got, err := p.Process(context.Background(), assets)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len(got) = %d, want 1 (策略应由配置切换为 host)", len(got))
	}
}

// ============================================================================
// DataCleaningProcessor 测试
// ============================================================================

// TestDataCleaningProcessor_Metadata 验证插件元信息方法
func TestDataCleaningProcessor_Metadata(t *testing.T) {
	p := NewDataCleaningProcessor()

	if p.Name() != "data_cleaning_processor" {
		t.Errorf("Name() = %q, want %q", p.Name(), "data_cleaning_processor")
	}
	if p.Version() != "1.0.0" {
		t.Errorf("Version() = %q, want %q", p.Version(), "1.0.0")
	}
	if p.Type() != plugin.PluginTypeProcessor {
		t.Errorf("Type() = %q, want %q", p.Type(), plugin.PluginTypeProcessor)
	}
	if p.Priority() != 10 {
		t.Errorf("Priority() = %d, want 10", p.Priority())
	}
}

// TestDataCleaningProcessor_TrimWhitespace 所有可清洗字段应去除首尾空白
func TestDataCleaningProcessor_TrimWhitespace(t *testing.T) {
	p := NewDataCleaningProcessor()
	asset := model.UnifiedAsset{
		IP:          "  1.1.1.1  ",
		Protocol:    "  HTTP  ",
		Host:        "  Example.com  ",
		URL:         "  http://example.com  ",
		Title:       "  Title  ",
		Server:      "  nginx  ",
		CountryCode: "  US  ",
		Region:      "  Region  ",
		City:        "  City  ",
		ASN:         "  AS123  ",
		Org:         "  Org  ",
		ISP:         "  ISP  ",
	}

	got, err := p.Process(context.Background(), []model.UnifiedAsset{asset})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	c := got[0]
	// protocol/host 去空白后还会被小写化
	if c.IP != "1.1.1.1" {
		t.Errorf("IP = %q, want 1.1.1.1", c.IP)
	}
	if c.Protocol != "http" {
		t.Errorf("Protocol = %q, want http (去空白+小写化)", c.Protocol)
	}
	if c.Host != "example.com" {
		t.Errorf("Host = %q, want example.com (去空白+小写化)", c.Host)
	}
	if c.URL != "http://example.com" {
		t.Errorf("URL = %q, want http://example.com", c.URL)
	}
	if c.Title != "Title" {
		t.Errorf("Title = %q, want Title", c.Title)
	}
	if c.Server != "nginx" {
		t.Errorf("Server = %q, want nginx", c.Server)
	}
	if c.CountryCode != "US" {
		t.Errorf("CountryCode = %q, want US", c.CountryCode)
	}
	if c.Region != "Region" {
		t.Errorf("Region = %q, want Region", c.Region)
	}
	if c.City != "City" {
		t.Errorf("City = %q, want City", c.City)
	}
	if c.ASN != "AS123" {
		t.Errorf("ASN = %q, want AS123", c.ASN)
	}
	if c.Org != "Org" {
		t.Errorf("Org = %q, want Org", c.Org)
	}
	if c.ISP != "ISP" {
		t.Errorf("ISP = %q, want ISP", c.ISP)
	}
}

// TestDataCleaningProcessor_NormalizeURL_TrailingSlash URL 尾部斜杠去除（含边界情况）
func TestDataCleaningProcessor_NormalizeURL_TrailingSlash(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"trailing slash", "http://example.com/", "http://example.com"},
		{"trailing slash with path", "http://example.com/path/", "http://example.com/path"},
		{"no trailing slash", "http://example.com", "http://example.com"},
		{"single slash stays", "/", "/"},        // len<=1 不去除尾部斜杠
		{"empty stays empty", "", ""},           // 空字符串不进入规范化
		{"whitespace plus slash", "  /  ", "/"}, // 先去空白，再判断 len<=1 保留
	}
	p := NewDataCleaningProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 给 IP 赋值，避免资产被当作空资产过滤
			got, err := p.Process(context.Background(), []model.UnifiedAsset{
				{IP: "1.1.1.1", URL: tt.url},
			})
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("len(got) = %d, want 1", len(got))
			}
			if got[0].URL != tt.want {
				t.Errorf("URL = %q, want %q", got[0].URL, tt.want)
			}
		})
	}
}

// TestDataCleaningProcessor_LowercaseNormalization protocol/host 小写化，URL 不小写化
func TestDataCleaningProcessor_LowercaseNormalization(t *testing.T) {
	p := NewDataCleaningProcessor()
	asset := model.UnifiedAsset{
		IP:       "1.1.1.1",
		Protocol: "HTTP",
		Host:     "Example.COM",
		URL:      "HTTP://Example.COM/Path",
	}

	got, err := p.Process(context.Background(), []model.UnifiedAsset{asset})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if got[0].Protocol != "http" {
		t.Errorf("Protocol = %q, want http", got[0].Protocol)
	}
	if got[0].Host != "example.com" {
		t.Errorf("Host = %q, want example.com", got[0].Host)
	}
	// URL 不在 lowercaseFields 中，应保持原样（仅做尾部斜杠规范化）
	if got[0].URL != "HTTP://Example.COM/Path" {
		t.Errorf("URL = %q, want HTTP://Example.COM/Path (URL 不应小写化)", got[0].URL)
	}
}

// TestDataCleaningProcessor_RemoveEmpty removeEmpty=true 时空资产被过滤、非空资产保留
func TestDataCleaningProcessor_RemoveEmpty(t *testing.T) {
	p := NewDataCleaningProcessor() // removeEmpty 默认为 true

	assets := []model.UnifiedAsset{
		{IP: "1.1.1.1", Port: 80},           // 非空，保留
		{IP: "", Host: "", URL: ""},         // 完全空，去除
		{IP: "   ", Host: "   ", URL: "  "}, // 仅空白，清洗后为空，去除
		{Host: "example.com"},               // Host 非空，保留
	}

	got, err := p.Process(context.Background(), assets)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (空资产应被过滤)", len(got))
	}
	if got[0].IP != "1.1.1.1" {
		t.Errorf("got[0].IP = %q, want 1.1.1.1", got[0].IP)
	}
	if got[1].Host != "example.com" {
		t.Errorf("got[1].Host = %q, want example.com", got[1].Host)
	}
}

// TestDataCleaningProcessor_KeepEmptyWhenDisabled removeEmpty=false 时空资产保留
func TestDataCleaningProcessor_KeepEmptyWhenDisabled(t *testing.T) {
	p := NewDataCleaningProcessor()
	// 通过 Initialize 关闭 removeEmpty
	if err := p.Initialize(&model.PluginConfig{
		Extra: map[string]any{"removeEmpty": false},
	}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	assets := []model.UnifiedAsset{
		{IP: "", Host: "", URL: ""}, // 空资产
		{IP: "1.1.1.1", Port: 80},   // 非空资产
	}

	got, err := p.Process(context.Background(), assets)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (removeEmpty=false 时空资产应保留)", len(got))
	}
}

// TestDataCleaningProcessor_EmptyInput 空输入应返回空切片且无错误
func TestDataCleaningProcessor_EmptyInput(t *testing.T) {
	p := NewDataCleaningProcessor()

	got, err := p.Process(context.Background(), []model.UnifiedAsset{})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}
