# 三层采集架构设计文档

> 生成日期：2026-06-06 | 状态：设计阶段 | 关联：`docs/EXTENSION_ANTI_SCRAPING_ARCHITECTURE.md`

## 1. 背景

当前 UniMap 采用 **CDP + Extension 双路互备** 架构，通过 `ScreenshotRouter` 健康检测在两条链路间切换。两条链路均基于 **DOM 解析** 采集数据，失效模式相同——当搜索引擎改版导致 DOM 结构变化时，两条链路可能同时失效。

从工程可靠性角度，需要引入多层采集，使不同层级的失效模式正交，实现真正的高可用。

## 2. 目标架构

```
Backend (Go)
    │
    ├── ScreenshotRouter (不变，仅负责截图路由)
    │       CDP 截图 ←→ Extension 截图
    │
    └── CollectionRouter (新建，独立负责采集路由)
            │
            ├── 优先级降级链 ──────────────────────────────┐
            │                                               │
            ▼                                               ▼
    L1: Network层 (CDP)                    L2: Hook层 (Extension)
        │  Network.responseReceived           │  fetch / XHR Hook
        │  拦截 API JSON 响应                  │  拦截网络请求返回值
        │                                     │
        ▼ 失败                                ▼ 失败
    L3: DOM层 (CDP / Extension)
        │  querySelector 解析页面结构
        │
        ▼
    数据校验层
        │  Hash 比较 / 字段一致性检查
        │
        ▼
    结果归并 → 入库
```

**职责分离**：
- `ScreenshotRouter`：截图路由（CDP vs Extension），保持现有逻辑不变
- `CollectionRouter`：采集路由（Network vs Hook vs DOM），新建独立模块
- 两者共享 `Provider` 接口和 `HealthChecker`，但路由逻辑各自独立
- 避免截图模式 × 采集层级的组合爆炸（截图 2 种 × 采集 3 种 = 6 种独立行为，非耦合）

## 3. 三层详细设计

### 3.1 L1: Network 层 (CDP Network.responseReceived)

**原理**：通过 CDP 协议监听浏览器网络请求，直接获取搜索引擎 API 返回的 JSON 数据。

**优势**：
- 权威数据源，无需解析 DOM
- 不受前端改版影响
- 数据结构化，无需类型转换

**劣势**：
- 依赖 API 接口稳定性（接口签名/加密变化会导致失效）
- WebSocket/GraphQL 协议变化需要适配

**实现要点**：
- 在 `chromedp` 导航前启用 `Network.enable`
- 监听 `Network.responseReceived` 事件
- 按 URL pattern 过滤目标 API 响应（如 FOFA 的 `/api/v1/search`）
- 读取 `Network.getResponseBody` 获取完整 JSON
- 解析为 `[]model.UnifiedAsset`

**⚠️ CDP 上下文隔离**：L1（Network 监听）和 L3（DOM 解析）都依赖 CDP，但使用不同的 CDP domain（Network vs Runtime/DOM）。两者应使用**独立的 chromedp context**，避免：
- L1 的 `Network.enable()` 状态残留影响 L3
- 降级时需要显式重置 CDP 状态（`Network.disable()`）
- 并行采集模式下的 CDP 连接竞态

**引擎 API 端点**：

| 引擎 | API 响应 URL Pattern | 响应格式 |
|------|---------------------|---------|
| FOFA | `/api/v1/search/http` | `{data: {list: [{ip, port, host, ...}]}}` |
| Hunter | `/api/v2/search` | `{data: {list: [{ip, port, ...}]}}` |
| ZoomEye | `/search/result` | `{list: [{ip, portinfo: {...}}]}` |
| Quake | `/quake_api/v3/search` | `{data: [{ip, port, ...}]}` |
| Shodan | `/dns/resolve` 或 `/shodan/host/search` | `{matches: [{ip_str, port, ...}]}` |

### 3.2 L2: Hook 层 (Extension fetch/XHR Hook)

**原理**：在 Chrome Extension 中 Hook `window.fetch` 和 `XMLHttpRequest`，拦截搜索引擎的网络请求返回值。

**优势**：
- 看到最终渲染前的原始数据
- 即使接口签名变化，只要页面能正常请求就有数据
- 运行在浏览器内部，不增加检测风险

**劣势**：
- `document_start` 注入仍有漏采窗口（页面内联 `<script>` 可能先于 content script 执行）
- SPA 框架可能使用非标准网络层
- 大响应通过 `chrome.runtime.sendMessage()` 传递不稳定（实践上限约几 MB）

