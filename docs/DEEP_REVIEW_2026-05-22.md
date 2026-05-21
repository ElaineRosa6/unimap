# UniMap 项目深度审查报告

**审查日期**：2026-05-22
**审查分支**：`master`
**基线提交**：`8c66e5f` — fix: extension cookie-based login detection and CDP URL/defer bugs
**Go 版本**：1.26
**审查范围**：架构合理性 + 业务闭环 + 完成度 + 缺陷排查（P0/P1/P2）

---

## 0. 摘要

| 维度 | 结论 |
|------|------|
| **架构** | ✅ 分层清晰、无循环依赖、接口设计符合 Go 习惯；**唯一短板是 10 个超大文件** |
| **业务闭环** | ✅ 主链路全通；**1 处实际断点：ICP 查询前后端不通** |
| **完成度** | **~92%**（距 v1.0 还差 ICP 接入 + Bridge 签名 + 节点 token + 测试覆盖率） |
| **代码质量** | 1 处 P1 功能 bug（ICP 路由）、5 处 P1 强化项、8 处 P2 边缘项；新增 P0 = 0 |
| **本次会话已修复** | CDP URL scheme、defer-in-loop、`extension_paired_session_unverified` 语义丢失（已并入 `8c66e5f`） |

---

## 1. 架构合理性评估

### 1.1 总体结构

```
135 个 Go 源文件 / 78,511 LOC
- cmd/ (3 入口)        unimap-cli, unimap-gui, unimap-web
- internal/ (28 子包)  按职责分层
- web/ (59 文件)        HTTP/WS 接入层
```

依赖方向通过 `go list -deps` 验证：`web → internal`，无反向依赖、无循环依赖。✅

### 1.2 分层评估

| 层 | 评价 |
|----|------|
| 接入层（cmd / web） | ✅ 三入口收敛到 `internal/service`，符合 hexagonal |
| 应用服务层（`internal/service`） | ✅ `QueryAppService`、`ScreenshotAppService`、`TamperAppService`、`MonitorAppService` 职责清晰 |
| 领域层（`adapter`、`core/unimap`、`tamper`、`screenshot`、`scheduler`） | ✅ 边界清晰 |
| 基础设施层（`utils`、`logger`、`metrics`、`monitoring`、`requestid`、`proxypool`） | ✅ 独立可复用 |
| 接口设计 | ✅ `EngineAdapter`/`Plugin`/`Runner` 均为小接口（1–3 方法）+ 在使用方定义 |

### 1.3 可扩展性

- 新增搜索引擎：在 `internal/adapter/` 增文件 + `orchestrator.go` 注册即可
- 新增定时任务 Runner：实现 `Type()` + `Execute()` 接口即可
- 截图引擎：`ScreenshotRouter` 抽象 CDP/Extension 双通道，扩展第三种通道成本低

### 1.4 架构问题点

| 严重度 | 问题 | 证据 |
|--------|------|------|
| 🟡 中 | **10 个超大文件**，违反项目约定（≤800 行） | `cmd/unimap-gui/monitor_native.go` **2150**、`internal/tamper/detector.go` **1744**、`internal/scheduler/scheduler.go` **1246**、`internal/config/config.go` **1210**、`internal/screenshot/manager.go` **1187**、`internal/scheduler/executor.go` **997**、`web/server.go` **979**、`internal/adapter/orchestrator.go` **888**、`web/screenshot_bridge_handlers.go` **834**、`internal/service/screenshot_app_service.go` **805** |
| 🟡 中 | `web/server.go` 单体类聚合 12+ 子系统引用，违反 SRP | `Server` 含 `screenshotMgr`、`bridge`、`distributed`、`scheduler`、`configManager`、`orchestrator`、`proxyPool`、`apiAuth`、`connManager` 等 |
| 🟢 低 | 模块名 `internal/error/` 与标准库 `errors` 冲突，需 alias | — |

---

## 2. 业务逻辑闭环诊断

### 2.1 核心链路完整性

