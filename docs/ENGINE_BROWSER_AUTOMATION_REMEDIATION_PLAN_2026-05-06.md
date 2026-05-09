# 引擎测绘、浏览器自动化与前端整改计划

## 本次核实结论（2026-05-07，执行完毕）

逐行核实代码后确认：**整改计划中的步骤 1、2、3A、3B、4A、4B 已全部实现，Step 5 首页视觉已收敛**。`go build ./...` 与 `go test -race ./internal/screenshot/... ./web/...` 均通过。

| 步骤 | 文档描述 | 代码状态 | 关键证据 |
|------|---------|---------|---------|
| **步骤 1** | 统一桥接状态前端展示 | **完成** | `main.js:239-270` 已消费 `router_mode/router_cdp_healthy/router_ext_healthy`，首页文案已改为"通过当前选择的浏览器执行模式打开结果页"（`index.html:158`） |
| **步骤 2** | 补强扩展配对端到端验收 | **完成** | `scripts/bridge_e2e.ps1` 已覆盖配对→任务→回调→可观测字段全链路；`buildBridgeDiagnosticSnapshot()` 已含 `last_pair_at/last_task_pull_at/last_callback_at`（`screenshot_bridge_handlers.go:768-771`）; 含 `router_mode/router_cdp_healthy/router_ext_healthy`（`ibid:792-794`） |
| **步骤 3A** | 前端浏览器动作拆分 UI 层 | **完成** | `index.html:138-157` 存在 `browser-action` radio 组（open/capture/collect），默认 capture，保留旧 checkbox 防腐层（`main.js:448-468`） |
| **步骤 3B** | 后端浏览器动作分流 | **完成** | `query_app_service.go:68-166` `RunBrowserQueryAsync()` 已按 action 分流；`query_handlers.go:64` 已输出 `browserAction` 字段 |
| **步骤 4A** | 扩展 collect 返回协议升级 | **完成** | `bridge_types.go:34` `StructuredCollectedData` 已追加；`router.go:558-575` `CollectSearchEngineResult` 已实现防腐层解析；`BROWSER_COLLECT_STRATEGY.md` 已创建 |
| **步骤 4B** | 扩展端 collect 能力 | **完成** | `background.js:42-141` 已按 action 分流（open/collect/screenshot）；`capture.js:169-315` 已实现 `extractEngineAssets()` 覆盖 4 个引擎 |
| **步骤 5** | 前端视觉系统收敛（首页） | **首页完成** | `index.html:111-159` 模式选择器和动作组已结构化为统一组件，帮助文案已同步更新 |

---

## 本次复核结论

截至 2026-05-07，我核对了 `docs/ENGINE_BROWSER_AUTOMATION_REMEDIATION_PLAN_2026-05-06.md` 涉及的实现、桥接链路、浏览器联动链路和首页前端逻辑，结论如下：

1. 文档中的部分"阻断问题"已经修复，但文档还没有同步更新。
2. 你上次提交前发现的两个问题里：
   - "启用认证后扩展连接配对不上"这一条，从当前代码看主因已修复，但仍可能存在状态表达不一致导致的误判。
   - "配对上后会循环加载查询示例/搜索页面"这一条，从当前代码看已经被专门规避，现有轮询逻辑不会再主动打开引擎结果页。
3. 当前更大的问题已经从"完全不可用"转成"能力边界、状态语义、前后端表达不一致"。

## 对你提到的两个问题的核实

### 1. 扩展连接配对不上

当前结论：原始阻断问题已基本修复，但仍有残余风险。

已确认修复：

- `web/middleware_auth.go` 已将 `/api/screenshot/bridge/` 放入 `isPublicPath()`，不再被全局后台认证先拦截。
- 桥接接口当前走自己的安全模型：loopback 限制 + bridge bearer token。
- `web/screenshot_bridge_handlers.go` 中的 `pair`、`tasks/next`、`mock/result` 仍会校验 loopback 与 token，有独立安全边界。

残余风险：

- 前端把"扩展在线"判定为 `engine === 'extension' && extensionEnabled && liveClients > 0`，这会把"扩展已配对但当前系统截图模式不是 extension"的情况显示成离线，容易误导用户。
- `buildBridgeDiagnosticSnapshot()` 里的 `engine` 来自 `config.Screenshot.Engine`，而真实执行时很多路径已经改为看 `ScreenshotRouter` 当前模式；状态源不是一个口径。
- `paired_clients` 与 `live_clients` 是基于 token 数量和最近活跃时间推导，不等于"当前一定可执行 collect/open/capture"。

结论：

- "认证一开就完全配对不上"的旧问题，当前代码里不再成立。
- 但"看起来没配对上"或"明明配对过却被 UI 说离线"的误判，仍然可能存在。

### 2. 配对后循环加载查询示例 / 无用搜索引擎查询页面

