> **Claude Code 项目记忆同步说明**
> 
> 本项目的完整 Claude Code 会话记忆同时保存在 C 盘客户端目录：
> 
> ```
> C:\Users\ljw\.claude\projects\D--Project-Go-project-unimap\memory
> ```

# Project Memory Index

## 当前事实快照（2026-07-29）

- [项目当前状态与后续行动 2026-07-24](project_current_state_2026-07-24.md) — 当前提交、
  巡检收口结果、浏览器层 SSRF 安全边界、七引擎事实矩阵和下一步顺序。
- 当前稳定 Web UI 引擎为 FOFA、Hunter、ZoomEye、Quake、Shodan；历史真实结构化采集证据来自
  Bridge/Extension，Quake/Hunter CDP 结构化采集已于 2026-07-29 通过真实验收；FOFA/ZoomEye/Shodan CDP 定级尚未执行。
- Censys、DayDayMap 仅完成服务端/CLI API 适配与 API 实机验证，不能标记为稳定 Web UI、
  Bridge 或 CDP 已完成。
- 自动“发现变化 → 证据截图 → 图片通知”尚未启用。调用前 URL 校验不能覆盖浏览器跨主机重定向
  和 DNS rebinding，浏览器层 SSRF 防护已实现（2026-07-29），自动证据截图需云端验收后启用。
- 本地 6 个提交（最新 `ba0d449`）领先 `origin/develop`，尚未推送。

> 本索引后续的日期条目是历史知识，不覆盖上述当前事实。当前实施状态以
> [`docs/REMAINING_WORK_2026-07-23.md`](../docs/REMAINING_WORK_2026-07-23.md) 和
> [`docs/IMPLEMENTATION_PLAN_2026-07-23.md`](../docs/IMPLEMENTATION_PLAN_2026-07-23.md) 为准。

## 项目知识（历史合并与专题记录）

- [B-01 CGO/SQLite 修复 2026-07-20](project_b01_cgo_sqlite_fix_2026-07-20.md) — Dockerfile builder 安装 build-base + CGO_ENABLED=1；CI 同步；容器内 SQLite 可用；QA security audit 16 扫描器未发现新增漏洞
- [B-02 生产 Compose 定型 2026-07-20](project_b02_prod_compose_2026-07-20.md) — 新增 docker-compose.prod.yaml（loopback 端口、移除 dev mount、backups 卷、日志轮转、资源限制、强制 admin token）
- [功能审计修复复核 2026-07-16](project_functional_audit_reverification_2026-07-16.md) — 12 项整改补修：11 FIXED、0 PARTIAL、1 MITIGATED；批次唯一性、调度提交边界、readiness 与截图 ID HTTP 契约已闭合
- [审计补修闭环 2026-07-07](project_audit_fix_closeout_2026-07-07.md) — ✅ 代码补修闭环；外部凭证已于 2026-07-13 确认完成轮换，记录仅保留为历史背景
- [浏览器采集架构知识](project_browser_collection_knowledge.md) — 历史 Bridge 采集验证与 DOM
  选择器知识；不能作为 CDP 真实闭环证据，当前状态以前述事实快照为准
- [Extension 桥接截图超时问题 2026-07-20](project_extension_bridge_screenshot_timeout_2026-07-20.md) — 批量截图 2/3 失败根因：ensureTab 不等待导航导致 waitForPageReady 监听器竞态 + 扩展端(最长33s)与Go后端(30s)超时不一致 + 固定15s渲染等待对普通网站过度
- [端口扫描全端口无结果 2026-07-20](project_port_scan_full_mode_no_results_2026-07-20.md) — 全端口(1-65535)+telnet扫描132.232.231.41无结果：65535端口即使256并发也要3.4分钟，默认5分钟超时；scan_failed状态不显示开放端口；历史保存为fire-and-forget，scan失败时不保存

## 核心架构决策

