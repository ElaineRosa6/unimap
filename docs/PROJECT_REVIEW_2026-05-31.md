# UniMap 项目审查报告

审查日期：2026-05-31

审查范围：当前仓库代码、配置、依赖、前端模板与静态脚本、Docker/Nginx/Compose、测试与文档。

验证结果：

- `go test ./...` 通过
- `go test -race ./...` 通过

## 1. 审查计划与检查清单

- 项目结构与模块边界：检查 `cmd/`、`web/`、`internal/`、`tools/extension-screenshot/`、`configs/`、`docs/`。
- 端到端联通性与网络代理配置：检查 `nginx.conf`、`Dockerfile`、`docker-compose.yml`、WebSocket 反代。
- API 契约一致性与实际可连通性：比对前端 `fetch/WebSocket` 调用与 `web/router.go` 路由表。
- 核心业务主流程与异常兜底：登录、查询、截图、篡改检测、调度器、通知、扩展桥接。
- 状态管理与状态流转：检查前端本地缓存、WebSocket 查询状态、后端 session、调度器状态。
- 鉴权、权限、越权与安全策略：检查 Cookie、CSP、CORS、Origin、Admin Token、Bridge Token。
- 输入校验、时区处理与数据一致性：检查 JSON decode、Form/Query 参数、Unix/RFC3339 混用。
- 错误码、错误提示与降级策略：检查 `writeAPIError`、第三方查询重试、熔断、浏览器 fallback。
- 并发、事务与数据竞争：执行 race 测试，检查调度器、WebSocket、缓存、SQLite。
- 性能瓶颈、慢查询与资源泄漏：检查前端定时器、WebSocket 广播、缓存、长任务。
- 第三方服务依赖风险与熔断降级：检查 FOFA/Hunter/Quake/ZoomEye/Shodan、通知 Webhook、CDP/Extension。
- 部署配置、环境变量与运维就绪度：检查健康检查、静态资源缓存、优雅停机、Migration。
- 测试覆盖与项目完成度：检查单测、race、E2E 脚本和未实现标记。

## 2. 架构与系统设计评估

### `web.Server` 职责过重

- 位置：`web/server.go:77-110`
- 问题：`Server` 同时持有模板、服务、调度器、WebSocket、Bridge、ICP DB、通知、鉴权等状态。
- 影响：新增功能会持续扩大结构体和初始化逻辑，Handler 难以独立测试和复用。
- 建议：拆分为 `QueryHandler`、`ScreenshotHandler`、`SchedulerHandler`、`BridgeHandler`，`Server` 只负责路由组合和生命周期编排。

### API 缺少版本边界

- 位置：`web/router.go:47-156`
- 问题：所有接口集中在 `/api/...`，没有 `/api/v1` 或兼容层。
- 影响：字段名、状态码或返回结构调整会直接影响 Web、CLI、扩展和脚本。
- 建议：新增 `/api/v1`，老接口保留 shim；为 CLI 和扩展依赖的接口补契约测试。

### 优雅停机顺序存在资源关闭风险

- 位置：`web/server.go:858-868`
- 问题：先停止调度器、关闭 ICP DB，再调用 `httpServer.Shutdown(ctx)`。
- 影响：正在处理的 HTTP 请求仍可能访问已关闭的 DB 或后台资源。
- 建议：先停止接收新请求并等待请求 drain，再关闭调度器、DB、截图 Router、插件。

### 调度器 Stop 不等待正在执行的任务

- 位置：`internal/scheduler/scheduler.go:852`、`internal/scheduler/scheduler.go:1250`
- 问题：任务 context 来自 `context.Background()`；`cron.Stop()` 返回的 context 未等待。
- 影响：长任务可能越过服务生命周期并访问已关闭资源。
- 建议：引入 scheduler-level context，`Stop()` 等待 `cron.Stop()`，任务 context 从 scheduler context 派生。

## 3. 核心业务与全链路闭环诊断

主链路基本闭环：

