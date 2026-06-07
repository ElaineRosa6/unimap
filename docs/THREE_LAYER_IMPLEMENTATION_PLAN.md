# 三层采集架构实施计划

> 生成日期：2026-06-06 | 预估总工时：15-20 天 | 优先级：P1

## 实施概览

```
Phase 0: 基础设施准备 (2天)
    ↓
Phase 1: L1 Network层 (4天)
    ↓
Phase 2: L2 Hook层 (3天)
    ↓
Phase 3: 数据校验层 (3天)
    ↓
Phase 4: 浏览器探针 (2天)
    ↓
Phase 5: 集成测试与文档 (2天)
```

---

## Phase 0: 基础设施准备（2 天）

### 0.1 Router 架构重构

**目标**：新建独立的 `CollectionRouter`，不修改现有 `ScreenshotRouter`

**文件**：
- `internal/screenshot/collection_router.go`（新建）
- `internal/screenshot/collection_types.go`（新建）

**改动**：
- 新增 `CollectionLayer` 接口：
  ```go
  type CollectionLayer interface {
      Name() string
      IsAvailable() bool
      Collect(ctx context.Context, engine, query string) (*LayerResult, error)
      Priority() int // 数值越小优先级越高
  }
  ```
- 新增 `LayerResult` 结构：
  ```go
  type LayerResult struct {
      Layer       string              // "network" / "hook" / "dom"
      Assets      []model.UnifiedAsset
      RawResponse []byte              // 原始响应 hash（用于校验，不存完整数据）
      Total       int
      HasMore     bool
      Metadata    LayerMetadata       // 强类型，替代 map[string]interface{}
  }

  type LayerMetadata struct {
      ResponseSize   int64
      InterceptTime  time.Time
      EngineVersion  string
      CollectionMode string // "parallel" / "single"
  }
  ```
- 新建 `CollectionRouter` 独立模块：
  ```go
  type CollectionRouter struct {
      layers  []CollectionLayer
      logger  *logger.Logger
      mu      sync.RWMutex
  }

  func (r *CollectionRouter) CollectWithFallback(ctx, engine, query, queryID string) (*LayerResult, error) {
      for _, layer := range r.sortedLayers() {
          if !layer.IsAvailable() {
              continue
          }
          result, err := layer.Collect(ctx, engine, query)
          if err == nil && result != nil {
              return result, nil
          }
          r.logger.Warn("layer failed, falling back", zap.String("layer", layer.Name()), zap.Error(err))
      }
      return nil, ErrAllLayersFailed
  }
  ```
- `ScreenshotRouter` **不做任何修改**，保持截图路由职责

**验证**：`go test -race ./internal/screenshot/...`

### 0.2 测试基础设施

**目标**：为每层采集建立独立的测试 harness

**改动**：
- 新增 `internal/screenshot/layer_test_helpers.go`
- Mock 各层的响应数据（5 引擎 × 3 层 = 15 组 fixture）
- 新增 `testdata/network_responses/` 目录存放各引擎 API 响应样本

---

## Phase 1: L1 Network 层（4 天）

### 1.1 CDP Network 监听器

**目标**：在 chromedp 导航期间拦截 API 响应

**文件**：
- `internal/screenshot/network_collector.go`（新建）
- `internal/screenshot/network_types.go`（新建）

**改动**：
```go
// network_collector.go
type NetworkCollector struct {
    responses   sync.Map // url -> responseBody
    patterns    []URLPattern
    mu          sync.Mutex
    maxMemory   int64        // 全局内存上限
    usedMemory  atomic.Int64 // 当前已用内存
}

type URLPattern struct {
    Engine  string
    Pattern *regexp.Regexp
    Method  string // GET / POST
}

func (nc *NetworkCollector) Enable(ctx context.Context) error {
    // 创建独立的 chromedp context，不与 L3 DOM 层共享
    // 调用 chromedp.Run(ctx, network.Enable())
    // 监听 EventResponseReceived
    // 按 pattern 过滤并缓存响应
    // 超过 maxMemory 时丢弃最早的缓存
}

func (nc *NetworkCollector) Disable(ctx context.Context) error {
    // 显式调用 network.Disable()，清理 CDP 状态
    // 降级到 L3 前必须调用
}

func (nc *NetworkCollector) GetResult(engine string) (*LayerResult, bool) {
    // 从缓存中获取匹配的响应
    // 解析为 model.UnifiedAsset
}
```

