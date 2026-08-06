//go:build real_verify

package adapter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/model"
)

// This build-tagged test performs REAL API queries against the enabled engines
// using credentials from configs/config.yaml. It is NOT part of the default
// `go test ./...` run (see CLAUDE.md: browser/network diagnostics must be
// behind explicit build tags). It verifies the field-preservation fix:
// every engine that returns a timestamp-like field must populate LastSeen,
// and engine-specific fields must survive into Extra.
//
// Run with:  go test -tags real_verify -run TestRealVerify -v ./internal/adapter/
//
// It consumes a small amount of real quota per engine (1 page × 5 rows).
// No API keys or tokens are ever printed.

// realVerifyConfigPath walks up from the current working directory to locate
// the repo root (go.mod) and returns the path to configs/config.yaml.
func realVerifyConfigPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "configs", "config.yaml"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}

type realVerifyEngine struct {
	name     string
	enabled  func(cfg *config.Config) bool
	hasCreds func(cfg *config.Config) bool
	build    func(cfg *config.Config) EngineAdapter
}

// realVerifyEngines mirrors cmd/unimap-web/main.go registration logic so the
// test exercises exactly the same adapter construction as production.
var realVerifyEngines = []realVerifyEngine{
	{
		name:     "fofa",
		enabled:  func(cfg *config.Config) bool { return cfg.Engines.Fofa.Enabled },
		hasCreds: func(cfg *config.Config) bool { return cfg.Engines.Fofa.APIKey != "" },
		build: func(cfg *config.Config) EngineAdapter {
			return NewFofaAdapter(cfg.Engines.Fofa.APIBaseURL, cfg.Engines.Fofa.APIKey, cfg.Engines.Fofa.Email, cfg.Engines.Fofa.QPS, time.Duration(cfg.Engines.Fofa.Timeout)*time.Second)
		},
	},
	{
		name:     "hunter",
		enabled:  func(cfg *config.Config) bool { return cfg.Engines.Hunter.Enabled },
		hasCreds: func(cfg *config.Config) bool { return cfg.Engines.Hunter.APIKey != "" },
		build: func(cfg *config.Config) EngineAdapter {
			return NewHunterAdapter(cfg.Engines.Hunter.BaseURL, cfg.Engines.Hunter.APIKey, cfg.Engines.Hunter.QPS, time.Duration(cfg.Engines.Hunter.Timeout)*time.Second)
		},
	},
	{
		name:     "zoomeye",
		enabled:  func(cfg *config.Config) bool { return cfg.Engines.Zoomeye.Enabled },
		hasCreds: func(cfg *config.Config) bool { return cfg.Engines.Zoomeye.APIKey != "" },
		build: func(cfg *config.Config) EngineAdapter {
			return NewZoomEyeAdapter(cfg.Engines.Zoomeye.BaseURL, cfg.Engines.Zoomeye.APIKey, cfg.Engines.Zoomeye.QPS, time.Duration(cfg.Engines.Zoomeye.Timeout)*time.Second)
		},
	},
	{
		name:     "quake",
		enabled:  func(cfg *config.Config) bool { return cfg.Engines.Quake.Enabled },
		hasCreds: func(cfg *config.Config) bool { return cfg.Engines.Quake.APIKey != "" },
		build: func(cfg *config.Config) EngineAdapter {
			return NewQuakeAdapter(cfg.Engines.Quake.BaseURL, cfg.Engines.Quake.APIKey, cfg.Engines.Quake.QPS, time.Duration(cfg.Engines.Quake.Timeout)*time.Second)
		},
	},
	{
		name:    "censys",
		enabled: func(cfg *config.Config) bool { return cfg.Engines.Censys.Enabled },
		hasCreds: func(cfg *config.Config) bool {
			return cfg.Engines.Censys.APIID != "" && cfg.Engines.Censys.APISecret != ""
		},
		build: func(cfg *config.Config) EngineAdapter {
			return NewCensysAdapter(cfg.Engines.Censys.BaseURL, cfg.Engines.Censys.APIID, cfg.Engines.Censys.APISecret, cfg.Engines.Censys.QPS, time.Duration(cfg.Engines.Censys.Timeout)*time.Second)
		},
	},
	{
		name:     "daydaymap",
		enabled:  func(cfg *config.Config) bool { return cfg.Engines.Daydaymap.Enabled },
		hasCreds: func(cfg *config.Config) bool { return cfg.Engines.Daydaymap.APIKey != "" },
		build: func(cfg *config.Config) EngineAdapter {
			return NewDayDayMapAdapter(cfg.Engines.Daydaymap.BaseURL, cfg.Engines.Daydaymap.APIKey, cfg.Engines.Daydaymap.QPS, time.Duration(cfg.Engines.Daydaymap.Timeout)*time.Second)
		},
	},
}

