# Project Memory Index

## 工作进展

- [全量代码审查与修复计划 2026-05-27](project_code_review_fix_plan_2026-05-27.md) — 4阶段修复计划（6 Critical / 5 High / 10 Medium / 7 Low + 5死代码包），rev.2已剔除H-04/H-08误报、H-01/C-08降级、修正C-04描述和计数，待执行
- [跨平台适配 2026-04-13](project_crossplatform_2026-04-13.md) — macOS/Linux适配：自动查询检查、定时任务渲染、SIGHUP兼容性、Chrome路径、CI多平台
- [实施进度 2026-04-15](project_implementation_progress_2026-04-15.md) — 安全重构步骤1-3完成，P0清零
- [实施指导完成 2026-04-16](project_implementation_guide_progress_2026-04-16.md) — 10步全部完成：20个Runner、E2E测试、Runbook、Grafana面板
- [测试覆盖率计划 2026-04-20](project_test_coverage_plan_2026-04-20.md) — 从40.4%提升到80%，分3个Phase执行，已制定详细计划
- [测试覆盖率 Phase1 进度 2026-04-21](project_test_coverage_phase1_2026-04-21.md) — Phase 1完成：adapter 17.7%、screenshot 20.8%、service 22.5%，数据竞争修复，整体65.1%
- [深度审查核实 2026-04-29](project_deep_review_verification_2026-04-29.md) — 第三轮code review 25项问题核实：仅2项修复，18项未修复（含8项Critical/High）
- [安全修复执行 2026-05-07](project_security_fix_2026-05-07.md) — 修复第三轮 review 遗留的 13 项未修复问题（4 Critical、3 High、4 Medium、2 Low），详细记录见 project_security_fix_2026-05-07.md
- [ICP 定时任务接入 2026-05-22](project_icp_scheduled_task_2026-05-22.md) — 新增 ST-21 TaskICPQuery + ICPQueryRunner + ICPSearchWithContext + 2 内置模板，21 个测试通过，P1/P2 全部修订

### ✅ 全部项目完成状态

| 阶段 | 状态 | 说明 |
|------|------|------|
| P0 缺陷修复 | ✅ 完成 | Unicode错误、Worker池泄漏、Logger竞态 |
| P1 缺陷修复 | ✅ 完成 | 优雅关闭、Context取消、Clone错误、重试逻辑、测试补充、告警通道 |
| P2 技术债务 | ✅ 完成 | SSRF防护、文件权限、CI完善、Docker安全、MD5→SHA256等 |
| 架构增强 | ✅ 完成 | ScreenshotRouter双模式、CDP跨平台 |
| 定时任务系统 | ✅ 完成 | 20个Runner(ST-01~ST-20)、Web API、前端页面、E2E测试 |
| 运维文档 | ✅ 完成 | RUNBOOK.md(6场景)、Grafana面板(7面板) |
| 测试覆盖 | ✅ 完成 | 32包通过-race检测，后续持续补充 |
| 第二轮 Code Review | ✅ 完成 | 24项全部修复 |
| 第三轮 Code Review | ⚠️ 未完全修复 | 文档声称24项全部修复，实际核实18项仍未修复（含8项Critical/High） |

### ⚠️ 第三轮 Code Review 核查结果 (2026-04-29) — 18项未修复

