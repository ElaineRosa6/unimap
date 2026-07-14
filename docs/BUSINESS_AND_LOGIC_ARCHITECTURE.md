# UniMap 业务与逻辑架构

> 当前状态：2026-07-13，基于 `develop` 工作区代码核对。本文替代 2026-06-05 的旧分支快照；接口细节见 [API.md](API.md)。

## 业务能力

```text
用户（Web / CLI / GUI / Extension）
                  │
资产查询 ── 截图 ── 巡检 ── 调度 ── 通知 ── 分布式节点
                  │
        查询适配器 / Browser / 基线和历史 / SQLite 与文件存储
```

| 业务域 | 当前职责 | 主要边界 |
|---|---|---|
| 统一查询 | UQL 解析、引擎翻译、并发查询、归并、导出 | `core/unimap`、`adapter`、`service` |
| 截图 | CDP/Extension、健康检查、降级、批次任务 | `screenshot`、Bridge API |
| 巡检 | URL 可达性、端口扫描、篡改检测、基线与历史 | `tamper`、`service`、`web` |
| 调度 | cron/once/delay、执行历史、通知 | `scheduler` |
| 分布式 | 节点注册/心跳、任务队列、故障转移 | `distributed` |
| 认证与运维 | Session/API Key/RBAC、配置、备份、指标 | `auth`、`config`、`backup`、`metrics` |

## 引擎范围

Web UI 的稳定引擎为 FOFA、Hunter、ZoomEye、Quake、Shodan。Censys 和 DayDayMap 可由服务端/CLI 在已配置时注册，扩展也有选择器，但不应被描述为稳定 UI 能力。

## 核心逻辑

### 查询

```text
UQL → Parser → EngineOrchestrator → 多个 EngineAdapter
    → 标准化 UnifiedAsset → 归并/缓存 → Web、CLI、GUI、导出
```

Web API 查询是 `POST /api/v1/query` 的表单协议；最大 `page_size` 为 500。浏览器采集是可选并行路径，不替代 API 适配器。

### 截图与巡检

```text
目标 URL → SSRF 检查 → ScreenshotRouter(CDP/Extension/auto)
       → 截图或网页内容 → tamper Detector → 基线/历史/通知
```

巡检模式为 `strict`、`relaxed`、`security`、`balanced`、`precise`。私有、回环和内部地址会被拒绝；这不是可配置的内网扫描能力。

### 调度与通知

`ScheduledTask` 支持 `cron`、`once`、`delay`。当前工作区定义 23 种任务类型，包含尚未提交的备份任务变更；发布说明必须随提交状态确认。通知通道实际由注册表决定，不能把文档中的数字当作静态协议。

### 分布式

```text
节点注册/心跳 → NodeRegistry
节点领取/回传 → NodeTaskQueue
管理员查看状态 → /api/v1/nodes/*
```

没有独立 `unimap-node` 二进制。节点客户端应调用既有 HTTP 协议，并使用节点令牌；管理员操作使用分布式管理令牌。

## 安全与运维原则

- 唯一业务 API 前缀是 `/api/v1`；没有旧 `/api` 兼容 shim。
- 管理配置、操作历史、备份等管理性资源由 handler 执行角色校验。
- Bridge 配对与回调只允许 loopback；配对 token 和管理 token 不得落入文档、日志或工单。
- `$VAR`/`${VAR}` 是显式配置值引用，而不是“环境变量永远优先”的全局覆盖规则。
- `/metrics` 在认证开启时要求管理令牌；未认证的非 loopback 部署会拒绝指标访问。

## 参考

- [架构](ARCHITECTURE.md)
- [API](API.md)
- [运维 Runbook](RUNBOOK.md)
- [插件架构](PLUGIN_ARCHITECTURE.md)
