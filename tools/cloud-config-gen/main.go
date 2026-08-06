//go:build cloud_config_gen

// Command cloud-config-gen merges the secrets-bearing engine/notification
// sections from a LOCAL config.yaml into a production container config,
// keeping all container-relative paths (screenshots, logs, data). The emitted
// file contains secrets and must be transferred to the server out-of-band of
// this conversation and deleted locally afterwards.
//
//	go run -tags cloud_config_gen ./tools/cloud-config-gen \
//	  -src configs/config.yaml -out data/prod-config.yaml
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	src := flag.String("src", "configs/config.yaml", "local config file carrying the live engine keys / notify channels")
	out := flag.String("out", "", "output path (prints to stdout when empty)")
	flag.Parse()

	data, err := os.ReadFile(*src)
	if err != nil {
		fatalf("read source config: %v", err)
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		fatalf("parse source config: %v", err)
	}

	engines, _ := raw["engines"].(map[string]any)
	if engines == nil {
		fatalf("source config has no 'engines' section")
	}
	notifications, _ := raw["notifications"].(map[string]any)
	if notifications == nil {
		fatalf("source config has no 'notifications' section")
	}

	// Sanity: notify webhook URLs must be plaintext http(s) URLs for the new
	// deployment pepper to be irrelevant. If a value is not a URL it is likely
	// an encrypted secret tied to the LOCAL pepper and would not resolve on the
	// server without the same pepper.
	if err := checkPlaintextWebhooks(notifications); err != nil {
		warnf("%v", err)
	}

	prod := buildProd(engines, notifications)

	if *out == "" {
		fmt.Print(prod)
		return
	}
	if err := os.WriteFile(*out, []byte(prod), 0o600); err != nil {
		fatalf("write output: %v", err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d bytes)\n", *out, len(prod))
}

// buildProd assembles the production config: container paths from the tracked
// config.prod.yaml baseline, live engines + notify channels lifted verbatim.
func buildProd(engines, notifications map[string]any) string {
	prod := map[string]any{
		"web": map[string]any{
			"bind_address": "127.0.0.1",
			"port":         8448,
			"auth": map[string]any{
				"enabled":       true,
				"admin_token":   "${UNIMAP_ADMIN_TOKEN}",
				"username":      "${UNIMAP_ADMIN_USERNAME}",
				"password_hash": "",
			},
		},
		"distributed": map[string]any{
			"enabled":         false,
			"admin_token":     "${UNIMAP_DISTRIBUTED_ADMIN_TOKEN}",
			"node_auth_tokens": map[string]any{},
			"scheduler":       map[string]any{"strategy": "health_load"},
		},
		"engines":       engines,
		"notifications": notifications,
		"screenshot": map[string]any{
			"enabled":              false,
			"mode":                 "cdp",
			"priority":             "cdp",
			"fallback":             false,
			"base_dir":             "/app/screenshots",
			"chrome_path":          "/usr/bin/chromium",
			"chrome_user_data_dir": "/app/chrome-profile",
			"headless":             true,
			"no_sandbox":           true,
			"max_sessions":         1,
			"timeout":              30,
			"window_width":         1365,
			"window_height":        768,
			"wait_time":            500,
		},
		"log": map[string]any{
			"level":    "info",
			"encoding": "console",
			"file":     "/app/logs/unimap.log",
		},
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(prod); err != nil {
		fatalf("encode production config: %v", err)
	}
	if err := enc.Close(); err != nil {
		fatalf("close encoder: %v", err)
	}
	return buf.String()
}

// checkPlaintextWebhooks warns when a notify channel webhook value is not an
// http(s) URL (i.e. it is likely pepper-encrypted and needs the local pepper).
func checkPlaintextWebhooks(notifications map[string]any) error {
	channels, _ := notifications["channels"].([]any)
	for i, ch := range channels {
		c, _ := ch.(map[string]any)
		if c == nil {
			continue
		}
		typ, _ := c["type"].(string)
		webhook, _ := c["webhook_url"].(string)
		if typ == "wecom" && webhook != "" && !strings.HasPrefix(webhook, "http://") && !strings.HasPrefix(webhook, "https://") {
			return fmt.Errorf("channel[%d] (%s): webhook_url is not an http(s) URL — likely encrypted with the LOCAL notify pepper; it will not resolve on the server unless the same UNIMAP_NOTIFY_PEPPER is set", i, typ)
		}
	}
	return nil
}

func warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[warn] "+format+"\n", args...)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[fatal] "+format+"\n", args...)
	os.Exit(1)
}
