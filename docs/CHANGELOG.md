# UniMap 变更日志

> 本文件按发生日期保留历史变更。条目中的“后续”“待办”“CI 全绿”和引擎/任务状态仅表示对应日期的事实；当前状态以 README、API、Runbook 和发布说明为准。

---

## [2026-09-04] 当前发布基线与文档同步

- 当前发布基线为 master/e31e03d；本地、origin/master 与 GitHub master 已核对一致。
- 最新 master 提交的 ci、bridge-smoke 和 Docker Build & Push 均成功。
- README、API、架构、使用、Runbook、开发指南和引擎/CDP 状态入口已统一到 Go 1.26.6 与当前日期化状态页。
- 历史计划和验收记录继续保留原始日期事实，并在入口处标明历史快照属性。

## [2026-08-25] 企微单通道可带附件；清空本地测试任务

- WeCom `markdown` / `markdown_v2` / `text` 在正文之后若有 Excel/截图则再发 `file`；`file` 类型先发明文再传文件。无附件时仍只发 markdown。这样 `dijia_01` 单独就能覆盖原来 `dijia_01`+`dijia_01_file` 的分工，两个通道不要再绑在同一任务上。
- 本地 `data/scheduler_tasks.json` 已清空（本机任务只作测试）。云端 12 个日更任务未改。
- 本地 `email_agent` 已接通：`smtp-relay/.env` 配发件箱 `SMTP_USER` 和收件箱 `MAIL_TO`（可多个），`python smtp-relay/relay.py` 监听 `127.0.0.1:8099`；UniMap 渠道指向该地址且 `allow_private_ip=true`。改发件/收件后重启 relay。
- 2026-08-25 本机 smtp-relay 使用与云端哈希一致的 SMTP 账号直发 webhook 测试信（标记 `local-smtp-verify-105439`）；用户确认收件箱已收到。当时本机 UniMap `:8448` 未运行，未走 Web 渠道测试接口。

## [2026-08-25] 云端轮换 DayDayMap API key

- 本机对官方 `POST /api/v1/raymap/search/all` 核验新 key（后缀 `46e868`）：`domain=` / `org=` / `ip=` 均 HTTP 200 且 `检索成功`；`org="中国移动通信集团云南有限公司"` total=156，不再踩旧 key 的企业查询试用限次。
- 云端 live 配置仍是已耗尽的 `26b816`（08-10 备份 `config.yaml.bak-ddm-swap-20260810102837` 还在，但 live 未保持当时换上的 `c163fc`）。已备份 `config.yaml.bak-ddm-swap-20260825094938`，写入新 key，`POST /api/v1/config` 热加载适配器（未 recreate 容器）。
- UniMap `domain="ynsxxcy.com"` 小查询：HTTP 200、status=success、engineStats daydaymap=1、errors 空。`/health/ready` 仍 ok。未跑日更任务、未打开截图。

## [2026-08-21] 插件 0.4.18 活页核验与文档

- DayDayMap 在用户已登录 Chrome、插件 0.4.18 上复抽：`tr.ant-table-row` 10 条，IP 为纯 IPv4，总数 2,163,417,935，非登录墙。
- 插件活抽与 CDP `CollectAndCapture` 分开记录；今天改过的 ExtractJS **尚未**再跑 CDP。下一步见 [PLUGIN_CDP_STATUS_2026-08-21.md](PLUGIN_CDP_STATUS_2026-08-21.md)。

## [2026-08-21] 七引擎插件活页选择器校准

- 在已登录的本机 Chrome 上用插件 `scripting` 对七个引擎结果页做了 DOM dump（插件 `0.4.16`）。
- FOFA：`.hsxa-meta-data-item` 仍命中；`span.hsxa-host` 现为纯 IP，端口在 `.hsxa-port`；不再把 host 写成 IP。
- Hunter：`.q-table tbody tr` 仍命中；丢掉 ICP 垃圾行；`443 tls` 只保留协议 `tls`。
- ZoomEye：`.org` 今日可开；卡片仍是 `.search-result-item-container`，但 SPA 需等 loading 结束；`hostname:port` 进 host 而不是 ip。去掉会误伤顶栏的 `[class*='result'] > div`。
- Quake：英文结果页仍是 `.item-container`，侧栏聚合块不再当资产；host `--` 视为空。
- Shodan：`.l-search-results .result` 继续 10 条；hostname 与 IP 去重。
- Censys：CSS module 没有 `result-card`，改走 `a[href*='/hosts/']`。查询语法需 `host.services.port:"80"`。
- DayDayMap：登录后活页为 Ant Design `tr.ant-table-row`（约 21.6 亿条）；原先 `table tbody tr` 先命中空的 `ant-table-measure-row` 抽出 0 条。插件/Go 改为 `.ant-table-row` + 列抽取，空结果页会先回首页再填检索。IP 单元格含「视频监控设备+0」时只保留 IPv4；总数按「共有 N 个」解析。
- Extension 与 Go `ExtractJS` 同步；活页报告在 `data/selector-audit-2026-08-21/`（不入库）。

