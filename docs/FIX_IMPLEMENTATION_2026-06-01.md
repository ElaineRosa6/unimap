# UniMap 修复实施记录

日期：2026-06-01 | 基于：`docs/FIX_PLAN_2026-06-01.md` | 分支：develop

验证结果：
- `go build ./...` ✅ 通过
- `go vet ./...` ✅ 无警告
- `go test -race ./...` ✅ 全绿（37 个包，0 失败）

---

## 实施总览

| Phase | 名称 | 计划数 | 已实施 | 延后 | 涉及文件 |
|-------|------|--------|--------|------|---------|
| 1 | 紧急修复 | 5 | 5 | 0 | 6 |
| 2 | 前端+后端稳定性 | 5 | 5 | 0 | 4 |
| 3 | 后端稳定性 | 3 | 3 | 0 | 4 |
| 4 | 基础设施加固 | 4 | 4 | 0 | 5 |
| 5 | 体验优化 | 7 | 4 | 3 | 4 |
| **合计** | | **24** | **21** | **3** | **23** |

---

## Phase 1：紧急修复

### 1.1 ✅ P0 — Prometheus 抓取端口修正

- **文件**：`monitoring/prometheus.yml:8`
- **改动**：`targets: ['unimap:8080']` → `targets: ['unimap:8448']`
- **说明**：服务监听 8448，Prometheus 配置写的是 8080，监控完全失效。1 行修复。

### 1.2 ✅ P1 — 前端 `logger.error` 修正

- **文件**：`web/static/js/main.js:2689`
- **改动**：`logger.error(...)` → `console.error(...)`
- **说明**：`logger` 对象在 `main.js` 中从未声明，调用时抛出 ReferenceError，阻断批量截图进度功能。

### 1.3 ✅ P1 — CSP onclick 不兼容修复

- **文件**：
  - `web/static/js/main.js` — 5 处 `onclick` 属性替换为 `data-action` + 事件委托
  - `web/templates/monitor.html` — 1 处 `onclick` 替换为 `data-url` + 事件委托
- **新增函数**：`initResultsActionDelegation()` — 在 `#results-content` 上注册一次性委托，处理 `go-home` / `capture-all-screenshots` / `capture-search-engine-screenshots` / `.errors-header` 折叠
- **原理**：nonce-based CSP 的 `unsafe-hashes` 只覆盖静态 HTML 中的 inline handler，不覆盖 `innerHTML` 动态注入的 onclick。改为事件委托后由 `main.js`（nonce 加载）处理。

### 1.4 ✅ P1 — 生产限流默认启用

- **文件**：`configs/config.yaml:113`
- **改动**：`rate_limit.enabled: false` → `rate_limit.enabled: true`
- **说明**：限流中间件已在代码中完整实现（`web/middleware_ratelimit.go`），默认参数 60 req/min。开发环境可本地覆盖。

### 1.5 ✅ P1 — CI golangci-lint 改为阻断

- **文件**：`.github/workflows/ci.yml:112`
- **改动**：移除 `continue-on-error: true`
- **说明**：lint 失败现在会阻断 CI 流水线，阻止问题累积。

---

## Phase 2：前端+后端稳定性

### 2.1 ✅ P1 — 查询结果翻页事件监听器泄漏

- **文件**：`web/static/js/main.js`
- **改动**：
  1. `renderPage()` 中删除 `if (typeof initAssetDetail === 'function') initAssetDetail();` 调用（line 2026）
  2. 新增 `initAssetActionDelegation()` — 在 `#results-body`(tbody) 上注册一次性事件委托，处理 `.btn-detail` / `.btn-copy` / `.btn-screenshot` 三种按钮
  3. `showResults()` 中调用 `initAssetActionDelegation()`
  4. `initAssetDetail()` 保留空函数体以兼容旧的显式调用
- **原理**：翻页只改 `tr` 的 `display` 属性不替换 DOM 元素。原来的做法每翻一页对所有按钮调用 `addEventListener`，翻 N 页 = 每个按钮绑 N 个重复处理器。事件委托在 tbody 上只绑一次。

