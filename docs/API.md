# UniMap API 文档

## 1. 概述

UniMap 提供了丰富的 API 接口，支持查询、截图、篡改检测等功能。本文档详细描述了所有 API 接口的使用方法和参数说明。

## 2. 基础信息

### 2.1 基础 URL

所有 API 接口的基础 URL 为：`http://localhost:8448`

### 2.2 请求格式

- 大多数 API 接口使用 `POST` 方法，需要在请求体中传递 JSON 格式的数据
- 部分查询接口使用 `GET` 方法，通过 URL 参数传递参数

### 2.3 响应格式

所有 API 接口返回 JSON 格式的数据，包含以下字段：

```json
{
  "success": true,  // 是否成功
  "message": "操作成功",  // 提示信息
  "data": {},  // 数据
  "error": "错误信息"  // 错误信息（仅当 success 为 false 时存在）
}
```

## 3. API 接口

### 3.1 查询相关接口

#### 3.1.1 健康检查

- **接口**: `GET /health`
- **功能**: 检查服务是否正常运行
- **参数**: 无
- **返回**: 
  ```json
  {
    "status": "ok",
    "timestamp": 1679800000
  }
  ```

#### 3.1.2 指标监控

- **接口**: `GET /metrics`
- **功能**: 获取服务指标
- **参数**: 无
- **返回**: Prometheus 格式的指标数据

#### 3.1.3 页面查询

- **接口**: `GET /query`
- **功能**: 页面查询接口（用于前端页面）
- **参数**: 
  - `q`: 查询语句
  - `e`: 引擎列表（逗号分隔）
  - `l`: 限制数量
  - `offset`: 偏移量
- **返回**: 查询结果页面

#### 3.1.4 API 查询

- **接口**: `POST /api/query`
- **功能**: API 查询接口
- **参数**: 
  ```json
  {
    "query": "country=\"CN\" && port=\"80\"",
    "engines": ["fofa", "hunter"],
    "limit": 100,
    "offset": 0,
    "timeout": 30
  }
  ```
- **返回**: 
  ```json
  {
    "success": true,
    "data": {
      "results": [...],
      "total": 100,
      "engines": ["fofa", "hunter"]
    }
  }
  ```

#### 3.1.5 查询状态

- **接口**: `GET /api/query/status`
- **功能**: 获取查询状态
- **参数**: 
  - `task_id`: 任务 ID
- **返回**: 
  ```json
  {
    "success": true,
    "data": {
      "status": "completed",
      "progress": 100,
      "results": [...]
    }
  }
  ```

### 3.2 Cookie 管理接口

#### 3.2.1 保存 Cookie

- **接口**: `POST /api/cookies`
- **功能**: 保存 Cookie
- **参数**: 
  ```json
  {
    "engine": "fofa",
    "cookies": "cookie1=value1; cookie2=value2"
  }
  ```
- **返回**: 
  ```json
  {
    "success": true,
    "message": "Cookie 保存成功"
  }
  ```

#### 3.2.2 验证 Cookie

- **接口**: `POST /api/cookies/verify`
- **功能**: 验证 Cookie 是否有效
- **参数**: 
  ```json
  {
    "engine": "fofa",
    "cookies": "cookie1=value1; cookie2=value2"
  }
  ```
- **返回**: 
  ```json
  {
    "success": true,
    "data": {
      "valid": true,
      "message": "Cookie 有效"
    }
  }
  ```

#### 3.2.3 导入 Cookie

- **接口**: `POST /api/cookies/import`
- **功能**: 导入 Cookie JSON
- **参数**: 
  ```json
  {
    "cookies": {
      "fofa": "cookie1=value1; cookie2=value2",
      "hunter": "cookie1=value1; cookie2=value2"
    }
  }
  ```
- **返回**: 
  ```json
  {
    "success": true,
    "message": "Cookie 导入成功"
  }
  ```

### 3.3 CDP 接口

#### 3.3.1 CDP 状态

- **接口**: `GET /api/cdp/status`
- **功能**: 获取 CDP 状态
- **参数**: 无
- **返回**: 
  ```json
  {
    "success": true,
    "data": {
      "status": "connected",
      "version": "110.0.5481.77"
    }
  }
  ```

#### 3.3.2 CDP 连接

- **接口**: `POST /api/cdp/connect`
- **功能**: 连接 CDP
- **参数**: 
  ```json
  {
    "address": "localhost:9222"
  }
  ```
