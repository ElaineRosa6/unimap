package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/metrics"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/notify"
	"github.com/unimap/project/internal/utils/urlguard"
)

// sendNotification sends notifications based on task configuration and execution result.
func (s *Scheduler) sendNotification(task *ScheduledTask, record ExecutionRecord) {
	if !s.shouldSendNotification(task, record) {
		logger.Warnf("[scheduler] notify: skipped for task %s (status=%s, shouldSend=false)", task.ID, record.Status)
		return
	}

	channelIDs := migrateChannelIDs(task.Notifications)
	if len(channelIDs) == 0 {
		logger.Warnf("[scheduler] notify: no channel IDs for task %s", task.ID)
		return
	}

	logger.Infof("[scheduler] notify: preparing to send to %d channels for task %s (status=%s)", len(channelIDs), task.ID, record.Status)
	msg := s.buildNotificationMessage(task, record)
	timeout := s.notifyTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	for _, chID := range channelIDs {
		if chID == "__task_inline_webhook__" && task.Notifications.WebhookURL != "" {
			s.sendInlineWebhookNotification(task.Notifications.WebhookURL, msg, timeout)
			continue
		}
		s.sendRegistryChannelNotification(chID, msg, timeout)
	}
}

func (s *Scheduler) shouldSendNotification(task *ScheduledTask, record ExecutionRecord) bool {
	if s.notifyCfgProvider != nil {
		globalCfg := s.notifyCfgProvider()
		if globalCfg == nil || !globalCfg.Enabled {
			logger.Warnf("[scheduler] notify: global config disabled (nil=%v)", globalCfg == nil)
			return false
		}
	}
	if task.Notifications == nil || !task.Notifications.Enabled {
		logger.Warnf("[scheduler] notify: task notifications disabled (nil=%v)", task.Notifications == nil)
		return false
	}

	shouldNotify := false
	switch record.Status {
	case "success":
		shouldNotify = task.Notifications.OnSuccess
	case "failed":
		shouldNotify = task.Notifications.OnFailure
	case "timeout":
		shouldNotify = task.Notifications.OnTimeout
	}
	if !shouldNotify {
		logger.Warnf("[scheduler] notify: on_%s=false for task %s", record.Status, task.ID)
	}
	return shouldNotify
}

func (s *Scheduler) buildNotificationMessage(task *ScheduledTask, record ExecutionRecord) notify.TaskNotification {
	msg := notify.TaskNotification{
		TaskID:    task.ID,
		TaskName:  sanitizeUTF8(task.Name),
		TaskType:  string(task.Type),
		Status:    record.Status,
		Result:    sanitizeUTF8(record.Result),
		Error:     sanitizeUTF8(record.Error),
		Duration:  float64(record.DurationMs),
		Timestamp: time.Now(),
		Payload:   payloadToMap(task.Payload),
	}
	msg.ImagePaths = extractImagePaths(record.Result)
	msg.Result = redactImagePaths(msg.Result, msg.ImagePaths)
	return msg
}

