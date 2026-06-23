# UniMap 修复计划（2026-06-16）

> 来源：`CLAUDE.md` 2026-06-16 全量问题核实、`memory/MEMORY.md`、浏览器采集相关记忆、`docs/ENGINE_ADAPTER_IMPLEMENTATION_PLAN.md`、`docs/compose/plans/2026-06-12-strong-typing-migration.md`、`docs/SCHEDULER_NEXT_STEPS.md`。
> 目标：把最新 13 项已知问题拆成可验证、可回滚的修复阶段，避免重复处理已闭环的旧审计项。

## 1. 当前判断

### 1.1 已闭环，不再纳入本计划主线

| 项目 | 结论 |
|------|------|
| 2026-06-09 全量审计 P0/P1/P2 | 已按文档闭环，本计划不重复列入 |
| 5 个基础引擎采集 | FOFA、Hunter、ZoomEye、Quake、Shodan 主链路已验证打通 |
| SPA 截图时机 | `collect_and_capture`、`spa` 等待策略已落地 |
| L1 Network | ZoomEye/Hunter/Quake 已集成到 combined collect+capture |
| TD-4 Phase 1-6 | 已完成大部分强类型迁移，剩余为系统边界和测试数据 |

### 1.2 本轮待修问题分组

| 优先级 | 问题 | 计划定位 |
|--------|------|----------|
| P0 | Extension 截图未继承登录态 | 阻断真实浏览器截图/采集可信度，优先处理 |
| P1 | ~~Hunter country/title/host 清理未生效~~ | ✅ 已修复，见 Phase 2 |
| P1 | ~~`web/` flaky test 并行端口冲突~~ | ✅ 已修复，见 Phase 3.1 |
| P1 | `extractPortFromHost` IPv6 误解析 | ✅ 已修复（2026-06-16）：`net.SplitHostPort` + 多冒号保护，避免 IPv6 资产污染 |
| P2 | `countGoroutines()` 空桩 | ✅ 已完成（2026-06-17） |
| P2 | ZoomEye 哈希 CSS class | 选择器脆弱性问题 |
| P2 | 新引擎端到端未闭环 | ✅ 代码基础设施已补齐（2026-06-17），⏳ 需 API Key 真机验证 |
| P2 | 定时任务缺少简易定时 | 需要模型/API/UI 联动，单独分阶段 |
| P3 | 剩余 `map[string]interface{}` | 长期技术债，按边界继续渐进迁移 |
| P3 | L2 Hook | 继续冻结，只在 telemetry 证明收益后启动 |

## 2. 修复原则

1. 先修阻断真实使用的 P0/P1，再处理端到端扩展和长期技术债。
2. 每个阶段必须有独立测试或真机验证证据。
3. Extension 修改后必须记录版本号，并执行 Chrome 扩展 Remove + Load unpacked 重载验证。
4. 新增引擎不直接全量开放，先走单引擎真机闭环，再批量开放 UI。
5. L2 Hook 不作为本轮交付项，除非 L1/L3 失败率或字段缺失 telemetry 证明收益明确。

## 3. 阶段计划

### Phase 0：建立当前基线

**目标**：确认修复前状态，避免后续无法区分新旧问题。

**行动**：

- 运行 `go test -race ./internal/collection ./internal/screenshot ./web`，记录当前失败项。
- 单独运行 `go test -race ./web -run TestClassifyBatchURLsPreservesOriginalIndices -count=20`，确认 flaky 触发条件。
- 用 Extension 路径分别触发 FOFA/Hunter `collect_and_capture`，保存截图、`structured_collected_data`、登录状态诊断。
- 统计剩余动态 map：`rg "map\\[string\\]interface\\{\\}" --glob "*.go"`。

**验收**：

- 形成一份修复前命令输出摘要，可附到后续 fix report。
- 明确截图未登录是 cookie/session 未共享、tab 复用导致丢态，还是页面登录判断误报。

### Phase 1：P0 Extension 截图登录态修复

