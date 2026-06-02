# 定时任务系统优化与测试计划

> 创建日期：2026-06-02
> 更新日期：2026-06-02
> 状态：待实施

---

## 一、前端优化：简化任务创建流程

### 1.1 现状问题

当前定时任务创建表单存在以下问题：

1. **下拉选项过多**：22 种任务类型平铺在一个 `<select>` 中，用户需要滚动查找
2. **JSON 参数难用**：用户需要手写 JSON payload，无模板、无校验、无提示
3. **无分类引导**：高频任务（查询/截图/篡改检测）和低频运维任务（清理/监控）混在一起
4. **Cron 表达式门槛高**：需要记住 6 字段格式（秒 分 时 日 月 周）

### 1.2 优化方案

#### A. 任务类型分组

将 22 种任务按用途分为 5 组，使用 `<optgroup>` 分组下拉：

```
📊 查询与采集
  ├── UQL 查询 (query)
  ├── 搜索引擎截图 (search_screenshot)
  ├── 批量截图 (batch_screenshot)
  ├── 数据导出 (export)
  └── ICP 备案查询 (icp_query)

🔍 监控与检测
  ├── 篡改检测 (tamper_check)
  ├── URL 可达性检测 (url_reachability)
  ├── 端口扫描 (port_scan)
  ├── 登录状态检测 (login_status_check)
  └── 配额监控 (quota_monitor)

🔧 维护与清理
  ├── 截图清理 (screenshot_cleanup)
  ├── 篡改记录清理 (tamper_cleanup)
  ├── 基线刷新 (baseline_refresh)
  └── 告警静默窗口 (alert_silence)

📡 基础设施
  ├── Cookie 验证 (cookie_verify)
  ├── Bridge 健康检查 (bridge_token)
  ├── 插件健康检查 (plugin_health)
  ├── 缓存预热 (cache_warmup)
  └── 分布式任务提交 (distributed_submit)

📥 导入与汇总
  ├── URL 导入 (url_import)
  ├── ICP 关键词导入 (icp_import)
  └── 告警汇总 (alert_summary)
```

#### B. 任务模板系统

为每种高频任务预设模板，选中任务类型后自动填充 JSON payload：

| 任务类型 | 模板名称 | 预填参数 |
|----------|----------|----------|
| UQL 查询 | "FOFA 每日资产扫描" | `{"query":"title=\"nginx\" && country=\"CN\"","engines":["fofa"],"page_size":20}` |
| 搜索引擎截图 | "FOFA 搜索结果截图" | `{"engine":"fofa","query":"title=\"login\""}` |
| 批量截图 | "官网截图巡检" | `{"urls":["https://example.com"],"concurrency":3}` |
| 篡改检测 | "首页篡改监控" | `{"urls":["https://example.com"],"mode":"strict"}` |
| ICP 备案查询 | "域名备案变更监控" | `{"queries":["example.com"],"types":["web"]}` |
| 配额监控 | "API 配额告警" | `{"low_threshold":10}` |
| URL 可达性检测 | "服务存活检测" | `{"urls":["https://example.com"]}` |

#### C. Cron 快捷选择

提供常用 Cron 预设按钮：

| 标签 | Cron 表达式 | 含义 |
|------|------------|------|
| 每小时 | `0 0 * * * *` | 整点执行 |
| 每天凌晨2点 | `0 0 2 * * *` | 每天 02:00 |
| 每天早8点 | `0 0 8 * * *` | 每天 08:00 |
| 每周一 | `0 0 9 * * 1` | 每周一 09:00 |
| 每月1号 | `0 0 3 1 * *` | 每月1号 03:00 |
| 每5分钟 | `0 */5 * * * *` | 高频监控 |
| 每30分钟 | `0 */30 * * * *` | 中频监控 |

#### D. JSON 参数编辑器增强

- 选中任务类型后，显示该类型的参数说明和示例
- 添加"从模板填充"按钮
- 添加 JSON 语法校验（提交前检查）

### 1.3 涉及文件

| 文件 | 改动 |
|------|------|
| `web/templates/scheduler.html` | 重构创建表单、添加分组/模板/Cron 预设 |
| `web/scheduler_handlers.go` | 添加模板 API（可选） |
| `internal/scheduler/scheduler.go` | 添加 `TaskTypeGroup()` 分组方法 |

---

## 二、后端测试：全量 Runner 验证

### 2.1 测试策略

**原则**：使用精确查询减少 API 消耗，避免 `port=80` 这类返回海量结果的查询。

#### 推荐的低消耗测试查询

