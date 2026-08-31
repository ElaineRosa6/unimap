# UniMap — 多引擎资产查询与网页巡检工具

> 最后按当前工作区核对：2026-08-21。发布与试运行跟踪 `master`（云端 `71371f1`，仓库文档提交 `fdb3292`）。`origin/develop` 停在 `71be507`，落后 master，不是当前基线。Go 版本以 `go.mod` 的 1.26.5 为准。后续执行顺序见 `docs/AGENT_CONTINUATION_PLAN_2026-08-20.md`。插件/CDP 分列见 `docs/PLUGIN_CDP_STATUS_2026-08-21.md`。

## 项目现状

UniMap 提供 Web 和 CLI 入口，覆盖资产查询、浏览器采集、网页截图、网页巡检、
调度、通知、分布式节点、备份和 Prometheus 指标。

稳定 Web UI 当前使用 FOFA、Hunter、ZoomEye、Quake、Shodan、Censys、DayDayMap。七引擎
Bridge 真实结构化采集均非空；DayDayMap 原生 CDP 通过，Censys challenge 识别与 Bridge
fallback 通过。FOFA/Shodan 原生 CDP 于 2026-08-21 通过；ZoomEye 同日站点/SSO 受限。
当天傍晚 Extension 0.4.18 活页选择器已校准（DayDayMap 复验 10 条）；改完的 ExtractJS
尚未再用 CDP CollectAndCapture 复跑。2026-08-25 本机 smtp-relay 已用与云端相同的 SMTP
配置发出测试信，用户确认收件；发件/收件只改 `smtp-relay/.env`。

普通公网网页的云端 CDP headless 截图已经通过。Quake、Hunter 已完成真实 CDP 登录、
结构化采集和结果页截图；Quake L1 Network 已于 2026-07-29 按当前接口重新实测通过。
Quake 真实 Extension 调度闭环和云端飞书通知已通过；受控页面变化后的自动证据图片通知仍待验收。

## 技术与入口

| 类别 | 当前实现 |
|---|---|
| 语言 | Go 1.26.5 |
| Web | `net/http`、`gorilla/websocket`、`go-resty` |
| CLI | Go 标准库 `flag`，不是 Cobra |
| 浏览器 | chromedp + Chrome Extension Bridge |
| 截图模式 | `auto`、`cdp`、`extension` |
| 调度 | robfig/cron/v3，23 种已提交任务类型 |
| 存储 | SQLite、YAML、文件存储；Redis 为可选缓存 |
| HTML | goquery + bluemonday |
| 通知 | Webhook、钉钉、飞书、飞书应用、企业微信、日志 |
| 部署 | Windows、无图形 Linux、Docker |

```text
cmd/unimap-cli/       CLI
cmd/unimap-web/       Web 服务（默认 :8448）
internal/adapter/     引擎适配器与编排
internal/service/     应用服务
internal/screenshot/  CDP、Bridge、截图路由、批次持久化
internal/tamper/      基线与网页巡检
internal/scheduler/   调度任务与 Runner
web/                  路由、handler、中间件、模板
configs/              配置模板与本地配置
docs/                 当前文档、ADR 与归档
memory/               历史知识索引
```

## 运行与验证

```bash
cp configs/config.yaml.example configs/config.yaml

go run ./cmd/unimap-web
go run ./cmd/unimap-cli --help

go fmt ./...
go test -race ./...
go vet ./...
go build ./...
```

默认测试不得启动 Chrome。真实浏览器测试必须显式使用对应 E2E build tag。

不要在仓库根目录保留可被 `go run .` 或 `go test ./...` 意外纳入的临时 `package main` 浏览器脚本。

## 已实现能力

### 多引擎查询

- UQL 解析和引擎语法转换；
- 多引擎并发、字段标准化、结果合并与去重；
- CSV、Excel、JSON 导出；
- 内存或 Redis 缓存；
- WebSocket 查询进度；
- SQLite 查询历史。

