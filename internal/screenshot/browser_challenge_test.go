package screenshot

import (
	"testing"

	"github.com/unimap/project/internal/collection"
)

func TestDetectBrowserChallenge(t *testing.T) {
	tests := []struct {
		name  string
		title string
		body  string
		want  bool
	}{
		{name: "cloudflare title", title: "Just a moment...", want: true},
		{name: "human verification", body: "Performing security verification. Verify you are human.", want: true},
		{name: "normal result", title: "Search results", body: "10 hosts found", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := detectBrowserChallenge(test.title, test.body); got != test.want {
				t.Fatalf("detectBrowserChallenge()=%v, want %v", got, test.want)
			}
		})
	}
}

func TestCollectionNeedsExtensionFallback(t *testing.T) {
	if !collectionNeedsExtensionFallback([]collection.CollectResult{{BrowserChallenge: true}}) {
		t.Fatal("browser challenge must trigger extension fallback")
	}
	if !collectionNeedsExtensionFallback([]collection.CollectResult{{LoginRequired: true}}) {
		t.Fatal("login wall must trigger extension fallback")
	}
	if collectionNeedsExtensionFallback([]collection.CollectResult{{Assets: nil, ExtractionError: "no_result_rows"}}) {
		t.Fatal("ordinary empty results must not trigger extension fallback")
	}
}
