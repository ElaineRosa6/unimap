# UniMap 实施收口状态（2026-08-02）

> 本页记录当前工作区、真实浏览器验收、云端发布与通知闭环的最新结论。历史报告如有冲突，以本页、当前代码和操作文档为准。

## 已完成

- 稳定 Web UI 已接入 FOFA、Hunter、ZoomEye、Quake、Shodan、Censys、DayDayMap 七个引擎。
- 七引擎 Extension Bridge 真实结构化采集均取得非空结果：10、10、10、10、10、9、1（DayDayMap 重试通过）。
- API 与 Browser 并行查询采用 `success` / `partial` / `error`；API 失败但 Browser 非空时返回 HTTP 200 `partial`，并只写入一条合并历史。
- Bridge 凭据交接支持类型化 Cookie 与同源 Web Storage；Cookie 持久化，Web Storage 仅保存在进程内。
- DayDayMap Bridge→CDP 在受控 loopback SOCKS5 出口取得 10 条资产和 PNG；Censys 的 Cloudflare 挑战被识别为 `browser_challenge`，`auto`/fallback 切换 Bridge 后取得 9 条资产和 PNG。
- guarded browser egress 只接受 literal-loopback SOCKS5，并在实时 DNS 公网复核后向代理发送固定公网 IP:port；HTTP、外部代理和远程 Chrome 保持失败关闭。
- Extension 版本为 0.4.15；可配置 API base 仅接受显式 loopback HTTP 地址与端口。
- 云端最终二进制和派生镜像已发布并验证健康、readiness、重启、旧镜像回滚、再次发布和配置哈希保持。
- 云端飞书应用通知已连续两次返回 HTTP 200；配置回滚、重新应用与重启后健康检查均通过，临时明文载荷已清理。
- 全量门禁通过：Extension 9/9、`go test -race ./...`、`go vet ./...`、`go build ./...`、前端语法与 `git diff --check`。

## 发布证据

- 最终二进制 SHA-256：`34caa9cecf19d4dab413171beec45c2db158c0a1790a524eaece09b636687187`
- 最终镜像摘要：`sha256:d5080e96eff7259b5508d473c77e647dfb788337e3937f254fcb1f1a836cd46d`
- 交付目录：`artifacts/delivery-20260802/`
- 浏览器验收日志：`artifacts/live-20260802/`
- 浏览器详细记录：[BROWSER_SEVEN_ENGINE_VERIFICATION_2026-08-02.md](BROWSER_SEVEN_ENGINE_VERIFICATION_2026-08-02.md)

## 尚待外部输入

自动“发现变化 → 证据截图 → 图片通知”继续保持默认关闭。最后一项云端受控 fixture 验收需要：

1. 受控测试域名及子域；
2. 可编辑对应 DNS zone/record 的短期凭据或等价接口；
3. fixture reset、flip、control 与 target URL。

完成 DNS rebinding、真实页面变化、图片送达、恢复与重启复验后，才启用自动巡检证据截图。

FOFA、ZoomEye、Shodan 的原生 CDP 非空采集仍按引擎单独定级；这不影响七引擎 Bridge 通过结论。Censys 原生 CDP 当前归类为挑战受限，其自动 Bridge fallback 已通过。
