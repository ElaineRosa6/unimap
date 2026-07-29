# UniMap 安全发布收口修复计划

> 制定日期：2026-07-29
> 事实基线：`develop` / `0d45fad`，本地领先 `origin/develop` 7 个提交
> 当前状态：**实施中；仍禁止在外部验收完成前按“安全收口完成”发布**
> 适用范围：CDP、Extension Bridge、截图、浏览器采集、网页巡检、Quake 主链路、Excel 导出和发布文档
> 实施原则：测试先行、失败关闭、逐链路验收、真实证据先于“已完成”状态

## 1. 目的

本计划用于修复 2026-07-29 核查确认的发布阻断问题，并建立可重复验证的发布门槛。
目标不是让现有测试暂时通过，而是保证：

1. CDP 浏览器请求在导航、重定向、子资源和实际连接阶段均受 SSRF 约束；
2. 防护初始化、事件处理或队列异常时任务失败，不允许退化为无防护导航；
3. Extension Bridge 的能力边界与 CDP 分开声明，不能把提交前检查描述成逐跳防护；
4. 截图文件扩展名、实际字节格式、HTTP MIME 和通知上传契约一致；
5. 调用者取消或超时后，Bridge 排队和执行中的任务及时终止；
6. Quake 的 CDP L1、CDP DOM、Extension DOM、Web/Router 合并链路分别验收；
7. 消除当前可达依赖漏洞并同步所有权威文档。

## 2. 已确认问题与目标状态

| ID | 优先级 | 问题 | 目标状态 |
|---|---|---|---|
| SEC-01 | P1 | `Fetch.enable` 在 executor 初始化前执行，失败后仅告警继续 | 在有效 Target executor 上启用；任何启用失败均终止任务 |
| SEC-02 | P1 | DNS、重定向、资源类型和队列异常覆盖不完整 | 全请求拦截、连接级地址约束、错误 fail-closed |
| IMG-01 | P1 | JPEG 字节以 `.png` 和 `image/png` 对外 | 仓库截图统一为真实 PNG，或全链路携带明确格式元数据 |
| QUAKE-01 | P1 | Quake 直接 CDP 有证据，但 Web 主链路采集为空 | CDP 与 Extension 两条链路分别获得非空结构化结果和截图证据 |
| BRIDGE-01 | P2 | Submit 取消不传播到排队/执行任务 | 已取消任务不启动，执行中任务观察到取消 |
| TEST-01 | P2 | 显式 headless E2E 被 SSRF 门禁破坏 | 使用受控测试策略，不绕过生产防护语义 |
| DEP-01 | P2 | Excelize `v2.8.0` 命中可达的 `GO-2026-5960` | 升级到已修复版本，导出回归和 `govulncheck` 通过 |
| DOC-01 | P2 | 引擎、SSRF、日期、选择器和提交数状态不一致 | 权威文档与代码及真实验收证据一致 |

“没有发现 P0”仅表示本次已核查范围内没有确认的 P0，不代表完成了全仓库安全审计。

### 2.1 本轮不扩大的范围

以下工作继续按原计划推进，不纳入本轮发布阻断修复：

- Censys、DayDayMap 的稳定 Web UI、Bridge 或 CDP 新能力；
- FOFA、ZoomEye、Shodan 的真实账号 CDP 定级；
- 与上述阻断项无关的大规模 Router、截图或巡检架构重写；
- 为追求“全绿”而放宽 SSRF 私网限制或加入生产可配置的测试 allowlist。

## 3. 实施前约束

### 3.1 保护当前工作区

当前已有以下未提交工作，不得覆盖、回退或当作已完成：

- `internal/screenshot/headless_e2e_test.go`
- `internal/screenshot/manager.go`
- `internal/screenshot/manager_collect.go`
- `internal/screenshot/ssrf_guard.go`
- `internal/screenshot/ssrf_guard_test.go`
- 未跟踪目录 `docs/.claude/`

开始实施前必须：

