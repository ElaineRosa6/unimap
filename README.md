# UniMap

多引擎网络空间资产查询与网页巡检工具，提供 Web 和 CLI 入口。

## 能力概览

- UQL 统一查询、结果归并与 CSV/Excel/JSON 导出。
- 稳定 Web UI 支持 FOFA、Hunter、ZoomEye、Quake、Shodan 五个引擎。Censys、DayDayMap 当前只完成服务端/CLI API 适配与 API 实机验证；虽然仓库内存在部分泛化选择器占位，但尚未接入稳定 UI，也未完成 Bridge/CDP 结构化抓取与真实测试。
- CDP 与 Chrome Extension 双截图引擎，可在 `cdp`、`extension`、`auto` 间切换。
- Linux/容器可直接使用无 `DISPLAY` 的 Chromium headless；统一路由覆盖截图、浏览器采集、调度与巡检，并限制浏览器会话避免云主机 OOM。
- 历史测绘引擎结构化采集 E2E 使用 Extension Bridge；Quake、Hunter 已有真实 CDP 结构化采集与截图证据，Quake L1 Network 已于 2026-07-29 重新实测通过；FOFA、ZoomEye、Shodan 仍待逐引擎定级。
- CDP 浏览器任务采用 URL/DNS 校验、全请求 Fetch 拦截和连接级 loopback 出口代理；未证明受控出口的 Extension 任意 URL 截图、外部上游代理和远程 Chrome 会失败关闭。
- 网页巡检：`strict`、`relaxed`、`security`、`balanced`、`precise` 五种模式。
- 巡检历史：支持 URL、类型、模式、关键词过滤，以及受限的 `limit` / `offset` 分页；详见 [API 文档](docs/API.md)。
- 调度、通知、分布式节点、备份、Prometheus 指标与操作历史。

当前明确未完成项和优先级见 [本地剩余工作清单](docs/REMAINING_WORK_2026-07-23.md)。

## 与 UniMap v2 的关系

[UniMap v2](../unimap-v2/) 是面向 AI Agent 的轻量侦察 Skill（MCP Server + PDTools），与本项目是**分层互补**关系：

- **本项目（v1）**：人操作平台 — Web UI、截图、巡检、调度、Bridge/CDP 浏览器自动化
- **UniMap v2**：AI Agent 工具 — MCP 协议、PDTools 工具链、轻量引擎执行、工作流编排

两者共享统一资产模型（`UnifiedAsset`），独立演进。需要浏览器结构化采集、截图、持续监控的场景使用本项目；需要 AI Agent 自动化侦察、红队信息收集的场景使用 v2。

## 技术栈

- Go 1.26.5
- Web：`net/http`、`gorilla/websocket`、`go-resty`
- CLI：Go 标准库 `flag`
- 浏览器自动化：chromedp；解析：goquery；存储：SQLite/YAML；缓存：内存/Redis

## 快速开始

```bash
cp configs/config.yaml.example configs/config.yaml
# 编辑 configs/config.yaml，使用 ${ENV_VAR} 占位符或受控的本地直接值配置凭证

go run ./cmd/unimap-web
# 打开 http://localhost:8448
```

CLI 示例：

```bash
go run ./cmd/unimap-cli -q 'country="CN" && port="80"' -e fofa,hunter -l 100

# API 子命令（默认认证开启时推荐 token 文件）
go run ./cmd/unimap-cli query --admin-token-file ./.secrets/unimap-admin-token -q 'port="443"' -e fofa
go run ./cmd/unimap-cli scheduler --admin-token-file ./.secrets/unimap-admin-token list
go run ./cmd/unimap-cli screenshot-batch --admin-token-file ./.secrets/unimap-admin-token --urls https://example.com
```


```bash
```

完整命令见 [快速开始](docs/QUICKSTART.md)，接口见 [API 文档](docs/API.md)，运维见 [Runbook](docs/RUNBOOK.md)。2026-07-17 的逻辑、API 与前后端交互修复状态见[问题报告](docs/CODE_LOGIC_API_UX_REVIEW_2026-07-17.md)和[修复/回滚指南](docs/REMEDIATION_GUIDE_2026-07-17.md)。

无图形界面云主机的容器和原生部署要求、readiness 判断与最终验收清单见 [Runbook 的 headless 章节](docs/RUNBOOK.md#无图形界面的-linux--云主机)。

## 验证

```bash
go test -race ./...

# 显式真实浏览器测试（默认测试不会启动 Chrome）
UNIMAP_CHROME_PATH=/path/to/chrome go test -tags=headless_e2e ./internal/screenshot ./internal/tamper
```

## 合规与安全

仅对拥有授权的目标、账户和数据执行查询、截图或巡检。凭证必须通过部署环境或受控配置提供，禁止提交、记录或共享真实 API Key、管理令牌、Bridge token 和 Cookie。