- **返回**: 
  ```json
  {
    "success": true,
    "message": "CDP 连接成功"
  }
  ```

### 3.4 WebSocket 接口

#### 3.4.1 WebSocket

- **接口**: `GET /api/ws`
- **功能**: WebSocket 连接
- **参数**: 无
- **返回**: WebSocket 连接

### 3.5 截图接口

#### 3.5.1 单页截图

- **接口**: `POST /api/screenshot`
- **功能**: 对单个页面进行截图
- **参数**: 
  ```json
  {
    "url": "https://example.com",
    "width": 1920,
    "height": 1080,
    "timeout": 30
  }
  ```
- **返回**: 
  ```json
  {
    "success": true,
    "data": {
      "file_path": "./screenshots/example.com.png"
    }
  }
  ```

#### 3.5.2 搜索引擎截图

- **接口**: `GET /api/screenshot/search-engine`
- **功能**: 对搜索引擎结果进行截图
- **参数**: 
  - `query`: 搜索查询
  - `engine`: 搜索引擎
  - `page`: 页码
- **返回**: 
  ```json
  {
    "success": true,
    "data": {
      "file_path": "./screenshots/search_fofa_example.png"
    }
  }
  ```

#### 3.5.3 目标截图

- **接口**: `POST /api/screenshot/target`
- **功能**: 对目标进行截图
- **参数**: 
  ```json
  {
    "target": {
      "ip": "192.168.1.1",
      "port": "80",
      "protocol": "http"
    },
    "width": 1920,
    "height": 1080,
    "timeout": 30
  }
  ```
- **返回**: 
  ```json
  {
    "success": true,
    "data": {
      "file_path": "./screenshots/192.168.1.1_80.png"
    }
  }
  ```

#### 3.5.4 批量截图

- **接口**: `POST /api/screenshot/batch`
- **功能**: 批量截图
- **参数**: 
  ```json
  {
    "targets": [
      {
        "ip": "192.168.1.1",
        "port": "80",
        "protocol": "http"
      },
      {
        "ip": "192.168.1.2",
        "port": "443",
        "protocol": "https"
      }
    ],
    "width": 1920,
    "height": 1080,
    "concurrency": 5,
    "timeout": 30
  }
  ```
- **返回**: 
  ```json
  {
    "success": true,
    "data": {
      "batch_id": "batch_1234567890"
    }
  }
  ```

#### 3.5.5 批量 URL 截图

- **接口**: `POST /api/screenshot/batch-urls`
- **功能**: 批量 URL 截图
- **参数**: 
  ```json
  {
    "urls": ["https://example.com", "https://google.com"],
    "batch_id": "batch_1234567890",
    "width": 1920,
    "height": 1080,
    "concurrency": 5,
    "timeout": 30
  }
  ```
- **返回**: 
  ```json
  {
    "success": true,
    "data": {
      "batch_id": "batch_1234567890",
      "total": 2,
      "success": 2,
      "failed": 0
    }
  }
  ```

#### 3.5.6 获取截图批次

- **接口**: `GET /api/screenshot/batches`
- **功能**: 获取截图批次列表
- **参数**: 无
- **返回**: 
  ```json
  {
    "success": true,
    "data": {
      "batches": [
        {
          "id": "batch_1234567890",
          "total": 2,
          "success": 2,
          "failed": 0,
          "timestamp": 1679800000
        }
      ]
    }
  }
  ```

#### 3.5.7 获取截图批次文件

- **接口**: `GET /api/screenshot/batches/files`
- **功能**: 获取截图批次文件列表
- **参数**: 
  - `batch_id`: 批次 ID
- **返回**: 
  ```json
  {
    "success": true,
    "data": {
      "files": [
        {
          "path": "./screenshots/batch_1234567890/example.com.png",
          "url": "https://example.com",
          "status": "success"
        }
      ]
    }
  }
  ```

#### 3.5.8 删除截图批次

- **接口**: `DELETE /api/screenshot/batches/delete`
- **功能**: 删除截图批次
- **参数**: 
  - `batch_id`: 批次 ID
- **返回**: 
  ```json
  {
    "success": true,
    "message": "批次删除成功"
  }
  ```

#### 3.5.9 删除截图文件

- **接口**: `DELETE /api/screenshot/file/delete`
- **功能**: 删除截图文件
- **参数**: 
  - `file_path`: 文件路径
- **返回**: 
  ```json
  {
    "success": true,
    "message": "文件删除成功"
  }
  ```

