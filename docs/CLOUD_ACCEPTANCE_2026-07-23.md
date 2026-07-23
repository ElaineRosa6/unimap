# UniMap 阿里云真实环境验收（2026-07-23）

> 验收主机：阿里云 Ubuntu 24.04.4 LTS、x86_64、2 vCPU、1.6 GiB RAM、40 GiB 系统盘。报告不记录 SSH 密钥、管理令牌、密码、Cookie 或公网登录细节。

## 结论

本次验收为**条件通过**：固定提交 `bce3ee3` 的容器构建、CGO/SQLite、CDP headless、单图、异步批量截图、网页巡检、调度、备份、登录和容器重启持久化均通过；但该主机低于生产容量基线，真实搜索引擎查询、TLS 反向代理、阿里云安全组和高负载容量仍需另行验收。

## 当前能力边界补记

截至 2026-07-23，本报告中的“CDP headless、单图和异步批量截图通过”只证明云端 Chromium 可以稳定打开并截图普通公网网页，**不等于真实测绘引擎的 CDP 查询闭环已经通过**。

| 能力 | 当前状态 | 说明 |
|---|---|---|
| 普通公网网页 CDP 截图 | 已通过 | 已在无图形容器内验证单图及 2/2 异步批量截图 |
| 五个稳定引擎结果页 CDP 截图 | 待验收 | FOFA、Hunter、ZoomEye、Quake、Shodan 有 CDP 代码路径，但尚未使用有效登录会话实测；以前的数据抓取 E2E 使用 Bridge |
| 五个稳定引擎 CDP 结构化采集与截图闭环 | 待验收 | `browser_query=true`、`browser_action=collect_and_capture` 尚未在真实引擎完成采集、截图、SQLite 和通知闭环 |
| Censys/DayDayMap API | 已有历史实机证据 | 仅限服务端/CLI API 适配；本次云机没有重新执行 |
| Censys/DayDayMap UI、Bridge、CDP | 未完成 | 未进入稳定 UI；Bridge/CDP URL 构造和结构化采集未接通，也没有 live E2E |
| Extension Bridge | 未部署、未验收 | 当前容器没有在线扩展客户端；无桌面主机若使用 Bridge，需要 Xvfb、headful Chromium、扩展和持久化 Profile |
| Cookie 长期无人值守 | 未达到 | 服务端会话无法保证永久有效，需要持久化 Profile、周期登录探针、失效熔断、通知和安全续期流程 |

当前生产基线继续选择 `cdp`、`headless=true`、持久化 `/app/chrome-profile` 和单会话串行。Bridge 不是云端常态化运行的前置条件；只有真实引擎无法通过 CDP 适配时，再单独部署和验收 Xvfb + Extension 方案。

## 环境准备

- 安装 Ubuntu 仓库 Docker 29.1.3 与 Docker Compose 2.40.3；
- 新增并持久化 2 GiB swap，验收期间未发生 OOM，swap 使用量保持 0；
- Docker Hub 在该主机超时。验收 Dockerfile 临时使用 DaoCloud 基础镜像代理、阿里云 Alpine 镜像和 `goproxy.cn`，应用源码、Go/Alpine 版本及构建参数不变；正式部署应使用组织批准的镜像仓库并固定摘要；
- 生产 Compose 使用 `UNIMAP_CPU_LIMIT=2`、`UNIMAP_MEMORY_LIMIT=1536M`、`UNIMAP_CPU_RESERVATION=1`、`UNIMAP_MEMORY_RESERVATION=512M` 适配验收机。默认生产值仍为 4 CPU / 6 GiB。

## 现场发现与修复

