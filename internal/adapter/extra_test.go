package adapter

import (
	"encoding/json"
	"testing"

	"github.com/unimap/project/internal/model"
)

// TestRawUnknown ensures unknown top-level JSON keys are captured into Extra
// while declared fields still decode into the typed struct.
func TestRawUnknown(t *testing.T) {
	type sample struct {
		IP    string `json:"ip"`
		Title string `json:"title"`
	}
	data := []byte(`{"ip":"1.2.3.4","title":"t","os":"linux","cert":{"cn":"example.com"}}`)
	var s sample
	extra, err := rawUnknown(data, &s)
	if err != nil {
		t.Fatalf("rawUnknown error: %v", err)
	}
	if s.IP != "1.2.3.4" || s.Title != "t" {
		t.Errorf("declared fields not decoded: %+v", s)
	}
	if extra["os"] != "linux" {
		t.Errorf("Extra[os] = %v, want linux", extra["os"])
	}
	cert, ok := extra["cert"].(map[string]interface{})
	if !ok || cert["cn"] != "example.com" {
		t.Errorf("Extra[cert] = %v, want {cn: example.com}", extra["cert"])
	}
	if _, ok := extra["ip"]; ok {
		t.Error("declared field ip must not be captured into Extra")
	}
}

// TestExtraString covers the type coercion used when promoting timestamp keys.
func TestExtraString(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want string
	}{
		{"string", "2026-08-06 09:00:00", "2026-08-06 09:00:00"},
		{"nil", nil, ""},
		{"float64", float64(1690000000), "1690000000"},
		{"json.Number", json.Number("1690000000"), "1690000000"},
		{"bool", true, "true"},
		{"int", 42, ""}, // unsupported type → empty
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extraString(tt.in); got != tt.want {
				t.Errorf("extraString(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestPromoteLastSeen verifies known timestamp keys inside Extra fill LastSeen.
func TestPromoteLastSeen(t *testing.T) {
	t.Run("lastupdatetime promoted", func(t *testing.T) {
		asset := &model.UnifiedAsset{Extra: map[string]interface{}{"lastupdatetime": "2026-08-06 09:00:00"}}
		promoteLastSeen(asset)
		if asset.LastSeen != "2026-08-06 09:00:00" {
			t.Errorf("LastSeen = %q", asset.LastSeen)
		}
	})
	t.Run("updated_at promoted", func(t *testing.T) {
		asset := &model.UnifiedAsset{Extra: map[string]interface{}{"updated_at": "2026-08-06T09:00:00Z"}}
		promoteLastSeen(asset)
		if asset.LastSeen != "2026-08-06T09:00:00Z" {
			t.Errorf("LastSeen = %q", asset.LastSeen)
		}
	})
	t.Run("numeric timestamp promoted as string", func(t *testing.T) {
		asset := &model.UnifiedAsset{Extra: map[string]interface{}{"timestamp": float64(1690000000)}}
		promoteLastSeen(asset)
		if asset.LastSeen != "1690000000" {
			t.Errorf("LastSeen = %q", asset.LastSeen)
		}
	})
	t.Run("time_stamp promoted (DayDayMap real field)", func(t *testing.T) {
		asset := &model.UnifiedAsset{Extra: map[string]interface{}{"time_stamp": "2026-08-06 10:00:00"}}
		promoteLastSeen(asset)
		if asset.LastSeen != "2026-08-06 10:00:00" {
			t.Errorf("LastSeen = %q, want 2026-08-06 10:00:00", asset.LastSeen)
		}
	})
	t.Run("existing LastSeen not overwritten", func(t *testing.T) {
		asset := &model.UnifiedAsset{LastSeen: "already", Extra: map[string]interface{}{"lastupdatetime": "2026-08-06"}}
		promoteLastSeen(asset)
		if asset.LastSeen != "already" {
			t.Errorf("LastSeen = %q, want already", asset.LastSeen)
		}
	})
	t.Run("no timestamp keys leaves LastSeen empty", func(t *testing.T) {
		asset := &model.UnifiedAsset{Extra: map[string]interface{}{"os": "linux"}}
		promoteLastSeen(asset)
		if asset.LastSeen != "" {
			t.Errorf("LastSeen = %q, want empty", asset.LastSeen)
		}
	})
	t.Run("nil Extra no panic", func(t *testing.T) {
		asset := &model.UnifiedAsset{}
		promoteLastSeen(asset)
		if asset.LastSeen != "" {
			t.Errorf("LastSeen = %q, want empty", asset.LastSeen)
		}
	})
}

// TestJsonFieldNames ensures jsonFieldNames respects embedded structs and
// json:"-" exclusion.
func TestJsonFieldNames(t *testing.T) {
	type embedded struct {
		Secret string `json:"-"`
	}
	type outer struct {
		embedded
		IP    string `json:"ip"`
		Extra string `json:"-"`
	}
	names := jsonFieldNames(outer{})
	if !names["ip"] {
		t.Error("ip should be a known field")
	}
	if names["secret"] {
		t.Error("embedded json:\"-\" field must be excluded")
	}
	if names["extra"] {
		t.Error("json:\"-\" field must be excluded")
	}
}
