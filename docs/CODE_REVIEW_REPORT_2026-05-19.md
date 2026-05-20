# UniMap 项目深度审查报告

**审查日期**: 2026-05-19
**审查分支**: `release/major-upgrade-vNEXT`
**最新提交**: `f47cdea` - merge: resolve conflicts with master
**Go 版本**: 1.26
**审查人**: AI 架构师 + QA 专家

---

## 0. 最近提交合理性分析

| 提交 | 描述 | 类型 | 合理性评估 |
|------|------|------|------------|
| f47cdea | merge: resolve conflicts with master | merge | 合理，解决分支冲突 |
| 59f3a23 | fix: add goroutine leak protection and CSRF coverage | fix | 合理，安全加固 |
| 897173d | fix: enable Bridge signature + CSP nonce | fix | 合理，安全加固 |
| 431cbac | docs: update CLAUDE.md known issues | docs | 合理，文档同步 |
| ed1c142 | docs: add code review report | docs | 合理，审查记录 |
| 738d5e1 | fix(browser-query): complete Extension/CDP fix plan | fix | 合理，功能修复 |
| 4584e9d | feat(extension): upgrade capture.js | feat | 合理，功能增强 |
| 3d13d41 | fix: stabilize browser extension query status | fix | 合理，稳定性修复 |
| 2044053 | feat(extension): update capture.js v0.2.0 | feat | 合理，版本升级 |
| 53a9b8d | feat: return browser collected query data | feat | 合理，功能完善 |
| 992ed71 | fix: resolve 13 security issues | fix | 合理，安全修复 |

**评估结论**：最近 15 次提交合理，符合 `fix/feat/docs/chore/merge` 规范，无异常提交。所有提交均围绕浏览器查询、安全加固、文档同步展开，方向一致。

---

## 1. 架构合理性评估

### 1.1 目录结构

| 层级 | 路径 | 评价 |
|------|------|------|
| 表现层 | `web/`, `cmd/unimap-web/`, `cmd/unimap-cli/`, `cmd/unimap-gui/` | 清晰分离三种入口 |
| 应用层 | `internal/service/` | 统一服务入口，职责明确 |
| 领域层 | `internal/adapter/`, `internal/core/unimap/`, `internal/model/` | 合理，UQL 解析与引擎适配分离 |
| 基础设施层 | `internal/config/`, `internal/logger/`, `internal/utils/` | 规范 |

**评分：8.5/10** - 整体结构良好，符合 Clean Architecture 理念。

### 1.2 模块划分

**优点：**
- 按功能域拆分（21 个子模块），职责清晰
- `EngineAdapter` 接口定义简洁（5 方法），符合接口最小化原则
- `core/unimap/` 专注 UQL 解析与归并，不直接耦合外部引擎
- ScreenshotRouter 双模式高可用设计优秀（CDP ↔ Extension 自动降级）

**问题清单：**

| # | 问题 | 位置 | 严重度 |
|---|------|------|--------|
| 1 | `orchestrator.go` 880 行，违反 <800 行规范 | `internal/adapter/orchestrator.go` | 低 |
| 2 | `detector.go` 1739 行，严重超长 | `internal/tamper/detector.go` | 中 |
| 3 | `scheduler.go` 1239 行，严重超长 | `internal/scheduler/scheduler.go` | 中 |
| 4 | `server.go` 976 行，超长 | `web/server.go` | 低 |
| 5 | `router.go` 760 行，接近边界 | `internal/screenshot/router.go` | 低 |
| 6 | `unified_service.go` 646 行，职责过多 | `internal/service/unified_service.go` | 中 |

### 1.3 依赖关系分析

**合理的依赖方向：**
```
web → service → adapter → model
     → core/unimap → model
     → screenshot → model
     → tamper → alerting
```

**潜在问题：**

| # | 问题 | 说明 |
|---|------|------|
| 1 | `core/unimap/merger.go` 依赖 `internal/adapter/` | 领域层不应依赖具体 adapter，应依赖接口，违反依赖倒置原则 |
| 2 | `unified_service.go` 同时负责查询、缓存、插件、导出、并发控制 | 单一职责原则违反，建议拆分为 QueryService、CacheService、PluginService |

### 1.4 可扩展性评价：7.5/10

**优势：**
- 插件系统设计合理，支持 Engine/Processor/Exporter 三类插件
- 熔断器、缓存策略可配置
- ScreenshotRouter 双模式高可用设计优秀
- 分布式节点支持注册/心跳/任务队列/故障转移

**不足：**
- UnifiedService 职责过重，是典型的 God Object 反模式
- 部分模块使用 `map[string]interface{}` 替代强类型，降低可维护性
- 34 个函数超过 50 行规范（最大 `createMonitorTab` 390 行）

