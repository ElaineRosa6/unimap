# UniMap 项目全面审查报告

审查日期：2026-06-01 | 审查分支：develop | Go 1.26

验证结果：
- `go build ./...` 通过
- `go test ./...` 通过
- `go test -race ./...` 通过

> 本报告基于 2026-05-31 审查报告的结论，聚焦于**新发现的问题**和**仍未修复的遗留问题**。2026-06-01 FIX_REPORT 中已修复的项不再重复。

---

## 审查计划与检查清单

| # | 审查维度 | 检查重点 |
|---|---------|---------|
| 1 | 项目结构与模块边界 | 包依赖、循环引用、接口设计、目录组织 |
| 2 | 端到端联通性与网络代理 | Nginx/网关配置、端口一致性、WebSocket 路由 |
| 3 | API 契约一致性 | 前后端路径/方法/参数匹配、API 版本化 |
| 4 | 核心业务主流程 | 查询→截图→篡改检测→通知 全链路 |
| 5 | 状态管理 | 前端全局变量/localStorage、后端会话/缓存一致性 |
| 6 | 鉴权与安全策略 | CSP/CORS/Cookie/SameSite/CSRF/SSRF |
| 7 | 输入校验与数据一致性 | 时区处理、null/undefined 处理、枚举值 |
| 8 | 错误处理与降级 | 错误码、前端错误展示、熔断降级 |
| 9 | 并发与数据竞争 | goroutine 泄漏、竞态、事务边界 |
| 10 | 性能与资源泄漏 | N+1、内存泄漏、对象池、慢查询 |
| 11 | 第三方依赖风险 | API 限流、熔断、重试策略 |
| 12 | 部署与运维就绪 | Docker、健康检查、优雅停机、Migration |
| 13 | 测试覆盖与完成度 | 覆盖率、测试质量、E2E |

---

## 1. 架构与系统设计评估

### 1.1 ✅ 模块划分总体良好

`cmd/`(入口) → `web/`(HTTP层) → `internal/service/`(服务层) → `internal/adapter/`(引擎适配) → `internal/core/`(核心引擎)。`internal/` 下 22 个子包职责明确，无循环依赖。

### 1.2 ✅ API 版本控制已实施

`web/router.go:205-210` — `addAPIRoute()` 为每个 API 自动注册 `/api/v1/<path>`（规范）和 `/api/<path>`（旧版，带 `Deprecation: true` + `Sunset: 2026-09-01` 头）。设计合理。

### 1.3 🔴 P1 — Scheduler 任务 context 不响应 Stop 信号

- **位置**：`internal/scheduler/scheduler.go:852`、`internal/scheduler/scheduler.go:1238-1255`
- **问题**：`executeTask()` 使用 `context.Background()` 创建任务 context，与 scheduler 生命周期脱钩。`Stop()` 只停止新任务触发（`cron.Stop()`），但正在执行的长任务（批量截图、篡改检测）收不到取消信号。
- **影响**：优雅停机时，scheduler 的长任务可能访问已关闭的 ICP DB、screenshot router 等资源。
- **状态**：2026-05-31 审查已指出，**仍未修复**。

### 1.4 🟡 P2 — 配置热更新存在并发读写风险

- **位置**：`internal/config/hot_update.go` — `checkConfigChanges` 直接赋值 `h.configManager.config = newConfig`
- **问题**：绕过 `configManager` 的读写锁，其他 goroutine 可能读到部分更新的配置。
- **影响**：热更新期间配置读取短暂不一致。
- **风险**：实际触发概率低（配置更新频率低），但并发语义不正确。

### 1.5 🟡 P2 — 前端全局状态管理混乱

- **位置**：`web/static/js/main.js:764-775`
- **问题**：12 个全局可变变量（`wsConnection`、`wsConnected`、`cdpOnline`、`bridgeOnline` 等）散布在文件中，任何函数都可以修改。
- **影响**：调试困难，状态变更难以追踪。

---

## 2. 核心业务与全链路闭环诊断

### 2.1 ✅ 查询主链路完整