- 登录：`web/login_handlers.go:46-119`
- 查询入口：`web/query_handlers.go:128-180`、`web/websocket_handlers.go:189-400`
- 服务编排：`internal/service/unified_service.go:201-388`
- 引擎熔断、缓存、重试：`internal/adapter/orchestrator.go:424-570`

主要断点：

1. Nginx 反代不可用。
   - 位置：`nginx.conf:19-20`、`Dockerfile:47`、`docker-compose.yml:7`
   - 问题：Nginx upstream 指向 `unimap:8080`，应用实际暴露 `8448`。

2. WebSocket 经 Nginx 会升级失败。
   - 位置：`nginx.conf:28-35`、`web/static/js/main.js:913-914`
   - 问题：缺少 `proxy_http_version 1.1`、`Upgrade`、`Connection`。

3. 批量截图页面返回 500。
   - 位置：`web/router.go:38`、`web/screenshot_handlers.go:553-554`
   - 问题：正常渲染页面时传入 `http.StatusInternalServerError`。

4. 通知类第三方依赖缺少持久化补偿。
   - 位置：`internal/notify/channels.go:120-146`、`internal/notify/bot_channels.go`
   - 现状：发送失败会返回错误或记录日志，但没有持久化 outbox、重试队列或人工补偿记录。
   - 适用性：对运维告警可接受；如果后续用于强一致业务通知，需要补齐。

5. WebSocket 查询具备鉴权、心跳、panic recovery。
   - 位置：`web/websocket_handlers.go:94-116`、`web/websocket_handlers.go:263-272`
   - 结论：未发现明确阻断问题，但存在全局广播状态隔离风险，见安全章节。

## 4. 前后端交互、联通性与契约一致性

| 接口/模块 | 前端位置 | 后端位置 | 联通/契约问题 | 风险 | 修复建议 |
|---|---|---|---|---|---|
| Nginx upstream | N/A | `nginx.conf:19-20` | upstream 指向 `8080`，服务实际 `8448` | 反代 502 | 改为 `server unimap:8448;` |
| `/api/ws` | `web/static/js/main.js:913-914` | `web/router.go:71` | Nginx 未配置 WebSocket Upgrade | 实时查询失败 | 增加 Upgrade 代理头 |
| `/batch-screenshot` | 页面入口 | `web/router.go:38`、`web/screenshot_handlers.go:553-554` | 正常页面返回 500 | 监控误报、用户误判 | 改为 `http.StatusOK` |
| `/api/screenshot/bridge/token/rotate` | `tools/extension-screenshot/src/api.js:101` | `web/router.go:87` | 扩展代码一致；但部分 handler 测试仍使用旧路径 | 维护误导 | 测试走真实 router |
| `/api/config` | `web/templates/settings.html:564` | `web/router.go:155-156` | 前端发空 Bearer token，实际依赖 Cookie | 认证路径混乱 | 统一 Cookie 或显式 token 登录流程 |
| 查询状态时间字段 | `web/static/js/main.js:995` | `web/server.go:38-48` | QueryStatus 无 JSON tag，返回 PascalCase；Bridge 多处使用 Unix 秒 | API 风格不统一 | 增加 JSON tag，统一时间格式 |
| 静态资源 | Nginx `/static/` | `nginx.conf:47-49` | nginx 服务注释块未挂载 `/app/web/static` | 启用 Nginx 后静态 404 | compose 增加静态目录挂载 |

## 5. 代码质量与缺陷排查

### [P1] Nginx 反代端口错误导致全站不可达

- 位置：`nginx.conf:19-20`，`Dockerfile:47`，`docker-compose.yml:7`
- 现象：启用 Nginx 后代理到 `unimap:8080`，但应用监听 `8448`。
- 触发场景：生产复用当前 `nginx.conf` 或启用 compose 中 nginx 服务。
- 根因：反代配置与容器端口不一致。
- 影响：所有 HTTP/API/Web 页面 502。
- 修复建议：

```nginx
upstream unimap {
    server unimap:8448;
}
```

