# UniMap 项目优化实施计划（核查修订版 · 二次更新）

> 原制定日期: 2026-05-19
> 修订日期: 2026-05-19
> 二次更新日期: 2026-05-19
> 基于文档: [REVIEW_ISSUES_2026-05-19.md](./REVIEW_ISSUES_2026-05-19.md)
> 二次更新来源: [refactor-plan-v1.html](./refactor-plan-v1.html) 切片执行审计
> 修订原则: 先处理真实且低风险的问题，再处理配置系统的并发与运行时生效模型，长期架构重构单独规划。

---

## 总览

| 阶段 | 范围 | 预计工期 | 任务数 | 目标 |
|------|------|----------|--------|------|
| 阶段一 | 小范围安全与稳定性修复 | 0.5-1 天 | 3 | 修复确定性缺陷，不改变核心架构 |
| 阶段二 | 配置系统基础设施 | 2-4 天 | 4 | 建立线程安全配置读写与变更应用边界 |
| 阶段三 | 设置页验证、截图路径与 ICP 体验 | 4-5 天 | 5 | 让 UI 验证真实可用，截图路径一致，补齐 ICP 查询与定时任务体验 |
| 阶段四 | 低风险维护性优化 | 1-2 天 | 3 | 限流配置化、Clone 测试增强、Tamper 复用评估 |
| 阶段五 | 长期架构重构 | 下一迭代 | 3 | Server/Router/接口边界重构 |
| 阶段六 | 重构计划剩余项补齐 | 1-2 天 | 4 | CLI 截图退场、Monitor 美化、构建发布、配置示例完善 |

---

## 阶段一：小范围安全与稳定性修复

**目标:** 先修复明确、低耦合、容易验证的问题。

### T1: 登录 Token 比较改为 secureCompare

| 属性 | 内容 |
|------|------|
| **问题编号** | SEC-001 |
| **文件** | `web/login_handlers.go` |
| **改动量** | 1 行 |
| **风险** | 极低 |

**方案:**

```go
if secureCompare(token, s.adminToken()) {
    http.Redirect(w, r, "/", http.StatusFound)
    return
}
```

**验证:**

- `go test ./web`
- 登录页已认证 token 跳转仍正常
- 无需新增复杂测试，可补一个 handler 单测覆盖已认证跳转

---

### T2: RunBrowserQueryAsync 添加 panic recover

| 属性 | 内容 |
|------|------|
| **问题编号** | FUNC-003 |
| **文件** | `internal/service/query_app_service.go` |
| **改动量** | 约 10 行 |
| **风险** | 低 |

**关键修正:** 原计划示例中 `recover` 闭包引用 `outcome`，但 `outcome` 在闭包后才声明。实现时必须先声明 `outcome`。

**方案:**

```go
go func() {
    outcome := BrowserQueryOutcome{Enabled: true}
    defer close(resultCh)
    defer func() {
        if r := recover(); r != nil {
            outcome.Errors = append(outcome.Errors, fmt.Sprintf("browser query panic: %v", r))
            select {
            case resultCh <- outcome:
            default:
            }
        }
    }()

    // existing logic...
}()
```

**验证:**

- `go test ./internal/service`
- 新增 panic 注入测试：mock `BrowserRouter` panic，确认 channel 返回错误且进程不崩溃

---

### T3: cleanupStaleQueries 删除前二次校验

| 属性 | 内容 |
|------|------|
| **问题编号** | FUNC-004 |
| **文件** | `web/server.go` |
| **改动量** | 约 5 行 |
| **风险** | 低 |

**方案:**

```go
if len(staleIDs) > 0 {
    now := time.Now()
    s.queryMutex.Lock()
    for _, id := range staleIDs {
        if st := s.queryStatus[id]; st != nil && now.Sub(st.StartTime) > maxAge {
            delete(s.queryStatus, id)
        }
    }
    s.queryMutex.Unlock()
}
```

**验证:**

- `go test ./web`
- 单测覆盖：收集 staleID 后模拟同 ID 新状态，不应被删除

---

## 阶段二：配置系统基础设施

