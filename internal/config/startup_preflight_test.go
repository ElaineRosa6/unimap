package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestAuthDefaultsNoImplicitDefaultCredentials 证明 loopback 默认值不再生成
// admin/admin 默认口令，也不再自动生成 admin_token。
func TestAuthDefaultsNoImplicitDefaultCredentials(t *testing.T) {
	mgr := NewManager("test.yaml")
	var cfg Config
	cfg.Web.BindAddress = "127.0.0.1"
	mgr.applyDefaults(&cfg)

	if cfg.Web.Auth.PasswordHash != "" {
		t.Fatalf("loopback defaults must not generate a password hash, got %q", cfg.Web.Auth.PasswordHash)
	}
	if cfg.Web.Auth.AdminToken != "" {
		t.Fatalf("loopback defaults must not auto-generate an admin token, got %q", cfg.Web.Auth.AdminToken)
	}
	if !cfg.Web.Auth.Enabled {
		t.Fatal("auth must be enabled by default")
	}
	if CheckPassword("admin", cfg.Web.Auth.PasswordHash) {
		t.Fatal("admin/admin must not be a valid credential after defaults")
	}
	// username 仅作 loopback 占位，不构成凭据。
	if cfg.Web.Auth.Username != "admin" {
		t.Fatalf("loopback username placeholder = %q, want admin", cfg.Web.Auth.Username)
	}
}

// TestAuthDefaultsNonLoopbackNoImplicitCredentials 证明非 loopback 默认值
// 不会回退出任何隐式凭据：username 不被默认成 admin、token 不被自动生成、密码为空。
func TestAuthDefaultsNonLoopbackNoImplicitCredentials(t *testing.T) {
	mgr := NewManager("test.yaml")
	var cfg Config
	cfg.Web.BindAddress = "0.0.0.0"
	mgr.applyDefaults(&cfg)

	if cfg.Web.Auth.PasswordHash != "" {
		t.Fatalf("non-loopback defaults must not generate a password hash, got %q", cfg.Web.Auth.PasswordHash)
	}
	if cfg.Web.Auth.AdminToken != "" {
		t.Fatalf("non-loopback defaults must not auto-generate an admin token, got %q", cfg.Web.Auth.AdminToken)
	}
	if cfg.Web.Auth.Username != "" {
		t.Fatalf("non-loopback defaults must not default username to %q", cfg.Web.Auth.Username)
	}
	if err := StartupPreflight(&cfg); err == nil {
		t.Fatal("non-loopback config without explicit credentials must fail startup preflight")
	} else if !strings.Contains(err.Error(), "admin_token") || !strings.Contains(err.Error(), "username") || !strings.Contains(err.Error(), "password_hash") {
		t.Fatalf("preflight error must name all missing fields, got: %v", err)
	}
}

// TestAuthDefaultsBootstrapPasswordHashedInMemory 证明 UNIMAP_BOOTSTRAP_PASSWORD
// 只以 bcrypt 哈希进入内存配置，明文不进入配置对象、不进入序列化结果。
func TestAuthDefaultsBootstrapPasswordHashedInMemory(t *testing.T) {
	const plaintext = "s3cret-bootstrap-password"
	t.Setenv("UNIMAP_BOOTSTRAP_PASSWORD", plaintext)
	t.Setenv("UNIMAP_ADMIN_TOKEN", "")
	t.Setenv("UNIMAP_ADMIN_USERNAME", "")

	mgr := NewManager("test.yaml")
	var cfg Config
	cfg.Web.BindAddress = "127.0.0.1"
	mgr.applyDefaults(&cfg)

	if cfg.Web.Auth.PasswordHash == "" {
		t.Fatal("bootstrap password must be hashed into config")
	}
	if cfg.Web.Auth.PasswordHash == plaintext {
		t.Fatal("bootstrap password must not remain plaintext in config")
	}
	if !CheckPassword(plaintext, cfg.Web.Auth.PasswordHash) {
		t.Fatal("hashed bootstrap password must verify against the plaintext")
	}
	// 序列化结果不得包含明文（证明不会持久化明文）。
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), plaintext) {
		t.Fatal("serialized config must not contain the bootstrap plaintext")
	}
	if cfg.Web.Auth.AdminToken != "" {
		t.Fatalf("bootstrap env must not trigger admin_token auto-generation, got %q", cfg.Web.Auth.AdminToken)
	}
}