当前结论：从现有代码看，这个问题已被修复。

关键依据：

- `web/cookie_handlers.go` 的 `handleCookieLoginStatus()` 在 `cdpConnected` 或 `extPaired` 为真时，不再调用打开页面或校验结果页的逻辑。
- 代码注释明确写了：不要打开页面，不要干扰用户浏览器会话，不要在轮询时触发不必要截图。
- 前端 `web/static/js/main.js` 中 `initLoginStatusPoll()` 虽然仍每 15 秒轮询 `/api/cookies/login-status`，但这个接口现在只回报状态，不会触发引擎页面加载。

现状补充：

- 首页"查询示例"点击事件只会把示例文本填入输入框，不会自动提交查询。
- 浏览器打开搜索引擎结果页的动作只在用户显式启用 `browser_query` 后，由 `RunBrowserQueryAsync()` 触发。

结论：

- 你上次看到的"配对后循环加载无用查询页面"，从当前代码路径看已不再存在。
- 如果现在仍出现类似现象，更可能是扩展端自身逻辑、浏览器扩展缓存旧脚本，或外部实际运行版本与当前仓库代码不一致，而不是这里的 Web 轮询逻辑再次触发。

## 原整改项实现状态

### A. 手动选择 `cdp` / `extension` / `auto`

状态：**已完成**。

已完成：

- 首页已提供 `cdp`、`extension`、`auto` 单选项。
- `web/static/js/main.js` 会调用 `/api/screenshot/set-mode` 切换模式。
- `internal/screenshot/router.go` 已实现 `SetMode()`、`CurrentMode()`、`resolveProvider()`。
- 浏览器联动 `OpenSearchEngineResult()`、截图 `CaptureSearchEngineResult()`、结构化采集 `CollectSearchEngineResult()` 已统一通过 router 按当前模式分发。
- ~~首页帮助文案仍写"通过已连接的 CDP 浏览器打开所选引擎结果页"~~ → 已改为"通过当前选择的浏览器执行模式打开所选引擎结果页"（`index.html:158`）
- ~~桥接状态 UI 仍把 `config.Screenshot.Engine` 当成主要状态源~~ → `refreshBridgeStatus()` 已优先消费 `router_mode`, `router_cdp_healthy`, `router_ext_healthy`

### B. 让浏览器测绘自动化成为真实能力

状态：**已闭环**。

已完成：

- Web-only adapter 不再只是返回 `not supported`；`internal/adapter/web_only_base.go` 的 `Search()` 已可调用浏览器后端 `CollectSearchEngineResult()`。
- `web/server.go` 在 router 初始化后会执行 `unifiedSvc.SetWebOnlyBrowserBackend(...)`，将浏览器采集能力注入 Web-only 引擎。
- `ScreenshotRouter` 和 `ExtensionProvider` 已暴露 `CollectSearchEngineResult()`。
- ~~扩展 `collect` 当前只是返回一个非常弱的 `CollectResult`~~ → `BridgeResult.StructuredCollectedData` 已新增（`bridge_types.go:34`），router.go:566-575 优先解析结构化数据。
- ~~`BridgeResult` 只有 `CollectedData string`~~ → 已同时支持 `StructuredCollectedData map[string]interface{}` 和旧 `CollectedData string` 防腐层。
- `parseStructuredCollectedData()` 可将 items[] 映射到 `model.UnifiedAsset`。

### C. 修复认证后扩展不可连接

状态：**已完成**。

已完成：

- `/api/screenshot/bridge/*` 已从全局 admin 鉴权豁免。
- 配对、取任务、回传结果都要求 loopback，请求不会直接暴露成公网桥接口。
- `pairing token + callback signature + nonce 防重放` 代码已实现，默认取决于 `config.Screenshot.Extension.CallbackSignatureRequired`。
- `scripts/bridge_e2e.ps1` 已覆盖配对→任务→回调→可观测字段完整生命周期，支持 `StrictSignature` 和 `RotateToken` 开关。

### D. 统一前端视觉系统

状态：首页已完成，其余页面待后续迭代。

- 首页模式选择器和浏览器动作组已结构化为统一组件（`index.html:122-157`）。
- `scheduler.html`、`monitor.html`、`batch-screenshot.html` 的统一设计系统收敛留待后续迭代。

## 已解决的矛盾点

### ~~1. 状态来源不一致~~ → 已解决

`buildBridgeDiagnosticSnapshot()` 已返回 `router_mode/router_cdp_healthy/router_ext_healthy`（`screenshot_bridge_handlers.go:792-794`），`refreshBridgeStatus()` 已优先消费这些字段（`main.js:243-284`），不再依赖 `config.Screenshot.Engine`。

### ~~2. "已配对"与"可执行"被混为一谈~~ → 已解决