`POST /api/v1/query` → `UnifiedService.Query()` → `EngineOrchestrator` → 各 adapter `Search()` → `ResultMerger.Merge()` → JSON 响应。链路完整，有缓存层、浏览器降级、WebSocket 实时推送。异常流（超时、部分引擎失败）有兜底。

### 2.2 ✅ 截图双模式高可用

`ScreenshotRouter` 双模式（CDP/Extension），HealthChecker 每 30s 探测，自动降级。SSRF 防护到位（`urlguard.Check()`）。

### 2.3 ✅ 篡改检测链路完整

5 种检测模式 + 3 种性能模式，基线存储、历史记录、告警集成完整。

### 2.4 🟡 P2 — BridgeTokenRotateRunner 名不副实

- **位置**：`internal/scheduler/executor.go` — ST-18 `bridge_token` 任务
- **问题**：`BridgeTokenRotateRunner` 实际只做状态检查，不执行令牌轮换。任务名暗示的功能缺失。
- **影响**：如果运维人员依赖 ST-18 做令牌轮换，会发现功能不符预期。

### 2.5 🟡 P2 — 浏览器查询降级"假成功"风险

- **位置**：`internal/adapter/orchestrator.go` — 浏览器降级路径
- **问题**：API 返回空时触发浏览器降级，但如果浏览器降级也返回空（DOM 选择器失效），前端显示"0 条结果"，用户无法区分"真的没有数据"还是"采集失败"。
- **影响**：用户可能误以为目标资产不存在。

---

## 3. 前后端交互、联通性与契约一致性

### 3.1 🔴 P0 — Prometheus 抓取端口错误

- **位置**：`monitoring/prometheus.yml:8`
- **问题**：配置 `targets: ['unimap:8080']`，但服务实际监听 8448（`docker-compose.yml:8`、`cmd/unimap-web/main.go`）。
- **影响**：**Prometheus 完全无法抓取指标数据**，Grafana 仪表板无数据。生产监控失效。
- **修复**：将 `8080` 改为 `8448`。

### 3.2 🔴 P1 — 前端 `logger.error` 未定义导致 ReferenceError

- **位置**：`web/static/js/main.js:2689`
- **代码**：`logger.error(`截图 ${engine} 失败:`, err);`
- **问题**：`logger` 对象在整个 `main.js`（2804 行）中从未声明或初始化。
- **触发场景**：用户执行多引擎搜索引擎截图，当任一引擎截图失败时触发。
- **影响**：`ReferenceError: logger is not defined`，**批量截图进度跟踪功能中断**，后续引擎的截图状态不会更新。
- **修复**：将 `logger.error(...)` 改为 `console.error(...)`。

### 3.3 🔴 P1 — CSP 策略动态 onclick 不兼容

- **位置**：
  - CSP 配置：`web/server.go:644-645`（`script-src 'nonce-...' 'unsafe-hashes'`）
  - onclick 注入：`web/static/js/main.js:1242, 1482, 1262, 1264`
  - 模板中：`web/templates/monitor.html:448`
- **问题**：CSP 允许 nonce-based 脚本和 unsafe-hashes，但通过 `innerHTML` 动态注入的 `onclick` 属性**不受 nonce 保护且无法被 unsafe-hashes 覆盖**（unsafe-hashes 只作用于静态 HTML 中的 inline event handler）。在严格 CSP 下这些 onclick 会被阻止。
- **触发场景**：
  1. 查询结果页面的"截屏所有目标"按钮（`main.js:1262`）
  2. "截屏搜索引擎"按钮（`main.js:1264`）
  3. 错误页面的"返回首页"按钮（`main.js:1242, 1482`）
  4. 监控页面的"删除基线"按钮（`monitor.html:448`）
- **影响**：用户点击这些按钮时无任何反应，功能完全不可用。
- **修复**：将 `innerHTML` + `onclick` 改为事件委托模式。

### 3.4 前后端契约一致性检查表