| 链路 | 状态 | 评注 |
|------|------|------|
| UQL 查询：CLI/Web → orchestrator → 6 adapters → 归并 | ✅ 通 | `core/unimap` 解析 + 并发分发 + dedup |
| 截图：CDP / Extension 双模式 + Router 自动降级 | ✅ 通 | `screenshot/router.go` + `bridge_health.go` |
| 篡改检测：5 种模式 | ✅ 通 | `tamper/detector.go` 5 模式入口齐备 |
| 20 个定时任务 Runner | ✅ 全部已实现 | `executor.go` 中 `ST-01…ST-20` 全部存在 |
| 分布式：node 注册/心跳/任务/快照/故障转移 | ✅ 通 | `distributed/` 完整 + 故障转移 e2e 已归档 |
| 告警：Webhook/Log + 去重+静默+频率 | ✅ 通 | `alerting/manager.go` |
| 数据备份 | ✅ 通 | `backup/backup.go` + handler |
| 代理池 | ✅ 通 | 轮询 + 失败冷却 |
| 登录会话：用户名密码 + AES-GCM Cookie | ✅ 通 | `web/session.go` |
| WebSocket 推送 query/progress/result | ✅ 通 | `websocket_handlers.go` |

### 2.2 前后端对齐 — **存在断点**

对比 `web/static/js/main.js` 中所有 `fetch('/api/...')` 与 `web/router.go` 的 75 条路由：

| 前端调用 | 后端 | 状态 |
|----------|------|------|
| `/api/icp/query` | **未注册** | ❌ **断点** |
| 其余 11 个 `/api/*` | 已注册 | ✅ |

ICP 查询适配器 `internal/adapter/icp.go` 已实现，但 Web 接入层完全没接；CLI `cmd/unimap-cli` 也未支持 `-e icp`。**这是真正的功能不闭环**。

### 2.3 异常流处理

| 场景 | 处理 |
|------|------|
| 网络失败（adapter 调用） | ✅ orchestrator 重试 + 熔断 + 降级 |
| 权限拒绝（Cookie 失效） | ✅ `verifyEngineSession` 区分 `login_required`/`no_results` |
| 引擎配额耗尽 | ✅ `quota.html` 显示 + 错误消息截断 |
| 数据为空 | ✅ 模板渲染兜底 |
| Bridge 失联 | ✅ `BridgeHealth` 周期探测 + Router 自动降级 |
| Chrome 进程崩溃 | ✅ `cdp_handlers.go:350` 异步监听 + `chromeCmd=nil` |
| WebSocket 中途断开 | ⚠️ 查询继续执行至完成（保留 5min），客户端重连可拉 `query_status`；功能 OK 但无主动续推 |

---

## 3. 项目完成度盘点

### 3.1 完成度估算：**约 92%**

| 维度 | 完成度 | 备注 |
|------|--------|------|
| 核心查询链路 | 95% | ICP 前后端不通 |
| 截图体系 | 98% | CDP+Extension+Router |
| 篡改检测 | 95% | 5 模式 + 数据库 + 告警全联通 |
| 定时任务 | 95% | 20 Runner + 持久化；前端缺编辑按钮 |
| 分布式 | 95% | 故障转移 e2e 全通过 |
| 告警 | 90% | Webhook + Log；缺邮件/Slack |
| 鉴权 | 85% | Admin Token + 用户密码 + Session Cookie；**Bridge 签名默认关闭 (M-05)、节点 token 未实现 (M-04)** |
| 测试覆盖 | 65% | Phase 1/2 完成，Phase 3 未到 80% |
| 文档 | 90% | CLAUDE.md 与 config 实际值不一致 |

### 3.2 必须补充的核心功能

1. **ICP 查询 Web 接入** — 前端已写但后端 0 行接入代码
2. **CLI 对 ICP 引擎的支持** — `unimap-cli -e icp,fofa` 当前无法识别 icp
3. **Bridge 通信签名校验默认开启**（M-05 仍未修复）
4. **分布式节点鉴权 token**（M-04 仍未修复，节点间互信现在零成本）

### 3.3 建议优化的体验点

1. 超大文件拆分（`monitor_native.go` 2150 行优先）
2. 测试覆盖率冲到 80%（CLI / service / tamper / web 包当前 < 60%）
3. Scheduler 前端编辑表单
4. 移除 CSP `'unsafe-eval'` 兜底（L-03）
5. CORS 字段去重（L-02）
6. 渐进强类型替换插件接口 `map[string]interface{}`（L-05）

---

## 4. 代码质量与缺陷排查（Bug Hunting）

### 🔴 P0 — 崩溃 / 核心阻断

