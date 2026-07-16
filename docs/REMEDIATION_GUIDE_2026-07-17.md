# UniMap 逻辑、API 与体验问题修复指南

> 日期：2026-07-17
> 对应问题报告：[`CODE_LOGIC_API_UX_REVIEW_2026-07-17.md`](CODE_LOGIC_API_UX_REVIEW_2026-07-17.md)
> 目标：修复报告中的 14 个确认问题，同时控制兼容性、数据迁移、并发和运行态回归。
> 实施状态：2026-07-17 已按本指南完成代码修复；最终验证结果见本文第 8 节。本文前半部分保留为风险与回滚指南。

## 0. 实施结果

| 问题组 | 完成状态 | 关键落点 |
|---|---|---|
| L-01/L-02 | 已完成 | Web 配置改为 `currentConfig` + copy-on-write `updateConfig`；写失败不发布候选值，移除 Cookie 旧原地写路径 |
| A-01/A-02/A-03 | 已完成 | CLI Bearer/token-file、调度 method/path/DTO/FlagSet 修正、CLI/GUI 异步截图轮询及接受后禁止本地重跑 |
| S-01/S-02 | 已完成 | 调度删除/停用回滚、任务与历史分离保存、可判别错误映射、`runtime_status`/`schedule_error` 与页面展示 |
| D-01/D-02 | 已完成 | 三条失败路径统一消耗重派次数；队列关键变更落盘失败回滚并返回错误 |
| U-01/U-02 | 已完成 | `session_version` 幂等迁移与逐请求校验；首管理员单条条件 INSERT |
| Q-01 | 已完成 | `v2` 查询 cache key 保存并恢复完整统计/错误元数据 |
| X-01/X-02 | 已完成 | 标准错误对象解析、轮询错误可见、设置页按 applied/restart_required 展示且失败不更新状态点 |

兼容性说明：旧 `userID:adminToken` Cookie 按隐式 session version 0 读取；发生禁用或改密后版本递增并立即失效。调度 `enabled` 字段语义不变，新增字段仅为响应加法。

## 1. 总体判断：修复本身可能引入什么问题

这些问题不能用“把报错行改掉”的方式处理。主要二次风险如下：

| 修复组 | 修复可能引入的新问题 | 必须采取的控制 |
|---|---|---|
| 配置所有权 | 死锁、请求被磁盘 I/O 长时间阻塞、运行态拿到加密密钥、热更新与手工保存互相覆盖、以前临时生效的 Cookie/令牌开始真正落盘 | copy-on-write；明文运行态与加密落盘分离；副作用在提交后执行；备份配置并做重启测试 |
| CLI 认证 | token 出现在进程参数、Shell 历史或日志；重定向时把认证头带到其他主机 | 优先环境变量/token 文件；日志脱敏；带 token 的 client 禁止跨主机重定向 |
| 调度 CLI/API | 为迁就错误 CLI 而破坏现有 Web/API；脚本依赖旧错误输出；history 过滤行为变化 | 服务端路由与成功响应保持不变，主要修 CLI；新增过滤仅做向后兼容扩展 |
| 异步截图 | CLI 阻塞更久；轮询增加请求量；轮询失败后本地回退导致同一批次执行两次；超时后后台任务仍继续 | 有界轮询和退避；服务端一旦接受任务就禁止自动本地重跑；超时必须打印可恢复的 `job_id` |
| 调度持久化 | 保存期间任务恰好触发；失败回滚重复布置 timer；任务文件已写而历史文件失败，形成半成功 | 明确状态转换；阻止新触发但不承诺取消已运行任务；任务与历史分开保存或建立单一事务文件 |
| 分布式重派 | 原来无限循环的任务开始进入 failed，短期失败数和告警增加；`attempt` 语义改变影响快照/客户端；磁盘慢导致队列吞吐下降 | 先冻结计数语义；保持 JSON 兼容；写失败返回错误并提供指标；上线前清点 pending/claimed 任务 |
| 会话撤销 | 每个请求查询 SQLite 带来延迟；数据库短暂故障导致所有用户掉线；Cookie 格式迁移错误会误伤在线用户 | 仅 DB 用户校验状态/版本，兼容旧 version 0 Cookie，legacy/admin-token 保持原路径；明确 fail-closed |
| 首管理员原子创建 | SQLite 锁竞争返回 busy；Repository interface 与测试 fake 同时变化 | 单条条件 INSERT 或短事务；返回可判别错误；同步更新所有 fake |
| 调度运行状态 | 直接把 `enabled` 改为 false 会丢失用户意图；新增字段可能让严格客户端失败 | 保留 `enabled` 的配置含义，新增只读运行状态字段；JSON 只做加法 |
| 查询缓存 | Redis 旧值无法按新结构解码；由合并后资产重算统计会与首次查询不一致 | 使用版本化 cache key；缓存完整响应元数据；不要从合并后的 `Source` 猜原始统计 |
| Web UX | 不同页面形成多套错误解析；文案声称“已应用”但运行时刷新失败 | 统一前端 helper；以 `resp.ok`、`success`、`applied`、`restart_required` 联合决定状态 |