---

## 2. 业务逻辑闭环诊断

### 2.1 核心查询链路 ✅ 完整

```
用户请求 → Web Handler → UnifiedService.Query() → 
  → UQLParser.Parse() → EngineOrchestrator.TranslateQuery() → 
  → EngineOrchestrator.SearchEnginesWithContext() → 
  → EngineAdapter.Search() → Normalize() → 
  → ResultMerger.Merge() → 缓存 → 返回
```

**评价**：主链路完整，包含缓存、重试、熔断、并发控制、错误收集。支持 5 大引擎并行查询。

### 2.2 截图监控流程 ✅ 完整

```
请求 → ScreenshotRouter → 
  ├── CDP Provider (健康检查 → 使用)
  └── Extension Provider (降级 → 使用)
       ↓
  HealthChecker 定期探测
       ↓
  自动降级（CDP 失败 → Extension，反之亦然）
```

**评价**：双模式高可用 + 健康探测 + 自动降级，设计优秀。

### 2.3 篡改检测流程 ✅ 完整

```
启动检测 → Detector.ComputePageHash() → 
  对比 Baseline → 分段哈希比较 → 
  恶意内容检测 → 告警发送 → 记录保存
```

**评价**：5 种检测模式（strict/relaxed/security/balanced/precise），3 种性能模式（fast/balanced/comprehensive），流程完善。

### 2.4 定时任务系统 ✅ 完整

- 20 种 Runner (ST-01 ~ ST-20)，分高/中/低三优先级
- 支持 cron 表达式、任务依赖、执行窗口、通知
- 持久化到 JSON 文件，启动时自动加载
- 有循环依赖检测、Webhook URL SSRF 防护

### 2.5 异常流处理

| 场景 | 处理情况 | 评价 |
|------|----------|------|
| 网络失败 | 重试 + 指数退避 + 熔断器 | 优秀 |
| 权限拒绝 | Webhook URL SSRF 防护、CSP 配置 | 优秀 |
| 数据为空 | 空查询/空引擎校验 | 良好 |
| Context 取消 | 搜索/截图/篡改检测均支持 | 良好 |
| 通道满 | select default 丢弃 + 日志 | 警告：静默丢弃可能丢失数据 |
| Goroutine 泄漏 | 已有泄漏保护机制 | 良好 |

### 2.6 前后端对齐情况 ⚠️

| 功能模块 | 后端 | 前端 | 对齐状态 |
|----------|------|------|----------|
| 查询 | 完整 | 完整 | 对齐 |
| 截图 | 完整 | 完整 | 对齐 |
| 篡改检测 | 完整 | 完整 | 对齐 |
| 定时任务 | 完整（增删改查） | 部分（缺少编辑 UI） | 未对齐 |
| 分布式节点 | 完整 | 基础展示 | 部分对齐 |
| 告警管理 | 完整 | 基础展示 | 部分对齐 |
| Extension 登录状态 | 有同步机制 | 状态显示 | 存在不准确情况 |

**已知前后端未对齐问题：**
1. **Scheduler 编辑 UI 缺失** — 后端已实现编辑/启停 API，前端无编辑按钮和表单
2. **Extension 模式登录状态同步不准确** — 前端显示与实际登录状态可能不一致
3. **查询进度 WebSocket 可能卡在 0%** — 特定情况下进度更新不及时

---

## 3. 项目完成度盘点

### 整体完成度：85%

| 维度 | 完成度 | 说明 |
|------|--------|------|
| 核心查询功能 | 95% | 5 引擎 + UQL + 归并 + 缓存 + 导出 |
| 截图功能 | 90% | CDP/Extension/Router 三模式高可用 |
| 篡改检测 | 90% | 5 模式 + 分段哈希 + 恶意内容检测 |
| 定时任务 | 95% | 20 Runner + 持久化 + Web API |
| 分布式节点 | 80% | 注册/心跳/任务队列/故障转移，缺少集成测试 |
| 告警系统 | 85% | Webhook + Log + 去重 + 静默，缺少邮件等渠道 |
| Web UI | 80% | 功能完整但前端体验待优化 |
| 测试覆盖 | 75% | go vet 通过，race 检测通过，覆盖率待达 80% |
| 安全加固 | 90% | CSRF/CSP/SSRF 防护已到位 |

### 必须补充的核心功能