**URL Pattern 配置**：

| 引擎 | Pattern | 响应解析 |
|------|---------|---------|
| FOFA | `api/v1/search` | `data.list[]` → UnifiedAsset |
| Hunter | `api/v2/search` | `data.list[]` → UnifiedAsset |
| ZoomEye | `search/result` | `list[]` → UnifiedAsset |
| Quake | `quake_api/v3/search` | `data[]` → UnifiedAsset |
| Shodan | `shodan/host/search` | `matches[]` → UnifiedAsset |

**验证**：
- 单元测试：Mock CDP 事件，验证过滤和解析逻辑
- 集成测试：实际 chromedp 导航 + Network 监听

### 1.2 Network 层健康检测

**文件**：`internal/screenshot/health.go`

**改动**：
- 新增 `NetworkLayerHealthChecker`：
  - 检测 CDP 连接是否支持 Network domain
  - 记录最近一次**成功完成监听周期**的时间戳（`Network.enable` → `loadingFinished`）
  - 连续 N 次**监听周期失败**（非"无拦截"）→ 标记为不可用
  - ❌ 不使用"无拦截"作为失效判据——"无拦截"可能是查询不涉及目标引擎或返回空结果

### 1.3 Network 层集成到 CollectionRouter

**文件**：`internal/screenshot/collection_router.go`

**改动**：
- 在 `NewCollectionRouter()` 中注册 `NetworkCollector` 作为 L1 layer
- 配置项：
  ```yaml
  screenshot:
    collection:
      network_layer:
        enabled: true
        timeout: 15s
        max_response_size: 10MB
  ```

### 1.4 测试

**文件**：`internal/screenshot/network_collector_test.go`

**用例**：
- `TestNetworkCollector_Enable` — 验证 Network.enable 调用
- `TestNetworkCollector_FilterByPattern` — 验证 URL 过滤
- `TestNetworkCollector_ParseFOFAResponse` — FOFA 响应解析
- `TestNetworkCollector_ParseHunterResponse` — Hunter 响应解析
- `TestNetworkCollector_Timeout` — 超时降级
- `TestNetworkCollector_LargeResponse` — 大响应截断

---

## Phase 2: L2 Hook 层（3 天）

### 2.1 Extension Hook 注入

**目标**：在 Extension 中 Hook fetch/XHR，拦截 API 响应

**文件**：
- `tools/extension-screenshot/src/hook.js`（新建）
- `tools/extension-screenshot/manifest.json`（修改）

**manifest.json 改动**：
```json
{
  "permissions": [
    "tabs", "tabCapture", "scripting", "storage", "activeTab", "cookies"
  ],
  "content_scripts": [
    {
      "matches": ["<all_urls>"],
      "js": ["src/hook.js"],
      "run_at": "document_start",
      "all_frames": false
    }
  ]
}
```

**⚠️ 不需要 `webRequest` 权限**：MV3 的 `webRequest` 是只读事件监听，无法读取响应体。fetch/XHR Hook 通过 content script 注入实现，已有 `host_permissions: ["<all_urls>"]` 足够。添加 `webRequest` 反而会触发 Chrome Web Store 安全审查和增加检测风险。

**`all_frames: false`**：仅注入主框架，避免广告/跟踪 iframe 重复拦截和误匹配。