### [P1] WebSocket 经过 Nginx 无法升级

- 位置：`nginx.conf:28-35`，`web/static/js/main.js:913-914`
- 现象：前端连接 `/api/ws`，反代未设置 Upgrade。
- 触发场景：通过 Nginx 访问实时查询。
- 根因：缺少 WebSocket 代理头。
- 影响：实时查询退化或失败。
- 修复建议：

```nginx
proxy_http_version 1.1;
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection "upgrade";
```

### [P1] 批量截图页面正常访问返回 500

- 位置：`web/screenshot_handlers.go:553-554`
- 现象：渲染 `batch-screenshot.html` 时传入 `http.StatusInternalServerError`。
- 触发场景：GET `/batch-screenshot`。
- 根因：状态码写错。
- 影响：监控误报、浏览器或代理认为页面失败。
- 修复建议：

```go
s.renderTemplateWithNonce(r, w, http.StatusOK, "batch-screenshot.html", data)
```

### [P1] 停机顺序可能中断正在进行的 DB 请求

- 位置：`web/server.go:858-868`
- 现象：先关闭调度器和 ICP DB，再等待 HTTP 请求 drain。
- 触发场景：停机时仍有 `/api/icp/*` 请求。
- 根因：资源关闭早于 `httpServer.Shutdown`。
- 影响：请求中途失败，极端情况下写入不完整。
- 修复建议：先 `httpServer.Shutdown(ctx)`，再停调度器、Router、DB、插件。

### [P2] 调度器 Stop 不等待正在执行的任务

- 位置：`internal/scheduler/scheduler.go:852`、`internal/scheduler/scheduler.go:1250`
- 现象：任务 context 来自 `context.Background()`，`cron.Stop()` 返回值未等待。
- 触发场景：长任务执行时停机。
- 根因：缺少 scheduler-level cancel/wait。
- 影响：停机后任务仍可能访问已关闭资源。
- 修复建议：保存 `stopCtx`，`Stop()` 调用 `ctx := s.cron.Stop(); <-ctx.Done()`，任务 context 派生自 scheduler context。

### [P2] 前端 WebSocket 重连累积 ping 定时器

- 位置：`web/static/js/main.js:916-946`
- 现象：每次 `onopen` 都调用 `startPingInterval()`，旧 interval 未清理。
- 触发场景：网络抖动、服务重启、反代断连。
- 根因：未保存 interval id。
- 影响：无意义定时器和重复心跳。
- 修复建议：

```js
let wsPingTimer = null;

function startPingInterval() {
  if (wsPingTimer) clearInterval(wsPingTimer);
  wsPingTimer = setInterval(() => {
    if (wsConnected && wsConnection && wsConnection.readyState === WebSocket.OPEN) {
      wsConnection.send(JSON.stringify({ type: 'ping' }));
    }
  }, 30000);
}
```

### [P2] 批量截图示例按钮包含私网地址，后端拦截后前端无错误处理

- 位置：`web/templates/batch-screenshot.html:79-83`、`web/screenshot_handlers.go:466-479`、`web/templates/batch-screenshot.html:119-136`
- 现象：点击"加载示例"后直接点"开始截图"，页面显示"截图完成"但结果列表为空，无任何错误提示。
- 触发场景：用户使用示例按钮快速体验批量截图功能。
- 根因：示例 URL 列表包含 `192.168.1.1:8080`（私网地址），后端 `isPrivateOrInternalIP` 检查命中后以 JSON 错误响应（`blocked_url`）整批返回；前端 fetch 后未检查 `resp.ok` 或 `data.success`，直接走"截图完成"渲染路径，`(data.results || [])` 为空导致列表无内容。
- 影响：用户首次使用批量截图功能时产生困惑，误以为功能不可用。
- 修复建议：

前端增加响应状态检查：

```js
const resp = await fetch('/api/screenshot/batch-urls', { /* ... */ });
const data = await resp.json();
if (!resp.ok || data.success === false) {
  progressText.textContent = '截图失败: ' + (data.error?.message || data.message || '请求被拒绝');
  btn.disabled = false; btn.textContent = '开始截图';
  return;
}
```

