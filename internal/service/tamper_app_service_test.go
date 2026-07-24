package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/unimap/project/internal/tamper"
)

func TestTamperAppServiceNewDetectorUsesRuntimeConfig(t *testing.T) {
	wantErr := errors.New("allocator sentinel")
	providerCalls := 0
	app := NewTamperAppService(t.TempDir(), nil, func() TamperRuntimeConfig {
		providerCalls++
		return TamperRuntimeConfig{
			InsecureSkipVerify: true,
			PortScanEnabled:    true,
			PortScanTimeout:    1250 * time.Millisecond,
		}
	})

	_, _, err := app.newDetector(context.Background(), tamper.DetectionModeSecurity, func(context.Context) (context.Context, context.CancelFunc, error) {
		return nil, nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("newDetector error = %v, want wrapped sentinel", err)
	}
	if providerCalls != 1 {
		t.Fatalf("runtime config provider calls = %d, want 1", providerCalls)
	}

	cfg := app.detectorConfig(tamper.DetectionModeSecurity)
	if !cfg.InsecureSkipVerify || !cfg.PortScanEnabled {
		t.Fatalf("runtime flags were not propagated: %#v", cfg)
	}
	if cfg.PortScanTimeout != 1250*time.Millisecond {
		t.Fatalf("port scan timeout = %s, want 1.25s", cfg.PortScanTimeout)
	}
	if cfg.DetectionMode != tamper.DetectionModeSecurity {
		t.Fatalf("detection mode = %q, want security", cfg.DetectionMode)
	}
}

func TestTamperAppServiceQueryHistoryPaginatesAfterFiltering(t *testing.T) {
	dir := t.TempDir()
	primaryURL := "https://primary.example.test"
	otherURL := "https://other.example.test"
	storage := tamper.NewHashStorage(dir)
	for _, record := range []*tamper.CheckRecord{
		{ID: "primary-newest", URL: primaryURL, CheckType: "first_check", Timestamp: 300},
		{ID: "primary-middle", URL: primaryURL, CheckType: "first_check", Timestamp: 200},
		{ID: "primary-oldest", URL: primaryURL, CheckType: "first_check", Timestamp: 100},
		{ID: "other-newest", URL: otherURL, CheckType: "first_check", Timestamp: 400},
	} {
		if err := storage.SaveCheckRecord(record.URL, record); err != nil {
			t.Fatalf("save check record: %v", err)
		}
	}

	result, err := NewTamperAppService(dir, nil).QueryHistory(HistoryFilter{
		URLFilter:  primaryURL,
		TypeFilter: "first_check",
		Limit:      1,
		Offset:     1,
	})
	if err != nil {
		t.Fatalf("query history: %v", err)
	}
	if result.Count != 3 || len(result.Records) != 1 {
		t.Fatalf("unexpected total/page size: count=%d records=%#v", result.Count, result.Records)
	}
	if result.Records[0].ID != "primary-middle" {
		t.Fatalf("record id = %q, want primary-middle", result.Records[0].ID)
	}
	if len(result.URLOptions) != 1 || result.URLOptions[0] != primaryURL {
		t.Fatalf("unexpected URL options: %#v", result.URLOptions)
	}
}

func TestTamperAppServiceQueryHistoryFiltersByTime(t *testing.T) {
	dir := t.TempDir()
	storage := tamper.NewHashStorage(dir)
	targetURL := "https://range.example.test"
	for _, record := range []*tamper.CheckRecord{
		{ID: "old", URL: targetURL, CheckType: "normal", Timestamp: 100},
		{ID: "start", URL: targetURL, CheckType: "normal", Timestamp: 200},
		{ID: "end", URL: targetURL, CheckType: "normal", Timestamp: 300},
		{ID: "new", URL: targetURL, CheckType: "normal", Timestamp: 400},
	} {
		if err := storage.SaveCheckRecord(record.URL, record); err != nil {
			t.Fatalf("save check record: %v", err)
		}
	}

	result, err := NewTamperAppService(dir, nil).QueryHistory(HistoryFilter{
		StartTime: 200,
		EndTime:   300,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("query history: %v", err)
	}
	if result.Count != 2 || len(result.Records) != 2 {
		t.Fatalf("unexpected time-filtered result: %#v", result)
	}
}

func TestLimitHistoryRecordsAllowsFilteredExportLimit(t *testing.T) {
	records := make([]HistoryRecord, 1001)
	got := limitHistoryRecords(records, 10000, 0)
	if len(got) != 1001 {
		t.Fatalf("filtered export page size = %d, want 1001", len(got))
	}
}
