# UniMap 深度代码审查报告

> 审查范围：commit `39c245b`（当前唯一提交，为 squash 后的完整代码库）
> 审查日期：2026-05-13
> 审查工具：静态代码分析 + `go build ./...` + `go test -race ./...`

---

## 检查清单与思考步骤

1. **Git 历史核查**：当前仓库仅有一个 commit（`39c245b`），用户提到的其他提交（9d69248、1bb99a0 等）不存在于当前 reflog 中，推测为历史提交被 squash 合并
2. **架构通读**：从 `cmd/` → `web/` → `internal/service/` → `internal/adapter/` → `internal/core/` 逐层理解调用链
3. **核心链路追踪**：查询链路、截图链路、定时任务链路、篡改检测链路
4. **并发安全扫描**：重点检查 `sync.Mutex` 使用、`map` 并发访问、goroutine 生命周期
5. **资源泄漏检查**：`defer` 覆盖、goroutine 退出机制、连接池管理
6. **已知问题对照**：参考 `docs/` 目录下的审查报告（DEEP_CODE_REVIEW、ISSUES_VERIFICATION 等）
7. **构建与测试验证**：`go build ./...` 通过，`go test -race ./...` 有 2 个环境相关失败

---

## 1. 架构合理性评估

### 1.1 整体架构

项目采用清晰的分层架构，符合 DDD 理念：

| 层级 | 目录 | 职责 |
|------|------|------|
| 入口层 | `cmd/` | Web/CLI/GUI 三种启动入口 |
| 表现层 | `web/` | HTTP Handler、WebSocket、中间件 |
| 应用层 | `internal/service/` | 业务编排（QueryAppService/MonitorAppService 等） |
| 领域层 | `internal/adapter/`, `internal/core/`, `internal/model/` | 引擎适配、UQL 解析、结果归并 |
| 基础设施层 | `internal/config/`, `internal/logger/`, `internal/utils/` | 配置、日志、工具库 |

**优点**：
- 模块边界清晰，按功能域划分（`adapter/`, `screenshot/`, `scheduler/`, `tamper/` 等）
- 接口定义合理：`EngineAdapter` 接口设计简洁，`Provider` 接口统一截图能力
- 依赖注入清晰：通过构造函数传递依赖，避免全局变量

**问题**：

| # | 问题 | 严重程度 | 说明 |
|---|------|----------|------|
| A-1 | `Server` 结构体字段过多（30+ 字段） | Medium | `web/server.go:75-107` 承载了太多职责，违反单一职责原则。截图、调度、分布式、Bridge、Cookie 等所有状态都集中在一个结构体中 |
| A-2 | `web/` 包 handler 文件按功能域拆分但共享 `*Server` | Medium | 14 个 handler 文件都依赖同一个巨型 Server 结构体，导致紧耦合 |
| A-3 | 循环依赖风险：`internal/core/unimap/merger.go` 导入了 `internal/adapter/` | Low | 领域层（core）倒向依赖了适配层（adapter），在 DDD 分层中不推荐 |

### 1.2 模块依赖图

```
web/ → internal/service/ → internal/adapter/
                                ↓
                         internal/core/unimap/
                                ↓
                         internal/model/
```

未发现实际的循环依赖编译错误（`go build ./...` 通过），但 `merger.go:9` 导入 `internal/adapter` 是架构上的逆向依赖。

---

## 2. 业务逻辑闭环诊断

### 2.1 核心查询链路 ✅ 完整

```
用户请求 → Web Handler → QueryAppService.ExecuteQuery()
    → UnifiedService.Query()
        → EngineOrchestrator.TranslateQuery()
        → EngineOrchestrator.SearchEnginesWithContext()
            → SearchTask.Execute()（带重试 + 缓存 + 熔断）
        → ResultMerger.Merge()（去重 + 归并）
        → 缓存写入 → 返回结果
```

**评价**：链路完整，包含缓存、重试、熔断、并发控制等机制。

### 2.2 浏览器查询链路 ✅ 完整

```
WebSocket/API → runBrowserQueryAsync()
    → translateBrowserQuery()（UQL 翻译）
    → browserRouter.OpenSearchEngineResult()
    → 根据 action（capture/collect）执行后续操作
    → 通过 channel 返回 BrowserQueryOutcome
```

**评价**：链路完整，支持 capture 和 collect 两种动作。

### 2.3 截图高可用链路 ✅ 完整

