# UniMap 全流程功能验收（2026-07-12 最终版）

> **历史验收快照**：结果仅代表 2026-07-12 的特定部署与凭证状态，不代表当前生产可用性。以下测试目标、组织名称、第三方代理和通知通道标识已脱敏；不得复用本记录中的目标或参数。
>
> **后续勘误（2026-07-14）**：本记录中“截图引擎不可用”仅是当日 Chrome Extension 未连接的环境状态。受控本机 Bridge 的后续成功态证据见 [`E2E_BRIDGE_SCREENSHOT_NOTIFICATION_2026-07-14.md`](E2E_BRIDGE_SCREENSHOT_NOTIFICATION_2026-07-14.md)。
>
> **后续勘误（2026-07-15）**：下文 ICP history `keyword` 精确匹配与调度 payload 严格格式仅是本次验收当日行为。当前 `keyword` 已支持字面量部分匹配；调度器已支持创建期 Runner 校验、逗号字符串数组及 `targets`/`engine` 兼容。当前契约以 [`API.md`](API.md) 为准。
>
> **后续勘误（2026-07-23）**：本记录中的 Censys、DayDayMap 结果属于 API 查询证据，不是 Bridge 或 CDP 抓取证据。两者当前仍未接入稳定 Web UI，也未完成 Bridge/CDP 结构化采集和 live E2E。

## 测试范围

- 资产目标：`<REDACTED_TEST_TARGET>`
- 篡改 URL：`https://<REDACTED_TEST_TARGET>`、`https://<REDACTED_TEST_TARGET>:<PORT>`
- ICP 关键词：`<REDACTED_TEST_KEYWORD>`
- 查询引擎：FOFA、Hunter、DayDayMap、Censys
- 通知渠道：飞书 Webhook、飞书应用、企业微信（通道标识已脱敏）

## 验收结果

| 功能 | API 调用 | 持久化 | 通知推送 | 调度任务 | 状态 |
|------|---------|--------|---------|---------|------|
| 资产查询 | ✅ 15 条 | ✅ 14 条历史 | ✅ 三渠道 | ✅ 5s | 通过 |
| ICP 备案查询 | ✅ 5 条 | ✅ 2 runs / 10 results | ✅ 三渠道 | ✅ 1s | 通过 |
| 篡改检测 | ✅ 2/2 normal | ✅ 4 条历史 | ✅ 三渠道 | ✅ 10s | 通过 |
| 端口扫描 | ✅ 4 端口开放 | N/A | ✅ 三渠道 | ✅ CDN 排除 | 通过 |
| URL 可达性 | ✅ 2/2 可达 | N/A | N/A | N/A | 通过 |
| Web 探测 | ✅ is_web=true | N/A | N/A | N/A | 通过 |
| 搜索引擎截图 | ❌ 无 Chrome 扩展 | N/A | N/A | N/A | 阻断 |

## 业务数据

### 资产查询

- 引擎：FOFA 15 条 + Hunter 6 条 + DayDayMap 13 条
- 去重总资产：15 条
- 主要端口：22、80、110、143、465、587、8080、8081、8888、993、995、3333、8018
- 持久化：`/api/v1/history` 14 条记录

### ICP 备案查询

- 单位：`<REDACTED_TEST_ORGANIZATION>`
- 主备案号：[REDACTED]08001197号
- 域名（5 条）：36.147.22.12、yunyusec.com、ynydbigdata.cn、yunyusec.cn、yunyusec.net
- 持久化：`icp_query_runs` 2 条、`icp_results` 10 条
- 注意：ICP 历史 API 的 `keyword` 参数要求**精确匹配**

### 篡改检测

| URL | 基线 | 检测 | 结果 |
|-----|------|------|------|
| `<REDACTED_TEST_TARGET>:<PORT>` | hash=91832dd5... | hash=91832dd5... | normal（一致） |
| `<REDACTED_TEST_TARGET>` | hash=d3f2e1de... | hash=248dd3e2... | normal（动态内容） |

- 持久化：`/api/v1/tamper/history` 4 条记录

### 端口扫描

- 开放端口：22 (SSH)、80 (HTTP)、8080、16181 (ICP)
- 注意：本次部署的直接 IP 扫描受 CDN 检测影响；这是历史环境观察，不应据此设计绕过策略。
- 调度任务 payload 格式：`urls`（字符串数组）+ `ports`（字符串数组）

### 通知推送

| 渠道 | 类型 | 状态 |
|------|------|------|
| `<REDACTED_CHANNEL>` | 飞书 Webhook | ✅ 送达 |
| feishu_app | 飞书应用 | ✅ 送达 |
| `<REDACTED_CHANNEL>` | 企业微信 | ✅ 送达 |

### 定时任务调度

| 任务 | 类型 | 耗时 | 状态 |
|------|------|------|------|
| 资产查询 | query | 5s | ✅ success |
| ICP 备案查询 | icp_query | 1s | ✅ success |
| 篡改检测 | tamper_check | 10s | ✅ success |
| 端口扫描 | port_scan | - | ✅ success（CDN 排除） |

## 阻断项

| # | 问题 | 严重度 | 说明 |
|---|------|--------|------|
| 1 | 截图引擎不可用 | 低 | 截图模式为 extension 但无 Chrome 扩展连接，需在浏览器中安装扩展 |

## 本轮代码改动

| 文件 | 改动 |
|------|------|
| `config/notify_secret.go` | PEPPER 缺失 Fatal → Warn |
| `config/config_load.go` | ResolveEnv 未解析返回空字符串 |
| `utils/urlguard/check.go` | SafeHTTPClient 代理兼容 |
| `config/config_test.go` | 测试预期同步 |
| `adapter/fofa.go` | 第三方 FOFA 接口兼容 |
| `docs/DECISIONS/0004-config-plaintext-secrets.md` | 决策：config 允许含密钥 |
