# UniMap HTTP API

> 最后按代码核对：2026-07-17。路由的唯一事实来源是 `web/router.go`；handler 的请求/响应细节以对应 `web/*_handlers.go` 为准。

## 约定

- 所有业务 API 均以 `/api/v1` 为前缀。旧 `/api/...` 路径已于 2026-06-09 移除，不能使用。
- 页面、健康检查、指标和图片预览不使用该前缀：`/health`、`/health/ready`、`/health/live`、`/metrics`、`/screenshots/...`。
- 启用 Web 认证时，请按部署配置提供会话、API Key 或管理令牌。节点接口另有节点令牌/分布式管理令牌要求。
- CLI API 子命令按 `--admin-token-file`、`UNIMAP_ADMIN_TOKEN`、`--admin-token` 的顺序读取管理令牌；推荐 token 文件或环境变量。CLI 拒绝把带认证的请求重定向到不同 origin。
- 修改性 JSON 请求会校验同源/受信任 Origin；命令行调用应从被允许的来源执行，或按部署的认证与 CORS 配置处理。
- 成功响应**没有统一信封**：有的 handler 直接返回业务对象，有的返回 `{ "success": true, ... }`。错误响应统一为：

```json
{
  "success": false,
  "error": {"code": "machine_readable_code", "message": "safe message", "details": {}}
}
```

## 健康、查询与会话

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/health` | 综合健康状态 |
| GET | `/health/ready` | 就绪状态 |
| GET | `/health/live` | 进程存活状态 |
| GET | `/metrics` | Prometheus 指标；认证开启时需要管理令牌，未认证的非 loopback 部署会拒绝访问 |
| POST | `/api/v1/login` | 登录 |
| POST | `/api/v1/logout` | 登出 |
| POST | `/api/v1/query` | API 查询；使用表单字段，不是 JSON 请求体 |
| GET | `/api/v1/query/status?query_id=...` | 查询状态 |
| GET | `/api/v1/ws` | WebSocket 查询通道 |
| POST | `/query` | 页面表单查询 |

### API 查询

`POST /api/v1/query` 接受 `application/x-www-form-urlencoded` 或 `multipart/form-data`。主要字段为：

| 字段 | 说明 |
|---|---|
| `query` | 必填，UQL 查询字符串 |
| `engines` | 逗号分隔的引擎列表；未指定时使用可用的稳定引擎 |
| `page_size` | 可选，默认 50，最大 500 |
| `browser_query` | 可选布尔值，是否同时走浏览器采集 |
| `browser_action` | 可选浏览器动作 |

响应为 `QueryAPIPayload`，核心字段是 `query`、`engines`、`assets`、`totalCount`、`engineStats`、`errors`、`persistence`，并可能附带浏览器采集状态。缓存使用版本化 key 并保存 `engineStats`/`errors` 元数据，因此首次响应与缓存命中的统计语义一致。`persistence.status` 为 `persisted`、`failed` 或 `disabled`。`GET /query` 是页面跳转入口，不是等价的 JSON 查询接口。

## Cookie 与 CDP

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/cookies` | 保存 Cookie |
| POST | `/api/v1/cookies/verify` | 验证 Cookie |
| POST | `/api/v1/cookies/import` | 导入 Cookie JSON |
| GET | `/api/v1/cookies/login-status` | 各引擎登录状态 |
| GET | `/api/v1/cdp/status` | CDP 状态 |
| POST | `/api/v1/cdp/connect` | 连接 CDP |

