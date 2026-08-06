package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/unimap/project/internal/model"
)

// Semantic exit codes for agent consumption.
const (
	ExitOK          = 0
	ExitQueryError  = 1
	ExitAuthError   = 2
	ExitNoEngines   = 3
	ExitUsageError  = 4
	ExitServerError = 5
	ExitTimeout     = 6
)

// CLIEnvelope is the unified JSON output structure for all subcommands.
type CLIEnvelope struct {
	OK       bool        `json:"ok"`
	Command  string      `json:"command"`
	Data     interface{} `json:"data,omitempty"`
	Error    *CLIError   `json:"error,omitempty"`
	ExitCode int         `json:"exit_code"`
}

// CLIError is a structured error inside the envelope.
type CLIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Engine  string `json:"engine,omitempty"`
}

// printJSON outputs a success envelope to stdout and exits.
func printJSON(command string, data interface{}, exitCode int) {
	env := CLIEnvelope{
		OK:       exitCode == ExitOK,
		Command:  command,
		Data:     data,
		ExitCode: exitCode,
	}
	b, _ := json.MarshalIndent(env, "", "  ")
	fmt.Println(string(b))
	os.Exit(exitCode)
}

// printJSONError outputs an error envelope to stdout and exits.
func printJSONError(command, code, message string, exitCode int) {
	env := CLIEnvelope{
		OK:       false,
		Command:  command,
		Error:    &CLIError{Code: code, Message: message},
		ExitCode: exitCode,
	}
	b, _ := json.MarshalIndent(env, "", "  ")
	fmt.Println(string(b))
	os.Exit(exitCode)
}

// progress prints informational messages to stderr (never stdout in JSON mode).
func progress(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
}

// isJSONFormat returns true when the user requested JSON output.
func isJSONFormat(format string) bool {
	return strings.EqualFold(strings.TrimSpace(format), "json")
}

// classifyError maps an error to a semantic exit code and error code string.
func classifyError(err error) (string, int) {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "unauthorized") || strings.Contains(msg, "401") ||
		strings.Contains(msg, "auth") || strings.Contains(msg, "api key") ||
		strings.Contains(msg, "forbidden") || strings.Contains(msg, "403"):
		return "AUTH_FAILED", ExitAuthError
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
		return "TIMEOUT", ExitTimeout
	case strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "dial tcp") || strings.Contains(msg, "server"):
		return "SERVER_UNREACHABLE", ExitServerError
	default:
		return "QUERY_FAILED", ExitQueryError
	}
}

// --- Query output data structures ---

// queryOutputData is the JSON data payload for query results.
type queryOutputData struct {
	Query       string               `json:"query"`
	Assets      []model.UnifiedAsset `json:"assets"`
	Total       int                  `json:"total"`
	Page        int                  `json:"page"`
	PageSize    int                  `json:"page_size"`
	HasMore     bool                 `json:"has_more"`
	EngineStats map[string]int       `json:"engine_stats"`
	Errors      []string             `json:"errors,omitempty"`
}

// engineInfoEntry is one engine in the engines subcommand output.
type engineInfoEntry struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	HasAPIKey bool   `json:"has_api_key"`
}

// --- Field selection ---

// assetFieldGetters maps field names to extractors for --fields support.
var assetFieldGetters = map[string]func(model.UnifiedAsset) string{
	"ip":          func(a model.UnifiedAsset) string { return a.IP },
	"port":        func(a model.UnifiedAsset) string { return strconv.Itoa(a.Port) },
	"protocol":    func(a model.UnifiedAsset) string { return a.Protocol },
	"host":        func(a model.UnifiedAsset) string { return a.Host },
	"domain":      func(a model.UnifiedAsset) string { return a.Host },
	"title":       func(a model.UnifiedAsset) string { return a.Title },
	"url":         func(a model.UnifiedAsset) string { return a.URL },
	"server":      func(a model.UnifiedAsset) string { return a.Server },
	"status_code": func(a model.UnifiedAsset) string { return strconv.Itoa(a.StatusCode) },
	"country":     func(a model.UnifiedAsset) string { return a.CountryCode },
	"region":      func(a model.UnifiedAsset) string { return a.Region },
	"city":        func(a model.UnifiedAsset) string { return a.City },
	"asn":         func(a model.UnifiedAsset) string { return a.ASN },
	"org":         func(a model.UnifiedAsset) string { return a.Org },
	"isp":         func(a model.UnifiedAsset) string { return a.ISP },
	"source":      func(a model.UnifiedAsset) string { return a.Source },
	"banner":      func(a model.UnifiedAsset) string { return a.BodySnippet },
}

// filterAssetFields returns a map with only the requested fields for each asset.
func filterAssetFields(assets []model.UnifiedAsset, fields []string) []map[string]string {
	result := make([]map[string]string, 0, len(assets))
	for _, a := range assets {
		row := make(map[string]string, len(fields))
		for _, f := range fields {
			f = strings.TrimSpace(strings.ToLower(f))
			if getter, ok := assetFieldGetters[f]; ok {
				row[f] = getter(a)
			}
		}
		result = append(result, row)
	}
	return result
}

// parseFields splits a comma-separated field list.
func parseFields(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	fields := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			fields = append(fields, p)
		}
	}
	return fields
}

// envOrDefault reads an environment variable, falling back to a default.
func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// envIntOrDefault reads an int environment variable, falling back to a default.
func envIntOrDefault(key string, fallback int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
