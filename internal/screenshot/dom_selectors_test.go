package screenshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
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

func TestShodanSearchFixture_CurrentLayout(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "shodan_search_results.html"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Find(".row.l-search-results .result").Length() != 0 {
		t.Fatal("fixture must not use the obsolete .row.l-search-results wrapper")
	}
	if n := doc.Find(".l-search-results .result").Length(); n != 2 {
		t.Fatalf(".l-search-results .result = %d, want 2", n)
	}
	href, _ := doc.Find(".result .heading a.title").First().Attr("href")
	if !strings.Contains(href, "/host/2600:9000:") {
		t.Fatalf("first host href = %q, want IPv6 /host/ path", href)
	}
	portHref, _ := doc.Find(".result .heading a.text-danger").First().Attr("href")
	if !strings.Contains(portHref, "]:80") {
		t.Fatalf("first port href = %q, want bracket IPv6 :80", portHref)
	}
	if doc.Find("h4.total-results").Text() != "5,169,318" {
		t.Fatalf("total-results = %q", doc.Find("h4.total-results").Text())
	}
}

func TestExtractShodanJS_CurrentLayoutHelpers(t *testing.T) {
	if strings.Contains(extractShodanJS, `/\\/host\\/`) || strings.Contains(extractShodanJS, `match(/\\/host`) {
		t.Fatal("extractShodanJS still uses a slash-escaped /host/ regex that is invalid JavaScript in this Go raw string")
	}
	for _, fragment := range []string{
		"function ipFromHostPath",
		"function portFromHref",
		".l-search-results .result",
		"h4.total-results",
		"marker = '/host/'",
		"lastIndexOf(']:')",
		"rowSelectorUsed",
		"rowsFound",
	} {
		if !strings.Contains(extractShodanJS, fragment) {
			t.Fatalf("extractShodanJS missing %q", fragment)
		}
	}
	if sel := selectorsByEngine["shodan"]; sel == nil || sel.RowSelector != ".l-search-results .result" {
		t.Fatalf("shodan RowSelector = %q", sel.RowSelector)
	}
	if !strings.Contains(selectorsByEngine["shodan"].TotalSelector, "h4.total-results") {
		t.Fatalf("shodan TotalSelector = %q", selectorsByEngine["shodan"].TotalSelector)
	}
}

func TestExtractorsMatchLive20260821Layout(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{"fofa", extractFofaJS, []string{".hsxa-port", ".hsxa-host, .hsxa-ip a", "hostPort"}},
		{"zoomeye", extractZoomEyeJS, []string{"hostPort", "asset.ip = ''"}},
		{"quake", extractQuakeJS, []string{"hostVal !== '--'", "v4 = clipText.match"}},
		{"censys", extractCensysJS, []string{"a[href*='/hosts/']", "row.tagName.toLowerCase() === 'a'"}},
		{"hunter", extractHunterJS, []string{"tls|ssl", "asset.ip.indexOf(':') < 0"}},
		{"daydaymap", extractDayDayMapJS, []string{"tr.ant-table-row", "measure-row", "cellText(1)"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, fragment := range tc.want {
				if !strings.Contains(tc.src, fragment) {
					t.Fatalf("%s extractor missing %q", tc.name, fragment)
				}
			}
		})
	}
	if got := selectorsByEngine["censys"].RowSelector; got != "a[href*='/hosts/']" {
		t.Fatalf("censys RowSelector = %q", got)
	}
}
