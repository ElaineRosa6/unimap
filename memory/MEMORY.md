# Project Memory Index

## 项目知识（从 C 盘记忆合并，2026-06-15）

- [浏览器采集架构知识](project_browser_collection_knowledge.md) — 4 引擎采集验证状态 + DOM 选择器 + 技术细节（Chrome MV3/service worker/port 提取等）

## 核心架构决策

- **Extension 模式是截图/采集的主力模式**：CDP headless 指纹暴露，Extension 使用真实浏览器会话
- **ScreenshotRouter 双模式自动降级**：CDP↔Extension 自动切换
- **三层采集架构（L1/L2/L3）**：L1 Network + L3 DOM 覆盖 5 引擎；L2 Hook 设计冻结
- **SPA 引擎截图必须用 `"spa"` 策略**：搜索结果页异步渲染，需 ~8s 等待（5s 初始 + 3s 完成后），`"load"` 策略仅 ~3s 不够
- **`collect_and_capture` 一次导航完成采集+截图**：避免分步调用导致页面重载丢失搜索结果
- **CSP `unsafe-inline` 完全移除**：21 静态 style→CSS 类，28 JS inline style→CSS 类，动态颜色用 CSS 变量
- **所有管理端点应启用限流**：即使有 admin auth 保护
- **操作历史持久化**：通用 SQLite 表，支持多类型操作
- **Web API 统一包 model.APIResponse**
- **handleScreenshot 统一走 Router**：不再直接创建 chromedp allocator
- **server.go 拆分完成**：1335→1128 行（middleware_security.go + server_helpers.go）

## 关键技术约束

- **Chrome MV3 service worker 不热重载**：修改 capture.js/background.js 后必须完全退出 Chrome 再重启
- **Chrome extension 热更新需要 Remove+re-add**
- **Go JSON string→int 静默失败**：JS 端数值字段必须用 `parseInt()` 转换
- **API 路径前缀是 `/api/v1/`**
- **Hunter 429 不重试是设计决策**
- **map→struct 迁移必须 grep JSON tag 冲突**

## 剩余长期项

1. **L2 Hook** — 设计冻结，仅当 L1/L3 telemetry 证明收益时启动
2. 227 处 `map[string]interface{}` 剩余（Web 响应和测试文件）
3. 新增引擎（Censys/DayDayMap/Onyphe/GreyNoise）端到端链路未闭环

## Claude Code 记忆（2026-06-15 合并）

> 从 `.claude/projects/D--Project-Go-project-unimap/memory/` 复制 26 个文件

### 最近工作（2026-06-12 ~ 2026-06-11）

- [查询页修复 2026-06-12](claude-code-memory/project_query_fixes_2026-06-12.md) — ✅ loading/超时/引擎过滤/域名修正/登录状态/页面重复打开；⏸ DOM选择器失效+API Key未配置
- [安全与稳定性修复 2026-06-11](claude-code-memory/project_repair_pause_state_2026-06-11.md) — ✅ 四轮19项修复：CRITICAL数据竞争+P0 XSS×7+P1 panic/幂等/同步+P2 logger/dockerignore
- [当前未完成问题 2026-06-11](claude-code-memory/project_open_issues_2026-06-11.md) — ✅ P0 Bridge状态语义统一+状态抖动+Token复制；⏸ 新增引擎端到端+UQL历史+长期项

### 最近工作（2026-06-15）

- [Quake采集验证更新 2026-06-15](claude-code-memory/project_quake_collection_complete_2026-06-15.md) — ✅ Quake采集成功（10 items, card_fallback方法）；✅ 5引擎全部打通；✅ 根因澄清：URL格式过期非反爬拦截
- [ZoomEye选择器更新 2026-06-15](claude-code-memory/project_zoomeye_selector_update_2026-06-15.md) — ✅ ZoomEye DOM选择器更新（capture.js + dom_selectors.go）；✅ WebOnly采集验证成功（10条数据）；✅ web包测试覆盖率42.6%→54.6%（+12%）
- [截图SPA渲染时机修复 2026-06-15](claude-code-memory/project_screenshot_spa_timing_fix_2026-06-15.md) — ✅ Extension screenshot加4000ms SPA等待；✅ Go Bridge调用改用"spa"策略；✅ 新增collect_and_capture action一次导航完成采集+截图；✅ Go CollectAndCaptureSearchEngineResult重构

