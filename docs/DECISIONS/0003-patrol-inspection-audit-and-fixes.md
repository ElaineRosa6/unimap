# 0003 巡检功能审计与逻辑缺陷修复

## 状态

已接受

## 背景

2026-06-30 对巡检（"巡检" = 篡改检测 + URL 可达性 + 批量截图 + 端口扫描 + ICP 巡检等定时监控任务）子系统做全量审计。三个 Explore agent 通读了 scheduler runners、tamper detector、monitor service、web handlers、templates 与存储/告警链路，识别出 **4 个确认 Bug** 与 **4 个完善性缺口**，并梳理出 7 个架构级缺口（列为后续，不在本次范围）。

确认 Bug 的根因集中在两处：

1. **检测模式语义割裂**：detector 层定义并实现了 5 种检测模式（relaxed/strict/security/balanced/precise，各有独立阈值），但 service 层用 `if mode != strict { mode = relaxed }` 把后三种静默降级为 relaxed，使 UI 与定时任务的模式选择形同虚设。每日篡改模板更进一步用了不存在的 `"full"` 模式。
2. **定时巡检与交互式巡检能力不对等**：交互式 `/api/v1/tamper/check` 经 `tamperAllocatorFactory` 注入浏览器 allocator，可渲染 JS/SPA 页面；但定时 `TamperCheckRunner` 注册时传 `nil`，只能走 HTTP/Fast 模式，对 SPA 目标拿到空 hash → 误报"篡改"或"不可达"。

## 决策

### Tier 1 — Bug 修复

1. **导出模式归一化函数**：`internal/tamper/detector_types.go` 的 `normalizeDetectionMode` 导出为 `NormalizeDetectionMode`，service 层改用它，5 种模式全部透传，`resp.Mode` 返回真实归一化结果。
2. **修正每日篡改模板**：`tmpl_daily_tamper_check` 的 `DetectMode` 由无效的 `"full"` 改为 `"relaxed"`。
3. **定时篡改巡检注入 allocator**：`web/server.go` 的 `initScheduler` 在 `screenshotMgr != nil` 时从其构造无代理 `TamperAllocatorFactory` 注入 `TamperCheckRunner`；nil 时保持原行为向后兼容。
4. **可达性汇总补全 blocked 桶**：`URLReachabilitySummary` 新增 `Blocked` 字段，汇总 switch 增加 `case "blocked"`，使 `Total == Reachable + Unreachable + InvalidFormat + Blocked`。

### Tier 2 — 完善性修复

5. **基线刷新并发化**：`BaselineRefreshRunner` 读取 payload 的 `concurrency`，用信号量 + WaitGroup + Mutex 并发执行 `SetBaseline`（原为逐 URL 串行）。
6. **"查看基线" UI 入口**：`monitor.html` 新增 `btn-list-baseline` 按钮，绑定既有的 `loadBaselineList()`。
7. **历史记录类型过滤补全**：`monitor.html` 的 `history-type-filter` 追加 `no_baseline/unreachable/suspicious` 三个选项。
8. **scheduler 帮助文档模式修正**：`scheduler.html` 的 `PARAM_HINTS.tamper_check` 由错误的 `malicious/performance/full` 改为 `security/balanced/precise`。

## 影响

### 正面影响
- 用户在 UI / 定时任务中选择的 `安全/平衡/精确` 模式现在真正生效，而非被静默降级为宽松。
- 定时篡改巡检能正确渲染 JS 重页面，消除 SPA 目标的误报。
- 可达性统计在 SSRF 拦截发生时不再出现"总数对不上"的误导。
- 基线刷新大批量场景显著提速。

### 负面影响
- 定时篡改巡检注入浏览器后，单次巡检的资源占用（Chrome 进程）上升；可通过 payload 的 `concurrency` 与 `timeout` 控制。
- `URLReachabilitySummary` 新增 `blocked` JSON 字段，属向前兼容的 additive 变更。

## 实施步骤

1. ✅ 导出 `NormalizeDetectionMode` + 改内部调用点 + 更新 `detector_test.go` 用例
2. ✅ service 层 `tamper_app_service.go` 改用 `NormalizeDetectionMode`
3. ✅ `scheduler_types.go` 模板模式 `full` → `relaxed`
4. ✅ `web/server.go` 注入 `screenshotMgr` allocator
5. ✅ `monitor_app_service.go` 抽取 `summarizeReachability` + 新增 `Blocked`
6. ✅ `executor_runners2.go` 基线刷新并发化
7. ✅ `monitor.html` 加查看基线按钮 + 历史过滤补全
8. ✅ `scheduler.html` 帮助文档模式修正
9. ✅ 新增 `internal/service/monitor_app_service_test.go` 汇总不变量单测

## 相关文件

- `internal/tamper/detector_types.go` — 导出 `NormalizeDetectionMode`
- `internal/tamper/detector_test.go` — 模式用例补全
- `internal/service/tamper_app_service.go` — 模式透传
- `internal/service/monitor_app_service.go` — `summarizeReachability` + `Blocked`
- `internal/service/monitor_app_service_test.go` — 汇总单测（新增）
- `internal/scheduler/scheduler_types.go` — 模板模式修正
- `internal/scheduler/executor_runners2.go` — 基线刷新并发化
- `web/server.go` — 定时篡改 allocator 注入
- `web/templates/monitor.html` — 查看基线按钮 + 历史过滤补全
- `web/templates/scheduler.html` — 帮助文档模式修正

## 验证

- `go build ./...` 通过
- `go test ./internal/tamper/... ./internal/service/... ./internal/scheduler/...` 全部通过
- `go vet` + `gofmt` 干净
- GitHub Actions CI（commit `2bf5fe5`，PR #11）：ci + bridge-smoke 两套工作流全部 success，含 ubuntu/macos 双平台 build/lint/test/security scan

## 列为后续的架构级缺口（本次未修）

1. 可达性/端口扫描未产生告警记录（`AlertTypeReachability` 定义但无人触发，需阈值/去重设计）
2. 告警与资源监控仅存内存，重启丢失
3. `operation_history` 表无内部写入者（仅客户端 POST）
4. 定时批量截图结果未进 `batchdb`
5. 篡改导出器无生产调用方（死代码）
6. 无定时备份任务类型
7. 篡改记录文件式无索引（量大时查询慢）

这些涉及阈值设计与架构决策，建议单独规划。