## [2026-08-21] Shodan 结果页选择器校准

- 当前 Shodan 搜索页是 `.l-search-results > div.result`（不再有 `.row`），host 链接为 IPv6 `/host/2600:…` 与 `http://[v6]:80`，总数在 `h4.total-results`。
- 原 ExtractJS 把 `/host/` 写成 Go raw string 里的正则，生成的 JS 非法，10 张卡片抽出 0 条。改为字符串解析 IPv4/IPv6；Extension `capture.js` 同步。
- 复测 CDP：Shodan DOM 10 条、total≈517 万、结果页 PNG。FOFA 仍为 10 条。ZoomEye 仍因站点/SSO 受限。

## [2026-08-21] FOFA/Shodan 本地 CDP 定级

- 在本机 CDP Chrome（:9222）走 `CollectAndCaptureSearchEngineResult`：FOFA 10 条结构化资产 + 结果页 PNG，定级通过。Shodan 已登录并打开搜索页（侧栏约 516 万），DOM 抽出 0 条，定级受限。ZoomEye 按站点问题记受限（`.org` 521，`.ai` 登录回调超时）。证据见 [CDP_VERIFICATION_FOFA_SHODAN_2026-08-21.md](CDP_VERIFICATION_FOFA_SHODAN_2026-08-21.md)。
- 未改云端调度，未打开云端截图。

## [2026-08-20] 云端 A 波：清镜像与 admin token 对齐

- A1：删除未在用的旧 UniMap 镜像（08-11 备份、08-02 browser-seven/acceptance/ACR 重复标签）及悬空层；`docker builder prune` 清悬空构建缓存。根分区 40G 从 73%（剩 11G）降到 43%（剩 22G）。保留 `unimap:local`、`unimap:local-bak-20260817`、`unimap-smtp-relay:latest` 及构建底包。
- A2：以运行配置 `web.auth.admin_token` 为准覆盖 `/opt/unimap/.env` 的 `UNIMAP_ADMIN_TOKEN`（先写 `.env.bak-token-*`），`--no-deps` recreate `unimap-unimap-1`。对齐后容器环境变量与配置一致，`GET /api/v1/scheduler/tasks` 200、12 任务（9 启用 / 3 Quake disabled）仍在。smtp-relay 未重建，仍 healthy。
- 未做：A3 提交号打进镜像（并入下次升级）；A5 安全组控制台复核。
- B1–B3 用户拍板（2026-08-20）：Quake 三任务因无 key 保持关闭；ZoomEye / Shodan / Censys 因无可用 key 不进日更；云端截图继续关闭。未改云端任务或截图配置。

## [2026-08-20] 试运行范围拍板

- 日更固定为 FOFA / Hunter / DayDayMap 查询（`email_agent`）+ ICP（企微）。Quake 三个任务维持 disabled；不把 ZoomEye、Shodan、Censys 写入调度；`screenshot.enabled` 保持 false。
- 后续 agent 在补齐 key 并再次确认前，不得自行启用上述引擎任务或打开云端截图。详见 [推进计划书](AGENT_CONTINUATION_PLAN_2026-08-20.md) B 波。

## [2026-08-20] 云端机内核查与后续推进计划

- SSH 核对阿里云试运行机：`unimap-unimap-1` / `unimap-smtp-relay` 均为 healthy；运行 git `71371f1`；8448 仅 loopback；截图关闭导致 readiness `screenshot=degraded` 为预期。
- 9 个启用调度任务在 08-17 升级后至 08-20 15:00 共 52 次 success、0 failed（FOFA/Hunter/DayDayMap 查询邮件 + ICP 企微）。Quake 三个任务仍 disabled。
- 发现运行配置 admin token 与 `.env` 不一致（环境变量调 API 401）；根分区约 73%，旧镜像待清理。
- 新增后续 agent 执行入口 [AGENT_CONTINUATION_PLAN_2026-08-20.md](AGENT_CONTINUATION_PLAN_2026-08-20.md)；`REMAINING_WORK`、README、`AGENTS.md`/`CLAUDE.md` 改为发布基线跟踪 `master`，08-02 清单降为历史快照。

