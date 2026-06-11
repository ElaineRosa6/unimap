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

### P1：Chrome MCP DOM 采集测试 🔄 进行中

> 更新：2026-06-03

**目标**：验证 5 个搜索引擎的浏览器采集选择器是否有效

**环境要求**：
- Node.js ✅ (v24.14.1 已安装)
- Chrome ✅ (已安装，已登录各引擎)
- Chrome CDP ✅ (`--remote-debugging-port=9222`)

**测试项**：

| 引擎 | 搜索页 | 验证内容 |
|------|--------|----------|
| FOFA | `fofa.info/result?qbase64=...` | 结果列表选择器、IP/端口/标题字段 |
| Hunter | `hunter.qianxin.com/list?searchValue=...` | 结果卡片、资产信息 |
| Quake | `quake.360.net/quake/#/searchResult?searchVal=...` | 结果表格、字段映射 |
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
| 🟡 中 | P1 Chrome MCP 测试 | 🟡 4/5 通过 | CDP 4 轮 30 测试：FOFA/Hunter/ZoomEye/Shodan ✅，Quake ❌（反爬检测） |
| 🟡 中 | P2 选择器修复 | 🟡 进行中 | capture.js FOFA/Hunter/ZoomEye 已修复；**🆕 parseStructuredCollectedData 4项修复（port/status_code string→int、banner→BodySnippet）+ 5测试**；Quake 待 Extension；飞书路径泄露已修复 |
| 🔵 低 | 飞书路径泄露 | ✅ 已修复 | bot_channels 不再显示路径；Result 路径已剥离为仅文件名 |

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

---

## P1/P2 进展记录 (2026-06-04 更新)

### CDP 测试执行情况（第 3-4 轮）

Chrome CDP (`--remote-debugging-port=9222`)，通过 junction 方式加载主 profile（保留登录会话）。

**测试配置**：
- 测试脚本：`tools/test_dom_selectors.mjs`（端口限定查询，减少消耗）
- 测试目标：6 个 IP+端口组合（Google DNS:53/443, Cloudflare DNS:53/443, Alibaba Cloud:80/443）
- 5 引擎 × 6 目标 = 30 测试
- 截图存档：`docs/test_screenshots/`

### 各引擎 DOM 选择器验证结果（最终）

| 引擎 | 测试 | 登录 | Row 选择器 | Cell 提取 | 状态 |
|------|------|------|-----------|----------|------|
| **FOFA** | 6/6 ✅ | ✅ | `.hsxa-meta-data-item` (2-10 rows) | 8/8 字段 | ✅ 通过 |
| **Hunter** | 4/6 ✅ | ✅ | `.q-table tbody tr` (24-30 rows) | 8 列 table | ✅ 通过（2 个无数据） |
| **ZoomEye** | 6/6 ✅ | ✅ | `.search-result-item` (10 rows) | ⚠️ card-based | ✅ 通过 |
| **Shodan** | 6/6 ✅ | ✅ | `.heading + div > div` (2-3 rows) | IP/Title/Banner ✅ | ✅ 通过 |
| **Quake** | 0/6 ❌ | ✅ 已登录 | — | — | 🔒 反爬检测拦截（账号有权限，手动查询正常） |

### 各引擎 URL 格式（CDP 验证正确）

| 引擎 | 正确 URL | 关键发现 |
|------|---------|---------|
| FOFA | `qbase64=base64(ip="X.X.X.X" && port="YYYY")` | 无需登录也可访问 |
| Hunter | `search=base64url(ip="X.X.X.X")` | 用 `search=` 不是 `searchValue=` |
| ZoomEye | `q=ip:X.X.X.X` | 端口过滤语法不支持 |
| Quake | `#/searchResult?searchVal=ip:"X.X.X.X"` | 反爬检测拦截（账号有权限，手动查询正常） |
| Shodan | `?query=net:"X.X.X.X/32" port:YYYY` | 搜索过滤需登录 |

### 关键发现（第 3-4 轮新增）

1. **FOFA `a[href*='qbase64=aXA9']` 误匹配**：该选择器匹配国家过滤链接（如"美国"），不是 IP。`span.hsxa-host`（返回"IP:Port"）才是正确的 IP 选择器
2. **Hunter 用 `search=` 不是 `searchValue=`**：旧参数名导致"语法不能为空"
3. **Hunter class 选择器全部无效**：`.ip-address`、`.port` 等 class 不存在，需用 `td:nth-child()` 列索引
4. **Hunter 列映射**：td:nth-child(2)=IP, (3)=域名, (4)=端口/服务, (5)=标题, (6)=状态码, (7)=ICP, (8)=组件
5. **ZoomEye `.ant-table tbody tr` 已死**：0 匹配。`.search-result-item`（10 匹配）是正确选择器
6. **Shodan 选择器验证通过**：`.heading + div > div`、`.result`、`div[class*='result']` 均有效
7. **Quake 反爬检测（非权限问题，2026-06-07 更正）**：`360U3166720809` **账号有正常搜索权限，用户手动查询可正常返回**。CDP 访问返回"用户缺少必要权限"/"暂无数据"是 360 反爬体系识别自动化后的拦截响应，不是真实权限错误。需 Extension 真实浏览器 + stealth 绕过
8. **Quake DOM 框架**：仍使用 Element UI（`el-*` 类名 186 个），但无 `<table>` 元素

### capture.js 改动清单（累计，含第 3-4 轮修复）