**目标:** 先建立正确的配置读写模型，再谈热更新和运行时生效。不要只把 `s.config` 替换成 `s.configManager.GetConfig()`。

### T4: 为 config.Manager 增加线程安全读写 API

| 属性 | 内容 |
|------|------|
| **问题编号** | FUNC-005, ARC-001 |
| **文件** | `internal/config/config.go`, `internal/config/hot_update.go` |
| **风险** | 中 |

**方案:**

- 在 `Manager` 中增加 `sync.RWMutex` 或改用 `atomic.Value` 存储不可变 `*Config`。
- `GetConfig()` 返回 clone，或明确返回只读快照。
- 新增 `Update(fn func(*Config) error) error`：内部 clone 当前配置，执行修改，校验，通过后一次性发布。
- `Save()` 必须在同一读写协议下读取配置快照。
- `HotUpdateManager` 不再直接写 `h.configManager.config = newConfig`，改用 `SetConfig` 或 `Update`。

**验收标准:**

- `go test -race ./internal/config`
- `hot_update.go` 不再直接访问 `Manager.config` 字段
- 并发读写配置不会触发 race

---

### T5: 重构设置页配置更新为 clone-validate-publish

| 属性 | 内容 |
|------|------|
| **问题编号** | FUNC-005 |
| **文件** | `web/config_handlers.go` |
| **风险** | 中 |

**方案:**

- `handleUpdateConfig` 不再直接修改 `cfg := s.configManager.GetConfig()` 返回对象。
- 改为调用 `s.configManager.Update(func(cfg *config.Config) error { ... })`。
- `applyEngineConfig`、`applyWebConfig` 等函数只修改传入副本。
- 更新失败时旧配置保持不变。

**注意:** `*cfg = *cloned` 不是并发安全方案，必须依赖 T4 的统一发布机制。

**验证:**

- `go test -race ./web ./internal/config`
- 设置页保存成功后文件内容更新
- 非法值校验失败时配置不被部分写入

---

### T6: 配置变更分级与运行时应用策略

| 属性 | 内容 |
|------|------|
| **问题编号** | ARC-001, ARCH-004 |
| **文件** | `web/server.go`, `cmd/unimap-web/main.go`, `internal/config` |
| **风险** | 中到高 |

**方案:**

为配置项建立运行时生效策略：

| 配置类型 | 策略 |
|----------|------|
| 纯读取项，如截图目录、部分认证字段 | 请求时读取配置快照 |
| 可热应用项，如截图 mode、限流参数 | 注册 `OnConfigChanged` 回调并更新组件状态 |
| 需要重建项，如引擎 adapter、scheduler、alerting | 在配置变化后显式重建或提示需重启 |
| 监听端口、绑定地址、中间件链结构 | 标记为需重启 |

**最小可交付:**

- 设置页返回每项配置的 `applied` 或 `restart_required` 状态。
- `ScreenshotRouter.SetMode` 用于应用截图 mode 变化。
- 引擎 API Key/BaseURL 变化暂时标记为 `restart_required`，除非同步实现 adapter 重建。

**验证:**

- 修改截图模式后 router 状态变化
- 修改端口后提示需重启，而不是假装立即生效
- 修改引擎配置后行为符合 UI 提示

---

### T7: 清理 Server.s.config 使用边界

| 属性 | 内容 |
|------|------|
| **问题编号** | ARC-001, ARCH-004 |
| **文件** | `web/*.go` |
| **风险** | 中 |

**方案:**

- 对适合动态读取的路径，统一通过 `s.currentConfig()` 获取配置快照。
- 对需要原地更新历史状态的代码，如自动生成 admin token、保存 cookie，需要走 `configManager.Update`。
- 不要机械替换所有 `s.config`，因为测试和部分生命周期代码仍可能依赖启动时配置。

**验证:**

- `go test ./web`
- `go test -race ./web`
- 重点覆盖认证、cookie 保存、CDP 配置、截图目录、分布式 token 读取

---

## 阶段三：设置页验证、截图路径与 ICP 体验

