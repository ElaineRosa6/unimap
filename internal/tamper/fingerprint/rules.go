package fingerprint

import (
	"regexp"
	"sync"
)

// RuleType 匹配维度
type RuleType string

const (
	TypeHeader RuleType = "header"
	TypeBody   RuleType = "body"
	TypeTitle  RuleType = "title"
	TypeCookie RuleType = "cookie"
)

// Category 规则分类
type Category string

const (
	CatWebServer  Category = "webserver"
	CatFramework  Category = "framework"
	CatFrontend   Category = "frontend"
	CatCMS        Category = "cms"
	CatOA         Category = "oa"
	CatMiddleware Category = "middleware"
	CatMonitoring Category = "monitoring"
	CatDevOps     Category = "devops"
	CatSecurity   Category = "security"
	CatDatabase   Category = "database"
	CatOther      Category = "other"
)

// FingerprintRule 单条指纹规则
type FingerprintRule struct {
	Name     string   `yaml:"name" json:"name"`
	Category Category `yaml:"category" json:"category"`
	Type     RuleType `yaml:"type" json:"type"`
	Key      string   `yaml:"key,omitempty" json:"key,omitempty"`
	Regex    string   `yaml:"regex" json:"regex"`

	// 编译后的正则（不从 YAML 解析）
	compiled *regexp.Regexp `yaml:"-" json:"-"`
}

// FingerprintResult 单条匹配结果
type FingerprintResult struct {
	RuleName string   `json:"rule_name"`
	Category Category `json:"category"`
}

// compile 编译正则表达式
func (r *FingerprintRule) compile() error {
	re, err := regexp.Compile(r.Regex)
	if err != nil {
		return err
	}
	r.compiled = re
	return nil
}

// Match 在给定输入上执行匹配
func (r *FingerprintRule) Match(headers map[string]string, body, title, setCookie string) bool {
	switch r.Type {
	case TypeHeader:
		if r.Key == "" {
			return false
		}
		val, ok := headers[r.Key]
		if !ok {
			return false
		}
		return r.compiled.MatchString(val)

	case TypeBody:
		return r.compiled.MatchString(body)

	case TypeTitle:
		return r.compiled.MatchString(title)

	case TypeCookie:
		return r.compiled.MatchString(setCookie)
	}
	return false
}

// compiledRuleSet 预编译规则集，线程安全
type compiledRuleSet struct {
	mu    sync.RWMutex
	rules []FingerprintRule
}

func (s *compiledRuleSet) set(rules []FingerprintRule) {
	s.mu.Lock()
	s.rules = rules
	s.mu.Unlock()
}

func (s *compiledRuleSet) get() []FingerprintRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rules
}
