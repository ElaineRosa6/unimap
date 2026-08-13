# UniMap 系统可用性审计与优化交付报告

**审计日期：** 2026-08-12  
**代码基线：** `d295776`（2026-08-12）  
**交付性质：** 现状审计、问题分级、优化计划与验收门禁  
**状态：** 报告已交付；产品代码与运行配置未在本次交付中修改

## 1. 执行摘要

当前系统具备 Web、CLI、GUI 三类入口和较宽的功能面，但“能力存在”尚未稳定转化为“用户容易启动、容易理解、失败可诊断、发布可复现”。本次核对确认了四类主要障碍：

1. **发布安全边界不清晰（P0）**：工作区存在邮件、Cookie/密码命名文件和个人工作目录；示例配置还描述了空密码哈希回落到默认 `admin` 密码。即使这些文件当前未被 Git 跟踪，也会增加误打包、误共享和弱口令部署风险。
2. **默认测试不离线且反馈过慢（P1，部分已修复）**：基线 `go test ./...` 在 124 秒以上仍未结束，且当时 ZoomEye 空 Key 场景进入网络重试。本轮已在 `internal/adapter/zoomeye.go` 的 `Search` 前置空 Key 快速失败，并以零 HTTP 请求测试验证；全量测试的完成性问题仍保留。
3. **启动契约分裂（P1）**：本地 README 要求复制 `config.yaml.example` 后直接运行，基础 Compose 挂载 `config.docker.yaml`，生产 Compose 又通过 entrypoint 从 `config.prod.yaml` 生成运行时配置，并要求一组强制环境变量。三条路径的前置条件、凭据策略和成功判据不一致。
4. **文档结论与证据成熟度混用（P1/P2）**：README 使用“稳定 Web UI”“七引擎”等能力表述，而架构文档明确记录部分 CDP 定级未执行、Censys 被挑战阻断；文档核对日期也早于 2026-08-11/12 的代码变更。用户难以判断哪些能力是代码存在、模拟验证、真实验证或仍受外部条件阻断。
5. **认证首启契约已完成核心修复**：隐式 `admin/admin` 与启动时生成但用户难以取得的随机管理 token 已移除；loopback `/login` 提供首管理员注册入口；非 loopback 启动由 preflight 强制显式凭据，匿名首注册在中间件和 handler 双层拒绝；Compose 与生产容器绑定契约已同步。
6. **运行时数据源码隔离已交付**：`.gitignore` 使用根锚定 `/*.eml` 与 `/mimoclaw_workspace/`，连同既有三项凭据类规则，五个目标均由 `git check-ignore -v` 命中。敏感文件未被读取或删除，审查结论为 **Approve**。
7. **冷启动错误语义已修复**：loopback bootstrap 待完成时，登录 API 返回 409 并引导创建首管理员；注册前 `Count` 失败返回 500 且用户创建次数为 0。Web 专项、全包、vet 与差异检查通过，复审结论为 **Approve**。

本轮认证、冷启动语义与运行时隔离修复均已取得目标证据并经复审 **Approve**。下一阶段仍需处理未知用户名登录可能返回 500、loopback `adminToken` 自动持久化、非 loopback 空数据库的配置型管理员/带外初始化路径，并完成全仓测试、race、Docker 实际运行与 E2E。

## 2. 范围与证据

### 2.1 审计范围

- 入口与首次启动：`cmd/unimap-web`、`cmd/unimap-cli`、`cmd/unimap-gui`。
- 配置与部署：`configs/`、`.env.example`、`Dockerfile`、`docker-compose*.yaml`、`scripts/docker-entrypoint.sh`。
- 测试反馈：全仓 Go 测试与空凭据场景。
- 文档一致性：根 [README](../README.md)、[架构说明](ARCHITECTURE.md)、[运行手册](RUNBOOK.md)、[快速开始](QUICKSTART.md)。
- 工作区卫生：`git status --short` 与敏感命名文件的存在性；未读取或记录凭据内容。

### 2.2 已观察证据

