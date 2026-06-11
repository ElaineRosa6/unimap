package tamper

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/unimap/project/internal/alerting"
	"github.com/unimap/project/internal/logger"
)

const (
	SegmentHead        = "head"
	SegmentBody        = "body"
	SegmentHeader      = "header"
	SegmentNav         = "nav"
	SegmentMain        = "main"
	SegmentArticle     = "article"
	SegmentSection     = "section"
	SegmentAside       = "aside"
	SegmentFooter      = "footer"
	SegmentScripts     = "scripts"
	SegmentStyles      = "styles"
	SegmentMeta        = "meta"
	SegmentLinks       = "links"
	SegmentImages      = "images"
	SegmentJSFiles     = "js_files"
	SegmentFavicon     = "favicon"
	SegmentButtons     = "buttons"
	SegmentForms       = "forms"
	SegmentFullContent = "full_content"

	DetectionModeRelaxed  = "relaxed"
	DetectionModeStrict   = "strict"
	DetectionModeSecurity = "security"
	DetectionModeBalanced = "balanced"
	DetectionModePrecise  = "precise"

	PerformanceModeFast          = "fast"
	PerformanceModeBalanced      = "balanced"
	PerformanceModeComprehensive = "comprehensive"
)

var relaxedVolatileSegments = map[string]struct{}{
	SegmentHead:        {},
	SegmentBody:        {},
	SegmentHeader:      {},
	SegmentNav:         {},
	SegmentFooter:      {},
	SegmentLinks:       {},
	SegmentScripts:     {},
	SegmentStyles:      {},
	SegmentMeta:        {},
	SegmentFullContent: {},
}

var compatibilityOptionalSegments = map[string]struct{}{
	SegmentJSFiles: {},
	SegmentFavicon: {},
	SegmentButtons: {},
}

var (
	reMultipleSpaces = regexp.MustCompile(`(?i)\s+`)
	reComments       = regexp.MustCompile(`(?i)<!--.*?-->`)
	reDataImages     = regexp.MustCompile(`(?i)data:image/[^"']*`)
	reNonce          = regexp.MustCompile(`(?i)nonce="[^"]*"`)
	reCSRFToken      = regexp.MustCompile(`(?i)csrf[^"]*_token["']?\s*[:=]\s*["'][^"']*["']`)
)

var (
	maliciousScriptKeywords = []string{
		"eval(",
		"document.write(",
		"Function(",
		"atob(",
		"btoa(",
		"unescape(",
		"decodeURIComponent(",
		"String.fromCharCode",
		"crypto",
		"miner",
		"coin-hive",
		"coinhive",
		"cryptonight",
	}

	suspiciousDomainKeywords = []string{
		"xxx", "porn", "sex", "adult", "casino", "gambling",
		"bet", "lottery", "crypto", "bitcoin", "mining",
		"coin-hive", "coinhive", "cryptonight",
	}

	suspiciousPathKeywords = []string{
		"shell", "backdoor", "webshell", "hacked", "deface",
		"phishing", "fake", "login", "admin",
	}

	hiddenIframePattern    = regexp.MustCompile(`(?i)<iframe[^>]*style\s*=\s*["'][^"']*(display\s*:\s*none|visibility\s*:\s*hidden|width\s*:\s*0|height\s*:\s*0)[^"']*["']`)
	dangerousEventPattern  = regexp.MustCompile(`(?i)on(?:error|load|mouseover|click|keyup|keydown|submit)\s*=\s*["'][^"']*(?:eval\(|document\.write\(|Function\(|atob\(|btoa\(|unescape\(|decodeURIComponent\(|String\.fromCharCode\(|crypto|miner|coin-hive|coinhive|cryptonight)[^"']*["']`)
	maliciousKeywordRes    = compileWordBoundaryRegexes(maliciousScriptKeywords)
	domainKeywordRes       = compileWordBoundaryRegexes(suspiciousDomainKeywords)
	pathKeywordRes         = compileWordBoundaryRegexes(suspiciousPathKeywords)
)

// compiledKeyword pairs a human-readable keyword with its regex.
type compiledKeyword struct {
	keyword string
	re      *regexp.Regexp
}

