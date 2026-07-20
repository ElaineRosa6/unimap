# UniMap 运维 Runbook

> 最后按代码核对：2026-07-20。所有业务 API 使用 `/api/v1/...`；旧 `/api/...` 路径已移除。

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

### 发布前审计门槛（2026-07-15）

2026-07-15 审计的 12 项问题已在当前工作区处理；2026-07-16 补修后为 11 项完整修复、1 项产品缓解（未实现的额度趋势/告警入口保持禁用），状态见 [`docs/AUDIT_REMEDIATION_GUIDE.md`](AUDIT_REMEDIATION_GUIDE.md)。发布前仍必须保留以下复验门槛：

- 危险截图 ID 不得在截图根目录外创建文件；同步和异步 handler 必须在任务创建前返回 400；
- 重复批次 ID 必须同时检查内存与 SQLite，内存清理或服务重启后仍返回 409，不得覆盖历史记录；
- 配置保存失败不改变当前内存/运行时配置；
- 调度和备份持久化失败不得报告成功；临近触发的一次性任务在保存失败后也不得执行；
- readiness 必须逐个验证所有已启用引擎，并通过加锁快照读取配置；公开检查响应不得暴露底层数据库错误；
- 未配置 `web.rate_limit.trusted_proxy_cidrs` 时忽略全部转发头；
- Shodan 跨字段 OR 与 ZoomEye 比较操作符返回明确能力错误。

每次修复后保存对应测试输出，并重新运行：

```powershell
go test ./...
go vet ./...
go test -race ./...
```

默认测试层不得启动 Chrome/Edge 或产生可见浏览器窗口。真实浏览器测试使用独立 build tag：

```powershell
$env:UNIMAP_CHROME_PATH = 'C:\Program Files\Google\Chrome\Application\chrome.exe'
go test -tags headless_e2e -run TestHeadlessChromeExecutesJavaScriptAndCapturesPNG ./internal/screenshot
go test -tags headless_e2e -run 'TestRelaxed_|TestStrict_MD5Change|TestNormalDynamic' ./internal/tamper
```

`headless_e2e` 会短暂产生多个 Chrome renderer/GPU/network 子进程，这是 Chrome 的正常多进程模型；测试结束后不得有新增进程残留。

不要把现有审计报告中的 `P0` 自动扫描计数当作已确认漏洞；测试占位密钥、固定文案 `innerHTML` 和历史归档脚本已经在人工审查中去重。

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

### 无图形界面的 Linux / 云主机

CDP 模式使用 Chrome/Chromium 自带的新版 headless，不依赖桌面环境、`DISPLAY`、X11 或 VNC。

> **B-01 已修复（2026-07-20）**：`Dockerfile` 和 CI 均已改为 `CGO_ENABLED=1` 并安装 Alpine `build-base`。本地 `go test ./internal/auth ./internal/history ./internal/screenshot/batchdb` 验证通过。容器镜像内 SQLite 与 headless 闭环仍需在正式云机执行验收（见 [云服务器部署评估](CLOUD_DEPLOYMENT_ASSESSMENT_2026-07-20.md)）。

修复该阻断后的容器启动基线是：

```bash
# 默认使用 configs/config.docker.yaml；自定义时设置 UNIMAP_CONFIG_FILE
# 云主机建议 screenshot.mode=cdp、headless=true、max_sessions=1
export UNIMAP_BOOTSTRAP_PASSWORD='使用密码管理器生成的随机长密码'
docker compose up -d --build
curl --fail http://127.0.0.1:8448/health/ready
```

Compose 通过专用的 `UNIMAP_CONTAINER_BIND_ADDRESS` 显式切换为 `0.0.0.0`，并要求 `UNIMAP_BOOTSTRAP_PASSWORD`；后者只在启动配置阶段用于生成 bcrypt 哈希，不写回配置或日志。使用完整自定义配置时执行 `UNIMAP_CONFIG_FILE=./configs/config.yaml docker compose up -d`。若不使用 Compose，镜像基线保持 loopback，仅可通过容器内检查或自行提供安全的公开监听配置访问。

