# UniMap 变更日志

---

## [2026-06-30] 巡检功能逻辑缺陷修复与业务完善

> **变更类型**: Bug 修复 + 业务完善
> **涉及模块**: tamper、scheduler、service/monitor、web、templates
> **提交**: `2bf5fe5` (develop) — PR #11 CI 全绿（ci + bridge-smoke，ubuntu/macos 双平台）

### 背景
对巡检子系统（篡改检测 + URL 可达性 + 批量截图 + 端口扫描 + ICP 巡检等定时监控任务）做全量审计，发现 4 个确认 Bug 与 4 个完善性缺口。

### 修复（Tier 1 — Bug）

- **篡改检测模式被强制降级**：service 层 `if mode != strict { mode = relaxed }` 把 `security/balanced/precise` 静默降级为 `relaxed`，UI 与定时任务的 5 模式选择形同虚设。导出 `NormalizeDetectionMode`，service 层改用它，5 模式全部透传。
- **每日篡改模板无效模式**：`tmpl_daily_tamper_check` 的 `DetectMode: "full"` 不是合法检测模式，会被降级。改为 `"relaxed"`。
- **定时篡改巡检不渲染 JS**：`TamperCheckRunner` 注册时传 nil allocator，只能走 HTTP/Fast 模式，SPA/JS 页面拿空 hash → 误报"篡改"或"不可达"。从 `screenshotMgr` 注入 allocator（nil 时保持原行为）。
- **SSRF 拦截的 URL 未计入可达性统计**：汇总 switch 缺 `blocked` 分支，有 SSRF 拦截时 `Total ≠ Reachable+Unreachable+InvalidFormat`。新增 `Blocked` 字段与分支，`Total = Reachable+Unreachable+InvalidFormat+Blocked`。

### 完善（Tier 2）

- **基线刷新并发化**：`BaselineRefreshRunner` 原逐 URL 串行 `SetBaseline`，忽略 payload 的 `concurrency`。改为信号量 + WaitGroup 并发执行。
- **"查看基线" UI 入口**：`monitor.html` 新增 `btn-list-baseline` 按钮，绑定既有的 `loadBaselineList()`。
- **历史记录类型过滤补全**：`monitor.html` 的 `history-type-filter` 追加 `no_baseline/unreachable/suspicious` 三个选项。
- **scheduler 帮助文档模式修正**：`scheduler.html` 的 `PARAM_HINTS.tamper_check` 由错误的 `malicious/performance/full` 改为 `security/balanced/precise`。

### 涉及文件

| 文件 | 变更 |
|------|------|
| `internal/tamper/detector_types.go` | 导出 `NormalizeDetectionMode` |
| `internal/tamper/detector_test.go` | 模式用例补全 security/balanced/precise |
| `internal/service/tamper_app_service.go` | 模式透传（调用 `NormalizeDetectionMode`） |
| `internal/service/monitor_app_service.go` | 抽取 `summarizeReachability` + 新增 `Blocked` 桶 |
| `internal/service/monitor_app_service_test.go` | 汇总不变量单测（新增） |
| `internal/scheduler/scheduler_types.go` | 模板模式 `full` → `relaxed` |
| `internal/scheduler/executor_runners2.go` | 基线刷新并发化 |
| `web/server.go` | 定时篡改 allocator 注入 |
| `web/templates/monitor.html` | 查看基线按钮 + 历史过滤补全 |
| `web/templates/scheduler.html` | 帮助文档模式修正 |

### 列为后续（架构级，本次未修）
可达性/端口扫描告警记录缺失；告警与资源监控仅存内存重启丢失；`operation_history` 无内部写入者；定时批量截图未进 batchdb；篡改导出器死代码；无定时备份任务；篡改记录文件式无索引。

### 验证结果
- `go build ./...` 通过
- `go test ./internal/tamper/... ./internal/service/... ./internal/scheduler/...` 全部通过
- `go vet` + `gofmt` 干净
- GitHub Actions CI 全绿

---