- **端口扫描随机化与多方法探测（2026-07-15）**：公网目标解析、SSRF/CDN/授权范围校验后，对全局去重的 `唯一 IP × 端口` 笛卡尔积随机打乱；每个组合可执行 connect、Telnet、UDP、FIN、NULL、Xmas 和 0-5000ms 随机抖动。确定响应使用 `open`，UDP/原始 TCP 无响应使用 `open_filtered`，不得合并进确定开放端口。FIN/NULL/Xmas 必须提供 `authorized_targets`，进程还需原始套接字权限。
- **2026-07-07 审计补修闭环**：`probe-web`/batch 已加 `urlguard` + 内网目标拦截，底层探测使用 `SafeHTTPClient` 防 DNS rebinding；`operation_history` 列表查询改为显式 SQL 分支；WebSocket 重连后会查询 `/api/v1/query/status` 恢复当前查询基础状态。外部平台密钥/admin token 已于 2026-07-13 确认轮换完成；不得再把该项列为未完成事项。
- **运行模式按环境选择**：桌面环境可用 Extension 复用真实浏览器会话；无图形 Linux/容器默认使用
  CDP headless。所有浏览器业务统一经过 ScreenshotRouter，不能把历史 Bridge 主力模式外推为
  云端默认模式。
- **ScreenshotRouter 双模式自动降级**：CDP↔Extension 自动切换
- **三层采集架构（L1/L2/L3）**：L1 Network + L3 DOM 覆盖 5 引擎；L2 Hook 设计冻结
- **SPA 引擎截图统一15秒等待**：collect/screenshot/collect_and_capture 三种 action 统一等待 15 秒 + 滚动触发懒加载 + 2 秒稳定等待。不同引擎差异化等待不可靠，统一最长等待更简单（2026-06-18 从 4/6 秒统一为 15 秒）
- **`collect_and_capture` 一次导航完成采集+截图**：避免分步调用导致页面重载丢失搜索结果
- **CSP `unsafe-inline` 完全移除**：21 静态 style→CSS 类，28 JS inline style→CSS 类，动态颜色用 CSS 变量
- **所有管理端点应启用限流**：即使有 admin auth 保护
- **操作历史持久化**：通用 SQLite 表，支持多类型操作
- **错误响应统一包 model.APIResponse**：成功响应仍按 handler 定义，不能假设所有 API 使用统一成功信封。
- **handleScreenshot 统一走 Router**：不再直接创建 chromedp allocator
- **server.go 拆分完成**：1335→1128 行（middleware_security.go + server_helpers.go）

## 关键技术约束