生产部署还必须设置固定管理令牌和非默认管理员用户名；只启用拥有有效凭据的查询引擎。缺少 API Key 时注册的 Web-only adapter 并不代表真实查询已经可用，必须执行一次真实查询或浏览器采集验收。配置以 `:ro` 挂载时，Web 设置页不能持久化修改，应通过声明式配置更新并重启。

镜像内置 Chromium 和中日韩字体，固定 `UNIMAP_CHROME_PATH=/usr/bin/chromium`，并持久化 `/app/data`、`/app/screenshots`、`/app/chrome-profile`。Compose 把 `/dev/shm` 提高到 256 MiB；容器基线显式设置 `no_sandbox: true`，普通主机应保持 false 以使用 Chrome sandbox。不得删掉 `--disable-dev-shm-usage` 或独立 `user-data-dir`。持久化 Chrome profile 有独占锁，程序会把其并发会话自动限制为 1；不使用固定 profile 时可按内存逐步提高 `screenshot.max_sessions`。

现有 Compose 的 2 CPU / 1 GiB 限制不是已验收的生产容量。单机完整功能建议从 4 vCPU、8 GiB RAM、80–100 GiB SSD 和 2–4 GiB swap 起步，容器内存限制从 4–6 GiB 起步，并按真实批量负载调整。生产还应删除 `./web:/app/web` 开发挂载、仅向反向代理暴露 8448、持久化 `/app/backups`、配置日志轮转并把备份复制到异机位置。

非容器部署至少确认：

```bash
command -v google-chrome || command -v chromium || command -v chromium-browser
export UNIMAP_CHROME_PATH=/usr/bin/chromium
export UNIMAP_DATA_DIR=/var/lib/unimap
export UNIMAP_CHROME_USER_DATA_DIR=/var/lib/unimap/chrome-profile
unset DISPLAY
```

`/health/live` 只证明进程存活；部署流量必须以 `/health/ready` 为准。截图启用时，readiness 会按配置模式判断真正可用的后端：强制 `extension` 且没有在线扩展时不会因为本机有 Chrome 就误报就绪，除非明确开启 fallback。`/api/v1/screenshot/router/status` 同时返回 `configured_mode`、`current_mode`、`ready` 和两个后端健康状态。

Windows 上 readiness 对 Chrome/Edge 使用 PE 静态验证，不执行 `chrome.exe --version`。后者会被已运行的 Chrome 转交给用户会话，可能打开或激活可见窗口。Router 未配置 CDP provider 时完全跳过 CDP 探针。

本地无法替代真实云机的最终验收项包括：目标发行版的 Chromium 包、容器运行时、出站 DNS/TLS、机器内存和供应商安全组。拿到正式测试机后，再执行镜像启动、ready、单 URL PNG、批量任务、浏览器采集、巡检和重启后持久化验收。

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

POST 返回 202 只表示任务已接受。CLI/GUI 会继续轮询；手工调用时必须保存 `job_id` 并查询到 `completed`/`failed`。浏览器端等待超时会显示明确的超时终态和 `job_id`，但不会取消服务端任务；拿到 `job_id` 后遇到断网或超时，不要本地重跑同一批 URL，以免产生重复截图，恢复后应继续查询原任务进度。`persistence_error` 表示截图终态完成但持久化降级。

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

通知通道列表只返回编辑所需的非凭据字段，不回传 Webhook URL、签名 secret 或 app secret。编辑既有通道时应提交 `preserve_existing=true`，并将不修改的凭据留空；不要把页面掩码值重新提交。该模式只允许编辑同 ID、同类型通道，渠道类型变化必须删除旧通道后重新创建。保存后再执行测试接口，确认服务端实际配置可投递。

任务列表的 `enabled=true` 表示期望启用；还要检查 `runtime_status`。`schedule_error` 表示加载或布置失败，具体诊断在同名错误字段中。删除、停用持久化失败会返回 500 并回滚内存调度状态，不应按 404 处理。

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

`max_reassign=N` 表示首次分配之外最多重新分配 N 次；离线、租约过期和 retryable 失败都会消耗次数。队列快照写失败时入队、认领、结果提交和删除不会返回成功，后台回收会恢复旧状态并记录错误。

