package adapter

import (
	"encoding/json"
	"testing"

	"github.com/unimap/project/internal/model"
)

// --- Hunter: unknown top-level keys → Extra, updated_at → LastSeen ---

func TestHunterDecodeCapturesUnknownKeys(t *testing.T) {
	data := []byte(`{
		"ip":"1.2.3.4","port":80,"protocol":"http","domain":"example.com",
		"web_title":"t","header_server":"nginx","status_code":200,
		"country":"中国","province":"浙江","city":"杭州","isp":"ISP","as_org":"Org",
		"url":"http://example.com",
		"updated_at":"2026-08-06T09:00:00Z",
		"company":{"name":"Example"}
	}`)
	var item HunterItem
	if err := json.Unmarshal(data, &item); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if item.IP != "1.2.3.4" || item.WebTitle != "t" {
		t.Errorf("declared fields lost: %+v", item)
	}
	if item.Extra["updated_at"] != "2026-08-06T09:00:00Z" {
		t.Errorf("Extra[updated_at] = %v", item.Extra["updated_at"])
	}
	company, ok := item.Extra["company"].(map[string]interface{})
	if !ok || company["name"] != "Example" {
		t.Errorf("Extra[company] = %v, want {name: Example}", item.Extra["company"])
	}

	asset := normalizeHunterMatch(&item)
	if asset == nil {
		t.Fatal("normalizeHunterMatch returned nil")
	}
	if asset.LastSeen != "2026-08-06T09:00:00Z" {
		t.Errorf("LastSeen = %q, want updated_at promoted", asset.LastSeen)
	}
	if asset.Extra == nil {
		t.Fatal("expected non-nil Extra")
	}
	if asset.Extra["company"] == nil {
		t.Error("Extra[company] missing")
	}
	if asset.Extra["updated_at"] == nil {
		t.Error("Extra[updated_at] missing (must be preserved alongside promotion)")
	}
}

// --- Quake: unknown top-level keys + nested service/http/location extras ---

func TestQuakeDecodeCapturesUnknownKeys(t *testing.T) {
	data := []byte(`{
		"ip":"1.2.3.4","port":443,"hostname":"example.com","domain":"example.com","url":"https://example.com",
		"time":"2026-08-06 09:00:00",
		"asn":"AS12345","org":"Example Org","isp":"Example ISP",
		"service":{"name":"https","status_code":200,"banner":"HTTP/1.1 200 OK"},
		"location":{"country_cn":"中国","province_cn":"浙江","city_cn":"杭州"}
	}`)
	var item QuakeItem
	if err := json.Unmarshal(data, &item); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if item.Extra["time"] != "2026-08-06 09:00:00" {
		t.Errorf("Extra[time] = %v", item.Extra["time"])
	}
	if item.Service == nil || item.Service.Extra["banner"] != "HTTP/1.1 200 OK" {
		t.Errorf("service Extra = %+v", item.Service)
	}
	if item.Location == nil || item.Location.Extra["country_cn"] != "中国" {
		t.Errorf("location Extra = %+v", item.Location)
	}

	asset := normalizeQuakeItem(&item, "quake")
	if asset == nil {
		t.Fatal("normalizeQuakeItem returned nil")
	}
	if asset.LastSeen != "2026-08-06 09:00:00" {
		t.Errorf("LastSeen = %q, want time promoted", asset.LastSeen)
	}
	if asset.Extra["asn"] != "AS12345" {
		t.Errorf("Extra[asn] = %v", asset.Extra["asn"])
	}
	if asset.Extra["banner"] != "HTTP/1.1 200 OK" {
		t.Errorf("Extra[banner] (from nested service) = %v", asset.Extra["banner"])
	}
	if asset.Extra["country_cn"] != "中国" {
		t.Errorf("Extra[country_cn] (from nested location) = %v", asset.Extra["country_cn"])
	}
}

// --- Shodan: location extraction + http object preserved + timestamp → LastSeen ---

