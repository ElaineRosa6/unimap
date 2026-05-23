# UniMap 优化整改计划

> **创建日期：** 2026-05-24
> **依据文档：** [DEEP_AUDIT_2026-05-24.md](./DEEP_AUDIT_2026-05-24.md)
> **基线分支：** `master` (`5e1a215`)
> **状态：** 待实施
> **目标版本：** 下一次生产候选版本

---

## 1. 整改目标

本计划将深度审查报告中的 P0/P1/P2 缺陷、业务闭环缺口和架构债拆解为可执行整改任务。整改顺序遵循：

1. 先修复会导致 panic、数据丢失、数据误删和 goroutine 泄漏的问题。
2. 再补齐核心业务闭环，包括查询去重、清理任务过滤、SSRF 防护和会话失效。
3. 最后处理架构降耦合、文件拆分、测试覆盖和运维文档补强。

### 1.1 目标

| 编号 | 目标 | 成功标准 |
|---|---|---|
| G-1 | 消除全部 P0 缺陷 | P0 问题回归测试覆盖，`go test ./...` 通过 |
| G-2 | 修复核心 P1 功能缺陷 | 查询结果去重、清理任务不误删、WebSocket 无明显竞态 |
| G-3 | 补齐安全缺口 | CacheWarmup 与 DNS 解析链路具备 SSRF/超时防护 |
| G-4 | 降低核心模块复杂度 | `web.Server` 装配职责开始拆分，新增功能不再继续扩字段膨胀 |
| G-5 | 提升测试信心 | `service`、`tamper`、`web` 重点新增回归用例 |

### 1.2 非目标

| 编号 | 非目标 | 原因 |
|---|---|---|
| N-1 | 大规模重写 Web 层或 Scheduler | 当前主链路可用，优先做风险收敛 |
| N-2 | 一次性拆完所有超大文件 | 拆分会扩大回归面，应结合修复逐步迁移 |
| N-3 | 引入新的持久化、认证或前端框架 | 审计问题可在现有技术栈内解决 |
| N-4 | 改变统一查询 API 返回格式 | 查询去重应保持兼容，避免破坏现有调用方 |

---

## 2. 优先级总览

| 阶段 | 主题 | 对应审计项 | 建议周期 | 发布策略 |
|---|---|---|---|---|
| Phase 0 | 修复阻断性风险 | P0-1 ~ P0-4 | 1-2 天 | 可单独热修 |
| Phase 1 | 修复核心业务缺陷 | P1-1 ~ P1-5、P2-3、P2-4 | 3-5 天 | 小版本发布 |
| Phase 2 | 并发、锁和数据一致性加固 | P1-6 ~ P1-8、P2-5、P2-6 | 3-5 天 | 小版本发布 |
| Phase 3 | 架构降耦合和可维护性 | 1.1、1.3、1.4、3.3 | 5-8 天 | 合并到候选版本 |
| Phase 4 | 测试、文档和生产验证 | 3.1、P2-7、运维补充 | 2-4 天 | 发布前准入 |

---

## 3. Phase 0：阻断性风险修复

### 3.1 任务清单

| ID | 任务 | 涉及文件 | 行动 | 验收标准 |
|---|---|---|---|---|
| R0-1 | 修复 `atomic.Value.Load()` 类型断言 panic | `internal/screenshot/router.go` | 新增安全读取方法，所有读取点统一走 helper；未初始化或类型异常时返回默认模式 | 覆盖未 Store、错误类型、正常类型 3 类测试 |
| R0-2 | 修复快照 `os.Rename` 错误静默丢弃 | `internal/distributed/task_queue.go` | 检查 `os.Rename` 返回值，失败时清理临时文件并返回错误或记录明确日志 | mock/临时目录场景验证 rename 失败不会静默 |
| R0-3 | 修复 Extension-only Bridge goroutine 泄漏 | `web/server.go` | 将 `bridgeSvc.Start(context.Background())` 改为使用服务关闭上下文 | 服务关闭后 Bridge worker 可退出 |
| R0-4 | 修复 context value 类型断言 panic | `web/server.go` | `v.(string)` 改为安全断言，异常值返回空字符串或默认值 | 新增异常 context value 单测 |

### 3.2 实施顺序

1. 先改 R0-1 和 R0-4，消除最直接的 panic 面。
2. 再改 R0-2，确保分布式任务快照数据完整性。
3. 最后改 R0-3，并通过启动/关闭路径测试确认无泄漏。

