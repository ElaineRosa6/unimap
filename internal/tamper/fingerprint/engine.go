package fingerprint

import (
	_ "embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed rules.yaml
var embeddedRules []byte

// Engine 指纹识别引擎
type Engine struct {
	rules compiledRuleSet
}

// NewEngine 从嵌入式 rules.yaml 创建指纹识别引擎
func NewEngine() (*Engine, error) {
	return NewEngineFromBytes(embeddedRules)
}

// NewEngineFromBytes 从 YAML 字节创建引擎（测试用）
func NewEngineFromBytes(data []byte) (*Engine, error) {
	var rules []FingerprintRule
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("fingerprint: parse rules: %w", err)
	}

	for i := range rules {
		if err := rules[i].compile(); err != nil {
			return nil, fmt.Errorf("fingerprint: compile rule %q: %w", rules[i].Name, err)
		}
	}

	e := &Engine{}
	e.rules.set(rules)
	return e, nil
}

// Match 运行所有规则，返回匹配的指纹列表
func (e *Engine) Match(headers map[string]string, body, title, setCookie string) []FingerprintResult {
	rules := e.rules.get()
	seen := make(map[string]bool, len(rules)/4)
	var results []FingerprintResult

	for i := range rules {
		r := &rules[i]
		if seen[r.Name] {
			continue
		}
		if r.compiled == nil {
			continue
		}
		if r.Match(headers, body, title, setCookie) {
			seen[r.Name] = true
			results = append(results, FingerprintResult{
				RuleName: r.Name,
				Category: r.Category,
			})
		}
	}

	return results
}

// RuleCount 返回已加载的规则数量
func (e *Engine) RuleCount() int {
	return len(e.rules.get())
}
