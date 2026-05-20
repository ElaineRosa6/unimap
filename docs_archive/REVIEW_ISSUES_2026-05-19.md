# UniMap 代码审查问题清单（核查修订版）

> 原审查日期: 2026-05-19
> 核查日期: 2026-05-19
> 核查依据: 当前仓库代码、`go test ./internal/config ./internal/service ./web`
> 核查结论: 多数问题真实存在，但原文中部分严重级别和影响范围偏重，优化计划需按本修订版调整。

---

## 核查摘要

| 结论 | 数量 | 编号 |
|------|------|------|
| **确认存在，建议修复** | 13 | SEC-001, FUNC-001~005, EDGE-001~004, UX-ICP-001, ARCH-001, ARCH-003 |
| **部分成立，需修正描述/方案** | 3 | ARC-001, EDGE-005, ARCH-004 |
| **架构债务，长期处理** | 1 | ARCH-002 |

当前相关包测试通过：

```bash
go test ./internal/config ./internal/service ./web
```

---

## P1 - 功能、安全与稳定性问题

### SEC-001: 登录 Token 比较未统一使用恒定时间比较

| 属性 | 内容 |
|------|------|
| **严重级别** | P1 |
| **类别** | 安全加固 |
| **文件** | `web/login_handlers.go:27` |
| **核查结论** | 真实存在 |
| **状态** | 待修复 |

`handleLoginPage` 在处理已认证用户跳转时使用 `strings.Compare`：

```go
token := r.Header.Get("X-Admin-Token")
if token != "" && s.adminToken() != "" {
    if strings.Compare(token, s.adminToken()) == 0 {
        http.Redirect(w, r, "/", http.StatusFound)
        return
    }
}
```

同文件已有 `secureCompare(a, b string) bool`。认证中间件中也已使用 `subtle.ConstantTimeCompare`，因此这里应保持一致。

**修复建议:** 将 `strings.Compare(token, s.adminToken()) == 0` 替换为 `secureCompare(token, s.adminToken())`。原文标为 P0 偏重，当前更适合归为 P1 安全加固。

---

### FUNC-001: API Key 验证为空壳实现

| 属性 | 内容 |
|------|------|
| **严重级别** | P1 |
| **类别** | 功能未实现 |
| **文件** | `web/config_handlers.go:354-356` |
| **核查结论** | 真实存在 |
| **状态** | 待修复 |

```go
func validateEngineAPIKey(engine, apiKey string) (bool, string) {
    return true, fmt.Sprintf("API key format for %s appears valid", engine)
}
```

该函数对任意输入都返回成功。用户在设置页无法提前发现错误凭证。

**修复建议:**

- 改为 `Server` 方法或传入当前配置，避免只拿到 `engine` 和 `apiKey` 后无法获取 FOFA email、BaseURL、timeout 等信息。
- 按项目当前适配器和各引擎实际 API 约定实现验证，不要直接照搬原计划中的端点。
- 至少对空值、未知引擎、HTTP 非 2xx、平台返回认证错误、网络超时给出明确消息。

---

### FUNC-002: 引擎连通性测试为空壳实现

| 属性 | 内容 |
|------|------|
| **严重级别** | P1 |
| **类别** | 功能未实现 |
| **文件** | `web/config_handlers.go:358-360` |
| **核查结论** | 真实存在 |
| **状态** | 待修复 |

```go
func testEngineHealth(engine string) (bool, string) {
    return true, fmt.Sprintf("%s connection test passed", engine)
}
```

该函数对所有引擎都返回成功。

**修复建议:** 改为读取当前配置中的 BaseURL、API Key、email、timeout 等，发起真实健康检查。当前签名 `testEngineHealth(engine string)` 信息不足，应改为 `Server` 方法或显式传入 `*config.Config`。

---

### FUNC-003: RunBrowserQueryAsync 缺少 panic recover

| 属性 | 内容 |
|------|------|
| **严重级别** | P1 |
| **类别** | 进程稳定性 |
| **文件** | `internal/service/query_app_service.go:118` |
| **核查结论** | 真实存在 |
| **状态** | 待修复 |