## [2026-06-11] Bridge 状态语义修复 + Token 复制 + 状态抖动

### 修复

- **Bridge/CDP 状态语义统一**：`ExtensionHealthChecker` 要求 `LiveClient` 返回 true 才报告健康；`buildBridgeDiagnosticSnapshot` 新增 `extension_online` 字段；设置页不再将"服务启动"显示为"在线"，改为三态：在线 / 等待扩展连接 / 离线。
- **Bridge 状态抖动修复**：`liveWindowSeconds` 从 15 秒提高到 60 秒，覆盖扩展执行任务期间不轮询的场景。
- **Account 页 Token 复制修复**：`GET /api/v1/account/admin-token` 返回真实 token（接口已受 auth 保护），不再返回脱敏值。

### 涉及文件

| 文件 | 变更 |
|------|------|
| `internal/screenshot/health.go` | `LiveClient == nil` → 返回 `false` |
| `internal/screenshot/router.go` | `extHealthy` 初始 `false`；`LiveClient` nil 也赋值 |
| `web/server.go` | `LiveClient` 回调改用 live token 检查 |
| `web/screenshot_bridge_handlers.go` | 新增 `extension_online`；`liveWindowSeconds` 15→60；提取共享函数 |
| `web/query_handlers.go` | admin token API 返回真实 token |
| `web/templates/settings.html` | Bridge 状态三态显示 |

### 测试

- 新增 7 个测试 + 修复 4 个已有测试适配新语义
- `go test -race ./internal/screenshot/... ./web/...` 全部通过

---

## [1.0.0] - 2026-05-26 生产级就绪正式版发布 (Major Upgrade)

> **变更类型**: 核心架构升级 & 正式发布
> **涉及模块**: CLI / Web UI / GUI / 核心引擎架构 / 全量系统

### 🎉 新增功能
- **全系引擎全量支持**: 深入集成并解耦 FOFA / Hunter / ZoomEye / Quake / Shodan 五大搜索引擎底座。
- **Screenshot 截图容灾**: 引入核心高可用 ScreenshotRouter 组件，实现 CDP 与 Extension 双擎自动探测及故障切换备份策略。
- **多端业务矩阵**: 重构分离出独立的 Web API 守护进程、CLI 终端查询工作流及 GUI 图形桌面端，业务矩阵全面闭环。
- **智能化篡改防护 (Tamper Detection)**: 新增网页动态区域特征隔离算法及针对不同颗粒度防护的五模监控匹配引擎。
- **分布式调度阵列 (Scheduler Node Cluster)**: 搭建基于 LRU/Redis 持久化的轻量级多任务心跳管理及 Task 自动分发阵列（涵盖20种专属定时检查 Runner）。
- **企业级告警通道**: 原生实现频率风暴限制、重复异常静默期管理的 Log/Webhook 推送集成。

### 🔄 优化与重构
- 统领 UQL（Unified Query Language）抽象语法字典及各引擎底层的自动转译机制。
- 完整脱敏开源仓库信息，统一合规及安全审查路径，升级并补全全部产品技术手册文档库 (docs/)。

---

## [2026-05-07] 引擎加载问题修复 — 空间搜索引擎默认启用

> **变更类型**: Bug 修复
> **涉及模块**: config、cmd/unimap-web

### 问题描述
当用户输入查询语句执行查询时，由于未加载空间引擎，无法执行查询。

### 问题根源
在 `internal/config/config.go` 中的 `applyDefaults` 函数未设置引擎的 `Enabled` 字段默认值，导致所有引擎默认都是禁用状态。

### 修复方案
修改 `internal/config/config.go` 中的 `applyDefaults` 函数：
- **默认启用所有搜索引擎** — Quake、ZoomEye、Hunter、FOFA、Shodan
- **设置 FOFA 为 Web 模式为默认** — `UseWebAPI = true`，即使没有 API Key 也能使用 Web 模式
- **保持验证函数的完整性** — 非 Web 模式下仍需验证 API Key