---

## [2026-08-17] 本地审查收口与阿里云镜像升级

- 示例配置 Redis `prefix` 改为带引号，`config.yaml.example` 可通过 `Manager.Load` 解析。
- Quake / Hunter / FOFA / Shodan `Search`：主备 Key 都空时本地失败、不发起 HTTP；空主键不再占用一次请求，只配备用 Key 时直接使用备用 Key。
- 用户库已有账号后，未知用户名登录返回 401；loopback 冷启动仍为 409；非 loopback 空库仍为 500 `login not configured`。
- `GET /api/v1/config` 的 `notifications.channels` 只返回公开字段，不再带出 webhook URL / 签名密钥。
- 文档与代码对齐：稳定 Web UI 为七引擎；`cmd/unimap-gui` 仍在（`gui` build tag）。
- 阿里云试运行机：运行配置中的默认 `web.auth.username=admin` 已改为非默认名（与 `UNIMAP_ADMIN_USERNAME` 对齐），以满足非 loopback `StartupPreflight`；随后将 `/opt/unimap` 与镜像 `unimap:local` 快进到 `71371f1` 并 recreate `unimap-unimap-1`。`/health/ready` 为 ok，12 个调度任务定义保留。升级前当日 08:00–09:40 的启用任务均为 success；升级后下一档为 15:00。旧镜像标签 `unimap:local-bak-20260817`。
- 仍不实施：Load 失败即退出、非 loopback 首管理员落库、注册时展示 token、保存 Cookie 时自动 CDP 探测、`/screenshots/` 强制登录、证据截图、恢复 API、配额趋势/告警、按事件分渠道。

---

## [2026-08-06] 七引擎字段不丢失修复与真实 API 复验

- 修复引擎字段丢失：各适配器结构体新增 `Extra` 捕获（`rawUnknown` + 自定义 `UnmarshalJSON`），未声明顶层键不再被 Go JSON 静默丢弃；`promoteLastSeen` 从 `lastupdatetime`/`time`/`updated_at`/`time_stamp` 等键统一提升资产 `last_seen`。覆盖 fofa/hunter/quake/zoomeye/daydaymap/censys/shodan。
- FOFA 特判：补齐 `lastupdatetime` 等 10 字段；`fofaFieldSets` 增至 4 档，新增「无 icon_hash 的 23 字段」降级档，规避真实代理 icon_hash 权限 820001 导致的字段丢失。
- 浏览器 DOM 路径同步：采集解析保留扩展字段 `Extra`，`os`/`product` 纳入扩展字段，`product→title` 兜底；Hunter 扩展 JS 补 ICP/App/tags/last_seen 列。
- 合并与对象池保留/重置 `LastSeen`，多引擎合并不再丢时间戳。
- 新增 `real_verify` build tag 真实 API 复验测试（`go test -tags real_verify -run TestRealVerify -v ./internal/adapter/`）：fofa/quake/daydaymap 实测 `last_seen` 真实非空，extra 保留 26–32 个引擎字段；hunter 当日限流、zoomeye 积分不足、censys 缺 `api_secret` 待复验。
- 全量 `go test -race ./...`、`go vet ./...`、`go build ./...` 通过。

---

## [2026-08-02] 七引擎、云端发布与通知收口

- 稳定 Web UI 扩展至 FOFA、Hunter、ZoomEye、Quake、Shodan、Censys、DayDayMap；七引擎 Bridge 真实结构化采集均非空。
- API/Browser 查询完成 `success`、`partial`、`error` 语义和单次 SQLite 合并持久化；Browser 非空不再被 API 失败覆盖。
- Bridge 凭据交接支持类型化 Cookie 与同源 Web Storage；DayDayMap 原生 CDP 通过，Censys challenge 被识别并自动回退 Bridge。
- guarded browser egress 限定 literal-loopback SOCKS5 和固定公网 IP:port；Extension 升至 0.4.15。
- 最终 CGO/SQLite 云端镜像通过健康、readiness、重启、旧镜像回滚、再次发布和配置哈希验证；飞书应用通知连续两次返回 HTTP 200，通知配置回滚与重应用通过。
- Extension 9/9、`go test -race ./...`、`go vet ./...`、`go build ./...`、前端语法和 `git diff --check` 全部通过。
- 自动巡检证据截图继续默认关闭，待受控域名、DNS 编辑参数及 fixture 控制/目标 URL 到位后完成最终 live E2E。

## [2026-07-24] 巡检运行时、历史筛选与证据截图安全边界

