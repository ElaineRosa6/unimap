# UniMap 云服务器部署评估（2026-07-20 初版 / 2026-07-23 配置复核）

> 本文记录 2026-07-20 对当前工作区的静态核查与定向验证结果。容量规格属于部署建议，必须在正式云机上用真实任务复测；已确认缺陷则在修复并重新验收前持续作为发布阻断项。

## 结论

当前代码已经具备无图形 Linux 的 CDP headless 运行路径：运行镜像安装 Chromium 与中日韩字体，截图模式可固定为 `cdp`，不依赖桌面环境、`DISPLAY`、X11、VNC 或 GPU。持久化 Chrome profile 存在独占锁，程序会把共享 profile 的浏览器会话自动限制为 1。

### B-01 修复状态（2026-07-20）

原阻断项 `Dockerfile` 使用 `CGO_ENABLED=0` 构建导致 `go-sqlite3` 初始化失败，已修复：

- `Dockerfile` builder 阶段安装 Alpine `build-base`（C 工具链），构建命令改为 `CGO_ENABLED=1`；
- `.github/workflows/ci.yml` 全局 `CGO_ENABLED` 同步改为 `1`，Docker build-arg 同步调整；
- 本地定向验证：`CGO_ENABLED=1 go test ./internal/auth ./internal/history ./internal/screenshot/batchdb` 全部通过；
- 全量 `go test -race ./...`、`go vet ./...`、`go build ./...` 通过。

但镜像内完整数据库与 headless 闭环仍需在正式云机执行验收（见下方验收清单第 3–8 项）。

推荐的单机完整功能基线是 **4 vCPU、8 GiB RAM、80–100 GiB SSD、2–4 GiB swap、x86_64 Linux**。现有 Compose 的 2 CPU / 1 GiB 限制只可视为未验收的启动下限，不适合作为批量截图和定时巡检的生产容量。

## 核查范围与证据

核对文件：

- `Dockerfile`
- `docker-compose.yml`
- `configs/config.docker.yaml`
- `configs/config.yaml.example`
- `internal/config/config_defaults.go`
- `internal/screenshot/manager.go`
- `internal/utils/path.go`
- `web/health_handlers.go`
- `web/server.go`
- `web/backup_handlers.go`

定向验证命令：

```powershell
$env:CGO_ENABLED = '0'
go test ./internal/auth ./internal/history ./internal/screenshot/batchdb -count=1
```

验证结果：三个包均失败，根因一致：

```text
Binary was compiled with 'CGO_ENABLED=0', go-sqlite3 requires cgo to work. This is a stub
```

本机未安装 Docker CLI，因此本轮没有执行 `docker compose config`、镜像构建或容器运行；也没有连接此前暂缓使用的云服务器。真实发行版、Docker/cgroup、出站网络、安全组和负载峰值仍需在正式测试机验证。

## 部署前发布阻断项

### B-01：镜像构建禁用 CGO，SQLite 不可用 — ✅ 代码已修复

原影响：

- 用户数据库无法初始化，认证启用时 readiness 不通过；
- 操作历史无法持久化；
- 截图批次数据库无法持久化；
- ICP、巡检等其他 SQLite 路径同样存在运行时失败风险；
- 镜像可以完成编译，但不能据此判定可运行。

**修复内容**（commit `5e136cb`）：

1. `Dockerfile` builder 阶段安装 Alpine `build-base`，构建命令改为 `CGO_ENABLED=1`；
2. `.github/workflows/ci.yml` 全局 `CGO_ENABLED` 同步改为 `1`，Docker build-arg 同步调整。

**已验证**：

- `CGO_ENABLED=1 go build ./...` — PASS
- `CGO_ENABLED=1 go test ./internal/auth ./internal/history ./internal/screenshot/batchdb` — PASS
- `CGO_ENABLED=1 go test -race ./...` — PASS
- `CGO_ENABLED=1 go vet ./...` — PASS

**仍需在正式云机执行的验收**（镜像内 SQLite + headless 闭环）：

1. 构建镜像并在容器内启动服务；
2. `/health/ready` 返回 200，`user_db`、`history_db` 和 `screenshot` 为 `ok`；
3. 完成用户登录、查询历史写入、截图批次写入和容器重启后重读；
4. 保存镜像标签、架构、测试输出和回滚镜像。

