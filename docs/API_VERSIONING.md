# API 版本化实施方案

日期：2026-05-31

状态：已实施

## 1. 背景

审查报告 `PROJECT_REVIEW_2026-05-31.md` 指出：所有 API 接口集中在 `/api/...`，没有版本边界。字段名、状态码或返回结构调整会直接影响 Web、CLI、扩展和脚本。

## 2. 方案选型

| 方案 | 优点 | 缺点 | 结论 |
|------|------|------|------|
| URL 路径版本 `/api/v1/...` | 直观、易路由、易测试 | 路径变长 | ✅ 采用 |
| Header 版本 `Accept: application/vnd.unimap.v1+json` | 路径不变 | 不直观、难调试、前端需统一处理 | ❌ |
| Query 参数 `?version=1` | 简单 | 缓存不友好、易遗漏 | ❌ |

## 3. 实施策略：双注册 + 旧路径 Shim

核心原则：**不破坏现有调用方，新旧路径并存**。

### 3.1 路由层

在 `web/router.go` 中新增两个辅助：

```go
// deprecateMiddleware 为旧路径注入废弃响应头
func deprecateMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Deprecation", "true")
        w.Header().Set("Sunset", "2026-09-01")
        next(w, r)
    }
}

// addAPIRoute 同时注册 /api/v1/...（正式）和 /api/...（旧路径，带废弃头）
func (r *Router) addAPIRoute(name, method, apiPath string, handler http.HandlerFunc, rateLimited bool) {
    v1Path := "/api/v1" + strings.TrimPrefix(apiPath, "/api")
    r.addRoute(name, method, v1Path, handler, rateLimited)
    r.addRoute(name+"-legacy", method, apiPath, deprecateMiddleware(handler), rateLimited)
}
```

每条 API 路由调用 `addAPIRoute` 后自动产生两条注册：

| 路径 | Handler | 响应头 |
|------|---------|--------|
| `/api/v1/query` | `handleAPIQuery` | 无额外头（正式） |
| `/api/query` | `deprecateMiddleware(handleAPIQuery)` | `Deprecation: true` + `Sunset: 2026-09-01` |

### 3.2 路由分类

**使用 `addAPIRoute` 的 API 路由（73 条，双注册产生 146 条 mux 条目）：**

| 模块 | 路径前缀 | 数量 |
|------|----------|------|
| 登录登出 | `/api/login`, `/api/logout` | 2 |
| 查询 | `/api/query`, `/api/query/status` | 2 |
| Cookie | `/api/cookies/*` | 4 |
| CDP | `/api/cdp/*` | 2 |
| WebSocket | `/api/ws` | 1 |
| 截图 | `/api/screenshot/*` | 17 |
| 导入/URL | `/api/import/*`, `/api/url/*` | 3 |
| 分布式节点 | `/api/nodes/*` | 12 |
| 定时任务 | `/api/scheduler/*` | 9 |
| 通知 | `/api/notifications/*` | 5 |
| 篡改检测 | `/api/tamper/*` | 6 |
| 备份 | `/api/backup/*` | 2 |
| 账号 | `/api/account/*` | 1 |
| ICP | `/api/icp/*` | 5 |
| 配置 | `/api/config` | 2 |

**保持 `addRoute` 的非 API 路由（17 条）：**

页面路由（`/`, `/results`, `/quota`, `/account`, `/batch-screenshot`, `/monitor`, `/scheduler`, `/icp`, `/settings` — 9 条）、登录页面（`/login` — 1 条）、基础设施路由（`/health`, `/health/ready`, `/health/live`, `/metrics` — 4 条）、功能路由（`/query`, `/screenshots/` — 2 条）、静态文件（`/static/` — 1 条）。

### 3.3 CSP 兼容性

当前 CSP 策略：

```
default-src 'self'; script-src 'self' 'nonce-...' 'unsafe-hashes'; ...
```

无 `connect-src` 指令，回退到 `default-src 'self'`。`/api/v1/...` 与 `/api/...` 同源（scheme + host + port 相同），路径差异不影响 CSP 判定。**无需修改 CSP 配置。**

## 4. 前端迁移

所有 `fetch('/api/...')` 调用已迁移到 `fetch('/api/v1/...')`。

### 4.1 main.js（16 处）

| 原路径 | 新路径 | 调用数 |
|--------|--------|--------|
| `/api/screenshot/set-mode` | `/api/v1/screenshot/set-mode` | 1 |
| `/api/screenshot/bridge/status` | `/api/v1/screenshot/bridge/status` | 1 |
| `/api/cookies` | `/api/v1/cookies` | 3 |
| `/api/cdp/status` | `/api/v1/cdp/status` | 1 |
| `/api/cdp/connect` | `/api/v1/cdp/connect` | 1 |
| `/api/cookies/import` | `/api/v1/cookies/import` | 1 |
| `/api/cookies/verify` | `/api/v1/cookies/verify` | 1 |
| `/api/cookies/login-status` | `/api/v1/cookies/login-status` | 1 |
| `/api/ws` | `/api/v1/ws` | 1 |
| `/api/query` | `/api/v1/query` | 1 |
| `/api/config` | `/api/v1/config` | 1 |
| `/api/screenshot` | `/api/v1/screenshot` | 1 |
| `/api/screenshot/search-engine` | `/api/v1/screenshot/search-engine` | 1 |
| `/api/screenshot/batch-urls` | `/api/v1/screenshot/batch-urls` | 1 |