- 将根目录临时 Chrome 测速入口迁移到 `tools/chrome-speedtest`，使用
  `manual_browser_test` build tag、显式 headless 和单浏览器顺序导航；默认 build/test 不再纳入该工具。
- `TamperAppService` 每次构造 Detector 时读取最新提交的端口扫描、TLS 和超时配置，删除 Web
  handler 内未被业务路径使用的重复构造逻辑。
- 定时基线刷新复用截图管理器 allocator，并按 `saved` 摘要判定真实成功，避免初始化失败仍误报刷新成功。
- 巡检历史新增包含边界的 `start_time`、`end_time`，列表和导出保持一致；`count` 改为过滤后真实总数，
  URL 选项也遵循时间范围。
- 独立审查确认浏览器层尚不能抵御跨主机重定向和 DNS rebinding；因此未启用自动变化证据截图，
  该能力继续保留为待办，避免把调用前检查误当作完整 SSRF 防护。
- 全量 race、vet、默认/GUI build、手动工具仅编译和前端语法检查通过；真实云端页面变化与
  飞书图片送达仍需受控验收。
- 上述代码与第一轮文档变更已提交为 `f9317a3`；截至本次核对位于本地 `develop`，尚未推送
  `origin/develop`。
- 同步当前待办、实施计划、`CLAUDE.md` 与项目 memory，明确 Bridge 历史证据不能替代 CDP
  真实闭环证据。

---

## [2026-07-23] 审计跟进与生产部署契约修复

- 在阿里云 Ubuntu 24.04 主机完成真实容器条件验收，报告见 `docs/CLOUD_ACCEPTANCE_2026-07-23.md`。
- 补记验收边界：普通网页 CDP 截图已通过，真实测绘引擎 CDP `collect_and_capture` 与 Extension Bridge 尚未在该主机验收；新增 `docs/CLOUD_STEADY_STATE_PLAN_2026-07-23.md` 记录常态化运行分工和门槛。
- 纠正引擎能力记录：Censys、DayDayMap 仅 API 适配与 API 实机验证完成，稳定 UI、Bridge/CDP 抓取和 live E2E 未完成；历史测绘数据抓取证据来自 Bridge，不能作为 CDP 通过证据。
- 新增 `docs/REMAINING_WORK_2026-07-23.md`，统一记录七引擎支持矩阵、生产配置、CDP 实测、Cookie 生命周期、UI 和发布清理待办。
- 现场修复单 URL 截图空 ID、生产资源硬编码、版本提交不可追踪、通知 pepper 缺失、Compose 废弃 version 警告以及 logs/backups 非 root 不可写问题。
- 实测通过 CGO/SQLite、CDP headless、单图、2/2 异步批量截图、security 巡检、调度备份、CSRF 登录、备份异机复制和容器重启持久化。
- 修正生产 Compose 合并语义：以 `!override` 完整替换基础端口和卷，避免公网 `8448` 与开发 `web` bind mount 在合并后残留；最低要求 Docker Compose 2.24.4。
- 新增 `configs/config.prod.yaml`，让固定 Web/分布式管理令牌与非默认管理员名真正从部署环境解析。
- 生产配置由只读单文件挂载改为 `unimap_config` 可写命名卷；入口脚本只在首次启动时初始化 `0600` 配置，支持设置页原子保存与重启持久化。
- 生产 Compose 支持 `UNIMAP_IMAGE` 预构建镜像；运行主机可固定 ACR 摘要并使用 `pull` + `up --no-build`，不再依赖 Docker Hub 构建。
- 未配置引擎不再被默认强制启用；Censys、DayDayMap 缺少完整 API 凭据时不再注册不可执行的 Web-only adapter。
- 开发 Docker 基线显式启用五个稳定 Web 引擎，避免移除隐式默认后出现兼容性回退。
- 生产模板显式启用低 QPS 的 Quake/Hunter 并透传可选 API Key；设置页 Hunter/Quake Base URL 提示与适配器默认值统一。
- 查询状态与结果页改用 DOM 节点和 `textContent` 构造，移除审计点名的动态 `innerHTML` 注入面。
- 补齐篡改历史导出的组合过滤、存储错误和错误方法回归测试；持久化审计 Finding #8 闭环。
- 本机仍无 Docker CLI，Compose 渲染、镜像运行、TLS、cgroup 和异机备份留待正式云机验收。

---

## [2026-07-20] 云服务器部署评估记录

> **变更类型**: 部署评估 + 文档纠偏
> **涉及模块**: Docker、Compose、SQLite、headless Chrome、Runbook、文档索引