| 编号 | 证据 | 结论边界 |
|---|---|---|
| E-01 | README 第 41 行给出 `go run ./cmd/unimap-web`；`cmd/` 下另有 CLI、GUI 入口 | 入口文件存在；不等同于三入口均已完成交互验收 |
| E-02 | **基线失败证据：** `go test ./...` 运行超过 124 秒；当时 ZoomEye 空 Key 测试仍触发网络重试 | 保留为修复前证据；全套测试仍未在合理时间内完成，结果不得记为通过 |
| E-03 | **基线证据：** 修复前 `configs/config.yaml.example:187-189` 描述 token 可自动生成、用户名为 `admin`、空 `password_hash` 回落到 `admin` | 默认管理员凭据风险已由当前交付移除；该行保留修复动因与历史边界 |
| E-04 | `git status --short` 显示未跟踪 `.eml` 和 `mimoclaw_workspace/`；仓库根还存在 Cookie/密码命名文件 | 存在误提交、误归档风险；本次未判定文件内容是否有效 |
| E-05 | README 宣称稳定七引擎 Web UI；`docs/ARCHITECTURE.md:49` 记载 FOFA/ZoomEye/Shodan CDP 未执行、Censys 受挑战阻断 | 能力声明与验证等级表达不一致 |
| E-06 | ARCHITECTURE 最后按代码核对为 2026-07-29，RUNBOOK 为 2026-07-24；最新提交为 2026-08-12 | 文档新鲜度落后于代码基线 |
| E-07 | 本地复制 `config.yaml.example`；基础 Compose 挂载 `config.docker.yaml`；生产 Compose 使用 `/app/runtime-config/config.yaml` 和 `config.prod.yaml` | 本地、基础容器和生产容器的启动契约分裂 |
| E-08 | **修复后证据：** `zoomeye.go` 的 `Search` 在主、备 Key 均为空时直接返回配置错误；测试以 `httptest` 请求计数断言为 0 | ZoomEye Search 空 Key 网络重试问题已修复并完成目标验证；不代表全量测试已通过 |
| E-09 | `StartupPreflight` 在 Web 主入口创建服务前执行；非 loopback 强制认证启用、显式非空 `admin_token`、非默认用户名和有效 bcrypt 哈希 | 非 loopback 配置缺失会在监听前失败；CLI/GUI 不受 Web preflight 误阻断 |
| E-10 | loopback `/login` 根据空用户库显示首管理员注册 UI；非 loopback 下中间件与 `handleRegister` 均拒绝匿名首注册 | 本地首启有可见路径，远程抢注首管理员竞态得到双层约束 |
| E-11 | 基础 Compose 明确 `UNIMAP_ADMIN_TOKEN`、`UNIMAP_ADMIN_USERNAME`、`UNIMAP_BOOTSTRAP_PASSWORD` 三项凭据；生产覆盖补齐容器内 `0.0.0.0` 绑定与 bootstrap 密码 | 容器内监听与宿主 `127.0.0.1:8448:8448` 发布契约一致；实际容器运行尚待验证 |
| E-12 | `password_hash` 进入环境变量解析；解析器仅识别合法 `$VAR`/`${VAR}`，bcrypt 的 `$2...` 前缀保持原值 | 支持哈希环境占位符，同时避免把字面 bcrypt 哈希误清空 |
| E-13 | `.gitignore:190-191` 新增根锚定 `/*.eml` 与 `/mimoclaw_workspace/`；邮件、MCP 工作区及三个既有凭据类目标共五项 `git check-ignore -v` 均 exit 0 | 目标文件从 Git 候选状态隔离；规则不遮蔽子目录测试 fixture |
| E-14 | loopback 空用户库且登录凭据尚未初始化时返回 HTTP 409，错误正文引导创建首管理员 | 冷启动不再以含混 500 表示正常 bootstrap 状态 |
| E-15 | `handleRegister` 对 `userRepo.Count()` 错误返回 HTTP 500；测试断言 `createCalls == 0` 且用户库保持为空 | 数据库计数故障不再被吞掉，也不会继续创建首用户 |

## 3. 问题清单与优先级

> 优先级定义：P0 = 发布阻断/凭据暴露或默认接管风险；P1 = 核心流程不可靠或难以验收；P2 = 明显降低效率与可信度；P3 = 一致性和体验改进。

