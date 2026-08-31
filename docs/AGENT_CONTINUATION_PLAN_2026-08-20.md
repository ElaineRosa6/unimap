# UniMap 后续推进计划书（2026-08-20）

> 对象：后续会话里的 agent。目标是按冻结基线继续推进，而不是重做已完成工作。
> 核对：2026-08-25（Asia/Shanghai）。仓库文档调查 + 阿里云 SSH 机内核查（08-20）；08-21 本机插件活页与 CDP 分列见 [PLUGIN_CDP_STATUS_2026-08-21.md](PLUGIN_CDP_STATUS_2026-08-21.md)；08-25 本机邮件通道与云端 SMTP 配置一致并经收件确认。
> 权威关系：执行顺序以本文为准；能力矩阵与完成标准以 [REMAINING_WORK_2026-07-23.md](REMAINING_WORK_2026-07-23.md) 为准；API/安全以代码、`docs/API.md`、`docs/RUNBOOK.md` 为准。
> 历史计划：[NEXT_STEPS_20260802.md](NEXT_STEPS_20260802.md)、[IMPLEMENTATION_PLAN_2026-07-23.md](IMPLEMENTATION_PLAN_2026-07-23.md) 只表达当时事实，不再当执行队列。

## 0. 新会话怎么开始

1. 读本文第 1–4 节和第 9 节状态板，找到**第一个未完成且无外部阻塞**的条目。
2. 读 `docs/REMAINING_WORK_2026-07-23.md` 对应 RW 编号，不要另起一套 ID。
3. 不要实施第 4 节「明确不做」清单。
4. 做完一项：更新本文状态板、`docs/REMAINING_WORK_2026-07-23.md`、`docs/CHANGELOG.md`；触及路由/任务/引擎/认证/配置/Bridge 时再同步 API/Runbook/README。
5. 改代码：先补或更新受影响测试，至少 `go test -race` 对应包；收口时 `go test -race ./...`、`go vet ./...`、`go build ./...`。
6. 禁止把 API Key、Cookie、admin token、Bridge token、Webhook、SMTP 口令写入 Git、文档、测试数据或聊天。

## 1. 冻结基线（2026-08-20）

| 项 | 事实 |
|---|---|
| 本地/远端代码 | `master` `fdb3292`（文档提交），与 `origin/master` 同步 |
| 阿里云运行代码 | `/opt/unimap` 与镜像 `unimap:local` = `71371f1`（2026-08-17 11:15） |
| `develop` | `origin/develop` 停在 `71be507`（2026-08-06），落后 `master` 24 个提交，**不是**当前发布基线 |
| Go | `go.mod` `1.26.5` |
| 云主机 | 阿里云 Ubuntu 24.04，2 vCPU / 1.6 GiB / 40 GiB，目录 `/opt/unimap` |
| 容器 | `unimap-unimap-1` Up + healthy；`unimap-smtp-relay` Up 12 days + healthy |
| 端口 | SSH `:22` 公网；应用 `127.0.0.1:8448`；smtp-relay `127.0.0.1:8099`。不要把 8448 改成公网 |
| 健康 | `/health` `/health/live` `/health/ready` 均为 ok；`screenshot=degraded` 是因为截图总开关关闭，不是进程故障 |
| 调度 | 12 任务：9 启用（fofa×2、hunter×4、daydaymap×2、icp×1），quake×3 disabled |
| 通知 | 查询 → `email_agent`；ICP → 企微 `dijia_01` |
| 08-17 后历史 | 到 2026-08-20 15:00：`success=52`，`failed=0` |

管理登录走 SSH 隧道，步骤见 [RUNBOOK.md](RUNBOOK.md) 与 [CLOUD_STEADY_STATE_PLAN_2026-07-23.md](CLOUD_STEADY_STATE_PLAN_2026-07-23.md) 第 10 节。密钥在 `~/.ssh/aliyun_ubuntu.pem`。不要在文档里复述口令或 token。