---

## 4. Phase 1：核心业务闭环与安全缺口

### 4.1 查询结果归并去重

| ID | 任务 | 涉及文件 | 行动 | 验收标准 |
|---|---|---|---|---|
| R1-1 | 主查询路径调用归并器 | `internal/service/unified_service.go` | 在各引擎结果 normalize 后调用 `s.merger.Merge(allAssets)`，保留引擎错误统计 | 多引擎返回相同 `IP:Port` 时最终只保留一条资产 |
| R1-2 | 补齐去重规则测试 | `internal/service/*_test.go`、`internal/core/unimap/*_test.go` | 覆盖同 IP:Port、同域名、字段互补、引擎错误不阻断 | 关键归并分支均有测试 |
| R1-3 | 校验导出/前端兼容性 | `web/*handlers.go`、`internal/exporter/` | 确认去重后数量和字段不破坏 HTML/API/导出 | API 响应结构不变，仅结果去重 |

### 4.2 篡改清理任务防误删

| ID | 任务 | 涉及文件 | 行动 | 验收标准 |
|---|---|---|---|---|
| R1-4 | 按 `maxAgeDays` 过滤清理记录 | `internal/scheduler/executor.go` | 根据记录时间字段计算 cutoff，仅删除过期记录 | 未过期记录不会被删除 |
| R1-5 | 明确无时间字段记录策略 | `internal/tamper/`、`internal/scheduler/` | 无 `LastCheck` 或零值记录默认保留，并记录 warning | 旧数据不会因字段缺失被清空 |
| R1-6 | Runner 增加 recover 边界 | `internal/scheduler/executor.go` | 对批处理任务执行层增加 panic recover 和错误记录 | 单条异常不导致整批任务崩溃 |

### 4.3 SSRF 与超时防护

| ID | 任务 | 涉及文件 | 行动 | 验收标准 |
|---|---|---|---|---|
| R1-7 | CacheWarmupRunner 增加 SSRF 防护 | `internal/scheduler/executor.go`、`internal/utils/urlguard/` | warmup URL 请求前调用 `urlguard.Check()` 或等价校验 | 内网、loopback、metadata 地址被拒绝 |
| R1-8 | DNS 解析加超时 | `web/http_helpers.go` | `isPrivateOrInternalIP` 接收 request context 或派生 5s timeout | DNS 卡住不会长期占用 handler goroutine |
| R1-9 | 补充安全回归测试 | `web/*_test.go`、`internal/scheduler/*_test.go` | 覆盖私网地址、慢 DNS、合法公网 URL | 关键安全分支可重复验证 |

### 4.4 会话失效机制

| ID | 任务 | 涉及文件 | 行动 | 验收标准 |
|---|---|---|---|---|
| R1-10 | 增加服务端会话版本或撤销表 | `web/session.go`、认证相关 handler | Cookie 中加入 session id/version，登出后服务端标记失效 | 登出后的旧 Cookie 不能继续访问受保护接口 |
| R1-11 | 控制存储复杂度 | `web/session.go` | 单机优先使用内存撤销表，设置 TTL 清理；后续再考虑 Redis | 不引入额外部署依赖 |
| R1-12 | 补充登录/登出回归测试 | `web/*_test.go` | 覆盖正常登录、登出、旧 Cookie 重放、过期 Cookie | 会话行为可验证 |

---

## 5. Phase 2：并发、锁和数据一致性加固

| ID | 任务 | 涉及文件 | 行动 | 验收标准 |
|---|---|---|---|---|
| R2-1 | WebSocket 连接 ID 改为随机值 | `web/websocket_handlers.go` | 使用 `crypto/rand` 生成 16 字节 hex ID | 高并发连接不会因时间戳碰撞互相覆盖 |
| R2-2 | `QueryStatus` 深拷贝 slice | `web/websocket_handlers.go` | `Results` 使用新 backing array；必要时深拷贝内部引用字段 | `go test -race` 不出现相关竞态 |
| R2-3 | 图片写入检查 `Close()` 错误 | `web/screenshot_bridge_handlers.go` | 显式关闭文件，关闭失败时删除损坏文件并返回错误 | 模拟 close 失败时不留下残缺图片 |
| R2-4 | `Snapshot()` 改用读锁 | `internal/distributed/task_queue.go` | 只读快照路径使用 `RLock()`，写路径保持排他锁 | 快照读取不阻塞普通读操作，测试通过 |
| R2-5 | 重试延迟下限保护 | `internal/distributed/task_queue.go` | jitter 后对 delay 做 `max(delay, 0)` | 小 baseDelay 不产生负 duration |
| R2-6 | 统一快照锁序 | `internal/distributed/snapshot.go` | 明确 `SnapshotManager`、`Registry`、`TaskQueue` 锁顺序，避免持锁回调 | 压力测试和 race 测试无死锁 |
| R2-7 | Scheduler 停止等待通知 goroutine | `internal/scheduler/scheduler.go` | `Stop()` 中等待 `notifyWg`，避免关闭后写 channel | 停止流程稳定，无 goroutine 泄漏 |
| R2-8 | Payload 深拷贝 | `internal/scheduler/scheduler.go` | `saveLocked()` 对嵌套 map/slice 做深拷贝 | 修改原 payload 不影响持久化快照 |

