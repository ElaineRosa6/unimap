# 定时任务推送通知 + FOFA 自定义 API 实施计划

> **创建日期：** 2026-05-22
> **分支：** `master`
> **作者：** UniMap 维护组
> **状态：** 📝 待审阅
> **关联文档：**
> - `docs/PLAN_SCHEDULED_ICP_QUERY_2026-05-22.md`
> - `memory/project_icp_scheduled_task_2026-05-22.md`
> - `internal/scheduler/scheduler.go` (NotificationConfig 已存在但未完工)
> - `internal/alerting/channels.go`（已有通用 Webhook 渠道）

---

## 0. 风险总览（务必先读）

本期改动涉及 **两条业务链** 的交叉修改，"依赖失序"是头号风险。把所有改动按 **依赖拓扑顺序** 推进，先解耦再加功能，避免出现"配置改了但运行时没读到 / 运行时读了但配置没下发 / 前端开关切了但后端不认"的链路断裂。

| 风险 | 等级 | 失序场景 | 缓解 |
|------|------|----------|------|
| **R-A：FOFA Web 模式被误用自定义 API 域** | 🔴 高 | 用户把 `base_url` 改成第三方代理后，截图/扩展/Cookie 链路如果错误读到该字段，会去打第三方域名 → cookie 不匹配、登录态丢失、截图 404 | 引入显式字段分离（§3.3），新增静态白名单常量 `FOFA_OFFICIAL_WEB_URL`，所有 Web 链路只能读这个常量或 `WebURL` 字段，**禁止读 `BaseURL`** |
| **R-B：通知 channels 类型与全局 channel 注册表错位** | 🔴 高 | 用户在任务里勾选 "dingtalk"，但全局 channel 未配置 → 静默丢失通知；或全局禁用 → 任务级开关勾了也不发 | 引入"两级开关 = 全局开关 AND 任务开关"语义（§4.5），任务级勾选不存在的全局渠道时返回明确错误 |
| **R-C：旧 `webhook` channel 与新 `webhook_generic`/`dingtalk` 名称冲突** | 🟡 中 | 现有 `ScheduledTask.Notifications.Channels[]` 已有 `"webhook"/"log"/"email"` 字面量，存量任务的 channel 名不能动 | 保留 `"webhook"` 语义不变（=通用 JSON Webhook），**新增**机器人类型用新枚举值（§4.3） |
| **R-D：bot 签名/加签密钥泄漏到 history JSON** | 🟡 中 | 通知 payload 落历史时如果序列化了密钥字段 → 数据备份/日志/API 返回都会暴露 | 密钥字段统一在 `MarshalJSON` 里脱敏，history 永远不写密钥（§4.7） |
| **R-E：通知发送阻塞 scheduler 主循环** | 🟡 中 | 同步调 5 个 channel × 3 秒重试 = 单次任务卡 15 秒，挤占 cron tick | 沿用现有 `sendWebhookNotification` 的 `go func` 异步模式（`scheduler.go:1077`），并加 `WaitGroup` 用于优雅关闭（参考 H-04 修复） |
| **R-F：SSRF（自定义 API/自定义 Webhook）** | 🟡 中 | 用户把 FOFA `base_url` 设成 `http://169.254.169.254/`、或把通知 webhook 设成内网地址 → 凭据外泄 / 内网探测 | 复用 `internal/alerting/channels.go:67` 已有的 IP/DNS 校验逻辑，提炼 `utils/urlguard` 公共包供两处调用（§5.2） |
| **R-G：配置热更新时通知/FOFA 客户端缓存读到陈旧值** | 🟡 中 | `config_watcher` reload 后，scheduler 仍持旧 channel 实例 → 通知发到老 webhook；FOFA adapter 实例仍持旧 baseURL | 沿用 ICP 方案的 `cfgProvider` 闭包模式（§4.6），每次执行实时读 `s.config` 快照 |
| **R-H：定时任务存量数据兼容** | 🟡 中 | 老任务 JSON 里 `Notifications` 为 nil → 反序列化正常；但如果加了必填字段（如 BotType）→ 升级即崩 | 所有新增字段加 `omitempty`，零值即"不通知"（§4.8） |
| **R-I：前端开关与后端字段命名漂移** | 🟢 低 | UI 写 `notification_enabled`，后端读 `notifications.enabled` → 永远关 | 锁定字段契约：见 §4.4 表格 |
| **R-J：CLI/GUI 入口未挂载 scheduler 但配置可填** | 🟢 低 | 用户在 GUI 里看到通知配置以为生效 → 实际不发 | 文档+UI 文案明示"通知仅 Web 模式生效"（与 ICP 方案保持一致） |

---

## 1. 背景

### 1.1 通知现状（已部分实现，未闭环）

`internal/scheduler/scheduler.go:233-269` 已经定义：

```go
type ScheduledTask struct {
    // ...
    Notifications *NotificationConfig `json:"notifications,omitempty"`
}

type NotificationConfig struct {
    OnSuccess  bool
    OnFailure  bool
    OnTimeout  bool
    Channels   []string  // "webhook", "log", "email"
    WebhookURL string
    Recipients []string
}
```