**实现要点**：
- **不需要 `webRequest` 权限**：MV3 的 `webRequest` 是只读事件监听，无法读取响应体。fetch/XHR Hook 通过 content script 注入实现，无需额外权限
- `manifest.json` 仅需 `content_scripts` 配置（已有 `<all_urls>` host_permissions 足够）
- 在 content script 中注入 Hook 代码：
  ```javascript
  // Hook fetch
  const originalFetch = window.fetch;
  window.fetch = async function(...args) {
    const response = await originalFetch.apply(this, args);
    const url = typeof args[0] === 'string' ? args[0] : args[0].url;
    if (isTargetAPI(url)) {
      const clone = response.clone();
      const data = await clone.json();
      captureNetworkResult(url, data);
    }
    return response;
  };
  ```
- **XHR Hook 竞态修复**：在 `open()` 中注册 `load` 监听器（闭包捕获当次 URL），而非在 `send()` 中注册：
  ```javascript
  XMLHttpRequest.prototype.open = function(method, url, ...rest) {
    this._hookUrl = url;
    this._hookMethod = method;
    return originalXHROpen.call(this, method, url, ...rest);
  };
  XMLHttpRequest.prototype.send = function(...args) {
    const capturedUrl = this._hookUrl; // 闭包捕获，不依赖 this._hookUrl
    this.addEventListener('load', function() {
      try {
        const matched = matchEngine(capturedUrl);
        if (matched) {
          const data = JSON.parse(this.responseText);
          reportIntercept(matched.engine, capturedUrl, data);
        }
      } catch (e) { /* 静默失败 */ }
    });
    return originalXHRSend.apply(this, args);
  };
  ```
- 大响应改用 `chrome.storage.session` 或 IndexedDB 传递，避免 `sendMessage` 大消息限制
- 通过 `chrome.runtime.sendMessage` 将拦截到的数据发送给 background script
- 需要识别各引擎的 API URL pattern

**⚠️ 漏采窗口已知限制**：`document_start` 不保证早于页面内联脚本。对于 SPA 框架的首次 API 请求，L2 可能漏采。此限制通过降级到 L3（DOM 解析）弥补，不影响整体可用性。

### 3.3 L3: DOM 层 (已有)

**当前实现**：
- CDP 路径：`internal/screenshot/dom_selectors.go` — Go 侧 JS 注入
- Extension 路径：`tools/extension-screenshot/src/capture.js` — `ENGINE_SELECTORS` 配置

**5 引擎 DOM 选择器覆盖**：

| 引擎 | CDP 选择器 | Extension 选择器 | 备选策略 |
|------|-----------|-----------------|---------|
| FOFA | `table.el-table__body tr` | `.hsxa-meta-data-item` | 6 种 fallback |
| Hunter | `.q-table tbody tr` | `q-table tbody tr` | 通用 table fallback |
| ZoomEye | `.search-result-item` | `.search-result-item` | card fallback |
| Quake | `.el-table tbody tr` | `el-table tbody tr` | 通用 table fallback |
| Shodan | `.result` | `.row.l-search-results .result` | card fallback |

**现状评估**：✅ 功能完整，5 引擎全覆盖，有 login wall 检测和多级 fallback。

## 4. 数据校验层

### 4.1 交叉验证策略

当多层同时返回结果时，进行一致性校验：

```
L1 Network 结果 ──┐
                   ├──→ Hash/字段比较 ──→ 一致性报告
L2 Hook 结果 ─────┤
                   ├──→ 不一致 → 告警 + 标记
L3 DOM 结果 ──────┘
```

**校验维度**：
- **数量校验**：各层返回的 asset 数量是否一致（允许合理偏差）
- **关键字段校验**：IP:Port 对是否匹配
- **完整性校验**：是否有某层缺失了其他层有的字段

### 4.2 结果归并优先级

```
L1 Network > L2 Hook > L3 DOM
```

原因：Network 层数据最接近权威数据源，DOM 层最容易受页面变化影响。

### 4.3 不一致告警

当检测到层间结果差异时：
- 记录差异详情到日志
- 触发告警通知（Webhook/飞书）
- 在结果中标记 `collection_discrepancy: true`
- 累计统计各引擎的不一致率，用于判断是否需要更新选择器

**⚠️ 校验层工作模式**：
- **并行采集模式**（`parallel_collect: true`）：多层同时采集，校验层可实时交叉验证
- **单层优先模式**（`parallel_collect: false`，默认）：仅使用最高优先级层，校验层降级为"异步校验"——先返回结果，后台异步用其他层校验，发现问题再告警
- 单层模式下校验层**不能**空转，否则失去了存在的价值

