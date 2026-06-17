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
)

func main() {
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		if runAPISubcommand(os.Args[1], os.Args[2:]) { return }
	}
	flags := parseCLIFlags()
	if flags.version { fmt.Printf("UniMap CLI %s\n", appversion.Full()); return }
	if flags.query == "" { fmt.Println("Error: Query string is required"); flag.Usage(); os.Exit(1) }
	cfg, cfgManager, svc := initCLIService(flags.config)
	if cfg != nil {
		if applyCookiesFromFlags(cfg, flags.fofaCookie, flags.hunterCookie, flags.quakeCookie, flags.zoomeyeCookie) {
			if err := cfgManager.Save(); err != nil { logger.Warnf("Failed to save cookies to %s: %v", flags.config, err) }
		}
	}
	registerEngines(svc, cfg)
	engines := selectCLIEngines(cfg, flags.engines, flags.config)
	fmt.Printf("Querying with engines: %v\n", engines)
	resp, err := svc.Query(context.Background(), service.QueryRequest{Query: flags.query, Engines: engines, PageSize: flags.limit, ProcessData: true})
	if err != nil { logger.Errorf("Query failed: %v", err); os.Exit(1) }
	outputCLIResults(resp, flags.output)
	if err := svc.Shutdown(); err != nil { logger.Warnf("Error during shutdown: %v", err) }
}

type cliFlags struct {
	query, engines, output, config, fofaCookie, hunterCookie, quakeCookie, zoomeyeCookie string
	limit int
	version bool
}

func parseCLIFlags() cliFlags {
	var f cliFlags
	flag.StringVar(&f.query, "q", "", "Query string")
	flag.StringVar(&f.engines, "e", "", "Comma-separated engines")
	flag.IntVar(&f.limit, "l", 100, "Result limit")
	flag.StringVar(&f.output, "o", "", "Output file path")
	flag.StringVar(&f.config, "c", "configs/config.yaml", "Config file path")
	flag.StringVar(&f.fofaCookie, "cookie-fofa", "", "FOFA cookie header")
	flag.StringVar(&f.hunterCookie, "cookie-hunter", "", "Hunter cookie header")
	flag.StringVar(&f.quakeCookie, "cookie-quake", "", "Quake cookie header")
	flag.StringVar(&f.zoomeyeCookie, "cookie-zoomeye", "", "ZoomEye cookie header")
	flag.BoolVar(&f.version, "version", false, "Print version")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "UniMap CLI %s\n\nUsage: %s -q '<uql>' [-e fofa,hunter] [-l 100] [-o results.csv]\n", appversion.Full(), os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	return f
}

func initCLIService(configPath string) (*config.Config, *config.Manager, *service.UnifiedService) {
	cfgManager := config.NewManager(configPath)
	if err := cfgManager.Load(); err != nil { logger.Warnf("Failed to load config from %s: %v. Using defaults.", configPath, err) }
	cfg := cfgManager.GetConfig()
	svc := service.NewUnifiedServiceWithConfig(cfg)
	return cfg, cfgManager, svc
}

func selectCLIEngines(cfg *config.Config, enginesFlag, configPath string) []string {
	var engines []string
	if enginesFlag != "" {
		for _, e := range strings.Split(enginesFlag, ",") {
			if e = strings.TrimSpace(e); e != "" { engines = append(engines, e) }
		}
	}
	if len(engines) == 0 { engines = getEnabledEngines(cfg) }
	if len(engines) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no engines configured/enabled. Set API keys in %s or use -e.\n", configPath)
		os.Exit(1)
	}
	return engines
}

func outputCLIResults(resp *service.QueryResponse, output string) {
	fmt.Printf("Found %d results.\n", resp.TotalCount)
	for engine, count := range resp.EngineStats { fmt.Printf("  %s: %d\n", engine, count) }
	for _, errMsg := range resp.Errors { fmt.Printf("  Error: %s\n", errMsg) }
	if output != "" {
		if err := saveResults(resp.Assets, output); err != nil { logger.Errorf("Failed to save results: %v", err) } else { fmt.Printf("Results saved to %s\n", output) }
	} else {
		for _, asset := range resp.Assets { fmt.Printf("%s\t%s:%d\t%s\n", asset.IP, asset.Host, asset.Port, asset.Title) }
	}
}

func applyCookiesFromFlags(cfg *config.Config, fofa, hunter, quake, zoomeye string) bool {
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
	if cfg.Engines.Binaryedge.Enabled {
		list = append(list, "binaryedge")
	}
	if cfg.Engines.Onyphe.Enabled {
		list = append(list, "onyphe")
	}
	if cfg.Engines.Greynoise.Enabled {
		list = append(list, "greynoise")
	}
	return list
}

