package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/appversion"
	"github.com/unimap/project/internal/config"
	"github.com/unimap/project/internal/exporter"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/service"
	"github.com/unimap/project/internal/utils"
)

func main() {
	if len(os.Args) > 1 {
		sub := os.Args[1]
		switch sub {
		case "engines":
			runEnginesCommand(os.Args[2:])
			return
		case "help":
			runHelpCommand(os.Args[2:])
			return
		case "quota":
			runQuotaCommand(os.Args[2:])
			return
		case "config":
			runConfigCommand(os.Args[2:])
			return
		}
		if !strings.HasPrefix(sub, "-") {
			if runAPISubcommand(sub, os.Args[2:]) {
				return
			}
		}
	}

	flags := parseCLIFlags()

	if flags.version {
		if isJSONFormat(flags.format) {
			printJSON("version", map[string]string{"version": appversion.Full()}, ExitOK)
		}
		fmt.Printf("UniMap CLI %s\n", appversion.Full())
		return
	}

	if flags.query == "" {
		if isJSONFormat(flags.format) {
			printJSONError("query", "USAGE_ERROR", "Query string is required. Use -q '<uql>'.", ExitUsageError)
		}
		fmt.Fprintln(os.Stderr, "Error: Query string is required")
		flag.Usage()
		os.Exit(ExitUsageError)
	}

	cfg, cfgManager, svc := initCLIService(flags.config)
	if cfg != nil {
		if applyCookiesFromFlags(cfg, flags.fofaCookie, flags.hunterCookie, flags.quakeCookie, flags.zoomeyeCookie, flags.shodanCookie, flags.censysCookie, flags.daydaymapCookie) {
			if err := cfgManager.Save(); err != nil {
				logger.Warnf("Failed to save cookies to %s: %v", flags.config, err)
			}
		}
	}

	registerEngines(svc, cfg)
	engines := selectCLIEngines(cfg, flags.engines, flags.config)

	if len(engines) == 0 {
		msg := fmt.Sprintf("No engines configured/enabled. Set API keys in %s or use -e.", flags.config)
		if isJSONFormat(flags.format) {
			printJSONError("query", "NO_ENGINES", msg, ExitNoEngines)
		}
		fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
		os.Exit(ExitNoEngines)
	}

	progress("Querying with engines: %v\n", engines)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(flags.timeout)*time.Second)
	defer cancel()

	resp, err := svc.Query(ctx, service.QueryRequest{
		Query:       flags.query,
		Engines:     engines,
		PageSize:    flags.limit,
		ProcessData: true,
	})
	if err != nil {
		code, exitCode := classifyError(err)
		if isJSONFormat(flags.format) {
			printJSONError("query", code, err.Error(), exitCode)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitCode)
	}

	outputCLIResults(resp, flags.query, flags.output, flags.format, flags.force, flags.page, flags.limit, flags.fields)

	if err := svc.Shutdown(); err != nil {
		logger.Warnf("Error during shutdown: %v", err)
	}
}

type cliFlags struct {
	query, engines, output, config, format string
	fofaCookie, hunterCookie, quakeCookie  string
	zoomeyeCookie, shodanCookie            string
	censysCookie, daydaymapCookie          string
	fields                                 string
	limit, timeout, page                   int
	version, force                         bool
}

