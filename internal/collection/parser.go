package collection

import (
	"regexp"
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

// ParseStructuredCollectedDataFromItems extracts assets from typed CollectedDataItem slice.
func ParseStructuredCollectedDataFromItems(items []model.CollectedDataItem, engine string, hasMore bool) ([]model.UnifiedAsset, int, bool) {
	if len(items) == 0 {
		return []model.UnifiedAsset{}, 0, false
	}
	assets := make([]model.UnifiedAsset, 0, len(items))
	for _, item := range items {
		asset := model.UnifiedAsset{
			IP:          item.IP,
			Port:        item.Port,
			Protocol:    item.Protocol,
			Host:        item.Host,
			URL:         item.URL,
			Title:       item.Title,
			BodySnippet: item.BodySnippet,
			Server:      item.Server,
			StatusCode:  item.StatusCode,
			CountryCode: item.CountryCode,
			Region:      item.Region,
			City:        item.City,
			ASN:         item.ASN,
			Org:         item.Org,
			ISP:         item.ISP,
			Source:      engine,
		}
		// Map Product to Title when Title is empty (e.g. Hunter engine)
		if asset.Title == "" && item.Product != "" {
			asset.Title = item.Product
		}
		// Post-process: extract port from host field (e.g. "1.2.3.4:8080" → port=8080)
		if asset.Port == 0 && asset.Host != "" {
			if idx := strings.LastIndex(asset.Host, ":"); idx > 0 {
				if p, err := strconv.Atoi(asset.Host[idx+1:]); err == nil && p > 0 && p < 65536 {
					asset.Port = p
					asset.Host = asset.Host[:idx]
				}
			}
		}
		// Post-process: extract port from IP field (e.g. "1.2.3.4:443" → port=443)
		if asset.Port == 0 && asset.IP != "" {
			if idx := strings.LastIndex(asset.IP, ":"); idx > 0 {
				if p, err := strconv.Atoi(asset.IP[idx+1:]); err == nil && p > 0 && p < 65536 {
					asset.Port = p
					asset.IP = asset.IP[:idx]
				}
			}
		}
		// Post-process: infer port from protocol if still missing
		if asset.Port == 0 && asset.Protocol != "" {
			asset.Port = defaultPortForProtocol(asset.Protocol)
		}
		assets = append(assets, asset)
	}
	return assets, len(items), hasMore
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
	// Post-process: extract port from host or IP if port is missing
	if asset.Port == 0 {
		asset.Port, asset.Host = extractPortFromHost(asset.Host)
	}
	if asset.Port == 0 {
		asset.Port, asset.IP = extractPortFromHost(asset.IP)
	}
	// Post-process: extract port from protocol field (e.g. "8081 http" → port=8081)
	if asset.Port == 0 && asset.Protocol != "" {
		if m := regexp.MustCompile(`(\d{1,5})`).FindStringSubmatch(asset.Protocol); len(m) > 1 {
			if p, err := strconv.Atoi(m[1]); err == nil && p > 0 && p < 65536 {
				asset.Port = p
			}
		}
	}
	// Post-process: infer port from protocol name if still missing
	if asset.Port == 0 && asset.Protocol != "" {
		asset.Port = defaultPortForProtocol(asset.Protocol)
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

// extractPortFromHost extracts port from "host:port" string. Returns (port, cleanHost).
func extractPortFromHost(s string) (int, string) {
	if idx := strings.LastIndex(s, ":"); idx > 0 {
		if p, err := strconv.Atoi(s[idx+1:]); err == nil && p > 0 && p < 65536 {
			return p, s[:idx]
		}
	}
	return 0, s
}

// defaultPortForProtocol returns the standard port for a protocol name.
func defaultPortForProtocol(proto string) int {
	switch strings.ToLower(strings.TrimSpace(proto)) {
	case "http", "http/server":
		return 80
	case "https", "ssl/http", "http/ssl", "tls":
		return 443
	case "ssh":
		return 22
	case "ftp":
		return 21
	case "smtp", "smtps", "ssl/smtp", "smtp/ssl":
		return 25
	case "pop3", "pop3s", "ssl/pop3", "pop3/ssl":
		return 110
	case "imap", "imaps", "ssl/imap", "imap/ssl":
		return 143
	case "mysql":
		return 3306
	case "rdp", "ms-wbt-server":
		return 3389
	case "smb", "microsoft-ds":
		return 445
	case "dns":
		return 53
	case "redis":
		return 6379
	}
	return 0
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
		// Post-process: extract port from host or IP if port is missing
		if a.Port == 0 {
			a.Port, a.Host = extractPortFromHost(a.Host)
		}
		if a.Port == 0 {
			a.Port, a.IP = extractPortFromHost(a.IP)
		}
		// Post-process: infer port from protocol if still missing
		if a.Port == 0 && a.Protocol != "" {
			a.Port = defaultPortForProtocol(a.Protocol)
		}
		if a.IP == "" && a.Host == "" && a.URL == "" {
			continue
		}
		assets = append(assets, a)
	}
	return assets
}