**hook.js 核心逻辑**：
```javascript
// hook.js — 在 document_start 时机注入
(function() {
  const HOOK_CONFIG = {
    engines: {
      fofa:   { pattern: /\/api\/v1\/search/i, extract: (data) => data?.data?.list },
      hunter: { pattern: /\/api\/v2\/search/i, extract: (data) => data?.data?.list },
      zoomeye:{ pattern: /\/search\/result/i,  extract: (data) => data?.list },
      quake:  { pattern: /\/quake_api\/v3\/search/i, extract: (data) => data?.data },
      shodan: { pattern: /\/shodan\/host\/search/i, extract: (data) => data?.matches },
    }
  };

  // Hook fetch
  const originalFetch = window.fetch;
  window.fetch = async function(...args) {
    const response = await originalFetch.apply(this, args);
    try {
      const url = typeof args[0] === 'string' ? args[0] : args[0]?.url || '';
      const matched = matchEngine(url);
      if (matched) {
        const clone = response.clone();
        const data = await clone.json();
        reportIntercept(matched.engine, url, data);
      }
    } catch (e) { /* 静默失败 */ }
    return response;
  };

  // Hook XMLHttpRequest
  const originalXHROpen = XMLHttpRequest.prototype.open;
  const originalXHRSend = XMLHttpRequest.prototype.send;
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

  function matchEngine(url) {
    for (const [engine, config] of Object.entries(HOOK_CONFIG.engines)) {
      if (config.pattern.test(url)) return { engine, config };
    }
    return null;
  }

  function reportIntercept(engine, url, data) {
    chrome.runtime.sendMessage({
      type: 'network_intercept',
      engine, url, data,
      timestamp: Date.now()
    });
  }
})();
```

### 2.2 Extension 后台处理

**文件**：`tools/extension-screenshot/src/background.js`

**改动**：
- 新增 `networkIntercepts` Map 存储拦截到的数据
- 监听 `chrome.runtime.onMessage` 中的 `network_intercept` 类型
- 当收到 collect 任务时，优先检查是否有已拦截的网络数据
- 在 `reportTaskResult` 中附加 `network_data` 字段
- **大响应处理**：拦截数据超过 1MB 时，改用 `chrome.storage.session` 存储，通过 key 引用传递，避免 `sendMessage` 大消息限制

### 2.3 后端 Hook 结果处理

**文件**：`internal/screenshot/collection_router.go`

**改动**：
- `HookLayer.Collect()` 中增加对 `network_data` 字段的处理
- 新增 `parseNetworkInterceptData()` 解析 Hook 层数据
- 当 `network_data` 存在时，标记 `collection_method: "hook"`

### 2.4 测试

**文件**：`tools/extension-screenshot/src/hook.test.js`（新建）

**用例**：
- fetch Hook 能拦截匹配 URL 的响应
- XHR Hook 能拦截匹配 URL 的响应
- 非目标 URL 不触发拦截
- 拦截失败不影响正常请求
- 多引擎并发拦截正确分离

---

## Phase 3: 数据校验层（3 天）

### 3.1 结果校验器

**目标**：对多层返回的结果进行一致性校验

**文件**：`internal/screenshot/validator.go`（新建）

**改动**：
```go
type ValidationResult struct {
    IsConsistent    bool
    LayerResults    map[string]*LayerResult  // layerName -> result
    Discrepancies   []Discrepancy
    RecommendedLayer string
    Confidence      float64  // 0.0 ~ 1.0
}

type Discrepancy struct {
    Field    string
    Layer1   string
    Layer2   string
    Value1   interface{}
    Value2   interface{}
    Severity string // "info" / "warning" / "critical"
}

type ResultValidator struct {
    enginePriority map[string]int
}

func (v *ResultValidator) Validate(results map[string]*LayerResult) *ValidationResult {
    // 1. 数量校验：各层 asset 数量差异
    // 2. 关键字段校验：IP:Port 集合的交集/差集
    // 3. 字段完整性校验：各层提供的字段覆盖度
    // 4. 置信度计算：多层一致 → 高置信，单层 → 中置信
}
```

### 3.2 结果归并器

**文件**：`internal/screenshot/merger.go`（新建）

**改动**：
```go
type LayerMerger struct {
    priorityOrder []string // ["network", "hook", "dom"]
}

func (m *LayerMerger) Merge(results map[string]*LayerResult) *LayerResult {
    // 1. 按优先级排序
    // 2. 高优先级结果为基础
    // 3. 低优先级补充缺失字段
    // 4. 标记来源：asset.Extra["collection_layers"] = ["network", "hook"]
}
```

### 3.3 不一致告警

**文件**：`internal/screenshot/alerts.go`（新建）

**改动**：
- 当 `Discrepancy.Severity == "critical"` 时触发告警
- 告警内容：引擎名、差异详情、各层数量
- 集成现有 `internal/notify/` 通知系统
- 累计统计：`prometheus.CounterVec` 按引擎×层级计数

