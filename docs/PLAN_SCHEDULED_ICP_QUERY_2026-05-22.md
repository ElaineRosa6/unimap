# 定时 ICP 备案查询接入方案（方案 B：新增独立 Runner）

> **创建日期：** 2026-05-22
> **分支：** `master`
> **作者：** UniMap 维护组
> **状态：** 📝 待实施
> **关联记忆：** `memory/project_icp_settings_reorg_2026-05-22.md`

0.方案审查情况：
审查结论                                                                                                              
  整体方向合理：ICP 不并入 EngineOrchestrator，而是新增独立 TaskICPQuery Runner，这和现有 ICP 查询路径、scheduler 分层是
  匹配的。但这份计划不建议原样实施，至少要先修正下面几个点。                                                            
                                                                                                                        
  发现                                                                                                                  
                                                                                                                        
  - P1: 计划让 Runner 直接调用 adapter.ICPSearch，但这个函数不接收 context.Context，内部固定 SetTimeout(30 *            
    time.Second)。Scheduler 已在 internal/scheduler/scheduler.go:804 给每次执行创建超时 context，但传到 handler.Execute 
    后，ICPSearch 不会使用它；对应计划里的 TimeoutSec、TestICPQueryRunner_ContextCancel、多关键词任务取消语义都会打折。 
    建议改成新增 ICPSearchWithContext，或复用已有的 internal/adapter/icp.go:177 / internal/adapter/icp.go:268。         
  - P1: 部分失败语义没有定义清楚。计划要求 history 记录命中数与错误，并给了 “1/2 queries succeeded” 的失败摘要，但现有  
    scheduler 只有成功时才写 record.Result，失败时只写 record.Error，见 internal/scheduler/scheduler.go:818 和 internal/
    scheduler/scheduler.go:824。如果 Runner 对部分失败返回 error，命中摘要不会进 Result；如果返回 nil error，任务会显示 
    success 且不会重试/告警。需要在计划中明确：部分失败到底算成功摘要、失败任务，还是要扩展 ExecutionRecord 支持        
    partial/result-on-error。                                                                                           
  - P2: cfgProvider 只返回 baseURL/apiKey/enabled，但 ICP 配置还有 timeout/default_type，见 internal/config/            
    config.go:190。Web handler 已读取 defaultType，见 web/icp_handlers.go:61，配置保存也支持 timeout/default_type，见   
    web/config_handlers.go:178。计划应改成返回完整 ICP 配置快照，否则定时任务行为会和 Web 查询/设置页不一致。           
  - P2: page_size 描述不准确。计划写“与现有 /api/icp/query 一致，最大约 40”，但现有 handler 上限是 100，见 web/         
    icp_handlers.go:98。adapter.ICPSearch 自身也没有上限校验，见 internal/adapter/icp.go:419。需要统一为 40 或 100，并在
    Runner 中显式校验。                                                                                                 
  - P2: “内置任务模板”目前只是 scheduler 包内部能力。DefaultTemplates() / CreateTaskFromTemplate() 存在于 internal/     
    scheduler/scheduler.go:96，但 Web 层没有模板列表/创建入口，/scheduler 帮助表也是静态内容。如果 G-5 想让用户可见，还 
    要补 API/UI；如果只是内部模板，应在计划里说明。                                                                     
                                                                                                                        
  合理部分                                                                                                              
  背景判断基本准确：当前 scheduler 无 ICP 类型，Web ICP 走独立 handler，CLI/GGUI 没注册 ICP adapter；/scheduler 下拉也确
  实会通过 AllTaskTypes()/TaskTypeLabel() 自动出现新类型。方案 B 的边界是对的，主要问题在执行细节和验收标准。建议先修订 
  上述 P1/P2，再进入实现。 


---

## 1. 背景

当前 `internal/scheduler/` 提供 20 种定时任务类型（`scheduler.go:28-53` `TaskQuery` … `TaskCacheWarmup`），可通过 cron 表达式定时触发资产查询、截图、篡改检测等流程。但 **ICP 备案查询 (`/api/icp/query`)** 走的是独立路径：

