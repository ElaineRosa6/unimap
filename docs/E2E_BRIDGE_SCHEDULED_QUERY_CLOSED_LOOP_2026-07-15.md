# 2026-07-15 查询通知与 Bridge 定时闭环验收

> **日期化验收记录**：本文只说明 2026-07-15 本机受控会话的结果，不承诺任意部署、账号或未来页面版本。未记录 API Key、Cookie、Bridge token、通知凭证或具体资产明细。
>
> **2026-07-23 范围勘误**：本文的结构化采集证据来自 Extension Bridge，不是 CDP。测试白名单只有 FOFA、Hunter、ZoomEye、Quake、Shodan；Censys、DayDayMap 当时只完成 API 适配/验证，没有 Bridge/CDP 抓取通过记录。

## 本次修订

定时 `query` 的成功通知不再只包含总数和各引擎计数。API 与 Bridge 查询共用同一资产明细格式，默认展开 50 条、最多 100 条，并有约 20 KiB 的正文安全上限；超过上限的资产仍全部持久化，并在通知中说明未展开数量。响应头、正文片段和任意 `extra` 字段不会写入通知。

Bridge 模式下，同一个任务完成：UQL 翻译 → API 查询（适配器可用时）→ Bridge `collect_and_capture` 结构化采集和截图 → API/Bridge 资产合并 → SQLite 单条查询历史与结果明细 → 文字明细和图片通知。

## 自动化证据

- `TestQueryRunner_Execute_APIQueryIncludesAssetDetailsForNotification`：普通 API 查询结果进入通知正文。
- `TestQueryRunner_Execute_QueryNotificationDetailLimit`：通知展开限制不影响完整持久化语义。
- `TestQueryRunner_Execute_BridgeCollectCapturePersistsCombinedResults`：API 与 Bridge 资产、截图路径均进入实际通知载荷，SQLite 保存合并明细。
- `TestLiveAPIScheduledQueryNotificationDetails`：使用当前 FOFA API 配置实测 10 条结果持久化、通知正文含明细、飞书应用投递成功。
- `TestLiveBridgeScheduledQueryClosedLoop`：逐引擎验证结构化结果、PNG、SQLite 明细、通知正文资产和飞书成功指标；PNG 另行目视检查登录态与结果区。

## Bridge 五引擎实测状态

| 引擎 | 结构化结果 / SQLite | 截图 | 明细通知 | 结论 |
| --- | ---: | ---: | --- | --- |
| FOFA | 10 条 | 377,765 字节，登录结果页 | 正文含持久化资产；飞书成功 | ✅ |
| Hunter | 9 条 | 248,281 字节，登录结果页 | 正文含持久化资产；飞书成功 | ✅ |
| Quake | 10 条 | 358,706 字节，登录结果页 | 正文含持久化资产；飞书成功 | ✅，同时修复端口翻译 |
| ZoomEye | 10 条 | 623,657 字节，实际结果页 | 正文含持久化资产；飞书成功 | ✅；首次截图读回瞬时失败，复测通过 |
| Shodan | 0 条 | 101,214 字节，登录页 | 未发送成功通知 | ❌ 当前搜索登录态失效，不能判为代码通过 |

Quake 首次以旧翻译 `port:"443"` 打开页面时返回 0 条。实际页面验证当前端口数值语法应为 `port:443`；适配器和回归测试已同步修复。Shodan 的 Bridge 已恢复连接并回传截图，但页面明确重定向至登录表单，因此本轮没有伪造全绿结论。

## 复测命令

普通 API 调度查询：

```powershell
$env:UNIMAP_LIVE_API_ENGINE = 'fofa'
go test -tags live_bridge_e2e ./web -run '^TestLiveAPIScheduledQueryNotificationDetails$' -count=1 -v
```

Bridge 逐引擎闭环：

```powershell
$env:UNIMAP_LIVE_BRIDGE_ENGINE = 'hunter' # fofa/hunter/quake/zoomeye/shodan
$env:UNIMAP_LIVE_BRIDGE_ARTIFACT_DIR = 'C:\tmp\unimap-live-bridge-diagnostics'
go test -tags live_bridge_e2e ./web -run '^TestLiveBridgeScheduledQueryClosedLoop$' -count=1 -v
```

扩展必须已配对，浏览器必须具备对应引擎的有效搜索登录态，且通知接收方已同意接收真实测试消息。验收必须同时满足：结构化结果非空、SQLite 明细非空、调度结果含资产明细、截图不是登录页、通知成功；仅有非空 PNG 或任务记录不算通过。

## 失败语义

- `browser_query=true` 的定时闭环只接受 `browser_action=collect_and_capture`。
- Bridge 采集结果、每个引擎的截图、历史数据库任一不可用时，任务返回失败，不报告假成功。
- API 查询失败但 Bridge 对所有指定引擎采集和截图完整时，任务可成功；API 错误作为部分结果写入历史摘要。
- 通知发生在结果持久化之后。通知失败记录日志和 Prometheus 失败计数，但不会回滚已经提交的查询结果。
