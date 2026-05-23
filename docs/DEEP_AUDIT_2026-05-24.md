# UniMap 全面深度审查报告

> 审查日期：2026-05-24 | 审查范围：架构/业务闭环/完成度/缺陷 | 基线分支：master (5e1a215)

## 审查方法

**思考步骤与检查清单：**

1. 架构分析：目录结构 → 包依赖图 → 分层合理性 → 接口设计 → 耦合/循环检测
2. 业务闭环：6 条核心链路逐行追踪 → 断点/未实现分支 → 异常流覆盖 → 前后端对齐
3. 完成度：功能清单对照 → 代码实际实现 → 测试覆盖率 → 运维就绪度
4. 缺陷扫描：空指针/panic → 资源泄漏 → 并发竞态 → 安全漏洞 → 逻辑错误

---

## 一、架构合理性评估

### 1.1 模块划分

项目采用清晰的分层架构，核心包职责明确：

| 层 | 包 | 职责 |
|---|---|---|
| 入口层 | `cmd/` | CLI / GUI / Web 三入口 |
| 表现层 | `web/` | HTTP 路由、中间件、处理器、模板 |
| 服务层 | `internal/service/` | 5 个应用服务编排业务流程 |
| 领域层 | `internal/core/unimap/` | UQL 解析 + 结果归并 |
| 适配层 | `internal/adapter/` | 5 引擎适配 + 编排器 + 熔断 |
| 基础层 | `screenshot/tamper/scheduler/distributed/...` | 独立子系统 |

**优点：** 大部分包边界清晰，叶子包（`model`, `notify`, `auth`, `proxypool`）零内部依赖，`distributed` 包完全自包含。

**问题：**

- **`core/unimap` → `adapter` 逆向依赖**：`merger.go` 的 `MergeEngineResults` 方法接收 `adapter.EngineAdapter` 接口，导致领域层依赖适配层，违反依赖方向。应将此方法上移到 `service` 层。
- **`adapter` → `screenshot` 跨域耦合**：`web_only_base.go` 的 `BrowserQueryBackend` 接口引用 `screenshot.CollectResult`，将引擎适配与截图子系统绑定。
- **`model.EngineAdapter` 与 `adapter.EngineAdapter` 双重接口**：`model` 包定义 4 方法版，`adapter` 包定义 6 方法版（多 `GetQuota` 和 `IsWebOnly`），易混淆，应废弃 `model` 版本。
- **`web.Server` 上帝对象**：持有 18+ 子系统引用，`NewServer()` 构造约 400 行，承担过多装配职责。

### 1.2 包依赖图

```
cmd/            → service, adapter, config, web, exporter, model
web/            → service, adapter, screenshot, scheduler, config, auth, ...
service/        → adapter, core/unimap, plugin, screenshot, tamper, config, ...
adapter/        → model, screenshot, utils, logger, metrics
core/unimap/    → adapter, model, utils       (⚠️ 问题：不应依赖 adapter)
screenshot/     → model, logger, utils, metrics
tamper/         → alerting, logger, utils
scheduler/      → metrics, notify, utils/urlguard  (executor 子文件依赖 service 等)
plugin/         → model, logger
model/          → (无内部依赖 — 叶子节点)
distributed/    → (无内部依赖 — 完全自包含)
notify/         → (无内部依赖)
auth/           → (无内部依赖)
proxypool/      → (无内部依赖)
```

无 Go 编译期循环依赖，但 `core/unimap → adapter` 是概念性逆向依赖。

### 1.3 文件体积超标

| 文件 | 行数 | 建议阈值 |
|---|---|---|
| `internal/tamper/detector.go` | ~1745 | 800 |
| `internal/scheduler/scheduler.go` | ~1335 | 800 |
| `internal/screenshot/manager.go` | ~1188 | 800 |
| `web/server.go` | ~1065 | 800 |
| `internal/adapter/orchestrator.go` | ~889 | 800 |

### 1.4 代码重复

- `registerEngines()` 在 `cmd/unimap-web/main.go` 和 `cmd/unimap-cli/main.go` 间复制粘贴，应提取到共享包。

### 1.5 可扩展性评价

**良好**：适配器模式支持新引擎（实现 `EngineAdapter` 接口即可）；插件系统有 Hook + Processor 管道；调度器有 `TaskHandler` 接口支持 22 种任务类型；截图有双 Provider + 健康路由。**主要瓶颈**是 `web.Server` 单体结构，新增功能域需要持续往此结构体加字段。

---

## 二、业务逻辑闭环诊断

### 2.1 六条核心链路状态

