package web

import (
	"testing"
)

func TestJudgeLoginByCookieNames_Hunter(t *testing.T) {
	tests := []struct {
		byName map[string]string
		want   bool
	}{
		{map[string]string{"next": "token123"}, true},
		{map[string]string{"next": ""}, false},
		{map[string]string{}, false},
	}
	for _, tt := range tests {
		got := judgeLoginByCookieNames("hunter", tt.byName)
		if got != tt.want {
			t.Errorf("judgeLoginByCookieNames(hunter, %v) = %v, want %v", tt.byName, got, tt.want)
		}
	}
}

func TestJudgeLoginByCookieNames_Fofa(t *testing.T) {
	tests := []struct {
		byName map[string]string
		want   bool
	}{
		{map[string]string{"user": "admin"}, true},
		{map[string]string{"USER": "admin"}, true}, // case-insensitive
		{map[string]string{"user": ""}, false},
		{map[string]string{}, false},
	}
	for _, tt := range tests {
		got := judgeLoginByCookieNames("fofa", tt.byName)
		if got != tt.want {
			t.Errorf("judgeLoginByCookieNames(fofa, %v) = %v, want %v", tt.byName, got, tt.want)
		}
	}
}

func TestJudgeLoginByCookieNames_Quake(t *testing.T) {
	tests := []struct {
		byName map[string]string
		want   bool
	}{
		{map[string]string{"Q": "token", "T": "session"}, true},
		{map[string]string{"Q": "token"}, false},   // missing T
		{map[string]string{"T": "session"}, false}, // missing Q
		{map[string]string{}, false},
	}
	for _, tt := range tests {
		got := judgeLoginByCookieNames("quake", tt.byName)
		if got != tt.want {
			t.Errorf("judgeLoginByCookieNames(quake, %v) = %v, want %v", tt.byName, got, tt.want)
		}
	}
}

func TestJudgeLoginByCookieNames_ZoomEye(t *testing.T) {
	tests := []struct {
		byName map[string]string
		want   bool
	}{
		{map[string]string{"_xsrf": "token"}, true},
		{map[string]string{"session": "abc123"}, true},
		{map[string]string{"_xsrf": "", "session": ""}, false},
		{map[string]string{}, false},
	}
	for _, tt := range tests {
		got := judgeLoginByCookieNames("zoomeye", tt.byName)
		if got != tt.want {
			t.Errorf("judgeLoginByCookieNames(zoomeye, %v) = %v, want %v", tt.byName, got, tt.want)
		}
	}
}

func TestJudgeLoginByCookieNames_UnknownEngine(t *testing.T) {
	got := judgeLoginByCookieNames("unknown", map[string]string{"key": "val"})
	if got {
		t.Fatal("expected false for unknown engine")
	}
}

func TestJudgeLoginByCookieNames_Whitespace(t *testing.T) {
	got := judgeLoginByCookieNames("hunter", map[string]string{"next": "  "})
	if got {
		t.Fatal("expected false for whitespace-only value")
	}
}