## 2. 修复期间必须保持的不变量

1. 唯一业务路由前缀继续是 `/api/v1/...`，不恢复旧 `/api/...` shim。
2. 运行态配置保存明文，只有写入磁盘的副本加密通知密钥；不得把加密值重新发布给运行态。
3. 配置写入失败时，运行态和 Manager 均不得发布候选值。
4. 对异步截图，HTTP 202 只代表“任务已接受”，不代表完成。
5. 调度删除/停用只阻止未来触发，不声称能取消已经开始的执行。
6. `max_reassign=N` 的建议定义是：首次分配之外，最多允许 N 次重新分配；初始 `attempt=0`。
7. 队列 API 只有在对应状态已经持久化后才返回 success。
8. 禁用、删除、修改密码后，DB 用户的旧会话应立即失效；admin token 与 legacy 单用户模式不被误伤。
9. 成功响应尽量保持现有服务端结构；修复错误调用方，不反向污染权威接口。
10. 所有外部 URL、截图、通知和 Bridge 路径继续保留现有 SSRF 与 loopback 限制。

## 3. 模块与 seam 调整建议

当前主要问题是复杂状态转换散落在 handler。建议建立以下深 module，让调用者只面对小 interface：

### 3.1 配置提交 module

在 `web.Server` 内先建立单一配置提交 seam，后续可下沉到 `internal/config`：

```go
type ConfigCommitResult struct {
    Previous        *config.Config
    Current         *config.Config
    Persisted       bool
    Applied         bool
    RestartRequired bool
}

func (s *Server) updateConfig(
    mutate func(*config.Config) error,
    apply func(previous, current *config.Config) error,
) (ConfigCommitResult, error)
```

实现必须隐藏“克隆、校验、加密落盘、发布、运行时刷新”的复杂顺序。`mutate` 只能改候选对象，不得做 I/O、日志、reload 或再次读取配置，避免锁重入。所有读取通过 `currentConfig()`；发布后的配置对象不可再原地修改。

过渡阶段可把 `configMutex` 改为 `sync.RWMutex`，采用 copy-on-write：写侧持锁创建候选并调用 `SaveConfig(candidate)`，成功后替换 `s.config`；读侧只取得已发布且永不再修改的指针。完成迁移后，应废弃 handler 对 `Manager.Save()`、`Manager.SetConfig()` 和 `s.config` 字段写入的直接调用。

### 3.2 CLI HTTP client module

在 `cmd/unimap-cli` 内集中为一个 client：

```go
type APIClient struct {
    BaseURL   *url.URL
    Token     string
    HTTP      *http.Client
}

func (c *APIClient) DoJSON(ctx context.Context, method, path string, in, out any) error
```

该 interface 内统一处理认证头、GET/POST、标准与遗留错误 envelope、超时、同主机重定向限制。不要继续保留“名称是 form helper、实现固定 POST”的浅 helper。

