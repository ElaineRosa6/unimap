# UniMap 云端安全收口验收 Runbook

> 日期：2026-07-29
> 状态：验收代码已就绪；外部受控设施尚未提供，不能标记通过
> 适用提交：包含 `e1126ab` 及其后的自动证据截图门禁提交
> 原则：只使用明确授权的测试域名、页面和通知接收端；禁止记录 token、Cookie、用户 ID 或完整配置

## 1. 验收目标

本 Runbook 只关闭两个仍未完成的发布门槛：

1. 同一域名从公网解析切换到私网/loopback 后，浏览器出口在新连接拨号前拒绝，私网 sink
   命中数不增加；
2. 受控页面建立基线后发生真实变化，定时巡检检出篡改、生成真实 PNG、通过图片通知送达，
   服务重启后任务和基线仍可恢复。

两项都通过前：

- `tamper.evidence_screenshot_enabled` 必须保持 `false`；
- 自动巡检证据截图不得对生产目标启用；
- 发布状态仍是候选版本验证，不能写“安全收口完成”。

## 2. 外部设施契约

### 2.1 DNS rebinding fixture

需要一个专用测试域名和三个位于另一公网控制域名的 HTTPS API：

| 环境变量 | 方法 | 契约 |
|---|---|---|
| `UNIMAP_LIVE_DNS_TARGET_URL` | GET | 被测域名；reset 后解析到可返回 2xx/3xx 的公网服务 |
| `UNIMAP_LIVE_DNS_RESET_URL` | POST | 恢复公网解析并把私网 sink 计数清零 |
| `UNIMAP_LIVE_DNS_FLIP_URL` | POST | 将被测域名切换为 RFC1918 或 loopback 地址 |
| `UNIMAP_LIVE_DNS_PRIVATE_HITS_URL` | GET | 返回 `{"hits":0}`；控制域名不能随被测域名一起切换 |
| `UNIMAP_LIVE_DNS_CONTROL_TOKEN` | Bearer | 控制 API 凭据，只放环境变量 |

建议 DNS TTL 不超过 30 秒。reset/flip 必须幂等；测试失败或中断后 reset 仍可调用。

### 2.2 可变页面 fixture

需要一个稳定的公网 HTTPS 页面和两个位于独立控制域名的 HTTPS API：

| 环境变量 | 方法 | 契约 |
|---|---|---|
| `UNIMAP_LIVE_TAMPER_URL` | GET | reset 后内容稳定；mutate 后主内容发生确定性变化 |
| `UNIMAP_LIVE_TAMPER_RESET_URL` | POST | 恢复基线页面 |
| `UNIMAP_LIVE_TAMPER_MUTATE_URL` | POST | 修改页面主要文本或 DOM，不能只改时间戳 |
| `UNIMAP_LIVE_TAMPER_CONTROL_TOKEN` | Bearer | 控制 API 凭据，只放环境变量 |

页面不得包含真实账号、PII、业务数据或密钥。通知渠道必须是已获接收方同意的测试渠道。

## 3. 预检

```powershell
git status --short --branch
git log -1 --oneline
go test -tags live_dns_e2e ./internal/screenshot -run '^$'
go test -tags live_tamper_e2e ./web -run '^$'
go test -race ./...
govulncheck ./...
```

预期：

- 工作区无普通未提交修改；
- 两个 live test 均可编译；
- 全量竞态测试通过；
- `govulncheck` 为 0 个可达漏洞。

## 4. DNS 变化连接级验收

```powershell
$env:UNIMAP_LIVE_DNS_TARGET_URL = 'https://rebind.example.invalid/probe'
$env:UNIMAP_LIVE_DNS_RESET_URL = 'https://control.example.invalid/dns/reset'
$env:UNIMAP_LIVE_DNS_FLIP_URL = 'https://control.example.invalid/dns/flip-private'
$env:UNIMAP_LIVE_DNS_PRIVATE_HITS_URL = 'https://control.example.invalid/dns/private-hits'
$env:UNIMAP_LIVE_DNS_CONTROL_TOKEN = '<从安全环境注入>'

go test -tags live_dns_e2e ./internal/screenshot `
  -run '^TestLiveBrowserEgressRejectsDNSRebinding$' -count=1 -v
```

测试自动完成：

1. reset 为公网解析；
2. 通过真实 loopback 出口代理成功访问公网服务；
3. 记录私网 sink 命中数；
4. flip 为受限地址并等待候选机 resolver 观测到变化；
5. 强制新连接并要求请求失败；
6. 再读命中数，要求增量为 0；
7. cleanup 恢复公网解析。

唯一通过标记：

```text
LIVE_DNS_REBIND success public_connect=true restricted_connect=false private_hits_delta=0
```

只看到 HTTP 失败但没有 sink 计数证据，不算通过。

## 5. 页面变化、证据截图、通知与重启验收

本项只在候选环境临时把克隆配置中的证据开关设为 true；不得先修改生产配置。

```powershell
$env:UNIMAP_LIVE_BRIDGE_PORT = '18449'
$env:UNIMAP_LIVE_TAMPER_URL = 'https://tamper-page.example.invalid/'
$env:UNIMAP_LIVE_TAMPER_RESET_URL = 'https://control.example.invalid/page/reset'
$env:UNIMAP_LIVE_TAMPER_MUTATE_URL = 'https://control.example.invalid/page/mutate'
$env:UNIMAP_LIVE_TAMPER_CONTROL_TOKEN = '<从安全环境注入>'

go test -tags live_tamper_e2e ./web `
  -run '^TestLiveTamperEvidenceNotificationAndRestart$' -count=1 -v
```

测试自动完成：

1. reset 页面并建立基线；
2. mutate 页面；
3. 创建一次 `tamper_check` 调度任务；
4. 要求状态为 `tampered`；
5. 通过 ScreenshotRouter/CDP 生成证据图并验证 PNG magic；
6. 要求飞书应用图片通知成功指标增加；
7. 关闭并重建 Web 服务；
8. 要求调度任务恢复，原基线仍能检出变化；
9. cleanup 恢复页面。

唯一通过标记：

```text
LIVE_TAMPER_EVIDENCE success tampered=true png_bytes=<N> notification_success=true task_restored=true baseline_persisted=true
```

人工同时确认通知图片可预览且内容为 mutate 后页面。截图可保存在忽略目录并记录 SHA-256，
不得提交含账号、用户 ID 或其他 PII 的原图。

## 6. 失败处理

- DNS flip 后 resolver 在 120 秒内未变化：先检查 TTL、权威记录和候选机 DNS 缓存，不放宽测试；
- 私网 sink 命中数增加：立即阻断发布并保持自动证据截图关闭；
- 页面变化未检出：检查变更是否属于被归一化的动态区域，不通过降低检测阈值掩盖问题；
- 截图失败：检查 ScreenshotRouter、CDP readiness 和 SSRF guard，不允许降级到无受控出口的 Extension 任意 URL；
- 通知失败：保留任务与截图证据，修复渠道后重新完整执行，不能只补发图片算通过；
- 重启后任务或基线丢失：阻断启用开关，检查 APPDATA/卷挂载和调度、hash store 持久化。

## 7. 通过后的收口动作

两项 live E2E 和人工图片确认全部通过后：

1. 将脱敏命令结果、提交号、时间、候选环境标识和截图 SHA-256 写入修复报告；
2. 在明确的部署变更中把 `tamper.evidence_screenshot_enabled` 改为 `true`；
3. 重跑全量发布门禁和 readiness；
4. 更新 README、架构、Runbook 与剩余工作清单；
5. 才能把声明改为“所声明范围内安全收口完成”。
