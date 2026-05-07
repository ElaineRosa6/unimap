# 引擎测绘、浏览器自动化与前端整改计划

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

状态：部分完成。

已完成：

- 首页已提供 `cdp`、`extension`、`auto` 单选项。
- `web/static/js/main.js` 会调用 `/api/screenshot/set-mode` 切换模式。
- `internal/screenshot/router.go` 已实现 `SetMode()`、`CurrentMode()`、`resolveProvider()`。
- 浏览器联动 `OpenSearchEngineResult()`、截图 `CaptureSearchEngineResult()`、结构化采集 `CollectSearchEngineResult()` 已统一通过 router 按当前模式分发。

未完成：

- 首页帮助文案仍写"通过已连接的 CDP 浏览器打开所选引擎结果页"，与扩展/auto 模式不符。
- 桥接状态 UI 仍把 `config.Screenshot.Engine` 当成主要状态源，和 router 的当前模式并不一致。

### B. 让浏览器测绘自动化成为真实能力

状态：比文档中的旧结论更进一步，但仍未完整闭环。

已完成：

- Web-only adapter 不再只是返回 `not supported`；`internal/adapter/web_only_base.go` 的 `Search()` 已可调用浏览器后端 `CollectSearchEngineResult()`。
- `web/server.go` 在 router 初始化后会执行 `unifiedSvc.SetWebOnlyBrowserBackend(...)`，将浏览器采集能力注入 Web-only 引擎。
- `ScreenshotRouter` 和 `ExtensionProvider` 已暴露 `CollectSearchEngineResult()`。

仍未完成：

- 扩展 `collect` 当前只是返回一个非常弱的 `CollectResult`，只把 `CollectedData` 填进 `Title`，没有真正回传结构化资产列表。
- `BridgeResult` 只有 `CollectedData string`，没有稳定的结构化结果协议，无法可靠表达多条资产、分页、总数、字段映射。
- 这意味着"Web-only 引擎可进入统一查询链路"已经成立，但"浏览器采集结果质量足以替代 API 查询"还远未达标。

### C. 修复认证后扩展不可连接

状态：主修复已完成。

已完成：

- `/api/screenshot/bridge/*` 已从全局 admin 鉴权豁免。
- 配对、取任务、回传结果都要求 loopback，请求不会直接暴露成公网桥接口。

仍需补强：

- 文档里提到的"pairing token + callback signature + nonce 防重放"当前代码大多已有实现，但是否默认启用仍取决于配置。
- 需要补一组覆盖认证开启场景的端到端验证，证明真实浏览器扩展而不只是 mock client 能稳定配对、拉任务、回传结果。

### D. 统一前端视觉系统

状态：基本未完成。

- 首页虽然新增了模式选择和状态块，但帮助文案、状态块结构、信息层次仍混杂。
- `scheduler.html`、`monitor.html`、`batch-screenshot.html` 仍未完成统一设计系统收敛。
- 现有页面仍大量使用内联样式和局部视觉规则。

## 当前业务逻辑与代码逻辑的矛盾点

### 1. 状态来源不一致

- 后端真实执行能力优先看 `ScreenshotRouter` 当前模式与健康状态。
- 但桥接状态接口 `buildBridgeDiagnosticSnapshot()` 仍把 `config.Screenshot.Engine` 暴露为主 `engine` 字段。
- 前端 `refreshBridgeStatus()` 又基于这个 `engine` 字段来决定"扩展在线/离线"的文案。

影响：

- 实际已经可通过 `auto` 或 `extension` 执行，但界面可能仍显示"当前截图引擎为 cdp，扩展桥接未启用"。

### 2. "已配对"与"可执行"被混为一谈

- `ext_paired`、`paired_clients`、`live_clients` 本质上是连接态。
- 但 UI 把它们直接用于"浏览器查询模式可不可用"的判定展示。

影响：

- 用户看到"已配对"并不等于当前强制模式、当前 action、当前任务链路都一定可执行。

### 3. `browser_query` 的语义仍然偏旧

- 代码里 `browser_query` 实际表示"执行统一查询的同时，再通过浏览器打开结果页"。
- 但文档目标里还区分了 `query-open`、`query-capture`、`query-collect`。
- 当前前端没有把这三种模式明确暴露给用户。

影响：

- 用户以为打开"浏览器查询"后是在做浏览器测绘，实际很多情况下只是 `open`，并不是 `collect`。

### 4. 首页文案与现有实现冲突

- 首页说明仍写"通过已连接的 CDP 浏览器打开所选引擎结果页"。
- 但实际代码已经支持扩展和 auto。

影响：

- 用户认知仍停留在旧模型，容易误解扩展模式是否真的参与浏览器联动。