func parseCLIFlags() cliFlags {
	var f cliFlags
	flag.StringVar(&f.query, "q", "", "Query string")
	flag.StringVar(&f.engines, "e", "", "Comma-separated engines")
	flag.IntVar(&f.limit, "l", 100, "Result limit")
	flag.IntVar(&f.page, "page", 1, "Page number")
	flag.StringVar(&f.output, "o", "", "Output file path")
	flag.StringVar(&f.config, "c", utils.DefaultConfigPath(), "Config file path")
	flag.StringVar(&f.format, "format", "table", "Output format: table or json")
	flag.StringVar(&f.fields, "fields", "", "Comma-separated output fields (ip,port,title,...)")
	flag.IntVar(&f.timeout, "timeout", 60, "Query timeout in seconds")
	flag.BoolVar(&f.force, "force", false, "Overwrite output file if exists")
	flag.StringVar(&f.fofaCookie, "cookie-fofa", "", "FOFA cookie header")
	flag.StringVar(&f.hunterCookie, "cookie-hunter", "", "Hunter cookie header")
	flag.StringVar(&f.quakeCookie, "cookie-quake", "", "Quake cookie header")
	flag.StringVar(&f.zoomeyeCookie, "cookie-zoomeye", "", "ZoomEye cookie header")
	flag.StringVar(&f.shodanCookie, "cookie-shodan", "", "Shodan cookie header")
	flag.StringVar(&f.censysCookie, "cookie-censys", "", "Censys cookie header")
	flag.StringVar(&f.daydaymapCookie, "cookie-daydaymap", "", "DayDayMap cookie header")
	flag.BoolVar(&f.version, "version", false, "Print version")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "UniMap CLI %s\n\n", appversion.Full())
		fmt.Fprintf(os.Stderr, "Usage: %s -q '<uql>' [-e fofa,hunter] [-l 100] [--page 1] [-o results.csv] [--format table|json] [--fields ip,port,title] [--timeout 60] [--force]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nSubcommands:\n")
		fmt.Fprintf(os.Stderr, "  engines         List configured engines and their status\n")
		fmt.Fprintf(os.Stderr, "  help            Show help for a subcommand\n")
		fmt.Fprintf(os.Stderr, "  quota           Show engine quota usage via Web API\n")
		fmt.Fprintf(os.Stderr, "  config          Manage CLI configuration\n")
		fmt.Fprintf(os.Stderr, "  query           Query via Web API\n")
		fmt.Fprintf(os.Stderr, "  tamper-check    Run tamper check via Web API\n")
		fmt.Fprintf(os.Stderr, "  screenshot-batch  Batch screenshot via Web API\n")
		fmt.Fprintf(os.Stderr, "  scheduler       Manage scheduled tasks via Web API\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Env var overrides (flag > env > default)
	if f.config == utils.DefaultConfigPath() {
		f.config = envOrDefault("UNIMAP_CONFIG_PATH", f.config)
	}
	if f.format == "table" {
		f.format = envOrDefault("UNIMAP_FORMAT", f.format)
	}
	if f.timeout == 60 {
		f.timeout = envIntOrDefault("UNIMAP_TIMEOUT", f.timeout)
	}

	return f
}

func runEnginesCommand(args []string) {
	fs := flag.NewFlagSet("engines", flag.ExitOnError)
	configPath := fs.String("c", utils.DefaultConfigPath(), "Config file path")
	format := fs.String("format", "table", "Output format: table or json")
	_ = fs.Parse(args)

	cfgManager := config.NewManager(*configPath)
	if err := cfgManager.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	cfg := cfgManager.GetConfig()

	engines := []engineInfoEntry{
		{"fofa", cfg.Engines.Fofa.Enabled, cfg.Engines.Fofa.APIKey != ""},
		{"hunter", cfg.Engines.Hunter.Enabled, cfg.Engines.Hunter.APIKey != ""},
		{"zoomeye", cfg.Engines.Zoomeye.Enabled, cfg.Engines.Zoomeye.APIKey != ""},
		{"quake", cfg.Engines.Quake.Enabled, cfg.Engines.Quake.APIKey != ""},
		{"shodan", cfg.Engines.Shodan.Enabled, cfg.Engines.Shodan.APIKey != ""},
		{"censys", cfg.Engines.Censys.Enabled, cfg.Engines.Censys.APIID != ""},
		{"daydaymap", cfg.Engines.Daydaymap.Enabled, cfg.Engines.Daydaymap.APIKey != ""},
	}

	if isJSONFormat(*format) {
		printJSON("engines", engines, ExitOK)
	}

	// Table output
	fmt.Printf("%-12s %-10s %-10s\n", "ENGINE", "ENABLED", "HAS_KEY")
	for _, e := range engines {
		fmt.Printf("%-12s %-10v %-10v\n", e.Name, e.Enabled, e.HasAPIKey)
	}
}

func initCLIService(configPath string) (*config.Config, *config.Manager, *service.UnifiedService) {
	cfgManager := config.NewManager(configPath)
	if err := cfgManager.Load(); err != nil {
		logger.Warnf("Failed to load config from %s: %v. Using defaults.", configPath, err)
	}
	cfg := cfgManager.GetConfig()
	svc := service.NewUnifiedServiceWithConfig(cfg)
	return cfg, cfgManager, svc
}

func selectCLIEngines(cfg *config.Config, enginesFlag, configPath string) []string {
	var engines []string
	if enginesFlag != "" {
		for _, e := range strings.Split(enginesFlag, ",") {
			if e = strings.TrimSpace(e); e != "" {
				engines = append(engines, e)
			}
		}
	}
	if len(engines) == 0 {
		engines = getEnabledEngines(cfg)
	}
	return engines
}

