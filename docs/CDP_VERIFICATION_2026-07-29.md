# Quake / Hunter CDP 真实验收证据

> 日期：2026-07-29
> 关联待办：RW-02（Quake/Hunter 生产配置与真实 CDP 验收）
> Chrome：152.0.7967.2（本地可见窗口，CDP 远程调试端口 9222）
> 用户数据目录：`data/chrome-cdp-profile`（会话持久化）

## 验收方法

1. 通过 `--remote-debugging-port=9222` 启动可见 Chrome，用户手动登录 Quake 和 Hunter。
2. 通过 CDP `Storage.getCookies` 捕获全部 Cookie 并写入 `configs/config.yaml`。
3. 使用应用代码路径 `Manager.CollectAndCaptureSearchEngineResult` 对两个引擎执行：
   - CDP 导航到搜索结果页（使用 `BuildSearchEngineURL` 构造 URL）
   - L1 Network 拦截 + L3 DOM 提取结构化资产
   - 截图（PNG 优先；JPEG 捕获回退会转码为 PNG）
4. 人工检查截图确认非登录页、非验证码页。

## Quake 验收结果

| 步骤 | 结果 | 证据 |
|---|---|---|
| Cookie 登录状态 | ✅ 已登录 | SSO 登录态有效；证据不记录用户标识或 Cookie 值 |
| CDP 打开搜索结果页 | ✅ 成功 | URL: `quake.360.net/quake/#/searchResult?searchVal=...` |
| 非空结构化资产 | ✅ 10 条 | 首条: IP=d18x4atlrp7sal.amplifyapp.com Port=80 Proto=http |
| 搜索结果数 | ✅ 206,493,526 条 | 41,302,478 独立 IP，耗时 6.355s |
| 截图 | ✅ 374,962 bytes (PNG) | `data/cdp-verify/app-screenshots/2026-07-29-verify_quake/` |
| 截图人工检查 | ✅ 搜索结果表格 | 含 IP/端口/服务/协议/地区列，非登录页 |
| LoginWall | ✅ false | 未检测到登录墙 |

## Hunter 验收结果

| 步骤 | 结果 | 证据 |
|---|---|---|
| Cookie 登录状态 | ✅ 已登录 | 登录态有效；证据不记录用户标识或 Cookie 值 |
| CDP 打开搜索结果页 | ✅ 成功 | URL: `hunter.qianxin.com/home/list?search=<base64>` |
| 非空结构化资产 | ✅ 10 条 | IP 地址已提取（Port/Proto 选择器需校准，见下方说明） |
| 搜索结果数 | ✅ 26,960,109 条 | 7,027,992 独立 IP |
| 截图 | ✅ 历史文件 245,824 bytes | 当时为 JPEG 内容配 `.png` 扩展名；该契约缺陷已在本轮修复 |
| 截图人工检查 | ✅ 搜索结果表格 | 含 IP/端口/域名/协议/地区列，非登录页 |
| LoginWall | ✅ false | 未检测到登录墙 |

## 截图回退修复

Hunter 页面复杂度导致 Chrome PNG 合成器挂起。已在 `internal/screenshot/screenshot_fallback.go`
`captureScreenshotWithFallback` 先尝试 PNG（10s 超时）；失败时用 JPEG（30s、85%）完成捕获，
随后在内存中转码为 PNG。新生成文件、扩展名和 HTTP MIME 统一为 PNG。
`CaptureScreenshotWithProxy` 和 `CollectAndCaptureSearchEngineResult` 均已接入。

## 已知限制

1. **Hunter DOM 选择器**：当前 `dom_selectors.go` 中 Hunter 的端口/协议提取选择器与 2026-07-29
   页面结构不完全匹配，导致 Port=0、Proto=""。IP 和 Host 提取正常。需在 P2 阶段校准。
2. **SQLite 持久化**：本次验证通过 `CollectAndCaptureSearchEngineResult` 直接调用，未经过
   `ExecuteQueryWithBrowserWorkflow` 的 SQLite 合并路径。该路径由现有单元测试覆盖。