func registerEngines(svc *service.UnifiedService, cfg *config.Config) {
	type engineReg struct {
		enabled bool
		reg     func()
	}
	regs := []engineReg{
		{cfg.Engines.Fofa.Enabled, func() { svc.RegisterAdapter(adapter.NewFofaAdapter(cfg.Engines.Fofa.APIBaseURL, cfg.Engines.Fofa.APIKey, cfg.Engines.Fofa.Email, cfg.Engines.Fofa.QPS, time.Duration(cfg.Engines.Fofa.Timeout)*time.Second)) }},
		{cfg.Engines.Hunter.Enabled, func() { svc.RegisterAdapter(adapter.NewHunterAdapter(cfg.Engines.Hunter.BaseURL, cfg.Engines.Hunter.APIKey, cfg.Engines.Hunter.QPS, time.Duration(cfg.Engines.Hunter.Timeout)*time.Second)) }},
		{cfg.Engines.Zoomeye.Enabled, func() { svc.RegisterAdapter(adapter.NewZoomEyeAdapter(cfg.Engines.Zoomeye.BaseURL, cfg.Engines.Zoomeye.APIKey, cfg.Engines.Zoomeye.QPS, time.Duration(cfg.Engines.Zoomeye.Timeout)*time.Second)) }},
		{cfg.Engines.Quake.Enabled, func() { svc.RegisterAdapter(adapter.NewQuakeAdapter(cfg.Engines.Quake.BaseURL, cfg.Engines.Quake.APIKey, cfg.Engines.Quake.QPS, time.Duration(cfg.Engines.Quake.Timeout)*time.Second)) }},
		{cfg.Engines.Shodan.Enabled, func() { svc.RegisterAdapter(adapter.NewShodanAdapter(cfg.Engines.Shodan.BaseURL, cfg.Engines.Shodan.APIKey, cfg.Engines.Shodan.QPS, time.Duration(cfg.Engines.Shodan.Timeout)*time.Second)) }},
		{cfg.Engines.Censys.Enabled, func() { svc.RegisterAdapter(adapter.NewCensysAdapter(cfg.Engines.Censys.BaseURL, cfg.Engines.Censys.APIID, cfg.Engines.Censys.APISecret, cfg.Engines.Censys.QPS, time.Duration(cfg.Engines.Censys.Timeout)*time.Second)) }},
		{cfg.Engines.Daydaymap.Enabled, func() { svc.RegisterAdapter(adapter.NewDayDayMapAdapter(cfg.Engines.Daydaymap.BaseURL, cfg.Engines.Daydaymap.APIKey, cfg.Engines.Daydaymap.QPS, time.Duration(cfg.Engines.Daydaymap.Timeout)*time.Second)) }},
		{cfg.Engines.Binaryedge.Enabled, func() { svc.RegisterAdapter(adapter.NewBinaryEdgeAdapter(cfg.Engines.Binaryedge.BaseURL, cfg.Engines.Binaryedge.APIKey, cfg.Engines.Binaryedge.QPS, time.Duration(cfg.Engines.Binaryedge.Timeout)*time.Second)) }},
		{cfg.Engines.Onyphe.Enabled, func() { svc.RegisterAdapter(adapter.NewOnypheAdapter(cfg.Engines.Onyphe.BaseURL, cfg.Engines.Onyphe.APIKey, cfg.Engines.Onyphe.QPS, time.Duration(cfg.Engines.Onyphe.Timeout)*time.Second)) }},
		{cfg.Engines.Greynoise.Enabled, func() { svc.RegisterAdapter(adapter.NewGreyNoiseAdapter(cfg.Engines.Greynoise.BaseURL, cfg.Engines.Greynoise.APIKey, cfg.Engines.Greynoise.QPS, time.Duration(cfg.Engines.Greynoise.Timeout)*time.Second)) }},
	}
	for _, r := range regs {
		if r.enabled {
			r.reg()
		}
	}
}

func saveResults(assets []model.UnifiedAsset, path string) error {
	// 根据文件扩展名选择导出格式
	lowerPath := strings.ToLower(path)

	switch {
	case strings.HasSuffix(lowerPath, ".json"):
		exp := exporter.NewJSONExporter()
		return exp.Export(assets, path)
	case strings.HasSuffix(lowerPath, ".xlsx") || strings.HasSuffix(lowerPath, ".xls"):
		exp := exporter.NewExcelExporter()
		return exp.Export(assets, path)
	default:
		// CSV default
		return saveResultsCSV(assets, path)
	}
}

// saveResultsCSV 保存为CSV格式
func saveResultsCSV(assets []model.UnifiedAsset, path string) error {
	// Use O_CREATE|O_EXCL to prevent overwriting existing files
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
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
