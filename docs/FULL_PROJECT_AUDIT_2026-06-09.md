# UniMap 全量项目审查报告（重新审查）

> **审查日期**: 2026-06-09（重新审查）
> **审查方法**: 6 个并行 Agent 按用户旅程深度审查 + 交叉验证汇总
> **审查维度**: 首次配置、资产查询、截图监控、定时任务、错误处理与安全、API 契约一致性
> **基线**: `go build ./...` ✅ | `go test -race ./...` ✅ (41 包全绿)

## 修复状态

| 级别 | 总数 | 已修复 | 剩余 |
|------|------|--------|------|
| P0 | 4 | 4 ✅ | 0 |
| P1 | 14 | 14 ✅ | 0 |
| P2 | 17 | 17 ✅ | 0 |

> **2026-06-10 跟进**：补齐复核发现的部分完成项：批量 URL 截图现在将无效/内网 URL 作为 `failed` 结果纳入 job，批量截图 Provider 支持逐项进度回调，`main.js` API 请求统一走 `apiFetch`，CSV/JSON 导出改为基于完整结果数组而非当前 DOM 页。

> **2026-06-11 跟进**：安全与稳定性修复 19 项。CRITICAL 数据竞争修复（`getSnapshot` 深拷贝）；P0 XSS 转义（monitor.html 6 处 + batch-screenshot.html）；P1 BridgeService panic recovery、ResourceMonitor 幂等 Stop、WS JSON.parse try/catch、fetch 统一 resp.ok 检查、adminToken 持久化、ScreenshotAppService RWMutex 同步、batch metrics 修正、cleanup goroutine、前端 polling 失败处理；P2 logger.Sync + .dockerignore。新增 5 个回归测试。全量 `go build/vet/test -race` 通过。剩余：Auth 测试覆盖、handleScreenshot 绕过 Router、CSP unsafe-inline、管理端点限流、server.go 拆分、硬编码路径、go.mod 版本。

---

## P0 — 阻断使用 / 安全漏洞

| # | 问题 | 位置 | 用户影响/攻击场景 | 修复建议 | 需前后端联动 |
|---|------|------|-------------------|----------|-------------|
| P0-1 | **viewScreenshot 发送 form-urlencoded 但后端期望 JSON** | `main.js:2690` + `screenshot_handlers.go:134` | 用户点击结果表中任意资产的"截图"按钮，请求必然失败（JSON 解析错误），返回 400。用户无法对单个目标截图。 | 前端改为发送 JSON：`{method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({url:target})}` | 是（前端改请求格式，后端无需改动） |
| P0-2 | **错误响应格式不匹配 — 前端将 [object Object] 渲染为错误文本** | `main.js:1129,1209` + `http_helpers.go:110` | 当 HTTP 回退查询失败时（如 WS 不可用），用户看到的错误信息是 `[object Object]` 而非可读文本。完全无法诊断问题。 | 前端提取 `typeof data.error === 'object' ? data.error.message : data.error` | 是（建议前端适配，改动最小） |
| P0-3 | **WebSocket query_error 消息被静默丢弃** | `main.js:991-1006` + `websocket_handlers.go:175` | 用户通过 WS 提交空查询或无效参数时，前端永远停留在"正在查询..."加载状态，按钮不恢复，无法重试。必须刷新页面。 | 在 switch 中添加 `case 'query_error':` 分支，调用 `showResultsError(message.error)` 并恢复按钮状态 | 否（纯前端修复） |
| P0-4 | **viewScreenshot 中 target 未转义直接拼入 innerHTML — XSS 注入点** | `main.js:2716` | 攻击者在查询结果中注入恶意 URL，其他用户点击查看截图时触发 XSS，可窃取 session cookie 或执行任意操作。 | 用 `escapeHtml(target)` 替换直接拼接，或用 `textContent` 替代 innerHTML | 否（纯前端修复） |

---

## P1 — 体验差 / 功能缺陷

