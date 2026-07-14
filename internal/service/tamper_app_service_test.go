package service

import (
	"testing"

	"github.com/unimap/project/internal/tamper"
)

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
	if result.Count != 1 || len(result.Records) != 1 {
		t.Fatalf("unexpected page size: count=%d records=%#v", result.Count, result.Records)
	}
	if result.Records[0].ID != "primary-middle" {
		t.Fatalf("record id = %q, want primary-middle", result.Records[0].ID)
	}
	if len(result.URLOptions) != 1 || result.URLOptions[0] != primaryURL {
		t.Fatalf("unexpected URL options: %#v", result.URLOptions)
	}
}