### 3.4 配置项

```yaml
screenshot:
  collection:
    validation:
      enabled: true
      parallel_collect: false       # true = 三层并行采集（耗资源）
      async_validate: true          # 单层模式下异步校验（先返回结果，后台校验告警）
      discrepancy_threshold: 0.3    # 数量差异超过 30% 触发告警
      alert_on_critical: true
    merger:
      priority: ["network", "hook", "dom"]
      fill_missing_fields: true     # 低优先级补充缺失字段
```

**⚠️ 校验层工作模式说明**：
- `parallel_collect: true`：多层同时采集，校验层实时交叉验证
- `parallel_collect: false` + `async_validate: true`（默认）：先返回最高优先级结果，后台异步用其他层校验，发现问题再告警
- `parallel_collect: false` + `async_validate: false`：校验层空转（不推荐，失去校验价值）

### 3.5 测试

**文件**：`internal/screenshot/validator_test.go`

**用例**：
- `TestValidator_ConsistentResults` — 三层一致 → 高置信
- `TestValidator_NetworkMoreAssets` — Network 多于 DOM → 信息级差异
- `TestValidator_DifferentIPs` — IP 不匹配 → 严重差异
- `TestValidator_SingleLayer` — 仅单层可用 → 中置信
- `TestMerger_PriorityOrder` — 高优先级字段覆盖低优先级
- `TestMerger_FillMissing` — 低优先级补充缺失字段
- `TestAlert_TriggerOnCritical` — 严重差异触发告警

---

## Phase 4: 浏览器探针（2 天）

### 4.1 Extension 心跳上报

**文件**：`tools/extension-screenshot/src/background.js`

**改动**：
```javascript
// 使用 chrome.alarms 保持 Service Worker 存活 + 定期探针上报
chrome.alarms.create('keepAlive', { periodInMinutes: 0.4 }); // 最小间隔 24s
chrome.alarms.onAlarm.addListener(async (alarm) => {
  if (alarm.name === 'keepAlive') {
    await reportProbe();
  }
});

async function reportProbe() {
  const tabs = await chrome.tabs.query({});
  const probe = {
    type: 'heartbeat',
    tab_count: tabs.length,
    active_tabs: tabs.filter(t => t.status === 'complete').length,
    current_urls: tabs.slice(0, 10).map(t => ({
      url: t.url,
      title: t.title?.substring(0, 100),
      status: t.status,
    })),
    browser_ua: navigator.userAgent,
    platform: navigator.platform,
    language: navigator.language,
    uptime_s: Math.floor((Date.now() - startTime) / 1000),
    pending_tasks: pendingTaskCount,
    last_error: lastError?.message || null,
    extension_version: chrome.runtime.getManifest().version,
  };

  await apiPost('/api/v1/screenshot/bridge/probe', probe, token);
}
```

**⚠️ 不使用 `for(;;)` 循环**：MV3 Service Worker 空闲 30s 终止、活跃最长 5min 强制终止。`for(;;)` 只会消耗 CPU 并加速终止判定。`chrome.alarms` 是 MV3 推荐的保活方式。

### 4.2 后端探针接收

**文件**：
- `web/screenshot_bridge_handlers.go`（新增 handler）
- `internal/screenshot/bridge_types.go`（新增结构）

**改动**：
```go
// bridge_types.go
type BridgeProbe struct {
    Timestamp    time.Time         `json:"timestamp"`
    TabCount     int               `json:"tab_count"`
    ActiveTabs   int               `json:"active_tabs"`
    CurrentURLs  []TabInfo         `json:"current_urls"`
    BrowserUA    string            `json:"browser_ua"`
    Platform     string            `json:"platform"`
    Language     string            `json:"language"`
    UptimeS      int64             `json:"uptime_s"`
    PendingTasks int               `json:"pending_tasks"`
    LastError    string            `json:"last_error"`
    Version      string            `json:"extension_version"`
}

type TabInfo struct {
    URL    string `json:"url"`
    Title  string `json:"title"`
    Status string `json:"status"`
}
```

**新增 API 端点**：
- `POST /api/v1/screenshot/bridge/probe` — 接收探针数据
- `GET /api/v1/screenshot/bridge/diagnostics` — 返回诊断快照（已有 `buildBridgeDiagnosticSnapshot`，增强探针数据）