### T8: 实现 API Key 验证

| 属性 | 内容 |
|------|------|
| **问题编号** | FUNC-001 |
| **文件** | `web/config_handlers.go` 或新增 `web/engine_validation.go` |
| **风险** | 中 |

**设计修正:**

当前函数 `validateEngineAPIKey(engine, apiKey string)` 信息不足。应改为：

```go
func (s *Server) validateEngineAPIKey(ctx context.Context, engine, apiKey string) (bool, string)
```

或：

```go
func validateEngineAPIKey(ctx context.Context, cfg *config.Config, engine, apiKey string) (bool, string)
```

**实现要求:**

- FOFA 需要同时读取 email。
- BaseURL 使用当前配置，不写死。
- timeout 使用当前配置或 10s 默认值。
- 外部 API 端点必须按对应平台当前官方文档或现有适配器约定确认。
- 不把网络不可达误报为 key 格式错误。

**验证:**

- 无 key 返回失败
- 未知引擎返回失败
- mock HTTP server 覆盖 200、401/403、5xx、timeout
- 不依赖真实外部网络的单元测试

---

### T9: 实现引擎连通性测试

| 属性 | 内容 |
|------|------|
| **问题编号** | FUNC-002 |
| **文件** | `web/config_handlers.go` 或新增 `web/engine_validation.go` |
| **风险** | 中 |

**设计修正:**

当前函数 `testEngineHealth(engine string)` 信息不足。应改为 `Server` 方法，并读取当前配置快照。

**实现要求:**

- 测试 BaseURL 可达性和认证可用性。
- 与 T8 共享 HTTP client、timeout 和错误分类逻辑。
- 对 Web-only 模式给出明确消息，例如 `engine configured for web-only mode; API health check skipped`。

**验证:**

- mock HTTP server 覆盖成功、认证失败、超时、DNS/连接失败
- 设置页按钮不再永远返回成功

---

### T10: `/api/screenshot` 接入 ScreenshotRouter

| 属性 | 内容 |
|------|------|
| **问题编号** | EDGE-001 |
| **文件** | `web/screenshot_handlers.go` |
| **风险** | 中 |

**方案:**

- 保留现有 URL 校验和 SSRF 防护。
- 优先调用 `s.screenshotRouter.CaptureTargetWebsite`。
- 为保持兼容，若旧接口需要返回 PNG bytes，则捕获后读取截图文件并 `http.ServeFile` 或写 bytes。
- 如果 router 不可用，再 fallback 到 `s.screenshotApp`；不要直接创建新的 chromedp allocator，除非作为最后兜底并有明确日志。

**验证:**

- `go test ./web ./internal/screenshot`
- `/api/screenshot` 返回图片格式不破坏前端
- router mode 为 extension/cdp/auto 时路径一致

---

### T11: 完善 ICP 定时任务能力

| 属性 | 内容 |
|------|------|
| **问题编号** | NEW-SCHED-ICP-001 |
| **文件** | `internal/scheduler/scheduler.go`, `internal/scheduler/icp_batch_runner.go`, `web/server.go`, `web/templates/scheduler.html` |
| **风险** | 中 |

**当前核查结论:**

当前系统不是完全没有 ICP 定时任务。代码中已存在：

- `TaskICPBatch = "icp_batch"`，并在 `AllTaskTypes()` 中返回。
- `TaskTypeLabel(TaskICPBatch)` 显示为 `ICP 批量备案查询`。
- `internal/scheduler/icp_batch_runner.go` 已实现 `ICPBatchRunner`。
- `web/server.go` 会在 `cfg.ICP.Enabled && cfg.ICP.BaseURL != ""` 时注册 `NewICPBatchRunner(cfg.ICP.BaseURL, cfg.ICP.APIKey)`。

但从业务可用性看仍不完整：