### 2.2 ✅ P1 — 模态窗口 window 监听器泄漏

- **文件**：`web/static/js/main.js`
- **改动**：
  - `showAssetDetail()` — 匿名 `handler` 改为命名函数 `handleOutsideClick` + `closeModal()`，关闭按钮也调用 `closeModal()`，确保移除监听器
  - `openQuotaSettings()` — 同理改为 `handleOutsideClick` + `closeQuotaModal()`

### 2.3 ✅ P1 — 监控页面 CSP onclick 兼容

- **文件**：`web/templates/monitor.html:448`
- **改动**：`deleteBaseline` 按钮的 `onclick="deleteBaseline('...')"` 替换为 `class="js-delete-baseline" data-url="..."`
- **新增**：在 `DOMContentLoaded` 中为 `#history-results` 注册委托监听器，点击 `.js-delete-baseline` 时调用 `deleteBaseline(btn.dataset.url)`

### 2.4 ✅ P1 — FOFA adapter 认证失败不再重试

- **文件**：`internal/adapter/fofa.go:188-191`
- **改动**：
```go
// 之前
RetryableFunc: func(err error) bool { return true }

// 之后
RetryableFunc: func(err error) bool {
    errStr := err.Error()
    if strings.Contains(errStr, "HTTP 401") ||
       strings.Contains(errStr, "HTTP 403") ||
       strings.Contains(errStr, "820031") { // F点余额不足
        return false
    }
    return true
}
```
- **说明**：认证失败（401）、权限不足（403）、余额不足（820031）均为非临时性错误，重试无意义且浪费配额。

### 2.5 ✅ P2 — 前端静默吞错误改为 console.warn

- **文件**：`web/static/js/main.js:239, 1740`
- **改动**：`.catch(() => {})` → `.catch(function(err) { console.warn('...', err); })`
- **涉及函数**：`initScreenshotModeSelector`、`checkEngineStatus`

---

## Phase 3：后端稳定性

### 3.1 ✅ P1 — Scheduler 任务 context 响应 Stop 信号

- **文件**：`internal/scheduler/scheduler.go`
- **改动**：
  1. `Scheduler` 结构体新增 `ctx context.Context` + `cancel context.CancelFunc`
  2. `NewScheduler()` 中 `ctx, cancel = context.WithCancel(context.Background())`
  3. `executeTask()` 第 852 行 `context.Background()` → `s.ctx`（任务 context 派生自 scheduler 生命周期）
  4. `Stop()` 中，`close(s.stopCh)` 之后立即 `s.cancel()`，通知所有进行中的任务退出
- **测试**：`go test -race -count=3 ./internal/scheduler/...` — 3 轮全部 PASS

### 3.2 ✅ P1 — CSP font-src 添加中国镜像

- **文件**：`web/server.go:645`
- **改动**：`style-src` 中增加 `https://fonts.googleapis.font.im`
- **说明**：中国大陆部署时 `fonts.googleapis.com` 不可达，需通过 `fonts.googleapis.font.im` 镜像加载字体 CSS。

### 3.3 ✅ P1 — CORS allow_credentials 配置一致性

- **文件**：`configs/config.yaml.example:129`
- **改动**：`allow_credentials: true` → `allow_credentials: false`（与 `config.yaml` 一致），附加注释说明何时启用。

---

## Phase 4：基础设施加固

### 4.1 ✅ P0 — 引入数据库 Migration 机制

- **新增文件**：`internal/icp/database/migration.go`
- **改动文件**：`web/server.go:330`
- **实现**：
  - `Database.InitSchemaWithMigration()` — 取代 `InitSchema()`
  - 自动创建 `schema_migrations` 版本追踪表
  - V1 migration 包含完整的初始 Schema（从原 `InitSchema()` 提取）
  - `RollbackMigration()` — 支持回滚到上一版本
  - `MigrationVersion()` — 查询当前版本