示例 URL 去掉私网地址，或在前端预校验时对私网地址给出提示：

```js
// 示例 URL 改为公网地址
urlList.value = 'https://www.example.com\nhttps://www.baidu.com\nhttp://test.example.org\nhttps://httpbin.org';
```

## 6. 安全与合规性排查

### [P1] 默认登录凭据为 admin/admin

- 位置：`internal/config/config.go:820-831`
- 攻击或泄露场景：生产配置未设置 `password_hash` 时，服务生成默认 `admin/admin`。
- 影响范围：Web 管理后台。
- 修复建议：非 loopback 绑定时禁止默认密码启动。

```go
if isPublicBind(config.Web.BindAddress) && defaultPassword {
    return fmt.Errorf("production must set web.auth.password_hash")
}
```

### [P1] 查询进度广播给所有 WebSocket 连接

- 位置：`web/websocket_handlers.go:404-440`
- 攻击或泄露场景：任意已登录客户端可收到其他客户端的 `query_id/progress_update`。
- 影响范围：多用户部署下的任务侧信道。
- 修复建议：按 `connID -> queryID` 定向发送，不使用全局 broadcast。

### [P2] WebSocket 支持 URL Query Token

- 位置：`web/websocket_handlers.go:169-175`，`web/server.go:751-756`
- 攻击或泄露场景：`/api/ws?token=...` 进入浏览器历史、代理日志。
- 影响范围：Admin Token 泄漏风险。
- 修复建议：保留 Cookie/Header，废弃 query token；至少禁止生产环境 query token。

### [P2] Chrome Extension Origin 默认全放行

- 位置：`web/http_helpers.go:44`，`web/http_helpers.go:212-216`
- 攻击或泄露场景：未配置 `allowed_extension_ids` 时任意扩展 origin 可通过 CORS origin 判断。
- 影响范围：依赖浏览器 Cookie 的管理接口。
- 修复建议：生产模式要求显式配置扩展 ID，空列表只允许开发模式。

### [P2] CSP 仍允许 unsafe-inline 样式

- 位置：`web/server.go:644-645`
- 攻击或泄露场景：样式注入风险低于脚本，但会削弱 CSP。
- 影响范围：前端页面。
- 修复建议：逐步迁移内联 style 到 CSS 文件，收紧 `style-src`。

### 未发现明确问题

- SQL 注入：主要 SQLite 查询使用参数化 `?`，如 `internal/icp/database/result_repository.go:45-95`。
- 路径穿越：截图文件读取有 `filepath.Clean/Rel` 防护，见 `web/screenshot_handlers.go:87-110`。
- Race：`go test -race ./...` 通过。

## 7. 部署、运维与基础设施就绪度

- 健康检查不完整：
  - 位置：`web/server.go:983-991`、`web/health_handlers.go:13-61`
  - 问题：`/health` 只返回 `status/time`；`/health/ready` 未检查 Redis、ICP SQLite、截图后端、通知通道。
  - 建议：补齐关键依赖探测，并区分 `live` 与 `ready`。

- Migration 无回滚：
  - 位置：`internal/icp/database/schema.go:40-98`、`internal/tamper/database/schema.go:37-106`
  - 问题：只有 `CREATE TABLE IF NOT EXISTS`，没有 schema version、down migration、破坏性 DDL 防护。
  - 建议：引入 `schema_migrations` 表，所有 DDL 版本化。

- 静态资源策略不统一：
  - 位置：`nginx.conf:49`、`web/server.go:303`
  - 问题：Nginx 直接 `expires 30d`，应用使用 `staticVersion` 查询参数，两套缓存策略并存。
  - 建议：统一为内容 hash 文件名或统一 query version，并设置 `Cache-Control`。

- Nginx gzip 未配置：
  - 位置：`nginx.conf`
  - 问题：未开启 gzip。
  - 建议：为 HTML/CSS/JS/JSON 开启 gzip。