| ID | 级别 | 问题 | 用户/业务影响 | 证据 | 建议 |
|---|---|---|---|---|---|
| U-01 | **P0（Git 候选隔离已修复）** | 工作区混入邮件、个人工作目录及 Cookie/密码命名文件 | 可能随压缩包或镜像构建上下文泄露 | E-04、E-13 | 五项目标已被 ignore 且未读取/删除；继续建立构建上下文与制品扫描，并由凭据负责人决定轮换或迁移 |
| U-02 | **P0（核心已修复）** | 基线示例配置允许空 `password_hash` 回落到默认 `admin` 密码，并自动生成难以取得的管理 token | 首次启动可能形成可预测登录或不可操作的 API 认证 | E-03、E-09—E-12 | 已移除隐式默认口令与随机 token；非 loopback fail-closed，loopback 走首管理员注册。后续补齐 loopback `adminToken` 自动持久化 |
| U-03 | **P1（ZoomEye 子项已修复）** | 默认测试可能访问外网并长时间重试 | 开发、CI 和故障定位反馈不可预测；外部波动会伪装成代码回归 | E-02、E-08 | ZoomEye Search 空 Key 已在请求前返回稳定错误；继续审计其余适配器与测试，将外网/E2E 改为显式 tag 或环境开关，并设置每包超时 |
| U-04 | **P1** | 缺少可完成的快速验证层 | README 的单一 `go test -race ./...` 对日常开发过重，124 秒仍无结论 | E-02；README“验证”章节 | 定义 `test-unit`（离线、≤60 秒）、`test-integration`、`test-e2e` 三层；CI 逐层报告包名、耗时和失败原因 |
| U-05 | **P1（认证契约部分已修复）** | 本地、Compose、生产 Compose 配置生成与凭据要求不同 | 同一应用在不同路径出现不同默认行为，支持人员难以复现用户问题 | E-07、E-09—E-12 | 认证必填项、绑定地址和 preflight 已统一；继续统一完整配置来源、健康判据并提供 `doctor` |
| U-06 | **P1** | 能力声明未显式区分实现状态与真实验收状态 | 用户基于“稳定”“七引擎”形成错误预期；故障时不知是配置、挑战还是缺陷 | E-05 | 建立七引擎 × API/CDP/Extension × 本地/容器的证据矩阵，状态仅允许“已实现/模拟通过/真实通过/阻断/未执行” |
| U-07 | **P2** | 架构与运维文档核对日期早于最新代码 | 新增备用 Key/自动故障切换等行为可能未进入操作说明，值班依据过期 | E-06 | 文档 CI 记录代码 SHA；触及入口、配置、路由、引擎策略时强制更新对应文档和核对日期 |
| U-08 | **P2** | 三入口的受众、能力差异和推荐路径不清楚 | 新用户不知选择 Web/CLI/GUI；GUI 的 build tag 等限制隐藏较深 | E-01；ARCHITECTURE 第 19 行 | README 首屏给决策表；Web 标为默认入口；为 CLI/GUI 分列前置条件、适用场景和最小成功示例 |
| U-09 | **P2** | 首次启动没有统一、可操作的失败诊断 | 配置缺失、浏览器缺失、Key 缺失、网络挑战可能都表现为“功能不好用” | RUNBOOK 有分散 readiness 规则，但快速开始缺少聚合诊断 | 新增 `unimap doctor` 或 `/health/ready` 详细管理视图，输出组件、原因、修复动作且不泄露秘密 |
| U-10 | **P3** | README 存在空代码块，验证入口与完整命令分散 | 降低文档完成度和复制执行体验 | README CLI 示例后有空 `bash` 代码块 | 删除空块；每段命令标注平台、预期输出与下一步；所有指南互链到唯一启动契约 |
| U-11 | **P1（已修复）** | loopback 冷启动登录路径曾返回含混 500 | 用户看到服务错误而不是明确首启动作 | E-14 | bootstrap 待完成时返回 409 并引导创建首管理员；目标测试与 web 全包通过 |
| U-12 | **P1（已修复）** | 用户库 `Count` 错误曾在注册路径被忽略 | 数据库故障可能触发错误的首用户判断 | E-15 | Count 失败返回 500 且零创建；目标测试覆盖错误码、调用次数和空用户库不变量 |
| U-13 | **P2** | loopback 首启后的 `adminToken` 尚无自动持久化闭环 | Web 首管理员可建立，但 CLI/API token 的后续使用仍可能需要人工配置 | 当前复审遗留项 | 首注册时原子生成并持久化 token，提供一次展示/下载或受控 token 文件，并验证重启恢复 |
| U-14 | **P1（源码隔离已修复）** | 邮件、个人工作目录及运行时凭据类文件曾出现在 Git 候选状态 | 可能误提交或误纳入人工交付 | E-04、E-13 | 根锚定规则与既有凭据规则已覆盖五项目标，且未读取/删除敏感文件；发布前仍需检查最终制品清单 |
| U-15 | **P2** | 未知用户名登录分支仍可能返回 500 | 普通认证失败与服务异常语义混淆 | 当前复审遗留项 | 将未知用户统一映射为稳定认证失败，保持防枚举响应一致并加入回归测试 |
| U-16 | **P1** | 非 loopback 空数据库不开放匿名首注册，但尚缺配置型管理员或带外初始化闭环 | 新部署满足安全拒绝后仍可能没有可用首管理员 | 当前复审遗留项 | 支持配置型管理员落库或受控带外初始化命令；验证一次性、幂等、审计与重启恢复 |

## 4. 可用性视角分析

### 4.1 可发现性

- Web 是事实上的默认入口，但 README 同时展示 CLI，GUI 只在架构文档中说明 build tag，入口选择成本高。
- “七引擎”是功能列表，不是用户任务路径。更有效的首屏应是：配置凭据 → 运行预检 → 发起首次查询 → 理解降级/失败 → 导出结果。

### 4.2 首次成功时间

- loopback 本地首启已从隐式 `admin/admin` 改为 `/login` 可见的首管理员注册 UI，降低了默认口令风险，也让初始化动作可发现。
- 非 loopback 由启动 preflight 在监听前校验凭据；Docker 明确 token、非默认用户名、bootstrap 密码三项输入。该契约已有静态和单元测试证据，实际容器启动闭环仍待执行。
- loopback bootstrap 登录已由 500 收敛为 409 引导；未知用户名仍可能返回 500，首注册后的 `adminToken` 持久化也尚未闭环，应列为下一阶段优先项。

### 4.3 反馈与错误恢复

- 空 Key 应立即、明确地提示“该引擎未配置”，而非进入网络重试。ZoomEye `Search` 已按此原则修复并验证；其余引擎仍按计划逐一核对。
- 第三方站点挑战、认证失效、限流、DNS/代理错误需要不同错误码和下一步动作。自动 fallback 必须在 UI 中显示实际执行路径，避免“查不到结果”等同于“没有资产”。
- 测试命令应持续输出包级进度/耗时，并设置总预算；静默等待 124 秒以上本身就是可用性缺陷。

