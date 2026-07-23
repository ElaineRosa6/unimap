# UniMap 云服务器常态化运行准备与协作清单

> 记录日期：2026-07-23。本文不记录 SSH 私钥、密码、Cookie、API Key、管理令牌、通知密钥或其他真实凭据。

## 1. 当前状态

- 阿里云 Ubuntu 24.04 主机已安装 Docker 29.1.3、Docker Compose 2.40.3，并启用 2 GiB 持久化 swap；
- UniMap 容器已经完成一次真实环境条件验收，Web、SQLite、调度、备份、普通网页 CDP headless 截图及重启持久化通过；
- 服务只绑定 `127.0.0.1:8448`，尚未配置正式域名、TLS 反向代理和公网访问；
- 主机为 2 vCPU、1.6 GiB RAM、40 GiB 系统盘，适合轻量验收，不满足完整生产建议基线；
- 当前运行模式是 CDP headless。Bridge 没有部署在线扩展客户端，也没有通过本机真实验收；
- FOFA、Hunter、ZoomEye、Quake、Shodan 的云端 CDP 真实登录会话、结构化采集、结果页截图、SQLite 合并和图片通知闭环均待验收；以前的结构化采集 E2E 使用 Bridge，不能作为 CDP 证据；
- Censys、DayDayMap 当前仅 API 适配与 API 实机验证完成，稳定 UI、Bridge/CDP 抓取与真实测试未完成；
- Cookie 只能视为可撤销的服务端会话，不能承诺永久有效；当前尚未形成自动探针、熔断、告警和安全续期闭环。

## 2. 建议的常态化架构

第一阶段采用 CDP-only，不把 Bridge 作为上线前置条件：

```text
Internet
   |
HTTPS / 域名
   |
反向代理（限流、访问日志、证书续期）
   |
127.0.0.1:8448 UniMap
   |
CDP headless Chromium（持久化 Profile、单会话）
   |
测绘引擎结果页 / 普通网页

持久化卷 ──> 数据、截图、Chrome Profile、日志、备份
监控告警 ──> readiness、任务失败、登录失效、磁盘、内存、备份
异机存储 <── 加密备份与恢复演练
```

只有某个引擎确认无法通过 CDP 稳定采集时，才增加 Xvfb + headful Chromium + Extension Bridge。Bridge 的浏览器和服务需要保持本机 loopback 语义，不应直接把 Bridge API 暴露到公网。

## 3. 需要用户配合提供或确认的事项

### 必需

| 项目 | 用户需要做什么 | 安全交付方式 |
|---|---|---|
| 服务器定位 | 确认当前低配主机只承载轻量任务，或提供至少 4 vCPU / 8 GiB / 80–100 GiB 的正式机 | 只需确认选择，不涉及秘密 |
| 域名与 DNS | 提供拟使用的域名并将 DNS 解析到服务器 | 提供域名即可；DNS 在云控制台操作 |
| 安全组 | 在阿里云控制台确认公网仅开放 SSH 和反向代理所需端口，Chrome CDP/Bridge 端口不得开放 | 用户在控制台操作或授权只读核查 |
| 引擎范围 | 明确正式启用哪些引擎、每个账号允许的频率、额度和测试查询 | 不要在文档中写入账号秘密 |
| 引擎凭据 | 在服务器本机的受限环境文件或管理界面中录入 API Key/Cookie；不要通过聊天、Git、命令输出发送 | 推荐用户直接录入；文件权限设为仅部署用户可读 |
| 测试授权 | 提供无破坏、低额度的测试查询，并确认允许执行真实引擎访问与截图 | 记录查询范围，不记录查询凭据 |
| 通知渠道 | 确认飞书、Webhook 等渠道及接收人，并在服务器本机录入密钥 | 密钥由用户直接录入 |
| 备份目标 | 指定另一台主机或对象存储、保留周期和恢复点目标 | 凭据使用专用最小权限账号 |
| 维护窗口 | 指定允许重启、升级、恢复演练和短暂停机的时间段 | 书面确认窗口 |

### 建议确认

- 预计每天查询次数、每次资产数量、截图数量和并发；
- 截图、查询历史、巡检记录和日志各自保留多少天；
- 登录会话失效后是自动暂停任务，还是允许继续使用 API 结果；
- 是否要求固定公网出口 IP；
- 是否接受每次引擎登录失效后由人工在受控环境更新 Cookie/Profile；
- 证书使用 ACME 自动续期还是组织提供的证书；
- 监控和告警由 UniMap 自身、Prometheus，还是现有云监控承接。

## 4. 我可以在获得上述信息后完成的工作

