# UniMap 架构

> 最后按代码核对：2026-07-29。Go 版本为 1.26.5；路由事实来源为 `web/router.go`。

## 分层

```text
Web / CLI / GUI / Extension
            │
web handlers ── internal/service 应用服务
            │                 │
            ├── adapter + core/unimap + model
            ├── screenshot + tamper + scheduler
            └── config + auth + history + alerting + backup
                         │
                  SQLite / YAML / file store / Redis(optional)
```

- `cmd/unimap-web`、`cmd/unimap-cli`、`cmd/unimap-gui` 是入口；GUI 受 `gui` build tag 保护。
- `web/` 负责 HTTP 路由、认证、中间件、模板和静态资源。
- `internal/service/` 协调查询、截图、巡检等应用用例，不直接成为仓库外部 SDK。
- `internal/adapter/`、`internal/core/unimap/` 和 `internal/model/` 构成查询核心。
- `internal/screenshot/` 负责 CDP、Bridge、ScreenshotRouter 与批次持久化；`internal/tamper/` 负责基线与巡检记录。

## 查询与引擎

稳定 Web UI 提供 FOFA、Hunter、ZoomEye、Quake、Shodan。Censys、DayDayMap 只完成服务端/CLI API 适配与 API 实机验证，不属于稳定 Web UI，也未完成 Bridge/CDP 结构化抓取。扩展和 Go DOM 表中存在的泛化选择器只是占位：Bridge/CDP 的搜索 URL 构造仍拒绝这两个引擎，Go DOM 条目也没有 `ExtractJS`。

```go
type EngineAdapter interface {
    Name() string
    Translate(ast *model.UQLAST) (string, error)
    Search(ctx context.Context, query string, page, pageSize int) (*model.EngineResult, error)
    Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error)
    GetQuota() (*model.QuotaInfo, error)
    IsWebOnly() bool
}
```

请求流：`UQL → Parser → EngineOrchestrator → Adapter → EngineResult → Normalize → ResultMerger → cache/export`。

## 截图与巡检

- 截图、浏览器查询、调度截图与巡检共用 ScreenshotRouter；`cdp`、`extension`、`auto` 的配置模式和实际活动模式分离，仅在允许 fallback 时切换后端。
- CDP 使用 Chrome/Chromium 新版 headless，不依赖图形会话。可执行文件按显式配置、`UNIMAP_CHROME_PATH`、平台探测顺序解析；显式路径错误会立即暴露。Windows readiness 使用无进程的 PE 静态验证，缺少 CDP provider 时不探测浏览器。浏览器 allocator 共享有界会话槽，固定 user-data-dir 时因 Chromium profile 锁自动串行。
- Extension Bridge 的配对、任务和回调仅允许 loopback 请求，使用短期 token；可选 HMAC 签名和 nonce 防重放。
- FOFA、Hunter、ZoomEye、Quake、Shodan 的历史结构化采集 E2E 使用 Bridge。Quake、Hunter 已有真实 CDP DOM 证据；Quake L1 Network 已按当前 endpoint/数组响应重新实测通过。FOFA、ZoomEye、Shodan 的 CDP 定级仍未执行。
- CDP 业务入口统一通过 guarded browser session：提交前 URL/实时 DNS 校验、CDP Fetch 全资源逐跳拦截，以及在实际拨号时重新解析并固定公网 IP 的 loopback 出口代理。外部上游代理和远程 Chrome 在无法证明等价出口约束时失败关闭。
- Extension 搜索引擎任务要求回调 `final_url` 且不得离开批准主机；未提供受控出口证明时，Extension 任意 URL 与批量截图保持禁用。
- 巡检模式为 `strict`、`relaxed`、`security`、`balanced`、`precise`。巡检与截图入口对公网目标进行 SSRF 防护。
- `TamperAppService` 在每次检查/设基线时读取最新已提交的端口扫描、TLS 和超时配置；手动巡检、定时巡检与基线刷新复用 Screenshot Manager 的受保护页面加载接口。
- 端口扫描采用两阶段模块：先并发解析目标并执行 SSRF、CDN 与可选授权 IPv4/CIDR 判断，再构造全局去重并随机化的 `唯一 IP × 端口` 计划。有界工作池可对每个组合执行 connect、Telnet、FIN/NULL/Xmas 与 UDP 探测及随机抖动，随后按解析关系把确定开放和 `open_filtered` 结果回填给原始目标；原始 TCP 方法强制要求显式授权范围。
- Cookie 与登录状态 API 是 `GET /api/v1/cookies/login-status`，不是旧 `/api/cookies/...`。

## 调度与分布式

调度器负责 cron、一次性和延迟任务。当前工作区定义 23 种已提交任务类型，包含备份任务。

分布式节点由 HTTP 协议注册、心跳、领取和回传任务，没有独立的 `unimap-node` 命令。接口仅在 `distributed.enabled=true` 下可用，节点令牌和分布式管理令牌职责不同。

## API 与安全边界

- 所有业务 API 使用 `/api/v1/...`；路由总数随当前注册表变化，不应在架构文档中写死。
- `/health`、`/health/ready`、`/health/live`、`/metrics` 不使用 API 前缀。
- `web/router.go` 给路由应用可选 API Key；handler 根据资源再要求管理员、节点令牌、CSRF/可信 Origin 或 loopback。
- 错误响应通过 `writeAPIError` 统一为 `{success:false,error:{code,message,details}}`；成功响应按 handler 定义，不存在强制统一成功信封。
- 配置中的 `$VAR`/`${VAR}` 为显式环境变量解析；直接 YAML 值不会被同名环境变量覆盖。

## 可观测性与存储

- Prometheus 指标在 `/metrics`，受部署认证与 loopback 约束。
- SQLite 用于用户、历史、任务与部分批次/巡检索引；YAML 用于配置；文件系统保存截图和基线；Redis 为可选缓存。
- 日志经 zap 输出，错误对外返回前通过 `sanitizeError` 处理。

路由详情见 [API.md](API.md)，故障处理见 [RUNBOOK.md](RUNBOOK.md)，插件边界见 [PLUGIN_ARCHITECTURE.md](PLUGIN_ARCHITECTURE.md)。
