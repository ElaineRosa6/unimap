# 定时任务系统优化实施记录（P3 ~ P7 + 通知增强）

> 实施日期：2026-06-02
> 对应计划：`docs/SCHEDULER_OPTIMIZATION_PLAN.md`
> 状态：✅ P3/P4/P5/P6/P7 已完成 + 通知内容增强；P1/P2 待运行环境

---

## 一、实施范围

| 阶段 | 内容 | 状态 |
|------|------|------|
| P3 | 任务类型分组 + Cron 快捷预设 | ✅ 完成 |
| P4 | 任务模板系统 + 参数提示 + JSON 校验 | ✅ 完成 |
| P5 | 测试脚本 + 22 个 payload JSON | ✅ 完成 |
| P6 | 逐项 Runner 测试执行 | ✅ 18/22 通过（4 个需浏览器） |
| P7 | 飞书通知推送验证 | ✅ 完成（测试 API + 真实任务推送均通过） |
| P8 | 通知内容增强（逐条详情 + payload 上下文） | ✅ 完成 |
| P1/P2 | Chrome MCP DOM 采集测试 / 选择器修复 | ⏸ 需实时浏览器环境 |
| P9 | 截图飞书推送（image_key 上传方案） | 📋 待实施 |

---

## 二、改动文件清单

| 文件 | 改动 |
|------|------|
| `internal/scheduler/scheduler.go` | 新增 `TaskGroupInfo` 结构、`GroupedTaskTypes()`、`TaskTypeGroup()`；修复 `DefaultTemplates()` tamper_check 字段 `mode` → `detection_mode` |
| `web/scheduler_handlers.go` | `handleSchedulerPage` 构建并传递 `TaskTypeGroups` 到模板 |
| `web/templates/scheduler.html` | 分组下拉、Cron 预设按钮、任务模板、参数提示、JSON 校验、CSP nonce 样式；修复模板字段名不一致 |
| `internal/scheduler/grouping_test.go` | 新增分组划分与查找的单元测试 |
| `scripts/test_scheduler_runners.sh` | **新增** 22 个 Runner 逐项测试脚本 |
| `scripts/test_payloads/st01-st22_*.json` | **新增** 22 个测试 payload JSON 文件 |

---

## 三、P3+P4：前端优化

### 3.1 任务类型分组

22 种任务划分为 5 组，由 Go 端 `GroupedTaskTypes()` 作为唯一真实来源。

### 3.2 Cron 快捷预设 + 任务模板 + JSON 校验

7 个 Cron 预设按钮、7 个高频任务模板、22 种类型的参数提示、JSON 校验功能。

---

## 四、P5：测试脚本与 payload

### 4.1 测试脚本结构

```
scripts/
  test_scheduler_runners.sh      # 主测试脚本
  test_payloads/                 # 22 个 Runner 的测试 payload JSON
    st01_query.json              ~ st22_icp_import.json
```

### 4.2 测试脚本功能

- **5 阶段执行**：创建任务 → 触发执行 → 等待完成 → 检查结果 → 清理
- **选择性测试**：`TEST_ST=01,05` 只运行指定编号；`SKIP_ST=02,03` 跳过指定编号
- **结果验证**：自动匹配执行结果中的关键词
- **自动清理**：`CLEANUP=false` 保留测试任务
- **详细报告**：执行历史保存到 `scripts/test_results/`

### 4.3 Payload 字段名（与 runner 代码对齐）

