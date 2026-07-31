# UniMap 当前剩余工作清单

> 最后核对：2026-08-01。本文是当前本地待办入口；实施顺序和按旬排期见
> [后续实施计划](IMPLEMENTATION_PLAN_2026-07-23.md)。历史审计、计划和 E2E 报告只表达对应日期的事实。
> 完成项必须同时满足代码、测试和所声明后端的真实验收，不能因存在选择器、接口或单元测试就标记完成。

## 1. 引擎能力事实矩阵

| 引擎 | 稳定 Web UI | API 适配 | Bridge 结构化采集实测 | CDP 结构化采集实测 |
|---|---|---|---|---|
| FOFA | 已接入 | 已有适配器；历史另有真实 API 证据 | 2026-07-15 通过 | 代码完成（L1+L3），待真实账号校准 |
| Hunter | 已接入 | 已有适配器 | 2026-07-15 通过 | 2026-07-29 通过（10 资产 + 截图，选择器已校准） |
| ZoomEye | 已接入 | 已有适配器 | 2026-07-15 通过 | 代码完成（L1+L3），待真实账号校准 |
| Quake | 已接入 | 已有适配器 | 2026-07-15 通过 | 2026-07-29 通过（10 资产 + 截图） |
| Shodan | 已接入 | 已有适配器 | 2026-07-15 因登录态失效未通过 | 代码完成（L1+L3），待真实账号校准 |
| Censys | 未接入 | 已完成，2026-06-27 API 实机验证通过 | 代码完成（Extension 选择器+搜索 URL），待真实验收 | 代码完成（L1+L3），待真实账号校准 |
| DayDayMap | 未接入 | 已完成，2026-06-27 API 实机验证通过 | 代码完成（Extension 选择器+搜索 URL），待真实验收 | 代码完成（L1+L3），待真实账号校准 |

说明：

- Censys、DayDayMap 的 Extension DOM 选择器和 Go DOM `ExtractJS` 已于 2026-08-01 完成；
- Bridge 和 CDP 搜索 URL 构造已支持全部七个引擎（2026-08-01）；
- Censys、DayDayMap 的 L1 Network 解析器已添加（2026-08-01），单元测试通过；
- `live_bridge_e2e` 白名单已扩展为七个引擎（2026-08-01）；
- 2026-07-23 已修复 Censys、DayDayMap 的能力误报：缺少完整 API 凭据时不再注册 Web-only adapter；两者仍未实现 Bridge/CDP；
- 普通网页 CDP 截图通过不等于测绘引擎 CDP 结构化采集通过。
- 2026-08-01：七引擎均已有 L1 Network 解析器（Censys/DayDayMap 新增）和 L3 DOM ExtractJS；
  FOFA DOM 选择器已加固（4 级行选择器回退）；Censys/DayDayMap 搜索 URL 构造、
  Extension 选择器、Bridge 白名单和设置页配置均已接通。

## 2. P1：下一阶段必须完成

### RW-01 生产配置持久化与秘密录入

状态：**已完成（2026-07-23）**。

生产 Compose 已改用 `unimap_config:/app/runtime-config` 可写命名卷。入口脚本仅在
`config.yaml` 不存在时从镜像内 `config.prod.yaml` 初始化，权限设为 `0600`；应用通过
`UNIMAP_CONFIG_PATH` 读取运行配置，支持同目录临时文件和原子替换。

完成标准：

- 使用受保护的可写运行配置目录或等价秘密存储；**已完成**
- 不把秘密写入 Git、日志或响应；
- 不可写模式下页面明确提示并禁用保存；**仍待增强**
- 保存、重启、回滚和备份恢复测试通过；**已完成**

阿里云实机已验证：ACR 固定摘要镜像、`unimap_config` 可写卷、`0600` 配置、应用原子保存、
容器重启哈希一致、业务备份、受限配置备份以及“临时改为 auto → 原子恢复 → 重启回到 cdp”。
不可写页面提示属于后续 UX 增强，不再阻断当前可写生产基线。

### RW-02 Quake/Hunter 生产配置与真实 CDP 验收

状态：**CDP 验收已通过（2026-07-29），SQLite 合并和通知待云端验证**。

2026-07-29 验收结果（详见 [CDP_VERIFICATION_2026-07-29.md](CDP_VERIFICATION_2026-07-29.md)）：

- Quake：Cookie 登录态有效，CDP 搜索返回 2.06 亿条结果，
  应用代码路径采集 10 条结构化资产，PNG 截图 375KB，人工确认非登录页；
- Hunter：Cookie 登录态有效，CDP 搜索返回 2696 万条资产，
  应用代码路径采集 10 条结构化资产，历史验收截图为 JPEG 内容，人工确认非登录页；
- 截图回退已修复为统一 PNG 契约：PNG 首次捕获失败时使用 JPEG 捕获，再转码为 PNG 后保存和返回；
- Cookie 已写入 `configs/config.yaml`，Chrome 用户数据目录持久化会话。