`RunBrowserQueryAsync` 启动 goroutine 后只 `defer close(resultCh)`，没有 `recover`。如果浏览器路由、截图服务或底层依赖发生 panic，会导致整个进程退出。

**修复建议:** 在 goroutine 内增加 `defer recover()`，将 panic 转成 `BrowserQueryOutcome.Errors`。注意实现时应先声明 `outcome`，再注册 `recover`，否则示例代码会有作用域问题。

---

### FUNC-004: cleanupStaleQueries 删除前未二次校验

| 属性 | 内容 |
|------|------|
| **严重级别** | P2 |
| **类别** | 低概率竞态 |
| **文件** | `web/server.go:780-803` |
| **核查结论** | 真实存在，但原严重级别偏高 |
| **状态** | 待修复 |

当前实现先在 `RLock` 下收集过期 ID，再在 `Lock` 下删除：

```go
s.queryMutex.RLock()
for id, status := range s.queryStatus {
    if now.Sub(status.StartTime) > maxAge {
        staleIDs = append(staleIDs, id)
    }
}
s.queryMutex.RUnlock()

s.queryMutex.Lock()
for _, id := range staleIDs {
    delete(s.queryStatus, id)
}
s.queryMutex.Unlock()
```

理论上，如果相同 queryID 在两个锁窗口之间被复用，新查询可能被误删。当前 queryID 基于 `time.Now().UnixNano()`，碰撞概率很低，因此不应按 P1 处理。

**修复建议:** 删除前重新读取并检查 `StartTime`：

```go
now := time.Now()
s.queryMutex.Lock()
for _, id := range staleIDs {
    if st := s.queryStatus[id]; st != nil && now.Sub(st.StartTime) > maxAge {
        delete(s.queryStatus, id)
    }
}
s.queryMutex.Unlock()
```

---

### FUNC-005: 配置更新缺少统一并发保护

| 属性 | 内容 |
|------|------|
| **严重级别** | P1 |
| **类别** | 并发安全 / 配置一致性 |
| **文件** | `web/config_handlers.go:32-85`, `web/config_handlers.go:261-352`, `internal/config/config.go:500` |
| **核查结论** | 真实存在，原描述需扩大到整个配置更新链路 |
| **状态** | 待修复 |

`handleUpdateConfig` 通过 `s.configManager.GetConfig()` 取得配置指针后直接原地修改，`applyEngineConfig` 等函数逐字段写入。`config.Manager.GetConfig()` 也未加锁。

影响不仅限于 `applyEngineConfig`，还包括 `applyICPConfig`、`applyWebConfig`、`applyScreenshotConfig` 等所有设置页更新路径。

**修复建议:**

- 为 `config.Manager` 增加统一的读写保护，或用 `atomic.Value` 管理不可变配置快照。
- 更新配置时先 clone，修改副本，校验副本，再通过 `Manager.Update(fn)` 或 `Manager.SetConfig()` 一次性发布。
- 仅 `*cfg = *cloned` 不是真正的并发安全原子替换，仍需统一读写协议。

---

## P2 - 局部缺陷与维护性问题

### EDGE-001: `/api/screenshot` 绕过 ScreenshotRouter

| 属性 | 内容 |
|------|------|
| **严重级别** | P2 |
| **类别** | 资源复用 / 行为一致性 |
| **文件** | `web/screenshot_handlers.go:126-206` |
| **核查结论** | 真实存在 |
| **状态** | 待修复 |

`handleScreenshot` 直接创建 `chromedp.NewExecAllocator` 并返回 PNG bytes，不经过 `ScreenshotRouter`。同文件其他截图接口如 `handleTargetScreenshot`、`handleBatchURLsScreenshot` 已优先使用 `s.screenshotRouter`。

**影响:**

- 单张截图路径无法复用 router 的 CDP/Extension 选择和健康降级。
- 可能额外启动 Chrome 进程。
- 行为与 `/api/screenshot/target` 等接口不一致。

**修复建议:** 将 `/api/screenshot` 适配到 `s.screenshotRouter.CaptureTargetWebsite`，或复用 `ScreenshotAppService`。需要决定响应保持 PNG bytes 还是改为 JSON path；若保持兼容，应捕获后读取文件并返回图片。

