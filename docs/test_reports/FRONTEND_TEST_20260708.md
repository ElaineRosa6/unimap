# UniMap v1 前端测试报告

**日期**: 2026-07-08
**测试人员**: Claude Code + Hermes Agent
**测试环境**: Windows 10 + Hermes Desktop + 内部浏览器
**服务地址**: http://127.0.0.1:8448
**登录凭证**: admin / admin123

---

## 测试概览

| 指标 | 数值 |
|------|------|
| 测试页面数 | 9 |
| 通过 | 6 |
| 存在问题 | 3 |
| 阻塞 | 0 |

---

## 文档状态说明

此文件是原始前端测试报告的当前项目副本，已叠加 Codex 源码核验。原始报告正文存在重复章节、互相矛盾结论，以及多处已被源码核验推翻的根因描述。

阅读和修复时按以下优先级取信：

1. **本次修复的问题清单**：说明本次已修复的问题及其原始依据。
2. **Codex 核验标注**：作为判断原始发现是否成立的证据链。
3. **原始功能测试详情/原始交互问题汇总**：仅作为历史测试记录，保留上下文；与前两者冲突时不作为待修依据。

---

## Codex 核验标注（2026-07-09）

> 标注范围：基于当前工作区 `D:\Project\Go_project\unimap` 源码核验。原始报告位置 `D:\d\Project\Go_project\unimap\docs\test_reports\FRONTEND_TEST_20260708.md` 未修改。

### 不成立或根因不成立的问题

| 报告问题/结论 | 核验结论 | 当前源码依据 |
|---|---|---|
| 首页查询表单实际提交到 `/api/v1/logout` | **不成立**。首页查询表单是 `action="/query"`，`/api/v1/logout` 是页头独立退出表单。测试可能误定位到 header 的 logout form。 | `web/templates/index.html` 查询 form；`web/templates/layout.html` logout form |
| 首页查询按钮无响应的根因是表单 action 指向 logout | **根因不成立**。当前主流程由 `web/static/js/main.js` 拦截 `form[action="/query"]` 并调用 `/api/v1/query`。但传统 fallback 仍有真实缺陷：form 是 POST `/query`，路由只注册 GET `/query`，如果 JS 未加载会失败。 | `web/static/js/main.js` `initQueryForm()`；`web/router.go` `/query` GET；`web/query_handlers.go` `handleQuery()` 期望 POST |
| 配额页面 401 的根因是前端未携带 Authorization header | **不成立/证据不足**。服务端认证中间件优先接受 session cookie，配额页由后端服务端调用各引擎 `GetQuota()` 后渲染。页面上看到的 401 更可能来自第三方引擎 API 或适配器错误，不是页面请求缺少 Authorization。 | `web/middleware_auth.go` session cookie auth；`web/query_handlers.go` `handleQuota()`/`fetchEngineQuotas()` |
| 任务执行历史 401 的根因是前端未携带 Authorization header | **不成立/证据不足**。调度页面 API 请求可依赖 session cookie；历史记录中展示的 HTTP 401 可能是任务执行结果里保存的外部 API 错误，并不能直接推出前端鉴权缺失。 | `web/middleware_auth.go`；`web/templates/scheduler.html` `loadHistory()` |
| API 认证机制不一致：页面 session cookie，但 API 必须 X-Admin-Token | **不成立**。当前中间件同时支持 session cookie、`X-Admin-Token`、`Authorization: Bearer`。 | `web/middleware_auth.go` |
| 引擎 Key 状态前端静态显示，未与实际配置联动 | **不成立**。当前 `checkEngineStatus()` 会请求 `/api/v1/config`，根据实际 `api_key` 显示 `Key ✓`/`Key ✗`，并自动取消未配置 Key 的引擎勾选。 | `web/static/js/main.js` `checkEngineStatus()` |
| 设置保存全部、系统保存、截图服务保存均无反馈 | **不成立**。当前保存函数会将成功/失败信息写入对应 result 元素，例如 `engines-result`、`system-result`、`screenshot-result`。 | `web/templates/settings.html` `saveAllEngines()`/`saveSystem()`/`saveScreenshot()` |
| Monitor 无 URL 点击“篡改检测”无反馈 | **不成立**。当前空 URL 时会 `alert('请输入至少一个URL')`。 | `web/templates/monitor.html` `btn-tamper-check` handler |
| Account 删除/禁用用户无反馈 | **不成立**。当前删除有 confirm，成功/失败通过 toast 提示；禁用/启用也会 toast 并刷新用户列表。 | `web/templates/account.html` `deleteUser()`/`updateUser()` |
| Scheduler 编辑任务无反馈、创建新任务折叠面板未展开 | **不成立/未复现于源码**。编辑按钮会打开 modal；创建新任务使用原生 `<details><summary>` 展开，不依赖自定义 JS。 | `web/templates/scheduler.html` `editTask()`；创建表单 `<details class="create-form">` |
| Results 导出按钮点击无反馈 | **不成立**。空结果时会提示“没有可导出的结果”；有数据时会触发下载并提示成功。 | `web/static/js/main.js` `exportToCSV()`/`exportToJSON()`/`exportToExcel()` |

