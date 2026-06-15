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
  history/             操作历史持久化 (SQLite, 通用操作日志)
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
1. ~~10 个文件超 800 行~~ ✅ 已全部拆分完成（最大 `metrics.go` 795 行）
2. ~~34 个函数超 50 行~~ ✅ 已拆分完成（191→144，最大 `plugin_demo.go:main` 183 行为示例文件）

### Low (后续迭代修复)
7. ~~**L-01** 错误消息大写~~ ✅ 已修复（2 处非缩写词改为小写，其余 21 处为 HTTP/API/ICP 等缩写词可接受）
8. ~~**L-05** `map[string]interface{}` 强类型~~ ✅ 已完成 Phase 1-5（799→227，减少 72%），剩余为低优先级 Web 响应负载和测试文件

### 2026-06-10 复核记录
- ✅ 批量 URL 截图改为异步 job + progress 查询；无效/内网 URL 作为单条 failed 结果返回，不再拒绝整个批次。
- ✅ 批量截图 Provider 增加逐项进度回调；CDP/Extension provider 均支持完成一条即更新 job 进度。
- ✅ `main.js` API 请求统一走 `apiFetch`；CSV/JSON 导出基于完整结果数组，不再只导出当前 DOM 页。
- ⏸ 仍剩长期项：L-05/TD-4 强类型渐进重构；L2 Hook。
- 📝 剩余长期项已补充评估与实施文档：`docs/ENGINE_ADAPTER_IMPLEMENTATION_PLAN.md` §7（TD-4/L2 Hook/stealth 总评估）与 `docs/EXTENSION_ANTI_SCRAPING_ARCHITECTURE.md` §11（stealth 执行方案）。

### 2026-06-11 安全与稳定性修复（19 项）
- ✅ **CRITICAL** `batchJobStore.get()` → `getSnapshot()` 深拷贝，消除 progress handler 与后台 goroutine 数据竞争。
- ✅ **HIGH** 批量截图后台 goroutine 使用 `s.shutdownCtx`，服务器关闭时正确取消。
- ✅ **HIGH** `BridgeService` worker 添加 `executeJobSafely` panic recovery，单 job panic 不杀死 worker。
- ✅ **P0** `monitor.html` 6 处 stored XSS 转义（`escapeHtml`）；`batch-screenshot.html` XSS 转义。
- ✅ **P1** `ResourceMonitor.Stop()` 改为 `sync.Once` 幂等化；WS `JSON.parse` 添加 try/catch。
- ✅ **P1** 所有 fetch 调用统一 `parseJsonResponse`（`resp.ok` 检查）；`adminToken()` auto-generate 持久化。
- ✅ **P1** `ScreenshotAppService` setters 加 `sync.RWMutex`，bridge 方法使用 `configSnapshot()` 快照。
- ✅ **P1** 批量截图 metrics success/partial 互斥；`handleSetScreenshotMode` config save 日志。
- ✅ **P1** `cleanupStaleBatchJobs` 定期清理 goroutine；前端 progress polling 失败停止 + 超时反馈。
- ✅ **P2** `cmd/unimap-web/main.go` 添加 `defer logger.Sync()`；`.dockerignore` 排除 `configs/config.yaml`。
- ✅ 死代码清理：`updateProgress` 方法、TOCTOU nil guard。
- ✅ 新增 5 个测试：batchJobStore cleanup、nil store、classify URLs、merge results、WS token rejection。
- ⏸ 剩余：L-05/TD-4 强类型渐进重构；L2 Hook。

### 2026-06-11 后续修复
- ✅ 所有管理端点添加限流保护（cookies、CDP、bridge、nodes、scheduler、notifications、tamper、config、backup、user 等）。
- ✅ Auth 测试覆盖率从 76.5% 提升至 94.7%（新增 user_db_test.go）。
- ✅ CSP 配置移除非官方 Google 域名（`fonts.googleapis.font.im`/`fonts.gstatic.font.im`）。
- ✅ `handleScreenshot` 改为通过 `ScreenshotRouter.CaptureTargetWebsite` 执行，移除直接 chromedp 调用，统一走 Router 降级链路。
- ✅ CSP `unsafe-inline` 完全移除：settings.html style 加 nonce，21处静态 inline style 迁移为 CSS 类（utils.css），28处 JS template literal inline style 迁移为 CSS 类，动态颜色用 CSS 变量。
- ✅ server.go 拆分：提取 middleware_security.go（安全中间件+CSP，38行）和 server_helpers.go（解析/验证工具函数，165行），从 1335 行降至 1128 行。
- ✅ 硬编码路径检查：所有 hardcoded 路径均在 test 文件中，生产代码无硬编码路径。
- ✅ go.mod 已为 go 1.26，与安装版本匹配。

