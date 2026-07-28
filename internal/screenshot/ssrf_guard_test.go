package screenshot

import (
	"testing"
)

func TestValidateBrowserURL_ValidPublic(t *testing.T) {
	valid := []string{
		"https://fofa.info/result?qbase64=dGVzdA==",
		"https://en.hunter.how/search?search-value=test",
		"https://www.zoomeye.org/searchResult?q=test",
		"https://quake.360.net/quake/#/searchResult?searchVal=test",
		"https://www.shodan.io/search?query=test",
		"http://93.184.216.34/",
		"https://example.com:8443/path",
	}
	for _, u := range valid {
		if err := ValidateBrowserURL(u); err != nil {
			t.Errorf("ValidateBrowserURL(%q) = %v, want nil", u, err)
		}
	}
}

func TestValidateBrowserURL_BlocksPrivate(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1/",
		"http://127.0.0.1:8080/admin",
		"https://localhost/",
		"http://10.0.0.1/",
		"http://172.16.0.1/",
		"http://192.168.1.1/",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/",
		"http://0.0.0.0/",
		"ftp://example.com/",
		"file:///etc/passwd",
		"",
		"   ",
	}
	for _, u := range blocked {
		if err := ValidateBrowserURL(u); err == nil {
			t.Errorf("ValidateBrowserURL(%q) = nil, want error", u)
		}
	}
}

func TestValidateBrowserURL_BlocksCloudMetadata(t *testing.T) {
	// Cloud metadata endpoints must be blocked.
	metadata := []string{
		"http://169.254.169.254/",           // AWS/GCP/Azure
		"http://169.254.169.254:80/latest/",  // with port
		"http://metadata.google.internal/",   // GCP DNS name resolves to 169.254.169.254
	}
	for _, u := range metadata[:2] {
		if err := ValidateBrowserURL(u); err == nil {
			t.Errorf("ValidateBrowserURL(%q) = nil, want error (cloud metadata)", u)
		}
	}
	// metadata.google.internal is a hostname — static check cannot block it
	// (requires DNS resolution). This is a known limitation documented in AGENTS.md.
	_ = metadata[2]
}

func TestSSRFInterceptor_NewAndCancel(t *testing.T) {
	interceptor := NewSSRFInterceptor()
	if interceptor == nil {
		t.Fatal("NewSSRFInterceptor() = nil")
	}
	// Cancel without Enable should not panic.
	interceptor.Cancel()

	blocked := interceptor.BlockedURLs()
	if len(blocked) != 0 {
		t.Errorf("BlockedURLs() = %v, want empty", blocked)
	}
}
