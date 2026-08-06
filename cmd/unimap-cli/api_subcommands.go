package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"strings"
	"time"

	"github.com/unimap/project/internal/model"
)

type apiQueryResponse struct {
	Query       string               `json:"query"`
	Assets      []model.UnifiedAsset `json:"assets"`
	TotalCount  int                  `json:"totalCount"`
	EngineStats map[string]int       `json:"engineStats"`
	Errors      []string             `json:"errors"`
}

type apiTamperResponse struct {
	Success bool                     `json:"success"`
	Mode    string                   `json:"mode"`
	Summary map[string]int           `json:"summary"`
	Results []map[string]interface{} `json:"results"`
}

type apiScreenshotBatchResponse struct {
	ID               string                   `json:"id,omitempty"`
	BatchID          string                   `json:"batch_id"`
	Total            int                      `json:"total"`
	Success          int                      `json:"success"`
	Failed           int                      `json:"failed"`
	Results          []map[string]interface{} `json:"results"`
	Status           string                   `json:"status,omitempty"`
	Error            string                   `json:"error,omitempty"`
	PersistenceError string                   `json:"persistence_error,omitempty"`
}

// cliSchedulerTask is the typed response for a scheduler task.
type cliSchedulerTask struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Enabled  bool                   `json:"enabled"`
	CronExpr string                 `json:"cron_expr"`
	Payload  map[string]interface{} `json:"payload,omitempty"`
}

// cliSchedulerHistoryEntry is the typed response for a scheduler history entry.
type cliSchedulerHistoryEntry struct {
	TaskID    string `json:"task_id"`
	TaskName  string `json:"task_name"`
	TaskType  string `json:"task_type"`
	Status    string `json:"status"`
	Result    string `json:"result"`
	Error     string `json:"error"`
	StartedAt string `json:"started_at"`
	Duration  int64  `json:"duration"`
}

var cliAdminToken string

func addAPIAuthFlags(fs *flag.FlagSet) (*string, *string) {
	token := fs.String("admin-token", "", "Admin token (prefer --admin-token-file or UNIMAP_ADMIN_TOKEN)")
	tokenFile := fs.String("admin-token-file", "", "Read admin token from file")
	return token, tokenFile
}

func configureAPIAuth(token, tokenFile string) error {
	resolved := strings.TrimSpace(os.Getenv("UNIMAP_ADMIN_TOKEN"))
	if resolved == "" {
		resolved = strings.TrimSpace(token)
	}
	if path := strings.TrimSpace(tokenFile); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read admin token file: %w", err)
		}
		resolved = strings.TrimSpace(string(data))
		if resolved == "" {
			return fmt.Errorf("admin token file is empty")
		}
	}
	cliAdminToken = resolved
	return nil
}

func runAPISubcommand(command string, args []string) bool {
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "query":
		runAPIQuery(args)
		return true
	case "tamper-check":
		runAPITamperCheck(args)
		return true
	case "screenshot-batch":
		runAPIScreenshotBatch(args)
		return true
	case "scheduler":
		runAPIScheduler(args)
		return true
	default:
		return false
	}
}

func runAPIQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	query := fs.String("q", "", "UQL query string")
	engines := fs.String("e", "", "Comma-separated engines, e.g. fofa,hunter")
	limit := fs.Int("l", 100, "Result limit / page size")
	page := fs.Int("page", 1, "Page number")
	apiBase := fs.String("api-base", "http://127.0.0.1:8448", "Web API base URL")
	timeoutSec := fs.Int("timeout", 60, "HTTP timeout in seconds")
	output := fs.String("o", "", "Output file path (csv/json/xlsx)")
	format := fs.String("format", "table", "Output format: table or json")
	adminToken, adminTokenFile := addAPIAuthFlags(fs)
	_ = fs.Parse(args)

	if *apiBase == "http://127.0.0.1:8448" {
		*apiBase = envOrDefault("UNIMAP_API_BASE", *apiBase)
	}

	if err := configureAPIAuth(*adminToken, *adminTokenFile); err != nil {
		if isJSONFormat(*format) {
			printJSONError("query", "AUTH_CONFIG", err.Error(), ExitAuthError)
		}
		progress("API authentication configuration failed: %v\n", err)
		os.Exit(ExitAuthError)
	}

	if strings.TrimSpace(*query) == "" {
		if isJSONFormat(*format) {
			printJSONError("query", "USAGE_ERROR", "-q is required", ExitUsageError)
		}
		progress("Error: -q is required\n")
		os.Exit(ExitUsageError)
	}

	values := neturl.Values{}
	values.Set("query", *query)
	if strings.TrimSpace(*engines) != "" {
		values.Set("engines", *engines)
	}
	if *limit > 0 {
		values.Set("page_size", fmt.Sprintf("%d", *limit))
	}
	if *page > 0 {
		values.Set("page", fmt.Sprintf("%d", *page))
	}

	var resp apiQueryResponse
	if err := doFormRequest(*apiBase, "/api/v1/query", *timeoutSec, values, &resp); err != nil {
		code, exitCode := classifyError(err)
		if isJSONFormat(*format) {
			printJSONError("query", code, err.Error(), exitCode)
		}
		progress("API query failed: %v\n", err)
		os.Exit(exitCode)
	}

	if isJSONFormat(*format) {
		data := queryOutputData{
			Query:       *query,
			Assets:      resp.Assets,
			Total:       resp.TotalCount,
			Page:        *page,
			PageSize:    *limit,
			HasMore:     resp.TotalCount > (*page)*(*limit),
			EngineStats: resp.EngineStats,
			Errors:      resp.Errors,
		}
		printJSON("query", data, ExitOK)
		return
	}

	progress("Found %d results.\n", resp.TotalCount)
	for engine, count := range resp.EngineStats {
		progress("  %s: %d\n", engine, count)
	}
	for _, errMsg := range resp.Errors {
		progress("  Error: %s\n", errMsg)
	}

	if strings.TrimSpace(*output) != "" {
		if err := saveResults(resp.Assets, *output); err != nil {
			progress("Failed to save results: %v\n", err)
			os.Exit(ExitQueryError)
		}
		progress("Results saved to %s\n", *output)
		return
	}

	for _, asset := range resp.Assets {
		fmt.Printf("%s\t%s:%d\t%s\n", asset.IP, asset.Host, asset.Port, asset.Title)
	}
}

func runAPITamperCheck(args []string) {
	fs := flag.NewFlagSet("tamper-check", flag.ExitOnError)
	urlsText := fs.String("urls", "", "Comma-separated URLs")
	concurrency := fs.Int("concurrency", 5, "Concurrency")
	mode := fs.String("mode", "relaxed", "Tamper mode: relaxed|strict")
	apiBase := fs.String("api-base", "http://127.0.0.1:8448", "Web API base URL")
	timeoutSec := fs.Int("timeout", 120, "HTTP timeout in seconds")
	output := fs.String("o", "", "Output JSON file path")
	format := fs.String("format", "table", "Output format: table or json")
	adminToken, adminTokenFile := addAPIAuthFlags(fs)
	_ = fs.Parse(args)

	if *apiBase == "http://127.0.0.1:8448" {
		*apiBase = envOrDefault("UNIMAP_API_BASE", *apiBase)
	}

	if err := configureAPIAuth(*adminToken, *adminTokenFile); err != nil {
		if isJSONFormat(*format) {
			printJSONError("tamper-check", "AUTH_CONFIG", err.Error(), ExitAuthError)
		}
		progress("API authentication configuration failed: %v\n", err)
		os.Exit(ExitAuthError)
	}

	urls := splitCSVText(*urlsText)
	if len(urls) == 0 {
		if isJSONFormat(*format) {
			printJSONError("tamper-check", "USAGE_ERROR", "--urls is required", ExitUsageError)
		}
		progress("Error: --urls is required\n")
		os.Exit(ExitUsageError)
	}

	type cliTamperRequest struct {
		URLs        []string `json:"urls"`
		Concurrency int      `json:"concurrency"`
		Mode        string   `json:"mode"`
	}
	payload := cliTamperRequest{
		URLs: urls, Concurrency: *concurrency,
		Mode: strings.ToLower(strings.TrimSpace(*mode)),
	}
	var resp apiTamperResponse
	if err := doJSONRequest(*apiBase, "/api/v1/tamper/check", *timeoutSec, payload, &resp); err != nil {
		code, exitCode := classifyError(err)
		if isJSONFormat(*format) {
			printJSONError("tamper-check", code, err.Error(), exitCode)
		}
		progress("API tamper-check failed: %v\n", err)
		os.Exit(exitCode)
	}

	if isJSONFormat(*format) {
		printJSON("tamper-check", resp, ExitOK)
		return
	}

	progress("Tamper check completed (mode=%s).\n", resp.Mode)
	for k, v := range resp.Summary {
		progress("  %s: %d\n", k, v)
	}
	progress("  results: %d\n", len(resp.Results))

	if strings.TrimSpace(*output) != "" {
		if err := writeJSONFile(*output, resp); err != nil {
			progress("Failed to save output: %v\n", err)
			os.Exit(ExitQueryError)
		}
		progress("Output saved to %s\n", *output)
	}
}