### 核查结论

- 确认无图形 Linux 可使用 CDP headless，不需要桌面、X11、VNC 或 GPU；持久化 Chrome profile 继续强制截图会话串行。
- 确认当前 `Dockerfile` 的 `CGO_ENABLED=0` 与 `go-sqlite3` 不兼容。相同构建条件下，`internal/auth`、`internal/history`、`internal/screenshot/batchdb` 的数据库测试均因 CGO stub 失败；该问题记录为容器生产发布阻断，尚未在本次文档变更中修复。
- 现有 Compose 的 2 CPU / 1 GiB 限制不再表述为生产推荐值；单机完整功能建议从 4 vCPU、8 GiB RAM、80–100 GiB SSD 起步，并在真实云机压测后调整。
- 补充生产 Compose 差距：开发静态目录挂载、8448 直接公开、`/app/backups` 未持久化、只读配置的运行语义、资源限制可能未被平台实现以及日志/TLS/异机备份要求。
- 补充固定管理令牌、引擎真实凭据、Web-only adapter 边界、端口与 `CAP_NET_RAW` 权限、磁盘估算和正式云机十二项验收清单。

### 文档

- 新增 `docs/CLOUD_DEPLOYMENT_ASSESSMENT_2026-07-20.md`。
- 更新 Runbook 云主机章节，阻止在 CGO/SQLite 修复前误用当前镜像投产。
- 更新文档索引。

### 验证边界

- 已执行 `CGO_ENABLED=0 go test ./internal/auth ./internal/history ./internal/screenshot/batchdb -count=1`，预期复现并确认发布阻断。
- 本机没有 Docker CLI，本轮未执行 Compose 渲染、镜像构建或真实云机验证。

---

## [2026-07-17] 逻辑、API 与前后端交互修复闭环

> **变更类型**: 可靠性修复 + API 契约适配 + Web UX 完善
> **涉及模块**: config、CLI/GUI、screenshot、scheduler、distributed、users/session、query cache、notifications、Web templates、文档
> **提交**: `d8ad66e`、`f89f642`、`80350dc`

### 核心变更

- 完成 14 项代码逻辑/API/体验问题修复：配置 copy-on-write、CLI 认证和调度契约、异步截图、调度与分布式持久化回滚、会话撤销、首管理员原子创建以及查询缓存元数据。
- 补齐批量截图 timeout/failed 明确终态、ICP 对象错误解析、账号角色更新与删除错误处理、调度历史响应校验。
- 通知编辑改为安全 DTO：列表不返回 Webhook URL、secret 或 app secret；`preserve_existing=true` 可原子保留留空凭据，且禁止直接改变渠道类型。
- API、Runbook、问题报告、修复/回滚指南和文档索引同步到同一实现语义。

### 验证结果

- `go test -race ./...`、`go vet ./...`、`go build ./...` 通过。
- GUI build/test、live Bridge E2E 编译检查和 `node --check web/static/js/main.js` 通过。
- `govulncheck ./...` 未发现当前代码调用的已知漏洞；外部引擎真实凭据、远程扩展和生产 Redis 留待上线受控验收。

---

## [2026-07-15] 持久化与查询/调度可用性完善

> **变更类型**: 可靠性回归覆盖 + API 易用性增强
> **涉及模块**: alerting、ICP database、scheduler、web、API 文档
> **提交**: `4278a3f`、`a982189`、`1a32f46`

### 核心变更

- 核实 Alert JSON 已使用 `.tmp` 写入后 `os.Rename` 原子替换，补充替换完成后不残留临时文件的回归断言。
- `/api/v1/icp/history?keyword=...` 支持字面量部分关键词匹配；LIKE 通配字符会被转义，`type` 与 `task_id` 仍按精确值匹配。
- 调度任务创建和更新在落库前按 Runner 类型校验必填字段，400 响应包含任务类型、规范字段和可用别名。
- 调度数组字段兼容逗号分隔字符串；`urls` 兼容 `targets`，`engines` 兼容 `engine`，旧 `extra` 查询字段仍可读取。
- Runner 执行期的缺字段错误统一包含 Runner 类型；API payload 速查同步更新。

### 验证结果

- `go test -race ./...` 通过。
- 三项改动分别提交并推送至 `develop`。

---

## [2026-07-15] 端口扫描准确性与多协议探测升级

> **变更类型**: 功能增强 + 准确性修复
> **涉及模块**: service/monitor、scheduler、web、templates、API、Runbook

### 核心变更