### 3.3 任务状态提交 module

Scheduler 和 TaskQueue 的共同原则是：调用者只看到一次状态转换，module 内部负责候选状态、持久化、发布和失败恢复。测试必须通过 Add/Update/Delete/Disable/Claim/SubmitResult 等外部 interface 观察结果，不再靠手工修改 `Attempt` 或内部 map 证明正确。

## 4. 分阶段实施指南

### 阶段 0：冻结行为并补失败测试

**目的**：先让每个问题有一个会失败的回归测试，防止修复过程中改变错误目标。

应新增：

- 配置入口表驱动测试：每个写入口执行后重新构造 `config.Manager` 并从磁盘 Load。
- 并发配置保存与 auth/query/scheduler 请求的 `-race` 测试。
- CLI binary/子命令对认证 `httptest.Server` 的 method/path/header/response 契约测试。
- CLI/GUI 异步截图 running → completed、running → failed、401、404、超时测试。
- Scheduler 注入失败 store，验证 delete/disable 失败后内存、cron/timer 与重载状态一致。
- TaskQueue 连续 N 次离线/租约过期测试，禁止手工设置内部 `Attempt`。
- 快照目录只读或 failing adapter 测试，确认 API 不再假成功。
- 用户禁用/删除/改密后使用旧 Cookie 访问普通受保护接口的测试。
- 两个 goroutine 同时创建不同用户名首用户的测试。
- 首次查询与缓存命中 QueryResponse 等价测试。

**门槛**：新增测试在修复前应稳定失败，且失败原因对应问题，而不是依赖 sleep 的偶现失败。

### 阶段 1：修复 L-01、L-02（配置所有权）

#### 实施步骤

1. 为当前 `configs/config.yaml` 创建不含真实凭据输出的备份流程；测试使用临时目录。
2. 实现 `currentConfig()` 与 `updateConfig()`，先迁移 `handleSaveConfig` 作为样板。
3. 按以下顺序迁移所有直接写入口：通知 → Cookie/代理 → CDP → 管理令牌 → 旧版密码 → 截图模式。
4. 删除这些入口中的 `configManager.Save()`；统一传候选对象给 `SaveConfig(candidate)`。
5. 把所有无锁读迁移到 `currentConfig()`；之后禁止对已发布对象原地写字段。
6. 运行时 applier 在提交并解锁后执行，并返回结果：
   - 引擎：重注册 adapter 与 browser fallback。
   - 通知：reload registry；Feishu app 的 pinned channel 也必须根据新配置重建，否则仍是假热更新。
   - 截图：更新 router、app、manager 的 mode/CDP/cookie/proxy。
   - system/Web server 参数：明确标为 restart required，不声称热更新。
7. `updateConfig` 返回真实的 `persisted/applied/restart_required`，前端以后据此展示。

#### 特别风险

- 修复后 Cookie、自动 token、CDP 地址会第一次真正持久化。必须确认配置文件权限仍为 `0600`，日志不输出凭据。
- 不要让 `s.config` 直接指向 Manager 内部可变对象；这会修复双副本却重新引入任意调用方修改和竞态。
- 不要在持有配置锁时调用 reload。adapter/notify reload 可能获取其他锁或执行较慢，容易形成死锁和长尾请求。
- `HotUpdateManager` 当前没有生产调用方，但它仍可直接 `SetConfig`。要么同步改走提交 seam，要么明确标记为未启用并禁止未来绕过。

#### 验收

- 每个写入口：当前进程读取正确、配置文件正确、重启后正确。
- 注入写失败：接口返回 5xx，Manager、Server 和运行时 adapter 均保持旧状态。
- `go test -race ./internal/config ./web` 无竞态。

#### 回退

代码回退前先停服务并恢复配置备份，避免新版本已持久化的 token/Cookie 与旧版本运行态不一致。不要只回退二进制。

### 阶段 2：修复 A-01、A-02、A-03（客户端契约）

#### 2.1 CLI 认证