| 修改项 | 文件 | 说明 |
|--------|------|------|
| 新增 `shodan` 引擎检测 | `capture.js:19` | `detectEngine()` 添加 `shodan.io` 识别 |
| FOFA IP selector 修复 | `capture.js:220` | `span.hsxa-host` 提升为 PRIMARY（`a[href*='qbase64=aXA9']` 误匹配已修复） |
| Hunter cell 列索引修复 | `capture.js:254-260` | td:nth-child(2)=IP, (3)=域名, (4)=端口, (5)=标题, (6)=状态码, (7)=ICP |
| Hunter cell 移除无效 class | `capture.js:254-260` | 移除 `.ip-address`, `.port`, `.protocol` 等不存在于 DOM 的选择器 |
| Hunter cell 新增字段 | `capture.js:259-260` | 新增 `status_code` 和 `org` |
| ZoomEye row 优先级修正 | `capture.js:268-280` | `.search-result-item` 提升为首选，`.ant-table` 降级为 DEPRECATED fallback |
| 诊断字段透传 | `background.js` | `structured_collected_data` 包含诊断字段 |

### Go 端数据映射修复 (2026-06-04)

> 详见 `docs/E2E_COLLECTION_VERIFICATION_2026-06-04.md` §3.6

| Bug | 文件 | 问题 | 修复 |
|-----|------|------|------|
| port 静默丢弃 | `internal/screenshot/router.go:769-773` | Extension 发 `"80"`（string），Go 只认 `float64`/`int` | 新增 `string` → `strconv.Atoi` |
| status_code 静默丢弃 | `internal/screenshot/router.go:786-788` | 同上 | 同上 |
| banner 映射缺失 | `internal/screenshot/router.go:780-782` | Extension 发 `banner`，Go 只映射 `body_snippet` | 新增 `banner` → `BodySnippet` 回退（`body_snippet` 优先） |
| known 字段不全 | `internal/screenshot/router.go:809-814` | `banner`/`os` 未标记 → Extra 重复 | 补全 known map |

**测试覆盖**: 5 新测试（`router_test.go`），`go test -race ./...` 全绿。

### 测试脚本改动

| 文件 | 改动 |
|------|------|
| `tools/test_dom_selectors.mjs` | 端口限定重写：6 个 IP+port 组合 |
| 同上 | Hunter URL 修复：`search=base64url()` |
| 同上 | ZoomEye URL 修复：`q=ip:X.X.X.X` |
| 同上 | 移除 Quake/Shodan 的"需登录"限制，纳入全量测试 |
| 同上 | Windows 文件名校验（`:` → `_`） |

### 遇到的问题

1. **Quake CDP 反爬检测（2026-06-04）**：
   - 用户手动查询有效，但通过 CDP 访问返回"暂无数据"或"用户缺少必要权限"
   - 确认 Quake 检测到了自动化访问（`Page.navigate` CDP 协议），触发了反爬机制
   - **解决方案**：必须走 Chrome Extension 路径（真实浏览器交互可绕过检测）
   - **影响**：Quake DOM 选择器无法通过 CDP 验证，只能在 Extension E2E 阶段验证

2. **飞书通知路径泄露（2026-06-04，已修复）**：
   - 截图上传失败时错误消息暴露服务器文件路径
   - 通知 Result 文本中嵌入完整路径（如 `screenshots/batch1/shot.png`）
   - 修复：bot_channels 失败消息移除路径 + Result 路径剥离为仅文件名（`redactImagePaths`）
   - 涉及文件：`bot_channels.go` + `scheduler.go`

3. **Hunter URL 参数名错误**：
   - 旧：`searchValue=base64(...)` → "语法不能为空"
   - 正：`search=base64url(...)` → 已修复于 test_dom_selectors.mjs

4. **ZoomEye 端口过滤不生效**：
   - `+port:XX` 语法在 ZoomEye 当前版本中不支持
   - 使用 `ip:X.X.X.X` 单 IP 查询已够精确

### 下一步：Extension 端到端验证

**目标**：验证 Extension 在真实浏览器中对各引擎的采集功能是否正常。

**前置条件**：
- ✅ Chrome CDP 可用（`--remote-debugging-port=9222`）+ junction 方式加载主 profile
- ✅ Extension 已加载（开发者模式 → `tools/extension-screenshot/`）
- ✅ 各引擎已登录（FOFA/Hunter/ZoomEye/Shodan 均验证通过，Quake 待验证）
- ⏸️ 服务器未运行（需先 `go run ./cmd/unimap-web`）

**验证步骤**：

```bash
# 1. 启动服务器
go run ./cmd/unimap-web

# 2. 确认 Extension 配对成功
curl http://127.0.0.1:8448/api/v1/screenshot/bridge/status

# 3. 触发批量截图/采集任务
#    Web UI → 定时任务 → 创建截图任务 → 执行
#    或通过 API 创建任务

# 4. 检查采集结果
#    查看 structured_collected_data 是否正确提取了 IP/Port/Title 等字段
```

**验证要点**（按引擎）：

| 引擎 | 验证项 | 预期 |
|------|--------|------|
| FOFA | `.hsxa-meta-data-item` row + `span.hsxa-host` IP | IP 显示为 "X.X.X.X:Port" |
| Hunter | `.q-table tbody tr` row + `td:nth-child(2)` IP | IP 正确提取 |
| ZoomEye | `.search-result-item` row + card cells | IP/Port/Title 提取正确 |
| Shodan | `.heading + div > div` row + `.result` cells | IP/Title/Banner 提取 |
| Quake | 真实浏览器绕过反爬 | 待首次验证 |

### 快速命令

```bash
# 启动 Chrome CDP（主 profile，junction 方式）
"C:\Program Files\Google\Chrome\Application\chrome.exe" --remote-debugging-port=9222 --user-data-dir="C:\Users\ljw\AppData\Local\Temp\chrome-cdp-profile"

# CDP 测试
node tools/test_dom_selectors.mjs

# 查看测试截图
ls docs/test_screenshots/

# 查看测试结果 JSON
cat docs/test_screenshots/dom_selector_results.json
```