## 5. 浏览器探针

### 5.1 当前状态

| 能力 | 状态 | 说明 |
|------|------|------|
| 存活检测 | ⚠️ 隐式 | 1s 轮询隐式表明存活 |
| Token 健康 | ✅ | `activeBridgeLiveTokens()` 15s 窗口 |
| 队列过载 | ✅ | `QueueLen() > 50` 告警 |
| 活跃度 | ✅ | 最近任务拉取/回调时间戳 |
| Tab 数量 | ❌ | 未上报 |
| 当前 URL | ❌ | 未上报 |
| 浏览器 UA | ❌ | 未上报 |
| 内存使用 | ❌ | 未上报 |

**L1 健康检测设计要点**：
- ❌ **错误做法**：连续 N 次无拦截 → 标记不可用（"无拦截" ≠ "不可用"，可能是查询不涉及目标引擎或返回空结果）
- ✅ **正确做法**：基于**是否成功完成完整的 Network 监听周期**（从 `Network.enable` 到收到 `loadingFinished` 事件），而非是否有数据被拦截
- 检测维度：CDP 连接是否支持 Network domain、最近一次成功监听周期的时间戳、连续 N 次监听周期失败 → 标记不可用

### 5.2 增强方案

在 Extension 的 `bridgeLoop` 中附加探针数据：

```javascript
// background.js bridgeLoop 中附加
const probe = {
  type: 'heartbeat',
  tab_count: tabs.length,
  current_urls: tabs.map(t => t.url),
  browser_ua: navigator.userAgent,
  memory_mb: performance?.memory?.usedJSHeapSize
    ? Math.round(performance.memory.usedJSHeapSize / 1024 / 1024)
    : null,
  uptime_s: Math.floor((Date.now() - startTime) / 1000),
  pending_tasks: pendingTaskCount,
};
```

后端新增 `BridgeProbe` 数据结构，存储到 `BridgeState` 中，暴露给诊断 API。

**⚠️ MV3 Service Worker 存活策略**：
- ❌ **错误做法**：`for(;;)` 循环保持活跃（Chrome 空闲 30s 终止，活跃最长 5min 强制终止，且会加速终止判定）
- ✅ **正确做法**：使用 `chrome.alarms` API 定期唤醒 Service Worker：
  ```javascript
  // background.js
  chrome.alarms.create('keepAlive', { periodInMinutes: 0.4 }); // 最小间隔 24s
  chrome.alarms.onAlarm.addListener((alarm) => {
    if (alarm.name === 'keepAlive') {
      // 执行心跳或探针上报
    }
  });
  ```
- 或使用长连接 `chrome.runtime.connect()` 保持 port 存活（适用于有 popup/options 页面的场景）

## 6. 与现有架构的兼容性

### 6.1 不变部分

- `Provider` 接口不变（6 个方法签名不变）
- `ScreenshotRouter` 的健康检测和 failover 机制不变
- Bridge 认证体系（pairing + admin token + HMAC）不变
- 多引擎查询系统的 adapter/merger/circuit breaker 不变

### 6.2 变化部分

| 组件 | 变化 |
|------|------|
| `CollectionRouter`（新建） | 独立的采集路由，优先级降级链 |
| `ScreenshotRouter` | **不变**，保持截图路由职责 |
| `manager.go` | 新增 Network 层监听逻辑 |
| `capture.js` | 新增 fetch/XHR Hook（无需 `webRequest` 权限） |
| `manifest.json` | 新增 content_scripts 注入配置（不新增权限） |
| `bridge_types.go` | 新增 `BridgeProbe` 结构体 |
| `health.go` | 新增 Network 层健康检测（基于监听周期，非拦截计数） |
| `router.go` | 新增结果校验和归并逻辑 |

### 6.3 降级兼容

```
截图请求 → ScreenshotRouter (不变)
              CDP ←→ Extension

采集请求 → CollectionRouter (新建)
              │
              L1 Network 可用？──→ 是 ──→ 使用 Network 结果
                      │
                      否
                      ▼
              L2 Hook 可用？──→ 是 ──→ 使用 Hook 结果
                      │
                      否
                      ▼
              L3 DOM 可用？──→ 是 ──→ 使用 DOM 结果（当前行为）
                      │
                      否
                      ▼
              返回错误
```