func (s *Scheduler) sendInlineWebhookNotification(webhookURL string, msg notify.TaskNotification, timeout time.Duration) {
	s.mu.RLock()
	stopping := s.stopped || s.stopping
	s.mu.RUnlock()
	if stopping {
		return
	}
	s.notifyWg.Add(1)
	go func(url string) {
		defer func() {
			s.notifyWg.Done()
			if r := recover(); r != nil {
				logger.Errorf("scheduler panic in inline webhook notification: %v", r)
			}
		}()
		ch, err := notify.NewGenericWebhookChannel("__inline__", url, nil, true, false)
		if err != nil {
			logger.Errorf("[scheduler] inline webhook URL blocked: %v", err)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := ch.Send(ctx, msg); err != nil {
			logger.Errorf("[scheduler] notify inline webhook failed: %v", err)
			metrics.IncSchedulerNotifyFail("webhook")
		} else {
			metrics.IncSchedulerNotifySuccess("webhook")
		}
	}(webhookURL)
}

func (s *Scheduler) sendRegistryChannelNotification(chID string, msg notify.TaskNotification, timeout time.Duration) {
	if s.notifyRegistry == nil {
		logger.Warnf("[scheduler] notify: registry is nil, skipping channel %s", chID)
		return
	}
	ch := s.notifyRegistry.Get(chID)
	if ch == nil {
		logger.Warnf("[scheduler] notify: channel %s not found in registry", chID)
		return
	}
	if !ch.IsEnabled() {
		logger.Warnf("[scheduler] notify: channel %s is disabled, skipping", chID)
		return
	}

	s.mu.RLock()
	stopping := s.stopped || s.stopping
	s.mu.RUnlock()
	if stopping {
		logger.Warnf("[scheduler] notify: scheduler stopping, skipping channel %s", chID)
		return
	}
	s.notifyWg.Add(1)
	go func(ch notify.NotifyChannel) {
		defer func() {
			s.notifyWg.Done()
			if r := recover(); r != nil {
				logger.Errorf("scheduler panic in notify channel %s: %v", ch.ID(), r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		logger.Infof("[scheduler] notify: sending to channel %s (%s), timeout=%v", ch.ID(), ch.Type(), timeout)
		if err := ch.Send(ctx, msg); err != nil {
			logger.Errorf("[scheduler] notify %s (%s) failed: %v", ch.ID(), ch.Type(), err)
			metrics.IncSchedulerNotifyFail(ch.Type())
		} else {
			logger.Infof("[scheduler] notify %s (%s) sent successfully", ch.ID(), ch.Type())
			metrics.IncSchedulerNotifySuccess(ch.Type())
		}
	}(ch)
}

// extractImagePaths extracts screenshot file paths from task result text.
func extractImagePaths(result string) []string {
	if result == "" {
		return nil
	}

	var paths []string
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.Contains(line, "→") {
			parts := strings.SplitN(line, "→", 2)
			if len(parts) == 2 {
				path := strings.TrimSpace(parts[1])
				if isImageFile(path) {
					paths = append(paths, path)
					continue
				}
			}
		}

		if strings.Contains(line, "保存:") {
			parts := strings.SplitN(line, "保存:", 2)
			if len(parts) == 2 {
				path := strings.TrimSpace(parts[1])
				if isImageFile(path) {
					paths = append(paths, path)
					continue
				}
			}
		}

		if strings.Contains(line, "截图保存:") || strings.Contains(line, "截图目录:") {
			continue
		}

		if isImageFile(line) {
			paths = append(paths, line)
		}
	}

	return paths
}

func isImageFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".png") ||
		strings.HasSuffix(lower, ".jpg") ||
		strings.HasSuffix(lower, ".jpeg") ||
		strings.HasSuffix(lower, ".gif") ||
		strings.HasSuffix(lower, ".webp")
}

// redactImagePaths strips full server paths from notification text, keeping only filenames.
func redactImagePaths(result string, paths []string) string {
	if result == "" || len(paths) == 0 {
		return result
	}
	for _, p := range paths {
		result = strings.ReplaceAll(result, p, filepath.Base(p))
	}
	return result
}

// sanitizePayload creates a copy of payload with all string values passed through sanitizeUTF8.
func sanitizePayload(payload *model.TaskPayload) *model.TaskPayload {
	if payload == nil {
		return nil
	}
	cp := *payload
	cp.Query = sanitizeUTF8(cp.Query)
	cp.Format = sanitizeUTF8(cp.Format)
	cp.DetectMode = sanitizeUTF8(cp.DetectMode)
	cp.Type = sanitizeUTF8(cp.Type)
	cp.URL = sanitizeUTF8(cp.URL)
	cp.CookieFile = sanitizeUTF8(cp.CookieFile)
	if cp.Engines != nil {
		for i, e := range cp.Engines {
			cp.Engines[i] = sanitizeUTF8(e)
		}
	}
	if cp.Queries != nil {
		for i, q := range cp.Queries {
			cp.Queries[i] = sanitizeUTF8(q)
		}
	}
	if cp.URLs != nil {
		for i, u := range cp.URLs {
			cp.URLs[i] = sanitizeUTF8(u)
		}
	}
	if cp.Extra != nil {
		extra := make(map[string]any, len(cp.Extra))
		for k, v := range cp.Extra {
			if s, ok := v.(string); ok {
				extra[k] = sanitizeUTF8(s)
			} else {
				extra[k] = v
			}
		}
		cp.Extra = extra
	}
	return &cp
}

// payloadToMap converts a TaskPayload to a map for notification serialization.
// Sensitive fields are redacted (see redactPayloadMap) before the map leaves
// the scheduler, so credentials/cookie paths are never forwarded to webhook /
// IM channels or recorded by the log channel.
func payloadToMap(payload *model.TaskPayload) map[string]interface{} {
	if payload == nil {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return redactPayloadMap(m)
}

// sensitivePayloadKeys are top-level payload fields that carry credentials or
// filesystem paths to credential files and must not be forwarded in
// notifications. They are replaced with a fixed placeholder.
var sensitivePayloadKeys = map[string]bool{
	"cookie_file": true,
}

// sensitiveExtraKeyFragments match substrings within Extra map keys that
// indicate a credential-bearing field. Matching is case-insensitive.
var sensitiveExtraKeyFragments = []string{
	"cookie", "token", "secret", "password", "passwd", "api_key", "apikey",
	"credential", "private_key", "access_key",
}

const redactedPlaceholder = "[REDACTED]"

// redactPayloadMap returns a copy of m with sensitive top-level keys removed
// and sensitive entries inside the nested "extra" map masked. The input map
// is not mutated.
func redactPayloadMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		if sensitivePayloadKeys[k] {
			out[k] = redactedPlaceholder
			continue
		}
		if k == "extra" {
			if extra, ok := v.(map[string]interface{}); ok {
				out[k] = redactExtraMap(extra)
				continue
			}
		}
		out[k] = v
	}
	return out
}