### 修改的关键代码
```go
// 默认启用所有引擎
config.Engines.Quake.Enabled = true
config.Engines.Zoomeye.Enabled = true
config.Engines.Hunter.Enabled = true
config.Engines.Fofa.Enabled = true
config.Engines.Shodan.Enabled = true
config.Engines.Fofa.UseWebAPI = true
```

### 验证结果
- `go build ./...` — 构建成功
- `go test -v ./internal/config/...` — 所有测试通过（0 failures）

---

## [2026-04-29] 第三轮安全修复 — 24 项 Code Review 剩余问题修复

> **分支**: `release/major-upgrade-vNEXT`
> **变更类型**: 安全修复 + 代码质量提升
> **涉及模块**: scheduler、auth、config、backup、distributed、web、cmd/unimap-cli

### 一、Webhook SSRF 防护（DNS/重定向绕过修复）

- **DNS 解析校验** — `ValidateWebhookURLPublic` 增加 `net.LookupIP` 校验所有解析 IP，阻止私有/回环地址 (`internal/scheduler/scheduler.go`)
- **安全 HTTP Client** — `safeWebhookClient()` 拒绝所有重定向 + 自定义 DialContext 阻止连接私有 IP (`internal/scheduler/scheduler.go`)

### 二、API Key 认证体系修复

- **SHA-256 hash 比对** — 引入 `KeyHash` 字段，`ValidateAPIKey`/`UpdateLastUsed` 改用 hash 比对，解决持久化后密钥不可验证问题 (`internal/auth/api_key.go`)
- **统一 map key 策略** — `GenerateAPIKey` 改用 ID 为 map key，`loadFromStorage` 使用 ID 保持一致 (`internal/auth/api_key.go`)
- **expiresIn=0 永不过期** — `zeroOrExpiry()` 将 0 转为 `IsZero` 时间 (`internal/auth/api_key.go`)
- **auth 平行认证接入** — Server 结构体新增 `apiAuth` 字段；路由层接入 `OptionalAPIKey` 中间件，API Key 认证体系正式接入主路由 (`web/server.go`, `web/router.go`)

### 三、分布式任务队列持久化

- **JSON 快照持久化** — 新增 `NewTaskQueueWithPath()` + `saveLocked()`/`loadSnapshot()`，所有状态变更（Enqueue/Claim/SubmitResult/Delete）自动保存 (`internal/distributed/task_queue.go`)
- **Server 接入** — TaskQueue 初始化传入 `./data/distributed_tasks.json` 路径 (`web/server.go`)

### 四、备份系统错误处理

- **Source 失败累积错误** — 静默跳过的 source 错误累积并在无有效文件时返回错误 (`internal/backup/backup.go`)
- **Tar 写入失败部分成功** — 累积 tar 错误，最终返回时包含失败统计 (`internal/backup/backup.go`)
- **BackupConfig 注释修正** — 注释改为"绝对路径或相对路径" (`internal/backup/backup.go`)

### 五、限流与安全头

- **X-Real-IP 代理检查** — 和 XFF 一样加入 `isPrivateOrInternalHost` 检查，防止可伪造代理头 (`web/middleware_ratelimit.go`)
- **CORS 默认 X-Admin-Token** — config 层默认 AllowedHeaders 添加 `X-Admin-Token` (`internal/config/config.go`)

### 六、可用性与安全性

- **Admin Token 非loopback 提示** — 非回环绑定打印 token 并提示保存 (`internal/config/config.go`)
- **CLI JSON 防覆盖** — `writeJSONFile` 改用 `O_EXCL`，与 CSV 一致 (`cmd/unimap-cli/api_subcommands.go`)

### 七、测试修复

- **TestGenerateAPIKey/zero_expiration** — `expiresIn=0` 返回 `IsZero` 时间，测试逻辑更新
- **TestUpdateLastUsed** — 改为按 ID 查找验证（map key 策略变更）
- **config 测试** — 默认 CORS headers 预期更新

### 验证结果

- `go build ./...` — 构建成功
- `go test -race ./...` — 全部通过（0 failures, 0 races）

---



