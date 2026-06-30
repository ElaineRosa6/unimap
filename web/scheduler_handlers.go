package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/scheduler"
)

const maxPayloadKeys = 50
const maxPayloadSizeBytes = 64 * 1024

func writeSchedulerJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": msg,
	})
}

func validateTaskPayload(payload map[string]interface{}) error {
	if payload == nil {
		return nil
	}
	if len(payload) > maxPayloadKeys {
		return fmt.Errorf("payload exceeds maximum of %d keys", maxPayloadKeys)
	}
	serialized, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("payload serialization failed: %w", err)
	}
	if len(serialized) > maxPayloadSizeBytes {
		return fmt.Errorf("payload exceeds maximum size of %d bytes", maxPayloadSizeBytes)
	}
	if webhookURL, ok := payload["webhook_url"].(string); ok && webhookURL != "" {
		if err := scheduler.ValidateWebhookURLPublic(webhookURL); err != nil {
			return fmt.Errorf("payload webhook_url invalid: %w", err)
		}
	}
	return nil
}

// handleSchedulerPage renders the scheduler management page.
func (s *Server) handleSchedulerPage(w http.ResponseWriter, r *http.Request) {
	// Pre-compute task type labels as map[string]string to avoid
	// html/template type conversion failure when indexing a map with
	// interface{} keys from range loops.
	taskTypes := make([]string, 0, 8)
	taskTypeLabels := make(map[string]string)
	for _, tt := range scheduler.AllTaskTypes() {
		s := string(tt)
		taskTypes = append(taskTypes, s)
		taskTypeLabels[s] = scheduler.TaskTypeLabel(tt)
	}

	// Build grouped task types for the <optgroup> select. Each option carries
	// its value+label so the template needs no further map lookups.
	type typeOption struct {
		Value string
		Label string
	}
	type taskGroupView struct {
		Name  string
		Icon  string
		Types []typeOption
	}
	groups := scheduler.GroupedTaskTypes()
	taskTypeGroups := make([]taskGroupView, 0, len(groups))
	for _, g := range groups {
		opts := make([]typeOption, 0, len(g.Types))
		for _, tt := range g.Types {
			opts = append(opts, typeOption{Value: string(tt), Label: scheduler.TaskTypeLabel(tt)})
		}
		taskTypeGroups = append(taskTypeGroups, taskGroupView{Name: g.Name, Icon: g.Icon, Types: opts})
	}

	if !s.renderTemplateWithNonce(r, w, http.StatusInternalServerError, "scheduler.html", map[string]interface{}{
		"staticVersion":  s.staticVersion,
		"TaskTypes":      taskTypes,
		"TaskTypeLabels": taskTypeLabels,
		"TaskTypeGroups": taskTypeGroups,
	}) {
		return
	}
}

