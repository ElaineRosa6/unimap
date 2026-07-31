# UniMap CLI Agent 友好化实施指南

> 目标：让 AI Agent（Codex、Claude Code、自定义脚本）能可靠地通过 CLI 完成查询、导出、
> 巡检、截图和调度操作，无需解析人类文本。

## 设计原则

1. **JSON 信封**：所有子命令支持 `--format json`，输出统一信封结构
2. **退出码语义化**：Agent 通过 `$?` 判断结果类型，无需解析 stderr
3. **零交互**：任何场景不弹 prompt，所有输入通过 flag/env/stdin
4. **幂等安全**：相同参数重复执行结果一致，不产生副作用
5. **分页可迭代**：Agent 能自动翻页直到拿完所有结果
6. **自描述**：`help --json` 输出机器可读的命令/参数 schema

---

## 一、统一 JSON 信封

### 当前问题

```bash
$ unimap-cli -q 'port="443"' -e fofa
Querying with engines: [fofa]        # ← 人类文本，混在 stdout
Found 1234 results.                  # ← 人类文本
  fofa: 1234                         # ← 人类文本
8.8.8.8    dns.google:53    DNS      # ← 无结构
```

Agent 无法可靠 parse。

### 目标格式

所有子命令 `--format json` 时，stdout **只输出一个 JSON 对象**：

```json
{
  "ok": true,
  "command": "query",
  "data": {
    "query": "port=\"443\"",
    "assets": [...],
    "total": 1234,
    "page": 1,
    "page_size": 100,
    "has_more": true,
    "engine_stats": {"fofa": 1234},
    "errors": []
  },
  "error": null,
  "exit_code": 0
}
```

失败时：

```json
{
  "ok": false,
  "command": "query",
  "data": null,
  "error": {
    "code": "AUTH_FAILED",
    "message": "fofa: API key invalid",
    "engine": "fofa"
  },
  "exit_code": 2
}
```

### 实施要点

**文件：`cmd/unimap-cli/output.go`（新建）**

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
)

// Exit codes — agent 通过 $? 判断
const (
    ExitOK          = 0
    ExitQueryError  = 1
    ExitAuthError   = 2
    ExitNoEngines   = 3
    ExitUsageError  = 4
    ExitServerError = 5
    ExitTimeout     = 6
)

// CLIEnvelope 是所有 JSON 输出的统一信封
type CLIEnvelope struct {
    OK       bool        `json:"ok"`
    Command  string      `json:"command"`
    Data     interface{} `json:"data,omitempty"`
    Error    *CLIError   `json:"error,omitempty"`
    ExitCode int         `json:"exit_code"`
}

type CLIError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Engine  string `json:"engine,omitempty"`
}

// printJSON 输出信封到 stdout 并退出
func printJSON(cmd string, data interface{}, exitCode int) {
    env := CLIEnvelope{OK: exitCode == 0, Command: cmd, Data: data, ExitCode: exitCode}
    if exitCode != 0 {
        env.OK = false
    }
    b, _ := json.MarshalIndent(env, "", "  ")
    fmt.Println(string(b))
    os.Exit(exitCode)
}

// printJSONError 输出错误信封
func printJSONError(cmd, code, message string, exitCode int) {
    env := CLIEnvelope{
        OK: false, Command: cmd,
        Error: &CLIError{Code: code, Message: message},
        ExitCode: exitCode,
    }
    b, _ := json.MarshalIndent(env, "", "  ")
    fmt.Println(string(b))
    os.Exit(exitCode)
}

// printTable 人类可读输出（--format table 时）
func printTable(format string, args ...interface{}) {
    fmt.Fprintf(os.Stderr, format, args...)  // 进度信息走 stderr
}
```

**关键规则**：
- `--format json` 时 stdout 只有 JSON，进度/警告走 stderr
- `--format table`（默认）时保持现有人类输出
- 所有 `fmt.Printf` 进度信息改为 `fmt.Fprintf(os.Stderr, ...)`

---

## 二、退出码规范

| 退出码 | 含义 | Agent 动作 |
|---|---|---|
| 0 | 成功 | 解析 data |
| 1 | 查询执行失败（引擎返回错误） | 读 error.message，可换引擎重试 |
| 2 | 认证失败（API Key/Cookie 无效） | 提示用户更新凭据 |
| 3 | 无可用引擎 | 提示用户配置引擎 |
| 4 | 参数错误 | 修正参数 |
| 5 | Web API 服务不可达（API 子命令） | 提示启动 unimap-web |
| 6 | 超时 | 增大 --timeout 或缩小查询范围 |

### 实施

替换所有 `os.Exit(1)` 为语义化退出码。在 `main.go` 和 `api_subcommands.go` 中：

```go
// Before
fmt.Fprintf(os.Stderr, "Error: %v\n", err)
os.Exit(1)

