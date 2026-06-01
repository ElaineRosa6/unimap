# Project Memory Index

## 当前遗留问题

- [Extension 模式问题 2026-05-09](project_extension_mode_issues_2026-05-09.md) — UQL未翻译(P0)、结构化数据返回(P0)、进度卡0%(P1)、登录状态不准(P1)

## 本轮修复记录（docs/）

- [问题修复报告 2026-06-01](../docs/FIX_REPORT_2026-06-01.md) — Phase 1-4 全量修复 + Hunter 限流根因
- [问题记录与计划 2026-06-01](../docs/ISSUES_AND_PLAN_2026-06-01.md) — 4 类问题记录 + 整改计划
- [项目审查报告 2026-05-31](../docs/PROJECT_REVIEW_2026-05-31.md) — 最近一次全量审查
- [项目审查报告 2026-06-01](../docs/PROJECT_REVIEW_2026-06-01.md) — 🆕 第二轮全量审查：7 维度深度分析，18 项新发现（2 P0 + 14 P1 + 11 P2）
- [修复实施计划 2026-06-01](../docs/FIX_PLAN_2026-06-01.md) — 🆕 5 阶段修复计划：22 个问题按依赖链排序，不破坏现有功能
- [修复实施记录 2026-06-01](../docs/FIX_IMPLEMENTATION_2026-06-01.md) — 🆕 实施完成记录：5 阶段 21/24 项已修复，23 文件变更，go test -race 全绿

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

## 已归档（docs/archive/）

| 目录 | 内容 |
|------|------|
| [reviews/](../docs/archive/reviews/) | 2026-05-19~05-25 审查链（7 个文件）：CODE_REVIEW → FIX → VERIFICATION → DEEP_REVIEW → AUDIT |
| [plans/](../docs/archive/plans/) | 已实施的功能计划（6 个文件）：ICP/通知/FOFA/浏览器降级策略 |

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