### 4.3 诊断 API 增强

**文件**：`web/screenshot_bridge_handlers.go`

**改动**：在 `buildBridgeDiagnosticSnapshot()` 中加入最新探针数据：
```json
{
  "paired_clients": 1,
  "live_clients": 1,
  "last_task_pull_at": "2026-06-06T10:30:00Z",
  "probe": {
    "tab_count": 5,
    "active_tabs": 3,
    "browser_ua": "Mozilla/5.0 ...",
    "uptime_s": 3600,
    "pending_tasks": 0,
    "last_error": null,
    "extension_version": "1.0.0"
  }
}
```

### 4.4 测试

**文件**：`tools/extension-screenshot/src/probe.test.js`（新建）

**用例**：
- 心跳数据包含所有必需字段
- tab 数量正确统计
- uptime 递增
- 后端正确接收和存储探针数据

---

## Phase 5: 集成测试与文档（2 天）

### 5.1 端到端测试

**场景**：
1. 三层均可用 → 使用 Network 层结果
2. Network 不可用 → 降级 Hook 层
3. Network + Hook 不可用 → 降级 DOM 层
4. 三层均不可用 → 返回错误
5. 三层结果不一致 → 触发告警 + 使用最高优先级结果
6. Extension 心跳正常 → 诊断 API 返回探针数据
7. Extension 断开 → 健康检测标记不可用 → 自动切 CDP

### 5.2 负载测试

- 5 引擎 × 3 层 × 100 查询 = 1500 次采集
- 验证资源消耗在可接受范围
- 验证降级链延迟 < 30s

### 5.3 文档更新

| 文档 | 更新内容 |
|------|---------|
| `CLAUDE.md` | 新增三层采集架构说明 |
| `docs/ARCHITECTURE.md` | 更新数据流图 |
| `docs/API.md` | 新增 probe API 文档 |
| `docs/RUNBOOK.md` | 新增「Network 层失效」和「Hook 层失效」故障处理 |
| `configs/config.yaml.example` | 新增 collection 配置项 |

---

## 配置参考

```yaml
screenshot:
  mode: auto  # auto / cdp / extension（仅控制截图路由）
  collection:  # 独立于截图路由
    network_layer:
      enabled: true
      timeout: 15s
      max_response_size: 10485760  # 10MB
      max_memory_mb: 50            # 全局内存上限
      engines:
        fofa:
          pattern: "api/v1/search"
        hunter:
          pattern: "api/v2/search"
        zoomeye:
          pattern: "search/result"
        quake:
          pattern: "quake_api/v3/search"
        shodan:
          pattern: "shodan/host/search"
    hook_layer:
      enabled: true
      intercept_timeout: 5s
      max_send_message_bytes: 1048576  # 1MB，超过改用 storage.session
    dom_layer:
      enabled: true  # 始终启用作为兜底
      wait_time: 3s  # SPA 渲染等待
    validation:
      enabled: true
      parallel_collect: false
      async_validate: true          # 单层模式下异步校验
      discrepancy_threshold: 0.3
      alert_on_critical: true
    merger:
      priority: ["network", "hook", "dom"]
      fill_missing_fields: true
    probe:
      heartbeat_interval: 30s
      report_tabs: true
      max_tabs_reported: 10
```

---

## 风险登记表

| # | 风险 | 概率 | 影响 | 缓解 |
|---|------|------|------|------|
| ~~R1~~ | ~~`webRequest` 权限触发网站检测~~ | — | — | **已排除**：不需要 `webRequest` 权限 |
| R2 | Network 层 API 加密导致拦截失败 | 中 | 中 | 自动降级 L2/L3 |
| R3 | Hook 注入时机导致漏采 | 中 | 中 | `document_start` + 接受已知漏采窗口 + 降级 L3 |
| R4 | 多层并行增加资源消耗 | 中 | 低 | 默认单层优先 + CDP 上下文独立 + 大响应内存上限 |
| R5 | DOM 选择器频繁过期 | 中 | 中 | 不一致告警 + 多级 fallback |
| R6 | Extension MV3 Service Worker 生命周期限制 | 低 | 中 | `chrome.alarms` 定期唤醒（非 `for(;;)`） |
| R7 | L1/L3 共享 CDP 连接竞态 | 中 | 高 | 独立 chromedp context + 降级时显式 `Network.disable()` |
| R8 | 大响应内存峰值 | 中 | 中 | L1 全局内存上限 + L2 用 storage.session + 校验层流式 hash |