| # | 问题 | 位置 | 用户影响 | 修复建议 | 需前后端联动 |
|---|------|------|----------|----------|-------------|
| P1-1 | **首次使用零引导** | `index.html` + `main.js:1718-1741` | 新用户点击"执行查询"，等待后看到 5 个引擎的折叠错误面板，全是认证失败。不知道如何配置 API Key，也不知道"设置"页面在哪里。 | 零 key 时在查询表单上方显示 banner："尚未配置 API Key，请先[前往设置]添加引擎密钥。" `!` 添加 title tooltip 并链接到设置页 | 否（纯前端） |
| P1-2 | **查询超时无前端通知、无取消机制** | `main.js:1137-1227` + `962-972` | 长查询时用户看到"正在查询...请稍候"无限旋转。WS 断连后查询进度完全丢失，但界面无任何变化。 | 添加 90s 客户端超时，超时后显示"查询超时，请重试"并恢复按钮。WS 重连失败后显示 banner。HTTP 回退用 AbortController | 否（纯前端） |
| P1-3 | **无预查询校验 — 可以用零 key 提交查询** | `main.js:104-157` | 用户浪费一次完整请求往返后才看到所有引擎报错。体验极差。 | 提交前检查引擎状态缓存，若无引擎有 key 则弹出提示并阻止提交 | 否（纯前端） |
| P1-4 | **批量截图无进度反馈，完全同步阻塞** | `screenshot_handlers.go:443-485` | 批量截图 50+ URL 时浏览器可能显示请求超时。用户无法判断进度，无法取消。 | 最小方案：添加 context timeout 并返回已完成的部分结果。进阶方案：改为异步 job + 轮询/WebSocket 推送 | 是（后端改为异步 API + 前端轮询） |
| P1-5 | **错误页面导航栏残缺** | `error.html:15-21` | 用户遇到错误跳转到错误页后，导航栏与主站不一致，无法直接跳转到设置或其他页面。 | 用 `{{template "header" ...}}` 替换硬编码导航 | 否（纯模板修复） |
| P1-6 | **index.html 有重复"监控"按钮** | `index.html:42,44` | 用户看到两个一模一样的"监控"按钮并排，明显的 copy-paste 错误。 | 删除第 44 行 | 否 |
| P1-7 | **截图模式切换不持久化** | `screenshot_handlers.go:788-796` | 用户设置为 Extension 模式，服务器重启后自动回退为 CDP 或 auto。用户不知情。 | 在模式切换后调用 `s.configManager.Save()` 持久化到 config.yaml | 否（纯后端） |
| P1-8 | **设置页面 CDP/Bridge 状态不自动刷新** | `settings.html` | 用户在设置页配置好 CDP 后，如果 CDP 意外断开，状态标签一直显示"已连接"直到手动刷新。 | 添加 `setInterval` 轮询 | 否（纯前端） |
| P1-9 | **详情弹窗数据为空** | `main.js:2154` vs `1453-1475` | WS 查询结果中点击"详情"按钮，弹窗只显示 IP 和端口，协议/主机/标题/服务器等字段全部为空。 | 在 `showResults` 的 `<tr>` 模板中添加 `data-*` 属性 | 否（纯前端） |
| P1-10 | **Excel 导出实为 CSV** | `main.js:1920-1928` | 用户期望 `.xlsx` 文件，实际下载 `.csv`。按钮标注"导出 Excel"但实际是 CSV。 | 引入 SheetJS 真正导出 xlsx，或将按钮改名为"导出 CSV" | 否（纯前端） |
| P1-11 | **认证失败错误信息无指向性** | `main.js:1058-1061` | 新用户看到"认证失败，请检查 API Key 配置"但不知道去哪里配置。 | 改为带链接的错误信息：`认证失败，请 <a href="/settings#panel-engines">配置 API Key</a>` | 否（纯前端） |
| P1-12 | **批量截图单个 URL 为私有 IP 时拒绝整个批次** | `screenshot_handlers.go:488-504` | 用户提交 100 个 URL，其中 1 个是私有 IP，结果得到 0 条结果而非 99 条。 | 改为逐条校验，私有 IP 标记为 failed 并在结果中返回，其余正常处理 | 是（后端改校验逻辑，前端适配响应） |
| P1-13 | **handleGetAdminToken 暴露完整 admin token** | `query_handlers.go:457-459` | 任何 XSS 漏洞都可立即泄露 admin token，进而伪造 session、解密所有会话数据。 | 仅返回前 8 字符 + `****`，完整查看需二次验证（输入密码） | 是（后端改返回，前端适配展示） |
| P1-14 | **WebSocket token 通过 URL 查询参数传递** | `server.go:906` + `websocket_handlers.go:153` | Admin token 明文出现在 URL 中，被日志系统/代理/浏览器历史记录泄露。 | 移除 URL query 参数认证方式，仅保留 session cookie 和 header 认证 | 是（后端移除 fallback + 前端适配认证方式） |