## 9. 备份、配置或历史记录

- 配置读取/保存：`GET`/`POST /api/v1/config`，需要管理员。
- 备份：`POST /api/v1/backup/create`、`GET /api/v1/backup/list`。
- 操作历史：`POST /api/v1/history/save`、`GET`/`DELETE /api/v1/history`，需要管理员。

配置保存响应包含 `persisted`、`applied` 和 `restart_required`。查询响应的 `persistence.status` 为 `persisted`、`failed` 或 `disabled`；批量截图任务可能包含 `persistence_error`。反向代理部署必须把直接代理网段加入 `web.rate_limit.trusted_proxy_cidrs`，直连部署保持空列表。

所有配置写入口采用候选副本提交：只有 `SaveConfig` 成功后才发布并执行运行态刷新。保存失败时当前 Manager 和运行态保持旧值。`restart_required=true` 时不要仅凭“保存成功”判断已热生效。

多用户数据库启动时会幂等迁移 `session_version`。禁用、删除或改密后旧会话下一请求应为 401；用户库故障时会话请求为 503，但管理令牌仍可作为运维恢复入口。首次管理员使用数据库条件写入，多个并发公开注册最多一个成功。

备份文件和本地配置可能包含敏感数据；限制文件系统权限，并通过受控部署流程恢复。

## 10. 端口扫描异常或全端口未完成

- Web 页支持“常用端口”“自定义端口”“全端口”三种模式。自定义表达式示例：`22,80,443,8000-8100`；调度任务使用同样的 `port_spec`，或以 `scan_mode: full` 扫描 1-65535。
- 多个目标会先解析并按公网 IPv4 全局去重，再将 `唯一 IP × 端口` 笛卡尔积随机打乱。选择多个 `probe_methods` 时，进度按 `IP × 端口 × 方法` 计数；多个域名指向同一 IP 不会重复探测。
- 如需强制限定授权范围，在 Web 或调度任务填写 `authorized_targets`（IPv4/CIDR 列表）。任一解析 IP 超出清单时，整个目标标记为 `not_authorized`，不进行部分扫描。留空仅表示操作者自行确认授权，不代表系统能够证明资产所有权。
- 全端口扫描默认全局端口并发 256、TCP 连接超时 800ms、扫描计划总超时 300 秒。高丢包网络可适当增大连接/总超时；目标较多时不要同时把目标解析并发和端口并发调到最大。
- 结果中的 `attempted_connections` 小于 `expected_connections` 表示扫描被总超时或请求取消中断；已发现的开放端口仍会保留。若经常超时，先降低目标并发，再提高 `scan_timeout_seconds`（最大 900 秒）。
- `blocked` 表示目标解析为 loopback、私有或内部地址，属于 SSRF 防护；`cdn_excluded` 表示检测到 CDN，属于避免扫描共享边缘节点的安全策略。不要通过关闭这些检查来排障。
- `connect` 执行完整 TCP 握手；`telnet` 在连接后发送 IAC 协商；`udp` 使用常见服务载荷并区分 `open` 与 `open_filtered`。`fin`、`null`、`xmas` 分别发送 FIN、无标志、FIN/PSH/URG TCP 段，必须填写授权范围并以管理员/root 或 `CAP_NET_RAW` 权限运行。
- UDP/FIN/NULL/Xmas 的“无响应”不能证明端口开放，结果会显示为 `open_filtered`，只有确定响应才进入 `open_ports`。防火墙、NAT、主机 TCP 栈差异都会影响原始扫描结果，建议与 connect/Telnet 混合复核。
- `jitter_min_ms` / `jitter_max_ms` 控制每次探测前的随机延迟（最大 5000ms）。抖动会显著增加全端口扫描耗时，需同步增大 `scan_timeout_seconds`。

## 11. 变更后检查清单

```powershell
go test -race ./...
```

随后至少验证 `/health`、受影响的 `/api/v1/...` 路由，以及相关 UI 流程。修改路由、认证或 Bridge 协议时，同步更新 [API 文档](API.md) 和本 Runbook。
