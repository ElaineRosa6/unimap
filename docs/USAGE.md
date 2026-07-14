# UniMap 使用指南

> 最后按代码核对：2026-07-13。Web、CLI 和 GUI 共用 `configs/config.yaml` 与配置管理器；配置保存不是“仅内存”。

## 启动方式

```bash
# Web
go run ./cmd/unimap-web

# CLI
go run ./cmd/unimap-cli --help

# GUI（必须带 build tag）
go run -tags gui ./cmd/unimap-gui
go build -tags gui -o unimap-gui ./cmd/unimap-gui
```

GUI 构建所需的系统图形依赖见 [GUI_BUILD.md](GUI_BUILD.md)。

## 配置引擎

复制 `configs/config.yaml.example` 为 `configs/config.yaml`，再配置所需引擎。稳定 Web UI 支持 FOFA、Hunter、ZoomEye、Quake、Shodan；CLI/服务端还可注册已配置的 Censys 与 DayDayMap。

GUI 的引擎配置对话框在保存时调用配置管理器的 `Save()`，会写入当前配置文件。配置文件和备份可能包含敏感值，应限制访问权限；不要在截图、日志、Issue 或聊天记录中粘贴真实凭证。

API Key 获取与配额属于外部平台策略，使用前应在对应平台确认当前套餐、API 权限和使用条款。不要通过注册多个账号规避配额或条款。

## 查询

UQL 示例：

```text
country="CN" && port="80"
title="login" && (port="80" || port="443")
port IN ["80", "443", "8080"]
```

CLI：

```bash
go run ./cmd/unimap-cli -q 'country="CN" && port="80"' -e fofa,hunter -l 100
go run ./cmd/unimap-cli -q 'title="login"' -e fofa,hunter,quake -o result.xlsx
```

Web API 使用 `POST /api/v1/query`，请求参数是表单字段 `query`、可选 `engines` 与 `page_size`（最大 500），不是旧版 JSON `limit/offset` 协议。详情见 [API.md](API.md)。

## 截图与巡检

- 截图模式可在 `cdp`、`extension`、`auto` 间切换。
- 单 URL 截图、批量截图和巡检均拒绝私有、loopback 与内部目标地址。
- 巡检模式为 `strict`、`relaxed`、`security`、`balanced`、`precise`；历史名称 `malicious`、`performance`、`full` 不再有效。
- 扩展配对与 Bridge 故障排查见 [OPS_SCREENSHOT_EXTENSION.md](OPS_SCREENSHOT_EXTENSION.md)。

## GUI 页面

当前 GUI 原生页面包括：资产查询、URL 监控、历史记录和截图管理。GUI 的引擎复选框与 Web 稳定引擎集合并不完全相同；以运行时界面及配置为准，不要将旧版示意图视为完整功能清单。

## 常见问题

### 查询没有可用引擎

确认引擎 `enabled`、API Key、配额与网络连通性；可通过 `-e` 显式选择 CLI 引擎。

### Web 可访问但截图失败

检查 `/api/v1/cdp/status` 与 `/api/v1/screenshot/router/status`，然后参照 [RUNBOOK.md](RUNBOOK.md) 排查 CDP、Extension、配对和 token。

### GUI 无法编译

确认使用 `-tags gui`，并安装本机平台所需的 C/C++ 与 OpenGL 图形依赖。
