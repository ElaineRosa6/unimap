# E2E 采集验证报告 — Shodan & Quake

> 日期：2026-06-04
> 状态：🟡 Shodan 采集数据流诊断中（服务端日志已添加 + parseStructuredCollectedData 4项修复）；Quake 账号无权限
> 相关：[[SCHEDULER_NEXT_STEPS]] P1/P2

**最新进展 (2026-06-04 17:30)**：
- ✅ CANARY 调试代码已清理（capture.js + background.js）
- ✅ 服务端诊断日志已添加到 `PushResult` 入口（`[bridge-collect]` tag）
- ✅ **`parseStructuredCollectedData` 4 项数据映射修复**（port/status_code 字符串→整数、banner→BodySnippet、known map 补全）+ 5 新测试
- ⏸️ 等待用户重启 Chrome + 服务器后重新测试 Shodan 采集，查看日志输出定位数据丢失点

---

## 一、执行摘要

通过 Web UI / API 对 Shodan 和 Quake 引擎进行了 Extension 端到端采集验证。截图采集完全成功（Shodan 6/6, Quake 2/2），但 DOM 结构化数据采集返回空（0 assets）。

在调试过程中发现并修复了 3 个代码 bug：
1. `waitForPageReady` SPA 策略超时（tab 已完成时不触发 onUpdated）
2. `extractCellText` 作用域错误（模块级函数无法在注入函数中访问）
3. Shodan 选择器过时（`.heading + div > div` 匹配的是详情区而非结果行）

---

## 二、验证环境

| 项目 | 值 |
|------|-----|
| Chrome | 150.0.7865.2 + CDP :9222 |
| Profile | Main profile (junction): `chrome-cdp-profile` |
| Extension | 开发者模式，`tools/extension-screenshot/` |
| Server | `unimap-web` :8448，screenshot engine=extension |
| Auth token | `1AggpIRXaHIQnH73SawdMLDfB8RnIy3X` |

---

## 三、已完成项

### 3.1 截图采集验证 ✅

| 引擎 | URL | 批处理 | 截图尺寸 | 结果 |
|------|-----|--------|---------|------|
| Shodan | `port:80` 搜索 | 1/1 | 2561x1398, 238KB | 含搜索结果 |
| Shodan | 6 个 IP+port 查询 | 6/6 | 2561x1398, 76-78KB | 通过 |
| Quake | 2 个 IP 查询 | 2/2 | 2561x1398, 163-165KB | 通过（绕过反爬！） |

### 3.2 CDP 选择器验证 ✅

用 Puppeteer 连接 Chrome CDP，直接在 Shodan 搜索结果页验证：

| 选择器 | 匹配数 | 状态 |
|--------|--------|------|
| `.row.l-search-results .result` | 10 | ✅ 精确匹配结果行 |
| `.nine.columns .result` | 10 | ✅ |
| `.heading + div > div` | 20 | ⚠️ 匹配详情/横幅区域（旧选择器，错误） |
| `a[href*='/host/']` | 10 | ✅ IP 链接 |

提取结果示例：
```
IP: 143.204.142.130 (从 /host/143.204.142.130 提取)
Title: "ERROR: The request could not be satisfied"
Banner: "HTTP/1.1 403 Forbidden..."
Org: "server-143-204-142-130.slc52.r.cloudfront.net"
```

### 3.3 Bug 修复 1: `waitForPageReady` SPA 超时

**文件**: `tools/extension-screenshot/src/capture.js:117-127`

**根因**: SPA 策略先等 5s，然后监听 `chrome.tabs.onUpdated` 的 "complete" 事件。但如果 tab 在 5s 内已完成加载，监听器永不触发 → 超时。

**修复**: 在 5s 延迟后检查 tab 是否已 "complete"，若是则直接返回（与 "load" 策略一致）。

### 3.4 Bug 修复 2: `extractCellText` 作用域 ⭐ 关键发现

**文件**: `tools/extension-screenshot/src/capture.js:478-509`

**根因**: `extractCellText` 原定义在模块级（capture.js 顶层），但调用发生在 `chrome.scripting.executeScript` 注入的函数内部。Chrome 序列化注入函数时无法访问外部作用域 → 调用 `extractCellText(row, cfg)` 时函数为 undefined → 卡片式引擎（Shodan/Quake/ZoomEye）永远返回空 items。

**为什么 FOFA/Hunter 能工作**: 表格式引擎走 `extractCellTextFromCells`，该函数定义在注入函数**内部**，可以正常访问。

**修复**: 将 `extractCellText` 从模块级移入注入函数内部（`rows.forEach` 之前）。

**影响范围**: Shodan、Quake、ZoomEye 的卡片/div 式提取全部修复。

### 3.5 Bug 修复 3: Shodan 选择器更新

**文件**: `tools/extension-screenshot/src/capture.js:342-378`