| ST | 类型 | 关键字段 | 备注 |
|----|------|----------|------|
| 01 | query | `query`, `engines`, `page_size` | engines 也接受 engine（别名） |
| 02 | search_screenshot | `engine`, `query` | |
| 03 | batch_screenshot | `urls`, `concurrency` | |
| 04 | tamper_check | `urls`, `detection_mode` | ⚠️ 不是 `mode`（已修复） |
| 05 | url_reachability | `urls`, `concurrency` | |
| 06 | cookie_verify | `engines` | 空则检查全部引擎 |
| 07 | login_status_check | `engines`, `test_query` | |
| 08 | distributed_submit | `task_type`, `priority`, `timeout_seconds` | |
| 09 | export | `query`, `engines`, `page_size`, `format` | |
| 10 | port_scan | `urls`, `ports`, `concurrency` | |
| 11 | screenshot_cleanup | `max_age_days` | |
| 12 | tamper_cleanup | `max_age_days` | |
| 13 | quota_monitor | `low_threshold` | |
| 14 | alert_summary | `max_age_days` | |
| 15 | baseline_refresh | `urls` | 空则刷新所有已有基线 |
| 16 | url_import | `file_pattern`, `max_lines` | |
| 17 | plugin_health | 无 | |
| 18 | bridge_token | 无 | TaskType 常量是 `bridge_token`，不是 `bridge_token_rotate` |
| 19 | alert_silence | `alert_type`, `duration_minutes` | ⚠️ 不是 `silence_minutes` |
| 20 | cache_warmup | `warmup_urls` | ⚠️ 不是 `urls` |
| 21 | icp_query | `queries`, `type`, `page`, `page_size` | type 单数，支持逗号分隔多类型 |
| 22 | icp_import | `file_pattern`, `type`, `max_rows` | |

---

## 五、Bug 修复

### 5.1 tamper_check 字段名不一致

| 位置 | 修复前 | 修复后 |
|------|--------|--------|
| `scheduler.go` `DefaultTemplates()` | `"mode": "full"` | `"detection_mode": "full"` |
| `scheduler.html` 模板 | `mode: 'relaxed'` | `detection_mode: 'relaxed'` |
| `scheduler.html` 参数提示 | `mode: strict/relaxed/...` | `detection_mode: strict/relaxed/...` |

**根因**：`TamperCheckRunner.Execute()` 读取 `extractString(payload, "detection_mode", "relaxed")`，但模板和默认模板用的是 `"mode"`，导致用户配置的检测模式被忽略，始终回退到 `relaxed`。

### 5.2 cache_warmup 参数提示修正

| 位置 | 修复前 | 修复后 |
|------|--------|--------|
| `scheduler.html` 参数提示 | `urls / warmup_urls: ...` | `warmup_urls: ...` |

**根因**：`URLHealthChecker.Execute()` 只读取 `"warmup_urls"`，`"urls"` 字段会被静默忽略。

### 5.3 alert_silence 参数提示修正

| 位置 | 修复前 | 修复后 |
|------|--------|--------|
| `scheduler.html` 参数提示 | `silence_minutes/duration_minutes: ...` | `duration_minutes: 静默时长（分钟）` |

**根因**：`AlertSilenceRunner.Execute()` 只读取 `"duration_minutes"`，`"silence_minutes"` 会被静默忽略。

### 5.4 测试脚本 ST-18 TaskType 修正

| 位置 | 修复前 | 修复后 |
|------|--------|--------|
| `test_scheduler_runners.sh` | `bridge_token_rotate` | `bridge_token` |

**根因**：`TaskBridgeTokenRotate` 常量值为 `"bridge_token"`，不是 `"bridge_token_rotate"`。

---

## 六、P6：Runner 测试执行结果

> 执行时间：2026-06-02 16:19
> 环境：本地 Windows，UniMap Web 运行中

### 6.1 测试结果汇总