### 仍成立的问题

| 报告问题 | 核验结论 | 当前源码依据 |
|---|---|---|
| Results 页面重复“导出CSV”按钮 | **成立**。`btn-export-csv` 与 `btn-export-excel` 文案均为“导出CSV”。 | `web/templates/results.html` |
| Results 页面空状态信息不完整 | **成立**。直接访问 `/results` 时会显示空查询/空引擎/空结果信息。 | `web/query_handlers.go` `handleResults()`；`web/templates/results.html` |
| 设置页 API Key 眼睛按钮位置可能偏移 | **成立**。输入框 `max-width: 340px`，外层 wrapper `width: 100%`，绝对定位按钮可能不贴输入框右侧。 | `web/templates/settings.html` |
| Scheduler 时间英文格式 | **基本成立**。任务时间使用未指定 locale 的 `toLocaleString()`，展示依赖浏览器默认语言。 | `web/templates/scheduler.html` |

### 更新内容核验（原文档 2026-07-09 02:02 UTC，SHA256 `7461A9BA...`）

| 新增报告问题/结论 | 核验结论 | 当前源码依据 |
|---|---|---|
| Monitor 批量截图前端无进度/结果反馈 | **不成立**。按钮绑定后会先显示“正在截图...”，提交 `/api/v1/screenshot/batch-urls`，再轮询 `/api/v1/screenshot/batch/progress` 更新完成数、成功数、失败数和最终状态。 | `web/templates/monitor.html` `btn-batch-screenshot` handler |
| Monitor 基线管理点击“设置基线/查看基线”无弹窗或状态变化 | **不成立**。设置基线空 URL 会 alert；有效 URL 会显示“正在设置基线...”及成功/失败状态。查看基线绑定 `loadBaselineList()`，会切换/渲染 `history-results`。 | `web/templates/monitor.html` `btn-set-baseline`/`btn-list-baseline` |
| Results 导出 CSV/JSON/Excel 按钮点击后无下载或反馈 | **不成立**。CSV/JSON/Excel 按钮均绑定导出函数；空结果会 `showMessage('没有可导出的结果')`，有数据会触发下载。已成立的问题是 Excel 按钮文案误写成“导出CSV”。 | `web/templates/results.html`；`web/static/js/main.js` `initExportButtons()` |
| Account 密码修改表单可提交但无成功/失败提示 | **不成立**。前端校验失败、接口失败、请求异常和成功都会调用 `showToast()`。成功提示为“密码已更新”。 | `web/templates/account.html` password submit handler |
| Account 禁用/删除无确认弹窗或状态反馈 | **不成立**。删除用户先 `confirm()`，成功/失败 toast；禁用/启用走 `updateUser()`，成功/失败 toast 并刷新列表。 | `web/templates/account.html` `deleteUser()`/`updateUser()` |
| Port-scan “开始扫描”无扫描进度/结果反馈 | **不成立**。空目标 toast；开始后禁用按钮并写入 `scan-status = '扫描中...'`；成功调用 `renderResults(data)`，失败 toast。 | `web/templates/port-scan.html` scan handler |
| Port-scan “历史”按钮可点但历史页面无扫描记录 | **不能作为源码缺陷成立**。历史面板会请求 `/api/v1/history?type=port_scan&limit=50` 并渲染记录；显示“无扫描历史”只能说明当前环境没有保存记录或保存失败，需要运行时数据佐证。扫描成功路径会调用 `/api/v1/history/save`。 | `web/templates/port-scan.html` history/save handlers |
| Port-scan 清空历史无确认/超时 | **“无确认”不成立；“超时”源码无法确认**。清空按钮存在 `confirm('确定要清空所有端口扫描历史吗？')`，随后 DELETE `/api/v1/history?type=port_scan`。是否超时需要服务端运行日志或网络抓包。 | `web/templates/port-scan.html` clear history handler |
| Settings “保存全部”及各设置保存无成功/失败提示 | **不成立**。搜索引擎、截图、系统、Cookie、通知渠道等保存路径均会写入对应 result 元素，成功/失败都有文本反馈。 | `web/templates/settings.html` `saveAllEngines()`/`saveScreenshot()`/`saveSystem()` |
| Scheduler “创建新任务”折叠面板无法展开 | **不成立/源码不支持该结论**。创建新任务使用原生 `<details><summary>`，展开不依赖 JS 事件绑定。若运行时不能展开，需浏览器/样式层复现证据。 | `web/templates/scheduler.html` create form |
| Scheduler 创建/编辑任务流程无法完成、编辑无反馈 | **不成立/源码不支持该结论**。创建、编辑、删除、立即执行、启用/禁用都有 API 调用和 toast；编辑会请求任务详情并打开 modal。 | `web/templates/scheduler.html` `createTask()`/`editTask()`/update handlers |
| API 节点注册/心跳需要配置 `node_auth_tokens` | **结论本身成立，但不应标为前端缺陷**。节点端点是分布式节点鉴权路径，支持 `X-Node-Token`，配置 token 是设计要求。 | `web/middleware_auth.go` `isNodeAuthPath()`/`authenticateNodeToken()`；`web/node_auth.go` |
| API health/ready、health/live、metrics 在 `/api/v1/...` 返回 HTML 而非 JSON | **路径结论不成立**。当前注册的是 `/health/ready`、`/health/live`、`/metrics`，不是 `/api/v1/health/ready`、`/api/v1/health/live`、`/api/v1/metrics`。健康检查 handler 会设置 `Content-Type: application/json`；`/api/v1/...` 不是对应路由。 | `web/router.go` `registerQueryRoutes()`；`web/health_handlers.go` |
| API `/api/v1/screenshot/batch*`、`/api/v1/url/*`、`/api/v1/backup/*`、`/api/v1/nodes/status`、`/api/v1/config` 正常 | **源码路径基本吻合，但未做运行时复测**。这些 API 由 `addAPIRoute()` 注册，实际可用性仍取决于登录态、配置、外部服务和运行环境。 | `web/router.go` |

