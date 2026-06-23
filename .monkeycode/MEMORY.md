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

### map→struct 强类型迁移级联模式（引擎适配器）
- Date: 2026-06-23
- Context: Agent 在对 Shodan/Hunter/Fofa/DayDayMap/Censys 5 个引擎适配器进行 map[string]interface{} → typed struct 迁移时总结
- Category: 代码模式
- Instructions:
  - 迁移步骤（级联，每步必须同步更新测试）：
    1) 定义类型化 struct（字段与 API JSON 响应一致，`float64` for JSON numbers）
    2) 改 `parse*Response` 的 JSON unmarshal target（匿名 struct → 命名 struct）+ rawData 赋值（`map[string]interface{}` → `&Struct{}` 指针）
    3) 改 `Normalize` 的 type assertion（`item.(map[string]interface{})` → `item.(*Struct)`）
    4) 改 `normalize*Item` 签名，移除 getStr/getInt 闭包（直接读 struct 字段），改为包级函数
    5) 移除 `asset.Extra = data`（旧 map 透传 — 除非有意保留）
  - 测试同步更新：RawData 构造从 `map[string]interface{}{"ip": ..., "port": float64(80)}` → `&Struct{IP: ..., Port: 80}`
  - 遵循该模式的引擎：Shodan (ShodanMatch), Hunter (HunterItem), DayDayMap (DayDayMapItem)
  - 所有 `Source` 字段从 `h.Name()`/`s.Name()` 改为硬编码字符串（包级函数无 receiver）

### 行数组→结构体映射模式（Fofa 特殊模式）
- Date: 2026-06-23
- Context: Fofa API 返回 `[][]interface{}` 行数组而非 JSON 对象数组，需要列映射
- Category: 代码模式
- Instructions:
  - API 返回行为 `[["1.2.3.4", 80, "http", ...], [...]]`，字段顺序由 `fields` query param 决定
  - 定义 `fofaRowToItem(row []interface{}, fieldNames []string) *FofaItem` 函数
  - 用 switch 按 fieldNames[j] 映射 row[j] → struct 字段
  - `parseFofaSearchResponse` 中调用 `fofaRowToItem(row, fieldNames)` 替代 `map[string]interface{}` 逐列赋值
  - 数字字段（port/status_code）用 `v.(float64)` 断言（JSON 统一 decode 为 float64）
  - 字符串字段用 `v.(string)` 断言，nil 值跳过

### 反规范化条目模式（Censys 特殊模式）
- Date: 2026-06-23
- Context: Censys v3 API 返回嵌套 services 数组，旧代码在 Normalize 中二次迭代
- Category: 代码模式
- Instructions:
  - API 返回单 host 含多个 services：`{result: {resource: {ip, services: [{port, http}, ...], location, autonomous_system}}}`
  - 旧模式：rawData 存一份带嵌套 services 的 map → Normalize 中二次迭代 `data["services"]`
  - 新模式：在 `parse*Response` 中将每个 service 拆分为独立 `CensysRawEntry`（合并 host-level 元数据：location/AS/DNS/ip）
  - Normalize 不再需要二次迭代嵌套数组，每个 entry 直接生成 1 个 UnifiedAsset
  - 入口结构体 `CensysRawEntry` 包含全部字段（service 字段 + host 元数据），为"反规范化"视图
  - 14 个辅助 struct：CensysLocation/CensysAS/CensysDNS/CensysHTTP/CensysTLS/CensysSoftware 等
  - 所有 extract/parse 辅助函数从 `map[string]interface{}` 改为 `*CensysRawEntry`

### DayDayMap POST API 测试修复
- Date: 2026-06-23
- Context: DayDayMap 适配器从 GET+query params 改为 POST+header auth 后，测试 mock 未同步更新导致 3 个测试失败
- Category: 测试方法
- Instructions:
  - 旧 mock：检查 `r.URL.Query().Get("apikey")` → 新 mock：检查 `r.Header.Get("api-key")`
  - 旧响应格式：`{"code": 0, "message": "success", "data": [...], "total": N}` → 新格式：`{"code": 200, "msg": "success", "data": {"list": [...], "total": N}}`
  - `GetQuota()` 改为直接返回 error（API 不提供配额端点），测试从期望成功改为期望 error
  - 修复后 27 个 DayDayMap 测试全部通过

### json.Number 处理数值/字符串双态字段（ZoomEye ASN）
- Date: 2026-06-23
- Context: ZoomEye API 的 `asn` 字段可能返回数值 `15169` 或字符串 `"15169"`，string 字段遇上 JSON number 会静默为空
- Category: 代码模式
- Instructions:
  - struct 字段用 `json.Number` 类型：`ASN json.Number \`json:"asn"\``
  - `json.Number` 同时接受 JSON number 和 string，底层都是字符串存储
  - 读取时调用 `.String()` 统一为字符串：`asset.ASN = it.ASN.String()`
  - 适用于 API 返回类型不一致的字段（如某些引擎 ASN 有时是 int 有时是 string）
  - 替代方案：`interface{}` + 类型 switch，但会破坏 struct 的纯类型化

### json.RawMessage 处理变体响应结构（Quake data 字段）
- Date: 2026-06-23
- Context: Quake API 的 `data` 字段可能是 `[{...}]`（数组）或 `{"list": [...]}` （对象），需要先探测再解析
- Category: 代码模式
- Instructions:
  - 响应 struct 中 `Data` 字段用 `json.RawMessage`：`Data json.RawMessage \`json:"data"\``
  - 先尝试 `json.Unmarshal(data, &[]QuakeItem)`（直接数组格式）
  - 失败则尝试 `json.Unmarshal(data, &struct{List []QuakeItem})`（对象包裹格式）
  - 还失败则 fallback 到 `map[string]interface{}` + marshal/unmarshal 常见键名（`list`/`service_list`/`items` 等）
  - 最后将 `[]QuakeItem` 转为 `[]interface{}`（`&item` 指针）放入 RawData
  - 避免使用 `interface{}` + 类型 switch（`case []interface{}` / `case map[string]interface{}`）的旧模式

### ZoomEye 点号字段映射（country.name / country.code 等）
- Date: 2026-06-23
- Context: ZoomEye v2 API 新格式返回 `"country.name": "China"` 而非嵌套对象
- Category: 代码结构
- Instructions:
  - struct 中 JSON tag 支持点号：`CountryName string \`json:"country.name"\``
  - Go JSON decoder 天然支持点号 key（`encoding/json` 将 `country.name` 作为整体 key 匹配）
  - 新旧格式可共存：struct 同时定义 `CountryName`（新）和 `GeoInfo`（旧），解析函数优先新格式再 fallback 旧格式

### ZoomEye 适配器 parseZoomEyeNetwork LastSeen 映射
- Date: 2026-06-23
- Context: 迁移后删除了旧的 `timestamp`→`last_seen` 映射分支，ZoomEye API 本身返回 `last_seen`
- Category: 代码结构
- Instructions:
  - ZoomEye v2 API 的 JSON 字段是 `last_seen`（非 `timestamp`）
  - Extension DOM 提取也使用 `last_seen` 字段名（`capture.js` 选择器 `div.heading div.timestamp` 提取后放入 `last_seen`）
  - 迁移后 `parseZoomEyeNetwork` 直接读 `it.LastSeen`（JSON 自动映射），无需类型断言
  - Shodan 同理：API v1 无 timestamp，LastSeen 由 Extension DOM 路径填充