---

### EDGE-002: TamperAppService 多处重复创建 Detector

| 属性 | 内容 |
|------|------|
| **严重级别** | P2 |
| **类别** | 性能 / 维护性 |
| **文件** | `internal/service/tamper_app_service.go:180-237` |
| **核查结论** | 真实存在，但原影响描述偏重 |
| **状态** | 待评估后修复 |

`ListBaselines`、`DeleteBaseline`、`LoadCheckRecords`、`ListAllCheckRecords`、`GetCheckStats`、`DeleteCheckRecords` 均调用 `tamper.NewDetector(...)`。

核查发现 `NewDetector` 主要构造 `HashStorage` 和配置对象，没有明显全量扫描文件系统的逻辑，因此“每次都重新扫描文件系统”的说法不准确。

**修复建议:** 可以缓存 `Detector` 或抽出 `HashStorage` 复用，但实施前需确认 `Detector`/`HashStorage` 的并发安全和生命周期需求。

---

### EDGE-003: config.Clone() 手写深拷贝仍需完整性测试

| 属性 | 内容 |
|------|------|
| **严重级别** | P2 |
| **类别** | 维护性 |
| **文件** | `internal/config/config.go:260`, `internal/config/config_test.go:261` |
| **核查结论** | 部分成立 |
| **状态** | 可增强 |

`Config.Clone()` 是手写深拷贝。当前已有 slice、map、pointer 相关测试，但测试不是完整结构反射覆盖，新增配置字段时仍可能遗漏。

**修复建议:** 补充反射型测试，确保 clone 后所有可变引用字段不共享，并覆盖新增字段。

---

### EDGE-004: 登录限流硬编码且仅内存态

| 属性 | 内容 |
|------|------|
| **严重级别** | P2 |
| **类别** | 安全加固 |
| **文件** | `web/login_handlers.go:14` |
| **核查结论** | 真实存在 |
| **状态** | 待修复 |

```go
var loginRateLimiter = NewRateLimiter(5, 15*time.Minute)
```

当前阈值不可配置，且服务重启后限流状态清零。

**修复建议:** 第一阶段先配置化并保留默认值 5 次/15 分钟；持久化可作为后续增强。

---

### EDGE-005: 本地配置文件含敏感值，但未被 Git 跟踪

| 属性 | 内容 |
|------|------|
| **严重级别** | P2 |
| **类别** | 本地安全风险 |
| **文件** | `configs/config.yaml`, `.gitignore`, `configs/config.yaml.example` |
| **核查结论** | 部分成立 |
| **状态** | 待本地清理 |

本地 `configs/config.yaml` 中存在非占位 API Key/token/password 等敏感值。但核查结果显示：

- `.gitignore` 已包含 `configs/config.yaml`
- `git ls-files` 未跟踪 `configs/config.yaml`
- `configs/config.yaml.example` 已存在，并使用环境变量占位符

因此原文“将 `configs/config.yaml` 加入 `.gitignore`”和“提供 example 模板”两项已完成。

**修复建议:** 清理本地 `configs/config.yaml` 中的真实值，改为环境变量占位符或仅在本机安全存储；同时检查历史提交中是否曾泄露。

---

### UX-ICP-001: 查询页 ICP 类型选择重复且语义冲突

| 属性 | 内容 |
|------|------|
| **严重级别** | P2 |
| **类别** | 前端交互 / 查询语义 |
| **文件** | `web/templates/index.html`, `web/static/js/main.js`, `web/query_handlers.go`, `internal/adapter/icp.go` |
| **核查结论** | 真实存在 |
| **状态** | 待修复 |

查询页当前有两套 ICP 选择入口：

1. 通用引擎区额外硬编码 ICP checkbox：
   - `icp-web`
   - `icp-app`
   - `icp-mapp`
   - `icp-kapp`

2. 独立 ICP 查询面板里还有 `icp_type` 下拉：
   - `web/app/mapp/kapp/bweb/bapp/bmapp/bkapp`

同时页面文案写着“独立ICP查询，不与UQL引擎混合”，但 ICP 面板的显隐由 `#icp-engine-selector input[type="checkbox"]` 控制，而这些 checkbox 又使用 `name="engines"`，会参与普通 `/query` 表单提交。