// TestLoopbackBind 覆盖 isLoopbackBind 的边界：空白、大小写、IPv4 127.0.0.0/8、::1，
// 且 0.0.0.0 不视为回环。
func TestLoopbackBind(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1", true},
		{"127.0.0.1:8448", true},
		{" 127.0.0.1 ", true},
		{"127.0.0.2", true}, // 127.0.0.0/8 整段
		{"127.255.255.254", true},
		{"localhost", true},
		{"LocalHost", true},
		{"LOCALHOST", true},
		{"localhost:8448", true},
		{"localhost.", true},
		{"::1", true},
		{"::1:8448", false}, // 缺 [] 的 IPv6+端口无法解析，按非回环处理
		{"[::1]:8448", true},
		{"0.0.0.0", false},
		{"0.0.0.0:8448", false},
		{"192.168.1.10", false},
		{"10.0.0.1", false},
		{"example.com", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsLoopbackBind(tc.addr); got != tc.want {
			t.Errorf("IsLoopbackBind(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

// TestStartupPreflightLoopbackAllowsEmptyCredentials 证明 loopback 绑定下
// 允许 admin_token/password_hash 为空（首用户注册初始化）。
func TestStartupPreflightLoopbackAllowsEmptyCredentials(t *testing.T) {
	for _, bind := range []string{"127.0.0.1", "127.0.0.1:8448", "localhost", "LocalHost", "::1", "[::1]:8448"} {
		cfg := &Config{}
		cfg.Web.BindAddress = bind
		cfg.Web.Auth.Enabled = true
		if err := StartupPreflight(cfg); err != nil {
			t.Errorf("loopback bind %q with empty credentials must pass, got: %v", bind, err)
		}
	}
}

// TestStartupPreflightNonLoopbackRequiresEnabled 证明非 loopback 且未启用 auth 时失败并指明字段。
func TestStartupPreflightNonLoopbackRequiresEnabled(t *testing.T) {
	cfg := &Config{}
	cfg.Web.BindAddress = "0.0.0.0"
	cfg.Web.Auth.Enabled = false
	cfg.Web.Auth.AdminToken = "tok"
	cfg.Web.Auth.Username = "operator"
	cfg.Web.Auth.PasswordHash = mustBcrypt(t, "pw")
	if err := StartupPreflight(cfg); err == nil || !strings.Contains(err.Error(), "web.auth.enabled=true") {
		t.Fatalf("expected enabled=true failure, got: %v", err)
	}
}

// TestStartupPreflightNonLoopbackRequiresExplicitCredentials 证明每个缺失字段都被点名。
func TestStartupPreflightNonLoopbackRequiresExplicitCredentials(t *testing.T) {
	cfg := &Config{}
	cfg.Web.BindAddress = "0.0.0.0"
	cfg.Web.Auth.Enabled = true
	// 全部缺失：一次性列出所有字段。
	err := StartupPreflight(cfg)
	if err == nil {
		t.Fatal("expected preflight failure")
	}
	for _, field := range []string{"admin_token", "username", "password_hash"} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("error must name %q, got: %v", field, err)
		}
	}

	// 仅缺 admin_token。
	cfg.Web.Auth.Username = "operator"
	cfg.Web.Auth.PasswordHash = mustBcrypt(t, "pw")
	err = StartupPreflight(cfg)
	if err == nil || !strings.Contains(err.Error(), "admin_token") {
		t.Fatalf("expected admin_token failure, got: %v", err)
	}
	if strings.Contains(err.Error(), "username") || strings.Contains(err.Error(), "password_hash") {
		t.Fatalf("error must not name satisfied fields, got: %v", err)
	}

	// 仅缺 username。
	cfg.Web.Auth.AdminToken = "tok"
	cfg.Web.Auth.Username = ""
	err = StartupPreflight(cfg)
	if err == nil || !strings.Contains(err.Error(), "username") {
		t.Fatalf("expected username failure, got: %v", err)
	}
}

// TestStartupPreflightNonLoopbackRejectsDefaultAdminUsername 证明 username 显式等于 admin 被拒绝。
func TestStartupPreflightNonLoopbackRejectsDefaultAdminUsername(t *testing.T) {
	cfg := &Config{}
	cfg.Web.BindAddress = "0.0.0.0"
	cfg.Web.Auth.Enabled = true
	cfg.Web.Auth.AdminToken = "tok"
	cfg.Web.Auth.Username = "admin"
	cfg.Web.Auth.PasswordHash = mustBcrypt(t, "pw")
	err := StartupPreflight(cfg)
	if err == nil || !strings.Contains(err.Error(), `must not be "admin"`) {
		t.Fatalf("expected username=admin rejection, got: %v", err)
	}
	// 大小写变体同样拒绝。
	cfg.Web.Auth.Username = "Admin"
	if err := StartupPreflight(cfg); err == nil || !strings.Contains(err.Error(), `must not be "admin"`) {
		t.Fatalf("expected case-insensitive username=admin rejection, got: %v", err)
	}
}

// TestStartupPreflightNonLoopbackRejectsInvalidBcrypt 证明非 bcrypt 的 password_hash 被拒绝。
func TestStartupPreflightNonLoopbackRejectsInvalidBcrypt(t *testing.T) {
	for _, hash := range []string{"", "plaintext", "$2a$10$too-short", "admin"} {
		cfg := &Config{}
		cfg.Web.BindAddress = "0.0.0.0"
		cfg.Web.Auth.Enabled = true
		cfg.Web.Auth.AdminToken = "tok"
		cfg.Web.Auth.Username = "operator"
		cfg.Web.Auth.PasswordHash = hash
		if err := StartupPreflight(cfg); err == nil || !strings.Contains(err.Error(), "password_hash") {
			t.Errorf("hash %q must be rejected with password_hash field named, got: %v", hash, err)
		}
	}
}

// TestStartupPreflightNonLoopbackValidBcryptPasses 证明完整有效配置通过。
func TestStartupPreflightNonLoopbackValidBcryptPasses(t *testing.T) {
	cfg := &Config{}
	cfg.Web.BindAddress = "0.0.0.0"
	cfg.Web.Auth.Enabled = true
	cfg.Web.Auth.AdminToken = "explicit-token"
	cfg.Web.Auth.Username = "operator"
	cfg.Web.Auth.PasswordHash = mustBcrypt(t, "correct horse battery staple")
	if err := StartupPreflight(cfg); err != nil {
		t.Fatalf("valid non-loopback config must pass, got: %v", err)
	}
}

// TestStartupPreflightNilConfig 证明 nil 配置返回错误。
func TestStartupPreflightNilConfig(t *testing.T) {
	if err := StartupPreflight(nil); err == nil {
		t.Fatal("nil config must fail preflight")
	}
}

// TestDockerConfigLoopbackPreflightPasses 证明镜像基线（loopback、空凭据）可启动。
func TestDockerConfigLoopbackPreflightPasses(t *testing.T) {
	t.Setenv("UNIMAP_CONTAINER_BIND_ADDRESS", "")
	t.Setenv("UNIMAP_ADMIN_TOKEN", "")
	t.Setenv("UNIMAP_ADMIN_USERNAME", "")
	t.Setenv("UNIMAP_BOOTSTRAP_PASSWORD", "")
	mgr := NewManager(filepath.Join("..", "..", "configs", "config.docker.yaml"))
	if err := mgr.Load(); err != nil {
		t.Fatalf("load docker baseline: %v", err)
	}
	cfg := mgr.GetConfig()
	if cfg.Web.BindAddress != "127.0.0.1" {
		t.Fatalf("baseline bind = %q, want loopback", cfg.Web.BindAddress)
	}
	if err := StartupPreflight(cfg); err != nil {
		t.Fatalf("loopback baseline must pass preflight, got: %v", err)
	}
	if cfg.Web.Auth.PasswordHash != "" {
		t.Fatal("loopback baseline must not carry an implicit password hash")
	}
}

// TestDockerConfigNonLoopbackStrictPreflight 证明 Compose 非 loopback 覆盖下，
// 缺失显式凭据时拒绝启动，补全后通过。
func TestDockerConfigNonLoopbackStrictPreflight(t *testing.T) {
	t.Setenv("UNIMAP_CONTAINER_BIND_ADDRESS", "0.0.0.0")
	t.Setenv("UNIMAP_ADMIN_TOKEN", "")
	t.Setenv("UNIMAP_ADMIN_USERNAME", "")
	t.Setenv("UNIMAP_BOOTSTRAP_PASSWORD", "")
	mgr := NewManager(filepath.Join("..", "..", "configs", "config.docker.yaml"))
	if err := mgr.Load(); err != nil {
		t.Fatalf("load docker config: %v", err)
	}
	cfg := mgr.GetConfig()
	if cfg.Web.BindAddress != "0.0.0.0" {
		t.Fatalf("container bind override failed: %q", cfg.Web.BindAddress)
	}
	err := StartupPreflight(cfg)
	if err == nil {
		t.Fatal("non-loopback docker config without explicit credentials must fail preflight")
	}
	for _, field := range []string{"admin_token", "username", "password_hash"} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("error must name %q, got: %v", field, err)
		}
	}

	// 补全显式凭据后通过。
	t.Setenv("UNIMAP_ADMIN_TOKEN", "explicit-docker-token")
	t.Setenv("UNIMAP_ADMIN_USERNAME", "container-operator")
	t.Setenv("UNIMAP_BOOTSTRAP_PASSWORD", "container-bootstrap-pw")
	mgr2 := NewManager(filepath.Join("..", "..", "configs", "config.docker.yaml"))
	if err := mgr2.Load(); err != nil {
		t.Fatalf("reload docker config: %v", err)
	}
	cfg2 := mgr2.GetConfig()
	if cfg2.Web.Auth.PasswordHash == "container-bootstrap-pw" {
		t.Fatal("bootstrap password must not remain plaintext")
	}
	if err := StartupPreflight(cfg2); err != nil {
		t.Fatalf("fully configured non-loopback docker config must pass, got: %v", err)
	}
}

