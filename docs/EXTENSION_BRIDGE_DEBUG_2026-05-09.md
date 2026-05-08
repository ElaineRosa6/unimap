# Extension 模式调试与修复报告

> **日期：** 2026-05-09
> **状态：** 扩展代码已更新，后端待修复

---

## 1. 调试结论摘要

| 组件 | 状态 | 说明 |
|------|------|------|
| Bridge 连接 | ✅ 正常 | 1 live client, router_ext_healthy=true |
| Bridge 认证 | ✅ 正常 | Token 配对/轮换/签名正常 |
| 结构化数据通道 | ✅ 正常 | `structured_collected_data` 可正确回传和解析 |
| 扩展代码 | ✅ 已更新 | 新增登录墙检测、SPA 支持、多选择器 fallback |
| 后端 URL 构建 | ❌ 待修复 | UQL 未翻译直接发送到搜索引擎（问题 #1） |
| 进度反馈 | ❌ 待修复 | WebSocket 查询进度卡在 0%（问题 #2） |
| 登录状态 | ❌ 待修复 | 扩展配对后所有引擎报 true（问题 #4） |

---

## 2. 扩展代码更新（v0.1.0 → v0.2.0）

### 2.1 capture.js 改进

**新增：**
- `ENGINE_SELECTORS` 配置对象，支持多选择器变体 fallback 链
- `isLoginWall()` 登录墙检测函数
- `is_login_wall` 检测结果回传字段
- SPA 页面等待策略（`waitForPageReady` 支持 `spa`/`networkidle`）
- 空行过滤（跳过完全空的采集行）
- 更丰富的错误详情回传（`extraction_error` 字段）

**改进的选择器策略：**
```javascript
// 旧：单一选择器
const rows = document.querySelectorAll(".list_content > tbody > tr");

// 新：多选择器 fallback
for (const rowSel of engineSelectors.row) {
  rows = document.querySelectorAll(rowSel);
  if (rows.length > 0) break;
}
```

**支持的引擎和字段：**
| 引擎 | ip | port | protocol | host | title | 额外字段 |
|------|----|----|----------|----|----|---------|
| FOFA | ✅ | ✅ | ✅ | ✅ | ✅ | country_code, banner |
| Hunter | ✅ | ✅ | ✅ | ✅ | ✅ | banner |
| ZoomEye | ✅ | ✅ | ✅ | ✅ | ✅ | country_code, banner |
| Quake | ✅ | ✅ | ✅ | ✅ | ✅ | server, city, isp |

### 2.2 background.js 改进

**新增：**
- `action === "collect"` 时自动使用 `spa` 等待策略
- 登录墙检测结果的特殊回传格式（`success: false, error_code: "login_required"`）
- `engine` 字段包含在 `structured_collected_data` 中
- `extraction_error` 字段传递采集错误信息

### 2.3 manifest.json 更新

- 版本升级至 `0.2.0`
- 描述更新为包含结构化数据采集
- 添加 icons 配置占位

---

## 3. 后端待修复问题

### 3.1 P0: BuildSearchEngineURL 必须翻译 UQL

**文件：** `internal/screenshot/manager.go` 第 728 行

**当前行为：**
```go
func (m *Manager) BuildSearchEngineURL(engine, query string) string {
    b64Query := base64.StdEncoding.EncodeToString([]byte(query))  // 原始 UQL
    // ...
}
```

**正确行为：**
```go
func (m *Manager) BuildSearchEngineURL(engine, query string, translator func(string, string) string) string {
    translated := translator(engine, query)  // UQL → 引擎原生语法
    b64Query := base64.StdEncoding.EncodeToString([]byte(translated))
    // ...
}
```

**影响：**
- FOFA：简单查询可用（语法巧合相似），复杂查询失败
- Hunter/ZoomEye/Quake：大部分查询失败

### 3.2 P1: 登录状态检测不准确

**文件：** `web/cookie_handlers.go` 第 418-437 行

**当前行为：**
```go
} else if extPaired {
    // 扩展配对了 = 所有引擎已登录
    "logged_in": true,  // 永远为 true
}
```

**修复方案：** 通过 Bridge 让扩展实际打开引擎页面检测登录状态，而不是盲目报告。

---

## 4. 扩展部署方法

### 4.1 本地加载

1. 打开 `chrome://extensions/`
2. 启用"开发者模式"
3. 点击"加载已解压的扩展程序"
4. 选择 `tools/extension-screenshot` 目录

### 4.2 已有扩展更新

1. 在扩展管理页面点击"更新"
2. 或移除后重新加载

### 4.3 验证连接

打开扩展控制台，查看 `chrome.storage.local.get()` 输出：
- `paired: true` — 已配对
- `last_success_at` — 最近成功任务时间
- `last_error` — 最近错误信息

---

## 5. 测试方法

### 5.1 手动测试 collect 流程

```bash
# 1. 获取 Bridge Token
TOKEN=$(curl -s -X POST http://127.0.0.1:8448/api/screenshot/bridge/pair \
  -d '{"client_id":"test","pair_code":"dev-pair"}' \
  -H 'Content-Type: application/json' | jq -r .token)

# 2. 模拟扩展回传结构化数据
curl -s -X POST "http://127.0.0.1:8448/api/screenshot/bridge/mock/result" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "request_id": "test-collect-1",
    "success": true,
    "structured_collected_data": {
      "title": "FOFA 搜索结果",
      "total": 2,
      "has_more": true,
      "engine": "fofa",
      "items": [
        {"ip": "192.168.1.1", "port": 443, "protocol": "https", "host": "test1.com"},
        {"ip": "10.0.0.1", "port": 80, "protocol": "http", "host": "test2.com"}
      ]
    }
  }'
```

### 5.2 验证 Bridge 状态

```bash
curl -s http://127.0.0.1:8448/api/screenshot/bridge/status | jq
```

---

## 6. 修复优先级

1. **P0（阻塞所有浏览器模式功能）**：BuildSearchEngineURL 增加 UQL 翻译
2. **P1（用户体验）**：WebSocket 查询进度反馈
3. **P1（UI 准确性）**：登录状态真实检测
4. **P2（可选增强）**：扩展选择器远程配置更新

## 7. 相关文件

| 文件 | 状态 | 说明 |
|------|------|------|
| `tools/extension-screenshot/src/capture.js` | ✅ 已更新 | v0.2.0 |
| `tools/extension-screenshot/src/background.js` | ✅ 已更新 | v0.2.0 |
| `tools/extension-screenshot/manifest.json` | ✅ 已更新 | v0.2.0 |
| `tools/extension-screenshot/README.md` | ✅ 已更新 | 中文文档 |
| `docs/ISSUES_EXTENSION_MODE_2026-05-09.md` | ✅ 已创建 | 问题清单 |
| `docs/EXTENSION_BRIDGE_DEBUG_2026-05-09.md` | ✅ 已创建 | 调试报告 |
| `internal/screenshot/manager.go` | ❌ 待修复 | BuildSearchEngineURL |
| `web/websocket_handlers.go` | ❌ 待修复 | 进度反馈 |
| `web/cookie_handlers.go` | ❌ 待修复 | 登录状态 |
