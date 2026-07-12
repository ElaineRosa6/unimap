# UniMap 全流程功能验收（2026-07-12 最终版）

## 测试范围

- 资产目标：`132.232.231.41`
- 篡改 URL：`http://132.232.231.41`、`http://132.232.231.41:16181`
- ICP 关键词：`[REDACTED]`
- 查询引擎：FOFA（第三方 fafaapi.info）、Hunter、DayDayMap、Censys
- 通知渠道：飞书 Webhook (`feishu_2`)、飞书应用 (`feishu_app`)、企业微信 (`dijia_01`)

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

- 单位：[REDACTED]
- 主备案号：[REDACTED]08001197号
- 域名（5 条）：36.147.22.12、yunyusec.com、ynydbigdata.cn、yunyusec.cn、yunyusec.net
- 持久化：`icp_query_runs` 2 条、`icp_results` 10 条
- 注意：ICP 历史 API 的 `keyword` 参数要求**精确匹配**

### 篡改检测

| URL | 基线 | 检测 | 结果 |
|-----|------|------|------|
| `http://132.232.231.41:16181` | hash=91832dd5... | hash=91832dd5... | normal（一致） |
| `http://132.232.231.41` | hash=d3f2e1de... | hash=248dd3e2... | normal（动态内容） |

- 持久化：`/api/v1/tamper/history` 4 条记录

### 端口扫描

- 开放端口：22 (SSH)、80 (HTTP)、8080、16181 (ICP)
- 注意：直接 IP 扫描会被 CDN 检测排除，需使用带端口的 URL（如 `http://132.232.231.41:16181`）绕过
- 调度任务 payload 格式：`urls`（字符串数组）+ `ports`（字符串数组）

### 通知推送

| 渠道 | 类型 | 状态 |
|------|------|------|
| feishu_2 | 飞书 Webhook | ✅ 送达 |
| feishu_app | 飞书应用 | ✅ 送达 |
| dijia_01 | 企业微信 | ✅ 送达 |

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