`refreshBridgeStatus()` 的 `bridgeOnline` 不再只看 token 数量；按 `effectiveMode` 结合 `cdpHealthy/extHealthy` 综合判断（`main.js:260-267`）。

### ~~3. `browser_query` 的语义仍然偏旧~~ → 已解决

前端已暴露 `browser-action` radio 组（open/capture/collect）（`index.html:138-154`），后端 `RunBrowserQueryAsync()` 已按 action 分流（`query_app_service.go:129-159`）。

### ~~4. 首页文案与现有实现冲突~~ → 已解决

首页文案已同步更新为"通过当前选择的浏览器执行模式打开所选引擎结果页"（`index.html:158`）。

### ~~5. Web-only 查询链路已接通，但返回模型仍过弱~~ → 已解决

`BridgeResult.StructuredCollectedData` 已支持结构化数据，`parseStructuredCollectedData()` 可映射到 `UnifiedAsset`（`router.go:648-742`）。

### ~~6. 扩展端完全无 action 分流~~ → 已解决

`background.js:80-121` 已按 action 分流（open/collect/screenshot）；`capture.js:169-315` 已实现 `extractEngineAssets()` 覆盖 FOFA/Hunter/ZoomEye/Quake。

---

## 安全执行切片计划

> 核心原则：禁止直接生成最终代码。所有破坏性改动通过"加新字段 + 优先读新字段 + 旧字段回退"实现防腐层，外部调用零感知。

### 步骤 1：统一桥接状态前端展示 ~~（已完成）~~

**具体动作**：修改 `web/static/js/main.js` 的 `refreshBridgeStatus()` 函数，改为消费后端已返回的 `router_mode`、`router_cdp_healthy`、`router_ext_healthy` 字段，替换当前基于 `config.Screenshot.Engine` 的判定逻辑。同时更新首页说明文案为"通过当前选择的浏览器执行模式打开结果页"。

**状态**：已完成。`main.js:239-284` 已消费 router 状态字段，`index.html:158` 帮助文案已同步更新。

**目的**：消除"已修复但 UI 仍误导"的状态语义不一致问题。

**目的**：消除"已修复但 UI 仍误导"的状态语义不一致问题。

**影响面**：仅 `main.js` 前端展示逻辑 + `index.html` 一段文案。不涉及后端 API 变更。

**验收标准**：
- 前端在 `auto`、`cdp`、`extension` 三种模式下，桥接状态徽章和提示文字准确反映实际健康状态
- `router_mode` 与前端模式选择器一致时，不再显示"扩展桥接未启用"类误导文案
- 构建通过，`go test -race ./...` 无回归

---

### 步骤 2：补强扩展配对端到端验收 ~~（已完成）~~

**具体动作**：完善 `scripts/bridge_e2e.ps1` 验收脚本，覆盖 `web.auth.enabled=true` 时真实扩展的完整生命周期：配对 → 拉取 open 任务 → 拉取 capture 任务 → 回传结果 → 状态可观测。在 `buildBridgeDiagnosticSnapshot()` 中增加 `last_pair_at`、`last_task_pull_at`、`last_callback_at` 时间戳字段。

**状态**：已完成。`bridge_e2e.ps1` 已覆盖配对→任务→回调→可观测验证；`buildBridgeDiagnosticSnapshot()` 已含 3 个时间戳字段 + 3 个 router 字段。

---

### 步骤 3A：前端浏览器动作拆分（UI 层） ~~（已完成）~~

**具体动作**：将首页"查询时同步在浏览器打开结果页"checkbox 替换为三个互斥选项：① 仅打开结果页（open） ② 打开并截图（capture） ③ 打开并采集结构化结果（collect）。前端通过新字段 `browser_action` 传递选择，同时保留旧 `browser_query` bool 字段以保持向后兼容。

**状态**：已完成。`index.html:138-154` 已实现 radio 组；`main.js:448-468` `getBrowserAction()`/`isBrowserQueryModeEnabled()` 已实现防腐层兼容。

---

### 步骤 3B：后端浏览器动作分流（API + 服务层） ~~（已完成）~~

**具体动作**：
1. WebSocket 和 REST 查询 API 接收 `browser_action` 参数（与 `browser_query` 并存）
2. `RunBrowserQueryAsync` 按 `browser_action` 分流：`open` 只调用 `OpenSearchEngineResult`，`capture` 走现有的 open+capture，`collect` 调用 `CollectSearchEngineResult` 并将结果注入查询响应
3. 保留 `browser_query=true` 的旧路径，行为等同于 `browser_action=capture`（防腐层）

**状态**：已完成。`query_app_service.go:88-159` 已按 action 分流，`browser_query` 回退为 `capture`。

---

### 步骤 4A：扩展 collect 返回协议升级（后端防腐层） ~~（已完成）~~