### 更新内容核验（原文档 2026-07-09 02:09 UTC，SHA256 `07C08B11...`）

| 新增报告问题/结论 | 核验结论 | 当前源码依据 |
|---|---|---|
| 42-49 再次列出的 Monitor/Results/Account/Port-scan/Settings/Scheduler 无反馈问题 | **不成立或不能作为源码缺陷成立，已在上一节逐项覆盖**。这些条目与 26-35 基本重复；当前源码存在 alert、toast、result 文本、进度文本、modal 或下载逻辑。 | 见上一节对应条目 |
| Settings 通知渠道“测试”按钮可点击但无测试结果反馈 | **不成立**。测试函数会先显示“测试发送中...”，请求 `/api/v1/notifications/channels/test`，随后显示“测试消息发送成功”或失败原因。 | `web/templates/settings.html` notification test handler |
| Scheduler 任务编辑/执行/禁用/删除按钮可点击但无状态变化反馈 | **不成立**。编辑会打开 modal；执行成功提示“任务已提交执行”；禁用/启用成功提示“任务已禁用/任务已启用”并刷新列表；删除有 confirm，成功提示“任务已删除”。 | `web/templates/scheduler.html` `editTask()`/`runTask()`/`toggleTask()`/`deleteTask()` |
| 首页历史“清空”按钮点击后超时 | **源码无法确认超时；“无反馈/无处理”不成立**。清空按钮有 confirm，确认后 DELETE `/api/v1/history?type=query`，成功后把列表更新为“无查询历史”。若运行时超时，需要服务端日志或网络请求证据。 | `web/static/js/main.js` `openQueryHistory()` |
| 首页查询按钮 form action 指向 `/api/v1/logout` | **不成立**。首页查询 form 是 `action="/query"`；`/api/v1/logout` 是 `layout.html` 页头退出表单。 | `web/templates/index.html`；`web/templates/layout.html` |

---

## 本次修复的问题清单（2026-07-09）

> 本节列出本次实际修复的问题。下方“原始功能测试详情”和“原始前后端交互问题汇总”只作为历史测试记录保留，凡与本节或 Codex 核验标注冲突的，以本节为准。

### P1 - 已确认功能/体验问题

#### 1. Results 页面重复“导出CSV”按钮

- **页面**: 查询结果 `/results`
- **结论**: 成立
- **修复状态**: 已修复，见“本次修复记录”。
- **描述**: `btn-export-csv` 与 `btn-export-excel` 的按钮文案均为“导出CSV”，其中 Excel 导出按钮文案错误。
- **源码依据**: `web/templates/results.html`
- **建议**: 将 `btn-export-excel` 文案改为“导出Excel”或删除重复入口。

### P2 - 已确认 UX/边界问题

#### 2. Results 页面空状态信息不完整

- **页面**: 查询结果 `/results`
- **结论**: 成立
- **修复状态**: 已修复，见“本次修复记录”。
- **描述**: 直接访问 `/results` 时，“查询/引擎/结果”信息为空，缺少“请先执行查询”等引导。
- **源码依据**: `web/query_handlers.go` `handleResults()`；`web/templates/results.html`
- **建议**: 对空查询、空引擎、空结果添加明确空状态文案和返回首页入口。

#### 3. 设置页 API Key 眼睛按钮位置可能偏移

- **页面**: 设置页 `/settings`
- **结论**: 成立
- **修复状态**: 已修复，见“本次修复记录”。
- **描述**: 输入框 `max-width: 340px`，外层 wrapper `width: 100%`，绝对定位按钮可能不贴输入框右侧。
- **源码依据**: `web/templates/settings.html`
- **建议**: 让输入框和显示/隐藏按钮共享同一定位容器宽度。

#### 4. Scheduler 时间格式依赖浏览器默认 locale