### 5. Web-only 查询链路已接通，但返回模型仍过弱

- 业务目标希望浏览器测绘成为真实采集链路。
- 代码目前只是让 Web-only 引擎"能跑通"，但扩展 collect 结果无法稳定映射成完整 `UnifiedAsset` 集。

影响：

- 从工程视角看已经"接线完成"，从产品视角看还不能宣称"真实可用"。

### 6. 扩展端完全无 action 分流

- `tools/extension-screenshot/src/background.js` 的 `handleTask()` 不处理 `task.action` 字段，所有任务都走截图路径。
- `capture.js` 只有截图能力，没有 DOM 提取能力。

影响：

- 后端即使发送 `action: "collect"` 任务，扩展端也只会截图，不会返回结构化数据。

---

## 安全执行切片计划

> 核心原则：禁止直接生成最终代码。所有破坏性改动通过"加新字段 + 优先读新字段 + 旧字段回退"实现防腐层，外部调用零感知。

### 步骤 1：统一桥接状态前端展示

**具体动作**：修改 `web/static/js/main.js` 的 `refreshBridgeStatus()` 函数，改为消费后端已返回的 `router_mode`、`router_cdp_healthy`、`router_ext_healthy` 字段，替换当前基于 `config.Screenshot.Engine` 的判定逻辑。同时更新首页说明文案为"通过当前选择的浏览器执行模式打开结果页"。

**目的**：消除"已修复但 UI 仍误导"的状态语义不一致问题。

**影响面**：仅 `main.js` 前端展示逻辑 + `index.html` 一段文案。不涉及后端 API 变更。

**验收标准**：
- 前端在 `auto`、`cdp`、`extension` 三种模式下，桥接状态徽章和提示文字准确反映实际健康状态
- `router_mode` 与前端模式选择器一致时，不再显示"扩展桥接未启用"类误导文案
- 构建通过，`go test -race ./...` 无回归

---

### 步骤 2：补强扩展配对端到端验收

**具体动作**：完善 `scripts/bridge_e2e.ps1` 验收脚本，覆盖 `web.auth.enabled=true` 时真实扩展的完整生命周期：配对 → 拉取 open 任务 → 拉取 capture 任务 → 回传结果 → 状态可观测。在 `buildBridgeDiagnosticSnapshot()` 中增加 `last_pair_at`、`last_task_pull_at`、`last_callback_at` 时间戳字段。

**目的**：用可重复的验收代替人工验证，为后续重构提供安全网。

**影响面**：完善现有 PS1 脚本 + `buildBridgeDiagnosticSnapshot()` 增加 3 个时间戳字段（纯追加，不删改）。

**验收标准**：
- 验收脚本可在 `auth.enabled=true` 环境下完整跑通配对→任务→回调链路
- `GET /api/screenshot/bridge/status` 返回新增的 3 个时间戳字段
- 现有 bridge handler 测试全部通过

---

### 步骤 3A：前端浏览器动作拆分（UI 层）

**具体动作**：将首页"查询时同步在浏览器打开结果页"checkbox 替换为三个互斥选项：① 仅打开结果页（open） ② 打开并截图（capture） ③ 打开并采集结构化结果（collect）。前端通过新字段 `browser_action` 传递选择，同时保留旧 `browser_query` bool 字段以保持向后兼容。

**目的**：让用户操作与系统能力一致，区分 open/capture/collect 三种语义。

**影响面**：仅 `index.html` + `main.js`。后端暂不消费 `browser_action`，仍用旧 bool 回退。

**验收标准**：
- 三种选项在 UI 上互斥（radio 组）
- 选中 collect 时，若当前模式不可用则给出明确提示
- 旧 `browser_query` checkbox 逻辑不中断（防腐层：前端同时发送 `browser_query` 和 `browser_action`，后端仍只读 `browser_query`）

---

### 步骤 3B：后端浏览器动作分流（API + 服务层）

**具体动作**：
1. WebSocket 和 REST 查询 API 接收 `browser_action` 参数（与 `browser_query` 并存）
2. `RunBrowserQueryAsync` 按 `browser_action` 分流：`open` 只调用 `OpenSearchEngineResult`，`capture` 走现有的 open+capture，`collect` 调用 `CollectSearchEngineResult` 并将结果注入查询响应
3. 保留 `browser_query=true` 的旧路径，行为等同于 `browser_action=capture`（防腐层）

**目的**：让后端真正支持三种浏览器动作，`collect` 路径可产出结构化资产数据。

**依赖前提**：步骤 1 验收通过（状态语义已统一），步骤 3A 验收通过（前端已发送新参数），步骤 4A 验收通过（`BridgeResult` 已有结构化字段）

