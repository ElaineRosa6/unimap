package screenshot

import (
	"strings"
	"testing"
)

func TestDayDayMapExtractorHasVirtualTableFallback(t *testing.T) {
	for _, fragment := range []string{
		"if (assets.length === 0)",
		"document.querySelectorAll('a, span, div, td')",
		"seenIPs",
		"asset.source = 'daydaymap'",
	} {
		if !strings.Contains(extractDayDayMapJS, fragment) {
			t.Fatalf("DayDayMap extractor is missing virtual-table fallback fragment %q", fragment)
		}
	}
}