- **页面**: 任务调度 `/scheduler`
- **结论**: 基本成立
- **修复状态**: 已修复，见“本次修复记录”。
- **描述**: 任务时间使用未指定 locale 的 `toLocaleString()`，不同浏览器语言环境会显示为英文或本地格式。
- **源码依据**: `web/templates/scheduler.html`
- **建议**: 指定 `zh-CN` 或统一格式化函数。

### P3 - 防御性/一致性问题

#### 5. 首页查询表单无 JS fallback 存在路由不匹配

- **页面**: 首页 `/`
- **结论**: 成立但触发条件受限，原报告根因不成立
- **修复状态**: 已修复，见“本次修复记录”。
- **描述**: 正常使用路径依赖 JS 拦截 `form[action="/query"]` 并调用 `/api/v1/query`，并非提交到 `/api/v1/logout`。只有在 JS 未加载、CSP/脚本异常或极端降级环境下，HTML fallback 的 `POST /query` 才会触发；而当前路由只注册 `GET /query`。
- **源码依据**: `web/templates/index.html`；`web/static/js/main.js` `initQueryForm()`；`web/router.go`
- **建议**: 作为防御性一致性修复处理，例如增加 POST `/query` 路由，或将表单降级路径改成已注册接口。

### 待运行时补证据的问题

| 原始现象 | 当前状态 | 下一步 |
|---|---|---|
| 配额页/任务历史出现 HTTP 401 | **源码不能支持“前端未带 Authorization”的根因**。服务端支持 session cookie，页面看到的 401 可能来自外部引擎/API 任务结果。 | 需要服务端日志、实际网络请求、第三方 API 响应来源。 |
| Port-scan/首页历史清空超时 | **源码存在 confirm、DELETE 请求和成功后 UI 更新**，不能从静态源码确认超时。 | 需要浏览器 Network、后端 handler 耗时、日志或可复现输入。 |
| “所有保存/提交按钮无反馈” | **不成立**。源码里大量按钮已有 toast/result/progress 反馈。 | 若运行时仍无反馈，应先查 JS 加载错误、CSP、浏览器控制台和接口响应。 |

---

## 原始页面清单（历史记录，含已推翻结论）

> 状态列主要来自原始报告，本表仅用于追溯测试过程；已在本次修复的问题会直接标注为“已修复”。当前结论以上方“本次修复的问题清单”和“本次修复记录”为准。

| 页面 | 路径 | 状态 | 备注 |
|------|------|------|------|
| 登录页 | `/login` | ✅ 正常 | CSRF、表单验证正常 |
| 首页 | `/` | ℹ️ 原始记录 | 当前有效问题仅为 P3：无 JS fallback 路由不匹配 |
| ICP 备案查询 | `/icp` | ✅ 正常 | 功能完整 |
| 端口扫描 | `/port-scan` | ✅ 正常 | 表单完整 |
| 网页巡检 | `/monitor` | ✅ 正常 | 功能完整 |
| 任务调度 | `/scheduler` | ✅ 正常 | 27个任务正常显示 |
| 设置页 | `/settings` | ✅ 已修复 | API Key 按钮定位已调整 |
| 查询结果 | `/results` | ✅ 已修复 | 重复按钮已修正、空状态已补齐 |
| 账号管理 | `/account` | ✅ 正常 | 标签页切换正常 |

---

## 测试环境信息

- **OS**: Windows 10
- **浏览器**: Hermes Internal Browser
- **服务版本**: unimap v1
- **端口**: 8448
- **环境变量**: UNIMAP_NOTIFY_PEPPER=[REDACTED]

---

## 修复后剩余建议

1. **已修复**: Results 页面 Excel 导出按钮文案、Results 空状态、首页查询表单 fallback、设置页 API Key 眼睛按钮定位、Scheduler 时间格式。
2. **先补证据再定性**: 配额页/任务历史 401、历史清空超时、Port-scan 超时等运行时现象，需要 Network/后端日志定位后再列为待修 bug。
3. **建议复测**: 用浏览器打开 `/results`、`/settings`、`/scheduler` 和首页表单，确认视觉和交互符合预期。

---

## 本次修复记录（2026-07-09）