`scheduler.go:997-1042 sendNotification()` 也已实现，但仅支持：
- `"log"` → 标准日志
- `"webhook"` → 通用 JSON POST（无 bot 格式适配）
- `"email"` → 占位未实现

`internal/alerting/channels.go` 提供了通用 `WebhookChannel`，带 SSRF 校验（67-114 行），但仅供 alerting 包内 tamper 告警使用，与 scheduler 通知路径完全分离。

**缺口：**
1. ❌ 前端 `web/templates/scheduler.html` 完全没有暴露 `Notifications` 配置（创建/编辑任务时无法配通知）
2. ❌ `web/scheduler_handlers.go` 未解析 `notifications` 入参（即使前端发来也丢弃）
3. ❌ 不支持企业微信群机器人、钉钉群机器人、飞书群机器人、Slack 这类带"签名/加签/卡片格式"的渠道
4. ❌ 全局通知渠道在 `config.yaml` 中无配置位置（仅 alerting.webhook 一个全局位）

### 1.2 FOFA 自定义 API 现状

`internal/config/config.go:42-52` 已经定义了双字段：

```go
Fofa struct {
    BaseURL   string  // API 模式使用
    WebURL    string  // 计划用于 Web 模式，但代码中从未被消费
    UseWebAPI bool
    // ...
}
```

实际现状：
- `cmd/unimap-web/main.go:102` 把 `BaseURL` 传给 `adapter.NewFofaAdapter` → API 模式正常使用
- `internal/screenshot/manager.go:736` 和 4 处其他位置 **硬编码** `https://fofa.info/result?qbase64=%s` → Web 模式不读任何配置
- `web/templates/settings.html:285` 已经允许 UI 编辑 `base_url` → **目前用户改了 base_url 就能影响 API 模式**，但是没有 web_url 编辑入口
- `internal/config/config.go:893` 校验 `WebURL == ""` 但生产代码不读 → **死字段**

**缺口：**
1. ❌ 用户期望"自定义 API（仅 API 模式）+ 官方 Web（永远固定）"，但当前 `base_url` 字段已被 API 模式使用，命名歧义
2. ❌ Web 链路硬编码官方域名，看似安全，但任何后续重构若误把 `BaseURL` 接入 Web 链路就会破坏隔离
3. ❌ Settings UI 没有明确分区告诉用户"这是 API 端点，仅 API 模式生效"

---

## 2. 目标与非目标

### 2.1 目标 (In Scope)

| 编号 | 目标 |
|------|------|
| G-1 | 配置文件 `config.yaml` 新增 `notifications.channels` 块，支持配置 **N 个全局通知渠道**（DingTalk/Feishu/WeCom/Webhook/Log），每个带独立的 webhook URL、签名密钥、启用开关 |
| G-2 | 新增四个具体 channel 实现：`DingTalkChannel`、`FeishuChannel`、`WeComChannel`、`GenericWebhookChannel`（复用现有 SSRF 校验） |
| G-3 | Scheduler 通知系统改造：`Notifications.Channels[]` 引用全局 channel ID（按名字匹配），不再嵌入 URL；保留旧 `WebhookURL` 字段作为"任务级覆盖"（向后兼容） |
| G-4 | 前端 `scheduler.html` 增加任务级通知开关：可勾选哪些事件（成功/失败/超时）通过哪些渠道（多选下拉，来自全局已配置渠道）推送 |
| G-5 | 通知 payload 适配各 bot 平台官方文档要求的消息体格式（DingTalk Markdown / Feishu 富文本卡片 / WeCom Markdown / Webhook 通用 JSON） |
| G-6 | FOFA 配置语义重构：`base_url` 改名/补字段，明确"API 端点（仅 API 模式）"与"Web 域名（固定官方）"的边界；Settings UI 加分区说明 |
| G-7 | 建立 `utils/urlguard` 公共包，统一管控所有"用户可配置的出站 URL"的 SSRF 校验 |
| G-8 | 单元测试覆盖：每个 channel ≥ 4 个用例（成功/失败/签名/SSRF）；FOFA URL 路由逻辑 ≥ 6 个用例 |

### 2.2 非目标 (Out of Scope)

| 编号 | 非目标 | 原因 |
|------|--------|------|
| N-1 | Email/SMTP 通知 | 现有占位继续保留，本期不实现 |
| N-2 | 通知模板自定义（用户编辑消息体） | 二期；本期采用固定模板格式 |
| N-3 | 通知到 IM 平台个人（用户级 OpenID） | 仅支持群机器人 Webhook 模式 |
| N-4 | 通知发送审计/重试队列 | 本期失败仅日志记录，不引入持久化队列 |
| N-5 | 其他引擎（Hunter/ZoomEye/Quake/Shodan）的自定义 API | FOFA 是用户明确需求，其他引擎 `base_url` 字段已可改，本期不动语义 |
| N-6 | CLI/GUI 入口接入通知 | 与现有 scheduler 不挂载 CLI/GUI 的事实一致 |

---

## 3. FOFA 自定义 API 设计（先讲，因为它的依赖更简单）

### 3.1 设计原则：显式优于隐式