// compileWordBoundaryRegexes builds word-boundary regexps from keyword list.
func compileWordBoundaryRegexes(keywords []string) []compiledKeyword {
	res := make([]compiledKeyword, 0, len(keywords))
	for _, kw := range keywords {
		quoted := regexp.QuoteMeta(kw)
		if len(kw) > 0 && isWordChar(kw[len(kw)-1]) {
			res = append(res, compiledKeyword{keyword: kw, re: regexp.MustCompile(`(?i)\b` + quoted + `\b`)})
		} else {
			res = append(res, compiledKeyword{keyword: kw, re: regexp.MustCompile(`(?i)\b` + quoted)})
		}
	}
	return res
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

func detectMaliciousContent(html string) []string {
	var flags []string
	for _, item := range maliciousKeywordRes {
		if item.re.MatchString(html) {
			flags = append(flags, fmt.Sprintf("suspicious_script: %s", item.keyword))
		}
	}
	domainMatches := 0
	for _, item := range domainKeywordRes {
		if item.re.MatchString(html) {
			domainMatches++
			if domainMatches >= 2 {
				flags = append(flags, "suspicious_domain_keywords: multiple matches")
				break
			}
		}
	}
	pathMatches := 0
	for _, item := range pathKeywordRes {
		if item.re.MatchString(html) {
			pathMatches++
			if pathMatches >= 2 {
				flags = append(flags, "suspicious_path_keywords: multiple matches")
				break
			}
		}
	}
	if hiddenIframePattern.MatchString(html) {
		flags = append(flags, "hidden_iframe_detected")
	}
	if dangerousEventPattern.MatchString(html) {
		flags = append(flags, "dangerous_event_handler")
	}
	return flags
}

// SegmentHash represents a hash of a specific page segment.
type SegmentHash struct {
	Name     string `json:"name"`
	Hash     string `json:"hash"`
	Content  string `json:"content,omitempty"`
	Length   int    `json:"length"`
	Elements int    `json:"elements"`
}

// PageHashResult holds the full hash computation result for a page.
type PageHashResult struct {
	URL           string        `json:"url"`
	Title         string        `json:"title"`
	FullHash      string        `json:"full_hash"`
	SimpleMD5Hash string        `json:"simple_md5_hash"`
	SegmentHashes []SegmentHash `json:"segment_hashes"`
	Timestamp     int64         `json:"timestamp"`
	HTMLLength    int           `json:"html_length"`
	Status        string        `json:"status"`
	RawHTML       string        `json:"-"`
}

// TamperCheckResult is the result of a single tamper check.
type TamperCheckResult struct {
	URL              string          `json:"url"`
	CurrentHash      *PageHashResult `json:"current_hash"`
	BaselineHash     *PageHashResult `json:"baseline_hash,omitempty"`
	Tampered         bool            `json:"tampered"`
	Status           string          `json:"status"`
	ErrorType        string          `json:"error_type,omitempty"`
	ErrorMessage     string          `json:"error_message,omitempty"`
	TamperedSegments []string        `json:"tampered_segments,omitempty"`
	Changes          []SegmentChange `json:"changes,omitempty"`
	SuspiciousFlags  []string        `json:"suspicious_flags,omitempty"`
	Timestamp        int64           `json:"timestamp"`
}

// SegmentChange describes a single segment-level change.
type SegmentChange struct {
	Segment     string `json:"segment"`
	OldHash     string `json:"old_hash"`
	NewHash     string `json:"new_hash"`
	ChangeType  string `json:"change_type"`
	Description string `json:"description"`
}

// HashStorage manages on-disk baseline and check-record persistence.
type HashStorage struct {
	baseDir string
	mu      sync.RWMutex
}

type cacheEntry struct {
	result    *PageHashResult
	timestamp time.Time
}

// Detector is the main tamper-detection engine.
type Detector struct {
	storage         *HashStorage
	allocCtx        context.Context
	allocCancel     context.CancelFunc
	detectionMode   string
	performanceMode string
	alertManager    *alerting.Manager
	cache           map[string]*cacheEntry
	cacheMu         sync.RWMutex
	mu              sync.Mutex
}

// DetectorConfig holds configuration for creating a new Detector.
type DetectorConfig struct {
	BaseDir         string
	DetectionMode   string
	PerformanceMode string
	AlertManager    *alerting.Manager
}

// CheckRecord represents a single tamper check record persisted to disk.
type CheckRecord struct {
	ID               string          `json:"id"`
	URL              string          `json:"url"`
	Tampered         bool            `json:"tampered"`
	DetectionMode    string          `json:"detection_mode,omitempty"`
	TamperedSegments []string        `json:"tampered_segments,omitempty"`
	Changes          []SegmentChange `json:"changes,omitempty"`
	CurrentHash      *PageHashResult `json:"current_hash"`
	BaselineHash     *PageHashResult `json:"baseline_hash,omitempty"`
	Timestamp        int64           `json:"timestamp"`
	CheckType        string          `json:"check_type"`
}

// NewHashStorage creates a new HashStorage with the given base directory.
func NewHashStorage(baseDir string) *HashStorage {
	if baseDir == "" {
		baseDir = "./hash_store"
	}
	return &HashStorage{baseDir: baseDir}
}

// SaveBaseline persists a page hash baseline to disk.
func (s *HashStorage) SaveBaseline(url string, result *PageHashResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create hash store directory: %w", err)
	}

	safeFilename := sanitizeFilenameForStorage(url)
	filePath := filepath.Join(s.baseDir, safeFilename+".json")

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal hash result: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to save baseline: %w", err)
	}

	logger.Infof("Saved baseline hash for %s to %s", url, filePath)
	return nil
}