// TestProductionConfigNonLoopbackPreflight 证明生产模板契约与新 preflight 一致。
func TestProductionConfigNonLoopbackPreflight(t *testing.T) {
	t.Setenv("UNIMAP_CONTAINER_BIND_ADDRESS", "0.0.0.0")
	t.Setenv("UNIMAP_ADMIN_TOKEN", "prod-token")
	t.Setenv("UNIMAP_ADMIN_USERNAME", "prod-operator")
	t.Setenv("UNIMAP_BOOTSTRAP_PASSWORD", "prod-bootstrap-pw")
	mgr := NewManager(filepath.Join("..", "..", "configs", "config.prod.yaml"))
	if err := mgr.Load(); err != nil {
		t.Fatalf("load production config: %v", err)
	}
	cfg := mgr.GetConfig()
	if err := StartupPreflight(cfg); err != nil {
		t.Fatalf("production template must satisfy preflight, got: %v", err)
	}
}

// TestResolveEnvPasswordHashResolved 证明 resolveEnv 支持 web.auth.password_hash：
// ${UNIMAP_PASSWORD_HASH} 占位符被环境变量值替换，且非 loopback preflight 以该 bcrypt 通过。
func TestResolveEnvPasswordHashResolved(t *testing.T) {
	hash := mustBcrypt(t, "env-provided-password")
	t.Setenv("UNIMAP_PASSWORD_HASH", hash)
	t.Setenv("UNIMAP_ADMIN_TOKEN", "explicit-token")
	t.Setenv("UNIMAP_ADMIN_USERNAME", "operator")
	tmp := filepath.Join(t.TempDir(), "config.yaml")
	body := "web:\n  bind_address: 0.0.0.0\n  port: 8448\n  auth:\n    enabled: true\n    admin_token: \"${UNIMAP_ADMIN_TOKEN}\"\n    username: \"${UNIMAP_ADMIN_USERNAME}\"\n    password_hash: \"${UNIMAP_PASSWORD_HASH}\"\n"
	if err := os.WriteFile(tmp, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(tmp)
	if err := mgr.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	cfg := mgr.GetConfig()
	if cfg.Web.Auth.PasswordHash != hash {
		t.Fatalf("password_hash must be resolved from env, got %q", cfg.Web.Auth.PasswordHash)
	}
	if strings.Contains(cfg.Web.Auth.PasswordHash, "UNIMAP_PASSWORD_HASH") {
		t.Fatal("password_hash must not retain the placeholder")
	}
	if err := StartupPreflight(cfg); err != nil {
		t.Fatalf("non-loopback preflight must pass with env-resolved bcrypt, got: %v", err)
	}
}

// TestResolveEnvPasswordHashUnsetBecomesEmpty 证明占位符不会泄漏：
// 未设置 UNIMAP_PASSWORD_HASH 时 resolveEnv 将其解析为空字符串（loopback 下允许，走首用户注册）。
func TestResolveEnvPasswordHashUnsetBecomesEmpty(t *testing.T) {
	t.Setenv("UNIMAP_PASSWORD_HASH", "")
	tmp := filepath.Join(t.TempDir(), "config.yaml")
	body := "web:\n  bind_address: 127.0.0.1\n  port: 8448\n  auth:\n    enabled: true\n    admin_token: \"\"\n    username: \"admin\"\n    password_hash: \"${UNIMAP_PASSWORD_HASH}\"\n"
	if err := os.WriteFile(tmp, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(tmp)
	if err := mgr.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	cfg := mgr.GetConfig()
	if cfg.Web.Auth.PasswordHash != "" {
		t.Fatalf("unset env must resolve to empty string, got %q", cfg.Web.Auth.PasswordHash)
	}
	if strings.Contains(cfg.Web.Auth.PasswordHash, "UNIMAP_PASSWORD_HASH") {
		t.Fatal("placeholder must not leak into the config")
	}
	if err := StartupPreflight(cfg); err != nil {
		t.Fatalf("loopback preflight must pass with empty resolved hash, got: %v", err)
	}
}

// TestProductionComposeBindContract 证明生产 Compose 监听契约：
// 容器内以 UNIMAP_CONTAINER_BIND_ADDRESS=0.0.0.0 启动（触发 non-loopback preflight），
// 宿主发布端口仍只映射 127.0.0.1:8448:8448。
func TestProductionComposeBindContract(t *testing.T) {
	prodPath := filepath.Join("..", "..", "docker-compose.prod.yaml")
	basePath := filepath.Join("..", "..", "docker-compose.yml")
	assertComposeEnvValue(t, prodPath, "UNIMAP_CONTAINER_BIND_ADDRESS", "0.0.0.0")
	assertComposePortsContain(t, prodPath, "127.0.0.1:8448:8448")
	assertComposeEnvValue(t, basePath, "UNIMAP_CONTAINER_BIND_ADDRESS", "0.0.0.0")
}

func composeRoot(t *testing.T, path string) *yaml.Node {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return &root
}

func composeMapAt(root *yaml.Node, path ...string) (*yaml.Node, bool) {
	cur := root
	if cur != nil && cur.Kind == yaml.DocumentNode && len(cur.Content) > 0 {
		cur = cur.Content[0]
	}
	for _, key := range path {
		if cur.Kind != yaml.MappingNode {
			return nil, false
		}
		found := false
		for i := 0; i+1 < len(cur.Content); i += 2 {
			if cur.Content[i].Value == key {
				cur = cur.Content[i+1]
				found = true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	return cur, true
}

func assertComposeEnvValue(t *testing.T, path, key, want string) {
	t.Helper()
	env, ok := composeMapAt(composeRoot(t, path), "services", "unimap", "environment")
	if !ok {
		t.Fatalf("%s: services.unimap.environment not found", path)
	}
	for i := 0; i+1 < len(env.Content); i += 2 {
		if env.Content[i].Value == key {
			if got := env.Content[i+1].Value; got != want {
				t.Fatalf("%s: %s = %q, want %q", path, key, got, want)
			}
			return
		}
	}
	t.Fatalf("%s: %s must be declared in services.unimap.environment", path, key)
}

func assertComposePortsContain(t *testing.T, path, want string) {
	t.Helper()
	ports, ok := composeMapAt(composeRoot(t, path), "services", "unimap", "ports")
	if !ok {
		t.Fatalf("%s: services.unimap.ports not found", path)
	}
	if ports.Kind != yaml.SequenceNode {
		t.Fatalf("%s: services.unimap.ports is not a sequence (kind=%d)", path, ports.Kind)
	}
	for _, item := range ports.Content {
		if item.Value == want {
			return
		}
	}
	t.Fatalf("%s: ports must contain %q, got %v", path, want, ports.Content)
}

// mustBcrypt 生成测试用 bcrypt 哈希。
func mustBcrypt(t *testing.T, password string) string {
	t.Helper()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}