### B-02：生产 Compose 代码基线已落实 — 待云机验收

当前 Compose 还存在以下生产差距：

- ~~`./web:/app/web` 是开发式 bind mount~~ — ✅ `docker-compose.prod.yaml` 使用 `volumes: !override` 完整替换基础卷，使用镜像内静态资源；
- ~~`8448:8448` 会直接绑定宿主机所有接口~~ — ✅ 使用 `ports: !override` 后仅保留 `127.0.0.1:8448:8448`，由反向代理提供 HTTPS；
- ~~`/app/backups` 没有持久化~~ — ✅ prod 覆盖文件新增 `unimap_backups` 卷；
- 配置文件以 `:ro` 挂载，适合声明式配置，但 Web 设置页的持久化保存会失败，必须明确采用”改部署配置并重启”的流程；
- `deploy.resources` 属于平台可选能力，部署后需检查 cgroup 是否真正应用 CPU/内存限制；
- ~~没有日志轮转~~ — ✅ prod 覆盖文件设置 `50m × 5`；TLS 反向代理和异机备份仍由部署环境落实。

**已新增**：`docker-compose.prod.yaml` 作为生产覆盖配置，与基础 `docker-compose.yml` 合并使用：

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yaml up -d --build
```

覆盖内容包括：通过 `!override` 形成唯一 loopback 端口绑定并移除 web bind mount、挂载 backups 卷、日志轮转（50MB × 5 文件）、生产资源限制（4 CPU / 6G RAM），以及通过 `configs/config.prod.yaml` 解析固定 `UNIMAP_ADMIN_TOKEN`、`UNIMAP_DISTRIBUTED_ADMIN_TOKEN` 和非默认 `UNIMAP_ADMIN_USERNAME`。`!override` 要求 Docker Compose 2.24.4 或更高版本。

仍待验证：实际云机上 cgroup 资源限制生效、日志轮转生效、反向代理 TLS 配置、异机备份流程。

## 建议容量

以下容量是工程估算，不是已完成的压测结果：

| 场景 | CPU | RAM | SSD | 建议 |
|---|---:|---:|---:|---|
| API 查询为主、轻量单用户 | 2 vCPU | 2–4 GiB | 40 GiB | 不用于高频截图 |
| 单机完整功能 | 4 vCPU | 8 GiB | 80–100 GiB | 推荐起点；截图串行 |
| 高频批量任务 | 8 vCPU | 16 GiB | 200 GiB 以上 | 需要浏览器池或独立 profile 设计才能提高截图吞吐 |

通用建议：

- 使用 Ubuntu 24.04 LTS 或 Debian 12 的 x86_64 实例；ARM64 尚未完成镜像与 Chromium 验收；
- 配置 2–4 GiB swap 作为突发缓冲，但不能用持续交换代替物理内存；
- 若在云机本地构建镜像，额外保留 10–15 GiB 构建空间；生产更适合拉取已验收的固定标签镜像；
- 4 vCPU / 8 GiB 主机可先将容器限制设置为 4 CPU、4–6 GiB RAM，并以 `docker stats` 和 OOM 记录校准；
- `/dev/shm` 可从 512 MiB 起步，截图继续保持 `max_sessions: 1`；
- `system.max_concurrent` 从 5–10 起步，不能同时把查询、目标和端口扫描并发调到最大。

## 生产配置要求

### 认证与凭据

必须提供：

- 高强度 `UNIMAP_BOOTSTRAP_PASSWORD`；
- 固定的 `web.auth.admin_token`，不要依赖每次启动自动生成；
- 非默认 Web 管理用户名；
- 实际启用引擎的 API Key，FOFA 还需要对应邮箱；
- 已启用通知渠道的凭据；
- 域名对应的 CORS origin；
- 使用反向代理时，精确配置代理容器/主机网段到 `trusted_proxy_cidrs`。

环境变量只有在 YAML 中写成 `$VAR` 或 `${VAR}` 占位符，且 Compose 把该变量传入容器时才会解析。生产覆盖默认挂载 `configs/config.prod.yaml`，其中已显式声明三个认证占位符。不要直接复制本机 `configs/config.yaml` 到服务器；如需自定义，应从生产基线或示例生成独立配置，并轮换已经在开发环境使用过的凭据。

### 查询引擎

缺少 API 凭据时，Web 服务会注册 Web-only 适配器；但浏览器查询 fallback 默认关闭。因此“adapter 已注册”或 `/health/ready` 成功不等于真实查询可用。

云端建议优先使用 API 模式。若必须使用浏览器采集，需要另外完成：

- 受控迁移 Cookie 或 Chrome profile；
- 对应引擎登录态验证；
- 验证云 IP 没有触发验证码、登录墙或访问限制；
- 真实执行结构化采集，而不是只检查 PNG 非空。

Extension Bridge 的配对、任务拉取和回调面向同机 loopback 浏览器，不应通过公网暴露来替代 CDP。

### 截图

推荐配置：

```yaml
screenshot:
  enabled: true
  mode: cdp
  priority: cdp
  fallback: false
  chrome_path: /usr/bin/chromium
  chrome_user_data_dir: /app/chrome-profile
  headless: true
  no_sandbox: true
  max_sessions: 1
  timeout: 30