**问题**：`tools/extension-screenshot/src/capture.js` 的 `ensureTab()` 通过 `chrome.tabs.create({ url, active: true })` 新开标签页。当前现象是 FOFA/Hunter 截图显示未登录，说明真实浏览器会话没有被目标 tab 正确继承，或目标页进入了未登录/隔离上下文。

**当前判断**：这是唯一仍然保持 P0 的问题，且更像运行时会话/标签页复用策略问题，不是代码静态修复就能完全确认闭环的项。

**涉及文件**：

- `tools/extension-screenshot/src/capture.js`
- `tools/extension-screenshot/src/background.js`
- `web/cookie_handlers.go`
- `internal/screenshot/router_extension.go`
- `docs/OPS_SCREENSHOT_EXTENSION.md`

**行动**：

1. 给 Extension 增加登录态诊断结果：tab id、window id、url、cookie count、domain、login wall marker、是否复用 tab。
2. 修改 `ensureTab()` 优先复用同域已登录 tab 或当前活动同域 tab；仅在没有可用 tab 时新建。
3. 新建 tab 后主动调用 `chrome.cookies.getAll({domain})` 诊断目标域 cookie 是否可见，不把 cookie 内容回传，只回传数量和名称哈希或名称列表白名单。
4. 对 FOFA/Hunter 增加页面级登录墙检测：检测登录按钮、未登录提示、部分数据提示，并把状态回传到 `StructuredCollectedData.Extra`。
5. 如果 cookie 可见但页面仍未登录，增加一次软刷新或导航到引擎首页再回搜索页的保守 fallback。
6. 更新 Extension 版本号和运维文档，注明修改后必须 Remove + Load unpacked。

**验收**：

- 已登录 Chrome + 已配对 Extension 下，FOFA/Hunter 搜索结果截图不再出现登录按钮或未登录提示。
- 登录失效时返回明确 `LOGIN_REQUIRED` 或 `is_login_wall=true`，不再把未登录页面当作正常截图成功。
- `go test -race ./internal/screenshot ./web` 通过。
- 真机记录包含 FOFA/Hunter 各 1 次截图和采集证据。

### Phase 2：P1 Hunter 字段清理调用链修复

**问题**：`collection.CleanHunterFields` 已实现，但 `browserCollectedData` 路径仍返回城市名、分类标签和 UI 噪声。当前代码在 `internal/screenshot/router_extension.go` 的 `populateCollectResultFromBridge` 和登录墙路径调用了清理函数，但 Web API payload 可能经过了未清理的 service/browser outcome 路径。

**涉及文件**：

- `internal/collection/parser.go`
- `internal/collection/parser_test.go`
- `internal/screenshot/router_extension.go`
- `internal/screenshot/manager_collect.go`
- `internal/service/query_app_service.go`
- `web/query_handlers.go`
- `web/query_handlers_test.go`

**状态**：✅ 已完成（2026-06-16）

**行动**：

1. `collection.NormalizeAssets(engine string, assets []model.UnifiedAsset)` 已落地。
2. Hunter 清理已统一接入 parser、L1 Hunter network parser、WebOnly adapter、browser fallback service、web API payload 组装。
3. `CleanHunterFields` 规则已覆盖：
   - `country_code` 纯中文地名归一为 `中国`。
   - `title` 截断常见分类组合。
   - `host` 清除 UI 噪声，清理后为空则置空。
4. 相关 table-driven 测试和 Web 契约测试已补齐。

**验收**：

- Hunter `browserCollectedData` 中 `country_code=中国`，title 不含分类标签，host 不含 UI 噪声。
- `go test -race ./internal/collection ./internal/screenshot ./internal/service ./web` 通过。

### Phase 3：P1 稳定性和低成本正确性修复

#### 3.1 修复 `web/` flaky test

**问题**：`TestClassifyBatchURLsPreservesOriginalIndices` 并行端口冲突，单独运行通过。

**状态**：✅ 已完成（2026-06-16）

**涉及文件**：

- `web/screenshot_handlers_test.go`
- `web/screenshot_handlers.go`

**行动**：