1. 记录 `git status --short --branch` 和 `git diff --stat`；
2. 逐项审查现有 SSRF 改动，保留可用测试与设计，修正当前 `context canceled` 回归；
3. 不使用 `git reset --hard`、`git checkout --` 或其他覆盖用户改动的命令；
4. 每个阶段独立提交，避免把依赖升级、行为修改和文档更新混入同一提交。

### 3.2 发布期间的功能边界

在 SEC-01、SEC-02 及对应真实验收完成前：

- “发现变化 → 自动证据截图 → 图片通知”保持禁用；
- 防护失败时不得自动降级到无连接级防护的 Extension 任意 URL 截图；
- 搜索引擎采集只能按已声明的引擎/域名范围运行；
- 不得在文档、UI 或日志中声称“浏览器层 SSRF 已完成”。

## 4. 目标安全模型

浏览器 SSRF 防护分为三层，三层缺一不可。

### 4.1 第一层：导航前 URL 策略

负责拒绝：

- 非 `http` / `https` scheme；
- 空主机、用户信息混淆和非法端口；
- IP 字面量形式的 loopback、private、link-local、unspecified 地址；
- 大小写、尾点及子域形式的 localhost；
- DNS 解析失败、空结果、私网/公网混合解析结果；
- 未在搜索引擎允许范围内的引擎跳转。

建议保留 `ValidateBrowserURL` 作为纯静态校验，引入可注入 resolver 的 live 校验对象，
不要在测试中把整个 validator 替换成永久返回 `nil`。

### 4.2 第二层：CDP Fetch 全请求逐跳门禁

使用 `URLPattern: "*"` 在 Request 阶段暂停所有可拦截请求，不按资源类型列举白名单。
Document、iframe、Script、XHR、Fetch、Image、Stylesheet、Font、Media 等统一进入同一策略。

以下情况必须 fail-closed：

- `Fetch.enable` 失败；
- 事件队列溢出；
- paused event 缺少 URL；
- validator、`ContinueRequest` 或 `FailRequest` 返回错误；
- interceptor goroutine 异常退出；
- 调用者或任务上下文取消。

拦截器必须返回调用方后续实际使用的 browser context，并提供可检查的终态错误。
所有 Manager chromedp 入口必须复用同一 guard 工厂，禁止各自复制接线。

### 4.3 第三层：连接级出口约束

Fetch 暂停时解析域名、随后再由 Chrome 自行解析，仍存在 DNS rebinding/TOCTOU 窗口。
因此“安全收口完成”必须增加实际连接级约束，优先方案是：

1. 启动 loopback 浏览器出口代理；
2. 代理在每次 HTTP、HTTPS CONNECT 和 Upgrade 实际拨号前解析目标；
3. 拒绝任一受限地址，并把连接固定到已验证地址；
4. Chrome allocator 统一通过该代理访问外部网络；
5. 远程调试/固定 Profile 模式在启动 Chrome 时显式配置该代理；
6. 外部上游代理只有在能证明执行等价目标地址校验时才允许使用。

建议新增：

- `internal/screenshot/browser_url_policy.go`
- `internal/screenshot/browser_egress_proxy.go`
- `internal/screenshot/browser_guard.go`

如果本轮不实现连接级出口约束，则只能将 SEC-02 标为“部分完成”，任意 URL 自动截图和
巡检证据截图继续禁用，不能以 Fetch + DNS 预检查替代连接级验收。

### 4.4 Extension Bridge 边界

Extension 不能因为服务端提交前执行过静态检查就标记为逐跳安全。

- 搜索引擎任务使用明确的引擎域名及必要 SSO 域名 allowlist；
- 回调记录最终 URL、登录墙和域名漂移，超出范围时任务失败；
- 任意 URL 截图仅在浏览器由受控出口代理启动时允许；
- 未满足连接级约束时，Router 不得把 CDP 安全失败自动降级到 Extension；
- Bridge 配对、拉取、回调和 token 轮换继续只允许 loopback。

## 5. 分阶段实施

### 阶段 0：冻结事实基线并审查现有 WIP

复杂度：低；依赖：无。

