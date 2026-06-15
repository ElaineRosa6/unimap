package model

// CollectedDataItem represents a single item extracted by the extension.
type CollectedDataItem struct {
	IP          string         `json:"ip,omitempty"`
	Port        int            `json:"port,omitempty"`
	Protocol    string         `json:"protocol,omitempty"`
	Host        string         `json:"host,omitempty"`
	URL         string         `json:"url,omitempty"`
	Title       string         `json:"title,omitempty"`
	BodySnippet string         `json:"body_snippet,omitempty"`
	Server      string         `json:"server,omitempty"`
	StatusCode  int            `json:"status_code,omitempty"`
	CountryCode string         `json:"country_code,omitempty"`
	Region      string         `json:"region,omitempty"`
	City        string         `json:"city,omitempty"`
	ASN         string         `json:"asn,omitempty"`
	Org         string         `json:"org,omitempty"`
	ISP         string         `json:"isp,omitempty"`
	Product     string         `json:"product,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}

// BridgeCollectedData replaces map[string]interface{} in BridgeResult.StructuredCollectedData.
type BridgeCollectedData struct {
	Engine string              `json:"engine,omitempty"`
	Total  int                 `json:"total,omitempty"`
	HasMore bool              `json:"has_more,omitempty"`
	Items  []CollectedDataItem `json:"items,omitempty"`
	Extra  map[string]any      `json:"extra,omitempty"`
}