- 检查该测试是否启动真实监听端口或共享全局状态。
- 改为 `httptest.Server` 动态端口，或构造纯函数输入，避免固定端口和并行共享资源。
- 如测试本身不需要并行，移除 `t.Parallel()`，但优先消除共享端口根因。

**验收**：

- `go test -race ./web -run TestClassifyBatchURLsPreservesOriginalIndices -count=50` 通过。
- `go test -race ./web -count=5` 通过。

#### 3.2 修复 IPv6 port 解析

**状态**：✅ 已完成（2026-06-16）。实现位置：`internal/collection/parser.go`；测试位置：`internal/collection/parser_test.go`。

**问题**：`internal/collection/parser.go` 中 `extractPortFromHost` 使用 `strings.LastIndex(s, ":")`，会把裸 IPv6 如 `2001:db8::1` 错误解析为 port 1。

**行动**：

- 使用 `net.SplitHostPort` 优先处理 `[IPv6]:port`、`host:port`。
- 对裸 IPv6 使用 `net/netip.ParseAddr` 或冒号数量判断，不解析为 port。
- 对非括号 IPv4/域名 `example.com:443` 保持兼容。

**验收测试用例**：

| 输入 | 预期 |
|------|------|
| `example.com:443` | port=443, host=`example.com` |
| `1.2.3.4:80` | port=80, host=`1.2.3.4` |
| `[2001:db8::1]:443` | port=443, host=`2001:db8::1` |
| `2001:db8::1` | port=0, host 原样 |
| `2001:db8::1:443` | port=0, host 原样，除非后续明确支持非标准格式 |

#### 3.3 修复 `countGoroutines()` 空桩 ✅ 已完成（2026-06-17）

**问题**：`internal/screenshot/router_test.go` 中 `countGoroutines()` 返回 0，导致 goroutine leak 测试无效。

**解决**：
- 改为 `runtime.NumGoroutine()`。
- 泄漏检测增加 5 协程容差（避免 runtime GC/timer 协程误判）。
- 20 次 `-race` 全通过。

### Phase 4：P2 ZoomEye 选择器韧性增强

**问题**：当前 ZoomEye IP 选择器依赖哈希 class：`span._public-hover_uxlu6_1`，前端重新构建后易失效。

**涉及文件**：

- `tools/extension-screenshot/src/capture.js`
- `internal/screenshot/dom_selectors.go`
- `memory/project_browser_collection_knowledge.md`

**行动**：

1. 将哈希 class 降级为最后 fallback。
2. 优先使用结构选择器和内容模式：
   - `div.ip-detail-box span`
   - `div.url-container span`
   - 从 `url-container` 解析 `host:port`
   - 使用 IPv4/IPv6/domain 正则从结果卡片文本中兜底提取。
3. 增加“选择器命中路径”诊断字段，便于下次前端改版定位。
4. 用已保存的 ZoomEye HTML 页面跑本地提取测试。

**验收**：

- 不依赖 `_public-hover_uxlu6_1` 仍能提取 IP/host/port。
- Extension 真机 ZoomEye 采集至少 10 条结果，字段正常。

### Phase 5：P2 新引擎端到端闭环 ✅ 代码基础设施已补齐（2026-06-17）

**问题**：Censys/DayDayMap/Onyphe/GreyNoise 适配器已有，但配置、Web UI、Extension DOM、结果展示和真机验证没有完整闭环。BinaryEdge 在代码中也存在配置/CLI 迹象，应一并核实范围。

**2026-06-17 代码补齐**：
- 全层级 5→10 引擎扩展（CLI / Web UI / Config API / 引擎重载 / 登录检测 / 稳定引擎列表）
- Censys `api_id`/`api_secret` 特殊字段处理（Go + JS 双侧）
- BinaryEdge Extension 引擎检测 + DOM 选择器
- Extension 版本 0.3.8→0.3.9
- `go build/vet/test -race` 全部通过
- ⏳ 仍需各引擎 API Key 真机验证

**涉及文件**：