- **Chrome MV3 service worker 不热重载**：修改 capture.js/background.js 后必须完全退出 Chrome 再重启
- **Chrome extension 热更新需要 Remove+re-add**
- **Go JSON string→int 静默失败**：JS 端数值字段必须用 `parseInt()` 转换
- **API 路径前缀是 `/api/v1/`**
- **Hunter 429 不重试是设计决策**
- **map→struct 迁移必须 grep JSON tag 冲突**
- **`style.display=''` 陷阱**：只在元素无隐藏 class 时才显示。有 `hidden-init`（`display:none`）等 class 时必须用 `display='block'`/`'inline-block'`，否则 class 接管元素仍隐藏（2026-06-18 通知表单/飞书字段 bug 根因）
- **`sync.Once.Do` 覆盖陷阱**：Once 闭包无论变量是否已赋值都会执行，会覆盖之前的 `SetXxxConfig`。懒加载默认值前必须检查 `if globalVar == nil`（2026-06-18 限流 60→300/min bug 根因）
- **CSP 拦截 innerHTML 内联 handler**：`script-src 'self' 'nonce-xxx'`（无 `unsafe-inline`）不仅拦截 HTML 里的 `onclick=`，也拦截 innerHTML 字符串里的。动态元素必须用 `addEventListener`
- **多用户模式**：`validateLoginCredentials` 是 DB 优先 → config 降级。要禁用 config 降级，清空 `config.yaml` 的 `web.auth.username`/`password_hash` 即可（代码无需改，字段为空时返回 "login not configured"）
- **admin token 越权**：`handleGetAdminToken` 返回明文 token，多用户模式下必须 `requireAdmin` 校验。单用户模式（userID=0）和 admin-token 身份（-1）放行
- **collect_and_capture 统一15秒等待**：所有 SPA 引擎（FOFA/Hunter/ZoomEye/Quake）搜索截图统一等待 15 秒+滚动触发懒加载+2 秒稳定等待，比差异化引擎策略更简单可靠。修改 background.js 后必须刷新 Chrome 扩展（2026-06-18）
- **extractImagePaths 双格式识别**：同时支持 `→`（批量截图格式）和 `保存:`（搜索引擎截图格式）两种路径分隔符（2026-06-18）
- **search_screenshot payload engine 在 extra 中**：定时任务搜索引擎截图的 `engine` 字段必须放在 `extra` 对象中：`{"query": "...", "extra": {"engine": "zoomeye"}}`
- **map→struct 迁移级联模式**：1) 定义类型化 struct → 2) 改 `parse*Response` 的 JSON unmarshal target + rawData 赋值 → 3) 改 `Normalize` 的 type assertion → 4) 改 `normalize*Item` 签名，移除 getStr/getInt 闭包 → 5) 改为包级函数（减少闭包开销）→ 6) 移除 `asset.Extra = data`（旧 map 透传）。每步都需同步更新测试的 RawData 构造。（2026-06-23）
- **行→结构体映射模式**（Fofa）：API 返回 `[][]interface{}` 时，定义 `fofaRowToItem(row, fieldNames)` 用 switch 按字段名映射列索引到 struct 字段。避免 map 中转。
- **反规范化条目模式**（Censys）：API 返回嵌套 services 数组时，在 `parse*Response` 中将每个 service 拆分为独立 RawData 条目并合并 host-level 元数据（location/AS/DNS）。Normalize 不再需要二次迭代嵌套数组。
- **json.Number 处理数值/字符串双态字段**（ZoomEye）：API 可能返回 `"asn": 15169`（数值）或 `"asn": "15169"`（字符串）。struct 字段用 `json.Number`，读取时调用 `.String()` 统一为字符串。避免 `string` 字段因 JSON 数值而静默为空。
- **json.RawMessage 处理变体响应**（Quake）：API data 可能是 `[]QuakeItem` 或 `{"list": [...]}`。用 `json.RawMessage` 先存原始 JSON，再尝试两种格式反序列化。避免 `interface{}` 类型断言链。

## 剩余长期项

> 在册引擎 7 个：核心 5（FOFA/Hunter/ZoomEye/Quake/Shodan）的稳定 Web UI 已接入，历史
> Bridge 证据存在，但 CDP 结构化采集仍待真实定级；Censys/DayDayMap 仅 API 验证通过。
> BinaryEdge/Onyphe/GreyNoise 已于 2026-06-20 移除。

1. **L2 Hook** — 设计冻结，仅当 L1/L3 telemetry 证明收益时启动
2. ~~`map[string]interface{}` 强类型迁移~~ ✅ **Phase 7 完结**（799→~170，adapter 层 ~100→18，-82%）。7 引擎全部完成，剩余为 Web 响应/配额解析/变量嵌套
3. ~~**新增引擎（Censys/DayDayMap）API 查询端到端验证**~~ ✅ DayDayMap curl 200 OK + Censys v3 单 IP 200 OK（2026-06-23）
4. ~~**ZoomEye `cleanZoomEyeTitle`**~~ ✅ 已修（commit 7e619f8），title 中的元数据前缀已清理
5. ~~**Shodan `timestamp` 选择器为空** — 需真机调试（commit 50dc187 已修复 timestamp 字段流，待真机验证）~~ ✅ 已修复（`9debb8f`：`capture.js` `div.heading div.timestamp` 选择器 + `dom_selectors.go` 同步 + Go `LastSeen` 字段映射）
6. ~~**Extension 版本号待升**~~ ✅ 已升至 0.4.1
7. ~~**审计项 FINDING-002~008**~~ ✅ 6 项全部闭环（2026-06-22）