### 4.4 一致性与信任

- README、ARCHITECTURE、RUNBOOK 对同一能力使用不同证据粒度。发布页面应引用同一机器可读能力矩阵。
- “已实现”不等于“真实服务通过”；“受挑战阻断”也不应写成产品代码失败。清晰分级可同时避免夸大和误判。

### 4.5 运维可诊断性

- readiness 已存在，但本地/容器的配置来源与检查上下文不同。应让 `doctor` 输出最终配置来源（仅路径和是否设置，不输出值）、端口、数据目录、浏览器探测和各引擎可用状态。
- 交付包应从干净 Git 树构建并生成清单，排除邮件、个人目录和凭据类文件。

## 5. 分阶段优化计划

### 5.1 立即（0—2 天）：止血与建立可信基线

1. **隔离和轮换凭据（Git 候选隔离已完成）**：五项目标已命中 ignore，且未读取/删除；下一步核对历史和构建产物，由凭据负责人判断迁移与轮换。
2. **禁用默认管理员密码（已完成核心实现）**：已移除隐式 `admin/admin` 和不可获知随机 token；默认绑定 loopback，未初始化时提供首管理员注册；非 loopback 使用严格 preflight。
3. **修复空 Key 网络调用**：各引擎在 client 层前置校验；测试使用 fake transport 并断言请求次数为 0。
4. **建立 60 秒离线冒烟集**：先覆盖配置加载、认证、路由注册、空 Key、核心查询转换；失败必须打印具体包和阶段。
5. **收敛认证首启边界（前两项已完成）**：冷启动 bootstrap 已返回 409，注册 `Count` 错误已返回 500 且零创建；下一步修复未知用户名 500，并为 loopback `adminToken` 建立安全持久化与重启恢复测试。
6. **冻结能力措辞**：在修复验收前，将 README 的绝对性声明链接到证据矩阵并标识未执行/阻断项。

**立即阶段退出条件：** 默认凭据路径被拒绝；秘密扫描无高置信结果；空 Key 测试零网络请求；离线冒烟集稳定完成。

### 5.2 短期（3—10 天）：统一用户路径

1. 定义并实现单一配置优先级（命令行 > 环境 > 指定配置 > 安全默认），本地与 Docker 共用同一校验器。
2. 增加 `doctor`/preflight，覆盖端口、数据目录权限、浏览器、代理、Key 是否设置、数据库、readiness；输出可复制的修复建议。
3. 重写 Quickstart 为 5 分钟闭环：安全初始化、启动、健康检查、一次离线/fixture 查询、停止与清理。
4. Web/CLI/GUI 建立入口决策表和一致的错误词汇；UI 展示实际引擎路径、fallback 原因和请求 ID。
5. 将测试分为 unit/integration/external-e2e；外网测试显式 opt-in，并归档时间、服务状态和脱敏证据。
6. 生成能力证据矩阵，README 只引用矩阵摘要；ARCHITECTURE/RUNBOOK 更新 SHA 和日期。
7. 运行时邮件、MCP 工作区和三项凭据类目标已由根锚定/既有 ignore 规则隔离；下一步建立构建上下文 allowlist 并检查最终制品清单。

**短期退出条件：** 三种启动路径使用同一必填项和成功判据；冷启动不出现含混 500；首管理员与 token 在重启后可恢复；新用户按 Quickstart 可在 5 分钟内到达健康状态；失败可由 `doctor` 定位到具体组件。

### 5.3 发布前（11—20 天）：闭环和发布门禁

1. 在干净临时目录从 Git checkout 构建，验证产物清单不含邮件、Cookie、密码或个人工作目录。
2. 对本地 Web、基础 Compose、生产 Compose 分别执行启动—登录—查询—导出—重启恢复—停止闭环。
3. 按能力矩阵执行真实服务验收；外部挑战保留为“阻断”并验证 fallback，不用模拟结果替代真实证据。
4. 运行全量 race、集成、容器健康和关键浏览器 E2E；对每层设定明确时间预算、重试上限和日志归档位置。
5. 演练配置/数据备份与回滚；确认旧版本可读取回滚数据，或提供单向迁移警告。
6. 发布说明列出已验证、未验证、外部阻断、已知限制及证据日期。

**发布门禁：** P0 清零；P1 有验证证据或明确阻断决定；文档 SHA 与候选版本一致；干净构建和回滚演练通过。

### 5.4 下一阶段执行顺序

1. **认证剩余语义闭环**：将未知用户名登录从 500 收敛为防枚举一致的认证失败；保留 bootstrap 409 与 Count 失败 500 的现有回归测试。
2. **凭据生命周期闭环**：设计并实现 loopback 首注册后的 `adminToken` 原子持久化、一次性展示或受控 token 文件，以及重启恢复与轮换测试。
3. **非 loopback 首用户闭环**：实现配置型管理员或受控带外初始化，确保空数据库在禁止匿名注册后仍有可审计、可恢复的首管理员路径。
4. **隔离发布验收**：保留五项 check-ignore 门禁，补充 `.dockerignore`/构建 allowlist 与最终镜像清单检查；敏感文件继续原地保留直至其负责人另行处置。
5. **扩大验证半径**：依次执行完整 `go test ./...`、`go test -race ./...`、基础/生产 Docker build-up-health-restart、Web 真实首启 E2E 和 GUI 验收。
6. **文档与发布收口**：根据运行证据更新 README、QUICKSTART、ARCHITECTURE、RUNBOOK 和能力矩阵，再执行干净构建与回滚演练。

