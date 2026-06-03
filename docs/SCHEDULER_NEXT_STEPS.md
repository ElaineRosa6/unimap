# 定时任务系统 — 下一步实施计划

> 创建日期：2026-06-02
> 更新日期：2026-06-02
> 前置状态：P3-P8 已完成（P8 已全量覆盖 22 个 Runner）

---

## 已完成

| 阶段 | 内容 | 状态 |
|------|------|------|
| P3 | 任务类型分组 + Cron 快捷预设 | ✅ |
| P4 | 任务模板系统 + 参数提示 + JSON 校验 | ✅ |
| P5 | 测试脚本 + 22 个 payload JSON | ✅ |
| P6 | 逐项 Runner 测试执行 | ✅ 18/22 通过 |
| P7 | 飞书通知推送验证 | ✅ |
| P8 | 通知内容增强（全量 22 个 Runner + UTF-8 防乱码） | ✅ |
| P9 | 截图飞书推送（飞书应用 API 图片上传） | ✅ |

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
- `internal/notify/bot_channels.go` — 新增 `FeishuAppChannel`（getToken/uploadImage/sendMessage/Send）
- `internal/notify/message.go` — 新增 `ImagePaths` 字段
- `internal/notify/registry.go` — 新增 `Pin()` 方法，防止 Reload 移除 feishu_app
- `internal/scheduler/scheduler.go` — `extractImagePaths()` 从结果中提取截图路径 + 修复重复提取 bug
- `web/server.go` — 注册 feishu_app 后 Pin

**2026-06-03 进展**：
- ✅ `extractImagePaths` bug 修复（箭头格式重复提取）
- ✅ `feishu_app` 被 Reload 移除的 bug 修复（Pin 机制）
- ✅ 18 个新单元测试全部通过（extractImagePaths + FeishuAppChannel + Pin/Reload）
- ✅ 通知全链路代码验证：任务执行 → 通知触发 → 渠道查找 → Send 调用 → getToken 请求飞书 API
- ✅ Go HTTP Transport 优化：IPv4 强制 + 自定义超时
- ✅ **乱码修复**：GBK→UTF-8 三层防御（HTTP入口/存储层/通知层），sanitizeUTF8 增强
- ✅ **飞书通知验证**：webhook 渠道通知正常到达，任务名和结果中文正确显示
- ✅ **截图超时**：根因已定位（服务器重启 → Bridge token 丢失 → Extension 401）
- ✅ **截图超时已修复并验证**：Admin Token fallback 方案落地，单元 + 真机 curl E2E 全绿

**截图超时修复（2026-06-03 闭环）**：
1. Chrome Extension 跨域请求不带 session cookie → "Admin session 兜底"方案不可行（已放弃，判断正确）
2. ✅ 改为 Admin Token 直接比对：`validateBridgeAuthIfRequired` 在 loopback 下接受静态 admin token（常量时间比对），服务器重启后扩展无需重新配对即可认证
3. ✅ `callback_signature_required` 与 `pairing_required` 已联动：pairing 关闭或 admin-token 路径（token=="")时跳过签名校验
4. ✅ 清理遗留调试日志：`[bridge-auth]` 每请求 Infof（扩展每秒轮询会刷屏）降级为仅 admin 接受时的 Debugf
5. ✅ 新增 5 个单元测试（admin 兜底接受/非 loopback 拒绝/未知 token 401/pairing 关闭跳签名/admin 任务拉取）
6. ✅ 真机 curl E2E（服务器运行中）：pair→200 / bridge token 拉取→200 / **admin token 兜底拉取→200** / bogus token→401 / admin 无签名回调→200；且观测到真实扩展 `paired_clients=1, live_clients=1`

**验证命令**（见文末"真机 E2E"）。Go：`go build ./... && go test ./web/ -race` 全绿。

**替代方案（无需 app 凭证）**：
- 将截图上传到图床（如 S3/OSS），返回公网 URL
- 飞书卡片中用 markdown 图片语法 `![screenshot](url)` 展示

---