**禁止"一个字段两种语义"。** 当前 `base_url` 已被 API 模式消费，但用户心智里它可能是"FOFA 站点根域"。重构后字段语义必须 **物理隔离**：

| 字段 | 用途 | 默认值 | 用户可改 |
|------|------|--------|----------|
| `engines.fofa.api_base_url` | **新字段**。仅 API 模式（`adapter.NewFofaAdapter`）读取 | `https://fofa.info` | ✅ 是 |
| `engines.fofa.web_base_url` | **保留字段**，明确语义为"Web 链路（截图/扩展/Cookie/分享 URL）使用的域名" | `https://fofa.info` | ❌ **否**（UI 隐藏 + 配置 reload 时强制重置；为未来代理预留字段，但本期锁死） |
| `engines.fofa.base_url` | **废弃但保留**。reload 时若用户填了，自动迁移到 `api_base_url` 并写日志 | `""` | ⚠️ 保留兼容期 1 个版本 |

### 3.2 代码读取约束（关键防线）

| 调用点 | 必须读 | 禁止读 |
|--------|--------|--------|
| `adapter/fofa.go NewFofaAdapter` | `Engines.Fofa.APIBaseURL` | `WebBaseURL`、硬编码常量 |
| `screenshot/manager.go BuildSearchEngineURL` | `Engines.Fofa.WebBaseURL` 或常量 `FOFA_OFFICIAL_WEB_URL` | `APIBaseURL` |
| `screenshot/router.go` 同上 | `WebBaseURL` 或常量 | `APIBaseURL` |
| `service/screenshot_app_service.go:440` | `WebBaseURL` 或常量 | `APIBaseURL` |
| `web/cookie_handlers.go:372 "fofa.info"` | 保持硬编码 cookie domain（Web 域是官方） | 不读 `APIBaseURL` |

**落地手段：** 在 `internal/adapter/fofa.go` 顶部新增导出常量 `const FOFAOfficialWebURL = "https://fofa.info"`，并在 PR 中加 lint 注释：截图/扩展/Web 链路只允许读这个常量或 `WebBaseURL`，code review 时人工把关。

### 3.3 Settings UI 改动

`web/templates/settings.html:277-286` FOFA 分组改为两行：

```
FOFA
  └── api_base_url   [https://fofa.info        ] ← 仅 API 模式生效，可填自建 FOFA 镜像/代理
  └── web_base_url   [https://fofa.info        ] ← 固定官方，Web/截图/扩展模式始终使用 (灰色禁用)
  └── email          [...]
  └── api_key        [...]
  └── ...
```

UI 端 `web_base_url` 输入框 `disabled + readonly`，旁边小字"Web 模式锁定为官方域名，避免登录态失效"。

### 3.4 数据迁移路径

`internal/config/config.go applyDefaults()` 新增：

```text
若 cfg.Engines.Fofa.APIBaseURL == "" && cfg.Engines.Fofa.BaseURL != "":
    cfg.Engines.Fofa.APIBaseURL = cfg.Engines.Fofa.BaseURL
    logger.Warnf("fofa.base_url 已迁移到 fofa.api_base_url，请更新 config.yaml")
若 cfg.Engines.Fofa.APIBaseURL == "":
    cfg.Engines.Fofa.APIBaseURL = "https://fofa.info"
强制：cfg.Engines.Fofa.WebBaseURL = "https://fofa.info"   ← 本期锁死，UI 也禁用
```

**为什么强制覆盖 `WebBaseURL`：** 防止用户在 yaml 里手改，导致 Web 链路打到代理域名造成登录态失效；后续如要开放，再单独评估。

### 3.5 SSRF 防护

`APIBaseURL` 在 `config.Validate()` 阶段先过一遍 `utils/urlguard.Check(url, AllowPrivate=false)`：
- 解析失败 → 拒绝启动
- 解析到回环/内网/链路本地 → 警告 + 拒绝启动
- HTTP scheme 非 http/https → 拒绝

**例外：** 用户若显式设置 `engines.fofa.allow_private_api: true`（高级配置，文档显著标注"仅限内网自建镜像"），允许放行私网 IP，但日志打 WARN。

---

## 4. 通知系统设计

### 4.1 整体架构

```
config.yaml
  └── notifications:
        enabled: true
        channels:                       ← 全局渠道注册表
          - id: "team-dingtalk"
            type: "dingtalk"
            enabled: true
            webhook_url: "https://oapi.dingtalk.com/robot/send?access_token=..."
            secret: "${DINGTALK_SECRET}"
          - id: "ops-feishu"
            type: "feishu"
            enabled: true
            webhook_url: "https://open.feishu.cn/open-apis/bot/v2/hook/..."
            secret: "${FEISHU_SECRET}"
          - id: "wechat-work"
            type: "wecom"
            enabled: true
            webhook_url: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=..."
          - id: "slack-generic"
            type: "webhook"            ← 通用 JSON
            enabled: false
            webhook_url: "https://hooks.slack.com/..."

ScheduledTask.Notifications:
  enabled: true                       ← 任务级总开关（新字段）
  on_success: false
  on_failure: true
  on_timeout: true
  channel_ids:                        ← 引用全局 channel.id（替代旧 channels[]）
    - "team-dingtalk"
    - "ops-feishu"
```