| 接口 | 前端位置 | 后端位置 | 问题 | 风险 |
|------|---------|---------|------|------|
| `POST /api/v1/query` | `main.js:1166` | `query_handlers.go:129` | ✅ 一致 | — |
| `POST /api/v1/cookies` | `main.js:689` | `cookie_handlers.go:134` | ✅ 一致 | — |
| `POST /api/v1/cdp/connect` | `main.js:277` | `cdp_handlers.go:50` | ✅ 一致 | — |
| `GET /api/v1/screenshot/search-engine` | `main.js:2689` | `screenshot_handlers.go:210` | ⚠️ logger.error 崩溃 | P1 |
| `POST /api/v1/screenshot/batch-urls` | `main.js:1262` | `screenshot_handlers.go:443` | ⚠️ onclick CSP 阻断 | P1 |
| `GET /api/v1/ws` | `main.js:941` | `websocket_handlers.go:49` | ✅ 一致 | — |
| `POST /api/v1/login` | `login.html:84` | `login_handlers.go:47` | ✅ CSRF 保护 | — |
| `POST /api/v1/scheduler/tasks/*` | `scheduler.html` | `scheduler_handlers.go` | ✅ 一致 | — |
| `GET /api/v1/icp/query` | `icp.html` | `icp_handlers.go:50` | ✅ 一致 | — |

### 3.5 ✅ 时区与时间格式

后端统一使用 `time.RFC3339`。`Asia/Shanghai` 在 Docker 环境变量中设置。未发现严重的时区不一致问题。

### 3.6 🟡 P2 — Cookie SameSite 属性未显式设置

- **位置**：`web/session.go` — `setSessionCookie()`
- **问题**：未设置 `SameSite` 属性，依赖浏览器默认值（现代浏览器默认 `Lax`）。
- **影响**：跨站请求场景下 Cookie 可能不被发送（取决于浏览器和部署拓扑）。

---

## 4. 代码质量与缺陷排查

### 问题清单

---

### [P1] 前端事件监听器翻页泄漏

- **位置**：`web/static/js/main.js:1992`（renderPage）、`main.js:2018-2059`（initAssetDetail）
- **现象**：`renderPage()` 每次翻页/切换每页条数时调用 `initAssetDetail()`
- **触发场景**：用户翻页浏览查询结果超过 2 页
- **根因**：`initAssetDetail()` 使用 `querySelectorAll` + `addEventListener` 直接绑定，翻页时 DOM 元素（只是 display:none 隐藏，未移除）被重复绑定
- **影响**：翻 10 页 = 每个按钮绑定了 11 个相同的 click 处理器。用户点击一次触发多次操作；长时间浏览后页面响应变慢。
- **修复建议**：将直接事件绑定改为在 `tbody` 上的**事件委托**（event delegation）
- **示例代码**：
```javascript
// 替换 initAssetDetail() 为一次性的事件委托
function initAssetDetailOnce() {
    const tbody = document.querySelector('#results-tbody');
    if (!tbody || tbody.dataset.delegated) return;
    tbody.dataset.delegated = '1';

    tbody.addEventListener('click', function(e) {
        const btn = e.target.closest('button');
        if (!btn) return;

        if (btn.classList.contains('btn-detail')) {
            showAssetDetail(btn.dataset.ip, btn.dataset.port);
        } else if (btn.classList.contains('btn-copy')) {
            const ip = btn.dataset.ip;
            if (!ip) return;
            copyToClipboard(ip).then(() => showMessage('IP地址已复制到剪贴板', 'success')).catch(() => fallbackCopy(ip));
        } else if (btn.classList.contains('btn-screenshot')) {
            viewScreenshot(btn.dataset.url, btn.dataset.ip, btn.dataset.port, btn.dataset.protocol);
        }
    });
}
// renderPage() 中删除 initAssetDetail() 调用，改为在 initPagination 入口调用一次 initAssetDetailOnce()
```

---

### [P1] 模态窗口关闭按钮导致 window 事件监听器泄漏

- **位置**：`web/static/js/main.js:2120-2130`（showAssetDetail）、`main.js:2380`（openQuotaSettings）
- **现象**：
```javascript
window.addEventListener('click', function handler(e) {
    if (e.target === modal) {
        modal.style.display = 'none';
        window.removeEventListener('click', handler);
    }
});
```
- **触发场景**：用户通过关闭按钮（×）关闭模态窗口
- **根因**：`handler` 只在点击模态外部时移除。关闭按钮的 click 处理器不负责移除这个 window 监听器
- **影响**：每次打开模态窗口泄露一个 window click 监听器。多次打开/关闭后 window 上累积大量无效监听器
- **修复建议**：在关闭按钮的处理器中也移除 `handler`，或给 `addEventListener` 添加 `{ once: true }` 选项（但后者会改变逻辑需谨慎）