## 实施优先级

| 优先级 | 阶段 | 状态 | 理由 |
|--------|------|------|------|
| 🔴 高 | 乱码修复 | ✅ 已完成 | GBK→UTF-8 三层防御，飞书通知中文正确 |
| 🔴 高 | P9 截图飞书推送 | ✅ 已完成 | 飞书通知链路打通 |
| 🔴 高 | 截图超时修复 | ✅ 已完成 | Bridge token 认证改造（Admin Token fallback）+ 单元/真机 E2E 验证 |
| 🟡 中 | P1 Chrome MCP 测试 | ⏸ 待实施 | 需要交互式浏览器测试 |
| 🟢 低 | P2 选择器修复 | ⏸ 待实施 | 依赖 P1 结果 |

---

## 快速命令

```bash
# P1: 启动 Chrome MCP
npx @anthropic-ai/chrome-mcp@latest

# P6 重跑（验证改动后）
go run ./cmd/unimap-web
SKIP_ST=02,03,06,07 AUTH_TOKEN=your_token ./scripts/test_scheduler_runners.sh

# 飞书通知测试（webhook 渠道）
curl -X POST http://localhost:8448/api/notifications/channels/test \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -H "Origin: http://localhost:8448" \
  -d '{"id": "feishu_2"}'

# 飞书应用测试（带图片上传）
# 创建一个截图任务，配置 notifications.channel_ids=["feishu_app"]
# 任务完成后会自动上传截图到飞书群
```

---

## 真机 E2E：Bridge Token 认证（截图超时修复验证）

> 前置：`go build -o /tmp/unimap-web.exe ./cmd/unimap-web && /tmp/unimap-web.exe`
> 把 `ADMIN` 换成 `configs/config.yaml` 里的 `web.auth.admin_token`。

```bash
ADMIN="<your admin_token>"
B="http://127.0.0.1:8448"

# 1) 配对（loopback，无需 admin）→ 拿到临时 bridge token
curl -s -X POST "$B/api/v1/screenshot/bridge/pair" \
  -H "Content-Type: application/json" \
  -d '{"client_id":"curl-e2e","pair_code":"dev-pair"}'      # 期望 success:true + token

# 2) 用 bridge token 拉任务 → 期望 HTTP 200
curl -s -o /dev/null -w "%{http_code}\n" "$B/api/v1/screenshot/bridge/tasks/next" \
  -H "Authorization: Bearer <token from step 1>"

# 3) 用 ADMIN token 拉任务（重启后兜底路径）→ 期望 HTTP 200
curl -s -o /dev/null -w "%{http_code}\n" "$B/api/v1/screenshot/bridge/tasks/next" \
  -H "Authorization: Bearer $ADMIN"

# 4) 用错误 token 拉任务 → 期望 HTTP 401
curl -s -o /dev/null -w "%{http_code}\n" "$B/api/v1/screenshot/bridge/tasks/next" \
  -H "Authorization: Bearer bogus"

# 5) ADMIN token 回调（无签名）→ 期望 HTTP 200（admin 路径跳过签名）
curl -s -o /dev/null -w "%{http_code}\n" -X POST "$B/api/v1/screenshot/bridge/mock/result" \
  -H "Content-Type: application/json" -H "Authorization: Bearer $ADMIN" \
  -d '{"request_id":"e2e-1","success":true,"image_path":"./screenshots/x.png"}'
```

**2026-06-03 实测结果**：1) token ✓ 2) 200 ✓ 3) 200 ✓ 4) 401 ✓ 5) 200 ✓；
`bridge/status` 观测到真实扩展 `paired_clients=1, live_clients=1`。

### 待用户做的真机截图回环（需 Chrome 扩展 + extension 引擎）

> 当前 `screenshot.engine=cdp` 优先且 CDP 健康，普通截图会走 CDP 不经扩展。
> 要验证扩展截图链路：临时设 `screenshot.engine=extension` 或让 CDP 不可用，
> 然后在已配对扩展的情况下触发一次批量截图，确认结果返回而非超时。

