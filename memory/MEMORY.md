# Project Memory Index

## 当前活跃

- [Extension 模式问题 2026-05-09](project_extension_mode_issues_2026-05-09.md) — ✅P0已修复(Shodan补齐+翻译路径验证)、✅P1进度已实现、P1登录状态部分解决
- [Bridge 认证修复 2026-06-03](project_bridge_auth_fix_2026-06-03.md) — ✅截图超时根因(重启丢token→401)修复：admin token loopback 兜底+签名/pairing联动+5测试+真机curl E2E全绿
- [定时任务优化进度 2026-06-02](project_scheduler_optimization_2026-06-02.md) — P3-P6完成详情+修复记录+后续计划

## 当前文档（docs/）

- [E2E采集验证 2026-06-04](../docs/E2E_COLLECTION_VERIFICATION_2026-06-04.md) — 🟡 截图✅ Shodan 6/6 + Quake 2/2；采集🔴 3bug修复(CANARY已清理)+诊断日志(`[bridge-collect]`)+**parseStructuredCollectedData 4项修复**已添加(port/status_code string→int、banner→BodySnippet)；下次重启Chrome+服务器后测试定位数据丢失点；Quake反爬拦截(非账号无权限,手动查询正常)
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