**未发现新增 P0**。本次会话提交 `8c66e5f` 已修复 3 个潜在阻断点：

| 已修复项 | 位置 |
|----------|------|
| CDP `WithURLs` 裸域名导致 cookie 永远空 | `web/cdp_handlers.go:261` |
| `defer cancel` 在 for 循环内累积 | `web/cookie_handlers.go:633,638` |
| `extension_paired_session_unverified` 语义丢失 | `web/cookie_handlers.go` 重构 |

### 🟠 P1 — 功能异常

#### P1-1 ICP 查询路由完全缺失
- **位置**：`web/router.go` 整文件、`web/static/js/main.js:2639`
- **现象**：前端 ICP 按钮调用 `GET /api/icp/query`，server 返回 404
- **根因**：路由表 75 条无 `/api/icp/*`；adapter `internal/adapter/icp.go` 已实现但未对接
- **修复**：
  ```go
  // web/router.go
  r.addRoute("icp-query", "GET", "/api/icp/query", r.server.handleICPQuery, true)
  ```
  新增 `web/icp_handlers.go`，参数 `type`+`search` 透传给 `adapter/icp.go` 的 `Search()`。

#### P1-2 `validateWebSocketRequest` 在 `auth.Enabled=false` 时无条件放行
- **位置**：`web/websocket_handlers.go:125-127`
- **代码**：
  ```go
  adminToken := s.adminToken()
  if adminToken == "" {
      return true // auth not configured
  }
  ```
- **风险**：admin 关闭鉴权且 server 绑定 0.0.0.0 时，任何人均可建立 WS。`server.go:599-601` 仅 logger.Warn 而无强制约束。
- **修复**：当 bind 非环回 + auth 关闭时拒绝 WS：
  ```go
  if adminToken == "" {
      if s.bindAddr() != "127.0.0.1" && s.bindAddr() != "localhost" {
          logger.Warn("WS rejected: auth disabled on non-loopback bind")
          return false
      }
      return true
  }
  ```

#### P1-3 Quota 收集协程的 `break` 不跳出 for-loop
- **位置**：`web/query_handlers.go:307-313`
- **代码**：
  ```go
  for i := 0; i < len(engines); i++ {
      var res result
      select {
      case res = <-ch:
      case <-ctx.Done():
          break              // 只跳出 select
      }
      results[res.engine] = res  // ctx done 时 res.engine=""，污染 map
  }
  ```
- **影响**：超时后仍跑完循环；最终输出由外层兜底正确，但浪费 CPU 且产生空 key
- **修复**：
  ```go
  outer:
  for i := 0; i < len(engines); i++ {
      select {
      case res := <-ch:
          results[res.engine] = res
      case <-ctx.Done():
          break outer
      }
  }
  ```

#### P1-4 `verifyEngineSession` 未做 `s.bridge` nil 检查
- **位置**：`web/cookie_handlers.go:279`
- **代码**：`if s.bridge.Service == nil {` — `s.bridge` 自身为 nil 时直接 panic
- **现状**：生产构造器总是初始化 bridge（`server.go:301`），但单元测试和未来重构有踩坑风险
- **修复**：与 `detectLoginViaExtension` 保持一致：
  ```go
  if s.bridge == nil || s.bridge.Service == nil {
      return false, "", "extension_not_paired", fmt.Errorf("bridge_unavailable")
  }
  ```

#### P1-5 告警 goroutine 在持有 RLock 下生成
- **位置**：`internal/alerting/manager.go:105-120`
- **代码**：`defer m.mutex.RUnlock()` 在整个 `SendAlert` 期间持锁
- **影响**：`channel.Send` 在 goroutine 内运行（已不占主锁）；但 channels 迭代仍持锁，极大 channels 列表时锁持有时间过长。**严格不算 bug，属次优**。
- **修复**：拷贝 channels 到本地切片提前 RUnlock：
  ```go
  m.mutex.RLock()
  chans := append([]AlertChannel(nil), m.channels...)
  m.mutex.RUnlock()
  for _, ch := range chans { ... }
  ```