---

## 6. Phase 3：架构降耦合与可维护性

### 6.1 依赖方向修正

| ID | 任务 | 涉及文件 | 行动 | 验收标准 |
|---|---|---|---|---|
| R3-1 | 移除 `core/unimap` 对 `adapter` 的逆向依赖 | `internal/core/unimap/merger.go`、`internal/service/` | 将接收 `adapter.EngineAdapter` 的逻辑上移到 service 层，领域层只处理 model/领域对象 | `core/unimap` 不再 import `internal/adapter` |
| R3-2 | 解耦 adapter 与 screenshot | `internal/adapter/web_only_base.go` | 定义更小的结果接口或 DTO，避免直接引用 `screenshot.CollectResult` | adapter 包不再绑定截图子系统实现类型 |
| R3-3 | 废弃重复接口 | `internal/model/`、`internal/adapter/` | 明确保留 `adapter.EngineAdapter`，`model.EngineAdapter` 标记 deprecated 或迁移引用 | 全仓库只有一个主适配器接口语义 |

### 6.2 `web.Server` 拆分

| ID | 任务 | 涉及文件 | 行动 | 验收标准 |
|---|---|---|---|---|
| R3-4 | 抽出依赖装配结构 | `web/server.go` | 新增 `ServerDeps` 或 `AppContainer`，`NewServer()` 只负责校验和组装 | `NewServer()` 行数明显下降，新增依赖不直接扩散 |
| R3-5 | 拆分路由注册 | `web/server.go`、`web/*_handlers.go` | 按 query/screenshot/tamper/scheduler/config 分组注册 router | 路由行为不变，单文件职责更清晰 |
| R3-6 | 提取共享引擎注册 | `cmd/unimap-web/main.go`、`cmd/unimap-cli/main.go` | 将重复 `registerEngines()` 提取到共享包 | Web/CLI 注册逻辑一致 |

### 6.3 大文件治理

| ID | 文件 | 当前问题 | 拆分建议 |
|---|---|---|---|
| R3-7 | `internal/tamper/detector.go` | 约 1745 行 | 拆为 fetch、compare、baseline、report、runner |
| R3-8 | `internal/scheduler/scheduler.go` | 约 1335 行 | 拆为 model、store、executor、notification、templates |
| R3-9 | `internal/screenshot/manager.go` | 约 1188 行 | 拆为 provider、health、routing、persistence |
| R3-10 | `web/server.go` | 约 1065 行 | 拆为 deps、routes、lifecycle、static |
| R3-11 | `internal/adapter/orchestrator.go` | 约 889 行 | 拆为 execution、circuit、quota、metrics |

拆分原则：每次只拆一个功能域，先移动代码并保持测试通过，不混入行为改动。

---

## 7. Phase 4：测试、文档和发布准入

### 7.1 测试补强

| 包 | 当前审计覆盖率 | 目标 | 重点用例 |
|---|---:|---:|---|
| `internal/service` | 44.8% | >= 65% | 多引擎归并、错误引擎降级、结果导出兼容 |
| `internal/tamper` | 53.1% | >= 70% | 清理过滤、批处理 recover、DNS/超时/TLS 错误分类 |
| `web` | 53.6% | >= 70% | 会话失效、JSON helper、WebSocket 状态复制、SSRF 拦截 |
| `internal/distributed` | 未指定 | 保持 race 通过 | 快照 rename、锁序、负 delay、Snapshot RLock |
| `internal/scheduler` | 未指定 | 保持核心 runner 覆盖 | CacheWarmup、Stop、Payload 深拷贝 |