- `internal/adapter/*`
- `cmd/unimap-web/main.go`
- `cmd/unimap-cli/main.go`
- `internal/config/config.go`
- `configs/config.yaml.example`
- `web/config_handlers.go`
- `web/templates/settings.html`
- `web/templates/index.html`
- `web/static/js/main.js`
- `tools/extension-screenshot/src/capture.js`
- `internal/screenshot/dom_selectors.go`
- `docs/SEARCH_ENGINE_SYNTAX.md`
- `docs/API_KEYS.md`

**行动**：

1. 先做状态盘点：确认每个新增引擎是否已注册 API adapter、是否有配置节、是否出现在 CLI、Web 设置页、查询页、配额页。
2. 按引擎分批闭环，建议顺序：
   - Censys：API 文档完善，优先 API 查询闭环。
   - DayDayMap：国内引擎，需确认真实 API 和 DOM。
   - Onyphe：OQL/category 语义特殊，先 API。
   - GreyNoise：威胁情报，不等价于资产搜索，UI 需标注用途。
   - BinaryEdge：若确认为已实现 adapter，则纳入；否则单独开任务。
3. UI 不默认勾选未配置 key 的新增引擎；设置页显示 key、base_url、enabled。
4. Extension DOM 选择器只在真机验证通过后标为 supported，否则返回 `unsupported_engine_web_collection`。
5. 更新文档和 API key 指南。

**验收**：

- 每个新增引擎至少满足一条：API 查询成功并展示结果，或 WebOnly 明确不支持并给出 UI 提示。
- `go test -race ./internal/adapter ./web ./cmd/unimap-cli` 通过。
- 至少 Censys/DayDayMap/Onyphe/GreyNoise 各有一条端到端验证记录。

### Phase 6：P2 定时任务简易定时功能

**问题**：Scheduler 当前只支持 cron 循环，不支持一次性、延迟、指定时间点等用户更容易理解的模式。

**涉及文件**：

- `internal/scheduler/scheduler_types.go`
- `internal/scheduler/scheduler.go`
- `internal/scheduler/store.go`
- `web/scheduler_handlers.go`
- `web/templates/scheduler.html`
- `docs/API.md`
- `docs/SCHEDULER_NEXT_STEPS.md`

**建议模型**：

```go
type ScheduleMode string

const (
    ScheduleModeCron     ScheduleMode = "cron"
    ScheduleModeOnceAt   ScheduleMode = "once_at"
    ScheduleModeDelay    ScheduleMode = "delay"
    ScheduleModeInterval ScheduleMode = "interval"
)
```

新增字段建议：

| 字段 | 说明 |
|------|------|
| `schedule_mode` | `cron`、`once_at`、`delay`、`interval` |
| `run_at` | 指定时间执行 |
| `delay_seconds` | 创建后延迟执行 |
| `interval_seconds` | 简易间隔循环 |
| `delete_after_run` | 一次性任务完成后自动删除或禁用 |

**行动**：

1. 数据模型保持向后兼容：无 `schedule_mode` 的旧任务视为 `cron`。
2. Scheduler 内部对 `once_at`/`delay` 使用 `time.Timer` 或转换为一次性 schedule；执行后自动禁用并持久化。
3. `interval` 可以先转换为 cron 支持的秒/分钟粒度；复杂间隔再后续扩展。
4. API 创建/更新任务增加字段校验，禁止同时填写冲突字段。
5. UI 增加 segmented control：Cron、一次性、延迟、间隔；保留高级 Cron 输入。
6. 增加 e2e 测试：delay 触发一次、once_at 到点执行、执行后禁用/删除、旧 cron 任务兼容。

**验收**：

- 旧任务 JSON 可正常加载。
- `go test -race ./internal/scheduler ./web` 通过。
- Web UI 可创建 1 分钟后执行的一次性任务，并在执行后显示已完成/已禁用状态。

### Phase 7：P3 剩余强类型迁移

**问题**：`CLAUDE.md` 记录剩余 653 处 `map[string]interface{}`，但 `memory/MEMORY.md` 同时记录核心逻辑已迁移，剩余主要是测试、Web 响应、collection/parser 等边界。数量差异需先重新统计。