认证来源建议优先级：`--admin-token-file` → `UNIMAP_ADMIN_TOKEN` → 显式 `--admin-token`。命令行明文 flag 仅为兼容入口，并在帮助中提示可能进入 Shell 历史和进程列表。不要把 token 写入错误消息、debug URL 或输出文件。

client 在同一主机请求时设置 `Authorization: Bearer ...` 或 `X-Admin-Token`；若发生跨主机重定向必须拒绝。空 token 仍允许连接显式关闭认证的服务，保持开发环境兼容。

#### 2.2 Scheduler CLI

服务端现有路由保持不动，CLI 改为：

| 子命令 | Method | Path | 解码 |
|---|---|---|---|
| list | GET | `/api/v1/scheduler/tasks` | `[]ScheduledTask` |
| create | POST | `/api/v1/scheduler/tasks/create` | `{id,message}` |
| run | POST | `/api/v1/scheduler/tasks/run` | `{message}` |
| enable/disable/delete | POST | 对应动作路径 | `{message}` |
| history | GET | `/api/v1/scheduler/history?...` | `[]ExecutionRecord` |

每个子命令创建自己的 `FlagSet`，先定义 flag 再 parse；不要在根 `FlagSet` 已遇到非 flag 参数后继续追加 flag。`history --task-id` 若保留，服务端可新增可选 `task_id` 过滤，这是向后兼容扩展；否则 CLI 应删除未生效选项。

#### 2.3 异步截图

抽出 start/poll workflow，CLI 与 GUI 共用响应 DTO：

1. POST 得到 202 `{job_id,total,status}`。
2. 若已是 completed，仍执行一次 GET 获取完整结果。
3. 以 1–2 秒间隔轮询，连续网络错误使用有上限退避。
4. completed 返回结果；failed 返回服务端 error；`persistence_error` 作为完成但降级的明确警告。
5. 客户端超时时输出 `job_id` 和恢复查询命令，返回非零退出码，但不得请求本地重复执行。
6. GUI 只有在 POST 未被服务端接受时才允许本地 fallback；一旦拿到 `job_id`，后续错误只能提示重试轮询。

#### 兼容风险与控制

- CLI 的正确输出会不同于当前空结果输出。JSON 输出可在一个版本内同时保留 `job_id` 与 deprecated `batch_id` 别名，随后移除别名。
- 默认轮询会让命令运行时间变长，这是从“假完成”恢复为真实语义，不应通过立即返回掩盖；可提供 `--no-wait` 明确选择只提交任务。
- 服务端内存任务终态保留约一小时，CLI 恢复查询应在帮助中说明；数据库恢复存在，但只恢复最近记录且运行中任务在服务重启后标记失败。

#### 验收

- 默认认证开启时，四类 API 子命令使用 token 成功；错误 token 为 401 且无泄露。
- scheduler 所有子命令 method/path/body/response 契约测试通过。
- 截图 CLI/GUI 不再把 202 打印成 completed；中途断网不产生第二批截图。

### 阶段 3：修复 S-01、S-02（调度状态）

#### 删除/停用事务

推荐状态转换：

1. 持锁读取并深拷贝 previous。
2. 设置内部 `transitioning`/generation gate，阻止尚未开始的新执行；已运行任务继续。
3. 构造候选任务集合并持久化。
4. 持久化成功后移除 cron/timer 并发布内存状态。
5. 持久化失败则移除 gate，恢复 previous，不重新创建重复 timer。

若不引入 gate 而选择“先保存再撤销 cron”，磁盘写入窗口内任务可能被触发；若选择“先撤销再保存并回滚”，一次性 timer 可能因恢复时已过期而立即运行。两者都必须用可控时钟测试，不能依赖真实 sleep。

#### Store 风险

当前 `Store.Save` 顺序写 task JSON 再写 history JSON。task 成功、history 失败会被报告为整体失败，但 task 实际已经提交。建议把任务配置与执行历史拆成 `SaveTasks`、`SaveHistory` 两条内部 seam，任务变更不重复写历史；长期可迁移为单 SQLite 事务。不要在本次修复中悄悄改变 JSON 文件路径或格式。

