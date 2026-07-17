# UniMap 代码逻辑、API 适配与用户体验问题报告

> 核对日期：2026-07-17
> 核对分支：`codex/audit-remediation-closeout`（截至 `80350dc`）
> 范围：Go 服务端、Web API、CLI、GUI、Web 模板、配置/调度/分布式任务持久化。
> 方法：直接阅读代码和测试、核对路由与调用方契约、运行 Go 静态检查与竞态测试；本次未使用 `qa-security-audit` 技能，也未执行外部引擎真实凭据联调。

> 修复状态更新（2026-07-17）：下列 14 项已在 `codex/audit-remediation-closeout` 完成修复与回归；问题细节保留为原始证据，实施与回滚说明见 [`REMEDIATION_GUIDE_2026-07-17.md`](REMEDIATION_GUIDE_2026-07-17.md)。

## 结论

初次核对确认了 **14 个问题**：P0 1 个、P1 9 个、P2 4 个。当前修复工作区已逐项处理；本报告以下证据描述的是修复前基线，不再代表当前实现。

初次核对时，自动化测试没有覆盖跨进程重启、CLI/GUI 到 Web API 的端到端契约、并发首次注册、禁用用户的既有会话，以及磁盘写失败后的内存/磁盘一致性。此次修复已补充 CLI 契约、并发首管理员、会话撤销、旧库迁移、持久化失败回滚及截图异步流程测试；真实外部引擎、扩展和生产 Redis 仍需按修复指南执行上线验收。

## 后续交互复核与闭环

初始 14 项修复完成后又进行了两轮前后端交互复核，未发现新的 P0/P1 残留。以下问题已在 `f89f642` 与 `80350dc` 中闭环；它们是对原报告 X-01/X-02 验收面的补充，不改变初始问题计数。

| 范围 | 复核发现 | 当前处理 |
|---|---|---|
| 批量截图 | monitor 页 10 分钟超时没有明确终态；主页轮询未处理 `failed` 终态，旧 timeout 还可能覆盖已完成结果 | timeout/失败均显示明确结果和 `job_id`，清理定时器并阻止重复结算 |
| ICP 设置 | 健康检查直接拼接对象型 `error`，可能显示 `[object Object]` | 统一通过错误提取函数展示服务端消息 |
| 通知编辑 | 列表 DTO 缺少非凭据字段，前端可能用空值覆盖已有凭据，类型变化语义不明确 | 列表仅返回必要非凭据字段；`preserve_existing=true` 原子保留留空凭据；禁止原地改变渠道类型 |
| 用户管理 | 角色更新失败仍可能显示成功；只读账号存在部分成功；删除失败直接显示对象错误 | 检查第二次请求结果，明确只读账号的部分成功，并统一解析错误对象 |
| 调度历史 | 非 2xx 或对象型错误可能被当作数组继续渲染 | 先检查 HTTP 状态与数组结构，再更新页面 |
| 文档状态 | Runbook 核对日期及操作语义未完全同步 | 已同步至 2026-07-17，并补充超时恢复与通知编辑说明 |

新增契约测试覆盖主页/monitor 截图轮询、账号管理、设置页、调度历史和通知通道编辑；通知测试同时验证凭据保留、渠道类型约束、非凭据字段往返与 SSRF 校验。

## 问题总表