## 截图

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/screenshot` | 单 URL 截图，JSON：`{"url":"https://example.com"}`；成功返回 PNG 数据 |
| GET | `/api/v1/screenshot/search-engine` | 参数：`engine`、`query`、可选 `query_id` |
| POST | `/api/v1/screenshot/target` | JSON：`url` 或 `ip`，可选 `port`、`protocol`、`query_id` |
| POST | `/api/v1/screenshot/batch` | 同步批量截图；JSON 含 `query_id`、`engines:[{engine,query}]`、`targets:[{url,ip,port,protocol}]` |
| POST | `/api/v1/screenshot/batch-urls` | 异步 URL 批量截图；JSON：`urls`、可选 `batch_id`、`concurrency` |
| GET | `/api/v1/screenshot/batch/progress?job_id=...` | 异步批次进度 |
| GET | `/api/v1/screenshot/batches` | 截图批次列表 |
| GET | `/api/v1/screenshot/batches/files?batch=...` | 某批次的文件列表；参数名是 `batch` |
| DELETE | `/api/v1/screenshot/batches/delete?batch=...` | 删除批次 |
| DELETE | `/api/v1/screenshot/file/delete?batch=...&file=...` | 删除批次中的文件 |
| GET | `/screenshots/{batch}/{file}` | 图片预览；要求受信任 Origin/Referer，且仅允许图片扩展名 |
| GET | `/api/v1/screenshot/router/status` | 截图路由与健康状态 |
| POST | `/api/v1/screenshot/set-mode` | JSON：`{"mode":"cdp|extension|auto"}` |

启动成功返回 202 和 `{job_id,total,status}`，调用方必须轮询进度直到 `completed` 或 `failed`，不能把 202 当作完成。重复 `batch_id` 返回 409；创建阶段无法持久化任务返回 503。完成响应可能包含 `persistence_error`，表示截图已完成但结果持久化降级。

非空的自定义 `query_id` / `batch_id` 不能是 `.`、`..`，也不能包含 `/` 或 `\`。handler 会在任务创建前拒绝非法值并返回 400：分别使用 `invalid_query_id` 与 `invalid_batch_id`；省略可选 ID 或传空字符串时由服务生成安全 ID。

URL、目标截图和批量 URL 路径会拒绝私有、回环与内部地址；不要将其当作内网探测接口。

## Screenshot Extension Bridge

Bridge 路由仅以 `/api/v1` 提供。配对、任务拉取、回调和令牌轮换限制为 loopback 请求；若开启配对，还需要 `Authorization: Bearer <bridge-token>`（loopback 下管理令牌可用于恢复）。不存在 `/diagnostic` 端点，使用 health/status 即可。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/screenshot/bridge/health` | loopback 返回诊断快照；远程仅返回最小健康信息 |
| GET | `/api/v1/screenshot/bridge/status` | 同上 |
| POST | `/api/v1/screenshot/bridge/pair` | loopback JSON：`client_id`、`pair_code`；返回短期 bridge token |
| POST | `/api/v1/screenshot/bridge/token/rotate` | loopback，JSON 可含 `revoke_old` |
| GET | `/api/v1/screenshot/bridge/tasks/next` | loopback 拉取下一个任务 |
| POST | `/api/v1/screenshot/bridge/mock/result` | loopback 回调任务结果；配置要求时还需签名头 |

## URL 导入与巡检

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/import/urls` | 导入 URL |
| POST | `/api/v1/url/reachability` | URL 可达性检测 |
| POST | `/api/v1/url/port-scan` | 公网 URL/IP 端口扫描；支持 TCP connect、Telnet、FIN/NULL/Xmas、UDP、混合模式和随机抖动 |
| POST | `/api/v1/url/probe-web` | Web 服务探测 |
| POST | `/api/v1/url/probe-web-batch` | 批量 Web 服务探测 |
| POST | `/api/v1/tamper/check` | JSON：`urls`、可选 `concurrency`、`mode`；模式为 `strict`、`relaxed`、`security`、`balanced`、`precise` |
| POST | `/api/v1/tamper/baseline` | JSON：`urls`、可选 `concurrency` |
| GET | `/api/v1/tamper/baseline/list` | 基线 URL 列表 |
| DELETE | `/api/v1/tamper/baseline/delete?url=...` | 删除单个基线 |
| GET | `/api/v1/tamper/history` | 可选 `limit`（最大 1000）、`offset`（最大 100000）、`url`、`type`、`mode`、`q` |
| GET | `/api/v1/tamper/history/export` | 可选 `limit`（最多 10000）、`url`、`type`、`mode`、`q`；下载 JSON |
| DELETE | `/api/v1/tamper/history/delete?url=...` | 删除一个 URL 的巡检历史 |

当前 HTTP 历史接口支持受限的 `offset` 分页；尚未公开 `start_time` 或 `end_time` 参数，不要依赖这两个参数。

### URL 端口扫描

`POST /api/v1/url/port-scan` 的 `scan_mode` 为 `common`、`custom` 或 `full`；`port_spec` 支持逗号分隔的端口和闭区间（如 `22,80,443,8000-8100`），也支持 `all` / `full`。旧版 `ports: [80,443]` 仍兼容。

```json
{
  "targets": ["https://example.com", "203.0.113.10"],
  "authorized_targets": ["203.0.113.0/24", "198.51.100.20"],
  "scan_mode": "custom",
  "port_spec": "22,80,443,8000-8100",
  "concurrency": 3,
  "port_concurrency": 256,
  "connect_timeout_ms": 800,
  "scan_timeout_seconds": 60,
  "probe_methods": ["connect", "telnet", "udp"],
  "jitter_min_ms": 10,
  "jitter_max_ms": 80
}
```

`targets` 接受 URL、域名或公网 IPv4；旧字段 `urls` 继续兼容。`authorized_targets` 是可选的 IPv4/CIDR 清单：填写后，一个目标解析出的每个 IP 都必须位于清单内，否则该目标状态为 `not_authorized` 且不会进入连接计划。留空表示操作者已确认所有输入目标均获授权。授权清单不会放宽私有地址、loopback、link-local 等 SSRF 限制。

全端口可使用 `"scan_mode":"full"`，默认扫描计划总超时为 300 秒。服务端先完成解析和安全判断，再对所有合格目标构造去重的 `唯一 IP × 端口` 笛卡尔积并随机打乱，以全局有界队列执行。`probe_methods` 可包含 `connect`、`telnet`、`fin`、`null`、`xmas`、`udp`；省略时默认为 `connect`。`jitter_min_ms` / `jitter_max_ms` 在每次发包前加入 0-5000ms 的随机延迟。响应中的 `planned_connections` 和 `attempted_connections` 按“IP × 端口 × 方法”计数。

`findings` 保留每个 IP 的方法、协议和状态。TCP connect/Telnet 完成连接或 UDP 收到响应时状态为 `open`，并汇总进兼容字段 `open_ports`；UDP 无响应以及 FIN/NULL/Xmas 未收到 RST 时只能判定为 `open_filtered`，不会误报成确定开放。FIN/NULL/Xmas 使用原始 IPv4 TCP 套接字，调用方必须填写 `authorized_targets`，运行进程还需要管理员/root 或 `CAP_NET_RAW` 权限；不满足时返回扫描未完成及具体原因。

端口扫描仅允许公网目标。解析到 loopback、私有或内部地址会返回/记录 `blocked`，检测到 CDN 的目标会记录为 `cdn_excluded`；全端口模式不会放宽这些安全边界。

## 调度、通知与备份

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/scheduler/tasks` | 任务列表 |
| GET | `/api/v1/scheduler/tasks/get?id=...` | 单任务 |
| POST | `/api/v1/scheduler/tasks/create` | 创建任务 |
| POST | `/api/v1/scheduler/tasks/update` | 更新任务 |
| POST | `/api/v1/scheduler/tasks/delete` | 删除任务 |
| POST | `/api/v1/scheduler/tasks/run` | 立即执行 |
| POST | `/api/v1/scheduler/tasks/enable` | 启用 |
| POST | `/api/v1/scheduler/tasks/disable` | 停用 |
| GET | `/api/v1/scheduler/history` | 执行历史；可选 `task_id`、`task_type`、`status`、`limit` |
| GET/POST/DELETE | `/api/v1/notifications/channels` | 列出、保存、删除通知通道 |
| POST | `/api/v1/notifications/channels/test` | 测试通道 |
| POST | `/api/v1/notifications/reload` | 重载通知配置 |
| POST | `/api/v1/backup/create` | 创建备份 |
| GET | `/api/v1/backup/list` | 备份列表 |