2026-07-29 追加：

- Hunter DOM 选择器已校准（列映射修正 + checkbox 自动检测 + status_code/region 提取）；
- 历史 Web 服务 E2E：POST /api/v1/query browser_query=true 返回 200，
  Hunter bridge-collect 采集 2 条资产；当时 Quake Extension DOM 为 0 条；
- Quake L1 已校准到 `/api/search/query_string/quake_service`，解析器兼容当前 `data` 数组响应，
  并于 2026-07-29 使用持久化登录态获得非空真实结果；
- Quake Extension DOM 已修复 `data-clipboard-text` 的 IP 提取并通过真实 Chrome 固定夹具；
  更新后的真实 Bridge/Web 会话仍待复验，不能用夹具结果代替；
- SQLite 合并路径由单元测试覆盖（TestRunBrowserQueryAsync_CollectsStructuredAssets）；
- 通知管线已由 Scheduler 后置管线覆盖，需配置真实飞书/Webhook 凭据后触发 E2E。

仍待完成：

- 更新并重载 Extension 后，复验 Quake 强制 Extension 与完整 Web/Router 主链路；
- 配置真实飞书/Webhook 通知目标后验证图片通知送达；
- 容器重启后会话和结果恢复验证。

`configs/config.prod.yaml` 已显式启用 Quake/Hunter，使用正确 Base URL、QPS 1、30 秒超时；
Compose 已透传可选 `QUAKE_API_KEY`、`HUNTER_API_KEY`。Key 留空时只表示允许进入 Web-only
路径，不代表登录态或 CDP 采集已通过。

逐引擎完成：

1. API 最小查询；
2. Cookie 登录状态；
3. CDP 打开真实结果页；
4. 非空结构化资产；
5. 结果页 PNG 人工检查；
6. SQLite 合并持久化；
7. 图片通知；
8. 容器重启后会话和结果恢复。

### RW-03 五个稳定引擎 CDP 能力定级

现有 CDP URL、Cookie、Network/DOM 和 `collect_and_capture` 代码不能替代真实验证。

完成标准：

- FOFA、Hunter、ZoomEye、Quake、Shodan 分别记录通过、受限或不支持；
- 登录页、验证码页、空资产不能报告成功；
- 页面改版导致的 selector/network 失败有明确诊断；
- 形成独立于 Bridge 的 CDP live E2E 和日期化证据。

### RW-04 Censys/DayDayMap 浏览器能力与 UI 决策

状态：**能力误报已修复，当前产品范围维持 API-only；浏览器能力仍未实现**。

先确定产品范围：

- 若保持 API-only：删除或明确隔离不可达的 Web-only/选择器占位，API/UI/文档返回一致的能力说明；
- 若要支持浏览器：补齐查询 URL、UQL Web 翻译、登录态、Cookie、Bridge/CDP 结构化采集、截图、前端选择、设置页、调度、通知和 live E2E。

两者不能仅因“选择器存在”标记为 Bridge/CDP 已支持。

缺少完整 API 凭据时现在会跳过注册并记录警告，readiness 和引擎列表不再把不可执行的
Censys/DayDayMap Web-only 后端计为可用。后续若选择浏览器路线，仍需完成上述整条链路。

### RW-05 Cookie/Profile 无人值守闭环

状态：**核心框架已完成（2026-07-25），真实引擎验收待执行**。

已完成：

- `internal/scheduler/session_health.go`：`SessionHealthTracker` 实现 per-engine 熔断器（closed/open/half-open），连续失败阈值和冷却时间可配置；
- `ClassifyFailureReason()`：将失败分为 6 类（cookie_missing / cookie_expired / login_wall / captcha / page_changed / network）；
- `RecoveryHint()`：每类失败返回中文恢复操作指引；
- `LoginStatusCheckRunner` 已集成 health tracker：记录成功/失败、熔断跳过、输出健康摘要和恢复提示；
- `AllowBrowserTask()`：熔断打开时阻止对应引擎的浏览器任务，但允许登录检查（探测恢复）；
- 单元测试覆盖熔断、冷却、分类、摘要和恢复提示。

2026-07-29 新增：

- AllowBrowserTask() 已接入 QueryRunner.Execute() 浏览器任务路径：熔断打开时跳过对应引擎的浏览器任务，仅允许登录检查探测恢复；
- SessionHealthTracker 改为 web/server.go 中创建的共享实例，LoginStatusCheckRunner 和 QueryRunner 共用同一熔断状态；
- 通知集成已由 Scheduler 后置管线覆盖：LoginStatusCheckRunner 结果文本包含健康摘要和恢复提示，任务启用通知后自动发送到飞书/Webhook。

仍待完成：

- 周期性打开真实结果页验证登录（需要真实引擎账号和 CDP 环境）；
- 更新 Cookie 后自动执行低额度恢复验收（需要在设置页保存 Cookie 的 handler 中触发一次 LoginStatusCheck）。

