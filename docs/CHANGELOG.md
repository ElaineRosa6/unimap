# UniMap 变更日志

> 本文件按发生日期保留历史变更。条目中的“后续”“待办”“CI 全绿”和引擎/任务状态仅表示对应日期的事实；当前状态以 README、API、Runbook 和发布说明为准。

---

## [2026-07-24] 巡检运行时、历史筛选与证据截图

- 将根目录临时 Chrome 测速入口迁移到 `tools/chrome-speedtest`，使用
  `manual_browser_test` build tag、显式 headless 和单浏览器顺序导航；默认 build/test 不再纳入该工具。
- `TamperAppService` 每次构造 Detector 时读取最新提交的端口扫描、TLS 和超时配置，删除 Web
  handler 内未被业务路径使用的重复构造逻辑。
- 定时基线刷新复用截图管理器 allocator，并按 `saved` 摘要判定真实成功，避免初始化失败仍误报刷新成功。
- 巡检历史新增包含边界的 `start_time`、`end_time`，列表和导出保持一致；`count` 改为过滤后真实总数，
  URL 选项也遵循时间范围。
- 独立审查确认浏览器层尚不能抵御跨主机重定向和 DNS rebinding；因此未启用自动变化证据截图，
  该能力继续保留为待办，避免把调用前检查误当作完整 SSRF 防护。
- 全量 race、vet、默认/GUI build、手动工具仅编译和前端语法检查通过；真实云端页面变化与
  飞书图片送达仍需受控验收。

---

## [2026-07-23] 审计跟进与生产部署契约修复

- 在阿里云 Ubuntu 24.04 主机完成真实容器条件验收，报告见 `docs/CLOUD_ACCEPTANCE_2026-07-23.md`。
- 补记验收边界：普通网页 CDP 截图已通过，真实测绘引擎 CDP `collect_and_capture` 与 Extension Bridge 尚未在该主机验收；新增 `docs/CLOUD_STEADY_STATE_PLAN_2026-07-23.md` 记录常态化运行分工和门槛。
- 纠正引擎能力记录：Censys、DayDayMap 仅 API 适配与 API 实机验证完成，稳定 UI、Bridge/CDP 抓取和 live E2E 未完成；历史测绘数据抓取证据来自 Bridge，不能作为 CDP 通过证据。
- 新增 `docs/REMAINING_WORK_2026-07-23.md`，统一记录七引擎支持矩阵、生产配置、CDP 实测、Cookie 生命周期、UI 和发布清理待办。
- 现场修复单 URL 截图空 ID、生产资源硬编码、版本提交不可追踪、通知 pepper 缺失、Compose 废弃 version 警告以及 logs/backups 非 root 不可写问题。
- 实测通过 CGO/SQLite、CDP headless、单图、2/2 异步批量截图、security 巡检、调度备份、CSRF 登录、备份异机复制和容器重启持久化。
- 修正生产 Compose 合并语义：以 `!override` 完整替换基础端口和卷，避免公网 `8448` 与开发 `web` bind mount 在合并后残留；最低要求 Docker Compose 2.24.4。
- 新增 `configs/config.prod.yaml`，让固定 Web/分布式管理令牌与非默认管理员名真正从部署环境解析。
- 生产配置由只读单文件挂载改为 `unimap_config` 可写命名卷；入口脚本只在首次启动时初始化 `0600` 配置，支持设置页原子保存与重启持久化。
- 生产 Compose 支持 `UNIMAP_IMAGE` 预构建镜像；运行主机可固定 ACR 摘要并使用 `pull` + `up --no-build`，不再依赖 Docker Hub 构建。
- 未配置引擎不再被默认强制启用；Censys、DayDayMap 缺少完整 API 凭据时不再注册不可执行的 Web-only adapter。
- 开发 Docker 基线显式启用五个稳定 Web 引擎，避免移除隐式默认后出现兼容性回退。
- 生产模板显式启用低 QPS 的 Quake/Hunter 并透传可选 API Key；设置页 Hunter/Quake Base URL 提示与适配器默认值统一。
- 查询状态与结果页改用 DOM 节点和 `textContent` 构造，移除审计点名的动态 `innerHTML` 注入面。
- 补齐篡改历史导出的组合过滤、存储错误和错误方法回归测试；持久化审计 Finding #8 闭环。
- 本机仍无 Docker CLI，Compose 渲染、镜像运行、TLS、cgroup 和异机备份留待正式云机验收。

---

## [2026-07-20] 云服务器部署评估记录

> **变更类型**: 部署评估 + 文档纠偏
> **涉及模块**: Docker、Compose、SQLite、headless Chrome、Runbook、文档索引

### 核查结论

- 确认无图形 Linux 可使用 CDP headless，不需要桌面、X11、VNC 或 GPU；持久化 Chrome profile 继续强制截图会话串行。
- 确认当前 `Dockerfile` 的 `CGO_ENABLED=0` 与 `go-sqlite3` 不兼容。相同构建条件下，`internal/auth`、`internal/history`、`internal/screenshot/batchdb` 的数据库测试均因 CGO stub 失败；该问题记录为容器生产发布阻断，尚未在本次文档变更中修复。
- 现有 Compose 的 2 CPU / 1 GiB 限制不再表述为生产推荐值；单机完整功能建议从 4 vCPU、8 GiB RAM、80–100 GiB SSD 起步，并在真实云机压测后调整。
- 补充生产 Compose 差距：开发静态目录挂载、8448 直接公开、`/app/backups` 未持久化、只读配置的运行语义、资源限制可能未被平台实现以及日志/TLS/异机备份要求。
- 补充固定管理令牌、引擎真实凭据、Web-only adapter 边界、端口与 `CAP_NET_RAW` 权限、磁盘估算和正式云机十二项验收清单。

