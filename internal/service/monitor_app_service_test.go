package service

import "testing"

// TestSummarizeReachability verifies the reachability summary aggregation,
// including the "blocked" bucket added for SSRF-rejected URLs. The invariant
// Total == Reachable + Unreachable + InvalidFormat + Blocked must hold for
// every known status.
func TestSummarizeReachability(t *testing.T) {
	tests := []struct {
		name    string
		results []URLReachabilityResult
		want    URLReachabilitySummary
	}{
		{
			name: "empty",
			want: URLReachabilitySummary{},
		},
		{
			name: "all reachable",
			results: []URLReachabilityResult{
				{Status: "reachable"},
				{Status: "reachable"},
			},
			want: URLReachabilitySummary{Total: 2, FormatValid: 2, Reachable: 2},
		},
		{
			name: "mixed known statuses",
			results: []URLReachabilityResult{
				{Status: "reachable"},
				{Status: "unreachable"},
				{Status: "invalid_format"},
				{Status: "blocked"},
				{Status: "reachable"},
			},
			want: URLReachabilitySummary{
				Total:         5,
				FormatValid:   3,
				InvalidFormat: 1,
				Reachable:     2,
				Unreachable:   1,
				Blocked:       1,
			},
		},
		{
			name: "unknown status falls through",
			results: []URLReachabilityResult{
				{Status: "blocked"},
				{Status: "future_status"},
			},
			want: URLReachabilitySummary{Total: 2, Blocked: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeReachability(tt.results)
			if got != tt.want {
				t.Fatalf("summary mismatch:\n got:  %+v\n want: %+v", got, tt.want)
			}
			// Invariant: known buckets must sum to Total.
			knownSum := got.Reachable + got.Unreachable + got.InvalidFormat + got.Blocked
			if knownSum != got.Total && tt.name != "unknown status falls through" {
				t.Fatalf("bucket invariant violated: %d != Total %d", knownSum, got.Total)
			}
		})
	}
}