**影响面**：`web/query_handlers.go`、`web/websocket_handlers.go`、`internal/service/query_app_service.go`

**验收标准**：
- `browser_action=open` 只打开页面，不截图
- `browser_action=capture` 行为与旧 `browser_query=true` 一致
- `browser_action=collect` 调用 `CollectSearchEngineResult`，返回结果中包含 `assets` 字段
- `browser_query=true` 且无 `browser_action` 时，回退为 `capture` 行为（防腐层生效）
- 所有路径 `-race` 测试通过

---

### 步骤 4A：扩展 collect 返回协议升级（后端防腐层）

**具体动作**：
1. `BridgeResult` 新增字段 `StructuredCollectedData map[string]interface{}`，**保留** `CollectedData string` 不动
2. `ExtensionProvider.CollectSearchEngineResult()` 中：优先解析 `StructuredCollectedData` 到 `CollectResult.Assets`、`Total`、`HasMore`；若为空则回退到 `CollectedData` 填 `Title` 的旧逻辑
3. 新建 `docs/BROWSER_COLLECT_STRATEGY.md`，定义 FOFA/Hunter/ZoomEye/Quake 四个引擎的 DOM 选择器和字段映射规则

**目的**：让浏览器采集结果能稳定映射到 `UnifiedAsset`，使 Web-only 查询链路真正可用。

**影响面**：`internal/screenshot/bridge_types.go`（加字段）、`internal/screenshot/router.go`（解析逻辑升级）、新增策略文档

**验收标准**：
- 旧扩展（只返回 `CollectedData string`）仍能正常工作，行为不变（防腐层生效）
- 新扩展返回 `StructuredCollectedData` 时，`CollectResult.Assets` 被正确填充
- `WebOnlyAdapterBase.Search()` 能将 `CollectResult.Assets` 映射到 `EngineResult.NormalizedData`
- 相关单元测试覆盖新旧两条路径

---

### 步骤 4B：扩展端 collect 能力实现（本仓库内）

**具体动作**（在 `tools/extension-screenshot/` 内）：

1. **`capture.js` 新增 `extractEngineAssets(engine, tabId)`**：按引擎类型从搜索结果页 DOM 中提取结构化数据（URL、标题、协议、端口、IP 等），返回 `items[]` 数组
2. **`background.js` 的 `handleTask()` 增加 action 分流**：
   - `action === "open"`：只做 `ensureTab()` + `waitForPageReady()`，不截图、不回传 image_data
   - `action === "collect"`：`ensureTab()` → `waitForPageReady()` → `extractEngineAssets()` → 回传 `structured_collected_data`（包含 items、total、has_more）
   - `action === "screenshot"` 或无 action：保持现有截图逻辑不变（防腐层）
3. **`api.js` 的回传格式**：新增 `structured_collected_data` 字段，与 `image_data` 并存

**关键代码位置**：
- `background.js:42-103` `handleTask()` — 当前全量走截图，需加 action 判断
- `capture.js:128-137` `normalizeImagePayload()` — 新增 `normalizeCollectPayload()` 伴生函数
- `api.js` — 回传 body 结构自然兼容新字段

**依赖前提**：步骤 4A 验收通过（后端已支持 `StructuredCollectedData` 字段解析），步骤 2 验收通过（有验收脚本验证端到端）

**影响面**：仅 `tools/extension-screenshot/` 目录内的 JS 文件，不影响 Go 后端编译

**验收标准**：
- 扩展对 `action: "open"` 任务只打开页面不截图
- 扩展对 `action: "collect"` 任务能从页面提取结构化数据并正确回传
- 扩展对 `action: "screenshot"` 或无 action 任务行为与当前一致（防腐层生效）
- 通过步骤 2 的验收脚本验证 open/capture/collect 三条链路
- FOFA 和 Hunter 引擎的 DOM 提取策略在 `BROWSER_COLLECT_STRATEGY.md` 中有明确定义，扩展代码与之对齐

---

### 步骤 5：前端视觉系统收敛

**具体动作**：基于已稳定的状态语义和浏览器动作，统一 `scheduler.html`、`monitor.html`、`batch-screenshot.html` 的视觉设计，消除内联样式，复用 `layout.html` 共享模板和 CSS tokens。

**目的**：在能力语义稳定后统一视觉，避免重做。

**依赖前提**：步骤 1 验收通过（状态展示稳定），步骤 3B 验收通过（浏览器动作语义稳定）

**影响面**：`web/templates/scheduler.html`、`web/templates/monitor.html`、`web/templates/batch-screenshot.html`、`web/static/css/style.css`

**验收标准**：
- 所有页面使用统一的 layout 模板
- 无内联 style 属性
- 状态徽章、卡片、按钮样式跨页面一致

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
