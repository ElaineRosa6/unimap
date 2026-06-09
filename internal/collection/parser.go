package collection

import (
	"strconv"
	"strings"

	"github.com/unimap/project/internal/model"
)

// ParseStructuredCollectedData extracts assets, total count, and has_more from
// the extension's structured collect payload.
func ParseStructuredCollectedData(data map[string]interface{}, engine string) ([]model.UnifiedAsset, int, bool) {
	total := 0
	hasMore := false
	if t, ok := data["total"].(float64); ok {
		total = int(t)
	}
	if hm, ok := data["has_more"].(bool); ok {
		hasMore = hm
	}
	rawItems, ok := data["items"]
	if !ok {
		return []model.UnifiedAsset{}, total, hasMore
	}
	items, ok := rawItems.([]interface{})
	if !ok {
		return []model.UnifiedAsset{}, total, hasMore
	}
	assets := make([]model.UnifiedAsset, 0, len(items))
	for _, raw := range items {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		assets = append(assets, ParseAssetItem(item, engine))
	}
	return assets, total, hasMore
}

// ParseAssetItem converts a single item map into a UnifiedAsset.
func ParseAssetItem(item map[string]interface{}, engine string) model.UnifiedAsset {
	asset := model.UnifiedAsset{Source: engine}
	if v, ok := item["url"].(string); ok {
		asset.URL = v
	}
	if v, ok := item["title"].(string); ok {
		asset.Title = v
	}
	if v, ok := item["ip"].(string); ok {
		asset.IP = v
	}
	asset.Port = ParseIntField(item, "port")
	if v, ok := item["protocol"].(string); ok {
		asset.Protocol = v
	}
	if v, ok := item["host"].(string); ok {
		asset.Host = v
	}
	if v, ok := item["body_snippet"].(string); ok && v != "" {
		asset.BodySnippet = v
	} else if v, ok := item["banner"].(string); ok && v != "" {
		asset.BodySnippet = v
	}
	if v, ok := item["server"].(string); ok {
		asset.Server = v
	}
	asset.StatusCode = ParseIntField(item, "status_code")
	if v, ok := item["country_code"].(string); ok {
		asset.CountryCode = v
	}
	if v, ok := item["region"].(string); ok {
		asset.Region = v
	}
	if v, ok := item["city"].(string); ok {
		asset.City = v
	}
	if v, ok := item["asn"].(string); ok {
		asset.ASN = v
	}
	if v, ok := item["org"].(string); ok {
		asset.Org = v
	}
	if v, ok := item["isp"].(string); ok {
		asset.ISP = v
	}
	if v, ok := item["country"].(string); ok && asset.CountryCode == "" {
		asset.CountryCode = v
	}
	if v, ok := item["product"].(string); ok {
		if asset.Title == "" {
			asset.Title = v
		}
	}
	asset.Extra = ExtractExtraFields(item)
	return asset
}

// ExtractExtraFields collects unrecognized fields into an Extra map.
func ExtractExtraFields(item map[string]interface{}) map[string]interface{} {
	known := map[string]bool{
		"url": true, "title": true, "ip": true, "port": true,
		"protocol": true, "host": true, "body_snippet": true,
		"banner": true, "server": true, "status_code": true,
		"country_code": true, "region": true, "city": true,
		"asn": true, "org": true, "isp": true, "os": true,
		"country": true, "product": true, "source": true,
	}
	extra := make(map[string]interface{})
	for k, v := range item {
		if !known[k] {
			extra[k] = v
		}
	}
	if len(extra) == 0 {
		return nil
	}
	return extra
}

// ParseIntField extracts an int value from a map, supporting float64 and string types.
func ParseIntField(data map[string]interface{}, key string) int {
	if v, ok := data[key].(float64); ok {
		return int(v)
	}
	if v, ok := data[key].(string); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return 0
}

// ParseExtractedAssets converts raw JS-extracted maps into UnifiedAsset structs.
func ParseExtractedAssets(raw []map[string]interface{}, engine string) []model.UnifiedAsset {
	assets := make([]model.UnifiedAsset, 0, len(raw))
	for _, row := range raw {
		a := model.UnifiedAsset{Source: engine}
		if v, ok := row["ip"].(string); ok {
			a.IP = v
		}
		a.Port = ParseIntField(row, "port")
		if v, ok := row["protocol"].(string); ok {
			a.Protocol = v
		}
		if v, ok := row["host"].(string); ok {
			a.Host = v
		}
		if v, ok := row["url"].(string); ok {
			a.URL = v
		}
		if v, ok := row["title"].(string); ok {
			a.Title = v
		}
		if v, ok := row["server"].(string); ok {
			a.Server = v
		}
		if v, ok := row["country"].(string); ok {
			a.CountryCode = v
		}
		if v, ok := row["region"].(string); ok {
			a.Region = v
		}
		if v, ok := row["city"].(string); ok {
			a.City = v
		}
		if v, ok := row["asn"].(string); ok {
			a.ASN = v
		}
		if v, ok := row["org"].(string); ok {
			a.Org = v
		}
		if v, ok := row["isp"].(string); ok {
			a.ISP = v
		}
		if v, ok := row["body_snippet"].(string); ok {
			a.BodySnippet = v
		}
		a.StatusCode = ParseIntField(row, "status_code")
		if v, ok := row["source"].(string); ok {
			a.Source = v
		}
		if a.IP == "" && a.Host == "" && a.URL == "" {
			continue
		}
		assets = append(assets, a)
	}
	return assets
}