```

容器继续以非 root 用户运行，不得为了 Chrome 或扫描功能启用 `privileged: true`。容器内 `no_sandbox: true` 是当前受限运行环境基线；非容器部署应保持 `false` 并使用 Chrome sandbox。

## 端口与权限

| 端口/能力 | 公网策略 | 用途 |
|---|---|---|
| 22/tcp | 仅允许固定管理 IP | SSH |
| 80/tcp | 可选公开 | 跳转 HTTPS/证书验证 |
| 443/tcp | 公开 | 生产 Web 入口 |
| 8448/tcp | 不直接公开 | UniMap Web，由反向代理访问 |
| 9222/tcp | 禁止公开 | Chrome DevTools/CDP |
| 19451/tcp | 禁止公开 | Extension Bridge loopback |
| Redis/ICP sidecar | 仅内部网络 | 可选依赖 |

普通查询、截图、巡检、TCP connect/Telnet/UDP 不需要容器特权。仅在确实启用 FIN/NULL/Xmas 原始 TCP 探测时增加 `CAP_NET_RAW`，同时仍必须填写 `authorized_targets`；不要增加 `NET_ADMIN` 或全量特权。

## 数据、磁盘与备份

必须持久化：

- `/app/data`：用户、历史、调度、API Key、截图任务等状态；
- `/app/screenshots`：截图文件；
- `/app/chrome-profile`：浏览器登录态；
- `/app/logs`：应用日志；
- `/app/backups`：本地备份输出，生产覆盖已挂载 `unimap_backups`。

截图容量估算：

```text
截图磁盘 ≈ 每日截图数量 × 平均单图大小 × 保留天数 × 1.3
```

SQLite WAL 文件、日志、导出文件、Chrome profile 和镜像层应另留空间。备份至少复制到另一块磁盘或对象存储，并定期做恢复演练；同机同盘的备份不能覆盖云主机或磁盘整体故障。

## 正式云机验收清单

1. 固定镜像标签和配置版本，不使用漂移的 `latest`；
2. 容器内 Chromium 路径、字体、DNS、TLS 和时区正确；
3. `/health/live` 与 `/health/ready` 返回 200；
4. readiness 中用户库、历史库、截图后端均为 `ok`；
5. 使用真实凭据完成一次查询并读取 SQLite 结果明细；
6. 完成单 URL PNG、异步批量截图和明确终态轮询；
7. 完成一次巡检和一次调度执行；
8. 重建容器后验证用户、任务、历史、截图和 profile 仍存在；
9. 创建备份、复制到异机位置，并完成一次恢复测试；
10. 确认 9222、19451、8448、Redis 等内部端口未暴露公网；
11. 在真实批量负载下记录 CPU、RSS、swap、磁盘增长、任务时延和 OOM；
12. 根据观测值调整容器资源限制，而不是直接提高截图并发。

## 后续顺序

1. ~~修复并验证 CGO/SQLite 镜像构建~~ — ✅ 已完成（commit `5e136cb`）；
2. ~~新增生产 Compose 与生产配置模板~~ — ✅ 已完成（`docker-compose.prod.yaml`）；
3. 在正式云机执行完整验收（B-01 镜像内 SQLite 闭环 + B-02 cgroup/日志/TLS/备份验证）；
4. 根据实测容量调整实例规格、保留周期和告警阈值；
5. 验收通过后更新本文，将 B-01/B-02 标记为已关闭并附提交与测试证据。
