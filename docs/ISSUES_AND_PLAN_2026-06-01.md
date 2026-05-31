# UniMap 问题记录与整改计划

日期：2026-06-01

---

## 一、问题记录

### 1.1 截图示例 URL 超时导致截图失败

**现象**：监控页面点击"加载示例"后执行批量截图，`https://httpbin.org` 响应慢（通常 3-10 秒），导致截图超时失败。

**根因**：`httpbin.org` 是公共测试服务，响应不稳定。截图引擎对单 URL 有超时限制，慢响应直接判定失败。

**影响**：用户体验差，首次使用批量截图功能时误以为功能不可用。

**当前状态**：示例 URL 已从 `test.example.org`（DNS NXDOMAIN 必然失败）改为 `httpbin.org`，但 httpbin 本身响应慢，仍会导致截图失败。

**建议修复**：替换为响应稳定的公网站点，如 `https://www.example.com`、`https://www.baidu.com`、`https://www.bing.com`。

---

### 1.2 查询页面多引擎报错

**现象**：执行查询时，页面底部显示多条错误信息：

```
• engine zoomeye error: ZoomEye API Payment Required (402): credits_insufficient
• engine hunter error: hunter rate limit exceeded: 请求太多啦，稍后再试试
• engine shodan error: HTTP 403: Requires membership or higher to access
• engine fofa error: FOFA API error: [820031] F点余额不足
```

**根因**：

| 引擎 | 错误码 | 原因 | 严重程度 |
|------|--------|------|----------|
| ZoomEye | 402 | 账户积分不足 | 账户问题 |
| Hunter | 429 | 请求频率超限 | 限流触发 |
| Shodan | 403 | 需要付费会员 | 账户权限 |
| FOFA | 820031 | F点余额不足 | 账户问题 |

**分析**：这些不是代码 Bug，而是第三方 API 账户额度/权限问题。但当前前端展示方式存在问题：
- 错误信息直接暴露原始 API 响应，用户难以理解
- 多个引擎同时报错时，错误列表很长，干扰正常结果查看
- 没有区分"部分引擎失败"和"全部失败"的展示策略

**建议修复**：
1. 后端错误消息脱敏，转换为用户友好提示（如"ZoomEye 额度不足，请充值或更换 API Key"）
2. 前端错误信息折叠展示，默认收起，点击展开详情
3. 区分"查询成功（部分引擎失败）"和"查询失败"两种状态
4. 对限流类错误（Hunter 429）自动重试一次

---

### 1.3 循环/轮询在操作结束后持续执行

**现象**：截图或查询操作执行完毕后，页面仍持续发送网络请求。

**根因**：`web/static/js/main.js` 中存在多个未清理的定时器：

| 位置 | 代码 | 问题 | 影响 |
|------|------|------|------|
| `main.js:175` | `setInterval(refresh, 15000)` | CDP 状态轮询，interval ID 未保存，无法清除 | 每 15 秒请求 `/api/v1/cdp/status` |
| `main.js:204` | `setInterval(refresh, 5000)` | Bridge 状态轮询，interval ID 未保存，无法清除 | **每 5 秒**请求 `/api/v1/screenshot/bridge/status` |
| `main.js:902` | `setInterval(refreshLoginStatus, 15000)` | 登录状态轮询，已保存 ID 但从未停止 | 每 15 秒请求 `/api/v1/cookies/login-status` |
| `main.js:934` | `setTimeout(initWebSocket, 5000)` | WebSocket 重连，无最大重试次数和退避策略 | 连接失败时每 5 秒无限重连 |

**最严重**：Bridge 状态轮询（每 5 秒一次）是最明显的，用户能感知到持续的网络活动。

**补充发现 — 后端 Goroutine 泄漏**：

| 位置 | 问题 | 影响 |
|------|------|------|
| `orchestrator.go:625-630` | `SearchEnginesWithContext` 中 ctx 取消后，关闭 channel 的 goroutine 仍在运行 | 泄漏最长 ~36 秒 |
| `workerpool.go:211-218` | `startLoadMonitoring` 使用 `default` 分支忙等待（每 100ms 唤醒一次） | 浪费 CPU |

**建议修复**：
1. **前端**：保存所有 `setInterval`/`setTimeout` 的 ID，页面离开或操作完成后清除
2. **前端**：WebSocket 重连增加指数退避（5s → 10s → 20s → 40s）和最大重试次数（如 5 次）
3. **后端**：`orchestrator.go` 的 goroutine 改为监听 ctx.Done() 后主动退出
4. **后端**：`workerpool.go` 的 `startLoadMonitoring` 移除 `default` 分支，改用 `exitCh` 通道

---

### 1.4 ICP 备案查询仅支持单类型

**现象**：ICP 备案查询页面只能选择单一类型（web/app/mapp/kapp 等），无法同时查询多个类型的备案信息。

