package unimap

import (
	"fmt"
	"strings"
	"text/scanner"

	"github.com/unimap/project/internal/model"
)

// UQLParser UQL查询语言解析器
type UQLParser struct {
	scanner *scanner.Scanner // nolint:unused
	current rune             // nolint:unused
}

// NewUQLParser 创建解析器
func NewUQLParser() *UQLParser {
	return &UQLParser{}
}

// Parse 解析UQL查询字符串为AST
func (p *UQLParser) Parse(query string) (*model.UQLAST, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// 简单的词法分析
	tokens := p.tokenize(query)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no valid tokens found")
	}

	// 构建AST
	root, err := p.buildAST(tokens)
	if err != nil {
		return nil, err
	}

	return &model.UQLAST{Root: root}, nil
}

// tokenize 将查询字符串分解为token
func (p *UQLParser) tokenize(query string) []string {
	tokens := []string{}
	current := ""
	inQuotes := false
	inBrackets := false
	escape := false
	runes := []rune(query)

	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if escape {
			current += string(ch)
			escape = false
			continue
		}
		if ch == '\\' {
			escape = true
			continue
		}
		if ch == '"' {
			inQuotes = !inQuotes
			current += string(ch)
			continue
		}

		if !inQuotes {
			if op, consumed := tryTokenizeOperator(runes, i); op != "" {
				if current != "" {
					tokens = append(tokens, current)
					current = ""
				}
				tokens = append(tokens, op)
				i += consumed - 1 // -1 because loop increments
				continue
			}

			switch ch {
			case '(':
				if current != "" {
					tokens = append(tokens, current)
					current = ""
				}
				tokens = append(tokens, "(")
				continue
			case ')':
				if current != "" {
					tokens = append(tokens, current)
					current = ""
				}
				tokens = append(tokens, ")")
				continue
			case '[':
				if current != "" {
					tokens = append(tokens, current)
					current = ""
				}
				tokens = append(tokens, "[")
				inBrackets = true
				continue
			case ']':
				if current != "" {
					tokens = append(tokens, current)
					current = ""
				}
				tokens = append(tokens, "]")
				inBrackets = false
				continue
			case ',':
				if inBrackets {
					if current != "" {
						tokens = append(tokens, current)
						current = ""
					}
					continue
				}
			}

			if !inBrackets && (ch == ' ' || ch == '\t' || ch == '\n') {
				if current != "" {
					tokens = append(tokens, current)
					current = ""
				}
				continue
			}
		}

		current += string(ch)
	}

	if current != "" {
		tokens = append(tokens, current)
	}
	return tokens
}

// tryTokenizeOperator 尝试从当前位置匹配多字符操作符，返回 (操作符, 消耗字符数)
func tryTokenizeOperator(runes []rune, pos int) (string, int) {
	ch := runes[pos]
	if ch != '&' && ch != '|' && ch != '=' && ch != '!' && ch != '>' && ch != '<' && ch != '~' {
		return "", 0
	}
	if pos+1 >= len(runes) {
		return string(ch), 1
	}
	next := runes[pos+1]
	switch {
	case ch == '&' && next == '&':
		return "&&", 2
	case ch == '|' && next == '|':
		return "||", 2
	case ch == '!' && next == '=':
		return "!=", 2
	case ch == '=' && next == '=':
		return "==", 2
	case ch == '>' && next == '=':
		return ">=", 2
	case ch == '<' && next == '=':
		return "<=", 2
	case ch == '<' && next == '>':
		return "<>", 2
	case ch == '~' && next == '=':
		return "~=", 2
	}
	return string(ch), 1
}

// buildAST 从token构建AST
func (p *UQLParser) buildAST(tokens []string) (*model.UQLNode, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no tokens")
	}

	var parseOr func(int) (*model.UQLNode, int, error)
	var parseAnd func(int) (*model.UQLNode, int, error)

	parseTerm := func(start int) (*model.UQLNode, int, error) {
		if start >= len(tokens) {
			return nil, start, fmt.Errorf("unexpected end of expression")
		}
		if tokens[start] == "(" {
			node, next, err := parseOr(start + 1)
			if err != nil {
				return nil, start, err
			}
			if next >= len(tokens) || tokens[next] != ")" {
				return nil, start, fmt.Errorf("missing closing parenthesis")
			}
			return node, next + 1, nil
		}
		return parseCondition(tokens, start)
	}

	parseAnd = func(start int) (*model.UQLNode, int, error) {
		left, next, err := parseTerm(start)
		if err != nil {
			return nil, start, err
		}
		for next < len(tokens) {
			token := tokens[next]
			if token != "&&" && strings.ToUpper(token) != "AND" {
				break
			}
			right, after, err := parseTerm(next + 1)
			if err != nil {
				return nil, next, err
			}
			left = &model.UQLNode{Type: "logical", Value: "AND", Children: []*model.UQLNode{left, right}}
			next = after
		}
		return left, next, nil
	}

	parseOr = func(start int) (*model.UQLNode, int, error) {
		left, next, err := parseAnd(start)
		if err != nil {
			return nil, start, err
		}
		for next < len(tokens) {
			token := tokens[next]
			if token != "||" && strings.ToUpper(token) != "OR" {
				break
			}
			right, after, err := parseAnd(next + 1)
			if err != nil {
				return nil, next, err
			}
			left = &model.UQLNode{Type: "logical", Value: "OR", Children: []*model.UQLNode{left, right}}
			next = after
		}
		return left, next, nil
	}

	root, next, err := parseOr(0)
	if err != nil {
		return nil, err
	}
	if next < len(tokens) {
		return nil, fmt.Errorf("unexpected token: %s", tokens[next])
	}
	return root, nil
}