`query` 调度任务可使用 `browser_query=true` 和 `browser_action=collect_and_capture`，进入浏览器
采集、截图、SQLite 持久化和图片通知工作流。

### 截图与浏览器

- CDP 和 Extension Bridge；
- 单 URL、搜索结果、目标和批量截图；
- 异步批次进度、明确终态、预览与删除；
- 无图形 Linux 使用 CDP headless；
- 固定 Chrome Profile 自动串行；
- Bridge 配对、任务和回调只允许 loopback。

### 网页巡检

- URL 可达性和 Web 探测；
- 手动与批量建立基线；
- `strict`、`relaxed`、`security`、`balanced`、`precise` 五种模式；
- 分段哈希、恶意内容和页面指纹检测；
- 基线列表、删除、历史、过滤、导出与清理；
- 定时巡检、基线刷新和告警。

五种模式的准确语义以 `internal/tamper/detector_check.go` 和对应测试为准，不在本文件重复维护
容易漂移的阈值描述。

### 调度与通知

- Cron、一次性和延迟任务；
- 任务启停、立即执行、超时、重试、历史与持久化；
- 当前共有 23 种任务，包括查询、截图、巡检、配额、ICP 和备份；
- 通知通道 CRUD、测试与运行时重新加载；
- 告警去重、静默和恢复记录。

`bridge_token` 是历史兼容任务键，当前执行 Bridge 健康检查，不轮换 token。

### 系统管理

- Session、管理 token 和 CLI token 认证；
- `admin`、`operator`、`readonly` 用户角色；
- 用户禁用、删除或修改密码后旧 Session 失效；
- 配置校验、原子保存与运行时重载；
- 操作历史、业务备份和健康检查；
- `/health`、`/health/live`、`/health/ready`、`/metrics`。

## API 与安全约束

- 唯一业务 API 前缀是 `/api/v1/...`，不得重新引入旧 `/api/...` shim；
- 非 API 路由包括健康检查、指标和截图文件；
- 外部 URL、截图、巡检和 Webhook 必须保留 SSRF 防护；
- 私有、loopback、链路本地和内部地址不得绕过默认限制；
- 认证默认启用；非 loopback 启动由 `StartupPreflight` 强制显式 `admin_token`、非 `admin` 用户名和合法 bcrypt；loopback 允许空凭据走首管理员注册；
- 示例配置启用 API 限流，可信代理来源必须显式配置；
- `screenshot.engine` 仅用于旧配置兼容，新配置使用 `screenshot.mode`；
- `$VAR` 和 `${VAR}` 是显式环境变量占位符，普通 YAML 值不会被同名环境变量隐式覆盖；
- API Key、Cookie、管理令牌、Bridge token、Webhook URL 和通知密钥不得进入代码、文档或日志；
- 配置文件可以在受控本地环境保存秘密，但必须被 Git 忽略并限制文件权限。

## 当前待办

后续 agent 从 [2026-08-20 推进计划书](docs/AGENT_CONTINUATION_PLAN_2026-08-20.md) 的状态板接着做，不要重开已完成的日更闭环。

### P0：工作区与发布基线

- 本地代码与文档基线已审查并提交；**已完成**
- 本文件已按用户要求保留并同步；**已完成**
- 根目录 Chrome 测速入口已迁移到带显式 build tag 的工具目录；**已完成**
- 修正文档中的过期云端状态；**已完成**
- 重新执行全量 race、vet、build、Extension 和前端检查；**已完成**
- 推送 `develop` 当时领先远端的本地提交；**已完成（2026-08-04）**。2026-08-20 起发布基线是 `master`；`develop` 落后，快进与否由用户决定

### P1：网页巡检收尾