| 修复项 | 修改文件 | 具体改动 | 审计关注点 |
|---|---|---|---|
| Results 页面重复“导出CSV”按钮 | `web/templates/results.html` | 将 `btn-export-excel` 按钮文案从“导出CSV”改为“导出Excel”。 | 按钮 `id` 和 JS 绑定未变，只改展示文案。 |
| Results 页面空状态信息不完整 | `web/templates/results.html`、`web/query_handlers.go` | 无查询参数时显示“尚未执行查询”引导和“返回查询”入口；不再渲染空查询/空引擎/空结果信息；无结果但有查询时显示“未找到匹配结果”；`handleResults()` 补齐 `totalCount=0` 和空 `engineStats`。 | 不改变查询执行逻辑，只调整 `/results` 直接访问和空结果展示。 |
| 首页查询表单无 JS fallback 路由不匹配 | `web/router.go` | 新增 `POST /query` 路由并复用现有 `handleQuery`。 | `handleQuery` 原本已处理 POST，本次只补齐路由注册；正常 JS 查询路径 `/api/v1/query` 不变。 |
| 设置页 API Key 眼睛按钮位置偏移 | `web/templates/settings.html` | 将 `.key-toggle-wrap` 限制为 `max-width: 340px`，内部 input 取消重复 `max-width`，让切换按钮贴住输入框右侧。 | 只改 CSS，不影响保存、显示/隐藏 JS 逻辑。 |
| Scheduler 时间英文格式 | `web/templates/scheduler.html` | 新增 `formatDateTime()`，统一使用 `zh-CN`、24 小时制，并替换任务列表、一次性任务时间、执行历史开始时间展示。 | 只改前端展示格式；提交任务时 `run_at` 仍按原逻辑转 ISO。 |

**验证**: 已执行 `gofmt -w web\router.go web\query_handlers.go` 和 `go test ./web`，测试通过。

---

## 原始功能测试详情（历史记录，含已推翻结论）

> 以下为原始测试记录，存在重复章节和相互矛盾结论。凡出现“提交到 `/api/v1/logout`”“前端未带 Authorization”“所有保存/提交无反馈”等描述，已被上方 Codex 核验标注推翻，不应继续作为待修结论。

### 登录与账号管理

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 登录 | ✅ 通过 | admin / admin123 登录成功 |
| 查看当前用户 | ✅ 通过 | 显示用户名 admin 和管理员 Token |
| 创建新用户 | ✅ 通过 | 创建 newuser / readonly 角色，用户列表可见 |
| 用户管理标签页 | ✅ 通过 | 显示用户列表，含 admin、test、newuser |
|| 修改密码 | ❌ 异常 | 提交后无成功/错误反馈，可能未保存 |
|| 当前用户标签页 | ✅ 通过 | 显示用户名和管理员 Token |
|| 用户管理标签页 | ✅ 通过 | 显示用户列表，含 admin、test、newuser |
|| 创建新用户 | ✅ 通过 | 创建 newuser / readonly 角色，用户列表可见 |
|| 删除/编辑用户 | ⚠️ 未测试 | 页面有按钮，未实际执行删除/编辑 |

### ICP 备案查询功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 输入域名查询 | ✅ 通过 | 输入 `baidu.com` 成功返回备案信息 |
| 结果显示 | ✅ 通过 | 结果列表显示 1 条记录，详情区域展示完整信息 |
| 详情字段 | ✅ 通过 | 域名、许可证号、主办单位、性质、更新时间等均正确显示 |
|| 历史记录 | ✅ 通过 | 点击"历史"按钮弹出历史面板，显示历史查询记录 |
|| 清空历史 | ⚠️ 未测试 | 历史面板有"清空"按钮，未实际执行清空 |
|| 对比功能 | ⚠️ 未测试 | 需要至少两次查询结果才能对比 |

**查询结果示例**:
- 域名: baidu.com
- 许可证号: 京ICP证030173号-1
- 主办单位: 北京百度网讯科技有限公司
- 性质: 企业
- 更新时间: 2019-05-16 16:06:21

**历史记录示例**:
- ip="132.232.231.41" 部分 7/6/2026, 3:44:08 PM
- title~="登录" && country="CN" 成功 6/30/2026, 3:57:50 PM
- country="CN" && port="80" && protocol="http" 6/21/2026, 8:08:13 PM

### 首页查询功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 输入查询语句 | ✅ 通过 | `country="CN"` 输入正常 |
| 执行查询按钮 | ❌ 失败 | 点击后无响应，未跳转结果页 |
| 表单提交分析 | ❌ 失败 | 实际提交到 `/api/v1/logout`，导致功能失效 |

**根因分析**:
- 页面存在 `<form>` 标签，action 属性与实际查询接口不符
- 前端 JS 未正确拦截表单提交或未绑定正确的查询逻辑
- 用户点击"执行查询"实际触发了登出流程

### 任务调度功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 任务列表展示 | ✅ 通过 | 27个任务正常显示，含名称、类型、调度、状态 |
| 任务操作按钮 | ✅ 通过 | 编辑/执行/禁用/删除按钮可见 |
| 执行历史 | ⚠️ 部分异常 | 页面可打开，但显示大量 HTTP 401 错误 |
| 创建新任务 | ⚠️ 未测试 | 有折叠面板，未实际展开创建 |

**执行历史异常**:
- 大量任务状态显示 `HTTP 401`
- 错误信息: `{"code":"login_required","error":"login required, missing Authorization header"}`
- 说明历史任务依赖 API token，但前端未正确携带

### 配额功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 页面加载 | ⚠️ 异常 | 显示 HTTP 401 错误 |
| 刷新按钮 | ⚠️ 未测试 | 页面异常，无法验证 |
| 导出按钮 | ⚠️ 未测试 | 页面异常，无法验证 |