// ExtractConditions 提取查询条件。
// 返回条件映射和重复字段告警列表。
func (p *UQLParser) ExtractConditions(ast *model.UQLAST) (map[string]interface{}, []string) {
	conditions := make(map[string]interface{})
	var warnings []string
	if ast == nil || ast.Root == nil {
		return conditions, warnings
	}

	seenFields := make(map[string]int)

	var traverse func(*model.UQLNode)
	traverse = func(node *model.UQLNode) {
		if node == nil {
			return
		}

		if node.Type == "condition" && len(node.Children) >= 2 {
			field := node.Value
			op := node.Children[0].Value
			val := node.Children[1].Value

			seenFields[field]++
			if seenFields[field] > 1 {
				warnings = append(warnings, fmt.Sprintf("duplicate field %q - only last value will be used", field))
			}

			if op == "IN" {
				// 解析数组
				values := strings.Split(val, ",")
				conditions[field] = map[string]interface{}{
					"operator": "IN",
					"value":    values,
				}
			} else {
				conditions[field] = map[string]interface{}{
					"operator": op,
					"value":    val,
				}
			}
		}

		// 递归子节点
		for _, child := range node.Children {
			traverse(child)
		}
	}

	traverse(ast.Root)
	return conditions, warnings
}

// parseCondition 解析单个条件表达式（field op value 或 field IN [...]）
func parseCondition(tokens []string, start int) (*model.UQLNode, int, error) {
	if start+2 >= len(tokens) {
		return nil, start, fmt.Errorf("incomplete condition")
	}
	field := tokens[start]
	operator := tokens[start+1]

	if strings.ToUpper(operator) == "IN" {
		return parseINExpression(tokens, field, start)
	}

	validOps := map[string]bool{
		"=": true, "==": true, "!=": true, "<>": true,
		">": true, "<": true, ">=": true, "<=": true, "~=": true,
	}
	if !validOps[operator] && strings.ToUpper(operator) != "CONTAINS" {
		return nil, start, fmt.Errorf("unsupported operator: %s", operator)
	}
	if start+2 >= len(tokens) {
		return nil, start, fmt.Errorf("missing value")
	}
	value := strings.Trim(tokens[start+2], `"`)
	return &model.UQLNode{
		Type:  "condition",
		Value: field,
		Children: []*model.UQLNode{
			{Type: "operator", Value: operator},
			{Type: "value", Value: value},
		},
	}, start + 3, nil
}

// parseINExpression 解析 IN [value1, value2, ...] 表达式
func parseINExpression(tokens []string, field string, start int) (*model.UQLNode, int, error) {
	if start+3 >= len(tokens) || tokens[start+2] != "[" {
		return nil, start, fmt.Errorf("expected [ after IN")
	}
	end := start + 3
	for end < len(tokens) && tokens[end] != "]" {
		end++
	}
	if end >= len(tokens) {
		return nil, start, fmt.Errorf("missing closing bracket")
	}
	values := make([]string, 0, end-start-3)
	for i := start + 3; i < end; i++ {
		values = append(values, strings.Trim(strings.TrimSpace(tokens[i]), `"`))
	}
	return &model.UQLNode{
		Type:  "condition",
		Value: field,
		Children: []*model.UQLNode{
			{Type: "operator", Value: "IN"},
			{Type: "array", Value: strings.Join(values, ",")},
		},
	}, end + 1, nil
}

// Validate 验证UQL查询的有效性
func (p *UQLParser) Validate(query string) error {
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("query cannot be empty")
	}

	// 检查基本结构
	if !strings.Contains(query, "=") && !strings.Contains(strings.ToUpper(query), "IN") {
		return fmt.Errorf("query must contain at least one condition with = or IN")
	}

	return nil
}

// Simplify 简化UQL查询（移除多余空格等）
func (p *UQLParser) Simplify(query string) string {
	query = strings.TrimSpace(query)
	// 标准化空格
	query = strings.ReplaceAll(query, "  ", " ")
	query = strings.ReplaceAll(query, "\t", " ")
	query = strings.ReplaceAll(query, "\n", " ")
	return strings.TrimSpace(query)
}