试运行机是轻量容量，**不得**把本机验收写成生产容量通过。

## 2. 硬约束

- 业务 API 只使用 `/api/v1/...`，禁止旧 `/api/...` shim。
- 外部 URL、截图、巡检、Webhook 必须保留 SSRF；私有/loopback/链路本地不得绕过。
- `tamper.evidence_screenshot_enabled` 保持 `false`，直到 RW-09 的受控域名、DNS 编辑权和 fixture URL 齐备并完成 live E2E。
- 登录页、验证码页、空资产、普通网页截图不能当成测绘引擎 CDP 通过。
- 云端当前是 API 查询日更，截图关闭。未得到明确指令前，不要在这台低配机上打开截图或 CDP 采集。
- **2026-08-20 用户拍板（在补齐可用 key 并再次确认前禁止推翻）：** 不启用 3 个 Quake 任务（现无 key）；不把 ZoomEye / Shodan / Censys 纳入日更（现无可用 key）；云端截图保持关闭。日更范围固定为 FOFA / Hunter / DayDayMap 查询 + ICP。
- 非 loopback / 容器内 `0.0.0.0` 绑定必须满足 `StartupPreflight`：认证开、`admin_token` 非空、用户名不是 `admin`、bcrypt 合法。升级镜像前先改运行配置，不要靠改 bind 绕过。
- 标识符 MixedCaps；`gofmt`；改导入用 `goimports`。

## 3. 已经完成，不要重做

- 七引擎稳定 Web UI；缺 API 凭据时 Web-only adapter。
- 七引擎 Bridge 结构化采集非空（2026-08-02）。
- Quake / Hunter / DayDayMap 原生 CDP 非空；Censys 挑战识别 + Bridge fallback。
- 空 Key fail-fast、备用 Key 跳过空主键、登录未知用户 401、`GET /api/v1/config` 渠道脱敏（`71371f1`）。
- 云端 FOFA / Hunter / DayDayMap 查询 → 邮件、ICP → 企微的日更闭环（08-17 后到 08-20 全 success）。
- smtp-relay 仍在工作；不要因为「08-17 没 recreate」就判定邮件通道已死。
- GUI 入口仍在 `cmd/unimap-gui`（`gui` build tag）。不要按过期 memory 删除它。

## 4. 明确不做（除非用户改口）

2026-08-17 / 08-07 已否或暂缓：

- Load 失败即退出
- 非 loopback 首管理员自动落库（可作为后续 AUTH-1，但不是默认本旬）
- 注册时展示 token
- 保存 Cookie 时自动跑 CDP
- `/screenshots/` 强制登录
- 打开自动证据截图
- 恢复 API
- 配额趋势 / 自动刷新 / 阈值告警
- 调度成功/失败/超时分渠道
- ARM64 镜像
- 把 2C/1.6G 试运行机验收成生产容量

## 5. 执行队列

按波次推进。同一波次内可并行的已标明。**B 波需要用户拍板，agent 不得自行启用引擎或打开截图。**

### A 波：运维卫生（无产品代码，优先）

#### A1 OPS-1 清理云端旧镜像

- 状态：完成（2026-08-20）
- 目的：根分区 40G 已用 73%，多份 1.3GB UniMap 镜像。
- 步骤：SSH 后 `docker images`；保留 `unimap:local`、`unimap:local-bak-20260817`、`unimap-smtp-relay:latest`、构建用 `golang:1.26.5-alpine` / `alpine:3.21`。其余旧 `unimap:browser-seven-*`、`unimap-acceptance-*`、重复 ACR 标签在列出清单后删除。若用户要求保留一份 08-02 回滚镜像，只留 `unimap:browser-seven-final-20260802`。
- 禁止：删当前 `unimap:local`；`docker system prune -a` 无确认。
- 完成：`df -h /` 可用空间明显回升；`docker ps` 仍 healthy；`curl -fsS http://127.0.0.1:8448/health/ready` 仍 ok。