- `docs/DEEP_CODE_REVIEW_2026-04-29.md` 声称"全部已修复"，但逐行代码核实发现 **仅2项真正修复，5项部分修复，18项未修复**
- 已修复: H-03(SSRF截图目标)、H-06(isTrustedRequest严格化)
- 部分修复: C-05(cookie标志)、M-01(时间戳修剪)、M-07(sanitizeError)、L-04(requireTrustedRequest)
- **未修复18项**，其中 **8项为 Critical/High**: C-01(auth默认关闭+bind 0.0.0.0)、C-02(text/template→XSS)、C-03(RoundRobinScheduler数据竞争)、C-04(RateLimiter数据竞争)、H-01(CSP unsafe-eval)、H-02(WebSocket无token放行)、H-04(告警goroutine无WaitGroup)、H-05(限流默认关闭)
- 未修复中优先级: M-02(文件上传MIME)、M-03(文件名消毒)、M-04(分布式token)、M-05(Bridge签名)、M-06(stringInt)、M-08(isOriginAllowed)、M-09(WebSocket超时)
- 未修复低优先级: L-01(错误大写)、L-02(CORS重复)、L-03(nonce fallback)、L-05(强类型)
- 验证报告: `docs/ISSUES_VERIFICATION_2026-04-29.md`
- **2026-05-07 修复进展**: 已修复 13 项（C-01/C-02/C-03/C-04/H-01/H-04/M-02/M-03/M-06/M-08/M-09/L-01 + H-05 已默认开启），全部通过 `go build` 和 `go test -race`
- **仍剩余 5 项未修复**: H-02(WebSocket token 已修复但需验证)、H-05(rate_limit 配置已默认true无需代码改动)、M-04(分布式节点token)、M-05(Bridge签名)、L-02(CORS重复)、L-03(nonce fallback)、L-05(强类型)

### Extension 模式问题清单 (2026-05-09)

- 4 个问题记录于 `docs/ISSUES_EXTENSION_MODE_2026-05-09.md`
- **P0**: UQL 未翻译直接发送到搜索引擎（BuildSearchEngineURL 绕过 Translate）
- **P0**: 结构化数据无法返回（UQL 问题连锁 + CDP 新浏览器实例 + Bridge 超时）
- **P1**: 查询进度卡在 0%（handleWebSocketQuery 从未调用 updateQueryProgress）
- **P1**: 登录状态同步不准确（扩展配对后所有引擎直接报 logged_in=true）
- 修复依赖：问题1 → 问题3，问题2 可独立修复，问题4 需扩展端配合

### 遗留低优先级事项

| # | 项目 | 严重度 | 说明 |
|---|------|--------|------|
| 1 | Scheduler 编辑 UI | 低 | 后端已实现，前端缺少编辑按钮和表单 |
| 2 | 测试覆盖率提升 | 中 | 当前65.1%，Phase 1已超额完成（目标55%），待继续Phase 2/3达到80%标准 |
| 3 | 数据竞争修复 | ✅ 已完成 | CircuitBreaker添加mutex保护，测试atomic计数器 |
| 4 | Phase 1 剩余模块 | 低 | adapter/screenshot/service 已有基础测试，mock层待完善 |

### 扩展结构化数据提取升级 (2026-05-09)

- **commit**: `4584e9d` — feat(extension): upgrade capture.js with card-based selectors and extraction fallbacks
- **核心问题**: FOFA 已从 table 布局迁移到卡片布局，原有选择器全部失效
- **已完成**:
  - FOFA 选择器从 4 个扩展到 12 个（卡片+混合+表格回退）
  - Hunter/ZoomEye/Quake 增加 .el-table 和通用 div 回退
  - 新增 cardBasedExtraction() 兜底提取（IP 模式匹配 + 链接分组）
  - 新增 href 模式匹配：`a[href*='qbase64=aXA9']` 等用于可靠识别 IP/端口/协议
  - 新增 4 个测试工具：test_extract_console.js（控制台）、test_extraction_node.js（Node.js）、dom_inspector.js（DOM分析）、test_collect.js（已同步更新）
- **验证结果**:
  - Chrome MCP 连接正常
  - FOFA 搜索结果页正常显示（`country="CN" && port="80"` 返回 65,947,250 条匹配）
  - Go 后端构建通过，`go test -race ./...` 全通过
  - Bridge 配对成功（获取 token）
  - 所有 JS 文件通过 `node -c` 语法检查
- **待验证（明天继续）**:
  - 在 Chrome 中加载扩展（developer mode load unpacked）
  - 在各搜索引擎已登录的搜索结果页运行 test_extract_console.js
  - 确认 items_count > 0，或根据 debug 输出调整选择器
  - Bridge 端到端 collect 模式测试（Go 下发任务 → 扩展执行 → 返回结构化数据）