- 将去重后的 `唯一 IP × 端口` 笛卡尔积随机打乱后再提交到有界工作池，避免固定的 IP/端口顺序。
- 保留完整 TCP connect，并新增 Telnet IAC 协商探测。
- 新增 FIN、NULL、Xmas（FIN/PSH/URG）原始 IPv4 TCP 探测；强制要求显式 `authorized_targets`，运行环境需要管理员/root 或 `CAP_NET_RAW`。
- 新增 UDP 探测和 TCP/UDP 多方法混合扫描，DNS、NTP、SNMP 使用对应协议载荷。
- 新增每次探测前的 `jitter_min_ms` / `jitter_max_ms` 随机抖动，范围限制为 0-5000ms。
- 响应新增结构化 `findings`。只有完成 TCP 连接或收到有效响应的结果才标记为 `open` 并进入 `open_ports`；UDP 或 FIN/NULL/Xmas 无响应标记为 `open_filtered`，避免误报确定开放。
- Web 页面、端口扫描 API 和 `port_scan` 定时任务均支持 `probe_methods` 与抖动参数。

### 安全与兼容边界

- 继续执行公网 IPv4、SSRF、CDN 排除和可选授权 CIDR 校验，原始扫描不会放宽任何边界。
- 未传 `probe_methods` 时保持原有 TCP connect 行为。
- `planned_connections`、`attempted_connections` 和目标级预期次数按 `IP × 端口 × 方法` 统计。
- `reports/` 为本地生成报告目录，加入 `.gitignore`，不进入版本控制。

### 验证结果

- `go test -race ./...` 通过。
- `go vet -mod=readonly ./...` 通过。
- `go build -mod=readonly -buildvcs=false ./...` 通过。
- `git diff --check` 通过。

---

## [2026-06-30] 巡检功能逻辑缺陷修复与业务完善

> **变更类型**: Bug 修复 + 业务完善
> **涉及模块**: tamper、scheduler、service/monitor、web、templates
> **提交**: `2bf5fe5` (develop) — PR #11 CI 全绿（ci + bridge-smoke，ubuntu/macos 双平台）

### 背景
对巡检子系统（篡改检测 + URL 可达性 + 批量截图 + 端口扫描 + ICP 巡检等定时监控任务）做全量审计，发现 4 个确认 Bug 与 4 个完善性缺口。

### 修复（Tier 1 — Bug）

- **篡改检测模式被强制降级**：service 层 `if mode != strict { mode = relaxed }` 把 `security/balanced/precise` 静默降级为 `relaxed`，UI 与定时任务的 5 模式选择形同虚设。导出 `NormalizeDetectionMode`，service 层改用它，5 模式全部透传。
- **每日篡改模板无效模式**：`tmpl_daily_tamper_check` 的 `DetectMode: "full"` 不是合法检测模式，会被降级。改为 `"relaxed"`。
- **定时篡改巡检不渲染 JS**：`TamperCheckRunner` 注册时传 nil allocator，只能走 HTTP/Fast 模式，SPA/JS 页面拿空 hash → 误报"篡改"或"不可达"。从 `screenshotMgr` 注入 allocator（nil 时保持原行为）。
- **SSRF 拦截的 URL 未计入可达性统计**：汇总 switch 缺 `blocked` 分支，有 SSRF 拦截时 `Total ≠ Reachable+Unreachable+InvalidFormat`。新增 `Blocked` 字段与分支，`Total = Reachable+Unreachable+InvalidFormat+Blocked`。

### 完善（Tier 2）

- **基线刷新并发化**：`BaselineRefreshRunner` 原逐 URL 串行 `SetBaseline`，忽略 payload 的 `concurrency`。改为信号量 + WaitGroup 并发执行。
- **"查看基线" UI 入口**：`monitor.html` 新增 `btn-list-baseline` 按钮，绑定既有的 `loadBaselineList()`。
- **历史记录类型过滤补全**：`monitor.html` 的 `history-type-filter` 追加 `no_baseline/unreachable/suspicious` 三个选项。
- **scheduler 帮助文档模式修正**：`scheduler.html` 的 `PARAM_HINTS.tamper_check` 由错误的 `malicious/performance/full` 改为 `security/balanced/precise`。

### 涉及文件

