# UniMap 安全发布收口修复记录

> 日期：2026-07-29
> 基线：`develop` / `0d45fad`
> 代码修复提交：`0a4cb53`
> 对应计划：[安全发布收口修复计划](SECURITY_RELEASE_CLOSURE_REPAIR_PLAN_2026-07-29.md)
> 当前结论：代码阻断项已修复并进入全量门禁；外部真实验收完成前仍不能标记“安全收口完成”

## 1. 修复结果

| 编号 | 原问题 | 当前结果 |
|---|---|---|
| SEC-01 | `Fetch.enable` 在 Target 创建前执行 | guarded session 先初始化 `about:blank` Target，再启用 Fetch；初始化失败立即返回 |
| SEC-02 | DNS、重定向、资源类型、队列和 TOCTOU 绕过 | 实时 DNS 全地址校验、Fetch 全请求拦截、队列/命令错误 fail-closed、loopback 出口代理在实际拨号时校验并固定 IP |
| IMG-01 | JPEG 内容配 `.png`/`image/png` | JPEG 捕获回退转码为 PNG；单图接口校验实际 MIME |
| BRIDGE-01 | 调用者超时后任务仍执行 | 排队、执行和重试继承 Submit 上下文；已取消排队任务不调用 client |
| QUAKE-01 | Quake Web/Bridge 结构化结果为空 | Extension DOM 修复 clipboard IP；Quake L1 更新 endpoint 并兼容当前数组响应 |
| TEST-01 | headless fixture 被 SSRF 拦截 | 测试只放行指定随机端口 origin，并保留第二个受限 fixture |
| DEP-01 | Excelize v2.8.0 可达漏洞 | 升级到 v2.11.0；导出竞态测试通过；`govulncheck` 为 0 个可达漏洞 |
| DOC-01 | 文档状态冲突 | README、架构、工程说明、待办、实施计划和日期化证据同步 |

## 2. 安全边界

CDP 浏览器业务统一经过三层防护：

1. 初始 URL 和每个 Fetch 暂停请求执行静态、实时 DNS 校验；
2. Document、Script、XHR、Fetch、Image、Stylesheet、Font、Media、WebSocket 等全部请求统一拦截；
3. Chrome 强制通过 loopback 出口代理，代理在 HTTP、CONNECT 和 Upgrade 的实际拨号前重新解析，
   拒绝任一受限地址，并只拨号到已验证 IP。

当前失败关闭边界：

- 外部上游浏览器代理尚未实现等价的目标地址证明，因此拒绝进入 guarded session；
- 远程 Chrome 无法证明启动时已注入本轮出口代理，因此拒绝任意浏览器业务；
- Extension 搜索引擎任务必须回报 `final_url` 且不得离开请求主机；
- Extension 任意 URL 和批量截图在没有受控出口证明时禁用；
- 自动巡检证据截图继续禁用，等待受控云端页面变化和图片送达验收。

## 3. Quake 分链路定级

| 链路 | 结果 | 证据边界 |
|---|---|---|
| CDP DOM | 通过 | 2026-07-29 既有真实登录态，10 条资产与截图人工确认 |
| CDP L1 Network | 通过 | 当前 endpoint 与数组响应修复后，真实登录态 live E2E 返回非空资产 |
| Extension DOM | 真实验收通过 | 独立 loopback 端口加载当前 Extension，Quake 返回 10 条结构化资产 |
| Router 合并 | 自动测试通过 | Quake `collect_and_capture` 非空结构化结果与图片路径测试通过 |
| 完整 Web/调度 + 真实 Extension | 真实验收通过 | Query → Bridge collect_and_capture → SQLite → 通知明细与飞书图片发送闭环通过 |

2026-07-29 实机复验使用独立 `127.0.0.1:18448` 测试端口，避免日常 Chrome 中同 ID 旧
Extension 抢占任务。结果为 10 条持久化资产、真实 PNG 380161 字节，PNG magic 正确，
证据图人工确认是 Quake `port:443` 结果页，飞书应用图片通知成功。证据图保存在忽略目录，
未写入仓库；SHA-256 前缀为 `6B788185DFC6`。

复验同时发现 Extension 成功回调在标签页导航状态竞争时可能缺少 `final_url`。当前实现已改为
最多三次读取标签页 URL，并以页面 `location.href` 作为实际页面兜底；仍无法观测最终 URL 时
失败关闭，不再提交“成功但空 URL”。新增三项独立 JavaScript 测试覆盖瞬时失败、页面兜底和
无法观测时失败关闭；Extension manifest 已升级到 `0.4.2`，部署验收必须确认已重载该版本。

## 4. 已执行专项验证

```text
go test -race ./internal/utils/urlguard ./internal/screenshot -run 'Test.*(SSRF|BrowserGuard|Egress)' -count=1
go test -race ./internal/screenshot ./internal/tamper ./internal/service ./internal/scheduler ./web -count=1
go test -tags headless_e2e ./internal/screenshot -run TestHeadless -count=1 -v
go test -tags live_e2e ./internal/screenshot -run TestLiveQuakeL1NetworkCollection -count=1 -v
node --test tools/extension-screenshot/test/capture_quake.test.mjs
node --test tools/extension-screenshot/test/tab_url.test.mjs
go test -tags live_bridge_e2e ./web -run '^TestLiveBridgeScheduledQueryClosedLoop$' -count=1 -v
go test -race ./internal/exporter -count=1
govulncheck ./...
```

结果：

- URL guard、SSRF、出口代理、Bridge 取消与受影响包竞态测试通过；
- headless JavaScript/PNG、私网重定向和私网子资源阻断通过；
- Quake L1 真实测试通过；
- Quake Extension DOM 固定夹具通过；
- Quake 真实 Extension 完整闭环通过：10 条资产、真实 PNG、SQLite 明细和飞书图片通知；
- 状态型 DNS 测试通过：同一主机首次解析为公网地址时固定到已验证 IP，第二次解析变为
  loopback 时重新查询并在拨号前失败关闭；受限目标实际连接数保持为 0；
- Excel 导出测试通过；
- `govulncheck`：0 个可达漏洞。

最终门禁结果：

- `go test -race ./...`：通过；
- `go vet ./...`：通过；
- `go build ./...`：通过；
- `go build -tags gui ./cmd/unimap-gui`：通过（输出到本地缓存目录）；
- `node --check`：Web 主脚本及 Extension `background.js`、`capture.js` 通过；
- `git diff --check`：通过；
- `staticcheck`、`golangci-lint`：当前环境未安装，未执行。

Go 专项审查最初发现 Tamper 安全 HTTP 客户端存在“校验后按 hostname 二次解析”及环境代理绕过风险。
现已改为解析、全地址校验和固定 IP 拨号同一步完成，并在公网限制模式禁用环境代理；
对应固定 IP、混合解析和代理禁用测试通过。复审未发现 CRITICAL/HIGH，结论为 Approve。

## 5. 发布判定

代码层 P1 阻断已修复，但以下外部证据仍未完成：

1. 受控 DNS 变化环境下的云端连接级复验（本地状态型 resolver 与连接前阻断已通过）；
2. 受控页面真实变化后的差异判定、证据截图和重启恢复。

真实 Quake Extension 采集、Web/调度合并、SQLite 持久化及飞书图片送达已经完成，不再列为
发布阻断；但一次普通搜索结果截图与“受控页面发生变化后自动生成证据”的巡检语义不同，
不能据此启用自动巡检证据截图。

因此当前可进入候选版本验证，但不能宣称“安全收口完成”，也不能启用自动巡检证据截图。
