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
	LastSeen    string         `json:"last_seen,omitempty"` // last probe time (e.g. Shodan timestamp)
	Extra       map[string]any `json:"extra,omitempty"`
}

// BrowserCookie is the credential handoff shape returned by the loopback
// Extension Bridge. Values are kept in memory/config only and must never be
// written to logs or API diagnostics.
type BrowserCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain,omitempty"`
	Path     string `json:"path,omitempty"`
	HTTPOnly bool   `json:"httpOnly,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
}

// BrowserStorage contains same-origin Web Storage copied from the paired
// browser profile. It is intentionally held in memory only: callers must not
// log it or expose it in API diagnostics.
type BrowserStorage struct {
	Local   map[string]string `json:"local,omitempty"`
	Session map[string]string `json:"session,omitempty"`
}

// BridgeCollectedData replaces map[string]interface{} in BridgeResult.StructuredCollectedData.
type BridgeCollectedData struct {
	Engine           string              `json:"engine,omitempty"`
	Title            string              `json:"title,omitempty"`
	Total            int                 `json:"total,omitempty"`
	HasMore          bool                `json:"has_more,omitempty"`
	IsLoginWall      bool                `json:"is_login_wall,omitempty"`
	LoginRequired    bool                `json:"login_required,omitempty"`
	ExtractionMethod string              `json:"extraction_method,omitempty"`
	RowSelectorUsed  string              `json:"row_selector_used,omitempty"`
	RowsFound        int                 `json:"rows_found,omitempty"`
	ExtractionError  string              `json:"extraction_error,omitempty"`
	Items            []CollectedDataItem `json:"items,omitempty"`
	Cookies          []BrowserCookie     `json:"cookies,omitempty"`
	Storage          *BrowserStorage     `json:"storage,omitempty"`
	LoginDiagnostics map[string]any      `json:"login_diagnostics,omitempty"`
	Extra            map[string]any      `json:"extra,omitempty"`
}
