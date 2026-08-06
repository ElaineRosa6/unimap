# UniMap 当前状态与后续行动（2026-07-29 更新）

## 用途

本文记录按当前工作区复核后的事实，作为后续会话的快速恢复入口。详细待办和排期
分别以 [`docs/REMAINING_WORK_2026-07-23.md`](../docs/REMAINING_WORK_2026-07-23.md) 与
[`docs/IMPLEMENTATION_PLAN_2026-07-23.md`](../docs/IMPLEMENTATION_PLAN_2026-07-23.md) 为准。

## 当前版本状态

- 主开发分支：`develop`。
- 最新提交：`ba0d449`（`docs: 更新 CDP 验收证据 — Web 服务 E2E + Hunter 选择器校准`）。
- 本地领先 `origin/develop` 6 个提交，尚未推送。
- 全量验证：`go test -race ./...` 全绿、`go vet` 通过、GUI build 通过。

## 2026-07-29 已完成

### 会话熔断 + 浏览器层 SSRF（`9aaea1e`）

- `SessionHealthTracker`：per-engine 熔断器（closed/open/half-open），6 类失败分类，
  中文恢复操作指引，`LoginStatusCheckRunner` 集成健康摘要输出。
- `AllowBrowserTask()` 接入 `QueryRunner.Execute()` 浏览器任务路径：熔断打开时跳过
  对应引擎的浏览器任务，仅允许登录检查探测恢复。
- `SessionHealthTracker` 改为 `web/server.go` 共享实例，`LoginStatusCheckRunner` 和
  `QueryRunner` 共用同一熔断状态。
- 通知由 Scheduler 后置管线自动覆盖（结果文本含摘要+恢复提示）。
- `ssrf_guard.go`：`ValidateBrowserURL()` 静态校验 + `SSRFInterceptor` CDP Fetch 域
  逐跳拦截（Document/XHR/Fetch/Script），覆盖 CDP 截图、DOM 采集、collect_and_capture、
  Extension buildTargetURL、Bridge Submit。

### Quake/Hunter CDP 真实验收（`c32583e`）

- Quake：Cookie 登录态有效（用户 360U3166720809），CDP 搜索 2.06 亿条结果，
  应用代码路径采集 10 条结构化资产，PNG 截图 375KB，人工确认非登录页。
- Hunter：Cookie 登录态有效（用户 173******09），CDP 搜索 2696 万条资产，
  应用代码路径采集 10 条结构化资产，JPEG 回退截图 246KB，人工确认非登录页。
- `screenshot_fallback.go`：PNG 10s 超时 → JPEG 30s 自动降级，修复 Hunter 等复杂
  SPA 页面 PNG 合成器挂起问题。
- Cookie 已写入 `configs/config.yaml`（本地，不入 Git）。

### Hunter DOM 选择器校准（`a09897a`）

- Hunter 鹰图平台移除 checkbox 列导致所有列索引偏移 1，已修正 ExtractJS 列映射。
- 增加 checkbox 列自动检测（offset），兼容新旧两种布局。
- 新增 status_code 和 region 字段提取。
- 修复 total 提取：取统计区域最大数值，避免年份干扰。
- CDP 验证：10 条资产全部正确提取 IP/Port/Proto/Host/Title/Status/Region。

### Web 服务 E2E + 文档更新（`ba0d449`）

- 启动完整 Web 服务（端口 8448），POST `/api/v1/query` browser_query=true 返回 200。
- Hunter bridge-collect 采集 2 条资产（IP/Port/Title 正确）。
- Quake L1 Network pattern 未匹配（已知限制，待校准）。
- 验收证据文档和待办清单同步更新。

## 七引擎当前事实

| 引擎范围 | API | 稳定 Web UI | Bridge 真实结构化采集 | CDP 真实结构化采集 |
|---|---|---|---|---|
| FOFA | 已适配 | 已接入 | 2026-07-15 通过 | 未执行 |
| Hunter | 已适配 | 已接入 | 2026-07-15 通过 | **2026-07-29 通过**（10 资产 + 截图，选择器已校准） |
| ZoomEye | 已适配 | 已接入 | 2026-07-15 通过 | 未执行 |
| Quake | 已适配 | 已接入 | 2026-07-15 通过 | **2026-07-29 通过**（10 资产 + 截图，L1 Network 待校准） |
| Shodan | 已适配 | 已接入 | 2026-07-15 因登录态失效未通过 | 未执行 |
| Censys | 已适配并实机验证 | 未完成 | 未完成 | 未完成 |
| DayDayMap | 已适配并实机验证 | 未完成 | 未完成 | 未完成 |

代码存在路径、选择器或单元测试，不等于对应引擎已通过真实验收。

## 安全边界更新

浏览器层 SSRF 防护已实现（2026-07-29）：

- `ValidateBrowserURL()` 静态校验：拒绝私有/loopback/link-local/内部地址和非 HTTP(S) 协议。
- `SSRFInterceptor` CDP Fetch 域逐跳拦截：导航、重定向和子资源请求均经 urlguard 校验。
- Extension/Bridge 路径在 `buildTargetURL` 和 `BridgeService.Submit` 中增加静态校验。
- 剩余限制：DNS rebinding 的完全防护需要连接级 IP 校验（当前 Fetch 拦截在请求阶段做
  静态 URL 检查）；`metadata.google.internal` 等 DNS 名称需运行时解析才能拦截。

自动"发现变化 → 证据截图 → 图片通知"仍未启用：SSRF 防护已就绪，但需在受控云端页面
完成真实变化与飞书图片送达验收后才能启用。

## 下一步顺序

1. ~~完成浏览器逐跳/连接级 SSRF 防护。~~ **已完成（2026-07-29）**
2. ~~完成 Quake、Hunter 的真实 CDP 查询、采集、截图。~~ **已完成（2026-07-29）**
3. ~~完成 Cookie/Profile 登录探针、失败熔断。~~ **核心框架+接线已完成（2026-07-29）**
4. 校准 Quake L1 Network pattern（当前 CDP 采集返回 0 条结构化资产）。
5. 在受控云端页面完成基线、真实变化、历史、恢复、重启和飞书图片送达闭环。
6. 对 FOFA、ZoomEye、Shodan 的 CDP 能力进行真实定级。
7. 配置真实飞书/Webhook 通知目标后验证图片通知送达。
8. 决定并实施 Censys、DayDayMap 的稳定 Web UI、Bridge/CDP 与 live E2E。
9. 完成配额历史、趋势、自动刷新和阈值告警。
10. 完成只读配置 UX、受控恢复、保留策略、TLS/域名、ARM64 和 24 小时稳定性验收。
11. 在用户确认发布窗口后推送本地 `develop` 提交。

## 需要用户配合的事项

- Quake L1 Network pattern 校准和 SSRF 测试无需外部账号，可直接实施。
- FOFA/ZoomEye/Shodan CDP 定级需要对应平台合法账号和可用登录态。
- 飞书/Webhook 通知 E2E 需要配置真实通知目标凭据。
- 云端常态化验收需要确定正式服务器、域名/TLS、通知目标、备份目标和允许的维护窗口。
- 任何凭据都不得写入 Git、文档、测试数据或日志。