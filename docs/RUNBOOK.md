# UniMap 运维 Runbook

> 最后按代码核对：2026-07-15。所有业务 API 使用 `/api/v1/...`；旧 `/api/...` 路径已移除。

## 0. 先确认服务与认证

```powershell
Invoke-RestMethod http://127.0.0.1:8448/health
Invoke-RestMethod http://127.0.0.1:8448/health/ready
Invoke-RestMethod http://127.0.0.1:8448/health/live
```

生产环境如启用了认证，按部署方式带上会话、API Key 或管理令牌。Prometheus 指标端点是 `/metrics`：启用认证时必须携带管理令牌；未认证的非 loopback 部署会被拒绝。

```powershell
Invoke-RestMethod http://127.0.0.1:8448/metrics -Headers @{ 'X-Admin-Token' = $env:UNIMAP_ADMIN_TOKEN }
```

不要把管理令牌、Bridge token 或引擎 Key 写入命令历史、工单或日志。

## 1. 服务无法启动

1. 检查配置路径与环境变量占位符：默认配置是 `configs/config.yaml`，示例为 `configs/config.yaml.example`。
2. 确认端口未被占用：`Get-NetTCPConnection -LocalPort 8448 -ErrorAction SilentlyContinue`。
3. 检查日志中的配置、数据库和浏览器初始化错误。
4. 最小校验：`go test -race ./...`，然后 `go run ./cmd/unimap-web`。

非 loopback 监听且 Web 认证未启用时，服务会 fail-closed；这是预期安全行为，应该修正部署配置而不是绕过认证。

## 2. 查询失败、无结果或某引擎不可用

1. 先确认登录状态：`GET /api/v1/cookies/login-status`。
2. 使用 API 查询时发送表单字段 `query`、可选 `engines` 与 `page_size`；不要发送旧文档中的 JSON `limit/offset/timeout` 请求体。
3. `page_size` 最大为 500。查询状态使用 `GET /api/v1/query/status?query_id=...`。
4. 检查目标引擎 API Key、Cookie、额度和网络连通性。当前 Web UI 展示核心五引擎；Censys/DayDayMap 已有适配器但不属于稳定 UI 列表。

## 3. Chrome/CDP 或截图失败

检查 CDP：

```powershell
Invoke-RestMethod http://127.0.0.1:8448/api/v1/cdp/status
```

检查截图路由：

```powershell
Invoke-RestMethod http://127.0.0.1:8448/api/v1/screenshot/router/status
```

可将模式切换为 `cdp`、`extension` 或 `auto`：

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8448/api/v1/screenshot/set-mode `
  -ContentType application/json -Body '{"mode":"auto"}'
```

单站截图使用 `POST /api/v1/screenshot`，请求体仅需 `{"url":"https://example.com"}`。目标 URL 被校验为公网 HTTP/HTTPS 地址；被拒绝的内网、loopback 或私有地址不是截图服务故障。

## 4. 浏览器扩展无法配对或截图

1. 扩展和服务应运行在同一台机器的 loopback 环境。
2. 在服务本机检查：

```powershell
Invoke-RestMethod http://127.0.0.1:8448/api/v1/screenshot/bridge/health
Invoke-RestMethod http://127.0.0.1:8448/api/v1/screenshot/bridge/status
```

3. 用 `POST /api/v1/screenshot/bridge/pair` 配对，JSON 为 `client_id` 和 `pair_code`。如果开启 pairing，后续 task/result/rotate 请求需 `Authorization: Bearer <bridge-token>`。
4. bridge token 过期或服务重启后，在 loopback 下重新配对；管理令牌只可作为本机恢复路径使用，不应配置到远程扩展。
5. 不要探测不存在的 `/diagnostic` 端点。完整协议见 [截图扩展运维说明](OPS_SCREENSHOT_EXTENSION.md)。

## 5. 批量截图、文件或任务进度异常

- 异步 URL 批量截图：`POST /api/v1/screenshot/batch-urls`，JSON：`urls`、可选 `batch_id`、`concurrency`。
- 进度：`GET /api/v1/screenshot/batch/progress?job_id=...`。
- 文件：`GET /api/v1/screenshot/batches/files?batch=...`。
- 删除批次：`DELETE /api/v1/screenshot/batches/delete?batch=...`。

`batch_id` 是创建请求中的可选 JSON 字段；列文件和删除时的查询参数叫 `batch`。

## 6. 巡检、篡改检测或基线异常

支持的模式只有：`strict`、`relaxed`、`security`、`balanced`、`precise`。`malicious`、`performance`、`full` 是历史名称，不能再用于新请求。

- 检测：`POST /api/v1/tamper/check`，JSON：`urls`、可选 `concurrency`、`mode`。
- 设置基线：`POST /api/v1/tamper/baseline`。
- 删除基线：`DELETE /api/v1/tamper/baseline/delete?url=...`。
- 历史：`GET /api/v1/tamper/history?limit=...&offset=...&url=...&type=...&mode=...&q=...`；`limit` 最大 1000，`offset` 最大 100000。
- 导出：`GET /api/v1/tamper/history/export`，支持同样的过滤参数。