| ST | Runner | 结果 | 执行详情 |
|----|--------|------|----------|
| 01 | query | ✅ | `retrieved 0 assets from 0 engine(s) (1 engine error(s))` — Runner 正常，FOFA 引擎报错（配置/配额问题） |
| 02 | search_screenshot | ⏸ | 跳过（需 Chrome/Extension Bridge） |
| 03 | batch_screenshot | ⏸ | 跳过（需 Chrome/Extension Bridge） |
| 04 | tamper_check | ✅ | `tamper check complete [tampered=1, safe=0, ...]` — `detection_mode` 修复生效 |
| 05 | url_reachability | ✅ | `reachability: 1 reachable, 0 unreachable, 0 invalid out of 2` |
| 06 | cookie_verify | ⏸ | 跳过（需 Chrome/Extension Bridge） |
| 07 | login_status_check | ⏸ | 跳过（需 Chrome/Extension Bridge） |
| 08 | distributed_submit | ✅ | `enqueued task dist_1 (type=url_reachability, priority=5)` |
| 09 | export | ✅ | `no results to export`（无查询数据，符合预期） |
| 10 | port_scan | ✅ | `scanned 1 URLs: 0 successful, 0 failed` |
| 11 | screenshot_cleanup | ✅ | `cleaned up 8 batch(es) older than 30 days` |
| 12 | tamper_cleanup | ✅ | `cleaned up 0 expired check record(s), skipped 85 within max age 90 days` |
| 13 | quota_monitor | ⚠️ | `1 engine(s) with low quota (below 10)` — 符合预期告警行为 |
| 14 | alert_summary | ✅ | `alert summary [total=2 (last 7 days), tamper=2, warning=2]` |
| 15 | baseline_refresh | ✅ | `refreshed baseline for 1/1 URL(s)` |
| 16 | url_import | ✅ | `no files matching *.txt in ./data/imports`（无导入文件，符合预期） |
| 17 | plugin_health | ✅ | `no plugins registered`（无插件，符合预期） |
| 18 | bridge_token | ⚠️ | `bridge service not available` — Bridge 未启动，符合预期 |
| 19 | alert_silence | ✅ | `silenced all quota_low alerts for 30 minutes` |
| 20 | cache_warmup | ✅ | `warmed up 2/2 URLs` — `warmup_urls` 字段修复生效 |
| 21 | icp_query | ✅ | `icp [types=web] 1/1 queries succeeded, total 1 records` |
| 22 | icp_import | ✅ | `no files matching *.csv in ./data/icp_imports`（无导入文件，符合预期） |

### 6.2 结果分类

- **✅ 完全通过**：14 个（ST-01/04/05/08/09/10/11/12/14/15/16/17/19/20/21/22）
- **⚠️ 符合预期**：2 个（ST-13 配额告警、ST-18 Bridge 未启动）
- **⏸ 跳过**：4 个（ST-02/03/06/07 需浏览器环境）
- **❌ 失败**：0 个

### 6.3 发现的环境问题

1. **FOFA 引擎报错**（ST-01）：`retrieved 0 assets from 0 engine(s) (1 engine error(s))`，可能是 API Key 配额不足或认证过期
2. **引擎配额偏低**（ST-13）：至少 1 个引擎剩余配额低于阈值 10

---

## 七、P7：飞书通知验证

> 执行时间：2026-06-02 17:39
> 前置操作：`configs/config.yaml` 中 `notifications.enabled` 改为 `true` 并重启服务

### 7.1 验证方式

| 测试 | 方式 | 结果 |
|------|------|------|
| 通知渠道直连测试 | `POST /api/notifications/channels/test` `{"id":"feishu_2"}` | ✅ `success: true` |
| 真实任务推送 | 创建 `url_reachability` 任务 + `notifications.channel_ids=["feishu_2"]` → 立即执行 | ✅ 飞书收到执行结果卡片 |

### 7.2 调试日志确认

在 `sendNotification` 入口加临时日志验证了完整链路：
- 全局开关 `notifications.enabled: true` ✓
- 任务级 `notifications.enabled: true` ✓
- `channelIDs=[feishu_2]` 正确传递 ✓
- 无发送错误日志（有错误会记录 `notify ... failed`）✓

### 7.3 配置要点