#### 3.5.10 获取截图文件

- **接口**: `GET /screenshots/`
- **功能**: 获取截图文件
- **参数**: 文件路径（作为 URL 路径的一部分）
- **返回**: 图片文件

### 3.6 导入接口

#### 3.6.1 导入 URL

- **接口**: `POST /api/import/urls`
- **功能**: 导入 URL 列表
- **参数**: 
  ```json
  {
    "urls": ["https://example.com", "https://google.com"]
  }
  ```
- **返回**: 
  ```json
  {
    "success": true,
    "data": {
      "total": 2,
      "valid": 2,
      "invalid": 0
    }
  }
  ```

#### 3.6.2 URL 可达性检测

- **接口**: `POST /api/url/reachability`
- **功能**: 检测 URL 可达性
- **参数**: 
  ```json
  {
    "urls": ["https://example.com", "https://google.com"],
    "concurrency": 5
  }
  ```
- **返回**: 
  ```json
  {
    "success": true,
    "data": {
      "summary": {
        "total": 2,
        "reachable": 2,
        "unreachable": 0,
        "invalid_format": 0
      },
      "results": [
        {
          "url": "https://example.com",
          "status": "reachable",
          "reason": "HTTP 200"
        }
      ]
    }
  }
  ```

### 3.7 篡改检测接口

#### 3.7.1 篡改检测

- **接口**: `POST /api/tamper/check`
- **功能**: 检测网站是否被篡改
- **参数**: 
  ```json
  {
    "urls": ["https://example.com", "https://google.com"],
    "concurrency": 5,
    "mode": "relaxed"
  }
  ```
- **返回**: 
  ```json
  {
    "success": true,
    "mode": "relaxed",
    "summary": {
      "total": 2,
      "tampered": 0,
      "safe": 2,
      "noBaseline": 0,
      "unreachable": 0,
      "failed": 0
    },
    "results": [
      {
        "url": "https://example.com",
        "current_hash": {
          "url": "https://example.com",
          "title": "Example Domain",
          "full_hash": "abcdef123456",
          "segment_hashes": [...],
          "timestamp": 1679800000
        },
        "baseline_hash": {
          "url": "https://example.com",
          "title": "Example Domain",
          "full_hash": "abcdef123456",
          "segment_hashes": [...],
          "timestamp": 1679700000
        },
        "tampered": false,
        "status": "normal",
        "timestamp": 1679800000
      }
    ]
  }
  ```

#### 3.7.2 设置基线

- **接口**: `POST /api/tamper/baseline`
- **功能**: 为网站设置基线
- **参数**: 
  ```json
  {
    "urls": ["https://example.com", "https://google.com"],
    "concurrency": 5
  }
  ```
- **返回**: 
  ```json
  {
    "success": true,
    "summary": {
      "total": 2,
      "saved": 2,
      "unreachable": 0,
      "failed": 0
    },
    "results": [
      {
        "url": "https://example.com",
        "title": "Example Domain",
        "full_hash": "abcdef123456",
        "segment_hashes": [...],
        "timestamp": 1679800000,
        "status": "success"
      }
    ]
  }
  ```

#### 3.7.3 获取基线列表

- **接口**: `GET /api/tamper/baseline/list`
- **功能**: 获取基线列表
- **参数**: 无
- **返回**: 
  ```json
  {
    "success": true,
    "urls": ["https://example.com", "https://google.com"],
    "count": 2
  }
  ```

#### 3.7.4 删除基线

- **接口**: `DELETE /api/tamper/baseline/delete`
- **功能**: 删除基线
- **参数**: 
  - `url`: URL
- **返回**: 
  ```json
  {
    "success": true,
    "message": "基线删除成功",
    "url": "https://example.com"
  }
  ```

#### 3.7.5 获取历史记录

- **接口**: `GET /api/tamper/history`
- **功能**: 获取检测历史记录
- **参数**: 
  - `limit`: 限制数量
  - `url`: URL 过滤
  - `type`: 类型过滤
  - `mode`: 模式过滤
  - `q`: 搜索关键词
  - `start_time`: 开始时间（时间戳）
  - `end_time`: 结束时间（时间戳）