任何一层不可用时自动降级，对上层调用完全透明。
截图和采集的路由逻辑独立，互不影响。

## 7. 引擎反爬难度评估

| 引擎 | API 加密 | DOM 稳定性 | Network 拦截难度 | 综合评估 |
|------|---------|-----------|-----------------|---------|
| FOFA | 中（Base64 + 签名） | 高（Vue SPA 稳定） | 低 | 三层均可 |
| Hunter | 低 | 中（Quasar UI） | 低 | 三层均可 |
| ZoomEye | 中 | 低（改版频繁） | 中 | L1/L2 优先 |
| Quake | 低 | 中（Element UI） | 低 | 三层均可 |
| Shodan | 低 | 高 | 低 | 三层均可 |

## 8. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| ~~`webRequest` 权限增加 Extension 被检测风险~~ | ~~低~~ | **已排除**：不需要 `webRequest` 权限，fetch/XHR Hook 通过 content script 注入实现 |
| Network 层 API 加密/签名变化 | L1 失效 | 自动降级到 L2/L3 |
| Hook 注入时机不当导致漏采 | L2 数据不完整 | `document_start` 时机 + 接受已知漏采窗口（SPA 首次请求），降级 L3 弥补 |
| 多层并行增加资源消耗 | 性能下降 | 默认单层优先 + CDP 上下文独立 + 大响应内存上限 |
| DOM 选择器过期 | L3 失效 | 多级 fallback + 不一致告警触发更新 |
| L1/L3 共享 CDP 连接竞态 | 采集异常 | L1 和 L3 使用独立 chromedp context，降级时显式重置 CDP 状态 |
| 大响应内存峰值 | OOM 风险 | L1 全局内存上限 + L2 大消息改用 storage.session + 校验层流式 hash 比较 |
| MV3 Service Worker 生命周期限制 | Extension 断连 | 使用 `chrome.alarms` 定期唤醒，而非 `for(;;)` 循环 |

## 9. 第二轮评审遗留缺陷（2026-06-06，对照实际代码）

> 第一轮评审修复了 13 项设计缺陷。第二轮对照 `capture.js` / `manifest.json` / `manager.go` / `go.mod` 复核后，发现以下缺陷**仍然存活**，动工前必须处理。证据均来自现有代码，非推测。

### 🔴 CRITICAL

**C-1　L2 Hook 运行在 ISOLATED world，覆盖 `window.fetch` 拦不到页面请求**
MV3 content script 默认运行在 ISOLATED world，与页面共享 DOM 但**不共享 JS 全局对象**。在 ISOLATED world 执行 `window.fetch = ...` / `XMLHttpRequest.prototype.open = ...` 只改隔离环境的副本，页面（MAIN world）调用的 fetch/XHR 完全不受影响——一条都拦不到。
- 对比证据：`capture.js:442` 用 `chrome.scripting.executeScript` 做 DOM 提取能工作，是因为 DOM 跨 world 共享；`window.fetch` 不是。文档把"能读 DOM"错误外推成"能 hook fetch"。
- 第一轮修的"XHR `open()` 闭包捕获 URL"竞态，是在一个根本不会触发的 world 里做的精装修。
- **修复**：manifest content_scripts 声明 `"world": "MAIN"`（Chrome 111+），或由 ISOLATED content script `document.createElement('script')` 注入页面 MAIN world。文档当前完全未提 `world`。

**C-2　修好 C-1 后 `chrome.runtime.sendMessage` 在 MAIN world 不存在，两个需求互斥**
MAIN world 没有 `chrome.runtime`（扩展 API 仅在 ISOLATED world 暴露），但 `reportIntercept()` 直接调 `chrome.runtime.sendMessage`。
- 覆盖页面 fetch → 必须 MAIN world；回传 background → 必须 ISOLATED world。一段脚本无法同时满足。
- **修复**：MAIN world 用 `window.postMessage` → ISOLATED content script 转发 → background，两段脚本 + postMessage 桥。

### 🟠 HIGH

**H-1　L1 与 L2 同源冗余，非"正交失效"**
L1（CDP `Network.responseReceived`）与 L2（Extension fetch hook）拦截的是**同一个对象**——引擎 API 的同一份 JSON 响应。失效触发条件完全一致：改端点、加密响应体、换 GraphQL/WebSocket 都会让两者同时失效。真正与之正交的只有 L3（DOM）。
- 因此实际是"**两层同源 API 拦截 + 一层 DOM**"，而非三层正交。L2 边际收益最低、实现风险（C-1/C-2）最高。
- 连带影响校验层：L1 vs L2 交叉校验几乎恒等，唯一有信息量的是 `API层(L1/L2) vs DOM(L3)`，第 4.1 节"三方交叉验证"夸大了校验价值。
- **建议**：先只做 L1；L2 暂缓，待 L1 验证 API 路可行后再评估是否仍需要。