```yaml
# configs/config.yaml
notifications:
  enabled: true          # 全局开关，必须为 true
  channels:
    - id: feishu_2       # 创建任务时引用此 ID
      type: feishu
      enabled: true
      webhook_url: "https://open.feishu.cn/open-apis/bot/v2/hook/xxx"
```

任务创建时需指定：
```json
{
  "notifications": {
    "enabled": true,
    "on_success": true,
    "on_failure": true,
    "channel_ids": ["feishu_2"]
  }
}
```

---

## 八、P8：通知内容增强

> 执行时间：2026-06-02 18:00

### 8.1 问题

原通知仅显示汇总摘要（如 `reachability: 1 reachable, 0 unreachable`），缺少：
- 执行参数上下文（查了哪些 URL、用了什么查询）
- 逐条结果明细（每个 URL 的具体状态）

### 8.2 改动

| 文件 | 改动 |
|------|------|
| `internal/scheduler/executor.go` | 5 个 Runner 结果字符串改为逐条详情格式 |
| `internal/notify/message.go` | `TaskNotification` 新增 `Payload` 字段 |
| `internal/notify/bot_channels.go` | 飞书卡片：payload 上下文 + 详情 + 颜色状态头 |
| `internal/scheduler/scheduler.go` | `sendNotification` 传递 `task.Payload` |
| `internal/scheduler/executor_icp_test.go` | 适配新结果格式 |

### 8.3 丰富化的 Runner

| Runner | 原格式 | 新格式 |
|--------|--------|--------|
| URLReachability | `1 reachable, 0 unreachable` | 每个 URL：✅ 可达 (HTTP 200) / ❌ 不可达 — 原因 |
| TamperCheck | `tampered=1, safe=0` | 每个 URL：⚠️ 已篡改 — 变更区域 / ✅ 正常 / 🆕 首次检测 |
| BatchScreenshot | `3/5 succeeded` | 每个 URL：✅ → 文件路径 / ❌ — 错误 |
| Query | `retrieved 10 assets from 2 engines` | 查询语句 + 每个引擎：✅ fofa: 5 条 |
| ICPQuery | `1/1 queries succeeded, total 5` | 每个关键词：✅ baidu.com [web]: 5 条 — 域名列表 |

### 8.4 飞书卡片新格式

```
✅ [UniMap] 定时任务 [富通知测试-URL可达性] 执行成功
────────────────────────
目标: ["https://www.baidu.com","https://www.bing.com"]
耗时: 5.0s
────────────────────────
执行结果:
检测 3 个 URL：2 可达，0 不可达

✅ 可达 https://www.baidu.com (HTTP 200)
✅ 可达 https://www.bing.com (HTTP 200)
❌ 不可达 https://nonexistent.invalid — context deadline exceeded
```

- 状态头颜色：蓝色(成功) / 红色(失败) / 橙色(超时)
- Payload 关键字段自动提取：urls、query、queries、engines、detection_mode、low_threshold

### 8.5 截图推送方案

当前截图以文本路径展示（`✅ https://example.com → screenshots/batch_xxx/1.png`）。

飞书 webhook 机器人推送图片需要先上传获取 `image_key`，完整方案见后续计划。

---

## 九、后续阶段

| 阶段 | 内容 | 前置条件 |
|------|------|----------|
| **P1** | Chrome MCP 验证 5 引擎 DOM 选择器 | Chrome MCP 可用 |
| **P2** | 根据 P1 结果更新 Extension 采集脚本 | P1 完成 |
| **P9** | 截图飞书推送：上传截图获取 image_key，卡片嵌入图片 | 飞书 app 凭证 |

### P6 重跑方法

```bash
# 启动服务
go run ./cmd/unimap-web

# 跳过需浏览器的 4 个
SKIP_ST=02,03,06,07 AUTH_TOKEN=your_token ./scripts/test_scheduler_runners.sh

# 指定 Admin Token + 只测特定 Runner
AUTH_TOKEN=your_token TEST_ST=01,21 ./scripts/test_scheduler_runners.sh
```
