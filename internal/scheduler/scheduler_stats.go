package scheduler

import "sort"

// TaskExecutionStats holds statistical analysis of task execution history.
type TaskExecutionStats struct {
	TaskID        string  `json:"task_id"`
	TaskName      string  `json:"task_name"`
	TaskType      string  `json:"task_type"`
	TotalRuns     int     `json:"total_runs"`
	SuccessCount  int     `json:"success_count"`
	FailedCount   int     `json:"failed_count"`
	TimeoutCount  int     `json:"timeout_count"`
	SkippedCount  int     `json:"skipped_count"`
	SuccessRate   float64 `json:"success_rate"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
	MaxDurationMs int64   `json:"max_duration_ms"`
	MinDurationMs int64   `json:"min_duration_ms"`
	P50DurationMs int64   `json:"p50_duration_ms"`
	P95DurationMs int64   `json:"p95_duration_ms"`
	TotalRetries  int     `json:"total_retries"`
	LastSuccessAt string  `json:"last_success_at,omitempty"`
	LastFailureAt string  `json:"last_failure_at,omitempty"`
}

// GetTaskExecutionStats analyzes execution history for a specific task.
func (s *Scheduler) GetTaskExecutionStats(taskID string) *TaskExecutionStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var task *ScheduledTask
	if t, ok := s.tasks[taskID]; ok {
		task = t
	}

	stats := &TaskExecutionStats{
		TaskID:        taskID,
		MinDurationMs: 0,
	}
	if task != nil {
		stats.TaskName = task.Name
		stats.TaskType = string(task.Type)
	}

	var durations []int64
	for _, record := range s.history {
		if record.TaskID != taskID {
			continue
		}

		stats.TotalRuns++
		durations = append(durations, record.DurationMs)

		switch record.Status {
		case "success":
			stats.SuccessCount++
			stats.LastSuccessAt = record.FinishedAt
		case "failed":
			stats.FailedCount++
			stats.LastFailureAt = record.FinishedAt
		case "timeout":
			stats.TimeoutCount++
			stats.LastFailureAt = record.FinishedAt
		case "skipped":
			stats.SkippedCount++
		}

		stats.TotalRetries += record.RetryCount

		if record.DurationMs > stats.MaxDurationMs {
			stats.MaxDurationMs = record.DurationMs
		}
		if stats.MinDurationMs < 0 || record.DurationMs < stats.MinDurationMs {
			stats.MinDurationMs = record.DurationMs
		}
	}

	if stats.TotalRuns > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalRuns) * 100

		var totalDuration int64
		for _, d := range durations {
			totalDuration += d
		}
		stats.AvgDurationMs = float64(totalDuration) / float64(len(durations))

		sortInt64(durations)
		if len(durations) > 0 {
			stats.MinDurationMs = durations[0]
			stats.MaxDurationMs = durations[len(durations)-1]
			stats.P50DurationMs = durations[len(durations)*50/100]
			stats.P95DurationMs = durations[len(durations)*95/100]
		}
	}

	return stats
}

// GetAllTasksStats returns execution stats for all tasks.
func (s *Scheduler) GetAllTasksStats() []*TaskExecutionStats {
	s.mu.RLock()
	taskIDs := make([]string, 0, len(s.tasks))
	for id := range s.tasks {
		taskIDs = append(taskIDs, id)
	}
	s.mu.RUnlock()

	stats := make([]*TaskExecutionStats, 0, len(taskIDs))
	for _, id := range taskIDs {
		stats = append(stats, s.GetTaskExecutionStats(id))
	}
	return stats
}

// GetRecentExecutions returns the most recent execution records.
func (s *Scheduler) GetRecentExecutions(limit int) []ExecutionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}

	start := len(s.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]ExecutionRecord, limit)
	copy(result, s.history[start:])
	return result
}

func sortInt64(s []int64) {
	sort.Slice(s, func(i, j int) bool {
		return s[i] < s[j]
	})
}