func runAPIScreenshotBatch(args []string) {
	fs := flag.NewFlagSet("screenshot-batch", flag.ExitOnError)
	urlsText := fs.String("urls", "", "Comma-separated URLs")
	batchID := fs.String("batch-id", "", "Batch id (optional)")
	concurrency := fs.Int("concurrency", 5, "Concurrency")
	apiBase := fs.String("api-base", "http://127.0.0.1:8448", "Web API base URL")
	timeoutSec := fs.Int("timeout", 300, "HTTP timeout in seconds")
	output := fs.String("o", "", "Output JSON file path")
	format := fs.String("format", "table", "Output format: table or json")
	adminToken, adminTokenFile := addAPIAuthFlags(fs)
	_ = fs.Parse(args)

	if *apiBase == "http://127.0.0.1:8448" {
		*apiBase = envOrDefault("UNIMAP_API_BASE", *apiBase)
	}

	if err := configureAPIAuth(*adminToken, *adminTokenFile); err != nil {
		if isJSONFormat(*format) {
			printJSONError("screenshot-batch", "AUTH_CONFIG", err.Error(), ExitAuthError)
		}
		progress("API authentication configuration failed: %v\n", err)
		os.Exit(ExitAuthError)
	}

	urls := splitCSVText(*urlsText)
	if len(urls) == 0 {
		if isJSONFormat(*format) {
			printJSONError("screenshot-batch", "USAGE_ERROR", "--urls is required", ExitUsageError)
		}
		progress("Error: --urls is required\n")
		os.Exit(ExitUsageError)
	}

	type cliScreenshotRequest struct {
		URLs        []string `json:"urls"`
		BatchID     string   `json:"batch_id"`
		Concurrency int      `json:"concurrency"`
	}
	payload := cliScreenshotRequest{
		URLs: urls, BatchID: strings.TrimSpace(*batchID), Concurrency: *concurrency,
	}
	var start struct {
		JobID  string `json:"job_id"`
		Total  int    `json:"total"`
		Status string `json:"status"`
	}
	if err := doJSONRequest(*apiBase, "/api/v1/screenshot/batch-urls", *timeoutSec, payload, &start); err != nil {
		code, exitCode := classifyError(err)
		if isJSONFormat(*format) {
			printJSONError("screenshot-batch", code, err.Error(), exitCode)
		}
		progress("API screenshot-batch failed: %v\n", err)
		os.Exit(exitCode)
	}
	resp, err := waitForScreenshotBatch(*apiBase, start.JobID, *timeoutSec)
	if err != nil {
		code, exitCode := classifyError(err)
		if isJSONFormat(*format) {
			printJSONError("screenshot-batch", code, fmt.Sprintf("screenshot job %s did not complete: %v", start.JobID, err), exitCode)
		}
		progress("Screenshot job %s did not complete: %v\n", start.JobID, err)
		os.Exit(exitCode)
	}
	resp.BatchID = start.JobID

	if isJSONFormat(*format) {
		printJSON("screenshot-batch", resp, ExitOK)
		return
	}

	if resp.PersistenceError != "" {
		progress("Warning: screenshot results completed but persistence was degraded: %s\n", resp.PersistenceError)
	}

	progress("Screenshot batch completed: batch_id=%s total=%d success=%d failed=%d\n", resp.BatchID, resp.Total, resp.Success, resp.Failed)

	if strings.TrimSpace(*output) != "" {
		if err := writeJSONFile(*output, resp); err != nil {
			progress("Failed to save output: %v\n", err)
			os.Exit(ExitQueryError)
		}
		progress("Output saved to %s\n", *output)
	}
}