- **返回**: 
  ```json
  {
    "success": true,
    "count": 10,
    "records": [
      {
        "id": "1234567890",
        "url": "https://example.com",
        "check_type": "normal",
        "detection_mode": "relaxed",
        "status": "normal",
        "tampered": false,
        "tampered_segments": [],
        "changes_count": 0,
        "timestamp": 1679800000,
        "baseline_timestamp": 1679700000,
        "current_full_hash": "abcdef123456",
        "baseline_full_hash": "abcdef123456"
      }
    ],
    "urls": ["https://example.com", "https://google.com"]
  }
  ```

#### 3.7.6 删除历史记录

- **接口**: `DELETE /api/tamper/history/delete`
- **功能**: 删除历史记录
- **参数**: 
  - `url`: URL
- **返回**: 
  ```json
  {
    "success": true,
    "url": "https://example.com"
  }
  ```

### 3.8 定时任务接口

#### 3.8.1 创建任务

- **接口**: `POST /api/scheduler/tasks`
- **功能**: 创建新的定时任务
- **参数**:
  ```json
  {
    "name": "每日企业备案巡检",
    "type": "icp_query",
    "cron_expr": "0 0 9 * * *",
    "payload": {
      "queries": ["example.com", "test.com"],
      "type": "web",
      "page": 1,
      "page_size": 40
    },
    "timeout_seconds": 600,
    "max_retries": 1
  }
  ```
- **返回**: 包含新创建任务信息的 JSON

#### 3.8.2 任务类型列表

定时任务系统支持以下任务类型（22 种）：

| 编号 | 类型 | 标签 | 说明 |
|------|------|------|------|
| ST-01 | `query` | UQL 查询 | 统一查询语言查询 |
| ST-02 | `search_screenshot` | 搜索引擎截图 | 搜索引擎结果截图 |
| ST-03 | `batch_screenshot` | 批量截图 | 批量 URL 截图 |
| ST-04 | `tamper_check` | 篡改检测 | 网站篡改检测 |
| ST-05 | `url_reachability` | URL 可达性检测 | URL 可达性检查 |
| ST-06 | `cookie_verify` | Cookie 验证 | 引擎 Cookie 验证 |
| ST-07 | `login_status_check` | 登录状态检测 | 引擎登录状态检查 |
| ST-08 | `distributed_submit` | 分布式任务提交 | 分布式节点任务提交 |
| ST-09 | `export` | 数据导出 | 数据导出（JSON/Excel/CSV） |
| ST-10 | `port_scan` | 端口扫描 | URL 端口扫描 |
| ST-11 | `screenshot_cleanup` | 截图清理 | 过期截图清理 |
| ST-12 | `tamper_cleanup` | 篡改记录清理 | 过期篡改记录清理 |
| ST-13 | `quota_monitor` | 配额监控 | 引擎 API 配额监控 |
| ST-14 | `alert_summary` | 告警汇总 | 告警记录汇总 |
| ST-15 | `baseline_refresh` | 基线刷新 | 篡改检测基线刷新 |
| ST-16 | `url_import` | URL 导入 | 从文件导入 URL 列表 |
| ST-17 | `plugin_health` | 插件健康检查 | 插件健康状态检查 |
| ST-18 | `bridge_token` | Bridge 令牌轮换 | Bridge 服务令牌管理 |
| ST-19 | `alert_silence` | 告警静默窗口 | 告警静默时段管理 |
| ST-20 | `cache_warmup` | 缓存预热 | 查询缓存预热 |
| ST-21 | `icp_query` | ICP 备案查询 | ICP 备案状态周期性查询 |
| ST-22 | `icp_import` | ICP 关键词导入 | 从 CSV 文件批量导入关键词并自动创建 ICP 查询任务 |

#### 3.8.3 `icp_query` 任务 Payload 字段

| 字段 | 类型 | 必填 | 默认 | 说明 |
|------|------|------|------|------|
| `queries` | `[]string` | 否* | `[]` | 关键词列表（公司名、域名等） |
| `query` | `string` | 否* | `""` | 单关键词；兼容单值场景 |
| `type` | `string` | 否 | 配置中的 `default_type` | 查询类型：`web/app/mapp/kapp/bweb/bapp/bmapp/bkapp` |
| `page` | `int` | 否 | `1` | 起始页 |
| `page_size` | `int` | 否 | `20` | 每页条数，最大 100 |
| `fail_fast` | `bool` | 否 | `false` | `true` 时遇到失败立即返回 |

\* `queries` 与 `query` 至少一个非空。最多支持 100 个关键词。

#### 3.8.4 内置任务模板