### 2026-06-11 P0 Bridge 状态修复 + Token 复制 + 状态抖动
- ✅ **P0** Bridge/CDP 状态语义统一：`ExtensionHealthChecker` 要求 `LiveClient` 返回 true；`buildBridgeDiagnosticSnapshot` 新增 `extension_online` 字段；`ready` 逻辑修正为 `engine == "cdp" || (engine == "extension" && extensionOnline)`。
- ✅ **P0** `router.extHealthy` 初始值改为 `false`（不再用 `extBridge != nil`）；`SetExtensionHealthSignals` 总是设置 `LiveClient`（nil 也设）。
- ✅ **P0** 设置页 Bridge 状态三态显示：在线（`extension_online || router_ext_healthy || live_clients > 0`）/ 等待扩展连接（`bridge_connected` 但无活跃客户端）/ 离线。
- ✅ **P1** Bridge 状态抖动修复：`liveWindowSeconds` 从 15 秒提高到 60 秒，覆盖扩展执行任务（10-25 秒）期间不轮询的场景。
- ✅ **P1** Account 页 Token 复制修复：`GET /api/v1/account/admin-token` 返回真实 token（接口已受 auth 保护），不再返回 `maskAPIKey` 脱敏值。
- ✅ 新增 7 个测试 + 修复 4 个已有测试适配新语义；`go test -race ./internal/screenshot/... ./web/...` 全部通过。
- ⏸ 仍剩：新增引擎端到端未闭环（UI/凭据/cookie/浏览器链路）；L2 Hook。

### 2026-06-12 TD-4/L-05 强类型重构 + 操作历史持久化
- ✅ **TD-4/L-05** `map[string]interface{}` 强类型渐进重构 Phase 1-5 完成：799→227 处（减少 72%）
  - Phase 1: 定义边界结构体（PluginConfig/HookData/TaskPayload/BridgeCollectedData/APIResponse）
  - Phase 2: Plugin.Initialize/HookFunc/HealthStatus.Details/NotificationMessage.Metadata → 强类型
  - Phase 3: ScheduledTask.Payload/TaskTemplate.Payload/TaskHandler.Execute → *model.TaskPayload
  - Phase 4: TaskEnvelope.Payload/TaskResult.Output/TaskRecord.Payload/Result → 强类型
  - Phase 5: BridgeResult.StructuredCollectedData → *model.BridgeCollectedData
  - Phase 6: Web API handlers (user/notification/node) → model.APIResponse
- ✅ **BUG FIX** PageSizeICP JSON tag 冲突（`json:"page_size"` → `json:"icp_page_size"`），修复 ICP 默认 page_size=40 被静默丢弃
- ✅ **UQL 查询历史服务端持久化**：新增 `internal/history/` 包，SQLite 存储操作历史+结果
  - 支持多类型查询：query/icp_query/port_scan/screenshot/tamper_check
  - API: POST/GET/DELETE `/api/v1/history`
  - 前端自动保存查询结果，历史列表从 localStorage 迁移到服务端 API

### 2026-06-15 Quake采集验证更新
- ✅ **Quake采集成功**：URL格式修正后（`.cn`→`.net` + 参数补充），Extension采集成功（10 items, card_fallback方法）
- ✅ **5引擎全部打通**：FOFA/ZoomEye/Shodan/Hunter/Quake均验证成功
- ✅ **根因澄清**：Quake之前是URL格式过期，不是反爬拦截（手动查询正常证明账号权限）

### 2026-06-15 ZoomEye选择器更新 + web包测试覆盖率提升
- ✅ **ZoomEye DOM选择器更新**：根据保存的HTML页面分析，更新`capture.js`和`dom_selectors.go`中的ZoomEye选择器
  - 旧：`table.table-condensed tbody tr` → 新：`div.search-result-item-container`
  - IP提取：`span._public-hover_uxlu6_1` + `div.url-container span`
  - 端口/协议：`div.protocol-port-box button`
  - 分页：`li.ant-pagination-next:not(.ant-pagination-disabled) a`
  - 总数：`li.ant-pagination-total-text span`
- ✅ **ZoomEye WebOnly采集验证**：选择器更新后Extension采集成功（10条数据，字段提取正常）
- ✅ **web包测试覆盖率**：42.6% → 54.6%（+12%），新增13个测试文件
  - `login_handlers_test.go` — 12个测试
  - `user_handlers_test.go` — 25个测试 + mockUserRepo
  - `session_test.go` — 30+个测试（加密/session/CSRF）
  - `config_handlers_test.go` — 15+个engine字段测试
  - `http_helpers_extended_test.go` — 20+个测试
  - `middleware_extended_test.go` — 25+个测试
  - `notification_handlers_test.go` — 15+个测试
  - `query_handlers_extended_test.go` — 8个测试
  - `bridge_helpers_test.go` — 20+个测试（图片处理/路径构建）
  - `server_helpers_extended_test.go` — 15+个测试（模板函数/配置）
  - `cookie_helpers_extended_test.go` — 10+个测试
  - `screenshot_bridge_mock_test.go` — 15+个测试
  - `websocket_helpers_test.go` — 8个测试

### 2026-06-15 截图SPA渲染时机修复
- ✅ **Extension screenshot action 加 SPA 等待**：`background.js` 中 screenshot 和 collect action 都加 4000ms 等待
- ✅ **Go Bridge 调用改用 `"spa"` 策略**：`router_extension.go` 中搜索结果页截图/采集/打开全部从 `"load"` 改为 `"spa"`（~8s 等待 vs ~3s）
- ✅ **新增 Extension `collect_and_capture` action**：一次导航中同时完成数据采集+截图，避免分步调用导致的页面重载问题
- ✅ **Go `CollectAndCaptureSearchEngineResult` 重构**：Extension 模式不再降级为分步调用，直接使用 `collect_and_capture` action
- 问题根因：SPA 引擎（FOFA/Hunter/ZoomEye/Quake）搜索结果异步渲染，截图需等待 5-8 秒

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