这会造成几个问题：

- 用户会同时在“引擎选择”和“ICP 查询参数”里选择 ICP 类型，不清楚哪个生效。
- `icp-web` 等 checkbox 既像 UQL 引擎，又像 ICP 面板开关，职责混杂。
- 独立 ICP 查询使用 `/api/icp/query?type=...`，与 `/query` 的 `engines=icp-*` 是两条不同执行路径。
- 当前通用引擎列表本身可能已包含 `icp-*` adapter，模板又硬编码一组 ICP checkbox，存在重复展示风险。
- `internal/adapter/icp.go` 已支持通过 UQL 的 `icp.type` 限制类型，和 UI 的双重选择也可能产生不一致。

**建议产品决策:** 将 ICP 拆成两个清晰模式，避免重复选择。

- 普通资产搜索区只展示 FOFA、Hunter、ZoomEye、Quake、Shodan 等资产搜索引擎，不展示 `icp-*`。
- ICP 查询作为独立面板存在，使用一个 `type` 下拉选择备案类型，不再用 `name="engines"` checkbox 控制。
- 如果保留 UQL ICP 查询能力，则提供“高级模式”入口：用户显式选择 `icp-*` engine，并用 UQL 字段 `icp.domain`、`icp.company`、`icp.licence`、`icp.type` 查询；不要和独立 ICP 面板共用控件。

**修复建议:**

- 后端 `handleIndex` 将 `s.orchestrator.ListAdapters()` 拆分为 `searchEngines` 与 `icpEngines`，模板普通引擎区只渲染 `searchEngines`。
- 删除 `#icp-engine-selector` 下 `name="engines"` 的 ICP checkbox，改为独立 ICP 面板的开关按钮或始终显示折叠面板。
- `main.js` 的 `initICPQuery()` 不再监听 ICP checkbox，而是监听独立面板按钮/折叠状态。
- 普通 `/query` 表单提交时过滤掉独立 ICP 控件，避免 `engines=icp-*` 被意外提交。
- 增加 UI 测试或 handler 测试，确认普通查询不会混入独立 ICP 类型。

---

## 配置运行时问题

### ARC-001: HotUpdateManager 替换配置指针不会同步 Server 缓存指针

| 属性 | 内容 |
|------|------|
| **严重级别** | P1 |
| **类别** | 配置热更新设计缺陷 |
| **文件** | `internal/config/hot_update.go:178`, `web/server.go:303` |
| **核查结论** | 部分成立 |
| **状态** | 待设计后修复 |

`HotUpdateManager.checkConfigChanges` 直接替换 `configManager.config`：

```go
h.configManager.config = newConfig
```

`Server` 构造时保存了旧配置指针：

```go
config: cfg,
configManager: cfgManager,
```

如果文件热更新管理器被启动，`s.config` 确实会继续指向旧对象。

但当前主程序 `cmd/unimap-web/main.go` 未看到 `NewHotUpdateManager(...).Start(...)` 的接入；设置页保存则是直接修改 `configManager.GetConfig()` 返回的同一个对象。因此原文“设置页面保存后所有运行时组件继续使用旧配置”不准确。

真实问题是：

- 文件热更新路径与 `Server.s.config` 缓存指针不一致。
- 设置页保存是原地写配置，缺少并发保护。
- 已创建的 adapter、`ScreenshotRouter`、scheduler、alerting、rate limit 中间件等启动期组件不会因为配置值变化自动重建或重新应用。

**修复建议:** 不要只把 `s.config` 替换成 `s.configManager.GetConfig()`。需要先建立线程安全配置发布机制，再为需要动态生效的组件定义 `OnConfigChanged` 或显式重建策略。

---

### ARCH-004: 配置变更缺少运行时组件通知

| 属性 | 内容 |
|------|------|
| **严重级别** | 架构债务 |
| **类别** | 配置系统 |
| **核查结论** | 真实存在 |
| **状态** | 待设计后修复 |

组件在启动时读取配置并构造内部对象。例如：

