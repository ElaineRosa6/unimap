package main

// help.go implements the "help" subcommand with --json self-description.

import (
	"flag"
	"fmt"

	"github.com/unimap/project/internal/appversion"
)

func runHelpCommand(args []string) {
	fs := flag.NewFlagSet("help", flag.ExitOnError)
	format := fs.String("format", "table", "Output format: table or json")
	_ = fs.Parse(args)

	if isJSONFormat(*format) {
		printJSON("help", helpSchema(), ExitOK)
		return
	}
	// Human-readable help
	fmt.Printf("UniMap CLI %s\n\n", appversion.Full())
	fmt.Println("Usage: unimap-cli -q '<uql>' [flags]")
	fmt.Println("       unimap-cli <subcommand> [flags]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  engines           List configured engines and their status")
	fmt.Println("  quota             Check engine API quota")
	fmt.Println("  config show       Show current configuration")
	fmt.Println("  help              Show this help (--format json for machine-readable)")
	fmt.Println("  query             Query via Web API")
	fmt.Println("  tamper-check      Run tamper check via Web API")
	fmt.Println("  screenshot-batch  Batch screenshot via Web API")
	fmt.Println("  scheduler         Manage scheduled tasks via Web API")
	fmt.Println()
	fmt.Println("Direct query flags:")
	fmt.Println("  -q string        UQL query string (required)")
	fmt.Println("  -e string        Comma-separated engines")
	fmt.Println("  -l int           Result limit / page size (default 100)")
	fmt.Println("  --page int       Page number (default 1)")
	fmt.Println("  -o string        Output file path (csv/json/xlsx)")
	fmt.Println("  --format string  Output format: table or json (default table)")
	fmt.Println("  --fields string  Comma-separated output fields")
	fmt.Println("  --timeout int    Query timeout in seconds (default 60)")
	fmt.Println("  --force          Overwrite output file if exists")
	fmt.Println("  -c string        Config file path")
	fmt.Println("  --version        Print version")
	fmt.Println()
	fmt.Println("Environment variables:")
	fmt.Println("  UNIMAP_CONFIG_PATH       Config file path")
	fmt.Println("  UNIMAP_FORMAT            Default output format")
	fmt.Println("  UNIMAP_API_BASE          Web API base URL")
	fmt.Println("  UNIMAP_ADMIN_TOKEN       Web API admin token")
	fmt.Println("  UNIMAP_FOFA_API_KEY      FOFA API key")
	fmt.Println("  UNIMAP_HUNTER_API_KEY    Hunter API key")
	fmt.Println("  UNIMAP_ZOOMEYE_API_KEY   ZoomEye API key")
	fmt.Println("  UNIMAP_QUAKE_API_KEY     Quake API key")
	fmt.Println("  UNIMAP_SHODAN_API_KEY    Shodan API key")
	fmt.Println("  UNIMAP_CENSYS_API_ID     Censys API ID")
	fmt.Println("  UNIMAP_CENSYS_API_SECRET Censys API secret")
	fmt.Println("  UNIMAP_DAYDAYMAP_API_KEY DayDayMap API key")
	fmt.Println()
	fmt.Println("Exit codes:")
	fmt.Println("  0  Success")
	fmt.Println("  1  Query error")
	fmt.Println("  2  Authentication error")
	fmt.Println("  3  No engines available")
	fmt.Println("  4  Usage error")
	fmt.Println("  5  Server unreachable")
	fmt.Println("  6  Timeout")
}

func helpSchema() map[string]interface{} {
	return map[string]interface{}{
		"name":    "unimap-cli",
		"version": appversion.Full(),
		"commands": []map[string]interface{}{
			{
				"name": "query (direct)", "description": "Direct query via API adapters (no web server needed)",
				"flags": []map[string]interface{}{
					{"name": "q", "type": "string", "required": true, "description": "UQL query string"},
					{"name": "e", "type": "string", "required": false, "description": "Comma-separated engines"},
					{"name": "l", "type": "int", "default": 100, "description": "Page size"},
					{"name": "page", "type": "int", "default": 1, "description": "Page number"},
					{"name": "o", "type": "string", "required": false, "description": "Output file (csv/json/xlsx)"},
					{"name": "format", "type": "enum", "values": []string{"table", "json"}, "default": "table"},
					{"name": "fields", "type": "string", "required": false, "description": "Comma-separated output fields"},
					{"name": "timeout", "type": "int", "default": 60, "description": "Timeout in seconds"},
					{"name": "force", "type": "bool", "default": false, "description": "Overwrite output file"},
				},
			},
			{"name": "engines", "description": "List configured engines", "flags": []map[string]interface{}{
				{"name": "format", "type": "enum", "values": []string{"table", "json"}, "default": "table"},
			}},
			{"name": "quota", "description": "Check engine API quota", "flags": []map[string]interface{}{
				{"name": "engine", "type": "string", "required": false, "description": "Check specific engine"},
				{"name": "format", "type": "enum", "values": []string{"table", "json"}, "default": "table"},
			}},
			{"name": "config show", "description": "Show current configuration", "flags": []map[string]interface{}{
				{"name": "format", "type": "enum", "values": []string{"table", "json"}, "default": "table"},
				{"name": "show-secrets", "type": "bool", "default": false, "description": "Show full API keys"},
			}},
			{"name": "query (api)", "description": "Query via Web API", "flags": []map[string]interface{}{
				{"name": "q", "type": "string", "required": true}, {"name": "e", "type": "string"},
				{"name": "l", "type": "int", "default": 100}, {"name": "page", "type": "int", "default": 1},
				{"name": "api-base", "type": "string", "default": "http://127.0.0.1:8448"},
				{"name": "format", "type": "enum", "values": []string{"table", "json"}, "default": "table"},
			}},
			{"name": "tamper-check", "description": "Run tamper check via Web API"},
			{"name": "screenshot-batch", "description": "Batch screenshot via Web API"},
			{"name": "scheduler", "description": "Manage scheduled tasks (list/run/create/enable/disable/delete/history)"},
		},
		"exit_codes": map[string]string{
			"0": "success", "1": "query error", "2": "auth error",
			"3": "no engines", "4": "usage error", "5": "server unreachable", "6": "timeout",
		},
	}
}