func outputCLIResults(resp *service.QueryResponse, query, output, format string, force bool, page, pageSize int, fields string) {
	if isJSONFormat(format) {
		data := queryOutputData{
			Query:       query,
			Assets:      resp.Assets,
			Total:       resp.TotalCount,
			Page:        page,
			PageSize:    pageSize,
			HasMore:     resp.TotalCount > page*pageSize,
			EngineStats: resp.EngineStats,
			Errors:      resp.Errors,
		}
		if fields != "" {
			printJSON("query", map[string]interface{}{
				"query":        data.Query,
				"assets":       filterAssetFields(resp.Assets, parseFields(fields)),
				"total":        data.Total,
				"page":         data.Page,
				"page_size":    data.PageSize,
				"has_more":     data.HasMore,
				"engine_stats": data.EngineStats,
				"errors":       data.Errors,
			}, ExitOK)
			return
		}
		printJSON("query", data, ExitOK)
		return
	}

	// Table mode
	progress("Found %d results.\n", resp.TotalCount)
	for engine, count := range resp.EngineStats {
		progress("  %s: %d\n", engine, count)
	}
	for _, errMsg := range resp.Errors {
		progress("  Error: %s\n", errMsg)
	}

	if output != "" {
		if err := saveResults(resp.Assets, output, force); err != nil {
			progress("Failed to save results: %v\n", err)
			os.Exit(ExitQueryError)
		}
		progress("Results saved to %s\n", output)
	} else {
		for _, asset := range resp.Assets {
			fmt.Printf("%s\t%s:%d\t%s\n", asset.IP, asset.Host, asset.Port, asset.Title)
		}
	}
}

func applyCookiesFromFlags(cfg *config.Config, fofa, hunter, quake, zoomeye, shodan, censys, daydaymap string) bool {
	if cfg == nil {
		return false
	}
	changed := false
	if strings.TrimSpace(fofa) != "" {
		cfg.Engines.Fofa.Cookies = config.ParseCookieHeader(fofa, config.DefaultCookieDomain("fofa"))
		changed = true
	}
	if strings.TrimSpace(hunter) != "" {
		cfg.Engines.Hunter.Cookies = config.ParseCookieHeader(hunter, config.DefaultCookieDomain("hunter"))
		changed = true
	}
	if strings.TrimSpace(quake) != "" {
		cfg.Engines.Quake.Cookies = config.ParseCookieHeader(quake, config.DefaultCookieDomain("quake"))
		changed = true
	}
	if strings.TrimSpace(zoomeye) != "" {
		cfg.Engines.Zoomeye.Cookies = config.ParseCookieHeader(zoomeye, config.DefaultCookieDomain("zoomeye"))
		changed = true
	}
	if strings.TrimSpace(shodan) != "" {
		cfg.Engines.Shodan.Cookies = config.ParseCookieHeader(shodan, config.DefaultCookieDomain("shodan"))
		changed = true
	}
	if strings.TrimSpace(censys) != "" {
		cfg.Engines.Censys.Cookies = config.ParseCookieHeader(censys, config.DefaultCookieDomain("censys"))
		changed = true
	}
	if strings.TrimSpace(daydaymap) != "" {
		cfg.Engines.Daydaymap.Cookies = config.ParseCookieHeader(daydaymap, config.DefaultCookieDomain("daydaymap"))
		changed = true
	}
	return changed
}

func getEnabledEngines(cfg *config.Config) []string {
	var list []string
	if cfg.Engines.Fofa.Enabled {
		list = append(list, "fofa")
	}
	if cfg.Engines.Hunter.Enabled {
		list = append(list, "hunter")
	}
	if cfg.Engines.Quake.Enabled {
		list = append(list, "quake")
	}
	if cfg.Engines.Zoomeye.Enabled {
		list = append(list, "zoomeye")
	}
	if cfg.Engines.Shodan.Enabled {
		list = append(list, "shodan")
	}
	if cfg.Engines.Censys.Enabled {
		list = append(list, "censys")
	}
	if cfg.Engines.Daydaymap.Enabled {
		list = append(list, "daydaymap")
	}
	return list
}