| ID | 级别 | 类别 | 问题 | 状态 |
|---|---|---|---|---|
| L-01 | P0 | 配置/持久化 | Web 配置与 Manager 是两个副本，多处修改后保存了旧副本 | 已修复 |
| L-02 | P1 | 并发 | `s.config` 的替换/字段修改与大量无锁读取并存 | 已修复 |
| A-01 | P1 | CLI/认证 | 所有 API 子命令都不能携带默认启用的管理令牌 | 已修复 |
| A-02 | P1 | CLI/调度 API | 调度 CLI 的方法、路径、参数和响应结构均有错配 | 已修复 |
| A-03 | P1 | CLI/GUI/截图 API | CLI、GUI 把异步截图启动响应当成完成响应 | 已修复 |
| S-01 | P1 | 调度持久化 | 删除/停用先改内存再保存，保存失败不回滚 | 已修复 |
| D-01 | P1 | 分布式任务 | 节点离线/租约过期不会增加 `Attempt` | 已修复 |
| D-02 | P1 | 分布式持久化 | 队列快照写失败只记日志，且部分状态变化不保存 | 已修复 |
| U-01 | P1 | 用户/会话 | 禁用或删除用户后，已签发会话仍被中间件直接放行 | 已修复 |
| U-02 | P1 | 用户/并发 | 首个用户的“唯一管理员”判定不是原子操作 | 已修复 |
| S-02 | P2 | 调度状态 | 加载任务时调度失败仍保留 `enabled=true` | 已修复 |
| Q-01 | P2 | 查询缓存/API | 缓存命中时所有 `engineStats` 被写成 0 | 已修复 |
| X-01 | P2 | Web UX | account 页面不能解析标准对象型错误；截图轮询静默失败 | 已修复 |
| X-02 | P2 | Web UX | 设置页忽略 `restart_required/applied`，失败时仍更新状态点 | 已修复 |

## 详细问题

### L-01（P0）Web 配置双副本导致保存旧值

**证据链**

1. `internal/config/config.go:30-38` 明确让 `Manager.GetConfig()` 返回深拷贝。
2. `cmd/unimap-web/main.go:31-38` 取得该拷贝并传给服务；`web/server.go:246-247` 又分别保存 `config` 与 `configManager`。
3. `Manager.Save()` 在 `internal/config/config.go:96-103` 只读取 Manager 自己的副本。
4. 多个 handler 修改 `s.config` 后调用 `s.configManager.Save()`，因此落盘的是修改前的 Manager 副本：
   - 通知保存/删除：`web/notification_handlers.go:296-305`、`336-360`
   - Cookie 导入/保存与代理同步：`web/cookie_handlers.go:45-68`、`191-203`、`226-238`
   - CDP 地址：`web/cdp_handlers.go:570-578`
   - 自动生成管理令牌：`web/middleware_auth.go:197-212`
   - 旧版密码修改：`web/query_handlers.go:618-628`
   - 截图模式：`web/screenshot_handlers.go:1155-1163`
5. 正确模式已经存在于 `web/config_handlers.go:148-191`：克隆候选配置，调用 `SaveConfig(candidate)`，成功后才发布给 `s.config`。

**实际后果**

- 通知、Cookie、CDP 地址和截图模式在当前进程内可能立即可见，重启后恢复旧值。
- 自动管理令牌没有按日志所述保存，重启会重新生成，现有 API 客户端全部失效。
- 旧版密码修改返回成功，但重启后旧密码恢复。
- 保存失败时，部分 handler 仍返回成功；另一些虽返回 500，却没有正确回滚 `s.config`。

**建议**：禁止 handler 原地修改共享配置。所有写入口统一为“锁内克隆 → 修改候选 → 校验 → `SaveConfig(candidate)` → 成功后原子发布 → 刷新运行时依赖”，并增加逐入口“保存、重建 Manager、重新加载”的重启测试。

### L-02（P1）共享配置存在系统性数据竞态

`web/config_handlers.go:148-182` 在 `configMutex` 下替换 `s.config` 指针，但认证、跨域检查、历史、备份、节点、登录、截图等大量请求路径直接读取 `s.config`，例如 `web/middleware_auth.go:101-108,190-203`、`web/backup_handlers.go:14-45`、`web/history_handlers.go:66-67`。`web/screenshot_handlers.go:1156-1159` 还会不持锁直接修改字段。

这不是只读对象：配置保存、Cookie、通知、CDP、令牌和截图模式均会并发写入。当前竞态测试没有构造“保存配置同时处理请求”的场景，因此测试通过未覆盖该风险。