// handleCreateTask creates a new scheduled task.
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		writeSchedulerJSONError(w, http.StatusServiceUnavailable, "scheduler not initialized")
		return
	}
	if !requireMethod(w, r, "POST") {
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}

	var req struct {
		Name          string                        `json:"name"`
		Type          string                        `json:"type"`
		Enabled       bool                          `json:"enabled"`
		CronExpr      string                        `json:"cron_expr"`
		Payload       map[string]any                `json:"payload"`
		TimeoutSec    int                           `json:"timeout_seconds"`
		MaxRetries    int                           `json:"max_retries"`
		Notifications *scheduler.NotificationConfig `json:"notifications,omitempty"`
		ScheduleType  string                        `json:"schedule_type,omitempty"`
		RunAt         *time.Time                    `json:"run_at,omitempty"`
		DelaySeconds  int                           `json:"delay_seconds,omitempty"`
	}

	if !decodeJSONBody(w, r, &req) {
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		writeSchedulerJSONError(w, http.StatusBadRequest, "task name is required")
		return
	}

	// Normalize schedule type
	scheduleType := strings.TrimSpace(req.ScheduleType)
	if scheduleType == "" {
		scheduleType = "cron"
	}

	// Validate schedule-specific requirements
	switch scheduleType {
	case "cron":
		if strings.TrimSpace(req.CronExpr) == "" {
			writeSchedulerJSONError(w, http.StatusBadRequest, "cron expression is required for cron schedule type")
			return
		}
	case "once":
		if req.RunAt == nil {
			writeSchedulerJSONError(w, http.StatusBadRequest, "run_at is required for once schedule type")
			return
		}
	case "delay":
		if req.DelaySeconds <= 0 {
			writeSchedulerJSONError(w, http.StatusBadRequest, "delay_seconds must be positive for delay schedule type")
			return
		}
	default:
		writeSchedulerJSONError(w, http.StatusBadRequest, "unknown schedule type: "+scheduleType+" (valid: cron, once, delay)")
		return
	}

	payload := mapToTaskPayload(req.Payload)

	task := &scheduler.ScheduledTask{
		Name:          strings.TrimSpace(req.Name),
		Type:          scheduler.TaskType(req.Type),
		Enabled:       req.Enabled,
		CronExpr:      strings.TrimSpace(req.CronExpr),
		Payload:       payload,
		TimeoutSec:    req.TimeoutSec,
		MaxRetries:    req.MaxRetries,
		Notifications: req.Notifications,
		ScheduleType:  scheduleType,
		RunAt:         req.RunAt,
		DelaySeconds:  req.DelaySeconds,
	}

	if req.Notifications != nil && len(req.Notifications.ChannelIDs) > 0 {
		if s.notifyRegistry == nil {
			writeSchedulerJSONError(w, http.StatusBadRequest, "notification system not initialized")
			return
		}
		for _, chID := range req.Notifications.ChannelIDs {
			if chID == "__task_inline_webhook__" {
				continue
			}
			if s.notifyRegistry.Get(chID) == nil {
				writeSchedulerJSONError(w, http.StatusBadRequest, "unknown notification channel: "+chID)
				return
			}
		}
	}

	if err := validateTaskPayload(req.Payload); err != nil {
		writeSchedulerJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.scheduler.AddTask(task); err != nil {
		writeSchedulerJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	logger.Infof("Scheduler task created: name=%s type=%s id=%s", task.Name, task.Type, task.ID)
	writeJSON(w, http.StatusCreated, map[string]interface{}{"id": task.ID, "message": "task created"})
}

// handleListTasks returns all scheduled tasks.
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		writeSchedulerJSONError(w, http.StatusServiceUnavailable, "scheduler not initialized")
		return
	}

	tasks := s.scheduler.ListTasks()
	// Sort by creation time (newest first)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	writeJSON(w, http.StatusOK, tasks)
}

// handleGetTask returns a single task by ID.
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		writeSchedulerJSONError(w, http.StatusServiceUnavailable, "scheduler not initialized")
		return
	}

	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeSchedulerJSONError(w, http.StatusBadRequest, "id is required")
		return
	}

	task, err := s.scheduler.GetTask(id)
	if err != nil {
		writeSchedulerJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, task)
}