- 定时基线刷新复用正确的浏览器 allocator；**代码与自动测试完成**
- 将端口扫描、TLS 等配置传入实际 Detector；**代码与自动测试完成**
- 接通历史开始时间和结束时间筛选；**代码与自动测试完成**
- 浏览器逐跳与连接级 SSRF 代码及本地 headless 验收已完成；自动证据链代码由
  `tamper.evidence_screenshot_enabled=false` 默认关闭，DNS 和页面变化 live E2E 已就绪；
  在受控云端验收前继续禁用“发现变化 → 证据截图 → 图片通知”；
- 执行云端真实基线、页面变化、告警和恢复闭环。

### P1：浏览器与引擎验收

- Quake、Hunter 真实 CDP 查询、采集、截图；**已完成（2026-07-29）**。云端试运行当前关闭截图，日更走 API。
- FOFA、ZoomEye、Shodan 的原生 CDP 非空定级；**FOFA/Shodan 08-21 通过，ZoomEye 受限。** ExtractJS 傍晚又改过，CDP 复跑与插件改后活抽见 [PLUGIN_CDP_STATUS_2026-08-21.md](docs/PLUGIN_CDP_STATUS_2026-08-21.md)
- Cookie/Profile 登录探针、失败熔断、告警和恢复；框架已接线，真实验收见计划书 C2
- Censys challenge 页面策略允许时复验原生 CDP；现有自动 Bridge fallback 已通过。

### P2/P3：产品与生产化

- 配额历史、趋势、自动刷新和阈值告警；
- 不可写配置模式的明确提示；
- 受控恢复命令或恢复 API；
- 保存 Cookie 后的主动登录验证（不在保存路径内自动拉 CDP）；
- 调度通知按成功/失败/超时区分渠道（2026-08-07 暂缓）；
- 数据、截图、巡检和日志保留策略；
- 域名、TLS、反向代理、安全组和异机备份；
- ARM64、容量、恢复和至少 24 小时稳定性验收。

## 开发约束

- 使用 `gofmt`；改动导入时使用 `goimports`；
- 先写或更新受影响测试，至少运行对应包的 `go test -race`；
- 完成代码修改后运行全量 race、vet 和 build；
- `internal` 包不能被仓库外模块导入；
- 插件为进程内源码注册，不支持外部动态二进制插件；
- 修改路由、任务类型、引擎范围、认证、配置或 Bridge 协议时，同步更新相关权威文档；
- 任何浏览器诊断工具必须位于显式工具目录，并通过 build tag 或专用命令运行。

## 权威文档

| 主题 | 文档 |
|---|---|
| 文档入口 | `docs/README.md` |
| 当前 API | `docs/API.md` |
| 运维 | `docs/RUNBOOK.md` |
| 架构 | `docs/ARCHITECTURE.md` |
| Bridge | `docs/OPS_SCREENSHOT_EXTENSION.md` |
| 当前待办 | `docs/REMAINING_WORK_2026-07-23.md` |
| 后续执行计划 | `docs/AGENT_CONTINUATION_PLAN_2026-08-20.md` |
| 插件/CDP 现状 | `docs/PLUGIN_CDP_STATUS_2026-08-21.md` |
| 实施计划（历史排期） | `docs/IMPLEMENTATION_PLAN_2026-07-23.md` |
| 云端验收 | `docs/CLOUD_ACCEPTANCE_2026-07-23.md` |
| 云端常态化 | `docs/CLOUD_STEADY_STATE_PLAN_2026-07-23.md` |
| 历史知识 | `memory/MEMORY.md`、`docs/archive/` |

历史审计、计划和 E2E 报告只表达对应日期的事实。若与当前代码冲突，以当前路由、handler、
测试、`AGENTS.md` 和上述权威文档为准。

## 工程经验

### 前端跳转

先列全场景，再盘点已有探测能力。当前结果页使用 `/api/v1/url/probe-web-batch` 批量探测并缓存，
避免依赖端口或协议猜测。

### 巡检

应用层不得擅自收窄底层检测模式。手动、调度和后台路径必须复用同一浏览器配置和业务流程，
不能因为交互入口可用就假定定时路径也可用。
