# Extension Screenshot Bridge

浏览器扩展桥接，支持截图任务执行和从网络空间搜索引擎采集结构化数据。

## 版本历史

| 版本 | 日期 | 说明 |
|------|------|------|
| 0.2.0 | 2026-05-09 | 新增结构化数据采集、登录墙检测、SPA 页面支持 |
| 0.1.0 | 2026-04-03 | MVP 初始版本 |

## 加载到 Chrome/Edge

1. 打开 `chrome://extensions/`
2. 启用"开发者模式"
3. 点击"加载已解压的扩展程序"
4. 选择 `tools/extension-screenshot` 目录

**重要提示**：Web 服务启动后，扩展需要等待 1-2 分钟才能连接成功（服务端需初始化截图管理器和桥接服务）。

## 配置

### API 地址

默认连接 `http://127.0.0.1:8448`。修改方法：

```javascript
chrome.storage.local.set({ apiBaseURL: "http://your-server:port" })
```

### 内存设置（`src/capture.js`）

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `MAX_TAB_POOL_SIZE` | 3 | Tab 池大小 |
| `TAB_REUSE_TIMEOUT_MS` | 30000 | Tab 复用超时(ms) |
| `CAPTURE_MIN_INTERVAL_MS` | 1200 | 截图最小间隔(ms) |

## 功能

### 支持的任务动作

| Action | 说明 | 等待策略 |
|--------|------|---------|
| `open` | 仅打开搜索结果页 | `load` |
| `screenshot` | 打开页面并截图 | `load` |
| `collect` | 打开页面并采集结构化数据 | `spa` (自动) |

### 支持的搜索引擎

| 引擎 | 数据采集 | 登录墙检测 |
|------|---------|-----------|
| FOFA | ✅ | ✅ |
| Hunter | ✅ | ✅ |
| ZoomEye | ✅ | ✅ |
| Quake | ✅ | ✅ |

### 采集数据结构

扩展回传 `structured_collected_data` JSON：

```json
{
  "title": "FOFA - 搜索结果",
  "total": 1250,
  "has_more": true,
  "engine": "fofa",
  "items": [
    {
      "ip": "1.2.3.4",
      "port": 443,
      "protocol": "https",
      "host": "example.com",
      "title": "Example Site",
      "country_code": "CN"
    }
  ]
}
```

### 登录墙检测

当页面检测到登录表单时，扩展会返回：

```json
{
  "success": false,
  "error_code": "login_required",
  "structured_collected_data": {
    "is_login_wall": true,
    "engine": "fofa",
    "title": "FOFA - 请登录"
  }
}
```

## DOM 选择器配置

选择器定义在 `capture.js` 的 `ENGINE_SELECTORS` 对象中，支持多个选择器变体（fallback 链）：

```javascript
const ENGINE_SELECTORS = {
  fofa: {
    row: [
      ".list_content > tbody > tr",      // 首选
      ".result-table tbody tr",           // 备选 1
      "[class*='result'] table tbody tr"  // 备选 2
    ],
    cells: {
      ip: { selector: "td:nth-child(1) a", fallback: "td:nth-child(1)" },
      // ...
    }
  }
};
```

修改选择器后无需重新部署扩展，下次采集任务自动生效。

## Bridge API 端点

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/api/screenshot/bridge/pair` | 配对获取 Token |
| GET | `/api/screenshot/bridge/tasks/next` | 拉取下一个任务 |
| POST | `/api/screenshot/bridge/mock/result` | 回传任务结果 |
| POST | `/api/screenshot/bridge/token/rotate` | 轮换 Token |
| GET | `/api/screenshot/bridge/status` | 查看连接状态 |
| GET | `/api/screenshot/bridge/diagnostic` | 诊断信息 |

## 安全特性

- **Bridge Token 认证**：Bearer Token 方式
- **回调签名**：HMAC-SHA256 签名 + Nonce 防重放
- **Token 轮换**：过期前自动轮换
- **Loopback 限制**：配对和任务拉取仅限本地回环

## 已知问题

1. **UQL 语法未翻译**：后端 `BuildSearchEngineURL` 使用原始 UQL 而非引擎原生语法，导致复杂查询在搜索引擎上失败。需要后端修复。（记录于 `docs/ISSUES_EXTENSION_MODE_2026-05-09.md`）
2. **SPA 选择器稳定性**：现代搜索引擎使用 Vue/React/Angular，DOM 结构可能随版本变化而改变。建议定期验证选择器有效性。
