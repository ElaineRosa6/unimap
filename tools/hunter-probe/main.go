//go:build hunter_probe

// Command hunter-probe runs a single Hunter search through the production
// HunterAdapter (including backup-key failover) with the logger initialized so
// failover switches are visible. Used to diagnose live API key state without
// modifying CLI or adapter code.
//
//	go run -tags hunter_probe ./tools/hunter-probe [config-path] [native-query]
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/logger"
)

func main() {
	cfgPath := "configs/config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}
	// Default is a neutral placeholder — pass the live target query as arg 2
	// (it must not be committed to the repo).
	query := `web.body="placeholder"`
	if len(os.Args) > 2 {
		query = os.Args[2]
	}
	pageSize := 5
	if len(os.Args) > 3 {
		fmt.Sscanf(os.Args[3], "%d", &pageSize)
	}

	logger.Init(logger.Config{Level: logger.LevelDebug, Encoding: "console", File: ""})

	mgr := config.NewManager(cfgPath)
	if err := mgr.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	cfg := mgr.GetConfig()

	h := adapter.NewHunterAdapter(
		cfg.Engines.Hunter.BaseURL,
		cfg.Engines.Hunter.APIKey,
		cfg.Engines.Hunter.BackupAPIKey,
		cfg.Engines.Hunter.QPS,
		time.Duration(cfg.Engines.Hunter.Timeout)*time.Second,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	fmt.Printf("== hunter probe: base=%s qps=%d page_size=%d query=%q\n", cfg.Engines.Hunter.BaseURL, cfg.Engines.Hunter.QPS, pageSize, query)
	res, err := h.Search(ctx, query, 1, pageSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "search error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("result: total=%d has_more=%v err=%q\n", res.Total, res.HasMore, res.Error)
	assets, nErr := h.Normalize(res)
	if nErr != nil {
		fmt.Fprintf(os.Stderr, "normalize error: %v\n", nErr)
		os.Exit(1)
	}
	fmt.Printf("normalized=%d\n", len(assets))
}