// After
if isAuthError(err) {
    printJSONError("query", "AUTH_FAILED", err.Error(), ExitAuthError)
} else {
    printJSONError("query", "QUERY_FAILED", err.Error(), ExitQueryError)
}
```

---

## 三、分页支持

### 直接查询模式

```bash
unimap-cli -q 'port="443"' -e fofa --page 1 --page-size 100 --format json
```

新增 flag：
- `--page N`（默认 1）
- `--page-size N`（默认 100，替代现有 `-l`，`-l` 保留为别名）

JSON 输出 data 中包含：
```json
{"page": 1, "page_size": 100, "total": 1234, "has_more": true}
```

Agent 迭代模式：
```bash
page=1
while true; do
  result=$(unimap-cli -q "$query" --page $page --format json)
  has_more=$(echo "$result" | jq -r '.data.has_more')
  # 处理 .data.assets ...
  [ "$has_more" = "true" ] || break
  page=$((page + 1))
done
```

### API 子命令

`query` 子命令同步增加 `--page` 和 `--page-size`，透传给 Web API。

---

## 四、所有子命令统一 `--format`

### 当前状态

| 子命令 | 支持 --format | 输出格式 |
|---|---|---|
| 直接查询 | ✅ json/table | 已实现 |
| `query` | ❌ | 人类文本 |
| `tamper-check` | ❌ | 人类文本 |
| `screenshot-batch` | ❌ | 人类文本 |
| `scheduler *` | ❌ | 人类文本 |
| `engines` | ❌ | 人类文本 |

### 目标

所有子命令增加 `--format json|table`（默认 table）。JSON 模式使用统一信封。

**实施模式**（以 `engines` 为例）：

```go
func runEnginesCommand(args []string) {
    fs := flag.NewFlagSet("engines", flag.ExitOnError)
    configPath := fs.String("c", utils.DefaultConfigPath(), "Config file path")
    format := fs.String("format", "table", "Output format: table or json")
    _ = fs.Parse(args)
    
    // ... 收集 engines 数据 ...
    
    if *format == "json" {
        printJSON("engines", engines, ExitOK)
        return
    }
    // 原有 table 输出
}
```

---

## 五、`quota` 子命令

```bash
unimap-cli quota [--engine fofa] [--format json]
```

输出：
```json
{
  "ok": true,
  "command": "quota",
  "data": {
    "engines": [
      {"engine": "fofa", "remaining": 4923, "total": 5000, "unit": "queries/day"},
      {"engine": "hunter", "remaining": 0, "total": 100, "unit": "queries/day"},
      {"engine": "censys", "remaining": -1, "total": -1, "unit": "unavailable"}
    ]
  }
}
```

`remaining: -1` 表示引擎不支持配额查询。

### 实施

调用各适配器的 `GetQuota()` 方法（已存在于 adapter 接口）。

---

## 六、`config show` 子命令

```bash
unimap-cli config show [--format json] [--show-secrets]
```

默认隐藏敏感值（API Key 只显示前 4 位 + `****`），`--show-secrets` 显示完整值。

```json
{
  "ok": true,
  "command": "config",
  "data": {
    "config_path": "/home/user/.unimap/config.yaml",
    "engines": {
      "fofa": {"enabled": true, "api_key": "abcd****", "base_url": "https://fofa.info"},
      "shodan": {"enabled": false, "api_key": "", "base_url": "https://api.shodan.io"}
    },
    "screenshot": {"mode": "cdp", "base_dir": "..."},
    "scheduler": {"enabled": true}
  }
}
```

---

## 七、`--fields` 列选择

```bash
unimap-cli -q 'port="443"' --fields ip,port,title,country --format json
```

JSON 输出中 assets 只包含指定字段：
```json
{"assets": [{"ip": "8.8.8.8", "port": 53, "title": "DNS", "country_code": "US"}]}
```

CSV 输出只包含指定列。

### 实施

在 `outputCLIResults` 中，如果 `fields != ""`，解析为 `[]string`，
对每个 asset 用反射或手动映射只保留指定字段。

推荐手动映射（避免反射）：

```go
var assetFieldMap = map[string]func(model.UnifiedAsset) string{
    "ip":       func(a model.UnifiedAsset) string { return a.IP },
    "port":     func(a model.UnifiedAsset) string { return strconv.Itoa(a.Port) },
    "protocol": func(a model.UnifiedAsset) string { return a.Protocol },
    "host":     func(a model.UnifiedAsset) string { return a.Host },
    "title":    func(a model.UnifiedAsset) string { return a.Title },
    "country":  func(a model.UnifiedAsset) string { return a.CountryCode },
    "city":     func(a model.UnifiedAsset) string { return a.City },
    "org":      func(a model.UnifiedAsset) string { return a.Org },
    "server":   func(a model.UnifiedAsset) string { return a.Server },
    "url":      func(a model.UnifiedAsset) string { return a.URL },
    "source":   func(a model.UnifiedAsset) string { return a.Source },
}
```

---

## 八、`help --json` 自描述

```bash
unimap-cli help --json
```

输出所有子命令和参数的机器可读 schema：

```json
{
  "name": "unimap-cli",
  "version": "1.2.0",
  "commands": [
    {
      "name": "query",
      "description": "Direct query via API adapters",
      "flags": [
        {"name": "q", "type": "string", "required": true, "description": "UQL query"},
        {"name": "e", "type": "string", "required": false, "description": "Comma-separated engines"},
        {"name": "page", "type": "int", "default": 1},
        {"name": "format", "type": "enum", "values": ["table", "json"], "default": "table"}
      ]
    },
    {
      "name": "screenshot-batch",
      "description": "Batch screenshot via Web API",
      "flags": [...]
    }
  ],
  "exit_codes": {
    "0": "success",
    "1": "query error",
    "2": "auth error",
    "3": "no engines",
    "4": "usage error",
    "5": "server unreachable",
    "6": "timeout"
  }
}
```

Agent 启动时调用一次即可获知所有能力。

---

## 九、环境变量支持

Agent 通常通过环境变量传递凭据，避免命令行泄露：

| 环境变量 | 用途 | 已有 |
|---|---|---|
| `UNIMAP_ADMIN_TOKEN` | Web API 认证 | ✅ |
| `UNIMAP_CONFIG_PATH` | 配置文件路径 | ❌ 新增 |
| `UNIMAP_API_BASE` | Web API 地址 | ❌ 新增 |
| `UNIMAP_FORMAT` | 默认输出格式 | ❌ 新增 |
| `UNIMAP_FOFA_API_KEY` | FOFA API Key | ❌ 新增 |
| `UNIMAP_SHODAN_API_KEY` | Shodan API Key | ❌ 新增 |
| `UNIMAP_HUNTER_API_KEY` | Hunter API Key | ❌ 新增 |
| `UNIMAP_QUAKE_API_KEY` | Quake API Key | ❌ 新增 |
| `UNIMAP_ZOOMEYE_API_KEY` | ZoomEye API Key | ❌ 新增 |
| `UNIMAP_CENSYS_API_ID` | Censys API ID | ❌ 新增 |
| `UNIMAP_CENSYS_API_SECRET` | Censys API Secret | ❌ 新增 |
| `UNIMAP_DAYDAYMAP_API_KEY` | DayDayMap API Key | ❌ 新增 |

优先级：flag > 环境变量 > 配置文件。

---

## 十、实施顺序

| 阶段 | 内容 | 文件 | 预估 |
|---|---|---|---|
| **P0** | JSON 信封 + 退出码 + stderr 分离 | `output.go`(新), `main.go`, `api_subcommands.go` | 1h |
| **P1** | 分页 `--page`/`--page-size` | `main.go`, `api_subcommands.go` | 30min |
| **P1** | 所有子命令 `--format json` | `api_subcommands.go`, `main.go` | 1h |
| **P2** | `quota` 子命令 | `main.go` 或 `quota.go`(新) | 30min |
| **P2** | `config show` 子命令 | `config_cmd.go`(新) | 30min |
| **P2** | `--fields` 列选择 | `main.go`, `output.go` | 30min |
| **P3** | 环境变量支持 | `main.go`, `api_subcommands.go` | 30min |
| **P3** | `help --json` 自描述 | `help.go`(新) | 30min |

P0+P1 完成后 Agent 即可可靠使用。P2/P3 是便利性增强。

---

## Agent 调用示例（P0+P1 完成后）

```bash
# 1. 检查引擎状态
unimap-cli engines --format json | jq '.data[] | select(.has_api_key)'

# 2. 查询第一页
result=$(unimap-cli -q 'app="nginx" && country="CN"' -e fofa --page 1 --format json)
echo "$result" | jq '.data.assets | length'    # 本页条数
echo "$result" | jq '.data.total'              # 总数
echo "$result" | jq '.data.has_more'           # 是否有下一页

# 3. 自动翻页
page=1
while true; do
  r=$(unimap-cli -q "$q" --page $page --format json 2>/dev/null)
  [ "$(echo "$r" | jq '.ok')" = "true" ] || break
  echo "$r" | jq -c '.data.assets[]' >> results.jsonl
  [ "$(echo "$r" | jq '.data.has_more')" = "true" ] || break
  page=$((page + 1))
done

# 4. 截图（需要 Web 服务运行中）
unimap-cli screenshot-batch -urls targets.txt --format json | jq '.data.success'

# 5. 错误处理
unimap-cli -q 'invalid%%%' --format json 2>/dev/null
echo $?  # 1 = 查询错误, 2 = 认证失败, 3 = 无引擎
```
