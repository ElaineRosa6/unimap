# UniMap 修复验证审计报告

> **审计日期**: 2026-05-20  
> **验证分支**: `release/major-upgrade-vNEXT`  
> **基线提交**: `a4c8641` (2026-05-19 docs: add comprehensive code review report)  
> **Go 版本**: 1.26  
> **审计人**: AI Agent

---

## 0. 审计背景

基于两份报告进行修复验证：
- `docs/SECURITY_AUDIT_REPORT.md` — 全量安全审计（25 + 24 = 49 项问题）
- `docs/CODE_REVIEW_REPORT_2026-05-19.md` — 深度代码审查（5 P1 + 12 P2 + 7 规范违反）
- `docs/FIX_REPORT_2026-05-19.md` — 声称已修复 5 项问题（2 P1 + 3 P2）

---

## 1. 环境状态

| 项目 | 结果 |
|------|------|
| 构建 `go build ./...` | ✅ 通过 |
| 静态检查 `go vet ./...` | ✅ 通过 |
| 测试 `go test -race ./...` | ✅ 30/32 通过 |
| Race 检测 | ✅ 0 数据竞争 |
| Pre-existing 失败 | 2（logger TestSync 缺 /dev/stdout、tamper 缺 chrome，非本次引起） |
| 基线后新提交数 | **0**（自 `a4c8641` 以来无任何代码提交） |

---

## 2. 安全审计报告验证 (SECURITY_AUDIT_REPORT.md)

### 2.1 严重问题 (Critical)

| 编号 | 问题 | 报告声称 | 实际验证 | 结论 |
|------|------|----------|----------|------|
| C-01 | WebSocket 时序攻击 | ✅ 已修复 | 代码使用 `subtle.ConstantTimeCompare` | ✅ 确认已修复 |
| C-02 | 认证默认禁用 | ✅ 已修复 | `applyDefaults` 生成随机 token | ✅ 确认已修复 |
| C-03 | 端口扫描 SSRF | ✅ 已修复 | `monitor_handlers.go` 有内网过滤 | ✅ 确认已修复 |
| C-04 | Webhook SSRF | ✅ 已修复 | `validateWebhookURL` 已实现 | ✅ 确认已修复 |

### 2.2 高优先级 (High)

| 编号 | 问题 | 报告声称 | 实际验证 | 结论 |
|------|------|----------|----------|------|
| H-01 | /metrics 端点无认证 | ❌ 未修复 | `/metrics` 不在 `isPublicPath`，需认证 | ✅ 已在更早提交中修复 |
| H-02 | /health 泄露引擎列表 | ❌ 未修复 | 只返回 `status`+`time`，无引擎信息 | ✅ 已在更早提交中修复 |
| H-03 | 默认绑定 0.0.0.0 | ❌ 未修复 | `bindAddr()` 默认返回 `127.0.0.1` | ✅ 已在更早提交中修复 |
| H-04 | queryStatus 无限增长 | ❌ 未修复 | `cleanupStaleQueries()` 已实现并定时调用 | ✅ 已在更早提交中修复 |
| H-05 | Chrome 调试地址可配置 | ❌ 未修复 | `cdp_handlers.go` 校验并强制回环地址 | ✅ 确认已修复 |
| H-06 | Payload 深度校验 | ❌ 未修复 | 待进一步验证 | ⏳ 待验证 |

### 2.3 中优先级 (Medium)

| 编号 | 问题 | 报告声称 | 实际验证 | 结论 |
|------|------|----------|----------|------|
| M-01 | Bridge PairCode 未校验 | ❌ 未修复 | 只检查非空，无实际配对码比对验证 | ❌ **仍未修复** |
| M-02 | Bridge Token 清理加锁遍历 | ❌ 未修复 | 待进一步验证 | ⏳ 待验证 |
| M-04 | CSP unsafe-inline | ❌ 未修复 | 待进一步验证 | ⏳ 待验证 |
| M-05 | isTrustedRequest 默认放行 | ❌ 未修复 | 待进一步验证 | ⏳ 待验证 |
| M-06 | sortInt64 冒泡排序 | ❌ 未修复 | 待进一步验证 | ⏳ 待验证 |
| M-07 | 统一 JSON 解码 | ❌ 未修复 | scheduler 已改为 `decodeJSONBody`，**但 bridge 仍有 3 处** `json.NewDecoder` | ⚠️ 部分修复 |
| M-08 | 循环依赖检测 | ❌ 未修复 | 待进一步验证 | ⏳ 待验证 |

### 2.4 低优先级 (Low)

| 编号 | 问题 | 报告声称 | 实际验证 | 结论 |
|------|------|----------|----------|------|
| L-02 | generateID 可预测 | ❌ 未修复 | 待进一步验证 | ⏳ 待验证 |
| L-03 | Webhook 占位实现 | ❌ 未修复 | 待进一步验证 | ⏳ 待验证 |
| L-06 | calculateRetryDelay 随机源 | ❌ 未修复 | 待进一步验证 | ⏳ 待验证 |

---

## 3. 代码审查修复报告验证 (FIX_REPORT_2026-05-19.md)

修复报告声称 5 项 **DONE**，实际逐行验证结果：

### 3.1 P1 — 功能异常

