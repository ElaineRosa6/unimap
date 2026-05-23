# 通知系统 + FOFA 自定义 API 实施评估报告

> **评估日期：** 2026-05-23
> **被评估分支：** `260522-feat-notification-fofa-api-s1-s13`
> **被评估提交：** `0819f07` — `feat: scheduled task push notification + FOFA custom API (S-1 to S-13)`
> **对照文档：** `docs/PLAN_NOTIFICATION_AND_FOFA_API_2026-05-22.md`
> **结论：** 主体功能已落地，但存在 **1 项实质性回归** 与一组验收清单未达成；合入前建议补 fix，否则 CI 将永久红。

---

## 1. 实施完成度概览

单 commit `0819f07` 覆盖 plan 中 S-1 ~ S-13 全部 13 个步骤，新增/修改 28 个文件（+1804 / −188）。`go build ./...` 通过；功能链路（urlguard → notify → registry → scheduler → handlers → UI）端到端连通。

| Step | 状态 | 说明 |
|------|------|------|
| **S-1** urlguard 包 | ⚠️ 有缺陷 | 文件齐全；但 `Check()` 强制 DNS 解析，离线即 fail。覆盖率 **72.1%**（plan 目标 ≥90%） |
| **S-2** 迁移 alerting/scheduler 到 urlguard | ✅ | 行为保持不变 |
| **S-3** config Notifications 顶层块 | ✅ | 类型定义 + Validate 完整 |
| **S-4** notify 基础包（接口 + Log + Webhook + Registry） | ⚠️ 测试不足 | 完成；覆盖率 **66.7%**（plan 目标 ≥85%） |
| **S-5** DingTalk / Feishu / WeCom 三个 bot + 签名 | ✅ | HMAC-SHA256 签名实现到位 |
| **S-6** server 注入 registry + cfgProvider | ✅ | `SetNotifyRegistry` / `SetNotifyCfgProvider` 已注入 Scheduler |
| **S-7** scheduler.sendNotification 重写 | ✅ | `builtin-log`、`__task_inline_webhook__`、`wg`、两级开关、旧 `Channels[]` → `ChannelIDs[]` 迁移均到位 |
| **S-8** scheduler_handlers notifications 字段 | ✅ | +96 行字段解析 + 校验 |
| **S-9** scheduler.html 通知面板 | ✅ | `/api/notifications/channels` 多选下拉、总开关、3 事件 checkbox 齐全 |
| **S-10** FOFA 字段拆分 | ⚠️ 静默迁移 | 字段拆好；但 **plan §3.4 要求的两条 WARN 日志缺失**（详见 §3.2） |
| **S-11** 3 个 main.go 入口 | ✅ | `NewFofaAdapter` 入参改 `APIBaseURL` |
| **S-12** settings.html FOFA 分组 | ✅ | 两行布局，`web_base_url` 禁用 + 说明文案 |
| **S-13** config.yaml.example | ✅ | notifications 块（dingtalk/feishu/wecom/webhook）+ FOFA 双字段示例 |

---

## 2. 关键架构交付物核对

| 交付物 | 文件 | 实际状态 |
|--------|------|---------|
| `notify.NotifyChannel` 接口 | `internal/notify/channels.go:149` | ✅ |
| Bot 渠道实现 | `internal/notify/bot_channels.go` (276 行) | ✅ DingTalk + Feishu + WeCom |
| 签名算法 | `internal/notify/signature.go` (33 行) | ✅ |
| Registry | `internal/notify/registry.go` (124 行) | ✅ |
| TaskNotification | `internal/notify/message.go` (15 行) | ✅ |
| Scheduler 注入点 | `internal/scheduler/scheduler.go:319-355` | ✅ `notifyRegistry` + `notifyCfgProvider` 字段 |
| 旧字段迁移 | `internal/scheduler/scheduler.go:1051,1109,1112` | ✅ `__task_inline_webhook__`、`builtin-log` |
| `/api/notifications/channels` | `web/notification_handlers.go` (57 行) | ✅ |
| 前端 channel_ids 字段 | `web/templates/scheduler.html:382,541,581` | ✅ |
| FOFA 常量 | `internal/model/unimap.go:4` | ⚠️ plan 要求放在 `internal/adapter/fofa.go`（功能等效） |
| 硬编码 URL 替换 | `internal/screenshot/manager.go:405,733`、`router.go:594`、`service/screenshot_app_service.go:438` | ✅ 改为 `model.FOFAOfficialWebURL` |