| 模板 ID | 名称 | Cron | 用途 |
|---------|------|------|------|
| `tmpl_daily_icp_company_watch` | 每日企业备案巡检 | `0 0 9 * * *` | 每天早上 9 点查询企业 ICP 备案状态 |
| `tmpl_weekly_icp_domain_scan` | 每周域名备案变更扫描 | `0 0 3 * * 1` | 每周一凌晨 3 点扫描域名 ICP 变更 |

#### 3.8.5 任务通知配置

创建或编辑任务时，可通过 `notifications` 字段配置执行结果推送：

```json
{
  "name": "每日企业备案巡检",
  "type": "icp_query",
  "cron_expr": "0 0 9 * * *",
  "notifications": {
    "enabled": true,
    "on_success": true,
    "on_failure": true,
    "on_timeout": true,
    "channel_ids": ["team-dingtalk", "ops-feishu"]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `enabled` | `bool` | 任务级通知总开关 |
| `on_success` | `bool` | 执行成功时是否通知 |
| `on_failure` | `bool` | 执行失败时是否通知 |
| `on_timeout` | `bool` | 执行超时时是否通知 |
| `channel_ids` | `[]string` | 引用全局已配置的通知渠道 ID（多选） |

**两级开关语义：** 实际发送 = `全局 notifications.enabled && task.notifications.enabled && channel.enabled && 事件匹配`

#### 3.9 通知系统接口

##### 3.9.1 列出全局通知渠道

- **接口**: `GET /api/notifications/channels`
- **功能**: 返回 `config.yaml` 中已配置的通知渠道列表（脱敏，不含密钥）
- **返回**:
  ```json
  {
    "channels": [
      {"id": "team-dingtalk", "type": "dingtalk", "enabled": true},
      {"id": "ops-feishu", "type": "feishu", "enabled": true}
    ]
  }
  ```
- **说明**: 前端创建/编辑任务时通过此接口获取渠道多选列表

##### 3.9.2 重载通知渠道

- **接口**: `POST /api/notifications/reload`
- **功能**: 从最新配置文件重载通知渠道注册表
- **返回**:
  ```json
  {"status": "ok", "loaded": 4}
  ```

##### 3.9.3 全局通知渠道配置

在 `config.yaml` 中通过 `notifications.channels` 块配置：

```yaml
notifications:
  enabled: true
  send_timeout_sec: 10
  max_retries: 0
  channels:
    - id: "team-dingtalk"
      type: "dingtalk"             # dingtalk | feishu | wecom | webhook
      enabled: true
      webhook_url: "https://oapi.dingtalk.com/robot/send?access_token=xxx"
      secret: "${DINGTALK_SECRET}"  # 钉钉/飞书加签密钥
      allow_private_ip: false       # 默认禁止内网 IP（SSRF 防护）
    - id: "ops-feishu"
      type: "feishu"
      enabled: true
      webhook_url: "https://open.feishu.cn/open-apis/bot/v2/hook/xxx"
      secret: "${FEISHU_SECRET}"
    - id: "ops-wecom"
      type: "wecom"
      enabled: true
      webhook_url: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | `string` | 是 | 渠道唯一标识，任务通过此 ID 引用 |
| `type` | `string` | 是 | 渠道类型：`dingtalk` / `feishu` / `wecom` / `webhook` |
| `enabled` | `bool` | 是 | 渠道启用开关 |
| `webhook_url` | `string` | 是 | 机器人 Webhook URL（钉钉/飞书/企微官方地址） |
| `secret` | `string` | 否 | 加签密钥（钉钉/飞书支持 HMAC-SHA256 签名） |
| `headers` | `map[string]string` | 否 | 自定义请求头（通用 webhook 用） |
| `allow_private_ip` | `bool` | 否 | 是否允许私网 IP（默认 false，SSRF 防护） |

### 4. 错误码

| 错误码 | 描述 |
|--------|------|
| 400 | 请求参数错误 |
| 401 | 未授权 |
| 403 | 禁止访问 |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |
| 503 | 服务不可用 |

## 5. 限流策略

部分 API 接口受到限流保护，具体包括：

- 查询相关接口
- 截图相关接口
- 导入相关接口
- 篡改检测相关接口

限流策略：
- 每个 IP 每分钟最多 60 个请求
- 每个 API 接口有独立的限流配置

## 6. 安全注意事项

