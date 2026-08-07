# UniMap 腾讯云部署分析（2026-08-06）

> **实际部署状态修订（2026-08-07）**：本文原计划把工作区部署到腾讯云服务器。实际部署最终落在
> **阿里云**（`8.160.177.101`，2 vCPU / 1.6 GiB / 40 GiB，Ubuntu 24.04，部署目录 `/opt/unimap`，
> `docker compose -f docker-compose.yml -f docker-compose.prod.yaml`，本地构建镜像 `unimap:local`），
> 部署已随 develop `cf47728` 闭环（详见「0. 当前状态」与实际任务清单）。第 1 节腾讯云选型仅作选型参考，
> 实际以阿里云部署为准。
>
> **低配主机注意（本次踩坑）**：`docker-compose.prod.yaml` 默认 `cpus: 4 / memory: 6G`，在 2 核
> 主机的 `up -d` recreate 阶段会直接失败（`range of CPUs is from 0.01 to 2.00, as there are only
> 2 CPUs available`）。需在部署目录 `.env` 覆盖 `UNIMAP_CPU_LIMIT=1.5`、`UNIMAP_MEMORY_LIMIT=1280M`、
> `UNIMAP_CPU_RESERVATION=0.5`、`UNIMAP_MEMORY_RESERVATION=512M`。
>
> 原目标行：把当前工作区（develop，commit `7f86376` 之后）部署到云服务器，重建定时任务每日执行，
> 查询 → 去重 → 企业微信推送。本文是专项分析；通用基线见
> [CLOUD_DEPLOYMENT_ASSESSMENT_2026-07-20.md](CLOUD_DEPLOYMENT_ASSESSMENT_2026-07-20.md)。

## 0. 当前状态（本机与云端均已闭环）

- 云端（阿里云 `8.160.177.101`，/opt/unimap，容器 `unimap-unimap-1`，镜像 `unimap:local` 本地构建）已随
  develop `cf47728` 更新并重建验证：`/health/ready` 200、容器 healthy、`notification_push_log` 表已建、
  调度任务持久化保留。
- 云端实际为 **12 个定时任务**（fofa×2、quake×3、hunter×4、daydaymap×2、每周快照×1），通知渠道 `dijia_01`
  （企业微信 webhook），`only_new=true` 增量推送，`format=excel`，`page_size=100`、`notification_detail_limit=100`。
  调度时间非单一 10:15：fofa/quake/daydaymap 每天 `0 9,15 * * *`，hunter 错峰 `0/10/20/40 9 * * *`，
  每周快照 `15 10 * * 1`。完整清单见「4. 云端任务清单」。
- 推送为紧凑 markdown 管道表格（`| 资产 | 标题 | 状态 |`），3800B 预算对齐 WeCom 4096 上限。
- 引擎 key 现状：fofa / hunter / quake / daydaymap ✅ 可用；**hunter 备用 key（`backup_api_key`）已同步云端**
  （2026-08-07，主 key 积分耗尽时 `withKeyFailover` 自动切换）；zoomeye ⚠️ key 有效但积分不足（402）；
  shodan ⚠️ key 有效但无 membership；censys ⚠️ api_secret 为空，仅 Web-only。
- 生产模板齐备：`Dockerfile`（CGO_ENABLED=1 + build-base，B-01 已修复）、`docker-compose.prod.yaml`（B-02）、
  `configs/config.prod.yaml`。
- 云端配置在可写命名卷 `unimap_config`（容器 `/app/runtime-config/config.yaml`），非 git 工作区文件。

## 1. 腾讯云选型

| 项 | 建议 | 说明 |
|---|---|---|
| 产品 | **轻量应用服务器 Lighthouse**（简单）或 **CVM**（更细控制） | 单机部署、单用户，Lighthouse 性价比更高 |
| 地域 | 腾讯云上送商可用区（如广州/上海），避开高峰期 | 出站到 fofa/hunter/quake/daydaymap API + 企业微信 webhook |
| 镜像 | Ubuntu 24.04 LTS x86_64（或 Debian 12） | 镜像内已内置 Chromium；ARM64 未验收，勿选 |
| 规格 | 4 vCPU / 8 GiB / 80–100 GiB SSD（参考评估文档建议容量） | 截图串行，max_sessions=1；可先 2C4G 起步压测再升 |
| 带宽 | 5–10 Mbps 起步 | 查询/截图出站为主，入站流量小 |
| 安全组 | 只放行 22（固定管理 IP）、80/443；8448 不公开 | 见评估文档「端口与权限」 |

## 2. 部署前置（需要准备）

1. **云机 SSH 访问**：IP、用户名、密钥/密码。
2. **镜像构建**：二选一
   - A. 云机本地构建：云机装 Docker 后 `docker compose -f docker-compose.yml -f docker-compose.prod.yaml up -d --build`；
     需先推送 `develop` 代码到远端并克隆，或上传工作区源码。
   - B. 预构建镜像推到腾讯云 TCR / 私有 registry，云机 `pull` + `up --no-build`（推荐生产，用 `@sha256` 固定摘要）。
3. **生产凭据（环境变量，不落盘代码）**：
   - `UNIMAP_BOOTSTRAP_PASSWORD`、`UNIMAP_ADMIN_USERNAME`、`UNIMAP_ADMIN_TOKEN`、`UNIMAP_DISTRIBUTED_ADMIN_TOKEN`、`UNIMAP_NOTIFY_PEPPER`；
   - `FOFA_EMAIL` / `FOFA_API_KEY`、`HUNTER_API_KEY`、`QUAKE_API_KEY`、`ZOOMEYE_API_KEY`、`SHODAN_API_KEY`；
   - 企业微信 webhook key（`86a6cd9e-...`）写入通知渠道配置。