## 6. 验收指标与命令

以下是**计划验收命令**，不是本次已通过声明。命令应在修复分支和干净测试环境执行；任何超时均按失败记录，不得省略退出码与日志。

| 目标 | 指标 | 建议命令/检查 |
|---|---|---|
| 工作区卫生 | 候选提交 `git status` 为空；扫描无高置信秘密；产物清单无禁止文件 | `git status --short`；`git ls-files`；项目选定的 secret scanner；`docker build --no-cache -t unimap:audit .` 后检查镜像文件清单 |
| 离线单测 | 60 秒内完成；0 次真实 DNS/HTTP；0 失败 | `go test -count=1 -timeout=60s ./internal/...`（在测试隔离完成后执行） |
| 空 Key | 每个引擎返回稳定配置错误；fake transport 请求计数为 0 | `go test -count=1 -run 'Empty.*Key|Missing.*Credential' ./internal/...` |
| 认证配置 | 无隐式默认口令或随机 token；loopback 允许首注册；非 loopback 缺任一凭据时启动前拒绝 | `go test ./internal/config -count=1` |
| 登录与注册 | loopback `/login` 显示首管理员注册；非 loopback 匿名注册在中间件及 handler 均为 403；首用户角色为 admin | `go test ./web -run 'TestHandleLoginPage_(RegistrationEntry|NoRegistrationEntry)|TestRegistrationMiddleware_|TestHandleRegister_(LoopbackBootstrap|NonLoopback)' -count=1` |
| 冷启动错误语义 | loopback bootstrap 登录返回 409 且包含首管理员引导；Count 失败返回 500、创建调用为 0 | `go test ./web -run 'TestHandleLoginAPI_LoopbackColdStartReturns409Bootstrap|TestHandleRegister_CountErrorReturns500NoCreate' -count=1` |
| 配置环境解析 | `${VAR}` 可解析到 `password_hash`；字面 bcrypt `$2...` 保持原值 | `go test ./internal/config -run 'TestResolveEnvPasswordHash' -count=1` |
| 生产 Compose 绑定 | 容器内 `0.0.0.0`，宿主仅 `127.0.0.1:8448:8448`；三项认证凭据必填 | `docker compose -f docker-compose.yml -f docker-compose.prod.yaml config`（脱敏查看） |
| 全量 Go 测试 | 在团队设定预算内结束；记录退出码、包级耗时；无 race | `go test -race -count=1 -timeout=10m ./...` |
| 本地启动 | 仅 loopback 监听；安全初始化完成；ready 返回成功 | `go run ./cmd/unimap-web`；另一终端请求 `http://127.0.0.1:8448/health/ready` |
| Compose 契约 | 合并配置仅绑定 `127.0.0.1:8448:8448`，无开发 bind mount，必填秘密缺失时 fail-fast | `docker compose -f docker-compose.yml -f docker-compose.prod.yaml config`（输出须脱敏且不落盘） |
| 容器闭环 | health 为 healthy；重启后认证、数据和最小查询仍工作 | `docker compose -f docker-compose.yml -f docker-compose.prod.yaml up -d --build`；`docker compose ps`；`docker compose restart unimap` |
| 文档一致性 | README/ARCHITECTURE/RUNBOOK 均记录候选 SHA；链接存在；能力矩阵无含混状态 | 检查 `git rev-parse --short HEAD`；运行仓库选定的 Markdown link checker |
| 首次成功时间 | 新环境从 checkout 到 ready ≤5 分钟；无需猜测配置项 | 由未参与开发的测试者按 QUICKSTART 计时并记录卡点 |
| 静态质量 | `go vet` 与 whitespace/error-marker 差异检查均为 exit 0 | `go vet ./...`；`git diff --check` |
| 运行时 Git 隔离 | 根目录邮件、MCP 工作区及三项凭据类目标均被忽略，且根规则不遮蔽子目录 fixture | `git check-ignore -v -- 'Fw__[UniMap]_成功_daydaymap_ynmobile_b.eml' mimoclaw_workspace cookie.txt unimap_session_cookie.txt qqemailpasswd.txt` |
| 真实引擎 | 每个声称“真实通过”的通道有日期、脱敏输入、非空结果、执行路径和证据 | 显式 external-E2E 命令；凭据由 CI secret 注入，日志排除凭据值 |

建议 CI 至少保存：命令原文、开始/结束时间、Git SHA、退出码、超时标志、失败包、脱敏日志和产物摘要哈希。

## 7. 风险与回滚

