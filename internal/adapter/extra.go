package adapter

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"

	"github.com/unimap/project/internal/model"
)

// lastSeenKeys lists engine-specific keys (observed in real API responses)
// that carry the "last seen / last updated" timestamp for an asset. The
// unified model exposes this as UnifiedAsset.LastSeen.
var lastSeenKeys = []string{
	"lastupdatetime", "last_updated_at", "updated_at", "update_time",
	"last_seen", "timestamp", "time", "time_stamp",
}

// collectJSONFieldNames walks a struct type (recursing into anonymous
// embedded fields) collecting the JSON tag name of every exported field.
func collectJSONFieldNames(t reflect.Type, known map[string]bool) {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" { // unexported
			continue
		}
		if f.Anonymous {
			collectJSONFieldNames(f.Type, known)
			continue
		}
		tag := f.Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name != "" && name != "-" {
			known[name] = true
		}
	}
}

// jsonFieldNames returns the set of JSON tag names that the given struct
// (or struct pointer) knows how to decode. Used by rawUnknown to decide which
// top-level response keys are "known" and which must be preserved as extras.
func jsonFieldNames(v interface{}) map[string]bool {
	known := map[string]bool{}
	collectJSONFieldNames(reflect.TypeOf(v), known)
	return known
}

// rawUnknown decodes v from data, and returns every top-level JSON key that
// v's type does not declare. Undeclared keys carry engine-specific fields that
// encoding/json would otherwise silently drop.
//
// It must be called with a pointer to a type that has NO custom UnmarshalJSON
// (the standard type-alias pattern) to avoid recursion.
func rawUnknown(data []byte, v interface{}) (map[string]interface{}, error) {
	if err := json.Unmarshal(data, v); err != nil {
		return nil, err
	}
	known := jsonFieldNames(v)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	extra := make(map[string]interface{})
	for key, val := range raw {
		if known[key] {
			continue
		}
		var decoded interface{}
		if err := json.Unmarshal(val, &decoded); err == nil {
			extra[key] = decoded
		}
	}
	return extra, nil
}

// extraString coerces a decoded JSON value to a displayable string. Maps and
// slices (never single scalar timestamps) return "".
func extraString(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case nil:
		return ""
	case json.Number:
		return s.String()
	case float64:
		if s == float64(int64(s)) {
			return strconv.FormatInt(int64(s), 10)
		}
		return strconv.FormatFloat(s, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(s)
	default:
		return ""
	}
}

// promoteLastSeen fills asset.LastSeen from any engine-specific timestamp key
// still present in asset.Extra, if LastSeen is not already set.
func promoteLastSeen(asset *model.UnifiedAsset) {
	if asset.LastSeen != "" || asset.Extra == nil {
		return
	}
	for _, key := range lastSeenKeys {
		if v, ok := asset.Extra[key]; ok {
			if s := extraString(v); s != "" {
				asset.LastSeen = s
				return
			}
		}
	}
}

// mergeAssetExtra merges engine-specific extra fields into a UnifiedAsset's
// Extra map, preserving any pre-existing entries.
func mergeAssetExtra(asset *model.UnifiedAsset, extra map[string]interface{}) {
	if len(extra) == 0 {
		return
	}
	if asset.Extra == nil {
		asset.Extra = make(map[string]interface{}, len(extra))
	}
	for k, v := range extra {
		if _, exists := asset.Extra[k]; !exists {
			asset.Extra[k] = v
		}
	}
}

// applyExtras merges an engine item's captured unknown fields into the asset
// and promotes any timestamp key to UnifiedAsset.LastSeen.
func applyExtras(asset *model.UnifiedAsset, extra map[string]interface{}) {
	mergeAssetExtra(asset, extra)
	promoteLastSeen(asset)
}