- `cmd/unimap-web/main.go` 注册引擎 adapter 时固化 API Key、BaseURL、QPS、timeout。
- `web/server.go` 创建 `ScreenshotRouter` 时固化 mode、priority、fallback 等。
- `Start()` 中读取 rate limit、CORS、request limit、auth 开关等并构造中间件链。

这些值不是全部都能通过 `GetConfig()` 动态读取解决。

**修复建议:** 为配置项分级：

- 可立即动态读取：CORS 来源、截图目录、登录用户名等。
- 可热应用：截图模式、限流参数、部分 bridge 参数。
- 需要重建组件：引擎 adapter、scheduler、alerting、HTTP 监听端口、中间件链。
- 需要重启：监听端口、绑定地址等。

---

## 长期架构债务

### ARCH-001: web.Server 责任过重

| 属性 | 内容 |
|------|------|
| **严重级别** | 架构债务 |
| **文件** | `web/server.go` |
| **核查结论** | 真实存在 |
| **状态** | 长期重构 |

`Server` 同时管理查询、WebSocket、截图、认证、配置、分布式节点、scheduler、proxy pool、bridge、Chrome 生命周期等职责。该问题真实存在，但不建议与 P1 修复混在同一批提交中处理。

**建议:** 后续拆为 `QueryServer`、`ScreenshotServer`、`AdminServer`、`NodeServer` 等按功能域组合。

---

### ARCH-002: 部分依赖缺少接口抽象

| 属性 | 内容 |
|------|------|
| **严重级别** | 架构债务 |
| **核查结论** | 基本成立 |
| **状态** | 长期处理 |

`DistributedState`、`scheduler.Scheduler`、`BridgeState` 等以具体类型组合进 `Server`，单元测试替换成本较高。

**建议:** 等拆分 `Server` 后，再为明确需要 mock 的边界补接口。不要为了接口而接口化。

---

### ARCH-003: 路由注册集中

| 属性 | 内容 |
|------|------|
| **严重级别** | 架构债务 |
| **文件** | `web/router.go` |
| **核查结论** | 真实存在 |
| **状态** | 可独立重构 |

`web/router.go` 集中注册大量路由。当前仍可维护，但按功能域拆分会提升可读性。

**建议:** 拆分为 `registerPageRoutes`、`registerAuthRoutes`、`registerQueryRoutes`、`registerScreenshotRoutes`、`registerNodeRoutes`、`registerSchedulerRoutes`、`registerTamperRoutes`、`registerBackupRoutes` 等。

---

## 附录：修订后问题索引

| 编号 | 文件路径 | 严重级 | 核查结论 |
|------|----------|--------|----------|
| SEC-001 | `web/login_handlers.go:27` | P1 | 真实存在 |
| FUNC-001 | `web/config_handlers.go:354-356` | P1 | 真实存在 |
| FUNC-002 | `web/config_handlers.go:358-360` | P1 | 真实存在 |
| FUNC-003 | `internal/service/query_app_service.go:118` | P1 | 真实存在 |
| FUNC-004 | `web/server.go:780-803` | P2 | 真实存在，低概率 |
| FUNC-005 | `web/config_handlers.go`, `internal/config/config.go` | P1 | 真实存在 |
| EDGE-001 | `web/screenshot_handlers.go:126-206` | P2 | 真实存在 |
| EDGE-002 | `internal/service/tamper_app_service.go:180-237` | P2 | 真实存在，影响偏轻 |
| EDGE-003 | `internal/config/config.go:260` | P2 | 部分成立，已有基础测试 |
| EDGE-004 | `web/login_handlers.go:14` | P2 | 真实存在 |
| EDGE-005 | `configs/config.yaml` | P2 | 部分成立，未被 Git 跟踪 |
| UX-ICP-001 | `web/templates/index.html`, `web/static/js/main.js` | P2 | 真实存在 |
| ARC-001 | `internal/config/hot_update.go`, `web/server.go` | P1 | 部分成立 |
| ARCH-001 | `web/server.go` | 架构 | 真实存在 |
| ARCH-002 | 多处 | 架构 | 基本成立 |
| ARCH-003 | `web/router.go` | 架构 | 真实存在 |
| ARCH-004 | 多处 | 架构 | 真实存在 |