---

## P2 — 优化建议

| # | 问题 | 位置 | 改善效果 | 修复建议 |
|---|------|------|----------|----------|
| P2-1 | **Blob URL 内存泄漏** | `main.js:1856,1908,2396,2700` | 长时间使用后浏览器内存持续增长 | 下载 click 后 setTimeout 调用 `URL.revokeObjectURL(url)` |
| P2-2 | **大量结果 DOM 性能问题** | `main.js:1441-1478,1753-1779,1945-1980` | 1000+ 行全部 innerHTML 插入，分页靠 CSS display:none，排序重新插入所有行 | 虚拟滚动或仅渲染当前页行 |
| P2-3 | **main.js 死代码** | `main.js:2086-2126` | 已被 `initAssetActionDelegation` 替代的事件监听仍在执行 | 删除 2086-2126 行 |
| P2-4 | **escapeHTML 与 escapeHtml 重复** | `main.js:1008,2873` | DOM 方式和字符串替换方式并存，开发者可能误用 | 删除 `escapeHTML`，统一用 `escapeHtml` |
| P2-5 | **exportToCSV/JSON 使用隐式全局变量** | `main.js:1826,1830` | `headerNames` 和 `displayedRows` 无声明关键字，变量污染风险 | 添加 `const` 声明 |
| P2-6 | **主页面无 CDP/Bridge 状态指示** | `main.js:163-214` + `index.html` | 用户提交浏览器模式查询前无法知道 CDP/Bridge 是否可用 | 在 index.html 引擎区域旁添加状态小标签 |
| P2-7 | **Bridge 状态轮询频率过高** | `main.js:209` vs `router.go:89` | 主页 5s 轮询但健康探针 30s 执行一次，5/6 次返回相同数据 | 轮询间隔改为 15-30s |
| P2-8 | **设置页 API Key 字段无 show/hide 切换** | `settings.html:289,296,305,314,323` | 用户无法验证粘贴的 key 是否正确 | 添加眼睛图标切换 password/text |
| P2-9 | **引擎 API Key 无格式提示** | `settings.html` | 新用户不知道各引擎 key 的格式差异 | 添加引擎特定 placeholder |
| P2-10 | **任务调度器历史筛选 CSP 阻断** | `scheduler.html:147,155,164` | 类型/状态/数量筛选器点击无反应（inline onchange 被 CSP 阻止） | 改为 addEventListener 绑定 |
| P2-11 | **任务列表"执行失败"计数永远为 0** | `scheduler.html:302,337` | `loadTasks()` 声明 `let errors = 0` 但从未递增，误导用户 | 从历史记录 API 获取真实失败数，或移除该卡片 |
| P2-12 | **通知 webhook URL 保存时不做 SSRF 校验** | `notification_handlers.go:243` | 用户配置了内网 webhook URL，保存成功，但重启后通知静默丢失 | 保存时调用 `urlguard.Check` 校验 |
| P2-13 | **isMaskedSecret 逻辑可改进** | `config_handlers.go:390-404` | 若用户真实 key 恰好含 `****` 会被静默拒绝 | 改为正则匹配 `maskAPIKey` 的精确输出格式 |
| P2-14 | **WebSocket 重连失败后无 UI 提示** | `main.js:964-972` | 6 次重连失败后仅 console.warn，用户无感知 | 显示持久 banner + 手动重连按钮 |
| P2-15 | **前端无网络离线检测** | `main.js` 全局 | 网络断开时操作静默失败，按钮卡在 loading 状态 | 添加 online/offline 监听，显示离线 banner |
| P2-16 | **前端不处理 401/403 HTTP 状态** | `main.js` 多处 | session 过期后 API 返回 401，前端显示泛化错误而非跳转登录 | 创建共享 `apiFetch` 包装器，401 重定向 /login |
| P2-17 | **handleSetScreenshotMode 未使用 decodeJSONBody** | `screenshot_handlers.go:777` | 直接用 `json.NewDecoder` 缺少 size limit/GBK 检测/unknown field 拒绝 | 改用 `decodeJSONBody(w, r, &req)` |

