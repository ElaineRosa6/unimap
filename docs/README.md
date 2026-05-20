# UniMap 文档索引

## 文档分类

### 用户文档

| 文档 | 说明 |
|------|------|
| [QUICKSTART.md](./QUICKSTART.md) | 快速入门指南 |
| [USAGE.md](./USAGE.md) | 使用手册 |
| [UQL_GUIDE.md](./UQL_GUIDE.md) | UQL 统一查询语言指南 |
| [TAMPER_DETECTION_FEATURE.md](./TAMPER_DETECTION_FEATURE.md) | 篡改检测功能说明 |
| [ZOOMEYE_TROUBLESHOOTING.md](./ZOOMEYE_TROUBLESHOOTING.md) | ZoomEye 故障排除指南 |
| [API_KEYS.md](./API_KEYS.md) | API 密钥获取指南 |

### 技术文档

| 文档 | 说明 |
|------|------|
| [ARCHITECTURE.md](./ARCHITECTURE.md) | 架构设计文档 |
| [API.md](./API.md) | API 接口文档 |
| [PLUGIN_ARCHITECTURE.md](./PLUGIN_ARCHITECTURE.md) | 插件架构文档 |
| [PLUGIN_DEVELOPMENT_GUIDE.md](./PLUGIN_DEVELOPMENT_GUIDE.md) | 插件开发指南 |
| [BROWSER_COLLECT_STRATEGY.md](./BROWSER_COLLECT_STRATEGY.md) | 浏览器采集策略 |
| [ICP_INTEGRATION.md](./ICP_INTEGRATION.md) | ICP 备案集成指南 |
| [DECISIONS/](./DECISIONS/) | 架构决策记录 (ADR) |

### 运维文档

| 文档 | 说明 |
|------|------|
| [RUNBOOK.md](./RUNBOOK.md) | 运维手册（6 场景故障处理） |
| [PRODUCTION_READINESS_PLAN.md](./PRODUCTION_READINESS_PLAN.md) | 生产就绪清单 |
| [OPS_SCREENSHOT_EXTENSION.md](./OPS_SCREENSHOT_EXTENSION.md) | 截图扩展运维 |
| [GUI_BUILD.md](./GUI_BUILD.md) | GUI 构建指南 |
| [DEVELOPMENT_GUIDE.md](./DEVELOPMENT_GUIDE.md) | 开发指南 |
| [CHANGELOG.md](./CHANGELOG.md) | 变更日志 |

### 工作计划

| 文档 | 说明 |
|------|------|
| [ACTIVE_WORK_PLAN.md](./ACTIVE_WORK_PLAN.md) | 当前活跃工作计划（状态跟踪） |

### 监控

| 文档 | 说明 |
|------|------|
| [grafana-dashboard.json](./grafana-dashboard.json) | Grafana 主面板配置 |
| [grafana-icp-dashboard.json](./grafana-icp-dashboard.json) | Grafana ICP 面板配置 |

## 归档文档

已完成的历史文档已归档到 `docs_archive/` 目录：

| 归档文档 | 说明 |
|---------|------|
| OPTIMIZATION_PLAN_2026-05-19.md | 优化实施计划（全文档 → ACTIVE_WORK_PLAN.md 摘要） |
| REVIEW_ISSUES_2026-05-19.md | 代码审查问题清单 |
| slice0-baseline.md | Slice 0 基线盘点 |
| slice2-bridge-auth-contract.md | Bridge 鉴权契约（→ DECISIONS/0002） |
| refactor-plan-v1.html | 重构切片计划 v1 |
| CODE_REVIEW_REPORT_2026-05-09.md | 代码审查报告 2026-05-09 |
| CODE_REVIEW_REPORT_2026-05-13.md | 代码审查报告 2026-05-13 |
| DEEP_CODE_REVIEW_2026-04-29.md | 深度代码审查 2026-04-29 |
| SECURITY_AUDIT_REPORT.md | 安全审计报告 2026-04-26 |
| SECURITY_REVIEW_REPORT_2026-05-14.md | 安全审查报告 2026-05-14 |
| 20260429_issues.md | 问题核查表 2026-04-29 |
| ISSUES_VERIFICATION_2026-04-29.md | 问题验证报告 2026-04-29 |
| ISSUES_EXTENSION_MODE_2026-05-09.md | 扩展模式问题清单 2026-05-09 |
| EXTENSION_BRIDGE_DEBUG_2026-05-09.md | 扩展桥接调试报告 2026-05-09 |
| ENGINE_BROWSER_AUTOMATION_REMEDIATION_PLAN_2026-05-06.md | 引擎浏览器自动化修复计划 |
| ICP_IMPLEMENTATION_SUMMARY.md | ICP 实施总结 |
| FRONTEND_OPTIMIZATION_2026-05-18.md | 前端优化记录 2026-05-18 |
| OPTIMIZATION_PLAN.md | 旧版优化改进计划 |
| IMPLEMENTATION_GUIDE.md | 安全重构实施指导 |
| TEST_COVERAGE_PLAN.md | 测试覆盖率提升计划 |
| WORK_LOG_2026-04-21.md | 工作日志 2026-04-21 |
| DOC_REALITY_CHECKLIST_2026-04-24.md | 文档真实性检查清单 |

## 维护指南

- **新增功能** → 更新 CHANGELOG.md
- **生产就绪** → 更新 PRODUCTION_READINESS_PLAN.md
- **安全审计** → 新报告放入 docs_archive/，更新 PRODUCTION_READINESS_PLAN.md
- **测试验证** → 直接在测试代码中体现，不单独写证据文件
- **历史计划** → 完成后移入 docs_archive/，在 ACTIVE_WORK_PLAN.md 更新状态
- **架构决策** → 新增 ADR 到 DECISIONS/ 目录，编号递增