#### A2 OPS-2 对齐 admin token

- 状态：完成（2026-08-20）
- 事实：`.env` 的 `UNIMAP_ADMIN_TOKEN` 与 `/app/runtime-config/config.yaml` 的 `web.auth.admin_token` 长度相同但值不同。用环境变量调 API 返回 401，用运行配置 token 返回 200。
- 步骤：比较长度/哈希，不打印明文；以**运行配置**为准写回 `.env`（或反过来，但必须二选一并验证）；用 `X-Admin-Token` 打 `GET /api/v1/scheduler/tasks` 得 200 且 12 条任务。
- 完成：环境变量与运行配置一致；文档不出现 token。

#### A3 OPS-3 镜像写入提交号

- 状态：未开始
- 事实：`/health/live` 为 `commit=unknown, built=unknown`，仓库 git 是 `71371f1`。
- 步骤：按 Dockerfile / Compose 已有的版本注入方式重建或下次升级时带上 `GIT_COMMIT`/`VERSION`；不得为了打标而改业务逻辑。
- 完成：live/ready 的 version 含真实短 SHA。可与下次镜像升级合并，不必单独为打标 recreate。

#### A4 OPS-4 分支与权威文档

- 状态：完成（2026-08-20 文档）
- 完成：`AGENTS.md` / `CLAUDE.md` / README 写明试运行与发布跟踪 `master`；`develop` 落后被标明。是否快进 `develop` 留给用户决定，agent 不擅自 `push --force`。

#### A5 OPS-5 安全组复核

- 状态：未开始
- 步骤：阿里云控制台确认未放行 8448/9222/19451；本机有 `127.0.0.1:7897` 代理，公网探测不能当证据。
- 完成：记录控制台截图或规则列表（无密钥）到运维记录，或由用户确认。

### B 波：试运行业务拍板（先问用户）

B1–B3 已于 2026-08-20 拍板，后续 agent **不得**再问、也不得自行 enable 任务或打开截图。B4–B6 仍待确认，默认保持现状。

| ID | 问题 | 决定 | 说明 |
|---|---|---|---|
| B1 BIZ-1 | 是否启用 3 个 Quake 任务 | **保持关闭** | 用户确认现无可用 key。`quake_ynmobile_a/b/b2` 维持 disabled。补 key 后须再问一次才能 enable |
| B2 BIZ-2 | ZoomEye / Shodan / Censys 是否进日更 | **不进日更** | 用户确认这三家现无可用 key。不新建调度任务；Web 查询页可保留，缺 key 时走既有 fail-fast |
| B3 BIZ-3 | 云端是否打开截图 | **不开** | 维持 `screenshot.enabled=false`，readiness degraded 为预期。低配机不当 CDP 验收 |
| B4 BIZ-4 | FOFA 字段降级是否可接受 | 未拍板，默认保持 | 15:00 日志有字段权限不足后降级，任务仍 success |
| B5 BIZ-5 | 下次升镜像是否 recreate smtp-relay | 未拍板，默认升 unimap 时顺手 recreate | 08-07 容器仍 healthy |
| B6 BIZ-6 | 是否上 TLS/域名 | 未拍板，默认继续 SSH 隧道 | 另开生产化议题 |

### C 波：产品 P1（有代码或真实验收）

#### C1 RW-03 FOFA / ZoomEye / Shodan 原生 CDP 定级