**具体动作**：
1. `BridgeResult` 新增字段 `StructuredCollectedData map[string]interface{}`，**保留** `CollectedData string` 不动
2. `ExtensionProvider.CollectSearchEngineResult()` 中：优先解析 `StructuredCollectedData` 到 `CollectResult.Assets`、`Total`、`HasMore`；若为空则回退到 `CollectedData` 填 `Title` 的旧逻辑
3. 新建 `docs/BROWSER_COLLECT_STRATEGY.md`，定义 FOFA/Hunter/ZoomEye/Quake 四个引擎的 DOM 选择器和字段映射规则

**状态**：已完成。`bridge_types.go:34` 已追加字段；`router.go:566-575` 已实现防腐层解析；`BROWSER_COLLECT_STRATEGY.md` 已创建。

---

### 步骤 4B：扩展端 collect 能力实现（本仓库内） ~~（已完成）~~

**具体动作**（在 `tools/extension-screenshot/` 内）：

1. **`capture.js` 新增 `extractEngineAssets(engine, tabId)`**：按引擎类型从搜索结果页 DOM 中提取结构化数据（URL、标题、协议、端口、IP 等），返回 `items[]` 数组
2. **`background.js` 的 `handleTask()` 增加 action 分流**：
   - `action === "open"`：只做 `ensureTab()` + `waitForPageReady()`，不截图、不回传 image_data
   - `action === "collect"`：`ensureTab()` → `waitForPageReady()` → `extractEngineAssets()` → 回传 `structured_collected_data`（包含 items、total、has_more）
   - `action === "screenshot"` 或无 action：保持现有截图逻辑不变（防腐层）
3. **`api.js` 的回传格式**：新增 `structured_collected_data` 字段，与 `image_data` 并存

**状态**：已完成。`background.js:42-141` 已按 action 分流；`capture.js:169-315` 已实现 `extractEngineAssets()` + `normalizeCollectPayload()`。

---

### 步骤 5：前端视觉系统收敛 ~~（首页已完成）~~

**具体动作**：基于已稳定的状态语义和浏览器动作，统一 `scheduler.html`、`monitor.html`、`batch-screenshot.html` 的视觉设计，消除内联样式，复用 `layout.html` 共享模板和 CSS tokens。

**状态**：首页已完成。`index.html:122-157` 模式/动作组已统一结构化为共享模板组件。其余页面（scheduler/monitor/batch-screenshot）留待后续迭代。

---

## 依赖关系总览

```
步骤 1 ───────────────────────────────────────→ 步骤 5
步骤 2 ───────────────────────────────────────→ 步骤 4B
步骤 4A ──────────────────────────────────────→ 步骤 3B → 步骤 5
步骤 3A ──────────────────────────────────────→ 步骤 3B
步骤 4B ──────────────────────────────────────→ 最终验收（本仓库内联调）
```

## 防腐层设计总览

| 防腐层 | 位置 | 机制 |
|--------|------|------|
| 状态展示 | `refreshBridgeStatus()` | 优先读 `router_mode`，`engine` 字段保留不回溯 |
| 浏览器动作 | API 层 `browser_action` + `browser_query` | 新字段优先，旧 bool 回退为 capture |
| 扩展协议 | `BridgeResult.StructuredCollectedData` + `CollectedData` | 新字段优先，string 回退为 Title |
| 扩展 action | `background.js` `handleTask()` | `action === "screenshot"` 或无 action 走旧截图路径 |

**核心原则**：所有破坏性改动都通过"加新字段 + 优先读新字段 + 旧字段回退"实现，外部调用零感知。只有在确认所有调用方已迁移到新路径后，才在最后一次提交中移除旧字段。

## 更新后的验收标准

1. 开启 Web 认证后，真实扩展可稳定完成配对、拉取任务、回传结果。
2. 首页状态明确区分：配置态、连接态、健康态、执行态。
3. 用户可以明确选择 `open`、`capture`、`collect` 三种浏览器动作。
4. 选择 `extension` 时，浏览器联动不再依赖 CDP 在线。
5. Web-only 引擎返回的浏览器采集结果可稳定映射到 `UnifiedAsset`，而不是仅返回一个弱化标题字段。
6. 首页文案、桥接状态、router 当前模式、实际执行路径四者保持一致。

## 本次核查涉及的主要代码

- `web/middleware_auth.go`
- `web/screenshot_bridge_handlers.go`
- `web/cookie_handlers.go`
- `web/query_handlers.go`
- `web/static/js/main.js`
- `web/templates/index.html`
- `web/server.go`
- `internal/service/query_app_service.go`
- `internal/adapter/web_only_base.go`
- `internal/screenshot/router.go`
- `internal/screenshot/bridge_types.go`
- `tools/extension-screenshot/src/background.js`
- `tools/extension-screenshot/src/capture.js`
- `tools/extension-screenshot/src/api.js`
