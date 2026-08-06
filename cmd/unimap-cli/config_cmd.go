package main

// config_cmd.go implements the "config show" subcommand.

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/utils"
)

func runConfigCommand(args []string) {
	if len(args) == 0 {
		progress("Usage: unimap-cli config show [--format json] [--show-secrets]\n")
		os.Exit(ExitUsageError)
	}
	switch args[0] {
	case "show":
		runConfigShow(args[1:])
	default:
		progress("Unknown config command: %s\n", args[0])
		os.Exit(ExitUsageError)
	}
}

func runConfigShow(args []string) {
	fs := flag.NewFlagSet("config show", flag.ExitOnError)
	configPath := fs.String("c", utils.DefaultConfigPath(), "Config file path")
	format := fs.String("format", "table", "Output format: table or json")
	showSecrets := fs.Bool("show-secrets", false, "Show full API keys instead of masked")
	_ = fs.Parse(args)

	cfgManager := config.NewManager(*configPath)
	if err := cfgManager.Load(); err != nil {
		progress("Warning: %v\n", err)
	}
	cfg := cfgManager.GetConfig()
	if cfg == nil {
		if isJSONFormat(*format) {
			printJSONError("config", "CONFIG_ERROR", "failed to load configuration", ExitUsageError)
		}
		progress("Error: failed to load configuration from %s\n", *configPath)
		os.Exit(ExitUsageError)
	}

	mask := func(s string) string {
		if *showSecrets || len(s) <= 4 {
			return s
		}
		return s[:4] + strings.Repeat("*", len(s)-4)
	}

	type engineConfig struct {
		Enabled      bool   `json:"enabled"`
		APIKey       string `json:"api_key,omitempty"`
		BackupAPIKey string `json:"backup_api_key,omitempty"`
		BaseURL      string `json:"base_url,omitempty"`
	}

	engines := map[string]engineConfig{
		"fofa":      {cfg.Engines.Fofa.Enabled, mask(cfg.Engines.Fofa.APIKey), "", cfg.Engines.Fofa.APIBaseURL},
		"hunter":    {cfg.Engines.Hunter.Enabled, mask(cfg.Engines.Hunter.APIKey), mask(cfg.Engines.Hunter.BackupAPIKey), cfg.Engines.Hunter.BaseURL},
		"zoomeye":   {cfg.Engines.Zoomeye.Enabled, mask(cfg.Engines.Zoomeye.APIKey), "", cfg.Engines.Zoomeye.BaseURL},
		"quake":     {cfg.Engines.Quake.Enabled, mask(cfg.Engines.Quake.APIKey), "", cfg.Engines.Quake.BaseURL},
		"shodan":    {cfg.Engines.Shodan.Enabled, mask(cfg.Engines.Shodan.APIKey), "", cfg.Engines.Shodan.BaseURL},
		"censys":    {cfg.Engines.Censys.Enabled, mask(cfg.Engines.Censys.APIID), "", cfg.Engines.Censys.BaseURL},
		"daydaymap": {cfg.Engines.Daydaymap.Enabled, mask(cfg.Engines.Daydaymap.APIKey), "", cfg.Engines.Daydaymap.BaseURL},
	}
	data := map[string]interface{}{
		"config_path": *configPath,
		"engines":     engines,
		"screenshot": map[string]interface{}{
			"mode":     cfg.Screenshot.Mode,
			"base_dir": cfg.Screenshot.BaseDir,
		},
		"scheduler": map[string]interface{}{
			"enabled": cfg.Scheduler.Enabled,
		},
	}

	if isJSONFormat(*format) {
		printJSON("config", data, ExitOK)
		return
	}
	// Table output
	fmt.Printf("Config: %s\n\n", *configPath)
	fmt.Printf("%-12s %-10s %-20s %-20s %s\n", "ENGINE", "ENABLED", "API_KEY", "BACKUP_KEY", "BASE_URL")
	for _, name := range []string{"fofa", "hunter", "zoomeye", "quake", "shodan", "censys", "daydaymap"} {
		e := engines[name]
		key := e.APIKey
		if key == "" {
			key = "(not set)"
		}
		backup := e.BackupAPIKey
		if backup == "" {
			backup = "-"
		}
		fmt.Printf("%-12s %-10v %-20s %-20s %s\n", name, e.Enabled, key, backup, e.BaseURL)
	}
	fmt.Printf("\nScreenshot: mode=%s base_dir=%s\n", cfg.Screenshot.Mode, cfg.Screenshot.BaseDir)
	fmt.Printf("Scheduler:  enabled=%v\n", cfg.Scheduler.Enabled)
}
