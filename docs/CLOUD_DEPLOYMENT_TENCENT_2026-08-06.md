# UniMap 腾讯云部署分析（2026-08-06）

> 目标：把当前工作区（develop，commit `7f86376` 之后）部署到腾讯云服务器，重建 10 个定时任务每日 10:15
> 执行，查询 → 去重 → 企业微信推送。本文是专项分析；通用基线见
> [CLOUD_DEPLOYMENT_ASSESSMENT_2026-07-20.md](CLOUD_DEPLOYMENT_ASSESSMENT_2026-07-20.md)。

## 0. 当前状态（本机已闭环，云端待办）

- 定时查询 → 企业微信推送链路已本地闭环：10 个任务（fofa×1 + quake×1 + hunter×8），cron `15 10 * * *`，
  通知渠道 `dijia_01`（企业微信 webhook），不截图；实测 FOFA 100 条 / quake 55 条 / Hunter 真实资产，去重后
  推送成功（WeCom errcode 0）。
- 推送为紧凑 markdown 管道表格（`| 资产 | 标题 | 状态 |`），3800B 预算对齐 WeCom 4096 上限，`notification_detail_limit=100`。
- 引擎 key 现状：fofa / hunter / quake / daydaymap ✅ 可用；zoomey ⚠️ key 有效但积分不足（402）；shodan ⚠️ key 有效但
  无 membership；censys ⚠️ api_secret 为空，仅 Web-only。
- 生产模板齐备：`Dockerfile`（CGO_ENABLED=1 + build-base，B-01 已修复）、`docker-compose.prod.yaml`（B-02）、
  `configs/config.prod.yaml`。

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
   `POST /api/v1/scheduler/tasks/create` 重建 10 个任务。

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

## 4. 重建 10 个定时任务（每日 10:15）

任务参数（与当前 data/scheduler_tasks.json 一致，`UpdateTask`/`create` 需全量 payload）：

| 任务 | 引擎 | 说明 | page_size | detail_limit |
|---|---|---|---|---|
| fofa 大查询 | fofa | `查询语句.txt` fofa 段合并 | 100 | 100 |
| quake_ynmobile_daily | quake | 13 个 favicon MD5 合并 OR | 100 | 100 |
| hunter×8 分片 | hunter | 8 个分片查询 | 100 | 100 |

- cron：`15 10 * * *`
- 通知渠道：`dijia_01`（企业微信 webhook）
- `screenshot_enabled: false`（不截图）
- 用 `GET /api/v1/scheduler/tasks` + `POST /api/v1/scheduler/tasks/run` 逐任务验收一次，再等每日 cron。

## 5. 云端验收清单（复用评估文档 12 项）

1. `/health/live` 与 `/health/ready` 200，readiness 各库 `ok`；
2. 真实凭据完成一次查询并读 SQLite 结果明细；
3. 重建容器后用户、任务、历史仍存在；备份复制到异机并恢复演练；
4. 8448 / 9222 / 19451 未暴露公网；只开 22/80/443；
5. 10 个任务各 run 一次：`/api/v1/scheduler/history` 为 `success`，通知含「| 资产 | 标题 | 状态 |」表头和已持久化资产；
6. 企业微信真机确认表格渲染；若管道表格不渲染，切 code-fence 对齐格式（需改代码 + 重新构建镜像）。

## 6. 当前阻塞

- 需要用户提供腾讯云服务器访问（IP/SSH），才能执行条件④的实际部署。