#### P1-6 `s.chromeCmd = cmd.Process` 在 goroutine 监听者已启动之后赋值
- **位置**：`web/cdp_handlers.go:350-361`
- **现状**：父函数持有 `chromeCmdMu.Lock()`（`startCDPChrome:282-283`），赋值与 goroutine 的读取都在锁内 → 实际无 race
- **风险**：未来移除父函数的锁即出问题
- **修复**：把赋值移到 `go func()` 之前 + 加注释保护：
  ```go
  s.chromeCmd = cmd.Process  // ✅ caller holds chromeCmdMu
  go func() { ... }()
  ```

### 🟡 P2 — 边缘情况 / 体验差

#### P2-1 WebSocket 查询 goroutine 修改闭包变量 `ctx`
- **位置**：`web/websocket_handlers.go:225-228`
- **代码**：
  ```go
  go func() {
      if ctx == nil {
          ctx = context.Background()  // 修改外部参数
      }
      ...
  }()
  ```
- **修复**：用局部变量：
  ```go
  go func() {
      effectiveCtx := ctx
      if effectiveCtx == nil { effectiveCtx = context.Background() }
      ...
  }()
  ```

#### P2-2 `plugin/manager.go` 健康监控 goroutine 未被 WaitGroup 跟踪
- **位置**：`internal/plugin/manager.go:180-198`
- **现状**：`StartHealthMonitor` 直接 `go func()`，`Shutdown()` 无对应等待
- **修复**：与 alerting 一致加 `wg.Add/Done` + `wg.Wait` 在 Shutdown

#### P2-3 配置文档 vs 实现不一致
- **位置**：`CLAUDE.md` 第 88 行 vs `configs/config.yaml.example:122-123` vs `internal/config/config.go:714,720,724`
- **现象**：CLAUDE.md 写 `web.auth.enabled` 默认 `false`，但 example 和代码默认都是 `true`
- **修复**：更新 CLAUDE.md

#### P2-4 路由数量与 CLAUDE.md 不同步
- **位置**：`CLAUDE.md` 第 38 行描述 69 路由
- **实际**：`grep -c "r.addRoute" web/router.go` → **75**

#### P2-5 编辑器 CRLF/LF 不一致
- **位置**：`tools/extension-screenshot/*.json/*.js` 每次 commit 产生 `LF will be replaced by CRLF`
- **修复**：根目录添加 `.gitattributes`：
  ```
  *.json text eol=lf
  *.js   text eol=lf
  *.go   text eol=lf
  ```

#### P2-6 ICP 适配器死代码风险
- **位置**：`internal/adapter/icp.go`
- **现状**：已实现但无 Web/CLI 调用点（见 P1-1）
- **修复**：补全调用链（首选），或挂 `//go:build experimental` 标签

#### P2-7 `internal/plugin/manager.go:194-196` 缩进不一致
- **修复**：运行 `gofmt -w internal/plugin/manager.go`

#### P2-8 已知遗留低优清单（与 MEMORY 一致）
- L-02 CORS 字段重复定义
- L-03 CSP `'unsafe-eval'` 兜底未清理
- L-05 插件接口 `map[string]interface{}` 强类型化
- M-04 分布式节点 token 鉴权
- M-05 Bridge 签名校验默认关闭

---

## 5. 修复优先级建议

| 优先级 | 任务 | 预计工时 |
|--------|------|----------|
| 立即 | P1-1 ICP 路由 + handler | 1h |
| 立即 | P1-3 quota select break | 5min |
| 本周 | P1-2 WS 在非环回 + auth 关闭时拒绝 | 30min |
| 本周 | P1-4 verifyEngineSession nil 检查 | 5min |
| 本周 | M-04/M-05 分布式 token + Bridge 签名默认开启 | 4h |
| 迭代 | 大文件拆分（`monitor_native.go`、`detector.go`） | 半天 |
| 迭代 | 测试覆盖率冲 80% | 1–2 天 |

---

## 6. 本次会话已闭环

| 项 | 提交 |
|----|------|
| CDP `WithURLs` URL scheme | `8c66e5f` |
| `defer cancel` 在 for 循环内累积 | `8c66e5f` |
| `extension_paired_session_unverified` 语义丢失（测试失败） | `8c66e5f` |
| CDP/Extension 判定逻辑去重（提取 `engineDomain` / `judgeLoginByCookieNames`） | `8c66e5f` |

**验证**：`go build ./...`、`go vet ./...`、`go test -race ./...` 全部 31 个测试包通过。
