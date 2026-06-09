# UniMap — 多引擎网络空间资产查询与网页监控工具

> 当前分支：`release/major-upgrade-vNEXT` | Go 1.26 | 主链路：`go build ./...`、`go test ./...`、`go test -race ./...` 均通过

## 项目概述

多引擎统一查询平台，支持 **FOFA、Hunter、ZoomEye、Quake、Shodan** 五大搜索引擎，提供 Web / CLI / GUI 三种入口。核心能力：资产查询、截图监控、篡改检测、定时任务、分布式节点、告警通知。

## 技术栈

| 类别 | 技术 |
|------|------|
| 语言 | Go 1.26 |
| Web | `net/http` + gorilla/websocket + go-resty |
| GUI | Fyne v2 |
| CLI | Cobra |
| 浏览器自动化 | chromedp (CDP protocol) |
| 定时任务 | robfig/cron/v3 (20 种 Runner) |
| 缓存 | 内存 + Redis (go-redis/v9) |
| 存储 | SQLite, YAML |
| HTML 解析 | goquery + goquery/b luemonday |
| 导出 | excelize (Excel) |
| 监控 | Prometheus client_golang |
| 日志 | zap (异步写入, 动态级别) |
| 部署 | Docker (alpine:3.21, 非 root 用户) |
| CI | GitHub Actions (test + lint + race + security + Docker push) |

## 目录结构

```
cmd/
  unimap-cli/          CLI 入口
  unimap-gui/          GUI 入口 (fyne, -tags gui)
  unimap-web/          Web 入口 (:8448)
internal/
  adapter/             引擎适配 (fofa/hunter/zoomeye/quake/shodan)
  alerting/            告警管理 (Webhook/Log, 去重/静默/频率控制)
  auth/                API Key + 权限管理
  backup/              数据备份 (基线/配置/Cookie)
  config/              配置管理 + 热更新
  core/unimap/         UQL 解析与结果归并
  distributed/         分布式节点 (注册/心跳/任务队列/故障转移)
  error/               统一错误类型
  exporter/            数据导出
  logger/              日志系统 (zap, 异步, 动态级别)
  metrics/             Prometheus 指标
  model/               数据模型
  monitoring/          资源监控 + 泄漏检测
  plugin/              插件系统与处理管道
  proxypool/           代理池
  requestid/           请求 ID 生成
  scheduler/           定时任务 (cron + 20 Runner + 持久化)
  screenshot/          截图 (CDP/Extension/ScreenshotRouter 双模式高可用)
  service/             统一服务层
  tamper/              网页篡改检测 (5 种模式)
  utils/               通用工具
web/
  server.go            Web 服务 + 路由 (73 API + 17 非 API = ~90 mux 条目)
  templates/           Go 页面模板
  static/              前端静态资源
  middleware_*.go      中间件 (auth/ratelimit/requestid/audit)
  *_handlers.go        HTTP handlers (按功能域分文件)
configs/
  config.yaml          当前配置
  config.yaml.example  示例配置
docs/
  ARCHITECTURE.md      分层架构 + 数据流向
  RUNBOOK.md           运维故障处理 (6 场景)
  grafana-dashboard.json  Grafana 面板 (7 面板)
  PRODUCTION_READINESS_PLAN.md  生产就绪清单
  API.md               API 文档
  API_VERSIONING.md    API 版本化方案
  PLUGIN_DEVELOPMENT_GUIDE.md  插件开发指南
memory/
  MEMORY.md            项目记忆索引
```

## 快速启动

```bash
# 1. 配置
cp configs/config.yaml.example configs/config.yaml
# 编辑 configs/config.yaml — 填入 API Keys

# 2. 启动 Web
go run ./cmd/unimap-web         # http://localhost:8448

# 3. CLI 查询
go run ./cmd/unimap-cli -q 'country="CN" && port="80"' -e fofa,hunter -l 100

# 4. GUI
go run -tags gui ./cmd/unimap-gui
```

### 关键配置参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `system.max_concurrent` | 查询并发上限 | 10 |
| `system.cache_ttl` | 缓存 TTL (秒) | 3600 |
| `web.port` | Web 端口 | 8448 |
| `web.auth.enabled` | 启用 Admin Token 鉴权 | false |
| `web.auth.admin_token` | 管理密钥 | "" (支持 `${ENV_VAR}`) |
| `screenshot.engine` | 截图引擎 | `cdp` 或 `extension` |
| `rate_limit.enabled` | API 限流 | false (⚠️ 应改为 true) |

## 核心功能

### 多引擎统一查询 (UQL)
- 统一语法，自动翻译为各引擎查询语言
- 结果归并去重，支持 CSV/Excel/JSON 导出
- 内存缓存 + Redis 缓存，可配置 TTL

### 截图高可用 (ScreenshotRouter)
```
请求 → ScreenshotRouter
       ├── CDP 模式 (健康 → 使用，失败 → 降级 Extension)
       └── Extension 模式 (健康 → 使用，失败 → 降级 CDP)
```
- HealthChecker 定期探测 CDP (`/json/version`) 和 Bridge 状态
- 自动降级，无需人工干预