**配额页异常**:
- 各引擎卡片显示 `HTTP 401` 错误
- 错误信息: `login required, missing Authorization header`
- 说明配额查询 API 需要额外的 Authorization header

### 设置页面功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 搜索引擎配置 | ✅ 通过 | FOFA/Hunter/ZoomEye/Quake/Shodan 配置项完整 |
| API Key 显示/隐藏 | ✅ 通过 | 👁 按钮可切换明文/密文 |
| QPS 配置 | ✅ 通过 | 可调整各引擎 QPS 限制 |
| 启用/禁用引擎 | ✅ 通过 | 开关可正常切换 |
| 保存全部配置 | ❌ 失败 | 点击后无任何反馈，配置未保存 |
| 设置子页面 | ⚠️ 部分异常 | 搜索引擎页正常，其他子页面未测试 |

**设置保存问题**:
- 点击"保存全部"按钮后页面无响应
- 无成功/错误提示
- 配置可能未持久化到后端

### 端口扫描功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 输入目标URL | ✅ 通过 | 支持多行输入，每行一个URL |
| 端口配置 | ✅ 通过 | 可自定义端口，留空使用默认端口 |
| 并发数配置 | ✅ 通过 | 可调整并发数，默认10 |
| 开始扫描按钮 | ✅ 已执行 | 点击后生成扫描任务，但状态显示"失败 1" |
| 结果表格 | ⚠️ 异常 | 扫描状态显示"失败"，未显示解析IP、CDN、开放端口 |
| 历史按钮 | ⚠️ 未测试 | 页面有历史按钮，未点击查看 |

**端口扫描异常**:
- 输入 `baidu.com`，端口 `80,443`，并发数 10
- 点击"开始扫描"后状态显示 **失败 1**
- 结果表格为空，未显示扫描结果
- 可能原因：扫描目标不可达、权限不足、或扫描服务异常

### 网页巡检功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 加载示例 | ✅ 通过 | 点击"加载示例"成功填充示例URL |
| URL输入 | ✅ 通过 | 支持多行URL输入 |
| 统计信息 | ✅ 正常 | 显示总URL数、合法URL、非法URL、可达URL、不可达URL |
| 判定模式 | ✅ 正常 | 支持宽松/平衡/安全/精确/严格五种模式 |
| 篡改检测 | ❌ 无反馈 | 无真实URL时点击无任何提示 |
| 设置基线 | ⚠️ 未测试 | 需要URL数据才能设置 |
| 批量截图 | ⚠️ 未测试 | 依赖CDP/桥接连接 |
| 导出JSON/CSV | ⚠️ 未测试 | 需要有巡检结果才能导出 |

**篡改检测异常**:
- 无真实URL时点击"篡改检测"按钮无任何反馈
- 页面状态未发生变化，无错误提示
- 建议：添加前置校验，无URL时提示"请先输入URL列表"

### 任务执行与历史测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 任务列表展示 | ✅ 通过 | 27个任务正常显示 |
| 任务操作按钮 | ✅ 通过 | 编辑/执行/禁用/删除按钮可见 |
| 执行历史 | ❌ 异常 | 显示大量 HTTP 401 和 ICP API error |
| 任务类型筛选 | ✅ 正常 | 支持按类型筛选：UQL查询、截图、篡改检测等 |
| 任务状态筛选 | ✅ 正常 | 支持按状态筛选：成功、失败、超时、跳过 |
| 创建新任务 | ⚠️ 未测试 | 有折叠面板，未实际展开创建 |

**执行历史异常详情**:
- 页面可正常打开，显示历史记录列表
- 大量记录显示 `HTTP 401: login required, missing Authorization header`
- 部分记录显示 `ICP API error`
- 错误信息: `all 20 ICP query(ies) failed`
- 说明：历史记录 API 需要 Authorization header，前端未正确传递

### 设置页面功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 搜索引擎配置 | ✅ 通过 | FOFA/Hunter/ZoomEye/Quake/Shodan 配置项完整 |
| API Key 显示/隐藏 | ✅ 通过 | 👁 按钮可切换明文/密文 |
| QPS 配置 | ✅ 通过 | 可调整各引擎 QPS 限制 |
| 启用/禁用引擎 | ✅ 通过 | 开关可正常切换 |
| 保存全部配置 | ❌ 无反馈 | 点击后无任何成功/错误提示 |
| 截图服务 | ⚠️ 未测试 | 页面可访问，未实际配置 |
| Cookie 管理 | ⚠️ 未测试 | 页面可访问，未实际配置 |
| 会话连接 | ⚠️ 部分异常 | 显示 CDP 状态"未连接"，Bridge 状态"在线" |
| 通知渠道 | ⚠️ 未测试 | 页面可访问，未实际配置 |
| 系统设置 | ⚠️ 未测试 | 页面可访问，未实际配置 |

**设置保存问题**:
- 点击"保存全部"按钮后页面无响应
- 无成功/错误提示
- 配置可能未持久化到后端