### 审查与修复（2026-06-10 ~ 2026-06-09）

- [全量审查修复完成 2026-06-10](claude-code-memory/project_audit_fix_complete_2026-06-10.md) — P0×4+P1×14+P2×17=35项全部闭环
- [全量项目审查 2026-06-09](claude-code-memory/project_full_audit_2026-06-09.md) — 6用户旅程维度，4P0+14P1+17P2
- [代码质量治理 2026-06-09](claude-code-memory/project_code_quality_refactor_2026-06-09.md) — 文件拆分+函数拆分+API shim移除

### 引擎适配（2026-06-07 ~ 2026-06-08）

- [全5引擎采集验证 2026-06-07](claude-code-memory/project_5engine_collection_verification_2026-06-07.md) — 5/5引擎Extension采集打通
- [适配器语法全面修正 2026-06-07](claude-code-memory/project_adapter_syntax_fix_2026-06-07.md) — 5引擎14项语法修正+12条新测试
- [引擎适配器下一步计划 2026-06-07](claude-code-memory/project_engine_adapter_next_steps_2026-06-07.md) — 阶段一核查+阶段二全部完成
- [阶段二引擎+语法核查 2026-06-08](claude-code-memory/project_engine_adapter_phase2_2026-06-08.md) — 5引擎语法全量核查+10个适配器完成
- [Shodan调试 2026-06-07](claude-code-memory/project_develop_commits_shodan_debug_2026-06-07.md) — Shodan 0 assets根因修复
- [SEC-1 token轮换 2026-06-08](claude-code-memory/project_sec1_token_rotation_2026-06-08.md) — token轮换+docs打码+gitignore

### 架构与采集（2026-06-05 ~ 2026-06-09）

- [L1 Network采集集成 2026-06-09](claude-code-memory/project_l1_network_integration_2026-06-09.md) — combined collect+capture路径集成L1
- [ARC-4抓包spike 2026-06-09](claude-code-memory/project_arc4_packet_capture_2026-06-09.md) — 5引擎全量验证：Hunter/ZoomEye/Quake为SPA→L1有价值
- [Extension反爬虫架构 2026-06-05](claude-code-memory/project_extension_anti_scraping_2026-06-05.md) — CDP vs Extension对比、stealth 5阶段实施方案
- [三层采集架构差距 2026-06-06](claude-code-memory/project_three_layer_collection_gap_2026-06-06.md) — 🔴 设计阶段；Phase -1需真机抓包
- [浏览器查询降级 2026-05-29](claude-code-memory/project_browser_fallback_2026-05-29.md) — 阶段1-5全部闭环

### 截图与认证（2026-06-03）

- [飞书截图+乱码+超时 2026-06-03](claude-code-memory/project_feishu_screenshot_2026-06-03.md) — 乱码/截图超时/Bridge认证/路径泄露闭环
- [Bridge认证改造 2026-06-03](claude-code-memory/project_screenshot_bridge_auth_2026-06-03.md) — Admin Token fallback+pairing/signature联动
- [P1/P2 + Extension重配对 2026-06-03](claude-code-memory/project_p1_p2_dom_selectors_2026-06-03.md) — 5/5引擎Extension采集打通

### 早期工作（2026-05）

- [前端全量审计修复 2026-05-28](claude-code-memory/project_frontend_audit_fix_2026-05-28.md) — 12项交互问题闭环
- [API版本化闭环 2026-05-31](claude-code-memory/project_api_versioning_2026-05-31.md) — /api/v1双注册+55文件迁移
- [问题修复报告 2026-06-01](claude-code-memory/project_issue_fix_2026-06-01.md) — 4类问题+Hunter限流根因修复
- [ICP GBK编码修复](claude-code-memory/project_icp_gbk_encoding_fix.md) — Windows GBK编码问题修复
- [已知问题清单 2026-05-09](claude-code-memory/project_remaining_issues_2026-05-09.md) — 原始问题清单
- [Quake反爬非权限问题 2026-06-07](claude-code-memory/project_quake_antiscraping_not_permission_2026-06-07.md) — Quake采集失败是反爬拦截