| 链路 | 状态 | 关键发现 |
|---|---|---|
| UQL 查询 | **有缺口** | 主路径 `UnifiedService.Query` 不调用 `ResultMerger.Merge()`，多引擎结果**未去重**，相同 IP:Port 资产会重复出现 |
| 截图高可用 | **闭环** | 健康探测 + 自动降级 + 指标回调完整；`isMockBridgeClient()` 永远返回 `false` 是小瑕疵 |
| 篡改检测 | **有缺口** | `TamperCleanupRunner` 忽略 `maxAgeDays` 过滤，删除**全部**记录；批处理任务缺少 `recover()` |
| 认证登录 | **闭环** | CSRF + AES-GCM Cookie + 常量时间比较 + 限流；但无服务端会话失效机制（Cookie 泄露后 24h 有效） |
| 定时任务 | **有缺口** | `CacheWarmupRunner` 是占位实现（仅 HTTP ping）；22 个 Runner 中仅少数有 SSRF 防护 |
| 分布式 | **闭环** | 注册→心跳→任务领取→故障转移链路完整；故障转移为被动模型（节点主动认领） |

### 2.2 异常流覆盖

**健壮**：熔断器保护引擎；重试 + 指数退避贯穿各子系统；部分错误不阻断全局（`engineResults` 中 `Error` 字段收集引擎级错误）。`tamper` 检测区分 DNS/超时/TLS 错误类型。

**不足**：
- `CacheWarmupRunner` 无 SSRF 防护，可被利用访问内网
- `isPrivateOrInternalIP` 用 `context.Background()` 做 DNS 解析，无超时保护

### 2.3 前后端对齐

HTML 模板路由和 JSON API 路由双路径设计合理。`handleQuery`（HTML）和 `handleAPIQuery`（JSON）最终调同一底层方法。所有 80+ 路由均有对应处理器。**无对齐问题**。

---

## 三、项目完成度盘点

### 3.1 整体完成度：88%

| 维度 | 完成度 | 说明 |
|---|---|---|
| 核心功能 | 95% | 5 引擎查询、截图、篡改检测、定时任务、分布式均可用 |
| 安全机制 | 90% | 认证/CSRF/SSRF 防护/限流已实现，个别缺口待补 |
| 运维就绪 | 92% | Docker/CI/Grafana/Runbook 齐全 |
| 测试覆盖 | 75% | 核心包 80%+，但 `web`(53.6%)、`tamper`(53.1%)、`service`(44.8%) 仍低 |
| 文档 | 90% | API/架构/运维/插件开发文档齐全 |

### 3.2 必须补充的核心功能

1. **查询结果去重**：`UnifiedService.Query` 主路径必须调用 `ResultMerger.Merge()`，否则多引擎查询结果存在大量重复
2. **`TamperCleanupRunner` 按 `maxAgeDays` 过滤**：当前删除全部记录，破坏生产数据
3. **`CacheWarmupRunner` SSRF 防护**：添加 `urlguard` 校验，防止内网探测
4. **服务端会话失效**：登出后 Cookie 仍有效 24h，生产环境不可接受

### 3.3 建议优化的体验点

1. WebSocket 连接 ID 改用 `crypto/rand` 生成，避免纳秒碰撞
2. `web.Server` 拆分为子 Router/Handler 结构，减少上帝对象
3. `Snapshot()` 操作改用 `RLock()`，减少锁竞争
4. 提升低覆盖率包的测试（`service`、`tamper`、`web`）

---

## 四、代码质量与缺陷排查 (Bug Hunting)

### P0 — 崩溃/核心阻断

| # | 问题 | 文件:行号 | 详情 | 修复建议 |
|---|---|---|---|---|
| P0-1 | **`atomic.Value.Load()` 未检查类型断言 — 9 处 panic 风险** | `internal/screenshot/router.go:166,215,236,248,253,262,271,288,297` | `r.currentMode.Load().(ScreenshotMode)` 若 `Store` 前调用或存入错误类型则 panic | 改为 `v, ok := r.currentMode.Load().(ScreenshotMode); if !ok { return 默认值 }` |
| P0-2 | **`os.Rename` 错误静默丢弃 — 数据丢失** | `internal/distributed/task_queue.go:740` | 快照写入后 `Rename` 失败不处理，新数据丢失且临时文件残留 | 检查 error：`if err := os.Rename(...); err != nil { os.Remove(tmpPath); return }` |
| P0-3 | **`BridgeService` 用 `context.Background()` 启动 — Goroutine 泄漏** | `web/server.go:492` | Extension-only 模式下 worker 无法随服务关闭而取消 | 改用 `bridgeSvc.Start(shutdownCtx)` 与 auto 模式一致 |
| P0-4 | **context value 未检查类型断言** | `web/server.go:572` | `v.(string)` 若意外存入非 string 类型则 panic | 改为 `v, ok := v.(string); if ok { return v }` |

### P1 — 功能异常

