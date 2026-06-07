---
name: scheduler-optimization-2026-06-02
description: 定时任务系统前端优化(P3/P4) + 测试脚本(P5) + Runner全量验证(P6) 完成
metadata:
  type: project
---

## 定时任务系统优化 — P3/P4/P5/P6 完成 (2026-06-02)

### 已完成
- **P3**: 任务类型分组(5组22类) + Cron预设(7个) + `<optgroup>` 下拉
- **P4**: 任务模板(7个高频) + 22种参数提示 + JSON校验按钮
- **P5**: `scripts/test_scheduler_runners.sh` + 22个 `scripts/test_payloads/st*.json`
- **P6**: 18个非浏览器Runner全部执行验证通过（14完全通过+2符合预期+2跳过=0失败）

### 修复的Bug
1. `tamper_check` 字段 `mode` → `detection_mode`（模板/默认模板/提示三处）
2. `cache_warmup` 提示 `urls` → `warmup_urls`（runner只读warmup_urls）
3. `alert_silence` 提示 `silence_minutes` → `duration_minutes`
4. 测试脚本 ST-18 TaskType `bridge_token_rotate` → `bridge_token`

### 涉及文件
- `internal/scheduler/scheduler.go` — 分组结构+DefaultTemplates修复
- `web/scheduler_handlers.go` — handler传递分组数据
- `web/templates/scheduler.html` — 前端全部增强+字段修复
- `internal/scheduler/grouping_test.go` — 分组单测
- `scripts/test_scheduler_runners.sh` — 测试脚本
- `scripts/test_payloads/` — 22个JSON

### P7 飞书验证完成 (2026-06-02 17:39)
- 通知渠道直连测试 API `POST /api/notifications/channels/test` ✅
- 真实任务（url_reachability）+ 飞书推送 ✅
- 调试日志确认完整链路：全局开关→任务级开关→channelIDs→发送无错误
- 前置操作：`configs/config.yaml` 中 `notifications.enabled: true` + 重启服务

### P8 通知内容增强完成 (2026-06-02 18:00)
- 5 个 Runner 结果改为逐条详情格式（URLReachability/TamperCheck/BatchScreenshot/Query/ICPQuery）
- `TaskNotification` 新增 `Payload` 字段传递原始参数
- 飞书卡片：payload 上下文 + 详情 + 颜色状态头（蓝/红/橙）
- 测试适配：executor_icp_test.go 更新断言

### 未完成 → 进行中

- **P1**: Chrome MCP DOM采集测试 ✅ 4/5引擎通过（FOFA/Hunter/ZoomEye/Shodan ✅，Quake ❌ 反爬检测），CDP 4轮30测试完成
- **P2**: Extension采集脚本更新 🟡 capture.js FOFA/Hunter/ZoomEye/Shodan选择器已修复；3 bug修复（waitForPageReady SPA超时、extractCellText作用域、Shodan选择器过时）；CANARY已清理；🟡 诊断日志已加（`web/screenshot_bridge_handlers.go` `[bridge-collect]` tag）；✅ **Go端 `parseStructuredCollectedData` 4项修复**（port/status_code string→int、banner→BodySnippet、known map）+ 5新测试；待重启 Chrome+服务器后测试定位 Shodan 0 assets 根因
- **P9**: 截图飞书推送 ✅ 已完成（bot_channels.go + scheduler.go + 18单测 + UTF-8乱码三层防御 + 路径泄露修复）

### P2 待定位问题
- **Shodan 采集返回 0 assets**：CDP/Puppeteer 直测证明提取逻辑正确（10 items），但 Extension→Bridge→Server 链路返回空。已添加 `[bridge-collect]` 诊断日志在 PushResult 入口，下次重启 Chrome+Server 后运行 collect 测试即可看到 items count/前3条数据/诊断字段。
- **关键注意事项**：Chrome MV3 service worker 不会热重载，修改 capture.js/background.js 后必须完全退出 Chrome 再重启。

**Why:** 计划详见 `docs/SCHEDULER_OPTIMIZATION_PLAN.md`，实施记录详见 `docs/SCHEDULER_OPTIMIZATION_P3P4_REPORT.md`
**How to apply:** 后续继续推进P1/P2/P7时参考此记录了解已完成的工作和发现的问题