| 编号 | 问题 | 修复报告状态 | 实际代码状态 | 结论 |
|------|------|-------------|-------------|------|
| P1-1 | orchestrator context race | SKIPPED (误报) | 代码逻辑正确 | ✅ 确认跳过合理 |
| P1-2 | acquireQueryLock panic 不回退 | **DONE** | `runWithQueryLock()` 方法**不存在**，仍使用原 `acquireQueryLock()` | ❌ **未修复** |
| P1-3 | scheduler TOCTOU | SKIPPED (部分夸大) | cron 串行触发，风险低 | ✅ 确认跳过合理 |
| P1-4 | detector goroutine panic | **DONE** | `computeSegmentHashes` 中 goroutine **无 `recover()`** | ❌ **未修复** |
| P1-5 | alerting Close 竞争 | SKIPPED (误报) | 应用关闭时可接受 | ✅ 确认跳过合理 |

### 3.2 P2 — 边缘情况

| 编号 | 问题 | 修复报告状态 | 实际代码状态 | 结论 |
|------|------|-------------|-------------|------|
| P2-1 | 分页缓存键固定 page=1 | **DONE** | `orchestrator.go:443,482` 仍硬编码 `1` | ❌ **未修复** |
| P2-2 | 退避上限 2s | SKIPPED | 保持现状 | ✅ 确认跳过合理 |
| P2-7 | HTTP 3xx 当作成功 | **DONE** | `detector.go:503` 仍为 `>=400` | ❌ **未修复** |
| P2-9 | MinDurationMs 初始化为 -1 | **DONE** | `scheduler.go:1136` 仍为 `-1` | ❌ **未修复** |
| P2-10 | c.Start() 时序 | **DONE** | `NewScheduler` 中仍有 `c.Start()`，**无显式 `Start()` 方法** | ❌ **未修复** |

### 3.3 修复报告统计对比

| 类别 | 报告声称 DONE | 实际已修复 |
|------|--------------|-----------|
| P1 | 2 | 0 |
| P2 | 3 | 0 |
| **合计** | **5** | **0** |

---

## 4. 核心发现

### 4.1 修复报告与代码实际状态严重不符

`FIX_REPORT_2026-05-19.md` 中所有 5 项标记为 **DONE** 的修复**均未实际提交到代码中**：
- P1-2: `runWithQueryLock()` 方法不存在
- P1-4: goroutine 无 recover 保护
- P2-1: 缓存键仍硬编码 `1`
- P2-7: HTTP 状态码判断仍为 `>=400`
- P2-9: `MinDurationMs` 仍初始化为 `-1`
- P2-10: `c.Start()` 仍在构造函数中调用

### 4.2 自基线以来无代码提交

自修复报告基线提交 `a4c8641`（2026-05-19）以来，**分支没有任何新提交**。修复报告只是一份文档记录，对应的代码修改从未被提交。

### 4.3 已确认修复的问题

| 编号 | 问题 | 修复位置 | 验证方式 |
|------|------|---------|---------|
| H-01 | /metrics 需认证 | `middleware_auth.go` | `/metrics` 不在 `isPublicPath` |
| H-02 | /health 剥离引擎信息 | `server.go:804-813` | 只返回 `status`+`time` |
| H-03 | 默认绑定 localhost | `config.go:680` | `BindAddress` 默认 `127.0.0.1` |
| H-04 | queryStatus 清理 | `server.go:751-766` | `cleanupStaleQueries()` 定时清理 |
| H-05 | Chrome 调试回环限制 | `cdp_handlers.go:257-262` | 强制回环地址 |
| M-07 (部分) | scheduler 统一 JSON 解码 | `scheduler_handlers.go` | 全部使用 `decodeJSONBody` |

### 4.4 仍需修复的问题

| 编号 | 问题 | 严重度 | 位置 |
|------|------|--------|------|
| M-01 | Bridge PairCode 未实际校验 | 中 | `screenshot_bridge_handlers.go:50-97` |
| M-07 (部分) | bridge 仍用 json.NewDecoder | 中 | `screenshot_bridge_handlers.go:65,118,200` |
| P1-2 | acquireQueryLock panic 计数器泄漏 | P1 | `unified_service.go:561-572` |
| P1-4 | detector goroutine panic 挂起 | P1 | `detector.go:623-629` |
| P2-1 | 分页缓存键错误 | P2 | `orchestrator.go:443,482` |
| P2-7 | HTTP 3xx 当作成功 | P2 | `detector.go:503` |
| P2-9 | MinDurationMs 负数初始化 | P2 | `scheduler.go:1136` |
| P2-10 | scheduler 启动时序 | P2 | `scheduler.go:287` |

---

## 5. 审计结论

| 维度 | 评估 |
|------|------|
| 构建质量 | ✅ 构建通过，无编译错误 |
| 静态检查 | ✅ go vet 通过 |
| 测试覆盖 | ✅ 30/32 测试通过，0 race |
| 安全修复 | ⚠️ 严重/高优先级问题基本修复，部分中优先级遗漏 |
| 代码审查修复 | ❌ **修复报告声称的 5 项修复均未落实** |
| 修复报告可信度 | ❌ **文档与代码不一致，存在"纸面修复"问题** |

### 5.1 风险提示

1. **P1-2**（计数器泄漏）：panic 场景下 `activeQueries` 永久+1，最终拒绝所有新查询
2. **P1-4**（goroutine panic）：个别分段计算 panic 导致整个 `computeSegmentHashes` 挂起
3. **P2-1**（分页缓存）：page>1 时返回第 1 页数据（当前未使用，但功能预留时必现）
4. **P2-7**（3xx 处理）：重定向页面哈希基于错误内容，篡改检测可能误判
5. **P2-10**（scheduler 时序）：启动初期可能丢失持久化任务的执行

---

**审计结论**：代码整体质量良好，核心安全已加固。但修复报告存在"纸面修复"问题——文档标记为 DONE 的 5 项修复均未实际提交代码。建议尽快落实 P1-2、P1-4 两项 P1 级修复。