**两级开关：** 实际发送 = `notifications.enabled (全局) && task.Notifications.enabled (任务) && channel.enabled (渠道) && event 匹配`

### 4.2 Config schema 改动

`internal/config/config.go` 新增顶层 `Notifications`：

```go
Notifications struct {
    Enabled    bool                       `yaml:"enabled"`
    Channels   []NotificationChannelCfg   `yaml:"channels"`
    SendTimeoutSec int                    `yaml:"send_timeout_sec"`   // 默认 10s
    MaxRetries     int                    `yaml:"max_retries"`        // 默认 0（不重试，本期）
} `yaml:"notifications"`

type NotificationChannelCfg struct {
    ID          string            `yaml:"id"`           // 唯一标识，任务通过这个 id 引用
    Type        string            `yaml:"type"`         // "dingtalk" | "feishu" | "wecom" | "webhook" | "log"
    Enabled     bool              `yaml:"enabled"`
    WebhookURL  string            `yaml:"webhook_url"`
    Secret      string            `yaml:"secret"`       // 钉钉加签 / 飞书加签密钥（环境变量推荐）
    Headers     map[string]string `yaml:"headers"`      // 通用 webhook 额外请求头
    AllowPrivateIP bool           `yaml:"allow_private_ip"` // 默认 false，企业内网私有部署可开
}
```

**`alerting.webhook` 旧块的兼容：** 不动，继续给 tamper 告警用；通知系统是独立的新字段块。

### 4.3 Channel 抽象

新建 `internal/notify/` 包（与 `internal/alerting/` 平级，**不复用 AlertChannel 接口**，因为通知体语义不同）：

```text
internal/notify/
  channels.go       NotifyChannel 接口 + DingTalk/Feishu/WeCom/Webhook/Log 五个实现
  registry.go       Registry：按 id 检索 channel，支持热更新
  message.go        TaskNotification 消息体（task_id/name/type/status/result/error/duration）
  signature.go      钉钉/飞书加签算法
  channels_test.go
```

`NotifyChannel` 接口：

```go
type NotifyChannel interface {
    ID() string
    Type() string
    Send(ctx context.Context, n TaskNotification) error
    IsEnabled() bool
    Close() error
}
```

**与 `ScheduledTask.Notifications.Channels[]` 旧字面量的迁移：**
- 旧值 `"log"` → 自动映射到 `id="builtin-log"`（启动时由 registry 注册一个默认 log channel，强制 enabled）
- 旧值 `"webhook"` + `WebhookURL != ""` → 视作"任务级一次性 webhook"，生成临时 channel 实例使用
- 旧值 `"email"` → 忽略并 WARN（占位本来就没实现）

### 4.4 字段契约（前后端锁定）

| 后端 JSON 字段 | 前端 ID | 类型 | 说明 |
|---------------|---------|------|------|
| `notifications.enabled` | `task-notify-enabled` | bool | 任务级总开关 |
| `notifications.on_success` | `task-notify-on-success` | bool | |
| `notifications.on_failure` | `task-notify-on-failure` | bool | |
| `notifications.on_timeout` | `task-notify-on-timeout` | bool | |
| `notifications.channel_ids[]` | `task-notify-channels` | string[] | 多选下拉的 value |
| `notifications.webhook_url` (deprecated) | （不再暴露） | string | 仅向后兼容反序列化 |

### 4.5 发送逻辑（替换 scheduler.go:997-1042）

```text
func (s *Scheduler) sendNotification(task, record):
    if !globalCfg.Notifications.Enabled: return
    if task.Notifications == nil || !task.Notifications.Enabled: return

    shouldNotify := switch record.Status {
        case success: task.Notifications.OnSuccess
        case failed:  task.Notifications.OnFailure
        case timeout: task.Notifications.OnTimeout
        default: false
    }
    if !shouldNotify: return

    msg := buildTaskNotification(task, record)

    for chID in task.Notifications.ChannelIDs:
        ch := registry.Get(chID)
        if ch == nil:
            log.Warn("channel not registered: " + chID)
            continue
        if !ch.IsEnabled():
            log.Debug("channel disabled: " + chID)
            continue
        s.wg.Add(1)
        go func(ch):
            defer s.wg.Done()
            ctx, cancel := context.WithTimeout(s.stopCh-aware, sendTimeout)
            defer cancel()
            if err := ch.Send(ctx, msg); err != nil:
                log.Warnf("notify %s failed: %v", ch.ID(), err)
                metrics.IncSchedulerNotifyFail(ch.Type())
            else:
                metrics.IncSchedulerNotifySuccess(ch.Type())
```

**关键：** 沿用现有 `wg.Add/wg.Done` 模式（参考 H-04 修复），保证 `Scheduler.Stop()` 能等通知 goroutine 退出再返回，防止泄漏。

### 4.6 配置热更新

