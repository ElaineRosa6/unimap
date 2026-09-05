package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCIReleaseGates(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	needs := yamlScalarValues(yamlNodeAt(t, &doc, "jobs", "docker", "needs"))
	for _, required := range []string{"test", "lint", "security", "headless-browser", "extension-scripts"} {
		found := false
		for _, dep := range needs {
			if dep == required {
				found = true
			}
		}
		if !found {
			t.Errorf("Docker publishing does not depend on %s", required)
		}
	}
	steps := yamlNodeAt(t, &doc, "jobs", "test", "steps")
	foundUpload := false
	for _, step := range steps.Content {
		var entry struct {
			Uses string            `yaml:"uses"`
			With map[string]string `yaml:"with"`
		}
		if err := step.Decode(&entry); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(entry.Uses, "actions/upload-artifact@") {
			foundUpload = true
			if !strings.Contains(entry.With["name"], "${{ matrix.os }}") {
				t.Errorf("coverage artifact %q collides across OS matrix jobs", entry.With["name"])
			}
		}
	}
	if !foundUpload {
		t.Fatal("coverage upload step missing")
	}
}