- `web/icp_handlers.go:47 handleICPQuery` 直接调用 `adapter.ICPSearch(baseURL, apiKey, req)`，未经过 `EngineOrchestrator`。
- 三个入口 `cmd/unimap-web/main.go:101-175`、`cmd/unimap-cli/main.go:193-226`、`cmd/unimap-gui/main.go:681+` 均**未**调用 `RegisterAdapter(NewICPAdapter(...))`。
- `scheduler.go:28-53` 的 `TaskType` 常量表中无 ICP 相关项，20 个 Runner 中没有 ICP Runner。

**结论：** 当前定时任务系统**无法**调度 ICP 备案查询。本方案在不污染统一查询链路 (`TaskQuery` 的资产合并/导出语义) 的前提下，新增独立任务类型 **ST-21 `TaskICPQuery`** 与对应 Runner，使 ICP 备案查询可周期化执行（如每日抓取重点域名最新备案变更，配合告警系统监控备案吊销/变更）。

---

## 2. 目标与非目标

### 2.1 目标 (In Scope)

| 编号 | 目标 |
|------|------|
| G-1 | 新增 `TaskICPQuery` (ST-21) 任务类型，复用 `Scheduler` 现有 cron/重试/历史/持久化能力 |
| G-2 | `ICPQueryRunner` 直接调用 `adapter.ICPSearch`，自洽于现有 ICP 配置 (`config.ICP.*`) |
| G-3 | payload 支持 8 种 ICP 查询类型 (`web/app/mapp/kapp/bweb/bapp/bmapp/bkapp`)、分页、多关键词批量 |
| G-4 | 任务执行结果写入定时任务历史 (`./data/scheduler_history.json`)，记录命中数与错误 |
| G-5 | 提供至少 2 个内置任务模板（每日企业备案巡检、每周域名备案变更扫描） |
| G-6 | 单元测试覆盖：payload 解析、参数校验、配置缺失分支、HTTP 失败分支，整体覆盖率 ≥ 80% |
| G-7 | 前端定时任务页 (`/scheduler`) 自动展示新任务类型（无需额外改动，依赖 `AllTaskTypes()` + `TaskTypeLabel()`） |

### 2.2 非目标 (Out of Scope)

| 编号 | 非目标 | 原因 |
|------|--------|------|
| N-1 | 将 `ICPAdapter` 注册进 `EngineOrchestrator`，使其参与 `TaskQuery` | 与方案 A 等价；ICP 返回字段与 5 大资产引擎差异大，会污染 `UnifiedAsset` 归并/导出列 |
| N-2 | 备案差异比对 / 历史快照 / 变更告警 | 二期工作，本方案先打通"能定时跑"的基础能力 |
| N-3 | ICP 结果导出 Excel / CSV | 留给后续 `TaskExport` 复用或新增 `TaskICPExport` |
| N-4 | 分布式 ICP 任务（`TaskDistributedSubmit` 转发） | 二期，需先确认 ICP sidecar 的多节点鉴权策略 |

---

## 3. 设计方案

### 3.1 数据流

```
Cron 触发
   ↓
Scheduler.executeTask(task, handler=ICPQueryRunner, ...)
   ↓
ICPQueryRunner.Execute(ctx, payload)
   ↓ 读取 payload：queries[], type, page, page_size
   ↓ 读取 server.config.ICP.{BaseURL, APIKey, Enabled}
   ↓
adapter.ICPSearch(baseURL, apiKey, ICPSearchRequest{...})  循环 queries
   ↓
聚合 (命中总数, 各 query 命中, 错误列表)
   ↓
返回 result string → 写入 ScheduledTask.LastResult & History
```

### 3.2 任务类型常量

```go
// scheduler.go
const (
    // ... existing 20 task types
    TaskICPQuery TaskType = "icp_query" // ST-21: ICP 备案查询
)

func AllTaskTypes() []TaskType {
    return []TaskType{
        // ... existing
        TaskICPQuery,
    }
}

func TaskTypeLabel(t TaskType) string {
    labels := map[TaskType]string{
        // ... existing
        TaskICPQuery: "ICP 备案查询",
    }
    // ...
}
```