func TestShodanDecodeCapturesUnknownKeys(t *testing.T) {
	data := []byte(`{
		"ip_str":"1.2.3.4","port":443,"transport":"https","hostnames":["example.com"],
		"country_code":"CN","region_code":"33","city":"Hangzhou",
		"asn":"AS12345","org":"Example Org","isp":"Example ISP",
		"timestamp":"2026-08-06T17:33:27",
		"location":{"country_code":"CN","country_name":"China","region_code":"33","city":"Hangzhou"},
		"http":{"title":"Example","server":"nginx","status_code":200,"html":"<html>hi</html>","robots":"noindex"},
		"ssl":{"cert":{"subject":{"CN":"example.com"}}}
	}`)
	var match ShodanMatch
	if err := json.Unmarshal(data, &match); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	// HTTP decodes as a map even with numeric status_code (previous map[string]string would fail).
	if match.HTTP == nil {
		t.Fatal("http object missing")
	}
	if match.HTTP["status_code"] != float64(200) {
		t.Errorf("http.status_code = %v", match.HTTP["status_code"])
	}
	if match.Extra["timestamp"] != "2026-08-06T17:33:27" {
		t.Errorf("Extra[timestamp] = %v", match.Extra["timestamp"])
	}
	if match.Extra["ssl"] == nil {
		t.Error("Extra[ssl] missing")
	}

	asset := normalizeShodanMatch(&match)
	if asset == nil {
		t.Fatal("normalizeShodanMatch returned nil")
	}
	if asset.CountryCode != "CN" || asset.Region != "33" || asset.City != "Hangzhou" {
		t.Errorf("geo from location: %q/%q/%q", asset.CountryCode, asset.Region, asset.City)
	}
	if asset.Title != "Example" || asset.Server != "nginx" || asset.StatusCode != 200 {
		t.Errorf("http fields: Title=%q Server=%q StatusCode=%d", asset.Title, asset.Server, asset.StatusCode)
	}
	if asset.LastSeen != "2026-08-06T17:33:27" {
		t.Errorf("LastSeen = %q, want timestamp promoted", asset.LastSeen)
	}
	if asset.Extra["http"] == nil {
		t.Error("Extra[http] missing (whole http object preserved)")
	}
	if asset.Extra["timestamp"] == nil {
		t.Error("Extra[timestamp] missing")
	}
}

// --- ZoomEye: unknown top-level keys → Extra + last_seen → LastSeen ---

func TestZoomEyeDecodeCapturesUnknownKeys(t *testing.T) {
	data := []byte(`{
		"ip":"1.2.3.4","port":8080,"service":"http","title":"Example","server":"nginx",
		"asn":12345,"org":"Example Org","isp":"Example ISP",
		"last_seen":"2026-08-06T09:00:00Z",
		"timestamp":"2026-08-06 17:33:27",
		"protocol":"http","banner":"HTTP/1.1 200 OK"
	}`)
	var item ZoomEyeItem
	if err := json.Unmarshal(data, &item); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if item.Extra["timestamp"] != "2026-08-06 17:33:27" {
		t.Errorf("Extra[timestamp] = %v", item.Extra["timestamp"])
	}

	asset := normalizeZoomEyeItem(&item, "zoomeye")
	if asset == nil {
		t.Fatal("normalizeZoomEyeItem returned nil")
	}
	if asset.LastSeen != "2026-08-06T09:00:00Z" {
		t.Errorf("LastSeen = %q, want last_seen", asset.LastSeen)
	}
	if asset.Extra["timestamp"] == nil {
		t.Error("Extra[timestamp] missing")
	}
}

// --- DayDayMap: unknown top-level keys → Extra ---

