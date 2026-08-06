//go:build uql_check

// Command uql-check parses and translates the scheduled ynmobile query set
// against each target engine, printing the translated engine-native query so
// translation correctness can be verified before cloud deployment.
//
//	go run -tags uql_check ./tools/uql-check
package main

import (
	"fmt"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/core/unimap"
	"github.com/unimap/project/internal/model"
)

type task struct {
	name   string
	query  string
	engine string
}

func main() {
	tasks := []task{
		{"fofa_ynmobile_a", `(org="[REDACTED] Communications Group Co., Ltd." && region="[REDACTED]") || (asn="[REDACTED]" && region="[REDACTED]") || (cert.subject.cn="*.[REDACTED].cn" && region="[REDACTED]")`, "fofa"},
		{"fofa_ynmobile_b", `(icon_hash="[REDACTED]" || icon_hash="[REDACTED]" || icon_hash="[REDACTED]" || icon_hash="[REDACTED]" || icon_hash="[REDACTED]") || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || domain="[REDACTED]" || domain="[REDACTED]" || title="[REDACTED]"`, "fofa"},
		{"hunter_ynmobile_a", `(body="[REDACTED]" && region="[REDACTED]") || (domain="[REDACTED]" && region="[REDACTED]") || (title="[REDACTED]" && region="[REDACTED]")`, "hunter"},
		{"hunter_ynmobile_b", `body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]"`, "hunter"},
		{"quake_ynmobile_a", `(org="[REDACTED]" && region="[REDACTED]") || (asn="[REDACTED]" && region="[REDACTED]")`, "quake"},
		{"quake_ynmobile_b", `(favicon="[REDACTED]" || favicon="[REDACTED]" || favicon="[REDACTED]" || favicon="[REDACTED]" || favicon="[REDACTED]" || favicon="[REDACTED]" || favicon="[REDACTED]" || favicon="[REDACTED]" || favicon="[REDACTED]" || favicon="[REDACTED]" || favicon="[REDACTED]" || favicon="[REDACTED]" || favicon="[REDACTED]") || response="[REDACTED]" || response="[REDACTED]" || response="[REDACTED]" || response="[REDACTED]" || response="[REDACTED]" || response="[REDACTED]"`, "quake"},
		{"daydaymap_ynmobile_a", `(org="[REDACTED]" && region="[REDACTED]") || (asn="[REDACTED]" && region="[REDACTED]")`, "daydaymap"},
		{"daydaymap_ynmobile_b", `body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || title="[REDACTED]"`, "daydaymap"},
		{"ynmobile_weekly_snapshot", `domain="[REDACTED]" || domain="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || body="[REDACTED]" || title="[REDACTED]"`, "multi"},
	}

	parser := unimap.NewUQLParser()
	translators := map[string]func(*model.UQLAST) (string, error){
		"fofa":      adapter.NewFofaAdapter("https://fofa.info", "x", "x@x.com", 1, 0).Translate,
		"hunter":    adapter.NewHunterAdapter("https://hunter.qianxin.com", "x", 1, 0).Translate,
		"quake":     adapter.NewQuakeAdapter("https://quake.360.net", "x", 1, 0).Translate,
		"daydaymap": adapter.NewDayDayMapAdapter("https://www.daydaymap.com", "x", 1, 0).Translate,
	}

	for _, t := range tasks {
		ast, err := parser.Parse(t.query)
		if err != nil {
			fmt.Printf("❌ %s: PARSE ERROR: %v\n", t.name, err)
			continue
		}
		engines := []string{t.engine}
		if t.engine == "multi" {
			engines = []string{"fofa", "hunter", "quake", "daydaymap"}
		}
		for _, eng := range engines {
			tr, ok := translators[eng]
			if !ok {
				fmt.Printf("❌ %s: no translator for %s\n", t.name, eng)
				continue
			}
			out, err := tr(ast)
			if err != nil {
				fmt.Printf("❌ %s [%s]: TRANSLATE ERROR: %v\n", t.name, eng, err)
				continue
			}
			fmt.Printf("✓ %s [%s]: %s\n", t.name, eng, out)
		}
	}
}