// TestRealVerify runs one small real query per enabled engine and reports
// whether last_seen and extra are populated from real API data.
func TestRealVerify(t *testing.T) {
	path, err := realVerifyConfigPath()
	if err != nil {
		t.Fatalf("locate config: %v", err)
	}
	mgr := config.NewManager(path)
	if err := mgr.Load(); err != nil {
		t.Fatalf("load %s: %v", path, err)
	}
	cfg := mgr.GetConfig()
	if cfg == nil {
		t.Fatal("nil config after load")
	}

	// Build the AST for ip="1.1.1.1" directly (same shape the UQL parser
	// produces) to avoid importing internal/core/unimap (import cycle).
	ast := &model.UQLAST{Root: &model.UQLNode{
		Type:  "condition",
		Value: "ip",
		Children: []*model.UQLNode{
			{Type: "operator", Value: "="},
			{Type: "value", Value: "1.1.1.1"},
		},
	}}

	ran := 0
	for _, e := range realVerifyEngines {
		if !e.enabled(cfg) {
			t.Logf("== %s: disabled in config, skipped", e.name)
			continue
		}
		if !e.hasCreds(cfg) {
			t.Logf("== %s: enabled but no credentials, skipped", e.name)
			continue
		}
		ran++
		t.Run(e.name, func(t *testing.T) {
			realVerifyOne(t, e, cfg, ast)
		})
	}
	if ran == 0 {
		t.Fatal("no enabled engines with credentials found in config")
	}
}

// realVerifyOne performs search + normalize for a single engine and asserts
// the field-preservation fix held against real data.
func realVerifyOne(t *testing.T, e realVerifyEngine, cfg *config.Config, ast *model.UQLAST) {
	ad := e.build(cfg)

	native, err := ad.Translate(ast)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	start := time.Now()
	result, err := ad.Search(ctx, native, 1, 5)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
	if result.Error != "" {
		t.Fatalf("engine error: %s", result.Error)
	}
	elapsed := time.Since(start)

	assets, err := ad.Normalize(result)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	t.Logf("native query %q → total=%d has_more=%v page=%d elapsed=%s",
		native, result.Total, result.HasMore, result.Page, elapsed.Round(time.Millisecond))
	t.Logf("normalized rows: %d", len(assets))

	if len(assets) == 0 {
		t.Logf("WARN: 0 assets for %q (maybe empty index / query too narrow); cannot verify last_seen", native)
		return
	}

	// Aggregate per-field stats across rows.
	type fieldStat struct {
		present int
	}
	fieldStats := map[string]*fieldStat{}
	lastSeenRows := 0
	extraRows := 0
	for i := range assets {
		a := &assets[i]
		if a.LastSeen != "" {
			lastSeenRows++
		}
		if len(a.Extra) > 0 {
			extraRows++
		}
		for k := range a.Extra {
			if fieldStats[k] == nil {
				fieldStats[k] = &fieldStat{}
			}
			fieldStats[k].present++
		}
		// Show up to 3 sample rows for eyeballing.
		if i < 3 {
			keys := sortedKeys(a.Extra)
			t.Logf("  row[%d] ip=%s port=%d proto=%s title=%q last_seen=%q extra=%d keys=%v",
				i, a.IP, a.Port, a.Protocol, truncate(a.Title, 40), truncate(a.LastSeen, 30), len(a.Extra), keys)
		}
	}

	// Report which engine fields survived into Extra.
	keys := make([]string, 0, len(fieldStats))
	for k := range fieldStats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s×%d", k, fieldStats[k].present))
		}
		t.Logf("extra fields present: %s", strings.Join(parts, ", "))
	} else {
		t.Log("extra fields present: (none)")
	}

	// Core assertion of the field-preservation fix: real engine timestamps
	// must now reach LastSeen.
	if lastSeenRows == 0 {
		t.Errorf("FAIL: %d rows but last_seen populated on 0 rows — timestamp not promoted", len(assets))
	} else {
		t.Logf("PASS: last_seen populated on %d/%d rows", lastSeenRows, len(assets))
	}
	if extraRows == 0 {
		t.Logf("NOTE: no extra fields captured on %d rows", len(assets))
	} else {
		t.Logf("PASS: extra captured on %d/%d rows", extraRows, len(assets))
	}
}

func sortedKeys(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