| 引擎 | 测试查询 | 预期结果数 | 说明 |
|------|----------|-----------|------|
| FOFA | `title="unimap" && country="CN"` | < 10 | 精确标题匹配 |
| FOFA | `host="example.com" && port="443"` | < 5 | 单域名+端口 |
| Hunter | `ip="1.1.1.1"` | < 5 | 单 IP 查询 |
| Quake | `title:"Apache" && country:"CN"` | < 20 | 精确标题 |
| ZoomEye | `hostname:"example.com"` | < 5 | 单主机名 |
| Shodan | `hostname:"example.com"` | < 5 | 单主机名 |

#### 禁止使用的查询（消耗过大）

| 查询 | 原因 |
|------|------|
| `port="80"` | 返回数万条 |
| `title="login"` | 返回数万条 |
| `country="CN"` | 返回数十万条 |
| 空查询 | 必须报错 |

### 2.2 逐项测试计划

#### ST-01: UQL 查询

```json
{
  "query": "title=\"nginx\" && country=\"CN\" && port=\"443\"",
  "engines": ["fofa"],
  "page_size": 5
}
```

**验证点**：
- 返回结果数 ≤ page_size
- assets 字段有数据
- engine_stats 正确
- 无 error

#### ST-02: 搜索引擎截图

```json
{
  "engine": "fofa",
  "query": "title=\"unimap\""
}
```

**验证点**：
- 返回截图文件路径
- 文件存在且 > 0 字节
- 无超时

#### ST-03: 批量截图

```json
{
  "urls": ["https://www.baidu.com", "https://www.example.com"],
  "concurrency": 2
}
```

**验证点**：
- success_count / total 正确
- 截图文件存在

#### ST-04: 篡改检测

```json
{
  "urls": ["https://www.example.com"],
  "mode": "relaxed"
}
```

**验证点**：
- 返回检测结果
- baseline 建立成功

#### ST-05: URL 可达性检测

```json
{
  "urls": ["https://www.example.com", "https://nonexistent.invalid"]
}
```

**验证点**：
- example.com → reachable
- nonexistent.invalid → unreachable

#### ST-06: Cookie 验证

```json
{
  "engines": ["fofa", "hunter"]
}
```

**验证点**：
- 返回每个引擎的 cookie 状态
- 无 panic

#### ST-07: 登录状态检测

```json
{}
```

**验证点**：
- 返回每个引擎的登录状态
- 不会因未登录而 panic

#### ST-08: 分布式任务提交

```json
{
  "task_type": "url_reachability",
  "priority": 5,
  "timeout_seconds": 60
}
```

**验证点**：
- 任务入队成功
- 返回 task_id

#### ST-09: 数据导出

```json
{
  "query": "title=\"example.com\"",
  "engines": ["fofa"],
  "page_size": 5,
  "format": "json"
}
```

**验证点**：
- 导出文件存在
- 文件内容为有效 JSON
- 结果数正确

#### ST-10: 端口扫描

```json
{
  "urls": ["https://www.example.com"]
}
```

**验证点**：
- 返回端口扫描结果
- 无超时

#### ST-11: 截图清理

```json
{
  "max_age_days": 30
}
```

**验证点**：
- 返回清理数量
- 不删除近期截图

#### ST-12: 篡改记录清理

```json
{
  "max_age_days": 90
}
```

**验证点**：
- 返回清理数量
- 不删除近期记录

#### ST-13: 配额监控

```json
{
  "low_threshold": 10
}
```

**验证点**：
- 返回各引擎配额信息
- 低配额引擎有告警

#### ST-14: 告警汇总

```json
{
  "days": 7
}
```

**验证点**：
- 返回统计摘要
- 按类型/级别分类

#### ST-15: 基线刷新

```json
{
  "urls": ["https://www.example.com"]
}
```

**验证点**：
- 基线更新成功
- 返回刷新数量

#### ST-16: URL 导入

需预先在 import 目录放置测试文件。

**验证点**：
- 文件被正确读取
- URL 列表返回

#### ST-17: 插件健康检查

```json
{}
```

**验证点**：
- 返回健康状态
- 无 panic

#### ST-18: Bridge 健康检查

```json
{}
```

**验证点**：
- 返回 bridge 状态（started/workers/queue/in_flight）
- 未启动时返回错误

#### ST-19: 告警静默窗口

```json
{
  "alert_type": "quota_low",
  "silence_minutes": 30
}
```

**验证点**：
- 静默设置成功
- 清理模式正常工作

#### ST-20: 缓存预热

```json
{
  "urls": ["https://www.example.com", "https://www.baidu.com"]
}
```

**验证点**：
- 返回可达性结果
- SSRF 内网地址被拦截

#### ST-21: ICP 备案查询

```json
{
  "queries": ["example.com"],
  "types": ["web"],
  "page": 1,
  "page_size": 10
}
```

**验证点**：
- 返回备案信息
- 结果持久化到数据库
- 变更告警正常

#### ST-22: ICP 关键词导入

需预先在 import 目录放置 CSV 文件。

**验证点**：
- CSV 解析正确
- 自动创建 ICP 查询任务