| 字段 | 旧选择器 | 新选择器 | 原因 |
|------|---------|---------|------|
| row (P0) | `.heading + div > div` | `.row.l-search-results .result` | 旧选择器匹配详情/横幅区域,不是结果行 |
| row (P1) | `div.search-results > div` | `.nine.columns .result` | 作为备选 |
| ip | `.ip, a[href*='/host/']` | `a[href*='/host/']` (attr:href, extract:ip_from_path) | IP 从 href 提取最可靠 |
| org | `.org, ...` | `.result-details, .org, ...` | 新选择器 |
| banner | `.banner, pre` | `.banner-data, pre, ...` | 新选择器 |
| country_code | `.country, ...` | `.result-details, .country, ...` | 添加详情区域 |

同时新增 `extractCellText` 的 `attr` 和 `extract` 支持：
- `attr: "href"` — 提取属性值而非文本
- `extract: "ip_from_path"` — 从 `/host/X.X.X.X` 路径解析 IP

### 3.6 Bug 修复 4: `parseStructuredCollectedData` 数据映射修复 ⭐

**文件**: `internal/screenshot/router.go:730-827`

**根因**: Extension 从 DOM 提取的所有字段值都是**字符串**（`extractCellText` 返回 `textContent.trim()`），但 Go 端 `parseStructuredCollectedData` 对 `port`、`status_code` 只做 `float64`/`int` 类型断言 → 类型不匹配，值被静默丢弃。同时 `banner` 字段（Shodan 关键数据）未被映射到 `BodySnippet`。

**修复项**:

| Bug | 字段 | 问题 | 修复 |
|-----|------|------|------|
| 1 | `port` | Extension 发 `"80"`（string），Go 只认 `float64`/`int` | 新增 `string` → `strconv.Atoi` 分支 |
| 2 | `status_code` | Extension 发 `"200"`（string），Go 只认 `float64` | 同上 |
| 3 | `banner` | Extension 发 `banner`，Go 只映射 `body_snippet` → `BodySnippet` | 新增 `banner` → `BodySnippet` 回退映射（`body_snippet` 优先） |
| 4 | `known` map | `banner`/`os` 未标记为已知字段 | 补全 `known` map，防止重复出现在 `Extra` |

**测试**: 新增 5 个测试用例（`internal/screenshot/router_test.go`）:
- `TestParseStructuredCollectedData_PortAsString` — port string→int
- `TestParseStructuredCollectedData_PortAsInvalidString` — 非法字符串→0
- `TestParseStructuredCollectedData_StatusCodeAsString` — status_code string→int
- `TestParseStructuredCollectedData_BannerToBodySnippet` — banner→BodySnippet + 不出现在 Extra
- `TestParseStructuredCollectedData_BodySnippetPreferredOverBanner` — body_snippet 优先

**影响**: 修复前 Extension 采集的 port、status_code、banner 数据在服务端被静默丢弃。修复后这些字段正确映射到 `UnifiedAsset`。

### 3.7 修改文件清单更新

**文件**: `tools/extension-screenshot/src/background.js:94-105`

- collect action 不再强制覆盖 wait strategy 为 "spa"（使用 server 指定的 "load"）
- 超时从 15s → 30s
- 新增 4s SPA 额外渲染等待

---

## 四、未完成 / 阻塞问题

### 4.1 🔴 Shodan 采集返回 0 assets（阻塞）

**现象**: 尽管 CDP/ Puppeteer 直测证明提取逻辑正确（10 items），但通过 Extension → Bridge → Server 的完整链路返回 `assets: 0`。

**已确认的环节**:

| 环节 | 验证方法 | 结果 |
|------|---------|------|
| capture.js 代码正确 | CDP 打开 chrome-extension:// 文件 → CANARY 确认 | ✅ |
| background.js 代码正确 | CDP 打开 chrome-extension:// 文件 → CANARY-BG-v1 确认 | ✅ |
| Server 收到 callback | 日志 `POST /mock/result status=200` | ✅ |
| Server 二进制正确 | `strings` 检查 MARKER 和 debug 字符串 | ✅ |
| `ExtensionProvider.CollectSearchEngineResult` 被调用 | MARKER-v1 出现在 API 响应 title 中 | ✅ |

**待定位**: 数据在 Extension `reportResult()` → HTTP POST → Server `decodeJSONReader` → `PushResult` → `AwaitResult` 链路的哪一步丢失。

**可疑点**:
- `decodeJSONReader` 使用 `DisallowUnknownFields()` — 可能有额外字段导致解析失败（但日志显示 status=200，说明没有失败）
- bridge mock 的 `requestID` 匹配问题
- `apiPostBridgeSigned` 发送的 JSON body 格式问题

### 4.2 🔒 Quake 反爬检测拦截（非权限问题）

**更正（2026-06-07）**：此前误判为"账号无搜索权限"，实为**反爬检测**。账号 `360U3166720809` **拥有正常搜索权限——用户手动查询可正常返回结果**。CDP/自动化访问时出现"用户缺少必要权限"/"暂无数据"，是 360 反爬体系识别出自动化特征（`Page.navigate` CDP 协议、缺少 stealth 伪装）后返回的拦截响应，**不是真实的权限错误**。