调度器任务请求的权威字段是 `web/scheduler_handlers.go` 中的创建/更新结构：`name`、`type`、`enabled`、`cron_expr`、`payload`、`timeout_seconds`、`max_retries`，以及可选 `notifications`、`schedule_type`、`run_at`、`delay_seconds`。任务响应额外包含只读 `runtime_status`（`scheduled`、`disabled`、`schedule_error`）和可选 `schedule_error`；`enabled` 仍表示用户期望，不代表任务一定已成功布置。当前工作区定义 23 种任务类型；其中备份任务属于未提交工作区改动，发布说明应随提交状态更新。

### 通用规则

- Payload 字段推荐放在**顶层**；为兼容旧任务，Runner 仍会读取 `extra` 中的任务字段。
- 数组字段优先使用 JSON 数组；字符串形式会按逗号拆分并去除首尾空白。
- `urls` 兼容旧别名 `targets`，`engines` 兼容单数别名 `engine`；新任务应使用规范字段名。
- 创建或更新任务时会按 Runner 类型校验必填字段；400 错误会包含任务类型、规范字段名和可用别名。

### 常见陷阱

| Runner | 错误用法 | 正确用法 |
|--------|----------|----------|
| `port_scan` | 旧格式 `{"targets": "https://a.test,https://b.test", "ports": "22,80"}` | 推荐 `{"urls": ["https://a.test", "https://b.test"], "ports": ["22", "80"], "concurrency": 50}` |
| `query` | 兼容 `{"engine": "fofa", "query": "..."}` | 推荐 `{"engines": ["fofa"], "query": "...", "page_size": 20}` |
| `tamper_check` | `{"url": "..."}`（不兼容的单数字段） | `{"urls": ["..."], "detection_mode": "relaxed"}` |

### 各 Runner Payload 速查

#### query
```json
{
  "query": "ip=\"1.2.3.4\"",
  "engines": ["fofa", "hunter"],
  "page_size": 20,
  "browser_query": false,
  "browser_action": "collect_and_capture",
  "query_id": "optional-correlation-id",
  "notification_detail_limit": 50
}
```

