# 定时任务系统优化与测试计划

> 创建日期：2026-06-02
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

## 三、飞书推送：测试结果通知

### 3.1 配置要求

在 `config.yaml` 中配置飞书通知渠道：

```yaml
notify:
  channels:
    - id: feishu-test
      type: feishu
      name: "测试结果通知"
      enabled: true
      webhook_url: "https://open.feishu.cn/open-apis/bot/v2/hook/YOUR_WEBHOOK_ID"
      sign_key: "${FEISHU_SIGN_KEY}"  # 可选，HMAC 签名
```

### 3.2 通知内容格式

每个 Runner 测试完成后，发送飞书卡片消息：

```
📋 定时任务测试报告
━━━━━━━━━━━━━━━━
任务类型: ST-01 UQL 查询
执行状态: ✅ 成功
耗时: 1.23s
结果摘要: 返回 5 条资产，引擎 fofa 统计正常
API 消耗: FOFA 查询 1 次
━━━━━━━━━━━━━━━━
任务类型: ST-02 搜索引擎截图
执行状态: ❌ 失败
错误: screenshot engine not initialized
建议: 需要启动 Chrome 或配置 Extension Bridge
```

### 3.3 汇总报告格式

全部测试完成后发送汇总：

```
📊 定时任务全量测试汇总
━━━━━━━━━━━━━━━━
测试时间: 2026-06-02 14:30:00
总任务数: 22
通过: 18 ✅
失败: 2 ❌
跳过: 2 ⏭️ (依赖未配置)
━━━━━━━━━━━━━━━━
❌ 失败项:
  - ST-02 搜索引擎截图: Chrome 未启动
  - ST-03 批量截图: Chrome 未启动
━━━━━━━━━━━━━━━━
⏭️ 跳过项:
  - ST-08 分布式任务提交: 分布式未启用
  - ST-18 Bridge 健康检查: Bridge 未启动
━━━━━━━━━━━━━━━━
API 消耗统计:
  - FOFA: 2 次查询
  - Hunter: 1 次查询
  - ICP: 1 次查询
  总计: 4 次 API 调用
```

---

## 四、实施顺序

| 阶段 | 内容 | 预计耗时 |
|------|------|----------|
| P1 | 前端优化：任务类型分组 + Cron 预设 | 1 天 |
| P2 | 前端优化：任务模板系统 + JSON 编辑器增强 | 1 天 |
| P3 | 后端测试：编写测试脚本 + 测试 payload 文件 | 1 天 |
| P4 | 后端测试：逐项执行测试 + 修复发现的问题 | 1-2 天 |
| P5 | 飞书推送：测试结果自动推送到飞书群 | 0.5 天 |

---

## 五、API 消耗预算

| 引擎 | 计划消耗 | 说明 |
|------|----------|------|
| FOFA | ≤ 5 次查询 | 使用精确 title/host 查询 |
| Hunter | ≤ 3 次查询 | 使用单 IP 查询 |
| Quake | ≤ 2 次查询 | 使用精确 title 查询 |
| ZoomEye | ≤ 2 次查询 | 使用 hostname 查询 |
| Shodan | ≤ 2 次查询 | 使用 hostname 查询 |
| ICP | ≤ 3 次查询 | 使用单域名查询 |
| **总计** | **≤ 17 次** | 全部使用精确查询，避免大批量 |

---

## 六、风险与注意事项

1. **截图相关任务（ST-02/ST-03）需要 Chrome 或 Extension Bridge**：测试前确认截图引擎可用
2. **分布式任务（ST-08）需要至少一个在线节点**：无节点时跳过
3. **ICP 查询（ST-21）依赖 ICP API 配置**：确认 `config.yaml` 中 ICP 配置正确
4. **Cookie 验证（ST-06）和登录状态（ST-07）依赖引擎 Cookie**：未配置 Cookie 时结果为"未登录"，这是正常的
5. **测试不应修改生产数据**：清理类任务（ST-11/ST-12/ST-19）使用小 max_age_days 避免误删