**会话连接状态**:
- CDP 浏览器：未连接，代理 `http://127.0.0.1:7890`
- Extension Bridge：在线，可点击"刷新"

### 系统设置功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 页面加载 | ✅ 通过 | 系统设置页面正常加载 |
| 配置项 | ✅ 正常 | 显示并发数、超时时间等配置项 |
| 保存配置 | ❌ 无反馈 | 点击"保存配置"后无任何反馈 |

**系统设置问题**:
- 点击"保存配置"按钮后页面无响应
- 无成功/错误提示
- 配置可能未持久化到后端

### Results 页面导出功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 页面加载 | ✅ 通过 | 查询结果页面正常加载 |
| 导出CSV按钮 | ❌ 无反馈 | 点击后无下载/错误提示 |
| 导出JSON按钮 | ❌ 无反馈 | 点击后无下载/错误提示 |
| 导出Excel按钮 | ❌ 无反馈 | 点击后无下载/错误提示 |
| 筛选器 | ✅ 正常 | IP/端口/协议/来源筛选器完整 |
| 应用/重置按钮 | ⚠️ 未测试 | 无结果数据时无法验证 |

**Results 导出问题**:
- 三个导出按钮点击后均无任何反馈
- 页面当前无查询结果，可能因缺少数据无法测试
- 控制台无 JS 错误

### Monitor 基线管理功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 查看基线 | ❌ 无反馈 | 点击后无弹窗/页面变化 |
| 设置基线 | ❌ 无反馈 | 无真实URL时点击无反应 |
| 批量截图 | ❌ 无反馈 | 点击后无进度/结果 |

**基线管理问题**:
- 所有基线相关按钮均无可见反馈
- 可能因缺少真实URL数据或API认证问题

### Scheduler 创建新任务功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 创建新任务折叠面板 | ❌ 未展开 | 点击"+ 创建新任务"后面板未展开 |
| 任务表单 | ⚠️ 未测试 | 折叠面板未展开，无法看到表单 |

**创建任务问题**:
- 点击"+ 创建新任务"后折叠面板未展开
- 可能因前端 JS 异常或交互问题

### Account 用户删除功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 用户列表 | ✅ 正常 | 显示 admin、test、newuser 三个用户 |
| 删除用户 | ❌ 无反馈 | 点击"删除"按钮后无确认弹窗/反馈 |
| 编辑角色 | ⚠️ 未测试 | 下拉框存在，未实际切换 |
| 禁用/启用 | ⚠️ 未测试 | 按钮存在，未实际点击 |

**删除用户问题**:
- 点击 newuser 的"删除"按钮后无任何反馈
- 无确认弹窗、无错误提示、无状态变化
- 与设置页保存、修改密码等无反馈问题一致

### Settings 截图服务功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 页面加载 | ✅ 通过 | 截图服务页面正常加载 |
| 模式选择 | ✅ 正常 | CDP/浏览器扩展下拉框可用 |
| 引擎选择 | ✅ 正常 | CDP/Extension 下拉框可用 |
| 超时配置 | ✅ 正常 | 可调整超时时间 |
| 保存配置 | ❌ 无反馈 | 点击"保存配置"后无任何反馈 |

### Account 用户编辑/禁用功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 用户列表 | ✅ 正常 | 显示 admin、test、newuser 三个用户 |
| 删除用户 | ❌ 无反馈 | 点击"删除"按钮后无确认弹窗/反馈 |
| 禁用用户 | ❌ 无反馈 | 点击 test 的"禁用"按钮后无状态变化 |
| 编辑角色 | ⚠️ 未测试 | 下拉框存在，未实际切换 |

**用户管理问题**:
- 禁用/删除按钮点击后均无任何反馈
- 与设置页保存、修改密码等无反馈问题一致

### Scheduler 编辑任务功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 编辑任务 | ❌ 无反馈 | 点击第一个任务的"编辑"按钮后无弹窗/反馈 |
| 创建新任务 | ❌ 未展开 | 点击"+ 创建新任务"折叠面板未展开 |

**任务调度问题**:
- 编辑/创建按钮均无可见反馈
- 可能因前端 JS 异常或 API 认证失败

### Monitor 历史记录功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 加载示例 | ✅ 正常 | 点击"加载示例"后URL输入框有内容 |
| 清空 | ⚠️ 未确认 | 点击后无确认弹窗 |
| 执行巡检 | ❌ 无反馈 | 输入 baidu.com 后点击"篡改检测"无反馈 |
| 历史记录标签 | ⚠️ 未找到 | 代码存在但前端视图未展示 |

**Monitor 问题**:
- 篡改检测按钮无反馈
- 历史记录功能代码存在但前端未暴露

### Port-scan 历史记录功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 历史按钮 | ✅ 正常 | 点击"历史"弹出历史记录面板 |
| 历史记录内容 | ✅ 正常 | 显示"无扫描历史" |
| 清空历史 | ❌ 超时 | 点击"清空"按钮后页面超时 |
| 开始扫描 | ❌ 无反馈 | 输入URL后点击"开始扫描"无反馈 |

