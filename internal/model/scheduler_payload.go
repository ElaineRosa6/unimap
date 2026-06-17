package model

// TaskPayload is the typed parameter bag for scheduled tasks.
// Each task type uses known fields; Extra holds engine-specific params.
type TaskPayload struct {
	// Common fields
	Query      string   `json:"query,omitempty"`
	Engines    []string `json:"engines,omitempty"`
	PageSize   int      `json:"page_size,omitempty"`
	Format     string   `json:"format,omitempty"`
	DetectMode string   `json:"detection_mode,omitempty"`
	MaxAgeDays int      `json:"max_age_days,omitempty"`
	LowThresh  int      `json:"low_threshold,omitempty"`
	TimeoutSec int      `json:"timeout_seconds,omitempty"`

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