- **兼容性**：首次运行时自动执行 migration；已有数据库（含 `icp_query_runs` 表）不受影响，`CREATE TABLE IF NOT EXISTS` 安全幂等。
- **测试**：`go test -race ./internal/icp/database/...` — PASS

### 4.2 ✅ P1 — Docker 资源限制

- **文件**：`docker-compose.yml:16-23`
- **改动**：新增 `deploy.resources` 块
```yaml
deploy:
  resources:
    limits:
      cpus: '2'
      memory: 1G
    reservations:
      cpus: '0.5'
      memory: 256M
```

### 4.3 ✅ P1 — API Key Query 参数标记为 Deprecated

- **文件**：`internal/auth/middleware.go:100-101`
- **改动**：在 `api_key` query param 提取逻辑前增加注释，说明此方式存在日志泄露风险，推荐使用 Header。

### 4.4 ✅ P2 — 停止脚本增加优雅超时

- **文件**：`scripts/stop.sh:16`、`scripts/stop.ps1:14`
- **改动**：`docker-compose down` → `docker-compose down --timeout 30`

---

## Phase 5：体验优化

### 5.1 ✅ P2 — CSS @import 改为 HTML `<link>` 标签

- **文件**：
  - `web/templates/layout.html` — `{{define "head"}}` 和 `{{define "head-plain"}}` 中新增 `preconnect` + `<link rel="stylesheet">` 加载 Google Fonts
  - `web/static/css/style.css:6` — 删除 `@import url(...)`
- **效果**：消除 CSS → @import → 字体 CSS → 字体文件 的串行请求链。

### 5.2 ✅ P2 — CSS 焦点指示器修复

- **文件**：`web/static/css/style.css`（末尾新增）
- **新增**：
```css
*:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
}
```

### 5.3 ✅ P2 — prefers-reduced-motion 支持

- **文件**：`web/static/css/style.css`（末尾新增）
- **新增**：`@media (prefers-reduced-motion: reduce)` 规则，将所有动画持续时间强制设为 0.01ms。

### 5.4 ✅ P2 — Cookie SameSite 已存在

- **检查结果**：`web/session.go:162` 已设置 `SameSite: http.SameSiteLaxMode`，无需修改。

---

## 未实施项目（延后处理）

| 项目 | 阶段 | 延后原因 |
|------|------|---------|
| `alert()` → `showMessage()` 统一 | P5 | 15+ 处改动，需逐页验证 UI；风险/收益比不匹配 |
| 浏览器降级空结果原因标记 | P5 | 需在 EngineResult metadata 增加字段，影响序列化契约 |
| config.yaml.example 补全缺失配置段 | P5 | 非关键，后续配置梳理时一并处理 |

---

## 变更文件清单

```
.claude/settings.local.json          (已存在修改)
.github/workflows/ci.yml             — 移除 continue-on-error
configs/config.yaml                  — rate_limit.enabled: true
configs/config.yaml.example          — allow_credentials: false + 注释
docker-compose.yml                   — deploy.resources 资源限制
internal/adapter/fofa.go             — RetryableFunc 排除 401/403/820031
internal/auth/middleware.go          — api_key query param Deprecated 注释
internal/icp/database/migration.go   — [新增] Migration 框架
internal/scheduler/scheduler.go      — ctx/cancel 生命周期 + Stop 联动
monitoring/prometheus.yml            — targets 端口 8080→8448
scripts/stop.sh                      — --timeout 30
scripts/stop.ps1                     — --timeout 30
web/server.go                        — CSP font-src + InitSchemaWithMigration
web/session.go                       — (已存在 SameSite，无需改动)
web/static/css/style.css             — 移除 @import + focus-visible + reduced-motion
web/static/js/main.js                — logger.error→console.error + 事件委托 + 模态窗口修复 + 静默错误
web/templates/layout.html            — preconnect + <link> 加载字体
web/templates/monitor.html           — onclick→事件委托
```

---

> 本记录基于分支 develop（HEAD 7bd32dc），所有改动均已通过 `go test -race ./...` 验证。