**端口扫描问题**:
- 清空历史按钮点击超时
- 开始扫描按钮无反馈

### Settings 保存全部功能测试

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 页面加载 | ✅ 正常 | 设置页面正常加载 |
| 子页面切换 | ✅ 正常 | 所有设置子页面可正常切换 |
| 保存全部 | ❌ 无反馈 | 点击"保存全部"后无任何反馈 |

**设置保存问题**:
- 所有设置子页面保存按钮均无反馈
- 与修改密码、删除用户等无反馈问题一致

---

### 原始前后端交互问题汇总（历史记录，含已推翻结论）

> 本节是原始报告总结，不是当前有效缺陷列表。其“首页提交到 `/api/v1/logout`”“配额/历史 401 根因是前端未带 Authorization”“所有保存/提交类按钮无反馈”“API 认证机制不一致”等根因已被 Codex 核验推翻或判定证据不足。

#### 功能性问题

1. **首页查询表单实际提交到 `/api/v1/logout`**
   - **严重程度**: P0
   - **描述**: 首页 `<form>` 的 `action` 属性被设置为 `/api/v1/logout`，导致点击"执行查询"时会执行登出操作
   - **根因**: `web/templates/index.html` 第 7 行 `<form action="/query" method="POST">` 与浏览器实际行为不符
   - **影响**: 用户无法通过首页表单提交查询，点击即登出
   - **建议**: 修复表单 action 为正确的查询接口，或改为前端 JS 拦截提交

2. **配额页面返回 HTTP 401**
   - **严重程度**: P1
   - **描述**: 配额页面各引擎卡片均显示 `HTTP 401: login required, missing Authorization header`
   - **根因**: 配额查询 API 需要 Authorization header，但前端请求未携带
   - **影响**: 用户无法查看各引擎配额使用情况
   - **建议**: 检查前端请求是否携带正确的认证信息

3. **任务执行历史返回大量 401**
   - **严重程度**: P1
   - **描述**: 任务调度页的执行历史显示大量 `HTTP 401` 错误
   - **根因**: 历史记录 API 需要 Authorization header，前端未正确传递
   - **影响**: 用户无法查看任务执行历史
   - **建议**: 统一 API 鉴权机制，确保所有 API 请求携带认证信息

4. **所有"保存/提交"类按钮均无反馈**
   - **严重程度**: P2
   - **描述**: 设置保存全部、修改密码、删除用户、截图服务保存、通知渠道测试等按钮点击后均无成功/错误提示
   - **可能原因**: 前端 JS 未正确处理保存响应，或后端接口异常，或 API 认证失败
   - **影响**: 用户无法确认操作是否成功
   - **建议**: 统一添加 toast 提示，检查 API 认证和错误处理

5. **端口扫描失败**
   - **严重程度**: P2
   - **描述**: 输入 `baidu.com` 执行扫描后状态显示"失败 1"
   - **可能原因**: 扫描目标不可达、权限不足、或扫描服务异常
   - **影响**: 用户无法正常使用端口扫描功能
   - **建议**: 检查扫描服务状态，添加更详细的错误提示

6. **篡改检测无反馈**
   - **严重程度**: P3
   - **描述**: 无真实URL时点击"篡改检测"按钮无任何反馈
   - **可能原因**: 缺少前置校验
   - **影响**: 用户不知道需要先输入URL
   - **建议**: 添加前置校验，无URL时提示"请先输入URL列表"

7. **API 认证机制不一致**
   - **严重程度**: P2
   - **描述**: 页面访问使用 session cookie，但 API 请求需要 X-Admin-Token header，导致部分页面（配额、执行历史）显示 401
   - **根因**: 前端 API 请求未正确携带认证信息
   - **影响**: 部分功能无法正常使用
   - **建议**: 统一认证机制，确保所有 API 请求携带正确的认证信息

#### 已验证通过的功能

1. ✅ ICP 查询功能正常，能正确返回备案信息
2. ✅ 登录功能正常，CSRF 保护有效
3. ✅ API 鉴权机制正常，未授权请求被拒绝
4. ✅ 页面路由正常，导航跳转无 404
5. ✅ 会话保持正常，登录后可访问所有页面
6. ✅ 用户管理功能正常，可创建新用户
7. ✅ 任务列表展示正常，27个任务完整显示
8. ✅ 引擎配置表单完整，所有输入项可正常编辑
9. ✅ 首页历史记录功能正常，可查看历史查询
10. ✅ 任务类型/状态筛选功能正常
11. ✅ 设置子页面切换正常，ICP 备案、截图服务、Cookie 管理、会话连接、通知渠道、系统设置均可访问
12. ✅ 任务类型说明页面正常，显示 20 种任务类型及参数示例
13. ✅ 通知渠道页面正常，显示4个已配置渠道（飞书、企业微信、Webhook、飞书应用）

---

---