- Runner 只有在 ICP 配置启用时才注册；未启用时页面仍可能展示任务类型，但创建或执行会失败。
- 调度页只有通用 payload JSON 文本框，没有 ICP 专用表单、示例和字段校验。
- `ICPBatchRunner` 直接调用 `adapter.ICPSearch`，没有复用已注册的 `icp-*` adapter/orchestrator，也不会自动感知运行时配置变化。
- 执行结果只返回总数摘要，失败域名被吞掉为 nil，缺少失败数量、失败原因和可追踪输出文件。
- 缺少针对 `ICPBatchRunner` 的单元测试和 Web 创建/执行路径测试。

**方案:**

1. 明确支持的 payload schema：

```json
{
  "type": "web",
  "domains": ["example.com", "example.org"],
  "page": 1,
  "page_size": 20,
  "output": {
    "format": "json",
    "path": "./data/icp_exports"
  },
  "continue_on_error": true
}
```

2. 调度页增加 ICP 专用表单或任务模板：

- 任务类型选择 `icp_batch` 时显示 `type/domains/page_size/continue_on_error/output` 字段。
- 支持文本域按行输入域名，提交前转换为 payload。
- 提供示例 payload，避免用户手写 JSON。

3. 改进 Runner：

- 校验 `type` 必须是 `adapter.AllICPQueryTypes()` 中的值。
- 校验 domains 非空、去重、trim、限制批量大小。
- 统计 success/failed/total_results，并返回包含失败原因的摘要。
- 可选输出详细结果到 `./data/icp_exports/*.json`，history 中记录文件路径。
- 支持 context cancellation 和 timeout。

4. 注册策略：

- 如果 ICP 未启用，不应在 UI 中误导用户“可用”；可在任务类型旁标记未配置，或创建时返回清晰错误。
- 若阶段二配置热更新完成，ICP runner 应从配置快照读取 BaseURL/APIKey，或在配置变化后重建 handler。

**验证:**

- `go test ./internal/scheduler ./web`
- 新增 `ICPBatchRunner` 单测：缺 domains、非法 type、字符串/数组 domains、部分失败、context cancel。
- Web 层测试：创建 `icp_batch` 任务、立即执行、history 记录包含正确 task_type 和执行结果。
- 手动验证：调度页选择 ICP 批量备案查询，按 cron 执行并产生历史记录。

---

## 阶段三（续）：T12 已完成

### ~~T12: 梳理查询页 ICP 入口，避免重复选择~~ ✅

| 属性 | 内容 |
|------|------|
| **问题编号** | UX-ICP-001 |
| **状态** | **已完成** (2026-05-19) |
| **文件** | `web/templates/index.html`, `web/static/js/main.js` |

**已完成内容:**

- 删除 `#icp-engine-selector` 下 4 个 ICP checkbox（`icp-web`/`icp-app`/`icp-mapp`/`icp-kapp`）。
- 改为单一开关：`启用 ICP 备案查询` 复选框。
- `initICPQuery()` 不再监听旧 checkbox，改为监听 `#icp-toggle`。
- ICP 面板 `icp-type` 下拉保留全部 8 种类型（备案 + 黑名单）。
- `go test -race ./web/...` 通过。

---

## 阶段四：低风险维护性优化

### T13: 登录限流配置化

| 属性 | 内容 |
|------|------|
| **问题编号** | EDGE-004 |
| **文件** | `internal/config/config.go`, `web/login_handlers.go` |
| **风险** | 低 |

**方案:**

- 在 `web.auth` 下新增 `login_rate_limit` 配置：

```yaml
login_rate_limit:
  max_attempts: 5
  window_seconds: 900
```

- 默认行为保持 5 次/15 分钟。
- 第一阶段只做配置化，不做持久化。

**验证:**

- 默认值兼容
- 配置修改后重启生效
- 限流触发返回 429

---

### T14: config.Clone() 增强完整性测试

| 属性 | 内容 |
|------|------|
| **问题编号** | EDGE-003 |
| **文件** | `internal/config/config_test.go` |
| **风险** | 低 |

**方案:**

- 保留已有 slice/map/pointer 测试。
- 新增反射测试，遍历 `Config` 中 slice、map、pointer 字段，确认 clone 后不共享可变引用。
- 新增字段时该测试应能暴露遗漏。

**验证:**

- `go test ./internal/config`