func waitForScreenshotBatch(base, jobID string, timeoutSec int) (apiScreenshotBatchResponse, error) {
	deadline := time.Now().Add(time.Duration(maxInt(timeoutSec, 1)) * time.Second)
	for {
		var job apiScreenshotBatchResponse
		if err := doGETRequest(base, "/api/v1/screenshot/batch/progress?job_id="+neturl.QueryEscape(jobID), min(maxInt(timeoutSec, 1), 30), &job); err != nil {
			return job, err
		}
		switch job.Status {
		case "completed":
			return job, nil
		case "failed":
			return job, fmt.Errorf("%s", job.Error)
		}
		if time.Now().After(deadline) {
			return job, fmt.Errorf("timed out; resume with job_id=%s", jobID)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func doFormRequest(apiBase, path string, timeoutSec int, values neturl.Values, out interface{}) error {
	base := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if base == "" {
		return fmt.Errorf("api base is empty")
	}
	client, err := newAPIHTTPClient(base, timeoutSec)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, base+path, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	applyCLIAuth(req)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response failed: %w", err)
	}
	return nil
}

func doGETRequest(apiBase, path string, timeoutSec int, out interface{}) error {
	base := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if base == "" {
		return fmt.Errorf("api base is empty")
	}
	req, err := http.NewRequest(http.MethodGet, base+path, nil)
	if err != nil {
		return err
	}
	applyCLIAuth(req)
	client, err := newAPIHTTPClient(base, timeoutSec)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response failed: %w", err)
	}
	return nil
}

func applyCLIAuth(req *http.Request) {
	token := strings.TrimSpace(cliAdminToken)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("UNIMAP_ADMIN_TOKEN"))
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func newAPIHTTPClient(base string, timeoutSec int) (*http.Client, error) {
	baseURL, err := neturl.Parse(base)
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("invalid API base URL")
	}
	return &http.Client{
		Timeout: time.Duration(maxInt(timeoutSec, 1)) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !strings.EqualFold(req.URL.Scheme, baseURL.Scheme) || !strings.EqualFold(req.URL.Host, baseURL.Host) {
				return fmt.Errorf("refusing API redirect to a different origin")
			}
			return nil
		},
	}, nil
}

func doJSONRequest(apiBase, path string, timeoutSec int, payload interface{}, out interface{}) error {
	base := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if base == "" {
		return fmt.Errorf("api base is empty")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client, err := newAPIHTTPClient(base, timeoutSec)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	applyCLIAuth(req)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode response failed: %w", err)
	}
	return nil
}

