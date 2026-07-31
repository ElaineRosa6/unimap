# UniMap 云端安全验收 Fixture

为 UniMap 云安全收口验收提供受控外部测试环境。满足
[云端安全收口验收 Runbook](../../docs/CLOUD_SECURITY_ACCEPTANCE_RUNBOOK_2026-07-29.md)
定义的两项发布门槛：

1. **DNS rebinding 连接级验收**：同一域名从公网解析切换到私网/loopback 后，
   浏览器出口代理在新连接拨号前拒绝，私网 sink 命中数不增加。
2. **受控页面篡改验收**：建立基线后发生真实变化，定时巡检检出篡改、生成真实 PNG、
   通过图片通知送达，服务重启后任务和基线仍可恢复。

## 架构

```
Test Machine (Windows)                    Cloud Server
┌──────────────────────┐                ┌─────────────────────────────────┐
│ go test -tags ...    │   HTTPS        │  Caddy (auto TLS)              │
│   live_dns_e2e       │──────────────► │    control.example.com → :9090 │
│   live_tamper_e2e    │                │    target.example.com  → :8081 │
│                      │                │                     /tamper→:8082│
│                      │                │                                 │
│                      │                │  acceptance-fixture             │
│                      │                │    :9090 control API            │
│                      │                │    :8081 target page            │
│                      │                │    :8082 mutable page           │
│                      │                │    :8083 private sink           │
│                      │                │                                 │
│                      │                │  Cloudflare API (DNS flip)      │
└──────────────────────┘                └─────────────────────────────────┘
```

## 前置条件

| 需要 | 说明 |
|------|------|
| 一台公网服务器 | 有固定公网 IP，已开放 80/443 端口 |
| 两个域名/子域名 | 控制域名（稳定）+ 目标域名（DNS 翻转用），均解析到服务器公网 IP |
| Cloudflare 托管 | 目标域名在 Cloudflare 管理，API Token 有 Zone.DNS 编辑权限 |
| Docker + Compose | 服务器上已安装 |

## 部署

### 1. 准备配置

```bash
cd tools/acceptance-fixture
cp .env.example .env
# 编辑 .env 填入实际值
cp Caddyfile.example Caddyfile
# 编辑 Caddyfile 替换域名
```

### 2. 启动

```bash
docker compose up -d
docker compose logs -f  # 确认 "acceptance fixture ready"
```

### 3. 验证

```bash
# 健康检查
curl https://control.example.com/healthz

# 控制 API（需要 token）
curl -H "Authorization: Bearer $TOKEN" https://control.example.com/dns/private-hits
# → {"hits":0}

# 目标页面
curl https://target.example.com/
# → <html>...dns-fixture-target...</html>

# 可变页面
curl https://target.example.com/tamper
# → <html>...original baseline content...</html>
```

## 运行验收测试

在开发机（Windows）上设置环境变量后执行：

### DNS Rebinding 验收

```powershell
$env:UNIMAP_LIVE_DNS_TARGET_URL = 'https://target.example.com/probe'
$env:UNIMAP_LIVE_DNS_RESET_URL = 'https://control.example.com/dns/reset'
$env:UNIMAP_LIVE_DNS_FLIP_URL = 'https://control.example.com/dns/flip'
$env:UNIMAP_LIVE_DNS_PRIVATE_HITS_URL = 'https://control.example.com/dns/private-hits'
$env:UNIMAP_LIVE_DNS_CONTROL_TOKEN = '<your-token>'

go test -tags live_dns_e2e ./internal/screenshot `
  -run '^TestLiveBrowserEgressRejectsDNSRebinding$' -count=1 -v
```

### Tamper Evidence 验收

```powershell
$env:UNIMAP_LIVE_TAMPER_URL = 'https://target.example.com/tamper'
$env:UNIMAP_LIVE_TAMPER_RESET_URL = 'https://control.example.com/page/reset'
$env:UNIMAP_LIVE_TAMPER_MUTATE_URL = 'https://control.example.com/page/mutate'
$env:UNIMAP_LIVE_TAMPER_CONTROL_TOKEN = '<your-token>'

go test -tags live_tamper_e2e ./web `
  -run '^TestLiveTamperEvidenceNotificationAndRestart$' -count=1 -v
```

## 控制 API 端点

所有端点（除 `/healthz`）需要 `Authorization: Bearer <token>` 头。

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/dns/reset` | 目标域名 → 公网 IP + sink 计数清零 |
| POST | `/dns/flip` | 目标域名 → 私网/loopback IP |
| GET | `/dns/private-hits` | 返回 `{"hits": N}` |
| POST | `/page/reset` | 可变页面恢复基线内容 |
| POST | `/page/mutate` | 可变页面确定性修改 |
| GET | `/healthz` | 健康检查（无需认证） |

## DNS 翻转机制

使用 Cloudflare API v4 更新目标域名的 A 记录：

- **Reset**：A 记录 → `FIXTURE_PUBLIC_IP`（服务器公网 IP），TTL=30s
- **Flip**：A 记录 → `FIXTURE_PRIVATE_IP`（默认 127.0.0.1），TTL=30s

测试端 `waitForLiveDNSState` 最多等待 120 秒 DNS 传播。Cloudflare 通常 30 秒内生效。

## 私网 Sink

当 DNS 翻转到私网 IP 后，如果浏览器出口代理未能拦截，连接会到达私网 sink（:8083），
计数器递增。验收通过要求 `hits` 在翻转前后保持不变（delta=0）。

注意：如果 `FIXTURE_PRIVATE_IP=127.0.0.1`，sink 监听在服务器的 loopback 上。
从测试机发起的连接即使绕过代理也无法到达服务器的 127.0.0.1，因此 sink 计数
不会增加。如需更严格的验证，将 `FIXTURE_PRIVATE_IP` 设为服务器的内网 IP，
并在该 IP 上绑定 sink。

## 安全

- 控制 token 仅通过环境变量传入，不写入镜像或日志。
- 页面内容不含真实账号、PII、业务数据或密钥。
- Caddy 自动获取 Let's Encrypt 证书，仅开放 80/443。
- fixture 容器不暴露到公网，仅通过 Caddy 反向代理。

## 本地开发（无 Cloudflare）

不配置 Cloudflare 时，DNS 相关端点返回 501。仍可测试 tamper fixture：

```bash
# 仅启动 fixture（不用 Docker）
go run ./tools/acceptance-fixture \
  -token dev-token \
  -control-addr :9090 \
  -target-addr :8081 \
  -mutable-addr :8082
```

Tamper 测试需要公网 HTTPS，因此本地开发时通常配合 ngrok/Cloudflare Tunnel 暴露。
