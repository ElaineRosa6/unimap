package web

import (
	"testing"
)

func TestFilterStableEngines(t *testing.T) {
	tests := []struct {
		input []string
		want  int
	}{
		{[]string{"fofa", "hunter"}, 2},
		{[]string{"fofa", "unknown"}, 1},
		{[]string{}, 0},
		{[]string{"FOFA", "Hunter"}, 2}, // case-insensitive
		{nil, 0},
	}
	for _, tt := range tests {
		got := filterStableEngines(tt.input)
		if len(got) != tt.want {
			t.Errorf("filterStableEngines(%v) returned %d items, want %d: %v", tt.input, len(got), tt.want, got)
		}
	}
}

func TestFilterStableEngines_AllStable(t *testing.T) {
	input := []string{"fofa", "hunter", "zoomeye", "quake", "shodan"}
	got := filterStableEngines(input)
	if len(got) != 5 {
		t.Fatalf("expected 5, got %d: %v", len(got), got)
	}
}

func TestTruncateQuotaError(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"", "failed to fetch quota"},
		{"short error", "short error"},
		{
			"this is a very long error message that exceeds one hundred and twenty characters in length and should be truncated to fit within the limit",
			"this is a very long error message that exceeds one hundred and twenty characters in length and should be truncated to fit within the limit"[:120] + "...",
		},
		{
			"  spaces  ",
			"spaces",
		},
		{
			"first line\nsecond line", // short, no truncation
			"first line\nsecond line",
		},
	}
	for _, tt := range tests {
		got := truncateQuotaError(tt.input)
		if got != tt.want {
			t.Errorf("truncateQuotaError(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTruncateQuotaError_LongFirstLine(t *testing.T) {
	longLine := ""
	for i := 0; i < 200; i++ {
		longLine += "x"
	}
	input := longLine + "\nsecond line"
	got := truncateQuotaError(input)
	if len(got) > 123 { // 120 + "..."
		t.Fatalf("expected truncated to ~120, got length %d: %q", len(got), got)
	}
}