### 3.3 Runner 接口

```go
// executor.go
type ICPQueryRunner struct {
    cfgProvider func() (baseURL, apiKey string, enabled bool)
}

func NewICPQueryRunner(p func() (string, string, bool)) *ICPQueryRunner {
    return &ICPQueryRunner{cfgProvider: p}
}

func (r *ICPQueryRunner) Type() TaskType { return TaskICPQuery }

func (r *ICPQueryRunner) Execute(ctx context.Context, payload map[string]interface{}) (string, error) {
    // 1. 读配置
    // 2. 解析 payload
    // 3. 校验 query type
    // 4. 循环 queries 调用 adapter.ICPSearch
    // 5. 聚合结果
}
```

**为什么用 `cfgProvider` 而不是直接传 `*config.Config`：**
- ICP 配置可热更新 (`internal/config/config.go` 支持 reload)；闭包每次读最新值，避免 stale 配置。
- 便于测试时注入 mock provider。

### 3.4 Payload 契约

| 字段 | 类型 | 必填 | 默认 | 说明 |
|------|------|------|------|------|
| `queries` | `[]string` | 否* | `[]` | 关键词列表（公司名、域名、统一社会信用代码等） |
| `query` | `string` | 否* | `""` | 单关键词；兼容 `queries` 不传的情况 |
| `type` | `string` | 否 | `"web"` | ICP 查询类型：`web/app/mapp/kapp/bweb/bapp/bmapp/bkapp` |
| `page` | `int` | 否 | `1` | 起始页 |
| `page_size` | `int` | 否 | `20` | 每页条数（与现有 `/api/icp/query` 一致，最大约 40） |
| `fail_fast` | `bool` | 否 | `false` | true 时遇到任意 query 失败立即返回；false 时收集所有错误继续 |

\* `queries` 与 `query` 至少一个非空，否则返回 `missing 'queries' or 'query'`。

#### 示例 payload

```json
{
  "queries": ["示例科技有限公司", "example.com"],
  "type": "web",
  "page": 1,
  "page_size": 40,
  "fail_fast": false
}
```

### 3.5 结果字符串格式

```
icp [type=web] 2/2 queries succeeded, total 87 records (page=1 size=40)
```

失败场景：
```
icp [type=web] 1/2 queries succeeded, total 23 records, 1 error(s): "example.com": ICP API error: rate limit
```

### 3.6 内置模板

```go
// scheduler.go: DefaultTemplates()
{
    ID:          "tmpl_daily_icp_company_watch",
    Name:        "每日企业备案巡检",
    Description: "每天早上 9 点查询关注企业的 ICP 备案状态",
    Type:        TaskICPQuery,
    CronExpr:    "0 0 9 * * *",
    Payload: map[string]interface{}{
        "queries":   []string{}, // 用户编辑时填入企业名列表
        "type":      "web",
        "page":      1,
        "page_size": 40,
    },
    TimeoutSec: 600,
    MaxRetries: 1,
    Tags:       []string{"icp", "daily", "compliance"},
},
{
    ID:          "tmpl_weekly_icp_domain_scan",
    Name:        "每周域名备案变更扫描",
    Description: "每周一凌晨 3 点扫描目标域名 ICP 备案变更",
    Type:        TaskICPQuery,
    CronExpr:    "0 0 3 * * 1",
    Payload: map[string]interface{}{
        "queries":   []string{},
        "type":      "web",
        "page":      1,
        "page_size": 40,
    },
    TimeoutSec: 1800,
    MaxRetries: 1,
    Tags:       []string{"icp", "weekly", "monitoring"},
},
```

---

## 4. 文件级改动清单