| # | 问题 | 文件:行号 | 详情 | 修复建议 |
|---|---|---|---|---|
| P1-1 | **查询结果未去重** | `internal/service/unified_service.go:300-340` | 多引擎查询直接 `append`，不调用 `Merge()`，同 IP:Port 重复出现 | 在 normalize 后调用 `s.merger.Merge(allAssets)` |
| P1-2 | **`TamperCleanupRunner` 忽略 `maxAgeDays`，删除全部记录** | `internal/scheduler/executor.go:608-626` | `ListAllCheckRecords` 返回所有记录后逐条删除，`maxAgeDays` 仅出现在日志字符串中 | 添加时间过滤：`if record.LastCheck.Before(cutoff) { delete }` |
| P1-3 | **WebSocket 连接 ID 纳秒碰撞** | `web/websocket_handlers.go:34` | `time.Now().UnixNano()` 高并发下可碰撞，导致前连接被覆盖成孤儿 | 改用 `crypto/rand` 生成 16 字节 hex ID |
| P1-4 | **`QueryStatus` 浅拷贝共享 slice 底层数组** | `web/websocket_handlers.go:303-306` | `statusCopy` 的 `Results` slice 与原始共享 backing array，并发修改产生竞态 | 深拷贝 slice：`cp.Results = append([]model.UnifiedAsset(nil), status.Results...)` |
| P1-5 | **`defer f.Close()` 不检查错误 — 图片写入可能损坏** | `web/screenshot_bridge_handlers.go:604,621,637` | `Close()` 刷盘失败被忽略，磁盘上可能是残缺图片 | `if err := f.Close(); err != nil { os.Remove(path); return err }` |
| P1-6 | **`Snapshot()` 用排他锁 `Lock()` 而非 `RLock()`** | `internal/distributed/task_queue.go:420-436` | 只读操作阻塞全部入队/认领/提交操作 | 改用 `RLock()`；若需一致快照，用 copy-on-write 模式 |
| P1-7 | **重试延迟计算可产生负值** | `internal/distributed/task_queue.go:660-668` | `delay -= jitter` 在小 baseDelay 场景下可能为负 | 添加 `if delay < 0 { delay = 0 }` |
| P1-8 | **`SnapshotManager.Save/Load` 锁嵌套 — 死锁风险** | `internal/distributed/snapshot.go:96-112, 159-216` | 持 `SnapshotManager.mu` 后再获取 `Registry.mu` + `TaskQueue.mu`，锁序复杂 | 先收集数据释放自身锁，再逐个加子锁写入；或统一锁序 |

### P2 — 边缘情况/体验差

| # | 问题 | 文件:行号 | 详情 | 修复建议 |
|---|---|---|---|---|
| P2-1 | **`json.NewEncoder(w).Encode()` 不设 Content-Type、不检查错误** | `web/screenshot_handlers.go:258,350,439,549`; `web/tamper_handlers.go:96,150,170,199,237` | 编码失败时返回 200 + 畸形 JSON | 统一使用 `writeJSON` helper |
| P2-2 | **`filepath.Abs` 错误忽略削弱路径遍历防护** | `internal/service/screenshot_app_service.go:667,726` | `absBaseDir` 为空时 `Rel()` 结果不可靠 | 检查 error 并拒绝请求 |
| P2-3 | **`CacheWarmupRunner` 无 SSRF 防护** | `internal/scheduler/executor.go:993-1007` | 用户配置的 warmup URL 可指向内网 | 添加 `urlguard.Check()` 校验 |
| P2-4 | **`isPrivateOrInternalIP` 用 `context.Background()` 无超时** | `web/http_helpers.go:240` | DNS 解析挂起时阻塞 HTTP handler goroutine | 传入 request context 并加 5s 超时 |
| P2-5 | **`Scheduler.Stop()` 不等待通知 goroutine** | `internal/scheduler/scheduler.go:1179-1191` | `notifyWg` 未 Wait，关闭时可能写已关闭 channel | 添加 `s.notifyWg.Wait()` |
| P2-6 | **`saveLocked()` Payload 浅拷贝** | `internal/scheduler/scheduler.go:411-414` | 嵌套 map/slice 与原始共享引用 | 使用 `encoding/json` 深拷贝或手写递归拷贝 |
| P2-7 | **Cookie Secure 标志在 HTTP 下为 false** | `web/session.go:82` | 开发环境常见，生产应强制 HTTPS | 生产部署文档标注必须 HTTPS |
| P2-8 | **`os.Executable()` 错误忽略** | `web/server.go:531` | 可能导致 web root 目录查找失败 | 记录 warning 日志并降级处理 |

### 汇总统计

| 严重级别 | 数量 | 关键风险 |
|---|---|---|
| **P0** | 4 | 运行时 panic、数据丢失、goroutine 泄漏 |
| **P1** | 8 | 结果重复、数据误删、竞态条件、死锁风险 |
| **P2** | 8 | 安全防护缺口、错误处理不完整、性能损耗 |

### 优先修复建议

1. **P0-1**：9 处类型断言 panic 风险最易触发，优先修复
2. **P1-1**：查询去重是核心功能缺陷，影响用户体验
3. **P0-2 / P0-3**：数据完整性和资源泄漏
4. **P1-2**：清理任务误删生产数据
5. 其余按 P0 → P1 → P2 顺序推进