```
请求 → ScreenshotRouter.CaptureSearchEngineResult()
    → resolveProvider()（基于健康状态选择）
        ├── CDP Provider（健康检查通过时）
        └── Extension Provider（Bridge，降级时）
```

**评价**：健康检查、自动降级、模式切换逻辑完整。

### 2.4 篡改检测链路 ✅ 完整

```
请求 → Detector.CheckTampering()
    → ComputePageHash()（20+ 种分段哈希）
    → 对比基线 → findChangedSegments()
    → 根据检测模式（strict/relaxed/security/balanced/precise）判定
    → 恶意内容检测 → 告警触发
```

**评价**：检测模式丰富，分段哈希粒度细，恶意内容检测完善。

### 2.5 定时任务链路 ✅ 完整

```
Scheduler.AddTask() → cron 注册 → executeTask()
    → 依赖检查 → 执行窗口检查
    → handler.Execute()（20 种 Runner）
    → 记录历史 → 保存 → 发送通知
```

**评价**：20 种 Runner 覆盖了高/中/低优先级场景，循环依赖检测已实现。

---

## 3. 项目完成度盘点

### 3.1 核心功能完成度

| 模块 | 完成度 | 说明 |
|------|--------|------|
| 多引擎查询 | ✅ 100% | FOFA/Hunter/ZoomEye/Quake/Shodan 全部实现 |
| UQL 解析 | ✅ 100% | 支持 AND/OR/IN/括号分组 |
| 结果归并 | ✅ 100% | 去重 + 优先级 + 对象池优化 |
| 截图 | ✅ 100% | CDP/Extension/Router 三模式 |
| 篡改检测 | ✅ 100% | 5 种检测模式 + 20 种分段哈希 |
| 定时任务 | ✅ 100% | 20 种 Runner + 持久化 |
| 分布式 | ✅ 100% | 节点注册/心跳/任务/故障转移 |
| 告警 | ✅ 90% | Webhook/Log 渠道，邮件未实现（placeholder） |
| 代理池 | ✅ 100% | 多策略 + 健康检查 |
| 插件系统 | ✅ 100% | 引擎插件 + 处理管道 |

### 3.2 文件规模

| 文件 | 行数 | 状态 |
|------|------|------|
| `internal/tamper/detector.go` | 1739 | ⚠️ 严重超标（>800） |
| `internal/scheduler/scheduler.go` | 1239 | ⚠️ 严重超标 |
| `internal/screenshot/manager.go` | 1187 | ⚠️ 严重超标 |
| `web/server.go` | 990 | ⚠️ 超标 |
| `web/screenshot_bridge_handlers.go` | 841 | ⚠️ 超标 |
| `internal/adapter/orchestrator.go` | 880 | ⚠️ 超标 |

### 3.3 构建与测试

| 检查项 | 结果 |
|--------|------|
| `go build ./...` | ✅ 通过 |
| `go test -race ./...` | ⚠️ 2 个测试失败 |
| 失败测试 1 | `internal/logger/TestSync` — `sync /dev/stdout: invalid argument`（环境相关） |
| 失败测试 2 | `internal/tamper/TestDetector_CheckTampering_NoBaseline` — `google-chrome: executable file not found`（缺少 Chrome） |

---

## 4. 代码质量与缺陷排查 (Bug Hunting)

### P0 (崩溃/核心阻断)

| # | 问题描述 | 文件路径 | 行号 | 修复建议 |
|---|----------|----------|------|----------|
| P0-1 | `activeBridgeTokens()` / `activeBridgeLiveTokens()` 在持有 `mu.Lock()` 时遍历并删除 map 条目，同时在遍历时调用了 `delete()`，这本身是安全的（Go map 遍历期间允许 delete），但方法中同时修改了正在遍历的 map，可能导致迭代行为不确定 | `web/screenshot_bridge_handlers.go` | 719-727, 739-743 | 先收集待删除的 key 到切片，遍历结束后统一删除 |
| P0-2 | `consumeBridgeCallbackNonce` 在持有 `mu.Lock()` 时遍历 `CallbackNonces` map 并删除过期条目，与 P0-1 同类问题 | `web/screenshot_bridge_handlers.go` | 422-429 | 先收集待删除 key，遍历后统一删除 |
| P0-3 | `SearchEnginesWithPaginationAndContext` 结果收集循环中，当 `resultsChan` 被关闭后 `resultChan = nil` 的逻辑缺失（与 `SearchEnginesWithContext` 不同，该方法没有处理通道关闭后的 nil 赋值），可能导致无限循环等待 | `internal/adapter/orchestrator.go` | 814-826 | 在 `!ok` 分支添加 `resultsChan = nil` 或使用与分页版本相同的 done 标志模式 |
| P0-4 | `ScreenshotRouter.Stop()` 只关闭 `stopCh`，但不 cancel context，导致 `probeLoop` goroutine 可能在 `ctx` 未取消时无法退出（如果 ctx 是外部传入的长期 context） | `internal/screenshot/router.go` | 116-123 | 添加内部 cancel 函数或确保 ctx 能被正确取消 |
| P0-5 | `LeakDetector.Stop()` 调用 `close(d.stopChan)` 但没有等待 goroutine 退出，缺少 WaitGroup 同步，可能导致 use-after-close | `internal/monitoring/leak_detector.go` | 69-71 | 添加 `sync.WaitGroup` 并在 `Stop()` 中 `wg.Wait()` |