### 7.2 必跑验证命令

```powershell
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

涉及 Web 前端或模板行为时，额外执行：

```powershell
go test ./web -run Test
```

### 7.3 文档更新

| ID | 文档 | 更新内容 |
|---|---|---|
| R4-1 | `docs/RUNBOOK.md` | 补充会话失效、Bridge 关闭、清理任务保护策略 |
| R4-2 | `docs/API.md` | 如查询结果数量语义受去重影响，说明返回资产为归并后结果 |
| R4-3 | `docs/ARCHITECTURE.md` | 更新依赖方向、引擎注册共享包、Server 拆分后的结构 |
| R4-4 | `docs/CHANGELOG.md` | 记录缺陷修复、安全加固和潜在行为变化 |
| R4-5 | 部署文档 | 强调生产环境必须启用 HTTPS，Cookie `Secure` 才能提供完整保护 |

---

## 8. 发布与回滚策略

### 8.1 发布节奏

| 发布批次 | 包含内容 | 是否可独立发布 | 回滚重点 |
|---|---|---|---|
| Hotfix-1 | Phase 0 全部 P0 修复 | 是 | 快照写入、Bridge 生命周期 |
| Patch-1 | 查询去重、清理过滤、SSRF/超时防护 | 是 | 查询结果数量、清理任务行为 |
| Patch-2 | WebSocket、快照锁、Scheduler 停止和 Payload 深拷贝 | 是 | 并发路径和任务持久化 |
| RC-1 | 架构拆分、测试补强、文档更新 | 否，建议走候选版本 | 路由注册、依赖装配 |

### 8.2 回滚预案

| 风险 | 触发信号 | 回滚动作 |
|---|---|---|
| 查询去重导致结果字段缺失 | 用户反馈资产字段减少或导出列异常 | 回滚 R1-1，保留测试样本，重新调整归并策略 |
| 清理任务过滤字段判断错误 | 过期记录未清理或误清理 | 暂停清理任务，恢复备份，修正 cutoff 字段 |
| 会话失效影响登录 | 大量 401 或用户无法保持登录 | 临时关闭撤销表校验，保留 Cookie 解密校验 |
| 锁序调整引入死锁 | `go test -race` 或压力测试挂起 | 回滚 R2-6，先补锁序设计文档再改 |
| Server 拆分引入路由遗漏 | 某些 API 404 或模板不可达 | 回滚路由拆分提交，按路由分组补测试 |

---

## 9. 里程碑

```text
第 1-2 天
  Phase 0：P0 panic / 数据丢失 / goroutine 泄漏修复

第 3-7 天
  Phase 1：查询去重、清理过滤、SSRF/超时防护、会话失效

第 8-12 天
  Phase 2：并发、锁、快照、Scheduler 停止和 Payload 深拷贝

第 13-20 天
  Phase 3：依赖方向、Server 拆分、共享引擎注册、大文件治理

第 21-24 天
  Phase 4：测试覆盖、文档、发布准入和候选版本验证
```

**总预估工作量：** 约 18-24 人天。若只做生产风险收敛，Phase 0 + Phase 1 + Phase 2 约 7-12 人天。

---

## 10. 最终验收标准

| 检查项 | 通过标准 |
|---|---|
| P0 缺陷 | 全部关闭，有回归测试或明确验证记录 |
| P1 缺陷 | 核心业务缺陷全部关闭，剩余项需有风险接受说明 |
| 安全缺口 | SSRF、DNS 超时、会话失效均有测试覆盖 |
| 并发稳定性 | `go test -race ./...` 通过，无新增数据竞争 |
| 构建质量 | `go test ./...`、`go vet ./...`、`go build ./...` 通过 |
| 兼容性 | API 响应结构不破坏，查询结果仅发生预期去重 |
| 文档 | Runbook、API、架构和 Changelog 完成必要更新 |
| 发布证据 | 记录测试命令、关键日志、回滚点和版本号 |

---

## 11. 建议立即启动的前三项

1. **R0-1：修复 `atomic.Value.Load()` 类型断言 panic。** 风险高、修复局部、适合作为第一笔改动。
2. **R1-1：统一查询结果归并去重。** 这是直接影响用户查询结果质量的核心功能缺陷。
3. **R1-4：修复 `TamperCleanupRunner` 误删风险。** 该问题可能破坏生产数据，应在任何清理任务运行前完成。
