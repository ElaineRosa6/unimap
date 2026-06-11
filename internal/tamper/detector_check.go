package tamper

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/unimap/project/internal/alerting"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/utils/workerpool"
)

// SaveBaseline delegates to the underlying storage.
func (d *Detector) SaveBaseline(url string, result *PageHashResult) error {
	return d.storage.SaveBaseline(url, result)
}

// LoadBaseline delegates to the underlying storage.
func (d *Detector) LoadBaseline(url string) (*PageHashResult, error) {
	return d.storage.LoadBaseline(url)
}

// HasBaseline delegates to the underlying storage.
func (d *Detector) HasBaseline(url string) bool {
	return d.storage.HasBaseline(url)
}

// ListBaselines delegates to the underlying storage.
func (d *Detector) ListBaselines() ([]string, error) {
	return d.storage.ListBaselines()
}

// DeleteBaseline delegates to the underlying storage.
func (d *Detector) DeleteBaseline(url string) error {
	return d.storage.DeleteBaseline(url)
}

// LoadCheckRecords delegates to the underlying storage.
func (d *Detector) LoadCheckRecords(url string, limit int) ([]*CheckRecord, error) {
	return d.storage.LoadCheckRecords(url, limit)
}

// ListAllCheckRecords delegates to the underlying storage.
func (d *Detector) ListAllCheckRecords() (map[string][]*CheckRecord, error) {
	return d.storage.ListAllCheckRecords()
}

// GetCheckStats delegates to the underlying storage.
func (d *Detector) GetCheckStats(url string) (map[string]interface{}, error) {
	return d.storage.GetCheckStats(url)
}

// DeleteCheckRecords delegates to the underlying storage.
func (d *Detector) DeleteCheckRecords(url string) error {
	return d.storage.DeleteCheckRecords(url)
}

// CheckTampering performs a tamper check for a single URL.
func (d *Detector) CheckTampering(ctx context.Context, url string) (*TamperCheckResult, error) {
	currentHash, err := d.ComputePageHash(ctx, url)
	if err != nil {
		return nil, err
	}

	baseline, err := d.storage.LoadBaseline(url)
	if err != nil {
		return d.handleBaselineLoadError(url, currentHash, err)
	}

	result := &TamperCheckResult{
		URL: url, CurrentHash: currentHash, BaselineHash: baseline,
		Tampered: false, Status: "normal", Timestamp: time.Now().Unix(),
	}
	checkType := "normal"

	suspiciousFlags := detectMaliciousContent(currentHash.RawHTML)
	result.SuspiciousFlags = suspiciousFlags

	if len(suspiciousFlags) > 0 {
		checkType = d.evaluateSuspiciousFlags(url, result, suspiciousFlags)
	} else {
		checkType = d.evaluateTamperChanges(url, currentHash, baseline, result)
	}

	d.saveCheckRecord(url, result, currentHash, baseline, checkType)
	return result, nil
}

// handleBaselineLoadError 处理基线加载错误和首次检查（无基线）两种情况
func (d *Detector) handleBaselineLoadError(url string, currentHash *PageHashResult, loadErr error) (*TamperCheckResult, error) {
	now := time.Now().Unix()
	if !errors.Is(loadErr, os.ErrNotExist) {
		result := &TamperCheckResult{
			URL: url, CurrentHash: currentHash, Tampered: false,
			Status: "failed", ErrorType: "baseline",
			ErrorMessage: fmt.Sprintf("failed to load baseline: %v", loadErr), Timestamp: now,
		}
		d.saveCheckRecord(url, result, currentHash, nil, "baseline_error")
		return result, nil
	}
	// 无基线 — 首次检查
	result := &TamperCheckResult{
		URL: url, CurrentHash: currentHash,
		Tampered: false, Status: "no_baseline", Timestamp: now,
	}
	d.saveCheckRecord(url, result, currentHash, nil, "first_check")
	return result, nil
}

