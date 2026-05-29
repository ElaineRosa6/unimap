# 浏览器运行时策略：CDP 与 Extension 的职责边界

## 1. 背景

UniMap 同时支持 Chrome DevTools Protocol（CDP）和 Chrome Extension Bridge。最初目标是提高浏览器能力的冗余度，并覆盖不同运行环境：

- Windows / Linux / macOS
- 图形化桌面环境
- 无图形化服务端环境
- 需要复用真实浏览器登录态的搜索引擎页面
- 批量目标站截图、定时任务、篡改检测等后台任务

实际落地后，CDP 与 Extension 并不是完全对等的两种截图引擎。它们更适合被定义为两类浏览器运行时：

- **CDP**：服务端/无头主通道。
- **Extension**：桌面浏览器会话桥。

本文档用于固定该架构判断，避免后续在截图、浏览器查询、登录态检测和无头部署之间继续混淆。

## 2. 总体结论

CDP 和 Chrome Extension 都保留，但职责不同：

| 能力 | 推荐主通道 | 定位 |
|------|------------|------|
| API 查询 | Engine Adapter | 主查询路径，不依赖浏览器 |
| 批量目标站截图 | CDP | 服务端自动化、无头运行、定时任务 |
| 搜索引擎结果页截图 | CDP 或 Extension | CDP 适合自动化；Extension 适合复用用户登录态 |
| Web-only 查询 / DOM 采集 | Extension 优先，CDP 可降级 | 辅助/降级能力，受页面 DOM 结构影响 |
| 登录态复用 | Extension 或 CDP remote session | 真实用户浏览器会话最可靠 |
| Linux 无图形环境 | CDP headless | Extension 不应作为无头冗余 |
| 桌面人工辅助 | Extension | 打开页面、采集页面、复用浏览器状态 |

因此，产品口径应调整为：

> API 查询始终是主查询路径；CDP 是服务端/无头截图与浏览器采集通道；Chrome Extension 是桌面环境下的会话增强能力。浏览器查询是 API 查询的补充路径，特别用于免费账号 API 不返回结果或 Web 端已登录的场景。

需要特别说明：从免费用户视角看，只有 Hunter 和 Quake 能稳定通过官方 API 获取查询结果；FOFA、ZoomEye、Shodan 等引擎即使保留 API adapter，也需要依赖 CDP 或 Extension 访问 Web 端并采集 DOM，才能在未购买对应 API 能力时拿到结果。因此浏览器查询不是核心 API 查询的替代品，但必须作为产品可用性路径保留。

具体落地步骤、配置开关、测试矩阵和回滚策略见 [BROWSER_QUERY_FALLBACK_PLAN.md](./BROWSER_QUERY_FALLBACK_PLAN.md)。

## 3. 运行环境矩阵

| 运行环境 | CDP | Extension | 推荐策略 |
|----------|-----|-----------|----------|
| Windows 桌面 | 支持 | 支持 | 默认 CDP，登录态增强时启用 Extension |
| macOS 桌面 | 支持 | 支持 | 默认 CDP，登录态增强时启用 Extension |
| Linux 桌面 | 支持 | 支持 | 默认 CDP，登录态增强时启用 Extension |
| Linux 服务器无图形环境 | 支持 headless | 不推荐 | 使用 CDP headless |
| Docker / CI | 支持 headless | 不推荐 | 使用 CDP headless，并显式配置 Chrome/Chromium |
| 用户本机已登录搜索引擎 | 支持 remote session | 支持 | Extension 或 CDP remote session |
| 后台定时任务 | 支持 | 不推荐作为主路径 | CDP 优先，Extension 只做桌面辅助 |

Extension 理论上可以通过虚拟显示环境运行，但这会引入额外运维复杂度，不适合作为 UniMap 的默认无头方案。

## 4. 能力分层

### 4.1 Core 模式

默认模式，面向服务端和无图形环境。

能力：

- API 查询
- CDP headless 截图
- 批量截图
- 定时任务
- 篡改检测

推荐配置：

```yaml
screenshot:
  enabled: true
  mode: cdp
  engine: cdp
  headless: true
  extension:
    enabled: false
```

### 4.2 Desktop Assist 模式

面向桌面用户，强调真实浏览器登录态复用和人工辅助。

能力：

- 打开搜索引擎结果页
- 复用已登录浏览器会话
- DOM 结构化采集
- 必要时进行结果页截图

推荐配置：

```yaml
screenshot:
  enabled: true
  mode: extension
  engine: extension
  headless: false
  extension:
    enabled: true
    fallback_to_cdp: true
```

### 4.3 Hybrid / Auto 模式

面向同时需要服务端自动化和桌面增强的场景。

推荐策略不是简单全局切换，而是按任务路由：

| 任务类型 | 首选 | 降级 |
|----------|------|------|
| 目标站截图 | CDP | Extension |
| 批量 URL 截图 | CDP | Extension |
| 搜索引擎结果页打开 | Extension | CDP |
| 搜索引擎 DOM 采集 | Extension | CDP |
| 搜索引擎结果页证据截图 | Extension 或 CDP | 另一通道 |
| 登录态检测 | Extension / CDP remote | Cookie 配置检测 |

推荐配置：

```yaml
screenshot:
  enabled: true
  mode: auto
  headless: true
  extension:
    enabled: true
    fallback_to_cdp: true
```

`mode: auto` 下，当前默认行为等价于 `priority: cdp` 且 `fallback: true`。如无特殊需求，不必显式配置这两个字段。

## 5. 查询与截图的边界

### 5.1 API 查询是主线

FOFA、Hunter、ZoomEye、Quake、Shodan 等引擎的 API adapter 应继续作为主查询路径。它们的特点是：