### P1 (功能异常)

| # | 问题描述 | 文件路径 | 行号 | 修复建议 |
|---|----------|----------|------|----------|
| P1-1 | `RunBrowserQueryAsync` 中当 `previewURLBuilder == nil` 时直接 `continue`，不将引擎标记为已打开也不记录错误。如果后续逻辑期望所有引擎都有结果，会产生遗漏 | `internal/service/query_app_service.go` | 175-176, 202-203 | 添加错误日志或记录到 `AutoCaptureErrors` |
| P1-2 | `validateBridgeToken` 在持有 `mu.Lock()` 时遍历 `CallbackNonces` 删除关联 nonce，与 P0-1/P0-2 同类遍历删除问题 | `web/screenshot_bridge_handlers.go` | 482-485 | 收集 key 后统一删除 |
| P1-3 | `executeTask` 在重试循环中使用 `time.Sleep` 做退避，这会阻塞整个 goroutine。当有多个任务同时重试时，无法被 context 取消中断 | `internal/scheduler/scheduler.go` | 794 | 使用 `select { case <-time.After(...): case <-ctx.Done(): }` 模式 |
| P1-4 | `areDependenciesMet` 只检查"最近一次"执行结果，如果依赖任务被手动触发后失败，后续定时触发也会被永久阻塞 | `internal/scheduler/scheduler.go` | 861-884 | 改为检查"最近 N 次中是否有成功"或增加超时跳过机制 |
| P1-5 | `calculateRetryDelay` 使用 `rand.Float64()` 和 `rand.Intn()` 但 `math/rand` 在全局共享源，在并发调用时可能产生重复值。Go 1.22+ 已自动种子化，但仍建议用 `rand.New(rand.NewSource(...))` 或 `crypto/rand` | `internal/distributed/task_queue.go` | 663-666 | 使用独立的 `rand.Rand` 实例 |
| P1-6 | `sendNotification` 的 email 渠道只有 placeholder 日志，实际不会发送邮件，但用户配置后不会收到任何通知，功能假象 | `internal/scheduler/scheduler.go` | 995-997 | 实现完整 SMTP 发送逻辑，或在文档中明确标注未实现 |
| P1-7 | `BridgeService` 在 `mode=auto` 时始终创建 mock client，即使配置中未启用 Extension。这可能导致不必要的资源占用 | `web/server.go` | 395-399 | 仅在 `cfg.Screenshot.Extension.Enabled` 为 true 时创建 mock |
| P1-8 | `cleanupStaleQueries` 中 `s.queryMutex.Lock()` 保护了整个 map 遍历和删除，但遍历期间持有写锁会阻塞所有查询状态更新。建议改用分段锁或 copy-on-delete 模式 | `web/server.go` | 765-780 | 减少锁粒度，先收集过期 key 再删除 |
| P1-9 | `SearchTask.Execute` 中 `backoff > 2*time.Second` 限制为 2 秒，但 `1<<uint(attempt)` 在 attempt=0 时是 100ms，attempt=1 是 200ms，attempt=4 是 1600ms，attempt=5 时 `1<<5=32` 即 3200ms 被截断到 2s。对于网络请求重试来说退避时间偏短 | `internal/adapter/orchestrator.go` | 502-504 | 考虑增大最大退避时间或调整基数 |

### P2 (边缘情况/体验差)