// evaluateSuspiciousFlags 根据检测模式评估可疑标记，返回 checkType
func (d *Detector) evaluateSuspiciousFlags(url string, result *TamperCheckResult, suspiciousFlags []string) string {
	shouldMark := false
	switch d.detectionMode {
	case DetectionModeStrict:
		shouldMark = true
	case DetectionModeSecurity:
		strong := 0
		for _, flag := range suspiciousFlags {
			if flag == "hidden_iframe_detected" || flag == "dangerous_event_handler" {
				strong++
			}
		}
		shouldMark = strong > 0 || len(suspiciousFlags) >= 2
	case DetectionModePrecise, DetectionModeRelaxed:
		for _, flag := range suspiciousFlags {
			if flag == "hidden_iframe_detected" || flag == "dangerous_event_handler" {
				shouldMark = true
				break
			}
		}
	case DetectionModeBalanced:
		shouldMark = len(suspiciousFlags) >= 2
	}

	if !shouldMark {
		return "normal"
	}
	result.Tampered = true
	result.Status = "suspicious"
	result.TamperedSegments = []string{"malicious_content"}
	if d.alertManager != nil {
		d.alertManager.SendCritical(alerting.AlertTypeTamper,
			"检测到恶意内容", fmt.Sprintf("URL %s 检测到恶意内容", url),
			map[string]interface{}{"flags": suspiciousFlags, "detection_mode": d.detectionMode},
			"tamper_detector", url)
	}
	return "suspicious"
}

// evaluateTamperChanges 评估内容变更是否构成篡改，返回 checkType
func (d *Detector) evaluateTamperChanges(url string, currentHash, baseline *PageHashResult, result *TamperCheckResult) string {
	simpleMD5Changed := baseline.SimpleMD5Hash != "" && currentHash.SimpleMD5Hash != "" &&
		baseline.SimpleMD5Hash != currentHash.SimpleMD5Hash

	tamperedSegments, changes := d.findChangedSegments(currentHash, baseline)
	result.TamperedSegments = tamperedSegments
	result.Changes = changes

	if !simpleMD5Changed && len(changes) == 0 {
		return "normal"
	}
	if simpleMD5Changed || d.isMeaningfulTamper(changes) {
		result.Tampered = true
		result.Status = "tampered"
		if d.alertManager != nil {
			d.alertManager.SendWarning(alerting.AlertTypeTamper,
				"检测到网页篡改", fmt.Sprintf("URL %s 检测到 %d 个分段被修改", url, len(tamperedSegments)),
				map[string]interface{}{
					"segments": tamperedSegments, "changes": len(changes),
					"detection_mode": d.detectionMode, "simple_md5_changed": simpleMD5Changed,
				}, "tamper_detector", url)
		}
		return "tampered"
	}
	if len(changes) > 0 {
		result.Status = "changed"
		result.Tampered = true
	}
	return "normal_dynamic"
}

// saveCheckRecord 保存检查记录到存储
func (d *Detector) saveCheckRecord(url string, result *TamperCheckResult, currentHash, baseline *PageHashResult, checkType string) {
	record := &CheckRecord{
		ID: fmt.Sprintf("%d", time.Now().UnixNano()), URL: url,
		Tampered: result.Tampered, TamperedSegments: result.TamperedSegments,
		Changes: result.Changes, CurrentHash: currentHash, BaselineHash: baseline,
		Timestamp: result.Timestamp, CheckType: checkType, DetectionMode: d.detectionMode,
	}
	if err := d.storage.SaveCheckRecord(url, record); err != nil {
		logger.Warnf("Failed to save check record: %v", err)
	}
}

func (d *Detector) findChangedSegments(current, baseline *PageHashResult) ([]string, []SegmentChange) {
	var tamperedSegments []string
	var changes []SegmentChange

	currentMap := make(map[string]SegmentHash)
	for _, seg := range current.SegmentHashes {
		currentMap[seg.Name] = seg
	}

	baselineMap := make(map[string]SegmentHash)
	for _, seg := range baseline.SegmentHashes {
		baselineMap[seg.Name] = seg
	}

	for name, currentSeg := range currentMap {
		if baselineSeg, exists := baselineMap[name]; exists {
			if currentSeg.Hash != baselineSeg.Hash {
				tamperedSegments = append(tamperedSegments, name)
				changeType := "modified"
				if currentSeg.Elements != baselineSeg.Elements {
					changeType = "structure_changed"
				}
				changes = append(changes, SegmentChange{
					Segment:     name,
					OldHash:     baselineSeg.Hash,
					NewHash:     currentSeg.Hash,
					ChangeType:  changeType,
					Description: fmt.Sprintf("Segment '%s' has been modified", name),
				})
			}
		} else {
			if isCompatibilityOptionalSegment(name) {
				continue
			}
			tamperedSegments = append(tamperedSegments, name)
			changes = append(changes, SegmentChange{
				Segment:     name,
				OldHash:     "",
				NewHash:     currentSeg.Hash,
				ChangeType:  "added",
				Description: fmt.Sprintf("Segment '%s' is new", name),
			})
		}
	}

	for name, baselineSeg := range baselineMap {
		if _, exists := currentMap[name]; !exists {
			if isCompatibilityOptionalSegment(name) {
				continue
			}
			tamperedSegments = append(tamperedSegments, name)
			changes = append(changes, SegmentChange{
				Segment:     name,
				OldHash:     baselineSeg.Hash,
				NewHash:     "",
				ChangeType:  "removed",
				Description: fmt.Sprintf("Segment '%s' has been removed", name),
			})
		}
	}

	return tamperedSegments, changes
}

