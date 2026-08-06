//go:build uql_check

// Command uql-check parses and translates the scheduled target query set
// against each target engine, printing the translated engine-native query so
// translation correctness can be verified before cloud deployment.
//
// The real target queries live in tools/cloud-config-gen/ynmobile_targets.json
// (gitignored — live target intel, never committed). Run from repo root or
// pass the path explicitly:
//
//	go run -tags uql_check ./tools/uql-check [targets.json]
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/core/unimap"
	"github.com/unimap/project/internal/model"
)

type task struct {
	Name   string `json:"name"`
	Query  string `json:"query"`
	Engine string `json:"engine"`
}

func main() {
	path := "tools/cloud-config-gen/ynmobile_targets.json"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load targets %s: %v\n", path, err)
		os.Exit(1)
	}
	var tasks []task
	if err := json.Unmarshal(data, &tasks); err != nil {
		fmt.Fprintf(os.Stderr, "parse targets: %v\n", err)
		os.Exit(1)
	}

	parser := unimap.NewUQLParser()
	translators := map[string]func(*model.UQLAST) (string, error){
		"fofa":      adapter.NewFofaAdapter("https://fofa.info", "x", "x@x.com", 1, 0).Translate,
		"hunter":    adapter.NewHunterAdapter("https://hunter.qianxin.com", "x", "", 1, 0).Translate,
		"quake":     adapter.NewQuakeAdapter("https://quake.360.net", "x", 1, 0).Translate,
		"daydaymap": adapter.NewDayDayMapAdapter("https://www.daydaymap.com", "x", 1, 0).Translate,
	}

	for _, t := range tasks {
		ast, err := parser.Parse(t.Query)
		if err != nil {
			fmt.Printf("❌ %s: PARSE ERROR: %v\n", t.Name, err)
			continue
		}
		engines := []string{t.Engine}
		if t.Engine == "multi" {
			engines = []string{"fofa", "hunter", "quake", "daydaymap"}
		}
		for _, eng := range engines {
			tr, ok := translators[eng]
			if !ok {
				fmt.Printf("❌ %s: no translator for %s\n", t.Name, eng)
				continue
			}
			out, err := tr(ast)
			if err != nil {
				fmt.Printf("❌ %s [%s]: TRANSLATE ERROR: %v\n", t.Name, eng, err)
				continue
			}
			fmt.Printf("✓ %s [%s]: %s\n", t.Name, eng, out)
		}
	}
}