---

## ⚠️ 第二轮评审遗留缺陷（2026-06-06）— 动工前必须处理

> 对照实际代码复核后发现以下缺陷在第一轮评审后**仍然存活**。详见 `docs/THREE_LAYER_COLLECTION_ARCHITECTURE.md` 第 9 节。本计划的 Phase 顺序需按文末「修正后实施路径」调整。

| # | 级别 | 缺陷 | 证据 | 修复 |
|---|------|------|------|------|
| C-1 | 🔴 | L2 hook.js 运行在 ISOLATED world，覆盖 `window.fetch` 拦不到页面请求 | MV3 content script 默认 ISOLATED world，不共享 JS 全局；`capture.js:442` 靠 DOM 跨 world 共享才能工作 | manifest 声明 `world: MAIN`，或注入 `<script>` 到页面 MAIN world |
| C-2 | 🔴 | 修好 C-1 后 MAIN world 无 `chrome.runtime`，无法 `sendMessage` | `chrome.runtime` 仅 ISOLATED world 暴露 | MAIN world `postMessage` → ISOLATED 转发 → background，两段脚本 |
| H-1 | 🟠 | L1 与 L2 同源冗余（拦同一份 API 响应），非正交失效 | 改端点/加密/换协议会让 L1+L2 同时失效；仅 L3 正交 | 先做 L1，L2 暂缓再评估；校验层主比较改为 API层 vs DOM |
| H-2 | 🟠 | API 端点为猜测，文档内部不自洽，Shodan 列的是官方 API 非网页端点 | ARCH `/api/v1/search/http` ≠ PLAN/config `api/v1/search`；`/shodan/host/search` 网页前端不调 | Phase 0 前加抓包 spike |
| H-3 | 🟠 | 对加密/签名响应体无应对，强反爬引擎 L1/L2 归零 | 第 7 节自标 FOFA/ZoomEye 加密「中」，但前提是 `JSON.parse` 即用 | spike 确认是否加密；诚实记录 ROI |
| M-1 | 🟡 | collection 代码新建在 `internal/screenshot/` 包，重蹈职责膨胀 | PLAN 30/105/338/376 全在 screenshot 包内 | 迁 `internal/collection/` 独立包 |
| M-2 | 🟡 | `<all_urls>` 全网注入与"降低检测"矛盾 | PLAN 214-216 | `matches` 收窄至 5 引擎域名 |
| M-3 | 🟡 | L1/L3 独立 context 对限流引擎请求量翻倍 | `manager.go:185/377/430` 已各自 NewContext；Hunter 限流刚修 | 同 context 先 L1 后 L3 + `network.Disable()` 复位 |
| M-4 | 🟡 | `async_validate` 后台校验再访问引擎页，放大限流 | 默认 `async_validate: true` | 评估后台校验请求成本，限流引擎降频 |

### 修正后实施路径

1. **Phase -1（新增，spike，2 天）**：人工在 5 个引擎页面抓真实 XHR/fetch，确认端点、响应结构、是否加密 → 决定 L1/L2 是否成立（解 H-2/H-3）。
2. **Phase 0**：CollectionRouter 等迁到 `internal/collection/` 独立包（解 M-1）。
3. **Phase 1（L1 Network）**：收益最大、不涉及 world 问题，优先实现；L1/L3 同 context 复用（解 M-3）。
4. **L2 暂缓**：待 L1 验证 API 路可行后再评估是否仍需要（解 H-1）。若保留，按两段式 MAIN-world 注入 + postMessage 桥重写，`matches` 收窄至 5 引擎域名（解 C-1/C-2/M-2）。
5. 校验层主比较改为 `API层(L1) vs DOM(L3)`，单层模式后台校验对限流引擎降频（解 H-1/M-4）。
6. 工时重估：原 15-20 天 + spike 2 天 + L2 world 桥接返工，按实际 spike 结论再定 L2 是否纳入本期。