通知系统使用 `cfgProvider func() *config.NotificationsCfg` 闭包模式（与 ICP 方案一致）：
- 每次 `sendNotification` 调用前 `cfg := s.notifyCfgProvider()` 读最新配置
- Registry 监听 config reload 事件，diff 出新增/删除/修改的 channel，重建实例
- 任务里引用的 channel_id 在 reload 后若失效 → 下次执行时记 WARN，不阻塞任务

### 4.7 密钥脱敏

`NotificationChannelCfg` 实现 `MarshalJSON`：
- `Secret` → `"***"`（如果非空）
- `WebhookURL` → 保留 host + path 前 20 字符，token 部分用 `...` 替代

应用范围：
- `GET /api/config` 返回时
- history JSON 写入时（虽然 record 不写 channel 配置，但 payload 经 channel.Send 序列化时如果意外把配置带进去要拦截）
- 日志打印时

### 4.8 数据迁移

存量 `scheduler_tasks.json` 中 `notifications` 字段处理：

```text
if task.Notifications == nil: keep nil
if task.Notifications != nil:
    if task.Notifications.Channels != nil && task.Notifications.ChannelIDs == nil:
        // 旧字段 → 新字段映射
        for _, name in task.Notifications.Channels:
            switch name:
                case "log": ChannelIDs.append("builtin-log")
                case "webhook":
                    if task.Notifications.WebhookURL != "":
                        // 一次性临时 channel，runtime 解析
                        ChannelIDs.append("__task_inline_webhook__")
                    else:
                        log.Warn("task %s webhook channel without URL skipped", task.ID)
                case "email":
                    log.Warn("email channel not supported, skipped")
        // 老字段保留不删，下次保存任务时自然消失
    if task.Notifications.Enabled is not set (老 JSON 没这字段):
        task.Notifications.Enabled = true   // 老任务默认认为已开启
```

### 4.9 各平台消息体格式

| 平台 | 必填字段 | 消息样例（成功） |
|------|----------|-----------------|
| DingTalk | `msgtype: markdown`，加签后查询参数 `timestamp` + `sign` | "**[UniMap] 定时任务 [每日企业巡检] 执行成功**\n- 类型: icp_query\n- 耗时: 12.3s\n- 结果: 2/2 queries succeeded, total 87 records" |
| Feishu | `msg_type: interactive`，富文本卡片；加签 `timestamp` + `sign` | 同上，卡片渲染 |
| WeCom | `msgtype: markdown` | 同上 |
| Webhook | 通用 JSON `{task_id, name, type, status, result, error, duration_ms, timestamp}` | 直接 marshal |
| Log | `logger.Infof / Warnf` 按 status 分级 | 文本 |

失败/超时模板类似，附 `Error` 字段。

---

## 5. 公共依赖：URL 防护包

### 5.1 为什么单独提

R-A（FOFA 域）和 R-F（通知 webhook SSRF）都依赖"出站 URL 必须经过校验"，目前散落在 `internal/alerting/channels.go:67-114` 和 `internal/scheduler/scheduler.go:1048-1075` 两处，逻辑不一致：
- alerting 包：解析失败/scheme 异常/loopback/private → 把 URL 置空 + 继续运行（吞错）
- scheduler 包：DNS resolve + 拒绝私网连接 → 真正拦截

新功能上线会把"用户可配 URL"的入口从 2 处扩到 6+ 处（FOFA API + 4 种 channel webhook + 任务级 inline webhook + secret reload）。如果继续散落，第 7 个入口必然漏校验。

### 5.2 设计

`internal/utils/urlguard/` 新包：

```text
urlguard/
  check.go        Check(rawURL string, opts CheckOptions) (*url.URL, error)
  dial.go         SafeDialer：构造拒绝私网 IP 的 net.Dialer，用于 http.Transport.DialContext
  options.go      CheckOptions: AllowPrivate, AllowedSchemes, AllowedHosts (可选白名单)
  check_test.go
```

**对外接口：**
```go
type CheckOptions struct {
    AllowPrivate    bool      // 默认 false
    AllowedSchemes  []string  // 默认 ["http", "https"]
    AllowedHostsRE  *regexp.Regexp // 可选：仅允许匹配 host 的 URL（如 *.dingtalk.com）
}

func Check(rawURL string, opts CheckOptions) (*url.URL, error)
func SafeHTTPClient(opts CheckOptions, timeout time.Duration) *http.Client
```

**迁移：**
- `internal/alerting/channels.go:NewWebhookChannel` 内联逻辑 → 改调 `urlguard.Check`
- `internal/scheduler/scheduler.go:safeWebhookClient` → 改调 `urlguard.SafeHTTPClient`
- 新通知 channel 全部使用 `urlguard.SafeHTTPClient`
- FOFA `APIBaseURL` 在 `config.Validate()` 阶段调 `urlguard.Check(url, {AllowPrivate: cfg.Engines.Fofa.AllowPrivateAPI})`

---

## 6. 实施步骤（按依赖拓扑严格排序）

> **每一步独立可验证**，前一步 PASS 才进下一步。绝不并行写。

