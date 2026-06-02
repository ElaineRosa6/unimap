# 定时任务系统 — 下一步实施计划

> 创建日期：2026-06-02
> 前置状态：P3-P8 已完成

---

## 待实施阶段

### P1：Chrome MCP DOM 采集测试

**目标**：验证 5 个搜索引擎的浏览器采集选择器是否有效

**环境要求**：
- Node.js ✅ (v24.14.1 已安装)
- Chrome ✅ (已安装)
- Chrome MCP：`npx @anthropic-ai/chrome-mcp@latest`

**测试项**：

| 引擎 | 搜索页 | 验证内容 |
|------|--------|----------|
| FOFA | `fofa.info/result?qbase64=...` | 结果列表选择器、IP/端口/标题字段 |
| Hunter | `hunter.qianxin.com/list?searchValue=...` | 结果卡片、资产信息 |
| Quake | `quake.360.cn/quake/#/searchResult?searchVal=...` | 结果表格、字段映射 |
| ZoomEye | `zoomeye.org/searchResult?q=...` | 结果列表、资产字段 |
| Shodan | `shodan.io/search?query=...` | 结果表格、banner 信息 |

**查询**：使用精确单 IP 查询避免大量结果
- FOFA: `ip="47.95.120.1" && port="80"`
- Hunter: `ip="47.95.120.1" && port="443"`
- 其余类似

**产出**：每个引擎的选择器有效性报告 + 截图存档

---

### P2：采集选择器修复

**目标**：根据 P1 结果更新 Extension 采集脚本

**前置**：P1 完成

**涉及文件**：`tools/extension-screenshot/` 下的采集脚本

**内容**：
- 修复失效的 DOM 选择器
- 更新字段映射
- 验证修复后的采集结果

---

### P9：截图飞书推送

**目标**：截图类任务完成后，飞书通知中嵌入截图图片预览

**方案**：

```
任务执行完成 → 截图文件路径
  → 读取截图文件
  → 调用飞书图片上传 API (im/v1/images) 获取 image_key
  → 构建飞书卡片，嵌入 img 元素
  → 发送 webhook
```

**前置条件**：
- 飞书应用凭证（app_id + app_secret），用于调用图片上传 API
- 或：截图文件通过公网 URL 可访问（直接用 img 标签的 URL 方式）

**涉及改动**：
- `internal/notify/bot_channels.go` — 新增图片上传逻辑
- `internal/notify/message.go` — 新增 `ImagePaths` 字段
- `internal/scheduler/scheduler.go` — 截图任务结果中提取文件路径

**替代方案（无需 app 凭证）**：
- 将截图上传到图床（如 S3/OSS），返回公网 URL
- 飞书卡片中用 markdown 图片语法 `![screenshot](url)` 展示

---

## 实施优先级

| 优先级 | 阶段 | 理由 |
|--------|------|------|
| 🔴 高 | P9 截图飞书推送 | 用户明确需求，截图是核心功能 |
| 🟡 中 | P1 Chrome MCP 测试 | 需要交互式浏览器测试，可安排专门时间 |
| 🟢 低 | P2 选择器修复 | 依赖 P1 结果 |

---

## 快速命令

```bash
# P1: 启动 Chrome MCP
npx @anthropic-ai/chrome-mcp@latest

# P6 重跑（验证改动后）
go run ./cmd/unimap-web
SKIP_ST=02,03,06,07 AUTH_TOKEN=your_token ./scripts/test_scheduler_runners.sh

# 飞书通知测试
curl -X POST http://localhost:8448/api/notifications/channels/test \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -H "Origin: http://localhost:8448" \
  -d '{"id": "feishu_2"}'
```