| 文件 | 变更 |
|------|------|
| `internal/tamper/detector_types.go` | 导出 `NormalizeDetectionMode` |
| `internal/tamper/detector_test.go` | 模式用例补全 security/balanced/precise |
| `internal/service/tamper_app_service.go` | 模式透传（调用 `NormalizeDetectionMode`） |
| `internal/service/monitor_app_service.go` | 抽取 `summarizeReachability` + 新增 `Blocked` 桶 |
| `internal/service/monitor_app_service_test.go` | 汇总不变量单测（新增） |
| `internal/scheduler/scheduler_types.go` | 模板模式 `full` → `relaxed` |
| `internal/scheduler/executor_runners2.go` | 基线刷新并发化 |
| `web/server.go` | 定时篡改 allocator 注入 |
| `web/templates/monitor.html` | 查看基线按钮 + 历史过滤补全 |
| `web/templates/scheduler.html` | 帮助文档模式修正 |

### 列为后续（架构级，本次未修）
可达性/端口扫描告警记录缺失；告警与资源监控仅存内存重启丢失；`operation_history` 无内部写入者；定时批量截图未进 batchdb；篡改导出器死代码；无定时备份任务；篡改记录文件式无索引。

### 验证结果
- `go build ./...` 通过
- `go test ./internal/tamper/... ./internal/service/... ./internal/scheduler/...` 全部通过
- `go vet` + `gofmt` 干净
- GitHub Actions CI 全绿

---

## [2026-06-11] Bridge 状态语义修复 + Token 复制 + 状态抖动

### 修复

- **Bridge/CDP 状态语义统一**：`ExtensionHealthChecker` 要求 `LiveClient` 返回 true 才报告健康；`buildBridgeDiagnosticSnapshot` 新增 `extension_online` 字段；设置页不再将"服务启动"显示为"在线"，改为三态：在线 / 等待扩展连接 / 离线。
- **Bridge 状态抖动修复**：`liveWindowSeconds` 从 15 秒提高到 60 秒，覆盖扩展执行任务期间不轮询的场景。
- **Account 页 Token 复制修复**：`GET /api/v1/account/admin-token` 返回真实 token（接口已受 auth 保护），不再返回脱敏值。

### 涉及文件

| 文件 | 变更 |
|------|------|
| `internal/screenshot/health.go` | `LiveClient == nil` → 返回 `false` |
| `internal/screenshot/router.go` | `extHealthy` 初始 `false`；`LiveClient` nil 也赋值 |
| `web/server.go` | `LiveClient` 回调改用 live token 检查 |
| `web/screenshot_bridge_handlers.go` | 新增 `extension_online`；`liveWindowSeconds` 15→60；提取共享函数 |
| `web/query_handlers.go` | admin token API 返回真实 token |
| `web/templates/settings.html` | Bridge 状态三态显示 |

### 测试

- 新增 7 个测试 + 修复 4 个已有测试适配新语义
- `go test -race ./internal/screenshot/... ./web/...` 全部通过

---

## [1.0.0] - 2026-05-26 生产级就绪正式版发布 (Major Upgrade)

> **变更类型**: 核心架构升级 & 正式发布
> **涉及模块**: CLI / Web UI / GUI / 核心引擎架构 / 全量系统

### 🎉 新增功能
- **全系引擎全量支持**: 深入集成并解耦 FOFA / Hunter / ZoomEye / Quake / Shodan 五大搜索引擎底座。
- **Screenshot 截图容灾**: 引入核心高可用 ScreenshotRouter 组件，实现 CDP 与 Extension 双擎自动探测及故障切换备份策略。
- **多端业务矩阵**: 重构分离出独立的 Web API 守护进程、CLI 终端查询工作流及 GUI 图形桌面端，业务矩阵全面闭环。
- **智能化篡改防护 (Tamper Detection)**: 新增网页动态区域特征隔离算法及针对不同颗粒度防护的五模监控匹配引擎。
- **分布式调度阵列 (Scheduler Node Cluster)**: 搭建基于 LRU/Redis 持久化的轻量级多任务心跳管理及 Task 自动分发阵列（涵盖20种专属定时检查 Runner）。
- **企业级告警通道**: 原生实现频率风暴限制、重复异常静默期管理的 Log/Webhook 推送集成。

### 🔄 优化与重构
- 统领 UQL（Unified Query Language）抽象语法字典及各引擎底层的自动转译机制。
- 完整脱敏开源仓库信息，统一合规及安全审查路径，升级并补全全部产品技术手册文档库 (docs/)。

---

## [2026-05-07] 引擎加载问题修复 — 空间搜索引擎默认启用

> **变更类型**: Bug 修复
> **涉及模块**: config、cmd/unimap-web

### 问题描述
当用户输入查询语句执行查询时，由于未加载空间引擎，无法执行查询。

### 问题根源
在 `internal/config/config.go` 中的 `applyDefaults` 函数未设置引擎的 `Enabled` 字段默认值，导致所有引擎默认都是禁用状态。