| Step | 内容 | 验证 | 依赖 | 工作量 |
|------|------|------|------|--------|
| **S-1** | 新建 `internal/utils/urlguard/` 包 + 单元测试 | `go test ./internal/utils/urlguard/...` 通过；不动任何调用方 | 无 | 0.5 天 |
| **S-2** | 把 `alerting/channels.go` 与 `scheduler/scheduler.go` 已有 SSRF 逻辑切换到 `urlguard`（**行为保持不变**） | 现有 `alerting` 和 `scheduler` 测试不回归 | S-1 | 0.5 天 |
| **S-3** | `config.go` 引入 `Notifications` 顶层块 + `NotificationChannelCfg` 类型（**仅定义，不消费**） + 默认值 + Validate（用 urlguard） | `go build ./...` + 现有 config 测试通过；`config.yaml.example` 加示例块 | S-1 | 0.5 天 |
| **S-4** | 新建 `internal/notify/` 包：接口 + Log channel + Webhook 通用 channel + Registry + 单测 | Registry/Log/Webhook 单测通过 | S-3 | 1 天 |
| **S-5** | 在 `internal/notify/` 实现 DingTalk + Feishu + WeCom 三个 channel（含加签算法）+ 集成测试（httptest mock） | 每个 channel ≥ 4 个用例通过；签名生成对比官方文档示例 | S-4 | 1.5 天 |
| **S-6** | `web/server.go` 初始化时构建 `notify.Registry`，注入 `Scheduler.SetNotifyRegistry(reg)` 与 `Scheduler.SetNotifyCfgProvider(...)` | 启动 web 不崩；reload config 后 registry 重建（手工 e2e） | S-5 | 0.5 天 |
| **S-7** | 改造 `scheduler.go:sendNotification`：迁移到 registry + channel_ids + wg + 数据迁移逻辑（§4.8） | scheduler 单测覆盖：channel_ids 模式、旧 channels[] 兼容模式、registry 缺失、全局/任务/渠道三级开关组合（≥ 8 个用例） | S-6 | 1 天 |
| **S-8** | `web/scheduler_handlers.go` 接受/返回 `notifications.{enabled, on_*, channel_ids[]}`，校验 channel_ids 必须存在于 registry | handler 单测：合法/未知 channel_id/总开关关 ≥ 4 个用例 | S-7 | 0.5 天 |
| **S-9** | 前端 `scheduler.html` 增加"通知"折叠面板：总开关、3 个事件 checkbox、多选下拉（从 `/api/notifications/channels` 拉全局列表） | 手工 e2e：创建任务勾选钉钉渠道，触发后钉钉群收到消息 | S-8 | 1 天 |
| **S-10** | `internal/config/config.go` FOFA 字段拆分：新增 `APIBaseURL` + 强制 `WebBaseURL` + 旧 `BaseURL` 迁移日志 | config 测试覆盖：旧 yaml/新 yaml/混填三种情况 | S-1 | 0.5 天 |
| **S-11** | `cmd/unimap-{web,cli,gui}/main.go` 把 `NewFofaAdapter` 入参从 `BaseURL` 改为 `APIBaseURL`；新增 `FOFAOfficialWebURL` 常量 | `go build ./...`；FOFA API 查询回归正常 | S-10 | 0.5 天 |
| **S-12** | `web/templates/settings.html` FOFA 分组改为两行（api_base_url 可编辑、web_base_url 灰色禁用 + 说明） | 手工 e2e：UI 改 api_base_url 保存 → 配置文件正确 → API 查询走新域名 | S-10 | 0.5 天 |
| **S-13** | `docs/API.md` + `docs/RUNBOOK.md` 更新；`memory/MEMORY.md` 加索引行；落地报告写入 `memory/project_notify_fofa_2026-05-22.md` | 文档评审通过 | 全部 | 0.5 天 |
| **合计** | | | | **~9 天** |

### 6.1 关键依赖图

```
S-1 (urlguard)
  ├── S-2 (alerting/scheduler 迁移)
  ├── S-3 (config 新块) ─── S-4 (notify 基础) ─── S-5 (3 个 bot) ─── S-6 (server 注入) ─── S-7 (scheduler 改造) ─── S-8 (handler) ─── S-9 (UI)
  └── S-10 (FOFA 字段) ─── S-11 (入口改造) ─── S-12 (UI)
                                                                                                                                          └─→ S-13 (文档)
```

**绝不允许的并行：**
- ❌ S-7 在 S-6 之前（registry 没注入就改 scheduler）
- ❌ S-8 在 S-7 之前（handler 字段还没定 schema）
- ❌ S-9 在 S-8 之前（前端调的接口还没有）
- ❌ S-11 在 S-10 之前（字段还没定义入口改了会编译失败）
- ❌ 通知链 (S-3~S-9) 与 FOFA 链 (S-10~S-12) 在 S-13 之前合并 PR（一旦串味难以回滚定位）

---

## 7. 测试计划

### 7.1 urlguard (S-1)

