# UniMap 安全审计发现登记（qa-security-audit 2026-08-05）

> **审计日期：** 2026-08-05
> **审计工具：** qa-security-audit v1.18.4（17 个 stdlib 扫描器 + AI 人工复核管线，默认安全模式：无执行/无网络）
> **审计产物：** `D:/Project/agent_project/qa-security-audit-docs/audits/unimap-20260805-suppressed/`（`audit_report.json`、`ai_manual_review_runner_result.json` 等）
> **复核基线：** `develop`（本文件复核时工作树无相关代码改动，2026-08-06 核对）
> **当前状态：** **2026-08-06 全部修复**（本文件原仅登记，修复见各 FINDING 的「修复记录」）

本文件登记 **2026-08-05 审计确认的 3 个真实漏洞**，外加 **1 个同模式补充发现**。所有行号均以 2026-08-06 复核时的实际代码为准。

## 摘要

| 编号 | 严重级 | 类别 | 位置 | 状态 |
|---|---|---|---|---|
| FINDING-001 | **P1** | 路径穿越（未认证远程文件系统枚举） | `internal/scheduler/executor_runners2.go:194` | ✅ FIXED |
| FINDING-002 | P2 | 竞态 / TOCTOU（WaitGroup Add-Wait） | `internal/scheduler/scheduler_notify.go:113-114` | ✅ FIXED |
| FINDING-003 | P2 | 数据竞争（截图采集 goroutine） | `internal/screenshot/manager_collect.go:147` | ✅ FIXED |
| FINDING-004 | **P1** | 路径穿越（同模式第二处，审计外补充） | `internal/scheduler/executor_runners2.go:844` | ✅ FIXED |

> **修复方式：** FINDING-001/004 抽公共 `sanitizeImportPattern` 校验（拒绝空值/绝对路径/根相对/`..` 段），并对
> `url_import`/`icp_import` 收紧 admin 门槛；FINDING-002 将 `stopping` 检查与 `notifyWg.Add(1)` 移入同一锁临界区；
> FINDING-003 改为 channel 传值（happens-before）。详见各 FINDING「修复记录」。全量 `go test -race ./...`、`go vet`、`go build` 通过。

> **为何 3 个确认问题仍有 1 个真实风险（P1）：** FINDING-001/004 的入口 `POST /api/v1/scheduler/tasks/create` 无强制认证
> （`internal/auth/middleware.go:58` `OptionalAPIKey` 无 key 也放行），且 `file_pattern` 零校验。FINDING-002/003 触发需并发关闭
> 或超时窗口，概率低但属确定性缺陷，`go build -race` 可复现。

---

## FINDING-001 (P1) 路径穿越 — 未认证远程文件系统枚举

**位置：** `internal/scheduler/executor_runners2.go:194`（URL 导入 runner，`TaskURLImport`）

**根因：** `file_pattern` 直接取自任务 payload，与 `importDir` `filepath.Join` 后交给 `filepath.Glob`；`..` 可逃逸 `importDir` 向上越界。

```go
// executor_runners2.go:191-194
filePattern := extractString(payload, "file_pattern", "*.txt")   // payload 可控
...
matches, err := filepath.Glob(filepath.Join(r.importDir, filePattern))
```

匹配到的文件随后被 `os.Open` 读取（`readURLsFromFile`，`executor_runners2.go:238`），返回文件名与行数。

**证据链（审计确认，closed）：**

1. 入口 `web/router.go:149` → `POST /api/v1/scheduler/tasks/create` → `web/scheduler_handlers.go:214` `handleCreateTask`：`requireBackupTaskAdmin` 仅对 `TaskBackup` 设 admin 门槛，`url_import` 直接放行（true）；`validateTaskPayload` 对 `TaskURLImport` 走 `default: return nil`，**`file_pattern` 零校验**。
2. 认证非强制：`internal/auth/middleware.go:58` `OptionalAPIKey` 无 key 也放行。
3. sink 链 `filepath.Glob` + `os.Open` 支持 `..` 逃逸，全链无守卫。