#### 运行状态

不要在 Load 调度失败时直接持久化 `enabled=false`，否则用户意图丢失。保留 `enabled` 表示期望状态，向 API DTO 添加只读字段：

```json
{
  "enabled": true,
  "runtime_status": "schedule_error",
  "schedule_error": "..."
}
```

正常状态可为 `scheduled`、`disabled`、`running`、`schedule_error`。页面用“启用但异常”展示，并提供修复配置/重新启用入口。

#### 错误映射

引入 `ErrTaskNotFound`、`ErrInvalidTask`、`ErrPersistence`、`ErrSchedule`，handler 分别返回 404、400、500/503、500；不要再按错误字符串判断。

#### 验收

- failing store 下 delete/disable 返回 5xx，任务仍按原计划运行且重启后一致。
- 成功删除后未来不再触发；已开始执行的记录仍能正常完成。
- 非法持久化任务启动后可见 `schedule_error`，而不是假装 scheduled。

### 阶段 4：修复 D-01、D-02（分布式队列）

#### 固定重派计数

在不改变 JSON 字段的前提下，统一三种失败路径：retryable result、node offline、lease expiry。

```text
if rec.Attempt < rec.MaxReassign:
    rec.Attempt++
    rec.Status = pending
else:
    rec.Status = failed
```

这使 `MaxReassign=1` 表示首次分配失败后最多再分配一次。保留现有快照中的 `attempt` 数值，不做减一/加一迁移。上线时，已经具有高 attempt 的 claimed 任务可能在下一次回收立即失败，这是正确但需要运维可见的行为变化。

注意：当前 `TaskEnvelope.MaxReassign` 的 0 同时承担“未指定”含义，不能通过请求明确表达 0。这个问题不应与本次计数修复混在一起；如需支持显式 0，后续把字段改为 `*int` 并单独做契约变更。

#### 持久化确认

把 `saveLocked()` 改为返回 error，并把状态转换写成候选快照：

- Enqueue/Claim/SubmitResult/Delete：写成功后才向 API 返回 success。
- 写失败时恢复内存任务与 pending 顺序。
- ReleaseNodeTasks/recycleExpiredLocked：状态改变后必须保存；后台失败要保留旧状态或进入可重试的 degraded 状态，不能静默吞掉。
- 增加 `task_queue_persistence_failures_total` 和 readiness/degraded 信号。

生产仍可使用当前 JSON 快照以降低迁移风险；先把文件写失败变为可观察错误，再评估 SQLite/WAL。不要在同一提交中同时更换存储后端和修改 attempt 语义。

#### 验收

- `MaxReassign=0/1/3` 在三种失败路径上的分配次数一致。
- 快照写失败时 API 返回 5xx，重启后不会出现 API 曾确认但磁盘不存在的状态。
- 旧快照可直接加载；新快照也能被本修复版本重载。
- 慢磁盘压测下 claim 延迟有指标且不会重复发放任务。

### 阶段 5：修复 U-01、U-02（用户与会话）

#### 会话失效

仅查询用户状态可以解决禁用/删除问题，但不能让“修改密码”撤销旧会话，也会让禁用后重新启用的旧 Cookie 再次有效。完整方案应给 DB 用户增加会话版本：

1. `users` 表新增 `session_version INTEGER NOT NULL DEFAULT 0`。`InitSchema` 必须对既有数据库执行可重复的列检测与 `ALTER TABLE`；不能只修改 `CREATE TABLE IF NOT EXISTS`。
2. 新 Cookie payload 使用 `userID:sessionVersion:adminToken`。现有 `userID:adminToken` Cookie 按 version 0 解析，因此迁移后不必强制所有在线用户立刻重新登录。
3. 登录时把数据库中的当前 version 写入 Cookie。
4. 禁用用户或修改密码时，在同一数据库操作中递增 `session_version`；重新启用不能把版本减回去。删除用户由“不存在”校验直接失效。
5. 中间件解析 Cookie 一次后执行：