---

### [P1] FOFA adapter 认证失败仍触发重试

- **位置**：`internal/adapter/fofa.go` — `Search()` 方法的 `RetryableFunc`
- **现象**：FOFA adapter 的 `RetryableFunc` 对所有错误返回 `true`，包括 HTTP 401 认证失败
- **触发场景**：FOFA API Key 过期或无效时
- **根因**：未像 ZoomEye adapter 那样排除不可重试的错误码（401/402/403）
- **影响**：无效 API Key 被重试 3 次，浪费时间和配额；错误日志中出现 3 条相同的认证失败
- **修复建议**：在 `RetryableFunc` 中排除非临时性错误（401/403）
- **示例代码**：
```go
RetryableFunc: func(err error) bool {
    // 认证错误不重试
    if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
        return false
    }
    return true
},
```

---

### [P2] 前端静默吞错误

- **位置**：`web/static/js/main.js:1704`（checkEngineStatus）、`main.js:239`（initScreenshotModeSelector）
- **现象**：`.catch(() => {})` — 完全静默丢弃错误
- **影响**：引擎状态检查失败时用户无感知；截图模式选择器初始化失败时静默降级
- **修复**：至少加 `console.warn('checkEngineStatus failed:', err)` 便于调试

---

### [P2] 前端输入验证全用 alert()

- **位置**：`web/static/js/main.js` — 至少 15 处 `alert()` 用于输入验证反馈
- **影响**：阻塞式弹窗打断用户操作流，无法定制样式，用户体验差
- **修复**：统一改为 `showMessage(message, 'error')` toast 提示

---

### [P2] CSS @import 性能反模式

- **位置**：`web/static/css/style.css:6`
- **代码**：`@import url('https://fonts.googleapis.com/css2?...');`
- **影响**：创建串行请求链（CSS → @import → 字体 CSS → 字体文件），延长首屏渲染
- **修复**：改为 HTML `<link rel="preconnect">` + `<link rel="stylesheet">`，或在 layout.html 中直接加载

---

### [P2] CSS outline:none 无焦点替代

- **位置**：`web/static/css/style.css:307, 483, 2182`
- **影响**：键盘导航用户无法确定当前聚焦元素
- **修复**：为 `:focus-visible` 添加可见的 box-shadow 或 outline 替代

---

### [P2] 缺少 prefers-reduced-motion 支持

- **位置**：`web/static/css/style.css` — 动画 `modalFadeIn`、`login-glow`、`spin`
- **影响**：对运动敏感的用户可能感到不适
- **修复**：包裹 `@media (prefers-reduced-motion: no-preference) { ... }`

---

## 5. 安全与合规性排查

### [P0] — 未发现新的 P0 安全漏洞

现有 SSRF 防护、安全 Headers、密码 bcrypt 哈希、Session AES-GCM 加密均已正确实施。`config.yaml` 虽含真实凭证但已通过 `.gitignore` 排除版本控制。

---

### [P1] CSP font-src 中国镜像不完整

- **位置**：`web/server.go:644-645`
- **当前**：
```go
"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://fonts.gstatic.font.im;"
"font-src 'self' data: https://fonts.gstatic.com https://fonts.gstatic.font.im;"
```
- **问题**：`font-src` 包含中国可访问的 `fonts.gstatic.font.im`，但 `style-src` 缺少 `fonts.googleapis.font.im`（Google Fonts CSS 的中国镜像）
- **影响**：中国大陆部署时 `fonts.googleapis.com` 不可达，字体加载失败
- **修复**：在 `style-src` 中添加 `https://fonts.googleapis.font.im`

---

### [P1] CORS allow_credentials 默认值与示例不一致

