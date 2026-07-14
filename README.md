# UniMap

多引擎网络空间资产查询与网页巡检工具，提供 Web、CLI 和可选 GUI 入口。

## 能力概览

- UQL 统一查询、结果归并与 CSV/Excel/JSON 导出。
- 稳定 Web UI 支持 FOFA、Hunter、ZoomEye、Quake、Shodan 五个引擎；Censys、DayDayMap 已有适配器与扩展选择器，但尚不在稳定 UI 引擎列表内。
- CDP 与 Chrome Extension 双截图引擎，可在 `cdp`、`extension`、`auto` 间切换。
- 网页巡检：`strict`、`relaxed`、`security`、`balanced`、`precise` 五种模式。
- 巡检历史：支持 URL、类型、模式、关键词过滤，以及受限的 `limit` / `offset` 分页；详见 [API 文档](docs/API.md)。
- 调度、通知、分布式节点、备份、Prometheus 指标与操作历史。

## 技术栈

- Go 1.26.5
- Web：`net/http`、`gorilla/websocket`、`go-resty`
- CLI：Go 标准库 `flag`
- GUI：Fyne v2（仅在 `-tags gui` 下构建）
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
```

GUI：

```bash
go run -tags gui ./cmd/unimap-gui
```

完整命令见 [快速开始](docs/QUICKSTART.md)，接口见 [API 文档](docs/API.md)，运维见 [Runbook](docs/RUNBOOK.md)。

## 验证

```bash
go test -race ./...
```

## 合规与安全

仅对拥有授权的目标、账户和数据执行查询、截图或巡检。凭证必须通过部署环境或受控配置提供，禁止提交、记录或共享真实 API Key、管理令牌、Bridge token 和 Cookie。