---

### T15: TamperAppService Detector 复用评估与小改

| 属性 | 内容 |
|------|------|
| **问题编号** | EDGE-002 |
| **文件** | `internal/service/tamper_app_service.go` |
| **风险** | 低到中 |

**方案:**

- 先确认 `tamper.Detector` 和 `HashStorage` 是否并发安全。
- 若安全，在 `TamperAppService` 中缓存 detector。
- 若不安全，只抽取 `newDetector()` helper，减少重复代码，不缓存共享实例。

**验证:**

- `go test -race ./internal/service ./internal/tamper`
- 篡改检测、基线列表、历史记录功能正常

---

## 阶段六：重构计划剩余项补齐

> 来源: [refactor-plan-v1.html](./refactor-plan-v1.html) 切片审计，Slice 9/11/13 未完成项 + config.yaml.example 缺失字段。

### T19: CLI 截图子命令运行时 deprecation 警告

| 属性 | 内容 |
|------|------|
| **来源** | refactor-plan-v1 Slice 9 |
| **文件** | `cmd/unimap-cli/main.go` |
| **风险** | 低 |

**当前状态:** 帮助文本已标注 `(deprecated, use Web UI)`，但运行时执行时无任何警告输出。

**方案:**

- 在 `screenshot-batch` 子命令入口输出 deprecation 警告（stderr）：
  ```
  ⚠ WARNING: screenshot-batch is deprecated and will be removed in a future version.
  Use the Web UI batch-screenshot page instead.
  ```
- 保留功能，不阻断现有脚本。
- 同步在 API 响应中增加 `Deprecation` header（可选）。

**验证:**

- 执行 `go run ./cmd/unimap-cli screenshot-batch` 时 stderr 出现警告
- 功能仍正常工作
- `go test ./cmd/unimap-cli/...`

---

### T20: Monitor 页面按钮语义化

| 属性 | 内容 |
|------|------|
| **来源** | refactor-plan-v1 Slice 11 |
| **文件** | `web/templates/monitor.html` |
| **风险** | 低 |

**当前状态:** monitor 页面所有 7 个操作按钮统一使用 `btn-outline-primary`，缺少语义区分。CSS 中已有 `btn-outline-danger`、`btn-outline-warning`、`btn-success` 等可用。

**方案:**

| 操作 | 当前类 | 建议类 |
|------|--------|--------|
| 执行篡改检测 | `btn-outline-primary` | `btn-primary` |
| 执行截图 | `btn-outline-primary` | `btn-outline-primary` |
| 导出结果 | `btn-outline-primary` | `btn-outline-primary` |
| 清除结果 | `btn-outline-primary` | `btn-outline-danger` |
| 加载示例 | `btn-outline-primary` | `btn-outline-secondary` |
| 导入配置 | `btn-outline-primary` | `btn-outline-warning` |
| 其他 | `btn-outline-primary` | 保持 |

- 同时检查 tab 类名一致性：scheduler 用 `.tab-btn`，monitor 用 `.tab`，统一为 `.tab-btn`。

**验证:**

- 页面按钮视觉层次清晰
- 功能不受影响

---

### T21: 构建与发布完善

| 属性 | 内容 |
|------|------|
| **来源** | refactor-plan-v1 Slice 13 |
| **文件** | `build.sh`, `.github/workflows/ci.yml` |
| **风险** | 低 |

**当前状态:**

- `build.sh` 仅支持交互模式（菜单选择 1-4）
- CI workflow 缺少 `windows-latest` 构建矩阵
- 无 `.goreleaser.yml` 自动发布

**方案:**

1. `build.sh` 增加非交互模式：
   ```bash
   # 非交互：./build.sh all (构建全部) 或 ./build.sh web-cli (仅 Web+CLI)
   if [ "$1" = "all" ] || [ "$1" = "web-cli" ]; then
       # 直接执行，不显示菜单
   fi
   ```

2. CI 增加 Windows 矩阵：
   ```yaml
   matrix:
     os: [ubuntu-latest, macos-latest, windows-latest]
   ```