| 用例 | 验证 |
|------|------|
| Check_ValidHTTPS | 正常返回 *url.URL |
| Check_RejectInvalidScheme | `ftp://...` 拒绝 |
| Check_RejectLoopback | `127.0.0.1`/`localhost`/`::1` 拒绝 |
| Check_RejectPrivate | `10.0.0.1`/`192.168.1.1`/`172.16.0.1` 拒绝 |
| Check_RejectLinkLocal | `169.254.169.254`（云元数据）拒绝 |
| Check_AllowPrivateOpt | `AllowPrivate=true` 时私网放行 |
| Check_AllowedHostsRegex | 仅匹配 `*.dingtalk.com` 等白名单 |
| SafeHTTPClient_RejectRedirectToPrivate | 服务端 302 到 127.0.0.1 时拒绝 |
| SafeHTTPClient_RejectDNSRebinding | mock DNS 返回私网 IP 时拒绝 |

### 7.2 Notify channels (S-4 / S-5)

每个 channel 至少：
- `Send_Success`：mock server 收到正确格式的 body（钉钉/飞书/WeCom 各自匹配 JSON 结构）
- `Send_NonSuccessStatusCode`：4xx/5xx 返回 error
- `Send_NetworkError`：连接拒绝返回 error 带 host
- `Send_ContextCancel`：ctx cancel 时立即返回
- `Sign_DingTalk`：用文档样例时间戳 + 密钥，对比 sign 值
- `Sign_Feishu`：同上
- `SSRF_RejectPrivateWebhook`：URL 指向私网时构造失败

### 7.3 Scheduler 通知改造 (S-7)

| 用例 | 验证 |
|------|------|
| Notify_GlobalDisabled_NoSend | 全局开关 off → channel mock 不应被调 |
| Notify_TaskDisabled_NoSend | 任务开关 off → 不发 |
| Notify_ChannelDisabled_NoSend | channel.enabled=false → 不发 |
| Notify_OnSuccessMatched | success 事件 + OnSuccess=true → 发 |
| Notify_OnFailureMatched | failed 事件 + OnFailure=true → 发 |
| Notify_OnTimeoutMatched | timeout 事件 + OnTimeout=true → 发 |
| Notify_UnknownChannelID_WarnSkip | channel_ids 含未注册 id → 日志 warn，不阻塞其他 channel |
| Notify_MultipleChannels_AllCalled | channel_ids 含 3 个 → 3 个 channel 都收到 |
| Notify_StopWaitsForGoroutines | Scheduler.Stop 等待所有通知 goroutine 退出 |
| Migrate_OldChannelsField | 老 JSON `Channels: ["log","webhook"]` + `WebhookURL=...` → 迁移到 `ChannelIDs: ["builtin-log", "__task_inline_webhook__"]` |

### 7.4 FOFA 字段拆分 (S-10 / S-11)

| 用例 | 验证 |
|------|------|
| Config_LegacyBaseURL_MigratedToAPI | yaml 仅有 `base_url: https://x.com` → APIBaseURL = "https://x.com"，日志含 WARN |
| Config_NewAPIBaseURL_NoMigration | yaml 直接给 `api_base_url` → 保留，不动 |
| Config_WebBaseURLForcedOfficial | 用户在 yaml 里把 `web_base_url` 改成自定义 → reload 后被强制覆盖回 `https://fofa.info`，日志 WARN |
| Config_APIBaseURLSSRFCheck_Reject | `api_base_url: http://127.0.0.1` 且 `allow_private_api: false` → Validate 失败 |
| Config_APIBaseURLSSRFCheck_Allow | 同上 + `allow_private_api: true` → Validate 通过，日志 WARN |
| Adapter_UsesAPIBaseURL | NewFofaAdapter 收到的 baseURL == cfg.APIBaseURL |

### 7.5 集成 / E2E

- 启动 web，跑通：
  - 创建定时任务勾选钉钉/飞书/企微/Slack 渠道 → 触发 → 群里确认收到正确格式消息
  - 关闭全局 notifications.enabled → 任务执行不发
  - 修改 fofa.api_base_url 为镜像域名 → 重启 / hot reload → API 查询走新域名，截图仍走 fofa.info
- `go test -race -count=1 ./...` 全包通过

---

## 8. 文件级改动清单