**当前架构**：

```
前端 <select> → handler 读取单个 type → adapter.ICPSearch(type) → sidecar GET /query/{type}
```

全链路假设 type 为单个字符串值。

**可用类型**：

| 类型 | 说明 | 类型 | 说明 |
|------|------|------|------|
| `web` | 网站备案 | `bweb` | 网站黑名单 |
| `app` | APP 备案 | `bapp` | APP 黑名单 |
| `mapp` | 小程序备案 | `bmapp` | 小程序黑名单 |
| `kapp` | 快应用备案 | `bkapp` | 快应用黑名单 |

**需求**：支持同时选择多个类型（如 web + app），并发请求到 sidecar，汇总结果返回。

---

## 二、整改计划

### Phase 1：紧急修复（影响用户体验）

| 序号 | 任务 | 涉及文件 | 工作量 |
|------|------|----------|--------|
| 1.1 | 替换截图示例 URL 为稳定公网站点 | `monitor.html` | 5 分钟 |
| 1.2 | 查询错误信息前端折叠展示 | `monitor.html` 或 `index.html` | 30 分钟 |
| 1.3 | 前端定时器清理（保存 ID + 页面离开时清除） | `main.js` | 1 小时 |

### Phase 2：后端稳定性修复

| 序号 | 任务 | 涉及文件 | 工作量 |
|------|------|----------|--------|
| 2.1 | Orchestrator goroutine 泄漏修复 | `orchestrator.go` | 1 小时 |
| 2.2 | WorkerPool 忙等待修复 | `workerpool.go` | 30 分钟 |
| 2.3 | WebSocket 重连增加退避策略 | `main.js` | 30 分钟 |

### Phase 3：ICP 多类型查询

| 序号 | 任务 | 涉及文件 | 工作量 |
|------|------|----------|--------|
| 3.1 | Adapter 层：新增 `ICPSearchMultiType()` 并发查询 | `internal/adapter/icp.go` | 2 小时 |
| 3.2 | Handler 层：支持 `type=web,app,mapp` 逗号分隔参数 | `web/icp_handlers.go` | 1 小时 |
| 3.3 | 前端：单选下拉改为多选复选框 | `web/templates/icp.html` | 1.5 小时 |
| 3.4 | 结果展示：按类型分组或增加类型标签 | `web/templates/icp.html` | 1 小时 |
| 3.5 | 调度器：支持多类型定时任务 | `internal/scheduler/executor.go` | 1.5 小时 |

### Phase 4：查询引擎错误优化（可选）

| 序号 | 任务 | 涉及文件 | 工作量 |
|------|------|----------|--------|
| 4.1 | 错误消息脱敏与用户友好化 | `internal/adapter/*.go` | 2 小时 |
| 4.2 | 限流错误自动重试（Hunter 429） | `internal/adapter/orchestrator.go` | 1 小时 |
| 4.3 | 前端区分"部分成功"与"全部失败" | `web/static/js/main.js` | 1 小时 |

---

## 三、ICP 多类型查询技术方案

### 3.1 推荐方案：Adapter 层并发扇出

```
前端 multi-select → handler 解析 types[] → adapter.ICPSearchMultiType(types)
                                                ├── goroutine: /query/web
                                                ├── goroutine: /query/app
                                                └── goroutine: /query/mapp
                                                → 合并结果 → 返回
```

**优点**：
- Sidecar API 保持单类型不变（外部服务，不改动）
- 并发请求，总耗时 = max(单次耗时)，而非 sum
- 每条结果携带 `type` 字段，前端可按类型分组展示

### 3.2 API 设计

**请求**：`GET /api/v1/icp/query?search=xxx&type=web,app,mapp`

**响应**：
```json
{
  "success": true,
  "results": [
    {"type": "web", "label": "网站备案", "total": 10, "items": [...]},
    {"type": "app", "label": "APP备案", "total": 3, "items": [...]},
    {"type": "mapp", "label": "小程序备案", "total": 0, "items": []}
  ],
  "total": 13
}
```

### 3.3 数据库兼容

- 现有 `icp_query_runs` 表的 `query_type` 列保持不变
- 多类型查询时，为每个 type 创建独立的 run 记录
- 历史查询和对比功能按 type 分别展示，无需改表结构

---

## 四、优先级排序

```
Phase 1 (紧急) → Phase 2 (稳定) → Phase 3 (新功能) → Phase 4 (优化)
   ↓                  ↓                  ↓                  ↓
 截图示例URL       goroutine泄漏      ICP多类型查询      错误消息优化
 定时器清理        忙等待修复          前端多选UI         限流重试
 错误折叠展示      WS重连退避          调度器支持         部分成功展示
```

**建议执行顺序**：Phase 1 全部 → Phase 2.1 + 2.2 → Phase 3 全部 → Phase 2.3 + Phase 4
