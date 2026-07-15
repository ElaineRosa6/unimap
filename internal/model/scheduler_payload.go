package model

// TaskPayload is the typed parameter bag for scheduled tasks.
// Each task type uses known fields; Extra holds engine-specific params.
type TaskPayload struct {
	// Common fields
	Query    string   `json:"query,omitempty"`
	Engines  []string `json:"engines,omitempty"`
	PageSize int      `json:"page_size,omitempty"`
	// NotificationDetailLimit controls how many persisted query assets are
	// expanded in a task notification. QueryRunner defaults to 50 and caps at
	// 100 so notification channels remain usable while SQLite retains all rows.
	NotificationDetailLimit int    `json:"notification_detail_limit,omitempty"`
	Format                  string `json:"format,omitempty"`
	DetectMode              string `json:"detection_mode,omitempty"`
	MaxAgeDays              int    `json:"max_age_days,omitempty"`
	LowThresh               int    `json:"low_threshold,omitempty"`
	TimeoutSec              int    `json:"timeout_seconds,omitempty"`

	// Optional browser/Bridge query workflow. When BrowserQuery is enabled for
	// a query task, BrowserAction must be collect_and_capture so the scheduler
	// can persist the collected assets and attach the evidence screenshot to
	// the task notification.
	BrowserQuery  bool   `json:"browser_query,omitempty"`
	BrowserAction string `json:"browser_action,omitempty"`
	QueryID       string `json:"query_id,omitempty"`

	// ICP-specific
	Queries     []string `json:"queries,omitempty"`
	Type        string   `json:"type,omitempty"`
	Page        int      `json:"page,omitempty"`
	PageSizeICP int      `json:"icp_page_size,omitempty"`

	// Batch screenshot
	URLs []string `json:"urls,omitempty"`

	// Tamper check
	URL string `json:"url,omitempty"`

	// Cookie verify
	CookieFile string `json:"cookie_file,omitempty"`

	Extra map[string]any `json:"extra,omitempty"`
}

// TaskOutput is the typed result returned by task handlers.
type TaskOutput struct {
	Message    string         `json:"message,omitempty"`
	Total      int            `json:"total,omitempty"`
	Success    int            `json:"success,omitempty"`
	Failed     int            `json:"failed,omitempty"`
	DurationMS int64          `json:"duration_ms,omitempty"`
	Extra      map[string]any `json:"extra,omitempty"`
}
