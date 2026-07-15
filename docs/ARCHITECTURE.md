# UniMap 架构

> 最后按代码核对：2026-07-13。Go 版本为 1.26.5；路由事实来源为 `web/router.go`。

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

稳定 Web UI 提供 FOFA、Hunter、ZoomEye、Quake、Shodan。服务端和 CLI 可在配置可用时注册 Censys、DayDayMap；它们不属于稳定 Web UI 列表。

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

- 截图可用 `cdp`、`extension`、`auto` 三种运行模式；ScreenshotRouter 根据健康状态执行降级。
- Extension Bridge 的配对、任务和回调仅允许 loopback 请求，使用短期 token；可选 HMAC 签名和 nonce 防重放。
- 巡检模式为 `strict`、`relaxed`、`security`、`balanced`、`precise`。巡检与截图入口对公网目标进行 SSRF 防护。
- 端口扫描采用两阶段模块：先并发解析目标并执行 SSRF、CDN 与可选授权 IPv4/CIDR 判断，再构造全局去重并随机化的 `唯一 IP × 端口` 计划。有界工作池可对每个组合执行 connect、Telnet、FIN/NULL/Xmas 与 UDP 探测及随机抖动，随后按解析关系把确定开放和 `open_filtered` 结果回填给原始目标；原始 TCP 方法强制要求显式授权范围。
- Cookie 与登录状态 API 是 `GET /api/v1/cookies/login-status`，不是旧 `/api/cookies/...`。

## 调度与分布式

调度器负责 cron、一次性和延迟任务。当前工作区定义 23 种任务类型，其中备份任务是未提交改动；正式发布文档应在提交后确认该数量。

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