行动：

1. 对比当前工作区与 `0d45fad` 的 SSRF 改动；
2. 复现并记录两类 headless 失败：
   - `0d45fad`：受控 loopback fixture 被静态门禁拒绝；
   - 当前 WIP：`Fetch.enable` 返回 `context canceled`；
3. 建立问题编号与测试矩阵，不在此阶段修改业务行为；
4. 将本计划状态从“待确认”改为“实施中”前取得明确确认。

完成标准：

- 现有改动来源和保留范围明确；
- 每个问题都有失败测试或可复现命令；
- 未覆盖用户工作区改动。

### 阶段 1：先写 SSRF 策略与拦截器失败测试

复杂度：高；依赖：阶段 0。

涉及文件：

- `internal/screenshot/ssrf_guard.go`
- `internal/screenshot/ssrf_guard_test.go`
- `internal/utils/urlguard/`
- 新增的 browser policy/guard/egress proxy 文件及测试

先新增失败测试：

1. `LOCALHOST`、`localhost.`、`x.localhost`；
2. 域名解析到 loopback/private/link-local/unspecified；
3. 同时解析到公网和私网；
4. DNS 错误或零地址；
5. redirect 从允许目标转向受限目标；
6. Image、Stylesheet、Font、Media 等子资源访问受限目标；
7. interceptor 初始化失败；
8. 可控的小容量队列溢出；
9. `ContinueRequest` / `FailRequest` 失败；
10. caller cancel 后 listener 和浏览器任务退出；
11. 实际出口代理拒绝受限地址且目标服务没有收到连接。

实现要求：

- resolver、队列容量和 CDP 命令执行器提供最小测试 seam；
- 初始化必须通过 `chromedp.Run` 在有效 executor 上执行；
- 不记录 Cookie、token、完整敏感查询或内部 URL 参数；
- 错误包含阶段上下文，但安全包装底层信息。

阶段验证：

```bash
go test -race ./internal/utils/urlguard ./internal/screenshot -run 'Test.*(SSRF|BrowserGuard|Egress)' -count=1
```

### 阶段 2：接入所有 CDP/chromedp 业务入口

复杂度：高；依赖：阶段 1。

必须逐项接入：

- `CaptureScreenshotWithProxy`
- `OpenSearchEngineResult`
- `collectViaDOM`
- `CollectViaNetwork`
- `CollectAndCaptureSearchEngineResult`
- `ValidateSearchEngineResult` / `loadPageContent`
- `CheckEngineLoginStatus`
- Tamper `ComputePageHash` chromedp fallback
- 手动截图、批量截图、调度截图和巡检使用的公共入口

涉及文件：

- `internal/screenshot/manager.go`
- `internal/screenshot/manager_collect.go`
- `internal/screenshot/network_collector.go`
- `internal/screenshot/manager_chrome.go`
- `internal/tamper/detector_hash.go`
- `web/server.go`
- 对应 service、scheduler 和测试文件

接线要求：

- 统一通过 `BrowserGuard` 创建和关闭上下文；
- guard 初始化失败立即返回错误；
- guard fatal error 优先于“截图成功”结果；
- Router 不得把安全错误当作普通 provider 健康错误进行不安全 fallback；
- 固定 Profile/远程调试 Chrome 必须验证其连接级出口配置。

阶段验证：

```bash
go test -race ./internal/screenshot ./internal/tamper ./internal/service ./internal/scheduler ./web
go test -tags headless_e2e ./internal/screenshot -run TestHeadless -count=1 -v
```

### 阶段 3：修复 headless E2E 和真实 SSRF 验收

复杂度：中；依赖：阶段 2。

测试设计：

- 为测试 fixture 注入仅允许“指定 IP + 随机端口”的策略，而不是 no-op validator；
- 使用第二个 loopback fixture 模拟禁止的重定向和子资源；
- 记录禁止 fixture 是否收到连接，以验证“连接前阻断”；
- 覆盖 JavaScript 执行、真实 PNG 截图、重定向、子资源和取消；
- 测试显式要求 `UNIMAP_CHROME_PATH`，缺少时给出清晰的环境错误。