### 运维项（2026-06-23 核实）

| 项 | 核实状态 |
|----|----------|
| 优雅关闭 | ✅ 已实现（main.go ShutdownManager + server.go Shutdown） |
| Extension 版本号 | ✅ 已升至 0.4.1 |
| `*.log` gitignore | ✅ 已加 |
| `*.exe` gitignore | ✅ 已加（`.gitignore` 第 2 行，无 exe 被 git 跟踪） |

## Claude Code 记忆（2026-06-15 合并）

> 从 `.claude/projects/D--Project-Go-project-unimap/memory/` 复制 26 个文件

### 最近工作（2026-06-12 ~ 2026-06-11）

- [查询页修复 2026-06-12](claude-code-memory/project_query_fixes_2026-06-12.md) — ✅ loading/超时/引擎过滤/域名修正/登录状态/页面重复打开；⏸ DOM选择器失效+API Key未配置
- [安全与稳定性修复 2026-06-11](claude-code-memory/project_repair_pause_state_2026-06-11.md) — ✅ 四轮19项修复：CRITICAL数据竞争+P0 XSS×7+P1 panic/幂等/同步+P2 logger/dockerignore
- [当前未完成问题 2026-06-11](claude-code-memory/project_open_issues_2026-06-11.md) — ✅ P0 Bridge状态语义统一+状态抖动+Token复制；⏸ 新增引擎端到端+UQL历史+长期项

### 最近工作（2026-06-15）

- [Quake采集验证更新 2026-06-15](claude-code-memory/project_quake_collection_complete_2026-06-15.md) — ✅ Quake采集成功（10 items, card_fallback方法）；✅ 5引擎全部打通；✅ 根因澄清：URL格式过期非反爬拦截
- [ZoomEye选择器更新 2026-06-15](claude-code-memory/project_zoomeye_selector_update_2026-06-15.md) — ✅ ZoomEye DOM选择器更新（capture.js + dom_selectors.go）；✅ WebOnly采集验证成功（10条数据）；✅ web包测试覆盖率42.6%→54.6%（+12%）
- [截图SPA渲染时机修复 2026-06-15](claude-code-memory/project_screenshot_spa_timing_fix_2026-06-15.md) — ✅ Extension screenshot加4000ms SPA等待；✅ Go Bridge调用改用"spa"策略；✅ 新增collect_and_capture action一次导航完成采集+截图；✅ Go CollectAndCaptureSearchEngineResult重构

### 审查与修复（2026-06-10 ~ 2026-06-09）

- [全量审查修复完成 2026-06-10](claude-code-memory/project_audit_fix_complete_2026-06-10.md) — P0×4+P1×14+P2×17=35项全部闭环
- [全量项目审查 2026-06-09](claude-code-memory/project_full_audit_2026-06-09.md) — 6用户旅程维度，4P0+14P1+17P2
- [代码质量治理 2026-06-09](claude-code-memory/project_code_quality_refactor_2026-06-09.md) — 文件拆分+函数拆分+API shim移除

### 引擎适配（2026-06-07 ~ 2026-06-08）

- [全5引擎采集验证 2026-06-07](claude-code-memory/project_5engine_collection_verification_2026-06-07.md) — 5/5引擎Extension采集打通
- [适配器语法全面修正 2026-06-07](claude-code-memory/project_adapter_syntax_fix_2026-06-07.md) — 5引擎14项语法修正+12条新测试
- [引擎适配器下一步计划 2026-06-07](claude-code-memory/project_engine_adapter_next_steps_2026-06-07.md) — 阶段一核查+阶段二全部完成
- [阶段二引擎+语法核查 2026-06-08](claude-code-memory/project_engine_adapter_phase2_2026-06-08.md) — 5引擎语法全量核查+10个适配器完成
- [Shodan调试 2026-06-07](claude-code-memory/project_develop_commits_shodan_debug_2026-06-07.md) — Shodan 0 assets根因修复
- [SEC-1 token轮换 2026-06-08](claude-code-memory/project_sec1_token_rotation_2026-06-08.md) — token轮换+docs打码+gitignore