**建议**：Server 只保留一个不可变配置快照，通过 `atomic.Pointer[config.Config]` 或完整的读写锁访问；字段级原地修改全部改为候选副本发布。增加并发配置保存 + auth/query/scheduler 请求的 `-race` 测试。

### A-01（P1）CLI API 模式无法适配默认认证

`configs/config.yaml.example:154-156` 默认启用 Web 认证并要求 API/CLI 使用管理 token；`web/middleware_auth.go:127-156` 也没有把 query、tamper、screenshot、scheduler 设为公开接口。但 CLI 的 API 子命令只提供 `--api-base` 和 timeout，`cmd/unimap-cli/api_subcommands.go:221-267` 的请求辅助函数既没有 token 参数，也没有设置 `X-Admin-Token` 或 Bearer 头。

因此 `query`、`tamper-check`、`screenshot-batch`、`scheduler` 在默认配置下都会收到 401。当前辅助函数单元测试没有在认证中间件后执行真实 CLI 请求。

**建议**：提供 `--admin-token`、受支持的环境变量/安全配置读取，统一构造带认证头的 client；禁止将 token 写入日志。增加 CLI 二进制对 `httptest` 认证服务器的端到端测试。

### A-02（P1）调度 CLI 与服务端契约全面错配

服务端权威路由为 `GET /api/v1/scheduler/tasks`、`POST /tasks/create`、`POST /tasks/run` 和 `GET /history`（`web/router.go:147-155`；版本化转换见 `web/router.go:230-235`）。CLI 存在以下错配：

- `schedulerList`、`schedulerHistory` 调用 `doFormRequest`；该函数固定使用 `PostForm`（`cmd/unimap-cli/api_subcommands.go:221-227`），对 GET 路由会得到 405。
- `schedulerCreate` 发到 `/tasks`，服务端创建路径是 `/tasks/create`（CLI `:409`，handler `web/scheduler_handlers.go:199-209`）。
- `schedulerRun` 查找从未注册的 `id` flag，必然报 “-id is required”（CLI `:364-369`）。
- list/history handler 直接返回数组（`web/scheduler_handlers.go:313-325,585-611`），CLI 却解码 `{success,tasks}` / `{success,history}`。
- 创建 handler 返回 `{id,message}`（`web/scheduler_handlers.go:303-309`），CLI 却解码 `{success,task}`，即使路径修正也会打印空任务。
- history CLI 发送 `task_id`，handler 只读取 `task_type` 和 `status`，过滤条件无效。

**建议**：从一套共享的请求/响应 DTO 或契约测试生成/约束 CLI；为每个 scheduler 子命令建立 method、path、body、status、response 的表驱动端到端测试。

### A-03（P1）CLI 和 GUI 未适配异步截图协议

服务端 `POST /api/v1/screenshot/batch-urls` 立即返回 202 与 `{job_id,total,status}`，随后要求轮询进度（`web/screenshot_handlers.go:724-771,774-790`；文档也在 `docs/API.md:68-69` 明确说明异步）。

- CLI 却解码 `{batch_id,total,success,failed,results}`，随即打印 “Screenshot batch completed”（`cmd/unimap-cli/api_subcommands.go:33-39,204-217`）。结果通常是空 batch ID、0 成功、0 失败。
- GUI 只解码 `results`（`cmd/unimap-gui/monitor_native.go:1675-1699`）；202 JSON 可以成功解码为空切片，于是 `runBatchScreenshotPreferAPI` 不触发本地回退并把空结果当作 API 成功（`:1702-1713`）。

**建议**：CLI/GUI 统一实现 start → poll（含取消、超时、认证、服务端失败）→ final results；输出必须使用 `job_id`，只有终态才显示完成。

### S-01（P1）调度删除/停用在保存失败时不回滚

`internal/scheduler/scheduler.go:421-439` 在保存前先移除 cron/timer 和内存任务；`DisableTask` 在 `:478-495` 先停用并取消调度，最后才保存。两者返回保存错误时都不恢复原任务/调度。旧磁盘文件仍保留删除前或启用状态，重启后任务会复活或重新启用。