func writeJSONFile(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("file %q already exists, refusing to overwrite: %w", path, err)
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func splitCSVText(raw string) []string {
	parts := strings.Split(strings.ReplaceAll(raw, "\n", ","), ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// runAPIScheduler handles CLI subcommands for the scheduler.
// Usage:
//
//	unimap-cli scheduler list
//	unimap-cli scheduler run --id <task-id>
//	unimap-cli scheduler create --name <name> --type <type> --cron <expr> --payload <json>
//	unimap-cli scheduler enable --id <task-id>
//	unimap-cli scheduler disable --id <task-id>
//	unimap-cli scheduler delete --id <task-id>
//	unimap-cli scheduler history [--task-id <id>]
func runAPIScheduler(args []string) {
	fs := flag.NewFlagSet("scheduler", flag.ExitOnError)
	apiBase := fs.String("api-base", "http://127.0.0.1:8448", "Web API base URL")
	timeoutSec := fs.Int("timeout", 30, "HTTP timeout in seconds")
	format := fs.String("format", "table", "Output format: table or json")
	adminToken, adminTokenFile := addAPIAuthFlags(fs)
	_ = fs.Parse(args)

	if *apiBase == "http://127.0.0.1:8448" {
		*apiBase = envOrDefault("UNIMAP_API_BASE", *apiBase)
	}

	if err := configureAPIAuth(*adminToken, *adminTokenFile); err != nil {
		if isJSONFormat(*format) {
			printJSONError("scheduler", "AUTH_CONFIG", err.Error(), ExitAuthError)
		}
		progress("API authentication configuration failed: %v\n", err)
		os.Exit(ExitAuthError)
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		if isJSONFormat(*format) {
			printJSONError("scheduler", "USAGE_ERROR", "subcommand required: list|run|create|enable|disable|delete|history", ExitUsageError)
		}
		progress("Usage: unimap-cli scheduler [global flags] <list|run|create|enable|disable|delete|history> [flags]\n")
		os.Exit(ExitUsageError)
	}
	subcmd := remaining[0]
	subArgs := remaining[1:]
	if subcmd == "" {
		if isJSONFormat(*format) {
			printJSONError("scheduler", "USAGE_ERROR", "subcommand required: list|run|create|enable|disable|delete|history", ExitUsageError)
		}
		progress("Usage: unimap-cli scheduler <list|run|create|enable|disable|delete|history> [flags]\n")
		os.Exit(ExitUsageError)
	}

	base := strings.TrimRight(*apiBase, "/")
	prefix := "/api/v1/scheduler"

	switch strings.ToLower(subcmd) {
	case "list":
		schedulerList(base, prefix, *timeoutSec, *format)
	case "run":
		schedulerRun(subArgs, base, prefix, *timeoutSec, *format)
	case "create":
		schedulerCreate(subArgs, base, prefix, *timeoutSec, *format)
	case "enable", "disable":
		schedulerToggle(subArgs, subcmd, base, prefix, *timeoutSec, *format)
	case "delete":
		schedulerDelete(subArgs, base, prefix, *timeoutSec, *format)
	case "history":
		schedulerHistory(subArgs, base, prefix, *timeoutSec, *format)
	default:
		if isJSONFormat(*format) {
			printJSONError("scheduler", "USAGE_ERROR", "unknown scheduler command: "+subcmd, ExitUsageError)
		}
		progress("Unknown scheduler command: %s\n", subcmd)
		os.Exit(ExitUsageError)
	}
}

func schedulerList(base, prefix string, timeoutSec int, format string) {
	var tasks []cliSchedulerTask
	if err := doGETRequest(base, prefix+"/tasks", timeoutSec, &tasks); err != nil {
		code, exitCode := classifyError(err)
		if isJSONFormat(format) {
			printJSONError("scheduler", code, err.Error(), exitCode)
		}
		progress("Error: %v\n", err)
		os.Exit(exitCode)
	}
	if isJSONFormat(format) {
		printJSON("scheduler", map[string]interface{}{"tasks": tasks, "count": len(tasks)}, ExitOK)
		return
	}
	progress("Scheduled tasks (%d):\n", len(tasks))
	for _, t := range tasks {
		fmt.Printf("  %-8s %-30s %-20s enabled=%v  cron=%s\n",
			t.ID, t.Name, t.Type, t.Enabled, t.CronExpr)
	}
}

func schedulerRun(args []string, base, prefix string, timeoutSec int, format string) {
	fs := flag.NewFlagSet("scheduler run", flag.ExitOnError)
	taskID := fs.String("id", "", "Task ID")
	_ = fs.Parse(args)
	if *taskID == "" {
		if isJSONFormat(format) {
			printJSONError("scheduler", "USAGE_ERROR", "-id is required for run", ExitUsageError)
		}
		progress("Error: -id is required for run\n")
		os.Exit(ExitUsageError)
	}
	payload := map[string]string{"id": *taskID}
	var resp struct {
		Success bool `json:"success"`
	}
	if err := doJSONRequest(base, prefix+"/tasks/run", timeoutSec, payload, &resp); err != nil {
		code, exitCode := classifyError(err)
		if isJSONFormat(format) {
			printJSONError("scheduler", code, err.Error(), exitCode)
		}
		progress("Error: %v\n", err)
		os.Exit(exitCode)
	}
	if isJSONFormat(format) {
		printJSON("scheduler", map[string]interface{}{"task_id": *taskID, "triggered": resp.Success}, ExitOK)
		return
	}
	progress("Task %s triggered successfully\n", *taskID)
}

func schedulerCreate(args []string, base, prefix string, timeoutSec int, format string) {
	fs := flag.NewFlagSet("scheduler create", flag.ExitOnError)
	name := fs.String("name", "", "Task name")
	taskType := fs.String("type", "", "Task type (e.g. icp_query, query)")
	cron := fs.String("cron", "", "Cron expression")
	payloadStr := fs.String("payload", "{}", "JSON payload")
	enabled := fs.Bool("enabled", true, "Enable task immediately")
	_ = fs.Parse(args)

	if *name == "" || *taskType == "" || *cron == "" {
		if isJSONFormat(format) {
			printJSONError("scheduler", "USAGE_ERROR", "-name, -type, and -cron are required for create", ExitUsageError)
		}
		progress("Error: -name, -type, and -cron are required for create\n")
		os.Exit(ExitUsageError)
	}

	var p map[string]interface{}
	if err := json.Unmarshal([]byte(*payloadStr), &p); err != nil {
		if isJSONFormat(format) {
			printJSONError("scheduler", "USAGE_ERROR", "invalid JSON payload: "+err.Error(), ExitUsageError)
		}
		progress("Invalid JSON payload: %v\n", err)
		os.Exit(ExitUsageError)
	}

	task := cliSchedulerTask{
		Name:     *name,
		Type:     *taskType,
		Enabled:  *enabled,
		CronExpr: *cron,
		Payload:  p,
	}
	var resp struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}
	if err := doJSONRequest(base, prefix+"/tasks/create", timeoutSec, task, &resp); err != nil {
		code, exitCode := classifyError(err)
		if isJSONFormat(format) {
			printJSONError("scheduler", code, err.Error(), exitCode)
		}
		progress("Error: %v\n", err)
		os.Exit(exitCode)
	}
	if isJSONFormat(format) {
		printJSON("scheduler", map[string]interface{}{"id": resp.ID, "name": task.Name, "type": task.Type, "created": true}, ExitOK)
		return
	}
	progress("Task created: id=%s name=%s type=%s\n", resp.ID, task.Name, task.Type)
}

func schedulerToggle(args []string, subcmd, base, prefix string, timeoutSec int, format string) {
	fs := flag.NewFlagSet("scheduler "+subcmd, flag.ExitOnError)
	taskID := fs.String("id", "", "Task ID")
	_ = fs.Parse(args)
	if *taskID == "" {
		if isJSONFormat(format) {
			printJSONError("scheduler", "USAGE_ERROR", "-id is required", ExitUsageError)
		}
		progress("Error: -id is required\n")
		os.Exit(ExitUsageError)
	}
	action := strings.ToLower(subcmd)
	payload := map[string]string{"id": *taskID}
	var resp struct {
		Success bool `json:"success"`
	}
	if err := doJSONRequest(base, prefix+"/tasks/"+action, timeoutSec, payload, &resp); err != nil {
		code, exitCode := classifyError(err)
		if isJSONFormat(format) {
			printJSONError("scheduler", code, err.Error(), exitCode)
		}
		progress("Error: %v\n", err)
		os.Exit(exitCode)
	}
	if isJSONFormat(format) {
		printJSON("scheduler", map[string]interface{}{"task_id": *taskID, "action": action, "success": resp.Success}, ExitOK)
		return
	}
	progress("Task %s %sd\n", *taskID, action)
}

func schedulerDelete(args []string, base, prefix string, timeoutSec int, format string) {
	fs := flag.NewFlagSet("scheduler delete", flag.ExitOnError)
	taskID := fs.String("id", "", "Task ID")
	_ = fs.Parse(args)
	if *taskID == "" {
		if isJSONFormat(format) {
			printJSONError("scheduler", "USAGE_ERROR", "-id is required", ExitUsageError)
		}
		progress("Error: -id is required\n")
		os.Exit(ExitUsageError)
	}
	payload := map[string]string{"id": *taskID}
	var resp struct {
		Success bool `json:"success"`
	}
	if err := doJSONRequest(base, prefix+"/tasks/delete", timeoutSec, payload, &resp); err != nil {
		code, exitCode := classifyError(err)
		if isJSONFormat(format) {
			printJSONError("scheduler", code, err.Error(), exitCode)
		}
		progress("Error: %v\n", err)
		os.Exit(exitCode)
	}
	if isJSONFormat(format) {
		printJSON("scheduler", map[string]interface{}{"task_id": *taskID, "deleted": resp.Success}, ExitOK)
		return
	}
	progress("Task %s deleted\n", *taskID)
}

func schedulerHistory(args []string, base, prefix string, timeoutSec int, format string) {
	fs := flag.NewFlagSet("scheduler history", flag.ExitOnError)
	taskID := fs.String("task-id", "", "Filter by task ID")
	limit := fs.Int("limit", 20, "Max history entries")
	_ = fs.Parse(args)

	url := prefix + "/history?"
	if *taskID != "" {
		url += "task_id=" + neturl.QueryEscape(*taskID) + "&"
	}
	url += fmt.Sprintf("limit=%d", *limit)

	var history []cliSchedulerHistoryEntry
	if err := doGETRequest(base, url, timeoutSec, &history); err != nil {
		code, exitCode := classifyError(err)
		if isJSONFormat(format) {
			printJSONError("scheduler", code, err.Error(), exitCode)
		}
		progress("Error: %v\n", err)
		os.Exit(exitCode)
	}
	if isJSONFormat(format) {
		printJSON("scheduler", map[string]interface{}{"history": history, "count": len(history)}, ExitOK)
		return
	}
	progress("Execution history (%d records):\n", len(history))
	for _, h := range history {
		status := h.Status
		result := h.Result
		if h.Error != "" {
			result = h.Error
		}
		fmt.Printf("  %-20s %-10s %-15s %s\n", h.StartedAt, status, h.TaskType, result)
	}
}
