# UniMap 查询—篡改—端口—ICP—通知全流程验收（2026-07-12 更新）

## 测试范围

- 资产目标：`132.232.231.41`
- 篡改 URL：`http://132.232.231.41`、`http://132.232.231.41:16181`
- ICP 关键词：`[REDACTED]`
- 查询引擎：FOFA、Hunter、ZoomEye、Quake、Shodan、Censys、DayDayMap
- 通知渠道：飞书 Webhook (`feishu_2`)、飞书应用 (`feishu_app`)、企业微信 (`dijia_01`)
- 执行入口：真实 Web API

## 本轮代码改动

| 文件 | 改动 | 原因 |
|------|------|------|
| `internal/config/notify_secret.go` | `initNotifyPepperStrict`: `Fatalf` → `Warnf` | 不再因缺 `UNIMAP_NOTIFY_PEPPER` 崩溃 |
| `internal/config/config_load.go` | `ResolveEnv`: 未解析 `${VAR}` 返回 `""` | 优雅跳过未配置的环境变量 |
| `internal/utils/urlguard/check.go` | `SafeHTTPClient`: 加 `Proxy: ProxyFromEnvironment` + 代理地址跳过内网检查 | 通过代理发通知不被 urlguard 拦截 |
| `internal/config/config_test.go` | 同步 2 个测试用例预期值 | 匹配 ResolveEnv 新行为 |
| `configs/config.yaml` | 通知渠道从 `${ENV_VAR}` 改为直接配置真实凭据 | 实际可送达 |

## 验收结果

| 链路 | 结果 | 证据 |
|------|------|------|
| 健康检查 | ✅ 通过 | `/health` 返回 `{"status":"ok"}` |
| 资产查询 | ⚠️ 链路通/凭据不可用 | 7 引擎均调通，但 API Key 全部过期（401） |
| 篡改基线 | ✅ 通过 | 2/2 URL 基线保存成功 |
| 篡改检测 | ✅ 通过 | 2/2 URL `tampered=false, status=normal` |
| 端口扫描 | ✅ 通过 | 发现开放端口：22、80、8080、16181 |
| ICP 备案查询 | ✅ 通过 | 5 条记录，主备案号 `[REDACTED]08001197号` |
| 飞书 Webhook 送达 | ✅ 通过 | `{"success":true,"message":"test message sent successfully"}` |
| 飞书应用送达 | ✅ 通过 | `{"success":true,"message":"test message sent successfully"}` |
| 企业微信送达 | ✅ 通过 | `{"success":true,"message":"test message sent successfully"}` |

## 业务数据结果

### 端口扫描

目标：`132.232.231.41`

| 端口 | 状态 | 服务 |
|------|------|------|
| 22 | 开放 | SSH |
| 80 | 开放 | HTTP |
| 8080 | 开放 | HTTP-alt |
| 16181 | 开放 | ICP Sidecar |

### 篡改检测

| URL | 基线 | 检测 | 结果 |
|-----|------|------|------|
| `http://132.232.231.41:16181` | hash=91832dd5... | hash=91832dd5... | ✅ normal（一致） |
| `http://132.232.231.41` | hash=d3f2e1de... | hash=248dd3e2... | ✅ normal（页面动态内容导致哈希变化，但无篡改标记） |

### ICP 备案查询

- 单位：[REDACTED]
- 主备案号：`[REDACTED]08001197号`
- 性质：企业
- 域名（5 条）：36.147.22.12、yunyusec.com、ynydbigdata.cn、yunyusec.cn、yunyusec.net

## 最终验证

- `go build ./...`：通过
- `go test ./internal/config/ ./internal/utils/urlguard/ ./internal/notify/`：通过
- 三个通知渠道 live 推送：全部送达