**依据**：`docs/compose/plans/2026-06-12-strong-typing-migration.md`

**行动**：

1. 重新统计并按用途分类：测试 fixture、模板数据、Web API DTO、collection legacy parser、插件扩展字段。
2. 保留合理的 `map[string]any` 场景：模板 dict、插件 Extra、未知 metadata。
3. 优先迁移对外契约：WebSocket message、query payload、bridge diagnostic、cookie/login status。
4. 每次迁移只覆盖一个边界，保留 JSON 字段名不变。

**验收**：

- 核心生产路径不再新增 `map[string]interface{}`。
- 剩余动态 map 有分类说明，不追求机械清零。

### Phase 8：L2 Hook 继续冻结

**结论**：L2 Hook 不是本轮修复项。当前 L1/L3 已覆盖主链路，L2 需要 MV3 MAIN world + ISOLATED bridge，复杂度高。

**启动条件**：

- L1 对 SPA 引擎持续失败，且页面 JS 能拿到明文响应。
- L3 DOM 因虚拟列表/懒加载导致字段完整度明显不足。
- 新增引擎必须依赖页面内部解密后的响应。

**若启动，必须先做单引擎 spike**：

- 仅选 ZoomEye 或 Hunter。
- MAIN world hook 捕获响应，postMessage 到 ISOLATED world。
- 校验 origin、task id、response size。
- 失败自动回退 L1/L3。

## 4. 推荐执行顺序

| 周期 | 内容 | 产出 |
|------|------|------|
| D1 | Phase 0 + Phase 1 诊断 | 截图登录态根因证据 |
| D2-D3 | Phase 1 修复 + 真机 FOFA/Hunter 验证 | P0 闭环 |
| D3-D4 | Phase 2 + Phase 3 | Hunter 字段、flaky、IPv6、goroutine test 闭环 |
| D5 | Phase 4 | ZoomEye 选择器韧性增强 |
| W2 | Phase 5 | 新增引擎按单引擎闭环 |
| W3 | Phase 6 | 简易定时功能 |
| 持续 | Phase 7 | 强类型边界迁移 |

## 5. 回归命令

```bash
go test -race ./internal/collection ./internal/screenshot ./internal/service ./web
go test -race ./internal/scheduler ./web
go test -race ./internal/adapter ./cmd/unimap-cli
go build ./...
go vet ./...
```

真机验证至少覆盖：

| 链路 | 验证 |
|------|------|
| FOFA Extension collect_and_capture | 已登录截图 + 结构化数据 |
| Hunter Extension collect_and_capture | 已登录截图 + 清理后字段 |
| ZoomEye Extension collect | 无哈希 class 依赖仍可提取 |
| Scheduler once/delay | 到点执行一次并持久化状态 |
| 新增引擎 | API 或 WebOnly 明确闭环 |

## 6. 风险与回滚

| 风险 | 缓解 |
|------|------|
| Extension tab 复用影响用户当前页面 | 只复用同域搜索页或扩展管理的 tab；必要时加配置开关 |
| 登录诊断泄露敏感 cookie | 不回传 cookie value，只回传计数、域名、白名单名称或哈希 |
| Hunter 清理误删正常标题 | table-driven 测试覆盖中文分类和正常标题，规则保持保守 |
| 简易定时破坏旧 cron 任务 | `schedule_mode` 默认 `cron`，旧 JSON 兼容 |
| 新引擎 UI 过早开放造成误用 | 未验证引擎默认不勾选，并显示支持状态 |

## 7. 完成标准

1. P0/P1 全部有测试或真机证据。
2. `go test -race ./...` 至少在合并前通过；如存在外部依赖导致跳过，必须记录原因。
3. 文档同步更新：Extension 运维、API、Scheduler、搜索引擎语法或 API Key 指南。
4. `CLAUDE.md` 和 `memory/MEMORY.md` 的 2026-06-16 问题状态更新为已完成或明确暂缓。