func (d *Detector) isMeaningfulTamper(changes []SegmentChange) bool {
	if len(changes) == 0 {
		return false
	}

	switch d.detectionMode {
	case DetectionModeStrict:
		return true

	case DetectionModeSecurity:
		for _, change := range changes {
			if change.Segment == SegmentScripts ||
				change.Segment == SegmentForms ||
				change.Segment == SegmentHead {
				return true
			}
		}
		return false

	case DetectionModePrecise:
		criticalChanges := 0
		for _, change := range changes {
			if isCriticalStableSegment(change.Segment) {
				criticalChanges++
			}
		}
		return criticalChanges > 0

	case DetectionModeBalanced:
		stableModifiedCount := 0
		criticalChanges := 0

		for _, change := range changes {
			if !d.isStableSegment(change.Segment) {
				continue
			}

			if change.ChangeType == "added" || change.ChangeType == "removed" {
				return true
			}

			stableModifiedCount++
			if isCriticalStableSegment(change.Segment) {
				criticalChanges++
			}
		}

		return criticalChanges > 0 || stableModifiedCount >= 2

	default: // DetectionModeRelaxed
		stableModifiedCount := 0
		for _, change := range changes {
			if !d.isStableSegment(change.Segment) {
				continue
			}

			if change.ChangeType == "added" || change.ChangeType == "removed" {
				return true
			}

			stableModifiedCount++
			if isCriticalStableSegment(change.Segment) {
				return true
			}
		}

		return stableModifiedCount >= 2
	}
}

func isCriticalStableSegment(segment string) bool {
	switch segment {
	case SegmentMain, SegmentArticle, SegmentForms:
		return true
	default:
		return false
	}
}

func (d *Detector) isStableSegment(segment string) bool {
	if d.detectionMode == DetectionModeStrict {
		return true
	}
	_, volatile := relaxedVolatileSegments[segment]
	return !volatile
}

func isCompatibilityOptionalSegment(segment string) bool {
	_, optional := compatibilityOptionalSegments[segment]
	return optional
}

// --- Batch operations ---

type tamperBatchCheckResult struct {
	index  int
	result TamperCheckResult
}

type tamperBatchCheckTask struct {
	detector   *Detector
	ctx        context.Context
	index      int
	targetURL  string
	resultChan chan<- tamperBatchCheckResult
	wg         *sync.WaitGroup
}

func (t *tamperBatchCheckTask) Execute() error {
	defer t.wg.Done()

	result, err := t.detector.CheckTampering(t.ctx, t.targetURL)
	if err != nil {
		t.resultChan <- tamperBatchCheckResult{
			index: t.index,
			result: TamperCheckResult{
				URL:          t.targetURL,
				Tampered:     false,
				Status:       "unreachable",
				ErrorType:    classifyTamperError(err.Error()),
				ErrorMessage: err.Error(),
				Timestamp:    time.Now().Unix(),
				CurrentHash: &PageHashResult{
					URL:    t.targetURL,
					Status: "error: " + err.Error(),
				},
			},
		}
		return nil
	}

	t.resultChan <- tamperBatchCheckResult{index: t.index, result: *result}
	return nil
}

type tamperBatchBaselineResult struct {
	index  int
	result PageHashResult
}

type tamperBatchBaselineTask struct {
	detector   *Detector
	ctx        context.Context
	index      int
	targetURL  string
	resultChan chan<- tamperBatchBaselineResult
	wg         *sync.WaitGroup
}