### 2.3 测试脚本结构

```
scripts/
  test_scheduler_runners.sh      # 主测试脚本
  test_payloads/                 # 各 Runner 的测试 payload JSON
    st01_query.json
    st02_screenshot.json
    ...
```

---

## 三、飞书通知：已实现，仅需配置

### 3.1 现状

飞书通知**已完全实现**，定时任务执行完成后会自动推送结果到飞书。无需额外开发。

通知链路：
```
任务执行完成 → sendNotification()
  ├── 检查全局开关 (NotifyGlobalCfg.Enabled)
  ├── 检查任务级开关 (task.Notifications.Enabled)
  ├── 匹配事件类型 (OnSuccess / OnFailure / OnTimeout)
  ├── 解析 ChannelIDs
  └── 并发发送到各渠道 (飞书/钉钉/企微/Webhook/日志)
```

### 3.2 配置步骤

**步骤 1：在 `config.yaml` 中配置飞书渠道**

```yaml
notifications:
  enabled: true
  send_timeout_sec: 10
  channels:
    - id: feishu-alert
      type: feishu
      name: "任务告警"
      enabled: true
      webhook_url: "https://open.feishu.cn/open-apis/bot/v2/hook/你的webhook_id"
      sign_key: "${FEISHU_SIGN_KEY}"  # 可选，HMAC 签名
```

**步骤 2：创建任务时启用通知**

```json
{
  "name": "FOFA 资产扫描",
  "type": "query",
  "cron": "0 0 2 * * *",
  "payload": { "query": "...", "engines": ["fofa"], "page_size": 5 },
  "notifications": {
    "enabled": true,
    "on_success": true,
    "on_failure": true,
    "on_timeout": true,
    "channel_ids": ["feishu-alert"]
  }
}
```

**步骤 3：测试时手动触发一次，验证飞书收到消息**

通过 Web UI 点击"立即执行"或 API 调用 `POST /api/v1/scheduler/tasks/{id}/run`。

### 3.3 飞书消息格式

飞书收到的是交互式卡片消息，包含：
- 任务名称、类型
- 执行状态（成功/失败/超时）
- 耗时
- 结果摘要（资产数量、引擎统计等）
- 错误信息（如有）

---

## 四、浏览器采集模式测试（Extension/CDP）

### 4.1 背景

项目支持两种搜索引擎查询模式：

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| **API 模式** | 通过引擎 API 直接查询 | 有 API Key 时 |
| **浏览器模式** | 通过 Extension/CDP 打开搜索页面，从 DOM 采集结果 | 无 API Key 或 API 不可用时 |

浏览器模式的核心挑战：**各搜索引擎的前端 DOM 结构会变化**，需要实际测试才能确认采集选择器是否有效。

### 4.2 需要测试的引擎 × 功能组合

| 引擎 | 搜索页 URL | 采集动作 | 关键 DOM 选择器 |
|------|-----------|----------|----------------|
| FOFA | `fofa.info/result?qbase64=...` | collect | 结果列表、IP/端口/标题字段 |
| Hunter | `hunter.qianxin.com/list?searchValue=...` | collect | 结果卡片、资产信息 |
| Quake | `quake.360.cn/quake/#/searchResult?searchVal=...` | collect | 结果表格、字段映射 |
| ZoomEye | `zoomeye.org/searchResult?q=...` | collect | 结果列表、资产字段 |
| Shodan | `shodan.io/search?query=...` | collect | 结果表格、banner 信息 |

### 4.3 测试方法：Chrome MCP 辅助

由于搜索引擎使用 Vue/React SPA 框架，DOM 结构动态渲染，静态分析无法确认选择器有效性。需要使用 **Chrome MCP** 实际打开页面并验证。

#### Chrome MCP 测试流程

```
1. 启动 Chrome MCP Server
   npx @anthropic-ai/chrome-mcp@latest

2. 对每个引擎执行：
   a. 打开搜索结果页（使用精确查询，如 title="unimap"）
   b. 等待页面渲染完成（SPA 需要额外等待）
   c. 检查 DOM 中是否存在采集目标元素
   d. 验证选择器能否提取到结构化数据（IP/端口/标题/URL）
   e. 截图记录页面状态
```

#### 测试用例模板

```markdown
### FOFA 采集测试

**查询**: `title="unimap" && country="CN"`
**URL**: `https://fofa.info/result?qbase64=dGl0bGU9InVuaW1hcCIgJiYgY291bnRyeT0iQ04i`

**检查项**:
- [ ] 页面加载完成（无白屏/报错）
- [ ] 结果列表元素存在
- [ ] 能提取到 IP 地址
- [ ] 能提取到端口号
- [ ] 能提取到标题
- [ ] 能提取到 URL
- [ ] 数据条数 > 0
- [ ] 登录墙检测正确（未登录时应提示）