此外 handler 把 Delete/Enable/Disable/Run 的所有错误统一映射为 404（`web/scheduler_handlers.go:465-467,503-505,540-542,577-579`），持久化故障被误报为“资源不存在”。

**建议**：与 Add/Update/Enable 当前策略一致，先构造可持久化状态，写成功后再布置/撤销运行态；失败时可靠恢复。引入可判别错误类型，把 not found、validation、persistence、execution 分别映射为 404/400/500/503。

### D-01（P1）节点离线和租约过期不会消耗重派次数

显式失败重试会在 `internal/distributed/task_queue.go:389-411` 增加 `Attempt`。但节点离线的 `ReleaseNodeTasks` 只检查 `Attempt <= MaxReassign+1` 并重新排队，从不递增（`:491-529`）；租约过期回收也只检查后重新排队（`:532-554`）。

结果是这两条最常见的故障路径永远不会达到上限，坏节点/超时任务可以无限重派。现有 `failover_test.go` 通过手工设置 `Attempt` 验证上限分支，没有验证多次真实释放/过期是否推进计数。

**建议**：定义清晰的 attempt/reassign 语义，在 claim 或每次重新入队时原子增加，并记录失败原因和历史；用假时钟测试 N 次租约过期及 N 次节点下线后的终态。

### D-02（P1）分布式队列对快照写失败返回成功

生产队列使用 `web/server.go:380` 的磁盘快照路径。Enqueue/Claim/SubmitResult/Delete 修改内存后调用无返回值的 `saveLocked()`（例如 `internal/distributed/task_queue.go:168-194,237-248,423-424,485-488`）。`saveLocked` 遇到建目录、序列化、写文件或 rename 失败只记录日志（`:727-765`），上层 API 仍返回 success（`web/node_task_handlers.go:31-37,124-130,218-225`）。`ReleaseNodeTasks` 和租约回收改变状态后甚至没有调用保存。

进程崩溃/重启后可能重新发放已完成任务、复活已删除任务，或丢失刚入队任务。

**建议**：让保存返回 error；关键状态转换采用“写成功才确认”的语义，或改用事务型存储/WAL。后台回收也必须持久化，并暴露持久化失败指标与健康状态。

### U-01（P1）禁用/删除用户不能撤销已有会话

登录时会拒绝非 active 用户（`web/login_handlers.go:104-127`），但中间件只要解出 session cookie 就直接放行并写入 user ID（`web/middleware_auth.go:39-45`），不查询用户是否仍存在/active。`getCurrentUser` 查询用户后也不检查状态（`web/user_handlers.go:461-486`）。用户状态更新只是写数据库（`:208-270,290-300`），没有会话版本或撤销列表。

因此被禁用的用户仍可继续使用旧会话；即使用户已删除，不依赖 `getCurrentUser` 的普通受保护接口仍会被中间件放行。

**建议**：认证中间件对 DB 用户执行存在性与 active 校验，或把 session_version/disabled_at 放入可校验会话机制；禁用、删除、改密时使旧会话立即失效。

### U-02（P1）首次注册可并发创建多个管理员

`web/user_handlers.go:59-85` 先独立执行 `Count()`，随后以 `count == 0` 决定 admin 角色，再在 `:87` 创建用户。两个不同用户名的并发请求都可看到 count=0，并都以 admin 创建；数据库唯一约束只约束用户名，无法阻止两个管理员。公开注册判断 `web/middleware_auth.go:152-170` 也是另一次独立 count。

**建议**：把“仅当用户表为空时创建首个管理员”做成数据库事务/单条条件语句，失败请求在事务后重新走已认证管理员创建流程；添加并发启动测试。

### S-02（P2）加载失败的任务仍显示为启用

