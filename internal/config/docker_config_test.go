package config

import (
	"os"
	"path/filepath"
	"strings"
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
	if !cfg.Engines.Fofa.Enabled || !cfg.Engines.Hunter.Enabled || !cfg.Engines.Zoomeye.Enabled ||
		!cfg.Engines.Quake.Enabled || !cfg.Engines.Shodan.Enabled {
		t.Fatal("Docker baseline must explicitly enable the five stable Web engines")
	}
	if cfg.Engines.Censys.Enabled || cfg.Engines.Daydaymap.Enabled {
		t.Fatal("Docker baseline must not enable API-only engines without credentials")
	}
}

func TestDeploymentYAMLFilesAreSyntacticallyValid(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "docker-compose.yml"),
		filepath.Join("..", "..", "docker-compose.prod.yaml"),
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

func TestProductionComposeReplacesDevelopmentPortsAndVolumes(t *testing.T) {
	path := filepath.Join("..", "..", "docker-compose.prod.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	ports := yamlNodeAt(t, &doc, "services", "unimap", "ports")
	if ports.Tag != "!override" {
		t.Fatalf("production ports tag = %q, want !override so the public base binding is removed", ports.Tag)
	}
	if got := yamlScalarValues(ports); len(got) != 1 || got[0] != "127.0.0.1:8448:8448" {
		t.Fatalf("unexpected production ports: %v", got)
	}

	volumes := yamlNodeAt(t, &doc, "services", "unimap", "volumes")
	if volumes.Tag != "!override" {
		t.Fatalf("production volumes tag = %q, want !override so the development web mount is removed", volumes.Tag)
	}
	joined := strings.Join(yamlScalarValues(volumes), "\n")
	for _, required := range []string{"unimap_config:/app/runtime-config", "/app/logs", "/app/data", "/app/screenshots", "/app/chrome-profile", "/app/backups"} {
		if !strings.Contains(joined, required) {
			t.Errorf("production volumes missing %q: %s", required, joined)
		}
	}
	if strings.Contains(joined, "./web:/app/web") {
		t.Fatalf("production volumes retain the development web bind mount: %s", joined)
	}
}

func TestProductionRuntimeConfigIsInitializedAndWritable(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("..", "..", "Dockerfile"): {
			"COPY scripts/docker-entrypoint.sh /usr/local/bin/unimap-entrypoint",
			"/app/runtime-config",
			`ENTRYPOINT ["unimap-entrypoint"]`,
		},
		filepath.Join("..", "..", "docker-compose.prod.yaml"): {
			"UNIMAP_CONFIG_PATH: /app/runtime-config/config.yaml",
			"UNIMAP_CONFIG_TEMPLATE: /app/configs/config.prod.yaml",
			"unimap_config:/app/runtime-config",
			"unimap_config:",
		},
		filepath.Join("..", "..", "scripts", "docker-entrypoint.sh"): {
			`config_path="${UNIMAP_CONFIG_PATH:-/app/configs/config.yaml}"`,
			`config_template="${UNIMAP_CONFIG_TEMPLATE:-/app/configs/config.docker.yaml}"`,
			`if [ ! -f "$config_path" ]; then`,
			`cp "$config_template" "$config_path"`,
			`chmod 600 "$config_path"`,
			`exec "$@"`,
		},
	}
	for path, required := range checks {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		content := string(data)
		for _, value := range required {
			if !strings.Contains(content, value) {
				t.Errorf("%s missing runtime config contract %q", path, value)
			}
		}
	}
}

func TestProductionComposeRequiresPepperAndSupportsCapacityOverrides(t *testing.T) {
	path := filepath.Join("..", "..", "docker-compose.prod.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, required := range []string{
		"UNIMAP_NOTIFY_PEPPER: ${UNIMAP_NOTIFY_PEPPER:?",
		"cpus: ${UNIMAP_CPU_LIMIT:-4}",
		"memory: ${UNIMAP_MEMORY_LIMIT:-6G}",
		"cpus: ${UNIMAP_CPU_RESERVATION:-2}",
		"memory: ${UNIMAP_MEMORY_RESERVATION:-4G}",
	} {
		if !strings.Contains(content, required) {
			t.Errorf("production Compose missing deployment contract %q", required)
		}
	}
	if strings.HasPrefix(strings.TrimSpace(content), "version:") {
		t.Error("production Compose retains obsolete top-level version")
	}
}

func TestDockerBuildInjectsVersionMetadata(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("..", "..", "Dockerfile"): {
			"ARG UNIMAP_VERSION=dev",
			"ARG UNIMAP_GIT_COMMIT=unknown",
			"ARG UNIMAP_BUILD_TIME=unknown",
			"appversion.Version=${UNIMAP_VERSION}",
			"appversion.GitCommit=${UNIMAP_GIT_COMMIT}",
			"appversion.BuildTime=${UNIMAP_BUILD_TIME}",
		},
		filepath.Join("..", "..", "docker-compose.yml"): {
			"UNIMAP_VERSION: ${UNIMAP_VERSION:-dev}",
			"UNIMAP_GIT_COMMIT: ${UNIMAP_GIT_COMMIT:-unknown}",
			"UNIMAP_BUILD_TIME: ${UNIMAP_BUILD_TIME:-unknown}",
		},
		filepath.Join("..", "..", ".github", "workflows", "ci.yml"): {
			"UNIMAP_VERSION=${{ github.ref_name }}",
			"UNIMAP_GIT_COMMIT=${{ github.sha }}",
			"UNIMAP_BUILD_TIME=${{ github.event.head_commit.timestamp }}",
		},
	}
	for path, required := range checks {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		for _, value := range required {
			if !strings.Contains(content, value) {
				t.Errorf("%s missing version build contract %q", path, value)
			}
		}
	}
}

