# UniMap Screenshot Extension

Chrome Manifest V3 扩展，为 UniMap 提供浏览器页面截图与结构化采集。

> 当前 manifest 版本：0.4.2。服务端协议以 `web/router.go` 和 `web/screenshot_bridge_handlers.go` 为准。

## 安装与配置

1. 在 Chrome 打开 `chrome://extensions`，开启开发者模式。
2. 选择“加载已解压的扩展程序”，指向本目录。
3. 在扩展设置中把 API 地址设为本机 UniMap，例如 `http://127.0.0.1:8448`。
4. 服务与扩展在同一台机器时，使用配对码完成 Bridge 配对。

默认地址可在扩展控制台调整：

```js
chrome.storage.local.set({ apiBaseURL: "http://127.0.0.1:8448" })
```

## Bridge API

所有 Bridge API 使用 `/api/v1` 前缀。旧 `/api/...` 路径和 `/diagnostic` 端点均不存在。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/screenshot/bridge/health` | 本机健康/诊断；远程仅返回最小状态 |
| GET | `/api/v1/screenshot/bridge/status` | 本机状态/诊断 |
| POST | `/api/v1/screenshot/bridge/pair` | 本机配对，JSON：`client_id`、`pair_code` |
| GET | `/api/v1/screenshot/bridge/tasks/next` | 拉取任务 |
| POST | `/api/v1/screenshot/bridge/mock/result` | 回传任务结果 |
| POST | `/api/v1/screenshot/bridge/token/rotate` | 本机轮换 bridge token |

配对、任务拉取、结果回调和令牌轮换限制为 loopback 请求。启用 pairing 时，任务拉取、回调和轮换需携带：

```text
Authorization: Bearer <bridge-token>
```

## 支持的动作与引擎

Bridge 任务动作：

- `collect`：采集结构化资产字段。
- `screenshot`：采集截图。
- `collect_and_capture`：同一次任务中采集字段并截图。

源码识别 FOFA、Hunter、ZoomEye、Quake、Shodan、Censys、DayDayMap。稳定 Web UI 当前展示全部七个引擎；缺少 API 凭据时走 Web-only adapter。

## 调试

服务端状态：

```text
GET /api/v1/screenshot/bridge/health
GET /api/v1/screenshot/bridge/status
GET /api/v1/screenshot/router/status
```

查看 Chrome 扩展的 service worker 控制台，检查 API 地址、配对码、token 到期时间和请求状态。不要把 token、配对码或页面 Cookie 复制到 issue、聊天记录或日志。

关于运维、回滚和回调签名，参见 [截图扩展运维说明](../../docs/OPS_SCREENSHOT_EXTENSION.md)。