---

## 3. 实质性问题清单（按严重度排序）

### 3.1 🔴 P0 — urlguard.Check 在配置 validate 阶段强制 DNS 解析

**位置：** `internal/utils/urlguard/check.go:73`

```go
ips, err := resolver.LookupIPAddr(ctx, host)
if err != nil {
    return fmt.Errorf("urlguard: DNS lookup failed for %q: %w", host, err)
}
```

**调用链：** `Manager.Load()` → `config.Validate()` → `urlguard.Check("https://fofa.info", ...)` → `LookupIPAddr` 同步等待 5 秒后 fail。

**实际后果（已观测）：**

```text
--- FAIL: TestHotUpdateManager_Start_Enabled (6.24s)
    hot_update_test.go:86: failed to load config: invalid config:
        fofa api_base_url validation failed: urlguard: DNS lookup failed
        for "fofa.info": lookup fofa.info: i/o timeout

--- FAIL: TestCheck_ValidHTTPS (5.00s)
--- FAIL: TestCheck_ValidHTTP (5.00s)
--- FAIL: TestCheck_AllowedHostsRegex (5.00s)
```

**影响范围：**

- 任何离线 / 受限网络 / CI 沙箱环境无法启动应用
- DNS 临时故障时整个服务进不去
- 自家 urlguard 测试在无外网环境同样挂

**修复方案（建议方向）：**

| 方案 | 利 | 弊 |
|------|----|----|
| A. `Check()` 只做语法 + scheme + IP 字面量校验；DNS 解析挪到 `SafeHTTPClient.DialContext`（已有） | 修复最小，校验时机最合理 | 失去"配置启动期早发现内网渗透"能力 |
| B. 新增 `CheckOptions.SkipDNS bool`，validate 调用时传 `SkipDNS: true` | 保留 strict 模式给发送端使用 | 多一个开关，调用方要记得开 |
| C. DNS 失败降级为 WARN 日志 + 放行 | 改动小 | 攻击者写一个不可解析的私网 host 会绕过 |

**推荐 A**：DNS rebinding 的真正防线本来就在 `SafeHTTPClient.DialContext`（已实现），重复校验不合理。

### 3.2 🟡 P1 — FOFA S-10 两条 WARN 日志缺失

**位置：** `internal/config/config.go:551-558`

```go
// 旧 base_url 迁移
if config.Engines.Fofa.APIBaseURL == "" && config.Engines.Fofa.BaseURL != "" {
    config.Engines.Fofa.APIBaseURL = config.Engines.Fofa.BaseURL
    // ❌ 缺 logger.Warnf("fofa.base_url 已迁移到 fofa.api_base_url，请更新 config.yaml")
}
// WebBaseURL 本期锁死为官方域名
config.Engines.Fofa.WebBaseURL = "https://fofa.info"
// ❌ 缺 plan §3.4 要求的"用户改了 web_base_url 在 yaml 里 → 被静默覆盖 + WARN"
```

**Plan §3.4 原文：**

> 若 `cfg.Engines.Fofa.APIBaseURL == "" && cfg.Engines.Fofa.BaseURL != ""`:
>     `cfg.Engines.Fofa.APIBaseURL = cfg.Engines.Fofa.BaseURL`
>     `logger.Warnf("fofa.base_url 已迁移到 fofa.api_base_url，请更新 config.yaml")`