## 3. P2：应当收尾

### RW-06 设置页 Base URL 纠正

状态：**已完成（2026-07-23）**。

- Hunter 占位值已与默认 `https://hunter.qianxin.com` 一致；
- Quake 占位值已与默认 `https://quake.360.net/api` 一致；
- 已增加前端契约测试，防止再次偏离 adapter 默认值。

### RW-07 配额功能

当前明确未实现：

- 配额趋势图；
- 自动刷新设置；
- 阈值告警。

入口继续保持禁用，直到服务端存储、调度、告警和 UI 全部完成。

### RW-08 发布与工作区清理

- 状态：**安全收口修复实施中（2026-08-01），尚未达到发布门槛**。
- 当前代码与第一轮文档变更已审查并提交为 `f9317a3`；**已完成**
- `CLAUDE.md` 已按用户要求保留并同步当前事实；**已完成**
- 根目录 `tmp_speedtest.go` 已迁移为带 `manual_browser_test` 标签的
  `tools/chrome-speedtest` 手动工具；**已完成**
- 推送 `develop` 当前领先远端的本地提交；**待用户确认远端发布时执行**
- 发布前再次执行 `go test -race ./...`、`go vet ./...`、`go build ./...`、GUI build tag、
  headless SSRF、Extension Quake 夹具与 `govulncheck`。**本轮终检中**
- 云端安全验收 fixture 已构建（`tools/acceptance-fixture/`）：DNS rebinding 控制 API +
  可变页面 + 私网 sink + Cloudflare DNS 翻转 + Caddy HTTPS + Docker Compose。
  部署到受控服务器后即可运行 `live_dns_e2e` 和 `live_tamper_e2e`。**部署计划：2026-08-02。**

### RW-09 网页巡检确定性收尾

状态：**部分完成（2026-07-24），安全证据截图与云端真实闭环待完成**。

- 手动巡检、定时巡检与基线刷新读取同一套实时 Tamper 配置，并统一使用 Screenshot Manager 的受保护页面加载接口；
- 基线刷新按实际 `saved` 结果统计，不再把业务失败计为成功；
- 历史列表和导出支持 `start_time`、`end_time`，`count` 返回过滤后总数；
- “发现变化 → 证据截图 → 图片通知”仍未启用：逐跳和连接级 SSRF 代码及本地验收已完成，
  但受控云端真实变化、图片送达和重启复验尚未完成；
- 尚需在云端用受控页面完成基线、真实变化、历史、飞书图片送达、恢复和重启后复验。

## 4. P3：可选增强

- ~~历史 API 增加 `start_time`、`end_time`；~~ **已完成（2026-07-24）**
- 提供受控恢复命令或恢复 API，而不只提供备份创建/列表；
- 完成配额趋势与告警后增加长期数据保留策略；
- ARM64 镜像与 Chromium 验收。

## 5. 本轮已纠正的记录

- GUI 入口 (cmd/unimap-gui) 已删除，Fyne 依赖已清除（2026-08-01）；项目入口为 unimap-web + unimap-cli；
- CLI 已完成 Agent 友好化改造：JSON 信封、语义退出码、分页、--format json、quota/config show/help --json；
- CLI/GUI 不再集成 CDP/Bridge，截图通过 Web API 代理；

- 不再把 Censys、DayDayMap 描述成已完成 Extension/Bridge 能力；
- 不再把历史 Bridge 抓取测试当作 CDP 证据；
- 将备份任务改为已提交的第 23 种任务；
- 生产运行配置改为首次初始化的可写命名卷；
- 未配置引擎不再被默认强制启用，Censys/DayDayMap 缺凭据时不再伪注册 Web-only；
- Quake/Hunter 生产模板和设置页 Base URL 已统一；
- 当前权威文档统一使用本文件的支持矩阵和待办编号。
- 巡检运行时配置、受保护页面加载接口、时间筛选和真实总数已完成代码收口；变化证据截图因
  云端真实变化与图片送达尚未验收继续保留待办。
- 本地代码基线已提交为 `f9317a3`，但截至本次核对尚未推送到 `origin/develop`。

## 6. 本地验证（2026-07-29）

- `go test -race ./...`：通过；
- `go vet ./...`：通过；
- `go build ./...`：通过；
- `go build -tags gui ./cmd/unimap-gui`：通过；
- `go build -tags manual_browser_test ./tools/chrome-speedtest`：仅编译通过，未运行 Chrome；
- `node --check web/static/js/main.js`：通过；
- `git diff --check`：通过。

本轮 Excelize 已升级到 v2.11.0，`govulncheck ./...` 返回 0 个可达漏洞；Quake L1 真实测试、
Quake Extension DOM 固定夹具、headless 私网重定向/子资源和连接前阻断均已通过。全量 race、
vet、默认/GUI build 和前端语法以本轮最终门禁记录为准。真实 Extension/Web Quake 复验、
云端页面变化与图片送达仍是外部验收边界。