3. 构建产物确保包含 `web/templates`、`web/static`、`configs/config.yaml.example`。

4. `.goreleaser.yml` 可后续迭代再引入，本阶段先做前两项。

**验证:**

- `./build.sh web-cli` 非交互执行
- CI 在三个平台均通过
- dist 产物包含模板和静态资源

---

### T22: config.yaml.example 补充缺失字段

| 属性 | 内容 |
|------|------|
| **来源** | refactor-plan-v1 数据与配置演进 |
| **文件** | `configs/config.yaml.example` |
| **风险** | 低 |

**当前缺失:**

| 字段 | Go struct 位置 | 说明 |
|------|---------------|------|
| `auth.facade_enabled` | `config.go:141` | AuthFacade 开关 |
| `auth.password_policy` | `password_service.go` | 密码策略（最小长度、数字要求等） |
| `auth.password_changed_at` | `config.go:145` | 密码修改时间戳 |
| `cli.api_base` | `api_subcommands.go:46` | CLI API 基地址 |
| `cli.plugin_mode` | `api_subcommands.go:48` | CLI 插件模式 (off/auto/required) |
| `bridge.pairing_required` | 部分存在 `screenshot.extension.pairing_required` | Bridge 配对要求 |

**方案:**

- 在 `config.yaml.example` 中新增 `auth` 和 `cli` 配置段，填入上述字段及注释。
- 所有新增字段必须有默认值，不影响现有行为。

**验证:**

- `cp configs/config.yaml.example configs/config.yaml` 后 `go run ./cmd/unimap-web/` 正常启动
- CLI `go run ./cmd/unimap-cli query` 可使用默认配置

---

## 阶段五：长期架构重构

### T16: Server 按功能域拆分

| 属性 | 内容 |
|------|------|
| **问题编号** | ARCH-001 |
| **风险** | 高 |

**建议拆分:**

- `QueryServer`: 查询、WebSocket、query status
- `ScreenshotServer`: 截图、bridge、router status
- `AdminServer`: 登录、配置、账号安全
- `NodeServer`: 分布式节点与任务
- `OpsServer`: monitor、backup、scheduler、tamper

该任务不应和 P1 修复混在同一提交中。

---

### T17: 路由注册按功能域拆分

| 属性 | 内容 |
|------|------|
| **问题编号** | ARCH-003 |
| **风险** | 低到中 |

**方案:**

```go
func (r *Router) RegisterRoutes() http.Handler {
    r.registerPageRoutes()
    r.registerAuthRoutes()
    r.registerConfigRoutes()
    r.registerQueryRoutes()
    r.registerScreenshotRoutes()
    r.registerNodeRoutes()
    r.registerSchedulerRoutes()
    r.registerTamperRoutes()
    r.registerBackupRoutes()
    return r.buildMux()
}
```

可独立完成，主要收益是可读性。

---

### T18: 为明确测试边界补接口

| 属性 | 内容 |
|------|------|
| **问题编号** | ARCH-002 |
| **风险** | 中 |

**方案:**

- 只为实际需要 mock 的依赖补接口。
- 优先在拆分后的子 server 边界定义接口。
- 不对所有具体类型做机械接口化。

---

## 已完成或不应重复执行的事项

- `configs/config.yaml` 已在 `.gitignore` 中。
- `configs/config.yaml.example` 已存在，并已使用环境变量占位符。
- `configs/config.yaml` 未被 Git 跟踪。仍需本地清理敏感值，并按需检查 Git 历史是否泄露。
- **T12 (UX-ICP-001)** 查询页 ICP 入口去重 — 已完成（2026-05-19）。删除 4 个 ICP checkbox，改为单一开关。

## refactor-plan-v1 切片执行状态

> 来自 [refactor-plan-v1.html](./refactor-plan-v1.html) 审计结果。

