# 2026-07-15 Bridge 定时查询闭环验收

> **日期化验收记录**：本文只说明 2026-07-15 本机受控会话的结果，不承诺任意部署、账号或未来页面版本。未记录 API Key、Cookie、Bridge token、通知凭证或资产目标。

## 验收链路

同一个 `query` 定时任务完成以下步骤：

1. 执行 UQL 查询；即使目标引擎没有注册 API 适配器，仍使用无凭证浏览器翻译器生成引擎查询语法。
2. 通过 Extension Bridge 的 `collect_and_capture` 在一次页面导航中获取结构化资产并截图。
3. 将 API 与 Bridge 资产合并为一条查询历史，使用事务写入 SQLite 历史头和结果明细。
4. 将实际存在的 PNG 路径放入调度执行结果，通知层提取图片并发送到已启用的飞书应用渠道。
5. 调度执行历史在任务完成后立即写入 JSON，而不依赖后续手工 `Save` 或任务配置变更。

## 实测结果

| 项目 | 结果 |
| --- | --- |
| 引擎 | FOFA |
| Bridge 结构化采集 | ✅ |
| Bridge PNG | ✅，360,996 字节 |
| SQLite 查询历史 | ✅，单条合并历史 |
| SQLite 结果明细 | ✅，10 条 |
| 飞书应用图片通知 | ✅ |
| 任务最终状态 | `success` |

执行命令：

```powershell
go test -tags live_bridge_e2e ./web -run '^TestLiveBridgeScheduledQueryClosedLoop$' -count=1 -v
```

关键成功输出：

```text
LIVE_BRIDGE_CLOSED_LOOP success engine=fofa persisted_results=10 screenshot_bytes=360996 notification_success=true
```

## 失败语义

- `browser_query=true` 的定时闭环只接受 `browser_action=collect_and_capture`。
- Bridge 采集结果、每个引擎的截图、历史数据库任一不可用时，任务返回失败，不报告假成功。
- API 查询失败但 Bridge 对所有指定引擎采集和截图完整时，任务可成功；API 错误作为部分结果写入历史摘要。
- 通知发生在结果持久化之后。通知失败记录日志和 Prometheus 失败计数，但不会回滚已经提交的查询结果。

## 复测范围

默认引擎为 FOFA。可设置 `UNIMAP_LIVE_BRIDGE_ENGINE` 为 `hunter`、`quake`、`zoomeye` 或 `shodan` 后执行相同测试。扩展必须已配对，浏览器必须具备对应引擎的有效登录态，并且通知接收方已同意接收真实测试消息。