- **位置**：`configs/config.yaml`（`allow_credentials: false`）vs `configs/config.yaml.example`（`allow_credentials: true`）
- **影响**：实际部署时跨域请求无法携带 Cookie，导致认证失败
- **修复**：统一两个文件的默认值（建议统一为生产安全的 `false`，并在注释中说明何时启用）

---

### [P1] API Key 可通过 URL Query 参数传递

- **位置**：`internal/auth/middleware.go` — `RequireAPIKey()` 从 `api_key` query param 提取 key
- **影响**：URL 中的 API Key 会被代理/网关/CDN/浏览器历史记录日志明文记录
- **修复**：在文档中标记此方式为 deprecated，推荐仅使用 Header

---

### [P2] 生产限流默认关闭

- **位置**：`configs/config.yaml` — `rate_limit.enabled: false`
- **影响**：生产部署时如果直接使用默认配置，API 端点无限流保护
- **修复**：生产配置中启用限流（`enabled: true`），开发环境可关闭

---

### ✅ SSRF 防护到位

所有截图/导入/可达性检测/端口扫描 handler 均通过 `urlguard.Check()` 进行 SSRF 防护（DNS 解析 + 私有/环回/链路本地 IP 检查）。实现质量高。

### ✅ 安全 Headers 配置完整

`web/server.go:641-647` — X-Frame-Options: DENY / X-Content-Type-Options: nosniff / Referrer-Policy / Permissions-Policy 均正确设置。

---

## 6. 部署、运维与基础设施就绪度

### [P0] 无数据库 Migration 机制

- **位置**：`web/server.go:326-336`、`internal/icp/database/schema.go`
- **问题**：ICP 数据库使用单次 `InitSchema()` 初始化，无版本管理、无回滚、无环境特定迁移
- **影响**：任何 Schema 变更无法安全发布；多实例部署时可能 Schema 不一致
- **修复**：引入 `golang-migrate/migrate` 或 `pressly/goose`，将 `InitSchema()` 拆分为版本化 up/down migration 文件

---

### [P1] CI golangci-lint 不阻断流水线

- **位置**：`.github/workflows/ci.yml` — `continue-on-error: true`
- **影响**：lint 错误不会阻止 PR 合并，代码质量问题持续累积
- **修复**：改为 `continue-on-error: false`，或至少对 master 分支严格

---

### [P1] 无 Docker 资源限制

- **位置**：`docker-compose.yml` — 未定义 `deploy.resources.limits`
- **影响**：容器可消耗所有宿主机 CPU/内存，OOM Killer 可能随机终止进程
- **修复**：添加 `deploy.resources.limits.memory` 和 `cpus`

---

### [P2] Nginx 静态文件路径未挂载

- **位置**：`nginx.conf` 的 `location /static/ { alias /app/web/static/; }` vs `docker-compose.yml`
- **问题**：docker-compose 中 nginx service 被注释掉了，即使启用也缺少 `/app/web/static` 卷挂载
- **影响**：启用 nginx 后静态文件全部 404

---

### [P2] 停止脚本无优雅超时

- **位置**：`scripts/stop.sh`、`scripts/stop.ps1`
- **代码**：`docker-compose down`（无 `--timeout` 参数）
- **影响**：容器仅获得默认 10 秒的关闭时间，可能不够完成请求排空

---

### [P2] 无 Kubernetes 部署清单

- **位置**：项目根目录
- **问题**：无 Deployment/Service/Ingress/HPA/ConfigMap 等 K8s 清单
- **影响**：无法直接部署到 K8s 集群

---

### [P2] Docker 镜像无漏洞扫描

- **位置**：`.github/workflows/ci.yml` — Docker Build & Push job
- **问题**：未集成 Trivy/Docker Scout 等镜像 CVE 扫描
- **影响**：可能发布包含已知漏洞的基础镜像

---

### ✅ 健康检查完备

`/health`（basic）、`/health/ready`（依赖检查，含 orchestrator/scheduler/distributed/ICP DB/screenshot router/proxy pool，返回 503 表示降级）、`/health/live`（liveness）。Docker HEALTHCHECK 已配置为 30s 间隔。

### ✅ CI/CD 流程完善

多 OS（ubuntu + macos）构建+测试、Race 检测、覆盖率上传、govulncheck 安全扫描、Docker 多标签推送（sha/branch/semver/latest）。

