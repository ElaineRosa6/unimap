package web

import (
	"testing"

	"github.com/unimap/project/internal/config"
)

func TestConvertConfigCookies_Extended(t *testing.T) {
	input := []config.Cookie{
		{Name: "session", Value: "abc123", Domain: ".example.com"},
		{Name: "token", Value: "xyz789", Path: "/api"},
	}
	got := convertConfigCookies(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(got))
	}
	if got[0].Name != "session" || got[0].Value != "abc123" {
		t.Fatalf("first cookie wrong: %+v", got[0])
	}
	if got[1].Name != "token" || got[1].Path != "/api" {
		t.Fatalf("second cookie wrong: %+v", got[1])
	}
}

func TestConvertConfigCookies_Empty_Extended(t *testing.T) {
	got := convertConfigCookies([]config.Cookie{})
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d", len(got))
	}
}

func TestConvertConfigCookies_Nil_Extended(t *testing.T) {
	got := convertConfigCookies(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d", len(got))
	}
}

func TestIcpConfigProvider(t *testing.T) {
	s := &Server{config: &config.Config{}}
	s.config.ICP.Enabled = true
	s.config.ICP.BaseURL = "http://localhost:16181"
	s.config.ICP.APIKey = "test-key"
	s.config.ICP.Timeout = 30
	s.config.ICP.DefaultType = "web"

	cfg := s.icpConfigProvider()
	if !cfg.Enabled {
		t.Fatal("expected ICP enabled")
	}
	if cfg.BaseURL != "http://localhost:16181" {
		t.Fatalf("expected base URL, got %q", cfg.BaseURL)
	}
}

func TestIcpConfigProvider_NilConfig(t *testing.T) {
	s := &Server{}
	cfg := s.icpConfigProvider()
	if cfg.Enabled {
		t.Fatal("expected ICP disabled for nil config")
	}
}

func TestAdminToken_Extended(t *testing.T) {
	s := &Server{config: &config.Config{}}
	s.config.Web.Auth.Enabled = true
	s.config.Web.Auth.AdminToken = "my-token"
	if s.adminToken() != "my-token" {
		t.Fatalf("expected 'my-token', got %q", s.adminToken())
	}
}

func TestAdminToken_Disabled_Extended(t *testing.T) {
	s := &Server{config: &config.Config{}}
	s.config.Web.Auth.Enabled = false
	s.config.Web.Auth.AdminToken = "my-token"
	if s.adminToken() != "" {
		t.Fatalf("expected empty when disabled, got %q", s.adminToken())
	}
}

func TestAdminToken_NilConfig_Extended(t *testing.T) {
	s := &Server{}
	if s.adminToken() != "" {
		t.Fatalf("expected empty for nil config, got %q", s.adminToken())
	}
}

func TestStaticVersion(t *testing.T) {
	s := &Server{}
	s.staticVersion = "1.2.3"
	if s.staticVersion != "1.2.3" {
		t.Fatalf("expected '1.2.3', got %q", s.staticVersion)
	}
}

func TestNewTemplateFuncMap(t *testing.T) {
	funcMap := newTemplateFuncMap()
	if funcMap == nil {
		t.Fatal("expected non-nil func map")
	}
	// Check expected functions
	if _, ok := funcMap["mul"]; !ok {
		t.Fatal("expected 'mul' function")
	}
	if _, ok := funcMap["div"]; !ok {
		t.Fatal("expected 'div' function")
	}
	if _, ok := funcMap["float"]; !ok {
		t.Fatal("expected 'float' function")
	}
	if _, ok := funcMap["join"]; !ok {
		t.Fatal("expected 'join' function")
	}
	if _, ok := funcMap["dict"]; !ok {
		t.Fatal("expected 'dict' function")
	}
}

func TestTemplateFuncMap_Mul(t *testing.T) {
	funcMap := newTemplateFuncMap()
	mulFn := funcMap["mul"].(func(float64, float64) float64)
	if mulFn(2.0, 3.0) != 6.0 {
		t.Fatal("mul(2,3) should be 6")
	}
}

func TestTemplateFuncMap_Div(t *testing.T) {
	funcMap := newTemplateFuncMap()
	divFn := funcMap["div"].(func(float64, float64) float64)
	if divFn(6.0, 3.0) != 2.0 {
		t.Fatal("div(6,3) should be 2")
	}
	if divFn(1.0, 0.0) != 0.0 {
		t.Fatal("div(1,0) should be 0")
	}
}

func TestTemplateFuncMap_Float(t *testing.T) {
	funcMap := newTemplateFuncMap()
	floatFn := funcMap["float"].(func(int) float64)
	if floatFn(42) != 42.0 {
		t.Fatal("float(42) should be 42.0")
	}
}

func TestTemplateFuncMap_Join(t *testing.T) {
	funcMap := newTemplateFuncMap()
	joinFn := funcMap["join"].(func([]string, string) string)
	if joinFn([]string{"a", "b", "c"}, ",") != "a,b,c" {
		t.Fatal("join error")
	}
}

func TestTemplateFuncMap_Dict(t *testing.T) {
	funcMap := newTemplateFuncMap()
	dictFn := funcMap["dict"].(func(values ...interface{}) (map[string]interface{}, error))
	result, err := dictFn("key1", "val1", "key2", "val2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key1"] != "val1" || result["key2"] != "val2" {
		t.Fatalf("unexpected dict: %v", result)
	}
}

func TestTemplateFuncMap_Dict_OddArgs(t *testing.T) {
	funcMap := newTemplateFuncMap()
	dictFn := funcMap["dict"].(func(values ...interface{}) (map[string]interface{}, error))
	_, err := dictFn("key1")
	if err == nil {
		t.Fatal("expected error for odd args")
	}
}