- 环境隔离无法确认：
  - 依据：配置样例直接指向真实第三方 API 域名，未看到 test/prod 分离配置模板。
  - 需要补充：测试环境第三方 API、Redis、通知 Webhook、截图目录、日志目录的独立配置。

## 8. 项目完成度盘点

整体完成度：约 82%。

| 优先级 | 类型 | 问题 | 影响 | 建议动作 |
|---|---|---|---|---|
| P1 | 部署 | Nginx upstream 端口错误 | 反代全站不可用 | 改 `unimap:8448` |
| P1 | 部署 | Nginx 缺 WebSocket Upgrade | 实时查询失败 | 增加 Upgrade 配置 |
| P1 | 功能 | `/batch-screenshot` 返回 500 | 页面假失败 | 改 `StatusOK` |
| P1 | 生命周期 | 停机先关 DB 再 drain 请求 | 请求中断 | 重排 Shutdown |
| P1 | 安全 | 默认 admin/admin | 后台弱口令 | 生产禁止默认密码 |
| P2 | 状态 | WebSocket 进度全局广播 | 多用户状态串扰 | 定向推送 |
| P2 | 前端 | ping interval 泄漏 | 长时间运行资源浪费 | 保存并清理 timer |
| P2 | 功能 | 批量截图示例含私网地址 + 前端无错误处理 | 用户误判功能不可用 | 去掉私网示例 + 前端检查 resp.ok |
| P2 | 运维 | `/health` 不含关键依赖 | 误判服务健康 | ready 检查 DB/Redis/截图 |
| P2 | API | 无版本化 | 兼容性风险 | 引入 `/api/v1` |
| P2 | 测试 | 缺少真实 Nginx/浏览器 E2E | 反代问题测试未覆盖 | 增加 compose e2e |

已完成核心能力：

- 登录鉴权、CSRF 登录校验、session Cookie。
- 查询编排、缓存、熔断、重试、浏览器 fallback。
- 截图/CDP/扩展桥接。
- 篡改检测、调度器、通知。
- Prometheus 指标。
- 较完整单测与 race 测试。

未完成或疑似未完成：

- Nginx 端到端部署链路未打通。
- 健康检查未覆盖关键依赖。
- API 版本化和 OpenAPI 契约缺失。
- 调度器停机与运行中任务的生命周期控制不足。
- 通知失败缺少持久化补偿。

## 9. 最应该优先修复的 Top 10

1. 修复 `nginx.conf:20` 的 upstream 端口为 `8448`。
2. 给 Nginx 增加 WebSocket Upgrade 代理配置。
3. 将 `web/screenshot_handlers.go:554` 的 `http.StatusInternalServerError` 改为 `http.StatusOK`。
4. 调整 `web/server.go:858-868` 停机顺序：先 drain HTTP，再关 DB/调度器/截图/插件。
5. `internal/scheduler/scheduler.go:1250` 等待 `cron.Stop()`，并让任务 context 可被停机取消。
6. 生产环境禁止默认 `admin/admin`。
7. WebSocket 查询进度改为按连接或订阅关系定向发送。
8. 前端 WebSocket ping interval 增加清理逻辑。
9. `/health/ready` 增加 Redis、ICP DB、截图后端、通知通道等依赖检查。
10. 引入 `/api/v1` 与 OpenAPI/契约测试，覆盖前端、CLI、扩展、Nginx 端到端链路。

### 实施状态

> 以上 10 项已全部实施完成（2026-05-31）。详细实施方案见 `docs/API_VERSIONING.md`。
>
> - 第 1-9 项：提交 `8136620`（`fix: 实施 PROJECT_REVIEW 全部 10 项修复`）
> - 第 10 项：API 版本化全量闭环（双注册 + 前端/CLI/GUI/扩展/脚本/注释迁移，共 55 文件）
> - 第 10 项补充：2026-05-31 全项目扫描修复 6 个脚本 (31 处)、9 个 handler 注释 (20 处)、settings.local.json (3 处)
