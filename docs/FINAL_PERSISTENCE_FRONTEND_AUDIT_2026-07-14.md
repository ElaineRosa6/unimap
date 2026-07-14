# 2026-07-14 持久化与前后端终检

> **日期化验收记录**：本文描述 2026-07-14 工作区代码与本机测试的结论。它不是未来部署状态的保证，也不包含真实凭证、Cookie、令牌、通知地址或资产信息。

## 结论

- 代码中设计为持久化的配置、账户、历史、截图批次元数据、巡检记录、调度、告警、分布式快照和备份目录均存在明确落盘路径；本轮对关键数据执行了写入后重开/重载读取验证。
- 稳定 Web UI 使用的 `/api/v1/...` 调用与 `web/router.go` 的注册路由逐项核对，未发现生产前端引用旧 `/api/...` shim 或调用不存在的路由。
- 修复了账户管理、巡检、端口扫描、调度和通知设置中少数服务端字段直接拼接 HTML 的问题。现在这些字段会被转义、数值化或限制到已知枚举后再渲染；损坏的本地配额设置和非 JSON 错误响应也不再被静默吞掉。

## 持久化边界与验证

| 数据类别 | 存储方式 | 重启/重载验证 |
| --- | --- | --- |
| 运行配置、Cookie 与通知渠道 | YAML 配置文件 | `TestSaveAndLoadPreservesOperationalConfiguration` |
| 用户账户 | SQLite | `TestUserPersistsAfterDatabaseReopen` |
| API Key 元数据 | 文件存储 | 现有 `TestNewAPIKeyManager/loads_from_existing_storage` |
| 查询/端口扫描历史及结果 | SQLite | `TestHistoryAndResultsPersistAfterDatabaseReopen` |
| ICP 查询运行与结果 | SQLite | `TestICPResultsPersistAfterDatabaseReopen` |
| 截图批次任务元数据 | SQLite；图片为受控目录文件 | `TestBatchJobPersistsAfterDatabaseReopen`；现有截图文件列表测试 |
| 巡检基线与检测记录 | 文件存储 | `TestHashStorage_SaveAndLoadBaseline`、`TestHashStorage_SaveAndLoadCheckRecords` |
| 调度任务与执行历史 | JSON 文件 | `TestSchedulerE2E_Persistence`、`TestStorePersist`、`TestAtomicPersist_RoundTrip` |
| 告警记录与静默状态 | JSON 文件 | `TestManager_PersistsAndRestoresAlertRecords`、`TestManager_PersistsSilencedAlert` |
| 分布式节点/队列快照 | JSON 文件 | `TestSnapshotManager_Load` |
| 备份归档 | 文件系统 tar.gz | 现有 `TestBackup_CreatesTarGz`、`TestListBackups_FindsBackups` |

以下状态按设计是短暂的，不应承诺跨进程保存：进行中的 WebSocket 查询、内存缓存、正在执行的浏览器/CDP/Bridge 任务、临时轮询器与 Prometheus 进程指标。持久化的任务定义、配置和完成记录不受此限制。

## 前后端契约与渲染检查

- 路由注册通过 `addAPIRoute` 统一映射到 `/api/v1/...`；生产模板与 `web/static/js/main.js` 的 API 调用均有对应路由。
- JavaScript 语法检查覆盖 `web/static/js/main.js` 和所有模板中的内联脚本。
- 查询结果、历史、ICP、批量截图和巡检结果的外部字段使用文本节点或 HTML/属性转义后写入 DOM。
- 本轮修复的模板为 `account.html`、`monitor.html`、`port-scan.html`、`scheduler.html` 与 `settings.html`。

通用静态扫描会将所有 `innerHTML` 赋值标记为 XSS，因此其原始 P0 计数不是已确认漏洞数。本次对生产前端所有该类写入点回溯了输入来源；固定文案、已转义字段和用 DOM API 写入的文本不构成该规则所描述的注入路径。

## 已执行验证

```powershell
go test -race -count=1 ./internal/config ./internal/auth ./internal/history ./internal/icp/database ./internal/screenshot/batchdb ./internal/alerting ./internal/scheduler ./internal/tamper ./internal/distributed
go test -race -count=1 ./web
go test -race -count=1 ./...
go vet ./...
go build ./...
node --check web/static/js/main.js
```

此外重新运行了 API 契约扫描和前端静态扫描。扫描器仍会报告历史文档、扩展测试和通用 `innerHTML` 规则命中；这些项目没有作为生产缺陷计入结论。

## 仍需在部署环境完成的人工验收

本记录验证了代码、路由、持久化重载和 JavaScript 语法；它不能替代每种浏览器、屏幕尺寸、反向代理/CORS 配置以及真实权限角色下的人工 UI 验收。Bridge 搜索截图与通知的受控真实联调见 [2026-07-14 Bridge 截图与通知验收](E2E_BRIDGE_SCREENSHOT_NOTIFICATION_2026-07-14.md)。