// LoadBaseline loads a page hash baseline from disk.
func (s *HashStorage) LoadBaseline(url string) (*PageHashResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	safeFilename := sanitizeFilenameForStorage(url)
	filePath := filepath.Join(s.baseDir, safeFilename+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("baseline not found for %s: %w", url, err)
	}

	var result PageHashResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal baseline: %w", err)
	}

	return &result, nil
}

// HasBaseline checks whether a baseline exists for the given URL.
func (s *HashStorage) HasBaseline(url string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	safeFilename := sanitizeFilenameForStorage(url)
	filePath := filepath.Join(s.baseDir, safeFilename+".json")

	_, err := os.Stat(filePath)
	return err == nil
}

// ListBaselines lists all stored baseline URLs.
func (s *HashStorage) ListBaselines() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	files, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, err
	}

	var urls []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			filePath := filepath.Join(s.baseDir, file.Name())
			data, readErr := os.ReadFile(filePath)
			if readErr != nil {
				continue
			}

			var result PageHashResult
			if unmarshalErr := json.Unmarshal(data, &result); unmarshalErr != nil {
				continue
			}

			if strings.TrimSpace(result.URL) != "" {
				urls = append(urls, result.URL)
			}
		}
	}
	sort.Strings(urls)
	return urls, nil
}

// DeleteBaseline removes a baseline from disk.
func (s *HashStorage) DeleteBaseline(url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	safeFilename := sanitizeFilenameForStorage(url)
	filePath := filepath.Join(s.baseDir, safeFilename+".json")

	return os.Remove(filePath)
}

// SaveCheckRecord persists a check record to disk.
func (s *HashStorage) SaveCheckRecord(url string, record *CheckRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	recordsDir := filepath.Join(s.baseDir, "records", sanitizeFilenameForStorage(url))
	if err := os.MkdirAll(recordsDir, 0755); err != nil {
		return fmt.Errorf("failed to create records directory: %w", err)
	}

	if record.ID == "" {
		record.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if record.Timestamp == 0 {
		record.Timestamp = time.Now().Unix()
	}

	filename := fmt.Sprintf("%s_%s.json", record.ID, record.CheckType)
	filePath := filepath.Join(recordsDir, filename)

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal check record: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to save check record: %w", err)
	}

	logger.Infof("Saved check record for %s to %s", url, filePath)
	return nil
}