### 2026-06-22/23 引擎适配器 map→struct 强类型迁移（Phase 7 完结）

- ✅ **7 个引擎全部迁移**：
  - **Shodan** (`7fb4998`): `ShodanSearchResponse` + `ShodanMatch` 结构体，`normalizeShodanMatch` 替代 `normalizeShodanItem`
  - **Hunter** (`7fb4998`): `HunterItem` 结构体（含 `Web`/`Location` `map[string]interface{}` 降级字段），`normalizeHunterMatch` + 包级 `parseHunterLegacyFields`
  - **Fofa** (`7fb4998`): `FofaItem` 结构体 + `fofaRowToItem` 行→结构体映射函数，`normalizeFofaItem` 包级化
  - **DayDayMap** (`a2a6beb`): `DayDayMapItem` + `dayDayMapSearchRequest` 结构体，修复 3 个使用旧 GET API 格式的测试
  - **Censys** (`4b8de2b`): 14 个类型化结构体（`CensysRawEntry`/`CensysService`/`CensysHTTP`/`CensysTLS` 等），25→0 map
  - **ZoomEye** (`1476490`): `ZoomEyeItem` 扩展点号字段 + `json.Number` for ASN + `zoomEyeSearchRequest`，17→5 map
  - **Quake** (`1476490`): `QuakeItem`/`QuakeService`/`QuakeHTTP`/`QuakeLocation` + `quakeSearchRequest`，12→6 map
- 📊 **adapter 层 map[string]interface{}: ~100 → 18（减少 82%）**
- 🔒 剩余 18 处均为有意保留：Hunter Web/Location (2) | ICP Extra (2) | orchestrator_circuit API (3) | Quake 配额解析 (6) | ZoomEye PortInfo/GeoInfo (5)
- 🏆 **map→struct 迁移正式完结**

### 架构与采集（2026-06-05 ~ 2026-06-09）

- [L1 Network采集集成 2026-06-09](claude-code-memory/project_l1_network_integration_2026-06-09.md) — combined collect+capture路径集成L1
- [ARC-4抓包spike 2026-06-09](claude-code-memory/project_arc4_packet_capture_2026-06-09.md) — 5引擎全量验证：Hunter/ZoomEye/Quake为SPA→L1有价值
- [Extension反爬虫架构 2026-06-05](claude-code-memory/project_extension_anti_scraping_2026-06-05.md) — CDP vs Extension对比、stealth 5阶段实施方案
- [三层采集架构差距 2026-06-06](claude-code-memory/project_three_layer_collection_gap_2026-06-06.md) — 🔴 设计阶段；Phase -1需真机抓包
- [浏览器查询降级 2026-05-29](claude-code-memory/project_browser_fallback_2026-05-29.md) — 阶段1-5全部闭环

### 截图与认证（2026-06-03）

- [飞书截图+乱码+超时 2026-06-03](claude-code-memory/project_feishu_screenshot_2026-06-03.md) — 乱码/截图超时/Bridge认证/路径泄露闭环
- [Bridge认证改造 2026-06-03](claude-code-memory/project_screenshot_bridge_auth_2026-06-03.md) — Admin Token fallback+pairing/signature联动
- [P1/P2 + Extension重配对 2026-06-03](claude-code-memory/project_p1_p2_dom_selectors_2026-06-03.md) — 5/5引擎Extension采集打通

### 早期工作（2026-05）

