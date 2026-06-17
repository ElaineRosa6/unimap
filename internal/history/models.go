package history

import "time"

// OperationType identifies the type of operation.
type OperationType string

const (
	OpTypeQuery       OperationType = "query"
	OpTypeICPQuery    OperationType = "icp_query"
	OpTypePortScan    OperationType = "port_scan"
	OpTypeScreenshot  OperationType = "screenshot"
	OpTypeTamperCheck OperationType = "tamper_check"
)

// OperationHistory represents one executed operation.
type OperationHistory struct {
	ID            int64         `json:"id"`
	OperationType OperationType `json:"operation_type"`
	Input         string        `json:"input"`
	Status        string        `json:"status"`
	TotalCount    int           `json:"total_count"`
	Summary       string        `json:"summary,omitempty"`
	DurationMS    int64         `json:"duration_ms,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
}

// OperationResult represents a single result row for an operation.
type OperationResult struct {
	ID        int64  `json:"id"`
	HistoryID int64  `json:"history_id"`
	Data      string `json:"data"`
}
