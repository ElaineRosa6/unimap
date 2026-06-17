package collection

import (
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/unimap/project/internal/model"
)

var rePortNum = regexp.MustCompile(`(\d{1,5})`)

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
	NormalizeAssets(engine, assets)
	return assets, total, hasMore
}

// ParseStructuredCollectedDataFromItems extracts assets from typed CollectedDataItem slice.
func ParseStructuredCollectedDataFromItems(items []model.CollectedDataItem, engine string, hasMore bool) ([]model.UnifiedAsset, int, bool) {
	if len(items) == 0 {
		return []model.UnifiedAsset{}, 0, hasMore
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
			LastSeen:    item.LastSeen,
			Source:      engine,
		}
		// Map Product to Title when Title is empty (e.g. Hunter engine)
		if asset.Title == "" && item.Product != "" {
			asset.Title = item.Product
		}
		// Post-process: extract port from host or IP if missing
		if asset.Port == 0 {
			asset.Port, asset.Host = extractPortFromHost(asset.Host)
		}
		if asset.Port == 0 {
			asset.Port, asset.IP = extractPortFromHost(asset.IP)
		}
		// Post-process: infer port from protocol if still missing
		if asset.Port == 0 && asset.Protocol != "" {
			asset.Port = defaultPortForProtocol(asset.Protocol)
		}
		assets = append(assets, asset)
	}
	NormalizeAssets(engine, assets)
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
		if m := rePortNum.FindStringSubmatch(asset.Protocol); len(m) > 1 {
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
	// last_seen: prefer "last_seen" key, fall back to legacy "timestamp"
	if v, ok := item["last_seen"].(string); ok && v != "" {
		asset.LastSeen = v
	} else if v, ok := item["timestamp"].(string); ok && v != "" {
		asset.LastSeen = v
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
		"timestamp": true, "last_seen": true,
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
	if s == "" {
		return 0, s
	}
	if host, portText, err := net.SplitHostPort(s); err == nil {
		if p, err := strconv.Atoi(portText); err == nil && p > 0 && p < 65536 {
			return p, host
		}
		return 0, s
	}
	if strings.Count(s, ":") != 1 {
		return 0, s
	}
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
		// last_seen: prefer "last_seen" key, fall back to legacy "timestamp"
		if v, ok := row["last_seen"].(string); ok && v != "" {
			a.LastSeen = v
		} else if v, ok := row["timestamp"].(string); ok && v != "" {
			a.LastSeen = v
		}
		if v, ok := row["body_snippet"].(string); ok {
			a.BodySnippet = v
		}
		if v, ok := row["banner"].(string); ok && v != "" && a.BodySnippet == "" {
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
	NormalizeAssets(engine, assets)
	return assets
}

// NormalizeAssets applies engine-specific cleanup after assets are parsed.
func NormalizeAssets(engine string, assets []model.UnifiedAsset) {
	if strings.EqualFold(strings.TrimSpace(engine), "hunter") {
		CleanHunterFields(assets)
	}
}

// CleanHunterFields fixes common Hunter DOM extraction artifacts.
// Called after parsing to normalize country, title, and host fields.
func CleanHunterFields(assets []model.UnifiedAsset) {
	for i := range assets {
		a := &assets[i]
		// Country: Chinese city/province names → "中国"
		if a.CountryCode != "" {
			cc := a.CountryCode
			if strings.ContainsAny(cc, "省市县区镇") || isPureChinese(cc) {
				a.CountryCode = "中国"
			}
		}
		// Host: remove UI filter artifacts
		if a.Host != "" {
			h := a.Host
			h = strings.ReplaceAll(h, "不看空域名", "")
			h = strings.TrimSpace(h)
			h = strings.Trim(h, " \t\r\n-")
			if h == "" || h == a.IP {
				a.Host = ""
			} else {
				a.Host = h
			}
		}
		// Title: strip trailing category labels
		if a.Title != "" {
			t := a.Title
			for _, label := range []string{"企业办公", "邮件系统", "开源", "企业", "个人", "政府", "金融", "邮件", "办公", "系统"} {
				if idx := strings.Index(t, label); idx > 0 {
					t = t[:idx]
				}
			}
			t = strings.TrimSpace(t)
			t = strings.Trim(t, " \t\r\n-")
			a.Title = t
		}
	}
}

func isPureChinese(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < 0x4e00 || r > 0x9fa5 {
			return false
		}
	}
	return true
}