func registerEngines(svc *service.UnifiedService, cfg *config.Config) {
	// Env var API key overrides (env fills only when config value is empty)
	if v := os.Getenv("UNIMAP_FOFA_API_KEY"); v != "" && cfg.Engines.Fofa.APIKey == "" {
		cfg.Engines.Fofa.APIKey = v
	}
	if v := os.Getenv("UNIMAP_HUNTER_API_KEY"); v != "" && cfg.Engines.Hunter.APIKey == "" {
		cfg.Engines.Hunter.APIKey = v
	}
	if v := os.Getenv("UNIMAP_ZOOMEYE_API_KEY"); v != "" && cfg.Engines.Zoomeye.APIKey == "" {
		cfg.Engines.Zoomeye.APIKey = v
	}
	if v := os.Getenv("UNIMAP_QUAKE_API_KEY"); v != "" && cfg.Engines.Quake.APIKey == "" {
		cfg.Engines.Quake.APIKey = v
	}
	if v := os.Getenv("UNIMAP_SHODAN_API_KEY"); v != "" && cfg.Engines.Shodan.APIKey == "" {
		cfg.Engines.Shodan.APIKey = v
	}
	if v := os.Getenv("UNIMAP_CENSYS_API_ID"); v != "" && cfg.Engines.Censys.APIID == "" {
		cfg.Engines.Censys.APIID = v
	}
	if v := os.Getenv("UNIMAP_CENSYS_API_SECRET"); v != "" && cfg.Engines.Censys.APISecret == "" {
		cfg.Engines.Censys.APISecret = v
	}
	if v := os.Getenv("UNIMAP_DAYDAYMAP_API_KEY"); v != "" && cfg.Engines.Daydaymap.APIKey == "" {
		cfg.Engines.Daydaymap.APIKey = v
	}

	type engineReg struct {
		enabled bool
		reg     func()
	}
	regs := []engineReg{
		{cfg.Engines.Fofa.Enabled, func() {
			svc.RegisterAdapter(adapter.NewFofaAdapter(cfg.Engines.Fofa.APIBaseURL, cfg.Engines.Fofa.APIKey, cfg.Engines.Fofa.Email, cfg.Engines.Fofa.QPS, time.Duration(cfg.Engines.Fofa.Timeout)*time.Second))
		}},
		{cfg.Engines.Hunter.Enabled, func() {
			svc.RegisterAdapter(adapter.NewHunterAdapter(cfg.Engines.Hunter.BaseURL, cfg.Engines.Hunter.APIKey, cfg.Engines.Hunter.BackupAPIKey, cfg.Engines.Hunter.QPS, time.Duration(cfg.Engines.Hunter.Timeout)*time.Second))
		}},
		{cfg.Engines.Zoomeye.Enabled, func() {
			svc.RegisterAdapter(adapter.NewZoomEyeAdapter(cfg.Engines.Zoomeye.BaseURL, cfg.Engines.Zoomeye.APIKey, cfg.Engines.Zoomeye.QPS, time.Duration(cfg.Engines.Zoomeye.Timeout)*time.Second))
		}},
		{cfg.Engines.Quake.Enabled, func() {
			svc.RegisterAdapter(adapter.NewQuakeAdapter(cfg.Engines.Quake.BaseURL, cfg.Engines.Quake.APIKey, cfg.Engines.Quake.QPS, time.Duration(cfg.Engines.Quake.Timeout)*time.Second))
		}},
		{cfg.Engines.Shodan.Enabled && cfg.Engines.Shodan.APIKey != "", func() {
			svc.RegisterAdapter(adapter.NewShodanAdapter(cfg.Engines.Shodan.BaseURL, cfg.Engines.Shodan.APIKey, cfg.Engines.Shodan.QPS, time.Duration(cfg.Engines.Shodan.Timeout)*time.Second))
		}},
		{cfg.Engines.Censys.Enabled, func() {
			svc.RegisterAdapter(adapter.NewCensysAdapter(cfg.Engines.Censys.BaseURL, cfg.Engines.Censys.APIID, cfg.Engines.Censys.APISecret, cfg.Engines.Censys.QPS, time.Duration(cfg.Engines.Censys.Timeout)*time.Second))
		}},
		{cfg.Engines.Daydaymap.Enabled, func() {
			svc.RegisterAdapter(adapter.NewDayDayMapAdapter(cfg.Engines.Daydaymap.BaseURL, cfg.Engines.Daydaymap.APIKey, cfg.Engines.Daydaymap.QPS, time.Duration(cfg.Engines.Daydaymap.Timeout)*time.Second))
		}},
	}
	for _, r := range regs {
		if r.enabled {
			r.reg()
		}
	}
}

func saveResults(assets []model.UnifiedAsset, path string, force ...bool) error {
	f := false
	if len(force) > 0 {
		f = force[0]
	}

	lowerPath := strings.ToLower(path)

	switch {
	case strings.HasSuffix(lowerPath, ".json"):
		if f {
			_ = os.Remove(path)
		}
		exp := exporter.NewJSONExporter()
		return exp.Export(assets, path)
	case strings.HasSuffix(lowerPath, ".xlsx") || strings.HasSuffix(lowerPath, ".xls"):
		if f {
			_ = os.Remove(path)
		}
		exp := exporter.NewExcelExporter()
		return exp.Export(assets, path)
	default:
		return saveResultsCSV(assets, path, f)
	}
}

func saveResultsCSV(assets []model.UnifiedAsset, path string, force bool) error {
	openFlags := os.O_CREATE | os.O_EXCL | os.O_WRONLY
	if force {
		openFlags = os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	}
	f, err := os.OpenFile(path, openFlags, 0644)
	if err != nil {
		return fmt.Errorf("file %q already exists, refusing to overwrite: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"IP", "Port", "Protocol", "Domain", "Title", "Country", "City", "ISP", "Source"}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, asset := range assets {
		record := []string{
			asset.IP,
			fmt.Sprintf("%d", asset.Port),
			asset.Protocol,
			asset.Host,
			asset.Title,
			asset.CountryCode,
			asset.City,
			asset.ISP,
			asset.Source,
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	return nil
}