| 问题 | 现场证据 | 修复 |
|---|---|---|
| 单 URL 截图传递空 query ID | `POST /api/v1/screenshot` 返回 `invalid query ID` | handler 自动生成安全 ID；回归测试覆盖（`dff67cf`） |
| 生产资源硬编码 4 CPU | 2 vCPU 主机被 Docker 拒绝创建容器 | CPU/内存 limit 与 reservation 支持环境覆盖（`dff67cf`） |
| 健康接口无法识别镜像提交 | 初始镜像显示 `commit=unknown` | Dockerfile、Compose、CI 注入版本/提交/构建时间（`dff67cf`） |
| 生产缺少通知 pepper | 启动日志回退到公开 legacy pepper | prod Compose 强制 `UNIMAP_NOTIFY_PEPPER`（`dff67cf`） |
| Compose 顶层 version 已废弃 | Compose 2.40.3 每次发出 warning | 删除两个文件的顶层 `version`（`dff67cf`） |
| logs/backups 对非 root 用户不可写 | 备份 API 500，卷为 `root:root` | 镜像创建/chown backups；生产 logs 改命名卷（`bce3ee3`） |

## 验收证据

### 构建与运行

- CGO 构建成功，最终镜像约 1.32 GB；
- `/health/live` 与 `/health/ready` 返回 200；
- readiness：`user_db`、`history_db`、`scheduler`、`screenshot`、`engines` 均为 `ok`；
- 版本：`acceptance (commit=bce3ee3, built=2026-07-23T10:30:00+08:00)`；
- 容器用户为 `unimap:unimap`，实际限制为 2 CPU / 1.5 GiB；
- 容器内 Chromium 136.0.7103.113、`Noto Sans CJK SC`、DNS 和 HTTPS 均正常。

### 功能闭环

- 单 URL 截图：`https://example.com` 返回 PNG，1365×629，16,939 bytes；
- 异步批量截图：2 个 URL，终态 `completed`，成功 2、失败 0；
- 网页巡检：`security` 模式成功获取页面与分段哈希，首次结果为 `no_baseline`；
- 备份：API 返回 201 并生成可读 `.tar.gz`；备份已复制到本机临时目录并成功列出 SQLite/WAL 与巡检记录；
- 调度：创建禁用的 backup 任务，手动执行后历史为 `success`，耗时 5 ms；
- 登录：按真实 CSRF Cookie 流程登录成功，随后已注销并删除临时 Cookie；
- 重启持久化：容器重启后，调度任务、成功历史、备份和 2/2 截图批次均可读取；readiness 继续为 200。

### 网络与资源

- 主机内核仅显示 `127.0.0.1:8448`，未监听 9222 或 19451；
- Codex 本机出口使用 `198.18.0.1` 合成代理，对未监听端口也会先建立代理连接，因此不能代替阿里云控制台安全组核查；
- 轻载容器约 10.6 MiB RSS、8 个进程；主机验收结束时约 1.1 GiB 可用内存、27 GiB 可用磁盘，无 OOM；
- 首次镜像与构建缓存把磁盘占用从约 2.7 GiB 提高到约 11 GiB，生产必须配置镜像/构建缓存清理策略。

## 未通过或未执行

1. 主机只有 2 vCPU / 1.6 GiB / 40 GiB，不满足 4 vCPU / 8 GiB / 80–100 GiB 推荐基线，不能据此确认批量生产容量；
2. 未提供 FOFA、Hunter、ZoomEye、Quake、Shodan 等真实 API Key/Cookie，未执行真实引擎查询与配额验收；readiness 的 Web-only adapter 数量不等于查询可用；
3. 未配置域名、TLS 反向代理和证书；
4. 阿里云安全组需在控制台确认仅开放必要端口；
5. 项目没有在线 restore API。本轮完成了异机复制和归档可读性检查，没有覆盖生产数据卷执行破坏性恢复；
6. 未进行高并发、长时间巡检、磁盘保留周期和 OOM 压测。

## 后续上线门槛

使用至少 4 vCPU / 8 GiB / 80–100 GiB 的正式测试机，提供实际启用引擎的测试凭据、域名和反向代理配置；核查安全组后执行真实查询、并发批量任务、备份恢复演练和 24 小时资源观测，再给出完全通过结论。

常态化运行的分工、凭据交付方式、分阶段实施和最终验收标准见 [云服务器常态化运行准备与协作清单](CLOUD_STEADY_STATE_PLAN_2026-07-23.md)。