**解决方向**：需 Extension 真实浏览器路径 + stealth 绕过（webdriver/指纹伪装等），详见 `docs/EXTENSION_ANTI_SCRAPING_ARCHITECTURE.md`。Extension 可访问页面（标题正确："360网络空间测绘"），但仍需补足 stealth 才能让自动触发的搜索返回真实数据。

---

## 五、修改文件清单

| 文件 | 修改内容 | 状态 |
|------|---------|------|
| `tools/extension-screenshot/src/capture.js` | waitForPageReady SPA 修复 | ✅ |
| `tools/extension-screenshot/src/capture.js` | extractCellText 移入注入函数 | ✅ |
| `tools/extension-screenshot/src/capture.js` | Shodan 选择器更新 + attr/extract 支持 | ✅ |
| `tools/extension-screenshot/src/capture.js` | CANARY 调试代码清理 | ✅ |
| `tools/extension-screenshot/src/background.js` | collect 超时优化 + wait strategy | ✅ |
| `tools/extension-screenshot/src/background.js` | CANARY-BG-v1 调试代码清理 | ✅ |
| `internal/screenshot/router.go` | MARKER-v1 调试代码（已 git checkout 恢复） | ✅ 已恢复 |
| `internal/screenshot/router.go` | 🆕 `parseStructuredCollectedData` 4 项修复（port/status_code string→int、banner→BodySnippet、known map） | ✅ |
| `internal/screenshot/router_test.go` | 🆕 5 个测试用例（PortAsString、StatusCodeAsString、BannerToBodySnippet 等） | ✅ |
| `web/screenshot_bridge_handlers.go` | 🆕 诊断日志 `[bridge-collect]` — PushResult 前打印 items/total/engine/method/rows + 前3条数据 | ✅ |

---

## 六、Chrome MV3 开发注意事项（踩坑记录）

1. **touch manifest.json 不会重载 service worker** — 必须重启 Chrome 才能加载新代码
2. **`chrome.scripting.executeScript` 的 func 参数是隔离作用域** — 注入函数无法访问模块级函数，所有辅助函数必须定义在注入函数内部
3. **多个二进制同名问题** — `taskkill` 后 port 可能未完全释放，新 server 启动失败；旧二进制继续运行 → debug log 不生效。需确认进程已终止再启动

---

## 七、下一步计划

1. **🔴 定位数据丢失点** — 重启 Chrome + 服务器，运行 Shodan collect 测试，查看 `[bridge-collect]` 日志
2. **验证 ZoomEye 采集** — 确认 extractCellText 修复后 ZoomEye 采集恢复正常
3. **Quake 账号升级** — 联系获取搜索权限
4. **5 引擎全量 E2E** — Extension 模式下对所有 5 引擎执行采集验证

### 测试步骤

```bash
# 1. 完全退出 Chrome，重新启动（MV3 service worker 不会热重载）
#    确保 Extension 已加载（开发者模式 → tools/extension-screenshot/）

# 2. 启动服务器
go run ./cmd/unimap-web

# 3. 测试 Shodan 采集（查看另一个终端的服务器日志）
ADMIN="<your_admin_token>"
curl -s -X POST "http://127.0.0.1:8448/api/v1/query" \
  -H "Authorization: Bearer $ADMIN" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -H "Origin: http://localhost:8448" \
  --data-urlencode 'query=port="80"' \
  --data-urlencode 'engines=shodan' \
  --data-urlencode 'browser_query=true' \
  --data-urlencode 'browser_action=collect'

# 4. 检查服务器日志中的 [bridge-collect] 行：
#    - items=N 应该 > 0（如果 Shodan 选择器生效）
#    - 检查 item[0]/item[1]/item[2] 的 ip/port/title 值
#    - 如果 items=0，检查 extraction_method/rows_found/row_selector_used 诊断字段
```

---

## 八、快速命令

```bash
# 启动 Chrome + CDP
"/c/Program Files/Google/Chrome/Application/chrome.exe" \
  --remote-debugging-port=9222 \
  --user-data-dir="C:\Users\ljw\AppData\Local\Temp\chrome-cdp-profile" &

# 启动 server
go run ./cmd/unimap-web

# 测试 Shodan 采集
ADMIN="1AggpIRXaHIQnH73SawdMLDfB8RnIy3X"
curl -s -X POST "http://127.0.0.1:8448/api/v1/query" \
  -H "Authorization: Bearer $ADMIN" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -H "Origin: http://localhost:8448" \
  --data-urlencode 'query=port="80"' \
  --data-urlencode 'engines=shodan' \
  --data-urlencode 'browser_query=true' \
  --data-urlencode 'browser_action=collect'

# CDP 测试 Shodan 选择器
node tools/test_extraction_direct.mjs

# 检查 bridge 状态
curl -s http://127.0.0.1:8448/api/v1/screenshot/bridge/status \
  -H "Authorization: Bearer $ADMIN"
```