### 2026-06-16 Hunter DOM提取修复 + 采集闭环

- ✅ **collect_and_capture 语法错误修复**：`else if` 在 `else` 之后 → 重构 if/else 链
- ✅ **collect_and_capture 变量错误修复**：`result` → `collectResult`（Critical）
- ✅ **HasMore 字段添加**：`BridgeCollectedData` 新增 `HasMore bool`
- ✅ **Parser 一致性**：`ParseStructuredCollectedDataFromItems` 接受 `hasMore` 参数，使用 `extractPortFromHost` helper
- ✅ **Login wall 处理**：JS + Go 双侧添加 `collect_and_capture` login wall 检测
- ✅ **CDP 选择器更新**：FOFA/Hunter/Quake 从旧 table 布局更新为新 SPA 卡片布局
- ✅ **Extension 版本迭代**：0.3.2 → 0.3.8（5 次 Chrome 重载测试）
- ✅ **Hunter 提取优化**：去重（ip:port）、protocol 只保留协议名、raw cell text 替代 tooltip
- ⏸ **Hunter country/title/host 清理**：Go 端 `CleanHunterFields` 已实现但未生效，需排查 `browserCollectedData` 路径
- ✅ **collection/parser 测试**：新增 7 个测试函数、55+ 子用例，覆盖全部 8 个导出函数
- ✅ **FOFA 采集验证**：10 条资产，字段完整，截图正常

### 已知问题（2026-06-16 全部核实）

**Critical（1 项）**
1. **截图未登录状态** — Extension 通过 `chrome.tabs.create` 打开新标签页，未继承用户已登录的 session cookies。FOFA 截图显示"登录"按钮，Hunter 显示"未登录仅展示部分数据"。需排查 tab 创建方式和 cookie 共享机制。

**High（4 项）**
2. **Hunter country_code 为城市名** — 浏览器采集返回 "成都市" 而非 "中国"。Go 端 `CleanHunterFields` 已实现但未在 `browserCollectedData` 路径触发。
3. **Hunter title 含分类标签** — "Dovecot imapd企业办公 邮件系统 开源 Dovecot imapd"，应截断到 "Dovecot imapd"。
4. **Hunter host 含 UI 噪声** — "不看空域名 -" 出现在 host 字段。
5. **CleanHunterFields 调用链断裂** — 函数在 `router_extension.go:315,349` 被调用，但 `browserCollectedData` 路径可能绕过了 `populateCollectResultFromBridge`。

**Medium（5 项）**
6. **653 处 `map[string]interface{}`** — 含测试文件（~293）、Web 响应（~80）、collection/parser（~80）等。核心逻辑已迁移，剩余为系统边界。
7. **新引擎端到端未闭环** — Censys/DayDayMap/Onyphe/GreyNoise 适配器代码存在（有 `Search` 方法），但 Extension DOM 选择器缺失、UI 未暴露这些引擎。
8. **L2 Hook 设计冻结** — 仅当 L1/L3 telemetry 证明收益时启动。
9. **web/ flaky test** — `TestClassifyBatchURLsPreservesOriginalIndices` 并行运行端口冲突失败，单独运行通过。
10. **定时任务缺少简易定时功能** — scheduler 只支持 cron 循环表达式，无一次性延迟执行、简易间隔设置、指定时间点执行。

**Low（3 项）**
11. **countGoroutines() 空桩** — `router_test.go:400` 硬编码 `return 0`，`TestRouterStartStop_NoGoroutineLeak` 无效。
12. **ZoomEye 哈希 CSS class** — `span._public-hover_uxlu6_1` 是编译哈希，前端改版即失效。
13. **extractPortFromHost IPv6 边界** — `strings.LastIndex(s, ":")` 对 `2001:db8::1` 会错误解析为 port=1。

## 当前活跃