### 4.2 模板文件

| 文件 | 改动数 | 涉及模块 |
|------|--------|----------|
| `settings.html` | 16 | config, cookies, cdp, icp, screenshot, notifications |
| `scheduler.html` | 9 | scheduler tasks CRUD, history, notifications |
| `monitor.html` | 7 | tamper check/baseline/history/delete, screenshot batch |
| `batch-screenshot.html` | 1 | screenshot batch-urls |
| `icp.html` | 1 | icp query |
| `login.html` | 1 | login POST |
| `account.html` | 1 | change-password POST |
| `layout.html` | 1 | logout form action |

注意事项：`settings.html` 中 Quake 引擎的 `https://quake.360.net/api/v1` 占位符属于第三方 API，未被替换。

## 5. CLI 迁移

文件：`cmd/unimap-cli/api_subcommands.go`

| 原路径 | 新路径 |
|--------|--------|
| `/api/query` | `/api/v1/query` |
| `/api/tamper/check` | `/api/v1/tamper/check` |
| `/api/screenshot/batch-urls` | `/api/v1/screenshot/batch-urls` |
| `/api/scheduler` 前缀 | `/api/v1/scheduler` 前缀（影响 tasks/run/history 等子路径） |

## 6. Chrome 扩展迁移

| 文件 | 原路径 | 新路径 |
|------|--------|--------|
| `src/background.js:25` | `/api/screenshot/bridge/tasks/next` | `/api/v1/screenshot/bridge/tasks/next` |
| `src/background.js:30` | `/api/screenshot/bridge/mock/result` | `/api/v1/screenshot/bridge/mock/result` |
| `src/pairing.js:9` | `/api/screenshot/bridge/pair` | `/api/v1/screenshot/bridge/pair` |
| `src/api.js:101` | `/api/screenshot/bridge/token/rotate` | `/api/v1/screenshot/bridge/token/rotate` |

## 7. GUI 迁移

文件：`cmd/unimap-gui/monitor_native.go`（11 处）

| 原路径 | 新路径 |
|--------|--------|
| `/api/tamper/check` | `/api/v1/tamper/check` |
| `/api/tamper/baseline` | `/api/v1/tamper/baseline` |
| `/api/tamper/baseline/list` | `/api/v1/tamper/baseline/list` |
| `/api/tamper/baseline/delete` | `/api/v1/tamper/baseline/delete` |
| `/api/tamper/history` | `/api/v1/tamper/history` |
| `/api/tamper/history/delete` | `/api/v1/tamper/history/delete` |
| `/api/screenshot/batches` | `/api/v1/screenshot/batches` |
| `/api/screenshot/batches/files` | `/api/v1/screenshot/batches/files` |
| `/api/screenshot/batches/delete` | `/api/v1/screenshot/batches/delete` |
| `/api/screenshot/file/delete` | `/api/v1/screenshot/file/delete` |
| `/api/screenshot/batch-urls` | `/api/v1/screenshot/batch-urls` |

## 8. 测试文件迁移

20 个测试文件共 199 处 `httptest.NewRequest` URL 迁移：

| 测试文件 | 改动数 |
|----------|--------|
| `screenshot_handlers_test.go` | 27 |
| `scheduler_handlers_test.go` | 26 |
| `node_handlers_test.go` | 14 |
| `node_task_handlers_test.go` | 13 |
| `tamper_handlers_test.go` | 13 |
| `node_auth_test.go` | 18 |
| `monitor_handlers_test.go` | 11 |
| `bridge_handlers_test.go` | 10 |
| `icp_handlers_test.go` | 10 |
| `query_handlers_test.go` | 9 |
| `cookie_handlers_test.go` | 9 |
| `query_cookie_helpers_test.go` | 9 |
| `screenshot_bridge_handlers_test.go` | 8 |
| `backup_handlers_test.go` | 5 |
| `config_handlers_test.go` | 4 |
| `middleware_audit_test.go` | 4 |
| `cdp_handlers_test.go` | 3 |
| `misc_handlers_test.go` | 3 |
| `middleware_auth_test.go` | 2 |
| `cdp_util_test.go` | 1 |

由于双注册机制，旧路径测试仍然能通过，但统一到 `/api/v1/...` 保持一致性。

## 9. 中间件兼容性修复

双注册后，部分中间件通过 `strings.HasPrefix` 匹配请求路径做特殊处理。如果只匹配 `/api/v1/...`，旧路径的请求会绕过这些逻辑。已修复：

### `web/middleware_auth.go`