- `userID > 0`：查询 user repository，要求用户存在且 `status=active`；否则清 Cookie 并返回 401。
- 同时要求 Cookie version 与数据库 `session_version` 相等；不相等时清 Cookie 并返回 401。
- `userID == 0`：保持 legacy 单用户模式。
- admin token 请求继续使用 synthetic admin，不查询用户表。

禁用、删除和修改密码完成后不需要枚举内存 session；下一个请求的状态/版本校验即可立即拒绝。现有单 session revocation store 继续用于 logout。中间件应把已加载的 active user 放进 request context，后续 `getCurrentUser` 不要再次查询数据库。

数据库错误应 fail-closed 并返回 503，而不是把用户误判为不存在或继续放行。需要记录错误指标，但不能记录 Cookie、密码 hash 或 token。若 SQLite 查询成为可测量瓶颈，再增加极短 TTL 的 active-user cache；禁用操作必须主动失效 cache，不能牺牲即时停权语义。

#### 首管理员创建

在 `UserRepository` 增加语义明确的方法，例如：

```go
CreateBootstrapAdmin(username, passwordHash string) (*User, error)
```

SQLite adapter 使用单条条件 INSERT 或 `BEGIN IMMEDIATE` 短事务完成“表为空 + 插入 admin”。若已存在用户，返回 `ErrBootstrapClosed`；handler 随后要求已认证 admin，不得基于旧 count 继续公开创建。同步更新测试 fake 和 repository contract tests。

#### 兼容风险

- 部署后，已被删除/禁用但仍在线的用户会立即收到 401，这是预期变化，应在发布说明中提示。
- DB 不可用时普通 session 用户会收到 503，而 admin-token 运维入口仍应可用，以便恢复数据库。
- `session_version` 的 SQLite 迁移失败时服务应拒绝进入多用户就绪状态，不能带着半迁移 schema 继续运行。
- 不要通过全局轮换 admin token 实现按用户失效，它会误伤所有 API 客户端和 legacy session。

#### 验收

- 禁用、删除、改密后旧 Cookie 的下一请求分别被拒绝。
- DB 临时错误返回 503；恢复后无需重启即可登录。
- 100 个并发首次注册最多一个成功获得 admin，其余不能匿名创建 readonly/admin。

### 阶段 6：修复 Q-01、X-01、X-02

#### 查询缓存

不要从合并后的 `UnifiedAsset.Source` 重算首次查询统计：同一资产可能包含 `fofa,hunter`，而首次查询的统计发生在合并前，数值不等价。

正确方向是把服务层 QueryResponse cache 与编排器 Asset cache 拆成两个 module：

- `AssetCache`：继续供 orchestrator 缓存 `[]UnifiedAsset`。
- `QueryResponseCache`：缓存 assets、total、engineStats、errors 等响应元数据。

底层 Memory/Redis adapter 可以共享序列化实现，但 interface 不应混用。Redis key 使用新版本前缀，例如 `query:v2:`；让旧 key 自然过期，不做在线批量迁移。缓存命中响应必须与首次响应在业务字段上等价，只允许 `cached`、耗时、request ID 等观测字段不同。

#### Web 错误与状态

1. 把 `extractErrorMessage` 和 JSON/HTTP 检查收敛到 `web/static/js/main.js` 的单一 helper。
2. account、monitor、settings 页面全部调用该 helper，不直接拼 `data.error`。
3. 轮询连续失败达到阈值后停止，显示最后错误、job ID 和重试按钮；10 分钟 timeout 也必须显示明确终态。
4. 设置页仅在 `resp.ok && result.success` 时更新状态点；根据 `applied/restart_required` 显示“已立即生效”“已保存，重启后生效”或“已保存但运行时应用失败”。
5. 前端新增文案保持中文一致，避免每页各自解释错误 envelope。

#### 验收