完成标准：

- headless 测试连续运行 3 次通过；
- 受限目标没有收到连接；
- `-race` 无数据竞争和 goroutine 泄漏；
- 自动巡检证据截图仍保持禁用，等待阶段 8 的云端验收。

### 阶段 4：修复截图格式契约

复杂度：中；可与阶段 3 后半段并行，但不得与 SSRF 核心文件混合提交。

推荐决策：仓库内截图继续统一使用 PNG 契约。Chrome JPEG 回退成功后，在 Go 内解码 JPEG
并重新编码为 PNG，再交给文件、HTTP 和通知层。

涉及文件：

- `internal/screenshot/screenshot_fallback.go`
- `internal/screenshot/manager.go`
- `web/screenshot_handlers.go`
- `internal/notify/bot_channels.go`
- 对应测试

先写测试：

1. JPEG fixture 经规范化后具有 PNG magic；
2. 保存的 `.png` 可由 `image/png` 解码；
3. 单图 API 使用实际检测后的 MIME；
4. 通知上传前拒绝扩展名、MIME、magic 不一致的文件；
5. PNG 主路径不发生额外有损转换。

完成标准：

- 新生成的 `.png` 全部具有 `89 50 4E 47 0D 0A 1A 0A` magic；
- 单图 API、文件预览和飞书上传使用一致契约；
- 现有 Hunter JPEG 回退场景仍能完成截图。

如果选择保留 JPEG 文件，则必须改为显式 `CapturedImage{Bytes, MIME, Extension}`，
并同步所有 Provider、路径生成、HTTP 响应、前端和通知调用方；不得只改扩展名。

### 阶段 5：传播 Bridge 取消与超时

复杂度：中；依赖：无代码依赖，但建议在 SSRF 核心稳定后合入。

涉及文件：

- `internal/screenshot/bridge_service.go`
- `internal/screenshot/bridge_health_test.go`
- `internal/screenshot/provider_test.go`

实现要求：

- `bridgeJob` 携带从 Submit 派生的上下文；
- worker 取到任务后先检查上下文，已取消任务不调用 Bridge client；
- 执行中的 client/retry/await 全部继承任务上下文；
- Service Stop 同时取消执行中任务；
- response channel 保持有界且发送不得阻塞已返回的调用者。

先写测试：

1. 队列中取消后 client 调用次数为 0；
2. 执行中取消可被 fake client 观察；
3. Submit 超时后任务不会稍后启动；
4. Stop 可及时结束 worker；
5. collect/collect_and_capture 的长超时策略仍有效。

### 阶段 6：分离并闭环 Quake 两条链路

复杂度：中到高；依赖：阶段 2、4、5。

不得再把 `bridge-collect=0` 直接归因于 CDP L1 Network pattern。

分别建立以下验收：

| 链路 | 采集机制 | 最低通过标准 |
|---|---|---|
| CDP L1 | `network_collector.go` API response | 匹配真实请求，解析不少于 1 条资产 |
| CDP L3 | `dom_selectors.go` | L1 不命中时 DOM 返回不少于 1 条资产 |
| CDP combined | Manager collect_and_capture | 资产、截图、登录状态一致 |
| Extension | `tools/extension-screenshot/src/capture.js` | `bridge-collect` 非空，选择器诊断可见 |
| Web/Router | `/api/v1/query` | 强制 CDP、强制 Extension 分别测试，不依赖 auto 猜测 |
| 持久化/通知 | query workflow + scheduler | 合并结果可查询，图片真实送达 |

涉及文件：

- `internal/screenshot/network_collector.go`
- `internal/screenshot/network_collector_test.go`
- `internal/screenshot/dom_selectors.go`
- `tools/extension-screenshot/src/capture.js`
- `tools/extension-screenshot/src/background.js`
- `internal/screenshot/router.go`
- `internal/service/query_app_service.go`
- `web/live_bridge_notification_e2e_test.go`