func TestDayDayMapDecodeCapturesUnknownKeys(t *testing.T) {
	data := []byte(`{
		"ip":"1.2.3.4","port":80,"protocol":"http","domain":"example.com",
		"title":"t","server":"nginx","body":"b","status_code":200,
		"country":"CN","province":"浙江","city":"杭州","asn":"AS12345","org":"Org","isp":"ISP",
		"last_seen":"2026-08-06 09:00:00","product":"nginx"
	}`)
	var item DayDayMapItem
	if err := json.Unmarshal(data, &item); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if item.Extra["last_seen"] != "2026-08-06 09:00:00" {
		t.Errorf("Extra[last_seen] = %v", item.Extra["last_seen"])
	}
	if item.Extra["product"] != "nginx" {
		t.Errorf("Extra[product] = %v", item.Extra["product"])
	}

	asset := normalizeDayDayMapItem(&item, "daydaymap")
	if asset == nil {
		t.Fatal("normalizeDayDayMapItem returned nil")
	}
	if asset.Extra["product"] != "nginx" {
		t.Errorf("Extra[product] = %v", asset.Extra["product"])
	}
}

// TestDayDayMapNormalizePromotesTimeStamp verifies DayDayMap's real timestamp
// field (time_stamp, observed in live API responses) reaches LastSeen while
// remaining preserved under Extra.
func TestDayDayMapNormalizePromotesTimeStamp(t *testing.T) {
	data := []byte(`{
		"ip":"1.2.3.4","port":80,"protocol":"http","domain":"example.com",
		"title":"t","server":"nginx","body":"b","status_code":200,
		"country":"CN","province":"浙江","city":"杭州","asn":"AS12345","org":"Org","isp":"ISP",
		"time_stamp":"2026-08-06 09:00:00","product":"nginx"
	}`)
	var item DayDayMapItem
	if err := json.Unmarshal(data, &item); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if item.Extra["time_stamp"] != "2026-08-06 09:00:00" {
		t.Errorf("Extra[time_stamp] = %v", item.Extra["time_stamp"])
	}

	asset := normalizeDayDayMapItem(&item, "daydaymap")
	if asset == nil {
		t.Fatal("normalizeDayDayMapItem returned nil")
	}
	if asset.LastSeen != "2026-08-06 09:00:00" {
		t.Errorf("LastSeen = %q, want time_stamp promoted", asset.LastSeen)
	}
	if asset.Extra["time_stamp"] == nil {
		t.Error("Extra[time_stamp] missing (must be preserved alongside promotion)")
	}
}

// --- Censys: host-level extras + last_updated_at → LastSeen ---

func TestCensysDecodeCapturesUnknownKeys(t *testing.T) {
	data := []byte(`{
		"result":{"resource":{
			"ip":"1.2.3.4",
			"last_updated_at":"2026-08-06T09:00:00Z",
			"operating_system":{"product":"Linux"},
			"services":[{"port":443,"service_name":"HTTPS"}]
		}}
	}`)
	var result *model.EngineResult
	if err := parseCensysV3HostResponse(data, 1, 100, "censys", &result); err != nil {
		t.Fatalf("parseCensysV3HostResponse error: %v", err)
	}
	if len(result.RawData) != 1 {
		t.Fatalf("expected 1 raw entry, got %d", len(result.RawData))
	}
	entry, ok := result.RawData[0].(*CensysRawEntry)
	if !ok {
		t.Fatalf("raw entry type = %T", result.RawData[0])
	}
	if entry.Extra["operating_system"] == nil {
		t.Error("Extra[operating_system] missing (host-level undeclared key)")
	}
	assets, err := (&CensysAdapter{client: nil, baseURL: "", apiID: "", apiSecret: "", qps: 0}).Normalize(result)
	if err != nil {
		t.Fatalf("normalize error: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
	if assets[0].LastSeen != "2026-08-06T09:00:00Z" {
		t.Errorf("LastSeen = %q, want last_updated_at", assets[0].LastSeen)
	}
	if assets[0].Extra["operating_system"] == nil {
		t.Error("asset Extra[operating_system] missing")
	}
}