- memory/Redis 两种后端的首次与缓存命中统计一致；旧 Redis key 不导致 decode error 风暴。
- account 所有失败路径不再出现 `[object Object]`。
- monitor 的 401、404、500、断网和 timeout 都有可见结果，按钮状态可恢复。
- 配置保存失败时引擎状态点不改变；restart-required 不再显示成即时生效。

## 5. 每阶段通用验证与提交纪律

每个阶段至少执行：

```text
gofmt / goimports（受影响 Go 文件）
go test -race ./受影响包
go vet ./...
```

阶段 2 涉及 GUI 时追加：

```text
go test -tags gui ./cmd/unimap-gui
```

全部阶段完成后：

```text
go test -race ./...
go build ./...
```

并执行三类非单元验收：

1. **真实重启**：保存配置/创建与停用任务/提交分布式任务 → 正常停止 → 重启 → 核对状态。
2. **故障注入**：只读目录、磁盘写失败、SQLite busy/不可用、Redis 中存在旧 key、截图轮询断网。
3. **并发**：配置保存与请求并发、首次注册并发、节点 claim/result/recycle 并发。

建议每个阶段独立提交，提交之间保持可运行；不要把配置所有权、队列存储后端迁移和前端改版揉成一个无法回退的大提交。

## 6. 发布与回滚清单

### 发布前

- 备份 `config.yaml`、scheduler task/history JSON、`distributed_tasks.json`、用户 SQLite 和截图 batch SQLite。
- 记录当前 admin token 的来源，但不要写入发布日志。
- 清点 enabled/schedule_error、pending/claimed 任务数量。
- 确认 CLI 使用环境变量或 token 文件，不在 CI 命令行展开 secret。
- 为新增 401/503、队列持久化失败、schedule_error 和截图轮询失败配置监控。

### 灰度观察

- 配置保存 5xx、重启后配置一致性。
- CLI 401/405 比例是否归零。
- screenshot job 的 running 时长、poll 404、重复 batch ID。
- scheduler 的 schedule_error、任务重复执行。
- distributed failed 数量是否因重派上限生效而短期上升。
- session user lookup 延迟与 SQLite busy。
- Redis query:v2 命中率和 decode error。

### 回滚原则

- 配置阶段回滚必须同时恢复配置备份；仅回滚二进制可能使 token/密码/Cookie 状态不一致。
- 分布式阶段保持快照字段向后兼容后，可回滚代码；若未来新增字段，不要删除旧字段或原地重写历史快照。
- 缓存阶段可直接回滚并让 `query:v2` key 自然过期；不要批量删除无关 Redis key。
- 会话阶段代码回滚不会恢复已删除用户；数据库回滚属于独立且高风险操作，默认不执行。
- 已启动的调度或截图任务不能靠代码回滚撤销，应通过现有任务状态与运维接口处理。

## 7. 完成定义

只有同时满足以下条件，才能把问题报告中的对应项标记为已修复：

- 原问题有稳定的失败测试，修复后转绿。
- 正常路径、持久化失败、重启、并发至少各有一项验证。
- API/CLI/GUI/Web 的同一业务语义一致。
- 错误不会被伪装成 2xx、404 或“已完成”。
- 不泄露 token、Cookie、密码 hash、Webhook secret 或 Bridge token。
- 权威 API、Runbook、README 与相关 ADR 已同步。
- 全量 `go test -race ./...`、`go vet ./...` 和构建通过。

## 8. 最终验证记录

2026-07-17 的最终本地验证结果：

```text
PASS  go test -race ./...
PASS  go vet ./...
PASS  go build ./...
PASS  go build -tags gui ./cmd/unimap-gui
PASS  go test -tags gui ./cmd/unimap-gui
PASS  go test -run '^$' -tags live_bridge_e2e ./web
PASS  govulncheck ./...（未发现当前代码调用的已知漏洞）
```

外部引擎真实账号、远程浏览器扩展和生产 Redis 不在本次本地验证权限范围内；其上线验收继续按第 6 节灰度观察项执行。