3. **图片通知**：需启动完整 Web 服务并通过调度任务触发，本次未执行。
4. **容器重启恢复**：用户数据目录 `data/chrome-cdp-profile` 已持久化 Cookie/Session，
   重启 Chrome 后登录态保持。容器场景需在云端验证。

## 验证截图路径

- Quake CDP 搜索截图: `data/cdp-verify/app-screenshots/2026-07-29-verify_quake/search-engine-results/`
- Hunter CDP 搜索截图: `data/cdp-verify/app-screenshots/2026-07-29-verify_hunter/search-engine-results/`
- Quake 原始 CDP 截图: `data/cdp-verify/quake_20260729_010730.png`
- Hunter 原始 CDP 截图: `data/cdp-verify/hunter_20260729_010803.png`
- Hunter JPEG 回退截图: `data/cdp-verify/hunter_cdp_final.png`

## Cookie 保存

- Quake: 9 个 Cookie（含 SSO `Q`/`T`/`__NS_Q`/`__NS_T`），已写入 `configs/config.yaml`
- Hunter: 13 个 Cookie（含 `token`/`csrf_token`/`User-Center`/`ack`），已写入 `configs/config.yaml`

## Web 服务 E2E 验证（2026-07-29 02:10）

通过 `go run ./cmd/unimap-web` 启动完整 Web 服务（端口 8448），使用 form-encoded POST
`/api/v1/query` 触发 `browser_query=true&browser_action=collect_and_capture`。

### 服务端日志确认

- Hunter bridge-collect: `items=2 total=7027992 engine=hunter`
  - item[0]: ip=3.169.137.99 port=80 title=301 Moved Permanently
  - item[1]: ip=154.220.39.226 port=80 title=没有找到站点
- Quake bridge-collect: `items=0 total=0 engine=quake`。这是修复前的 Extension DOM 结果，
  不能作为 CDP L1 结论；根因之一是复制按钮的 IP 位于 `data-clipboard-text`。
- 最终 POST /api/v1/query 返回 status=200，耗时 70s

## 本轮复验补充（2026-07-29）

- Quake CDP L1：
  - 旧匹配 `/api/visitor/search/query_string/quake_service` 已失效；
  - 当前接口为 `/api/search/query_string/quake_service`；
  - 当前响应使用顶层 `data` 数组和 `meta.pagination.total`；
  - 修复 endpoint 与解析器后，`TestLiveQuakeL1NetworkCollection` 使用既有持久化登录态获得非空资产并通过。
- Quake Extension DOM：
  - `data-clipboard-text="IP:port"` 现按属性提取，Host/ASN/Org/ISP 按标签归一化；
  - `node --test tools/extension-screenshot/test/capture_quake.test.mjs` 使用真实 Chrome 和固定 DOM 夹具通过；
  - 尚未重载真实 Extension 并重新执行完整 Web E2E，因此真实 Bridge/Web 仍不得标记为通过。
- SSRF：
  - headless 私网重定向与 Image/Stylesheet 子资源均在目标收到连接前失败；
  - loopback 出口代理测试确认受限目标命中数为 0。

### SQLite 持久化

- `data/screenshot_batches.db` 包含 2 条历史批量截图任务记录（batch_1784534262558374500 等）
- 浏览器查询结构化采集结果通过 API 响应合并返回，不单独持久化到 SQLite
- 合并逻辑由 `TestRunBrowserQueryAsync_CollectsStructuredAssets` 和
  `TestExecuteQueryWithBrowserWorkflow` 单元测试覆盖

### 通知管线

- 通知由 Scheduler 后置管线统一发送，Runner 不持有通知器
- `LoginStatusCheckRunner` 结果文本包含健康摘要和恢复提示，任务启用通知后自动发送
- 飞书/Webhook 渠道需在配置中设置真实凭据后才能触发 E2E（当前未配置外部通知目标）