// handleUpdateTask updates an existing task.
func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		writeSchedulerJSONError(w, http.StatusServiceUnavailable, "scheduler not initialized")
		return
	}
	if !requireMethod(w, r, "POST") {
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}

	var req struct {
		ID            string                        `json:"id"`
		Name          string                        `json:"name"`
		Type          string                        `json:"type"`
		Enabled       bool                          `json:"enabled"`
		CronExpr      string                        `json:"cron_expr"`
		Payload       map[string]any                `json:"payload"`
		TimeoutSec    int                           `json:"timeout_seconds"`
		MaxRetries    int                           `json:"max_retries"`
		Notifications *scheduler.NotificationConfig `json:"notifications,omitempty"`
		ScheduleType  string                        `json:"schedule_type,omitempty"`
		RunAt         *time.Time                    `json:"run_at,omitempty"`
		DelaySeconds  int                           `json:"delay_seconds,omitempty"`
	}

	if !decodeJSONBody(w, r, &req) {
		return
	}

	if strings.TrimSpace(req.ID) == "" {
		writeSchedulerJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	if req.Notifications != nil && len(req.Notifications.ChannelIDs) > 0 {
		if s.notifyRegistry == nil {
			writeSchedulerJSONError(w, http.StatusBadRequest, "notification system not initialized")
			return
		}
		for _, chID := range req.Notifications.ChannelIDs {
			if chID == "__task_inline_webhook__" {
				continue
			}
			if s.notifyRegistry.Get(chID) == nil {
				writeSchedulerJSONError(w, http.StatusBadRequest, "unknown notification channel: "+chID)
				return
			}
		}
	}

	task := &scheduler.ScheduledTask{
		ID:            strings.TrimSpace(req.ID),
		Name:          strings.TrimSpace(req.Name),
		Type:          scheduler.TaskType(req.Type),
		Enabled:       req.Enabled,
		CronExpr:      strings.TrimSpace(req.CronExpr),
		Payload:       mapToTaskPayload(req.Payload),
		TimeoutSec:    req.TimeoutSec,
		MaxRetries:    req.MaxRetries,
		Notifications: req.Notifications,
		ScheduleType:  strings.TrimSpace(req.ScheduleType),
		RunAt:         req.RunAt,
		DelaySeconds:  req.DelaySeconds,
	}

	if err := validateTaskPayload(req.Payload); err != nil {
		writeSchedulerJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.scheduler.UpdateTask(task); err != nil {
		writeSchedulerJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	logger.Infof("Scheduler task updated: id=%s", task.ID)
	writeJSON(w, http.StatusOK, map[string]string{"message": "task updated"})
}

// handleDeleteTask deletes a scheduled task.
func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		writeSchedulerJSONError(w, http.StatusServiceUnavailable, "scheduler not initialized")
		return
	}
	if !requireMethod(w, r, "POST") {
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}

	var req struct {
		ID string `json:"id"`
	}

	if !decodeJSONBody(w, r, &req) {
		return
	}

	if strings.TrimSpace(req.ID) == "" {
		writeSchedulerJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	if err := s.scheduler.DeleteTask(req.ID); err != nil {
		writeSchedulerJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	logger.Infof("Scheduler task deleted: id=%s", req.ID)
	writeJSON(w, http.StatusOK, map[string]string{"message": "task deleted"})
}

// handleRunTaskNow executes a task immediately.
func (s *Server) handleRunTaskNow(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		writeSchedulerJSONError(w, http.StatusServiceUnavailable, "scheduler not initialized")
		return
	}
	if !requireMethod(w, r, "POST") {
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}

	var req struct {
		ID string `json:"id"`
	}

	if !decodeJSONBody(w, r, &req) {
		return
	}

	if strings.TrimSpace(req.ID) == "" {
		writeSchedulerJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	if err := s.scheduler.RunTaskNow(req.ID); err != nil {
		writeSchedulerJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "task scheduled for immediate execution"})
}

// handleEnableTask enables a task.
func (s *Server) handleEnableTask(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		writeSchedulerJSONError(w, http.StatusServiceUnavailable, "scheduler not initialized")
		return
	}
	if !requireMethod(w, r, "POST") {
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}

	var req struct {
		ID string `json:"id"`
	}

	if !decodeJSONBody(w, r, &req) {
		return
	}

	if strings.TrimSpace(req.ID) == "" {
		writeSchedulerJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	if err := s.scheduler.EnableTask(req.ID); err != nil {
		writeSchedulerJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "task enabled"})
}

// handleDisableTask disables a task.
func (s *Server) handleDisableTask(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		writeSchedulerJSONError(w, http.StatusServiceUnavailable, "scheduler not initialized")
		return
	}
	if !requireMethod(w, r, "POST") {
		return
	}
	if !requireTrustedRequest(w, r, allowedOriginsFromConfig(s.config)) {
		return
	}

	var req struct {
		ID string `json:"id"`
	}

	if !decodeJSONBody(w, r, &req) {
		return
	}

	if strings.TrimSpace(req.ID) == "" {
		writeSchedulerJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	if err := s.scheduler.DisableTask(req.ID); err != nil {
		writeSchedulerJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "task disabled"})
}

// handleTaskHistory returns execution history.
func (s *Server) handleTaskHistory(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		writeSchedulerJSONError(w, http.StatusServiceUnavailable, "scheduler not initialized")
		return
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &limit); n == 1 && err == nil {
			if limit < 1 || limit > 500 {
				limit = 50
			}
		} else {
			limit = 50
		}
	}

	taskType := r.URL.Query().Get("task_type")
	status := r.URL.Query().Get("status")

	history := s.scheduler.GetHistory(limit, taskType, status)
	if history == nil {
		history = []scheduler.ExecutionRecord{}
	}

	writeJSON(w, http.StatusOK, history)
}

// mapToTaskPayload converts a raw map (from JSON) to a typed TaskPayload.
func mapToTaskPayload(m map[string]any) *model.TaskPayload {
	if m == nil {
		return &model.TaskPayload{}
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return &model.TaskPayload{}
	}
	var p model.TaskPayload
	_ = json.Unmarshal(raw, &p)
	return &p
}