1. **认证授权**：部分 API 接口需要认证，请确保在请求中包含正确的认证信息
2. **输入验证**：所有用户输入都会经过验证，请勿尝试注入恶意代码
3. **HTTPS**：建议在生产环境中使用 HTTPS 协议
4. **CORS**：API 接口支持 CORS，但仅允许指定的域名访问

## 7. 示例代码

### 7.1 使用 cURL 调用 API

```bash
# 调用篡改检测接口
curl -X POST http://localhost:8448/api/tamper/check \
  -H "Content-Type: application/json" \
  -d '{"urls": ["https://example.com"], "concurrency": 5, "mode": "relaxed"}'

# 调用设置基线接口
curl -X POST http://localhost:8448/api/tamper/baseline \
  -H "Content-Type: application/json" \
  -d '{"urls": ["https://example.com"], "concurrency": 5}'

# 获取历史记录（带时间范围）
curl "http://localhost:8448/api/tamper/history?limit=100&start_time=1679700000&end_time=1679800000"

# 获取基线列表
curl http://localhost:8448/api/tamper/baseline/list

# 删除基线
curl -X DELETE "http://localhost:8448/api/tamper/baseline/delete?url=https://example.com"

# 删除历史记录
curl -X DELETE "http://localhost:8448/api/tamper/history/delete?url=https://example.com"
```

### 7.2 使用 Python 调用 API

```python
import requests
import json

# 调用篡改检测接口
url = "http://localhost:8448/api/tamper/check"
data = {
    "urls": ["https://example.com"],
    "concurrency": 5,
    "mode": "relaxed"
}
response = requests.post(url, json=data)
print(response.json())

# 调用设置基线接口
url = "http://localhost:8448/api/tamper/baseline"
data = {
    "urls": ["https://example.com"],
    "concurrency": 5
}
response = requests.post(url, json=data)
print(response.json())

# 获取历史记录（带时间范围）
url = "http://localhost:8448/api/tamper/history"
params = {
    "limit": 100,
    "start_time": 1679700000,
    "end_time": 1679800000
}
response = requests.get(url, params=params)
print(response.json())

# 获取基线列表
url = "http://localhost:8448/api/tamper/baseline/list"
response = requests.get(url)
print(response.json())

# 删除基线
url = "http://localhost:8448/api/tamper/baseline/delete"
params = {
    "url": "https://example.com"
}
response = requests.delete(url, params=params)
print(response.json())

# 删除历史记录
url = "http://localhost:8448/api/tamper/history/delete"
params = {
    "url": "https://example.com"
}
response = requests.delete(url, params=params)
print(response.json())
```

### 7.3 使用 JavaScript 调用 API

```javascript
// 调用篡改检测接口
fetch('http://localhost:8448/api/tamper/check', {
    method: 'POST',
    headers: {
        'Content-Type': 'application/json'
    },
    body: JSON.stringify({
        urls: ['https://example.com'],
        concurrency: 5,
        mode: 'relaxed'
    })
})
.then(response => response.json())
.then(data => console.log(data));

// 调用设置基线接口
fetch('http://localhost:8448/api/tamper/baseline', {
    method: 'POST',
    headers: {
        'Content-Type': 'application/json'
    },
    body: JSON.stringify({
        urls: ['https://example.com'],
        concurrency: 5
    })
})
.then(response => response.json())
.then(data => console.log(data));

// 获取历史记录（带时间范围）
const params = new URLSearchParams({
    limit: 100,
    start_time: 1679700000,
    end_time: 1679800000
});
fetch(`http://localhost:8448/api/tamper/history?${params.toString()}`)
.then(response => response.json())
.then(data => console.log(data));

// 获取基线列表
fetch('http://localhost:8448/api/tamper/baseline/list')
.then(response => response.json())
.then(data => console.log(data));

// 删除基线
fetch('http://localhost:8448/api/tamper/baseline/delete?url=https://example.com', {
    method: 'DELETE'
})
.then(response => response.json())
.then(data => console.log(data));

// 删除历史记录
fetch('http://localhost:8448/api/tamper/history/delete?url=https://example.com', {
    method: 'DELETE'
})
.then(response => response.json())
.then(data => console.log(data));
```

## 8. 总结

本文档详细描述了 UniMap 的 API 接口，包括查询、截图、篡改检测等功能。使用这些 API 接口，您可以：

1. 执行网络空间资产查询
2. 对网站进行截图
3. 检测网站是否被篡改
4. 管理检测历史记录
5. 导出检测结果

如果您有任何疑问或建议，请随时联系我们。