真实验收记录不得包含 Cookie、token、手机号、用户 ID 或查询凭据。

### 阶段 7：升级 Excelize 并回归导出

复杂度：低；可在阶段 4—6 期间独立提交。

行动：

1. 将 `github.com/xuri/excelize/v2` 升级到 `v2.11.0` 或当时更高的兼容修复版本；
2. 执行 `go mod tidy`；
3. 回归查询结果、巡检结果和大数据量 Excel 导出；
4. 验证工作表名称、单元格值、样式、关闭和错误路径；
5. 重新执行官方漏洞扫描。

验证：

```bash
go test -race ./internal/exporter ./internal/service
govulncheck ./...
```

完成标准：`GO-2026-5960` 不再出现于可达漏洞结果，Excel 文件可由 Excel/LibreOffice 正常打开。

### 阶段 8：文档同步、云端验收与发布定级

复杂度：中；依赖：阶段 1—7。

必须更新：

- `README.md`
- `CLAUDE.md`
- `docs/ARCHITECTURE.md`
- `docs/API.md`
- `docs/RUNBOOK.md`
- `docs/OPS_SCREENSHOT_EXTENSION.md`
- `docs/REMAINING_WORK_2026-07-23.md`
- `docs/IMPLEMENTATION_PLAN_2026-07-23.md`
- `docs/CDP_VERIFICATION_2026-07-29.md`
- `docs/CHANGELOG.md`

文档规则：

- Hunter/Quake 只声明已实际通过的链路；
- CDP、Extension、Bridge、L1、L3 不混用；
- “SSRF 已完成”必须附连接级和逐跳测试证据；
- 提交领先数、核对日期由发布时实际命令生成；
- 历史报告保留当日事实，并增加勘误，不覆盖历史结果；
- 新建无凭据、无 PII 的验收证据文档。

云端验收：

1. 受控公网页面跳转到受限地址时被阻断；
2. 受控 DNS 变化不能使浏览器连接到私网；
3. Quake/Hunter 强制 CDP 分别返回结构化资产和有效截图；
4. Quake/Hunter 强制 Extension 分别定级；
5. 图片通知真实送达并可预览；
6. 容器重启后 guard、Chrome 会话和任务恢复行为一致；
7. 自动巡检证据截图通过以上门槛后才允许启用。

## 6. 依赖关系与建议提交顺序

| 顺序 | 建议提交 | 依赖 |
|---|---|---|
| 1 | SSRF 失败测试与 URL policy | 无 |
| 2 | BrowserGuard + Fetch fail-closed | 1 |
| 3 | 连接级出口代理及 allocator 接线 | 1、2 |
| 4 | 所有 chromedp 入口统一接线 | 2、3 |
| 5 | headless/SSRF E2E | 3、4 |
| 6 | PNG 格式契约 | 可独立，建议在 5 后 |
| 7 | Bridge 取消传播 | 可独立，建议在 5 后 |
| 8 | Quake CDP/Extension/Web 闭环 | 4、6、7 |
| 9 | Excelize 升级 | 可独立 |
| 10 | 文档、云端证据与发布定级 | 全部 |

禁止将尚未通过 headless 和连接级验收的 SSRF WIP 与“安全完成”文档一起提交。

## 7. 风险与缓解

| 风险 | 等级 | 缓解措施 |
|---|---|---|
| Fetch listener 中同步执行 CDP 命令导致死锁/取消 | 高 | listener 只入有界队列；单独执行循环；专门的 headless 压力测试 |
| DNS 预检查与 Chrome 实际解析不一致 | 高 | 使用受控出口代理固定实际拨号地址 |
| 固定 Profile/远程 Chrome 无法运行时注入代理 | 高 | 启动时强制配置并做 readiness 校验；否则禁用任意 URL 能力 |
| 安全错误触发 Router 不安全 fallback | 高 | 定义不可 fallback 的安全错误类型并覆盖路由测试 |
| 全资源拦截导致页面性能下降 | 中 | 记录指标并压测；禁止通过遗漏资源类型换取性能 |
| JPEG 转 PNG 增加 CPU/文件体积 | 中 | 限制尺寸与超时，记录转码耗时；不能以错误 MIME 作为优化 |
| Bridge 取消改变重试语义 | 中 | 保留 action 超时策略，增加排队/执行/重试三层测试 |
| Quake 页面或 API 再次变化 | 中 | 保存脱敏 fixture、记录 selector/pattern 诊断、保留 L1→L3 降级 |
| Excelize 升级存在 API 行为变化 | 低到中 | 独立提交、导出 golden/打开测试、必要时仅回滚依赖提交 |

