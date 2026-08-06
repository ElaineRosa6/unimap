# 七引擎浏览器验收记录（2026-08-02）

> 分支：`develop`。记录只包含脱敏结果与仓库内相对证据路径，不包含 Cookie、API Key、Bridge token、通知凭据或云主机地址。

## 1. Extension Bridge 结构化采集

使用当前已登录 Chrome、Extension 0.4.15 和 `TestLiveBridgeStructuredCollection` 逐引擎串行执行：

| 引擎 | 结果 | 结构化资产 | 页面候选行 |
|---|---:|---:|---:|
| FOFA | 通过 | 10 | 10 |
| Hunter | 通过 | 10 | 30 |
| ZoomEye | 通过 | 10 | 10 |
| Quake | 通过 | 10 | 26 |
| Shodan | 通过 | 10 | 10 |
| Censys | 通过 | 9 | 107 |
| DayDayMap | 重试通过 | 1 | 2 |

原始脱敏日志：`artifacts/live-20260802/seven-engine-bridge.log`、`artifacts/live-20260802/daydaymap-bridge-retry.log`。DayDayMap 的当前路由是 `/searchResult?keyword=...`；Bridge task DTO 必须携带 `query`。

## 2. Bridge 凭据交接到 CDP

新动作 `get_browser_credentials` 在目标引擎同源标签页读取类型化 Cookie、localStorage、sessionStorage 和最终 URL。服务端校验最终 URL 同源：

- Cookie 写入既有受保护配置并应用于本地/attached CDP；
- Web Storage 仅驻留进程内，CDP 首次导航到同源后注入并刷新；
- 域名白名单、loopback Bridge 和 SSRF fail-closed 规则保持不变。

Censys 与 DayDayMap 的真实交接动作均成功。系统代理是 `127.0.0.1:7897`，新 guarded egress 只允许 literal-loopback SOCKS5 上游，并在 UniMap 内完成实时 DNS 公网复核后仅向 SOCKS 传递固定 IP:port；HTTP、外部或主机名形式的代理仍失败关闭。

- DayDayMap：交接 15 项 Web Storage，经该受控出口取得 10 条结构化资产与 125,017 字节 PNG，测试通过。证据：`artifacts/live-20260802/daydaymap-cdp-socks5-final.log` 与对应 `live-bridge-daydaymap-cdp-*.png`。
- Censys：交接 1 个 Cookie 和 16 项 Web Storage 后，headless/headful CDP 均到达 Cloudflare `Just a moment...` 挑战页。采集器现返回 `browser_challenge=true`，`auto`/fallback 对该单次任务切换到 Extension；真实复验取得 9 条资产和 361,206 字节 PNG。证据：`artifacts/live-20260802/censys-cdp-bridge-fallback-final.log`。

因此 DayDayMap 标记为 **CDP 通过**；Censys 标记为 **CDP 挑战已识别、Bridge fallback 通过**，不把挑战页的 0 资产误报为正常结果。

## 3. 截图、SQLite 与通知

Censys、DayDayMap 的调度闭环均验证：Bridge 结构化采集非空、截图生成、调度成功路径、SQLite 恰好一条合并历史和结果明细持久化。代表截图：

- `artifacts/live-20260802/live-bridge-censys-1785613983248726300.png`
- `artifacts/live-20260802/live-bridge-daydaymap-1785614727364283400.png`

Windows 环境曾出现系统套接字策略拒绝，随后在云端运行环境完成最终复验：启用 `feishu_app` 通道、60 秒超时和 2 次重试后，两次真实测试均返回 HTTP 200 与 `success=true`。配置回滚脚本已实际执行，原配置哈希恢复后再次应用通知配置并重启，服务保持 healthy；本地、云主机和容器内的临时明文通道载荷均已删除。回滚文件：`artifacts/delivery-20260802/cloud-feishu-config-rollback.sh`。

## 4. 云端发布与 fixture

当前代码已构建为启用 CGO/SQLite 的静态 musl Linux amd64 二进制，并通过旧运行镜像的派生镜像发布；服务继续只绑定 loopback。发布后 `/health/live`、`/health/ready` 均为 200，history/user/ICP DB 检查为 `ok`，容器健康状态为 `healthy`。配置哈希在升级、容器重启、旧镜像回滚和再次发布四个阶段保持一致；回滚脚本已实际执行通过。一次 CGO-disabled 候选因 SQLite stub 导致 readiness 503，已立即回滚并由 CGO 版本修正。

最终二进制 SHA-256 为 `34caa9cecf19d4dab413171beec45c2db158c0a1790a524eaece09b636687187`；最终派生镜像摘要为 `sha256:d5080e96eff7259b5508d473c77e647dfb788337e3937f254fcb1f1a836cd46d`。交付与验证记录位于 `artifacts/delivery-20260802/`。

验收 fixture 已以静态 Linux amd64 二进制形式上传到隔离 staging 目录并核对本地/远端 SHA-256 一致，但尚未启动 DNS rebinding/真实变化验收：当前缺少受控测试域名、DNS 编辑凭据和 fixture 控制/目标 URL。

## 5. 回归入口

```powershell
go test -race ./cmd/unimap-web ./web ./internal/service ./internal/screenshot ./internal/scheduler ./internal/config
node --test tools/extension-screenshot/test/*.test.mjs
go test -race ./...
go vet ./...
go build ./...
```