| 变更 | 主要风险 | 缓解 | 回滚策略 |
|---|---|---|---|
| 移除默认管理员密码 | 现有无人值守部署启动失败 | 发布前扫描部署变量，提供一次性迁移检查 | 临时回退到上一镜像，但保持 loopback/防火墙限制；不得恢复公开默认密码 |
| 统一配置优先级 | 旧配置被新优先级覆盖 | 启动日志只打印来源与字段名，提供 `doctor --config` 预览 | 保留旧解析器一个版本，通过显式兼容开关回滚 |
| 禁止测试外网 | 原测试依赖真实响应而暴露 mock 缺口 | 保留显式 external-E2E 流水线 | 回滚测试结构而非恢复默认外网；外网层继续显式 opt-in |
| 新增 fail-fast | 用户看到更多启动失败 | 错误包含字段、来源、修复命令 | 允许短期 warning 模式，仅限 loopback 开发环境并设移除日期 |
| 文档能力降级标识 | 被误解为功能退化 | 区分代码能力、真实验证和外部阻断 | 由新证据升级状态，不以删除限制说明回滚 |
| 清理工作区文件 | 误删未归档的业务资料 | 先哈希、备份到仓库外受控位置，再删除；确认负责人 | 从受控备份恢复到仓库外，不恢复进源码树 |
| non-loopback preflight | 旧部署缺少显式凭据时启动中止 | 部署前运行 Compose config 和 preflight 目标测试，提前补齐三项凭据 | 回退应用版本并保持宿主 loopback/防火墙限制；同时补齐凭据后重新升级 |
| loopback 首管理员注册 | 空用户库暴露时间窗或重复注册竞态 | 仅 loopback 开放，中间件与 handler 双层检查，数据库保持用户名唯一约束 | 关闭服务，恢复用户数据库备份，重新执行本地首启 |
| `password_hash` 环境解析 | `$` 前缀字面值被误识别或部署变量缺失 | 仅合法变量名参与解析；bcrypt 字面值与变量占位符均有回归测试 | 恢复上一配置解析器并改用显式 YAML 哈希，随后重新验证 preflight |
| 根锚定 ignore 规则 | 规则过宽导致测试 fixture 消失，或仅隐藏而未阻止制品打包 | 使用 `/*.eml` 与 `/mimoclaw_workspace/` 根锚定；发布前独立检查 Docker 上下文与镜像清单 | 回退 `.gitignore` 两行并改用更精确根文件名；敏感目标继续留在仓库外部交付清单之外 |
| bootstrap 409 语义 | 旧客户端只处理 401/500 | API 文档明确 409，前端据此切换注册 UI，并保留兼容提示 | 回退 handler 语义改动；仍保持默认凭据移除与 loopback 注册限制 |

每个实现批次应保持小提交，并记录：变更前配置、数据库/数据目录备份、旧镜像 tag、迁移命令、回滚命令和回滚后健康检查。涉及认证或配置格式时，先验证向后兼容或明确不可逆边界。

## 8. 当前交付状态

### 8.1 已验证

- [x] 仓库存在 Web、CLI、GUI 入口目录；默认 Web 命令为 `go run ./cmd/unimap-web`。
- [x] README、ARCHITECTURE、RUNBOOK、示例配置及 Docker 启动相关文件已做静态交叉核对。
- [x] 基线示例配置曾包含 `admin` 用户名和空哈希回落到 `admin` 密码的文字规则；当前交付已移除该回落说明与实现。
- [x] 基线 `git status --short` 曾显示未跟踪邮件文件与 `mimoclaw_workspace/`；当前根锚定规则已使二者退出 Git 候选状态。
- [x] 根目录存在 Cookie/密码命名文件；本次没有读取其内容，也未断言其中凭据有效。
- [x] ARCHITECTURE/RUNBOOK 的代码核对日期早于当前最新提交日期。
- [x] 本地、基础 Compose、生产 Compose 的配置路径和必填环境变量存在差异。
- [x] 本报告内部相对链接目标在编写时存在。
- [x] ZoomEye `Search` 在 `internal/adapter/zoomeye.go:238-242` 前置判断主、备 Key 均为空并快速返回；目标测试断言 HTTP 请求计数为 0。
- [x] ZoomEye `GetQuota` 的空 Key 快速失败测试同样断言 HTTP 请求计数为 0。
- [x] 配置默认逻辑不再生成 `admin/admin` 或启动后难以取得的随机 `admin_token`。
- [x] 非 loopback Web 启动在监听前执行认证 preflight；loopback `/login` 显示首管理员注册 UI。
- [x] 非 loopback 匿名首注册在路由中间件和注册 handler 两层返回 403。
- [x] 基础与生产 Compose 均明确三项认证凭据；生产容器内绑定和宿主端口发布契约已修正。
- [x] `password_hash` 环境变量占位符解析与字面 bcrypt 保留均有目标测试。
- [x] 代码复审结论：**Approve**。
- [x] 五项目标经 `git check-ignore -v` 命中：根目录邮件、`mimoclaw_workspace/`、`cookie.txt`、`unimap_session_cookie.txt`、`qqemailpasswd.txt`；均为 exit 0。
- [x] 隔离交付过程中未读取、移动或删除上述敏感文件；审查结论：**Approve**。
- [x] loopback 冷启动登录已返回 409 并给出首管理员注册引导；注册 `Count` 失败返回 500 且用户创建调用为 0；复审结论：**Approve**。

