# 引擎测绘、浏览器自动化与前端整改计划

## 背景

本次检查聚焦以下问题：

- 多引擎网络空间搜索引擎当前的调用查询方式。
- 启用认证后，Chrome 扩展桥接无法连接的问题。
- CDP / 扩展双模式在“引擎测绘自动化 + 截屏记录”场景下的真实能力边界。
- 前端页面风格割裂、Cookie / Headless 区域层次弱、可用性差的问题。

## 现状结论

### 1. 多引擎查询当前分成两条链路

1. 统一查询主链路：`UQL -> EngineOrchestrator -> adapter.Translate -> adapter.Search -> Normalize -> UnifiedAsset`
2. 浏览器联动链路：`QueryAppService.RunBrowserQueryAsync -> OpenSearchEngineResult / CaptureSearchEngineResult`

结论：

- 统一查询主链路目前只真正支持 API 模式引擎。
- 如果某个引擎未配置 API Key，则 Web 启动时会注册 `*AdapterWebOnly()`。
- `WebOnlyAdapterBase.Translate()` 和 `WebOnlyAdapterBase.Search()` 直接返回 `web-only mode: ... not supported`，因此不能参与真正的统一测绘查询。
- 当前所谓“browser query”不是用浏览器替代 API 做引擎结果采集，而是在已有查询流程之外，额外打开浏览器结果页，并按配置触发截图。

### 2. 扩展模式当前主要是“截图执行器”，不是“浏览器测绘执行器”

扩展桥接当前通过 `BridgeTask{URL,...}` 拉起页面并回传截图文件路径，核心能力是：

- 打开某个搜索结果 URL 并截图。
- 打开目标站点 URL 并截图。
- 批量 URL 截图。

当前缺口：

- 没有“扩展端提取搜索结果列表并回传结构化资产”的能力。
- 没有“CDP/扩展 二选一执行真实网页测绘”的统一抽象。
- 前端的“查询时同步在浏览器打开结果页”当前被 `cdpOnline` 强绑定，扩展在线时也不能启用该能力。

### 3. 认证开启后扩展连接失败的高概率根因已确认

根因链路：

1. `Server.Start()` 在启用认证时对整个 HTTP handler 套用了 `adminAuthMiddleware()`。
2. `/api/screenshot/bridge/*` 路由不在 `isPublicPath()` 白名单中。
3. 扩展桥接接口设计依赖“本地回环 + bridge bearer token”鉴权。
4. 但请求在进入桥接 handler 前，先被全局 admin token / session 认证拦截。

结果：

- 扩展没有 Web 登录 session。
- 扩展通常也不会携带 `X-Admin-Token`。
- 因此会在桥接自身鉴权前被 401 或跳转，表现为“加了认证后扩展连接不上”。

### 4. 双模式高可用框架已有，但入口与能力不统一

已有能力：

- `ScreenshotRouter` 已支持 `cdp` / `extension` 健康探测、优先级和 fallback。
- `ScreenshotAppService` 已支持 `engine=extension` 和 `fallback_to_cdp`。

现有问题：

- `ScreenshotRouter.CaptureSearchEngineResult()` / `CaptureTargetWebsite()` 当前固定调用 `resolveProvider(ModeCDP)`，没有按当前活动模式或用户手动选择模式执行，语义不清。
- 配置层存在 `screenshot.engine`、`screenshot.mode`、`screenshot.priority`、`screenshot.fallback` 多套概念，容易冲突。
- 前端只有“连接 CDP”和“刷新扩展状态”，没有统一的“执行模式选择器”。

### 5. 前端样式与信息架构存在系统性问题

发现的问题：

- `scheduler.html`、`monitor.html`、`batch-screenshot.html` 都内联大量 `<style>`，并复写局部视觉体系。
- 首页使用全局 `style.css`，其余页面部分走独立视觉，造成颜色、间距、边框、卡片、按钮风格明显不一致。
- 首页 Cookie / Headless 区域全部堆在一个表单块里，缺少分组、层级和主次关系。
- “Headless 模式备用 Cookie 填写”是折叠块，但内容密度高、状态弱、缺少卡片分区与引导文案。
- 页面中仍存在大量行内样式，难以维护，也会持续加重风格漂移。

## 代码级证据

### 查询与引擎注册

- `cmd/unimap-web/main.go`
- `internal/adapter/web_only_base.go`
- `internal/service/query_app_service.go`

### 浏览器联动与截图

- `internal/service/query_app_service.go`
- `internal/service/screenshot_app_service.go`
- `internal/screenshot/router.go`
- `internal/screenshot/manager.go`

### 扩展桥接与认证冲突

- `web/server.go`
- `web/middleware_auth.go`
- `web/router.go`
- `web/screenshot_bridge_handlers.go`

### 前端样式割裂

- `web/templates/index.html`
- `web/templates/scheduler.html`
- `web/templates/monitor.html`
- `web/templates/batch-screenshot.html`
- `web/static/css/style.css`
- `web/static/js/main.js`

## 整改目标

### 目标 A：让用户可以手动选择 CDP 或扩展执行浏览器自动化

系统 SHALL 支持以下执行模式：

- `cdp`
- `extension`
- `auto`

并满足：

