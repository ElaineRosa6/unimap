package adapter

import (
	"testing"

	"github.com/unimap/project/internal/model"
)

// TestFofaRowToItem verifies the array-based FOFA row maps to FofaItem by field
// order, capturing unknown columns into Extra instead of dropping them.
func TestFofaRowToItem(t *testing.T) {
	fields := []string{"ip", "port", "protocol", "title", "lastupdatetime", "custom_field"}
	row := []interface{}{
		"1.2.3.4",
		float64(8080),
		"http",
		"Example",
		"2026-08-06 09:00:00",
		"custom-value",
	}
	item := fofaRowToItem(row, fields)
	if item == nil {
		t.Fatal("fofaRowToItem returned nil")
	}
	if item.IP != "1.2.3.4" || item.Port != 8080 || item.Protocol != "http" || item.Title != "Example" {
		t.Errorf("declared fields lost: %+v", item)
	}
	if item.Lastupdatetime != "2026-08-06 09:00:00" {
		t.Errorf("Lastupdatetime = %q", item.Lastupdatetime)
	}
	if item.Extra["custom_field"] != "custom-value" {
		t.Errorf("Extra[custom_field] = %v", item.Extra["custom_field"])
	}
}

// TestFofaNormalizePromotesLastSeen verifies lastupdatetime becomes LastSeen.
func TestFofaNormalizePromotesLastSeen(t *testing.T) {
	item := &FofaItem{
		IP:             "1.2.3.4",
		Port:           80,
		Protocol:       "http",
		Title:          "Example",
		Lastupdatetime: "2026-08-06 09:00:00",
		Host:           "example.com",
	}
	asset := normalizeFofaItem(item)
	if asset == nil {
		t.Fatal("normalizeFofaItem returned nil")
	}
	if asset.LastSeen != "2026-08-06 09:00:00" {
		t.Errorf("LastSeen = %q, want 2026-08-06 09:00:00", asset.LastSeen)
	}
	// Host column has no dedicated unified slot → preserved under Extra.
	if asset.Extra["host"] != "example.com" {
		t.Errorf("Extra[host] = %v", asset.Extra["host"])
	}
}

// TestIsFofaPermissionError verifies the detector recognizes the two real
// permission-error shapes ("没有权限" message and 820001 code).
func TestIsFofaPermissionError(t *testing.T) {
	table := []struct {
		name string
		body []byte
		want bool
	}{
		{
			"no permission errmsg",
			[]byte(`{"error":true,"errmsg":"没有权限"}`),
			true,
		},
		{
			"820001 in errmsg",
			[]byte(`{"error":true,"errmsg":"820001 no permission"}`),
			true,
		},
		{
			"non-permission error",
			[]byte(`{"error":true,"errmsg":"query syntax error"}`),
			false,
		},
		{
			"success response",
			[]byte(`{"error":false,"results":[]}`),
			false,
		},
		{
			"malformed body",
			[]byte(`not json`),
			false,
		},
	}
	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			if got := isFofaPermissionError(tt.body); got != tt.want {
				t.Errorf("isFofaPermissionError(%s) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}

// TestFofaFieldSetsOrder ensures degradation goes from the widest field set
// down to the smallest, so the last set is always a permission-safe baseline.
func TestFofaFieldSetsOrder(t *testing.T) {
	if len(fofaFieldSets) < 3 {
		t.Fatalf("expected >=3 field sets, got %d", len(fofaFieldSets))
	}
	for i := 0; i < len(fofaFieldSets)-1; i++ {
		cur := len(splitFofaFields(fofaFieldSets[i]))
		next := len(splitFofaFields(fofaFieldSets[i+1]))
		if cur <= next {
			t.Errorf("field set %d (%d fields) should be wider than set %d (%d fields)", i, cur, i+1, next)
		}
	}
	if !containsFofaField(fofaFieldSets[0], "lastupdatetime") {
		t.Error("widest field set must request lastupdatetime so LastSeen is populated")
	}
	// The second tier (no icon_hash) must also request lastupdatetime, so a
	// degraded query (icon_hash permission denied) still populates LastSeen.
	if !containsFofaField(fofaFieldSets[1], "lastupdatetime") {
		t.Error("no-icon_hash tier must still request lastupdatetime so LastSeen survives degradation")
	}
	if containsFofaField(fofaFieldSets[1], "icon_hash") {
		t.Error("second tier must exclude icon_hash (it triggers 820001 permission error)")
	}
}

func splitFofaFields(s string) []string {
	var out []string
	var cur string
	quoted := false
	for _, r := range s {
		switch r {
		case '"':
			quoted = !quoted
			cur += string(r)
		case ',':
			if quoted {
				cur += string(r)
			} else {
				out = append(out, cur)
				cur = ""
			}
		default:
			cur += string(r)
		}
	}
	out = append(out, cur)
	return out
}

func containsFofaField(s, want string) bool {
	for _, f := range splitFofaFields(s) {
		if f == want {
			return true
		}
	}
	return false
}

// TestParseFofaSearchResponseExtras ensures columns not declared on FofaItem
// still land in Extra after a full parse round-trip.
func TestParseFofaSearchResponseExtras(t *testing.T) {
	body := []byte(`{"error":false,"total":1,"results":[["1.2.3.4","80","http","title","2026-08-06 09:00:00","custom-val"]]}`)
	fields := "ip,port,protocol,title,lastupdatetime,custom_field"
	var result *model.EngineResult
	if err := parseFofaSearchResponse(body, fields, 1, 100, "fofa", &result); err != nil {
		t.Fatalf("parseFofaSearchResponse error: %v", err)
	}
	if len(result.RawData) != 1 {
		t.Fatalf("expected 1 raw entry, got %d", len(result.RawData))
	}
	item, ok := result.RawData[0].(*FofaItem)
	if !ok {
		t.Fatalf("raw entry type = %T", result.RawData[0])
	}
	if item.Extra["custom_field"] != "custom-val" {
		t.Errorf("Extra[custom_field] = %v", item.Extra["custom_field"])
	}
	if item.Lastupdatetime != "2026-08-06 09:00:00" {
		t.Errorf("Lastupdatetime = %q", item.Lastupdatetime)
	}
}
