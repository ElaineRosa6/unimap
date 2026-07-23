# 2026-07-14 Bridge 搜索截图与通知验收

> **历史验收快照**：本记录仅说明 2026-07-14 本机受控会话中的结果，不代表任意部署、账号或后续页面版本。未记录 API Key、Cookie、Bridge token、通知凭证或资产目标。
>
> **2026-07-23 范围勘误**：本文验证的是 Extension Bridge，不是 CDP。Censys、DayDayMap 不在本次范围内，当前也没有完成它们的 Bridge/CDP 抓取与 live E2E。

## 范围与边界

- 临时 UniMap 服务仅监听 `127.0.0.1:8448`。
- Chrome Extension Bridge 使用已配对的本机浏览器会话；测试仅验证稳定 Web 引擎范围内的 Hunter、Quake、ZoomEye。
- 每个引擎运行一次 `search_screenshot` 任务，检查 Bridge 回调、非空 PNG 和飞书应用图片通知成功计数。
- PNG 仅在显式设置 `UNIMAP_LIVE_BRIDGE_ARTIFACT_DIR` 时保留到本地诊断目录；默认测试不保留图片，也不修改原配置或持久化任务。

## 结果

| 引擎 | 结果页视觉核验 | Bridge/通知 | 备注 |
| --- | --- | --- | --- |
| Hunter | ✅ `port="443"` 查询、资产统计和结果表均已加载 | ✅ 回调、PNG、飞书应用图片通知成功 | 页面显示已登录账号。 |
| Quake | ✅ `port:443` 查询、结果总数和资产详情均已加载 | ✅ 回调、PNG、飞书应用图片通知成功 | 页面显示已登录账号。 |
| ZoomEye | ✅ `port:443` 查询、结果总数和资产卡片均已加载 | ✅ 回调、PNG、飞书应用图片通知成功 | 页头账号文本为 `None`，但未出现登录墙且可返回实际结果；这不证明账号标识本身正常。 |

## 结论与复测方法

本轮未复现“等待不足导致首页或登录页截图”的问题。此前的验收脚本只验证非空图片与通知送达，无法单独证明图片内容；现已支持保留一份仅本地可读的 PNG 用于人工核验。

```powershell
$env:UNIMAP_LIVE_BRIDGE_ENGINE = 'hunter' # 或 quake、zoomeye
$env:UNIMAP_LIVE_BRIDGE_ARTIFACT_DIR = 'C:\tmp\unimap-live-bridge-diagnostics'
go test -tags live_bridge_e2e ./web -run '^TestLiveBridgeSearchScreenshotNotification$' -count=1 -v
```

验收时必须同时确认：查询词与结果区可见、未出现登录墙，以及飞书应用收到对应图片。不要把页面页头账号文本、图片非空或单纯的任务成功状态单独作为完整成功判据。

定时 `query` 任务的“Bridge 采集 + 截图 + SQLite 查询结果 + 图片通知”闭环不属于本历史截图测试范围；后续实测见 [2026-07-15 Bridge 定时查询闭环验收](E2E_BRIDGE_SCHEDULED_QUERY_CLOSED_LOOP_2026-07-15.md)。
