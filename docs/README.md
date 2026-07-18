# UniMap 文档索引

## 当前操作文档

- [快速开始](QUICKSTART.md)：本地配置、Web、CLI、GUI 启动。
- [使用指南](USAGE.md)：查询、配置、截图和 GUI。
- [API](API.md)：当前 `/api/v1` HTTP 契约。
- [运维 Runbook](RUNBOOK.md)：服务、认证、截图、Bridge、调度和节点排障。
- [截图扩展运维](OPS_SCREENSHOT_EXTENSION.md)：本机配对、Bridge token 与回调协议。
- [无图形浏览器运行时决策](DECISIONS/0006-headless-browser-runtime.md)：云主机 CDP、统一路由、会话限制和容器边界。
- [架构](ARCHITECTURE.md) 与 [业务架构](BUSINESS_AND_LOGIC_ARCHITECTURE.md)。
- [UQL 指南](UQL_GUIDE.md) 与 [搜索引擎语法快照](SEARCH_ENGINE_SYNTAX.md)。
- [插件架构](PLUGIN_ARCHITECTURE.md) 与 [插件开发](PLUGIN_DEVELOPMENT_GUIDE.md)。
- [GUI 构建](GUI_BUILD.md)。
- [变更日志](CHANGELOG.md)：按日期记录已完成的功能、兼容性与验证结果。

## 决策与历史资料

- [2026-07-17 代码逻辑、API 适配与用户体验问题报告](CODE_LOGIC_API_UX_REVIEW_2026-07-17.md)：14 项初始问题及后续截图、通知、账号、ICP、调度前后端交互复核记录。
- [2026-07-17 完整修复与回滚指南](REMEDIATION_GUIDE_2026-07-17.md)：实施结果、两轮交互闭环、兼容性、验证门槛以及上线回滚清单。
- [决策记录](DECISIONS/)：保留当时的背景与结论；若与当前代码冲突，以当前 API/架构文档和代码为准。
- [archive](archive/)：历史计划、审计、测试与提交资料，不是当前操作指引。
- [API 版本化实施方案](API_VERSIONING.md)：已完成的历史设计；旧 `/api` shim 已移除。
- [生产就绪计划](PRODUCTION_READINESS_PLAN.md)：历史计划快照，不是当前发布门禁。
- [2026-07-14 Bridge 截图与通知验收](E2E_BRIDGE_SCREENSHOT_NOTIFICATION_2026-07-14.md)：稳定引擎的受控真实联调快照。
- [2026-07-15 查询通知与 Bridge 定时闭环验收](E2E_BRIDGE_SCHEDULED_QUERY_CLOSED_LOOP_2026-07-15.md)：API/Bridge 查询、资产明细通知、截图、SQLite 结果和五引擎复测状态。
- [2026-07-14 持久化与前后端终检](FINAL_PERSISTENCE_FRONTEND_AUDIT_2026-07-14.md)：持久化重载、API 契约和前端渲染的日期化验收。
- [2026-07-15 逻辑可用性三项完善闭环](LOGIC_USABILITY_CLOSEOUT_2026-07-15.md)：Alert 原子持久化回归、ICP history 部分匹配与 scheduler payload 兼容记录。

浏览器运行策略和查询降级计划已归档至 [archive/plans](archive/plans/)。

## 安全与隐私

部分历史资料仍在仓库内，用于追溯决策和验证；它们不应被当作当前事实或操作步骤。所有文档、测试记录和 issue 中都不得新增真实 API Key、Cookie、管理令牌、Bridge token、通知凭证或未授权资产信息。