| 函数 | 问题 | 修复 |
|------|------|------|
| `isScreenshotBridgePath()` | 只匹配 `/api/v1/screenshot/bridge/` | 增加 `/api/screenshot/bridge/` 匹配 |
| `isPublicPath()` 公共前缀 | 只有 `/api/v1/screenshot/bridge/` | 增加 `/api/screenshot/bridge/` |
| `isPublicPath()` 精确匹配 | 只有 `/api/login`, `/api/logout` | 增加 `/api/v1/login`, `/api/v1/logout` |

### `web/server.go`

| 位置 | 问题 | 修复 |
|------|------|------|
| WebSocket auth 检查 (line 748) | 只匹配 `/api/ws` | 改为 `r.URL.Path == "/api/v1/ws" \|\| r.URL.Path == "/api/ws"` |

**原则：** 所有通过路径做判断的中间件逻辑，必须同时匹配新旧两个路径，否则旧路径请求会丢失 auth bypass、CORS 豁免等特殊处理。

## 10. Sunset 计划

| 阶段 | 时间 | 动作 |
|------|------|------|
| 当前 | 2026-05-31 | 双注册，旧路径带 `Deprecation` 头 |
| 观察期 | 2026-06 ~ 2026-08 | 监控旧路径调用量，确认无活跃调用方 |
| 移除 | 2026-09-01 | 删除旧路径注册和 `deprecateMiddleware` |

## 11. 验证结果

- `go build ./...` ✅
- `go vet ./...` ✅
- `go test -race ./...` ✅（已知 flaky test `TestHandleURLPortScan_NoMonitorApp_Returns503` 为预存在问题）

## 12. 变更文件清单

| 层 | 文件 | 改动类型 |
|----|------|----------|
| 路由 | `web/router.go` | 新增 `addAPIRoute` + `deprecateMiddleware`，62 条路由双注册 |
| 中间件 | `web/middleware_auth.go` | bridge/login/logout 路径同时匹配新旧 |
| 服务 | `web/server.go` | WebSocket 路径匹配新旧 |
| 前端 | `web/static/js/main.js` | 16 处 URL 迁移 |
| 前端 | `web/templates/settings.html` | 16 处 URL 迁移 |
| 前端 | `web/templates/scheduler.html` | 9 处 URL 迁移 |
| 前端 | `web/templates/monitor.html` | 7 处 URL 迁移 |
| 前端 | `web/templates/batch-screenshot.html` | 1 处 URL 迁移 |
| 前端 | `web/templates/icp.html` | 1 处 URL 迁移 |
| 前端 | `web/templates/login.html` | 1 处 URL 迁移 |
| 前端 | `web/templates/account.html` | 1 处 URL 迁移 |
| 前端 | `web/templates/layout.html` | 1 处 URL 迁移 |
| CLI | `cmd/unimap-cli/api_subcommands.go` | 4 处 URL 迁移 |
| GUI | `cmd/unimap-gui/monitor_native.go` | 11 处 URL 迁移 |
| 扩展 | `tools/extension-screenshot/src/background.js` | 2 处 URL 迁移 |
| 扩展 | `tools/extension-screenshot/src/pairing.js` | 1 处 URL 迁移 |
| 扩展 | `tools/extension-screenshot/src/api.js` | 1 处 URL 迁移 |
| 测试 | 20 个 `web/*_test.go` 文件 | 199 处 URL 迁移 |
| 脚本 | `scripts/bridge_e2e.ps1` | 10 处 URL 迁移 |
| 脚本 | `scripts/rollback_extension_to_cdp.ps1` | 2 处 URL 迁移 |
| 脚本 | `scripts/rollback_extension_to_cdp.sh` | 2 处 URL 迁移 |
| 脚本 | `scripts/load_test.sh` | 3 处 URL 迁移 |
| 脚本 | `scripts/day15_distributed_e2e.sh` | 7 处 URL 迁移 |
| 脚本 | `scripts/day15_distributed_e2e.ps1` | 7 处 URL 迁移 |
| 注释 | `web/backup_handlers.go` | 2 处注释更新 |
| 注释 | `web/config_handlers.go` | 2 处注释更新 |
| 注释 | `web/cookie_handlers.go` | 1 处注释更新 |
| 注释 | `web/icp_handlers.go` | 5 处注释更新 |
| 注释 | `web/login_handlers.go` | 2 处注释更新 |
| 注释 | `web/node_handlers.go` | 2 处注释更新 |
| 注释 | `web/node_task_handlers.go` | 2 处注释更新 |
| 注释 | `web/notification_handlers.go` | 3 处注释更新 |
| 注释 | `web/query_handlers.go` | 1 处注释更新 |
| 配置 | `.claude/settings.local.json` | 3 处 URL 迁移 |
| 文档 | `docs/API_VERSIONING.md` | 新建 |
| 文档 | `CLAUDE.md` | 索引更新 |
| 文档 | `docs/PROJECT_REVIEW_2026-05-31.md` | 实施状态标记 |
| **合计** | **55 文件** | **+447 -383** |