- [前端全量审计修复 2026-05-28](claude-code-memory/project_frontend_audit_fix_2026-05-28.md) — 12项交互问题闭环
- [API版本化闭环 2026-05-31](claude-code-memory/project_api_versioning_2026-05-31.md) — /api/v1双注册+55文件迁移
- [问题修复报告 2026-06-01](claude-code-memory/project_issue_fix_2026-06-01.md) — 4类问题+Hunter限流根因修复
- [ICP GBK编码修复](claude-code-memory/project_icp_gbk_encoding_fix.md) — Windows GBK编码问题修复
- [已知问题清单 2026-05-09](claude-code-memory/project_remaining_issues_2026-05-09.md) — 原始问题清单
- [Quake反爬非权限问题 2026-06-07](claude-code-memory/project_quake_antiscraping_not_permission_2026-06-07.md) — Quake采集失败是反爬拦截

### 2026-06-17 新引擎代码基础设施全量补齐 + 空桩修复

- ✅ **countGoroutines() 空桩**：`return 0` → `runtime.NumGoroutine()`，加 5 协程容差，20 次 `-race` 通过
- ✅ **新引擎全层级集成**：
  - **10 引擎启用**：CLI + Web UI（设置页/查询页）+ Config API（GET/POST）+ 引擎重载 + 登录检测 + 稳定引擎列表 → 全部从 5 扩展到 10
  - **Censys 特殊处理**：`api_id`/`api_secret` 字段（非 `api_key`），Go 端 `applyCensysFields` + JS 端 `saveAllEngines`/`loadConfig` 适配
  - **BinaryEdge 补齐**：Extension 引擎检测 + DOM 选择器 + Go `dom_selectors.go` 选择器
  - **Extension 版本**：0.3.8 → 0.3.9
- ✅ `go build ./...` / `go vet ./...` / `go test -race ./...` 全部通过（33/33 包）
- ⏳ **待真机验证**：5 个新引擎需要 API Key 进行 API 查询验证

### 2026-06-16 Hunter DOM提取修复 + 采集闭环

- ✅ **collect_and_capture 语法错误修复**：`else if` 在 `else` 之后 → 重构 if/else 链
- ✅ **collect_and_capture 变量错误修复**：`result` → `collectResult`（Critical）
- ✅ **HasMore 字段添加**：`BridgeCollectedData` 新增 `HasMore bool`
- ✅ **Parser 一致性**：`ParseStructuredCollectedDataFromItems` 接受 `hasMore` 参数，使用 `extractPortFromHost` helper
- ✅ **Login wall 处理**：JS + Go 双侧添加 `collect_and_capture` login wall 检测
- ✅ **CDP 选择器更新**：FOFA/Hunter/Quake 从旧 table 布局更新为新 SPA 卡片布局
- ✅ **Extension 版本迭代**：0.3.2 → 0.3.8（5 次 Chrome 重载测试）
- ✅ **Hunter 提取优化**：去重（ip:port）、protocol 只保留协议名、raw cell text 替代 tooltip
- ✅ **Hunter country/title/host 清理**：`NormalizeAssets` + `CleanHunterFields` 全链路生效，browserCollectedData 已同步清理
- ✅ **collection/parser 测试**：新增 7 个测试函数、55+ 子用例，覆盖全部 8 个导出函数
- ✅ **FOFA 采集验证**：10 条资产，字段完整，截图正常

### 已知问题（2026-06-16 全部核实）

**Critical（1 项）**
1. **截图未登录状态** — Extension 通过 `chrome.tabs.create` 打开新标签页，未继承用户已登录的 session cookies。FOFA 截图显示"登录"按钮，Hunter 显示"未登录仅展示部分数据"。需排查 tab 创建方式和 cookie 共享机制。

**High（4 项）**
2. ~~**Hunter country_code 为城市名**~~ ✅ 已修复（2026-06-16，`NormalizeAssets` + `CleanHunterFields` 生效）
3. ~~**Hunter title 含分类标签**~~ ✅ 已修复（2026-06-16，标题截断规则已覆盖）
4. ~~**Hunter host 含 UI 噪声**~~ ✅ 已修复（2026-06-16，UI 噪声清理已覆盖）
5. ~~**CleanHunterFields 调用链断裂**~~ ✅ 已修复（2026-06-16，parser / fallback / web payload 全链路调用）