**H-2　API URL pattern 未经验证，且文档内部不自洽**
端点表无来源标注，三处打架：ARCH 3.1（`/api/v1/search/http`）≠ PLAN Phase1（`api/v1/search`）≠ config 示例（`api/v1/search`）。更实质：Shodan 列的 `/shodan/host/search` 是 `api.shodan.io` 官方 REST API，**网页前端不调它**（走内部接口）；其余四家同理，浏览器内部端点 ≠ 官方 API 文档端点。
- **修复**：Phase 0 之前加抓包 spike，人工在 5 个引擎页面抓真实 XHR/fetch，确认端点、响应结构、是否加密，再决定 L1/L2 是否成立。

**H-3　对加密/签名响应体无应对**
本文档第 7 节自标 FOFA「中（Base64+签名）」、ZoomEye「中」，但 L1/L2 全部前提是"响应体 = `JSON.parse` 即用"。若 list 载荷加密，拦到的是密文，解析直接失败，唯一回答是"降级 L3"。即**对反爬最强的引擎，L1/L2 退化为零，回到现状 DOM**，三层架构对这些引擎无新增可用性。需在文档诚实写出此 ROI。

### 🟡 MEDIUM

**M-1　collection 代码塞进 `internal/screenshot/` 包，重蹈被评审修掉的职责膨胀**
PLAN 把 `collection_router.go`/`network_collector.go`/`validator.go`/`merger.go` 全部新建在 `internal/screenshot/` 内，同包两 Router 编译期耦合，"独立"仅是文件名。**修复**：另起 `internal/collection/` 包。

**M-2　`<all_urls>` 注入与"降低检测风险"自相矛盾**
PLAN 让 hook.js `matches: ["<all_urls>"]` 注入用户访问的每个网站，暴露面比 webRequest 更大（全网页面被改写 `window.fetch`）。**修复**：`matches` 收窄到 5 个引擎域名（`*://fofa.info/*` 等）。

**M-3　L1/L3 独立 chromedp context 成本被低估**
`manager.go:185/377/430` 每个截图任务已各自 `NewContext`。L1 与 L3 各用独立 context 意味着同一次采集开两个 target、导航两次同一查询页，对限流引擎（Hunter 刚修过限流）请求量翻倍。**更省**：同 context 内先挂 Network 监听拿 L1，失败再同 tab 跑 L3，靠 `network.Disable()` 复位。

**M-4　单层模式 `async_validate` 后台校验放大限流**
后台校验要再访问一次引擎页面，与 M-3 叠加对限流敏感引擎是隐患。文档只论证"不空转"，未论证后台校验的请求成本。

### 🟢 LOW

- **L-1**　`performance.memory` 非标准、仅 Chrome、多数情况可能返回 null，探针内存字段实用性有限。
- **L-2**　15-20 天工时未含 H-2 端点逆向 spike 与 C-1/C-2 world 桥接返工，偏乐观。

### 修订后的风险表补充

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| **L2 content script 运行于 ISOLATED world** | **L2 整层失效** | **声明 `world: MAIN` 或注入页面脚本 + postMessage 桥（C-1/C-2）** |
| **L1/L2 同源冗余，非正交** | 可靠性增益被高估 | 先做 L1，L2 暂缓再评估（H-1） |
| **API 端点为猜测且可能加密** | L1/L2 对强反爬引擎归零 | Phase 0 前抓包 spike 验证（H-2/H-3） |
| collection 代码与 screenshot 同包 | 职责膨胀复发 | 迁 `internal/collection/` 独立包（M-1） |
| `<all_urls>` 全网注入 | 暴露面/指纹增大 | 收窄至 5 引擎域名（M-2） |

## 10. 相关文档

- `docs/EXTENSION_ANTI_SCRAPING_ARCHITECTURE.md` — 反爬虫架构分析
- `docs/ARCHITECTURE.md` — 系统分层架构
- `docs/RUNBOOK.md` — 运维故障处理
- `internal/screenshot/router.go` — ScreenshotRouter 实现
- `internal/screenshot/health.go` — 健康检测实现
- `tools/extension-screenshot/src/capture.js` — Extension DOM 采集