### 篡改检测 (5 种模式)
| 模式 | 说明 |
|------|------|
| strict | 任何内容变化都告警 |
| relaxed | 忽略非关键区域变化 |
| malicious | 仅检测恶意脚本/可疑内容 |
| performance | 快速 HTTP 模式，跳过渲染 |
| full | 完整浏览器渲染 + 所有检测 |

### 定时任务系统 (20 种 Runner)
- Cron 表达式创建、启停、编辑、删除
- 执行历史追踪、持久化
- 高优先级：UQL 查询、截图、篡改检测、URL 检测、Cookie 验证
- 中优先级：数据导出、端口扫描、缓存预热、配额监控
- 低优先级：插件健康检查、告警静默

### 分布式节点
- 节点注册 / 心跳 / 任务领取 / 故障转移
- 快照持久化，支持节点恢复

### 告警系统
- 通知渠道：Webhook + Log
- 功能：阈值检测、去重、静默窗口、频率控制
- 关键指标告警阈值：

| 指标 | 警告 | 严重 |
|------|------|------|
| 查询 P95 延迟 | > 30s | > 60s |
| 缓存命中率 | < 50% | < 20% |
| 截图成功率 | < 90% | < 70% |
| 节点在线率 | < 80% | < 50% |
| Goroutine 数 | > 1000 | > 5000 |
| 磁盘使用 | > 80% | > 90% |

## 代码规范

### Go 风格
- `gofmt` + `goimports` 必须
- Accept interfaces, return structs
- 小接口 (1-3 方法)
- 错误包装：`fmt.Errorf("context: %w", err)`
- 函数 < 50 行，文件 < 800 行
- 不可变优先，避免就地修改

### 测试
- Table-driven tests
- **始终加 `-race` 标志**：`go test -race ./...`
- 覆盖率 >= 80%
- AAA 模式 (Arrange-Act-Assert)
- 测试命名：`test('returns empty array when no markets match query', ...)`

### 安全
- 禁止硬编码密钥 — 使用环境变量或 `config.yaml`
- 所有用户输入必须验证
- SQL 参数化查询
- HTML 使用 `html/template` ✅
- CSP 配置，避免 `'unsafe-eval'` ✅

## 已知待修复事项

> 来源：2026-05-09 全量代码扫描，详见 `memory/project_remaining_issues_2026-05-09.md`

### ✅ 已全部修复
- C-01 ~ C-04 (Critical)、H-01 ~ H-05 (High) — 全部闭环
- M-02 ~ M-06, M-08, M-09 (Medium) — 全部闭环
- L-02, L-03 (Low) — CORS 死代码已清理、Scheduler CSP nonce 已添加

### High (建议合并前修复)
无

### Medium (后续迭代修复)
1. 5 个文件超 800 行 (最大 `screenshot/manager.go` 1189 行；原 10 个已拆分 5 个)
2. 34 个函数超 50 行 (最大 `createMonitorTab` 390 行)

### Low (后续迭代修复)
7. **L-01** 错误消息大写 (23 处，多数为缩写词可接受)
8. **L-05** `map[string]interface{}` 强类型 (插件接口等广泛使用，渐进重构)

## 常用命令

```bash
# 代码检查
go vet ./...
go test -race ./...

# 覆盖率
go test -cover ./...
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

# 格式化
gofmt -w .
goimports -w .

# 构建
go build ./...
go build -tags gui ./cmd/unimap-gui

# 安全扫描
gosec ./...
```

## Skills 使用指南

**Go 代码相关（自动触发）：**
- `golang-patterns` — Go 惯用法、并发、错误处理、包组织
- `golang-testing` — table-driven tests、race detection、coverage
- `go-build` — 构建失败时
- `go-review` — 编写/修改 Go 代码后
- `go-test` — 运行和调试测试

**通用任务：**
- `code-review` — 通用代码审查工作流
- `security-review` — 认证、输入处理、外部 API 变更
- `tdd` — 新功能或 Bug 修复
- `e2e` — 关键用户流程测试
- `docker-patterns` — Dockerfile 或 compose 变更

> Skills 按需懒加载，不调用不消耗 token。避免一次性加载所有 skills。

## 记忆系统

项目记忆索引：`memory/MEMORY.md`
详细记忆文件：`memory/project_*.md`

每次开始工作前检查记忆了解：
- 已完成的工作和修复历史
- 已知 Bug 和技术债务
- 测试覆盖率进展
- Code Review 发现

## Git 工作流

- 分支策略：`master` (稳定) ← `release/major-upgrade-vNEXT` (开发)
- 提交格式：`<type>: <description>` (feat/fix/refactor/docs/test/chore)
- PR 前要求：`go test -race ./...` 通过，覆盖率达标，code review 完成

## 运维文档

| 文档 | 路径 |
|------|------|
| 架构文档 | `docs/ARCHITECTURE.md` |
| 运维 Runbook | `docs/RUNBOOK.md` (6 场景故障处理) |
| Grafana 面板 | `docs/grafana-dashboard.json` (7 面板) |
| 生产就绪清单 | `docs/PRODUCTION_READINESS_PLAN.md` |
| API 文档 | `docs/API.md` |
| API 版本化方案 | `docs/API_VERSIONING.md` |
| 插件开发指南 | `docs/PLUGIN_DEVELOPMENT_GUIDE.md` |
| 插件桥接运维 | `docs/OPS_SCREENSHOT_EXTENSION.md` |