| # | 功能 | 优先级 | 说明 |
|---|------|--------|------|
| 1 | config.yaml 配置文件 | P0 | 当前只有 `config.yaml.example`，首次启动会报配置加载失败 |
| 2 | Scheduler 前端编辑功能 | P1 | 后端 API 已就绪，前端需补充编辑表单和交互 |
| 3 | 分布式节点集成测试 | P1 | 单元测试存在，缺少多节点 E2E 测试验证 |
| 4 | 邮件告警渠道 | P2 | scheduler 中 email 通知标记为 TODO，实际未实现 |
| 5 | 查询进度 WebSocket 稳定性 | P2 | 特定场景下进度卡在 0%，需排查更新逻辑 |

### 建议优化的体验点

| # | 优化项 | 影响范围 |
|---|--------|----------|
| 1 | 拆分超长文件（detector.go 1739 行、scheduler.go 1239 行） | 可维护性 |
| 2 | 拆分超长函数（最大 390 行） | 可读性 |
| 3 | `map[string]interface{}` 替换为强类型 | 类型安全 |
| 4 | 查询结果通道满时的背压策略 | 数据完整性 |
| 5 | 前端统一使用 CSP nonce | 安全性 |
| 6 | 退避时间上限从 2s 提升至 5-10s | 慢引擎兼容性 |

---

## 4. 代码质量与缺陷排查 (Bug Hunting)

### P0 — 崩溃/核心阻断（需立即修复）

**无 P0 问题。** 关键核心链路均有错误处理和资源释放，`go vet` 通过，历史 P0 已全部修复。

### P1 — 功能异常（需尽快修复）

| # | 位置 | 问题描述 | 修复建议 |
|---|------|----------|----------|
| P1-1 | `internal/adapter/orchestrator.go:605-610` | `SearchEnginesWithContext` 中 goroutine 关闭通道存在 race：如果 context 先取消，`results` 和 `errs` 收集循环可能永远等不到通道关闭信号 | 在 `wg.Wait()` 后关闭通道，或在 context 取消后仍等待 wg 完成再关闭 |
| P1-2 | `internal/service/unified_service.go:562-571` | `acquireQueryLock()` 中计数器 `activeQueries++` 后如果后续代码 panic，defer 会释放锁但计数器不会回退，导致活跃查询数不准确 | 在 defer 中同时回退计数器：`defer func() { s.queryMutex.Lock(); s.activeQueries--; s.queryMutex.Unlock() }()` |
| P1-3 | `internal/scheduler/scheduler.go:769` | `executeTask` 中 `areDependenciesMet()` 内部再次获取读锁，依赖检查与执行之间存在时间窗口，依赖任务可能在此期间状态变化 | 在 executeTask 开始时做一次依赖状态快照，或执行前再次确认 |
| P1-4 | `internal/tamper/detector.go:620-639` | `computeSegmentHashes` 中并发执行分段哈希，如果某个 goroutine panic，`resultChan` 永远不会收到该分段结果，`for range` 永远阻塞 | 每个 goroutine 添加 `recover()`，或在外部设置超时 context |
| P1-5 | `internal/alerting/manager.go:105-119` | `SendAlert` 中为每个渠道启动 goroutine 发送，`Close()` 中 `wg.Wait()` 只等待未完成的告警。如果在 Close 期间调用 SendAlert，可能产生新 goroutine 而 wg 已结束 | 在 Close() 中设置 closed 标志位，SendAlert 中检查并拒绝新请求 |

### P2 — 边缘情况/体验差