| # | 文件 | 改动类型 | 说明 | 预估行 |
|---|------|---------|------|--------|
| F-1 | `internal/utils/urlguard/check.go` | 新建 | Check + SafeHTTPClient | +120 |
| F-2 | `internal/utils/urlguard/check_test.go` | 新建 | 9 个测试 | +220 |
| F-3 | `internal/alerting/channels.go` | 修改 | 切换到 urlguard，保持行为 | ~40 |
| F-4 | `internal/scheduler/scheduler.go safeWebhookClient` | 修改 | 切换到 urlguard | ~30 |
| F-5 | `internal/config/config.go` | 修改 | +Notifications 块 + Fofa 字段拆分 + Validate | +80 |
| F-6 | `configs/config.yaml.example` | 修改 | 加 notifications 示例 + fofa.api_base_url | +30 |
| F-7 | `internal/notify/channels.go` | 新建 | 接口 + 5 个实现 | +320 |
| F-8 | `internal/notify/signature.go` | 新建 | DingTalk/Feishu 加签 | +60 |
| F-9 | `internal/notify/registry.go` | 新建 | Registry + reload | +90 |
| F-10 | `internal/notify/message.go` | 新建 | TaskNotification 类型 | +40 |
| F-11 | `internal/notify/channels_test.go` | 新建 | ≥ 25 个测试 | +500 |
| F-12 | `internal/scheduler/scheduler.go sendNotification` | 修改 | 用 registry 重写 + 数据迁移 | +90 / -45 |
| F-13 | `internal/scheduler/scheduler_test.go` | 修改 | 新增通知改造测试 | +180 |
| F-14 | `web/server.go` | 修改 | 初始化 registry + 注入 | +25 |
| F-15 | `web/scheduler_handlers.go` | 修改 | 接受/返回 notifications 新字段 + 校验 | +60 |
| F-16 | `web/notification_handlers.go` | 新建 | `GET /api/notifications/channels` 列全局已注册渠道（脱敏） | +50 |
| F-17 | `web/templates/scheduler.html` | 修改 | 通知折叠面板 + JS 多选 | +130 |
| F-18 | `web/templates/settings.html` | 修改 | FOFA 分组拆分两行 + 灰色禁用 + 说明 | +20 |
| F-19 | `cmd/unimap-web/main.go` | 修改 | `NewFofaAdapter` 入参改 APIBaseURL | +2 |
| F-20 | `cmd/unimap-cli/main.go` | 修改 | 同上 | +1 |
| F-21 | `cmd/unimap-gui/main.go` | 修改 | 同上 | +1 |
| F-22 | `internal/adapter/fofa.go` | 修改 | 导出 `FOFAOfficialWebURL` 常量 | +3 |
| F-23 | `internal/screenshot/manager.go` / `router.go` / `service/screenshot_app_service.go` | 修改 | 硬编码 `https://fofa.info` 改读常量（语义不变，仅消除魔法字符串） | ~6 处 |
| F-24 | `docs/API.md` | 修改 | 通知接口 + FOFA 字段说明 | +60 |
| F-25 | `docs/RUNBOOK.md` | 修改 | 通知排查场景（消息没收到时排查清单） | +40 |
| F-26 | `memory/MEMORY.md` | 修改 | 索引行 | +1 |
| F-27 | `memory/project_notify_fofa_2026-05-22.md` | 新建 | 落地报告 | ~80 |

---

## 9. 验收清单（合并 PR 前必过）

- [ ] `go build ./...`
- [ ] `go test -race -count=1 ./...`
- [ ] `go vet ./...`、`gofmt -l .` 干净
- [ ] `internal/notify/` 覆盖率 ≥ 85%
- [ ] `internal/utils/urlguard/` 覆盖率 ≥ 90%
- [ ] 钉钉 / 飞书 / 企业微信 / 通用 Webhook 群机器人各跑一次真实推送，截图存档
- [ ] FOFA `api_base_url` 改成镜像后 API 查询走新域名；截图任务依然走 `fofa.info`（抓包/日志佐证）
- [ ] 旧 `config.yaml` 仅有 `engines.fofa.base_url` 时启动成功 + 看到迁移 WARN
- [ ] 存量 `scheduler_tasks.json`（含老 `Channels: ["webhook"]` + `WebhookURL`）启动成功 + 通知行为兼容
- [ ] 通知密钥不出现在 `/api/config` 返回、不出现在 history JSON、不出现在 INFO/WARN 日志
- [ ] `Scheduler.Stop()` 在通知 goroutine 在飞时能等待退出（race 测试通过）
- [ ] 文档与 memory 已更新

---

## 10. 回滚预案

每个 Step 是独立 commit，必要时按下列顺序回滚（与依赖图相反）：

- 通知链问题 → 回滚 S-9 → S-8 → S-7 → S-6 → S-5 → S-4 → S-3，scheduler 通知回到当前主干行为（仅 log/通用 webhook）
- FOFA 链问题 → 回滚 S-12 → S-11 → S-10，FOFA 配置回到当前主干行为
- urlguard 问题 → 回滚 S-2，alerting/scheduler 回到内联校验

**不可回滚的红线：** 一旦 S-10 改了 yaml schema 上线，用户改动了 `api_base_url`，回滚到只认 `base_url` 的旧版会读不到这个字段。所以 S-10 上线前在 `config.go applyDefaults` 里**先**写好双向兼容（新版优先 `api_base_url`，没有则 fallback `base_url`），回滚老版本只认 `base_url` 也能继续跑（用户需手动把值复制回 base_url）。

---

## 11. 参考

- 现有通知半成品：`internal/scheduler/scheduler.go:233-269, 997-1109`
- 现有 SSRF 安全样板：`internal/alerting/channels.go:67-114`、`internal/scheduler/scheduler.go:1048-1075`
- DingTalk 群机器人加签文档：https://open.dingtalk.com/document/group/customize-robot-security-settings
- 飞书自定义机器人签名校验：https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot
- 企业微信群机器人：https://developer.work.weixin.qq.com/document/path/91770
- ICP 方案先例（cfgProvider 闭包 + 数据迁移）：`docs/PLAN_SCHEDULED_ICP_QUERY_2026-05-22.md`、`memory/project_icp_scheduled_task_2026-05-22.md`