func TestProductionImagePreparesWritableLogsAndBackups(t *testing.T) {
	dockerfile, err := os.ReadFile(filepath.Join("..", "..", "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dockerfile), "/app/logs /app/backups") {
		t.Error("Dockerfile does not create and chown both logs and backups before switching to the non-root user")
	}

	compose, err := os.ReadFile(filepath.Join("..", "..", "docker-compose.prod.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(compose)
	if !strings.Contains(content, "unimap_logs:/app/logs") {
		t.Error("production Compose must use the initialized logs volume")
	}
	if strings.Contains(content, "./logs:/app/logs") {
		t.Error("production Compose retains a host log bind mount with uncontrolled ownership")
	}
	if !strings.Contains(content, "unimap_logs:") {
		t.Error("production Compose does not declare the logs volume")
	}
}

func TestProductionConfigResolvesStableAdminTokens(t *testing.T) {
	t.Setenv("UNIMAP_ADMIN_TOKEN", "stable-web-admin-token")
	t.Setenv("UNIMAP_DISTRIBUTED_ADMIN_TOKEN", "stable-distributed-admin-token")
	t.Setenv("UNIMAP_ADMIN_USERNAME", "prod-operator")
	t.Setenv("UNIMAP_BOOTSTRAP_PASSWORD", "a-long-random-deployment-password")
	mgr := NewManager(filepath.Join("..", "..", "configs", "config.prod.yaml"))
	if err := mgr.Load(); err != nil {
		t.Fatalf("load production config: %v", err)
	}
	cfg := mgr.GetConfig()
	if cfg.Web.Auth.AdminToken != "stable-web-admin-token" {
		t.Fatalf("web admin token = %q", cfg.Web.Auth.AdminToken)
	}
	if cfg.Distributed.AdminToken != "stable-distributed-admin-token" {
		t.Fatalf("distributed admin token = %q", cfg.Distributed.AdminToken)
	}
	if cfg.Web.Auth.Username != "prod-operator" {
		t.Fatalf("web admin username = %q, want deployment username", cfg.Web.Auth.Username)
	}
	if cfg.Web.Auth.PasswordHash == "" || cfg.Web.Auth.PasswordHash == "a-long-random-deployment-password" {
		t.Fatal("production bootstrap password was not securely hashed")
	}
}

func TestProductionConfigEnablesSelectedTrialEngines(t *testing.T) {
	t.Setenv("UNIMAP_ADMIN_TOKEN", "stable-web-admin-token")
	t.Setenv("UNIMAP_DISTRIBUTED_ADMIN_TOKEN", "stable-distributed-admin-token")
	t.Setenv("UNIMAP_ADMIN_USERNAME", "prod-operator")
	t.Setenv("UNIMAP_BOOTSTRAP_PASSWORD", "a-long-random-deployment-password")
	t.Setenv("HUNTER_API_KEY", "hunter-test-key")
	t.Setenv("QUAKE_API_KEY", "quake-test-key")

	mgr := NewManager(filepath.Join("..", "..", "configs", "config.prod.yaml"))
	if err := mgr.Load(); err != nil {
		t.Fatalf("load production config: %v", err)
	}
	cfg := mgr.GetConfig()
	if !cfg.Engines.Hunter.Enabled || cfg.Engines.Hunter.APIKey != "hunter-test-key" {
		t.Fatalf("unexpected Hunter trial config: enabled=%v key_set=%v", cfg.Engines.Hunter.Enabled, cfg.Engines.Hunter.APIKey != "")
	}
	if cfg.Engines.Hunter.BaseURL != "https://hunter.qianxin.com" {
		t.Fatalf("Hunter base URL = %q", cfg.Engines.Hunter.BaseURL)
	}
	if !cfg.Engines.Quake.Enabled || cfg.Engines.Quake.APIKey != "quake-test-key" {
		t.Fatalf("unexpected Quake trial config: enabled=%v key_set=%v", cfg.Engines.Quake.Enabled, cfg.Engines.Quake.APIKey != "")
	}
	if cfg.Engines.Quake.BaseURL != "https://quake.360.net/api" {
		t.Fatalf("Quake base URL = %q", cfg.Engines.Quake.BaseURL)
	}
	if cfg.Engines.Censys.Enabled || cfg.Engines.Daydaymap.Enabled {
		t.Fatal("API-only engines must remain disabled in the production trial template")
	}

	compose, err := os.ReadFile(filepath.Join("..", "..", "docker-compose.prod.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(compose)
	for _, required := range []string{
		"HUNTER_API_KEY: ${HUNTER_API_KEY:-}",
		"QUAKE_API_KEY: ${QUAKE_API_KEY:-}",
	} {
		if !strings.Contains(content, required) {
			t.Errorf("production Compose missing engine secret pass-through %q", required)
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

func yamlNodeAt(t *testing.T, doc *yaml.Node, path ...string) *yaml.Node {
	t.Helper()
	if len(doc.Content) == 0 {
		t.Fatal("empty YAML document")
	}
	node := doc.Content[0]
	for _, key := range path {
		if node.Kind != yaml.MappingNode {
			t.Fatalf("YAML path %v reached non-mapping node", path)
		}
		var next *yaml.Node
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				next = node.Content[i+1]
				break
			}
		}
		if next == nil {
			t.Fatalf("YAML path %v missing key %q", path, key)
		}
		node = next
	}
	return node
}

func yamlScalarValues(node *yaml.Node) []string {
	values := make([]string, 0, len(node.Content))
	for _, child := range node.Content {
		if child.Kind == yaml.ScalarNode {
			values = append(values, child.Value)
		}
	}
	return values
}