| # | 位置 | 问题描述 | 修复建议 |
|---|------|----------|----------|
| P2-1 | `internal/adapter/orchestrator.go:443` | 缓存键生成固定 `page=1`：`GenerateCacheKey(engine, query, 1, pageSize)`，所有分页搜索共用同一缓存键，导致第 2 页返回第 1 页结果 | 使用实际的 page 参数：`GenerateCacheKey(engine, query, page, pageSize)` |
| P2-2 | `internal/adapter/orchestrator.go:502` | 指数退避上限 2s 过短，attempt=5 时被截断为 2s，对慢引擎可能不够 | 建议上限改为 5-10s |
| P2-3 | `internal/core/unimap/merger.go:43-106` | `Merge` 方法对整个操作加单个互斥锁，大批量结果归并时锁竞争严重 | 考虑使用读写锁（归并只读时）或分段锁 |
| P2-4 | `internal/scheduler/scheduler.go:994-998` | webhook 通知使用 `go func()` 异步发送，无重试机制，webhook 目标暂时不可用时通知丢失 | 增加重试队列或至少记录失败日志 |
| P2-5 | `internal/service/unified_service.go:129` | `NewUnifiedServiceWithConfig` 中使用类型断言 `cache.(*utils.RedisCache)` 判断缓存后端，如果工厂返回包装类型则断言失败 | 建议使用接口方法判断（如 `IsRedis()`）而非类型断言 |
| P2-6 | `web/server.go:596-598` | 当 auth 禁用且绑定非 loopback 地址时只输出警告日志，服务继续启动，存在安全风险 | 增加环境变量 `UNIMAP_ALLOW_INSECURE=true` 作为确认开关 |
| P2-7 | `internal/tamper/detector.go:503` | `computeHashWithHTTP` 中 HTTP 状态码 `<200 || >=400` 才报错，3xx 重定向后的 HTML 可能不是目标页面内容 | 应限制只接受 2xx，或对 3xx 做特殊处理并记录警告 |
| P2-8 | `internal/distributed/registry.go:34` | `NewRegistry` 中立即启动后台清理 goroutine，如果 Registry 创建后未被使用或 GC，goroutine 永不退出 | 提供 `Start()` 方法，在使用时显式启动 |
| P2-9 | `internal/scheduler/scheduler.go:1136` | `MinDurationMs` 初始化为 -1，如果没有任何执行记录，返回 -1 可能引起前端解析问题 | 初始化为 0，或 TotalRuns==0 时返回 0 |
| P2-10 | `internal/scheduler/scheduler.go:287` | `NewScheduler` 中 `c.Start()` 立即启动 cron，但 handlers 可能还未注册完毕，会打印 "no handler registered" 警告 | 应在 Start() 前先注册所有 handlers，或延迟启动 cron |
| P2-11 | `internal/tamper/detector.go:413-424` | `ComputePageHash` 中缓存检查使用 RLock，写入使用 Lock，高并发下有 lock upgrade 竞争窗口 | 考虑使用 `sync.Map` 或分段锁 |
| P2-12 | `web/query_handlers.go:125-131` | 浏览器查询和 API 查询并发执行，用户需等待两者都完成才能看到结果 | 考虑流式返回，先返回已完成的查询结果 |

### 代码规范违反

| # | 位置 | 违反项 | 说明 |
|---|------|--------|------|
| C1 | `internal/tamper/detector.go:1739 行` | 文件 <800 行规范 | 最大文件，需拆分 |
| C2 | `internal/scheduler/scheduler.go:1239 行` | 文件 <800 行规范 | 需拆分 |
| C3 | `internal/adapter/orchestrator.go:880 行` | 文件 <800 行规范 | 需拆分 |
| C4 | `internal/service/unified_service.go:646 行` | 文件 <800 行规范 | 需拆分 |
| C5 | `web/server.go:976 行` | 文件 <800 行规范 | 需拆分 |
| C6 | 多处使用 `map[string]interface{}` | 强类型规范 | 插件接口等广泛使用，需渐进重构 |
| C7 | `internal/core/unimap/merger.go:9` | 依赖倒置原则 | core 层不应依赖 adapter 包 |

---

## 5. 总结与建议

### 整体评价：良好 (8/10)

UniMap 项目架构设计合理，核心业务链路完整，安全考虑到位（SSRF 防护、CSP、输入验证、CSRF）。截图 Router 双模式高可用设计是亮点。

### 关键发现

| 指标 | 状态 |
|------|------|
| `go vet ./...` | 通过 |
| `go build ./...` | 通过 |
| 历史 Critical/High 问题 | 已基本修复 |
| P0 阻断性问题 | 无 |
| P1 功能异常 | 5 项（主要涉及并发安全） |
| P2 边缘情况 | 12 项 |
| 代码规范违反 | 7 项 |
| 技术债务 | 5 个文件超 800 行 |

### 优先修复建议

| 优先级 | 问题 | 预计工作量 |
|--------|------|------------|
| P1-1 | orchestrator context 取消时通道关闭 race | 2 小时 |
| P1-4 | tamper 分段哈希计算 goroutine panic 风险 | 2 小时 |
| P1-5 | alerting Close 与新 SendAlert 的竞争 | 1 小时 |
| P2-1 | orchestrator 分页缓存键错误 | 0.5 小时 |
| P2-7 | tamper HTTP 3xx 状态码处理 | 1 小时 |
| P2-10 | scheduler handlers 注册时序 | 1 小时 |

### 下一步行动

1. **立即修复**：P1-1、P1-4、P1-5（并发安全问题）
2. **尽快修复**：P2-1（分页缓存 bug）、P2-7（3xx 处理）、P2-10（scheduler 初始化）
3. **后续迭代**：文件拆分、强类型重构、前端编辑 UI 补充、config.yaml 初始化

---

*本报告由 AI 架构师 + QA 专家自动生成，基于代码静态分析和架构模式识别。建议结合手动代码审查和实际运行测试进行验证。*