| 切片 | 状态 | 说明 |
|------|------|------|
| Slice 0 基线冻结 | ✅ 已完成 | `docs/slice0-baseline.md` |
| Slice 1 认证防腐层 | ✅ 已完成 | AuthFacade + middleware |
| Slice 2 Bridge 鉴权 | ✅ 已完成 | 独立 Bearer token + HMAC |
| Slice 3 修改密码 | ✅ 已完成 | API + bcrypt + 策略 |
| Slice 4 认证管理页 | ✅ 已完成 | account-security.html |
| Slice 5 CLI 鉴权适配 | ✅ 已完成 | token 优先级链 |
| Slice 6 CLI 查询收敛 | ✅ 已完成 | API-first + legacy 共存 |
| Slice 7 CLI 监控补齐 | ✅ 已完成 | reachability + port-scan |
| Slice 8 CLI 插件模式 | ✅ 已完成 | off/auto/required |
| Slice 9 CLI 截图退场 | ⚠️ 进行中 | 仅帮助文本标注，缺运行时警告 → **T19** |
| Slice 10 前端设计令牌 | ✅ 已完成 | 50+ CSS 变量 |
| Slice 11 定时任务/监控美化 | ⚠️ 进行中 | scheduler 完善，monitor 按钮缺语义 → **T20** |
| Slice 12 跨平台 Chrome | ✅ 已完成 | cdp_handlers.go |
| Slice 13 构建与发布 | ⚠️ 进行中 | 缺非交互模式、Windows CI → **T21** |
| config.yaml.example | ⚠️ 进行中 | 缺 auth/cli 配置段 → **T22** |

---

## 推荐执行顺序

```text
阶段一
├── T1 secureCompare
├── T2 panic recover
└── T3 stale query 二次校验

阶段二
├── T4 Manager 线程安全 API
├── T5 设置页 clone-validate-publish
├── T6 配置变更分级与运行时应用策略
└── T7 清理 Server.s.config 使用边界

阶段三
├── T8 API Key 验证
├── T9 引擎连通性测试
├── T10 /api/screenshot 接入 ScreenshotRouter
├── T11 完善 ICP 定时任务能力
└── ~~T12 梳理查询页 ICP 入口~~ ✅ 已完成

阶段四
├── T13 登录限流配置化
├── T14 Clone 完整性测试
└── T15 Tamper Detector 复用评估

阶段五
├── T16 Server 拆分
├── T17 路由拆分
└── T18 测试边界接口化

阶段六
├── T19 CLI 截图 deprecation 警告
├── T20 Monitor 按钮语义化
├── T21 构建与发布完善
└── T22 config.yaml.example 补充字段
```

---

## 通用质量门槛

每个阶段至少执行：

```bash
go test ./internal/config ./internal/service ./web
```

涉及并发或配置发布的阶段还需执行：

```bash
go test -race ./internal/config ./internal/service ./web
```

涉及全局适配器、截图、scheduler 的阶段建议再执行：

```bash
go test ./...
```

---

## 关键风险与缓解

| 风险 | 影响任务 | 缓解 |
|------|----------|------|
| 配置 Manager 改动引入大量调用点调整 | T4-T7 | 先增加兼容 API，再逐步迁移调用点 |
| 引擎 API 文档变化或平台差异 | T8-T9 | 以 mock 测试为主，真实平台验证作为手动验收 |
| `/api/screenshot` 响应格式兼容性 | T10 | 保持旧接口返回图片 bytes，不强制改 JSON |
| ICP 定时任务 UI 与现有通用 payload 冲突 | T11 | 保留 JSON 高级模式，同时增加 ICP 专用表单 |
| 查询页 ICP 普通模式与高级模式混淆 | ~~T12~~ | ~~默认拆分~~ ✅ 已完成 |
| Detector 缓存引入并发问题 | T15 | 先跑 race，必要时只抽 helper 不共享实例 |
| Server 拆分影响面大 | T16 | 下一迭代单独规划，小步提交 |
| CLI deprecation 警告影响脚本解析 | T19 | 仅写 stderr，不改变 stdout 格式 |
| Monitor 按钮类变更影响 JS 选择器 | T20 | 只改 CSS 类，不改 DOM id |
| config.yaml.example 字段遗漏导致启动失败 | T22 | 新增字段必须有默认值，启动验证 |