---

## 7. 项目完成度盘点

### 整体完成度：约 82%

| 能力域 | 完成度 | 说明 |
|--------|--------|------|
| 多引擎查询 | 95% | 5 引擎全部完成，UQL 解析/翻译/归并/缓存完备 |
| 截图系统 | 90% | CDP+Extension 双模式高可用，健康检查/自动降级完备 |
| 篡改检测 | 90% | 5 模式+3 性能模式+基线+历史+告警 |
| 定时任务 | 85% | 22 种 Runner 类型，cron+DAG依赖+执行窗口+通知 |
| 分布式 | 80% | 节点注册/心跳/故障转移/任务队列完成，缺 K8s 部署 |
| 告警通知 | 85% | Webhook+DingTalk+WeCom+Feishu，去重/静默/频率控制 |
| Web 前端 | 75% | 功能完整但事件泄漏/CSP不兼容/UX粗糙 |
| CLI | 70% | 基本查询可用，API-first 子命令部分完成 |
| GUI | 60% | 框架完成但 `monitor_native.go` 2150 行过大 |
| 部署运维 | 65% | Docker 化但缺 K8s/资源限制/Migration |
| 测试覆盖 | 80% | CI 包含 race/coverage，E2E 仅 2 个场景 |
| 文档 | 85% | 架构/API/Runbook/ADR/Grafana 仪表板较完整 |

### 已完成的核心能力
- 五大搜索引擎统一查询与归并去重
- CDP/Extension 双模式截图高可用
- 5 种网页篡改检测模式 + 3 性能模式
- 22 种定时任务（含依赖链和执行窗口）
- 分布式节点管理（注册/心跳/故障转移）
- 告警系统（多渠道/去重/静默/频率控制）
- API 版本化（v1 规范 + 旧路径 shim + Sunset 头）
- SSRF 防护、安全 Headers、CSP nonce
- Prometheus 指标 + Grafana 仪表板
- CI/CD 全流程（test/lint/security/Docker push）

### 未完成或疑似未完成
- Kubernetes 部署清单
- 数据库 Migration 框架
- `config.yaml` 凭证外部化（环境变量注入）
- BridgeTokenRotateRunner 实际轮换逻辑
- 前端 CSP onclick 兼容性修复

---

## 8. 总结：最应该优先修复的 Top 10 问题

| 排名 | 严重度 | 问题 | 位置 | 修复难度 | 已在之前审查中指出 |
|------|--------|------|------|---------|-------------------|
| 1 | P0 | Prometheus 抓取端口 8080→8448 | `monitoring/prometheus.yml:8` | 低（1行） | 否 |
| 2 | P0 | 无数据库 Migration 机制 | `web/server.go:326` | 中（需引入工具） | 否 |
| 3 | P1 | 前端 `logger.error` 未定义崩溃 | `web/static/js/main.js:2689` | 低（1行） | 否 |
| 4 | P1 | CSP + innerHTML onclick 不兼容 | `main.js:1242,1482,1262,1264` + `monitor.html:448` | 中 | 否 |
| 5 | P1 | 前端事件监听器翻页泄漏 | `main.js:1992, 2018-2059` | 中 | 否 |
| 6 | P1 | Scheduler 任务不响应 Stop 信号 | `internal/scheduler/scheduler.go:852` | 中 | 是（05-31） |
| 7 | P1 | 生产限流默认关闭 | `configs/config.yaml` | 低（改配置） | 否 |
| 8 | P1 | CI golangci-lint 不阻断 | `.github/workflows/ci.yml` | 低（改配置） | 否 |
| 9 | P1 | FOFA adapter 认证失败仍重试 | `internal/adapter/fofa.go` | 低 | 否 |
| 10 | P1 | CSP font-src 中国镜像不完整 | `web/server.go:644-645` | 低 | 否 |

---

> 本报告基于 `D:\Project\Go_project\unimap` 当前代码（分支 develop，HEAD 7bd32dc），覆盖 22 个 internal 子包、web 层 73 API 路由、前端 2804 行 JS + 2437 行 CSS、Docker/CI/脚本。