// LoadCheckRecords loads check records for a URL.
func (s *HashStorage) LoadCheckRecords(url string, limit int) ([]*CheckRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	recordsDir := filepath.Join(s.baseDir, "records", sanitizeFilenameForStorage(url))

	files, err := os.ReadDir(recordsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read records directory: %w", err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() > files[j].Name()
	})

	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}

	var records []*CheckRecord
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(recordsDir, file.Name())
		data, readErr := os.ReadFile(filePath)
		if readErr != nil {
			logger.Warnf("Failed to read check record %s: %v", filePath, readErr)
			continue
		}

		var record CheckRecord
		if unmarshalErr := json.Unmarshal(data, &record); unmarshalErr != nil {
			logger.Warnf("Failed to unmarshal check record %s: %v", filePath, unmarshalErr)
			continue
		}

		records = append(records, &record)
	}

	return records, nil
}

// ListAllCheckRecords lists all check records grouped by URL.
func (s *HashStorage) ListAllCheckRecords() (map[string][]*CheckRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	recordsBaseDir := filepath.Join(s.baseDir, "records")

	urlDirs, err := os.ReadDir(recordsBaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string][]*CheckRecord), nil
		}
		return nil, fmt.Errorf("failed to read records base directory: %w", err)
	}

	result := make(map[string][]*CheckRecord)
	for _, urlDir := range urlDirs {
		if !urlDir.IsDir() {
			continue
		}

		recordsDir := filepath.Join(recordsBaseDir, urlDir.Name())

		files, readErr := os.ReadDir(recordsDir)
		if readErr != nil {
			continue
		}

		sort.Slice(files, func(i, j int) bool {
			return files[i].Name() > files[j].Name()
		})

		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".json") {
				continue
			}

			filePath := filepath.Join(recordsDir, file.Name())
			data, readErr := os.ReadFile(filePath)
			if readErr != nil {
				continue
			}

			var record CheckRecord
			if unmarshalErr := json.Unmarshal(data, &record); unmarshalErr != nil {
				continue
			}

			if record.URL != "" {
				result[record.URL] = append(result[record.URL], &record)
			}
		}
	}

	return result, nil
}

// GetCheckStats returns aggregate statistics for a URL's check records.
func (s *HashStorage) GetCheckStats(url string) (map[string]interface{}, error) {
	records, err := s.LoadCheckRecords(url, 0)
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return map[string]interface{}{
			"total_checks":      0,
			"tampered_count":    0,
			"safe_count":        0,
			"first_check_count": 0,
		}, nil
	}

	var tamperedCount, safeCount, firstCheckCount int
	for _, r := range records {
		if r.CheckType == "first_check" {
			firstCheckCount++
		} else if r.Tampered {
			tamperedCount++
		} else {
			safeCount++
		}
	}

	return map[string]interface{}{
		"total_checks":      len(records),
		"tampered_count":    tamperedCount,
		"safe_count":        safeCount,
		"first_check_count": firstCheckCount,
		"last_check_time":   records[0].Timestamp,
		"first_check_time":  records[len(records)-1].Timestamp,
	}, nil
}

// DeleteCheckRecords deletes all check records for a URL.
func (s *HashStorage) DeleteCheckRecords(url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	recordsDir := filepath.Join(s.baseDir, "records", sanitizeFilenameForStorage(url))
	return os.RemoveAll(recordsDir)
}

// NewDetector creates a new Detector with the given configuration.
func NewDetector(cfg DetectorConfig) *Detector {
	storage := NewHashStorage(cfg.BaseDir)
	return &Detector{
		storage:         storage,
		detectionMode:   normalizeDetectionMode(cfg.DetectionMode),
		performanceMode: normalizePerformanceMode(cfg.PerformanceMode),
		alertManager:    cfg.AlertManager,
		cache:           make(map[string]*cacheEntry),
	}
}

func normalizeDetectionMode(raw string) string {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case DetectionModeStrict:
		return DetectionModeStrict
	case DetectionModeSecurity:
		return DetectionModeSecurity
	case DetectionModeBalanced:
		return DetectionModeBalanced
	case DetectionModePrecise:
		return DetectionModePrecise
	default:
		return DetectionModeRelaxed
	}
}

func normalizePerformanceMode(raw string) string {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case PerformanceModeFast:
		return PerformanceModeFast
	case PerformanceModeBalanced:
		return PerformanceModeBalanced
	case PerformanceModeComprehensive:
		return PerformanceModeComprehensive
	default:
		return PerformanceModeBalanced
	}
}