| # | 文件 | 改动类型 | 关键位置 | 预估行数 |
|---|------|---------|---------|---------|
| F-1 | `internal/scheduler/scheduler.go` | 修改 | `TaskType` 常量块新增 `TaskICPQuery`；`AllTaskTypes()` 追加；`TaskTypeLabel()` 追加；`DefaultTemplates()` 追加 2 个模板 | +20 |
| F-2 | `internal/scheduler/executor.go` | 修改 | 文件末尾新增 `ICPQueryRunner` 类型 + `NewICPQueryRunner` + `Type()` + `Execute()` | +95 |
| F-3 | `internal/scheduler/executor_test.go` | 修改 | 新增 `TestICPQueryRunner_*` 系列（见 §5） | +180 |
| F-4 | `web/server.go` | 修改 | `RegisterHandler` 块尾新增一行 `sched.RegisterHandler(scheduler.NewICPQueryRunner(srv.icpConfigProvider))` | +1 |
| F-5 | `web/server.go` | 修改 | 新增私有方法 `(*Server).icpConfigProvider() (string, string, bool)`，包装 `config.ICP.{BaseURL,APIKey,Enabled}` 的线程安全读取 | +12 |
| F-6 | `docs/API.md` | 修改 | 在"定时任务任务类型"小节追加 `icp_query` 描述 + payload 字段表 | +30 |
| F-7 | `memory/MEMORY.md` | 修改 | 追加索引行 `- [ICP 定时任务接入 2026-05-22](project_icp_scheduled_task_2026-05-22.md) — 新增 TaskICPQuery 任务类型...` | +1 |
| F-8 | `memory/project_icp_scheduled_task_2026-05-22.md` | 新建 | 落地报告（实施完成后写入） | ~60 |

**不需要改动**的文件：
- `cmd/unimap-cli/main.go`、`cmd/unimap-gui/main.go`：CLI/GUI 当前未挂载 scheduler，本期不引入。
- `web/scheduler_handlers.go`：已通过 `AllTaskTypes()` 动态发现新任务类型。
- 前端模板 `web/templates/scheduler.html`：已通过 `taskTypeLabels` map 动态渲染下拉。
- `internal/config/config.go`：`ICP` 配置块已存在，复用即可。

---

## 5. 测试计划

| 测试 | 验证点 | 类型 |
|------|--------|------|
| `TestICPQueryRunner_Type` | 返回 `TaskICPQuery` | 单元 |
| `TestICPQueryRunner_MissingQueries` | `queries`/`query` 都为空 → 返回错误 `missing 'queries' or 'query'` | 单元 |
| `TestICPQueryRunner_DisabledByConfig` | provider 返回 `enabled=false` → 返回错误 `ICP query is disabled` | 单元 |
| `TestICPQueryRunner_MissingBaseURL` | `baseURL=""` → 返回错误 `ICP base_url not configured` | 单元 |
| `TestICPQueryRunner_InvalidType` | `type="zzz"` → 返回错误 `invalid ICP query type` | 单元 |
| `TestICPQueryRunner_SingleQuerySuccess` | 启动 `httptest.Server` mock ICP sidecar，返回 1 个 query 的成功路径 | 集成 |
| `TestICPQueryRunner_MultiQueryPartialFailure` | 3 个 queries，第 2 个 sidecar 返回 500 → `fail_fast=false` 时收集错误并继续 | 集成 |
| `TestICPQueryRunner_FailFast` | 3 个 queries，第 1 个失败 → 立即返回，不调用后续 | 集成 |
| `TestICPQueryRunner_ContextCancel` | 在 query 间 cancel ctx → 立即返回 `context canceled` | 单元 |
| `TestICPQueryRunner_PayloadDefaults` | 仅传 `queries`，验证 `type/page/page_size` 走默认值 | 单元 |
| `TestAllTaskTypes_ContainsICP` | `slices.Contains(AllTaskTypes(), TaskICPQuery)` | 单元 |
| `TestTaskTypeLabel_ICP` | `TaskTypeLabel(TaskICPQuery) == "ICP 备案查询"` | 单元 |
| `TestDefaultTemplates_ContainsICPTemplates` | 至少 2 个 `Type==TaskICPQuery` 的模板 | 单元 |

**性能/race：**
- `go test -race ./internal/scheduler/...` 必须通过。
- 集成测试使用 `httptest.Server` 启动 mock sidecar，避免真实网络依赖。

**覆盖率目标：** `internal/scheduler/` 包整体不低于现有水平，`ICPQueryRunner` 函数覆盖率 ≥ 90%。