`internal/scheduler/scheduler.go:69-87` 加载持久化任务时先放入 map；若 `scheduleTask` 失败，只记日志并保留 `Enabled=true`。任务列表与 UI 会显示启用，但没有 cron/timer，且接口没有 degraded 状态。

**建议**：加载失败时明确标为 disabled/error，保存诊断字段并在 readiness/页面展示；不要让 `enabled` 同时表达“用户期望”和“当前已布置”。

### Q-01（P2）缓存命中的引擎统计恒为 0

正常查询在 `internal/service/unified_service.go:378-409` 按引擎生成实际统计；缓存命中却在 `:313-325` 为所有请求引擎写入 0，同时返回非空 assets 和 TotalCount。Web 与 CLI 都展示 `engineStats`，用户会看到“总数大于 0、各引擎均为 0”。

**建议**：缓存完整 QueryResponse 元数据，或从缓存资产的来源字段可靠重建统计；增加首次查询与缓存命中响应等价测试。

### X-01（P2）错误对象显示异常，截图轮询会静默卡住

- 标准错误是 `{success:false,error:{code,message,details}}`（`web/http_helpers.go:97-118`）。account 页部分操作直接把 `data.error` 传给 toast/HTML（`web/templates/account.html:190,211,296,330`），浏览器会显示 `[object Object]`；同页只有登录状态路径正确提取了 `error.message`（`:142`）。
- monitor 页的截图轮询不检查 `pResp.ok`，遇到错误对象只 `return`，JSON 解析/网络失败也静默忽略（`web/templates/monitor.html:542-574`）。401、任务丢失或服务故障时用户会看到进度长时间不动，10 分钟后按钮恢复但没有失败原因。

**建议**：全站只使用一个 `extractErrorMessage`/`apiFetch`；轮询设置连续失败阈值，显示最后错误并允许重试/取消。

### X-02（P2）设置页忽略生效状态且失败时更新状态点

配置 API 明确返回 `persisted`、`applied`、`restart_required`（`web/config_handlers.go:200-208`）。设置页保存截图和系统配置时只显示后端固定的 `message: "saved"`，没有提示重启（`web/templates/settings.html:788-818`）；通知入口的“重启后生效”也因 `result.message` 总存在而不会采用 fallback 文案（`:695-703`）。

`saveAllEngines` 不论 `result.success` 是否为 false，都会按用户勾选值更新状态点（`:732-743`），造成视觉上的假成功。

**建议**：所有保存动作统一解释 `persisted/applied/restart_required`；只有 `resp.ok && result.success` 时更新状态和重新加载配置，失败时保留服务端已知状态。

## 验证结果

以下命令在本次核对中通过：

```text
go vet ./...
go test -race ./...
go test -tags gui ./cmd/unimap-gui
node --check web/static/js/main.js
```

`staticcheck` 与 `golangci-lint` 在当前环境未安装，因此没有把它们的结果作为结论依据。外部 FOFA、Hunter、ZoomEye、Quake、Shodan、Censys、DayDayMap 未使用真实账号和在线配额联调；本报告中的“API 适配”结论主要针对仓库内 Web 服务与 CLI/GUI/Web 调用方的可验证契约。

## 建议修复顺序

1. **L-01 + L-02**：先统一配置所有权与原子发布，否则后续认证、通知、截图修复仍可能重启失效或产生竞态。
2. **A-01 + A-02 + A-03**：补 CLI 认证，再修 scheduler 和异步截图契约，并建立端到端契约测试。
3. **S-01 + D-01 + D-02**：修复任务系统状态转换和持久化确认语义，加入故障注入/重启测试。
4. **U-01 + U-02**：实现会话撤销与原子首管理员创建。
5. **S-02 + Q-01 + X-01 + X-02**：收敛状态表达、缓存元数据和前端错误/生效提示。

每一组修复完成后都应运行受影响包的 `go test -race`；全部完成后再运行全量竞态测试，并至少做一次“写配置/创建任务 → 结束进程 → 重启 → 核对状态”的真实重启回归。