### 修复方案
修改 `internal/config/config.go` 中的 `applyDefaults` 函数：
- **默认启用所有搜索引擎** — Quake、ZoomEye、Hunter、FOFA、Shodan
- **设置 FOFA 为 Web 模式为默认** — `UseWebAPI = true`，即使没有 API Key 也能使用 Web 模式
- **保持验证函数的完整性** — 非 Web 模式下仍需验证 API Key

### 修改的关键代码
```go
// 默认启用所有引擎
config.Engines.Quake.Enabled = true
config.Engines.Zoomeye.Enabled = true
config.Engines.Hunter.Enabled = true
config.Engines.Fofa.Enabled = true
config.Engines.Shodan.Enabled = true
config.Engines.Fofa.UseWebAPI = true
```

### 验证结果
- `go build ./...` — 构建成功
- `go test -v ./internal/config/...` — 所有测试通过（0 failures）

---

## [2026-04-29] 第三轮安全修复 — 24 项 Code Review 剩余问题修复

> **分支**: `release/major-upgrade-vNEXT`
> **变更类型**: 安全修复 + 代码质量提升
> **涉及模块**: scheduler、auth、config、backup、distributed、web、cmd/unimap-cli

### 一、Webhook SSRF 防护（DNS/重定向绕过修复）

- **DNS 解析校验** — `ValidateWebhookURLPublic` 增加 `net.LookupIP` 校验所有解析 IP，阻止私有/回环地址 (`internal/scheduler/scheduler.go`)
- **安全 HTTP Client** — `safeWebhookClient()` 拒绝所有重定向 + 自定义 DialContext 阻止连接私有 IP (`internal/scheduler/scheduler.go`)

### 二、API Key 认证体系修复

- **SHA-256 hash 比对** — 引入 `KeyHash` 字段，`ValidateAPIKey`/`UpdateLastUsed` 改用 hash 比对，解决持久化后密钥不可验证问题 (`internal/auth/api_key.go`)
- **统一 map key 策略** — `GenerateAPIKey` 改用 ID 为 map key，`loadFromStorage` 使用 ID 保持一致 (`internal/auth/api_key.go`)
- **expiresIn=0 永不过期** — `zeroOrExpiry()` 将 0 转为 `IsZero` 时间 (`internal/auth/api_key.go`)
- **auth 平行认证接入** — Server 结构体新增 `apiAuth` 字段；路由层接入 `OptionalAPIKey` 中间件，API Key 认证体系正式接入主路由 (`web/server.go`, `web/router.go`)

### 三、分布式任务队列持久化

- **JSON 快照持久化** — 新增 `NewTaskQueueWithPath()` + `saveLocked()`/`loadSnapshot()`，所有状态变更（Enqueue/Claim/SubmitResult/Delete）自动保存 (`internal/distributed/task_queue.go`)
- **Server 接入** — TaskQueue 初始化传入 `./data/distributed_tasks.json` 路径 (`web/server.go`)

### 四、备份系统错误处理

- **Source 失败累积错误** — 静默跳过的 source 错误累积并在无有效文件时返回错误 (`internal/backup/backup.go`)
- **Tar 写入失败部分成功** — 累积 tar 错误，最终返回时包含失败统计 (`internal/backup/backup.go`)
- **BackupConfig 注释修正** — 注释改为"绝对路径或相对路径" (`internal/backup/backup.go`)

### 五、限流与安全头

- **X-Real-IP 代理检查** — 和 XFF 一样加入 `isPrivateOrInternalHost` 检查，防止可伪造代理头 (`web/middleware_ratelimit.go`)
- **CORS 默认 X-Admin-Token** — config 层默认 AllowedHeaders 添加 `X-Admin-Token` (`internal/config/config.go`)

### 六、可用性与安全性

- **Admin Token 非loopback 提示** — 非回环绑定打印 token 并提示保存 (`internal/config/config.go`)
- **CLI JSON 防覆盖** — `writeJSONFile` 改用 `O_EXCL`，与 CSV 一致 (`cmd/unimap-cli/api_subcommands.go`)

### 七、测试修复

- **TestGenerateAPIKey/zero_expiration** — `expiresIn=0` 返回 `IsZero` 时间，测试逻辑更新
- **TestUpdateLastUsed** — 改为按 ID 查找验证（map key 策略变更）
- **config 测试** — 默认 CORS headers 预期更新

### 验证结果

- `go build ./...` — 构建成功
- `go test -race ./...` — 全部通过（0 failures, 0 races）

---