- 状态：**部分完成（2026-08-21 上午）**。FOFA 通过；Shodan 校准后通过；ZoomEye 当时 `.org` 521 / `.ai` SSO 记受限。证据 [CDP_VERIFICATION_FOFA_SHODAN_2026-08-21.md](CDP_VERIFICATION_FOFA_SHODAN_2026-08-21.md)。
- 同日傍晚：本机已登录 Chrome + 插件 0.4.18 活页选择器校准；DayDayMap 活抽 10 条已复验。**不是 CDP 复测。** 现状与下一步：[PLUGIN_CDP_STATUS_2026-08-21.md](PLUGIN_CDP_STATUS_2026-08-21.md)。
- 前置：真实账号与登录态；**不要**在当前云端截图关闭的机器上做。优先本机有 Chrome Profile 的环境。
- 下一步（本机，保持云端截图关闭）：
  1. 用**当前** ExtractJS 再跑 `CollectAndCaptureSearchEngineResult`：先 DayDayMap、ZoomEye（先 `.org`）、Censys；再 FOFA/Hunter/Quake（当天改过 JS）。要非空资产 + 结果页 PNG。
  2. 插件改后活抽：FOFA、Hunter、Censys、ZoomEye host 字段、Quake 假行。DayDayMap 0.4.18 已过。
  3. 不要把插件活抽写成 Bridge/`tasks/next`/SQLite/通知闭环。
- 完成：每引擎日期化证据（通过 / 受限 / 不支持）。登录页、挑战页、空结果不得报成功。更新能力矩阵。

#### C2 RW-05 Cookie/Profile 无人值守真实验收

- 状态：框架已完成，真实验收未做
- 不要在 `handleSaveCookies` 里自动拉 CDP。探测继续用 `GET /api/v1/cookies/login-status`。
- 完成：周期登录检查在真实引擎跑通；失败告警；熔断能跳过坏会话的浏览器任务。

#### C3 RW-09 巡检证据截图云端闭环

- 状态：阻塞于外部输入
- 需要：受控测试域名/子域、可编辑 DNS 的短期凭据或等价接口、fixture 控制 URL 与变化目标 URL。
- 在输入齐备前保持 `evidence_screenshot_enabled=false`。
- 完成：DNS rebinding + 真实页面变化 + 图片送达 + 重启复验，见 [CLOUD_SECURITY_ACCEPTANCE_RUNBOOK_2026-07-29.md](CLOUD_SECURITY_ACCEPTANCE_RUNBOOK_2026-07-29.md)。

Censys 原生 CDP 不作为本波必做：挑战已识别，fallback 已通过。仅当页面策略允许且用户要求时复验。

### D 波：P2，不挡日更

| ID | 事项 | 说明 |
|---|---|---|
| D1 RW-07 | 配额趋势/刷新/告警 | UI 入口保持禁用，直到存储、调度、告警、UI 齐套 |
| D2 CFG-1 | 不可写配置的页面提示 | 生产可写卷已通；只读模式仍无明确禁用保存 |
| D3 AUTH-1 | 非 loopback 空库带外初始化 | 现在是预检拒绝 + 500 `login not configured` |
| D4 AUTH-2 | loopback 首注册后 token 安全持久化 | 08-17 不做「注册展示 token」；若做需单独设计一次展示/下载 |
| D5 OPS-6 | 异机备份恢复演练 | 有创建/列表，无恢复 API；演练用手工恢复 |
| D6 OPS-7 | 数据保留策略 | `history.db` 约 196MB，尚非主因；先清镜像 |
| D7 DOC-1 | README 能力措辞分级 | 已实现 / 模拟通过 / 真实通过 / 阻断 / 未执行 |

## 6. 云端操作备忘（给执行 A 波的 agent）

```powershell
ssh -i "$HOME\.ssh\aliyun_ubuntu.pem" -o IdentitiesOnly=yes root@8.160.177.101
```

机内只读检查（不泄露秘密）：

```bash
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
curl -fsS http://127.0.0.1:8448/health/ready
docker exec unimap-unimap-1 printenv TZ UNIMAP_ADMIN_USERNAME
df -h /; free -h
```

调调度 API 时使用**运行配置**里的 admin token，不要假设 `.env` 一定有效。不要把 token 回传到聊天。