#### port_scan
```json
{
  "urls": ["http://example.com"],
  "ports": ["22", "80", "443"],
  "concurrency": 50,
  "scan_mode": "common",
  "port_spec": "22,80,443,8000-8100",
  "probe_methods": ["connect"],
  "jitter_min_ms": 10,
  "jitter_max_ms": 80
}
```

#### tamper_check
```json
{
  "urls": ["http://example.com"],
  "concurrency": 5,
  "mode": "relaxed"
}
```

#### icp_query
```json
{
  "queries": ["[REDACTED]"],
  "type": "web",
  "page": 1,
  "page_size": 40
}
```

#### url_reachability
```json
{
  "urls": ["http://example.com"],
  "concurrency": 10
}
```

#### backup
```json
{
  "sources": ["baseline", "config", "cookies"],
  "output_dir": "",
  "prefix": "unimap",
  "max_backups": 7
}
```

#### export
```json
{
  "query": "ip=\"1.2.3.4\"",
  "engines": ["fofa"],
  "page_size": 100,
  "format": "json",
  "output_file": ""
}
```

`query` 任务的基础 payload 为 `query`、`engines`、`page_size`。查询成功通知会展开资产明细，而不是只发送数量；`notification_detail_limit` 控制通知中最多展开多少条，默认 50、最大 100。明细正文另有约 20 KiB 的渠道安全上限。超过任一上限的资产仍全部写入 SQLite，通知会标明未展开数量。通知不会包含响应头、正文片段或任意扩展字段。

需要完整 Bridge 闭环时增加：

```json
{
  "query": "port=\"443\"",
  "engines": ["fofa"],
  "page_size": 10,
  "notification_detail_limit": 50,
  "browser_query": true,
  "browser_action": "collect_and_capture",
  "query_id": "optional-correlation-id"
}
```

该模式对每个引擎执行一次 Bridge 结构化采集与截图，将 API 和 Bridge 资产合并为一条 SQLite 查询历史，并把合并后的资产明细和本地 PNG 路径交给任务通知。它要求历史数据库、Bridge provider 和截图文件均可用；任一必需环节失败时任务不会报告成功。未设置 `browser_query` 的查询任务保持普通 API 查询行为，但成功通知同样包含 API 资产明细。

## 分布式节点

所有节点接口仅在 `distributed.enabled=true` 时可用。注册、心跳、领取与回传使用节点令牌；状态与队列管理使用分布式管理令牌。

| 方法 | 路径 |
|---|---|
| POST | `/api/v1/nodes/register` |
| POST | `/api/v1/nodes/heartbeat` |
| GET | `/api/v1/nodes/status` |
| GET | `/api/v1/nodes/get?node_id=...` |
| DELETE | `/api/v1/nodes/deregister?node_id=...` |
| GET | `/api/v1/nodes/network/profile` |
| POST | `/api/v1/nodes/task/enqueue` |
| POST | `/api/v1/nodes/task/claim` |
| POST | `/api/v1/nodes/task/result` |
| GET | `/api/v1/nodes/task/status` |
| GET | `/api/v1/nodes/task/get?task_id=...` |
| DELETE | `/api/v1/nodes/task/delete?task_id=...` |

## 配置、账户、用户、ICP 与操作历史

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/account/change-password` | 修改当前账户密码 |
| GET | `/api/v1/account/admin-token` | 获取管理令牌；多用户模式需管理员 |
| POST | `/api/v1/users/register` | 注册用户 |
| GET | `/api/v1/users` | 用户列表 |
| GET/PUT/DELETE | `/api/v1/users/{id}` | 读取、更新、删除用户 |
| POST | `/api/v1/users/{id}/password` | 修改用户密码 |
| GET/POST | `/api/v1/config` | 读取、保存配置；需管理员 |

数据库用户会话在每次受保护请求上校验用户存在、active 状态和 `session_version`。禁用、删除或修改密码后，旧 Cookie 的下一次请求会返回 401；用户数据库不可用时返回 503。legacy 单用户会话与管理令牌认证不读取用户表。
| POST | `/api/v1/history/save` | 保存操作历史；需管理员 |
| GET/DELETE | `/api/v1/history` | 列出、清空操作历史；需管理员 |
| GET/DELETE | `/api/v1/history/{id}` | 读取、删除操作历史；需管理员 |
| GET | `/api/v1/icp/health` | |
| GET | `/api/v1/icp/query` | `type`、`search`、`page`、`page_size`；`search` 必填且最大 256 字符 |
| GET | `/api/v1/icp/history` | `keyword` 支持部分关键词匹配，`type` 默认 `web`；`task_id` 可选且保持精确匹配 |
| GET | `/api/v1/icp/history/results` | `run_id` 必填，返回该次查询的明细结果 |
| GET | `/api/v1/icp/compare` | |

## 变更规则

新增或修改路由时，必须同时更新本文件、对应 handler 测试，以及调用方。禁止重新引入 `/api/...` 旧路径 shim。
