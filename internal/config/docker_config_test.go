package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDockerBaselineConfigLoads(t *testing.T) {
	mgr := NewManager(filepath.Join("..", "..", "configs", "config.docker.yaml"))
	if err := mgr.Load(); err != nil {
		t.Fatalf("load Docker baseline config: %v", err)
	}
	cfg := mgr.GetConfig()
	if cfg.Web.BindAddress != "127.0.0.1" || !cfg.Screenshot.Enabled || cfg.Screenshot.Mode != "cdp" {
		t.Fatalf("unexpected Docker baseline: bind=%q screenshot=%v mode=%q", cfg.Web.BindAddress, cfg.Screenshot.Enabled, cfg.Screenshot.Mode)
	}
}

func TestDeploymentYAMLFilesAreSyntacticallyValid(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "docker-compose.yml"),
		filepath.Join("..", "..", ".github", "workflows", "ci.yml"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(data, &doc); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
	}
}

func TestDockerDeploymentOverridesBindAndHashesBootstrapPassword(t *testing.T) {
	t.Setenv("UNIMAP_CONTAINER_BIND_ADDRESS", "0.0.0.0")
	t.Setenv("UNIMAP_BOOTSTRAP_PASSWORD", "a-long-random-deployment-password")
	mgr := NewManager(filepath.Join("..", "..", "configs", "config.docker.yaml"))
	if err := mgr.Load(); err != nil {
		t.Fatalf("load Docker deployment config: %v", err)
	}
	cfg := mgr.GetConfig()
	if cfg.Web.BindAddress != "0.0.0.0" || cfg.Web.Auth.PasswordHash == "" {
		t.Fatalf("container overrides were not applied")
	}
	if cfg.Web.Auth.PasswordHash == "a-long-random-deployment-password" {
		t.Fatal("bootstrap password was retained as plaintext")
	}
}
