package main

// quota.go implements the "quota" subcommand.

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/utils"
)

type quotaEntry struct {
	Engine    string `json:"engine"`
	Remaining int    `json:"remaining"`
	Total     int    `json:"total"`
	Unit      string `json:"unit"`
	Error     string `json:"error,omitempty"`
}

func runQuotaCommand(args []string) {
	fs := flag.NewFlagSet("quota", flag.ExitOnError)
	configPath := fs.String("c", utils.DefaultConfigPath(), "Config file path")
	engine := fs.String("engine", "", "Check specific engine only")
	format := fs.String("format", "table", "Output format: table or json")
	_ = fs.Parse(args)

	cfgManager := config.NewManager(*configPath)
	if err := cfgManager.Load(); err != nil {
		progress("Warning: %v\n", err)
	}
	cfg := cfgManager.GetConfig()
	if cfg == nil {
		if isJSONFormat(*format) {
			printJSONError("quota", "CONFIG_ERROR", "failed to load configuration", ExitUsageError)
		}
		progress("Error: failed to load configuration from %s\n", *configPath)
		os.Exit(ExitUsageError)
	}

	type quotaCheck struct {
		name    string
		enabled bool
		check   func() (remaining, total int, unit string, err error)
	}
	checks := []quotaCheck{
		{"fofa", cfg.Engines.Fofa.Enabled, func() (int, int, string, error) {
			a := adapter.NewFofaAdapter(cfg.Engines.Fofa.APIBaseURL, cfg.Engines.Fofa.APIKey, cfg.Engines.Fofa.Email, cfg.Engines.Fofa.QPS, time.Duration(cfg.Engines.Fofa.Timeout)*time.Second)
			q, err := a.GetQuota()
			if err != nil {
				return -1, -1, "", err
			}
			if q == nil {
				return -1, -1, "", fmt.Errorf("not available")
			}
			return q.Remaining, q.Total, q.Unit, nil
		}},
		{"hunter", cfg.Engines.Hunter.Enabled, func() (int, int, string, error) {
			a := adapter.NewHunterAdapter(cfg.Engines.Hunter.BaseURL, cfg.Engines.Hunter.APIKey, cfg.Engines.Hunter.BackupAPIKey, cfg.Engines.Hunter.QPS, time.Duration(cfg.Engines.Hunter.Timeout)*time.Second)
			q, err := a.GetQuota()
			if err != nil {
				return -1, -1, "", err
			}
			if q == nil {
				return -1, -1, "", fmt.Errorf("not available")
			}
			return q.Remaining, q.Total, q.Unit, nil
		}},
		{"zoomeye", cfg.Engines.Zoomeye.Enabled, func() (int, int, string, error) {
			a := adapter.NewZoomEyeAdapter(cfg.Engines.Zoomeye.BaseURL, cfg.Engines.Zoomeye.APIKey, cfg.Engines.Zoomeye.QPS, time.Duration(cfg.Engines.Zoomeye.Timeout)*time.Second)
			q, err := a.GetQuota()
			if err != nil {
				return -1, -1, "", err
			}
			if q == nil {
				return -1, -1, "", fmt.Errorf("not available")
			}
			return q.Remaining, q.Total, q.Unit, nil
		}},
		{"quake", cfg.Engines.Quake.Enabled, func() (int, int, string, error) {
			a := adapter.NewQuakeAdapter(cfg.Engines.Quake.BaseURL, cfg.Engines.Quake.APIKey, cfg.Engines.Quake.QPS, time.Duration(cfg.Engines.Quake.Timeout)*time.Second)
			q, err := a.GetQuota()
			if err != nil {
				return -1, -1, "", err
			}
			if q == nil {
				return -1, -1, "", fmt.Errorf("not available")
			}
			return q.Remaining, q.Total, q.Unit, nil
		}},
		{"shodan", cfg.Engines.Shodan.Enabled, func() (int, int, string, error) {
			a := adapter.NewShodanAdapter(cfg.Engines.Shodan.BaseURL, cfg.Engines.Shodan.APIKey, cfg.Engines.Shodan.QPS, time.Duration(cfg.Engines.Shodan.Timeout)*time.Second)
			q, err := a.GetQuota()
			if err != nil {
				return -1, -1, "", err
			}
			if q == nil {
				return -1, -1, "", fmt.Errorf("not available")
			}
			return q.Remaining, q.Total, q.Unit, nil
		}},
		{"censys", cfg.Engines.Censys.Enabled, func() (int, int, string, error) {
			return -1, -1, "", fmt.Errorf("quota API not available")
		}},
		{"daydaymap", cfg.Engines.Daydaymap.Enabled, func() (int, int, string, error) {
			return -1, -1, "", fmt.Errorf("quota API not available")
		}},
	}

	var results []quotaEntry
	for _, c := range checks {
		if *engine != "" && c.name != *engine {
			continue
		}
		if !c.enabled && *engine == "" {
			continue
		}
		entry := quotaEntry{Engine: c.name}
		remaining, total, unit, err := c.check()
		if err != nil {
			entry.Remaining = -1
			entry.Total = -1
			entry.Unit = unit
			entry.Error = err.Error()
		} else {
			entry.Remaining = remaining
			entry.Total = total
			entry.Unit = unit
		}
		results = append(results, entry)
	}

	if isJSONFormat(*format) {
		printJSON("quota", map[string]interface{}{"engines": results}, ExitOK)
		return
	}
	// Table output
	fmt.Printf("%-12s %-12s %-12s %-15s %s\n", "ENGINE", "REMAINING", "TOTAL", "UNIT", "ERROR")
	for _, e := range results {
		errStr := e.Error
		if errStr == "" {
			errStr = "-"
		}
		unitStr := e.Unit
		if unitStr == "" {
			unitStr = "-"
		}
		fmt.Printf("%-12s %-12d %-12d %-15s %s\n", e.Engine, e.Remaining, e.Total, unitStr, errStr)
	}
}
