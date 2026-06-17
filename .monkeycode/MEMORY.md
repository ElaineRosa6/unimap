# 用户指令记忆

本文件记录了用户的指令、偏好和教导，用于在未来的交互中提供参考。

## 格式

### 用户指令条目
用户指令条目应遵循以下格式：

[用户指令摘要]
- Date: [YYYY-MM-DD]
- Context: [提及的场景或时间]
- Instructions:
  - [用户教导或指示的内容，逐行描述]

### 项目知识条目
Agent 在任务执行过程中发现的条目应遵循以下格式：

[项目知识摘要]
- Date: [YYYY-MM-DD]
- Context: Agent 在执行 [具体任务描述] 时发现
- Category: [代码结构|代码模式|代码生成|构建方法|测试方法|依赖关系|环境配置]
- Instructions:
  - [具体的知识点，逐行描述]

## 去重策略
- 添加新条目前，检查是否存在相似或相同的指令
- 若发现重复，跳过新条目或与已有条目合并
- 合并时，更新上下文或日期信息
- 这有助于避免冗余条目，保持记忆文件整洁

## 条目

### CORS 中间件对 Bridge 路由的绕过机制
- Date: 2026-05-07
- Context: Agent 在修复浏览器扩展 CORS 错误时发现
- Category: 代码模式
- Instructions:
  - `corsMiddleware` 会跳过 `/api/screenshot/bridge/*` 路径的 CORS origin 白名单检查
  - Bridge 路由有自己的认证机制（loopback IP 校验 + bearer token）
  - 对 bridge 路径直接返回 `Access-Control-Allow-Origin: *`，允许任意 origin（包括 `chrome-extension://`）
  - 辅助函数 `isScreenshotBridgePath()` 定义在 `web/middleware_auth.go` 中，用于判断路径是否属于 bridge API
  - `isOriginAllowed()` 函数也额外允许了 `chrome-extension://` 前缀的 origin，但此逻辑不应用于 bridge 路由（bridge 路由完全绕过 CORS 检查）

### 空间搜索引擎默认启用机制
- Date: 2026-05-07
- Context: Agent 在修复"未加载空间引擎导致查询失败"问题时发现
- Category: 代码结构
- Instructions:
  - 在 `internal/config/config.go` 的 `applyDefaults` 函数中设置所有搜索引擎 `Enabled = true`
  - FOFA 同时设置 `UseWebAPI = true` 作为默认值，允许在没有 API Key 时仍能使用 Web 模式
  - 验证函数保持完整性，仅在 `UseWebAPI = false` 时才要求提供 API Key
  - 所有引擎：Quake、ZoomEye、Hunter、FOFA、Shodan

### 扩展浏览器查询进度与登录状态判定机制
- Date: 2026-05-08
- Context: Agent 在修复浏览器插件空间搜索查询卡在 running/0% 与登录状态同步问题时发现
- Category: 代码模式
- Instructions:
  - `RunBrowserQueryAsync` 通过 progress callback 将每个 engine 的完成或失败回传给 WebSocket 查询状态，避免浏览器联动查询长期停留在 0%
  - WebSocket 查询完成时复用 `buildQueryAPIPayload` 合并 API 查询结果与 browser collect 的结构化资产，保证 HTTP 和 WebSocket 结果一致
  - `updateQueryProgress` 不允许进度回退，避免异步 browser progress 覆盖更高进度
  - 扩展模式的 `OpenSearchEngineResult` 必须设置 `BridgeTask.Action = "open"`，否则扩展端默认会按 screenshot 任务处理
  - 扩展已配对只表示 bridge 在线，不能等同于搜索引擎已登录；登录状态需通过 collect 结果中的 `login_required`、登录关键词、`items` 或 `total` 判定