### 8.2 本轮修复与验证记录

| 范围 | 命令 | 用时 | 退出码 | 结果边界 |
|---|---|---:|---:|---|
| ZoomEye Search 空 Key 目标测试 | `go test ./internal/adapter -run 'TestZoomEyeAdapter_Search/empty_api_key_fails_fast_without_network' -count=1` | 0.175s | 0 | 快速失败且请求计数为 0，已通过 |
| ZoomEye GetQuota 空 Key 目标测试 | `go test ./internal/adapter -run 'TestZoomEyeAdapter_GetQuota/empty_api_key_fails_fast_without_network' -count=1` | 0.164s | 0 | 快速失败且请求计数为 0，已通过 |
| adapter 全包测试 | `go test ./internal/adapter -count=1` | 13.217s | 0 | 适配器包在本轮修复状态下通过 |
| 全仓编译-only | `go test ./... -run '^$'` | 43.5s | 0 | 全仓包可编译；未执行测试用例，不等同于全量测试通过 |
| config 全包测试 | `go test ./internal/config -count=1` | 0.814s | 0 | 默认凭据移除、preflight、Docker/生产配置、`password_hash` 环境解析测试通过 |
| Web 认证目标测试 | `go test ./web -run 'TestHandleLoginPage_(RegistrationEntry_LoopbackColdStart|NoRegistrationEntry_NonLoopback)|TestRegistrationMiddleware_(LoopbackBootstrapPublic|NonLoopbackAnonymousRejected)|TestHandleRegister_(LoopbackBootstrapCreatesAdminRole|NonLoopbackRejected)' -count=1` | 0.669s | 0 | loopback 首注册 UI 与 non-loopback 双层拒绝目标行为通过 |
| web 全包测试 | `go test ./web -count=1` | 11.909s | 0 | Web 包当前测试集通过 |
| Go 静态检查 | `go vet ./...` | 未记录 | 0 | 本轮复审静态检查通过 |
| 差异格式检查 | `git diff --check` | 未记录 | 0 | 未发现 whitespace/error-marker 问题 |
| 代码复审 | 认证首启变更复审 | — | — | **Approve**；遗留项已列入 8.3 与下一阶段计划 |
| 运行时隔离检查 | 对邮件、MCP 工作区、两个 Cookie 文件和密码命名文件逐项执行 `git check-ignore -v` | 未记录 | 0（五项） | 五项目标均命中；敏感文件未读取或删除 |
| 冷启动语义目标测试 | `go test ./web -run 'TestHandleLoginAPI_LoopbackColdStartReturns409Bootstrap|TestHandleRegister_CountErrorReturns500NoCreate' -count=1` | 未记录 | 0 | bootstrap 409 引导与 Count 失败 500/零创建通过 |
| 冷启动修复后 web 全包 | `go test ./web -count=1` | 未记录 | 0 | 新增错误语义回归测试纳入全包并通过 |
| 冷启动修复后静态检查 | `go vet ./...`；`git diff --check` | 未记录 | 0 / 0 | vet 与差异检查通过 |
| 隔离与冷启动复审 | 两项变更复审 | — | — | **Approve** |

基线中“ZoomEye 空 Key 仍发生网络重试”的问题已由本轮修复关闭。基线 `go test ./...` 超过 124 秒的证据继续保留，因为本轮只取得编译-only 全仓结果，尚无全量执行完成证据。

### 8.3 当前仍未验证

- [ ] `go test ./...` 全量测试执行结果；基线超过 124 秒后未取得完成状态。
- [ ] `go test -race ./...` 全量结果。
- [ ] 未知用户名登录分支仍可能返回 500，尚未收敛为防枚举一致的认证失败。
- [ ] loopback 首管理员注册后 `adminToken` 自动持久化及重启恢复。
- [ ] 非 loopback 空数据库的配置型管理员或受控带外初始化路径。
- [ ] Web 登录、首次查询、导出和重启恢复的本地端到端闭环。
- [ ] GUI 构建与交互流程。
- [ ] 基础 Compose 与生产 Compose 的实际 build/up/health/restart 闭环。
- [ ] 最终 Docker 构建上下文与镜像制品清单的敏感文件排除验收；Git 候选隔离已完成。
- [ ] 七引擎当前版本的真实 API、CDP、Extension 全矩阵；沿用文档中的“未执行/阻断”边界，不提升状态。
- [ ] 工作区敏感命名文件内容是否有效、是否曾进入历史或外部制品。
- [ ] 文档中的全部命令、锚点和外部链接自动化检查。
- [ ] 除已完成的 ZoomEye 和认证首启核心修复外，其余配置、CI、诊断与文档体系改造。

### 8.4 当前交付文件列表