---

## 6. 风险与缓解

| 风险 | 等级 | 缓解措施 |
|------|------|---------|
| R-1：ICP sidecar 限流（备案查询接口本身易被限流） | 中 | Runner 在 query 间插入可配置 sleep（payload `interval_ms`，默认 0），后续可加；本期暂不引入，靠 `MaxRetries` + 模板默认 page_size=40 |
| R-2：ICP 返回结果体过大撑爆 history JSON | 低 | 历史只记录 result 字符串摘要，不持久化原始结果；Runner 内部 result 不超过 ~500 字符 |
| R-3：配置热更新时 provider 闭包读到中间状态 | 低 | `Server.icpConfigProvider` 持 `configMutex`，与 `handleICPQuery` 一致 |
| R-4：用户在 payload 里塞 1000 个 queries 导致超时 | 中 | Runner 起始处校验 `len(queries) <= 100`，超出返回错误；可通过模板 `TimeoutSec` 进一步限制 |
| R-5：CLI/GUI 不挂载 scheduler，用户预期 CLI 也能跑 | 低 | 文档明示当前只有 Web 模式支持定时任务（与现状一致）；CLI 接入是单独 backlog |

---

## 7. 实施步骤与里程碑

| 阶段 | 内容 | 验收 | 工作量 |
|------|------|------|--------|
| Phase 1 | F-1 + F-2：常量 + Runner 骨架 | `go build ./...` 通过 | 0.5 天 |
| Phase 2 | F-3：单元 + 集成测试 | `go test -race ./internal/scheduler/...` 通过，覆盖率达标 | 1 天 |
| Phase 3 | F-4 + F-5：Web server 注册 | 启动 web 后访问 `/scheduler`，下拉框看到 "ICP 备案查询" | 0.5 天 |
| Phase 4 | 手工 e2e：创建定时任务（cron `*/2 * * * * *`），观察 2 次执行 | 历史记录正确，`LastResult` 含命中数 | 0.5 天 |
| Phase 5 | F-6：API.md 文档；F-7/F-8：memory 索引与落地报告 | 文档评审通过 | 0.5 天 |
| **合计** | | | **~3 天** |

---

## 8. 验证清单（PR 前必过）

- [ ] `go build ./...`
- [ ] `go test -race ./...`
- [ ] `go vet ./...`
- [ ] `gofmt -l .` 输出为空
- [ ] `internal/scheduler` 覆盖率不低于实施前
- [ ] 启动 web，`/scheduler` 页面下拉看到 "ICP 备案查询"
- [ ] 手工创建一个 ICP 定时任务并触发一次，history 中可见命中数
- [ ] 关闭 `config.ICP.Enabled` 后任务执行返回明确错误
- [ ] `memory/MEMORY.md` 已追加索引行

---

## 9. 后续工作（二期 backlog）

1. **ICP 结果持久化**：将每次任务结果（命中列表）写入 SQLite，支持时间序列比对。
2. **备案变更告警**：检测 `licence` / `unitName` / `updateRecord` 字段变化，触发 `alerting` Webhook。
3. **批量导入查询源**：从 CSV 文件读取关键词列表（参考 `TaskURLImport` 模式）。
4. **CLI 定时任务支持**：将 `Scheduler` 接入 `cmd/unimap-cli`，提供 `unimap-cli scheduler list/run` 子命令。
5. **分布式 ICP 任务转发**：通过 `TaskDistributedSubmit` 把 ICP 查询分发到工作节点，缓解 sidecar 限流。

---

## 10. 参考

- 现有 Runner 模板：`internal/scheduler/executor.go:75-115` (`QueryRunner`)
- ICP 适配层：`internal/adapter/icp.go:412-456` (`ICPSearch`)
- ICP HTTP Handler：`web/icp_handlers.go:47 handleICPQuery`
- Scheduler 注册入口：`web/server.go:323-349`
- 任务类型注册流：`internal/scheduler/scheduler.go:28-94`
- ICP 配置：`internal/config/config.go:187-194`
- ICP 接入历史：`memory/project_icp_settings_reorg_2026-05-22.md`


