package fingerprint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEngine_LoadsEmbeddedRules(t *testing.T) {
	engine, err := NewEngine()
	require.NoError(t, err)
	assert.Greater(t, engine.RuleCount(), 50, "应加载至少 50 条规则")
	t.Logf("Loaded %d rules", engine.RuleCount())
}

func TestNewEngineFromBytes_InvalidYAML(t *testing.T) {
	_, err := NewEngineFromBytes([]byte("!!invalid yaml!!"))
	assert.Error(t, err)
}

func TestNewEngineFromBytes_InvalidRegex(t *testing.T) {
	data := []byte(`
- name: BadRule
  category: webserver
  type: header
  key: Server
  regex: (?i)[unclosed
`)
	_, err := NewEngineFromBytes(data)
	assert.Error(t, err)
}

func TestEngine_Match_HeaderRule(t *testing.T) {
	data := []byte(`
- name: Nginx
  category: webserver
  type: header
  key: Server
  regex: (?i)nginx
- name: Apache
  category: webserver
  type: header
  key: Server
  regex: (?i)apache
`)
	engine, err := NewEngineFromBytes(data)
	require.NoError(t, err)

	headers := map[string]string{"Server": "nginx/1.24.0"}
	results := engine.Match(headers, "", "", "")

	require.Len(t, results, 1)
	assert.Equal(t, "Nginx", results[0].RuleName)
	assert.Equal(t, CatWebServer, results[0].Category)
}

func TestEngine_Match_BodyRule(t *testing.T) {
	data := []byte(`
- name: WordPress
  category: cms
  type: body
  regex: (?i)wp-content
`)
	engine, err := NewEngineFromBytes(data)
	require.NoError(t, err)

	body := `<html><head><link href="/wp-content/themes/style.css"></head><body></body></html>`
	results := engine.Match(nil, body, "", "")

	require.Len(t, results, 1)
	assert.Equal(t, "WordPress", results[0].RuleName)
}

func TestEngine_Match_TitleRule(t *testing.T) {
	data := []byte(`
- name: Grafana
  category: monitoring
  type: title
  regex: (?i)grafana
`)
	engine, err := NewEngineFromBytes(data)
	require.NoError(t, err)

	results := engine.Match(nil, "", "Grafana Dashboard", "")

	require.Len(t, results, 1)
	assert.Equal(t, "Grafana", results[0].RuleName)
}

func TestEngine_Match_CookieRule(t *testing.T) {
	data := []byte(`
- name: PHPSESSID
  category: framework
  type: cookie
  regex: (?i)PHPSESSID
`)
	engine, err := NewEngineFromBytes(data)
	require.NoError(t, err)

	cookies := "PHPSESSID=abc123; path=/"
	results := engine.Match(nil, "", "", cookies)

	require.Len(t, results, 1)
	assert.Equal(t, "PHPSESSID", results[0].RuleName)
}

func TestEngine_Match_DeduplicatesByName(t *testing.T) {
	data := []byte(`
- name: Nginx
  category: webserver
  type: header
  key: Server
  regex: (?i)nginx
- name: Nginx
  category: webserver
  type: body
  regex: (?i)nginx
`)
	engine, err := NewEngineFromBytes(data)
	require.NoError(t, err)

	headers := map[string]string{"Server": "nginx"}
	body := "Nginx powered"
	results := engine.Match(headers, body, "", "")

	assert.Len(t, results, 1, "同名规则应去重")
}

func TestEngine_Match_NoMatch(t *testing.T) {
	data := []byte(`
- name: Nginx
  category: webserver
  type: header
  key: Server
  regex: (?i)nginx
`)
	engine, err := NewEngineFromBytes(data)
	require.NoError(t, err)

	headers := map[string]string{"Server": "Apache/2.4"}
	results := engine.Match(headers, "", "", "")

	assert.Empty(t, results)
}

func TestEngine_Match_HeaderKeyMissing(t *testing.T) {
	data := []byte(`
- name: Nginx
  category: webserver
  type: header
  key: Server
  regex: (?i)nginx
`)
	engine, err := NewEngineFromBytes(data)
	require.NoError(t, err)

	headers := map[string]string{"X-Custom": "value"}
	results := engine.Match(headers, "", "", "")

	assert.Empty(t, results)
}

func TestEngine_Match_EmptyRuleSet(t *testing.T) {
	engine, err := NewEngineFromBytes([]byte(`[]`))
	require.NoError(t, err)
	assert.Equal(t, 0, engine.RuleCount())

	results := engine.Match(nil, "body", "title", "")
	assert.Empty(t, results)
}

func TestEmbeddedRules_AllCompile(t *testing.T) {
	engine, err := NewEngine()
	require.NoError(t, err, "嵌入式 rules.yaml 所有规则应成功编译")
	assert.Greater(t, engine.RuleCount(), 0)
	t.Logf("Total embedded rules: %d", engine.RuleCount())
}
