# Screenshot Extension Ops Runbook

> 最后按代码核对：2026-07-15。适用于 `tools/extension-screenshot` manifest 0.4.1。

## 运行边界

- 扩展 Bridge 面向本机浏览器；配对、令牌轮换、任务拉取和结果回调均限制为 loopback 请求。
- API 路径必须使用 `/api/v1/screenshot/bridge/...`。旧 `/api/screenshot/bridge/...` 已移除。
- `health` 和 `status` 对远程请求仅返回最小信息；本机请求可获得诊断快照。
- 未实现 `/diagnostic` 端点；不要将它加入探活、监控或自动化脚本。

## 启动与诊断

启动服务和浏览器扩展后，在服务所在机器执行：

```powershell
Invoke-RestMethod http://127.0.0.1:8448/api/v1/screenshot/bridge/health
Invoke-RestMethod http://127.0.0.1:8448/api/v1/screenshot/bridge/status
Invoke-RestMethod http://127.0.0.1:8448/api/v1/screenshot/router/status
```

扩展连接失败时，依次确认：

1. 扩展配置中的服务地址指向正确的本机 UniMap 服务。
2. 服务和扩展处于同一 host，且本机可访问 8448 端口。
3. 配对码与 `screenshot.extension.pair_code` 一致。
4. pairing 开启时，扩展保存的 bridge token 未过期；必要时重新配对。
5. CDP 或 Extension 引擎健康状态是否可用。可通过 `POST /api/v1/screenshot/set-mode` 在 `cdp`、`extension`、`auto` 间切换。

## 配对与令牌

配对只允许 loopback 请求：

```powershell
$body = @{ client_id = 'chrome-extension'; pair_code = $env:UNIMAP_EXTENSION_PAIR_CODE } | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8448/api/v1/screenshot/bridge/pair `
  -ContentType application/json -Body $body
```

成功后服务返回 `token`、`expires_in` 和 `expire_at`。将 token 仅保存在扩展本地存储。任务拉取、回调与轮换使用：

```text
Authorization: Bearer <bridge-token>
```

令牌轮换也仅允许 loopback：

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8448/api/v1/screenshot/bridge/token/rotate `
  -Headers @{ Authorization = "Bearer $env:UNIMAP_BRIDGE_TOKEN" } `
  -ContentType application/json -Body '{"revoke_old":true}'
```

如果服务重启导致短期 token 失效，重新配对即可。管理令牌可在 loopback 本机恢复流程中被接受，但不得暴露给远程扩展或记录到日志。

## 任务协议

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/screenshot/bridge/tasks/next` | 获取一个待执行任务；无任务时返回 `task: null` |
| POST | `/api/v1/screenshot/bridge/mock/result` | 回传任务结果 |

任务可使用 `collect`、`screenshot` 或 `collect_and_capture` 动作。扩展对三种动作执行一致的等待、懒加载与稳定性处理；不要将 `collect_and_capture` 当作未支持的私有动作。

当启用回调签名时，回传还必须包含 `X-Bridge-Timestamp`、`X-Bridge-Nonce` 与 `X-Bridge-Signature`。服务会验证时间窗口、nonce 重放和 body HMAC。

## 受支持的采集目标

当前扩展源码含 FOFA、Hunter、ZoomEye、Quake、Shodan、Censys、DayDayMap 的页面识别和选择器。Web 界面仅将前五个作为稳定引擎展示；后两个的 API 适配器与扩展选择器存在，但不应据此承诺 UI 已全面启用。

## 常用验证

搜索页截图：

```powershell
Invoke-RestMethod 'http://127.0.0.1:8448/api/v1/screenshot/search-engine?engine=fofa&query=port%3D%2280%22'
```

异步 URL 批量截图：

```powershell
$body = @{ urls = @('https://example.com'); concurrency = 1 } | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8448/api/v1/screenshot/batch-urls `
  -ContentType application/json -Body $body
```

随后用返回的 `job_id` 调用：

```text
GET /api/v1/screenshot/batch/progress?job_id=<job_id>
```

服务会拒绝私有、回环或内部目标；这是 SSRF 防护，不能通过重试或切换扩展绕过。

## Bridge 查询、截图与通知联调

以下显式标签测试会启动一个仅监听 `127.0.0.1:8448` 的临时服务，创建一次
搜索截图任务，并验证 Bridge 回调、截图文件及飞书应用图片通知：

```powershell
go test -tags live_bridge_e2e ./web -run '^TestLiveBridgeSearchScreenshotNotification$' -count=1 -v
```

完整定时查询闭环（Bridge 结构化采集 + 截图 + SQLite 查询结果 + 图片通知）使用：

```powershell
go test -tags live_bridge_e2e ./web -run '^TestLiveBridgeScheduledQueryClosedLoop$' -count=1 -v
```

默认引擎为 FOFA；可在 PowerShell 设置 `UNIMAP_LIVE_BRIDGE_ENGINE` 为 `hunter`、`zoomeye`、`quake` 或 `shodan` 后执行同一命令。

需要人工核验截图内容时，可将 `UNIMAP_LIVE_BRIDGE_ARTIFACT_DIR` 设为一个绝对路径。测试会以仅当前用户可读的权限保留一份 PNG 副本；未设置时，截图随临时测试目录清理。

它会使用本机 `configs/config.yaml`、已配对的扩展和当前登录的搜索引擎会话，向已启用的
`feishu_app` 渠道发送一条真实测试消息。该测试不适合 CI，也不得在未获通知接收方同意时执行；
它不会打印凭证，且不会修改原配置或持久化任务。

## 变更与回滚

- 修改 Bridge API、token 或回调协议前，先更新扩展与服务端，再执行 `go test -race ./...`。
- 将模式切回 CDP：`POST /api/v1/screenshot/set-mode`，JSON 为 `{"mode":"cdp"}`。
- 扩展问题不能通过恢复旧 `/api/...` shim 解决；调用方必须迁移到 `/api/v1/...`。