---

## 与上一份审计报告的差异

### 新增发现（之前报告未覆盖）

| # | 问题 | 说明 |
|---|------|------|
| P0-1 | viewScreenshot form-urlencoded vs JSON 不匹配 | 此前无报告检查过单资产截图的请求格式 |
| P0-2 | 错误响应格式 object vs string 不匹配 | 此前无报告系统性检查过 API 错误格式一致性 |
| P0-3 | query_error WS 消息丢失 | 此前无报告检查过 WS 消息处理完整性 |
| P0-4 | viewScreenshot innerHTML XSS | 此前无报告检查过截图弹窗的 XSS 风险 |
| P1-12 | 批量截图私有 IP 拒绝整个批次 | 此前无报告检查过此行为 |
| P1-13 | admin token 完整暴露 | 此前报告仅在安全节列出，未标为 P1 |
| P1-14 | WS token URL 参数泄露 | 此前无报告检查过 WS 认证路径 |
| P2-10 | scheduler.html CSP 阻断 inline onchange | 此前报告列为 P1，本次降为 P2（仅影响筛选器） |
| P2-12 | webhook URL 保存时无 SSRF 校验 | 此前无报告检查过通知保存路径 |

### 定性修正（之前报告描述不准确）

| 问题 | 之前定性 | 修正后 | 原因 |
|------|---------|--------|------|
| `normalizeCDPBaseURL` 两个分歧实现 | HIGH — 实际 BUG | P2 — 重复代码 | 实际场景中用户不会传纯数字端口，正常配置输入两版结果一致 |
| `isMaskedSecret` | "rejects any string containing `****`" | P2 — 逻辑可改进 | 实际代码先遍历字符判断是否全为掩码字符，风险低于此前描述 |
| XSS via viewScreenshot linkBtn.href | 独立 HIGH | 合并入 P0-4 | `target` 来自经 `escapeAttr` 设置的 `data-url` 属性，前提条件较严格 |
| CSS selector injection in `showAssetDetail` | MEDIUM | 不列入 | `ip` 和 `port` 来自已转义的 `data-*` 属性，实际场景极难触发 |

### 删除项（不成立或不影响用户）

| 问题 | 删除原因 |
|------|---------|
| "saveAllEngines 发送所有 5 个引擎配置" | 后端 `isMaskedSecret` 检查防止覆写，行为正确 |
| "handleLogoutAPI 返回 302 而非 JSON" | logout 由表单提交触发，非 fetch 调用，不影响用户体验 |
| "分页完全是客户端" | 设计决策（page_size 限制总返回量），非性能缺陷 |
| "openQueryHistory 重复绑定事件" | 低优先级优化，不影响功能 |
| "多标签页无冲突处理" | 低概率场景，不影响正常使用 |