1. 固化生产 Compose、资源限制、命名卷、健康检查、自动重启和安全环境变量；
2. 配置反向代理、HTTPS、安全响应头、可信代理范围和请求限流；
3. 核查主机监听端口、容器权限、CDP loopback、文件权限和 Docker 日志轮转；
4. 先为五个稳定 Web 引擎中的实际启用项执行真实 API 查询和 CDP `collect_and_capture` 验收；Censys、DayDayMap 在浏览器链路实现前只验收 API；
5. 检查截图是否为真实结果页而非登录页、验证码页或风控页；
6. 验证结构化资产、截图路径、SQLite 历史和通知内容一致；
7. 增加或完善引擎登录探针、连续失败熔断、会话失效告警和恢复状态；
8. 配置定时备份、异机复制、保留策略，并在维护窗口执行恢复演练；
9. 配置磁盘、内存、容器重启、readiness、任务失败和备份失败监控；
10. 执行并发基线、24 小时稳定性观察、重启恢复和发布回滚验收；
11. 更新 Runbook、验收报告和变更记录，给出最终通过或阻断结论。

## 5. Cookie 与 Profile 的运行约束

- 持久化 `/app/chrome-profile`，不得在每次发布时重建；
- 固定 Profile 下保持 `max_sessions=1`，避免 Chromium Profile 锁和会话数据竞争；
- 尽量保持出口 IP、User-Agent、时区、语言和 Chromium 主版本稳定；
- Cookie/Profile 目录等同账号凭据，必须限制权限、加密备份且禁止进入 Git 和日志；
- 不能只检查“Cookie 已配置”，必须定期打开真实结果页并验证登录墙、结果列表和结构化资产；
- 连续登录失败后暂停对应浏览器任务并通知，不能把登录页截图报告为成功；
- Cookie 更新后立即执行一次低额度查询、截图、持久化和通知闭环；
- 优先用官方 API 获取稳定数据，CDP 用于截图、补充采集和交叉验证。

## 6. 分阶段实施和验收

### 阶段 A：基础生产化

- 确认服务器规格和磁盘余量；
- 固化镜像来源与版本摘要；
- 配置域名、TLS、反向代理和安全组；
- 验证只有 SSH、HTTP/HTTPS 按设计暴露，8448、9222、Bridge 端口不公网开放；
- 完成异机备份目标和日志/镜像清理策略。

### 阶段 B：真实引擎闭环

对每个启用引擎分别验证：

1. API 或 CDP 登录状态有效；
2. 执行约定的低额度查询；
3. CDP 打开真实结果页；
4. 结构化资产非空且字段合理；
5. 截图人工确认不是登录页或验证码页；
6. SQLite 查询历史与截图持久化；
7. 通知包含正确明细和图片；
8. 模拟 Cookie 失效，确认任务失败、熔断和告警语义正确；
9. 更新会话后确认能够恢复。

### 阶段 C：运维闭环

- 容器和主机重启后服务、任务、Profile、历史和截图可恢复；
- 备份能够复制到异机，并完成一次隔离环境恢复；
- 执行代表性并发任务和至少 24 小时资源观察；
- 磁盘、内存、OOM、任务失败、登录失效和备份失败均有告警；
- 发布与回滚步骤由 Runbook 实际走通。

## 7. 完全通过标准

以下条件全部满足后，才把云服务器状态从“条件通过”改为“常态化运行通过”：

- 服务器容量符合确认的业务负载，而不只是能够启动；
- 域名、TLS、安全组和反向代理验收通过；
- 所有正式启用的稳定 Web 引擎至少完成一次真实查询与 CDP 闭环；API-only 引擎只按已声明范围验收，不虚报浏览器能力；
- 登录失效不会误报成功，并能通知和恢复；
- 备份有异机副本且恢复演练成功；
- 代表性并发和 24 小时观察没有 OOM、进程泄漏或磁盘失控；
- 监控、告警、升级、回滚和凭据轮换责任人明确。

## 8. 最小下一步

用户只需先确认四件事即可启动下一阶段：

1. 继续使用当前低配服务器做轻量常态化试运行，还是更换正式规格服务器；
2. 第一批要验收的一个测绘引擎；
3. 域名和通知渠道是否已准备好；
4. 是否允许在维护窗口执行真实低额度查询、容器重启和隔离恢复演练。

凭据本身不应直接发送到聊天中。确定范围后，再由用户在服务器本机安全录入，我负责验证配置是否生效以及完整业务闭环是否通过。

## 9. 已确认的试运行决策

2026-07-23 已确认：

- 继续使用当前主机作为轻量常态化试运行机，不据此承诺高并发生产容量；
- 第一批真实验收引擎为 Quake 和 Hunter；
- 域名和通知渠道已经具备，具体非秘密信息及服务器配置仍待核对；
- 允许在约定维护窗口执行低额度真实查询、容器重启和隔离恢复演练；
- 第一阶段继续采用 CDP-only，暂不部署 Bridge。