**修复：** 两处分别加 `logger.Warnf`，在 reload 时（非首次启动）做"用户配置的 WebBaseURL 与官方域名不一致"的对比，对比失败时也 WARN。

### 3.3 🟡 P1 — Plan §9 验收清单四项未达成

| 项目 | 状态 | Plan 引用 |
|------|------|----------|
| `go test -race -count=1 ./...` 全包通过 | ❌ `internal/config` 与 `internal/utils/urlguard` 离线失败 | §9 |
| `internal/utils/urlguard/` 覆盖率 ≥ 90% | ❌ 实际 72.1% | §9 |
| `internal/notify/` 覆盖率 ≥ 85% | ❌ 实际 66.7% | §9 |
| 文档与 memory 已更新 | ❌ `docs/API.md`、`docs/RUNBOOK.md`、`memory/MEMORY.md`、`memory/project_notify_fofa_2026-05-22.md` 全部未更新 | §9 / F-24~F-27 |

### 3.4 🟢 P2 — 常量位置与 plan 不符

Plan §3.2 / F-22 指定 `FOFAOfficialWebURL` 放在 `internal/adapter/fofa.go`；实际放在 `internal/model/unimap.go:4`。

**影响：** 仅"约束模型/适配器层不互相 import"原则上略奇怪；功能完全等效，使用方都改 import 即可。

---

## 4. 验证矩阵

### 4.1 包级测试结果

| 包 | 结果 | 覆盖率 | 备注 |
|----|------|--------|------|
| `internal/utils/urlguard` | ❌ FAIL | 72.1% | DNS lookup 离线失败 3 个用例 |
| `internal/notify` | ✅ PASS | 66.7% | 未达 plan 85% 目标 |
| `internal/scheduler` | ✅ PASS | — | race 检测干净 |
| `internal/alerting` | ✅ PASS | — | |
| `internal/adapter` | ✅ PASS | — | |
| `internal/config` | ❌ FAIL | — | `TestHotUpdateManager_Start_Enabled` DNS 超时 |
| `go build ./...` | ✅ | — | |

### 4.2 端到端（手工，未执行）

Plan §7.5 要求：

- [ ] 创建定时任务勾选钉钉/飞书/企微 → 触发 → 群里确认收到（**未验证**）
- [ ] 关闭全局 notifications.enabled → 任务执行不发（**未验证**）
- [ ] 修改 fofa.api_base_url 为镜像 → 重启 → API 走新域名，截图仍走 fofa.info（**未验证**）

---

## 5. 合入前最小修复建议

按优先级，三件事建议作为一次后续 hotfix commit 合在一起：

1. **修 urlguard DNS 严格性**（P0）
   - `urlguard/check.go`：把 `LookupIPAddr` 块移除；DNS 校验保留在 `SafeHTTPClient.DialContext`（已经在做）
   - `urlguard/check_test.go`：移除依赖外网的 3 个用例（或改成 `127.0.0.1` 之类的 IP 字面量）

2. **补 S-10 两条 WARN 日志**（P1）
   - `internal/config/config.go:551-558`：迁移分支 + Web 覆盖分支各加 `logger.Warnf`

3. **补 plan §9 文档与 memory**（P1）
   - `docs/API.md` 加通知接口章节（参考 ICP `/api/icp/history` 写法）
   - `docs/RUNBOOK.md` 加"通知没收到时排查清单"
   - `memory/MEMORY.md` 加索引行
   - `memory/project_notify_fofa_2026-05-22.md` 落地报告（plan F-27）

覆盖率提升（urlguard → 90%、notify → 85%）建议作为单独一次 commit，避免和功能修复混在一起。

---

## 6. 合入决策记录

- **本次合并选择：** 仍按用户决定合入 master（feature → master 合并提交保留分支历史）
- **遗留风险：** master 合并后 CI 在离线环境将红，直到 P0 修复落地
- **跟进 owner：** 待指派
- **跟进 due：** 建议合入后 1 个工作日内推 P0 hotfix；P1 文档可在 1 周内补齐