## 8. 回滚策略

1. SSRF 新 guard 失败时回滚功能提交，但保持自动证据截图禁用；不得回滚到 fail-open 后发布；
2. 连接级代理出现兼容问题时，临时只允许明确搜索引擎域名并禁用任意 URL 浏览器任务；
3. PNG 转码失败时任务返回明确错误，不保存伪 PNG；
4. Bridge 取消修复按独立提交回滚，不影响 SSRF 与图片修复；
5. Quake CDP 和 Extension 修复分开提交，可单独定级为“受限”；
6. Excelize 若发生兼容回归，先阻断 Excel 功能或修复调用方，不得带已知可达漏洞发布；
7. 文档状态只随已验证代码回滚，不保留超前的“已完成”声明。

## 9. 最终发布门槛

### 9.1 自动验证

以下命令必须全部通过：

```bash
go test -race ./...
go vet ./...
go build ./...
go build -tags gui ./cmd/unimap-gui
go test -tags headless_e2e ./internal/screenshot -run TestHeadless -count=3 -v
node --check web/static/js/main.js
govulncheck ./...
git diff --check
```

如环境安装了 `staticcheck`、`golangci-lint`，同时执行并归档结果；未安装必须明确记录，
不得写成“通过”。

### 9.2 安全与业务证据

- Fetch 拦截器确认在有效 executor 上启用；
- 初始化、队列和 CDP 命令错误均 fail-closed；
- 重定向、DNS 变化和各类子资源不能连接受限地址；
- 所有 chromedp 业务入口完成接线清单；
- 新截图 magic、扩展名、MIME 和通知上传一致；
- Bridge 排队及执行取消测试通过；
- Quake CDP 与 Extension 分别定级，Web 合并链路有非空结果；
- Excelize 可达漏洞清零；
- 自动巡检证据截图只在云端真实验收后启用。

### 9.3 发布状态

满足全部门槛后，状态才可改为：

> 浏览器 SSRF 已完成所声明范围内的静态、逐跳和连接级防护；Quake/Hunter 已按 CDP 与
> Extension 分别定级；截图格式、取消传播、依赖漏洞和文档状态已收口。

任何一项未完成时，应发布为“受限预览”或继续阻断发布，不能使用“安全收口完成”。

## 10. 工作量估算

| 工作包 | 估算 |
|---|---:|
| SSRF policy、Fetch guard、连接级出口与测试 | 5—8 人日 |
| 全 chromedp 入口接线与 headless E2E | 2—4 人日 |
| 截图格式契约 | 1—2 人日 |
| Bridge 取消传播 | 1—2 人日 |
| Quake 双链路真实闭环 | 2—4 人日，取决于页面/API 与账号状态 |
| Excelize 升级和导出回归 | 0.5—1 人日 |
| 文档、云端验收和发布记录 | 1—2 人日 |
| 合计 | 12.5—23 人日 |

外部依赖包括：合法引擎账号和登录态、可见或远程调试 Chrome、受控 DNS/网页测试环境、
真实飞书测试目标及云端容器操作窗口。

## 11. 确认点

本文件定义修复与验收计划。2026-07-29 已确认以下实施边界：

1. 接受“连接级出口代理”为完整 SSRF 收口方案；
2. 接受截图继续以真实 PNG 为统一存储契约；
3. 接受自动巡检证据截图在云端验收前保持禁用；
4. 明确当前未提交 SSRF WIP 的归属与保留方式。