2026-07-23 本地代码已将生产配置改为可写命名卷：首次启动从镜像内生产模板初始化
`/app/runtime-config/config.yaml`，随后由 Web 设置页使用原子保存更新。该变更尚需在云机执行
“保存—重启—回滚—恢复”验收后，才能作为常态化运行证据。API Key 可在首次初始化时由
服务器环境变量注入；一旦设置页保存配置，解析后的值会写入受限运行卷，后续轮换应通过设置页
或受控修改运行配置完成，不能假定环境变量永久覆盖。Cookie/Profile 和通知凭据同样保存在仅
容器运行用户可读的卷中，且不得回显到日志。

## 10. 管理登录与凭据录入操作

### 10.1 通过 SSH 隧道登录

在正式域名和 TLS 尚未验收前，不要临时公开 8448。Windows PowerShell 使用：

```powershell
ssh -i "$HOME\.ssh\aliyun_ubuntu.pem" `
  -o ExitOnForwardFailure=yes `
  -N -L 18448:127.0.0.1:8448 `
  root@8.160.177.101
```

保持该窗口运行，在浏览器打开 `http://127.0.0.1:18448/login`。该 HTTP 请求只在本机与 SSH 隧道内传输。

在另一个终端查看首次登录所需的非回显部署环境：

```powershell
ssh -i "$HOME\.ssh\aliyun_ubuntu.pem" root@8.160.177.101
cd /opt/unimap-acceptance
less .env
```

只在本人的受控终端查找 `UNIMAP_ADMIN_USERNAME` 和 `UNIMAP_BOOTSTRAP_PASSWORD`，不得复制到聊天、工单或命令参数。登录后进入“账户”页面修改密码。若 bootstrap 密码已不匹配，不直接修改 SQLite，应执行受控密码重置。

### 10.2 Quake 与 Hunter 所需信息

两个引擎均分别准备：

- API Key：用于官方 API 查询和结构化结果；
- Web 登录 Cookie：用于 CDP 打开登录后的搜索结果页、采集 DOM 和截图。

API Key 不能代替 Web Cookie。轻量试运行初始 QPS 使用 1、超时使用 30 秒，Base URL 保持当前代码默认值。

### 10.3 获取 Cookie

1. 在本地受控 Chrome 中正常登录对应引擎并打开一次搜索结果页；
2. 优先使用可信方式导出包含 `name`、`value`、`domain`、`path`、`httpOnly`、`secure` 的 Cookie JSON 数组；
3. 无法导出 JSON 时，在开发者工具 Network 中刷新结果页，从同域 Document/XHR 请求的 Request Headers 复制完整 `Cookie` 头；
4. 不使用 `document.cookie`，因为它无法读取 HttpOnly Cookie；
5. Cookie 只允许通过 SSH 隧道中的 UniMap 页面或服务器本机受限秘密文件录入，不发送到聊天。

### 10.4 录入前检查与当前限制

生产 Compose 已使用 `unimap_config` 可写命名卷，不再把单个配置文件以 `:ro` 挂载。首次部署先检查：

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yaml config |
  grep -E 'runtime-config|UNIMAP_CONFIG_PATH|UNIMAP_CONFIG_TEMPLATE'
docker compose -f docker-compose.yml -f docker-compose.prod.yaml up -d --build
docker compose -f docker-compose.yml -f docker-compose.prod.yaml exec unimap \
  sh -c 'test -w /app/runtime-config && test -f /app/runtime-config/config.yaml && stat -c "%a" /app/runtime-config/config.yaml'
```

预期配置文件权限为 `600`。输出 Compose 展开结果前必须脱敏。不要把秘密直接写入仓库中的
`configs/config.prod.yaml`。云机尚未执行保存、容器重启和恢复验证，因此第一次录入仍应在维护
窗口进行，并先备份 `unimap_config` 卷。

录入顺序为：

1. “设置 → 搜索引擎”启用 Hunter、Quake并录入 API Key；
2. “设置 → Cookie 管理”分别导入 Cookie JSON 或 Cookie Header；
3. 点击 Cookie“验证”和“刷新状态”；
4. “设置 → 通知”录入渠道并发送测试消息；
5. 执行单引擎、低额度 `collect_and_capture`；
6. 检查真实结果页截图、结构化资产、SQLite 历史和通知图片；
7. 重启容器后再次确认 Key、Cookie、通知配置和 Chrome Profile 均可恢复。

### 10.5 可以公开提供与禁止提供的信息

可以在协作记录中提供：域名、通知渠道类型、维护窗口、低额度测试查询和保留周期。

不得提供：API Key、Cookie、Webhook URL、签名 Secret、App Secret、管理密码、管理令牌、Bridge token 或 SSH 私钥内容。