// redactExtraMap masks any entry whose key contains a sensitive fragment.
func redactExtraMap(extra map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(extra))
	for k, v := range extra {
		if isSensitiveExtraKey(k) {
			out[k] = redactedPlaceholder
			continue
		}
		out[k] = v
	}
	return out
}

func isSensitiveExtraKey(key string) bool {
	lower := strings.ToLower(key)
	for _, frag := range sensitiveExtraKeyFragments {
		if strings.Contains(lower, frag) {
			return true
		}
	}
	return false
}

// migrateChannelIDs migrates legacy Channels[] to ChannelIDs[].
func migrateChannelIDs(nc *NotificationConfig) []string {
	if len(nc.ChannelIDs) > 0 {
		return nc.ChannelIDs
	}

	var ids []string
	for _, name := range nc.Channels {
		switch name {
		case "log":
			ids = append(ids, "builtin-log")
		case "webhook":
			if nc.WebhookURL != "" {
				ids = append(ids, "__task_inline_webhook__")
			} else {
				logger.Warnf("[scheduler] task webhook channel without URL skipped")
			}
		case "email":
			logger.Warnf("[scheduler] email channel not supported, skipped")
		}
	}
	return ids
}

// validateWebhookURL validates a webhook URL for safety.
func validateWebhookURL(webhookURL string) error {
	if webhookURL == "" {
		return nil
	}
	if !strings.HasPrefix(webhookURL, "http://") && !strings.HasPrefix(webhookURL, "https://") {
		return fmt.Errorf("webhook URL must start with http:// or https://")
	}
	return nil
}

// ValidateWebhookURLPublic is the exported version of validateWebhookURL.
func ValidateWebhookURLPublic(webhookURL string) error {
	return validateWebhookURL(webhookURL)
}

// nolint:unused
func safeWebhookClient() *http.Client {
	return urlguard.SafeHTTPClient(urlguard.CheckOptions{}, 30*time.Second)
}

// nolint:unused
func (s *Scheduler) sendWebhookNotification(webhookURL string, payload map[string]interface{}) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("scheduler panic in sendWebhookNotification: %v", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		jsonData, err := json.Marshal(payload)
		if err != nil {
			logger.Errorf("[scheduler] failed to marshal webhook payload: %v", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(jsonData))
		if err != nil {
			logger.Errorf("[scheduler] failed to create webhook request: %v", err)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "UniMap-Scheduler/1.0")

		client := safeWebhookClient()
		resp, err := client.Do(req)
		if err != nil {
			logger.Errorf("[scheduler] failed to send webhook to %s: %v", webhookURL, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			logger.Warnf("[scheduler] webhook to %s returned non-success status: %d", webhookURL, resp.StatusCode)
		}
	}()
}