这些 URL 也会进行 SSRF 防护。对于 SPA 目标，先确认截图/浏览器能力可用，再判断空 hash 或不可达结果。

## 7. 调度器与通知

```powershell
Invoke-RestMethod http://127.0.0.1:8448/api/v1/scheduler/tasks
Invoke-RestMethod http://127.0.0.1:8448/api/v1/scheduler/history
```

创建任务的端点是 `POST /api/v1/scheduler/tasks/create`，不是旧的 `/api/scheduler/tasks`。一次性、延迟和 cron 任务分别通过 `schedule_type` 的 `once`、`delay`、`cron` 表示。通知通道从 `GET /api/v1/notifications/channels` 查看，并可用 `/api/v1/notifications/channels/test` 验证。

需要定时查询的完整 Bridge 闭环时，`query` 任务 payload 必须包含：

```json
{
  "query": "port=\"443\"",
  "engines": ["fofa"],
  "page_size": 10,
  "notification_detail_limit": 50,
  "browser_query": true,
  "browser_action": "collect_and_capture"
}
```

并为任务启用成功通知及至少一个支持图片的渠道。排障顺序：

1. `/api/v1/screenshot/bridge/status` 显示扩展近期有拉取或回调活动。
2. 调度执行结果包含“Bridge 截图保存”和“采集结果已合并并持久化”。
3. `/api/v1/history?type=query` 只有一条对应查询历史，并能读取结果明细。
4. `/api/v1/scheduler/history` 中该次执行为 `success`，且 `result` 含“查询结果明细”和至少一条已持久化资产；不能只核对总数。
5. 通知接收端确认文字明细与图片均送达。`notification_detail_limit` 默认 50、最大 100，正文另有约 20 KiB 的渠道安全上限；超过部分仍持久化并在通知中提示。通知投递仍是任务完成后的异步阶段，投递失败会记录日志和指标，不会回滚已经持久化的查询结果。

普通 API 定时查询不设置 `browser_query`，但通知明细规则相同。真实 API 配置可用时可执行显式联调：

```powershell
$env:UNIMAP_LIVE_API_ENGINE = 'fofa'
go test -tags live_bridge_e2e ./web -run '^TestLiveAPIScheduledQueryNotificationDetails$' -count=1 -v
```

## 8. 分布式节点不可用

分布式接口只在 `distributed.enabled=true` 时可用；否则返回 `distributed_disabled`。没有单独的 `unimap-node` 可执行文件，节点应通过现有 HTTP 协议集成。

```powershell
Invoke-RestMethod http://127.0.0.1:8448/api/v1/nodes/status -Headers @{ 'X-Admin-Token' = $env:UNIMAP_DISTRIBUTED_ADMIN_TOKEN }
Invoke-RestMethod http://127.0.0.1:8448/api/v1/nodes/network/profile -Headers @{ 'X-Admin-Token' = $env:UNIMAP_DISTRIBUTED_ADMIN_TOKEN }
```

注册、心跳、领取和结果回传应使用对应节点令牌；状态、任务队列管理使用分布式管理令牌。

## 9. 备份、配置或历史记录

- 配置读取/保存：`GET`/`POST /api/v1/config`，需要管理员。
- 备份：`POST /api/v1/backup/create`、`GET /api/v1/backup/list`。
- 操作历史：`POST /api/v1/history/save`、`GET`/`DELETE /api/v1/history`，需要管理员。

备份文件和本地配置可能包含敏感数据；限制文件系统权限，并通过受控部署流程恢复。

## 10. 端口扫描异常或全端口未完成

- Web 页支持“常用端口”“自定义端口”“全端口”三种模式。自定义表达式示例：`22,80,443,8000-8100`；调度任务使用同样的 `port_spec`，或以 `scan_mode: full` 扫描 1-65535。
- 全端口扫描默认端口并发 256、TCP 连接超时 800ms、单目标总超时 300 秒。高丢包网络可适当增大连接/总超时；目标较多时不要同时把目标并发和端口并发调到最大。
- 结果中的 `attempted_connections` 小于 `expected_connections` 表示扫描被总超时或请求取消中断；已发现的开放端口仍会保留。若经常超时，先降低目标并发，再提高 `scan_timeout_seconds`（最大 900 秒）。
- `blocked` 表示目标解析为 loopback、私有或内部地址，属于 SSRF 防护；`cdn_excluded` 表示检测到 CDN，属于避免扫描共享边缘节点的安全策略。不要通过关闭这些检查来排障。
- 端口扫描执行真实 TCP connect，不识别 UDP 服务，也不进行服务版本探测。请只扫描已获授权的公网资产。

## 11. 变更后检查清单

```powershell
go test -race ./...
```

随后至少验证 `/health`、受影响的 `/api/v1/...` 路由，以及相关 UI 流程。修改路由、认证或 Bridge 协议时，同步更新 [API 文档](API.md) 和本 Runbook。