### Extension 截图登录态保持机制（P0 修复）
- Date: 2026-06-16
- Context: 修复 FOFA/Hunter/Quake 截图显示未登录页面的 P0 问题
- Category: 代码模式
- Instructions:
  - `releaseTab()` 不得导航到 `about:blank`，否则销毁 session cookies；tab 保持在当前 URL
  - `ensureTab()` 使用 4 级复用策略：同域 pool tab → 任意 pool tab → 浏览器同域 tab → 新建 tab
  - `extractOrigin()` 辅助函数从 URL 提取 origin 用于同域匹配
  - `captureVisibleTab` 在 manifest v3 中需要 `<all_urls>` host permission（`activeTab` 仅在用户手动点击时生效，bridge 轮询是程序化调用不适用）
  - bridge callback handler 不能使用 `DisallowUnknownFields()`，extension 可能新增诊断字段（如 `login_diagnostics`）
  - `checkLoginCookies()` 按引擎名匹配已知登录 cookie（fofa_token/HSESSION 等），返回 `has_login_cookies` + `cookie_count`
  - 登录诊断信息附加在 `structured_collected_data.login_diagnostics` 中，Go 侧通过 `BridgeCollectedData.Extra` 透传

### 定时任务简易定时功能
- Date: 2026-06-16
- Context: 实现一次性/延迟/指定时间执行的定时任务功能
- Category: 代码模式
- Instructions:
  - `ScheduledTask.ScheduleType` 支持三种模式：`"cron"`（默认，向后兼容）、`"once"`（指定时间）、`"delay"`（延迟秒数）
  - `RunAt` 存储绝对执行时间（RFC3339），`DelaySeconds` 存储延迟秒数
  - `scheduleOneTimeTask` 用 `time.AfterFunc` 调度，执行后调用 `disableOneTimeTask` 自动禁用
  - `Load()` 重启后 delay 任务优先用持久化 `RunAt` 计算延迟，避免 `DelaySeconds` 重算导致晚执行
  - `EnableTask` 拒绝 re-enable 已过期+已执行的一次性任务（`RunAt.Before(now) && LastRunAt != nil`）
  - `disableOneTimeTask` 清除 `NextRunAt`（一次性任务不显示"下次执行"）
  - `scheduleOneTimeTask` 对 `RunAt` 做 nil 保护，防止磁盘数据损坏时 panic
  - `Stop()` 清理所有 one-time task timers
  - API：`schedule_type` + `run_at` / `delay_seconds`，handler 层做类型验证
  - 前端：创建表单"调度类型"下拉框（cron/once/delay），切换显示对应字段（cron 预设按钮 / datetime-local 日期选择器 / delay 预设按钮）；任务列表"调度"列显示类型标签（cron 蓝/once 橙/delay 绿）；编辑弹窗同步
  - `scheduler.html` 使用 `<input type="datetime-local">` 选择执行时间，浏览器原生日历+时间选择器

### 新引擎端到端集成模式（10 引擎）
- Date: 2026-06-17
- Context: 补齐 Censys/DayDayMap/BinaryEdge/Onyphe/GreyNoise 5 个新引擎在各层级的集成
- Category: 代码结构
- Instructions:
  - 新增引擎需要在 7 个层级同步更新：CLI `listEnabledEngines` + Web `stableEngines` map + settings.html 表单 + Config API GET/POST + `reloadEngineAdapters`/`registerCoreEngineAdapters` + 登录检测引擎列表 + Extension DOM 选择器
  - `settings.html` 引擎列表（`engines` 数组）、`saveAllEngines` JS 函数、`loadConfig` JS 函数必须三者同步更新
  - Censys 使用 `api_id`/`api_secret`（非 `api_key`），Go 端 `applyCensysFields` 和 JS 端 `saveAllEngines`/`loadConfig` 都需要特殊分支
  - `dom_selectors.go` 和 `capture.js` 的 DOM 选择器必须同步，否则 Extension 和 CDP 行为不一致
  - `stableEngines` map 控制首页引擎可见性；设置页 `engines` 数组控制保存范围；二者独立但建议保持一致

### goroutine leak 检测空桩修复
- Date: 2026-06-17
- Context: `countGoroutines()` 返回硬编码 0，导致 `TestRouterStartStop_NoGoroutineLeak` 测试无效
- Category: 测试方法
- Instructions:
  - goroutine leak 检测必须用 `runtime.NumGoroutine()`，不能用硬编码值
  - leak 断言需要 5-10 协程的容差，因为 runtime 的 GC/timer 协程会正常波动
  - `TestRouterStartStop_NoGoroutineLeak` 的正确模式：`before := countGoroutines()` → Start+Stop → `after := countGoroutines()` → `if after > before+margin { t.Fatal(...) }`