- 用户可在前端显式选择本次执行模式。
- `auto` 使用健康探测 + fallback。
- `cdp` / `extension` 为强制模式，不被静默切换，仅在失败时明确返回错误或按用户选择决定是否允许降级。

### 目标 B：让“浏览器测绘自动化”成为真实能力，而不是“打开页面再截图”

系统 SHOULD 将浏览器执行拆成两类能力：

1. `query-open`：只打开结果页。
2. `query-capture`：打开结果页并截图。
3. `query-collect`：打开结果页、抽取结构化结果、回传统一资产。

其中 `query-collect` 才是用户所需的“利用网络空间搜索引擎进行测绘自动化”。

### 目标 C：修复认证后扩展不可连接

桥接接口必须与后台管理认证解耦，采用专用本地桥接安全模型：

- 回环地址限制。
- bridge pairing token。
- 可选回调签名与 nonce 防重放。

### 目标 D：统一前端视觉系统

将首页、定时任务、监控、批量截图页统一到同一套：

- 页面容器
- 顶部导航
- 卡片、表单、按钮、状态徽章
- 分区标题
- 表格与空状态
- 折叠区和高级配置区

## 分阶段整改计划

### 第一阶段：修复阻断问题

1. 调整桥接认证边界
- 将 `/api/screenshot/bridge/*` 从 `adminAuthMiddleware` 的全局拦截中剥离。
- 保留回环校验与 bridge token 校验。
- 明确哪些 bridge 接口允许匿名配对，哪些必须携带 bridge bearer token。

2. 修复前端“浏览器查询只能依赖 CDP”的错误约束
- 将 `browser_query` 前端校验从 `cdpOnline` 改为“当前所选浏览器执行模式可用”。
- 当扩展在线且模式为 `extension` 或 `auto` 可用时，允许浏览器联动。

3. 统一模式配置语义
- 明确 `execution_mode` / `capture_mode` 概念，移除或兼容混乱字段。
- 后端单点解析：`cdp | extension | auto`。

### 第二阶段：补齐双模式执行抽象

1. 引入统一浏览器执行接口
- 抽象 `BrowserAutomationProvider`：`OpenSearchResult`、`CaptureSearchResult`、`CollectSearchResult`、`CaptureTarget`。
- CDP 和扩展分别实现该接口。

2. 修正 `ScreenshotRouter` 选择逻辑
- 按用户本次选择模式或路由当前活动模式执行，而不是固定 `ModeCDP`。
- 为强制模式和自动模式分别提供明确入口。

3. 扩展端补齐“结构化结果采集”能力
- 扩展任务类型从单一 URL 截图扩展为：`open`、`capture`、`collect`。
- `collect` 任务返回结构化搜索结果 JSON，而不只是 `image_path`。

### 第三阶段：补齐网页测绘自动化主链路

1. 设计 Browser Query Provider
- 当引擎为 API 模式时，默认走 API 统一查询。
- 当引擎为 Web-only 或用户指定浏览器测绘时，走 Browser Query Provider。

2. 为每个搜索引擎实现网页采集策略
- FOFA 结果列表采集。
- Hunter 结果列表采集。
- ZoomEye 结果列表采集。
- Quake 结果列表采集。
- Shodan 网页结果采集可评估是否落在后续阶段。

3. 统一结果归并
- 浏览器采集结果映射回 `model.UnifiedAsset`。
- 与 API 模式复用同一归并、去重、导出流程。

### 第四阶段：前端统一改版

1. 建立共享页面骨架
- 提取统一 header / nav / page section / status chip / metrics card 样式。
- 清除 `scheduler.html`、`monitor.html`、`batch-screenshot.html` 中的大块内联样式。

2. 重构首页查询面板
- 拆分为：查询输入、引擎选择、浏览器执行模式、登录态 / 连接态、Cookie 备用配置、高级选项。
- 将 Headless Cookie 区做成带状态标记的卡片式折叠面板。
- 用差异化颜色区分：引擎状态、CDP 状态、扩展状态、风险提示、备用配置。

3. 统一定时任务页和其他页面风格
- 任务统计卡片、标签页、表格、弹窗、表单控件全部复用全局组件样式。
- 监控页与批量截图页同样迁移到统一样式体系。

## 建议实施顺序

1. 先修复桥接认证冲突。
2. 再修复模式选择与前端联动逻辑。
3. 然后补齐双模式浏览器自动化抽象。
4. 最后做全站 UI 收敛与页面重构。

## 风险与注意事项

- 扩展桥接接口放开到 public path 时，必须保留 loopback 限制，不能退化成公网可访问接口。
- 浏览器网页采集会受引擎页面结构变化影响，需要为每个引擎建立可回归的解析测试样例。
- 扩展模式若承担 `collect` 能力，需要定义稳定的数据回传协议和版本字段。
- 前端改版不要只改首页，应同步清理内联样式源头，否则后续仍会继续分叉。

## 建议验收标准

1. 开启 Web 认证后，扩展仍能完成配对、拉取任务和回传结果。
2. 用户可在 UI 中选择 `cdp`、`extension`、`auto`。
3. 选择 `extension` 时，浏览器联动不再要求 CDP 在线。
4. Web-only 引擎可以通过浏览器采集模式返回结构化结果，而不是仅截图。
5. 首页、定时任务、监控、批量截图四页达到统一视觉和交互风格。