**截图**: 保存到 `docs/test_screenshots/fofa_collect.png`
```

### 4.4 采集测试与定时任务测试的关系

浏览器采集测试是以下定时任务的前置条件：

| 定时任务 | 依赖采集测试 | 说明 |
|----------|-------------|------|
| ST-01 UQL 查询 | 间接 | API 模式不依赖，但浏览器降级时需要 |
| ST-02 搜索引擎截图 | 直接 | 需要打开搜索页面 |
| ST-07 登录状态检测 | 直接 | 需要打开搜索页面检测登录墙 |
| Extension 模式全量 | 直接 | 所有 Extension 采集操作 |

### 4.5 采集测试工具链

```
Chrome MCP (页面验证)
    ├── 打开各引擎搜索页
    ├── 检查 DOM 选择器有效性
    ├── 验证数据提取
    └── 截图存档

UniMap Extension Bridge (实际采集)
    ├── 接收搜索 URL
    ├── 在浏览器中打开
    ├── 执行 DOM 采集脚本
    └── 返回结构化数据

后端验证 (数据校验)
    ├── 对比 API 结果和浏览器采集结果
    ├── 检查字段完整性
    └── 记录差异
```

### 4.6 采集测试计划

| 阶段 | 引擎 | 查询 | 验证内容 |
|------|------|------|----------|
| T1 | FOFA | `title="unimap"` | DOM 选择器、数据提取、登录墙检测 |
| T2 | Hunter | `ip="1.1.1.1"` | DOM 选择器、数据提取 |
| T3 | Quake | `title:"Apache"` | DOM 选择器、数据提取 |
| T4 | ZoomEye | `hostname:"example.com"` | DOM 选择器、数据提取 |
| T5 | Shodan | `hostname:"example.com"` | DOM 选择器、数据提取 |

每个引擎测试完成后，更新 `tools/extension-screenshot/` 中的选择器配置（如有变化）。

---

## 五、实施顺序（修订）

| 阶段 | 内容 | 预计耗时 | 前置条件 |
|------|------|----------|----------|
| **P1** | 浏览器采集测试：Chrome MCP 验证 5 引擎 DOM 选择器 | 1 天 | Chrome MCP 可用 |
| **P2** | 采集选择器修复：根据 P1 结果更新 Extension 采集脚本 | 0.5-1 天 | P1 完成 |
| **P3** | 前端优化：任务类型分组 + Cron 预设 | 1 天 | 无 |
| **P4** | 前端优化：任务模板系统 + JSON 编辑器增强 | 1 天 | P3 完成 |
| **P5** | 后端测试：编写测试脚本 + 测试 payload 文件 | 1 天 | 无 |
| **P6** | 后端测试：逐项执行 22 个 Runner 测试 | 1-2 天 | P2 + P5 完成 |
| **P7** | 飞书验证：确认通知配置正确、测试消息推送 | 0.5 天 | 飞书 webhook 配置好 |

**关键路径**：P1 → P2 → P6（浏览器采集测试是后端测试的前置条件）

---

## 六、API 消耗预算

| 引擎 | 计划消耗 | 说明 |
|------|----------|------|
| FOFA | ≤ 5 次查询 | 使用精确 title/host 查询 |
| Hunter | ≤ 3 次查询 | 使用单 IP 查询 |
| Quake | ≤ 2 次查询 | 使用精确 title 查询 |
| ZoomEye | ≤ 2 次查询 | 使用 hostname 查询 |
| Shodan | ≤ 2 次查询 | 使用 hostname 查询 |
| ICP | ≤ 3 次查询 | 使用单域名查询 |
| **总计** | **≤ 17 次** | 全部使用精确查询，避免大批量 |

浏览器采集测试额外消耗（不走 API，走 Extension Bridge）：
- 每个引擎打开 1 次搜索页 → 5 次页面加载
- 不消耗引擎 API 配额

---

## 七、风险与注意事项

1. **搜索引擎 DOM 结构变化**：SPA 框架（Vue/React）升级可能导致选择器失效，需要定期验证
2. **截图相关任务（ST-02/ST-03）需要 Chrome 或 Extension Bridge**：测试前确认截图引擎可用
3. **分布式任务（ST-08）需要至少一个在线节点**：无节点时跳过
4. **ICP 查询（ST-21）依赖 ICP API 配置**：确认 `config.yaml` 中 ICP 配置正确
5. **Cookie 验证（ST-06）和登录状态（ST-07）依赖引擎 Cookie**：未配置 Cookie 时结果为"未登录"，这是正常的
6. **测试不应修改生产数据**：清理类任务（ST-11/ST-12/ST-19）使用小 max_age_days 避免误删
7. **Chrome MCP 依赖**：需要 Node.js 环境和 Chrome 浏览器，确认测试环境可用