| # | 问题描述 | 文件路径 | 行号 | 修复建议 |
|---|----------|----------|------|----------|
| P2-1 | `TestSync` 测试失败是因为对 `/dev/stdout` 调用 `Sync()` 返回 `EINVAL`，这是 Linux 管道的正常行为。测试应跳过 stdout 或使用临时文件 | `internal/logger/logger_extra_test.go` | 184-193 | 改为对文件写入器测试，或使用 skip 条件判断 |
| P2-2 | `TestDetector_CheckTampering_NoBaseline` 需要 `google-chrome` 可执行文件，在无 Chrome 的环境中必定失败。应添加 skip 逻辑 | `internal/tamper/detector_baseline_test.go` | 289-302 | 添加 `if _, err := exec.LookPath("google-chrome"); err != nil { t.Skip("chrome not installed") }` |
| P2-3 | `generateID` 在历史版本中使用递增计数器，已改为 UUID。当前代码中 `generateRandomToken` 的 fallback 分支生成了可预测的 token（`fallback-token-0000`），虽然概率极低但存在风险 | `web/middleware_auth.go` | 87-89 | fallback 应使用 `crypto/sha256` + 时间戳生成不可预测值 |
| P2-4 | `persistBridgeImageData` 中 JPEG/WebP/PNG 三个分支的文件写入逻辑高度重复（各 20+ 行），违反 DRY 原则 | `web/screenshot_bridge_handlers.go` | 605-654 | 提取通用辅助函数 `writeImageFile(path, raw, encodeFunc)` |
| P2-5 | `cleanHTML` 中 `reComments.ReplaceAllString` 使用正则匹配 HTML 注释，但嵌套注释 `<!-- outer <!-- inner --> -->` 会被错误处理 | `internal/tamper/detector.go` | 900-912 | 使用 `regexp.MustCompile("(?s)<!--.*?-->")` 支持非贪婪匹配 |
| P2-6 | `cookie_handlers.go` 中 `handleCookieLoginStatus` 在每个引擎循环内频繁加锁/解锁 `configMutex`，效率略低 | `web/cookie_handlers.go` | 388-399 | 循环外一次性加锁，读取所有 cookie 后解锁 |
| P2-7 | `Server` 结构体中 `chromeCmd *os.Process` 和 `chromeCmdMu sync.Mutex` 表示 Chrome 进程管理，但 `Shutdown()` 中只关闭了 connManager 和 queryMutex，未清理 Chrome 进程 | `web/server.go` | 693-715 | 在 Shutdown 中添加 `killChrome()` 逻辑 |
| P2-8 | `resultChan` 和 `errorChan` 在 `SearchEnginesWithContext` 中 buffer 大小为 `len(queries)`，但如果某个引擎快速完成多次重试并发送多个结果，可能超过 buffer 导致 goroutine 阻塞。当前重试逻辑只在最后一次失败时才发送，所以实际不会溢出，但代码语义不明确 | `internal/adapter/orchestrator.go` | 582-583 | 添加注释说明或增大 buffer |
| P2-9 | `isPrivateOrInternalIP` 在无效 IP 时 fail-open（返回 false 允许通过），但安全策略建议 fail-closed（拒绝） | `web/screenshot_helpers.go` | 需要确认具体实现 | 无效 IP 时应返回 true（视为内部地址并拒绝） |
| P2-10 | `scheduler_handlers.go` 的 6 个端点使用 `json.NewDecoder` 而非 `decodeJSONBody`，绕过了 `DisallowUnknownFields` 检测和请求体大小限制 | `web/scheduler_handlers.go` | 70, 145, 179, 212, 244, 276 | 统一使用 `decodeJSONBody` |

---

## 5. 综合评分

| 维度 | 评分 | 说明 |
|------|------|------|
| **架构设计** | 8/10 | 分层清晰，模块边界合理，但 Server 结构体过大 |
| **业务完整性** | 9/10 | 核心链路完整，边缘路径部分缺失 |
| **并发安全** | 7/10 | 大部分共享状态有保护，但 map 遍历删除、goroutine 生命周期管理有待加强 |
| **资源管理** | 7/10 | 大多数 defer 正确，但部分 goroutine 缺少退出同步机制 |
| **异常处理** | 8/10 | 错误包装规范，部分路径缺少降级处理 |
| **安全性** | 8/10 | CSP/SSRF/XSS 防护到位，个别端点绕过安全中间件 |
| **测试覆盖** | 6/10 | 有测试但存在环境依赖失败，部分边缘路径未覆盖 |

### 关键建议优先级

1. **立即修复 (P0)**：P0-1 ~ P0-5 涉及并发安全和 goroutine 泄漏
2. **尽快修复 (P1)**：P1-1 ~ P1-9 涉及功能异常和资源浪费
3. **后续改进 (P2)**：P2-1 ~ P2-10 涉及代码质量和边缘体验