func (t *tamperBatchBaselineTask) Execute() error {
	defer t.wg.Done()

	hashResult, err := t.detector.ComputePageHash(t.ctx, t.targetURL)
	if err != nil {
		t.resultChan <- tamperBatchBaselineResult{
			index: t.index,
			result: PageHashResult{
				URL:    t.targetURL,
				Status: "error: " + err.Error(),
			},
		}
		return nil
	}

	if err := t.detector.SaveBaseline(t.targetURL, hashResult); err != nil {
		t.resultChan <- tamperBatchBaselineResult{
			index: t.index,
			result: PageHashResult{
				URL:    t.targetURL,
				Status: "error saving baseline: " + err.Error(),
			},
		}
		return nil
	}

	t.resultChan <- tamperBatchBaselineResult{index: t.index, result: *hashResult}
	return nil
}

func collectOrderedTamperCheckResults(resultChan <-chan tamperBatchCheckResult, size int) []TamperCheckResult {
	results := make([]TamperCheckResult, size)
	for item := range resultChan {
		if item.index < 0 || item.index >= size {
			continue
		}
		results[item.index] = item.result
	}
	return results
}

func collectOrderedTamperBaselineResults(resultChan <-chan tamperBatchBaselineResult, size int) []PageHashResult {
	results := make([]PageHashResult, size)
	for item := range resultChan {
		if item.index < 0 || item.index >= size {
			continue
		}
		results[item.index] = item.result
	}
	return results
}

// BatchCheckTampering performs tamper checks on multiple URLs concurrently.
func (d *Detector) BatchCheckTampering(ctx context.Context, urls []string, concurrency int) ([]TamperCheckResult, error) {
	if len(urls) == 0 {
		return nil, fmt.Errorf("no URLs provided")
	}

	if concurrency <= 0 {
		concurrency = 5
	}

	pool := workerpool.NewPool(concurrency)
	pool.Start()

	var wg sync.WaitGroup
	resultChan := make(chan tamperBatchCheckResult, len(urls))

	for i, url := range urls {
		wg.Add(1)
		task := &tamperBatchCheckTask{
			detector:   d,
			ctx:        ctx,
			index:      i,
			targetURL:  url,
			resultChan: resultChan,
			wg:         &wg,
		}
		pool.Submit(task)
	}

	go func() {
		wg.Wait()
		pool.Stop()
		close(resultChan)
	}()

	results := collectOrderedTamperCheckResults(resultChan, len(urls))
	return results, nil
}

// BatchSetBaseline sets baselines for multiple URLs concurrently.
func (d *Detector) BatchSetBaseline(ctx context.Context, urls []string, concurrency int) ([]PageHashResult, error) {
	if len(urls) == 0 {
		return nil, fmt.Errorf("no URLs provided")
	}

	if concurrency <= 0 {
		concurrency = 5
	}

	pool := workerpool.NewPool(concurrency)
	pool.Start()

	var wg sync.WaitGroup
	resultChan := make(chan tamperBatchBaselineResult, len(urls))

	for i, url := range urls {
		wg.Add(1)
		task := &tamperBatchBaselineTask{
			detector:   d,
			ctx:        ctx,
			index:      i,
			targetURL:  url,
			resultChan: resultChan,
			wg:         &wg,
		}
		pool.Submit(task)
	}

	go func() {
		wg.Wait()
		pool.Stop()
		close(resultChan)
	}()

	results := collectOrderedTamperBaselineResults(resultChan, len(urls))
	return results, nil
}

func sanitizeFilenameForStorage(url string) string {
	replacer := strings.NewReplacer(
		"http://", "",
		"https://", "",
		"/", "_",
		":", "_",
		"?", "_",
		"&", "_",
		"=", "_",
		".", "_",
	)
	return replacer.Replace(url)
}

func classifyTamperError(message string) string {
	msg := strings.ToLower(strings.TrimSpace(message))
	if msg == "" {
		return "unknown"
	}

	switch {
	case strings.Contains(msg, "baseline"):
		return "baseline"
	case strings.Contains(msg, "name_not_resolved") || strings.Contains(msg, "dns"):
		return "dns"
	case strings.Contains(msg, "timed out") || strings.Contains(msg, "timeout"):
		return "timeout"
	case strings.Contains(msg, "ssl") || strings.Contains(msg, "tls") || strings.Contains(msg, "certificate"):
		return "tls"
	case strings.Contains(msg, "connection refused") || strings.Contains(msg, "connrefused"):
		return "connection_refused"
	case strings.Contains(msg, "connection reset"):
		return "connection_reset"
	default:
		return "network"
	}
}