- [Extension 模式问题 2026-05-09](project_extension_mode_issues_2026-05-09.md) — ✅P0已修复(Shodan补齐+翻译路径验证)、✅P1进度已实现、P1登录状态部分解决
- [Bridge 认证修复 2026-06-03](project_bridge_auth_fix_2026-06-03.md) — ✅截图超时根因(重启丢token→401)修复：admin token loopback 兜底+签名/pairing联动+5测试+真机curl E2E全绿
- [定时任务优化进度 2026-06-02](project_scheduler_optimization_2026-06-02.md) — P3-P6完成详情+修复记录+后续计划
- [Quake采集验证 2026-06-07](project_quake_antiscraping_not_permission_2026-06-07.md) — ✅URL格式修正后采集成功（10 items, card_fallback方法）

## 当前文档（docs/）

- [E2E采集验证 2026-06-04](../docs/E2E_COLLECTION_VERIFICATION_2026-06-04.md) — ✅ 截图✅ Shodan 6/6 + Quake 2/2；采集✅ 5引擎全部打通（2026-06-07验证）：FOFA/ZoomEye/Shodan/Hunter/Quake均成功；Quake URL格式修正后采集成功（10 items, card_fallback方法）
- [问题修复报告 2026-06-01](../docs/FIX_REPORT_2026-06-01.md) — Phase 1-4 全量修复 + Hunter 限流根因
- [项目审查报告 2026-06-01](../docs/PROJECT_REVIEW_2026-06-01.md) — 第二轮全量审查：7 维度深度分析，18 项新发现
- [定时任务下一步计划](../docs/SCHEDULER_NEXT_STEPS.md) — 🟡 P1 CDP 4/5通过(Quake反爬) + P2 capture.js已修复 + 飞书路径泄露已修复；下一步Extension E2E

## 已归档（docs/archive/）

| 目录 | 内容 |
|------|------|
| [plans/](../docs/archive/plans/) | 9 文件：ICP/通知/FOFA/浏览器降级策略/修复计划/定时任务优化计划 |
| [reviews/](../docs/archive/reviews/) | 10 文件：2026-05-19~06-01 审查链 + 定时任务P3P4报告 + 项目审查05-31 |
| [COMMIT_HISTORY_2026-05.md](../docs/archive/COMMIT_HISTORY_2026-05.md) | 2026-05 提交历史（git 已跟踪，冗余归档） |

## 已归档（memory/archive/）

| 文件 | 说明 |
|------|------|
| [全量代码审查修复 2026-05-27](archive/project_code_review_fix_report_2026-05-27.md) | 55文件+625/-3700行，20项修复 |
| [安全修复 2026-05-07](archive/project_security_fix_2026-05-07.md) | 第三轮 review 遗留 13 项修复 |
| [通知与 FOFA 接入 2026-05-23](archive/project_notify_fofa_2026-05-23.md) | Webhook/Log + FOFA adapter |
| [ICP 定时任务 2026-05-22](archive/project_icp_scheduled_task_2026-05-22.md) | ST-21 TaskICPQuery + ICPQueryRunner |
| [静态版本修复 2026-05-18](archive/project_static_version_fix_2026-05-18.md) | staticVersion 传递修复 |
| [测试覆盖率进度 2026-04-21](archive/project_test_coverage_phase1_2026-04-21.md) | Phase 1-3 进度，整体约 75% |
| [实施指南完成 2026-04-16](archive/project_implementation_guide_progress_2026-04-16.md) | 10步全部完成 |
| [跨平台适配 2026-04-13](archive/project_crossplatform_2026-04-13.md) | macOS/Linux 6 项适配 |

## 核心文档（docs/）

| 文档 | 说明 |
|------|------|
| [ARCHITECTURE.md](../docs/ARCHITECTURE.md) | 分层架构 + 数据流向 |
| [RUNBOOK.md](../docs/RUNBOOK.md) | 运维故障处理（6 场景） |
| [API.md](../docs/API.md) | API 文档 |
| [API_VERSIONING.md](../docs/API_VERSIONING.md) | API 版本化方案 |
| [PLUGIN_DEVELOPMENT_GUIDE.md](../docs/PLUGIN_DEVELOPMENT_GUIDE.md) | 插件开发指南 |
| [PRODUCTION_READINESS_PLAN.md](../docs/PRODUCTION_READINESS_PLAN.md) | 生产就绪清单 |
| [OPS_SCREENSHOT_EXTENSION.md](../docs/OPS_SCREENSHOT_EXTENSION.md) | 截图扩展运维 |