**Medium（5 项）**
6. ~~**653 处 `map[string]interface{}`**~~ ✅ Phase 1-7 完成（799→~170，减少 79%）。核心引擎适配器全部类型化，剩余为 Web 响应负载、ZoomEye/Quake 适配器、测试文件。
7. **新引擎端到端未闭环** — Censys/DayDayMap/Onyphe/GreyNoise 适配器代码存在（有 `Search` 方法），但 Extension DOM 选择器缺失、UI 未暴露这些引擎。
8. **L2 Hook 设计冻结** — 仅当 L1/L3 telemetry 证明收益时启动。
9. ~~**web/ flaky test**~~ ✅ 已修复（2026-06-16，稳定输入+`-race` 复核通过）
10. **定时任务缺少简易定时功能** — scheduler 只支持 cron 循环表达式，无一次性延迟执行、简易间隔设置、指定时间点执行。

**Low（3 项）**
11. **countGoroutines() 空桩** — `router_test.go:400` 硬编码 `return 0`，`TestRouterStartStop_NoGoroutineLeak` 无效。
12. **ZoomEye 哈希 CSS class** — `span._public-hover_uxlu6_1` 是编译哈希，前端改版即失效。
13. ~~**extractPortFromHost IPv6 边界** — `strings.LastIndex(s, ":")` 对 `2001:db8::1` 会错误解析为 port=1。~~ ✅ 已修复（2026-06-16，`net.SplitHostPort` 支持 `[IPv6]:port`，裸 IPv6 不再误判端口）

## 当前活跃

- [项目当前状态与后续行动 2026-07-24](project_current_state_2026-07-24.md) — 本地基线已提交，
  下一主线是浏览器层 SSRF、云端巡检闭环和真实 CDP 引擎验收
- [CI 全绿 + GUI 构建修复 2026-06-29](project_ci_green_2026-06-29.md) — ✅ CI 全绿：build/vet/gofmt/race/govulncheck/Extension JS 全部通过；修复 `monitor_native.go` 类型名 `apiTamperHistoryItem` → `guiTamperHistoryRecord`
- [CI 收尾执行清单 2026-06-26](project_ci_closeout_checklist_2026-06-26.md) — ✅ 已闭环：CGO_ENABLED=1、依赖升级、LF 行尾治理、govulncheck 0 漏洞
- [巡检功能增强计划 2026-06-25](project_monitoring_enhancement_plan_2026-06-25.md) — ✅ 全部完成（5 Phase）：误报修复 + 指纹引擎(107规则) + HTTP指纹 + UA池 + 端口联动；+28 测试；页面"监控"→"巡检"
- [截图等待时间统一15秒 + 飞书应用图片推送修复 2026-06-18](project_screenshot_wait_timing_fix_2026-06-18.md) — ✅ 5引擎截图完整+飞书推送正常；collect_and_capture 统一15秒等待+滚动触发懒加载；extractImagePaths 双格式识别
- [Extension 模式问题 2026-05-09](project_extension_mode_issues_2026-05-09.md) — ✅P0已修复(Shodan补齐+翻译路径验证)、✅P1进度已实现、P1登录状态部分解决
- [Bridge 认证修复 2026-06-03](project_bridge_auth_fix_2026-06-03.md) — ✅截图超时根因(重启丢token→401)修复：admin token loopback 兜底+签名/pairing联动+5测试+真机curl E2E全绿
- [定时任务优化进度 2026-06-02](project_scheduler_optimization_2026-06-02.md) — P3-P6完成详情+修复记录+后续计划
- [引擎集成与采集状态](project_engine_integration_2026-06-17.md) — 历史引擎集成与真机验证记录；当前选择器以扩展源码为准

## 当前文档（docs/）