4. **配置迁移注意**：不要直接复制本机 `configs/config.yaml`（含 dev 凭据）到服务器；从 `configs/config.prod.yaml`
   基线生成独立配置并轮换已用过的凭据。调度任务 `data/scheduler_tasks.json` 可迁移（gitignored），或通过
   `POST /api/v1/scheduler/tasks/create` 重建 12 个任务。

## 3. 部署步骤

```bash
# 云机（Lighthouse Ubuntu 24.04）
sudo apt-get update && sudo apt-get install -y docker.io docker-compose-plugin
git clone <unimap-repo> && cd unimap && git checkout develop

# 用环境变量注入生产凭据（.env 或 export，不入库）
export UNIMAP_BOOTSTRAP_PASSWORD='<随机长密码>'
export UNIMAP_ADMIN_USERNAME='<非默认管理员>'
export UNIMAP_ADMIN_TOKEN='<随机令牌>'
export UNIMAP_DISTRIBUTED_ADMIN_TOKEN='<另一枚令牌>'
export UNIMAP_NOTIFY_PEPPER='<独立 pepper>'
export UNIMAP_CHROME_PATH=/usr/bin/chromium

docker compose -f docker-compose.yml -f docker-compose.prod.yaml up -d --build
curl --fail http://127.0.0.1:8448/health/ready
```

反向代理（可选）：用 Caddy / Nginx 在 443 终止 TLS 并代理到 127.0.0.1:8448，把代理网段加入
`web.rate_limit.trusted_proxy_cidrs`。

## 4. 云端任务清单（12 个，2026-08-07 实测）

云端 `scheduler_tasks.json` 持久化于容器卷 `/app/data/`（宿主 volume `unimap_unimap_data`），2026-08-07
实测清单如下（查询语句含目标指纹，已 gitignore 不入库，仅受控传输使用；各任务 payload 均可通过
`POST /api/v1/scheduler/tasks/create` 重建，或迁移服务器端 `scheduler_tasks.json`）：

| 任务 | 引擎 | cron | page_size | detail_limit | 说明 |
|---|---|---|---|---|---|
| fofa_ynmobile_a | fofa | `0 9,15 * * *` | 100 | 100 | 分片查询 a |
| fofa_ynmobile_b | fofa | `0 9,15 * * *` | 100 | 100 | 分片查询 b |
| quake_ynmobile_a | quake | `0 9,15 * * *` | 100 | 100 | favicon MD5 合并 |
| quake_ynmobile_b | quake | `0 9,15 * * *` | 100 | 100 | 分片 |
| quake_ynmobile_b2 | quake | `0 9,15 * * *` | 100 | 100 | 分片 |
| hunter_ynmobile_a | hunter | `40 9 * * *` | 100 | 100 | 错峰 9:40 |
| hunter_ynmobile_b | hunter | `0 9 * * *` | 100 | 100 | 错峰 9:00 |
| hunter_ynmobile_b2 | hunter | `10 9 * * *` | 100 | 100 | 错峰 9:10 |
| hunter_ynmobile_b3 | hunter | `20 9 * * *` | 100 | 100 | 错峰 9:20 |
| daydaymap_ynmobile_a | daydaymap | `0 9,15 * * *` | 100 | 100 | 分片 |
| daydaymap_ynmobile_b | daydaymap | `0 9,15 * * *` | 100 | 100 | 分片 |
| ynmobile_weekly_snapshot | fofa,hunter,quake,daydaymap | `15 10 * * 1` | 100 | 100 | 每周一 10:15 全引擎快照 |

- 全部 `enabled=true`；`format=excel`、`only_new=true`（增量推送，weekly 无 only_new 字段）、`timeout_seconds=300`、`max_retries=2`。
- 通知渠道：`dijia_01`（企业微信 webhook）；`screenshot_enabled` 未启用（不截图）。
- hunter 4 个任务错峰于 9 点档，避免同一时刻打满每日配额/限流。
- 用 `GET /api/v1/scheduler/tasks` + `POST /api/v1/scheduler/tasks/run` 可逐任务验收。

## 5. 云端验收清单（复用评估文档 12 项）

1. `/health/live` 与 `/health/ready` 200，readiness 各库 `ok`；
2. 真实凭据完成一次查询并读 SQLite 结果明细；
3. 重建容器后用户、任务、历史仍存在；备份复制到异机并恢复演练；
4. 8448 / 9222 / 19451 未暴露公网；只开 22/80/443；
5. 12 个任务各 run 一次：`/api/v1/scheduler/history` 为 `success`，通知含「| 资产 | 标题 | 状态 |」表头和已持久化资产；
6. 企业微信真机确认表格渲染；若管道表格不渲染，切 code-fence 对齐格式（需改代码 + 重新构建镜像）。

## 6. 当前状态与待办

- ✅ 云端部署已完成（2026-08-07，阿里云 `8.160.177.101` /opt/unimap，develop `cf47728`），
  12 个定时任务 + hunter 备用 key 均已生效；`only_new` 增量推送已在任务 payload 中（语义见 docs/API.md「增量推送（只推新增）」）。
- 待办仍属云端常态化范围：TLS 反向代理与域名、安全组收紧（8448 仅 loopback 已就绪）、异机备份与恢复演练、
  24 小时资源观察（2C/1.6G 低配，CPU/内存覆盖值见顶部修订块）。
- 任务清单（查询语句）gitignore 不入库，仅服务器端 `scheduler_tasks.json` 与受控传输使用。