- 结构化结果稳定
- 易于分页、缓存、去重、归并
- 适合 CLI / Web / GUI / 定时任务
- 易于观测配额和错误

浏览器查询不应替代 API 查询。但当引擎 API 因免费账号、额度、付费限制或未配置 API Key 无法返回结果时，查询流程应允许通过浏览器 runtime 继续采集 Web 端结果。实际产品行为应理解为：

- **API-first**：优先使用已配置且可用的 API adapter。
- **Browser-capable fallback**：API 不可用、未配置 Key 或用户主动启用浏览器查询时，可通过 CDP / Extension 打开搜索页并采集结构化数据。
- **Engine reality**：免费用户只有 Hunter、Quake 通常能走 API 返回结果；FOFA、ZoomEye、Shodan 更依赖浏览器采集链路。

### 5.2 浏览器查询是辅助线

浏览器查询适合以下场景：

- 用户未配置 API Key，但已经在浏览器登录 Web 端
- 免费账号 API 无结果，但 Web 端可以人工或登录态访问结果
- 需要打开搜索结果页供人工查看
- 需要采集页面上的结构化结果作为降级路径
- 需要保留搜索结果页截图作为证据

风险：

- DOM 选择器易受页面改版影响
- 登录墙、验证码、风控策略会影响稳定性
- 不适合高并发后台任务
- Extension 依赖桌面浏览器在线和已配对

## 6. 当前实现中的注意点

当前代码已经具备基本架构基础：

- `internal/screenshot/router.go`：CDP / Extension / Auto 路由。
- `internal/screenshot/provider.go`：统一 Provider 接口。
- `internal/screenshot/provider_cdp.go`：CDP Provider。
- `internal/screenshot/bridge_service.go`：Extension Bridge 任务队列。
- `internal/adapter/web_only_base.go`：Web-only 查询通过浏览器 backend 采集结构化结果。
- `internal/service/query_app_service.go`：浏览器打开、采集、截图联动流程。
- `web/server.go` / `web/query_handlers.go`：Web-only adapter 与显式浏览器查询都应接入当前可用的浏览器 provider（CDP、Extension 或 Auto Router）。

后续需要重点收敛的问题：

1. **健康检查语义**
   - CDP 在没有 remote debug URL 时不应简单视为绝对健康。
   - 当前实现中，`CDPHealthChecker` 在 `RemoteDebugURL` 为空时直接返回健康，表示假设本地 Chrome 可按需启动；这会混淆“可启动”和“本地 Chrome 不存在”两种状态。
   - 至少应区分：可启动、本地 Chrome 不存在、remote CDP 在线、remote CDP 离线。
   - Extension 健康不应只看 BridgeService 是否启动，还应看是否存在 live client、最近拉取任务、最近回调。
   - 当前实现只检查 `BridgeService` 是否存在、是否启动，以及队列长度是否超过阈值；尚未检查 live client 和最近任务活动。

2. **配置语义**
   - `screenshot.engine` 是 legacy 字段。
   - `screenshot.mode` 应成为用户可理解的主字段：`cdp`、`extension`、`auto`。
   - `priority` 和 `fallback` 只应在 `auto` 模式下生效。
   - `screenshot.fallback` 控制 Auto Router 在 CDP / Extension 之间的路由降级。
   - `screenshot.extension.fallback_to_cdp` 控制传统 extension-first 截图流程失败后是否回落到 CDP；它不等同于顶层 `fallback`。
   - 前端设置页需要避免同时暴露多个含义相近的选择器。

3. **任务路由**
   - 当前 Auto 更偏全局模式选择。
   - 应用层已经存在基于 `browser_action` 的流程区分，但 Router 层仍按当前模式全局选择 provider。
   - 更理想的是将应用层 action / 任务语义下沉到 Router 层，由 Router 按任务类型选择 provider。
   - 例如目标站截图优先 CDP，搜索引擎 Web 采集优先 Extension。
   - `resolveProvider` 当前还有一层隐式降级：先按健康状态选择最优模式；如果该模式的 provider 实例为空，再在 `fallback` 启用时尝试另一个 provider。

4. **前端行为命名**
   - 理想语义应收敛为：`open` 只打开页面，`collect` 采集结构化结果，`capture` 截图。
   - 当前代码语义与该命名不完全一致：`open` 只打开页面，`capture` 实际执行结构化采集但不截图，`collect` 实际执行结构化采集并进行证据截图。
   - 后续应修正代码、API 或 UI 文案，使 action 名称与行为一致；如果保留“采集 + 截图”，建议命名为 `collect_and_capture` 或在 UI 上明确表达。

5. **Bridge 可靠性**
   - Extension Bridge 当前通过 `BridgeService.executeWithRetry` 对 timeout、提交失败和 connection 类错误进行有限重试。
   - 这提高了桌面桥接通道的瞬时故障容忍度，但不能替代 live client 健康检查和任务活动观测。

## 7. 产品对外描述

建议在 README、设置页和运维文档中统一使用以下表述：

> UniMap 在无图形环境下通过 CDP headless 提供截图能力；在桌面环境下可选启用 Chrome Extension Bridge，以复用真实浏览器会话完成登录态页面打开、搜索结果采集和截图增强。API 查询始终是主查询路径，浏览器查询是辅助/降级路径。

## 8. 决策

本项目后续按以下原则演进：

1. 保留 CDP 与 Extension 双能力。
2. CDP 是默认主通道，覆盖服务端、无头、批量、定时任务。
3. Extension 是桌面增强通道，覆盖登录态复用、人工辅助、Web 页面采集。
4. 不把 Extension 作为无头部署的默认冗余方案。
5. API 查询继续作为核心查询路径。
6. 浏览器查询只作为辅助、增强或降级能力。
7. Auto 模式最终应演进为按任务路由，而不是单一全局模式切换。