- [CI 全绿 + GUI 构建修复 2026-06-29](../docs/archive/CI_CLOSEOUT_CHECKLIST_2026-06-26.md) — 历史 CI 收尾资料
- [巡检功能增强计划 2026-06-25](../docs/archive/MONITORING_ENHANCEMENT_PLAN.md) — 历史计划与验收资料
- [E2E采集验证 2026-06-04](../docs/archive/E2E_COLLECTION_VERIFICATION_2026-06-04.md) — 历史采集验证资料
- [问题修复报告 2026-06-01](../docs/archive/FIX_REPORT_2026-06-01.md) — 历史修复报告
- [项目审查报告 2026-06-01](../docs/archive/PROJECT_REVIEW_2026-06-01.md) — 历史审查报告
- [定时任务下一步计划](../docs/archive/SCHEDULER_NEXT_STEPS.md) — 历史计划

## 已归档（docs/archive/）

| 目录 | 内容 |
|------|------|
| [plans/](../docs/archive/plans/) | 9 文件：ICP/通知/FOFA/浏览器降级策略/修复计划/定时任务优化计划 |
| [reviews/](../docs/archive/reviews/) | 10 文件：2026-05-19~06-01 审查链 + 定时任务P3P4报告 + 项目审查05-31 |
| [COMMIT_HISTORY_2026-05.md](../docs/archive/COMMIT_HISTORY_2026-05.md) | 2026-05 提交历史（git 已跟踪，冗余归档） |

## 已归档（memory/archive/）

| 文件 | 说明 |
|------|------|
| [全量代码审查修复 2026-05-27](archive/project_code_review_fix_report_2026-05-27.md) | 55文件+625/-3700行，20项修复 |
| [安全修复 2026-05-07](archive/project_security_fix_2026-05-07.md) | 第三轮 review 遗留 13 项修复 |
| [通知与 FOFA 接入 2026-05-23](archive/project_notify_fofa_2026-05-23.md) | Webhook/Log + FOFA adapter |
| [ICP 定时任务 2026-05-22](archive/project_icp_scheduled_task_2026-05-22.md) | ST-21 TaskICPQuery + ICPQueryRunner |
| [静态版本修复 2026-05-18](archive/project_static_version_fix_2026-05-18.md) | staticVersion 传递修复 |
| [测试覆盖率进度 2026-04-21](archive/project_test_coverage_phase1_2026-04-21.md) | Phase 1-3 进度，整体约 75% |
| [实施指南完成 2026-04-16](archive/project_implementation_guide_progress_2026-04-16.md) | 10步全部完成 |
| [跨平台适配 2026-04-13](archive/project_crossplatform_2026-04-13.md) | macOS/Linux 6 项适配 |

## 历史归档

- [CLAUDE.md 历史修复记录归档 2026-07](project_claude_md_history_archive_2026-07.md) — CLAUDE.md 超过 40k 字符限制后移出的详细历史修复记录（2026-06-10 至 2026-07-20），包含安全审计、巡检修复、引擎适配器强类型迁移、前端交互修复等完整历史

## 核心文档（docs/）

| 文档 | 说明 |
|------|------|
| [ARCHITECTURE.md](../docs/ARCHITECTURE.md) | 分层架构 + 数据流向 |
| [RUNBOOK.md](../docs/RUNBOOK.md) | 运维故障处理（6 场景） |
| [API.md](../docs/API.md) | API 文档 |
| [API_VERSIONING.md](../docs/API_VERSIONING.md) | API 版本化方案 |
| [PLUGIN_DEVELOPMENT_GUIDE.md](../docs/PLUGIN_DEVELOPMENT_GUIDE.md) | 插件开发指南 |
| [PRODUCTION_READINESS_PLAN.md](../docs/PRODUCTION_READINESS_PLAN.md) | 生产就绪清单 |
| [OPS_SCREENSHOT_EXTENSION.md](../docs/OPS_SCREENSHOT_EXTENSION.md) | 截图扩展运维 |