- Date: 2026-06-16
- Context: `span._public-hover_uxlu6_1` 是 CSS Modules 哈希类名，前端改版即失效；title 字段包含拼接元数据
- Category: 代码模式
- Instructions:
  - `capture.js` ZoomEye IP 选择器改为 `div.url-container span, div.ip-detail-box span, div.header-bar span`
  - `dom_selectors.go` `extractZoomEyeJS` 同步更新，改用稳定选择器 + IP 正则提取
  - 两个文件必须同步更新，否则 Extension 和 CDP 路径行为不一致
  - ZoomEye card text 格式：`"ip数据更新城市应用:X组织:YASN:Z标题:actual_title"`
  - `cleanZoomEyeTitle()` 提取 "标题:xxx"（停在末尾日期前），提取嵌入元数据（country_code/org/asn/server），清理残留前缀
  - Go 侧 `extractZoomEyeJS` 需同步添加同样的 title 清理 + 元数据提取逻辑

### 前端 hidden-init + display='' 隐藏陷阱
- Date: 2026-06-18
- Context: settings 页面通知渠道编辑表单、飞书应用字段、scheduler 模板按钮均无法显示，排查发现 `style.display=''` 与 `hidden-init` class 冲突
- Category: 代码模式
- Instructions:
  - `.hidden-init { display: none; }`（utils.css）。`element.style.display = ''` 只清空内联 display，class 的 `display:none` 会接管，元素仍隐藏
  - 对带 `hidden-init` class 的元素，显示时必须用 `style.display = 'block'`（块级）或 `'inline-block'`（行内），不能用 `''`
  - 受影响位置：`showNotifyForm`(notify-form-panel/btn-nch-test)、`updateNotifyFormVisibility`(nch-app-row)、`onTaskTypeChange`(btn-fill-template)
  - 无隐藏 class 的元素（如 port-scan 的 scan-results 用内联 `style="display:none"`）用 `display=''` 是 OK 的

### sync.Once 覆盖 SetXxxConfig 配置陷阱
- Date: 2026-06-18
- Context: 全局限流始终 60/min 而非 config 的 300/min，导致 settings 页面按钮无反应（429）
- Category: 代码模式
- Instructions:
  - `sync.Once.Do(fn)` 无论目标变量是否已赋值都会执行 `fn`，会覆盖之前的 `SetXxxConfig` 设置
  - `getGlobalLimiter` 的 Once 闭包无条件 `globalLimiter = NewRateLimiter(60, time.Minute)`，覆盖了 `SetRateLimitConfig(300)` 的值
  - 修复：Once 闭包内 `if globalLimiter == nil { globalLimiter = NewRateLimiter(default...) }`
  - 通用规则：懒加载默认值前必须检查变量是否已被显式配置

### CSP 拦截 innerHTML 内联事件处理器
- Date: 2026-06-18
- Context: main.js 动态生成的"重新连接"/"重新查询"按钮点击无反应
- Category: 代码模式
- Instructions:
  - CSP `script-src 'self' 'nonce-xxx'`（无 `unsafe-inline`）不仅拦截 HTML 属性 `onclick=`，也拦截 innerHTML 字符串里的内联 handler
  - 动态生成元素必须用 `createElement` + `addEventListener`，不能在 innerHTML 字符串里写 `onclick=`
  - 受影响：main.js WS 断连横幅、查询超时按钮；account.html 用户管理表格行

### 多用户模式启用与 config 降级禁用
- Date: 2026-06-18
- Context: 启用多用户后 admin/admin123 弱密码仍能登录（config 降级路径）
- Category: 环境配置
- Instructions:
  - `validateLoginCredentials`（login_handlers.go）是 DB 优先 → config 降级
  - 禁用 config 降级：清空 `config.yaml` 的 `web.auth.username`/`password_hash`，代码无需改（字段为空时返回 "login not configured"）
  - 首个用户通过 `/api/v1/users/register` 创建（0 用户时 public），创建后注册自动关闭
  - `handleGetAdminToken` 多用户模式（userID>0）下必须 `requireAdmin`，否则普通用户越权拿 admin token