查询语句、公司名单、Webhook 只存在服务器 `scheduler_tasks.json` 与 gitignore 的本地运维文件中，禁止拷进仓库。

## 7. 建议的实际顺序

1. A1 清镜像 — 完成
2. A2 token 对齐 — 完成
3. A4 文档 — 完成
4. B1–B3 拍板 — 完成（Quake/ZoomEye/Shodan/Censys 无 key 不进日更；截图不开）
5. A3 并入下一次镜像升级
6. 有账号再做 C1/C2；有域名再做 C3。云端截图仍关，C1 不要在这台试运行机上做
7. D 波按需，不插队挡日更

## 8. 文档同步清单

完成任一波次后至少更新：

- 本文第 9 节状态板
- `docs/REMAINING_WORK_2026-07-23.md`
- `docs/CHANGELOG.md` 当日条目

若改了云端运行事实：`docs/CLOUD_DEPLOYMENT_TENCENT_2026-08-06.md` 第 0 节。
若改了入口/分支策略：`AGENTS.md`、`CLAUDE.md`、根 `README.md`。

## 9. 状态板

| ID | 事项 | 状态 | 更新 |
|---|---|---|---|
| A1 OPS-1 | 清旧镜像 | 完成：40G 盘 73%→43%，保留 local + bak-20260817 + smtp-relay | 2026-08-20 |
| A2 OPS-2 | admin token 对齐 | 完成：以运行配置为准写回 `.env` 并 recreate unimap；API 200、12 任务 | 2026-08-20 |
| A3 OPS-3 | 镜像提交号 | 未开始（可并入下次升级） | 2026-08-20 |
| A4 OPS-4 | 分支/文档基线 | 完成（本轮文档） | 2026-08-20 |
| A5 OPS-5 | 安全组复核 | 未开始（需控制台） | 2026-08-20 |
| B1 BIZ-1 | Quake 三任务 | 拍板：保持关闭（无 key） | 2026-08-20 |
| B2 BIZ-2 | ZoomEye/Shodan/Censys 日更 | 拍板：不进日更（无 key） | 2026-08-20 |
| B3 BIZ-3 | 云端截图 | 拍板：不开 | 2026-08-20 |
| B4–B6 | FOFA 降级 / smtp-relay / TLS | 未拍板，默认保持 | 2026-08-20 |
| C1 RW-03 | FOFA/ZoomEye/Shodan CDP | 上午：FOFA/Shodan 通过，ZoomEye 受限。傍晚：插件 0.4.18 活页校准（DayDayMap 复验 10 条）。ExtractJS 改后 CDP **未复跑**。下一步见 PLUGIN_CDP_STATUS_2026-08-21.md | 2026-08-21 |
| C2 RW-05 | 会话真实验收 | 框架完成，验收未做 | 2026-08-20 |
| C3 RW-09 | 证据截图闭环 | 阻塞外部输入 | 2026-08-20 |
| D1–D7 | P2 收尾 | 未开始 | 2026-08-20 |

日更业务（FOFA/Hunter/DayDayMap/ICP）**不是待办**。故障时先查调度历史和 `email_agent` 日志，不要当成新功能缺口。

2026-08-25：云端 DayDayMap 已换成核验通过的新 key（后缀 `46e868`）。live 此前仍是耗尽的 `26b816`。证据见 [CLOUD_DEPLOYMENT_TENCENT_2026-08-06.md](CLOUD_DEPLOYMENT_TENCENT_2026-08-06.md) 第 0 节。

2026-08-25：本机邮件通道已用与云端相同的 SMTP 配置测通，用户确认收件。本机跑 `python smtp-relay/relay.py`（`127.0.0.1:8099`），发件/收件只改 `smtp-relay/.env`。云端日更仍走容器 `unimap-smtp-relay`。本机没有调度任务。详见 [smtp-relay/README.md](../smtp-relay/README.md) 与 [CHANGELOG.md](CHANGELOG.md)。