**PoC：**

```
无认证 POST /api/v1/scheduler/tasks/create
body: {"name":"enum","type":"url_import","enabled":true,"schedule_type":"delay","delay_seconds":0,
       "payload":{"file_pattern":"../../../../../../etc/*"}}
任务运行后，Result 列出 importDir 之外匹配的文件名与行数 → 文件系统枚举 / 存在性 oracle。
```

**实际影响：** 未认证远程文件系统枚举（文件名 + 行数 oracle），**非任意文件内容读取**（任务结果只回传匹配文件名与行数，不读内容）。构成侦察面扩展，可辅助后续攻击。

**修复记录（2026-08-06）：** 新增 `sanitizeImportPattern`（`executor_runners2.go`）并接入 URL/ICP 两个导入 runner：拒绝空值、
绝对路径（`filepath.IsAbs`）、以 `/` 或 `\` 开头的根相对路径（Windows 上 `IsAbs` 不识别）、以及含 `..` 段的路径
（`filepath.Clean` 后逐段检查）。两处 `filepath.Glob(filepath.Join(importDir, cleaned))` 不再可逃逸；非法 `file_pattern` 令任务
失败而非枚举。另对 `url_import`/`icp_import` 收紧 admin 门槛（`web/scheduler_handlers.go` `requireBackupTaskAdmin`）。新增
`TestSanitizeImportPattern`、`TestURLImportRunnerRejectsEscapingPattern`、`TestICPImportRunnerRejectsEscapingPattern` 回归。

---

## FINDING-002 (P2) 竞态 / TOCTOU — WaitGroup Add 与 Wait 并发

**位置：** `internal/scheduler/scheduler_notify.go:113-114`（`sendInlineWebhookNotification`）；同模式第二处 `:159`（`sendRegistryChannelNotification`）

**根因：** `stopping` 检查与 `notifyWg.Add(1)` 之间跨锁间隙；`Stop()` 并发执行 `notifyWg.Wait()` 时构成 Go 明令禁止的 Add-concurrent-with-Wait，可触发运行时 panic。

```go
// scheduler_notify.go:107-114 —— check 与 act 非原子
s.mu.RLock()
stopping := s.stopped || s.stopping
s.mu.RUnlock()
if stopping {
    return
}
s.notifyWg.Add(1)   // 此处与下方 Stop 的 Wait() 存在交错窗口
```

```go
// scheduler.go:1096 —— Stop 路径
s.notifyWg.Wait()
```

**证据链（审计确认，closed）：**

1. 通知 goroutine 通过 `stopping` 检查（读到 false）后，`Stop()` 翻转 `stopping` 并进入 `Wait()`。
2. 随后通知方 `Add(1)` 与 `Wait()` 并发执行 → Go 运行时 panic `sync: WaitGroup misuse: Add called concurrently with Wait`。
3. 第二处 `sendRegistryChannelNotification`（`:152-159`）同模式。

**实际影响：** 进程关闭与通知发送重叠时可能 panic；触发需并发关闭时序，概率低，但属确定性缺陷，`-race` 可复现。

**修复记录（2026-08-06）：** 两处通知函数（`sendInlineWebhookNotification` / `sendRegistryChannelNotification`）改为在同一
`mu.Lock()` 临界区内完成 `stopping` 检查与 `notifyWg.Add(1)`，再 `Unlock` 后启动 goroutine。`Stop()` 以同一把锁设置 `stopping=true`
后 `Wait()`，与 Add 互斥，不再存在 Add-concurrent-with-Wait 窗口。`-race` 全量通过。

---

## FINDING-003 (P2) 数据竞争 — 截图采集 goroutine 与调用方无同步读

**位置：** `internal/screenshot/manager_collect.go:147`（`EventLoadingFinished` 处理中启动的 goroutine）

**根因：** goroutine 在 `mu.Unlock()` 之后、**锁外**写入 `*result = collection.CollectResult{...}`；而调用方在超时分支**无同步地**读取 `l1Result`，两处之间没有任何 happens-before 边。

```go
// manager_collect.go:147 —— goroutine 内（锁外）写 *result
if needFetch {
    go func() {
        ...
        *result = collection.CollectResult{ ... }   // 锁外写入
```

```go
// manager_collect.go:283-288 —— 调用方超时分支无同步读
select {
case <-l1Ch:
case <-time.After(l1Wait):     // 超时走这里
case <-ctx.Done():
}
if l1Result != nil && len(l1Result.Assets) > 0 {   // L288 读取，与 goroutine 写无 happens-before
```

**证据链（审计确认，closed）：**

1. `collectViaNetworkOnContext` 返回 `result`（L126/L188），调用方 `CollectAndCaptureSearchEngineResult` 持有 `l1Result`（L219）。
2. 调用方 select 可能走 `case <-time.After(l1Wait)` 超时分支（L285）后于 L288 读取 `l1Result.Assets`。
3. goroutine 在超时后（截图阶段数秒窗口，`browserCtx` 尚未 cancel）才完成写 → 读（L288）与写之间无同步；调用方从未加 `mu`，超时路径也不收 `respCh`（channel 接收仅在正常路径）。
4. 真实 Go data race，`go build -race` 可复现。

**实际影响：** 数据竞争（UB），可能读到部分初始化的 `CollectResult`（空/半填 Assets），表现为偶发丢失采集结果；不构成内存破坏（无指针误用证明）。

**修复记录（2026-08-06）：** `collectViaNetworkOnContext` 签名改为仅返回 `chan collection.CollectResult`，**不再返回共享 `*result` 指针**；
goroutine 在本地构造 `collection.CollectResult` 值后向带缓冲通道非阻塞 send（channel 提供 happens-before）。调用方 `l1Result` 单独声明为
nil，**仅 `case res := <-l1Ch` 分支赋值**，超时/`ctx.Done` 分支保持 nil，不会读取 goroutine 仍在写入的数据。函数与调用方之间不存在
任何锁外共享读。全量 `go test -race ./...`、`go vet ./...`、`go build ./...` 通过。并发回归测试受限于 chromedp 依赖暂未新增，修复为静态保证。

---

## FINDING-004 (P1) 路径穿越 — 同模式第二处（本次补充发现，未在原审计队列）

**位置：** `internal/scheduler/executor_runners2.go:844`（ICP 关键词导入 runner，`TaskICPImport`）

**说明：** 与 FINDING-001 **同款漏洞、独立代码**，2026-08-05 审计队列未覆盖到该 runner，本次复核时发现：

```go
// executor_runners2.go:844-852
filePattern := extractString(payload, "file_pattern", "*.csv")
...
matches, err := filepath.Glob(filepath.Join(r.importDir, filePattern))
```

`file_pattern` 同样来自 payload、零校验、可 `..` 逃逸。全库 grep 确认除这两处外无其他 `file_pattern` sink；入口认证状态与 FINDING-001 相同。

**PoC 与影响：** 同 FINDING-001（未认证枚举），修复时应与 FINDING-001 一并处理（抽公共 sanitizeFilePattern 或统一校验）。

**修复记录（2026-08-06）：** 与 FINDING-001 同批修复——同一 `sanitizeImportPattern` 校验接入 `ICPImportRunner`（`executor_runners2.go`），
`icp_import` 同样纳入 admin 门槛；`TestICPImportRunnerRejectsEscapingPattern` 回归。

---

## 修复验证（2026-08-06）

1. FINDING-001/004：`sanitizeImportPattern` 抽公共校验 + `url_import`/`icp_import` admin 门槛，新增 3 个回归测试。
2. FINDING-002：`stopping` 检查与 `notifyWg.Add(1)` 同锁原子化（两处通知函数）。
3. FINDING-003：`collectViaNetworkOnContext` 改 channel 传值，消除锁外共享读。
4. 验证命令全通过：`go build ./...`、`go vet ./...`、`go test -race ./internal/scheduler/... ./internal/screenshot/... ./web/...`、`go test -race ./...`（全量）。