| 文件 | 交付角色 | 当前状态 |
|---|---|---|
| `.gitignore` | 根锚定邮件/MCP 运行时工作区隔离；复用三项既有凭据规则 | 已修改；五项 check-ignore exit 0 |
| `cmd/unimap-web/main.go` | Web 服务创建前接入认证启动 preflight | 已修改 |
| `configs/config.docker.yaml` | Docker loopback/非 loopback 认证契约说明与环境占位符 | 已修改 |
| `configs/config.prod.yaml` | 生产认证字段与 bootstrap 密码契约 | 已修改 |
| `configs/config.yaml.example` | 移除默认口令/随机 token 语义，补充安全首启说明 | 已修改 |
| `docker-compose.yml` | 基础 Compose 三项认证凭据 fail-fast | 已修改 |
| `docker-compose.prod.yaml` | 生产容器绑定与 bootstrap 密码修复 | 已修改 |
| `internal/config/config_defaults.go` | 移除隐式凭据生成，保留 loopback 首注册初始化 | 已修改 |
| `internal/config/config_load.go` | `password_hash` 环境解析与合法占位符识别 | 已修改 |
| `internal/config/notify_secret.go` | 复用统一 loopback 判定 | 已修改 |
| `internal/config/startup_preflight.go` | non-loopback 启动认证 preflight | 新增 |
| `internal/config/startup_preflight_test.go` | 默认凭据、preflight、Compose、env 解析回归测试 | 新增；config 全包通过 |
| `web/login_handlers.go` | 登录页注入首注册开放状态 | 已修改 |
| `web/login_handlers_test.go` | loopback/non-loopback 登录页首注册入口测试 | 已修改 |
| `web/middleware_auth.go` | 匿名注册仅限 loopback 且空用户库 | 已修改 |
| `web/middleware_extended_test.go` | 注册开放与中间件拒绝测试 | 已修改 |
| `web/templates/login.html` | loopback 冷启动首管理员注册 UI | 已修改 |
| `web/user_handlers.go` | handler 层 non-loopback 匿名注册防御性拒绝 | 已修改 |
| `web/user_handlers_test.go` | 首用户 admin 与 non-loopback 拒绝测试 | 已修改；web 全包通过 |
| `internal/adapter/zoomeye.go` | ZoomEye `Search` 空 Key 前置快速失败实现 | 已修改；目标测试通过 |
| `internal/adapter/adapter_test.go` | Search/GetQuota 空 Key 零网络请求回归测试 | 已修改；适配器全包通过 |
| `docs/SYSTEM_USABILITY_AUDIT_2026-08-12.md` | 审计、修复状态、验证证据、计划与回滚 | 本文件；已更新并自检 |

邮件、`mimoclaw_workspace/` 与三个凭据类文件属于被隔离的运行时目标，不属于交付文件；本轮未读取、移动或删除其内容。

### 8.5 回滚命令

以下 PowerShell 命令是**完整交付回滚提示**，会丢弃所列文件中的未提交交付改动。执行前应先导出补丁并核对文件所有权；既有邮件与 `mimoclaw_workspace/` 不在命令范围内。两个新增 preflight 文件和本报告当前未跟踪，需单独移除。

```powershell
# 保存已跟踪文件补丁及三个新增文件
git diff --binary -- .gitignore cmd/unimap-web/main.go configs/config.docker.yaml configs/config.prod.yaml configs/config.yaml.example docker-compose.yml docker-compose.prod.yaml internal/adapter/adapter_test.go internal/adapter/zoomeye.go internal/config/config_defaults.go internal/config/config_load.go internal/config/notify_secret.go web/login_handlers.go web/login_handlers_test.go web/middleware_auth.go web/middleware_extended_test.go web/templates/login.html web/user_handlers.go web/user_handlers_test.go > .\unimap-auth-delivery.patch
New-Item -ItemType Directory -Force .\unimap-auth-new-files | Out-Null
Copy-Item -LiteralPath internal/config/startup_preflight.go,internal/config/startup_preflight_test.go,docs/SYSTEM_USABILITY_AUDIT_2026-08-12.md -Destination .\unimap-auth-new-files\

# 回滚所有已跟踪交付文件
git restore --source=HEAD --worktree -- .gitignore cmd/unimap-web/main.go configs/config.docker.yaml configs/config.prod.yaml configs/config.yaml.example docker-compose.yml docker-compose.prod.yaml internal/adapter/adapter_test.go internal/adapter/zoomeye.go internal/config/config_defaults.go internal/config/config_load.go internal/config/notify_secret.go web/login_handlers.go web/login_handlers_test.go web/middleware_auth.go web/middleware_extended_test.go web/templates/login.html web/user_handlers.go web/user_handlers_test.go

# 移除本轮新增文件
Remove-Item -LiteralPath internal/config/startup_preflight.go,internal/config/startup_preflight_test.go,docs/SYSTEM_USABILITY_AUDIT_2026-08-12.md

# 验证回滚范围
git status --short
```

## 9. 交付判定

本报告完成了问题分级、可用性分析、阶段计划、验收门禁和回滚设计。ZoomEye 空 Key、认证首启核心、运行时 Git 隔离、bootstrap 409 与 Count 失败 500/零创建均已修复并取得 exit 0 证据；相关复审结论均为 **Approve**。发布可用性仍待完整验收：未知用户名 500、loopback token 持久化、非 loopback 首用户初始化、全仓测试、race、Docker 实际运行、GUI、E2E 和最终制品隔离检查仍为未完成项。