### 文档

- 新增 `docs/CLOUD_DEPLOYMENT_ASSESSMENT_2026-07-20.md`。
- 更新 Runbook 云主机章节，阻止在 CGO/SQLite 修复前误用当前镜像投产。
- 更新文档索引。

### 验证边界

- 已执行 `CGO_ENABLED=0 go test ./internal/auth ./internal/history ./internal/screenshot/batchdb -count=1`，预期复现并确认发布阻断。
- 本机没有 Docker CLI，本轮未执行 Compose 渲染、镜像构建或真实云机验证。

---

## [2026-07-17] 逻辑、API 与前后端交互修复闭环

> **变更类型**: 可靠性修复 + API 契约适配 + Web UX 完善
> **涉及模块**: config、CLI/GUI、screenshot、scheduler、distributed、users/session、query cache、notifications、Web templates、文档
> **提交**: `d8ad66e`、`f89f642`、`80350dc`

### 核心变更

- 完成 14 项代码逻辑/API/体验问题修复：配置 copy-on-write、CLI 认证和调度契约、异步截图、调度与分布式持久化回滚、会话撤销、首管理员原子创建以及查询缓存元数据。
- 补齐批量截图 timeout/failed 明确终态、ICP 对象错误解析、账号角色更新与删除错误处理、调度历史响应校验。
- 通知编辑改为安全 DTO：列表不返回 Webhook URL、secret 或 app secret；`preserve_existing=true` 可原子保留留空凭据，且禁止直接改变渠道类型。
- API、Runbook、问题报告、修复/回滚指南和文档索引同步到同一实现语义。

### 验证结果

- `go test -race ./...`、`go vet ./...`、`go build ./...` 通过。
- GUI build/test、live Bridge E2E 编译检查和 `node --check web/static/js/main.js` 通过。
- `govulncheck ./...` 未发现当前代码调用的已知漏洞；外部引擎真实凭据、远程扩展和生产 Redis 留待上线受控验收。

---

## [2026-07-15] 持久化与查询/调度可用性完善

> **变更类型**: 可靠性回归覆盖 + API 易用性增强
> **涉及模块**: alerting、ICP database、scheduler、web、API 文档
> **提交**: `4278a3f`、`a982189`、`1a32f46`

### 核心变更

- 核实 Alert JSON 已使用 `.tmp` 写入后 `os.Rename` 原子替换，补充替换完成后不残留临时文件的回归断言。
- `/api/v1/icp/history?keyword=...` 支持字面量部分关键词匹配；LIKE 通配字符会被转义，`type` 与 `task_id` 仍按精确值匹配。
- 调度任务创建和更新在落库前按 Runner 类型校验必填字段，400 响应包含任务类型、规范字段和可用别名。
- 调度数组字段兼容逗号分隔字符串；`urls` 兼容 `targets`，`engines` 兼容 `engine`，旧 `extra` 查询字段仍可读取。
- Runner 执行期的缺字段错误统一包含 Runner 类型；API payload 速查同步更新。

### 验证结果

- `go test -race ./...` 通过。
- 三项改动分别提交并推送至 `develop`。

---

## [2026-07-15] 端口扫描准确性与多协议探测升级

> **变更类型**: 功能增强 + 准确性修复
> **涉及模块**: service/monitor、scheduler、web、templates、API、Runbook

### 核心变更

- 将去重后的 `唯一 IP × 端口` 笛卡尔积随机打乱后再提交到有界工作池，避免固定的 IP/端口顺序。
- 保留完整 TCP connect，并新增 Telnet IAC 协商探测。
- 新增 FIN、NULL、Xmas（FIN/PSH/URG）原始 IPv4 TCP 探测；强制要求显式 `authorized_targets`，运行环境需要管理员/root 或 `CAP_NET_RAW`。
- 新增 UDP 探测和 TCP/UDP 多方法混合扫描，DNS、NTP、SNMP 使用对应协议载荷。
- 新增每次探测前的 `jitter_min_ms` / `jitter_max_ms` 随机抖动，范围限制为 0-5000ms。
- 响应新增结构化 `findings`。只有完成 TCP 连接或收到有效响应的结果才标记为 `open` 并进入 `open_ports`；UDP 或 FIN/NULL/Xmas 无响应标记为 `open_filtered`，避免误报确定开放。
- Web 页面、端口扫描 API 和 `port_scan` 定时任务均支持 `probe_methods` 与抖动参数。

### 安全与兼容边界

- 继续执行公网 IPv4、SSRF、CDN 排除和可选授权 CIDR 校验，原始扫描不会放宽任何边界。
- 未传 `probe_methods` 时保持原有 TCP connect 行为。
- `planned_connections`、`attempted_connections` 和目标级预期次数按 `IP × 端口 × 方法` 统计。
- `reports/` 为本地生成报告目录，加入 `.gitignore`，不进入版本控制。

### 验证结果

- `go test -race ./...` 通过。
- `go vet -mod=readonly ./...` 通过。
- `go build -mod=readonly -buildvcs=false ./...` 通过。
- `git diff --check` 通过。

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
